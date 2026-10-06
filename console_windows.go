//go:build windows

package main

import (
	"os"
	"syscall"
)

// 桌面版按 GUI 程序链接，双击运行不会弹出控制台，因此也没有标准输出。
// 从终端带子命令运行时借用父进程的控制台，让命令行模式的输出能显示出来；
// 已重定向的输出（管道、文件）保持不变。
func attachParentConsole() {
	if t, err := syscall.GetFileType(syscall.Stdout); err == nil && t != 0 {
		return // FILE_TYPE_UNKNOWN 为 0：没有可用的输出
	}
	const attachParentProcess = ^uintptr(0) // (DWORD)-1
	r, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole").Call(attachParentProcess)
	if r == 0 {
		return
	}
	if out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0); err == nil {
		os.Stdout, os.Stderr = out, out
	}
	if in, err := os.OpenFile("CONIN$", os.O_RDWR, 0); err == nil {
		os.Stdin = in
	}
}
