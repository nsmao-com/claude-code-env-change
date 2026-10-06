package main

// Gemini 协议入口：Gemini CLI 等只会说 Google Gemini API（generateContent / streamGenerateContent）
// 的客户端，请求先转成 OpenAI Chat Completions，交给现有的 Chat 入口（再由它转成路由上游的协议），
// 回复再转回 Gemini 格式；流式按 alt=sse 输出。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// parseGeminiEndpoint 识别 v1beta/models/<模型>:<方法>，以及模型列表 v1beta/models
func parseGeminiEndpoint(endpoint string) (model, method string, ok bool) {
	rest := endpoint
	for _, v := range []string{"v1beta/", "v1alpha/", "v1/"} {
		if strings.HasPrefix(rest, v) {
			rest = strings.TrimPrefix(rest, v)
			break
		}
	}
	// v1/models 是 OpenAI 的模型列表，只有 v1beta / v1alpha 下的才是 Gemini 的
	if rest == "models" && (strings.HasPrefix(endpoint, "v1beta/") || strings.HasPrefix(endpoint, "v1alpha/")) {
		return "", "list", true
	}
	if !strings.HasPrefix(rest, "models/") {
		return "", "", false
	}
	name, m, found := strings.Cut(strings.TrimPrefix(rest, "models/"), ":")
	if !found || name == "" {
		return "", "", false
	}
	switch m {
	case "generateContent", "streamGenerateContent", "countTokens":
		return name, m, true
	}
	return "", "", false
}

// geminiNativeRoute Antigravity 平台自己的路由：上游按 Gemini 协议直连，请求原样透传
func geminiNativeRoute(route APIRoute) bool {
	return strings.EqualFold(route.Name, providerRouteName("antigravity")) && normalizeAPIFormat(route.TargetFormat) == "openai"
}

type geminiPart struct {
	Text             string `json:"text,omitempty"`
	Thought          bool   `json:"thought,omitempty"`
	ThoughtSignature string `json:"thoughtSignature,omitempty"`
	InlineData       *struct {
		MimeType string `json:"mimeType"`
		Data     string `json:"data"`
	} `json:"inlineData,omitempty"`
	FileData *struct {
		MimeType string `json:"mimeType"`
		FileURI  string `json:"fileUri"`
	} `json:"fileData,omitempty"`
	FunctionCall *struct {
		ID   string          `json:"id,omitempty"`
		Name string          `json:"name"`
		Args json.RawMessage `json:"args,omitempty"`
	} `json:"functionCall,omitempty"`
	FunctionResponse *struct {
		ID       string          `json:"id,omitempty"`
		Name     string          `json:"name"`
		Response json.RawMessage `json:"response,omitempty"`
	} `json:"functionResponse,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	Contents          []geminiContent `json:"contents"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	Tools             []struct {
		FunctionDeclarations []struct {
			Name                 string          `json:"name"`
			Description          string          `json:"description"`
			Parameters           json.RawMessage `json:"parameters"`
			ParametersJSONSchema json.RawMessage `json:"parametersJsonSchema"`
		} `json:"functionDeclarations"`
	} `json:"tools"`
	ToolConfig *struct {
		FunctionCallingConfig *struct {
			Mode    string   `json:"mode"`
			Allowed []string `json:"allowedFunctionNames"`
		} `json:"functionCallingConfig"`
	} `json:"toolConfig"`
	GenerationConfig *struct {
		Temperature     *float64 `json:"temperature"`
		TopP            *float64 `json:"topP"`
		MaxOutputTokens int      `json:"maxOutputTokens"`
		StopSequences   []string `json:"stopSequences"`
	} `json:"generationConfig"`
}

// geminiSchema Gemini 的 OpenAPI 子集（类型大写，如 OBJECT）转成普通 JSON Schema
func geminiSchema(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			switch k {
			case "type":
				if s, ok := val.(string); ok {
					out[k] = strings.ToLower(s)
					continue
				}
			case "propertyOrdering":
				continue
			}
			out[k] = geminiSchema(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = geminiSchema(val)
		}
		return out
	}
	return v
}

