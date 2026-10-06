package main

// 订阅账号上游：路由（或备用上游）的 Base URL 写成 account://codex、account://copilot 或 account://claude 时，
// 网关用本机 Codex 的 ChatGPT 登录、GitHub Copilot 的登录或已登录的 Claude Code（见 claude_bridge.go）去请求，
// 让其它工具也能用上这些订阅的模型。
//
// 注意：这属于在官方客户端之外使用订阅，供应商可能视为违反使用条款并限制账号，请自行评估风险。

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"sync"
	"time"
)

const (
	accountScheme      = "account://"
	codexBackendBase   = "https://chatgpt.com/backend-api/codex"
	codexTokenEndpoint = "https://auth.openai.com/oauth/token"
	codexOAuthClientID = "app_EMoamEEZ73f0CkXaXp7hrann" // Codex CLI 自己的 OAuth client
	codexFallbackVer   = "0.150.0"
	copilotTokenURL    = "https://api.github.com/copilot_internal/v2/token"
	copilotDefaultAPI  = "https://api.githubcopilot.com"
)

// accountKind 返回 account:// 上游的类型
func accountKind(base string) (string, bool) {
	b := strings.ToLower(strings.TrimSpace(base))
	if !strings.HasPrefix(b, accountScheme) {
		return "", false
	}
	kind := strings.Trim(strings.TrimPrefix(b, accountScheme), "/")
	switch kind {
	case "codex", "copilot", "claude":
		return kind, true
	}
	return "", false
}

func envUsesAccount(env *EnvConfig) bool {
	if env == nil {
		return false
	}
	base, _, _ := upstreamVarsForEnv(env)
	_, ok := accountKind(base)
	return ok
}

// accountFix 请求发出后需要对响应做的处理
type accountFix struct {
	toJSON bool // 客户端要非流式，而上游只给流式
}

// prepareAccountRequest 把网关构造的上游请求改写成订阅账号的请求
func prepareAccountRequest(req *http.Request, kind string) (*http.Request, *accountFix, error) {
	var body []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, nil, err
		}
		body = b
	}
	path := req.URL.Path
	fix := &accountFix{}
	var target string
	header := http.Header{}
	for k, v := range req.Header {
		switch strings.ToLower(k) {
		case "authorization", "x-api-key", "x-goog-api-key", "host", "content-length":
			continue
		}
		header[k] = v
	}
	switch kind {
	case "codex":
		token, accountID, err := codexAccessToken()
		if err != nil {
			return nil, nil, err
		}
		switch {
		case strings.HasSuffix(path, "/responses"):
			target = codexBackendBase + "/responses"
			var wantStream bool
			body, wantStream = codexRequestBody(body, token, accountID)
			fix.toJSON = !wantStream
		case strings.HasSuffix(path, "/models"):
			target = codexBackendBase + "/models?client_version=" + url.QueryEscape(codexClientVersion())
		default:
			return nil, nil, fmt.Errorf("ChatGPT 订阅只支持 Responses 协议：请把该路由的上游格式设为 Responses")
		}
		ver := codexClientVersion()
		header.Set("Authorization", "Bearer "+token)
		if accountID != "" {
			header.Set("chatgpt-account-id", accountID)
		}
		header.Set("OpenAI-Beta", "responses=experimental")
		header.Set("originator", "codex_cli_rs")
		header.Set("version", ver)
		header.Set("User-Agent", codexUserAgent(ver))
		if body != nil {
			header.Set("Accept", "text/event-stream")
			var v struct {
				Key string `json:"prompt_cache_key"`
			}
			_ = json.Unmarshal(body, &v)
			if v.Key != "" {
				header.Set("session_id", v.Key)
				header.Set("conversation_id", v.Key)
			}
		}
	case "copilot":
		session, err := copilotSessionToken()
		if err != nil {
			return nil, nil, err
		}
		api := strings.TrimRight(firstNonEmpty(session.Endpoints.API, copilotDefaultAPI), "/")
		switch {
		case strings.HasSuffix(path, "/chat/completions"):
			target = api + "/chat/completions"
		case strings.HasSuffix(path, "/responses"):
			target = api + "/responses"
		case strings.HasSuffix(path, "/messages"):
			target = api + "/v1/messages"
		case strings.HasSuffix(path, "/models"):
			target = api + "/models"
		default:
			return nil, nil, fmt.Errorf("Copilot 订阅不支持接口 %s", path)
		}
		header.Set("Authorization", "Bearer "+session.Token)
		for k, v := range copilotEditorHeaders {
			header.Set(k, v)
		}
		header.Set("X-GitHub-Api-Version", "2026-01-09")
		header.Set("Openai-Intent", "conversation-agent")
	default:
		return nil, nil, fmt.Errorf("未知的订阅账号类型 %s", kind)
	}
	out, err := http.NewRequestWithContext(req.Context(), req.Method, target, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	out.Header = header
	if body != nil {
		out.Header.Set("Content-Type", "application/json")
	}
	return out, fix, nil
}

