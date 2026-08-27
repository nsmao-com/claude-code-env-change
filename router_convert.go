package main

// Anthropic Messages API 与 OpenAI Chat Completions API 之间的格式转换。
// 仅依赖标准库，所有结构体字段均使用 RawMessage 保留未知内容。

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ============ Anthropic 请求/响应结构 ============

type anthropicContentBlock struct {
	Type string `json:"type"`

	// text
	Text string `json:"text,omitempty"`

	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`

	// image
	Source *anthropicImageSource `json:"source,omitempty"`
}

type anthropicImageSource struct {
	Type      string `json:"type"` // base64 | url
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicRequest struct {
	Model         string             `json:"model"`
	MaxTokens     int                `json:"max_tokens,omitempty"`
	System        json.RawMessage    `json:"system,omitempty"`
	Messages      []anthropicMessage `json:"messages"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Tools         []anthropicTool    `json:"tools,omitempty"`
	ToolChoice    json.RawMessage    `json:"tool_choice,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Model        string                  `json:"model"`
	Content      []anthropicContentBlock `json:"content"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence *string                 `json:"stop_sequence"`
	Usage        anthropicUsage          `json:"usage"`
}

type anthropicErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// ============ OpenAI 请求/响应结构 ============

type openaiFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type openaiToolCall struct {
	Index    int               `json:"index,omitempty"`
	ID       string            `json:"id,omitempty"`
	Type     string            `json:"type,omitempty"`
	Function openaiFunctionCall `json:"function"`
}

type openaiContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *openaiImageURL `json:"image_url,omitempty"`
}

type openaiImageURL struct {
	URL string `json:"url"`
}

type openaiMessage struct {
	Role       string           `json:"role"`
	Content    json.RawMessage  `json:"content,omitempty"`
	Name       string           `json:"name,omitempty"`
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openaiToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type openaiTool struct {
	Type     string         `json:"type"`
	Function openaiToolSpec `json:"function"`
}

type openaiStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type openaiRequest struct {
	Model         string              `json:"model"`
	Messages      []openaiMessage     `json:"messages"`
	MaxTokens     *int                `json:"max_tokens,omitempty"`
	Temperature   *float64            `json:"temperature,omitempty"`
	TopP          *float64            `json:"top_p,omitempty"`
	Stop          []string            `json:"stop,omitempty"`
	Tools         []openaiTool        `json:"tools,omitempty"`
	ToolChoice    json.RawMessage     `json:"tool_choice,omitempty"`
	Stream        bool                `json:"stream,omitempty"`
	StreamOptions *openaiStreamOptions `json:"stream_options,omitempty"`
}

type openaiRespMessage struct {
	Role      string           `json:"role"`
	Content   json.RawMessage  `json:"content"`
	ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
}

type openaiResponseChoice struct {
	Index        int               `json:"index"`
	Message      openaiRespMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openaiResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []openaiResponseChoice `json:"choices"`
	Usage   *openaiUsage           `json:"usage,omitempty"`
}

// ============ 通用辅助 ============

func mustMarshalJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return data
}

func jsonString(s string) json.RawMessage {
	return json.RawMessage(strconv.Quote(s))
}

func rawToString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, true
	}
	return "", false
}

func rawToBlocks(raw json.RawMessage) ([]anthropicContentBlock, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var blocks []anthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		return blocks, true
	}
	return nil, false
}

func rawToParts(raw json.RawMessage) ([]openaiContentPart, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var parts []openaiContentPart
	if err := json.Unmarshal(raw, &parts); err == nil {
		return parts, true
	}
	return nil, false
}

