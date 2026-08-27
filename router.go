package main

// 本地 API 路由网关：在一个本地端口上提供多条路由，
// 在 Anthropic Messages 与 OpenAI Chat Completions 两种协议之间双向转换（含流式 SSE），
// 让 Claude Code 可以调用 OpenAI 兼容上游、Codex 可以调用 Anthropic 上游，
// 同时支持同协议直连（仅做模型名映射转发）。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	routerStoreFile           = "router.json"
	routerLogFile             = "router-logs.jsonl"
	defaultRouterPort         = 8790
	maxGatewayBodyBytes       = 64 << 20 // 64MB
	defaultAnthropicMaxTokens = 8192
	maxRouterLogsMemory       = 1000
	maxRouterLogsStatus       = 50
)

var routeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// APIRoute 单条路由配置
type APIRoute struct {
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	SourceFormat string            `json:"source_format"` // anthropic | openai（客户端使用的协议）
	TargetFormat string            `json:"target_format"` // anthropic | openai（上游 API 协议）
	BaseURL      string            `json:"base_url"`
	APIKey       string            `json:"api_key,omitempty"`
	ModelMapping map[string]string `json:"model_mapping,omitempty"` // 源模型名 -> 上游模型名，"*" 为兜底
	DefaultModel string            `json:"default_model,omitempty"`
	Enabled      bool              `json:"enabled"`
}

// RouterConfig 网关配置
type RouterConfig struct {
	Port      int        `json:"port"`
	AutoStart bool       `json:"auto_start"`
	Routes    []APIRoute `json:"routes"`
}

// RouteStats 路由运行统计
type RouteStats struct {
	TotalRequests  int64  `json:"total_requests"`
	FailedRequests int64  `json:"failed_requests"`
	LastError      string `json:"last_error,omitempty"`
	LastRequestAt  int64  `json:"last_request_at,omitempty"` // unix 毫秒
}

// RouterLogEntry 请求日志
type RouterLogEntry struct {
	Time       string `json:"time"`
	Route      string `json:"route"`
	Path       string `json:"path"`
	Model      string `json:"model,omitempty"`
	StatusCode int    `json:"status_code"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// RouterLogQuery 完整日志查询
type RouterLogQuery struct {
	Route      string `json:"route"`
	Keyword    string `json:"keyword"`
	OnlyErrors bool   `json:"only_errors"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
}

// RouterLogPage 分页日志
type RouterLogPage struct {
	Items []RouterLogEntry `json:"items"`
	Total int              `json:"total"`
}

// GatewayStatus 网关状态快照
type GatewayStatus struct {
	Running bool                   `json:"running"`
	Port    int                    `json:"port"`
	Stats   map[string]*RouteStats `json:"stats"`
	Logs    []RouterLogEntry       `json:"logs"`
}

// RouterTestResult 路由连通性测试结果
type RouterTestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Latency int64  `json:"latency"`
}

// RouterService 路由网关服务
type RouterService struct {
	mu       sync.Mutex
	config   RouterConfig
	server   *http.Server
	listener net.Listener
	running  bool
	lastErr  string

	statsMu sync.Mutex
	stats   map[string]*RouteStats
	logs    []RouterLogEntry

	client *http.Client
}

// NewRouterService 创建服务并加载本地配置
func NewRouterService() *RouterService {
	rs := &RouterService{
		stats: map[string]*RouteStats{},
		client: &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   15 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:        100,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 15 * time.Second,
			},
			// 不设整体超时：流式响应需要长连接
		},
	}
	rs.loadConfig()
	rs.loadPersistedLogs()
	return rs
}

// OnStartup 应用启动时自动开启网关（若配置了 AutoStart）
func (rs *RouterService) OnStartup(ctx context.Context) {
	rs.mu.Lock()
	autoStart := rs.config.AutoStart
	rs.mu.Unlock()
	if autoStart {
		if err := rs.StartGateway(); err != nil {
			rs.mu.Lock()
			rs.lastErr = err.Error()
			rs.mu.Unlock()
		}
	}
}

// ============ 配置持久化 ============

func (rs *RouterService) configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, routerStoreFile), nil
}

func (rs *RouterService) loadConfig() error {
	path, err := rs.configPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var config RouterConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	rs.normalizeConfig(&config)
	rs.mu.Lock()
	rs.config = config
	rs.mu.Unlock()
	return nil
}

