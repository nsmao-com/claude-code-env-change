package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// EnvConfig 环境配置
type EnvConfig struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Variables   map[string]string `json:"variables"`
	Provider    string            `json:"provider"`            // "claude", "codex", "gemini"
	Templates   map[string]string `json:"templates,omitempty"` // 自定义模板内容，key为文件名
	Icon        string            `json:"icon,omitempty"`      // emoji 图标
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
		configPath: resolveMainConfigPath(),
	}
}

// OnStartup is called when the app starts up
func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
	a.loadConfig()
	_ = RecordEnvActivation("claude", a.config.CurrentEnvClaude, time.Now())
	_ = RecordEnvActivation("codex", a.config.CurrentEnvCodex, time.Now())
	_ = RecordEnvActivation("gemini", a.config.CurrentEnvGemini, time.Now())
}

// GetConfig 获取配置
func (a *App) GetConfig() Config {
	return a.config
}

// GetEnvVar 获取环境变量
func (a *App) GetEnvVar(key string) string {
	return a.getPlatformEnvVar(key)
}

// SetEnvVar 设置环境变量
func (a *App) SetEnvVar(key, value string) error {
	// 设置当前进程的环境变量
	err := os.Setenv(key, value)
	if err != nil {
		return fmt.Errorf("设置环境变量失败: %v", err)
	}

	// 调用平台特定的持久化方法
	return a.setPlatformEnvVar(key, value)
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

	now := time.Now()
	_ = RecordEnvActivation("claude", a.config.CurrentEnvClaude, now)
	_ = RecordEnvActivation("codex", a.config.CurrentEnvCodex, now)
	_ = RecordEnvActivation("gemini", a.config.CurrentEnvGemini, now)

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

// ClaudeSettings Claude settings.json 结构
type ClaudeSettings struct {
	Env map[string]string `json:"env"`
}

// GetClaudeSettings 读取 Claude settings.json 配置
func (a *App) GetClaudeSettings() map[string]string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	settingsFile := filepath.Join(homeDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		return nil
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil
	}

	// 提取 env 字段
	if envData, ok := settings["env"]; ok {
		if envMap, ok := envData.(map[string]interface{}); ok {
			result := make(map[string]string)
			for k, v := range envMap {
				if str, ok := v.(string); ok {
					result[k] = str
				}
			}
			return result
		}
	}

	return nil
}

// GetCodexSettings 读取 Codex 配置
func (a *App) GetCodexSettings() map[string]string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	result := make(map[string]string)

	// 读取 auth.json
	authFile := filepath.Join(homeDir, ".codex", "auth.json")
	if data, err := os.ReadFile(authFile); err == nil {
		var authData map[string]string
		if json.Unmarshal(data, &authData) == nil {
			for k, v := range authData {
				result[k] = v
			}
		}
	}

	// 读取 config.toml 的关键字段
	configFile := filepath.Join(homeDir, ".codex", "config.toml")
	if data, err := os.ReadFile(configFile); err == nil {
		// 优先用 TOML 解析，避免出现单引号/双引号包裹导致前端显示 "'xxx'"
		var payload map[string]any
		if err := toml.Unmarshal(data, &payload); err == nil && payload != nil {
			if v, ok := payload["model"].(string); ok {
				result["model"] = strings.TrimSpace(v)
			}

			// base_url 可能位于:
			// 1) 顶层 base_url
			// 2) [model_providers.<model_provider>].base_url
			// 3) 其他 provider 表（兜底取第一个找到的 base_url）
			if v, ok := payload["base_url"].(string); ok && strings.TrimSpace(v) != "" {
				result["base_url"] = strings.TrimSpace(v)
			}

			modelProvider := ""
			if v, ok := payload["model_provider"].(string); ok {
				modelProvider = strings.TrimSpace(v)
			}
			if strings.TrimSpace(result["base_url"]) == "" {
				if mp, ok := payload["model_providers"].(map[string]any); ok && len(mp) > 0 {
					if modelProvider != "" {
						if pv, ok := mp[modelProvider].(map[string]any); ok {
							if v, ok := pv["base_url"].(string); ok && strings.TrimSpace(v) != "" {
								result["base_url"] = strings.TrimSpace(v)
							}
						}
					}
					if strings.TrimSpace(result["base_url"]) == "" {
						for _, pv := range mp {
							if t, ok := pv.(map[string]any); ok {
								if v, ok := t["base_url"].(string); ok && strings.TrimSpace(v) != "" {
									result["base_url"] = strings.TrimSpace(v)
									break
								}
							}
						}
					}
				}
			}
		} else {
			// 兜底：旧逻辑按行提取，同时去掉单双引号
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "model =") {
					result["model"] = strings.Trim(strings.TrimPrefix(line, "model ="), " \"'")
				}
				if strings.HasPrefix(line, "base_url =") {
					result["base_url"] = strings.Trim(strings.TrimPrefix(line, "base_url ="), " \"'")
				}
			}
		}
	}

	return result
}

