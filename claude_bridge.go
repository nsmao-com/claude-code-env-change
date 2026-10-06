package main

// Claude 订阅上游（account://claude）
//
// Anthropic 会按请求内容判断是不是 Claude Code 本身发出的，其它客户端重放 OAuth 令牌的请求
// 会被算进额外用量甚至拒绝。所以这里不直接请求 API，而是驱动本机已登录的 claude 命令行
// （-p + stream-json），调用方的工具经本程序的 `mcp claude-bridge` 子命令（stdio MCP）接入：
// Claude 调用工具时，网关把 tool_use 回给调用方，该 claude 进程原地等待；
// 调用方带着 tool_result 发来下一个请求时，结果交回同一进程，继续生成下一段回复。
// 思路参考 yetone/magpie 与 pi-claude-bridge（MIT）。

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	claudeBridgeServer = "aienv"
	claudeToolPrefix   = "mcp__" + claudeBridgeServer + "__"
	claudeBridgePrefix = "_claude"
	claudeParkLongest  = 10 * time.Minute // 等调用方送回工具结果的最长时间
	claudeRunLongest   = 2 * time.Hour    // 单个 claude 进程最长存活时间
	// 一轮答完后进程留着等这段对话的下一轮：只送新消息，前面的内容能命中 Anthropic 的缓存（1 小时）
	claudeIdleLongest = time.Hour
	claudeIdleMost    = 6
)

type claudeBridge struct {
	mu    sync.Mutex
	runs  map[string]*claudeRun // 回调令牌 → 进程
	calls map[string]*claudeRun // 等待结果的 tool_use ID → 进程
	idle  map[string]*claudeRun // 对话至今的摘要 → 答完一轮、等下一轮的进程
}

var globalClaudeBridge = &claudeBridge{runs: map[string]*claudeRun{}, calls: map[string]*claudeRun{}, idle: map[string]*claudeRun{}}

// claudeToolResult MCP tools/call 的结果
type claudeToolResult struct {
	Content []map[string]any `json:"content"`
	IsError bool             `json:"isError,omitempty"`
}

type claudeFailure struct {
	status  int
	message string
}

// claudeEvent 发给调用方的一条 Anthropic SSE 事件，或一个错误
type claudeEvent struct {
	name string
	data []byte
	err  *claudeFailure
}

type claudeRun struct {
	bridge *claudeBridge
	token  string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	tmp    string
	done   chan struct{}
	model  string
	tools  string // 工具列表摘要：工具变了不能复用进程

	mu      sync.Mutex
	segment chan claudeEvent
	pending map[string]chan claudeToolResult
	early   map[string]claudeToolResult
	asked   []claudeAsked // 这段回复里调用的工具
	claimed map[string]bool
	park    *time.Timer
	life    *time.Timer
	idleKey string
	idleAt  time.Time
	// 写给 claude 的用户消息数与它答完的轮数：每轮结束都有一条 result，
	// 上一轮迟到的 result 不能结束下一轮
	turns   int
	results int
	closed  bool
	stderr  strings.Builder
	// 本段回复所答的对话（调用方请求里的消息）与回复本身，用来算下一轮的复用键
	msgs  []cbMessage
	reply strings.Builder
	calls []string
}

type claudeAsked struct{ id, name string }

// ===== 请求解析 =====

type cbRequest struct {
	Model        string            `json:"model"`
	System       json.RawMessage   `json:"system"`
	Messages     []cbMessage       `json:"messages"`
	Tools        []json.RawMessage `json:"tools"`
	ToolChoice   json.RawMessage   `json:"tool_choice"`
	Stream       bool              `json:"stream"`
	OutputConfig *struct {
		Effort string `json:"effort"`
	} `json:"output_config"`
}

type cbMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type cbBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
	Source    json.RawMessage `json:"source"`
	raw       json.RawMessage
}

func (m cbMessage) blocks() []cbBlock {
	var s string
	if json.Unmarshal(m.Content, &s) == nil {
		return []cbBlock{{Type: "text", Text: s}}
	}
	var raws []json.RawMessage
	_ = json.Unmarshal(m.Content, &raws)
	out := make([]cbBlock, 0, len(raws))
	for _, r := range raws {
		var b cbBlock
		if json.Unmarshal(r, &b) == nil {
			b.raw = r
			out = append(out, b)
		}
	}
	return out
}

// claudeBillingHeader Claude Code 放在自己系统提示词第一行的计费标记，转给 claude 时去掉
var claudeBillingHeader = regexp.MustCompile(`^x-anthropic-billing-header:(\s*[A-Za-z_]+=[^;\s]*;)*\s*`)

func (in *cbRequest) systemText() string {
	var s string
	if json.Unmarshal(in.System, &s) != nil {
		var blocks []cbBlock
		_ = json.Unmarshal(in.System, &blocks)
		parts := make([]string, 0, len(blocks))
		for _, b := range blocks {
			if t := claudeBillingHeader.ReplaceAllString(b.Text, ""); strings.TrimSpace(t) != "" {
				parts = append(parts, t)
			}
		}
		s = strings.Join(parts, "\n\n")
	}
	return strings.TrimSpace(claudeBillingHeader.ReplaceAllString(s, ""))
}

// toolChoice 返回 auto / any / none / tool:<名称>
func (in *cbRequest) toolChoice() string {
	var c struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	_ = json.Unmarshal(in.ToolChoice, &c)
	if c.Type == "tool" && c.Name != "" {
		return "tool:" + c.Name
	}
	return c.Type
}

