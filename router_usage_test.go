package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 逐字节喂给嗅探器：分片边界落在任何位置都不能影响解析
func sniffBytewise(t *testing.T, body string) gatewayUsage {
	t.Helper()
	s := newUsageSniffer(io.NopCloser(strings.NewReader(body)))
	buf := make([]byte, 1)
	var got strings.Builder
	for {
		n, err := s.Read(buf)
		got.Write(buf[:n])
		if err != nil {
			break
		}
	}
	if got.String() != body {
		t.Fatalf("嗅探器改变了响应内容")
	}
	return s.usage()
}

func TestUsageSnifferFormats(t *testing.T) {
	cases := []struct {
		name string
		body string
		want gatewayUsage
	}{
		{
			name: "Anthropic 流式：message_start 给输入与缓存，message_delta 给累计输出",
			body: "event: message_start\n" +
				`data: {"type":"message_start","message":{"id":"m","model":"claude-sonnet-5","usage":{"input_tokens":100,"cache_read_input_tokens":20,"cache_creation_input_tokens":5,"output_tokens":1}}}` + "\n\n" +
				"event: content_block_delta\n" +
				`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}` + "\n\n" +
				"event: message_delta\n" +
				`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":50}}` + "\n\n",
			want: gatewayUsage{Model: "claude-sonnet-5", InputTokens: 100, OutputTokens: 50, CacheReadTokens: 20, CacheWriteTokens: 5},
		},
		{
			name: "Chat 流式：最后一块带 usage，prompt_tokens 含缓存",
			body: `data: {"id":"c","model":"gpt-5","choices":[{"index":0,"delta":{"content":"hi"}}],"usage":null}` + "\n\n" +
				`data: {"id":"c","model":"gpt-5","choices":[],"usage":{"prompt_tokens":120,"completion_tokens":30,"prompt_tokens_details":{"cached_tokens":20}}}` + "\n\n" +
				"data: [DONE]\n\n",
			want: gatewayUsage{Model: "gpt-5", InputTokens: 100, OutputTokens: 30, CacheReadTokens: 20},
		},
		{
			name: "Responses 流式：response.completed 带 usage",
			body: "event: response.output_text.delta\n" +
				`data: {"type":"response.output_text.delta","delta":"hi"}` + "\n\n" +
				"event: response.completed\n" +
				`data: {"type":"response.completed","response":{"id":"r","model":"grok-4.6","usage":{"input_tokens":120,"input_tokens_details":{"cached_tokens":20},"output_tokens":30}}}` + "\n\n",
			want: gatewayUsage{Model: "grok-4.6", InputTokens: 100, OutputTokens: 30, CacheReadTokens: 20},
		},
		{
			name: "Anthropic 非流式",
			body: `{"id":"m","type":"message","model":"claude-x","content":[],"usage":{"input_tokens":7,"output_tokens":3,"cache_read_input_tokens":2}}`,
			want: gatewayUsage{Model: "claude-x", InputTokens: 7, OutputTokens: 3, CacheReadTokens: 2},
		},
		{
			name: "Chat 非流式",
			body: "\n" + `{"id":"c","model":"gpt-5","choices":[],"usage":{"prompt_tokens":9,"completion_tokens":4}}`,
			want: gatewayUsage{Model: "gpt-5", InputTokens: 9, OutputTokens: 4},
		},
		{
			name: "Gemini 分片数组：usageMetadata 是累计值，思考 token 算输出",
			body: `[{"candidates":[],"usageMetadata":{"promptTokenCount":50,"candidatesTokenCount":2},"modelVersion":"gemini-2.5-pro"},` +
				`{"candidates":[],"usageMetadata":{"promptTokenCount":50,"cachedContentTokenCount":10,"candidatesTokenCount":8,"thoughtsTokenCount":4}}]`,
			want: gatewayUsage{Model: "gemini-2.5-pro", InputTokens: 40, OutputTokens: 12, CacheReadTokens: 10},
		},
		{
			name: "没有用量",
			body: `{"object":"list","data":[]}`,
			want: gatewayUsage{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sniffBytewise(t, tc.body); got != tc.want {
				t.Fatalf("用量解析错误:\n got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

// 端到端：经过网关的请求在日志里带上 token；应用路由 opencode 的用量进统计，
// 自定义路由不进统计（不知道对应哪个客户端，计入会与本地日志重复）
func TestGatewayRecordsUsage(t *testing.T) {
	upstream := newFakeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"c","object":"chat.completion","model":"gpt-5","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":30,"completion_tokens":6,"prompt_tokens_details":{"cached_tokens":10}}}`))
	})
	rs := newTestRouter(t, APIRoute{
		Name: "opencode", SourceFormat: "openai", TargetFormat: "openai",
		BaseURL: upstream.URL, APIKey: "k", Enabled: true,
	})
	rs.config.Routes = append(rs.config.Routes, APIRoute{
		Name: "my-relay", SourceFormat: "openai", TargetFormat: "openai",
		BaseURL: upstream.URL, APIKey: "k", Enabled: true,
	})

	send := func(route string) {
		body := `{"model":"gpt-5","messages":[{"role":"user","content":"hi"}]}`
		req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8790/"+route+"/v1/chat/completions", strings.NewReader(body))
		req.Host = "127.0.0.1:8790"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		gatewayGuard(http.HandlerFunc(rs.handleRoot)).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s 应返回 200，实际 %d: %s", route, rec.Code, rec.Body.String())
		}
	}
	send("opencode")
	entry := lastLog(rs)
	if entry.InputTokens != 20 || entry.OutputTokens != 6 || entry.CacheReadTokens != 10 {
		t.Fatalf("请求日志应带用量，实际 %+v", entry)
	}
	send("my-relay")
	if e := lastLog(rs); e.OutputTokens != 6 {
		t.Fatalf("自定义路由的请求日志也应带用量，实际 %+v", e)
	}

	lines := readGatewayUsageLines()
	if len(lines) != 2 {
		t.Fatalf("两次请求都应写入用量文件，实际 %d 行", len(lines))
	}
	if lines[0].Provider != "opencode" || lines[0].Model != "gpt-5" || lines[0].Cost <= 0 {
		t.Fatalf("用量记录不对: %+v", lines[0])
	}

	ls := NewLogService()
	if recs := ls.loadRecordsForPlatform(7, "opencode"); len(recs) != 1 || recs[0].InputTokens != 20 {
		t.Fatalf("OpenCode 统计应只含这一条网关记录，实际 %+v", recs)
	}
	if recs := readGatewayUsage(7, ""); len(recs) != 1 {
		t.Fatalf("自定义路由不应计入统计，实际 %d 条", len(recs))
	}
}

// Chat 同协议直连的流式请求：没带 stream_options 时网关要求上游返回用量
func TestChatPassthroughRequestsStreamUsage(t *testing.T) {
	var gotBody string
	upstream := newFakeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(`data: {"id":"c","model":"gpt-5","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":2}}` + "\n\ndata: [DONE]\n\n"))
	})
	rs := newTestRouter(t, APIRoute{
		Name: "grok", SourceFormat: "openai", TargetFormat: "openai",
		BaseURL: upstream.URL, APIKey: "k", Enabled: true,
	})
	body := `{"model":"gpt-5","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8790/grok/v1/chat/completions", strings.NewReader(body))
	req.Host = "127.0.0.1:8790"
	rec := httptest.NewRecorder()
	gatewayGuard(http.HandlerFunc(rs.handleRoot)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	if !strings.Contains(gotBody, `"include_usage":true`) {
		t.Fatalf("流式请求应附加 stream_options.include_usage，上游收到: %s", gotBody)
	}
	if e := lastLog(rs); e.InputTokens != 5 || e.OutputTokens != 2 {
		t.Fatalf("流式透传的用量应记入日志，实际 %+v", e)
	}
}

// 协议转换的流式路径（Anthropic 入站 → Chat 上游）：转换器读的是被嗅探的响应体，用量同样要记下
func TestGatewayUsageOnConvertedStream(t *testing.T) {
	upstream := newFakeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(`data: {"id":"c","model":"glm-5","choices":[{"index":0,"delta":{"content":"hi"}}]}` + "\n\n" +
			`data: {"id":"c","model":"glm-5","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n" +
			`data: {"id":"c","model":"glm-5","choices":[],"usage":{"prompt_tokens":11,"completion_tokens":3}}` + "\n\n" +
			"data: [DONE]\n\n"))
	})
	rs := newTestRouter(t, APIRoute{
		Name: "claude_desktop", SourceFormat: "anthropic", TargetFormat: "openai",
		BaseURL: upstream.URL, APIKey: "k", Enabled: true,
	})
	body := `{"model":"claude-x","max_tokens":8,"stream":true,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8790/claude_desktop/v1/messages", strings.NewReader(body))
	req.Host = "127.0.0.1:8790"
	rec := httptest.NewRecorder()
	gatewayGuard(http.HandlerFunc(rs.handleRoot)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "message_stop") {
		t.Fatalf("转换后的流应正常结束，实际 %d: %s", rec.Code, rec.Body.String())
	}
	if e := lastLog(rs); e.InputTokens != 11 || e.OutputTokens != 3 {
		t.Fatalf("转换路径的用量应记入日志，实际 %+v", e)
	}
	recs := readGatewayUsage(0, "claude_desktop")
	if len(recs) != 1 || recs[0].Model != "glm-5" {
		t.Fatalf("Claude Desktop 的网关用量应计入统计并按上游模型计价，实际 %+v", recs)
	}
}
