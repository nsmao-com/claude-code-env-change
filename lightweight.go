package main

// 轻量模式：主窗口隐藏到托盘 30 秒后，把本程序及其 WebView2 子进程的工作集交还给系统
// （任务管理器里的“内存”随之下降），隐藏期间每隔几分钟再做一次；窗口再显示时页面按需换回，
// 不需要重新载入界面。网关、额度监控、托盘等后台功能不受影响。

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"
)

const (
	lightweightDelay  = 30 * time.Second
	lightweightRepeat = 5 * time.Minute
)

type uiPrefs struct {
	Lightweight bool `json:"lightweight"`
}

var lightweight struct {
	sync.Mutex
	loaded bool
	prefs  uiPrefs
	timer  *time.Timer
	hidden bool
}

func loadUIPrefsLocked() {
	if lightweight.loaded {
		return
	}
	lightweight.loaded = true
	if p, err := storePath("ui.json"); err == nil {
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &lightweight.prefs)
		}
	}
}

// GetLightweightMode 是否开启轻量模式
func (a *App) GetLightweightMode() bool {
	lightweight.Lock()
	defer lightweight.Unlock()
	loadUIPrefsLocked()
	return lightweight.prefs.Lightweight
}

// SetLightweightMode 开关轻量模式
func (a *App) SetLightweightMode(on bool) error {
	lightweight.Lock()
	defer lightweight.Unlock()
	loadUIPrefsLocked()
	lightweight.prefs.Lightweight = on
	if !on && lightweight.timer != nil {
		lightweight.timer.Stop()
		lightweight.timer = nil
	}
	p, err := storePath("ui.json")
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(lightweight.prefs, "", "  ")
	return writeFileAtomic(p, b, 0o600)
}

// HideToTray 界面右上角的关闭按钮：有托盘时隐藏到托盘，返回 false 表示没有托盘（由界面自行退出）
func (a *App) HideToTray() bool {
	return trayHideMain()
}

// lightweightOnHide 窗口隐藏到托盘：开启轻量模式时稍后释放内存，隐藏期间定期重复
func lightweightOnHide(ctx context.Context) {
	lightweight.Lock()
	defer lightweight.Unlock()
	loadUIPrefsLocked()
	lightweight.hidden = true
	if !lightweight.prefs.Lightweight {
		return
	}
	if lightweight.timer != nil {
		lightweight.timer.Stop()
	}
	var trim func()
	trim = func() {
		lightweight.Lock()
		still := lightweight.hidden && lightweight.prefs.Lightweight
		lightweight.Unlock()
		if !still {
			return
		}
		trimProcessTreeMemory()
		lightweight.Lock()
		if lightweight.hidden {
			lightweight.timer = time.AfterFunc(lightweightRepeat, trim)
		}
		lightweight.Unlock()
	}
	lightweight.timer = time.AfterFunc(lightweightDelay, trim)
}

// lightweightOnShow 窗口重新显示：停止释放
func lightweightOnShow(ctx context.Context) {
	lightweight.Lock()
	defer lightweight.Unlock()
	lightweight.hidden = false
	if lightweight.timer != nil {
		lightweight.timer.Stop()
		lightweight.timer = nil
	}
}
