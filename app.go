package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// EnvConfig 环境配置
type EnvConfig struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Variables   map[string]string `json:"variables"`
	Provider    string            `json:"provider"`            // "claude", "codex", "gemini"
	Templates   map[string]string `json:"templates,omitempty"` // 自定义模板内容，key为文件名
}

// Config 主配置
type Config struct {
	CurrentEnv       string      `json:"current_env"` // Deprecated: 兼容旧版本
	CurrentEnvClaude string      `json:"current_env_claude"`
	CurrentEnvCodex  string      `json:"current_env_codex"`
	CurrentEnvGemini string      `json:"current_env_gemini"`
	Environments     []EnvConfig `json:"environments"`
}

// App struct
type App struct {
	ctx        context.Context
	configPath string
	config     Config
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		configPath: "config.json",
	}
}

// OnStartup is called when the app starts up
func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
	a.loadConfig()
}

// GetConfig 获取配置
func (a *App) GetConfig() Config {
	return a.config
}

// GetEnvVar 获取环境变量
func (a *App) GetEnvVar(key string) string {
	// 在Windows系统上，直接从注册表读取最新值
	if runtime.GOOS == "windows" {
		return a.getWindowsEnvVar(key)
	}
	return os.Getenv(key)
}

// getWindowsEnvVar 从Windows注册表读取环境变量
func (a *App) getWindowsEnvVar(key string) string {
	// 使用reg query命令查询用户环境变量
	cmd := exec.Command("reg", "query", "HKCU\\Environment", "/v", key)
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

// SetEnvVar 设置环境变量
func (a *App) SetEnvVar(key, value string) error {
	// 设置当前进程的环境变量
	err := os.Setenv(key, value)
	if err != nil {
		return fmt.Errorf("设置环境变量失败: %v", err)
	}

	// 在Windows系统上，同时设置系统环境变量
	if runtime.GOOS == "windows" {
		return a.setWindowsEnvVar(key, value)
	}

	return nil
}

// setWindowsEnvVar 在Windows系统上设置环境变量
func (a *App) setWindowsEnvVar(key, value string) error {
	// 使用setx命令设置用户环境变量，不添加双引号
	cmd_exec := exec.Command("setx", key, value)

	output, err := cmd_exec.CombinedOutput()
	if err != nil {
		return fmt.Errorf("执行setx命令失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

// deleteWindowsEnvVar 在Windows系统上删除环境变量
func (a *App) deleteWindowsEnvVar(key string) error {
	// 使用reg命令删除用户环境变量
	cmd_exec := exec.Command("reg", "delete", "HKCU\\Environment", "/v", key, "/f")

	output, err := cmd_exec.CombinedOutput()
	if err != nil {
		// 如果环境变量不存在，reg命令会返回错误，但这不是真正的错误
		if !strings.Contains(string(output), "无法找到指定的注册表项或值") && !strings.Contains(string(output), "The system was unable to find the specified registry key or value") {
			return fmt.Errorf("执行reg delete命令失败: %v, 输出: %s", err, string(output))
		}
	}
	return nil
}

// SwitchToEnv 切换环境
func (a *App) SwitchToEnv(name string) error {
	// 查找环境配置以确定 Provider
	var provider string
	for _, env := range a.config.Environments {
		if env.Name == name {
			provider = env.Provider
			break
		}
	}

	// 默认为 claude
	if provider == "" {
		provider = "claude"
	}

	// 根据 Provider 更新对应的 CurrentEnv
	switch provider {
	case "codex":
		a.config.CurrentEnvCodex = name
	case "gemini":
		a.config.CurrentEnvGemini = name
	default:
		a.config.CurrentEnvClaude = name
	}

	// 兼容旧字段
	a.config.CurrentEnv = name

	return a.saveConfig()
}

// AddEnv adds a new environment configuration
func (a *App) AddEnv(env EnvConfig) error {
	// Check if environment already exists
	for i, existing := range a.config.Environments {
		if existing.Name == env.Name {
			// Update existing environment
			a.config.Environments[i] = env
			return a.saveConfig()
		}
	}

	// Add new environment
	a.config.Environments = append(a.config.Environments, env)
	return a.saveConfig()
}

// UpdateEnv updates an existing environment configuration by old name
func (a *App) UpdateEnv(oldName string, newEnv EnvConfig) error {
	for i, existing := range a.config.Environments {
		if existing.Name == oldName {
			// Update in place to maintain order
			a.config.Environments[i] = newEnv

			// Update current env references if name changed
			if oldName != newEnv.Name {
				if a.config.CurrentEnv == oldName {
					a.config.CurrentEnv = newEnv.Name
				}
				if a.config.CurrentEnvClaude == oldName {
					a.config.CurrentEnvClaude = newEnv.Name
				}
				if a.config.CurrentEnvCodex == oldName {
					a.config.CurrentEnvCodex = newEnv.Name
				}
				if a.config.CurrentEnvGemini == oldName {
					a.config.CurrentEnvGemini = newEnv.Name
				}
			}

			return a.saveConfig()
		}
	}
	return fmt.Errorf("environment '%s' not found", oldName)
}

// DeleteEnv deletes an environment configuration by name
func (a *App) DeleteEnv(name string) error {
	for i, env := range a.config.Environments {
		if env.Name == name {
			// Remove environment from slice
			a.config.Environments = append(a.config.Environments[:i], a.config.Environments[i+1:]...)

			// Clear current env references
			if a.config.CurrentEnv == name {
				a.config.CurrentEnv = ""
			}
			if a.config.CurrentEnvClaude == name {
				a.config.CurrentEnvClaude = ""
			}
			if a.config.CurrentEnvCodex == name {
				a.config.CurrentEnvCodex = ""
			}
			if a.config.CurrentEnvGemini == name {
				a.config.CurrentEnvGemini = ""
			}

			return a.saveConfig()
		}
	}
	return fmt.Errorf("environment '%s' not found", name)
}

// ReorderEnvs reorders the environments based on the provided list of names
func (a *App) ReorderEnvs(names []string) error {
	if len(names) != len(a.config.Environments) {
		return fmt.Errorf("environment count mismatch")
	}

	newEnvs := make([]EnvConfig, 0, len(names))
	envMap := make(map[string]EnvConfig)

	// Create a map for quick lookup
	for _, env := range a.config.Environments {
		envMap[env.Name] = env
	}

	// Reconstruct the slice in the new order
	for _, name := range names {
		if env, ok := envMap[name]; ok {
			newEnvs = append(newEnvs, env)
		} else {
			return fmt.Errorf("environment '%s' not found in current config", name)
		}
	}

	a.config.Environments = newEnvs
	return a.saveConfig()
}

// TestLatency 测试 URL 延迟
func (a *App) TestLatency(urlStr string) (int64, error) {
	if urlStr == "" {
		return 0, fmt.Errorf("URL 为空")
	}

	// 简单的 HTTP GET 请求测速
	start := time.Now()
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(urlStr)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	duration := time.Since(start).Milliseconds()
	return duration, nil
}

// ApplyCurrentEnv 应用当前环境 (根据传入的配置名称，或者默认应用所有激活的配置)
func (a *App) ApplyCurrentEnv() (string, error) {
	// 这里我们修改逻辑：不再只应用单一的 CurrentEnv，而是应用所有 Provider 的当前激活环境
	// 但为了保持 API 简单，我们假设前端调用 SwitchToEnv 后会调用这个方法
	// 实际上，更合理的做法是 SwitchToEnv 内部直接调用 apply 逻辑，或者前端分别调用

	// 为了响应用户的 "应用" 操作，我们这里只应用最近一次切换的环境
	// 但由于 SwitchToEnv 已经更新了状态，我们这里需要知道用户想应用哪个
	// 简化起见，我们遍历所有激活的环境并应用它们

	var msgs []string

	// 1. Apply Claude
	if a.config.CurrentEnvClaude != "" {
		if env := a.findEnv(a.config.CurrentEnvClaude); env != nil {
			if msg, err := a.applyClaudeEnv(env); err == nil {
				msgs = append(msgs, "Claude: "+msg)
			}
		}
	}

	// 2. Apply Codex
	if a.config.CurrentEnvCodex != "" {
		if env := a.findEnv(a.config.CurrentEnvCodex); env != nil {
			if msg, err := a.applyCodexEnv(env); err == nil {
				msgs = append(msgs, "Codex: "+msg)
			}
		}
	}

	// 3. Apply Gemini
	if a.config.CurrentEnvGemini != "" {
		if env := a.findEnv(a.config.CurrentEnvGemini); env != nil {
			if msg, err := a.applyGeminiEnv(env); err == nil {
				msgs = append(msgs, "Gemini: "+msg)
			}
		}
	}

	if len(msgs) == 0 {
		return "没有激活的环境可应用", nil
	}

	return strings.Join(msgs, "\n"), nil
}

func (a *App) findEnv(name string) *EnvConfig {
	for _, env := range a.config.Environments {
		if env.Name == name {
			return &env
		}
	}
	return nil
}

// applyClaudeEnv 应用 Claude 环境变量配置
func (a *App) applyClaudeEnv(env *EnvConfig) (string, error) {
	// 设置当前进程的环境变量
	for key, value := range env.Variables {
		a.SetEnvVar(key, value)
	}

	// 生成环境变量设置脚本 (保留原有逻辑作为备份/导出)
	var script strings.Builder
	var filename string

	if runtime.GOOS == "windows" {
		script.WriteString("@echo off\n")
		script.WriteString("echo 正在设置 Claude Code 环境变量...\n")
		for key, value := range env.Variables {
			script.WriteString(fmt.Sprintf("set %s=%s\n", key, value))
		}
		script.WriteString("echo 环境变量设置完成！\n")
		script.WriteString("pause\n")
		filename = "claude_env_setup.bat"
	} else {
		script.WriteString("#!/bin/bash\n")
		script.WriteString("echo \"正在设置 Claude Code 环境变量...\"\n")
		for key, value := range env.Variables {
			script.WriteString(fmt.Sprintf("export %s=%s\n", key, value))
		}
		script.WriteString("echo \"环境变量设置完成！\"\n")
		filename = "claude_env_setup.sh"
	}

	err := os.WriteFile(filename, []byte(script.String()), 0755)
	if err != nil {
		return "", fmt.Errorf("保存脚本文件失败: %v", err)
	}

	return "环境变量已应用", nil
}

// applyCodexEnv 应用 Codex 配置
func (a *App) applyCodexEnv(env *EnvConfig) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户目录失败: %v", err)
	}

	codexDir := filepath.Join(homeDir, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		return "", fmt.Errorf("创建 .codex 目录失败: %v", err)
	}

	// 1. 处理 config.toml
	var configContent string
	if tmpl, ok := env.Templates["config.toml"]; ok && tmpl != "" {
		// 使用自定义模板，替换变量
		configContent = tmpl
		configContent = strings.ReplaceAll(configContent, "{{model}}", env.Variables["model"])
		configContent = strings.ReplaceAll(configContent, "{{base_url}}", env.Variables["base_url"])
	} else {
		// 使用默认模板
		configContent = fmt.Sprintf(`model_provider = "duckcoding"
model = "%s"
model_reasoning_effort = "high"
network_access = "enabled"
disable_response_storage = true

[model_providers.duckcoding]
name = "duckcoding"
base_url = "%s"
wire_api = "responses"
requires_openai_auth = true
`, env.Variables["model"], env.Variables["base_url"])
	}

	configFile := filepath.Join(codexDir, "config.toml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		return "", fmt.Errorf("写入 config.toml 失败: %v", err)
	}

	// 2. 处理 auth.json
	var authContent string
	if tmpl, ok := env.Templates["auth.json"]; ok && tmpl != "" {
		authContent = tmpl
		authContent = strings.ReplaceAll(authContent, "{{OPENAI_API_KEY}}", env.Variables["OPENAI_API_KEY"])
	} else {
		authContent = fmt.Sprintf(`{
  "OPENAI_API_KEY": "%s"
}`, env.Variables["OPENAI_API_KEY"])
	}

	authFile := filepath.Join(codexDir, "auth.json")
	if err := os.WriteFile(authFile, []byte(authContent), 0644); err != nil {
		return "", fmt.Errorf("写入 auth.json 失败: %v", err)
	}

	return "Codex 配置已应用", nil
}

// applyGeminiEnv 应用 Gemini CLI 配置
func (a *App) applyGeminiEnv(env *EnvConfig) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户目录失败: %v", err)
	}

	geminiDir := filepath.Join(homeDir, ".gemini")
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
		return "", fmt.Errorf("创建 .gemini 目录失败: %v", err)
	}

	// 1. 处理 .env 文件
	var envContent string
	if tmpl, ok := env.Templates[".env"]; ok && tmpl != "" {
		envContent = tmpl
		envContent = strings.ReplaceAll(envContent, "{{GOOGLE_GEMINI_BASE_URL}}", env.Variables["GOOGLE_GEMINI_BASE_URL"])
		envContent = strings.ReplaceAll(envContent, "{{GEMINI_API_KEY}}", env.Variables["GEMINI_API_KEY"])
		envContent = strings.ReplaceAll(envContent, "{{GEMINI_MODEL}}", env.Variables["GEMINI_MODEL"])
	} else {
		envContent = fmt.Sprintf(`GOOGLE_GEMINI_BASE_URL=%s
GEMINI_API_KEY=%s
GEMINI_MODEL=%s
`, env.Variables["GOOGLE_GEMINI_BASE_URL"], env.Variables["GEMINI_API_KEY"], env.Variables["GEMINI_MODEL"])
	}

	envFile := filepath.Join(geminiDir, ".env")
	if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
		return "", fmt.Errorf("写入 .env 失败: %v", err)
	}

	// 2. 处理 settings.json 文件
	var settingsContent string
	if tmpl, ok := env.Templates["settings.json"]; ok && tmpl != "" {
		settingsContent = tmpl
		// settings.json 目前没有变量需要替换，但保留扩展性
	} else {
		settingsContent = `{
  "ide": {
    "enabled": true
  },
  "security": {
    "auth": {
      "selectedType": "gemini-api-key"
    }
  }
}`
	}

	settingsFile := filepath.Join(geminiDir, "settings.json")
	if err := os.WriteFile(settingsFile, []byte(settingsContent), 0644); err != nil {
		return "", fmt.Errorf("写入 settings.json 失败: %v", err)
	}

	return "Gemini CLI 配置已应用", nil
}

