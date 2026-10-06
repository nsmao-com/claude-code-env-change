package main

// 外部修改检测：命令行模式或手动编辑改了 config.json / router.json 时，
// 运行中的界面在几秒内重新加载，避免下次保存时用旧的内存状态把改动覆盖掉。

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"time"
)

type watchedFile struct {
	path string
	mod  time.Time
	size int64
}

func (w *watchedFile) changed() bool {
	info, err := os.Stat(w.path)
	if err != nil {
		return false
	}
	if info.ModTime().Equal(w.mod) && info.Size() == w.size {
		return false
	}
	w.mod, w.size = info.ModTime(), info.Size()
	return true
}

func startConfigWatch(ctx context.Context, app *App, rs *RouterService) {
	cfg := &watchedFile{path: app.configPath}
	cfg.changed()
	var router *watchedFile
	if p, err := rs.configPath(); err == nil {
		router = &watchedFile{path: p}
		router.changed()
	}
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			if cfg.changed() && app.reloadIfExternal() {
				emitAppEvent(ctx, "config:external-change")
			}
			if router != nil && router.changed() && rs.reloadIfExternal() {
				emitAppEvent(ctx, "router:external-change")
			}
		}
	}()
}

// reloadIfExternal 磁盘内容与内存不同（不是本进程写入的）时重新加载
func (a *App) reloadIfExternal() bool {
	disk, err := os.ReadFile(a.configPath)
	if err != nil || !json.Valid(disk) {
		return false
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	if a.configLoadErr != nil {
		return false
	}
	mem, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil || bytes.Equal(bytes.TrimSpace(disk), bytes.TrimSpace(mem)) {
		return false
	}
	return a.loadConfig() == nil
}

func (rs *RouterService) reloadIfExternal() bool {
	path, err := rs.configPath()
	if err != nil {
		return false
	}
	disk, err := os.ReadFile(path)
	if err != nil || !json.Valid(disk) {
		return false
	}
	var next RouterConfig
	if json.Unmarshal(disk, &next) != nil {
		return false
	}
	rs.normalizeConfig(&next)
	rs.mu.Lock()
	mem, _ := json.MarshalIndent(rs.config, "", "  ")
	if bytes.Equal(bytes.TrimSpace(disk), bytes.TrimSpace(mem)) {
		rs.mu.Unlock()
		return false
	}
	needRestart := rs.running && (next.Port != rs.config.Port || next.LANShare != rs.config.LANShare)
	rs.config = next
	rs.mu.Unlock()
	if needRestart {
		_ = rs.StopGateway()
		_ = rs.StartGateway()
	}
	return true
}