func anthropicSystemToString(raw json.RawMessage) string {
	if s, ok := rawToString(raw); ok {
		return s
	}
	blocks, ok := rawToBlocks(raw)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func anthropicToolResultContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if s, ok := rawToString(raw); ok {
		return s
	}
	blocks, ok := rawToBlocks(raw)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func openAIFinishToAnthropicStop(reason string) string {
	switch reason {
	case "length":
		return "max_tokens"
	case "tool_calls", "function_call":
		return "tool_use"
	default:
		return "end_turn"
	}
}

func anthropicStopToOpenAIFinish(reason string) string {
	switch reason {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}

// ============ 请求转换：Anthropic → OpenAI ============

// anthropicToOpenAIMessages 把 Anthropic 消息列表转换为 OpenAI 消息列表。
// system → system 消息；assistant tool_use → tool_calls；user tool_result → role:"tool"。
func anthropicToOpenAIMessages(req anthropicRequest) []openaiMessage {
	result := make([]openaiMessage, 0, len(req.Messages)+1)

	if sys := anthropicSystemToString(req.System); sys != "" {
		result = append(result, openaiMessage{Role: "system", Content: jsonString(sys)})
	}

	for _, msg := range req.Messages {
		switch msg.Role {
		case "assistant":
			om := openaiMessage{Role: "assistant"}
			if s, ok := rawToString(msg.Content); ok {
				if s != "" {
					om.Content = jsonString(s)
				}
				result = append(result, om)
				continue
			}
			blocks, _ := rawToBlocks(msg.Content)
			var texts []string
			var toolCalls []openaiToolCall
			for _, b := range blocks {
				switch b.Type {
				case "text":
					if b.Text != "" {
						texts = append(texts, b.Text)
					}
				case "tool_use":
					args := b.Input
					if len(strings.TrimSpace(string(args))) == 0 || !json.Valid(args) {
						args = json.RawMessage("{}")
					}
					toolCalls = append(toolCalls, openaiToolCall{
						ID:   b.ID,
						Type: "function",
						Function: openaiFunctionCall{
							Name:      b.Name,
							Arguments: string(args),
						},
					})
				}
			}
			if content := strings.Join(texts, ""); content != "" {
				om.Content = jsonString(content)
			}
			om.ToolCalls = toolCalls
			result = append(result, om)

		case "user":
			if s, ok := rawToString(msg.Content); ok {
				result = append(result, openaiMessage{Role: "user", Content: jsonString(s)})
				continue
			}
			blocks, _ := rawToBlocks(msg.Content)

			var pending []openaiContentPart
			flushPending := func() {
				if len(pending) == 0 {
					return
				}
				if len(pending) == 1 && pending[0].Type == "text" {
					result = append(result, openaiMessage{Role: "user", Content: jsonString(pending[0].Text)})
				} else {
					result = append(result, openaiMessage{Role: "user", Content: mustMarshalJSON(pending)})
				}
				pending = nil
			}

			for _, b := range blocks {
				switch b.Type {
				case "text":
					if b.Text != "" {
						pending = append(pending, openaiContentPart{Type: "text", Text: b.Text})
					}
				case "image":
					if part := anthropicImageToOpenAIPart(b); part != nil {
						pending = append(pending, *part)
					}
				case "tool_result":
					// tool_result 必须作为独立 tool 消息，先冲刷累积的文本
					flushPending()
					result = append(result, openaiMessage{
						Role:       "tool",
						ToolCallID: b.ToolUseID,
						Content:    jsonString(anthropicToolResultContent(b.Content)),
					})
				}
			}
			flushPending()

		default:
			// 未知角色按 user 处理，尽量不丢消息
			if s, ok := rawToString(msg.Content); ok {
				result = append(result, openaiMessage{Role: msg.Role, Content: jsonString(s)})
			}
		}
	}

	return result
}

func anthropicImageToOpenAIPart(b anthropicContentBlock) *openaiContentPart {
	if b.Source == nil {
		return nil
	}
	switch b.Source.Type {
	case "base64":
		if b.Source.Data == "" {
			return nil
		}
		mediaType := b.Source.MediaType
		if mediaType == "" {
			mediaType = "image/png"
		}
		return &openaiContentPart{
			Type:     "image_url",
			ImageURL: &openaiImageURL{URL: fmt.Sprintf("data:%s;base64,%s", mediaType, b.Source.Data)},
		}
	case "url":
		if b.Source.URL == "" {
			return nil
		}
		return &openaiContentPart{Type: "image_url", ImageURL: &openaiImageURL{URL: b.Source.URL}}
	}
	return nil
}

func anthropicToolChoiceToOpenAI(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var tc struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &tc); err != nil {
		return nil
	}
	switch tc.Type {
	case "auto":
		return json.RawMessage(`"auto"`)
	case "any":
		return json.RawMessage(`"required"`)
	case "tool":
		if tc.Name != "" {
			return json.RawMessage(fmt.Sprintf(`{"type":"function","function":{"name":%s}}`, strconv.Quote(tc.Name)))
		}
	case "none":
		return json.RawMessage(`"none"`)
	}
	return nil
}

// anthropicRequestToOpenAI 完整请求转换（不含模型映射，模型由调用方处理）。
func anthropicRequestToOpenAI(req anthropicRequest, model string) openaiRequest {
	out := openaiRequest{
		Model:    model,
		Messages: anthropicToOpenAIMessages(req),
		Stream:   req.Stream,
	}
	if req.MaxTokens > 0 {
		out.MaxTokens = &req.MaxTokens
	}
	out.Temperature = req.Temperature
	out.TopP = req.TopP
	if len(req.StopSequences) > 0 {
		out.Stop = req.StopSequences
	}
	if len(req.Tools) > 0 {
		tools := make([]openaiTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			schema := t.InputSchema
			if len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object"}`)
			}
			tools = append(tools, openaiTool{
				Type: "function",
				Function: openaiToolSpec{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  schema,
				},
			})
		}
		out.Tools = tools
		out.ToolChoice = anthropicToolChoiceToOpenAI(req.ToolChoice)
	}
	if out.Stream {
		out.StreamOptions = &openaiStreamOptions{IncludeUsage: true}
	}
	return out
}

