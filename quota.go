package main

// 订阅额度：读取本机已登录 CLI 订阅（Claude Code / Codex / Copilot）的滚动额度窗口。
// 只读取各 CLI 自己保存的登录凭证，不复制、不刷新、不回写它们的令牌文件；
// Claude 的额度通过运行本机 claude 的 /usage 获取（与 CLI 自己展示的一致），
// 只在用户打开额度页或手动刷新时运行，后台监控仅在 Claude Code 有新会话写入后才会再读。

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// QuotaWindow 一个滚动额度窗口（如 5 小时、7 天、本月高级请求）
type QuotaWindow struct {
	Name      string  `json:"name"`
	Used      float64 `json:"used"`                // 已用百分比 0-100
	ResetsAt  int64   `json:"resets_at,omitempty"` // 重置时间，unix 毫秒
	SpanHours int     `json:"span_hours,omitempty"`
	Display   string  `json:"display,omitempty"` // 供应商给出的计数，如 "120 / 300"
	Unlimited bool    `json:"unlimited,omitempty"`
}

// SubscriptionQuota 一个订阅账号的额度
type SubscriptionQuota struct {
	Provider string        `json:"provider"` // claude | codex | copilot
	Name     string        `json:"name"`
	Account  string        `json:"account,omitempty"`
	Plan     string        `json:"plan,omitempty"`
	Windows  []QuotaWindow `json:"windows"`
	Credits  string        `json:"credits,omitempty"`
	Until    int64         `json:"until,omitempty"` // 套餐有效期，unix 毫秒
	Status   string        `json:"status"`          // ok | pending | signed_out | not_installed | expired | api_billing | error
	Error    string        `json:"error,omitempty"`
	ReadAt   int64         `json:"read_at,omitempty"`
}

// QuotaService 订阅额度与余额服务
type QuotaService struct {
	app *App
	ctx context.Context

	mu          sync.Mutex
	quotas      map[string]SubscriptionQuota
	quotasAt    time.Time
	claudeTried time.Time
	reading     chan struct{}

	balanceMu sync.Mutex
	balances  []BalanceCard
	balanceAt time.Time

	alertMu sync.Mutex
	alerted map[string]int64 // 已提醒过的窗口/余额，值为对应的重置时间或 1，避免重复打扰
}

func NewQuotaService(a *App) *QuotaService {
	qs := &QuotaService{app: a, quotas: map[string]SubscriptionQuota{}, alerted: map[string]int64{}}
	globalQuotaService = qs
	return qs
}

// globalQuotaService 供托盘面板读取已缓存的额度（不触发网络请求）
var globalQuotaService *QuotaService

// cachedSnapshot 返回已读到的额度与余额，不发起读取
func (qs *QuotaService) cachedSnapshot() ([]SubscriptionQuota, []BalanceCard) {
	qs.mu.Lock()
	quotas := qs.snapshotLocked()
	qs.mu.Unlock()
	qs.balanceMu.Lock()
	balances := append([]BalanceCard{}, qs.balances...)
	qs.balanceMu.Unlock()
	return quotas, balances
}

const (
	quotaCacheTTL      = 60 * time.Second
	claudeUsageFloor   = 30 * time.Second
	claudeUsageTimeout = 60 * time.Second
	quotaHTTPTimeout   = 15 * time.Second
)

var (
	quotaCodexUsageURL  = "https://chatgpt.com/backend-api/wham/usage"
	quotaCopilotUserURL = "https://api.github.com/copilot_internal/user"
)

// GetSubscriptionQuotas 读取各订阅额度；force 表示用户主动刷新，会跳过缓存。
func (qs *QuotaService) GetSubscriptionQuotas(force bool) []SubscriptionQuota {
	return qs.readQuotas(force, true)
}

