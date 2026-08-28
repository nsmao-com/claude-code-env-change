package main

// 路由协议转换的单元测试：覆盖消息转换、响应转换与流式转换的核心路径。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mustUnmarshal(t *testing.T, data string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(data), target); err != nil {
		t.Fatalf("解析 JSON 失败: %v\n%s", err, data)
	}
}

func TestAnthropicToOpenAIMessages(t *testing.T) {
	raw := `{
		"model": "claude-sonnet-4",
		"max_tokens": 1024,
		"system": "你是一个助手",
		"messages": [
			{"role": "user", "content": "你好"},
			{"role": "assistant", "content": [
				{"type": "text", "text": "我来看看"},
				{"type": "tool_use", "id": "toolu_1", "name": "get_weather", "input": {"city": "北京"}}
			]},
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "toolu_1", "content": "晴，25 度"},
				{"type": "text", "text": "总结一下"}
			]}
		],
		"tools": [{"name": "get_weather", "description": "查天气", "input_schema": {"type": "object"}}],
		"tool_choice": {"type": "auto"},
		"stream": true
	}`
	var req anthropicRequest
	mustUnmarshal(t, raw, &req)

	out := anthropicRequestToOpenAI(req, "glm-4.6")

	if out.Model != "glm-4.6" {
		t.Errorf("模型映射失败: %s", out.Model)
	}
	if len(out.Messages) != 5 {
		t.Fatalf("消息数量不符，期望 5 (system/user/assistant/tool/user)，实际 %d: %+v", len(out.Messages), out.Messages)
	}
	if out.Messages[0].Role != "system" {
		t.Errorf("第一条应为 system: %+v", out.Messages[0])
	}
	// assistant: content + tool_calls
	assistant := out.Messages[2]
	if assistant.Role != "assistant" {
		t.Fatalf("第三条应为 assistant: %+v", assistant)
	}
	content, _ := rawToString(assistant.Content)
	if content != "我来看看" {
		t.Errorf("assistant 文本不符: %q", content)
	}
	if len(assistant.ToolCalls) != 1 || assistant.ToolCalls[0].Function.Name != "get_weather" {
		t.Fatalf("tool_calls 转换失败: %+v", assistant.ToolCalls)
	}
	// input 作为 RawMessage 原样透传（保留上游 JSON 中的空白）
	if assistant.ToolCalls[0].Function.Arguments != `{"city": "北京"}` {
		t.Errorf("tool_use input 序列化不符: %s", assistant.ToolCalls[0].Function.Arguments)
	}
	// tool result → tool 消息
	toolMsg := out.Messages[3]
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "toolu_1" {
		t.Fatalf("tool_result 转换失败: %+v", toolMsg)
	}
	toolContent, _ := rawToString(toolMsg.Content)
	if toolContent != "晴，25 度" {
		t.Errorf("tool_result 内容不符: %q", toolContent)
	}
	// tool_result 之后的同消息文本应拆为独立 user 消息
	tailMsg := out.Messages[4]
	if tailMsg.Role != "user" {
		t.Fatalf("第五条应为 user: %+v", tailMsg)
	}
	tailContent, _ := rawToString(tailMsg.Content)
	if tailContent != "总结一下" {
		t.Errorf("尾部 user 消息不符: %q", tailContent)
	}
	if len(out.Tools) != 1 || out.Tools[0].Function.Name != "get_weather" {
		t.Errorf("tools 转换失败: %+v", out.Tools)
	}
	if string(out.ToolChoice) != `"auto"` {
		t.Errorf("tool_choice 转换失败: %s", out.ToolChoice)
	}
	if !out.Stream || out.StreamOptions == nil || !out.StreamOptions.IncludeUsage {
		t.Errorf("流式参数应透传并附加 include_usage")
	}
}