func geminiText(c *geminiContent) string {
	if c == nil {
		return ""
	}
	var parts []string
	for _, p := range c.Parts {
		if p.Text != "" && !p.Thought {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// geminiRequestToChat Gemini 请求转成 Chat Completions 请求。函数调用没有 ID 时按顺序生成，
// 函数结果按同名调用的先后对上
func geminiRequestToChat(body []byte, model string, stream bool) (map[string]any, error) {
	var req geminiRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	messages := []any{}
	if s := geminiText(req.SystemInstruction); s != "" {
		messages = append(messages, map[string]any{"role": "system", "content": s})
	}
	pending := map[string][]string{} // 函数名 → 还没收到结果的调用 ID
	seq := 0
	for _, c := range req.Contents {
		if c.Role == "model" {
			var text strings.Builder
			calls := []any{}
			for _, p := range c.Parts {
				switch {
				case p.FunctionCall != nil:
					seq++
					id := p.FunctionCall.ID
					if id == "" {
						id = fmt.Sprintf("call_%d_%s", seq, p.FunctionCall.Name)
					}
					pending[p.FunctionCall.Name] = append(pending[p.FunctionCall.Name], id)
					args := strings.TrimSpace(string(p.FunctionCall.Args))
					if args == "" || args == "null" {
						args = "{}"
					}
					calls = append(calls, map[string]any{"id": id, "type": "function", "function": map[string]any{"name": p.FunctionCall.Name, "arguments": args}})
				case p.Text != "" && !p.Thought:
					text.WriteString(p.Text)
				}
			}
			msg := map[string]any{"role": "assistant", "content": text.String()}
			if len(calls) > 0 {
				msg["tool_calls"] = calls
				if text.Len() == 0 {
					msg["content"] = nil
				}
			}
			if text.Len() > 0 || len(calls) > 0 {
				messages = append(messages, msg)
			}
			continue
		}
		parts := []any{}
		var results []any
		for _, p := range c.Parts {
			switch {
			case p.FunctionResponse != nil:
				id := p.FunctionResponse.ID
				if q := pending[p.FunctionResponse.Name]; id == "" && len(q) > 0 {
					id = q[0]
				}
				if q := pending[p.FunctionResponse.Name]; len(q) > 0 {
					for i, v := range q {
						if v == id {
							pending[p.FunctionResponse.Name] = append(q[:i:i], q[i+1:]...)
							break
						}
					}
				}
				if id == "" {
					seq++
					id = fmt.Sprintf("call_%d_%s", seq, p.FunctionResponse.Name)
				}
				content := strings.TrimSpace(string(p.FunctionResponse.Response))
				// {"output": "..."} 这种只有一段文字的结果直接给文字
				var wrapped map[string]any
				if json.Unmarshal(p.FunctionResponse.Response, &wrapped) == nil && len(wrapped) == 1 {
					if s, ok := wrapped["output"].(string); ok {
						content = s
					}
				}
				results = append(results, map[string]any{"role": "tool", "tool_call_id": id, "content": content})
			case p.InlineData != nil:
				if strings.HasPrefix(p.InlineData.MimeType, "image/") {
					parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + p.InlineData.MimeType + ";base64," + p.InlineData.Data}})
				} else {
					parts = append(parts, map[string]any{"type": "text", "text": fmt.Sprintf("[附件 %s，%d 字节（base64）]", p.InlineData.MimeType, len(p.InlineData.Data))})
				}
			case p.FileData != nil:
				parts = append(parts, map[string]any{"type": "text", "text": "[文件 " + p.FileData.FileURI + "]"})
			case p.Text != "" && !p.Thought:
				parts = append(parts, map[string]any{"type": "text", "text": p.Text})
			}
		}
		messages = append(messages, results...)
		if len(parts) > 0 {
			messages = append(messages, map[string]any{"role": "user", "content": parts})
		}
	}
	out := map[string]any{"model": model, "messages": messages, "stream": stream}
	if stream {
		out["stream_options"] = map[string]any{"include_usage": true}
	}
	tools := []any{}
	for _, t := range req.Tools {
		for _, fd := range t.FunctionDeclarations {
			var params any = map[string]any{"type": "object", "properties": map[string]any{}}
			switch {
			case len(fd.ParametersJSONSchema) > 0:
				_ = json.Unmarshal(fd.ParametersJSONSchema, &params)
			case len(fd.Parameters) > 0:
				var raw any
				if json.Unmarshal(fd.Parameters, &raw) == nil {
					params = geminiSchema(raw)
				}
			}
			fn := map[string]any{"name": fd.Name, "parameters": params}
			if fd.Description != "" {
				fn["description"] = fd.Description
			}
			tools = append(tools, map[string]any{"type": "function", "function": fn})
		}
	}
	if len(tools) > 0 {
		out["tools"] = tools
		if req.ToolConfig != nil && req.ToolConfig.FunctionCallingConfig != nil {
			fc := req.ToolConfig.FunctionCallingConfig
			switch strings.ToUpper(fc.Mode) {
			case "ANY":
				if len(fc.Allowed) == 1 {
					out["tool_choice"] = map[string]any{"type": "function", "function": map[string]any{"name": fc.Allowed[0]}}
				} else {
					out["tool_choice"] = "required"
				}
			case "NONE":
				out["tool_choice"] = "none"
			}
		}
	}
	if g := req.GenerationConfig; g != nil {
		if g.Temperature != nil {
			out["temperature"] = *g.Temperature
		}
		if g.TopP != nil {
			out["top_p"] = *g.TopP
		}
		if g.MaxOutputTokens > 0 {
			out["max_tokens"] = g.MaxOutputTokens
		}
		if len(g.StopSequences) > 0 {
			out["stop"] = g.StopSequences
		}
	}
	return out, nil
}