// readQuotas readClaude=false 时只读 Codex / Copilot 的 HTTP 接口，Claude 沿用上次读数
func (qs *QuotaService) readQuotas(force, readClaude bool) []SubscriptionQuota {
	qs.mu.Lock()
	if !force && len(qs.quotas) > 0 && time.Since(qs.quotasAt) < quotaCacheTTL {
		out := qs.snapshotLocked()
		qs.mu.Unlock()
		return out
	}
	if wait := qs.reading; wait != nil {
		// 已有一次读取在进行：等它完成，复用结果，避免并发拉起多个 claude 进程
		qs.mu.Unlock()
		<-wait
		qs.mu.Lock()
		out := qs.snapshotLocked()
		qs.mu.Unlock()
		return out
	}
	done := make(chan struct{})
	qs.reading = done
	runClaude := readClaude && time.Since(qs.claudeTried) >= claudeUsageFloor
	if runClaude {
		qs.claudeTried = time.Now()
	}
	previousClaude, hadClaude := qs.quotas["claude"]
	qs.mu.Unlock()

	results := make([]SubscriptionQuota, 3)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		if runClaude || !hadClaude {
			if runClaude {
				results[0] = claudeSubscriptionQuota()
			} else {
				results[0] = claudeIdentityOnly()
			}
		} else {
			results[0] = previousClaude
		}
	}()
	go func() { defer wg.Done(); results[1] = codexSubscriptionQuota() }()
	go func() { defer wg.Done(); results[2] = copilotSubscriptionQuota() }()
	wg.Wait()

	qs.mu.Lock()
	for _, q := range results {
		// 读取失败时保留上次成功的窗口，并在错误里说明，避免界面整体闪空
		if q.Status == "error" {
			if prev, ok := qs.quotas[q.Provider]; ok && prev.Status == "ok" && len(prev.Windows) > 0 {
				q.Windows, q.ReadAt = prev.Windows, prev.ReadAt
				if q.Plan == "" {
					q.Plan = prev.Plan
				}
			}
		}
		qs.quotas[q.Provider] = q
	}
	qs.quotasAt = time.Now()
	qs.reading = nil
	out := qs.snapshotLocked()
	qs.mu.Unlock()
	close(done)
	return out
}

func (qs *QuotaService) snapshotLocked() []SubscriptionQuota {
	out := []SubscriptionQuota{}
	for _, id := range []string{"claude", "codex", "copilot"} {
		if q, ok := qs.quotas[id]; ok {
			q.Windows = append([]QuotaWindow{}, q.Windows...)
			out = append(out, q)
		}
	}
	return out
}

// ============ Claude Code ============

func claudeConfigHome() string {
	home, _ := os.UserHomeDir()
	if dir := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); dir != "" {
		return expandAndNormalizePath(dir, home, filepath.Join(home, ".claude"))
	}
	return filepath.Join(home, ".claude")
}

// claudeLoginInfo 读 Claude Code 的 OAuth 登录：账号邮箱与订阅类型。只读，不碰令牌。
// Windows / Linux 的订阅令牌在 .credentials.json 的 claudeAiOauth 里；macOS 存在钥匙串，
// 只能凭 ~/.claude.json 的 oauthAccount 判断。
func claudeLoginInfo() (account, plan string, signedIn bool) {
	var creds struct {
		OAuth *struct {
			AccessToken      string `json:"accessToken"`
			SubscriptionType string `json:"subscriptionType"`
		} `json:"claudeAiOauth"`
	}
	if b, err := os.ReadFile(filepath.Join(claudeConfigHome(), ".credentials.json")); err == nil {
		if json.Unmarshal(b, &creds) == nil && creds.OAuth != nil && creds.OAuth.AccessToken != "" {
			signedIn = true
			plan = strings.TrimSpace(creds.OAuth.SubscriptionType)
		}
	}
	home, _ := os.UserHomeDir()
	for _, path := range []string{filepath.Join(home, ".claude.json"), filepath.Join(claudeConfigHome(), ".claude.json")} {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var profile struct {
			Account *struct {
				Email   string `json:"emailAddress"`
				OrgType string `json:"organizationType"`
			} `json:"oauthAccount"`
		}
		if json.Unmarshal(b, &profile) != nil || profile.Account == nil || profile.Account.Email == "" {
			continue
		}
		account = profile.Account.Email
		if plan == "" {
			plan = strings.TrimPrefix(strings.ToLower(profile.Account.OrgType), "claude_")
		}
		if goruntime.GOOS == "darwin" {
			signedIn = true
		}
		break
	}
	return account, titleCasePlan(plan), signedIn
}

func titleCasePlan(plan string) string {
	plan = strings.TrimSpace(plan)
	if plan == "" {
		return ""
	}
	return strings.ToUpper(plan[:1]) + plan[1:]
}

