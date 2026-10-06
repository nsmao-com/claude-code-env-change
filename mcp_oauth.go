package main

// MCP 远程服务器 OAuth 登录：对需要 OAuth 的 HTTP MCP 服务器（Linear、Notion 等），
// 由 AI ENV 完成一次授权（RFC 9728 资源元数据 → 授权服务器元数据 → 动态注册客户端 → PKCE），
// 令牌保存在本机；同步到各工具时把地址换成本机网关 /_mcp/<名称>，网关转发时带上并按需刷新令牌，
// 这样不支持 MCP OAuth 的工具也能使用这些服务器。

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const mcpRelayPrefix = "_mcp"

// MCPOAuthToken 一个服务器的登录信息（保存在 mcp-oauth.json，权限 0600）
type MCPOAuthToken struct {
	ServerURL     string `json:"server_url"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret,omitempty"`
	TokenEndpoint string `json:"token_endpoint"`
	Resource      string `json:"resource,omitempty"`
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token,omitempty"`
	ExpiresAt     int64  `json:"expires_at,omitempty"` // unix 秒
	Scope         string `json:"scope,omitempty"`
}

// MCPOAuthStatus 界面展示用
type MCPOAuthStatus struct {
	Server    string `json:"server"`
	SignedIn  bool   `json:"signed_in"`
	Pending   bool   `json:"pending"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
	Error     string `json:"error,omitempty"`
}

var mcpOAuthStore = struct {
	sync.Mutex
	loaded  bool
	tokens  map[string]MCPOAuthToken
	pending map[string]string // server -> 进行中的提示 / 错误
}{tokens: map[string]MCPOAuthToken{}, pending: map[string]string{}}

func loadMCPOAuthLocked() {
	if mcpOAuthStore.loaded {
		return
	}
	mcpOAuthStore.loaded = true
	if p, err := storePath("mcp-oauth.json"); err == nil {
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &mcpOAuthStore.tokens)
		}
	}
	if mcpOAuthStore.tokens == nil {
		mcpOAuthStore.tokens = map[string]MCPOAuthToken{}
	}
}

func saveMCPOAuthLocked() error {
	p, err := storePath("mcp-oauth.json")
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(mcpOAuthStore.tokens, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(p, b, 0o600)
}

func mcpOAuthToken(server string) (MCPOAuthToken, bool) {
	mcpOAuthStore.Lock()
	defer mcpOAuthStore.Unlock()
	loadMCPOAuthLocked()
	t, ok := mcpOAuthStore.tokens[strings.ToLower(server)]
	return t, ok
}

// ===== 发现与注册 =====

type oauthServerMeta struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RegistrationEndpoint  string   `json:"registration_endpoint"`
	ScopesSupported       []string `json:"scopes_supported"`
}

var resourceMetaRE = regexp.MustCompile(`resource_metadata="([^"]+)"`)

func oauthGetJSON(ctx context.Context, u string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.Unmarshal(b, dst)
}