// ClearEnvVar 清除特定环境变量
func (a *App) ClearEnvVar(key string) error {
	// 清除当前进程的环境变量
	err := os.Unsetenv(key)
	if err != nil {
		return fmt.Errorf("清除进程环境变量失败: %v", err)
	}

	// 在Windows系统上，同时删除系统环境变量
	if runtime.GOOS == "windows" {
		return a.deleteWindowsEnvVar(key)
	}

	return nil
}

// ClearAllEnv 清除所有相关环境变量
func (a *App) ClearAllEnv() error {
	// 定义需要清除的环境变量
	envVars := []string{
		"ANTHROPIC_BASE_URL",
		"ANTHROPIC_AUTH_TOKEN",
		"ANTHROPIC_MODEL",
		"ANTHROPIC_API_KEY",
		"CLAUDE_MODEL",
		"API_BASE_URL",
		"CLAUDE_MAX_TOKENS",
		"CLAUDE_TEMPERATURE",
	}

	// 逐个清除环境变量
	for _, key := range envVars {
		err := a.ClearEnvVar(key)
		if err != nil {
			// 记录错误但继续清除其他变量
			fmt.Printf("清除环境变量 %s 时出错: %v\n", key, err)
		}
	}

	return nil
}

// RefreshConfig 刷新配置
func (a *App) RefreshConfig() error {
	return a.loadConfig()
}