func claudeIdentityOnly() SubscriptionQuota {
	q := SubscriptionQuota{Provider: "claude", Name: "Claude Code", Windows: []QuotaWindow{}}
	account, plan, signedIn := claudeLoginInfo()
	q.Account, q.Plan = account, plan
	if !signedIn {
		q.Status = "signed_out"
		return q
	}
	q.Status = "pending"
	return q
}

func claudeSubscriptionQuota() SubscriptionQuota {
	q := claudeIdentityOnly()
	if q.Status == "signed_out" {
		return q
	}
	bin := findCliBinary("claude")
	if bin == "" {
		q.Status = "not_installed"
		return q
	}
	ctx, cancel := context.WithTimeout(context.Background(), claudeUsageTimeout)
	defer cancel()
	text, err := runClaudeUsage(ctx, bin)
	if err != nil {
		q.Status, q.Error = "error", err.Error()
		return q
	}
	windows, err := parseClaudeUsageText(text, time.Now())
	if errors.Is(err, errClaudeAPIBilling) {
		q.Status = "api_billing"
		return q
	}
	if err != nil {
		q.Status, q.Error = "error", err.Error()
		return q
	}
	q.Windows, q.Status, q.ReadAt = windows, "ok", time.Now().UnixMilli()
	return q
}

func findCliBinary(name string) string {
	return pickBin(distinctBins(append(listCommandCopies(name), lookPathAll(name)...)))
}

// claudeBlockedEnv 运行 /usage 时去掉的变量：否则 claude 会按第三方 Key / 网关回答，
// 读到的不是 OAuth 订阅的额度
var claudeBlockedEnv = map[string]bool{
	"ANTHROPIC_BASE_URL": true, "ANTHROPIC_API_KEY": true, "ANTHROPIC_AUTH_TOKEN": true,
	"CLAUDECODE": true, "CLAUDE_CODE_ENTRYPOINT": true, "CLAUDE_CODE_SSE_PORT": true,
	"CLAUDE_CODE_OAUTH_TOKEN": true, "ANTHROPIC_MODEL": true,
}