// ============ 请求转换：OpenAI → Anthropic ============

func openAIToolChoiceToAnthropic(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if s, ok := rawToString(raw); ok {
		switch s {
		case "auto":
			return json.RawMessage(`{"type":"auto"}`)
		case "required":
			return json.RawMessage(`{"type":"any"}`)
		case "none":
			return json.RawMessage(`{"type":"none"}`)
		}
		return nil
	}
	var tc struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &tc); err == nil && tc.Type == "function" && tc.Function.Name != "" {
		return json.RawMessage(fmt.Sprintf(`{"type":"tool","name":%s}`, strconv.Quote(tc.Function.Name)))
	}
	return nil
}

func dataURLToImageBlock(url string) *anthropicContentBlock {
	if strings.HasPrefix(url, "data:") {
		rest := url[len("data:"):]
		if semi := strings.Index(rest, ";base64,"); semi > 0 {
			return &anthropicContentBlock{
				Type: "image",
				Source: &anthropicImageSource{
					Type:      "base64",
					MediaType: rest[:semi],
					Data:      rest[semi+len(";base64,"):],
				},
			}
		}
	}
	return &anthropicContentBlock{
		Type:   "image",
		Source: &anthropicImageSource{Type: "url", URL: url},
	}
}

// openAIToAnthropicMessages 转换消息列表；连续的 tool 消息会合并进同一个 user 消息
//（Anthropic 要求一次 tool_use 的所有 tool_result 位于同一条 user 消息中）。
func openAIToAnthropicMessages(req openaiRequest) (system string, messages []anthropicMessage) {
	var sysParts []string
	var pendingToolResults []anthropicContentBlock

	flushTools := func() {
		if len(pendingToolResults) == 0 {
			return
		}
		messages = append(messages, anthropicMessage{
			Role:    "user",
			Content: mustMarshalJSON(pendingToolResults),
		})
		pendingToolResults = nil
	}

	for _, m := range req.Messages {
		switch m.Role {
		case "system", "developer":
			if s, ok := rawToString(m.Content); ok && s != "" {
				sysParts = append(sysParts, s)
			} else if len(m.Content) > 0 {
				sysParts = append(sysParts, string(m.Content))
			}

		case "tool":
			contentStr := ""
			if s, ok := rawToString(m.Content); ok {
				contentStr = s
			} else if len(m.Content) > 0 {
				contentStr = string(m.Content)
			}
			pendingToolResults = append(pendingToolResults, anthropicContentBlock{
				Type:     "tool_result",
				ToolUseID: m.ToolCallID,
				Content:  jsonString(contentStr),
			})

		case "assistant":
			flushTools()
			blocks := make([]anthropicContentBlock, 0, 2)
			if s, ok := rawToString(m.Content); ok && s != "" {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: s})
			} else if parts, ok := rawToParts(m.Content); ok {
				var texts []string
				for _, p := range parts {
					if p.Type == "text" && p.Text != "" {
						texts = append(texts, p.Text)
					}
				}
				if s := strings.Join(texts, ""); s != "" {
					blocks = append(blocks, anthropicContentBlock{Type: "text", Text: s})
				}
			}
			for _, tc := range m.ToolCalls {
				input := json.RawMessage(tc.Function.Arguments)
				if len(strings.TrimSpace(tc.Function.Arguments)) == 0 || !json.Valid(input) {
					input = json.RawMessage("{}")
				}
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			if len(blocks) == 0 {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: ""})
			}
			messages = append(messages, anthropicMessage{Role: "assistant", Content: mustMarshalJSON(blocks)})

		case "user":
			flushTools()
			if s, ok := rawToString(m.Content); ok {
				messages = append(messages, anthropicMessage{Role: "user", Content: jsonString(s)})
				continue
			}
			parts, ok := rawToParts(m.Content)
			if !ok {
				messages = append(messages, anthropicMessage{Role: "user", Content: jsonString(string(m.Content))})
				continue
			}
			blocks := make([]anthropicContentBlock, 0, len(parts))
			for _, p := range parts {
				switch p.Type {
				case "text":
					if p.Text != "" {
						blocks = append(blocks, anthropicContentBlock{Type: "text", Text: p.Text})
					}
				case "image_url":
					if p.ImageURL != nil && p.ImageURL.URL != "" {
						if block := dataURLToImageBlock(p.ImageURL.URL); block != nil {
							blocks = append(blocks, *block)
						}
					}
				}
			}
			if len(blocks) == 0 {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: ""})
			}
			messages = append(messages, anthropicMessage{Role: "user", Content: mustMarshalJSON(blocks)})

		default:
			flushTools()
			if s, ok := rawToString(m.Content); ok {
				messages = append(messages, anthropicMessage{Role: m.Role, Content: jsonString(s)})
			}
		}
	}
	flushTools()

	return strings.Join(sysParts, "\n\n"), messages
}