// apply 处理订阅账号的响应：非流式请求时把 SSE 收拢成一个 JSON 响应
func (f *accountFix) apply(resp *http.Response) *http.Response {
	if f == nil || !f.toJSON || resp == nil || resp.StatusCode >= 300 || !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		return resp
	}
	defer resp.Body.Close()
	var final map[string]any
	var failure string
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64<<10), 32<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var ev map[string]any
		if json.Unmarshal([]byte(strings.TrimSpace(line[5:])), &ev) != nil {
			continue
		}
		switch ev["type"] {
		case "response.completed", "response.done", "response.incomplete":
			if r, ok := ev["response"].(map[string]any); ok {
				final = r
			}
		case "response.failed", "error":
			b, _ := json.Marshal(ev)
			failure = string(b)
		}
	}
	status := http.StatusOK
	var data []byte
	switch {
	case final != nil:
		data, _ = json.Marshal(final)
	case failure != "":
		status = http.StatusBadGateway
		data = []byte(failure)
	default:
		status = http.StatusBadGateway
		data, _ = json.Marshal(map[string]any{"error": map[string]any{"message": "ChatGPT 订阅没有返回完整响应"}})
	}
	out := *resp
	out.StatusCode = status
	out.Status = fmt.Sprintf("%d %s", status, http.StatusText(status))
	out.Header = resp.Header.Clone()
	out.Header.Set("Content-Type", "application/json")
	out.Header.Del("Content-Length")
	out.Body = io.NopCloser(bytes.NewReader(data))
	out.ContentLength = int64(len(data))
	return &out
}

// ===== Codex（ChatGPT 登录） =====

var codexTokenMu sync.Mutex

func codexAuthPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(resolveCodexHome(home), "auth.json")
}

// codexAccessToken 读取 Codex 的 ChatGPT 登录；令牌将要过期时按 Codex 的方式刷新并写回 auth.json，
// 其它字段保持原样，Codex 下次启动直接用新令牌
func codexAccessToken() (token, accountID string, err error) {
	codexTokenMu.Lock()
	defer codexTokenMu.Unlock()
	path := codexAuthPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("没有找到 Codex 的 ChatGPT 登录，请先运行 codex login")
	}
	var auth codexAuthFile
	if json.Unmarshal(raw, &auth) != nil || auth.Tokens == nil || auth.Tokens.AccessToken == "" || strings.EqualFold(auth.AuthMode, "apikey") {
		return "", "", fmt.Errorf("Codex 当前不是 ChatGPT 登录，请运行 codex login 用 ChatGPT 账号登录")
	}
	accountID = auth.Tokens.AccountID
	if accountID == "" {
		accountID = claimText(jwtPayload(auth.Tokens.IDToken), "https://api.openai.com/auth", "chatgpt_account_id")
	}
	exp, _ := jwtPayload(auth.Tokens.AccessToken)["exp"].(float64)
	if exp == 0 || time.Until(time.Unix(int64(exp), 0)) > 5*time.Minute {
		return auth.Tokens.AccessToken, accountID, nil
	}
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil {
		return "", "", fmt.Errorf("Codex 的 auth.json 无法解析")
	}
	tokens, _ := doc["tokens"].(map[string]any)
	refresh, _ := tokens["refresh_token"].(string)
	if refresh == "" {
		return "", "", fmt.Errorf("Codex 登录已过期，请运行 codex login 重新登录")
	}
	payload, _ := json.Marshal(map[string]string{"client_id": codexOAuthClientID, "grant_type": "refresh_token", "refresh_token": refresh, "scope": "openid profile email"})
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Post(codexTokenEndpoint, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", "", fmt.Errorf("刷新 Codex 登录失败: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var fresh struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if resp.StatusCode != http.StatusOK || json.Unmarshal(b, &fresh) != nil || fresh.AccessToken == "" {
		return "", "", fmt.Errorf("Codex 登录已失效（刷新返回 HTTP %d），请运行 codex login 重新登录", resp.StatusCode)
	}
	tokens["access_token"] = fresh.AccessToken
	if fresh.IDToken != "" {
		tokens["id_token"] = fresh.IDToken
	}
	if fresh.RefreshToken != "" {
		tokens["refresh_token"] = fresh.RefreshToken
	}
	doc["tokens"] = tokens
	doc["last_refresh"] = time.Now().UTC().Format(time.RFC3339Nano)
	if out, err := json.MarshalIndent(doc, "", "  "); err == nil {
		_ = writeFileAtomic(path, append(out, '\n'), 0o600)
	}
	return fresh.AccessToken, accountID, nil
}