func TestOpenAIToAnthropicRequest(t *testing.T) {
	raw := `{
		"model": "gpt-4o",
		"messages": [
			{"role": "system", "content": "系统提示"},
			{"role": "user", "content": "hi"},
			{"role": "assistant", "content": null, "tool_calls": [
				{"id": "call_1", "type": "function", "function": {"name": "search", "arguments": "{\"q\": \"go\"}"}}
			]},
			{"role": "tool", "tool_call_id": "call_1", "content": "结果"},
			{"role": "tool", "tool_call_id": "call_2", "content": "结果2"}
		],
		"tools": [{"type": "function", "function": {"name": "search", "parameters": {"type": "object"}}}],
		"stream": true
	}`
	var req openaiRequest
	mustUnmarshal(t, raw, &req)

	out := openAIRequestToAnthropic(req, "claude-sonnet-4", 4096)

	if out.Model != "claude-sonnet-4" {
		t.Errorf("模型映射失败: %s", out.Model)
	}
	if out.MaxTokens != 4096 {
		t.Errorf("max_tokens 默认值失败: %d", out.MaxTokens)
	}
	sys := anthropicSystemToString(out.System)
	if sys != "系统提示" {
		t.Errorf("system 转换失败: %q", sys)
	}

	// messages: user, assistant(tool_use), user(两个 tool_result 合并)
	if len(out.Messages) != 3 {
		t.Fatalf("消息数量不符，期望 3，实际 %d: %+v", len(out.Messages), out.Messages)
	}
	if out.Messages[0].Role != "user" {
		t.Errorf("第一条应为 user")
	}
	assistant := out.Messages[1]
	blocks, ok := rawToBlocks(assistant.Content)
	if !ok || len(blocks) != 1 || blocks[0].Type != "tool_use" || blocks[0].ID != "call_1" {
		t.Fatalf("assistant tool_use 转换失败: %+v", blocks)
	}
	// json.Marshal 会压缩 RawMessage 内的空白
	if string(blocks[0].Input) != `{"q":"go"}` {
		t.Errorf("tool_call arguments 不符: %s", blocks[0].Input)
	}
	userBlocks, ok := rawToBlocks(out.Messages[2].Content)
	if !ok || len(userBlocks) != 2 {
		t.Fatalf("连续 tool 消息应合并为一条 user 消息: %+v", userBlocks)
	}
	if userBlocks[0].ToolUseID != "call_1" || userBlocks[1].ToolUseID != "call_2" {
		t.Errorf("tool_use_id 映射失败")
	}
	if len(out.Tools) != 1 {
		t.Errorf("tools 转换失败")
	}
}

func TestOpenAIResponseToAnthropic(t *testing.T) {
	raw := `{
		"id": "chatcmpl-123",
		"model": "glm-4.6",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "你好",
				"tool_calls": [{"id": "call_9", "type": "function", "function": {"name": "calc", "arguments": "{\"x\": 1}"}}]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`
	var resp openaiResponse
	mustUnmarshal(t, raw, &resp)

	out := openAIResponseToAnthropic(&resp, "claude-sonnet-4")
	if out.Type != "message" || out.Role != "assistant" {
		t.Errorf("基础字段错误: %+v", out)
	}
	if len(out.Content) != 2 {
		t.Fatalf("content 块数量不符: %+v", out.Content)
	}
	if out.Content[0].Type != "text" || out.Content[0].Text != "你好" {
		t.Errorf("文本块转换失败: %+v", out.Content[0])
	}
	if out.Content[1].Type != "tool_use" || out.Content[1].Name != "calc" {
		t.Errorf("tool_use 块转换失败: %+v", out.Content[1])
	}
	if out.StopReason != "tool_use" {
		t.Errorf("stop_reason 映射失败: %s", out.StopReason)
	}
	if out.Usage.InputTokens != 10 || out.Usage.OutputTokens != 5 {
		t.Errorf("usage 转换失败: %+v", out.Usage)
	}
}

func TestAnthropicResponseToOpenAI(t *testing.T) {
	raw := `{
		"id": "msg_abc",
		"type": "message",
		"role": "assistant",
		"model": "claude-sonnet-4",
		"content": [{"type": "text", "text": "答案"}, {"type": "tool_use", "id": "toolu_2", "name": "calc", "input": {"x": 2}}],
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 7, "output_tokens": 3}
	}`
	var resp anthropicResponse
	mustUnmarshal(t, raw, &resp)

	out := anthropicResponseToOpenAI(&resp)
	if out.Object != "chat.completion" || len(out.Choices) != 1 {
		t.Fatalf("基础结构错误: %+v", out)
	}
	choice := out.Choices[0]
	content, _ := rawToString(choice.Message.Content)
	if content != "答案" {
		t.Errorf("content 转换失败: %q", content)
	}
	if len(choice.Message.ToolCalls) != 1 || choice.Message.ToolCalls[0].Function.Name != "calc" {
		t.Errorf("tool_calls 转换失败: %+v", choice.Message.ToolCalls)
	}
	if choice.FinishReason != "tool_calls" {
		t.Errorf("finish_reason 映射失败: %s", choice.FinishReason)
	}
	if out.Usage == nil || out.Usage.PromptTokens != 7 {
		t.Errorf("usage 转换失败: %+v", out.Usage)
	}
}

func TestConvertOpenAIStreamToAnthropic(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"你好"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"世界"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"calc","arguments":""}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"x\":"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: {"id":"chatcmpl-1","choices":[],"usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20}}`,
		`data: [DONE]`,
		"\n\n",
	}, "\n\n")

	recorder := httptest.NewRecorder()
	if err := convertOpenAIStreamToAnthropic(strings.NewReader(upstream), recorder, "claude-sonnet-4"); err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	body := recorder.Body.String()
	for _, expect := range []string{
		"event: message_start",
		"event: content_block_start",
		`"text_delta"`,
		"event: content_block_stop",
		`"tool_use"`,
		`"input_json_delta"`,
		`"stop_reason":"tool_use"`,
		"event: message_stop",
	} {
		if !strings.Contains(body, expect) {
			t.Errorf("输出缺少 %s\n--- 完整输出 ---\n%s", expect, body)
		}
	}
	// 文本块必须在工具块之前关闭
	textStopIdx := strings.Index(body, `{"index":0,"type":"content_block_stop"}`)
	toolStartIdx := strings.Index(body, `"tool_use"`)
	if textStopIdx == -1 || toolStartIdx == -1 || textStopIdx > toolStartIdx {
		t.Errorf("文本块应在工具块开始前关闭\n%s", body)
	}
	if !strings.Contains(body, `"output_tokens":8`) {
		t.Errorf("usage 未转换: %s", body)
	}
}