func (rs *RouterService) normalizeConfig(config *RouterConfig) {
	if config.Port <= 0 {
		config.Port = defaultRouterPort
	}
	if config.Routes == nil {
		config.Routes = []APIRoute{}
	}
	for i := range config.Routes {
		config.Routes[i].SourceFormat = normalizeAPIFormat(config.Routes[i].SourceFormat)
		config.Routes[i].TargetFormat = normalizeAPIFormat(config.Routes[i].TargetFormat)
	}
}

func normalizeAPIFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "openai":
		return "openai"
	default:
		return "anthropic"
	}
}

// GetRouterConfig 获取路由配置
func (rs *RouterService) GetRouterConfig() RouterConfig {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.config
}

// SaveRouterConfig 保存路由配置；网关运行中会自动重启生效
func (rs *RouterService) SaveRouterConfig(config RouterConfig) error {
	rs.normalizeConfig(&config)

	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("端口必须在 1-65535 之间")
	}

	seen := map[string]struct{}{}
	for i := range config.Routes {
		route := &config.Routes[i]
		route.Name = strings.TrimSpace(route.Name)
		route.BaseURL = strings.TrimSpace(route.BaseURL)
		route.SourceFormat = normalizeAPIFormat(route.SourceFormat)
		route.TargetFormat = normalizeAPIFormat(route.TargetFormat)

		if !routeNamePattern.MatchString(route.Name) {
			return fmt.Errorf("路由名称 %q 不合法：仅允许字母/数字/连字符/下划线", route.Name)
		}
		key := strings.ToLower(route.Name)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("路由名称重复: %s", route.Name)
		}
		seen[key] = struct{}{}

		if route.BaseURL == "" {
			return fmt.Errorf("路由 %s 必须填写上游 Base URL", route.Name)
		}
		if !strings.HasPrefix(route.BaseURL, "http://") && !strings.HasPrefix(route.BaseURL, "https://") {
			return fmt.Errorf("路由 %s 的 Base URL 必须以 http:// 或 https:// 开头", route.Name)
		}
	}

	path, err := rs.configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}

	rs.mu.Lock()
	wasRunning := rs.running
	rs.config = config
	rs.mu.Unlock()

	notifyCloudSync()

	// 配置变化后重启网关使其生效
	if wasRunning {
		_ = rs.StopGateway()
		if err := rs.StartGateway(); err != nil {
			return fmt.Errorf("配置已保存，但网关重启失败: %v", err)
		}
	}
	return nil
}

// ReloadFromDisk 从磁盘重新加载配置（云同步拉取后调用）
func (rs *RouterService) ReloadFromDisk() error {
	if err := rs.loadConfig(); err != nil {
		return err
	}
	rs.mu.Lock()
	wasRunning := rs.running
	rs.mu.Unlock()
	if wasRunning {
		_ = rs.StopGateway()
		return rs.StartGateway()
	}
	return nil
}

// ============ 网关生命周期 ============

// StartGateway 启动本地网关
func (rs *RouterService) StartGateway() error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.running {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", rs.handleRoot)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", rs.config.Port))
	if err != nil {
		return fmt.Errorf("监听 127.0.0.1:%d 失败: %v", rs.config.Port, err)
	}

	server := &http.Server{Handler: mux}
	rs.server = server
	rs.listener = ln
	rs.running = true
	rs.lastErr = ""

	go func() {
		serveErr := server.Serve(ln)
		rs.mu.Lock()
		rs.running = false
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			rs.lastErr = serveErr.Error()
		}
		rs.mu.Unlock()
	}()

	return nil
}

// StopGateway 停止本地网关
func (rs *RouterService) StopGateway() error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if !rs.running || rs.server == nil {
		rs.running = false
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := rs.server.Shutdown(ctx)
	rs.running = false
	rs.server = nil
	rs.listener = nil
	if err != nil {
		return fmt.Errorf("停止网关失败: %v", err)
	}
	return nil
}

// GetGatewayStatus 获取网关运行状态与统计
func (rs *RouterService) GetGatewayStatus() GatewayStatus {
	rs.mu.Lock()
	running, port := rs.running, rs.config.Port
	rs.mu.Unlock()

	rs.statsMu.Lock()
	defer rs.statsMu.Unlock()

	stats := make(map[string]*RouteStats, len(rs.stats))
	for name, s := range rs.stats {
		copied := *s
		stats[name] = &copied
	}
	n := len(rs.logs)
	start := 0
	if n > maxRouterLogsStatus {
		start = n - maxRouterLogsStatus
	}
	logs := make([]RouterLogEntry, n-start)
	copy(logs, rs.logs[start:])

	return GatewayStatus{
		Running: running,
		Port:    port,
		Stats:   stats,
		Logs:    logs,
	}
}

