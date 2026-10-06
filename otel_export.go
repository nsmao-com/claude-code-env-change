package main

// 可观测性导出：把网关请求以 OTLP/HTTP JSON 的 Trace 发到 Langfuse 或任意 OTLP Collector。
// 每次请求一个 span，带模型、上游、Token 用量、状态码与按价格估算的费用（Langfuse cost_details）；
// 可选附带请求 / 响应内容（遮盖密钥，最多 256 KB）。后台批量发送，队列满时丢弃，不影响网关请求。

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// OtelSettings 导出设置
type OtelSettings struct {
	Enabled  bool              `json:"enabled"`
	Mode     string            `json:"mode,omitempty"` // langfuse | otlp
	Endpoint string            `json:"endpoint,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	// Langfuse：主机与公私钥，保存时生成 endpoint 与 Authorization
	LangfuseHost      string `json:"langfuse_host,omitempty"`
	LangfusePublicKey string `json:"langfuse_public_key,omitempty"`
	LangfuseSecretKey string `json:"langfuse_secret_key,omitempty"`
	Bodies            bool   `json:"bodies,omitempty"`
}

type otelSpanJob struct {
	entry   RouterLogEntry
	start   time.Time
	end     time.Time
	capture *RequestCapture
}

var otelState = struct {
	sync.Mutex
	settings OtelSettings
	loadedAt time.Time
	pending  map[string]otelSpanJob
	queue    chan otelSpanJob
	started  bool
	lastErr  string
	sent     int64
}{pending: map[string]otelSpanJob{}}

func otelSettings() OtelSettings {
	otelState.Lock()
	defer otelState.Unlock()
	if time.Since(otelState.loadedAt) > 10*time.Second {
		wb, _ := loadWorkbenchLocked()
		otelState.settings, otelState.loadedAt = wb.Otel, time.Now()
	}
	return otelState.settings
}

func otelBodiesEnabled() bool {
	s := otelSettings()
	return s.Enabled && s.Bodies
}

// otelRecordEntry 请求日志写好后登记，等响应完全结束（含捕获内容）再导出
func otelRecordEntry(e RouterLogEntry, start time.Time) {
	if e.ID == "" || !otelSettings().Enabled {
		return
	}
	otelState.Lock()
	otelState.pending[e.ID] = otelSpanJob{entry: e, start: start, end: time.Now()}
	if len(otelState.pending) > 1000 {
		otelState.pending = map[string]otelSpanJob{}
	}
	otelState.Unlock()
}

// otelComplete 请求处理完毕时调用
func otelComplete(id string, capture *RequestCapture) {
	if id == "" {
		return
	}
	otelState.Lock()
	job, ok := otelState.pending[id]
	delete(otelState.pending, id)
	otelState.Unlock()
	if !ok {
		return
	}
	if capture != nil && otelBodiesEnabled() {
		job.capture = capture
	}
	otelEnqueue(job)
}

func otelEnqueue(job otelSpanJob) {
	otelState.Lock()
	if !otelState.started {
		otelState.started = true
		otelState.queue = make(chan otelSpanJob, 256)
		go otelLoop(otelState.queue)
	}
	q := otelState.queue
	otelState.Unlock()
	select {
	case q <- job:
	default: // 队列满：丢弃，不阻塞网关
	}
}

func otelLoop(q chan otelSpanJob) {
	batch := []otelSpanJob{}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	flush := func() {
		if len(batch) == 0 {
			return
		}
		err := otelSend(otelSettings(), batch)
		otelState.Lock()
		if err != nil {
			otelState.lastErr = err.Error()
		} else {
			otelState.lastErr = ""
			otelState.sent += int64(len(batch))
		}
		otelState.Unlock()
		batch = batch[:0]
	}
	for {
		select {
		case job := <-q:
			batch = append(batch, job)
			if len(batch) >= 32 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func otelAttr(key string, value any) map[string]any {
	switch v := value.(type) {
	case string:
		return map[string]any{"key": key, "value": map[string]any{"stringValue": v}}
	case int:
		return map[string]any{"key": key, "value": map[string]any{"intValue": strconv.Itoa(v)}}
	case int64:
		return map[string]any{"key": key, "value": map[string]any{"intValue": strconv.FormatInt(v, 10)}}
	case float64:
		return map[string]any{"key": key, "value": map[string]any{"doubleValue": v}}
	case bool:
		return map[string]any{"key": key, "value": map[string]any{"boolValue": v}}
	}
	return map[string]any{"key": key, "value": map[string]any{"stringValue": fmt.Sprint(value)}}
}

func otelSpan(job otelSpanJob, wb WorkbenchConfig, envs []EnvConfig) map[string]any {
	e := job.entry
	name := "gateway " + e.Route
	if e.Model != "" {
		name = "chat " + e.Model
	}
	attrs := []map[string]any{
		otelAttr("langfuse.observation.type", "generation"),
		otelAttr("gen_ai.operation.name", "chat"),
		otelAttr("gen_ai.request.model", e.Model),
		otelAttr("gen_ai.response.model", e.Model),
		otelAttr("gen_ai.provider.name", e.Upstream),
		otelAttr("gen_ai.usage.input_tokens", e.InputTokens+e.CacheReadTokens+e.CacheWriteTokens),
		otelAttr("gen_ai.usage.output_tokens", e.OutputTokens),
		otelAttr("gen_ai.usage.cache_read_input_tokens", e.CacheReadTokens),
		otelAttr("gen_ai.usage.cache_creation_input_tokens", e.CacheWriteTokens),
		otelAttr("http.response.status_code", e.StatusCode),
		otelAttr("url.path", e.Path),
		otelAttr("aienv.route", e.Route),
		otelAttr("langfuse.observation.model.name", e.Model),
	}
	if e.FirstTokenMs > 0 {
		attrs = append(attrs, otelAttr("aienv.first_token_ms", e.FirstTokenMs))
	}
	if e.Failover != "" {
		attrs = append(attrs, otelAttr("aienv.failover", e.Failover))
	}
	if e.ServedBy != "" {
		attrs = append(attrs, otelAttr("aienv.served_by", e.ServedBy))
	}
	if e.CallerName != "" {
		attrs = append(attrs, otelAttr("aienv.caller_key", e.CallerName))
	}
	usage, _ := json.Marshal(map[string]int{"input": e.InputTokens + e.CacheReadTokens + e.CacheWriteTokens, "output": e.OutputTokens, "total": e.InputTokens + e.CacheReadTokens + e.CacheWriteTokens + e.OutputTokens})
	attrs = append(attrs, otelAttr("langfuse.observation.usage_details", string(usage)))
	cost, priced := gatewayEntryCost(wb, envs, e)
	if !priced {
		cost, priced = listPriceCost(e)
	}
	if priced && e.UsageReported {
		details, _ := json.Marshal(map[string]float64{"total": cost})
		attrs = append(attrs, otelAttr("langfuse.observation.cost_details", string(details)))
	}
	if c := job.capture; c != nil {
		attrs = append(attrs, otelAttr("langfuse.observation.input", c.RequestBody), otelAttr("langfuse.observation.output", c.ResponseBody))
	}
	status := map[string]any{"code": 1}
	if e.StatusCode >= 400 || e.Error != "" {
		status = map[string]any{"code": 2, "message": clipText(firstNonEmpty(e.Error, fmt.Sprintf("HTTP %d", e.StatusCode)), 300)}
	}
	start := job.start
	if start.IsZero() {
		start = job.end.Add(-time.Duration(e.DurationMs) * time.Millisecond)
	}
	return map[string]any{
		"traceId":           randHex(16),
		"spanId":            randHex(8),
		"name":              name,
		"kind":              3, // CLIENT
		"startTimeUnixNano": strconv.FormatInt(start.UnixNano(), 10),
		"endTimeUnixNano":   strconv.FormatInt(job.end.UnixNano(), 10),
		"attributes":        attrs,
		"status":            status,
	}
}

// otelTarget 由设置得出 traces 地址与请求头
func otelTarget(s OtelSettings) (string, map[string]string, error) {
	headers := map[string]string{}
	endpoint := strings.TrimRight(strings.TrimSpace(s.Endpoint), "/")
	if s.Mode == "langfuse" {
		host := strings.TrimRight(strings.TrimSpace(s.LangfuseHost), "/")
		if host == "" {
			host = "https://cloud.langfuse.com"
		}
		if s.LangfusePublicKey == "" || s.LangfuseSecretKey == "" {
			return "", nil, fmt.Errorf("请填写 Langfuse 的 Public Key 与 Secret Key")
		}
		endpoint = host + "/api/public/otel"
		headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(s.LangfusePublicKey+":"+s.LangfuseSecretKey))
		headers["x-langfuse-ingestion-version"] = "4"
	}
	if err := validEndpoint(endpoint); err != nil {
		return "", nil, fmt.Errorf("导出地址无效：%v", err)
	}
	for k, v := range s.Headers {
		if strings.TrimSpace(k) != "" {
			headers[strings.TrimSpace(k)] = v
		}
	}
	return endpoint + "/v1/traces", headers, nil
}

func otelSend(s OtelSettings, jobs []otelSpanJob) error {
	if !s.Enabled {
		return nil
	}
	target, headers, err := otelTarget(s)
	if err != nil {
		return err
	}
	wb, _ := loadWorkbenchLocked()
	envs := currentEnvsSnapshot()
	spans := make([]map[string]any, 0, len(jobs))
	for _, j := range jobs {
		spans = append(spans, otelSpan(j, wb, envs))
	}
	payload := map[string]any{"resourceSpans": []any{map[string]any{
		"resource": map[string]any{"attributes": []any{otelAttr("service.name", "ai-env-gateway"), otelAttr("service.version", appVersion)}},
		"scopeSpans": []any{map[string]any{
			"scope": map[string]any{"name": "ai-env", "version": appVersion},
			"spans": spans,
		}},
	}}}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		req, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		switch {
		case resp.StatusCode < 300:
			return nil
		case resp.StatusCode == 429 || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("导出地址返回 HTTP %d", resp.StatusCode)
			continue
		default:
			return fmt.Errorf("导出地址返回 HTTP %d（检查地址与认证信息）", resp.StatusCode)
		}
	}
	return lastErr
}

// OtelStatus 导出状态
type OtelStatus struct {
	Settings OtelSettings `json:"settings"`
	Sent     int64        `json:"sent"`
	LastErr  string       `json:"last_error,omitempty"`
}

func (w *WorkbenchService) GetOtelStatus() OtelStatus {
	s := otelSettings()
	otelState.Lock()
	defer otelState.Unlock()
	return OtelStatus{Settings: s, Sent: otelState.sent, LastErr: otelState.lastErr}
}

func (w *WorkbenchService) SaveOtelSettings(s OtelSettings) error {
	s.Endpoint, s.LangfuseHost = strings.TrimSpace(s.Endpoint), strings.TrimSpace(s.LangfuseHost)
	s.LangfusePublicKey, s.LangfuseSecretKey = strings.TrimSpace(s.LangfusePublicKey), strings.TrimSpace(s.LangfuseSecretKey)
	if s.Mode != "langfuse" {
		s.Mode = "otlp"
	}
	if s.Enabled {
		if _, _, err := otelTarget(s); err != nil {
			return err
		}
	}
	workbenchMu.Lock()
	c, err := loadWorkbench()
	if err == nil {
		c.Otel = s
		err = saveWorkbench(c)
	}
	workbenchMu.Unlock()
	if err == nil {
		otelState.Lock()
		otelState.settings, otelState.loadedAt = s, time.Now()
		otelState.Unlock()
	}
	return err
}

// TestOtelExport 用当前（未保存的）设置发送一条测试 span
func (w *WorkbenchService) TestOtelExport(s OtelSettings) error {
	s.Enabled = true
	if s.Mode != "langfuse" {
		s.Mode = "otlp"
	}
	now := time.Now()
	job := otelSpanJob{entry: RouterLogEntry{Route: "test", Path: "/test/v1/chat/completions", Model: "aienv-test", StatusCode: 200, DurationMs: 120,
		InputTokens: 10, OutputTokens: 5, UsageReported: true, Upstream: "example.com"}, start: now.Add(-120 * time.Millisecond), end: now}
	return otelSend(s, []otelSpanJob{job})
}