var codexVersionCache struct {
	sync.Mutex
	v  string
	at time.Time
}

var semverRE = regexp.MustCompile(`\d+\.\d+\.\d+`)

// codexClientVersion Codex CLI 的版本：模型列表只返回不比客户端新的模型
func codexClientVersion() string {
	codexVersionCache.Lock()
	defer codexVersionCache.Unlock()
	if codexVersionCache.v != "" && time.Since(codexVersionCache.at) < 30*time.Minute {
		return codexVersionCache.v
	}
	v := codexFallbackVer
	home, _ := os.UserHomeDir()
	var cache struct {
		ClientVersion string `json:"client_version"`
	}
	if b, err := os.ReadFile(filepath.Join(resolveCodexHome(home), "models_cache.json")); err == nil && json.Unmarshal(b, &cache) == nil {
		if c := semverRE.FindString(cache.ClientVersion); c != "" && versionLess(v, c) {
			v = c
		}
	}
	if bin := findCliBinary("codex"); bin != "" {
		if out, err := runToolRaw(5*time.Second, bin, "--version"); err == nil {
			if c := semverRE.FindString(out); c != "" && versionLess(v, c) {
				v = c
			}
		}
	}
	codexVersionCache.v, codexVersionCache.at = v, time.Now()
	return v
}

func codexUserAgent(version string) string {
	arch := map[string]string{"arm64": "arm64", "amd64": "x86_64"}[goruntime.GOARCH]
	if arch == "" {
		arch = goruntime.GOARCH
	}
	osName := map[string]string{"darwin": "Mac OS", "windows": "Windows", "linux": "Linux"}[goruntime.GOOS]
	term := map[string]string{"darwin": "Apple_Terminal/455", "windows": "WindowsTerminal"}[goruntime.GOOS]
	if term == "" {
		term = "xterm-256color"
	}
	return "codex_cli_rs/" + version + " (" + osName + "; " + arch + ") " + term
}

var codexPromptCache = struct {
	sync.Mutex
	prompts map[string]string
	fetched time.Time
}{}

func parseCodexPrompts(b []byte) map[string]string {
	var list struct {
		Models []struct {
			Slug     string `json:"slug"`
			Base     string `json:"base_instructions"`
			Messages struct {
				Template string `json:"instructions_template"`
			} `json:"model_messages"`
		} `json:"models"`
	}
	out := map[string]string{}
	if json.Unmarshal(b, &list) != nil {
		return out
	}
	for _, m := range list.Models {
		if m.Messages.Template != "" {
			out[m.Slug] = m.Messages.Template
		} else if m.Base != "" {
			out[m.Slug] = m.Base
		}
	}
	return out
}