// serveGeminiEndpoint 处理 Gemini 协议请求
func (rs *RouterService) serveGeminiEndpoint(w http.ResponseWriter, r *http.Request, route APIRoute, model, method string) {
	switch method {
	case "list":
		models := []any{}
		for _, id := range routeModelIDs(route) {
			models = append(models, map[string]any{"name": "models/" + id, "displayName": id,
				"supportedGenerationMethods": []string{"generateContent", "streamGenerateContent", "countTokens"}})
		}
		writeJSON(w, http.StatusOK, map[string]any{"models": models})
		return
	case "countTokens":
		body, _ := io.ReadAll(io.LimitReader(r.Body, maxGatewayBodyBytes))
		writeJSON(w, http.StatusOK, map[string]any{"totalTokens": len(body) / 4})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGatewayBodyBytes))
	if err != nil {
		writeGeminiError(w, http.StatusBadRequest, "读取请求体失败")
		return
	}
	stream := method == "streamGenerateContent"
	chat, err := geminiRequestToChat(body, model, stream)
	if err != nil {
		writeGeminiError(w, http.StatusBadRequest, "请求不是有效的 Gemini 格式: "+err.Error())
		return
	}
	payload, _ := json.Marshal(chat)
	r2 := r.Clone(r.Context())
	r2.Body = io.NopCloser(bytes.NewReader(payload))
	r2.ContentLength = int64(len(payload))
	r2.Header.Set("Content-Type", "application/json")
	r2.Header.Del("Content-Length")
	gw := &geminiWriter{w: w, header: http.Header{}, stream: stream, sse: r.URL.Query().Get("alt") == "sse", model: model}
	rs.serveOpenAIEndpoint(gw, r2, route)
	gw.close()
}

// routeModelIDs 路由能提供的模型名：模型映射的源名与默认模型
func routeModelIDs(route APIRoute) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if id = strings.TrimSpace(id); id != "" && id != "*" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	add(route.DefaultModel)
	for k := range route.ModelMapping {
		add(k)
	}
	return out
}

func writeGeminiError(w http.ResponseWriter, status int, msg string) {
	codes := map[int]string{400: "INVALID_ARGUMENT", 401: "UNAUTHENTICATED", 403: "PERMISSION_DENIED", 404: "NOT_FOUND", 429: "RESOURCE_EXHAUSTED", 500: "INTERNAL", 502: "UNAVAILABLE", 503: "UNAVAILABLE", 504: "DEADLINE_EXCEEDED"}
	st := codes[status]
	if st == "" {
		st = "UNKNOWN"
	}
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": status, "message": msg, "status": st}})
}

// geminiWriter 接住 Chat 入口写出的回复，转成 Gemini 格式写给调用方
type geminiWriter struct {
	w      http.ResponseWriter
	header http.Header
	stream bool
	sse    bool
	model  string
	status int
	buf    bytes.Buffer // 非流式回复或错误
	line   bytes.Buffer // 流式：未读完的一行
	begun  bool         // 流式：已向调用方写出响应头
	first  bool         // JSON 数组形式的流已写出第一个元素
	calls  map[int]*geminiCallAcc
	order  []int
	usage  map[string]any
	done   bool
	// 流式：Chat 已给出结束原因，等用量分块到齐后再发结尾
	pendingFinish string
}