// GetGeminiSettings 读取 Gemini 配置
func (a *App) GetGeminiSettings() map[string]string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	result := make(map[string]string)

	// 读取 .env 文件
	envFile := filepath.Join(homeDir, ".gemini", ".env")
	if data, err := os.ReadFile(envFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				result[parts[0]] = parts[1]
			}
		}
	}

	return result
}

// applyClaudeEnv 应用 Claude 配置到 ~/.claude/settings.json
func (a *App) applyClaudeEnv(env *EnvConfig) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户目录失败: %v", err)
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return "", fmt.Errorf("创建 .claude 目录失败: %v", err)
	}

	settingsFile := filepath.Join(claudeDir, "settings.json")

	// 读取现有的 settings.json (如果存在)
	var settings map[string]interface{}
	if data, err := os.ReadFile(settingsFile); err == nil {
		json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = make(map[string]interface{})
	}

	// 更新 env 字段
	envMap := make(map[string]string)
	for key, value := range env.Variables {
		if value != "" {
			envMap[key] = value
		}
	}
	settings["env"] = envMap

	// 写入 settings.json
	settingsContent, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.WriteFile(settingsFile, settingsContent, 0644); err != nil {
		return "", fmt.Errorf("写入 settings.json 失败: %v", err)
	}

	return "Claude 配置已应用到 ~/.claude/settings.json", nil
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
	configData, err := buildCodexConfigData(configContent, configFile)
	if err != nil {
		return "", fmt.Errorf("序列化 config.toml 失败: %v", err)
	}
	if err := os.WriteFile(configFile, configData, 0644); err != nil {
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

func buildCodexConfigData(configContent, configFile string) ([]byte, error) {
	existingMcpServers := readCodexMcpServers(configFile)
	var payload map[string]any
	if err := toml.Unmarshal([]byte(configContent), &payload); err == nil && payload != nil {
		if len(existingMcpServers) > 0 {
			if _, ok := payload["mcp_servers"]; !ok {
				payload["mcp_servers"] = existingMcpServers
			}
		}
		return toml.Marshal(payload)
	}

	data := []byte(configContent)
	if len(existingMcpServers) > 0 && !strings.Contains(configContent, "mcp_servers") {
		if mcpData, err := toml.Marshal(map[string]any{"mcp_servers": existingMcpServers}); err == nil {
			data = []byte(strings.TrimRight(configContent, "\r\n\t ") + "\n\n" + string(mcpData))
		}
	}
	return data, nil
}

func readCodexMcpServers(configFile string) map[string]map[string]any {
	data, err := os.ReadFile(configFile)
	if err != nil || len(data) == 0 {
		return nil
	}
	var payload codexMcpFilePayload
	if err := toml.Unmarshal(data, &payload); err != nil {
		return nil
	}
	if len(payload.Servers) == 0 {
		return nil
	}
	return payload.Servers
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

	settingsFile := filepath.Join(geminiDir, "settings.json")
	desiredSettings := map[string]any{}
	if tmpl, ok := env.Templates["settings.json"]; ok && strings.TrimSpace(tmpl) != "" {
		if err := json.Unmarshal([]byte(tmpl), &desiredSettings); err != nil {
			return "", fmt.Errorf("解析 settings.json 模板失败: %v", err)
		}
	} else {
		desiredSettings = map[string]any{
			"ide": map[string]any{
				"enabled": true,
			},
			"security": map[string]any{
				"auth": map[string]any{
					"selectedType": "gemini-api-key",
				},
			},
		}
	}

	// 保留现有 settings.json 中的其他设置（如 mcpServers / experimental.skills 等）
	existingSettings := map[string]any{}
	if data, err := os.ReadFile(settingsFile); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &existingSettings); err != nil {
			existingSettings = map[string]any{}
		}
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("读取 settings.json 失败: %v", err)
	}

	deepMergeMap(existingSettings, desiredSettings)
	settingsContent, err := json.MarshalIndent(existingSettings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化 settings.json 失败: %v", err)
	}

	if err := os.WriteFile(settingsFile, settingsContent, 0644); err != nil {
		return "", fmt.Errorf("写入 settings.json 失败: %v", err)
	}

	return "Gemini CLI 配置已应用", nil
}

