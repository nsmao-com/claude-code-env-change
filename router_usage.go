package main

// 网关用量统计：从上游响应里解析 token 用量（非流式 JSON 与 SSE 流都支持），
// 写进请求日志，并追加到 gateway-usage.jsonl 供统计页使用。
//
// 统计页只把 Claude Desktop / OpenCode / Grok 的网关用量计入：
// Claude Code、Codex、Antigravity 自己会写本地用量日志，走网关的请求在那里已经算过，
// 再算一次就重复了。自定义路由不知道对应哪个客户端，只在请求日志里显示。

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	gatewayUsageFile = "gateway-usage.jsonl"
	// 非流式响应体最多缓存这么多字节用于解析 usage；超出只影响统计，不影响转发
	maxUsageBodyBytes = 8 << 20
	// 单行 SSE 超过此长度直接跳过（usage 所在的事件都很短）
	maxUsageLineBytes = 1 << 20
	// 用量文件超过此大小时，启动时只保留最近 gatewayUsageKeepDays 天
	gatewayUsagePruneBytes = 8 << 20
	gatewayUsageKeepDays   = 180
)

// gatewayStatsProviders 网关用量计入统计的客户端（没有自己的本地用量日志）
var gatewayStatsProviders = map[string]bool{
	"claude_desktop": true,
	"opencode":       true,
	"grok":           true,
}

// gatewayUsage 一次请求的用量。InputTokens 不含缓存命中部分（与本地日志的口径一致）
type gatewayUsage struct {
	Model            string
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
}

func (u gatewayUsage) empty() bool {
	return u.InputTokens == 0 && u.OutputTokens == 0 && u.CacheReadTokens == 0 && u.CacheWriteTokens == 0
}

// usageAcc 累计各事件里的用量字段。流式事件里的数值是累计值（Anthropic 的
// message_delta、Gemini 的每个分片），取最大值即可，不能相加。
type usageAcc struct {
	model              string
	input              int
	output             int
	cacheRead          int
	cacheWrite         int
	inputIncludesCache bool // OpenAI / Responses / Gemini 的输入数含缓存命中部分
}

func maxInt(a, b int) int {
	if b > a {
		return b
	}
	return a
}

func jsonInt(m map[string]any, key string) int {
	if v, ok := m[key].(float64); ok && v > 0 {
		return int(v)
	}
	return 0
}

func jsonMap(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}

// absorb 从一个 JSON 对象（整段响应或一条 SSE 事件）里取出 usage 与模型名
func (a *usageAcc) absorb(obj map[string]any) {
	if obj == nil {
		return
	}
	// 用量可能挂在顶层（非流式、Chat 流的最后一块）、message 下（Anthropic message_start）、
	// response 下（Responses 的 response.completed），Gemini 叫 usageMetadata
	holders := []map[string]any{obj, jsonMap(obj, "message"), jsonMap(obj, "response")}
	for _, h := range holders {
		if h == nil {
			continue
		}
		for _, key := range []string{"model", "modelVersion"} {
			if s, ok := h[key].(string); ok && strings.TrimSpace(s) != "" && a.model == "" {
				a.model = s
			}
		}
		if u := jsonMap(h, "usage"); u != nil {
			a.absorbUsage(u)
		}
		if u := jsonMap(h, "usageMetadata"); u != nil {
			a.absorbGemini(u)
		}
	}
}

func (a *usageAcc) absorbUsage(u map[string]any) {
	// Anthropic: input_tokens 不含缓存；cache_read / cache_creation 单列
	a.cacheRead = maxInt(a.cacheRead, jsonInt(u, "cache_read_input_tokens"))
	a.cacheWrite = maxInt(a.cacheWrite, jsonInt(u, "cache_creation_input_tokens"))
	// Responses: input_tokens 含缓存，命中数在 input_tokens_details.cached_tokens
	if d := jsonMap(u, "input_tokens_details"); d != nil {
		a.inputIncludesCache = true
		a.cacheRead = maxInt(a.cacheRead, jsonInt(d, "cached_tokens"))
	}
	a.input = maxInt(a.input, jsonInt(u, "input_tokens"))
	a.output = maxInt(a.output, jsonInt(u, "output_tokens"))
	// Chat Completions: prompt_tokens 含缓存，命中数在 prompt_tokens_details.cached_tokens
	if _, ok := u["prompt_tokens"]; ok {
		a.inputIncludesCache = true
		a.input = maxInt(a.input, jsonInt(u, "prompt_tokens"))
	}
	if d := jsonMap(u, "prompt_tokens_details"); d != nil {
		a.cacheRead = maxInt(a.cacheRead, jsonInt(d, "cached_tokens"))
	}
	a.output = maxInt(a.output, jsonInt(u, "completion_tokens"))
}

