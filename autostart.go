package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// 开机自启：Windows 写 HKCU 注册表 Run 键；macOS 写 LaunchAgent plist；
// Linux 写 XDG autostart .desktop。三平台共用 Get/SetAutostart 两个绑定。

const autostartName = "AI-ENV"

// currentExePath 返回当前可执行文件的绝对路径（自启必须用绝对路径，
// 注册表/desktop 里写相对路径会因启动目录不同而失效）
func currentExePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exe)
}

// GetAutostartEnabled 当前是否启用了开机自启
func (a *App) GetAutostartEnabled() bool {
	enabled, err := autostartEnabled()
	if err != nil {
		return false
	}
	return enabled
}

// SetAutostart 开启/关闭开机自启
func (a *App) SetAutostart(enabled bool) error {
	if enabled {
		if err := enableAutostart(); err != nil {
			return fmt.Errorf("设置开机自启失败: %v", err)
		}
		return nil
	}
	if err := disableAutostart(); err != nil {
		return fmt.Errorf("关闭开机自启失败: %v", err)
	}
	return nil
}

func autostartLaunchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", autostartName+".plist"), nil
}

func autostartDesktopPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "autostart", autostartName+".desktop"), nil
}