type geminiCallAcc struct {
	id, name string
	args     strings.Builder
}

func (g *geminiWriter) Header() http.Header { return g.header }

func (g *geminiWriter) WriteHeader(code int) {
	if g.status == 0 {
		g.status = code
	}
}

func (g *geminiWriter) streaming() bool {
	return g.stream && g.status == http.StatusOK && strings.Contains(g.header.Get("Content-Type"), "event-stream")
}

func (g *geminiWriter) Write(p []byte) (int, error) {
	if g.status == 0 {
		g.status = http.StatusOK
	}
	if !g.streaming() {
		g.buf.Write(p)
		return len(p), nil
	}
	g.begin()
	for _, b := range p {
		if b == '\n' {
			g.handleLine(strings.TrimSpace(g.line.String()))
			g.line.Reset()
			continue
		}
		g.line.WriteByte(b)
	}
	return len(p), nil
}

func (g *geminiWriter) Flush() {
	if g.begun {
		if f, ok := g.w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func (g *geminiWriter) begin() {
	if g.begun {
		return
	}
	g.begun = true
	if g.sse {
		g.w.Header().Set("Content-Type", "text/event-stream")
	} else {
		g.w.Header().Set("Content-Type", "application/json")
	}
	g.w.Header().Set("Cache-Control", "no-cache")
	g.w.WriteHeader(http.StatusOK)
	if !g.sse {
		_, _ = io.WriteString(g.w, "[")
	}
}

func (g *geminiWriter) emit(resp map[string]any) {
	b, _ := json.Marshal(resp)
	if g.sse {
		fmt.Fprintf(g.w, "data: %s\r\n\r\n", b)
	} else {
		if g.first {
			_, _ = io.WriteString(g.w, ",\r\n")
		}
		g.first = true
		_, _ = g.w.Write(b)
	}
	g.Flush()
}

func (g *geminiWriter) candidate(parts []any, finish string) map[string]any {
	c := map[string]any{"content": map[string]any{"role": "model", "parts": parts}, "index": 0}
	if finish != "" {
		c["finishReason"] = finish
	}
	resp := map[string]any{"candidates": []any{c}, "modelVersion": g.model}
	return resp
}

func geminiFinish(reason string) string {
	switch reason {
	case "length":
		return "MAX_TOKENS"
	case "content_filter":
		return "SAFETY"
	case "":
		return ""
	}
	return "STOP"
}

func geminiUsage(u map[string]any) map[string]any {
	if u == nil {
		return nil
	}
	num := func(m map[string]any, k string) int {
		f, _ := m[k].(float64)
		return int(f)
	}
	out := map[string]any{
		"promptTokenCount":     num(u, "prompt_tokens"),
		"candidatesTokenCount": num(u, "completion_tokens"),
		"totalTokenCount":      num(u, "total_tokens"),
	}
	if d, ok := u["prompt_tokens_details"].(map[string]any); ok && num(d, "cached_tokens") > 0 {
		out["cachedContentTokenCount"] = num(d, "cached_tokens")
	}
	if d, ok := u["completion_tokens_details"].(map[string]any); ok && num(d, "reasoning_tokens") > 0 {
		out["thoughtsTokenCount"] = num(d, "reasoning_tokens")
	}
	return out
}

func (g *geminiWriter) callParts() []any {
	parts := []any{}
	for _, i := range g.order {
		c := g.calls[i]
		var args any
		if json.Unmarshal([]byte(c.args.String()), &args) != nil || args == nil {
			args = map[string]any{}
		}
		parts = append(parts, map[string]any{"functionCall": map[string]any{"id": c.id, "name": c.name, "args": args}})
	}
	g.calls, g.order = nil, nil
	return parts
}

// handleLine 处理 Chat 流式回复的一行
func (g *geminiWriter) handleLine(line string) {
	if !strings.HasPrefix(line, "data:") || g.done {
		return
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "[DONE]" {
		g.finishStream("")
		return
	}
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				ToolCalls        []struct {
					Index    int    `json:"index"`
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage map[string]any `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(payload), &chunk) != nil {
		return
	}
	if chunk.Error != nil {
		g.emit(map[string]any{"error": map[string]any{"code": 500, "message": chunk.Error.Message, "status": "INTERNAL"}})
		g.done = true
		return
	}
	if chunk.Usage != nil {
		g.usage = chunk.Usage
	}
	for _, ch := range chunk.Choices {
		if ch.Delta.ReasoningContent != "" {
			g.emit(g.candidate([]any{map[string]any{"text": ch.Delta.ReasoningContent, "thought": true}}, ""))
		}
		if ch.Delta.Content != "" {
			g.emit(g.candidate([]any{map[string]any{"text": ch.Delta.Content}}, ""))
		}
		for _, tc := range ch.Delta.ToolCalls {
			if g.calls == nil {
				g.calls = map[int]*geminiCallAcc{}
			}
			c := g.calls[tc.Index]
			if c == nil {
				c = &geminiCallAcc{}
				g.calls[tc.Index] = c
				g.order = append(g.order, tc.Index)
			}
			if tc.ID != "" {
				c.id = tc.ID
			}
			if tc.Function.Name != "" {
				c.name = tc.Function.Name
			}
			c.args.WriteString(tc.Function.Arguments)
		}
		if ch.FinishReason != "" {
			parts := g.callParts()
			if len(parts) > 0 {
				g.emit(g.candidate(parts, ""))
			}
			g.pendingFinish = geminiFinish(ch.FinishReason)
		}
	}
}

func (g *geminiWriter) finishStream(reason string) {
	if g.done {
		return
	}
	g.done = true
	if parts := g.callParts(); len(parts) > 0 {
		g.emit(g.candidate(parts, ""))
	}
	if reason == "" {
		reason = g.pendingFinish
	}
	if reason == "" {
		reason = "STOP"
	}
	resp := g.candidate([]any{map[string]any{"text": ""}}, reason)
	if u := geminiUsage(g.usage); u != nil {
		resp["usageMetadata"] = u
	}
	g.emit(resp)
}

// close Chat 入口返回后：流式补上结尾，非流式与错误整体转换
func (g *geminiWriter) close() {
	if g.begun {
		if g.line.Len() > 0 {
			g.handleLine(strings.TrimSpace(g.line.String()))
			g.line.Reset()
		}
		g.finishStream("")
		if !g.sse {
			_, _ = io.WriteString(g.w, "]")
		}
		return
	}
	status := g.status
	if status == 0 {
		status = http.StatusBadGateway
	}
	if status != http.StatusOK {
		var e struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		msg := strings.TrimSpace(g.buf.String())
		if json.Unmarshal(g.buf.Bytes(), &e) == nil && e.Error != nil && e.Error.Message != "" {
			msg = e.Error.Message
		}
		writeGeminiError(g.w, status, msg)
		return
	}
	var resp struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Content          *string `json:"content"`
				ReasoningContent string  `json:"reasoning_content"`
				ToolCalls        []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage map[string]any `json:"usage"`
	}
	if err := json.Unmarshal(g.buf.Bytes(), &resp); err != nil || len(resp.Choices) == 0 {
		writeGeminiError(g.w, http.StatusBadGateway, "上游回复无法转换为 Gemini 格式")
		return
	}
	ch := resp.Choices[0]
	parts := []any{}
	if ch.Message.ReasoningContent != "" {
		parts = append(parts, map[string]any{"text": ch.Message.ReasoningContent, "thought": true})
	}
	if ch.Message.Content != nil && *ch.Message.Content != "" {
		parts = append(parts, map[string]any{"text": *ch.Message.Content})
	}
	for _, tc := range ch.Message.ToolCalls {
		var args any
		if json.Unmarshal([]byte(tc.Function.Arguments), &args) != nil || args == nil {
			args = map[string]any{}
		}
		parts = append(parts, map[string]any{"functionCall": map[string]any{"id": tc.ID, "name": tc.Function.Name, "args": args}})
	}
	if len(parts) == 0 {
		parts = append(parts, map[string]any{"text": ""})
	}
	out := g.candidate(parts, firstNonEmpty(geminiFinish(ch.FinishReason), "STOP"))
	if u := geminiUsage(resp.Usage); u != nil {
		out["usageMetadata"] = u
	}
	out["responseId"] = resp.ID
	if g.stream {
		// 要的是流式但上游给了完整回复：作为单个分块发出
		g.begin()
		g.emit(out)
		if !g.sse {
			_, _ = io.WriteString(g.w, "]")
		}
		return
	}
	writeJSON(g.w, http.StatusOK, out)
}