func (a *usageAcc) absorbGemini(u map[string]any) {
	a.inputIncludesCache = true
	a.input = maxInt(a.input, jsonInt(u, "promptTokenCount"))
	a.cacheRead = maxInt(a.cacheRead, jsonInt(u, "cachedContentTokenCount"))
	// 思考 token 按输出计费
	a.output = maxInt(a.output, jsonInt(u, "candidatesTokenCount")+jsonInt(u, "thoughtsTokenCount"))
}

func (a *usageAcc) result() gatewayUsage {
	input := a.input
	if a.inputIncludesCache {
		input -= a.cacheRead
		if input < 0 {
			input = 0
		}
	}
	return gatewayUsage{
		Model:            a.model,
		InputTokens:      input,
		OutputTokens:     a.output,
		CacheReadTokens:  a.cacheRead,
		CacheWriteTokens: a.cacheWrite,
	}
}

// absorbJSON 解析一段 JSON：对象直接取；数组（Gemini 非 SSE 的流式接口）逐个取
func (a *usageAcc) absorbJSON(data []byte) {
	var v any
	if json.Unmarshal(data, &v) != nil {
		return
	}
	switch t := v.(type) {
	case map[string]any:
		a.absorb(t)
	case []any:
		for _, item := range t {
			if m, ok := item.(map[string]any); ok {
				a.absorb(m)
			}
		}
	}
}

const (
	sniffUndecided = iota
	sniffSSE
	sniffJSON
)

// usageSniffer 包在上游响应体外面：数据原样交给调用方，同时旁路解析 usage。
// 不改变任何转发行为，解析失败只是统计不到。
type usageSniffer struct {
	rc       io.ReadCloser
	mode     int
	pending  []byte // 尚未判定格式时暂存的前导数据
	line     []byte
	skipLine bool
	body     []byte
	overflow bool
	acc      usageAcc
}

func newUsageSniffer(rc io.ReadCloser) *usageSniffer {
	return &usageSniffer{rc: rc}
}

func (s *usageSniffer) Read(p []byte) (int, error) {
	n, err := s.rc.Read(p)
	if n > 0 {
		s.feed(p[:n])
	}
	return n, err
}

func (s *usageSniffer) Close() error {
	return s.rc.Close()
}

func (s *usageSniffer) feed(chunk []byte) {
	if s.mode == sniffUndecided {
		s.pending = append(s.pending, chunk...)
		trimmed := bytes.TrimLeft(s.pending, " \t\r\n\uFEFF")
		if len(trimmed) == 0 {
			return
		}
		if trimmed[0] == '{' || trimmed[0] == '[' {
			s.mode = sniffJSON
		} else {
			s.mode = sniffSSE
		}
		chunk, s.pending = s.pending, nil
	}
	if s.mode == sniffJSON {
		if s.overflow {
			return
		}
		if len(s.body)+len(chunk) > maxUsageBodyBytes {
			s.overflow = true
			s.body = nil
			return
		}
		s.body = append(s.body, chunk...)
		return
	}
	for len(chunk) > 0 {
		i := bytes.IndexByte(chunk, '\n')
		if i < 0 {
			s.appendLine(chunk)
			return
		}
		s.appendLine(chunk[:i])
		s.flushLine()
		chunk = chunk[i+1:]
	}
}

func (s *usageSniffer) appendLine(part []byte) {
	if s.skipLine {
		return
	}
	if len(s.line)+len(part) > maxUsageLineBytes {
		s.skipLine = true
		s.line = s.line[:0]
		return
	}
	s.line = append(s.line, part...)
}

func (s *usageSniffer) flushLine() {
	line := bytes.TrimSpace(s.line)
	s.line = s.line[:0]
	if s.skipLine {
		s.skipLine = false
		return
	}
	if !bytes.HasPrefix(line, []byte("data:")) {
		return
	}
	payload := bytes.TrimSpace(line[len("data:"):])
	// 只解析带用量的事件，逐块的文本增量不必反序列化
	if !bytes.Contains(payload, []byte("sage")) {
		return
	}
	s.acc.absorbJSON(payload)
}