func runClaudeUsage(ctx context.Context, bin string) (string, error) {
	tmp, err := os.MkdirTemp("", "aienv-claude-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	// --setting-sources "" 让 claude 不读 settings.json 里本应用写入的第三方 env；
	// --tools "" 与 --strict-mcp-config 保证这次运行不加载任何工具，只执行 /usage
	args := []string{"-p", "/usage", "--output-format", "json", "--tools", "", "--strict-mcp-config",
		"--setting-sources", "", "--no-session-persistence"}
	cmd := startToolCommand(ctx, bin, args, nil)
	env := make([]string, 0, len(cmd.Env)+3)
	for _, kv := range cmd.Env {
		k, _, _ := strings.Cut(kv, "=")
		if !claudeBlockedEnv[strings.ToUpper(k)] {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, "DISABLE_TELEMETRY=1", "DISABLE_ERROR_REPORTING=1", "DISABLE_AUTOUPDATER=1")
	cmd.Dir = tmp
	cmd.Stdin = strings.NewReader("")
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var runErr error
	select {
	case runErr = <-done:
	case <-ctx.Done():
		killCmd(cmd)
		<-done
		return "", fmt.Errorf("读取 Claude Code 额度超时")
	}
	if res, ok := claudePrintResult(out.Bytes()); ok {
		if res.IsError {
			return "", errors.New("Claude Code: " + clipText(res.Result, 240))
		}
		return res.Result, nil
	}
	msg := strings.TrimSpace(stderr.String())
	if msg == "" {
		msg = strings.TrimSpace(out.String())
	}
	if msg == "" && runErr != nil {
		return "", runErr
	}
	return "", errors.New("Claude Code: " + clipText(msg, 240))
}

type claudePrintRun struct {
	Type    string `json:"type"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`
}

// claudePrintResult 解析 claude -p --output-format json 的结果：单个对象，
// 或用户开启 verbose 时的消息数组（结果在最后）
func claudePrintResult(b []byte) (claudePrintRun, bool) {
	b = bytes.TrimSpace(b)
	var one claudePrintRun
	if json.Unmarshal(b, &one) == nil && (one.Type != "" || one.Result != "") {
		return one, true
	}
	var all []claudePrintRun
	if json.Unmarshal(b, &all) == nil {
		for i := len(all) - 1; i >= 0; i-- {
			if all[i].Type == "result" {
				return all[i], true
			}
		}
	}
	return claudePrintRun{}, false
}

var (
	ansiEscapeRE = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	// "Current session: 13% used · resets Oct 1 at 3:30pm (Asia/Shanghai)"
	// "Current week (all models): 4% used · resets Oct 3 at 2pm (Asia/Shanghai)"
	// "Current week (Opus): 0% used"
	claudeUsageLineRE   = regexp.MustCompile(`^Current (session|week(?: \(([^)]+)\))?):\s*([0-9.]+)% used(?:\s*·\s*resets (.+))?$`)
	errClaudeAPIBilling = errors.New("Claude Code 当前按 API 计费，没有订阅额度窗口")
	claudeUsageDenied   = regexp.MustCompile(`(?i)\b(401|403)\b|not (logged|signed) in|signed out|sign-in (has )?expired|unauthorized|forbidden|authentication (failed|required)|invalid (access )?token`)
)

func parseClaudeUsageText(text string, now time.Time) ([]QuotaWindow, error) {
	out := []QuotaWindow{}
	text = strings.TrimSpace(ansiEscapeRE.ReplaceAllString(text, ""))
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if claudeUsageDenied.MatchString(line) {
			return out, errors.New("Claude Code 未返回额度：" + clipText(strings.TrimSpace(line), 120))
		}
	}
	seen := map[string]bool{}
	for _, line := range lines {
		m := claudeUsageLineRE.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		used, err := strconv.ParseFloat(m[3], 64)
		if err != nil {
			continue
		}
		w := QuotaWindow{Name: "5h", Used: used, SpanHours: 5}
		if m[1] != "session" {
			w.Name, w.SpanHours = "7d", 24*7
			if scope := strings.TrimSpace(m[2]); scope != "" && !strings.EqualFold(scope, "all models") {
				w.Name = "7d · " + scope
			}
		}
		if seen[w.Name] {
			continue
		}
		seen[w.Name] = true
		if t, ok := parseClaudeResetTime(m[4], now); ok {
			w.ResetsAt = t.UnixMilli()
		}
		out = append(out, w)
	}
	if len(out) == 0 {
		// 没有订阅登录时 /usage 退化为本次会话的费用统计
		if strings.Contains(text, "Total cost:") {
			return out, errClaudeAPIBilling
		}
		first, _, _ := strings.Cut(text, "\n")
		if first == "" {
			return out, errors.New("Claude Code 的 /usage 暂时没有返回额度，请稍后再试")
		}
		return out, errors.New("Claude Code 未返回额度：" + clipText(first, 120))
	}
	return out, nil
}

// parseClaudeResetTime 解析 "Oct 1 at 3:30pm (Asia/Shanghai)" 一类的重置时间；
// 没写年份时按最近的未来日期补全
func parseClaudeResetTime(s string, now time.Time) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	loc := time.Local
	if i := strings.LastIndex(s, "("); i >= 0 && strings.HasSuffix(s, ")") {
		if l, err := time.LoadLocation(s[i+1 : len(s)-1]); err == nil {
			loc = l
		}
		s = strings.TrimSpace(s[:i])
	}
	s = strings.ReplaceAll(strings.ReplaceAll(s, "AM", "am"), "PM", "pm")
	ref := now.In(loc)
	for _, layout := range []string{
		"Jan 2 at 3:04pm", "Jan 2 at 3pm", "Jan 2, 2006 at 3:04pm", "Jan 2, 2006 at 3pm",
		"Jan 2, 3:04pm", "Jan 2, 3pm", "Jan 2, 2006, 3:04pm", "Jan 2, 2006, 3pm",
	} {
		t, err := time.ParseInLocation(layout, s, loc)
		if err != nil {
			continue
		}
		if t.Year() == 0 {
			t = t.AddDate(ref.Year(), 0, 0)
			if t.Before(ref.Add(-24 * time.Hour)) {
				t = t.AddDate(1, 0, 0)
			}
		}
		return t, true
	}
	for _, layout := range []string{"3:04pm", "3pm"} {
		t, err := time.ParseInLocation(layout, s, loc)
		if err != nil {
			continue
		}
		t = time.Date(ref.Year(), ref.Month(), ref.Day(), t.Hour(), t.Minute(), 0, 0, loc)
		if t.Before(ref) {
			t = t.AddDate(0, 0, 1)
		}
		return t, true
	}
	return time.Time{}, false
}