// discoverOAuth 找到 MCP 服务器的授权服务器与可用 scope
func discoverOAuth(ctx context.Context, serverURL string) (oauthServerMeta, []string, error) {
	u, err := url.Parse(serverURL)
	if err != nil || u.Host == "" {
		return oauthServerMeta{}, nil, fmt.Errorf("MCP 地址无效")
	}
	origin := u.Scheme + "://" + u.Host
	var resource struct {
		AuthorizationServers []string `json:"authorization_servers"`
		ScopesSupported      []string `json:"scopes_supported"`
	}
	// 先看 401 响应里的 resource_metadata，再试 well-known 路径
	candidates := []string{}
	probe, _ := http.NewRequestWithContext(ctx, http.MethodPost, serverURL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"AI ENV","version":"`+appVersion+`"}}}`))
	if probe != nil {
		probe.Header.Set("Content-Type", "application/json")
		probe.Header.Set("Accept", "application/json, text/event-stream")
		if resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(probe); err == nil {
			if m := resourceMetaRE.FindStringSubmatch(resp.Header.Get("WWW-Authenticate")); m != nil {
				candidates = append(candidates, m[1])
			}
			resp.Body.Close()
			if resp.StatusCode < 300 {
				return oauthServerMeta{}, nil, fmt.Errorf("该服务器不需要登录即可使用")
			}
		}
	}
	path := strings.TrimRight(u.Path, "/")
	candidates = append(candidates, origin+"/.well-known/oauth-protected-resource"+path, origin+"/.well-known/oauth-protected-resource")
	authServer := ""
	for _, c := range candidates {
		if oauthGetJSON(ctx, c, &resource) == nil && len(resource.AuthorizationServers) > 0 {
			authServer = strings.TrimRight(resource.AuthorizationServers[0], "/")
			break
		}
	}
	if authServer == "" {
		authServer = origin // 旧规范：授权服务器与 MCP 服务器同域
	}
	as, err := url.Parse(authServer)
	if err != nil || as.Host == "" {
		return oauthServerMeta{}, nil, fmt.Errorf("授权服务器地址无效")
	}
	asOrigin := as.Scheme + "://" + as.Host
	asPath := strings.TrimRight(as.Path, "/")
	var meta oauthServerMeta
	for _, c := range []string{
		asOrigin + "/.well-known/oauth-authorization-server" + asPath,
		asOrigin + "/.well-known/openid-configuration" + asPath,
		authServer + "/.well-known/openid-configuration",
		asOrigin + "/.well-known/oauth-authorization-server",
	} {
		if oauthGetJSON(ctx, c, &meta) == nil && meta.AuthorizationEndpoint != "" && meta.TokenEndpoint != "" {
			return meta, resource.ScopesSupported, nil
		}
	}
	return oauthServerMeta{}, nil, fmt.Errorf("找不到该服务器的 OAuth 配置（不支持 MCP 授权发现）")
}

func registerOAuthClient(ctx context.Context, endpoint, redirect string) (string, string, error) {
	body, _ := json.Marshal(map[string]any{
		"client_name": "AI ENV", "redirect_uris": []string{redirect},
		"grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"},
		"token_endpoint_auth_method": "none",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return "", "", fmt.Errorf("注册 OAuth 客户端失败: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if resp.StatusCode >= 300 || json.Unmarshal(b, &out) != nil || out.ClientID == "" {
		return "", "", fmt.Errorf("注册 OAuth 客户端失败（HTTP %d）", resp.StatusCode)
	}
	return out.ClientID, out.ClientSecret, nil
}

func randomURLSafe(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func exchangeOAuthToken(ctx context.Context, endpoint string, form url.Values) (MCPOAuthToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return MCPOAuthToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return MCPOAuthToken{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var t struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
		Description  string `json:"error_description"`
	}
	if json.Unmarshal(b, &t) != nil || t.AccessToken == "" {
		msg := firstNonEmpty(t.Description, t.Error, fmt.Sprintf("HTTP %d", resp.StatusCode))
		return MCPOAuthToken{}, fmt.Errorf("换取令牌失败: %s", msg)
	}
	out := MCPOAuthToken{AccessToken: t.AccessToken, RefreshToken: t.RefreshToken, Scope: t.Scope}
	if t.ExpiresIn > 0 {
		out.ExpiresAt = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second).Unix()
	}
	return out, nil
}

// ===== 登录流程 =====

// StartMCPOAuth 发起登录：打开浏览器授权，回调由本机临时端口接收。立即返回，结果通过事件 mcp:oauth 通知
func (ms *MCPService) StartMCPOAuth(name string) error {
	servers, err := ms.ListServers()
	if err != nil {
		return err
	}
	var server *MCPServer
	for i := range servers {
		if strings.EqualFold(servers[i].Name, name) {
			server = &servers[i]
		}
	}
	if server == nil || server.URL == "" || normalizeServerType(server.Type) == "stdio" {
		return fmt.Errorf("只有 HTTP 类型的远程 MCP 服务器可以 OAuth 登录")
	}
	if strings.EqualFold(server.Type, "sse") {
		return fmt.Errorf("SSE 类型的服务器暂不支持经网关转发，请改用 Streamable HTTP 地址")
	}
	key := strings.ToLower(server.Name)
	mcpOAuthStore.Lock()
	if mcpOAuthStore.pending[key] == "waiting" {
		mcpOAuthStore.Unlock()
		return fmt.Errorf("正在等待浏览器授权")
	}
	mcpOAuthStore.pending[key] = "waiting"
	mcpOAuthStore.Unlock()

	fail := func(err error) error {
		mcpOAuthStore.Lock()
		mcpOAuthStore.pending[key] = err.Error()
		mcpOAuthStore.Unlock()
		ms.emitOAuth(server.Name)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	meta, scopes, err := discoverOAuth(ctx, server.URL)
	cancel()
	if err != nil {
		return fail(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fail(err)
	}
	redirect := fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port)
	clientID, clientSecret := "", ""
	if meta.RegistrationEndpoint != "" {
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		clientID, clientSecret, err = registerOAuthClient(c, meta.RegistrationEndpoint, redirect)
		cancel()
		if err != nil {
			ln.Close()
			return fail(err)
		}
	} else {
		ln.Close()
		return fail(fmt.Errorf("该授权服务器不支持动态注册客户端，暂无法自动登录"))
	}
	verifier := randomURLSafe(48)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	state := randomURLSafe(18)
	q := url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirect},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {state}, "resource": {server.URL},
	}
	if len(scopes) > 0 {
		q.Set("scope", strings.Join(scopes, " "))
	}
	authURL := meta.AuthorizationEndpoint
	if strings.Contains(authURL, "?") {
		authURL += "&" + q.Encode()
	} else {
		authURL += "?" + q.Encode()
	}

	go func() {
		defer ln.Close()
		done := make(chan error, 1)
		srv := &http.Server{ReadHeaderTimeout: 10 * time.Second}
		srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/callback" {
				http.NotFound(w, r)
				return
			}
			qs := r.URL.Query()
			finish := func(ok bool, msg string) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				title := "授权成功，可以关闭此页面回到 AI ENV"
				if !ok {
					title = "授权失败：" + msg
				}
				fmt.Fprintf(w, "<!doctype html><meta charset=utf-8><title>AI ENV</title><body style=\"font-family:sans-serif;padding:40px\"><h2>%s</h2></body>", htmlEscape(title))
			}
			if qs.Get("state") != state {
				finish(false, "state 不匹配")
				done <- fmt.Errorf("授权回调的 state 不匹配，已拒绝")
				return
			}
			if e := qs.Get("error"); e != "" {
				finish(false, firstNonEmpty(qs.Get("error_description"), e))
				done <- fmt.Errorf("授权被拒绝: %s", firstNonEmpty(qs.Get("error_description"), e))
				return
			}
			form := url.Values{"grant_type": {"authorization_code"}, "code": {qs.Get("code")}, "redirect_uri": {redirect},
				"client_id": {clientID}, "code_verifier": {verifier}, "resource": {server.URL}}
			if clientSecret != "" {
				form.Set("client_secret", clientSecret)
			}
			c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			tok, err := exchangeOAuthToken(c, meta.TokenEndpoint, form)
			cancel()
			if err != nil {
				finish(false, err.Error())
				done <- err
				return
			}
			tok.ServerURL, tok.ClientID, tok.ClientSecret, tok.TokenEndpoint, tok.Resource = server.URL, clientID, clientSecret, meta.TokenEndpoint, server.URL
			mcpOAuthStore.Lock()
			loadMCPOAuthLocked()
			mcpOAuthStore.tokens[key] = tok
			err = saveMCPOAuthLocked()
			mcpOAuthStore.Unlock()
			finish(err == nil, fmt.Sprint(err))
			done <- err
		})
		go func() { _ = srv.Serve(ln) }()
		var result error
		select {
		case result = <-done:
		case <-time.After(5 * time.Minute):
			result = fmt.Errorf("等待浏览器授权超时")
		}
		time.Sleep(300 * time.Millisecond) // 让回调页面写完
		_ = srv.Close()
		mcpOAuthStore.Lock()
		if result != nil {
			mcpOAuthStore.pending[key] = result.Error()
		} else {
			delete(mcpOAuthStore.pending, key)
		}
		mcpOAuthStore.Unlock()
		if result == nil {
			// 登录成功：把该服务器同步为经网关转发的地址
			_, _ = ms.SyncToPlatforms()
			if rs := globalRouterService; rs != nil {
				_ = rs.StartGateway()
			}
		}
		ms.emitOAuth(server.Name)
	}()
	openOAuthURL(authURL)
	return nil
}

// openOAuthURL 在系统浏览器中打开授权页
var openOAuthURL = func(u string) {
	if globalApp != nil && globalApp.ctx != nil {
		runtime.BrowserOpenURL(globalApp.ctx, u)
	}
}

func (ms *MCPService) emitOAuth(name string) {
	if globalApp != nil && globalApp.ctx != nil {
		emitAppEvent(globalApp.ctx, "mcp:oauth", name)
	}
}

func htmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// GetMCPOAuthStatuses 各服务器的登录状态
func (ms *MCPService) GetMCPOAuthStatuses() []MCPOAuthStatus {
	mcpOAuthStore.Lock()
	defer mcpOAuthStore.Unlock()
	loadMCPOAuthLocked()
	out := []MCPOAuthStatus{}
	seen := map[string]bool{}
	for name, t := range mcpOAuthStore.tokens {
		st := MCPOAuthStatus{Server: name, SignedIn: true, ExpiresAt: t.ExpiresAt}
		if p := mcpOAuthStore.pending[name]; p == "waiting" {
			st.Pending = true
		}
		out = append(out, st)
		seen[name] = true
	}
	for name, p := range mcpOAuthStore.pending {
		if seen[name] {
			continue
		}
		st := MCPOAuthStatus{Server: name, Pending: p == "waiting"}
		if p != "waiting" {
			st.Error = p
		}
		out = append(out, st)
	}
	return out
}

// SignOutMCPOAuth 删除登录信息并把服务器同步回原地址
func (ms *MCPService) SignOutMCPOAuth(name string) error {
	key := strings.ToLower(name)
	mcpOAuthStore.Lock()
	loadMCPOAuthLocked()
	delete(mcpOAuthStore.tokens, key)
	delete(mcpOAuthStore.pending, key)
	err := saveMCPOAuthLocked()
	mcpOAuthStore.Unlock()
	if err != nil {
		return err
	}
	_, err = ms.SyncToPlatforms()
	return err
}

// freshMCPToken 返回可用的访问令牌，将要过期时用 refresh_token 刷新
func freshMCPToken(name string, force bool) (MCPOAuthToken, error) {
	key := strings.ToLower(name)
	mcpOAuthStore.Lock()
	defer mcpOAuthStore.Unlock()
	loadMCPOAuthLocked()
	t, ok := mcpOAuthStore.tokens[key]
	if !ok {
		return MCPOAuthToken{}, fmt.Errorf("MCP 服务器 %s 没有登录", name)
	}
	if !force && (t.ExpiresAt == 0 || time.Until(time.Unix(t.ExpiresAt, 0)) > time.Minute) {
		return t, nil
	}
	if t.RefreshToken == "" {
		return t, fmt.Errorf("MCP 服务器 %s 的登录已过期，请在 AI ENV 中重新登录", name)
	}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {t.RefreshToken}, "client_id": {t.ClientID}}
	if t.Resource != "" {
		form.Set("resource", t.Resource)
	}
	if t.ClientSecret != "" {
		form.Set("client_secret", t.ClientSecret)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fresh, err := exchangeOAuthToken(ctx, t.TokenEndpoint, form)
	if err != nil {
		return t, fmt.Errorf("刷新 %s 的登录失败，请重新登录: %v", name, err)
	}
	t.AccessToken, t.ExpiresAt = fresh.AccessToken, fresh.ExpiresAt
	if fresh.RefreshToken != "" {
		t.RefreshToken = fresh.RefreshToken
	}
	mcpOAuthStore.tokens[key] = t
	_ = saveMCPOAuthLocked()
	return t, nil
}

// withMCPOAuthHeader 已 OAuth 登录的远程服务器在连接测试时带上令牌
func withMCPOAuthHeader(s MCPServer) MCPServer {
	if s.URL == "" {
		return s
	}
	// 刷新失败时仍用旧令牌测试，服务器返回 401 即如实显示失败
	tok, _ := freshMCPToken(s.Name, false)
	if tok.AccessToken == "" {
		return s
	}
	headers := map[string]string{}
	for k, v := range s.Headers {
		if !strings.EqualFold(k, "Authorization") {
			headers[k] = v
		}
	}
	headers["Authorization"] = "Bearer " + tok.AccessToken
	s.Headers = headers
	return s
}

// withMCPRelays 同步到各工具前：已登录的服务器改用本机网关地址，Authorization 由网关添加
func withMCPRelays(servers []MCPServer) []MCPServer {
	mcpOAuthStore.Lock()
	loadMCPOAuthLocked()
	tokens := mcpOAuthStore.tokens
	mcpOAuthStore.Unlock()
	if len(tokens) == 0 || globalRouterService == nil {
		return servers
	}
	port := routerPort(globalRouterService)
	out := make([]MCPServer, len(servers))
	for i, s := range servers {
		out[i] = s
		if _, ok := tokens[strings.ToLower(s.Name)]; ok && s.URL != "" {
			out[i].URL = fmt.Sprintf("http://127.0.0.1:%d/%s/%s", port, mcpRelayPrefix, url.PathEscape(s.Name))
			out[i].Type = "http"
			headers := map[string]string{}
			for k, v := range s.Headers {
				if !strings.EqualFold(k, "Authorization") {
					headers[k] = v
				}
			}
			out[i].Headers = headers
		}
	}
	return out
}

// serveMCPRelay 网关转发 /_mcp/<名称>：带上令牌请求远程 MCP 服务器，原样回传（含流式响应）
func (rs *RouterService) serveMCPRelay(w http.ResponseWriter, r *http.Request, name string) {
	tok, err := freshMCPToken(name, false)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, err.Error())
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGatewayBodyBytes))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "读取请求体失败")
		return
	}
	send := func(t MCPOAuthToken) (*http.Response, error) {
		req, err := http.NewRequestWithContext(r.Context(), r.Method, t.ServerURL, strings.NewReader(string(body)))
		if err != nil {
			return nil, err
		}
		for _, h := range []string{"Content-Type", "Accept", "Mcp-Session-Id", "Mcp-Protocol-Version", "Last-Event-Id"} {
			if v := r.Header.Get(h); v != "" {
				req.Header.Set(h, v)
			}
		}
		req.Header.Set("Authorization", "Bearer "+t.AccessToken)
		return rs.client.Do(req)
	}
	resp, err := send(tok)
	if err == nil && resp.StatusCode == http.StatusUnauthorized && tok.RefreshToken != "" {
		resp.Body.Close()
		if fresh, ferr := freshMCPToken(name, true); ferr == nil {
			resp, err = send(fresh)
		}
	}
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "转发 MCP 请求失败: "+err.Error())
		return
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		switch strings.ToLower(k) {
		case "content-length", "transfer-encoding", "connection", "www-authenticate":
			continue
		}
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 32<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if rerr != nil {
			return
		}
	}
}