// GetRouterLogs 查询完整请求日志（最新在前）
func (rs *RouterService) GetRouterLogs(query RouterLogQuery) RouterLogPage {
	rs.statsMu.Lock()
	defer rs.statsMu.Unlock()

	routeFilter := strings.TrimSpace(query.Route)
	keyword := strings.ToLower(strings.TrimSpace(query.Keyword))
	matched := make([]RouterLogEntry, 0, len(rs.logs))
	for i := len(rs.logs) - 1; i >= 0; i-- {
		entry := rs.logs[i]
		if routeFilter != "" && !strings.EqualFold(entry.Route, routeFilter) {
			continue
		}
		if query.OnlyErrors && entry.StatusCode < 400 && entry.Error == "" {
			continue
		}
		if keyword != "" {
			blob := strings.ToLower(entry.Route + " " + entry.Path + " " + entry.Model + " " + entry.Error)
			if !strings.Contains(blob, keyword) {
				continue
			}
		}
		matched = append(matched, entry)
	}

	total := len(matched)
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return RouterLogPage{Items: matched[offset:end], Total: total}
}

// ClearRouterLogs 清空内存与磁盘日志
func (rs *RouterService) ClearRouterLogs() error {
	rs.statsMu.Lock()
	rs.logs = nil
	rs.statsMu.Unlock()
	path, err := rs.logFilePath()
	if err != nil {
		return err
	}
	return os.WriteFile(path, nil, 0o644)
}

// TestRoute 对路由做一次最小化连通性测试（非流式）
func (rs *RouterService) TestRoute(name string) RouterTestResult {
	start := time.Now()

	rs.mu.Lock()
	var route *APIRoute
	for i := range rs.config.Routes {
		if strings.EqualFold(strings.TrimSpace(rs.config.Routes[i].Name), strings.TrimSpace(name)) {
			route = &rs.config.Routes[i]
			break
		}
	}
	rs.mu.Unlock()

	if route == nil {
		return RouterTestResult{Success: false, Message: fmt.Sprintf("路由 %q 不存在", name)}
	}

	model := route.DefaultModel
	if model == "" {
		if mapped, ok := route.ModelMapping["*"]; ok && mapped != "" {
			model = mapped
		}
	}
	if model == "" {
		for _, mapped := range route.ModelMapping {
			if mapped != "" {
				model = mapped
				break
			}
		}
	}

	var req *http.Request
	var err error
	target := normalizeAPIFormat(route.TargetFormat)

	if target == "anthropic" {
		if model == "" {
			model = "claude-3-5-haiku-latest"
		}
		payload := map[string]any{
			"model":      model,
			"max_tokens": 16,
			"messages":   []map[string]string{{"role": "user", "content": "ping"}},
		}
		req, err = rs.newUpstreamRequest(*route, "POST", "/v1/messages", payload)
	} else {
		if model == "" {
			model = "gpt-4o-mini"
		}
		payload := map[string]any{
			"model":      model,
			"max_tokens": 16,
			"messages":   []map[string]string{{"role": "user", "content": "ping"}},
		}
		req, err = rs.newUpstreamRequest(*route, "POST", "/v1/chat/completions", payload)
	}
	if err != nil {
		return RouterTestResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}

	resp, err := rs.client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return RouterTestResult{Success: false, Message: fmt.Sprintf("连接失败: %v", err), Latency: latency}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return RouterTestResult{Success: true, Message: fmt.Sprintf("上游连通 (HTTP %d)", resp.StatusCode), Latency: latency}
	}
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 200 {
		snippet = snippet[:200] + "..."
	}
	return RouterTestResult{Success: false, Message: fmt.Sprintf("上游返回 HTTP %d: %s", resp.StatusCode, snippet), Latency: latency}
}

// ============ HTTP 处理 ============

func (rs *RouterService) handleRoot(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSONError(w, http.StatusNotFound, "路由不存在，路径格式: /{route}/v1/messages")
		return
	}

	route, ok := rs.findRoute(parts[0])
	if !ok {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("路由 %q 不存在", parts[0]))
		return
	}
	if !route.Enabled {
		writeJSONError(w, http.StatusServiceUnavailable, fmt.Sprintf("路由 %s 已停用", route.Name))
		return
	}

	endpoint := strings.TrimPrefix(strings.Join(parts[1:], "/"), "v1/")

	switch endpoint {
	case "messages":
		rs.serveAnthropicEndpoint(w, r, route)
	case "chat/completions":
		rs.serveOpenAIEndpoint(w, r, route)
	case "models":
		rs.serveModels(w, route)
	case "responses":
		rs.serveResponsesEndpoint(w, r, route)
	default:
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("未知接口: /%s", path))
	}
}