// usage 在响应体读完后调用
func (s *usageSniffer) usage() gatewayUsage {
	switch s.mode {
	case sniffJSON:
		if !s.overflow && len(s.body) > 0 {
			s.acc.absorbJSON(s.body)
			s.body = nil
		}
	case sniffSSE:
		if len(s.line) > 0 {
			s.flushLine()
		}
	}
	return s.acc.result()
}

// gatewayUsageLine gateway-usage.jsonl 的一行
type gatewayUsageLine struct {
	Time             string  `json:"time"`
	Route            string  `json:"route"`
	Provider         string  `json:"provider,omitempty"`
	Model            string  `json:"model"`
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	CacheReadTokens  int     `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int     `json:"cache_write_tokens,omitempty"`
	Cost             float64 `json:"cost"`
}

// routeProvider 应用路由以模型商 id 命名；自定义路由返回空
func routeProvider(routeName string) string {
	name := strings.ToLower(strings.TrimSpace(routeName))
	switch name {
	case "claude", "claude_desktop", "codex", "antigravity", "opencode", "grok":
		return name
	}
	return ""
}

func gatewayUsagePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, gatewayUsageFile), nil
}

func newGatewayUsageLine(route, fallbackModel string, u gatewayUsage, at time.Time) gatewayUsageLine {
	model := u.Model
	if strings.TrimSpace(model) == "" {
		model = fallbackModel
	}
	return gatewayUsageLine{
		Time:             at.Local().Format(recordTimeLayout),
		Route:            route,
		Provider:         routeProvider(route),
		Model:            model,
		InputTokens:      u.InputTokens,
		OutputTokens:     u.OutputTokens,
		CacheReadTokens:  u.CacheReadTokens,
		CacheWriteTokens: u.CacheWriteTokens,
		Cost:             (&LogService{}).calculateCost(model, u.InputTokens, u.OutputTokens, u.CacheWriteTokens, u.CacheReadTokens),
	}
}

// appendGatewayUsage 追加一行用量（best-effort，失败只影响统计）
func appendGatewayUsage(line gatewayUsageLine) {
	path, err := gatewayUsagePath()
	if err != nil {
		return
	}
	data, err := json.Marshal(line)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	_, _ = f.Write(append(data, '\n'))
	_ = f.Close()
}

func readGatewayUsageLines() []gatewayUsageLine {
	path, err := gatewayUsagePath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var lines []gatewayUsageLine
	for _, raw := range bytes.Split(data, []byte("\n")) {
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 {
			continue
		}
		var line gatewayUsageLine
		if json.Unmarshal(raw, &line) == nil && line.Time != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// pruneGatewayUsage 文件过大时只保留最近 gatewayUsageKeepDays 天
func pruneGatewayUsage() {
	path, err := gatewayUsagePath()
	if err != nil {
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() < gatewayUsagePruneBytes {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -gatewayUsageKeepDays).Format(recordTimeLayout)
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	for _, line := range readGatewayUsageLines() {
		if line.Time >= cutoff {
			_ = enc.Encode(line)
		}
	}
	_ = writeFileAtomic(path, b.Bytes(), 0o600)
}

// readGatewayUsage 统计用：只返回计入统计的客户端（见 gatewayStatsProviders），
// provider 非空时只返回该客户端
func readGatewayUsage(days int, provider string) []UsageRecord {
	cutoff := ""
	if days > 0 {
		cutoff = time.Now().AddDate(0, 0, -days).Format(recordTimeLayout)
	}
	var records []UsageRecord
	for _, line := range readGatewayUsageLines() {
		if !gatewayStatsProviders[line.Provider] {
			continue
		}
		if provider != "" && line.Provider != provider {
			continue
		}
		if cutoff != "" && line.Time < cutoff {
			continue
		}
		records = append(records, UsageRecord{
			Timestamp:        line.Time,
			Model:            line.Model,
			InputTokens:      line.InputTokens,
			OutputTokens:     line.OutputTokens,
			CacheReadTokens:  line.CacheReadTokens,
			CacheWriteTokens: line.CacheWriteTokens,
			TotalCost:        line.Cost,
			ProjectPath:      "gateway/" + line.Route,
			Provider:         line.Provider,
		})
	}
	return records
}