// bridgeTools 调用方的工具转成 MCP 工具；服务端工具（没有 input_schema 的）不转
func (in *cbRequest) bridgeTools() []map[string]any {
	if in.toolChoice() == "none" {
		return []map[string]any{}
	}
	out := []map[string]any{}
	for _, raw := range in.Tools {
		var t struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"input_schema"`
		}
		if json.Unmarshal(raw, &t) != nil || t.Name == "" || len(t.InputSchema) == 0 {
			continue
		}
		tool := map[string]any{"name": t.Name, "inputSchema": t.InputSchema}
		if t.Description != "" {
			tool["description"] = t.Description
		}
		out = append(out, tool)
	}
	return out
}

// renderPrompt 把整段对话写成给 claude 的一条用户消息：调用方的系统提示词作为一块单独缓存，
// 之后按“Human: / Assistant:”逐条写出，工具调用与结果写成文字，图片保留为图片块
func (in *cbRequest) renderPrompt() []map[string]any {
	var blocks []map[string]any
	var text strings.Builder
	system := in.systemText()
	choice := in.toolChoice()
	if system != "" || choice == "any" || strings.HasPrefix(choice, "tool:") {
		text.WriteString("<external_system_instructions>\n")
		text.WriteString(system)
		switch {
		case choice == "any":
			text.WriteString("\nYou must call at least one available tool before answering.")
		case strings.HasPrefix(choice, "tool:"):
			fmt.Fprintf(&text, "\nYou must call the %s tool.", strings.TrimPrefix(choice, "tool:"))
		}
		text.WriteString("\n</external_system_instructions>\n\n")
		blocks = append(blocks, map[string]any{"type": "text", "text": text.String(), "cache_control": map[string]string{"type": "ephemeral", "ttl": "1h"}})
		text.Reset()
	}
	for _, m := range in.Messages {
		label := "Human"
		if m.Role == "assistant" {
			label = "Assistant"
		}
		text.WriteString(label + ": ")
		blocks = renderClaudeBlocks(blocks, &text, m.blocks())
		text.WriteString("\n\n")
	}
	if text.Len() > 0 {
		blocks = append(blocks, map[string]any{"type": "text", "text": text.String()})
	}
	if len(blocks) == 0 {
		blocks = append(blocks, map[string]any{"type": "text", "text": "[continue]"})
	}
	return blocks
}

func renderClaudeBlocks(blocks []map[string]any, text *strings.Builder, parts []cbBlock) []map[string]any {
	flush := func() {
		if text.Len() > 0 {
			blocks = append(blocks, map[string]any{"type": "text", "text": text.String()})
			text.Reset()
		}
	}
	for _, p := range parts {
		switch p.Type {
		case "text":
			text.WriteString(p.Text)
		case "thinking":
			text.WriteString(p.Thinking)
		case "tool_use", "server_tool_use":
			args := strings.TrimSpace(string(p.Input))
			if args == "" {
				args = "{}"
			}
			fmt.Fprintf(text, "\n[tool call %s id=%s args=%s]", p.Name, p.ID, args)
		case "tool_result":
			mark := ""
			if p.IsError {
				mark = " error"
			}
			fmt.Fprintf(text, "\n[tool result id=%s%s]\n", p.ToolUseID, mark)
			var s string
			if json.Unmarshal(p.Content, &s) == nil {
				text.WriteString(s)
				continue
			}
			var inner []cbBlock
			_ = json.Unmarshal(p.Content, &inner)
			for _, b := range inner {
				switch b.Type {
				case "text":
					text.WriteString(b.Text)
				case "image":
					flush()
					blocks = append(blocks, map[string]any{"type": "image", "source": json.RawMessage(b.Source)})
				}
			}
		case "image", "document":
			flush()
			var block map[string]any
			if json.Unmarshal(p.raw, &block) == nil {
				delete(block, "cache_control")
				blocks = append(blocks, block)
			}
		}
	}
	return blocks
}

// ===== 入口 =====

// roundTrip 把网关发往 account://claude 的请求交给本机 claude 处理
func (b *claudeBridge) roundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(io.LimitReader(req.Body, 64<<20))
		req.Body.Close()
	}
	path := req.URL.Path
	switch {
	case strings.HasSuffix(path, "/models"):
		return claudeModelsResponse(req.Context()), nil
	case strings.HasSuffix(path, "/count_tokens"):
		return claudeJSONResponse(http.StatusOK, map[string]any{"input_tokens": len(body) / 4}), nil
	case !strings.HasSuffix(path, "/messages"):
		return claudeErrorResponse(&claudeFailure{http.StatusBadRequest, "Claude 订阅只支持 Anthropic Messages 协议：请把该路由的上游格式设为 Anthropic Messages"}), nil
	}
	var in cbRequest
	if err := json.Unmarshal(body, &in); err != nil || len(in.Messages) == 0 {
		return claudeErrorResponse(&claudeFailure{http.StatusBadRequest, "请求体不是有效的 Anthropic Messages 请求"}), nil
	}
	run, seg := b.resume(&in)
	if run == nil {
		run, seg = b.reuse(&in)
	}
	if run == nil {
		var err error
		run, seg, err = b.start(&in)
		if err != nil {
			return claudeErrorResponse(&claudeFailure{http.StatusBadGateway, err.Error()}), nil
		}
	}
	return claudeRespond(req.Context(), run, seg, in.Stream), nil
}

// claudeExecutable 本机 claude；npm 安装的是脚本外壳时，尽量直接用它包装的 claude.exe
func claudeExecutable() (string, error) {
	bin := findCliBinary("claude")
	if bin == "" {
		return "", errors.New("没有找到 Claude Code（claude 命令）：请先安装并登录（claude 后执行 /login）")
	}
	if ext := strings.ToLower(filepath.Ext(bin)); ext == ".cmd" || ext == ".ps1" || ext == "" {
		exe := filepath.Join(filepath.Dir(bin), "node_modules", "@anthropic-ai", "claude-code", "bin", "claude.exe")
		if fi, err := os.Stat(exe); err == nil && !fi.IsDir() {
			return exe, nil
		}
	}
	return bin, nil
}

func claudeRandomToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// claudeWorkDir 所有进程共用的空工作目录：不在任何 git 仓库里，提示词前缀稳定，便于命中缓存
func claudeWorkDir() string {
	dir := filepath.Join(os.TempDir(), "aienv-claude-work")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return os.TempDir()
	}
	return dir
}

// claudeRunEnv claude 进程的环境：去掉会让它改用第三方 Key / 网关的变量，关掉记忆、
// 自动压缩、工具延迟加载等会改变提示词或拖慢回复的功能
func claudeRunEnv() []string {
	blocked := map[string]bool{
		"CLAUDE_CODE_PROMPT_CACHE_TTL": true, "CLAUDE_CODE_EXTRA_BODY": true, "ANTHROPIC_BETAS": true,
		"ANTHROPIC_DEFAULT_OPUS_MODEL": true, "ANTHROPIC_DEFAULT_SONNET_MODEL": true, "ANTHROPIC_DEFAULT_HAIKU_MODEL": true,
		"ANTHROPIC_SMALL_FAST_MODEL": true, "ANTHROPIC_CUSTOM_HEADERS": true,
	}
	env := make([]string, 0, len(os.Environ())+16)
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		up := strings.ToUpper(k)
		if claudeBlockedEnv[up] || blocked[up] {
			continue
		}
		env = append(env, kv)
	}
	outboundProxyMu.RLock()
	proxy := outboundProxyCurrent
	outboundProxyMu.RUnlock()
	if proxy.Enabled && (strings.HasPrefix(proxy.URL, "http://") || strings.HasPrefix(proxy.URL, "https://")) {
		env = append(env, "HTTPS_PROXY="+proxy.URL, "HTTP_PROXY="+proxy.URL)
	}
	return append(env,
		"ENABLE_CLAUDEAI_MCP_SERVERS=0", "DISABLE_AUTO_COMPACT=1", "ENABLE_TOOL_SEARCH=false",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1",
		"CLAUDE_CODE_DISABLE_CLAUDE_MDS=1", "CLAUDE_CODE_DISABLE_GIT_INSTRUCTIONS=1",
		"CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK=1", "CLAUDE_CODE_PROMPT_CACHE_TTL=1h",
		"MCP_TOOL_TIMEOUT=900000", "MCP_TIMEOUT=30000",
		"DISABLE_TELEMETRY=1", "DISABLE_ERROR_REPORTING=1", "DISABLE_AUTOUPDATER=1")
}

// start 为一个新对话启动 claude 进程，送入整段对话
func (b *claudeBridge) start(in *cbRequest) (*claudeRun, <-chan claudeEvent, error) {
	bin, err := claudeExecutable()
	if err != nil {
		return nil, nil, err
	}
	self, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}
	if globalRouterService == nil {
		return nil, nil, errors.New("网关未启动")
	}
	tmp, err := os.MkdirTemp("", "aienv-claude-")
	if err != nil {
		return nil, nil, err
	}
	toolsPath := filepath.Join(tmp, "tools.json")
	toolBytes, _ := json.Marshal(in.bridgeTools())
	token := claudeRandomToken()
	callback := fmt.Sprintf("http://127.0.0.1:%d/%s/%s", routerPort(globalRouterService), claudeBridgePrefix, token)
	mcpConfig, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{
		claudeBridgeServer: map[string]any{"type": "stdio", "command": self, "args": []string{"mcp", "claude-bridge", callback, toolsPath}},
	}})
	mcpPath := filepath.Join(tmp, "mcp.json")
	if err := os.WriteFile(toolsPath, toolBytes, 0o600); err != nil {
		_ = os.RemoveAll(tmp)
		return nil, nil, err
	}
	if err := os.WriteFile(mcpPath, mcpConfig, 0o600); err != nil {
		_ = os.RemoveAll(tmp)
		return nil, nil, err
	}
	model := strings.TrimSpace(in.Model)
	if model == "" {
		model = "sonnet"
	}
	args := []string{
		"-p", "--output-format", "stream-json", "--input-format", "stream-json",
		"--include-partial-messages", "--verbose", "--model", model,
		"--tools", "", "--strict-mcp-config", "--mcp-config", mcpPath,
		"--allowedTools", "mcp__" + claudeBridgeServer,
		"--setting-sources", "", "--no-session-persistence",
	}
	if in.OutputConfig != nil {
		switch e := strings.ToLower(in.OutputConfig.Effort); e {
		case "low", "medium", "high", "xhigh", "max":
			args = append(args, "--effort", e)
		}
	}
	var cmd *exec.Cmd
	if ext := strings.ToLower(filepath.Ext(bin)); ext == ".cmd" || ext == ".bat" {
		cmd = startToolCommand(context.Background(), bin, args, nil)
	} else {
		cmd = exec.Command(bin, args...)
		configureHiddenCmd(cmd)
	}
	cmd.Env = claudeRunEnv()
	cmd.Dir = claudeWorkDir()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = os.RemoveAll(tmp)
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = os.RemoveAll(tmp)
		return nil, nil, err
	}
	run := &claudeRun{
		bridge: b, token: token, cmd: cmd, stdin: stdin, tmp: tmp, done: make(chan struct{}),
		model: in.Model, tools: claudeToolsKey(in), msgs: in.Messages, turns: 1,
		pending: map[string]chan claudeToolResult{}, early: map[string]claudeToolResult{}, claimed: map[string]bool{},
	}
	cmd.Stderr = &claudeStderr{run: run}
	seg := run.attach()
	b.mu.Lock()
	b.runs[token] = run
	b.mu.Unlock()
	if err := cmd.Start(); err != nil {
		run.close()
		return nil, nil, fmt.Errorf("启动 Claude Code 失败: %v", err)
	}
	run.life = time.AfterFunc(claudeRunLongest, run.abort)
	go func() {
		run.readOutput(stdout)
		_ = cmd.Wait()
		run.close()
	}()
	line, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": in.renderPrompt()}})
	if _, err := stdin.Write(append(line, '\n')); err != nil {
		run.abort()
		return nil, nil, fmt.Errorf("向 Claude Code 发送对话失败: %v", err)
	}
	return run, seg, nil
}

// resume 请求最后一条是工具结果、且对应的调用正由某个进程等待时，把结果交给它继续
func (b *claudeBridge) resume(in *cbRequest) (*claudeRun, <-chan claudeEvent) {
	last := in.Messages[len(in.Messages)-1]
	if last.Role != "user" {
		return nil, nil
	}
	results := map[string]cbBlock{}
	for _, blk := range last.blocks() {
		if blk.Type == "tool_result" && blk.ToolUseID != "" {
			results[blk.ToolUseID] = blk
		}
	}
	if len(results) == 0 {
		return nil, nil
	}
	b.mu.Lock()
	var run *claudeRun
	for id := range results {
		if r := b.calls[id]; r != nil {
			run = r
			break
		}
	}
	if run != nil {
		for id, r := range b.calls {
			if r == run {
				delete(b.calls, id)
			}
		}
	}
	b.mu.Unlock()
	if run == nil {
		return nil, nil
	}
	run.mu.Lock()
	if run.closed || run.segment != nil {
		run.mu.Unlock()
		return nil, nil
	}
	if run.park != nil {
		run.park.Stop()
	}
	asked := run.asked
	run.asked = nil
	run.msgs = in.Messages
	run.mu.Unlock()
	seg := run.attach()
	for _, a := range asked {
		res, ok := results[a.id]
		if !ok {
			run.deliver(a.id, claudeToolResult{IsError: true, Content: []map[string]any{{"type": "text", "text": "The caller returned no result for this call."}}})
			continue
		}
		run.deliver(a.id, mcpResultOf(res))
	}
	return run, seg
}

// reuse 请求是某段已答完对话的下一轮时，交给留着的进程，只送新的消息
func (b *claudeBridge) reuse(in *cbRequest) (*claudeRun, <-chan claudeEvent) {
	last := -1
	for i := len(in.Messages) - 1; i >= 0; i-- {
		if in.Messages[i].Role == "assistant" {
			last = i
			break
		}
	}
	if last < 0 || last == len(in.Messages)-1 {
		return nil, nil
	}
	key := claudeConvKey(in.Model, claudeToolsKey(in), in.Messages[:last+1])
	b.mu.Lock()
	run := b.idle[key]
	delete(b.idle, key)
	b.mu.Unlock()
	if run == nil {
		return nil, nil
	}
	run.mu.Lock()
	if run.closed || run.segment != nil {
		run.mu.Unlock()
		return nil, nil
	}
	if run.park != nil {
		run.park.Stop()
	}
	run.idleKey = ""
	run.msgs = in.Messages
	run.turns++
	run.mu.Unlock()
	seg := run.attach()
	line, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": renderClaudeTurn(in.Messages[last+1:])}})
	if _, err := run.stdin.Write(append(line, '\n')); err != nil {
		run.abort()
		return nil, nil
	}
	return run, seg
}

// renderClaudeTurn 留着的进程已经有前面的对话，只告诉它之后的新消息
func renderClaudeTurn(msgs []cbMessage) []map[string]any {
	var blocks []map[string]any
	var text strings.Builder
	labeled := false
	for _, m := range msgs {
		if m.Role == "assistant" {
			labeled = true
		}
	}
	for i, m := range msgs {
		if i > 0 {
			text.WriteString("\n\n")
		}
		if labeled {
			if m.Role == "assistant" {
				text.WriteString("Assistant: ")
			} else {
				text.WriteString("Human: ")
			}
		}
		blocks = renderClaudeBlocks(blocks, &text, m.blocks())
	}
	if text.Len() > 0 {
		blocks = append(blocks, map[string]any{"type": "text", "text": text.String()})
	}
	if len(blocks) == 0 {
		blocks = append(blocks, map[string]any{"type": "text", "text": "[continue]"})
	}
	return blocks
}

// keepIdle 一轮答完：进程留着等这段对话的下一轮，最多留 claudeIdleMost 个、各留一小时
func (b *claudeBridge) keepIdle(r *claudeRun) {
	r.mu.Lock()
	reply := cbMessage{Role: "assistant"}
	reply.Content, _ = json.Marshal(r.replyBlocks())
	msgs := append(append([]cbMessage(nil), r.msgs...), reply)
	key := claudeConvKey(r.model, r.tools, msgs)
	r.idleKey, r.idleAt = key, time.Now()
	r.park = time.AfterFunc(claudeIdleLongest, r.abort)
	r.mu.Unlock()
	b.mu.Lock()
	if old := b.idle[key]; old != nil && old != r {
		go old.abort()
	}
	b.idle[key] = r
	var oldest *claudeRun
	if len(b.idle) > claudeIdleMost {
		for _, run := range b.idle {
			if oldest == nil || run.idleAt.Before(oldest.idleAt) {
				oldest = run
			}
		}
	}
	b.mu.Unlock()
	if oldest != nil {
		go oldest.abort()
	}
}

// replyBlocks mu 已持有：这段回复里发给调用方的文字与工具调用
func (r *claudeRun) replyBlocks() []map[string]any {
	out := []map[string]any{}
	if t := r.reply.String(); t != "" {
		out = append(out, map[string]any{"type": "text", "text": t})
	}
	for _, id := range r.calls {
		out = append(out, map[string]any{"type": "tool_use", "id": id})
	}
	return out
}

// claudeConvKey 对话的摘要：只看角色、文字、工具调用与结果的 ID，调用方去掉思考块、
// 拆分文字块都不影响
func claudeConvKey(model, tools string, msgs []cbMessage) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00", model, tools)
	for _, m := range msgs {
		fmt.Fprintf(h, "\x01%s\x00", m.Role)
		var text strings.Builder
		for _, b := range m.blocks() {
			switch b.Type {
			case "text":
				text.WriteString(b.Text)
			case "tool_use":
				fmt.Fprintf(&text, "\x02use:%s", b.ID)
			case "tool_result":
				fmt.Fprintf(&text, "\x02result:%s:%x", b.ToolUseID, sha256.Sum256(b.Content))
			case "image", "document":
				fmt.Fprintf(&text, "\x02%s:%x", b.Type, sha256.Sum256(b.Source))
			}
		}
		h.Write([]byte(strings.TrimSpace(text.String())))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func claudeToolsKey(in *cbRequest) string {
	b, _ := json.Marshal(in.bridgeTools())
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// mcpResultOf 调用方的 tool_result 转成 MCP 结果
func mcpResultOf(b cbBlock) claudeToolResult {
	out := claudeToolResult{IsError: b.IsError, Content: []map[string]any{}}
	var s string
	if json.Unmarshal(b.Content, &s) == nil {
		out.Content = append(out.Content, map[string]any{"type": "text", "text": s})
	} else {
		var inner []cbBlock
		_ = json.Unmarshal(b.Content, &inner)
		for _, p := range inner {
			switch p.Type {
			case "text":
				out.Content = append(out.Content, map[string]any{"type": "text", "text": p.Text})
			case "image":
				var src struct {
					Type      string `json:"type"`
					MediaType string `json:"media_type"`
					Data      string `json:"data"`
					URL       string `json:"url"`
				}
				_ = json.Unmarshal(p.Source, &src)
				if src.Data != "" {
					out.Content = append(out.Content, map[string]any{"type": "image", "data": src.Data, "mimeType": src.MediaType})
				} else if src.URL != "" {
					out.Content = append(out.Content, map[string]any{"type": "text", "text": src.URL})
				}
			default:
				if len(p.raw) > 0 {
					out.Content = append(out.Content, map[string]any{"type": "text", "text": string(p.raw)})
				}
			}
		}
	}
	if len(out.Content) == 0 {
		out.Content = append(out.Content, map[string]any{"type": "text", "text": "(no output)"})
	}
	return out
}

// ===== 进程 =====

type claudeStderr struct{ run *claudeRun }

func (w *claudeStderr) Write(p []byte) (int, error) {
	w.run.mu.Lock()
	if w.run.stderr.Len() < 64<<10 {
		w.run.stderr.Write(p)
	}
	w.run.mu.Unlock()
	return len(p), nil
}

func (r *claudeRun) attach() chan claudeEvent {
	ch := make(chan claudeEvent, 256)
	r.mu.Lock()
	r.segment = ch
	r.mu.Unlock()
	return ch
}

func (r *claudeRun) emit(ev claudeEvent) {
	r.mu.Lock()
	ch := r.segment
	r.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- ev:
		return
	default:
	}
	select {
	case ch <- ev:
	case <-r.done:
	}
}

// endWith 先从进程上摘下这段回复，再送出最后一条事件：调用方收到结尾时，
// 进程已经处在等工具结果 / 等下一轮的状态，紧接着发来的请求能找到它
func (r *claudeRun) endWith(ev claudeEvent) {
	r.mu.Lock()
	ch := r.segment
	r.segment = nil
	r.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- ev:
	case <-r.done:
	}
	close(ch)
}

func (r *claudeRun) endSegment() {
	r.mu.Lock()
	ch := r.segment
	r.segment = nil
	r.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

func (r *claudeRun) fail(status int, msg string) {
	if status == 0 {
		status = http.StatusBadGateway
	}
	r.emit(claudeEvent{err: &claudeFailure{status, msg}})
	r.endSegment()
}

// parkNow 这段回复以调用方的工具调用结束：登记这些调用，等调用方送回结果
func (r *claudeRun) parkNow() {
	r.mu.Lock()
	asked := append([]claudeAsked(nil), r.asked...)
	r.park = time.AfterFunc(claudeParkLongest, r.abort)
	r.mu.Unlock()
	r.bridge.mu.Lock()
	for _, a := range asked {
		r.bridge.calls[a.id] = r
	}
	r.bridge.mu.Unlock()
}

// deliver 把工具结果交给正在等待的 MCP 调用；调用还没到时先存起来
func (r *claudeRun) deliver(id string, res claudeToolResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ch := r.pending[id]; ch != nil {
		delete(r.pending, id)
		ch <- res
		return
	}
	r.early[id] = res
}

// finishTurn 这一轮已答完：关闭输入，claude 随之退出
func (r *claudeRun) finishTurn() {
	_ = r.stdin.Close()
}

func (r *claudeRun) abort() {
	killCmd(r.cmd)
	r.close()
}

func (r *claudeRun) close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	close(r.done)
	pending := r.pending
	r.pending = map[string]chan claudeToolResult{}
	if r.park != nil {
		r.park.Stop()
	}
	if r.life != nil {
		r.life.Stop()
	}
	msg := strings.TrimSpace(r.stderr.String())
	r.mu.Unlock()
	for _, ch := range pending {
		close(ch)
	}
	r.fail(http.StatusBadGateway, "Claude Code 已退出"+claudeTail(msg))
	_ = r.stdin.Close()
	b := r.bridge
	b.mu.Lock()
	delete(b.runs, r.token)
	for id, run := range b.calls {
		if run == r {
			delete(b.calls, id)
		}
	}
	for key, run := range b.idle {
		if run == r {
			delete(b.idle, key)
		}
	}
	b.mu.Unlock()
	go func(dir string) {
		time.Sleep(2 * time.Second) // 等子进程放开文件
		_ = os.RemoveAll(dir)
	}(r.tmp)
}

// abortAll 退出时结束所有 claude 进程
func (b *claudeBridge) abortAll() {
	b.mu.Lock()
	runs := make([]*claudeRun, 0, len(b.runs))
	for _, r := range b.runs {
		runs = append(runs, r)
	}
	b.mu.Unlock()
	for _, r := range runs {
		r.abort()
	}
}

func claudeTail(msg string) string {
	if msg == "" {
		return ""
	}
	return "：" + clipText(msg, 400)
}

func sseEvent(name string, v any) claudeEvent {
	b, _ := json.Marshal(v)
	return claudeEvent{name: name, data: b}
}

// readOutput 读取 claude 的 stream-json 输出，转成发给调用方的 Anthropic SSE 事件
func (r *claudeRun) readOutput(rd io.Reader) {
	s := bufio.NewScanner(rd)
	s.Buffer(make([]byte, 64<<10), 64<<20)
	var (
		stopReason string
		sawDelta   bool
		continuing bool // 上一次请求流被截断，claude 正在重试：接在同一条回复后面
		nextIndex  int
		remap      = map[int]int{}
		skip       = map[int]bool{}
	)
	for s.Scan() {
		var env struct {
			Type           string          `json:"type"`
			Subtype        string          `json:"subtype"`
			IsError        bool            `json:"is_error"`
			Result         string          `json:"result"`
			APIErrorStatus int             `json:"api_error_status"`
			Event          json.RawMessage `json:"event"`
		}
		if json.Unmarshal(s.Bytes(), &env) != nil {
			continue
		}
		switch env.Type {
		case "stream_event":
			var ev map[string]any
			if json.Unmarshal(env.Event, &ev) != nil {
				continue
			}
			typ, _ := ev["type"].(string)
			index := -1
			if f, ok := ev["index"].(float64); ok {
				index = int(f)
			}
			switch typ {
			case "message_start":
				stopReason, sawDelta = "", false
				remap, skip = map[int]int{}, map[int]bool{}
				if continuing {
					continuing = false
					continue
				}
				nextIndex = 0
				r.mu.Lock()
				r.asked = nil
				r.reply.Reset()
				r.calls = nil
				r.mu.Unlock()
				r.emit(sseEvent(typ, ev))
			case "content_block_start":
				block, _ := ev["content_block"].(map[string]any)
				if block != nil && block["type"] == "tool_use" {
					name, _ := block["name"].(string)
					short, ours := strings.CutPrefix(name, claudeToolPrefix)
					if !ours {
						skip[index] = true // claude 自己的工具，由它自己处理
						continue
					}
					block["name"] = short
					id, _ := block["id"].(string)
					r.mu.Lock()
					r.asked = append(r.asked, claudeAsked{id: id, name: short})
					r.calls = append(r.calls, id)
					r.mu.Unlock()
				}
				remap[index] = nextIndex
				nextIndex++
				ev["index"] = remap[index]
				r.emit(sseEvent(typ, ev))
			case "content_block_delta", "content_block_stop":
				if skip[index] {
					continue
				}
				if d, ok := ev["delta"].(map[string]any); ok && d["type"] == "text_delta" {
					r.mu.Lock()
					r.reply.WriteString(fmt.Sprint(d["text"]))
					r.mu.Unlock()
				}
				if i, ok := remap[index]; ok {
					ev["index"] = i
				}
				r.emit(sseEvent(typ, ev))
			case "message_delta":
				sawDelta = true
				if d, ok := ev["delta"].(map[string]any); ok {
					stopReason, _ = d["stop_reason"].(string)
				}
				r.emit(sseEvent(typ, ev))
			case "message_stop":
				if !sawDelta {
					continuing = true // 流被截断，claude 会再请求一次
					continue
				}
				r.mu.Lock()
				calls := len(r.asked)
				r.mu.Unlock()
				switch {
				case stopReason == "tool_use" && calls > 0:
					r.parkNow()
					r.endWith(sseEvent(typ, ev))
				case stopReason == "tool_use":
					r.emit(sseEvent(typ, ev)) // claude 自己的工具，接着生成
				default:
					r.bridge.keepIdle(r)
					r.endWith(sseEvent(typ, ev))
				}
			}
		case "result":
			r.mu.Lock()
			r.results++
			stale := r.results < r.turns
			r.mu.Unlock()
			if stale {
				continue
			}
			if env.IsError {
				msg := strings.TrimSpace(env.Result)
				if msg == "" {
					msg = "Claude Code 返回错误（" + env.Subtype + "）"
				}
				r.fail(claudeErrorStatus(env.APIErrorStatus, msg), msg)
				r.finishTurn()
				continue
			}
			r.endSegment()
		}
	}
}

// claudeErrorStatus 让额度用完、限流等错误返回 429，网关可据此切换备用上游
func claudeErrorStatus(status int, msg string) int {
	if status >= 400 {
		return status
	}
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "limit") || strings.Contains(low, "quota") || strings.Contains(low, "overloaded"):
		return http.StatusTooManyRequests
	case strings.Contains(low, "login") || strings.Contains(low, "log in") || strings.Contains(low, "auth") || strings.Contains(low, "token"):
		return http.StatusUnauthorized
	}
	return http.StatusBadGateway
}

// ===== 回给网关的响应 =====

func claudeJSONResponse(status int, v any) *http.Response {
	b, _ := json.Marshal(v)
	return &http.Response{
		StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(bytes.NewReader(b)),
		ContentLength: int64(len(b)),
	}
}

func claudeErrorResponse(f *claudeFailure) *http.Response {
	kind := "api_error"
	switch f.status {
	case http.StatusTooManyRequests:
		kind = "rate_limit_error"
	case http.StatusUnauthorized:
		kind = "authentication_error"
	case http.StatusBadRequest:
		kind = "invalid_request_error"
	}
	return claudeJSONResponse(f.status, map[string]any{"type": "error", "error": map[string]any{"type": kind, "message": f.message}})
}

// claudeRespond 等到第一条事件：出错直接返回错误状态（便于切换备用上游），否则按流式或一次性返回
func claudeRespond(ctx context.Context, run *claudeRun, seg <-chan claudeEvent, stream bool) *http.Response {
	var first claudeEvent
	select {
	case ev, ok := <-seg:
		if !ok {
			return claudeErrorResponse(&claudeFailure{http.StatusBadGateway, "Claude Code 没有返回内容"})
		}
		first = ev
	case <-ctx.Done():
		run.abort()
		return claudeErrorResponse(&claudeFailure{499, "请求已取消"})
	}
	if first.err != nil {
		return claudeErrorResponse(first.err)
	}
	if !stream {
		msg, f := collectClaudeMessage(ctx, run, first, seg)
		if f != nil {
			return claudeErrorResponse(f)
		}
		return claudeJSONResponse(http.StatusOK, msg)
	}
	pr, pw := io.Pipe()
	go func() {
		ended := false
		write := func(ev claudeEvent) bool {
			if ev.err != nil {
				ev = sseEvent("error", map[string]any{"type": "error", "error": map[string]any{"type": "api_error", "message": ev.err.message}})
			}
			_, err := fmt.Fprintf(pw, "event: %s\ndata: %s\n\n", ev.name, ev.data)
			return err == nil
		}
		ok := write(first)
		for ok {
			select {
			case ev, open := <-seg:
				if !open {
					ended = true
					ok = false
					break
				}
				ok = write(ev)
			case <-ctx.Done():
				ok = false
			}
		}
		if !ended {
			run.abort() // 调用方中途断开
		}
		_ = pw.Close()
	}()
	return &http.Response{
		StatusCode: http.StatusOK, Status: "200 OK", Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header:        http.Header{"Content-Type": {"text/event-stream"}, "Cache-Control": {"no-cache"}},
		Body:          pr,
		ContentLength: -1,
	}
}

// collectClaudeMessage 把一段 SSE 事件拼成一个完整的 Messages 响应
func collectClaudeMessage(ctx context.Context, run *claudeRun, first claudeEvent, seg <-chan claudeEvent) (map[string]any, *claudeFailure) {
	var msg map[string]any
	blocks := map[int]map[string]any{}
	partial := map[int]*strings.Builder{}
	order := []int{}
	apply := func(ev claudeEvent) *claudeFailure {
		if ev.err != nil {
			return ev.err
		}
		var e map[string]any
		if json.Unmarshal(ev.data, &e) != nil {
			return nil
		}
		index := -1
		if f, ok := e["index"].(float64); ok {
			index = int(f)
		}
		switch ev.name {
		case "message_start":
			msg, _ = e["message"].(map[string]any)
		case "content_block_start":
			if b, ok := e["content_block"].(map[string]any); ok {
				blocks[index] = b
				order = append(order, index)
			}
		case "content_block_delta":
			b := blocks[index]
			d, _ := e["delta"].(map[string]any)
			if b == nil || d == nil {
				return nil
			}
			switch d["type"] {
			case "text_delta":
				b["text"] = fmt.Sprint(b["text"]) + fmt.Sprint(d["text"])
			case "thinking_delta":
				b["thinking"] = fmt.Sprint(b["thinking"]) + fmt.Sprint(d["thinking"])
			case "signature_delta":
				b["signature"] = d["signature"]
			case "input_json_delta":
				if partial[index] == nil {
					partial[index] = &strings.Builder{}
				}
				partial[index].WriteString(fmt.Sprint(d["partial_json"]))
			}
		case "message_delta":
			if msg == nil {
				return nil
			}
			if d, ok := e["delta"].(map[string]any); ok {
				for k, v := range d {
					msg[k] = v
				}
			}
			if u, ok := e["usage"].(map[string]any); ok {
				usage, _ := msg["usage"].(map[string]any)
				if usage == nil {
					usage = map[string]any{}
				}
				for k, v := range u {
					usage[k] = v
				}
				msg["usage"] = usage
			}
		}
		return nil
	}
	if f := apply(first); f != nil {
		return nil, f
	}
	for {
		select {
		case ev, ok := <-seg:
			if !ok {
				if msg == nil {
					return nil, &claudeFailure{http.StatusBadGateway, "Claude Code 没有返回完整回复"}
				}
				content := make([]any, 0, len(order))
				for _, i := range order {
					b := blocks[i]
					if p := partial[i]; p != nil {
						var input any
						if json.Unmarshal([]byte(p.String()), &input) != nil {
							input = map[string]any{}
						}
						b["input"] = input
					}
					content = append(content, b)
				}
				msg["content"] = content
				return msg, nil
			}
			if f := apply(ev); f != nil {
				return nil, f
			}
		case <-ctx.Done():
			run.abort()
			return nil, &claudeFailure{499, "请求已取消"}
		}
	}
}

// ===== MCP 回调（网关 /_claude/<令牌>） =====

// serveClaudeCallback MCP 小助手转来的工具调用：等调用方送回结果后再应答
func (b *claudeBridge) serveCallback(w http.ResponseWriter, r *http.Request, token string) {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err != nil || !net.ParseIP(host).IsLoopback() || r.Method != http.MethodPost {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	b.mu.Lock()
	run := b.runs[token]
	b.mu.Unlock()
	if run == nil {
		writeJSONError(w, http.StatusGone, "Claude Code 进程已结束")
		return
	}
	var call struct {
		ToolCallID string `json:"tool_call_id"`
		Name       string `json:"name"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 16<<20)).Decode(&call) != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid tool call")
		return
	}
	run.mu.Lock()
	id := call.ToolCallID
	if id == "" || !run.knows(id) {
		// 没带 tool_use ID 时，按名称认领这段回复里第一个还没认领的调用
		for _, a := range run.asked {
			if a.name == call.Name && !run.claimed[a.id] {
				id = a.id
				break
			}
		}
	}
	run.claimed[id] = true
	if res, ok := run.early[id]; ok {
		delete(run.early, id)
		run.mu.Unlock()
		writeJSON(w, http.StatusOK, res)
		return
	}
	ch := make(chan claudeToolResult, 1)
	run.pending[id] = ch
	run.mu.Unlock()
	select {
	case res, ok := <-ch:
		if !ok {
			writeJSONError(w, http.StatusGone, "Claude Code 进程已结束")
			return
		}
		writeJSON(w, http.StatusOK, res)
	case <-r.Context().Done():
	}
}