func (rs *RouterService) findRoute(name string) (APIRoute, bool) {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for _, route := range rs.config.Routes {
		if strings.EqualFold(strings.TrimSpace(route.Name), trimmed) {
			return route, true
		}
	}
	return APIRoute{}, false
}

// serveAnthropicEndpoint 处理入站 Anthropic 协议（Claude Code 等）
func (rs *RouterService) serveAnthropicEndpoint(w http.ResponseWriter, r *http.Request, route APIRoute) {
	start := time.Now()

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGatewayBodyBytes))
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadRequest, "", fmt.Errorf("读取请求体失败: %v", err), true)
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "读取请求体失败")
		return
	}

	var req anthropicRequest
	if err := json.Unmarshal(body, &req); err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadRequest, "", fmt.Errorf("请求不是有效的 Anthropic 格式: %v", err), true)
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "请求不是有效的 Anthropic Messages 格式")
		return
	}

	inboundModel := req.Model
	mappedModel := route.mapModel(req.Model)
	target := normalizeAPIFormat(route.TargetFormat)

	if target == "anthropic" {
		// 同协议直连：仅替换模型名后透传
		payload := map[string]any{}
		_ = json.Unmarshal(body, &payload)
		payload["model"] = mappedModel
		upstream, err := rs.newUpstreamRequest(route, r.Method, "/v1/messages", payload)
		if err != nil {
			rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
			writeAnthropicError(w, http.StatusBadGateway, "api_error", err.Error())
			return
		}
		resp, err := rs.client.Do(upstream)
		if err != nil {
			rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
			writeAnthropicError(w, http.StatusBadGateway, "api_error", fmt.Sprintf("上游请求失败: %v", err))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			rs.relayUpstreamError(w, resp, route, r, start, inboundModel, true)
			return
		}
		rs.proxyResponse(w, resp, route, r, start, inboundModel)
		return
	}

	// Anthropic → OpenAI 转换
	converted := anthropicRequestToOpenAI(req, mappedModel)
	endpoint := "/v1/chat/completions"
	if req.Stream {
		converted.Stream = true
		converted.StreamOptions = &openaiStreamOptions{IncludeUsage: true}
	}

	upstream, err := rs.newUpstreamRequest(route, http.MethodPost, endpoint, converted)
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
		writeAnthropicError(w, http.StatusBadGateway, "api_error", err.Error())
		return
	}

	resp, err := rs.client.Do(upstream)
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
		writeAnthropicError(w, http.StatusBadGateway, "api_error", fmt.Sprintf("上游请求失败: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rs.relayUpstreamError(w, resp, route, r, start, inboundModel, true)
		return
	}

	if req.Stream {
		if err := convertOpenAIStreamToAnthropic(resp.Body, w, inboundModel); err != nil {
			rs.finishRequest(w, route, r, start, http.StatusInternalServerError, inboundModel, err, true)
			return
		}
		rs.finishRequest(w, route, r, start, http.StatusOK, inboundModel, nil, false)
		return
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxGatewayBodyBytes))
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
		writeAnthropicError(w, http.StatusBadGateway, "api_error", "读取上游响应失败")
		return
	}
	var oResp openaiResponse
	if err := json.Unmarshal(respBody, &oResp); err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, fmt.Errorf("上游响应解析失败: %v", err), true)
		writeAnthropicError(w, http.StatusBadGateway, "api_error", "上游响应不是有效的 OpenAI 格式")
		return
	}

	result := openAIResponseToAnthropic(&oResp, inboundModel)
	rs.finishRequest(w, route, r, start, http.StatusOK, inboundModel, nil, false)
	writeJSON(w, http.StatusOK, result)
}

