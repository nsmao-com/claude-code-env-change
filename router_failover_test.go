package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGatewayUsageMeasuresJSONAndSSE(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			up := newFakeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
				if stream {
					w.Header().Set("Content-Type", "text/event-stream")
					io.WriteString(w, "data: {\"id\":\"a\",\"model\":\"priced-model\",\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":20,\"prompt_tokens_details\":{\"cached_tokens\":40}}}\n\ndata: [DONE]\n\n")
				} else {
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, `{"model":"priced-model","usage":{"prompt_tokens":100,"completion_tokens":20,"prompt_tokens_details":{"cached_tokens":40}},"choices":[{"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`)
				}
			})
			rs := newTestRouter(t, APIRoute{Name: "usage", SourceFormat: "openai", TargetFormat: "openai", BaseURL: up.URL, Enabled: true})
			body := fmt.Sprintf(`{"model":"client-model","messages":[{"role":"user","content":"hello"}],"stream":%t}`, stream)
			req := httptest.NewRequest("POST", "http://127.0.0.1:8790/usage/v1/chat/completions", strings.NewReader(body))
			req.Host = "127.0.0.1:8790"
			rec := httptest.NewRecorder()
			gatewayGuard(http.HandlerFunc(rs.handleRoot)).ServeHTTP(rec, req)
			entry := lastLog(rs)
			if !entry.UsageReported || entry.InputTokens != 60 || entry.OutputTokens != 20 || entry.CacheReadTokens != 40 || entry.Model != "priced-model" {
				t.Fatalf("usage mismatch: %+v", entry)
			}
			if stream && entry.FirstTokenMs < 1 {
				t.Fatal("missing first token latency")
			}
		})
	}
}
func TestCircuitThresholdAndRecoveryProbe(t *testing.T) {
	r := APIRoute{Name: "circuit-test", FailureThreshold: 2, CooldownSeconds: 60}
	u := RouteUpstream{BaseURL: "http://unit.invalid"}
	releaseUpstream(r, u, false, 5)
	if !claimUpstream(r, u) {
		t.Fatal("threshold reached too early")
	}
	releaseUpstream(r, u, false, 5)
	if claimUpstream(r, u) {
		t.Fatal("cooling upstream allowed")
	}
	policyState.Lock()
	metricLocked(r, u).CoolUntil = time.Now().Add(-time.Second).UnixMilli()
	policyState.Unlock()
	if !claimUpstream(r, u) || claimUpstream(r, u) {
		t.Fatal("half-open must allow exactly one probe")
	}
	releaseUpstream(r, u, true, 7)
	if !claimUpstream(r, u) {
		t.Fatal("recovered upstream blocked")
	}
}
func TestWeightedAndSessionRouting(t *testing.T) {
	r := APIRoute{Name: "weighted-test", BaseURL: "http://one.invalid", Weight: 3, Strategy: "weighted", Fallbacks: []RouteUpstream{{BaseURL: "http://two.invalid", Weight: 1}}}
	counts := map[string]int{}
	for i := 0; i < 40; i++ {
		counts[policyUpstreams(r, nil)[0].BaseURL]++
	}
	if counts[r.BaseURL] != 30 {
		t.Fatalf("weight distribution: %v", counts)
	}
	r.Strategy = "session"
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Session-ID", "session-one")
	first := policyUpstreams(r, req)[0]
	for i := 0; i < 20; i++ {
		if policyUpstreams(r, req)[0] != first {
			t.Fatal("session affinity changed")
		}
	}
}

type fakeUpstream struct {
	*httptest.Server
	hits    atomic.Int32
	lastKey atomic.Value
}

func newFakeUpstream(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *fakeUpstream {
	t.Helper()
	fu := &fakeUpstream{}
	fu.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fu.hits.Add(1)
		fu.lastKey.Store(r.Header.Get("x-api-key") + r.Header.Get("Authorization"))
		handler(w, r)
	}))
	t.Cleanup(fu.Close)
	return fu
}

func anthropicOK(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-x","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
}