// knows mu 已持有
func (r *claudeRun) knows(id string) bool {
	for _, a := range r.asked {
		if a.id == id {
			return true
		}
	}
	_, early := r.early[id]
	return early
}

// ===== 模型列表与登录状态 =====

// claudeOAuthToken 读取 Claude Code 保存的订阅令牌（只读，不刷新）
func claudeOAuthToken() (string, bool) {
	b, err := os.ReadFile(filepath.Join(claudeConfigHome(), ".credentials.json"))
	if err != nil {
		return "", false
	}
	var creds struct {
		OAuth *struct {
			AccessToken string `json:"accessToken"`
			ExpiresAt   int64  `json:"expiresAt"`
		} `json:"claudeAiOauth"`
	}
	if json.Unmarshal(b, &creds) != nil || creds.OAuth == nil || creds.OAuth.AccessToken == "" {
		return "", false
	}
	fresh := creds.OAuth.ExpiresAt == 0 || time.UnixMilli(creds.OAuth.ExpiresAt).After(time.Now().Add(time.Minute))
	return creds.OAuth.AccessToken, fresh
}

var claudeModelAliases = []string{"sonnet", "opus", "haiku"}

// claudeModelsResponse 订阅可用的模型：先问 Anthropic（令牌过期时只给 claude 认识的别名）
func claudeModelsResponse(ctx context.Context) *http.Response {
	data := []map[string]any{}
	seen := map[string]bool{}
	for _, alias := range claudeModelAliases {
		seen[alias] = true
		data = append(data, map[string]any{"id": alias, "type": "model", "display_name": "Claude " + alias + "（最新）"})
	}
	if token, fresh := claudeOAuthToken(); token != "" && fresh {
		cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(cctx, http.MethodGet, "https://api.anthropic.com/v1/models?limit=100", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("anthropic-beta", "oauth-2025-04-20")
		if resp, err := http.DefaultClient.Do(req); err == nil {
			var list struct {
				Data []struct {
					ID          string `json:"id"`
					DisplayName string `json:"display_name"`
				} `json:"data"`
			}
			if resp.StatusCode == http.StatusOK && json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&list) == nil {
				for _, m := range list.Data {
					if m.ID != "" && !seen[m.ID] {
						seen[m.ID] = true
						data = append(data, map[string]any{"id": m.ID, "type": "model", "display_name": m.DisplayName})
					}
				}
			}
			resp.Body.Close()
		}
	}
	return claudeJSONResponse(http.StatusOK, map[string]any{"data": data, "has_more": false})
}