// openAIRequestToAnthropic 完整请求转换。Anthropic 必须提供 max_tokens，
// 上游未指定时使用默认值。
func openAIRequestToAnthropic(req openaiRequest, model string, defaultMaxTokens int) anthropicRequest {
	system, messages := openAIToAnthropicMessages(req)
	out := anthropicRequest{
		Model:    model,
		Messages: messages,
		Stream:   req.Stream,
	}
	if system != "" {
		out.System = jsonString(system)
	}
	maxTokens := defaultMaxTokens
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}
	out.MaxTokens = maxTokens
	out.Temperature = req.Temperature
	out.TopP = req.TopP
	if len(req.Stop) > 0 {
		out.StopSequences = req.Stop
	}
	if len(req.Tools) > 0 {
		tools := make([]anthropicTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			schema := t.Function.Parameters
			if len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object"}`)
			}
			tools = append(tools, anthropicTool{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				InputSchema: schema,
			})
		}
		out.Tools = tools
		out.ToolChoice = openAIToolChoiceToAnthropic(req.ToolChoice)
	}
	return out
}

// ============ 响应转换（非流式） ============

func openAIResponseToAnthropic(resp *openaiResponse, requestModel string) *anthropicResponse {
	out := &anthropicResponse{
		ID:         "msg_" + strings.TrimPrefix(resp.ID, "chatcmpl-"),
		Type:       "message",
		Role:       "assistant",
		Model:      requestModel,
		Content:    []anthropicContentBlock{},
		StopReason: "end_turn",
	}
	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		if s, ok := rawToString(choice.Message.Content); ok && s != "" {
			out.Content = append(out.Content, anthropicContentBlock{Type: "text", Text: s})
		}
		for _, tc := range choice.Message.ToolCalls {
			input := json.RawMessage(tc.Function.Arguments)
			if len(strings.TrimSpace(tc.Function.Arguments)) == 0 || !json.Valid(input) {
				input = json.RawMessage("{}")
			}
			out.Content = append(out.Content, anthropicContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			})
		}
		out.StopReason = openAIFinishToAnthropicStop(choice.FinishReason)
	}
	if out.ID == "msg_" {
		out.ID = fmt.Sprintf("msg_router_%d", time.Now().UnixMilli())
	}
	if resp.Usage != nil {
		out.Usage = anthropicUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		}
	}
	return out
}

