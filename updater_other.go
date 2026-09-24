//go:build !windows
// +build !windows

package main

func applyUpdateAndRestart(_ *App, _, _ string) error {
	return errorf("当前系统请从 GitHub 下载安装包后手动更新")
}

func startInstallerAfterExit(_ *App, _, _ string) error {
	return errorf("当前系统请从 GitHub 下载安装包后手动更新")
}
