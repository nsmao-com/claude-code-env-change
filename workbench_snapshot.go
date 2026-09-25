package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func (w *WorkbenchService) snapshot(reason string) (string, error) {
	paths := []string{w.app.configPath}
	for _, name := range []string{"mcp.json", "skills.json", "router.json", "workbench.json"} {
		p, e := storePath(name)
		if e != nil {
			return "", e
		}
		paths = append(paths, p)
	}
	for _, dir := range w.app.ListConfigDirs() {
		for _, file := range dir.Files {
			if file.Exists {
				paths = append(paths, file.Path)
			}
		}
	}
	files := []HistoryFile{}
	seen := map[string]bool{}
	for _, p := range paths {
		if p == "" || seen[p] || !historyAllowed(p) {
			continue
		}
		seen[p] = true
		b, e := os.ReadFile(p)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return "", e
		}
		files = append(files, HistoryFile{filepath.Clean(p), b})
	}
	return saveHistory(reason, files)
}
func (w *WorkbenchService) SnapshotProjectPreset(name string) (string, error) {
	c, e := w.GetWorkbench()
	if e != nil {
		return "", e
	}
	for _, p := range c.Projects {
		if p.Name == name {
			return w.snapshot("应用项目套装前 · " + name)
		}
	}
	return "", fmt.Errorf("项目套装不存在")
}