func statusHandler(code int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
		w.Write([]byte(`{"error":{"type":"x","message":"upstream says no"}}`))
	}
}

func newTestRouter(t *testing.T, route APIRoute) *RouterService {
	t.Helper()
	withHomeRoot(t)
	rs := NewRouterService()
	rs.config = RouterConfig{Port: defaultRouterPort, Routes: []APIRoute{route}}
	return rs
}

func sendMessages(t *testing.T, rs *RouterService, routeName string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"model":"claude-x","max_tokens":8,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8790/"+routeName+"/v1/messages", strings.NewReader(body))
	req.Host = "127.0.0.1:8790"
	rec := httptest.NewRecorder()
	gatewayGuard(http.HandlerFunc(rs.handleRoot)).ServeHTTP(rec, req)
	return rec
}

func lastLog(rs *RouterService) RouterLogEntry {
	rs.statsMu.Lock()
	defer rs.statsMu.Unlock()
	return rs.logs[len(rs.logs)-1]
}

// 主上游限流时切到备用上游，并用备用上游自己的 Key；日志记下实际上游与被跳过的原因
func TestRouterFailoverOnRateLimit(t *testing.T) {
	primary := newFakeUpstream(t, statusHandler(http.StatusTooManyRequests))
	backup := newFakeUpstream(t, anthropicOK)
	rs := newTestRouter(t, APIRoute{
		Name: "fo-429", SourceFormat: "anthropic", TargetFormat: "anthropic",
		BaseURL: primary.URL, APIKey: "key-primary", Enabled: true,
		Fallbacks: []RouteUpstream{{BaseURL: backup.URL, APIKey: "key-backup"}},
	})

	rec := sendMessages(t, rs, "fo-429")
	if rec.Code != http.StatusOK {
		t.Fatalf("应由备用上游返回 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	if primary.hits.Load() != 1 || backup.hits.Load() != 1 {
		t.Fatalf("主/备命中次数应为 1/1，实际 %d/%d", primary.hits.Load(), backup.hits.Load())
	}
	if got := backup.lastKey.Load().(string); got != "key-backup" {
		t.Fatalf("备用上游应收到自己的 Key，实际 %q", got)
	}
	entry := lastLog(rs)
	if entry.Upstream != strings.TrimPrefix(backup.URL, "http://") {
		t.Errorf("日志应记录实际上游 %s，实际 %q", backup.URL, entry.Upstream)
	}
	if !strings.Contains(entry.Failover, "HTTP 429") {
		t.Errorf("日志应记录跳过原因: %q", entry.Failover)
	}
	if rs.stats["fo-429"].FailoverCount != 1 {
		t.Errorf("故障转移计数应为 1: %+v", rs.stats["fo-429"])
	}

	// 冷却期内主上游排到队尾：下一个请求直接走备用
	rec = sendMessages(t, rs, "fo-429")
	if rec.Code != http.StatusOK || primary.hits.Load() != 1 || backup.hits.Load() != 2 {
		t.Fatalf("冷却期内应直接走备用：code=%d 主=%d 备=%d", rec.Code, primary.hits.Load(), backup.hits.Load())
	}
}

// 请求本身的错误（400）换上游也没用，不能重发
func TestRouterNoFailoverOnBadRequest(t *testing.T) {
	primary := newFakeUpstream(t, statusHandler(http.StatusBadRequest))
	backup := newFakeUpstream(t, anthropicOK)
	rs := newTestRouter(t, APIRoute{
		Name: "fo-400", SourceFormat: "anthropic", TargetFormat: "anthropic",
		BaseURL: primary.URL, APIKey: "k", Enabled: true,
		Fallbacks: []RouteUpstream{{BaseURL: backup.URL, APIKey: "k2"}},
	})
	rec := sendMessages(t, rs, "fo-400")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("400 应原样返回，实际 %d", rec.Code)
	}
	if backup.hits.Load() != 0 {
		t.Fatalf("400 不应切到备用上游")
	}
}

