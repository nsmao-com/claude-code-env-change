//go:build !windows

package main

import "context"

// StartTray 非 Windows 平台的托盘占位。
// getlantern/systray 在 macOS 上需要独占 NSApplication 主循环，与 Wails
// 同进程运行有冲突风险；Windows 是本项目主要平台，先只在那里启用托盘。
func StartTray(a *App, ctx context.Context) {}