// claudeUsedSince Claude Code 自 t 起是否写过会话（项目目录下的 .jsonl 有更新）
func claudeUsedSince(t time.Time) bool {
	projects := filepath.Join(claudeConfigHome(), "projects")
	dirs, _ := os.ReadDir(projects)
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(projects, d.Name()))
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			if fi, err := f.Info(); err == nil && fi.ModTime().After(t) {
				return true
			}
		}
	}
	return false
}

// ============ Codex（ChatGPT 登录） ============

type codexAuthFile struct {
	AuthMode string `json:"auth_mode"`
	APIKey   string `json:"OPENAI_API_KEY"`
	Tokens   *struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
		AccountID   string `json:"account_id"`
	} `json:"tokens"`
}

func jwtPayload(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil
	}
	var claims map[string]any
	if json.Unmarshal(raw, &claims) != nil {
		return nil
	}
	return claims
}

func claimText(claims map[string]any, path ...string) string {
	var cur any = claims
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[p]
	}
	s, _ := cur.(string)
	return s
}

func codexSubscriptionQuota() SubscriptionQuota {
	q := SubscriptionQuota{Provider: "codex", Name: "Codex", Windows: []QuotaWindow{}}
	home, _ := os.UserHomeDir()
	b, err := os.ReadFile(filepath.Join(resolveCodexHome(home), "auth.json"))
	if err != nil {
		q.Status = "signed_out"
		return q
	}
	var auth codexAuthFile
	if json.Unmarshal(b, &auth) != nil || auth.Tokens == nil || auth.Tokens.AccessToken == "" || strings.EqualFold(auth.AuthMode, "apikey") {
		q.Status = "signed_out"
		return q
	}
	id := jwtPayload(auth.Tokens.IDToken)
	q.Account = claimText(id, "email")
	q.Plan = titleCasePlan(claimText(id, "https://api.openai.com/auth", "chatgpt_plan_type"))
	if until, err := time.Parse(time.RFC3339, claimText(id, "https://api.openai.com/auth", "chatgpt_subscription_active_until")); err == nil && until.After(time.Now()) {
		q.Until = until.UnixMilli()
	}
	accountID := auth.Tokens.AccountID
	if accountID == "" {
		accountID = claimText(id, "https://api.openai.com/auth", "chatgpt_account_id")
	}
	if exp, ok := jwtPayload(auth.Tokens.AccessToken)["exp"].(float64); ok && exp > 0 && time.Now().After(time.Unix(int64(exp), 0)) {
		// 不替 Codex 刷新令牌：刷新会轮换 refresh_token，与正在运行的 Codex 抢写会导致需要重新登录
		q.Status = "expired"
		return q
	}

	var data struct {
		PlanType string `json:"plan_type"`
		Credits  *struct {
			Has       bool   `json:"has_credits"`
			Unlimited bool   `json:"unlimited"`
			Balance   string `json:"balance"`
		} `json:"credits"`
		RateLimit struct {
			Primary   *codexRateWindow `json:"primary_window"`
			Secondary *codexRateWindow `json:"secondary_window"`
		} `json:"rate_limit"`
	}
	headers := map[string]string{"Authorization": "Bearer " + auth.Tokens.AccessToken}
	if accountID != "" {
		headers["chatgpt-account-id"] = accountID
	}
	if err := quotaGetJSON(quotaCodexUsageURL, headers, &data); err != nil {
		q.Status, q.Error = "error", err.Error()
		var se *quotaStatusError
		if errors.As(err, &se) && (se.status == http.StatusUnauthorized || se.status == http.StatusForbidden) {
			q.Status = "expired"
		}
		return q
	}
	if data.PlanType != "" {
		q.Plan = titleCasePlan(data.PlanType)
	}
	if c := data.Credits; c != nil && c.Has && !c.Unlimited {
		if n, err := strconv.ParseFloat(strings.TrimSpace(c.Balance), 64); err == nil && n > 0 {
			q.Credits = strconv.FormatFloat(n, 'f', -1, 64)
		}
	}
	for _, w := range []*codexRateWindow{data.RateLimit.Primary, data.RateLimit.Secondary} {
		if w != nil {
			q.Windows = append(q.Windows, w.window())
		}
	}
	q.Status, q.ReadAt = "ok", time.Now().UnixMilli()
	return q
}

type codexRateWindow struct {
	UsedPercent     float64 `json:"used_percent"`
	LimitWindowSecs int64   `json:"limit_window_seconds"`
	ResetAt         int64   `json:"reset_at"`
	ResetAfterSecs  int64   `json:"reset_after_seconds"`
}