// codexInstructions 某个模型应带的官方 instructions：先看 Codex CLI 的 models_cache.json，
// 再看本程序缓存，都没有时向 ChatGPT 的模型列表要一次
func codexInstructions(model, token, accountID string) string {
	home, _ := os.UserHomeDir()
	if b, err := os.ReadFile(filepath.Join(resolveCodexHome(home), "models_cache.json")); err == nil {
		if s := parseCodexPrompts(b)[model]; s != "" {
			return s
		}
	}
	codexPromptCache.Lock()
	defer codexPromptCache.Unlock()
	if codexPromptCache.prompts == nil {
		codexPromptCache.prompts = map[string]string{}
		if p, err := storePath("codex-prompts.json"); err == nil {
			if b, err := os.ReadFile(p); err == nil {
				_ = json.Unmarshal(b, &codexPromptCache.prompts)
			}
		}
	}
	if s := codexPromptCache.prompts[model]; s != "" {
		return s
	}
	if time.Since(codexPromptCache.fetched) < 10*time.Minute {
		return firstPrompt(codexPromptCache.prompts)
	}
	codexPromptCache.fetched = time.Now()
	req, err := http.NewRequest(http.MethodGet, codexBackendBase+"/models?client_version="+url.QueryEscape(codexClientVersion()), nil)
	if err == nil {
		req.Header.Set("Authorization", "Bearer "+token)
		if accountID != "" {
			req.Header.Set("chatgpt-account-id", accountID)
		}
		req.Header.Set("originator", "codex_cli_rs")
		req.Header.Set("User-Agent", codexUserAgent(codexClientVersion()))
		if resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req); err == nil {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				for k, v := range parseCodexPrompts(b) {
					codexPromptCache.prompts[k] = v
				}
				if p, err := storePath("codex-prompts.json"); err == nil {
					if data, err := json.Marshal(codexPromptCache.prompts); err == nil {
						_ = writeFileAtomic(p, data, 0o600)
					}
				}
			}
		}
	}
	if s := codexPromptCache.prompts[model]; s != "" {
		return s
	}
	return firstPrompt(codexPromptCache.prompts)
}

func firstPrompt(m map[string]string) string {
	for _, v := range m {
		return v
	}
	return ""
}

func firstLineOf(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// boundCallID ChatGPT 后端的 call_id 最长 64 字符，更长的用其 sha256 代替（同一 ID 始终得到同一值）
func boundCallID(id string) string {
	if len(id) <= 64 {
		return id
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(id)))
	return hex.EncodeToString(sum[:])
}

// codexInputItems 适配不保存状态的请求：去掉引用与 id、孤立的工具结果改写为说明、system 改为 developer
func codexInputItems(input []any) []any {
	calls := map[string]string{}
	for _, item := range input {
		it, _ := item.(map[string]any)
		id, _ := it["call_id"].(string)
		switch it["type"] {
		case "function_call", "function_call_output", "local_shell_call", "local_shell_call_output", "custom_tool_call", "custom_tool_call_output":
			if b := boundCallID(id); b != id {
				id = b
				it["call_id"] = id
			}
		}
		switch it["type"] {
		case "function_call":
			calls[strings.TrimSpace(id)] = "function_call_output"
		case "local_shell_call":
			calls[strings.TrimSpace(id)] = "local_shell_call_output"
		case "custom_tool_call":
			calls[strings.TrimSpace(id)] = "custom_tool_call_output"
		}
	}
	out := make([]any, 0, len(input))
	for _, item := range input {
		it, ok := item.(map[string]any)
		if !ok {
			out = append(out, item)
			continue
		}
		if it["type"] == "item_reference" {
			continue
		}
		delete(it, "id")
		switch t, _ := it["type"].(string); t {
		case "function_call_output", "custom_tool_call_output", "local_shell_call_output":
			id, _ := it["call_id"].(string)
			id = strings.TrimSpace(id)
			want := calls[id]
			if id != "" && want != t && !(t == "function_call_output" && want == "local_shell_call_output") {
				text, ok := it["output"].(string)
				if !ok {
					b, _ := json.Marshal(it["output"])
					text = string(b)
				}
				if len(text) > 16000 {
					text = strings.ToValidUTF8(text[:16000], "") + "\n...[truncated]"
				}
				it = map[string]any{"type": "message", "role": "assistant", "content": "[Previous tool result; call_id=" + id + "]: " + text}
			}
		}
		if it["role"] == "system" && (it["type"] == nil || it["type"] == "message") {
			it["role"] = "developer"
		}
		out = append(out, it)
	}
	return out
}

