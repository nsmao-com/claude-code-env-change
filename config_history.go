package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const historyLimit = 100

var historyMu sync.Mutex

type HistoryFile struct {
	Path string `json:"path"`
	Data []byte `json:"data"`
}
type HistoryEntry struct {
	ID     string        `json:"id"`
	At     int64         `json:"at"`
	Reason string        `json:"reason"`
	Files  []HistoryFile `json:"files"`
}
type HistorySummary struct {
	ID     string   `json:"id"`
	At     int64    `json:"at"`
	Reason string   `json:"reason"`
	Paths  []string `json:"paths"`
}
type FileChange struct {
	Path    string `json:"path"`
	Before  string `json:"before"`
	After   string `json:"after"`
	Changed bool   `json:"changed"`
}
type HistoryPreview struct {
	Token   string       `json:"token"`
	Changes []FileChange `json:"changes"`
}

func storePath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}
func contentHash(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func newRecordID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
func pathWithin(root, path string) bool {
	r, err := filepath.Rel(root, path)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(os.PathSeparator)) && !filepath.IsAbs(r)
}
func historyAllowed(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	base := strings.ToLower(filepath.Base(abs))
	if strings.HasSuffix(base, ".bak") || strings.Contains(base, ".tmp-") {
		return false
	}
	if strings.EqualFold(abs, resolveMainConfigPath()) {
		return true
	}
	central := filepath.Join(home, mcpStoreDir)
	if pathWithin(central, abs) {
		return filepath.Dir(abs) == central && (base == "config.json" || base == "mcp.json" || base == "skills.json" || base == "router.json" || base == "workbench.json" || base == "cloud.json" || base == "uptime.json")
	}
	if strings.EqualFold(abs, filepath.Join(home, ".claude.json")) {
		return true
	}
	if desktop, e := claudeDesktopConfigPath(); e == nil && strings.EqualFold(abs, desktop) {
		return true
	}
	for _, root := range []string{filepath.Join(home, ".claude"), os.Getenv("CLAUDE_CONFIG_DIR"), resolveCodexHome(home), filepath.Join(home, ".gemini"), resolveOpencodeConfigDir(nil), filepath.Join(home, ".grok"), os.Getenv("GROK_HOME")} {
		if root != "" && pathWithin(root, abs) && (strings.HasSuffix(base, ".json") || strings.HasSuffix(base, ".toml") || strings.HasSuffix(base, ".md") || base == ".env") {
			return true
		}
	}
	return false
}
func historyDir() (string, error) {
	p, e := storePath("history")
	if e == nil {
		e = os.MkdirAll(p, 0700)
	}
	return p, e
}
func saveHistory(reason string, files []HistoryFile) (string, error) {
	if len(files) == 0 {
		return "", nil
	}
	historyMu.Lock()
	defer historyMu.Unlock()
	dir, err := historyDir()
	if err != nil {
		return "", err
	}
	e := HistoryEntry{newRecordID(), time.Now().UnixMilli(), reason, files}
	data, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	if len(data) > 64<<20 {
		return "", fmt.Errorf("备份超过 64 MB，请减少范围")
	}
	if err = writeFileAtomic(filepath.Join(dir, e.ID+".json"), data, 0600); err != nil {
		return "", err
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) > historyLimit {
		sort.Slice(entries, func(i, j int) bool {
			a, _ := entries[i].Info()
			b, _ := entries[j].Info()
			return a != nil && b != nil && a.ModTime().Before(b.ModTime())
		})
		for _, old := range entries[:len(entries)-historyLimit] {
			if !old.IsDir() && strings.HasSuffix(old.Name(), ".json") {
				_ = os.Remove(filepath.Join(dir, old.Name()))
			}
		}
	}
	return e.ID, nil
}
func captureBeforeWrite(path string, next []byte) error {
	if !historyAllowed(path) {
		return nil
	}
	// Package attachments are backed up together in skills.json. Per-file snapshots
	// would evict that package snapshot when a large package is synchronized.
	if strings.Contains(filepath.ToSlash(path), "/skills/") && !strings.EqualFold(filepath.Base(path), "SKILL.md") {
		return nil
	}
	previous, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if bytes.Equal(previous, next) {
		return nil
	}
	_, err = saveHistory("写入前备份", []HistoryFile{{path, previous}})
	return err
}

var secretLine = regexp.MustCompile(`(?im)([\w.\-]*(?:key|token|secret|password|passphrase|authorization|credential)[\w.\-]*["']?\s*[:=]\s*)([^\r\n]+)`)