func (w codexRateWindow) window() QuotaWindow {
	out := QuotaWindow{Used: w.UsedPercent, SpanHours: int(w.LimitWindowSecs / 3600), Name: durationLabel(w.LimitWindowSecs)}
	switch {
	case w.ResetAt > 0:
		out.ResetsAt = w.ResetAt * 1000
	case w.ResetAfterSecs > 0:
		out.ResetsAt = time.Now().Add(time.Duration(w.ResetAfterSecs) * time.Second).UnixMilli()
	}
	return out
}

func durationLabel(seconds int64) string {
	switch {
	case seconds > 0 && seconds%(24*3600) == 0:
		return fmt.Sprintf("%dd", seconds/(24*3600))
	case seconds > 0 && seconds%3600 == 0:
		return fmt.Sprintf("%dh", seconds/3600)
	case seconds > 0:
		return fmt.Sprintf("%dm", seconds/60)
	}
	return "—"
}

// ============ GitHub Copilot ============

func copilotConfigDirs() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{}
	if cfg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); cfg != "" {
		dirs = append(dirs, filepath.Join(cfg, "github-copilot"))
	}
	dirs = append(dirs, filepath.Join(home, ".config", "github-copilot"))
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		dirs = append(dirs, filepath.Join(local, "github-copilot"))
	}
	return dirs
}

type copilotLogin struct{ User, Token string }

// copilotAppRank Copilot 官方客户端（VS Code / JetBrains 插件、Copilot CLI）的 GitHub 应用排在前面
func copilotAppRank(key string) int {
	for i, app := range []string{"Iv1.b507a08c87ecfe98", "Iv23ctfURkiMfJ4xr5mv"} {
		if strings.HasSuffix(key, ":"+app) {
			return i
		}
	}
	return 9
}

// copilotGitHubLogins Copilot 编辑器插件 / CLI 保存的全部 github.com 登录，顺序固定：
// apps.json 里可能同时有几个应用、几个账号，并非每个都有 Copilot 订阅
func copilotGitHubLogins() []copilotLogin {
	var out []copilotLogin
	seen := map[string]bool{}
	for _, dir := range copilotConfigDirs() {
		for _, name := range []string{"apps.json", "hosts.json"} {
			b, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			var apps map[string]struct {
				User  string `json:"user"`
				Token string `json:"oauth_token"`
			}
			if json.Unmarshal(b, &apps) != nil {
				continue
			}
			keys := make([]string, 0, len(apps))
			for key := range apps {
				keys = append(keys, key)
			}
			sort.Slice(keys, func(i, j int) bool {
				if a, b := copilotAppRank(keys[i]), copilotAppRank(keys[j]); a != b {
					return a < b
				}
				return keys[i] < keys[j]
			})
			for _, key := range keys {
				app := apps[key]
				// 只把 github.com 的令牌发给 api.github.com，企业版域名的令牌不外发
				if strings.HasPrefix(key, "github.com") && app.Token != "" && !seen[app.Token] {
					seen[app.Token] = true
					out = append(out, copilotLogin{User: app.User, Token: app.Token})
				}
			}
		}
	}
	return out
}

// copilotGitHubToken 当前使用的 github.com 登录：已验证可换到 Copilot 令牌的优先
func copilotGitHubToken() (user, token string) {
	logins := copilotGitHubLogins()
	if len(logins) == 0 {
		return "", ""
	}
	copilotSessionCache.Lock()
	working := copilotSessionCache.github
	copilotSessionCache.Unlock()
	for _, l := range logins {
		if l.Token == working {
			return l.User, l.Token
		}
	}
	return logins[0].User, logins[0].Token
}

var copilotPlanNames = map[string]string{
	"free": "Free", "individual": "Pro", "individual_pro": "Pro+", "business": "Business", "enterprise": "Enterprise",
}

func copilotSubscriptionQuota() SubscriptionQuota {
	q := SubscriptionQuota{Provider: "copilot", Name: "GitHub Copilot", Windows: []QuotaWindow{}}
	logins := copilotGitHubLogins()
	if len(logins) == 0 {
		q.Status = "signed_out"
		return q
	}
	// 当前使用的登录排第一，其余依次尝试：取第一个有 Copilot 的账号
	if user, token := copilotGitHubToken(); token != "" {
		ordered := []copilotLogin{{User: user, Token: token}}
		for _, l := range logins {
			if l.Token != token {
				ordered = append(ordered, l)
			}
		}
		logins = ordered
	}
	var last SubscriptionQuota
	for _, l := range logins {
		last = copilotQuotaFor(l.User, l.Token)
		if last.Status != "expired" {
			return last
		}
	}
	return last
}