// codexRequestBody 把 Responses 请求改成 Codex CLI 会发的样子：只流式、不保存、带官方 instructions，
// 客户端自己的 instructions 作为第一条 developer 消息；返回改写后的请求体与客户端是否要流式
func codexRequestBody(body []byte, token, accountID string) ([]byte, bool) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var m map[string]any
	if dec.Decode(&m) != nil || m == nil {
		return body, true
	}
	wantStream, _ := m["stream"].(bool)
	for _, k := range []string{"max_output_tokens", "max_completion_tokens", "max_tokens", "temperature", "top_p", "previous_response_id", "user", "safety_identifier", "metadata"} {
		delete(m, k)
	}
	if m["service_tier"] != "priority" {
		delete(m, "service_tier")
	}
	model, _ := m["model"].(string)
	if s, ok := m["input"].(string); ok {
		m["input"] = []any{map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": s}}}}
	}
	input, _ := m["input"].([]any)
	input = codexInputItems(input)
	own, _ := m["instructions"].(string)
	if k, _ := m["prompt_cache_key"].(string); k == "" {
		h := sha256.New()
		h.Write([]byte(own))
		for _, item := range input {
			b, _ := json.Marshal(item)
			h.Write(b)
			if it, _ := item.(map[string]any); it["role"] == "user" {
				s := hex.EncodeToString(h.Sum(nil))
				m["prompt_cache_key"] = s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32]
				break
			}
		}
	}
	official := codexInstructions(model, token, accountID)
	if strings.TrimSpace(own) != "" {
		if firstLineOf(own) == firstLineOf(official) {
			official = own
		} else {
			input = append([]any{map[string]any{"type": "message", "role": "developer",
				"content": []any{map[string]any{"type": "input_text", "text": own}}}}, input...)
		}
	}
	m["input"] = input
	if official != "" {
		m["instructions"] = official
	} else {
		delete(m, "instructions")
	}
	m["store"] = false
	m["stream"] = true
	if _, ok := m["tool_choice"]; !ok {
		m["tool_choice"] = "auto"
	}
	if _, ok := m["parallel_tool_calls"]; !ok {
		m["parallel_tool_calls"] = true
	}
	reasoning, _ := m["reasoning"].(map[string]any)
	effort, _ := reasoning["effort"].(string)
	if effort != "none" {
		if reasoning != nil {
			if _, ok := reasoning["summary"]; !ok {
				reasoning["summary"] = "auto"
			}
		}
		include, _ := m["include"].([]any)
		has := false
		for _, v := range include {
			if v == "reasoning.encrypted_content" {
				has = true
			}
		}
		if !has {
			m["include"] = append(include, "reasoning.encrypted_content")
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return body, wantStream
	}
	return out, wantStream
}

// ===== GitHub Copilot =====

var copilotEditorHeaders = map[string]string{
	"Editor-Version":         "vscode/1.140.0",
	"Editor-Plugin-Version":  "copilot-chat/0.68.0",
	"Copilot-Integration-Id": "vscode-chat",
	"User-Agent":             "GitHubCopilotChat/0.68.0",
}

type copilotSession struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	Endpoints struct {
		API string `json:"api"`
	} `json:"endpoints"`
}

var copilotSessionCache = struct {
	sync.Mutex
	github  string
	session copilotSession
}{}

// copilotSessionToken 用 GitHub 登录换取短期 Copilot 令牌（缓存到过期前 2 分钟）。
// 本机可能存有多个 github.com 登录，依次尝试，记住能用的那个
func copilotSessionToken() (copilotSession, error) {
	logins := copilotGitHubLogins()
	if len(logins) == 0 {
		return copilotSession{}, fmt.Errorf("没有找到 GitHub Copilot 登录：先在 VS Code 或 Copilot CLI 中登录")
	}
	copilotSessionCache.Lock()
	defer copilotSessionCache.Unlock()
	cached := copilotSessionCache.github
	ordered := make([]copilotLogin, 0, len(logins))
	for _, l := range logins {
		if l.Token == cached {
			if time.Until(time.Unix(copilotSessionCache.session.ExpiresAt, 0)) > 2*time.Minute {
				return copilotSessionCache.session, nil
			}
			ordered = append([]copilotLogin{l}, ordered...)
			continue
		}
		ordered = append(ordered, l)
	}
	var lastErr error
	for _, l := range ordered {
		s, err := exchangeCopilotSession(l.Token)
		if err == nil {
			copilotSessionCache.github, copilotSessionCache.session = l.Token, s
			return s, nil
		}
		lastErr = err
	}
	return copilotSession{}, lastErr
}

