package main

// OpenAI Chat Completions → Responses：Chat 或 Anthropic 协议的客户端（OpenCode、Claude Code 等）
// 用只说 Responses 的上游（ChatGPT 订阅、部分中转站）时，请求转成 Responses，回复再转回 Chat；
// Anthropic 客户端经 Chat 中转（Anthropic ⇄ Chat 的转换已有）。

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// openAIRequestToResponses Chat Completions 请求转成 Responses 请求
func openAIRequestToResponses(req openaiRequest, model string) map[string]any {
	var instructions []string
	input := []any{}
	for _, m := range req.Messages {
		switch m.Role {
		case "system", "developer":
			if s := chatContentText(m.Content); s != "" {
				instructions = append(instructions, s)
			}
		case "user":
			content := []any{}
			if s, ok := rawToString(m.Content); ok {
				content = append(content, map[string]any{"type": "input_text", "text": s})
			} else if parts, ok := rawToParts(m.Content); ok {
				for _, p := range parts {
					switch {
					case p.Type == "text":
						content = append(content, map[string]any{"type": "input_text", "text": p.Text})
					case p.Type == "image_url" && p.ImageURL != nil:
						content = append(content, map[string]any{"type": "input_image", "image_url": p.ImageURL.URL})
					}
				}
			}
			if len(content) > 0 {
				input = append(input, map[string]any{"role": "user", "content": content})
			}
		case "assistant":
			if s := chatContentText(m.Content); s != "" {
				input = append(input, map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": s}}})
			}
			for _, tc := range m.ToolCalls {
				args := tc.Function.Arguments
				if strings.TrimSpace(args) == "" {
					args = "{}"
				}
				input = append(input, map[string]any{"type": "function_call", "call_id": tc.ID, "name": tc.Function.Name, "arguments": args})
			}
		case "tool":
			input = append(input, map[string]any{"type": "function_call_output", "call_id": m.ToolCallID, "output": chatContentText(m.Content)})
		}
	}
	body := map[string]any{"model": model, "input": input, "stream": req.Stream}
	if len(instructions) > 0 {
		body["instructions"] = strings.Join(instructions, "\n\n")
	}
	if len(req.Tools) > 0 {
		tools := make([]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tool := map[string]any{"type": "function", "name": t.Function.Name}
			if t.Function.Description != "" {
				tool["description"] = t.Function.Description
			}
			if len(t.Function.Parameters) > 0 {
				tool["parameters"] = t.Function.Parameters
			} else {
				tool["parameters"] = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			tools = append(tools, tool)
		}
		body["tools"] = tools
	}
	if len(req.ToolChoice) > 0 {
		var s string
		if json.Unmarshal(req.ToolChoice, &s) == nil {
			body["tool_choice"] = s
		} else {
			var c struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			}
			if json.Unmarshal(req.ToolChoice, &c) == nil && c.Function.Name != "" {
				body["tool_choice"] = map[string]any{"type": "function", "name": c.Function.Name}
			}
		}
	}
	switch {
	case req.MaxCompletionTokens != nil:
		body["max_output_tokens"] = *req.MaxCompletionTokens
	case req.MaxTokens != nil:
		body["max_output_tokens"] = *req.MaxTokens
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}
	return body
}

// serveChatViaResponses 路由上游只说 Responses 时：Chat 请求（Anthropic 请求已先转成 Chat）转成
// Responses 发出，回复转回调用方的协议
func (rs *RouterService) serveChatViaResponses(w http.ResponseWriter, r *http.Request, route APIRoute, chat openaiRequest, mappedModel, inboundModel string, start time.Time, anthropicInbound bool) {
	writeErr := func(status int, msg string) {
		if anthropicInbound {
			writeAnthropicError(w, status, "api_error", msg)
		} else {
			writeOpenAIError(w, status, "api_error", msg)
		}
	}
	body := openAIRequestToResponses(chat, mappedModel)
	resp, err := rs.doWithFailover(r, route, func(rt APIRoute) (*http.Request, error) {
		return rs.newUpstreamRequest(rt, http.MethodPost, "/v1/responses", body)
	})
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
		writeErr(http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		rs.relayUpstreamError(w, resp, route, r, start, inboundModel, anthropicInbound)
		return
	}
	if chat.Stream {
		if anthropicInbound {
			pr, pw := io.Pipe()
			go func() {
				pw.CloseWithError(convertResponsesStreamToOpenAI(resp.Body, pw, nil, inboundModel))
			}()
			err = convertOpenAIStreamToAnthropic(pr, w, inboundModel)
			_ = pr.Close()
		} else {
			flusher, _ := w.(http.Flusher)
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("X-Accel-Buffering", "no")
			w.WriteHeader(http.StatusOK)
			err = convertResponsesStreamToOpenAI(resp.Body, w, func() {
				if flusher != nil {
					flusher.Flush()
				}
			}, inboundModel)
		}
		if err != nil {
			rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
			return
		}
		rs.finishRequest(w, route, r, start, http.StatusOK, inboundModel, nil, false)
		return
	}
	respBody, err := readUpstreamBody(resp)
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, err, true)
		writeErr(http.StatusBadGateway, "读取上游响应失败")
		return
	}
	oResp, err := responsesToOpenAIResponse(respBody, inboundModel)
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, fmt.Errorf("上游响应解析失败: %v", err), true)
		writeErr(http.StatusBadGateway, "上游响应不是有效的 Responses 格式")
		return
	}
	rs.finishRequest(w, route, r, start, http.StatusOK, inboundModel, nil, false)
	if anthropicInbound {
		writeJSON(w, http.StatusOK, openAIResponseToAnthropic(oResp, inboundModel))
		return
	}
	writeJSON(w, http.StatusOK, oResp)
}