func anthropicResponseToOpenAI(resp *anthropicResponse) *openaiResponse {
	out := &openaiResponse{
		ID:      "chatcmpl-" + strings.TrimPrefix(resp.ID, "msg_"),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   resp.Model,
	}
	choice := openaiResponseChoice{Index: 0, FinishReason: "stop"}
	choice.Message.Role = "assistant"

	var texts []string
	var toolCalls []openaiToolCall
	for _, b := range resp.Content {
		switch b.Type {
		case "text":
			texts = append(texts, b.Text)
		case "tool_use":
			input := b.Input
			if len(input) == 0 {
				input = json.RawMessage("{}")
			}
			toolCalls = append(toolCalls, openaiToolCall{
				ID:   b.ID,
				Type: "function",
				Function: openaiFunctionCall{
					Name:      b.Name,
					Arguments: string(input),
				},
			})
		}
	}
	content := strings.Join(texts, "")
	if content != "" || len(toolCalls) == 0 {
		choice.Message.Content = jsonString(content)
	}
	choice.Message.ToolCalls = toolCalls
	choice.FinishReason = anthropicStopToOpenAIFinish(resp.StopReason)
	out.Choices = []openaiResponseChoice{choice}

	if resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
		out.Usage = &openaiUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}
	return out
}

// ============ 流式转换 ============

type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func newSSEWriter(w http.ResponseWriter) (*sseWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("ResponseWriter 不支持流式输出")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	return &sseWriter{w: w, flusher: flusher}, nil
}

func (s *sseWriter) sendEvent(event string, payload any) {
	data, _ := json.Marshal(payload)
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data)
	s.flusher.Flush()
}

func (s *sseWriter) sendData(payload any) {
	data, _ := json.Marshal(payload)
	fmt.Fprintf(s.w, "data: %s\n\n", data)
	s.flusher.Flush()
}

// sseScanner 逐行读取上游 SSE data 帧
type sseScanner struct {
	reader *bufio.Reader
	done   bool
}

func newSSEScanner(body io.Reader) *sseScanner {
	return &sseScanner{reader: bufio.NewReaderSize(body, 64*1024)}
}

func (s *sseScanner) next() (string, bool) {
	for {
		line, err := s.reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(line[len("data:"):])
			if payload == "[DONE]" {
				return "", false
			}
			return payload, true
		}
		if err != nil {
			break
		}
	}
	return "", false
}

// ---- OpenAI chunks → Anthropic SSE（供 Claude Code 消费） ----

type anthropicStreamToolState struct {
	blockIndex  int
	id          string
	name        string
	started     bool
	pendingArgs strings.Builder
}