// 主上游连不上（网络错误）时切到备用；Key 失效（401）同理
func TestRouterFailoverOnNetworkErrorAndAuth(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()
	unauthorized := newFakeUpstream(t, statusHandler(http.StatusUnauthorized))
	backup := newFakeUpstream(t, anthropicOK)
	rs := newTestRouter(t, APIRoute{
		Name: "fo-net", SourceFormat: "anthropic", TargetFormat: "anthropic",
		BaseURL: deadURL, APIKey: "k", Enabled: true,
		Fallbacks: []RouteUpstream{{BaseURL: unauthorized.URL, APIKey: "expired"}, {BaseURL: backup.URL, APIKey: "k3"}},
	})
	rec := sendMessages(t, rs, "fo-net")
	if rec.Code != http.StatusOK {
		t.Fatalf("应最终由第二个备用返回 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	if unauthorized.hits.Load() != 1 || backup.hits.Load() != 1 {
		t.Fatalf("命中次数不对：401 上游=%d 备用=%d", unauthorized.hits.Load(), backup.hits.Load())
	}
	if f := lastLog(rs).Failover; !strings.Contains(f, "HTTP 401") {
		t.Errorf("跳过原因应包含 401: %q", f)
	}
}

// 所有上游都失败时，把最后一个上游的错误按入站协议返回
func TestRouterFailoverAllFail(t *testing.T) {
	a := newFakeUpstream(t, statusHandler(http.StatusServiceUnavailable))
	b := newFakeUpstream(t, statusHandler(http.StatusTooManyRequests))
	rs := newTestRouter(t, APIRoute{
		Name: "fo-all", SourceFormat: "anthropic", TargetFormat: "anthropic",
		BaseURL: a.URL, APIKey: "k", Enabled: true,
		Fallbacks: []RouteUpstream{{BaseURL: b.URL, APIKey: "k2"}},
	})
	rec := sendMessages(t, rs, "fo-all")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("应返回最后一个上游的 429，实际 %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "upstream says no") {
		t.Fatalf("应带上上游错误信息: %s", rec.Body.String())
	}
}

// 协议转换 + 流式：Anthropic 入站转 OpenAI 上游，主上游 500 后备用上游的 SSE 正常转换
func TestRouterFailoverWithStreamingConversion(t *testing.T) {
	primary := newFakeUpstream(t, statusHandler(http.StatusInternalServerError))
	backup := newFakeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"c1\",\"model\":\"gpt\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"pong\"}}]}\n\n")
		io.WriteString(w, "data: {\"id\":\"c1\",\"model\":\"gpt\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	})
	rs := newTestRouter(t, APIRoute{
		Name: "fo-sse", SourceFormat: "anthropic", TargetFormat: "openai",
		BaseURL: primary.URL, APIKey: "k", Enabled: true,
		Fallbacks: []RouteUpstream{{BaseURL: backup.URL, APIKey: "k2"}},
	})
	body := `{"model":"claude-x","max_tokens":8,"stream":true,"messages":[{"role":"user","content":"ping"}]}`
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8790/fo-sse/v1/messages", strings.NewReader(body))
	req.Host = "127.0.0.1:8790"
	rec := httptest.NewRecorder()
	gatewayGuard(http.HandlerFunc(rs.handleRoot)).ServeHTTP(rec, req)

	out := rec.Body.String()
	if !strings.Contains(out, "event: message_start") || !strings.Contains(out, "pong") {
		t.Fatalf("备用上游的流应被转换成 Anthropic SSE:\n%s", out)
	}
	if got := backup.lastKey.Load().(string); got != "Bearer k2" {
		t.Fatalf("OpenAI 上游应收到 Bearer 备用 Key，实际 %q", got)
	}
}

func TestSaveRouterConfigValidatesFallbacks(t *testing.T) {
	withHomeRoot(t)
	rs := NewRouterService()
	err := rs.SaveRouterConfig(RouterConfig{Port: 8790, Routes: []APIRoute{{
		Name: "r1", BaseURL: "https://a.example.com", Enabled: true,
		Fallbacks: []RouteUpstream{{BaseURL: "ftp://bad"}},
	}}})
	if err == nil || !strings.Contains(err.Error(), "备用上游") {
		t.Fatalf("非 http(s) 的备用上游应被拒绝，实际 %v", err)
	}

	err = rs.SaveRouterConfig(RouterConfig{Port: 8790, Routes: []APIRoute{{
		Name: "r1", BaseURL: "https://a.example.com", Enabled: true,
		Fallbacks: []RouteUpstream{{BaseURL: "  "}, {BaseURL: " https://b.example.com ", APIKey: " k "}},
	}}})
	if err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	fbs := rs.GetRouterConfig().Routes[0].Fallbacks
	if len(fbs) != 1 || fbs[0].BaseURL != "https://b.example.com" || fbs[0].APIKey != "k" {
		t.Fatalf("空的备用上游应被去掉、其余去空白: %+v", fbs)
	}

	// 重新应用配置生成自动路由时，手动配的备用上游要保留
	if err := rs.upsertAutoRoute(APIRoute{Name: "r1", BaseURL: "https://c.example.com", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if fbs := rs.GetRouterConfig().Routes[0].Fallbacks; len(fbs) != 1 {
		t.Fatalf("自动路由更新后备用上游丢失: %+v", fbs)
	}
}

func TestJoinUpstreamURLVersionSuffix(t *testing.T) {
	cases := []struct{ base, endpoint, want string }{
		{"https://api.anthropic.com", "/v1/messages", "https://api.anthropic.com/v1/messages"},
		{"https://api.openai.com/v1", "/v1/responses", "https://api.openai.com/v1/responses"},
		{"https://api.openai.com/v1/", "/v1/chat/completions", "https://api.openai.com/v1/chat/completions"},
		{"https://open.bigmodel.cn/api/paas/v4", "/v1/chat/completions", "https://open.bigmodel.cn/api/paas/v4/chat/completions"},
		{"https://open.bigmodel.cn/api/anthropic", "/v1/messages", "https://open.bigmodel.cn/api/anthropic/v1/messages"},
		{"https://g.example.com/v1beta", "/v1/models", "https://g.example.com/v1beta/models"},
		{"https://relay.example.com/dev1", "/v1/messages", "https://relay.example.com/dev1/v1/messages"},
	}
	for _, c := range cases {
		if got := joinUpstreamURL(c.base, c.endpoint); got != c.want {
			t.Errorf("joinUpstreamURL(%q, %q) = %q，期望 %q", c.base, c.endpoint, got, c.want)
		}
	}
}

// Codex 的 base_url 习惯带 /v1，经网关转发时不能变成 /v1/v1/responses（透传同理）
func TestRouterDoesNotDoubleV1(t *testing.T) {
	var paths []string
	up := newFakeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"resp_1","object":"response","output":[]}`))
	})
	rs := newTestRouter(t, APIRoute{Name: "codex-v1", SourceFormat: "openai", TargetFormat: "responses", BaseURL: up.URL + "/v1", APIKey: "k", Enabled: true})
	for _, path := range []string{"/codex-v1/v1/responses", "/codex-v1/v1/embeddings"} {
		req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8790"+path, strings.NewReader(`{"model":"m","input":"hi"}`))
		req.Host = "127.0.0.1:8790"
		gatewayGuard(http.HandlerFunc(rs.handleRoot)).ServeHTTP(httptest.NewRecorder(), req)
	}
	if len(paths) != 2 || paths[0] != "/v1/responses" || paths[1] != "/v1/embeddings" {
		t.Fatalf("上游收到的路径应为 /v1/responses、/v1/embeddings，实际 %v", paths)
	}
}

func TestGatewayUsageFinalSSEWithoutNewline(t *testing.T) {
	trace := &gatewayTrace{}
	body := &meteredBody{ReadCloser: io.NopCloser(strings.NewReader(`data: {"model":"m","usage":{"prompt_tokens":12,"completion_tokens":3}}`)), trace: trace, start: time.Now(), stream: true}
	if _, err := io.ReadAll(body); err != nil {
		t.Fatal(err)
	}
	if !trace.reported || trace.input != 12 || trace.output != 3 {
		t.Fatalf("final event lost: %+v", trace)
	}
}