// chatContentText Chat 消息内容（字符串或文字片段数组）里的文字
func chatContentText(raw json.RawMessage) string {
	if s, ok := rawToString(raw); ok {
		return s
	}
	parts, _ := rawToParts(raw)
	var b strings.Builder
	for _, p := range parts {
		if p.Type == "text" {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

type responsesOutputItem struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Content   []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type responsesResult struct {
	ID                string                `json:"id"`
	Model             string                `json:"model"`
	Status            string                `json:"status"`
	CreatedAt         int64                 `json:"created_at"`
	Output            []responsesOutputItem `json:"output"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func (r *responsesResult) finishReason(hasCalls bool) string {
	switch {
	case r.Status == "incomplete" && r.IncompleteDetails != nil && r.IncompleteDetails.Reason == "max_output_tokens":
		return "length"
	case r.Status == "incomplete" && r.IncompleteDetails != nil && r.IncompleteDetails.Reason == "content_filter":
		return "content_filter"
	case hasCalls:
		return "tool_calls"
	}
	return "stop"
}

func (r *responsesResult) chatUsage() *openaiUsage {
	if r.Usage == nil {
		return nil
	}
	total := r.Usage.TotalTokens
	if total == 0 {
		total = r.Usage.InputTokens + r.Usage.OutputTokens
	}
	return &openaiUsage{PromptTokens: r.Usage.InputTokens, CompletionTokens: r.Usage.OutputTokens, TotalTokens: total}
}

// responsesToOpenAIResponse Responses 的完整回复转成 Chat Completions 回复
func responsesToOpenAIResponse(body []byte, inboundModel string) (*openaiResponse, error) {
	var r responsesResult
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	var text strings.Builder
	var calls []openaiToolCall
	for _, item := range r.Output {
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				if c.Type == "output_text" || c.Type == "text" {
					text.WriteString(c.Text)
				}
			}
		case "function_call":
			calls = append(calls, openaiToolCall{ID: firstNonEmpty(item.CallID, item.ID), Type: "function", Function: openaiFunctionCall{Name: item.Name, Arguments: item.Arguments}})
		}
	}
	msg := openaiRespMessage{Role: "assistant", Content: jsonString(text.String()), ToolCalls: calls}
	if text.Len() == 0 && len(calls) > 0 {
		msg.Content = json.RawMessage("null")
	}
	created := r.CreatedAt
	if created == 0 {
		created = time.Now().Unix()
	}
	return &openaiResponse{
		ID:      "chatcmpl-" + strings.TrimPrefix(r.ID, "resp_"),
		Object:  "chat.completion",
		Created: created,
		Model:   firstNonEmpty(inboundModel, r.Model),
		Choices: []openaiResponseChoice{{Index: 0, Message: msg, FinishReason: r.finishReason(len(calls) > 0)}},
		Usage:   r.chatUsage(),
	}, nil
}

// convertResponsesStreamToOpenAI Responses 的 SSE 转成 Chat Completions 的流式分块
func convertResponsesStreamToOpenAI(upstream io.Reader, out io.Writer, flush func(), inboundModel string) error {
	id := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	created := time.Now().Unix()
	send := func(delta map[string]any, finish any, usage *openaiUsage) error {
		chunk := map[string]any{"id": id, "object": "chat.completion.chunk", "created": created, "model": inboundModel}
		if delta != nil || finish != nil {
			chunk["choices"] = []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}
		} else {
			chunk["choices"] = []any{}
		}
		if usage != nil {
			chunk["usage"] = usage
		}
		b, _ := json.Marshal(chunk)
		if _, err := fmt.Fprintf(out, "data: %s\n\n", b); err != nil {
			return err
		}
		if flush != nil {
			flush()
		}
		return nil
	}
	started := false
	start := func() error {
		if started {
			return nil
		}
		started = true
		return send(map[string]any{"role": "assistant", "content": ""}, nil, nil)
	}
	tools := map[string]int{} // 输出项 ID / call_id → 工具调用序号
	toolArgs := map[int]bool{}
	finished := false
	finish := func(r *responsesResult) error {
		if finished {
			return nil
		}
		finished = true
		if err := start(); err != nil {
			return err
		}
		reason := "stop"
		var usage *openaiUsage
		if r != nil {
			reason = r.finishReason(len(tools) > 0)
			usage = r.chatUsage()
		} else if len(tools) > 0 {
			reason = "tool_calls"
		}
		if err := send(map[string]any{}, reason, nil); err != nil {
			return err
		}
		if usage != nil {
			if err := send(nil, nil, usage); err != nil {
				return err
			}
		}
		_, err := io.WriteString(out, "data: [DONE]\n\n")
		if flush != nil {
			flush()
		}
		return err
	}
	scanner := bufio.NewScanner(upstream)
	scanner.Buffer(make([]byte, 64<<10), 32<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var ev struct {
			Type     string               `json:"type"`
			Delta    string               `json:"delta"`
			ItemID   string               `json:"item_id"`
			Item     *responsesOutputItem `json:"item"`
			Response *responsesResult     `json:"response"`
			Message  string               `json:"message"`
			Error    *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(payload), &ev) != nil {
			continue
		}
		var err error
		switch ev.Type {
		case "response.created":
			if ev.Response != nil && ev.Response.ID != "" {
				id = "chatcmpl-" + strings.TrimPrefix(ev.Response.ID, "resp_")
			}
			err = start()
		case "response.output_text.delta":
			if err = start(); err == nil && ev.Delta != "" {
				err = send(map[string]any{"content": ev.Delta}, nil, nil)
			}
		case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
			if err = start(); err == nil && ev.Delta != "" {
				err = send(map[string]any{"reasoning_content": ev.Delta}, nil, nil)
			}
		case "response.output_item.added":
			if ev.Item != nil && ev.Item.Type == "function_call" {
				if err = start(); err != nil {
					break
				}
				n := len(toolArgs)
				toolArgs[n] = false
				tools[ev.Item.ID] = n
				if ev.Item.CallID != "" {
					tools[ev.Item.CallID] = n
				}
				err = send(map[string]any{"tool_calls": []any{map[string]any{"index": n, "id": firstNonEmpty(ev.Item.CallID, ev.Item.ID), "type": "function",
					"function": map[string]any{"name": ev.Item.Name, "arguments": ""}}}}, nil, nil)
			}
		case "response.function_call_arguments.delta":
			if n, ok := tools[ev.ItemID]; ok && ev.Delta != "" {
				toolArgs[n] = true
				err = send(map[string]any{"tool_calls": []any{map[string]any{"index": n, "function": map[string]any{"arguments": ev.Delta}}}}, nil, nil)
			}
		case "response.output_item.done":
			// 有的上游不发参数增量，只在结束时给出完整参数
			if ev.Item != nil && ev.Item.Type == "function_call" {
				if n, ok := tools[ev.Item.ID]; ok && !toolArgs[n] && ev.Item.Arguments != "" {
					toolArgs[n] = true
					err = send(map[string]any{"tool_calls": []any{map[string]any{"index": n, "function": map[string]any{"arguments": ev.Item.Arguments}}}}, nil, nil)
				}
			}
		case "response.completed", "response.done", "response.incomplete":
			err = finish(ev.Response)
		case "response.failed", "error":
			msg := ev.Message
			if ev.Error != nil && ev.Error.Message != "" {
				msg = ev.Error.Message
			}
			var failed struct {
				Response *struct {
					Error *struct {
						Message string `json:"message"`
					} `json:"error"`
				} `json:"response"`
			}
			if json.Unmarshal([]byte(payload), &failed) == nil && failed.Response != nil && failed.Response.Error != nil && failed.Response.Error.Message != "" {
				msg = failed.Response.Error.Message
			}
			if msg == "" {
				msg = "上游返回错误"
			}
			return fmt.Errorf("%s", msg)
		}
		if err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return finish(nil)
}
