//go:build windows
// +build windows

package main

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// getWindowsEnvVar 从Windows注册表读取环境变量
func (a *App) getWindowsEnvVar(key string) string {
	// 使用reg query命令查询用户环境变量
	cmd := exec.Command("reg", "query", "HKCU\\Environment", "/v", key)

	// 隐藏CMD窗口
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	output, err := cmd.Output()
	if err != nil {
		// 如果用户环境变量不存在，尝试读取进程环境变量
		return os.Getenv(key)
	}

	// 解析reg命令的输出
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 查找包含变量名和REG_SZ的行
		if strings.Contains(line, key) && strings.Contains(line, "REG_SZ") {
			// 使用正则表达式或字符串分割来解析
			// 格式通常是: "变量名    REG_SZ    值"
			regSzIndex := strings.Index(line, "REG_SZ")
			if regSzIndex != -1 {
				// 获取REG_SZ后面的值
				valueStart := regSzIndex + len("REG_SZ")
				if valueStart < len(line) {
					value := strings.TrimSpace(line[valueStart:])
					return value
				}
			}
		}
	}

	// 如果解析失败，返回进程环境变量
	return os.Getenv(key)
}

// setWindowsEnvVar 在Windows系统上设置环境变量
func (a *App) setWindowsEnvVar(key, value string) error {
	// 使用setx命令设置用户环境变量，不添加双引号
	cmd_exec := exec.Command("setx", key, value)

	// 隐藏CMD窗口
	cmd_exec.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	_, err := cmd_exec.CombinedOutput()
	return err
}

// deleteWindowsEnvVar 在Windows系统上删除环境变量
func (a *App) deleteWindowsEnvVar(key string) error {
	// 使用reg命令删除用户环境变量
	cmd_exec := exec.Command("reg", "delete", "HKCU\\Environment", "/v", key, "/f")

	// 隐藏CMD窗口
	cmd_exec.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	output, err := cmd_exec.CombinedOutput()
	if err != nil {
		// 如果环境变量不存在，reg命令会返回错误，但这不是真正的错误
		if !strings.Contains(string(output), "无法找到指定的注册表项或值") && !strings.Contains(string(output), "The system was unable to find the specified registry key or value") {
			return err
		}
	}
	return nil
}

// getPlatformEnvVar 获取环境变量 (Windows实现)
func (a *App) getPlatformEnvVar(key string) string {
	return a.getWindowsEnvVar(key)
}

// setPlatformEnvVar 设置环境变量 (Windows实现)
func (a *App) setPlatformEnvVar(key, value string) error {
	return a.setWindowsEnvVar(key, value)
}

// deletePlatformEnvVar 删除环境变量 (Windows实现)
func (a *App) deletePlatformEnvVar(key string) error {
	return a.deleteWindowsEnvVar(key)
}