// serveOpenAIEndpoint 处理入站 OpenAI 协议（Codex 等）
func (rs *RouterService) serveOpenAIEndpoint(w http.ResponseWriter, r *http.Request, route APIRoute) {
	start := time.Now()

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGatewayBodyBytes))
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadRequest, "", fmt.Errorf("读取请求体失败: %v", err), true)
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "读取请求体失败")
		return
	}

	var req openaiRequest
	if err := json.Unmarshal(body, &req); err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadRequest, "", fmt.Errorf("请求不是有效的 OpenAI 格式: %v", err), true)
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "请求不是有效的 OpenAI Chat Completions 格式")
		return
	}

	inboundModel := req.Model
	mappedModel := route.mapModel(req.Model)
	target := normalizeAPIFormat(route.TargetFormat)

	if target == "openai" {
		// 同协议直连：仅替换模型名后透传
		payload := map[string]any{}
		_ = json.Unmarshal(body, &payload)
		payload["model"] = mappedModel
		upstream, err := rs.newUpstreamRequest(route, r.Method, "/v1/chat/completions", payload)
		if err != nil {
			rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
			writeOpenAIError(w, http.StatusBadGateway, "api_error", err.Error())
			return
		}
		resp, err := rs.client.Do(upstream)
		if err != nil {
			rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
			writeOpenAIError(w, http.StatusBadGateway, "api_error", fmt.Sprintf("上游请求失败: %v", err))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			rs.relayUpstreamError(w, resp, route, r, start, inboundModel, false)
			return
		}
		rs.proxyResponse(w, resp, route, r, start, inboundModel)
		return
	}

	// OpenAI → Anthropic 转换
	converted := openAIRequestToAnthropic(req, mappedModel, defaultAnthropicMaxTokens)

	upstream, err := rs.newUpstreamRequest(route, http.MethodPost, "/v1/messages", converted)
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
		writeOpenAIError(w, http.StatusBadGateway, "api_error", err.Error())
		return
	}

	resp, err := rs.client.Do(upstream)
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
		writeOpenAIError(w, http.StatusBadGateway, "api_error", fmt.Sprintf("上游请求失败: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rs.relayUpstreamError(w, resp, route, r, start, inboundModel, false)
		return
	}

	if req.Stream {
		if err := convertAnthropicStreamToOpenAI(resp.Body, w, inboundModel); err != nil {
			rs.finishRequest(w, route, r, start, http.StatusInternalServerError, inboundModel, err, true)
			return
		}
		rs.finishRequest(w, route, r, start, http.StatusOK, inboundModel, nil, false)
		return
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxGatewayBodyBytes))
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
		writeOpenAIError(w, http.StatusBadGateway, "api_error", "读取上游响应失败")
		return
	}
	var aResp anthropicResponse
	if err := json.Unmarshal(respBody, &aResp); err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, fmt.Errorf("上游响应解析失败: %v", err), true)
		writeOpenAIError(w, http.StatusBadGateway, "api_error", "上游响应不是有效的 Anthropic 格式")
		return
	}

	result := anthropicResponseToOpenAI(&aResp)
	rs.finishRequest(w, route, r, start, http.StatusOK, inboundModel, nil, false)
	writeJSON(w, http.StatusOK, result)
}

// proxyResponse 同协议透传：转发状态码与响应体（支持流式）
func (rs *RouterService) proxyResponse(w http.ResponseWriter, resp *http.Response, route APIRoute, r *http.Request, start time.Time, model string) {
	header := w.Header()
	for _, key := range []string{"Content-Type", "Cache-Control"} {
		if v := resp.Header.Get(key); v != "" {
			header.Set(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				rs.finishRequest(w, route, r, start, resp.StatusCode, model, werr, werr != nil)
				return
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if err != nil {
			break
		}
	}
	rs.finishRequest(w, route, r, start, resp.StatusCode, model, nil, resp.StatusCode >= 400)
}

// relayUpstreamError 把上游错误转换成入站协议的错误格式
func (rs *RouterService) relayUpstreamError(w http.ResponseWriter, resp *http.Response, route APIRoute, r *http.Request, start time.Time, model string, anthropicInbound bool) {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	message := strings.TrimSpace(string(body))
	if len(message) > 500 {
		message = message[:500] + "..."
	}
	errType := "upstream_error"

	// 尽量提取结构化的错误信息
	var payload struct {
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.Error != nil {
		if payload.Error.Message != "" {
			message = payload.Error.Message
		}
		if payload.Error.Type != "" {
			errType = payload.Error.Type
		}
	}

	rs.finishRequest(w, route, r, start, resp.StatusCode, model, errors.New(message), true)
	if anthropicInbound {
		writeAnthropicError(w, resp.StatusCode, errType, fmt.Sprintf("上游错误: %s", message))
	} else {
		writeOpenAIError(w, resp.StatusCode, errType, fmt.Sprintf("上游错误: %s", message))
	}
}

// serveModels 模型列表（取映射后的模型名）
func (rs *RouterService) serveModels(w http.ResponseWriter, route APIRoute) {
	seen := map[string]struct{}{}
	data := make([]map[string]any, 0)
	appendModel := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		data = append(data, map[string]any{
			"id":       name,
			"object":   "model",
			"created":  1700000000,
			"owned_by": "router",
		})
	}
	appendModel(route.DefaultModel)
	for _, mapped := range route.ModelMapping {
		appendModel(mapped)
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

// ============ 辅助 ============

// mapModel 模型名映射：精确匹配 → "*" 兜底 → DefaultModel → 原名
func (rt *APIRoute) mapModel(model string) string {
	if model == "" {
		return rt.DefaultModel
	}
	if mapped, ok := rt.ModelMapping[model]; ok && strings.TrimSpace(mapped) != "" {
		return mapped
	}
	if mapped, ok := rt.ModelMapping["*"]; ok && strings.TrimSpace(mapped) != "" {
		return mapped
	}
	if rt.DefaultModel != "" {
		return rt.DefaultModel
	}
	return model
}

func joinUpstreamURL(baseURL, endpoint string) string {
	return strings.TrimRight(baseURL, "/") + endpoint
}

func (rs *RouterService) newUpstreamRequest(route APIRoute, method, endpoint string, payload any) (*http.Request, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, joinUpstreamURL(route.BaseURL, endpoint), strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	target := normalizeAPIFormat(route.TargetFormat)
	if target == "anthropic" {
		if route.APIKey != "" {
			req.Header.Set("x-api-key", route.APIKey)
		}
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		if route.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+route.APIKey)
		}
	}
	return req, nil
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]any{"error": map[string]any{"message": message}})
}