func copilotQuotaFor(user, token string) SubscriptionQuota {
	q := SubscriptionQuota{Provider: "copilot", Name: "GitHub Copilot", Windows: []QuotaWindow{}, Account: user}
	var data struct {
		Plan      string `json:"copilot_plan"`
		AccessSKU string `json:"access_type_sku"`
		Snapshots map[string]struct {
			Unlimited   bool    `json:"unlimited"`
			HasQuota    bool    `json:"has_quota"`
			Entitlement float64 `json:"entitlement"`
			Remaining   float64 `json:"quota_remaining"`
		} `json:"quota_snapshots"`
		Reset    string `json:"quota_reset_date_utc"`
		ResetDay string `json:"quota_reset_date"`
	}
	headers := map[string]string{
		"Authorization":          "token " + token,
		"Editor-Version":         "vscode/1.140.0",
		"Editor-Plugin-Version":  "copilot-chat/0.68.0",
		"Copilot-Integration-Id": "vscode-chat",
		"User-Agent":             "GitHubCopilotChat/0.68.0",
	}
	if err := quotaGetJSON(quotaCopilotUserURL, headers, &data); err != nil {
		q.Status, q.Error = "error", err.Error()
		var se *quotaStatusError
		if errors.As(err, &se) && (se.status == http.StatusUnauthorized || se.status == http.StatusForbidden) {
			q.Status = "expired"
		}
		return q
	}
	switch {
	case data.AccessSKU == "free_educational_quota":
		q.Plan = "Education"
	case data.AccessSKU == "free_limited_copilot":
		q.Plan = "Free"
	case copilotPlanNames[strings.ToLower(data.Plan)] != "":
		q.Plan = copilotPlanNames[strings.ToLower(data.Plan)]
	default:
		q.Plan = titleCasePlan(data.Plan)
	}
	var resets int64
	if t, err := time.Parse(time.RFC3339, data.Reset); err == nil {
		resets = t.UnixMilli()
	} else if t, err := time.Parse("2006-01-02", data.ResetDay); err == nil {
		resets = t.UnixMilli()
	}
	for _, item := range []struct{ id, name string }{
		{"premium_interactions", "Premium"}, {"chat", "Chat"}, {"completions", "Completions"},
	} {
		w, ok := data.Snapshots[item.id]
		if !ok {
			continue
		}
		if w.Unlimited {
			q.Windows = append(q.Windows, QuotaWindow{Name: item.name, Unlimited: true})
			continue
		}
		if !w.HasQuota || w.Entitlement <= 0 {
			continue
		}
		used := w.Entitlement - w.Remaining
		q.Windows = append(q.Windows, QuotaWindow{
			Name:      item.name,
			Used:      100 * used / w.Entitlement,
			ResetsAt:  resets,
			SpanHours: 24 * 30,
			Display:   fmt.Sprintf("%s / %s", compactFloat(used), compactFloat(w.Entitlement)),
		})
	}
	q.Status, q.ReadAt = "ok", time.Now().UnixMilli()
	return q
}

// ============ 公共 ============

type quotaStatusError struct {
	status int
	body   string
}

func (e *quotaStatusError) Error() string {
	if e.body != "" {
		return fmt.Sprintf("HTTP %d: %s", e.status, e.body)
	}
	return fmt.Sprintf("HTTP %d", e.status)
}

func quotaGetJSON(url string, headers map[string]string, dst any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := credentialClient(quotaHTTPTimeout).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.Join(strings.Fields(string(b)), " ")
		if !strings.HasPrefix(snippet, "{") {
			snippet = ""
		}
		return &quotaStatusError{status: resp.StatusCode, body: clipText(snippet, 160)}
	}
	return json.Unmarshal(b, dst)
}

func compactFloat(n float64) string {
	if n == float64(int64(n)) {
		return strconv.FormatInt(int64(n), 10)
	}
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(n, 'f', 2, 64), "0"), ".")
}

func clipText(s string, max int) string {
	if r := []rune(s); len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}
