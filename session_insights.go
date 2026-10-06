package main

// 会话洞察：扫描 Claude Code 与 Codex 的本地会话日志，按天汇总提问、回复、
// 工具调用、Skills、MCP、模型与 Token，用于回答"时间花在哪、哪些工具最常用"。
// 每个文件按 (大小, 修改时间) 缓存逐日聚合结果，切换时间范围不必重新解析。

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type InsightCount struct {
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Tokens int64  `json:"tokens,omitempty"`
}

type InsightDay struct {
	Date     string `json:"date"`
	Sessions int    `json:"sessions"`
	Prompts  int    `json:"prompts"`
	Tokens   int64  `json:"tokens"`
}

type InsightSession struct {
	Provider      string `json:"provider"`
	SessionID     string `json:"session_id"`
	Title         string `json:"title"`
	Project       string `json:"project"`
	Tokens        int64  `json:"tokens"`
	Prompts       int    `json:"prompts"`
	ToolCalls     int    `json:"tool_calls"`
	ActiveMinutes int64  `json:"active_minutes"`
	Last          int64  `json:"last"`
}

type SessionInsightQuery struct {
	Provider string `json:"provider"` // all | claude | codex
	Days     int    `json:"days"`     // 0 = 全部
	Project  string `json:"project"`
}

type SessionInsights struct {
	From             string           `json:"from"`
	To               string           `json:"to"`
	Sessions         int              `json:"sessions"`
	Prompts          int              `json:"prompts"`
	Replies          int              `json:"replies"`
	ToolCalls        int              `json:"tool_calls"`
	InputTokens      int64            `json:"input_tokens"`
	OutputTokens     int64            `json:"output_tokens"`
	CacheReadTokens  int64            `json:"cache_read_tokens"`
	CacheWriteTokens int64            `json:"cache_write_tokens"`
	ActiveMinutes    int64            `json:"active_minutes"`
	MedianTokens     int64            `json:"median_tokens"`
	P90Tokens        int64            `json:"p90_tokens"`
	Days             []InsightDay     `json:"days"`
	Hours            []int            `json:"hours"`
	Weekdays         []int            `json:"weekdays"`
	Projects         []InsightCount   `json:"projects"`
	Models           []InsightCount   `json:"models"`
	Tools            []InsightCount   `json:"tools"`
	Skills           []InsightCount   `json:"skills"`
	MCP              []InsightCount   `json:"mcp"`
	Lengths          []InsightCount   `json:"lengths"`
	Top              []InsightSession `json:"top"`
	Scanned          int              `json:"scanned"`
	Warnings         []string         `json:"warnings"`
}

// insightDayAgg 一个会话文件在某一天的汇总
type insightDayAgg struct {
	Prompts, Replies, ToolCalls   int
	Input, Output, CacheR, CacheW int64
	ActiveSeconds                 int64
	Hours                         [24]int
	Tools, Skills, MCP            map[string]int
	ModelTokens                   map[string]int64
}

type insightFile struct {
	size, mod int64
	provider  string
	session   string // 会话 ID；子代理文件归到父会话
	project   string
	title     string
	last      int64
	days      map[string]*insightDayAgg
}

var insightCache = struct {
	sync.Mutex
	files map[string]*insightFile
}{files: map[string]*insightFile{}}

func newInsightDay() *insightDayAgg {
	return &insightDayAgg{Tools: map[string]int{}, Skills: map[string]int{}, MCP: map[string]int{}, ModelTokens: map[string]int64{}}
}

func (f *insightFile) day(t time.Time) *insightDayAgg {
	key := t.Local().Format("2006-01-02")
	d := f.days[key]
	if d == nil {
		d = newInsightDay()
		f.days[key] = d
	}
	return d
}

// mcpServerOf 从工具名提取 MCP 服务器：Claude 为 mcp__server__tool
func mcpServerOf(name string) string {
	if !strings.HasPrefix(name, "mcp__") {
		return ""
	}
	rest := strings.TrimPrefix(name, "mcp__")
	server, _, ok := strings.Cut(rest, "__")
	if !ok || server == "" {
		return ""
	}
	return server
}

// insightActivity 记录一次活动：与上一次间隔不超过 5 分钟的部分计入活跃时长
type insightClock struct{ last time.Time }

func (c *insightClock) tick(f *insightFile, t time.Time) {
	if !c.last.IsZero() {
		if gap := t.Sub(c.last); gap > 0 && gap <= 5*time.Minute {
			f.day(t).ActiveSeconds += int64(gap.Seconds())
		}
	}
	if t.After(c.last) {
		c.last = t
	}
	if ms := t.UnixMilli(); ms > f.last {
		f.last = ms
	}
}