func exchangeCopilotSession(github string) (copilotSession, error) {
	req, err := http.NewRequest(http.MethodGet, copilotTokenURL, nil)
	if err != nil {
		return copilotSession{}, err
	}
	req.Header.Set("Authorization", "token "+github)
	req.Header.Set("Accept", "application/json")
	for k, v := range copilotEditorHeaders {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return copilotSession{}, fmt.Errorf("获取 Copilot 令牌失败: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var s copilotSession
	if resp.StatusCode != http.StatusOK || json.Unmarshal(b, &s) != nil || s.Token == "" {
		return copilotSession{}, fmt.Errorf("GitHub Copilot 登录无效或没有 Copilot 订阅（HTTP %d）", resp.StatusCode)
	}
	return s, nil
}

// ===== 状态 =====

// AccountUpstreamStatus 订阅账号上游是否可用
type AccountUpstreamStatus struct {
	Kind      string `json:"kind"`
	Available bool   `json:"available"`
	Account   string `json:"account,omitempty"`
	Message   string `json:"message,omitempty"`
}

// GetAccountUpstreams 检测本机可作为上游的订阅登录（只读取本地登录信息，不发网络请求）
func (rs *RouterService) GetAccountUpstreams() []AccountUpstreamStatus {
	out := []AccountUpstreamStatus{}
	codex := AccountUpstreamStatus{Kind: "codex"}
	if raw, err := os.ReadFile(codexAuthPath()); err == nil {
		var auth codexAuthFile
		if json.Unmarshal(raw, &auth) == nil && auth.Tokens != nil && auth.Tokens.AccessToken != "" && !strings.EqualFold(auth.AuthMode, "apikey") {
			codex.Available = true
			codex.Account = claimText(jwtPayload(auth.Tokens.IDToken), "email")
		}
	}
	if !codex.Available {
		codex.Message = "未检测到 Codex 的 ChatGPT 登录（codex login）"
	}
	copilot := AccountUpstreamStatus{Kind: "copilot"}
	if user, token := copilotGitHubToken(); token != "" {
		copilot.Available, copilot.Account = true, user
	} else {
		copilot.Message = "未检测到 GitHub Copilot 登录"
	}
	claude := AccountUpstreamStatus{Kind: "claude"}
	account, plan, signedIn := claudeLoginInfo()
	_, binErr := claudeExecutable()
	switch {
	case binErr != nil:
		claude.Message = binErr.Error()
	case !signedIn:
		claude.Message = "未检测到 Claude Code 的订阅登录（claude 后执行 /login）"
	default:
		claude.Available, claude.Account = true, strings.TrimSpace(account+" "+plan)
	}
	return append(out, codex, copilot, claude)
}

// ListAccountModels 读取订阅账号可用的模型 ID
func (rs *RouterService) ListAccountModels(kind string) ([]string, error) {
	if _, ok := accountKind(accountScheme + kind); !ok {
		return nil, fmt.Errorf("未知的订阅账号类型")
	}
	req, err := http.NewRequest(http.MethodGet, accountScheme+kind+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := rs.sendUpstream(req, accountScheme+kind)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("读取模型列表失败（HTTP %d）", resp.StatusCode)
	}
	var list struct {
		Models []struct {
			Slug       string `json:"slug"`
			Visibility string `json:"visibility"`
		} `json:"models"`
		Data []struct {
			ID         string `json:"id"`
			Capability *struct {
				Type string `json:"type"`
			} `json:"capabilities"`
			Picker *bool `json:"model_picker_enabled"`
			Policy *struct {
				State string `json:"state"`
			} `json:"policy"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("模型列表格式无法识别")
	}
	out := []string{}
	seen := map[string]bool{}
	for _, m := range list.Models {
		if m.Slug != "" && m.Visibility != "hide" && !seen[m.Slug] {
			seen[m.Slug] = true
			out = append(out, m.Slug)
		}
	}
	// Copilot 也列出内部用的模型（选择器里看不到、也没有启用状态）：先只取可选或已启用的，
	// 一个都没有时再放宽；在账号策略里被停用的始终不要
	for _, strict := range []bool{true, false} {
		for _, m := range list.Data {
			if m.ID == "" || seen[m.ID] || m.Capability != nil && m.Capability.Type != "" && m.Capability.Type != "chat" ||
				m.Policy != nil && m.Policy.State == "disabled" {
				continue
			}
			if strict && m.Picker != nil && !*m.Picker && (m.Policy == nil || m.Policy.State != "enabled") {
				continue
			}
			seen[m.ID] = true
			out = append(out, m.ID)
		}
		if len(out) > 0 {
			break
		}
	}
	return out, nil
}
