//go:build !windows

package main

import "context"

// StartTray 非 Windows 平台的托盘占位。
// 托盘面板依赖 Win32（Shell_NotifyIcon + WebView2 弹出窗口），且 macOS 上
// 自建 NSApplication 循环会与 Wails 冲突；Windows 是本项目主要平台，
// 托盘只在那里启用。StopTray / trayShouldHideOnClose 同样为空实现。
func StartTray(a *App, ctx context.Context, router *RouterService) {}

// StopTray 非 Windows 平台无托盘，无需清理。
func StopTray() {}

// trayShouldHideOnClose 非 Windows 平台保持"关闭即退出"。
func trayShouldHideOnClose() bool { return false }
