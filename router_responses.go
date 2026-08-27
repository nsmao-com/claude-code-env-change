package main

// OpenAI Responses API（Codex 默认 wire_api = "responses"）与
// Chat Completions / Anthropic Messages 之间的转换。
// 入站 /v1/responses：把 Codex 请求转成上游 Chat/Anthropic，再把结果还原成 Responses SSE/JSON。

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ============ Responses 请求结构 ============

type responsesRequest struct {
	Model           string          `json:"model"`
	Input           json.RawMessage `json:"input,omitempty"`
	Instructions    string          `json:"instructions,omitempty"`
	MaxOutputTokens *int            `json:"max_output_tokens,omitempty"`
	Temperature     *float64        `json:"temperature,omitempty"`
	TopP            *float64        `json:"top_p,omitempty"`
	Tools           json.RawMessage `json:"tools,omitempty"`
	ToolChoice      json.RawMessage `json:"tool_choice,omitempty"`
	Stream          bool            `json:"stream,omitempty"`
	Store           *bool           `json:"store,omitempty"`
}

// serveResponsesEndpoint 处理入站 Responses API（Codex 等）
func (rs *RouterService) serveResponsesEndpoint(w http.ResponseWriter, r *http.Request, route APIRoute) {
	start := time.Now()

	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		rs.finishRequest(w, route, r, start, http.StatusMethodNotAllowed, "", fmt.Errorf("仅支持 POST /v1/responses"), true)
		writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "仅支持 POST /v1/responses")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGatewayBodyBytes))
	if err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadRequest, "", fmt.Errorf("读取请求体失败: %v", err), true)
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "读取请求体失败")
		return
	}

	var req responsesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		rs.finishRequest(w, route, r, start, http.StatusBadRequest, "", fmt.Errorf("请求不是有效的 Responses 格式: %v", err), true)
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "请求不是有效的 OpenAI Responses 格式")
		return
	}

	inboundModel := req.Model
	mappedModel := route.mapModel(req.Model)
	target := normalizeAPIFormat(route.TargetFormat)
	converted := responsesRequestToOpenAI(req, mappedModel)

	if target == "openai" {
		upstream, err := rs.newUpstreamRequest(route, http.MethodPost, "/v1/chat/completions", converted)
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
			if err := convertOpenAIStreamToResponses(resp.Body, w, inboundModel); err != nil {
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
		var oResp openaiResponse
		if err := json.Unmarshal(respBody, &oResp); err != nil {
			rs.finishRequest(w, route, r, start, http.StatusBadGateway, inboundModel, fmt.Errorf("上游响应解析失败: %v", err), true)
			writeOpenAIError(w, http.StatusBadGateway, "api_error", "上游响应不是有效的 OpenAI 格式")
			return
		}
		rs.finishRequest(w, route, r, start, http.StatusOK, inboundModel, nil, false)
		writeJSON(w, http.StatusOK, openAIResponseToResponses(&oResp, inboundModel))
		return
	}

	anthropicReq := openAIRequestToAnthropic(converted, mappedModel, defaultAnthropicMaxTokens)
	upstream, err := rs.newUpstreamRequest(route, http.MethodPost, "/v1/messages", anthropicReq)
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
		if err := convertAnthropicStreamToResponses(resp.Body, w, inboundModel); err != nil {
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
	oResp := anthropicResponseToOpenAI(&aResp)
	rs.finishRequest(w, route, r, start, http.StatusOK, inboundModel, nil, false)
	writeJSON(w, http.StatusOK, openAIResponseToResponses(oResp, inboundModel))
}

// ============ 请求转换：Responses → Chat Completions ============

func responsesRequestToOpenAI(req responsesRequest, model string) openaiRequest {
	out := openaiRequest{
		Model:       model,
		Messages:    responsesInputToMessages(req.Input, req.Instructions),
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		Tools:       responsesToolsToOpenAI(req.Tools),
		ToolChoice:  responsesToolChoiceToOpenAI(req.ToolChoice),
	}
	if req.MaxOutputTokens != nil && *req.MaxOutputTokens > 0 {
		out.MaxTokens = req.MaxOutputTokens
	}
	if out.Stream {
		out.StreamOptions = &openaiStreamOptions{IncludeUsage: true}
	}
	return out
}

func responsesInputToMessages(input json.RawMessage, instructions string) []openaiMessage {
	msgs := make([]openaiMessage, 0, 8)
	if strings.TrimSpace(instructions) != "" {
		msgs = append(msgs, openaiMessage{Role: "system", Content: jsonString(instructions)})
	}
	if len(input) == 0 {
		return msgs
	}
	if s, ok := rawToString(input); ok {
		if strings.TrimSpace(s) != "" {
			msgs = append(msgs, openaiMessage{Role: "user", Content: jsonString(s)})
		}
		return msgs
	}

	var items []json.RawMessage
	if err := json.Unmarshal(input, &items); err != nil {
		return msgs
	}

	var pending *openaiMessage
	flush := func() {
		if pending != nil {
			msgs = append(msgs, *pending)
			pending = nil
		}
	}

	for _, raw := range items {
		var item map[string]json.RawMessage
		if json.Unmarshal(raw, &item) != nil {
			continue
		}
		typ := rawJSONString(item["type"])
		role := rawJSONString(item["role"])
		if typ == "" && role != "" {
			typ = "message"
		}

		switch typ {
		case "reasoning", "item_reference":
			continue

		case "function_call":
			callID := firstNonEmpty(rawJSONString(item["call_id"]), rawJSONString(item["id"]))
			name := rawJSONString(item["name"])
			args := string(item["arguments"])
			if s, ok := rawToString(item["arguments"]); ok {
				args = s
			}
			if pending == nil {
				pending = &openaiMessage{Role: "assistant"}
			}
			pending.ToolCalls = append(pending.ToolCalls, openaiToolCall{
				ID:   callID,
				Type: "function",
				Function: openaiFunctionCall{
					Name:      name,
					Arguments: args,
				},
			})

		case "function_call_output":
			flush()
			callID := rawJSONString(item["call_id"])
			outStr := ""
			if s, ok := rawToString(item["output"]); ok {
				outStr = s
			} else if len(item["output"]) > 0 {
				outStr = string(item["output"])
			}
			msgs = append(msgs, openaiMessage{
				Role:       "tool",
				ToolCallID: callID,
				Content:    jsonString(outStr),
			})

		default:
			flush()
			if role == "" {
				role = "user"
			}
			if role == "developer" {
				role = "system"
			}
			content := item["content"]
			msg := openaiMessage{Role: role}
			if s, ok := rawToString(content); ok {
				msg.Content = jsonString(s)
				msgs = append(msgs, msg)
				continue
			}
			parts, ok := responsesContentToOpenAIParts(content)
			if !ok {
				if len(content) > 0 {
					msg.Content = content
				} else {
					msg.Content = jsonString("")
				}
				msgs = append(msgs, msg)
				continue
			}
			if len(parts) == 1 && parts[0].Type == "text" {
				msg.Content = jsonString(parts[0].Text)
			} else {
				msg.Content = mustMarshalJSON(parts)
			}
			msgs = append(msgs, msg)
		}
	}
	flush()
	return msgs
}

func responsesContentToOpenAIParts(raw json.RawMessage) ([]openaiContentPart, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var items []map[string]json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return nil, false
	}
	parts := make([]openaiContentPart, 0, len(items))
	for _, it := range items {
		typ := rawJSONString(it["type"])
		switch typ {
		case "input_text", "output_text", "text":
			text := rawJSONString(it["text"])
			if text != "" {
				parts = append(parts, openaiContentPart{Type: "text", Text: text})
			}
		case "input_image", "image_url":
			url := rawJSONString(it["image_url"])
			if url == "" {
				var nested struct {
					URL string `json:"url"`
				}
				_ = json.Unmarshal(it["image_url"], &nested)
				url = nested.URL
			}
			if url == "" {
				url = rawJSONString(it["url"])
			}
			if url != "" {
				parts = append(parts, openaiContentPart{Type: "image_url", ImageURL: &openaiImageURL{URL: url}})
			}
		}
	}
	return parts, true
}