func deepMergeMap(dst, src map[string]any) {
	for key, srcVal := range src {
		if srcMap, ok := srcVal.(map[string]any); ok && srcMap != nil {
			if dstMap, ok := dst[key].(map[string]any); ok && dstMap != nil {
				deepMergeMap(dstMap, srcMap)
				continue
			}
		}
		dst[key] = srcVal
	}
}

// ClearClaudeSettings 清除 Claude settings.json 中的 env 配置
func (a *App) ClearClaudeSettings() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %v", err)
	}

	settingsFile := filepath.Join(homeDir, ".claude", "settings.json")

	// 读取现有的 settings.json
	var settings map[string]interface{}
	if data, err := os.ReadFile(settingsFile); err == nil {
		json.Unmarshal(data, &settings)
	}
	if settings == nil {
		return nil // 文件不存在，无需清除
	}

	// 清除 env 字段
	delete(settings, "env")

	// 写回文件
	settingsContent, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.WriteFile(settingsFile, settingsContent, 0644); err != nil {
		return fmt.Errorf("写入 settings.json 失败: %v", err)
	}

	return nil
}

// ClearCodexSettings 清除 Codex 配置文件
func (a *App) ClearCodexSettings() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %v", err)
	}

	codexDir := filepath.Join(homeDir, ".codex")

	// 删除配置文件
	os.Remove(filepath.Join(codexDir, "config.toml"))
	os.Remove(filepath.Join(codexDir, "auth.json"))

	return nil
}

// ClearGeminiSettings 清除 Gemini 配置文件
func (a *App) ClearGeminiSettings() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %v", err)
	}

	geminiDir := filepath.Join(homeDir, ".gemini")

	// 删除配置文件
	os.Remove(filepath.Join(geminiDir, ".env"))

	return nil
}

// ClearAllEnv 清除所有配置 (Claude/Codex/Gemini)
func (a *App) ClearAllEnv() error {
	var errors []string

	if err := a.ClearClaudeSettings(); err != nil {
		errors = append(errors, fmt.Sprintf("Claude: %v", err))
	}

	if err := a.ClearCodexSettings(); err != nil {
		errors = append(errors, fmt.Sprintf("Codex: %v", err))
	}

	if err := a.ClearGeminiSettings(); err != nil {
		errors = append(errors, fmt.Sprintf("Gemini: %v", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("部分清除失败: %s", strings.Join(errors, "; "))
	}

	return nil
}

// RefreshConfig 刷新配置
func (a *App) RefreshConfig() error {
	return a.loadConfig()
}

// ExportConfig 导出配置到指定路径（带文件选择对话框）
func (a *App) ExportConfig(defaultName string) (string, error) {
	// 打开保存文件对话框
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出配置",
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON 文件", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("打开对话框失败: %v", err)
	}
	if filePath == "" {
		return "", nil // 用户取消
	}

	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化配置失败: %v", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return "", fmt.Errorf("导出配置文件失败: %v", err)
	}

	return filePath, nil
}