func TestConvertAnthropicStreamToOpenAI(t *testing.T) {
	upstream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4","usage":{"input_tokens":5,"output_tokens":1}}}`,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"你"}}`,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"好"}}`,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"calc","input":{}}}`,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"x\":"}}`,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"1}"}}`,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}`,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		"\n",
	}, "\n")

	recorder := httptest.NewRecorder()
	if err := convertAnthropicStreamToOpenAI(strings.NewReader(upstream), recorder, "gpt-4o"); err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	body := recorder.Body.String()
	for _, expect := range []string{
		`"role":"assistant"`,
		`"content":"你"`,
		`"content":"好"`,
		`"tool_calls"`,
		`"name":"calc"`,
		`"arguments":"{\"x\":"`,
		`"arguments":"1}"`,
		`"finish_reason":"tool_calls"`,
		`data: [DONE]`,
	} {
		if !strings.Contains(body, expect) {
			t.Errorf("输出缺少 %s\n--- 完整输出 ---\n%s", expect, body)
		}
	}
	if recorder.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type 应为 text/event-stream")
	}
}

func TestMapModel(t *testing.T) {
	route := APIRoute{
		ModelMapping: map[string]string{
			"claude-sonnet-4": "glm-4.6",
			"*":               "deepseek-chat",
		},
	}
	if got := route.mapModel("claude-sonnet-4"); got != "glm-4.6" {
		t.Errorf("精确匹配失败: %s", got)
	}
	if got := route.mapModel("unknown-model"); got != "deepseek-chat" {
		t.Errorf("通配兜底失败: %s", got)
	}

	route2 := APIRoute{DefaultModel: "kimi-k2"}
	if got := route2.mapModel("anything"); got != "kimi-k2" {
		t.Errorf("默认模型失败: %s", got)
	}
}

func TestRouterGatewayEndToEnd(t *testing.T) {
	// 模拟一个 OpenAI 兼容上游，验证 Anthropic 入站请求被正确转换
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("上游路径不符: %s", r.URL.Path)
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("上游鉴权头缺失")
		}
		var req openaiRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "glm-4.6" {
			t.Errorf("上游收到的模型不符: %s", req.Model)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-e2e",
			"choices": []map[string]any{{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "转换成功",
				},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5},
		})
	}))
	defer upstream.Close()

	rs := NewRouterService()
	rs.config = RouterConfig{
		Port: 0,
		Routes: []APIRoute{{
			Name:         "e2e",
			SourceFormat: "anthropic",
			TargetFormat: "openai",
			BaseURL:      upstream.URL,
			APIKey:       "sk-test",
			ModelMapping: map[string]string{"*": "glm-4.6"},
			Enabled:      true,
		}},
	}

	recorder := httptest.NewRecorder()
	anthropicBody := `{"model":"claude-sonnet-4","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/e2e/v1/messages", strings.NewReader(anthropicBody))
	rs.handleRoot(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	var resp anthropicResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应解析失败: %v", err)
	}
	if len(resp.Content) != 1 || resp.Content[0].Text != "转换成功" {
		t.Errorf("响应内容不符: %+v", resp)
	}
	if resp.Model != "claude-sonnet-4" {
		t.Errorf("响应应保留入站模型名: %s", resp.Model)
	}
	if resp.Usage.InputTokens != 3 {
		t.Errorf("usage 转换失败: %+v", resp.Usage)
	}
}

func TestRouterGatewayStreaming(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"流\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"式\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	rs := NewRouterService()
	rs.config = RouterConfig{
		Routes: []APIRoute{{
			Name:         "stream",
			SourceFormat: "anthropic",
			TargetFormat: "openai",
			BaseURL:      upstream.URL,
			Enabled:      true,
		}},
	}

	recorder := httptest.NewRecorder()
	anthropicBody := `{"model":"claude-sonnet-4","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/stream/v1/messages", strings.NewReader(anthropicBody))
	rs.handleRoot(recorder, req)

	body := recorder.Body.String()
	for _, expect := range []string{"event: message_start", `"text_delta"`, `"流"`, `"式"`, `"stop_reason":"end_turn"`, "event: message_stop"} {
		if !strings.Contains(body, expect) {
			t.Errorf("流式输出缺少 %s\n%s", expect, body)
		}
	}
}

func TestRouterGatewayUnknownRoute(t *testing.T) {
	rs := NewRouterService()
	rs.config = RouterConfig{}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/nope/v1/messages", strings.NewReader("{}"))
	rs.handleRoot(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("未知路由应返回 404，实际 %d", recorder.Code)
	}
}