func responsesToolsToOpenAI(raw json.RawMessage) []openaiTool {
	if len(raw) == 0 {
		return nil
	}
	var items []map[string]json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return nil
	}
	tools := make([]openaiTool, 0, len(items))
	for _, it := range items {
		typ := rawJSONString(it["type"])
		if typ != "" && typ != "function" {
			continue
		}
		name := rawJSONString(it["name"])
		desc := rawJSONString(it["description"])
		params := it["parameters"]
		if len(it["function"]) > 0 {
			var fn openaiToolSpec
			if json.Unmarshal(it["function"], &fn) == nil {
				if name == "" {
					name = fn.Name
				}
				if desc == "" {
					desc = fn.Description
				}
				if len(params) == 0 {
					params = fn.Parameters
				}
			}
		}
		if name == "" {
			continue
		}
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object"}`)
		}
		tools = append(tools, openaiTool{
			Type: "function",
			Function: openaiToolSpec{
				Name:        name,
				Description: desc,
				Parameters:  params,
			},
		})
	}
	if len(tools) == 0 {
		return nil
	}
	return tools
}

func responsesToolChoiceToOpenAI(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if s, ok := rawToString(raw); ok {
		switch s {
		case "auto", "none", "required":
			return json.RawMessage(strconvQuote(s))
		}
		return nil
	}
	var tc struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if json.Unmarshal(raw, &tc) != nil {
		return nil
	}
	switch tc.Type {
	case "auto":
		return json.RawMessage(`"auto"`)
	case "none":
		return json.RawMessage(`"none"`)
	case "required", "any":
		return json.RawMessage(`"required"`)
	case "function":
		name := tc.Name
		if name == "" {
			name = tc.Function.Name
		}
		if name != "" {
			return json.RawMessage(fmt.Sprintf(`{"type":"function","function":{"name":%s}}`, strconvQuote(name)))
		}
	}
	return nil
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func rawJSONString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if s, ok := rawToString(raw); ok {
		return s
	}
	return strings.Trim(string(raw), `"`)
}