// ImportConfig 从指定路径导入配置（带文件选择对话框）
func (a *App) ImportConfig() (int, error) {
	// 打开文件选择对话框
	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "导入配置",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON 文件", Pattern: "*.json"},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("打开对话框失败: %v", err)
	}
	if filePath == "" {
		return 0, nil // 用户取消
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var importedConfig Config
	err = json.Unmarshal(data, &importedConfig)
	if err != nil {
		return 0, fmt.Errorf("解析配置文件失败: %v", err)
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
		return 0, fmt.Errorf("保存配置失败: %v", err)
	}

	return importCount, nil
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
		return fmt.Errorf("读取配置文件失败 (%s): %v", a.configPath, err)
	}

	err = json.Unmarshal(data, &a.config)
	if err != nil {
		return fmt.Errorf("解析配置文件失败 (%s): %v", a.configPath, err)
	}

	return nil
}

func (a *App) saveConfig() error {
	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	if dir := filepath.Dir(a.configPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建配置目录失败 (%s): %v", dir, err)
		}
	}

	err = os.WriteFile(a.configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("保存配置文件失败 (%s): %v", a.configPath, err)
	}

	return nil
}

// PromptFile 提示词文件信息
type PromptFile struct {
	Provider string `json:"provider"` // claude, codex, gemini
	Path     string `json:"path"`     // 文件路径
	Content  string `json:"content"`  // 文件内容
	Exists   bool   `json:"exists"`   // 文件是否存在
}

// GetPromptFiles 获取所有提示词文件
func (a *App) GetPromptFiles() ([]PromptFile, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %v", err)
	}

	files := []PromptFile{
		{Provider: "claude", Path: filepath.Join(homeDir, ".claude", "CLAUDE.md")},
		{Provider: "codex", Path: filepath.Join(homeDir, ".codex", "AGENTS.md")},
		{Provider: "gemini", Path: filepath.Join(homeDir, ".gemini", "GEMINI.md")},
	}

	for i := range files {
		if data, err := os.ReadFile(files[i].Path); err == nil {
			files[i].Content = string(data)
			files[i].Exists = true
		} else {
			files[i].Content = ""
			files[i].Exists = false
		}
	}

	return files, nil
}

// GetPromptFile 获取单个提示词文件
func (a *App) GetPromptFile(provider string) (PromptFile, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return PromptFile{}, fmt.Errorf("获取用户目录失败: %v", err)
	}

	var filePath string
	switch provider {
	case "claude":
		filePath = filepath.Join(homeDir, ".claude", "CLAUDE.md")
	case "codex":
		filePath = filepath.Join(homeDir, ".codex", "AGENTS.md")
	case "gemini":
		filePath = filepath.Join(homeDir, ".gemini", "GEMINI.md")
	default:
		return PromptFile{}, fmt.Errorf("未知的 Provider: %s", provider)
	}

	file := PromptFile{Provider: provider, Path: filePath}
	if data, err := os.ReadFile(filePath); err == nil {
		file.Content = string(data)
		file.Exists = true
	}

	return file, nil
}

// SavePromptFile 保存提示词文件
func (a *App) SavePromptFile(provider, content string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %v", err)
	}

	var filePath string
	var dirPath string
	switch provider {
	case "claude":
		dirPath = filepath.Join(homeDir, ".claude")
		filePath = filepath.Join(dirPath, "CLAUDE.md")
	case "codex":
		dirPath = filepath.Join(homeDir, ".codex")
		filePath = filepath.Join(dirPath, "AGENTS.md")
	case "gemini":
		dirPath = filepath.Join(homeDir, ".gemini")
		filePath = filepath.Join(dirPath, "GEMINI.md")
	default:
		return fmt.Errorf("未知的 Provider: %s", provider)
	}

	// 确保目录存在
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	return nil
}

// DeletePromptFile 删除提示词文件
func (a *App) DeletePromptFile(provider string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %v", err)
	}

	var filePath string
	switch provider {
	case "claude":
		filePath = filepath.Join(homeDir, ".claude", "CLAUDE.md")
	case "codex":
		filePath = filepath.Join(homeDir, ".codex", "AGENTS.md")
	case "gemini":
		filePath = filepath.Join(homeDir, ".gemini", "GEMINI.md")
	default:
		return fmt.Errorf("未知的 Provider: %s", provider)
	}

	// 删除文件（如果不存在则忽略）
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除文件失败: %v", err)
	}

	return nil
}
