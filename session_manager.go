package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type SessionSummary struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Provider  string `json:"provider"`
	Title     string `json:"title"`
	Project   string `json:"project"`
	Model     string `json:"model"`
	Updated   int64  `json:"updated"`
	Messages  int    `json:"messages"`
	Archived  bool   `json:"archived"`
	Path      string `json:"path"`
	Warning   string `json:"warning,omitempty"`
}
type SessionMessage struct {
	Role string `json:"role"`
	Text string `json:"text"`
	At   string `json:"at"`
}
type SessionQuery struct {
	Keyword  string `json:"keyword"`
	Provider string `json:"provider"`
	Project  string `json:"project"`
	Archived bool   `json:"archived"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
	From     int64  `json:"from"`
	To       int64  `json:"to"`
}
type SessionPage struct {
	Items    []SessionSummary `json:"items"`
	Total    int              `json:"total"`
	Warnings []string         `json:"warnings"`
}
type SessionDetail struct {
	Session  SessionSummary   `json:"session"`
	Messages []SessionMessage `json:"messages"`
	Total    int              `json:"total"`
}
type sessionCacheItem struct {
	size, modified int64
	summary        SessionSummary
	messages       []SessionMessage
	offset         int64
	prefix         []byte
}
type SessionService struct {
	app   *App
	mu    sync.Mutex
	cache map[string]sessionCacheItem
}

func NewSessionService(a *App) *SessionService {
	return &SessionService{app: a, cache: map[string]sessionCacheItem{}}
}
func sessionRoots() map[string][]string {
	home, _ := os.UserHomeDir()
	return map[string][]string{"claude": {NewLogService().getClaudeProjectsDir()}, "codex": {filepath.Join(resolveCodexHome(home), "sessions"), filepath.Join(resolveCodexHome(home), "archived_sessions")}, "antigravity": {filepath.Join(home, ".gemini", "tmp")}}
}
func messageText(v any) string {
	switch c := v.(type) {
	case string:
		return c
	case []any:
		parts := []string{}
		for _, x := range c {
			if m, ok := x.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					parts = append(parts, t)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}
func parseSessionLine(line []byte, s *SessionSummary) []SessionMessage {
	var r map[string]any
	if json.Unmarshal(line, &r) != nil {
		return nil
	}
	kind, _ := r["type"].(string)
	at, _ := r["timestamp"].(string)
	if cwd, ok := r["cwd"].(string); ok {
		s.Project = cwd
	}
	if id, ok := r["sessionId"].(string); ok {
		s.SessionID = id
	}
	p, _ := r["payload"].(map[string]any)
	if kind == "session_meta" {
		if id, ok := p["id"].(string); ok {
			s.SessionID = id
		}
		if cwd, ok := p["cwd"].(string); ok {
			s.Project = cwd
		}
		return nil
	}
	if kind == "turn_context" {
		if m, ok := p["model"].(string); ok {
			s.Model = m
		}
		return nil
	}
	var msg map[string]any
	if s.Provider == "claude" {
		msg, _ = r["message"].(map[string]any)
	} else if kind == "response_item" {
		msg = p
	}
	if msg == nil {
		return nil
	}
	role, _ := msg["role"].(string)
	if role != "user" && role != "assistant" {
		return nil
	}
	if model, ok := msg["model"].(string); ok {
		s.Model = model
	}
	text := messageText(msg["content"])
	if strings.TrimSpace(text) == "" {
		return nil
	}
	if role == "user" && (s.Title == "" || s.Title == "未命名会话") {
		runes := []rune(text)
		if len(runes) > 100 {
			runes = runes[:100]
		}
		s.Title = string(runes)
	}
	return []SessionMessage{{role, text, at}}
}
func (ss *SessionService) indexFile(provider, p string, info os.FileInfo) sessionCacheItem {
	old, exists := ss.cache[p]
	if exists && old.size == info.Size() && old.modified == info.ModTime().UnixNano() {
		return old
	}
	item := sessionCacheItem{size: info.Size(), modified: info.ModTime().UnixNano(), summary: SessionSummary{ID: contentHash([]byte(p)), Provider: provider, Path: p, Updated: info.ModTime().UnixMilli(), SessionID: strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))}, messages: []SessionMessage{}}
	if provider == "antigravity" {
		if info.Size() > 32<<20 {
			item.summary.Warning = "会话超过 32 MB"
			return item
		}
		b, e := os.ReadFile(p)
		if e != nil || len(b) > 32<<20 {
			item.summary.Warning = "会话读取失败或超过 32 MB"
			return item
		}
		var root map[string]any
		if json.Unmarshal(b, &root) != nil {
			return item
		}
		item.summary.SessionID, _ = root["sessionId"].(string)
		item.summary.Project, _ = root["projectHash"].(string)
		messages, _ := root["messages"].([]any)
		for _, v := range messages {
			m, ok := v.(map[string]any)
			if !ok {
				continue
			}
			role, _ := m["type"].(string)
			if role == "gemini" {
				role = "assistant"
			}
			if role != "user" && role != "assistant" {
				continue
			}
			text := messageText(m["content"])
			if text != "" {
				at, _ := m["timestamp"].(string)
				item.messages = append(item.messages, SessionMessage{role, text, at})
				if item.summary.Title == "" && role == "user" {
					item.summary.Title = truncateSessionTitle(text)
				}
			}
		}
	} else {
		f, e := os.Open(p)
		if info.Size() > 256<<20 {
			if e == nil {
				f.Close()
			}
			item.summary.Warning = "会话超过 256 MB，请通过 CLI 查看"
			return item
		}
		if e != nil {
			item.summary.Warning = e.Error()
			return item
		}
		defer f.Close()
		item.prefix = make([]byte, min(info.Size(), 4096))
		_, _ = io.ReadFull(f, item.prefix)
		_, _ = f.Seek(0, io.SeekStart)
		if exists && info.Size() > old.size && old.offset > 0 && bytes.HasPrefix(item.prefix, old.prefix) {
			item.summary = old.summary
			item.messages = append([]SessionMessage(nil), old.messages...)
			item.offset = old.offset
			_, e = f.Seek(old.offset, io.SeekStart)
			if e != nil {
				item.summary.Warning = e.Error()
				return item
			}
		}
		reader := bufio.NewReaderSize(f, 64<<10)
		for {
			line, e := reader.ReadBytes('\n')
			if e == io.EOF {
				if len(line) > 0 && len(line) <= 10<<20 && json.Valid(line) {
					item.messages = append(item.messages, parseSessionLine(line, &item.summary)...)
					item.offset += int64(len(line))
				}
				break
			}
			if e != nil {
				item.summary.Warning = e.Error()
				break
			}
			item.offset += int64(len(line))
			if len(line) > 10<<20 {
				item.summary.Warning = "已跳过超过 10 MB 的单条消息"
				continue
			}
			item.messages = append(item.messages, parseSessionLine(line, &item.summary)...)
			if len(item.messages) > 100000 {
				item.summary.Warning = "消息超过 100000 条，请导出原始日志处理"
				break
			}
		}
	}
	item.summary.Updated = info.ModTime().UnixMilli()
	item.summary.Messages = len(item.messages)
	if item.summary.Title == "" {
		item.summary.Title = "未命名会话"
	}
	return item
}
func truncateSessionTitle(text string) string {
	r := []rune(text)
	if len(r) > 100 {
		r = r[:100]
	}
	return string(r)
}
func (ss *SessionService) ListSessions(q SessionQuery) (SessionPage, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	result := SessionPage{Items: []SessionSummary{}, Warnings: []string{}}
	workbenchMu.Lock()
	cfg, e := loadWorkbench()
	workbenchMu.Unlock()
	if e != nil {
		return result, e
	}
	archived := map[string]bool{}
	for _, id := range cfg.ArchivedSessions {
		archived[id] = true
	}
	alive := map[string]bool{}
	matches := []SessionSummary{}
	keyword := strings.ToLower(strings.TrimSpace(q.Keyword))
	for provider, roots := range sessionRoots() {
		if q.Provider != "" && q.Provider != "all" && q.Provider != provider {
			continue
		}
		for _, root := range roots {
			err := filepath.WalkDir(root, func(p string, d os.DirEntry, walkErr error) error {
				if walkErr != nil {
					if !os.IsNotExist(walkErr) {
						result.Warnings = append(result.Warnings, walkErr.Error())
					}
					return nil
				}
				if d.Type()&os.ModeSymlink != 0 {
					return nil
				}
				if d.IsDir() {
					return nil
				}
				if provider == "antigravity" {
					if !strings.HasPrefix(d.Name(), "session-") || filepath.Ext(p) != ".json" {
						return nil
					}
				} else if filepath.Ext(p) != ".jsonl" {
					return nil
				}
				info, e := d.Info()
				if e != nil {
					return nil
				}
				item := ss.indexFile(provider, p, info)
				ss.cache[p] = item
				alive[p] = true
				s := item.summary
				s.Archived = archived[s.ID] || strings.Contains(filepath.ToSlash(p), "/archived_sessions/")
				if s.Archived != q.Archived {
					return nil
				}
				if q.Project != "" && !strings.Contains(strings.ToLower(s.Project), strings.ToLower(q.Project)) {
					return nil
				}
				if q.From > 0 && s.Updated < q.From || q.To > 0 && s.Updated > q.To {
					return nil
				}
				if keyword != "" {
					found := strings.Contains(strings.ToLower(s.Title+" "+s.Project+" "+s.Model), keyword)
					if !found {
						for _, m := range item.messages {
							if strings.Contains(strings.ToLower(m.Text), keyword) {
								found = true
								break
							}
						}
					}
					if !found {
						return nil
					}
				}
				matches = append(matches, s)
				return nil
			})
			if err != nil && !os.IsNotExist(err) {
				result.Warnings = append(result.Warnings, err.Error())
			}
		}
	}
	for p, item := range ss.cache {
		if (q.Provider == "" || q.Provider == "all" || q.Provider == item.summary.Provider) && !alive[p] {
			delete(ss.cache, p)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Updated > matches[j].Updated })
	result.Total = len(matches)
	if q.Offset < 0 {
		q.Offset = 0
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 30
	}
	if q.Offset < len(matches) {
		end := q.Offset + q.Limit
		if end > len(matches) {
			end = len(matches)
		}
		result.Items = matches[q.Offset:end]
	}
	return result, nil
}
func (ss *SessionService) session(id string) (sessionCacheItem, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for p, item := range ss.cache {
		if item.summary.ID == id {
			info, e := os.Stat(p)
			if e != nil {
				return item, e
			}
			item = ss.indexFile(item.summary.Provider, p, info)
			workbenchMu.Lock()
			cfg, e := loadWorkbench()
			workbenchMu.Unlock()
			if e != nil {
				return item, e
			}
			item.summary.Archived = strings.Contains(filepath.ToSlash(p), "/archived_sessions/")
			for _, id := range cfg.ArchivedSessions {
				if id == item.summary.ID {
					item.summary.Archived = true
				}
			}
			ss.cache[p] = item
			return item, nil
		}
	}
	return sessionCacheItem{}, fmt.Errorf("请先刷新会话列表")
}
func (ss *SessionService) GetSession(id string, offset, limit int) (SessionDetail, error) {
	item, e := ss.session(id)
	if e != nil {
		return SessionDetail{}, e
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	detail := SessionDetail{Session: item.summary, Total: len(item.messages), Messages: []SessionMessage{}}
	if offset < len(item.messages) {
		end := offset + limit
		if end > len(item.messages) {
			end = len(item.messages)
		}
		detail.Messages = item.messages[offset:end]
	}
	return detail, nil
}
func (ss *SessionService) SessionMarkdown(id string) (string, error) {
	item, e := ss.session(id)
	if e != nil {
		return "", e
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n工具：%s\n\n项目：%s\n\n", item.summary.Title, item.summary.Provider, item.summary.Project)
	for _, m := range item.messages {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", m.Role, m.Text)
	}
	return b.String(), nil
}
func (ss *SessionService) ExportSession(id string) (string, error) {
	body, e := ss.SessionMarkdown(id)
	if e != nil {
		return "", e
	}
	target, e := runtime.SaveFileDialog(ss.app.ctx, runtime.SaveDialogOptions{Title: "导出会话 Markdown", DefaultFilename: "conversation-" + id[:12] + ".md", Filters: []runtime.FileFilter{{DisplayName: "Markdown", Pattern: "*.md"}}})
	if e != nil || target == "" {
		return "", e
	}
	return target, writeFileAtomic(target, []byte(body), 0600)
}
func (ss *SessionService) OpenSessionDirectory(id string) error {
	item, e := ss.session(id)
	if e != nil {
		return e
	}
	return revealInFileManager(item.summary.Path)
}
func (ss *SessionService) ArchiveSession(id string, archived bool) error {
	item, e := ss.session(id)
	if e != nil {
		return e
	}
	if !archived && strings.Contains(filepath.ToSlash(item.summary.Path), "/archived_sessions/") {
		return fmt.Errorf("这是 Codex 原生归档，请在 Codex 中恢复会话")
	}
	workbenchMu.Lock()
	defer workbenchMu.Unlock()
	c, e := loadWorkbench()
	if e != nil {
		return e
	}
	out := []string{}
	for _, s := range c.ArchivedSessions {
		if s != id {
			out = append(out, s)
		}
	}
	if archived {
		out = append(out, id)
	}
	c.ArchivedSessions = out
	return saveWorkbench(c)
}
func (ss *SessionService) ResumeSession(id string) error {
	item, e := ss.session(id)
	if e != nil {
		return e
	}
	s := item.summary
	if !regexp.MustCompile(`^[a-fA-F0-9-]{16,64}$`).MatchString(s.SessionID) {
		return fmt.Errorf("无法识别可恢复的会话 ID")
	}
	if !filepath.IsAbs(s.Project) || !dirExists(s.Project) {
		return fmt.Errorf("会话项目目录不存在，请先恢复项目目录")
	}
	args := []string{}
	switch s.Provider {
	case "codex":
		args = []string{"resume", s.SessionID}
	case "claude":
		args = []string{"--resume", s.SessionID}
	default:
		return fmt.Errorf("此工具暂未提供兼容的会话恢复命令")
	}
	return openCommandTerminal(s.Provider, args, s.Project)
}