func convertOpenAIStreamToAnthropic(upstream io.Reader, w http.ResponseWriter, inboundModel string) error {
	sse, err := newSSEWriter(w)
	if err != nil {
		return err
	}

	state := struct {
		sentStart    bool
		textOpen     bool
		nextBlock    int
		tools        map[int]*anthropicStreamToolState
		finishReason string
		usage        anthropicUsage
	}{
		tools: map[int]*anthropicStreamToolState{},
	}
	closeTextBlock := func() {
		if state.textOpen {
			sse.sendEvent("content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
			state.textOpen = false
		}
	}
	ensureStart := func(id string) {
		if state.sentStart {
			return
		}
		if id == "" {
			id = fmt.Sprintf("router-%d", time.Now().UnixMilli())
		}
		sse.sendEvent("message_start", map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":            "msg_" + strings.TrimPrefix(id, "chatcmpl-"),
				"type":          "message",
				"role":          "assistant",
				"content":       []any{},
				"model":         inboundModel,
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage": map[string]any{
					"input_tokens":  state.usage.InputTokens,
					"output_tokens": 1,
				},
			},
		})
		sse.sendEvent("ping", map[string]any{"type": "ping"})
		state.sentStart = true
	}

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
			state.usage.InputTokens = chunk.Usage.PromptTokens
			state.usage.OutputTokens = chunk.Usage.CompletionTokens
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		delta := choice.Delta

		if delta.Content != "" {
			ensureStart(chunk.ID)
			if !state.textOpen {
				sse.sendEvent("content_block_start", map[string]any{
					"type":          "content_block_start",
					"index":         0,
					"content_block": map[string]any{"type": "text", "text": ""},
				})
				state.textOpen = true
				state.nextBlock = 1
			}
			sse.sendEvent("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]any{"type": "text_delta", "text": delta.Content},
			})
		}

		for _, tc := range delta.ToolCalls {
			tool, known := state.tools[tc.Index]
			if !known {
				tool = &anthropicStreamToolState{}
				state.tools[tc.Index] = tool
			}
			if tc.ID != "" {
				tool.id = tc.ID
			}
			if tc.Function.Name != "" {
				tool.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				tool.pendingArgs.WriteString(tc.Function.Arguments)
			}
			if !tool.started && tool.id != "" && tool.name != "" {
				ensureStart(chunk.ID)
				closeTextBlock()
				tool.blockIndex = state.nextBlock
				state.nextBlock++
				tool.started = true
				sse.sendEvent("content_block_start", map[string]any{
					"type":  "content_block_start",
					"index": tool.blockIndex,
					"content_block": map[string]any{
						"type":  "tool_use",
						"id":    tool.id,
						"name":  tool.name,
						"input": map[string]any{},
					},
				})
			}
			if tool.started && tool.pendingArgs.Len() > 0 {
				sse.sendEvent("content_block_delta", map[string]any{
					"type":  "content_block_delta",
					"index": tool.blockIndex,
					"delta": map[string]any{
						"type":         "input_json_delta",
						"partial_json": tool.pendingArgs.String(),
					},
				})
				tool.pendingArgs.Reset()
			}
		}

		if choice.FinishReason != "" {
			state.finishReason = openAIFinishToAnthropicStop(choice.FinishReason)
		}
	}

	// 收尾：保证事件序列完整
	if !state.sentStart {
		ensureStart("")
		if !state.textOpen {
			sse.sendEvent("content_block_start", map[string]any{
				"type":          "content_block_start",
				"index":         0,
				"content_block": map[string]any{"type": "text", "text": ""},
			})
			state.textOpen = true
		}
	}
	closeTextBlock()

	// 按 blockIndex 顺序关闭工具块
	toolList := make([]*anthropicStreamToolState, 0, len(state.tools))
	for _, tool := range state.tools {
		toolList = append(toolList, tool)
	}
	for i := 0; i < len(toolList); i++ {
		for j := i + 1; j < len(toolList); j++ {
			if toolList[j].blockIndex < toolList[i].blockIndex {
				toolList[i], toolList[j] = toolList[j], toolList[i]
			}
		}
	}
	for _, tool := range toolList {
		if tool.started {
			if tool.pendingArgs.Len() > 0 {
				sse.sendEvent("content_block_delta", map[string]any{
					"type":  "content_block_delta",
					"index": tool.blockIndex,
					"delta": map[string]any{
						"type":         "input_json_delta",
						"partial_json": tool.pendingArgs.String(),
					},
				})
				tool.pendingArgs.Reset()
			}
			sse.sendEvent("content_block_stop", map[string]any{
				"type":  "content_block_stop",
				"index": tool.blockIndex,
			})
		}
	}

	stopReason := state.finishReason
	if stopReason == "" {
		stopReason = "end_turn"
	}
	sse.sendEvent("message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil},
		"usage": map[string]any{"output_tokens": state.usage.OutputTokens},
	})
	sse.sendEvent("message_stop", map[string]any{"type": "message_stop"})
	return nil
}

// OpenAI 流式 chunk 结构
type openaiChunk struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []openaiChunkChoice `json:"choices"`
	Usage   *openaiUsage        `json:"usage,omitempty"`
}