func isClaudePromptText(content any) (string, bool) {
	switch c := content.(type) {
	case string:
		return c, strings.TrimSpace(c) != ""
	case []any:
		parts := []string{}
		for _, x := range c {
			m, _ := x.(map[string]any)
			switch m["type"] {
			case "tool_result":
				return "", false
			case "text":
				if t, ok := m["text"].(string); ok {
					parts = append(parts, t)
				}
			}
		}
		text := strings.Join(parts, "\n")
		return text, strings.TrimSpace(text) != ""
	}
	return "", false
}

func parseClaudeInsightFile(path string, f *insightFile) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 64<<10)
	seenUsage := map[string]bool{}
	clock := &insightClock{}
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 && len(line) <= 16<<20 {
			var r struct {
				Type      string `json:"type"`
				Timestamp string `json:"timestamp"`
				Cwd       string `json:"cwd"`
				SessionID string `json:"sessionId"`
				RequestID string `json:"requestId"`
				IsMeta    bool   `json:"isMeta"`
				Message   *struct {
					ID      string `json:"id"`
					Role    string `json:"role"`
					Model   string `json:"model"`
					Content any    `json:"content"`
					Usage   *struct {
						Input  int64 `json:"input_tokens"`
						Output int64 `json:"output_tokens"`
						CacheR int64 `json:"cache_read_input_tokens"`
						CacheW int64 `json:"cache_creation_input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if json.Unmarshal(line, &r) == nil && r.Message != nil {
				if r.Cwd != "" && f.project == "" {
					f.project = r.Cwd
				}
				if r.SessionID != "" && f.session == "" {
					f.session = r.SessionID
				}
				ts, tsErr := parseTimestamp(r.Timestamp)
				if tsErr == nil {
					d := f.day(ts)
					switch r.Type {
					case "user":
						if text, ok := isClaudePromptText(r.Message.Content); ok && !r.IsMeta && !strings.HasPrefix(strings.TrimSpace(text), "<local-command-") {
							d.Prompts++
							d.Hours[ts.Local().Hour()]++
							if f.title == "" && !strings.HasPrefix(strings.TrimSpace(text), "<") {
								f.title = truncateSessionTitle(strings.TrimSpace(text))
							}
						}
						clock.tick(f, ts)
					case "assistant":
						if blocks, ok := r.Message.Content.([]any); ok {
							for _, x := range blocks {
								m, _ := x.(map[string]any)
								switch m["type"] {
								case "tool_use":
									name, _ := m["name"].(string)
									if name == "" {
										continue
									}
									d.ToolCalls++
									d.Tools[name]++
									if server := mcpServerOf(name); server != "" {
										d.MCP[server]++
									}
									if name == "Skill" {
										if input, ok := m["input"].(map[string]any); ok {
											if skill, _ := input["skill"].(string); skill != "" {
												d.Skills[skill]++
											} else if cmd, _ := input["command"].(string); cmd != "" {
												d.Skills[cmd]++
											}
										}
									}
								case "text":
									d.Replies++
								}
							}
						}
						if u := r.Message.Usage; u != nil {
							key := r.Message.ID + "|" + r.RequestID
							if r.Message.ID == "" || !seenUsage[key] {
								seenUsage[key] = true
								d.Input += u.Input
								d.Output += u.Output
								d.CacheR += u.CacheR
								d.CacheW += u.CacheW
								if r.Message.Model != "" && r.Message.Model != "<synthetic>" {
									d.ModelTokens[r.Message.Model] += u.Input + u.Output + u.CacheW
								}
							}
						}
						clock.tick(f, ts)
					}
				}
			}
		}
		if readErr != nil {
			break
		}
	}
	return nil
}

func parseCodexInsightFile(path string, f *insightFile) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 64<<10)
	model := ""
	var lastTotal *codexTokenUsage
	clock := &insightClock{}
	sawUserEvent, sawReplyEvent := false, false
	itemPrompts, itemReplies, itemHours := map[string]int{}, map[string]int{}, map[string]int{}
	itemTitle := ""
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 && len(line) <= 16<<20 {
			var r struct {
				Type      string          `json:"type"`
				Timestamp string          `json:"timestamp"`
				Payload   json.RawMessage `json:"payload"`
			}
			if json.Unmarshal(line, &r) == nil {
				ts, tsErr := parseTimestamp(r.Timestamp)
				var p struct {
					Type    string `json:"type"`
					ID      string `json:"id"`
					Cwd     string `json:"cwd"`
					Model   string `json:"model"`
					Role    string `json:"role"`
					Name    string `json:"name"`
					Message string `json:"message"`
					Content any    `json:"content"`
					Info    *struct {
						Total *codexTokenUsage `json:"total_token_usage"`
					} `json:"info"`
				}
				_ = json.Unmarshal(r.Payload, &p)
				switch r.Type {
				case "session_meta":
					if p.ID != "" {
						f.session = p.ID
					}
					if p.Cwd != "" {
						f.project = p.Cwd
					}
				case "turn_context":
					if p.Model != "" {
						model = p.Model
					}
				case "event_msg":
					if tsErr != nil {
						break
					}
					d := f.day(ts)
					switch p.Type {
					case "user_message":
						sawUserEvent = true
						d.Prompts++
						d.Hours[ts.Local().Hour()]++
						if f.title == "" && strings.TrimSpace(p.Message) != "" {
							f.title = truncateSessionTitle(strings.TrimSpace(p.Message))
						}
						clock.tick(f, ts)
					case "agent_message":
						sawReplyEvent = true
						d.Replies++
						clock.tick(f, ts)
					case "token_count":
						if p.Info == nil || p.Info.Total == nil {
							break
						}
						tc := p.Info.Total
						var in, cached, out int64
						if lastTotal != nil {
							in = int64(tc.InputTokens - lastTotal.InputTokens)
							cached = int64(tc.CachedInputTokens - lastTotal.CachedInputTokens)
							out = int64(tc.OutputTokens - lastTotal.OutputTokens)
						} else {
							in, cached, out = int64(tc.InputTokens), int64(tc.CachedInputTokens), int64(tc.OutputTokens)
						}
						lastTotal = tc
						if in < 0 || out < 0 || cached < 0 {
							break
						}
						uncached := in - cached
						if uncached < 0 {
							uncached = 0
						}
						d.Input += uncached
						d.CacheR += cached
						d.Output += out
						if model != "" {
							d.ModelTokens[model] += uncached + out
						}
					}
				case "response_item":
					if tsErr != nil {
						break
					}
					switch p.Type {
					case "message":
						// 新版 Codex 不再写 user_message / agent_message 事件，提问与回复只在 response_item 里
						text := strings.TrimSpace(messageText(p.Content))
						if text != "" {
							f.day(ts) // 保证该日有聚合项，合并时才能计入
						}
						if p.Role == "user" && text != "" && !isCodexInjected(text) {
							day := ts.Local().Format("2006-01-02")
							itemPrompts[day]++
							itemHours[day+"|"+fmt.Sprint(ts.Local().Hour())]++
							if itemTitle == "" {
								itemTitle = truncateSessionTitle(text)
							}
							clock.tick(f, ts)
						} else if p.Role == "assistant" && text != "" {
							itemReplies[ts.Local().Format("2006-01-02")]++
							clock.tick(f, ts)
						}
					case "function_call", "custom_tool_call", "local_shell_call", "web_search_call", "tool_search_call":
						name := p.Name
						if name == "" {
							name = strings.TrimSuffix(p.Type, "_call")
						}
						d := f.day(ts)
						d.ToolCalls++
						d.Tools[name]++
						if server := mcpServerOf(name); server != "" {
							d.MCP[server]++
						}
						clock.tick(f, ts)
					}
				}
			}
		}
		if readErr != nil {
			break
		}
	}
	if !sawUserEvent {
		for day, n := range itemPrompts {
			if d := f.days[day]; d != nil {
				d.Prompts += n
			}
		}
		for key, n := range itemHours {
			day, hour, _ := strings.Cut(key, "|")
			if d := f.days[day]; d != nil {
				var h int
				fmt.Sscan(hour, &h)
				if h >= 0 && h < 24 {
					d.Hours[h] += n
				}
			}
		}
		if f.title == "" {
			f.title = itemTitle
		}
	}
	if !sawReplyEvent {
		for day, n := range itemReplies {
			if d := f.days[day]; d != nil {
				d.Replies += n
			}
		}
	}
	return nil
}

// isCodexInjected Codex 自动注入到对话里的说明（AGENTS.md、环境信息等），不是用户的提问
func isCodexInjected(text string) bool {
	t := strings.TrimSpace(text)
	for _, prefix := range []string{"# AGENTS.md instructions", "<environment_context", "<user_instructions", "<INSTRUCTIONS>", "<permissions", "<turn_aborted", "<user_shell_command", "<subagent_notification", "<skill"} {
		if strings.HasPrefix(t, prefix) {
			return true
		}
	}
	return false
}

type insightTarget struct {
	provider, path string
	info           os.FileInfo
}

func collectInsightTargets(provider string, cutoff time.Time) ([]insightTarget, []string) {
	out := []insightTarget{}
	warnings := []string{}
	for p, roots := range sessionRoots() {
		if p == "antigravity" || provider != "" && provider != "all" && provider != p {
			continue
		}
		for _, root := range roots {
			err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
				if walkErr != nil || d.Type()&os.ModeSymlink != 0 || d.IsDir() || filepath.Ext(path) != ".jsonl" {
					return nil
				}
				info, err := d.Info()
				if err != nil {
					return nil
				}
				if !cutoff.IsZero() && info.ModTime().Before(cutoff) {
					return nil
				}
				out = append(out, insightTarget{provider: p, path: path, info: info})
				return nil
			})
			if err != nil && !os.IsNotExist(err) {
				warnings = append(warnings, err.Error())
			}
		}
	}
	return out, warnings
}

func loadInsightFile(t insightTarget) *insightFile {
	insightCache.Lock()
	cached := insightCache.files[t.path]
	insightCache.Unlock()
	if cached != nil && cached.size == t.info.Size() && cached.mod == t.info.ModTime().UnixNano() {
		return cached
	}
	f := &insightFile{size: t.info.Size(), mod: t.info.ModTime().UnixNano(), provider: t.provider, days: map[string]*insightDayAgg{}}
	if t.info.Size() > 512<<20 {
		return f
	}
	if t.provider == "claude" {
		_ = parseClaudeInsightFile(t.path, f)
		// 子代理日志：<项目>/<会话 ID>/subagents/agent-xxx.jsonl，归到父会话
		if parent := filepath.Base(filepath.Dir(t.path)); parent == "subagents" {
			f.session = filepath.Base(filepath.Dir(filepath.Dir(t.path)))
			f.title = ""
		}
	} else {
		_ = parseCodexInsightFile(t.path, f)
	}
	if f.session == "" {
		f.session = strings.TrimSuffix(filepath.Base(t.path), filepath.Ext(t.path))
	}
	insightCache.Lock()
	insightCache.files[t.path] = f
	insightCache.Unlock()
	return f
}

func topCounts(m map[string]int, tokens map[string]int64, limit int) []InsightCount {
	out := make([]InsightCount, 0, len(m))
	for k, v := range m {
		out = append(out, InsightCount{Name: k, Count: v, Tokens: tokens[k]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// GetSessionInsights 汇总指定范围内的会话洞察
func (ss *SessionService) GetSessionInsights(q SessionInsightQuery) (SessionInsights, error) {
	now := time.Now()
	var cutoff time.Time
	if q.Days > 0 {
		y, m, d := now.AddDate(0, 0, -(q.Days - 1)).Date()
		cutoff = time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	}
	targets, warnings := collectInsightTargets(q.Provider, cutoff)

	files := make([]*insightFile, len(targets))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for i, t := range targets {
		wg.Add(1)
		go func(i int, t insightTarget) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			files[i] = loadInsightFile(t)
		}(i, t)
	}
	wg.Wait()

	out := SessionInsights{Hours: make([]int, 24), Weekdays: make([]int, 7), Warnings: warnings, Scanned: len(targets), To: now.Format("2006-01-02")}
	if !cutoff.IsZero() {
		out.From = cutoff.Format("2006-01-02")
	}
	cutoffKey := out.From
	projectFilter := strings.ToLower(strings.TrimSpace(q.Project))

	type sessionAgg struct {
		InsightSession
		days map[string]bool
	}
	sessions := map[string]*sessionAgg{}
	dayTotals := map[string]*InsightDay{}
	daySessions := map[string]map[string]bool{}
	tools, skills, mcp, models := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	modelTokens := map[string]int64{}
	projects := map[string]int{}
	projectTokens := map[string]int64{}

	for _, f := range files {
		if f == nil || len(f.days) == 0 {
			continue
		}
		if projectFilter != "" && !strings.Contains(strings.ToLower(f.project), projectFilter) {
			continue
		}
		key := f.provider + ":" + f.session
		for date, d := range f.days {
			if cutoffKey != "" && date < cutoffKey {
				continue
			}
			if d.Prompts == 0 && d.Replies == 0 && d.ToolCalls == 0 && d.Output == 0 {
				continue
			}
			s := sessions[key]
			if s == nil {
				s = &sessionAgg{InsightSession: InsightSession{Provider: f.provider, SessionID: f.session, Project: f.project}, days: map[string]bool{}}
				sessions[key] = s
			}
			if s.Title == "" {
				s.Title = f.title
			}
			if s.Project == "" {
				s.Project = f.project
			}
			if f.last > s.Last {
				s.Last = f.last
			}
			tokens := d.Input + d.Output + d.CacheW
			s.Tokens += tokens
			s.Prompts += d.Prompts
			s.ToolCalls += d.ToolCalls
			s.ActiveMinutes += d.ActiveSeconds
			s.days[date] = true

			out.Prompts += d.Prompts
			out.Replies += d.Replies
			out.ToolCalls += d.ToolCalls
			out.InputTokens += d.Input
			out.OutputTokens += d.Output
			out.CacheReadTokens += d.CacheR
			out.CacheWriteTokens += d.CacheW
			out.ActiveMinutes += d.ActiveSeconds
			for h, n := range d.Hours {
				out.Hours[h] += n
			}
			if t, err := time.ParseInLocation("2006-01-02", date, time.Local); err == nil {
				out.Weekdays[(int(t.Weekday())+6)%7] += d.Prompts
			}
			dt := dayTotals[date]
			if dt == nil {
				dt = &InsightDay{Date: date}
				dayTotals[date] = dt
				daySessions[date] = map[string]bool{}
			}
			dt.Prompts += d.Prompts
			dt.Tokens += tokens
			daySessions[date][key] = true
			for k, v := range d.Tools {
				tools[k] += v
			}
			for k, v := range d.Skills {
				skills[k] += v
			}
			for k, v := range d.MCP {
				mcp[k] += v
			}
			for k, v := range d.ModelTokens {
				modelTokens[k] += v
				models[k]++
			}
			if f.project != "" {
				projectTokens[f.project] += tokens
			}
		}
	}

	out.Sessions = len(sessions)
	out.ActiveMinutes /= 60
	tokenList := make([]int64, 0, len(sessions))
	top := make([]InsightSession, 0, len(sessions))
	lengthBuckets := []struct {
		name string
		min  int
	}{{"1-5", 1}, {"6-15", 6}, {"16-30", 16}, {"31-60", 31}, {"61+", 61}}
	lengthCounts := make([]int, len(lengthBuckets))
	for _, s := range sessions {
		s.ActiveMinutes /= 60
		tokenList = append(tokenList, s.Tokens)
		top = append(top, s.InsightSession)
		if s.Project != "" {
			projects[s.Project]++
		}
		for i := len(lengthBuckets) - 1; i >= 0; i-- {
			if s.Prompts >= lengthBuckets[i].min {
				lengthCounts[i]++
				break
			}
		}
	}
	for i, b := range lengthBuckets {
		out.Lengths = append(out.Lengths, InsightCount{Name: b.name, Count: lengthCounts[i]})
	}
	sort.Slice(tokenList, func(i, j int) bool { return tokenList[i] < tokenList[j] })
	if n := len(tokenList); n > 0 {
		out.MedianTokens = tokenList[n/2]
		out.P90Tokens = tokenList[min(n-1, n*9/10)]
	}
	sort.Slice(top, func(i, j int) bool { return top[i].Tokens > top[j].Tokens })
	if len(top) > 8 {
		top = top[:8]
	}
	out.Top = top

	for date, d := range dayTotals {
		d.Sessions = len(daySessions[date])
		out.Days = append(out.Days, *d)
	}
	sort.Slice(out.Days, func(i, j int) bool { return out.Days[i].Date < out.Days[j].Date })
	if out.From == "" && len(out.Days) > 0 {
		out.From = out.Days[0].Date
	}

	out.Tools = topCounts(tools, nil, 15)
	out.Skills = topCounts(skills, nil, 12)
	out.MCP = topCounts(mcp, nil, 12)
	out.Projects = topCounts(projects, projectTokens, 10)
	// 模型按 Token 排序更有意义
	out.Models = topCounts(models, modelTokens, 0)
	sort.Slice(out.Models, func(i, j int) bool { return out.Models[i].Tokens > out.Models[j].Tokens })
	if len(out.Models) > 10 {
		out.Models = out.Models[:10]
	}
	return out, nil
}
