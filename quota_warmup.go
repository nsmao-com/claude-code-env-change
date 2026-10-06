package main

// 额度预热：订阅的 5 小时窗口从第一次使用开始计时。每天在设定时间（或窗口重置后）
// 发一个极小的请求让新窗口提前开始，工作到一半时窗口就会重置。
// Claude 通过本机 claude 命令发送；Codex 通过 ChatGPT 登录直接请求。
// “重置后预热”依赖已读取的额度窗口，需要开启后台检查。

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// WarmupSettings 预热设置
type WarmupSettings struct {
	Claude  bool     `json:"claude"`
	Codex   bool     `json:"codex"`
	Times   []string `json:"times,omitempty"` // 每天的预热时间，如 "07:00"
	OnReset bool     `json:"on_reset"`        // 5 小时窗口重置后立即预热
}

var warmTimeRE = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

var warmState = struct {
	sync.Mutex
	done      map[string]bool  // "claude|2026-10-05 07:00" 已执行
	lastReset map[string]int64 // provider -> 已处理过的 5h 重置时间
	lastRun   map[string]time.Time
	lastMsg   map[string]string
}{done: map[string]bool{}, lastReset: map[string]int64{}, lastRun: map[string]time.Time{}, lastMsg: map[string]string{}}

func validateWarmup(w WarmupSettings) error {
	for _, t := range w.Times {
		if !warmTimeRE.MatchString(t) {
			return fmt.Errorf("预热时间格式应为 HH:MM，如 07:00")
		}
	}
	if len(w.Times) > 6 {
		return fmt.Errorf("每天最多设置 6 个预热时间")
	}
	return nil
}

// warmClaude 用本机 claude 以 Haiku 发一句极短的话
func warmClaude(ctx context.Context) error {
	if _, _, signedIn := claudeLoginInfo(); !signedIn {
		return fmt.Errorf("Claude Code 未登录订阅")
	}
	bin := findCliBinary("claude")
	if bin == "" {
		return fmt.Errorf("未找到 claude 命令")
	}
	args := []string{"-p", "hi", "--model", "haiku", "--output-format", "json", "--tools", "", "--strict-mcp-config",
		"--setting-sources", "", "--no-session-persistence"}
	cmd := startToolCommand(ctx, bin, args, nil)
	env := make([]string, 0, len(cmd.Env))
	for _, kv := range cmd.Env {
		k, _, _ := strings.Cut(kv, "=")
		if !claudeBlockedEnv[strings.ToUpper(k)] {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, "DISABLE_TELEMETRY=1", "DISABLE_ERROR_REPORTING=1", "DISABLE_AUTOUPDATER=1")
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.CombinedOutput()
	if res, ok := claudePrintResult(out); ok {
		if res.IsError {
			return fmt.Errorf("Claude Code: %s", clipText(res.Result, 160))
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("运行 claude 失败: %v", err)
	}
	return nil
}

// warmCodex 用 ChatGPT 登录直接发一个最小的 Responses 请求
func warmCodex(ctx context.Context) error {
	rs := globalRouterService
	if rs == nil {
		rs = NewRouterService()
	}
	model := "gpt-5.4-mini"
	if models, err := rs.ListAccountModels("codex"); err == nil && len(models) > 0 {
		model = models[0]
		for _, m := range models {
			if strings.Contains(m, "mini") {
				model = m
				break
			}
		}
	}
	body := fmt.Sprintf(`{"model":%q,"input":"hi","stream":false,"reasoning":{"effort":"low"}}`, model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, accountScheme+"codex/v1/responses", strings.NewReader(body))
	if err != nil {
		return err
	}
	resp, err := rs.sendUpstream(req, accountScheme+"codex")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ChatGPT 返回 HTTP %d: %s", resp.StatusCode, clipText(strings.Join(strings.Fields(string(b)), " "), 160))
	}
	return nil
}

func runWarmup(provider string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var err error
	switch provider {
	case "claude":
		err = warmClaude(ctx)
	case "codex":
		err = warmCodex(ctx)
	default:
		err = fmt.Errorf("不支持预热 %s", provider)
	}
	warmState.Lock()
	warmState.lastRun[provider] = time.Now()
	if err != nil {
		warmState.lastMsg[provider] = err.Error()
	} else {
		warmState.lastMsg[provider] = ""
	}
	warmState.Unlock()
	return err
}

// checkWarmup 在后台监控的每分钟检查里调用
func (qs *QuotaService) checkWarmup(now time.Time) {
	s := qs.GetQuotaSettings().Warmup
	if !s.Claude && !s.Codex {
		return
	}
	providers := []string{}
	if s.Claude {
		providers = append(providers, "claude")
	}
	if s.Codex {
		providers = append(providers, "codex")
	}
	due := map[string]bool{}
	for _, t := range s.Times {
		at, err := time.ParseInLocation("2006-01-02 15:04", now.Format("2006-01-02")+" "+t, now.Location())
		// 每分钟检查一次，计时器有漂移，留 3 分钟窗口避免错过
		if err != nil || now.Before(at) || now.Sub(at) >= 3*time.Minute {
			continue
		}
		for _, p := range providers {
			key := p + "|" + now.Format("2006-01-02") + " " + t
			warmState.Lock()
			if !warmState.done[key] {
				warmState.done[key] = true
				due[p] = true
			}
			warmState.Unlock()
		}
	}
	if s.OnReset {
		qs.mu.Lock()
		quotas := qs.snapshotLocked()
		qs.mu.Unlock()
		for _, q := range quotas {
			if (q.Provider == "claude" && !s.Claude) || (q.Provider == "codex" && !s.Codex) || q.Provider == "copilot" {
				continue
			}
			for _, w := range q.Windows {
				if w.SpanHours != 5 || w.ResetsAt == 0 || now.UnixMilli() < w.ResetsAt {
					continue
				}
				warmState.Lock()
				if warmState.lastReset[q.Provider] != w.ResetsAt {
					warmState.lastReset[q.Provider] = w.ResetsAt
					due[q.Provider] = true
				}
				warmState.Unlock()
			}
		}
	}
	for p := range due {
		go func(p string) { _ = runWarmup(p) }(p)
	}
}

// WarmupNow 立即预热一次（界面上的“立即预热”）
func (qs *QuotaService) WarmupNow(provider string) error {
	return runWarmup(provider)
}

// WarmupStatus 最近一次预热结果
type WarmupStatus struct {
	Provider string `json:"provider"`
	LastRun  int64  `json:"last_run,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (qs *QuotaService) GetWarmupStatus() []WarmupStatus {
	warmState.Lock()
	defer warmState.Unlock()
	out := []WarmupStatus{}
	for _, p := range []string{"claude", "codex"} {
		st := WarmupStatus{Provider: p, Error: warmState.lastMsg[p]}
		if t, ok := warmState.lastRun[p]; ok {
			st.LastRun = t.UnixMilli()
		}
		out = append(out, st)
	}
	return out
}
