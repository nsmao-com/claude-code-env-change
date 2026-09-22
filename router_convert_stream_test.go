package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// 空参数的 tool_use（上游不发 input_json_delta）也必须发出 tool_calls 首帧，
// usage 要合并 message_start 的 input_tokens 与 message_delta 的 output_tokens
func TestConvertAnthropicStreamToOpenAIEmptyToolArgsAndUsage(t *testing.T) {
	upstream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_9","type":"message","role":"assistant","model":"claude-sonnet-4","usage":{"input_tokens":42,"output_tokens":1}}}`,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_a","name":"read_file","input":{}}}`,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":7}}`,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		"\n",
	}, "\n")

	recorder := httptest.NewRecorder()
	if err := convertAnthropicStreamToOpenAI(strings.NewReader(upstream), recorder, "gpt-4o"); err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `"name":"read_file"`) {
		t.Errorf("空参数 tool_use 未发出首帧 tool_calls\n%s", body)
	}
	if !strings.Contains(body, `"prompt_tokens":42`) {
		t.Errorf("message_start 的 input_tokens 丢失（prompt_tokens 应为 42）\n%s", body)
	}
	if !strings.Contains(body, `"completion_tokens":7`) {
		t.Errorf("message_delta 的 output_tokens 丢失\n%s", body)
	}
}

// tool_result 的 is_error 要以文本标记传达，不能静默丢弃
func TestAnthropicToolResultIsErrorMarked(t *testing.T) {
	out := string(anthropicToolResultToOpenAIContent(nil, true))
	if !strings.Contains(out, "tool execution failed") {
		t.Errorf("is_error 未标记: %s", out)
	}
	out = string(anthropicToolResultToOpenAIContent(nil, false))
	if strings.Contains(out, "tool execution failed") {
		t.Errorf("非错误结果被误标记: %s", out)
	}
}

// OpenAI→Anthropic 流式：文本在工具块之后到达时分配新的块索引，
// 且每个块的 start/stop 严格串行成对
func TestConvertOpenAIStreamToAnthropicLateTextBlockOrder(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"calc","arguments":"{}"}}]}]}`,
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"done"}}]}`,
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		"data: [DONE]",
		"\n",
	}, "\n")

	recorder := httptest.NewRecorder()
	if err := convertOpenAIStreamToAnthropic(strings.NewReader(upstream), recorder, "claude-sonnet-4"); err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	body := recorder.Body.String()
	// 文本块复用 index 0、工具块拿 index 1：同 index 的第二个 start 即为冲突
	if strings.Count(body, `"index":0,"content_block_start"`) > 1 && strings.Count(body, "content_block_start") > 2 {
		t.Errorf("文本块与工具块 index 冲突\n%s", body)
	}
	var starts, stops int
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "content_block_start") {
			starts++
		}
		if strings.Contains(line, "content_block_stop") {
			stops++
		}
	}
	if starts != stops {
		t.Errorf("content_block start/stop 不成对: start=%d stop=%d\n%s", starts, stops, body)
	}
}