// ExportConfig 导出配置到指定路径
func (a *App) ExportConfig(filePath string) error {
	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("导出配置文件失败: %v", err)
	}

	return nil
}

// ImportConfig 从指定路径导入配置
func (a *App) ImportConfig(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	var importedConfig Config
	err = json.Unmarshal(data, &importedConfig)
	if err != nil {
		return fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 合并配置：检查是否有重名的环境配置
	existingNames := make(map[string]bool)
	for _, env := range a.config.Environments {
		existingNames[env.Name] = true
	}

	// 导入新配置，重名的配置添加后缀
	importCount := 0
	for _, importedEnv := range importedConfig.Environments {
		name := importedEnv.Name
		if existingNames[name] {
			// 如果重名，添加后缀
			suffix := 1
			for {
				newName := fmt.Sprintf("%s_imported_%d", name, suffix)
				if !existingNames[newName] {
					importedEnv.Name = newName
					break
				}
				suffix++
			}
		}
		a.config.Environments = append(a.config.Environments, importedEnv)
		existingNames[importedEnv.Name] = true
		importCount++
	}

	// 保存合并后的配置
	err = a.saveConfig()
	if err != nil {
		return fmt.Errorf("保存配置失败: %v", err)
	}

	return nil
}

func (a *App) loadConfig() error {
	// 如果配置文件不存在，创建默认配置
	if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
		a.config = Config{
			Environments: []EnvConfig{
				{
					Name:        "Development",
					Description: "开发环境",
					Variables: map[string]string{
						"ANTHROPIC_API_KEY": "your-dev-api-key",
						"CLAUDE_MODEL":      "claude-3-5-sonnet-20241022",
						"API_BASE_URL":      "https://api.anthropic.com",
					},
				},
				{
					Name:        "Production",
					Description: "生产环境",
					Variables: map[string]string{
						"ANTHROPIC_API_KEY": "your-prod-api-key",
						"CLAUDE_MODEL":      "claude-3-5-sonnet-20241022",
						"API_BASE_URL":      "https://api.anthropic.com",
						"CLAUDE_MAX_TOKENS": "4096",
					},
				},
			},
			CurrentEnv: "Development",
		}
		return a.saveConfig()
	}

	// 读取配置文件
	data, err := os.ReadFile(a.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	err = json.Unmarshal(data, &a.config)
	if err != nil {
		return fmt.Errorf("解析配置文件失败: %v", err)
	}

	return nil
}

func (a *App) saveConfig() error {
	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	err = os.WriteFile(a.configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("保存配置文件失败: %v", err)
	}

	return nil
}
