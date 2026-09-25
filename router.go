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
	"net/url"
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
	// Fallbacks 备用上游（与主上游同协议）：主上游网络错误、限流、鉴权/额度失败或 5xx 时依次切换
	Fallbacks        []RouteUpstream `json:"fallbacks,omitempty"`
	FailureThreshold int             `json:"failure_threshold,omitempty"`
	CooldownSeconds  int             `json:"cooldown_seconds,omitempty"`
	Strategy         string          `json:"strategy,omitempty"`
	Weight           int             `json:"weight,omitempty"`
}

// RouterConfig 网关配置
type RouterConfig struct {
	Port       int             `json:"port"`
	AutoStart  bool            `json:"auto_start"`
	Routes     []APIRoute      `json:"routes"`
	AppRouting map[string]bool `json:"app_routing,omitempty"` // 按模型商开关：claude/codex/antigravity/opencode/grok
}

// RouteStats 路由运行统计
type RouteStats struct {
	TotalRequests  int64  `json:"total_requests"`
	FailedRequests int64  `json:"failed_requests"`
	FailoverCount  int64  `json:"failover_count"` // 发生过上游切换的请求数
	LastError      string `json:"last_error,omitempty"`
	LastRequestAt  int64  `json:"last_request_at,omitempty"` // unix 毫秒
}