func writeAnthropicError(w http.ResponseWriter, statusCode int, errType, message string) {
	writeJSON(w, statusCode, map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    errType,
			"message": message,
		},
	})
}

func writeOpenAIError(w http.ResponseWriter, statusCode int, errType, message string) {
	writeJSON(w, statusCode, map[string]any{
		"error": map[string]any{
			"type":    errType,
			"message": message,
		},
	})
}

// finishRequest 记录统计与日志。failed 仅表示需要计入失败（4xx/5xx 或网关错误）
func (rs *RouterService) finishRequest(_ http.ResponseWriter, route APIRoute, r *http.Request, start time.Time, statusCode int, model string, reqErr error, failed bool) {
	duration := time.Since(start).Milliseconds()

	entry := RouterLogEntry{
		Time:       time.Now().Format("2006-01-02 15:04:05"),
		Route:      route.Name,
		Path:       r.URL.Path,
		Model:      model,
		StatusCode: statusCode,
		DurationMs: duration,
	}
	if reqErr != nil {
		entry.Error = reqErr.Error()
	}

	rs.statsMu.Lock()
	defer rs.statsMu.Unlock()

	stats, ok := rs.stats[route.Name]
	if !ok {
		stats = &RouteStats{}
		rs.stats[route.Name] = stats
	}
	stats.TotalRequests++
	stats.LastRequestAt = time.Now().UnixMilli()
	if failed {
		stats.FailedRequests++
		if reqErr != nil {
			stats.LastError = reqErr.Error()
		} else {
			stats.LastError = fmt.Sprintf("HTTP %d", statusCode)
		}
	}

	rs.logs = append(rs.logs, entry)
	trimmed := false
	if len(rs.logs) > maxRouterLogsMemory {
		rs.logs = rs.logs[len(rs.logs)-maxRouterLogsMemory:]
		trimmed = true
	}
	rs.persistLogLocked(entry, trimmed)
}

func (rs *RouterService) logFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, routerLogFile), nil
}

func (rs *RouterService) persistLogLocked(entry RouterLogEntry, rewrite bool) {
	path, err := rs.logFilePath()
	if err != nil {
		return
	}
	if rewrite {
		var b strings.Builder
		enc := json.NewEncoder(&b)
		for _, item := range rs.logs {
			_ = enc.Encode(item)
		}
		_ = os.WriteFile(path, []byte(b.String()), 0o644)
		return
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	_, _ = f.Write(append(line, '\n'))
	_ = f.Close()
}

func (rs *RouterService) loadPersistedLogs() {
	path, err := rs.logFilePath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	logs := make([]RouterLogEntry, 0, 64)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry RouterLogEntry
		if json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}
		logs = append(logs, entry)
	}
	if len(logs) > maxRouterLogsMemory {
		logs = logs[len(logs)-maxRouterLogsMemory:]
	}
	rs.logs = logs
}
