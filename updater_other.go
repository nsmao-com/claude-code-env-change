//go:build !windows
// +build !windows

package main

import "fmt"

func applyUpdateAndRestart(_ *App, _, _ string) error {
	return fmt.Errorf("当前系统请从 GitHub 下载安装包后手动更新")
}