// ============ 响应转换：Chat Completions → Responses ============

func openAIResponseToResponses(resp *openaiResponse, requestModel string) map[string]any {
	id := "resp_" + strings.TrimPrefix(resp.ID, "chatcmpl-")
	if id == "resp_" {
		id = fmt.Sprintf("resp_router_%d", time.Now().UnixMilli())
	}
	output := make([]map[string]any, 0, 2)
	status := "completed"

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		text := ""
		if s, ok := rawToString(choice.Message.Content); ok {
			text = s
		}
		if text != "" || len(choice.Message.ToolCalls) == 0 {
			output = append(output, map[string]any{
				"id":      "msg_" + id,
				"type":    "message",
				"status":  "completed",
				"role":    "assistant",
				"content": []map[string]any{{"type": "output_text", "text": text, "annotations": []any{}}},
			})
		}
		for i, tc := range choice.Message.ToolCalls {
			callID := tc.ID
			if callID == "" {
				callID = fmt.Sprintf("call_%s_%d", id, i)
			}
			fcID := "fc_" + callID
			output = append(output, map[string]any{
				"id":        fcID,
				"type":      "function_call",
				"status":    "completed",
				"name":      tc.Function.Name,
				"call_id":   callID,
				"arguments": tc.Function.Arguments,
			})
		}
		if choice.FinishReason == "length" {
			status = "incomplete"
		}
	}

	result := map[string]any{
		"id":         id,
		"object":     "response",
		"created_at": time.Now().Unix(),
		"status":     status,
		"model":      requestModel,
		"output":     output,
		"error":      nil,
	}
	if resp.Usage != nil {
		result["usage"] = map[string]any{
			"input_tokens":  resp.Usage.PromptTokens,
			"output_tokens": resp.Usage.CompletionTokens,
			"total_tokens":  resp.Usage.TotalTokens,
		}
	}
	if status == "incomplete" {
		result["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
	}
	return result
}