// RouterLogEntry 请求日志
type RouterLogEntry struct {
	Time             string `json:"time"`
	Route            string `json:"route"`
	Path             string `json:"path"`
	Model            string `json:"model,omitempty"`
	StatusCode       int    `json:"status_code"`
	DurationMs       int64  `json:"duration_ms"`
	Error            string `json:"error,omitempty"`
	Upstream         string `json:"upstream,omitempty"` // 实际响应的上游 host
	Failover         string `json:"failover,omitempty"` // 被跳过的上游及原因
	InputTokens      int    `json:"input_tokens,omitempty"`
	OutputTokens     int    `json:"output_tokens,omitempty"`
	CacheReadTokens  int    `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int    `json:"cache_write_tokens,omitempty"`
	FirstTokenMs     int64  `json:"first_token_ms,omitempty"`
	UsageReported    bool   `json:"usage_reported,omitempty"`
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
		stats:  map[string]*RouteStats{},
		client: &http.Client{
			// Transport 为空时使用 http.DefaultTransport，以便跟随系统出站代理
		},
	}
	globalRouterService = rs
	rs.loadConfig()
	rs.loadPersistedLogs()
	return rs
}

// globalRouterService 供应用配置（ApplyEnv）在应用时自动接管路由
var globalRouterService *RouterService

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
	config.AppRouting = normalizeAppRouting(config.AppRouting)
}

func normalizeAPIFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "openai", "chat_completions":
		return "openai"
	case "responses":
		return "responses"
	default:
		return "anthropic"
	}
}

func defaultAppRouting() map[string]bool {
	return map[string]bool{
		"claude":         false,
		"claude_desktop": false,
		"codex":          false,
		"antigravity":    false,
		"opencode":       false,
		"grok":           false,
	}
}

func knownProvider(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "claude", "claude_desktop", "codex", "antigravity", "opencode", "grok":
		return strings.ToLower(strings.TrimSpace(value)), true
	case "gemini":
		// 旧平台名，归一到 antigravity
		return "antigravity", true
	}
	return "", false
}

func normalizeAppRouting(raw map[string]bool) map[string]bool {
	out := defaultAppRouting()
	for key, enabled := range raw {
		if provider, ok := knownProvider(key); ok {
			out[provider] = enabled
		}
	}
	return out
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
		if err := validateRoutePolicy(*route); err != nil {
			return err
		}
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
		fallbacks := make([]RouteUpstream, 0, len(route.Fallbacks))
		for i, fb := range route.Fallbacks {
			fb.BaseURL = strings.TrimSpace(fb.BaseURL)
			fb.APIKey = strings.TrimSpace(fb.APIKey)
			if fb.BaseURL == "" {
				continue
			}
			if !strings.HasPrefix(fb.BaseURL, "http://") && !strings.HasPrefix(fb.BaseURL, "https://") {
				return fmt.Errorf("路由 %s 的备用上游 %d 必须以 http:// 或 https:// 开头", route.Name, i+1)
			}
			fallbacks = append(fallbacks, fb)
		}
		route.Fallbacks = fallbacks
	}

	path, err := rs.configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFileAtomic(path, data, 0o600); err != nil {
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

	server := &http.Server{
		Handler:           gatewayGuard(mux),
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       120 * time.Second,
		// 故意不设 WriteTimeout：会掐断 SSE 流式响应
	}
	rs.server = server
	rs.listener = ln
	rs.running = true
	rs.lastErr = ""

	go func() {
		serveErr := server.Serve(ln)
		rs.mu.Lock()
		// Stop→Start 快速交替时，迟退出的旧 goroutine 不能把新实例标成 stopped
		if rs.server == server {
			rs.running = false
			rs.server = nil
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			rs.lastErr = serveErr.Error()
		}
		rs.mu.Unlock()
	}()

	return nil
}

// StopGateway 停止本地网关。
// Shutdown 必须在锁外调用：在途请求处理要拿 rs.mu（findRoute 等），
// 持锁等待它们结束会互等直到超时，期间所有新请求与保存配置都被卡住。
func (rs *RouterService) StopGateway() error {
	rs.mu.Lock()
	if !rs.running || rs.server == nil {
		rs.running = false
		rs.mu.Unlock()
		return nil
	}
	server := rs.server
	rs.running = false
	rs.server = nil
	rs.listener = nil
	rs.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := server.Shutdown(ctx)
	if err != nil {
		// 优雅关停超时（常见于挂在 SSE 上的长连接）：强制断开，避免连接滞留
		server.Close()
		return fmt.Errorf("停止网关超时，已强制断开全部连接")
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
			blob := strings.ToLower(entry.Route + " " + entry.Path + " " + entry.Model + " " + entry.Error + " " + entry.Upstream + " " + entry.Failover)
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
	rs.mu.Lock()
	var route *APIRoute
	for i := range rs.config.Routes {
		if strings.EqualFold(strings.TrimSpace(rs.config.Routes[i].Name), strings.TrimSpace(name)) {
			copied := rs.config.Routes[i]
			route = &copied
			break
		}
	}
	rs.mu.Unlock()

	if route == nil {
		return RouterTestResult{Success: false, Message: fmt.Sprintf("路由 %q 不存在", name)}
	}

	upstreams := route.upstreams()
	primary := rs.testUpstream(*route)
	if len(upstreams) == 1 {
		return primary
	}

	// 有备用上游时逐个测：只要有一个不通，就提示用户（故障转移时会用到它）
	result := RouterTestResult{Success: primary.Success, Latency: primary.Latency}
	parts := []string{"主上游：" + primary.Message}
	for i, up := range upstreams[1:] {
		r := rs.testUpstream(route.withUpstream(up))
		if !r.Success {
			result.Success = false
		}
		parts = append(parts, fmt.Sprintf("备用 %d（%s）：%s", i+1, upstreamHost(up.BaseURL), r.Message))
	}
	result.Message = strings.Join(parts, "；")
	return result
}

// testUpstream 向单个上游发一个最小请求，验证地址、Key 与模型是否可用
func (rs *RouterService) testUpstream(route APIRoute) RouterTestResult {
	start := time.Now()

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

	switch target {
	case "anthropic":
		if model == "" {
			// 未配置模型时用当前在售的最便宜型号（Haiku 3.5 已下线，会被上游当成模型不存在）
			model = "claude-haiku-4-5"
		}
		payload := map[string]any{
			"model":      model,
			"max_tokens": 16,
			"messages":   []map[string]string{{"role": "user", "content": "ping"}},
		}
		req, err = rs.newUpstreamRequest(route, "POST", "/v1/messages", payload)
	case "responses":
		if model == "" {
			model = "gpt-4o-mini"
		}
		payload := map[string]any{
			"model":             model,
			"input":             "ping",
			"max_output_tokens": 16,
		}
		req, err = rs.newUpstreamRequest(route, "POST", "/v1/responses", payload)
	default:
		if model == "" {
			model = "gpt-4o-mini"
		}
		payload := map[string]any{
			"model":      model,
			"max_tokens": 16,
			"messages":   []map[string]string{{"role": "user", "content": "ping"}},
		}
		req, err = rs.newUpstreamRequest(route, "POST", "/v1/chat/completions", payload)
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

// gatewayGuard 拦截浏览器发起的请求。网关会给上游注入真实 API Key，
// 只监听 127.0.0.1 并不够：任意网页都能向本机端口发"简单请求"（text/plain 的 POST
// 不触发预检）来盗刷额度，DNS 重绑定还能让网页读到响应。CLI 客户端不带 Origin /
// Sec-Fetch-Site，也总是用回环地址访问，因此下面三条不会误伤正常调用。
func gatewayGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reason := rejectGatewayRequest(r); reason != "" {
			writeJSONError(w, http.StatusForbidden, reason)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func rejectGatewayRequest(r *http.Request) string {
	if !isLoopbackHost(r.Host) {
		return "网关只接受 127.0.0.1 / localhost 访问"
	}
	switch strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))) {
	case "", "none", "same-origin":
	default:
		return "网关拒绝来自网页的跨站请求"
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || !isLoopbackHost(u.Host) {
			return "网关拒绝来自网页的跨站请求"
		}
	}
	return ""
}

// isLoopbackHost 判断 Host（可带端口）是否指向本机回环地址
func isLoopbackHost(hostport string) bool {
	host := strings.TrimSpace(hostport)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (rs *RouterService) handleRoot(w http.ResponseWriter, r *http.Request) {
	r = withGatewayTrace(r)
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
		rest := strings.Join(parts[1:], "/")
		if rest == "" {
			writeJSONError(w, http.StatusNotFound, fmt.Sprintf("未知接口: /%s", path))
			return
		}
		rs.servePassthrough(w, r, route, rest)
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
		resp, err := rs.doWithFailover(r, route, func(rt APIRoute) (*http.Request, error) {
			req, err := rs.newUpstreamRequest(rt, r.Method, "/v1/messages", payload)
			if err == nil {
				copyClientHeaders(req, r)
			}
			return req, err
		})
		if err != nil {
			rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
			writeAnthropicError(w, http.StatusBadGateway, "api_error", err.Error())
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

	resp, err := rs.doWithFailover(r, route, func(rt APIRoute) (*http.Request, error) {
		return rs.newUpstreamRequest(rt, http.MethodPost, endpoint, converted)
	})
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
		writeAnthropicError(w, http.StatusBadGateway, "api_error", err.Error())
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
			writeAnthropicError(w, http.StatusInternalServerError, "api_error", "流式转换失败: "+err.Error())
			return
		}
		rs.finishRequest(w, route, r, start, http.StatusOK, inboundModel, nil, false)
		return
	}

	respBody, err := readUpstreamBody(resp)
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
		resp, err := rs.doWithFailover(r, route, func(rt APIRoute) (*http.Request, error) {
			req, err := rs.newUpstreamRequest(rt, r.Method, "/v1/chat/completions", payload)
			if err == nil {
				copyClientHeaders(req, r)
			}
			return req, err
		})
		if err != nil {
			rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
			writeOpenAIError(w, http.StatusBadGateway, "api_error", err.Error())
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

	resp, err := rs.doWithFailover(r, route, func(rt APIRoute) (*http.Request, error) {
		return rs.newUpstreamRequest(rt, http.MethodPost, "/v1/messages", converted)
	})
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
		writeOpenAIError(w, http.StatusBadGateway, "api_error", err.Error())
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
			writeOpenAIError(w, http.StatusInternalServerError, "api_error", "流式转换失败: "+err.Error())
			return
		}
		rs.finishRequest(w, route, r, start, http.StatusOK, inboundModel, nil, false)
		return
	}

	respBody, err := readUpstreamBody(resp)
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

// versionSuffixPattern 匹配以 API 版本段结尾的 Base URL，如 /v1、/v4、/v1beta
var versionSuffixPattern = regexp.MustCompile(`(?i)/v\d+[a-z]*\d*$`)

// joinUpstreamURL 拼接上游地址。Base URL 常按客户端习惯带着版本段
// （Codex 的 https://api.openai.com/v1、智谱的 .../api/paas/v4），
// 此时不能再追加 endpoint 自带的 /v1，否则会请求到 /v1/v1/responses。
func joinUpstreamURL(baseURL, endpoint string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if versionSuffixPattern.MatchString(base) && strings.HasPrefix(endpoint, "/v1/") {
		endpoint = endpoint[len("/v1"):]
	}
	return base + endpoint
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
	applyRouteAuth(req, route)
	return req, nil
}

// copyClientHeaders 把入站请求的客户端头透传给上游（跳过逐跳头、长度与鉴权）。
// 同协议直连时 Claude Code 依赖的 anthropic-beta / anthropic-version 等头
// 不再被丢弃；鉴权三件套除外——路由配置的 key 必须优先。
func copyClientHeaders(dst *http.Request, src *http.Request) {
	if dst == nil || src == nil {
		return
	}
	for key, values := range src.Header {
		canonical := http.CanonicalHeaderKey(key)
		if hopByHopHeaders[canonical] {
			continue
		}
		switch canonical {
		case "Host", "Content-Length",
			"Authorization", "X-Api-Key", "X-Goog-Api-Key",
			"Content-Type", "Accept", "Accept-Encoding":
			continue
		}
		dst.Header[key] = append([]string(nil), values...)
	}
}

// readUpstreamBody 读上游响应体，超过 maxGatewayBodyBytes 时报错而不是静默截断
// （截断的 JSON 只会换来一句"响应不是有效格式"，用户查不出原因）
func readUpstreamBody(resp *http.Response) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxGatewayBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxGatewayBodyBytes {
		return nil, fmt.Errorf("上游响应超过 %dMB 上限", maxGatewayBodyBytes>>20)
	}
	return data, nil
}

func applyRouteAuth(req *http.Request, route APIRoute) {
	if route.APIKey == "" {
		return
	}
	target := normalizeAPIFormat(route.TargetFormat)
	if target == "anthropic" {
		req.Header.Set("x-api-key", route.APIKey)
		if req.Header.Get("anthropic-version") == "" {
			req.Header.Set("anthropic-version", "2023-06-01")
		}
		return
	}
	if req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+route.APIKey)
	}
	if req.Header.Get("x-goog-api-key") == "" {
		req.Header.Set("x-goog-api-key", route.APIKey)
	}
}

var hopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailers":            true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

func (rs *RouterService) servePassthrough(w http.ResponseWriter, r *http.Request, route APIRoute, rest string) {
	start := time.Now()

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGatewayBodyBytes))
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadRequest, "", fmt.Errorf("读取请求体失败: %v", err), true)
		writeJSONError(w, http.StatusBadRequest, "读取请求体失败")
		return
	}

	resp, err := rs.doWithFailover(r, route, func(rt APIRoute) (*http.Request, error) {
		targetURL := joinUpstreamURL(rt.BaseURL, "/"+strings.TrimLeft(rest, "/"))
		if r.URL.RawQuery != "" {
			targetURL += "?" + r.URL.RawQuery
		}
		upstream, err := http.NewRequest(r.Method, targetURL, strings.NewReader(string(body)))
		if err != nil {
			return nil, err
		}
		for key, values := range r.Header {
			if hopByHopHeaders[http.CanonicalHeaderKey(key)] {
				continue
			}
			if strings.EqualFold(key, "Host") || strings.EqualFold(key, "Content-Length") {
				continue
			}
			upstream.Header[key] = append([]string(nil), values...)
		}
		// 客户端带来的占位 Authorization 会让路由配置的 key 静默失效，摘掉后由路由注入
		upstream.Header.Del("Authorization")
		upstream.Header.Del("X-Api-Key")
		upstream.Header.Del("X-Goog-Api-Key")
		applyRouteAuth(upstream, rt)
		return upstream, nil
	})
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadGateway, "", err, true)
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()
	rs.proxyResponse(w, resp, route, r, start, "")
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
	if trace := gatewayTraceFrom(r); trace != nil {
		entry.Upstream = trace.upstream
		entry.Failover = strings.Join(trace.skipped, "；")
		entry.InputTokens = trace.input
		entry.OutputTokens = trace.output
		entry.CacheReadTokens = trace.cacheRead
		entry.CacheWriteTokens = trace.cacheWrite
		entry.FirstTokenMs = trace.firstToken
		entry.UsageReported = trace.reported
		if trace.model != "" {
			entry.Model = trace.model
		}
	}

	rs.statsMu.Lock()

	stats, ok := rs.stats[route.Name]
	if !ok {
		stats = &RouteStats{}
		rs.stats[route.Name] = stats
	}
	stats.TotalRequests++
	stats.LastRequestAt = time.Now().UnixMilli()
	if entry.Failover != "" {
		stats.FailoverCount++
	}
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
	var snapshot []RouterLogEntry
	if len(rs.logs) > maxRouterLogsMemory {
		rs.logs = rs.logs[len(rs.logs)-maxRouterLogsMemory:]
		trimmed = true
		snapshot = make([]RouterLogEntry, len(rs.logs))
		copy(snapshot, rs.logs)
	}
	rs.statsMu.Unlock()

	// 文件 IO 在锁外执行：高并发下持 statsMu 做同步写会放大每请求延迟
	rs.persistLog(entry, snapshot, trimmed)
	persistGatewayUsage(entry)
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

// persistLog 持久化请求日志（在 statsMu 之外调用）。rewrite 时用调用方传入的
// 内存快照整体重写；快照与磁盘之间极小窗口内的并发日志允许丢失，日志是 best-effort。
func (rs *RouterService) persistLog(entry RouterLogEntry, snapshot []RouterLogEntry, rewrite bool) {
	path, err := rs.logFilePath()
	if err != nil {
		return
	}
	if rewrite {
		var b strings.Builder
		enc := json.NewEncoder(&b)
		for _, item := range snapshot {
			_ = enc.Encode(item)
		}
		_ = writeFileAtomic(path, []byte(b.String()), 0o600)
		return
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
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
