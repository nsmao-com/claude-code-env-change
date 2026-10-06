package main

// 会话回收站：删除会话时把原始日志（以及 Claude Code 同名的会话附属目录）移到
// ~/.claude-env-switcher/session-trash/，可随时恢复到原路径；清空回收站才会真正删除。

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const sessionTrashDir = "session-trash"

type TrashedSession struct {
	ID        string   `json:"id"`
	Provider  string   `json:"provider"`
	Title     string   `json:"title"`
	Project   string   `json:"project"`
	SessionID string   `json:"session_id"`
	Original  string   `json:"original"`
	Extras    []string `json:"extras,omitempty"` // 一并移走的附属路径（原路径）
	DeletedAt int64    `json:"deleted_at"`
	Size      int64    `json:"size"`
}

var sessionTrashMu sync.Mutex

func sessionTrashRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir, sessionTrashDir)
	return dir, os.MkdirAll(dir, 0o700)
}

func loadSessionTrash() ([]TrashedSession, string, error) {
	root, err := sessionTrashRoot()
	if err != nil {
		return nil, "", err
	}
	items := []TrashedSession{}
	b, err := os.ReadFile(filepath.Join(root, "index.json"))
	if err == nil {
		_ = json.Unmarshal(b, &items)
	} else if !os.IsNotExist(err) {
		return nil, root, err
	}
	return items, root, nil
}

func saveSessionTrash(root string, items []TrashedSession) error {
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(root, "index.json"), b, 0o600)
}

// movePath 先尝试重命名，跨盘时复制后删除
func movePath(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := copyDirTree(src, dst); err != nil {
			_ = os.RemoveAll(dst)
			return err
		}
		return os.RemoveAll(src)
	}
	if err := copyOneFile(src, dst, info.Mode()); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return os.Remove(src)
}

func copyOneFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyDirTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyOneFile(p, target, info.Mode())
	})
}

func pathSize(p string) int64 {
	var total int64
	_ = filepath.WalkDir(p, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, e := d.Info(); e == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// isUnderSessionRoot 只允许移动各 CLI 会话目录里的文件，防止被传入任意路径
func isUnderSessionRoot(p string) bool {
	clean := strings.ToLower(filepath.Clean(p))
	for _, roots := range sessionRoots() {
		for _, root := range roots {
			r := strings.ToLower(filepath.Clean(root))
			if r != "" && r != "." && strings.HasPrefix(clean, r+string(os.PathSeparator)) {
				return true
			}
		}
	}
	return false
}

// DeleteSession 把会话移入回收站
func (ss *SessionService) DeleteSession(id string) (TrashedSession, error) {
	item, err := ss.session(id)
	if err != nil {
		return TrashedSession{}, err
	}
	s := item.summary
	if !isUnderSessionRoot(s.Path) {
		return TrashedSession{}, fmt.Errorf("只能删除 CLI 会话目录中的会话")
	}
	if info, err := os.Stat(s.Path); err == nil && time.Since(info.ModTime()) < time.Minute {
		return TrashedSession{}, fmt.Errorf("该会话一分钟内仍有写入，可能正在使用，请结束会话后再删除")
	}
	sessionTrashMu.Lock()
	defer sessionTrashMu.Unlock()
	items, root, err := loadSessionTrash()
	if err != nil {
		return TrashedSession{}, err
	}
	entry := TrashedSession{
		ID:        fmt.Sprintf("%d-%s", time.Now().UnixMilli(), id[:min(12, len(id))]),
		Provider:  s.Provider,
		Title:     s.Title,
		Project:   s.Project,
		SessionID: s.SessionID,
		Original:  s.Path,
		DeletedAt: time.Now().UnixMilli(),
	}
	entryDir := filepath.Join(root, entry.ID)
	entry.Size = pathSize(s.Path)
	if err := movePath(s.Path, filepath.Join(entryDir, "0", filepath.Base(s.Path))); err != nil {
		return TrashedSession{}, fmt.Errorf("移动会话失败: %v", err)
	}
	// Claude Code 在会话文件旁用同名目录保存子代理与工具结果
	if s.Provider == "claude" {
		sibling := strings.TrimSuffix(s.Path, filepath.Ext(s.Path))
		if dirExists(sibling) && isUnderSessionRoot(sibling) {
			size := pathSize(sibling)
			if err := movePath(sibling, filepath.Join(entryDir, "1", filepath.Base(sibling))); err == nil {
				entry.Extras = append(entry.Extras, sibling)
				entry.Size += size
			}
		}
	}
	items = append(items, entry)
	if err := saveSessionTrash(root, items); err != nil {
		return entry, err
	}
	ss.mu.Lock()
	delete(ss.cache, s.Path)
	ss.mu.Unlock()
	return entry, nil
}

// ListTrashedSessions 回收站内容，最近删除的在前
func (ss *SessionService) ListTrashedSessions() ([]TrashedSession, error) {
	sessionTrashMu.Lock()
	defer sessionTrashMu.Unlock()
	items, _, err := loadSessionTrash()
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DeletedAt > items[j].DeletedAt })
	return items, nil
}

// RestoreTrashedSession 恢复到原路径；原路径已有同名文件时拒绝覆盖
func (ss *SessionService) RestoreTrashedSession(trashID string) error {
	sessionTrashMu.Lock()
	defer sessionTrashMu.Unlock()
	items, root, err := loadSessionTrash()
	if err != nil {
		return err
	}
	idx := -1
	for i, it := range items {
		if it.ID == trashID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("回收站里没有这条会话")
	}
	entry := items[idx]
	entryDir := filepath.Join(root, entry.ID)
	if _, err := os.Stat(entry.Original); err == nil {
		return fmt.Errorf("原位置已存在同名会话文件，未覆盖")
	}
	if !isUnderSessionRoot(entry.Original) {
		return fmt.Errorf("原路径不在 CLI 会话目录中，拒绝恢复")
	}
	if err := movePath(filepath.Join(entryDir, "0", filepath.Base(entry.Original)), entry.Original); err != nil {
		return fmt.Errorf("恢复会话失败: %v", err)
	}
	for _, extra := range entry.Extras {
		if _, err := os.Stat(extra); err == nil || !isUnderSessionRoot(extra) {
			continue
		}
		_ = movePath(filepath.Join(entryDir, "1", filepath.Base(extra)), extra)
	}
	_ = os.RemoveAll(entryDir)
	items = append(items[:idx], items[idx+1:]...)
	return saveSessionTrash(root, items)
}

// PurgeTrashedSessions 永久删除回收站中的会话；ids 为空表示清空全部
func (ss *SessionService) PurgeTrashedSessions(ids []string) (int, error) {
	sessionTrashMu.Lock()
	defer sessionTrashMu.Unlock()
	items, root, err := loadSessionTrash()
	if err != nil {
		return 0, err
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	kept := []TrashedSession{}
	removed := 0
	for _, it := range items {
		if len(ids) > 0 && !want[it.ID] {
			kept = append(kept, it)
			continue
		}
		// 条目 ID 只由本程序生成，仍防御性地拒绝包含路径分隔符的值
		if strings.ContainsAny(it.ID, `/\`) || strings.Contains(it.ID, "..") {
			kept = append(kept, it)
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, it.ID)); err != nil {
			kept = append(kept, it)
			continue
		}
		removed++
	}
	return removed, saveSessionTrash(root, kept)
}
