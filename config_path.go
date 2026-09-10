package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
		if fileExistsFile(legacy) {
			if canWriteExistingFile(legacy) {
				return legacy
			}
			if userPath, err := ensureUserMainConfigPath(); err == nil {
				if !fileExistsFile(userPath) {
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

func fileExistsFile(path string) bool {
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

// claudeDesktopConfigPath 返回 Claude Desktop 官方配置文件路径。
// Desktop 与 Claude Code 使用完全不同的配置文件，不能复用 ~/.claude/settings.json。
func claudeDesktopConfigPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv("CLAUDE_DESKTOP_CONFIG")); override != "" {
		return override, nil
	}

	var legacyDir string
	var localDirs []string
	switch runtime.GOOS {
	case "windows":
		config, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		local, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		legacyDir = filepath.Join(config, "Claude")
		localDirs = []string{
			filepath.Join(local, "Claude-3p", "configLibrary"),
			filepath.Join(config, "Claude-3p", "configLibrary"),
			filepath.Join(local, "Claude", "configLibrary"),
			filepath.Join(config, "Claude", "configLibrary"),
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		legacyDir = filepath.Join(home, "Library", "Application Support", "Claude")
		localDirs = []string{filepath.Join(home, "Library", "Application Support", "Claude-3p", "configLibrary")}
	default:
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		legacyDir = filepath.Join(base, "Claude")
		localDirs = []string{filepath.Join(base, "Claude-3p", "configLibrary")}
	}

	// Claude Desktop 3P 使用 configLibrary/_meta.json + <id>.json。
	// 优先读取当前应用配置，兼容旧版 %APPDATA%/Claude/claude_desktop_config.json。
	for _, dir := range localDirs {
		if active := claudeDesktopActiveConfig(dir); active != "" {
			return active, nil
		}
	}
	legacy := filepath.Join(legacyDir, "claude_desktop_config.json")
	if fileExistsFile(legacy) {
		return legacy, nil
	}
	// 目录已经存在但还没有配置时，返回稳定的首个配置文件路径，供首次保存使用。
	for _, dir := range localDirs {
		if claudeDesktopDirExists(dir) {
			return filepath.Join(dir, claudeDesktopFallbackID+".json"), nil
		}
	}
	if len(localDirs) > 0 {
		return filepath.Join(localDirs[0], claudeDesktopFallbackID+".json"), nil
	}
	return legacy, nil
}

func mustClaudeDesktopConfigPath() string {
	path, err := claudeDesktopConfigPath()
	if err != nil {
		return ""
	}
	return path
}

const claudeDesktopFallbackID = "00000000-0000-4000-8000-000000000001"

func claudeDesktopDirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func claudeDesktopActiveConfig(dir string) string {
	if !claudeDesktopDirExists(dir) {
		return ""
	}
	metaPath := filepath.Join(dir, "_meta.json")
	if data, err := os.ReadFile(metaPath); err == nil {
		var meta struct {
			AppliedID string `json:"appliedId"`
		}
		if json.Unmarshal(data, &meta) == nil && isClaudeDesktopConfigID(meta.AppliedID) {
			candidate := filepath.Join(dir, strings.TrimSpace(meta.AppliedID)+".json")
			if fileExistsFile(candidate) {
				return candidate
			}
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var newest string
	var newestTime int64
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "_meta.json" || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if newest == "" || info.ModTime().UnixNano() > newestTime {
			newest = filepath.Join(dir, entry.Name())
			newestTime = info.ModTime().UnixNano()
		}
	}
	return newest
}

func isClaudeDesktopConfigID(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && filepath.Base(value) == value &&
		!strings.ContainsAny(value, `/\\`) && value != "." && value != ".."
}