// ===== MCP 小助手（claude 启动的 stdio 子进程） =====

// runClaudeBridgeMCP `mcp claude-bridge <回调地址> <工具文件>`：把调用方的工具提供给 claude，
// 每次 tools/call 转给网关，等到调用方的结果再应答
func runClaudeBridgeMCP(args []string) error {
	if len(args) != 2 {
		return errors.New("用法：mcp claude-bridge <回调地址> <工具文件>")
	}
	callback, toolsPath := args[0], args[1]
	if u, err := url.Parse(callback); err != nil || u.Scheme != "http" {
		return errors.New("回调地址无效")
	}
	raw, err := os.ReadFile(toolsPath)
	if err != nil {
		return err
	}
	var tools []json.RawMessage
	if err := json.Unmarshal(raw, &tools); err != nil {
		return fmt.Errorf("读取工具列表失败: %v", err)
	}
	client := &http.Client{} // tools/call 会一直等到调用方送回结果
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 64<<10), 32<<20)
	enc := json.NewEncoder(os.Stdout)
	var outMu sync.Mutex
	respond := func(v any) {
		outMu.Lock()
		_ = enc.Encode(v)
		outMu.Unlock()
	}
	for in.Scan() {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(in.Bytes(), &req) != nil || len(req.ID) == 0 {
			continue
		}
		go func() {
			var result, rpcErr any
			switch req.Method {
			case "initialize":
				var p struct {
					ProtocolVersion string `json:"protocolVersion"`
				}
				_ = json.Unmarshal(req.Params, &p)
				if p.ProtocolVersion == "" {
					p.ProtocolVersion = "2025-06-18"
				}
				result = map[string]any{
					"protocolVersion": p.ProtocolVersion,
					"capabilities":    map[string]any{"tools": map[string]any{}},
					"serverInfo":      map[string]any{"name": claudeBridgeServer, "version": appVersion},
				}
			case "ping":
				result = map[string]any{}
			case "tools/list":
				result = map[string]any{"tools": tools}
			case "tools/call":
				var p struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
					Meta      map[string]any  `json:"_meta"`
				}
				if err := json.Unmarshal(req.Params, &p); err != nil {
					rpcErr = map[string]any{"code": -32602, "message": err.Error()}
					break
				}
				id, _ := p.Meta["claudecode/toolUseId"].(string)
				payload, _ := json.Marshal(map[string]any{"tool_call_id": id, "name": p.Name, "arguments": p.Arguments})
				resp, err := client.Post(callback, "application/json", bytes.NewReader(payload))
				if err != nil {
					rpcErr = map[string]any{"code": -32000, "message": err.Error()}
					break
				}
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					rpcErr = map[string]any{"code": -32000, "message": string(body)}
					break
				}
				var res claudeToolResult
				if err := json.Unmarshal(body, &res); err != nil {
					rpcErr = map[string]any{"code": -32000, "message": err.Error()}
					break
				}
				result = res
			default:
				rpcErr = map[string]any{"code": -32601, "message": "method not found"}
			}
			out := map[string]any{"jsonrpc": "2.0", "id": req.ID}
			if rpcErr != nil {
				out["error"] = rpcErr
			} else {
				out["result"] = result
			}
			respond(out)
		}()
	}
	return in.Err()
}
