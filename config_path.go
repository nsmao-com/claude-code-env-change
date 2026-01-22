package main

import (
	"os"
	"path/filepath"
	"strings"
)

const mainConfigFile = "config.json"

// resolveMainConfigPath 解析主配置文件路径：
// 1) 若当前工作目录存在可写的 config.json，则继续使用（兼容旧版本/便携用法）
// 2) 若存在但不可写（例如 macOS 安装到 /Applications 后的 .app 目录），则迁移到用户目录并使用之
// 3) 其他情况默认使用用户目录 ~/.claude-env-switcher/config.json
func resolveMainConfigPath() string {
	if override := strings.TrimSpace(os.Getenv("CLAUDIA_CONFIG_PATH")); override != "" {
		return override
	}

	cwd, err := os.Getwd()
	if err == nil && strings.TrimSpace(cwd) != "" {
		legacy := filepath.Join(cwd, mainConfigFile)
		if fileExists(legacy) {
			if canWriteExistingFile(legacy) {
				return legacy
			}
			if userPath, err := ensureUserMainConfigPath(); err == nil {
				if !fileExists(userPath) {
					_ = copyFile(legacy, userPath)
				}
				return userPath
			}
			return legacy
		}
	}

	if userPath, err := ensureUserMainConfigPath(); err == nil {
		return userPath
	}

	return mainConfigFile
}

func ensureUserMainConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, mainConfigFile), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func canWriteExistingFile(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	// 只在 dst 不存在时调用；这里避免覆盖已有用户配置
	return os.WriteFile(dst, data, 0o644)
}