// ============ 流式：Chat Completions chunks → Responses SSE ============

type responsesSSEEmitter struct {
	sse       *sseWriter
	seq       int
	id        string
	model     string
	createdAt int64
	output    []map[string]any

	msgOpen   bool
	msgDone   bool
	msgID     string
	msgIndex  int
	textOpen  bool
	textBuf   strings.Builder
	textIndex int

	tools []*responsesToolStream
}

type responsesToolStream struct {
	outputIndex int
	id          string
	callID      string
	name        string
	args        strings.Builder
	started     bool
	done        bool
}

func newResponsesSSEEmitter(w http.ResponseWriter, model string) (*responsesSSEEmitter, error) {
	sse, err := newSSEWriter(w)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &responsesSSEEmitter{
		sse:       sse,
		id:        fmt.Sprintf("resp_router_%d", now.UnixMilli()),
		model:     model,
		createdAt: now.Unix(),
		output:    []map[string]any{},
	}, nil
}

func (e *responsesSSEEmitter) send(event string, payload map[string]any) {
	payload["type"] = event
	payload["sequence_number"] = e.seq
	e.seq++
	e.sse.sendEvent(event, payload)
}

func (e *responsesSSEEmitter) baseResponse(status string, output []map[string]any, usage *openaiUsage) map[string]any {
	resp := map[string]any{
		"id":         e.id,
		"object":     "response",
		"created_at": e.createdAt,
		"status":     status,
		"model":      e.model,
		"output":     output,
		"error":      nil,
	}
	if usage != nil {
		resp["usage"] = map[string]any{
			"input_tokens":  usage.PromptTokens,
			"output_tokens": usage.CompletionTokens,
			"total_tokens":  usage.TotalTokens,
		}
	}
	return resp
}

func (e *responsesSSEEmitter) start() {
	empty := []map[string]any{}
	e.send("response.created", map[string]any{
		"response": e.baseResponse("in_progress", empty, nil),
	})
	e.send("response.in_progress", map[string]any{
		"response": e.baseResponse("in_progress", empty, nil),
	})
}

func (e *responsesSSEEmitter) ensureMessage() {
	if e.msgOpen || e.msgDone {
		return
	}
	e.msgID = "msg_" + e.id
	e.msgIndex = len(e.output)
	e.msgOpen = true
	item := map[string]any{
		"id":      e.msgID,
		"type":    "message",
		"status":  "in_progress",
		"role":    "assistant",
		"content": []any{},
	}
	e.send("response.output_item.added", map[string]any{
		"output_index": e.msgIndex,
		"item":         item,
	})
}

func (e *responsesSSEEmitter) textDelta(s string) {
	if s == "" {
		return
	}
	e.ensureMessage()
	if !e.textOpen {
		e.textOpen = true
		e.send("response.content_part.added", map[string]any{
			"output_index":  e.msgIndex,
			"content_index": e.textIndex,
			"item_id":       e.msgID,
			"part":          map[string]any{"type": "output_text", "text": "", "annotations": []any{}},
		})
	}
	e.textBuf.WriteString(s)
	e.send("response.output_text.delta", map[string]any{
		"output_index":  e.msgIndex,
		"content_index": e.textIndex,
		"item_id":       e.msgID,
		"delta":         s,
	})
}