type openaiChunkChoice struct {
	Index        int              `json:"index"`
	Delta        openaiChunkDelta `json:"delta"`
	FinishReason string           `json:"finish_reason,omitempty"`
}

type openaiChunkDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
}

// ---- Anthropic SSE → OpenAI chunks（供 Codex 等消费） ----

type anthropicStreamEvent struct {
	Type string `json:"type"`

	Message *anthropicResponse `json:"message,omitempty"`

	Index        int                   `json:"index"`
	ContentBlock *anthropicContentBlock `json:"content_block,omitempty"`
	Delta        *anthropicStreamDelta `json:"delta,omitempty"`
	Usage        *anthropicUsage       `json:"usage,omitempty"`
}

type anthropicStreamDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

type openaiStreamToolState struct {
	callIndex int
	id        string
	name      string
	started   bool
}

func convertAnthropicStreamToOpenAI(upstream io.Reader, w http.ResponseWriter, outboundModel string) error {
	sse, err := newSSEWriter(w)
	if err != nil {
		return err
	}

	chunkID := fmt.Sprintf("chatcmpl-router-%d", time.Now().UnixMilli())
	created := time.Now().Unix()
	emit := func(delta openaiChunkDelta, finish string, usage *openaiUsage) {
		chunk := openaiChunk{
			ID:      chunkID,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   outboundModel,
			Choices: []openaiChunkChoice{{
				Index:        0,
				Delta:        delta,
				FinishReason: finish,
			}},
			Usage: usage,
		}
		sse.sendData(chunk)
	}

	sentRole := false
	nextToolIndex := 0
	tools := map[int]*openaiStreamToolState{}
	finish := ""
	var usage *openaiUsage

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
				chunkID = "chatcmpl-" + strings.TrimPrefix(event.Message.ID, "msg_")
			}
			if !sentRole {
				emit(openaiChunkDelta{Role: "assistant", Content: ""}, "", nil)
				sentRole = true
			}

		case "content_block_start":
			if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
				tools[event.Index] = &openaiStreamToolState{
					callIndex: nextToolIndex,
					id:        event.ContentBlock.ID,
					name:      event.ContentBlock.Name,
				}
				nextToolIndex++
			}

		case "content_block_delta":
			if event.Delta == nil {
				break
			}
			switch event.Delta.Type {
			case "text_delta":
				if !sentRole {
					emit(openaiChunkDelta{Role: "assistant", Content: ""}, "", nil)
					sentRole = true
				}
				emit(openaiChunkDelta{Content: event.Delta.Text}, "", nil)
			case "input_json_delta":
				tool, known := tools[event.Index]
				if !known {
					break
				}
				if !tool.started {
					emit(openaiChunkDelta{
						ToolCalls: []openaiToolCall{{
							Index:    tool.callIndex,
							ID:       tool.id,
							Type:     "function",
							Function: openaiFunctionCall{Name: tool.name},
						}},
					}, "", nil)
					tool.started = true
				}
				emit(openaiChunkDelta{
					ToolCalls: []openaiToolCall{{
						Index:    tool.callIndex,
						Function: openaiFunctionCall{Arguments: event.Delta.PartialJSON},
					}},
				}, "", nil)
			}

		case "message_delta":
			if event.Delta != nil && event.Delta.StopReason != "" {
				finish = anthropicStopToOpenAIFinish(event.Delta.StopReason)
			}
			if event.Usage != nil {
				usage = &openaiUsage{
					PromptTokens:     event.Usage.InputTokens,
					CompletionTokens: event.Usage.OutputTokens,
					TotalTokens:      event.Usage.InputTokens + event.Usage.OutputTokens,
				}
			}

		case "message_stop":
			// 循环结束后统一收尾
		}
	}

	if !sentRole {
		emit(openaiChunkDelta{Role: "assistant", Content: ""}, "", nil)
	}
	if finish == "" {
		finish = "stop"
	}
	emit(openaiChunkDelta{}, finish, usage)
	fmt.Fprintf(sse.w, "data: [DONE]\n\n")
	sse.flusher.Flush()
	return nil
}
