//go:build darwin
// +build darwin

package main

import (
	"os"
)

// getPlatformEnvVar 获取环境变量 (macOS实现)
func (a *App) getPlatformEnvVar(key string) string {
	return os.Getenv(key)
}

// setPlatformEnvVar 设置环境变量 (macOS实现)
// macOS 不需要设置系统环境变量，配置通过文件管理
func (a *App) setPlatformEnvVar(key, value string) error {
	return os.Setenv(key, value)
}

// deletePlatformEnvVar 删除环境变量 (macOS实现)
func (a *App) deletePlatformEnvVar(key string) error {
	return os.Unsetenv(key)
}