func (e *responsesSSEEmitter) closeMessage() {
	if !e.msgOpen || e.msgDone {
		return
	}
	text := e.textBuf.String()
	if e.textOpen {
		e.send("response.output_text.done", map[string]any{
			"output_index":  e.msgIndex,
			"content_index": e.textIndex,
			"item_id":       e.msgID,
			"text":          text,
		})
		e.send("response.content_part.done", map[string]any{
			"output_index":  e.msgIndex,
			"content_index": e.textIndex,
			"item_id":       e.msgID,
			"part":          map[string]any{"type": "output_text", "text": text, "annotations": []any{}},
		})
		e.textOpen = false
	}
	item := map[string]any{
		"id":     e.msgID,
		"type":   "message",
		"status": "completed",
		"role":   "assistant",
		"content": []map[string]any{
			{"type": "output_text", "text": text, "annotations": []any{}},
		},
	}
	e.send("response.output_item.done", map[string]any{
		"output_index": e.msgIndex,
		"item":         item,
	})
	e.output = append(e.output, item)
	e.msgOpen = false
	e.msgDone = true
}

func (e *responsesSSEEmitter) toolStart(index int, callID, name string) *responsesToolStream {
	e.closeMessage()
	for len(e.tools) <= index {
		e.tools = append(e.tools, nil)
	}
	if e.tools[index] != nil {
		t := e.tools[index]
		if name != "" {
			t.name = name
		}
		if callID != "" && !t.started {
			t.callID = callID
			t.id = "fc_" + callID
		}
		return t
	}
	if callID == "" {
		callID = fmt.Sprintf("call_%s_%d", e.id, index)
	}
	t := &responsesToolStream{
		outputIndex: len(e.output) + pendingToolCount(e.tools),
		id:          "fc_" + callID,
		callID:      callID,
		name:        name,
	}
	e.tools[index] = t
	return t
}

func pendingToolCount(tools []*responsesToolStream) int {
	n := 0
	for _, t := range tools {
		if t != nil && t.started && !t.done {
			n++
		}
	}
	return n
}

func (e *responsesSSEEmitter) ensureToolStarted(t *responsesToolStream) {
	if t == nil || t.started || t.name == "" || t.callID == "" {
		return
	}
	t.outputIndex = len(e.output) + pendingToolCount(e.tools)
	item := map[string]any{
		"id":        t.id,
		"type":      "function_call",
		"status":    "in_progress",
		"name":      t.name,
		"call_id":   t.callID,
		"arguments": "",
	}
	e.send("response.output_item.added", map[string]any{
		"output_index": t.outputIndex,
		"item":         item,
	})
	t.started = true
}

func (e *responsesSSEEmitter) toolArgs(index int, delta string) {
	if index < 0 || index >= len(e.tools) || e.tools[index] == nil {
		return
	}
	t := e.tools[index]
	e.ensureToolStarted(t)
	if !t.started || delta == "" {
		return
	}
	t.args.WriteString(delta)
	e.send("response.function_call_arguments.delta", map[string]any{
		"output_index": t.outputIndex,
		"item_id":      t.id,
		"delta":        delta,
	})
}

func (e *responsesSSEEmitter) closeTools() {
	for _, t := range e.tools {
		if t == nil || t.done {
			continue
		}
		e.ensureToolStarted(t)
		if !t.started {
			continue
		}
		args := t.args.String()
		e.send("response.function_call_arguments.done", map[string]any{
			"output_index": t.outputIndex,
			"item_id":      t.id,
			"arguments":    args,
		})
		item := map[string]any{
			"id":        t.id,
			"type":      "function_call",
			"status":    "completed",
			"name":      t.name,
			"call_id":   t.callID,
			"arguments": args,
		}
		e.send("response.output_item.done", map[string]any{
			"output_index": t.outputIndex,
			"item":         item,
		})
		e.output = append(e.output, item)
		t.done = true
	}
}