func redactConfig(data []byte) string {
	var value any
	if json.Unmarshal(data, &value) == nil {
		redactJSON(value)
		b, _ := json.MarshalIndent(value, "", "  ")
		return string(b)
	}
	return secretLine.ReplaceAllString(string(data), "${1}\"••••••\"")
}
func redactJSON(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			low := strings.ToLower(k)
			if strings.Contains(low, "token") && (strings.HasSuffix(low, "tokens") || strings.Contains(low, "limit") || strings.Contains(low, "budget")) {
				continue
			}
			if low == "files" {
				if files, ok := val.(map[string]any); ok {
					for name := range files {
						files[name] = "[附件内容已隐藏]"
					}
				}
			} else if strings.Contains(low, "key") || strings.Contains(low, "token") || strings.Contains(low, "secret") || strings.Contains(low, "password") || strings.Contains(low, "passphrase") || strings.Contains(low, "credential") || low == "authorization" {
				x[k] = "••••••"
			} else if text, ok := val.(string); ok {
				x[k] = redactEmbedded(text)
			} else {
				redactJSON(val)
			}
		}
	case []any:
		for _, item := range x {
			redactJSON(item)
		}
	}
}
func redactEmbedded(text string) string {
	var embedded any
	if json.Unmarshal([]byte(text), &embedded) == nil {
		switch embedded.(type) {
		case map[string]any, []any:
			redactJSON(embedded)
			b, _ := json.MarshalIndent(embedded, "", "  ")
			return string(b)
		}
	}
	return secretLine.ReplaceAllString(text, "${1}\"••••••\"")
}
func loadHistory(id string) (HistoryEntry, error) {
	var e HistoryEntry
	if !regexp.MustCompile(`^[a-f0-9]{24}$`).MatchString(id) {
		return e, fmt.Errorf("无效的历史记录")
	}
	dir, err := historyDir()
	if err != nil {
		return e, err
	}
	b, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		return e, err
	}
	err = json.Unmarshal(b, &e)
	return e, err
}
func (a *App) ListConfigHistory() ([]HistorySummary, error) {
	dir, err := historyDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := []HistorySummary{}
	for _, file := range entries {
		if file.IsDir() {
			continue
		}
		e, err := loadHistory(strings.TrimSuffix(file.Name(), ".json"))
		if err != nil {
			continue
		}
		s := HistorySummary{ID: e.ID, At: e.At, Reason: e.Reason, Paths: []string{}}
		for _, f := range e.Files {
			s.Paths = append(s.Paths, f.Path)
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At > out[j].At })
	return out, nil
}
func previewHistoryFiles(files []HistoryFile) (HistoryPreview, error) {
	p := HistoryPreview{Changes: []FileChange{}}
	h := sha256.New()
	for _, f := range files {
		if !historyAllowed(f.Path) {
			return p, fmt.Errorf("该路径不属于本机管理的配置: %s", f.Path)
		}
		b, err := os.ReadFile(f.Path)
		if err != nil && !os.IsNotExist(err) {
			return p, err
		}
		h.Write([]byte(f.Path))
		h.Write([]byte(contentHash(b)))
		h.Write([]byte(contentHash(f.Data)))
		p.Changes = append(p.Changes, FileChange{f.Path, redactConfig(b), redactConfig(f.Data), !bytes.Equal(b, f.Data)})
	}
	p.Token = hex.EncodeToString(h.Sum(nil))
	return p, nil
}
func (a *App) PreviewConfigHistory(id string) (HistoryPreview, error) {
	e, err := loadHistory(id)
	if err != nil {
		return HistoryPreview{}, err
	}
	return previewHistoryFiles(e.Files)
}
func (a *App) RestoreConfigHistory(id, token string) error {
	return a.RestoreConfigHistoryFiles(id, token, nil)
}
func (a *App) RestoreConfigHistoryFiles(id, token string, paths []string) error {
	e, err := loadHistory(id)
	if err != nil {
		return err
	}
	preview, err := previewHistoryFiles(e.Files)
	if err != nil {
		return err
	}
	if token == "" || token != preview.Token {
		return fmt.Errorf("配置已变化，请重新预览后恢复")
	}
	restored := []HistoryFile{}
	for _, f := range e.Files {
		if paths != nil {
			selected := false
			for _, p := range paths {
				if p == f.Path {
					selected = true
				}
			}
			if !selected {
				continue
			}
		}
		if err = os.MkdirAll(filepath.Dir(f.Path), 0700); err != nil {
			return err
		}
		if err = writeFileAtomic(f.Path, f.Data, 0600); err != nil {
			return err
		}
		restored = append(restored, f)
	}
	if err = a.RefreshConfig(); err != nil {
		return err
	}
	if cloudSyncInst != nil {
		cs := cloudSyncInst
		for _, f := range restored {
			switch filepath.Base(f.Path) {
			case "router.json":
				if cs.router != nil {
					if err = cs.router.ReloadFromDisk(); err != nil {
						return err
					}
				}
			case "mcp.json":
				if cs.mcp != nil {
					if err = cs.mcp.applyStoreToPlatforms(); err != nil {
						return err
					}
				}
			case "skills.json":
				if cs.skills != nil {
					if err = cs.skills.applyStoreToPlatforms(); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
