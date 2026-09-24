package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"
)

const outboundProxyFile = "outbound-proxy.json"

// OutboundProxySettings 软件全局出站代理（Clash 等）
type OutboundProxySettings struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
}

// ProxyTestResult 代理连通测试结果
type ProxyTestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Latency int64  `json:"latency"`
}

var (
	outboundProxyMu      sync.RWMutex
	outboundProxyCurrent OutboundProxySettings
)

// switchableTransport 固定挂在 http.DefaultTransport 上；切换代理只原子替换内部的
// *http.Transport。此前直接改写 http.DefaultTransport 与路由网关的 client 字段，
// 与正在处理的请求并发读写，是数据竞争。
type switchableTransport struct {
	current atomic.Pointer[http.Transport]
}

func (s *switchableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return s.current.Load().RoundTrip(req)
}

func (s *switchableTransport) CloseIdleConnections() {
	if t := s.current.Load(); t != nil {
		t.CloseIdleConnections()
	}
}

var outboundTransport = func() *switchableTransport {
	st := &switchableTransport{}
	t, _ := newOutboundTransport(OutboundProxySettings{})
	st.current.Store(t)
	return st
}()

func init() {
	// 所有未显式指定 Transport 的 http.Client（路由网关、监控、云同步、更新检查）都走这里
	http.DefaultTransport = outboundTransport
}

func outboundProxyPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, outboundProxyFile), nil
}

func loadOutboundProxy() OutboundProxySettings {
	cfg := OutboundProxySettings{}
	path, err := outboundProxyPath()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	if normalized, err := normalizeProxyURL(cfg.URL); err == nil {
		cfg.URL = normalized
	}
	return cfg
}

func saveOutboundProxy(cfg OutboundProxySettings) error {
	path, err := outboundProxyPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	// 代理地址可能带账号密码，仅本人可读
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func applyOutboundProxy(cfg OutboundProxySettings) error {
	if cfg.Enabled {
		normalized, err := normalizeProxyURL(cfg.URL)
		if err != nil {
			return err
		}
		cfg.URL = normalized
	}

	transport, err := newOutboundTransport(cfg)
	if err != nil {
		return err
	}

	outboundProxyMu.Lock()
	outboundProxyCurrent = cfg
	old := outboundTransport.current.Swap(transport)
	outboundProxyMu.Unlock()
	// 旧代理的空闲连接不会再被复用，及时关掉
	if old != nil && old != transport {
		old.CloseIdleConnections()
	}
	return nil
}

// GetOutboundProxy / SetOutboundProxy / TestOutboundProxy 供设置页出站代理使用。
func (a *App) GetOutboundProxy() OutboundProxySettings {
	outboundProxyMu.RLock()
	defer outboundProxyMu.RUnlock()
	return outboundProxyCurrent
}

func (a *App) SetOutboundProxy(cfg OutboundProxySettings) error {
	if cfg.Enabled {
		normalized, err := normalizeProxyURL(cfg.URL)
		if err != nil {
			return err
		}
		cfg.URL = normalized
	} else if strings.TrimSpace(cfg.URL) != "" {
		if normalized, err := normalizeProxyURL(cfg.URL); err == nil {
			cfg.URL = normalized
		}
	}
	if err := saveOutboundProxy(cfg); err != nil {
		return err
	}
	return applyOutboundProxy(cfg)
}

func (a *App) TestOutboundProxy(cfg OutboundProxySettings) ProxyTestResult {
	start := time.Now()
	if cfg.Enabled {
		if _, err := normalizeProxyURL(cfg.URL); err != nil {
			return ProxyTestResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
		}
	}
	transport, err := newOutboundTransport(cfg)
	if err != nil {
		return ProxyTestResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}
	client := &http.Client{Timeout: 8 * time.Second, Transport: transport}
	req, err := http.NewRequest(http.MethodGet, "https://www.gstatic.com/generate_204", nil)
	if err != nil {
		return ProxyTestResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}
	req.Header.Set("User-Agent", "AI-ENV/"+appVersion)
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return ProxyTestResult{Success: false, Message: sprintf("无法连接: %v", err), Latency: latency}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 32<<10))
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		via := tr("系统网络")
		if cfg.Enabled {
			via = displayProxyURL(cfg.URL)
		}
		return ProxyTestResult{Success: true, Message: sprintf("出站正常（%s）", via), Latency: latency}
	}
	return ProxyTestResult{Success: false, Message: fmt.Sprintf("HTTP %d", resp.StatusCode), Latency: latency}
}

func normalizeProxyURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errorf("请填写代理地址，例如 http://127.0.0.1:7890")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || strings.TrimSpace(u.Host) == "" {
		return "", errorf("代理地址无效")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "socks5", "socks5h":
	default:
		return "", errorf("仅支持 http / https / socks5")
	}
	if u.Port() == "" {
		return "", errorf("请带上端口，例如 127.0.0.1:7890")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	return u.String(), nil
}

func displayProxyURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "****")
	}
	return u.String()
}

func newOutboundTransport(cfg OutboundProxySettings) (*http.Transport, error) {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   12 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	if !cfg.Enabled {
		return transport, nil
	}

	raw, err := normalizeProxyURL(cfg.URL)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(u.Scheme) {
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if u.User != nil {
			pass, _ := u.User.Password()
			auth = &proxy.Auth{User: u.User.Username(), Password: pass}
		}
		dialer, err := proxy.SOCKS5("tcp", u.Host, auth, proxy.Direct)
		if err != nil {
			return nil, errorf("创建 SOCKS5 代理失败: %v", err)
		}
		direct := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
		transport.Proxy = nil
		if cd, ok := dialer.(proxy.ContextDialer); ok {
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				if shouldBypassProxy(addr) {
					return direct.DialContext(ctx, network, addr)
				}
				return cd.DialContext(ctx, network, addr)
			}
		} else {
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				if shouldBypassProxy(addr) {
					return direct.DialContext(ctx, network, addr)
				}
				return dialer.Dial(network, addr)
			}
		}
	default:
		transport.Proxy = func(req *http.Request) (*url.URL, error) {
			if req.URL != nil && shouldBypassProxy(req.URL.Host) {
				return nil, nil
			}
			return u, nil
		}
	}
	return transport, nil
}

func shouldBypassProxy(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	lower := strings.ToLower(strings.TrimSpace(host))
	if lower == "localhost" || lower == "127.0.0.1" || lower == "::1" || lower == "0.0.0.0" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

func initOutboundProxy() {
	cfg := loadOutboundProxy()
	if err := applyOutboundProxy(cfg); err != nil {
		_ = applyOutboundProxy(OutboundProxySettings{Enabled: false, URL: cfg.URL})
	}
}