func (e *responsesSSEEmitter) complete(finish string, usage *openaiUsage) {
	if e.msgOpen {
		e.closeMessage()
	} else if !e.msgDone && len(e.tools) == 0 {
		e.ensureMessage()
		e.closeMessage()
	}
	e.closeTools()
	status := "completed"
	if finish == "length" {
		status = "incomplete"
	}
	payload := map[string]any{
		"response": e.baseResponse(status, e.output, usage),
	}
	if status == "incomplete" {
		payload["response"].(map[string]any)["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
	}
	e.send("response.completed", payload)
	fmt.Fprintf(e.sse.w, "data: [DONE]\n\n")
	e.sse.flusher.Flush()
}

func convertOpenAIStreamToResponses(upstream io.Reader, w http.ResponseWriter, inboundModel string) error {
	em, err := newResponsesSSEEmitter(w, inboundModel)
	if err != nil {
		return err
	}
	em.start()

	var usage *openaiUsage
	finish := ""
	scanner := newSSEScanner(upstream)
	for {
		payload, ok := scanner.next()
		if !ok {
			break
		}
		var chunk openaiChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		if chunk.ID != "" {
			id := "resp_" + strings.TrimPrefix(chunk.ID, "chatcmpl-")
			if id != "resp_" {
				em.id = id
			}
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if choice.Delta.Content != "" {
			em.textDelta(choice.Delta.Content)
		}
		for _, tc := range choice.Delta.ToolCalls {
			t := em.toolStart(tc.Index, tc.ID, tc.Function.Name)
			em.ensureToolStarted(t)
			if tc.Function.Arguments != "" {
				em.toolArgs(tc.Index, tc.Function.Arguments)
			}
		}
		if choice.FinishReason != "" {
			finish = choice.FinishReason
		}
	}
	em.complete(finish, usage)
	return nil
}

func convertAnthropicStreamToResponses(upstream io.Reader, w http.ResponseWriter, inboundModel string) error {
	em, err := newResponsesSSEEmitter(w, inboundModel)
	if err != nil {
		return err
	}
	em.start()

	var usage *openaiUsage
	finish := ""
	toolsByBlock := map[int]int{}
	nextTool := 0

	scanner := newSSEScanner(upstream)
	for {
		payload, ok := scanner.next()
		if !ok {
			break
		}
		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		switch event.Type {
		case "message_start":
			if event.Message != nil && event.Message.ID != "" {
				em.id = "resp_" + strings.TrimPrefix(event.Message.ID, "msg_")
			}
			if event.Message != nil && (event.Message.Usage.InputTokens > 0 || event.Message.Usage.OutputTokens > 0) {
				usage = &openaiUsage{
					PromptTokens:     event.Message.Usage.InputTokens,
					CompletionTokens: event.Message.Usage.OutputTokens,
					TotalTokens:      event.Message.Usage.InputTokens + event.Message.Usage.OutputTokens,
				}
			}
		case "content_block_start":
			if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
				idx := nextTool
				nextTool++
				toolsByBlock[event.Index] = idx
				t := em.toolStart(idx, event.ContentBlock.ID, event.ContentBlock.Name)
				em.ensureToolStarted(t)
			}
		case "content_block_delta":
			if event.Delta == nil {
				break
			}
			switch event.Delta.Type {
			case "text_delta":
				em.textDelta(event.Delta.Text)
			case "input_json_delta":
				if idx, ok := toolsByBlock[event.Index]; ok {
					em.toolArgs(idx, event.Delta.PartialJSON)
				}
			}
		case "message_delta":
			if event.Delta != nil && event.Delta.StopReason != "" {
				finish = anthropicStopToOpenAIFinish(event.Delta.StopReason)
			}
			if event.Usage != nil {
				if usage == nil {
					usage = &openaiUsage{}
				}
				if event.Usage.InputTokens > 0 {
					usage.PromptTokens = event.Usage.InputTokens
				}
				if event.Usage.OutputTokens > 0 {
					usage.CompletionTokens = event.Usage.OutputTokens
				}
				usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			}
		}
	}
	em.complete(finish, usage)
	return nil
}
