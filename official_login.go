package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// 官方登录配置。
//
// 思路与 cc-switch 的官方预设一致：官方 = 空配置。各 CLI 只要读不到第三方
// base_url / api_key，就会回落到自带的官方账号登录（OAuth / 订阅账号），
// 所以“应用官方登录”本质上是把本工具写进去的第三方接入摘干净，
// 同时保留用户自己的其它设置（permissions、hooks、mcp_servers 等）。

type officialLoginMeta struct {
	provider string
	name     string
	desc     string
}

var officialLoginCatalog = []officialLoginMeta{
	{"claude", "Claude 官方登录", "清空第三方接入，使用 Claude Code 自带的账号登录"},
	{"claude_desktop", "Claude Desktop 官方登录", "清空第三方网关，使用 Claude Desktop 自带的账号登录"},
	{"codex", "Codex 官方登录", "清空第三方接入，使用 ChatGPT 账号登录（codex login）"},
	{"antigravity", "Antigravity 官方登录", "清空 API Key，使用 Google 账号登录（agy）"},
	{"opencode", "OpenCode 官方登录", "移除自定义 provider，使用 opencode auth login 的账号"},
	{"grok", "Grok 官方登录", "清空自定义模型表，使用 xAI 账号登录（grok）"},
}

const officialLoginIcon = "🔐"

func officialLoginMetaFor(provider string) *officialLoginMeta {
	p := normalizeProviderID(provider)
	for i := range officialLoginCatalog {
		if officialLoginCatalog[i].provider == p {
			return &officialLoginCatalog[i]
		}
	}
	return nil
}

// officialLoginEnvFor 生成一条官方登录配置（不落盘）
func officialLoginEnvFor(provider string) *EnvConfig {
	meta := officialLoginMetaFor(provider)
	if meta == nil {
		return nil
	}
	return &EnvConfig{
		Name:          meta.name,
		Description:   meta.desc,
		Provider:      meta.provider,
		Variables:     map[string]string{},
		Icon:          officialLoginIcon,
		OfficialLogin: true,
	}
}

// normalizeProviderID 统一服务商标识，兼容旧版本的 gemini / 空值
func normalizeProviderID(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	switch p {
	case "":
		return "claude"
	case "gemini":
		return "antigravity"
	default:
		return p
	}
}

// AddOfficialLoginEnvs 为指定服务商补上官方登录配置；provider 为空或 all 时补齐全部六个。
// 已存在同名配置时跳过，不覆盖用户改过的内容。
func (a *App) AddOfficialLoginEnvs(provider string) ([]EnvConfig, error) {
	p := normalizeProviderID(provider)
	if strings.TrimSpace(provider) == "" || p == "all" {
		p = "all"
	}

	targets := make([]officialLoginMeta, 0, len(officialLoginCatalog))
	if p == "all" {
		targets = append(targets, officialLoginCatalog...)
	} else {
		meta := officialLoginMetaFor(p)
		if meta == nil {
			return nil, fmt.Errorf("未知平台 %s", provider)
		}
		targets = append(targets, *meta)
	}

	added := make([]EnvConfig, 0, len(targets))
	for _, meta := range targets {
		if a.findEnvIn(meta.provider, meta.name) != nil {
			continue
		}
		env := officialLoginEnvFor(meta.provider)
		if env == nil {
			continue
		}
		if err := a.AddEnv(*env); err != nil {
			return nil, err
		}
		added = append(added, *env)
	}
	if len(added) == 0 {
		return nil, fmt.Errorf("官方登录配置已经存在，无需重复添加")
	}
	return added, nil
}

// localOfficialLoginEnv 本机处于官方登录状态时，生成对应的导入配置。
// 名称固定，重复导入会原地更新而不是堆出一串副本。
func (a *App) localOfficialLoginEnv(provider string) *EnvConfig {
	if !localOfficialLoginDetected(provider) {
		return nil
	}
	env := officialLoginEnvFor(provider)
	if env == nil {
		return nil
	}
	env.Description = "本机当前为官方登录（未检测到第三方接入）"
	return env
}

// localOfficialLoginDetected 判断本机是否装了该工具。
// 调用点已经确认读不到任何第三方接入信息，配置目录还在就说明是官方登录。
func localOfficialLoginDetected(provider string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	switch normalizeProviderID(provider) {
	case "claude":
		return dirExistsPath(filepath.Join(home, ".claude"))
	case "claude_desktop":
		path, err := claudeDesktopConfigPath()
		return err == nil && fileExistsFile(path)
	case "codex":
		return dirExistsPath(filepath.Join(home, ".codex"))
	case "antigravity":
		return dirExistsPath(filepath.Join(home, ".gemini"))
	case "opencode":
		return fileExistsFile(opencodeConfigFile(nil)) || fileExistsFile(opencodeAuthFile())
	case "grok":
		configFile := grokConfigFile(nil)
		return fileExistsFile(configFile) || dirExistsPath(filepath.Dir(configFile))
	}
	return false
}

func dirExistsPath(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// applyOfficialLogin 应用官方登录：按服务商清掉第三方接入，保留用户其它设置
func (a *App) applyOfficialLogin(env *EnvConfig) (string, error) {
	switch normalizeProviderID(env.Provider) {
	case "claude_desktop":
		return a.officialLoginClaudeDesktop()
	case "codex":
		return a.officialLoginCodex()
	case "antigravity":
		return a.officialLoginAntigravity()
	case "opencode":
		return a.officialLoginOpencode(env)
	case "grok":
		return a.officialLoginGrok(env)
	default:
		return a.officialLoginClaude()
	}
}

// claudeThirdPartyEnvKeys 本工具写入 settings.json 的第三方接入变量。
// 只删这些键，用户自己加的其它 env 保持不动。
var claudeThirdPartyEnvKeys = []string{
	"ANTHROPIC_BASE_URL",
	"ANTHROPIC_AUTH_TOKEN",
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_MODEL",
	"ANTHROPIC_SMALL_FAST_MODEL",
	"ANTHROPIC_DEFAULT_HAIKU_MODEL",
	"ANTHROPIC_DEFAULT_SONNET_MODEL",
	"ANTHROPIC_DEFAULT_OPUS_MODEL",
	"ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME",
	"ANTHROPIC_DEFAULT_SONNET_MODEL_NAME",
	"ANTHROPIC_DEFAULT_OPUS_MODEL_NAME",
	"ANTHROPIC_CUSTOM_HEADERS",
	"CLAUDE_CODE_SKIP_BEDROCK_AUTH",
	"CLAUDE_CODE_SKIP_VERTEX_AUTH",
	"CLAUDE_CODE_USE_BEDROCK",
	"CLAUDE_CODE_USE_VERTEX",
}

func (a *App) officialLoginClaude() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户目录失败: %v", err)
	}
	settingsFile := filepath.Join(homeDir, ".claude", "settings.json")

	data, readErr := os.ReadFile(settingsFile)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			// 没有 settings.json 时本来就是官方登录状态
			return "已切换到 Claude 官方登录；运行 claude 按提示登录即可", nil
		}
		return "", fmt.Errorf("读取 %s 失败: %v", settingsFile, readErr)
	}

	settings := map[string]any{}
	if len(strings.TrimSpace(string(data))) > 0 {
		parsed, parseErr := parseJSONLikeObject(data)
		if parseErr != nil {
			return "", fmt.Errorf("解析 %s 失败，为保护原文件已中止写入: %v", settingsFile, parseErr)
		}
		settings = parsed
	}

	if envMap, ok := settings["env"].(map[string]any); ok && envMap != nil {
		for _, key := range claudeThirdPartyEnvKeys {
			delete(envMap, key)
		}
		if len(envMap) == 0 {
			delete(settings, "env")
		} else {
			settings["env"] = envMap
		}
	}

	content, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化配置失败: %v", err)
	}
	if err := os.WriteFile(settingsFile, content, 0644); err != nil {
		return "", fmt.Errorf("写入 settings.json 失败: %v", err)
	}
	return "已切换到 Claude 官方登录；如未登录过请运行 claude 按提示登录", nil
}

func (a *App) officialLoginClaudeDesktop() (string, error) {
	settingsFile, err := claudeDesktopConfigPath()
	if err != nil {
		return "", fmt.Errorf("获取 Claude Desktop 配置路径失败: %v", err)
	}
	// 3P 档案放在 configLibrary，_meta.json 的 appliedId 还指着它就仍然算第三方接入，
	// 只删字段不够，必须把这份档案连同 meta 记录一起摘掉才会回到官方账号。
	if strings.EqualFold(filepath.Base(filepath.Dir(settingsFile)), "configLibrary") {
		if err := os.Remove(settingsFile); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("移除 Claude Desktop 第三方档案失败: %v", err)
		}
		id := strings.TrimSuffix(filepath.Base(settingsFile), filepath.Ext(settingsFile))
		if err := removeClaudeDesktopMetaEntry(filepath.Dir(settingsFile), id); err != nil {
			return "", err
		}
		return "已切换到 Claude Desktop 官方登录；重启 Claude Desktop 生效", nil
	}

	data, readErr := os.ReadFile(settingsFile)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return "已切换到 Claude Desktop 官方登录", nil
		}
		return "", fmt.Errorf("读取 %s 失败: %v", settingsFile, readErr)
	}

	settings := map[string]any{}
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return "", fmt.Errorf("解析 %s 失败，为保护原文件已中止写入: %v", settingsFile, err)
		}
		if settings == nil {
			settings = map[string]any{}
		}
	}

	// 3P 网关字段整组摘掉，Desktop 会回到官方账号
	for _, key := range []string{
		"inferenceProvider",
		"inferenceGatewayBaseUrl",
		"inferenceGatewayApiKey",
		"inferenceGatewayAuthScheme",
		"inferenceModels",
	} {
		delete(settings, key)
	}
	if envMap, ok := settings["env"].(map[string]any); ok && envMap != nil {
		for _, key := range claudeThirdPartyEnvKeys {
			delete(envMap, key)
		}
		if len(envMap) == 0 {
			delete(settings, "env")
		} else {
			settings["env"] = envMap
		}
	}

	content, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化 Claude Desktop 配置失败: %v", err)
	}
	if err := os.WriteFile(settingsFile, content, 0o600); err != nil {
		return "", fmt.Errorf("写入 Claude Desktop 配置失败: %v", err)
	}
	return "已切换到 Claude Desktop 官方登录；重启 Claude Desktop 生效", nil
}

func (a *App) officialLoginCodex() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户目录失败: %v", err)
	}
	codexDir := filepath.Join(homeDir, ".codex")

	// config.toml：摘掉自定义模型供应商，保留 mcp_servers 等用户设置
	configFile := filepath.Join(codexDir, "config.toml")
	if data, readErr := os.ReadFile(configFile); readErr == nil && len(data) > 0 {
		payload := map[string]any{}
		if err := toml.Unmarshal(data, &payload); err != nil {
			return "", fmt.Errorf("解析 %s 失败，为保护原文件已中止写入: %v", configFile, err)
		}
		delete(payload, "model_provider")
		delete(payload, "model_providers")
		delete(payload, "model")
		out, err := toml.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("序列化 config.toml 失败: %v", err)
		}
		if err := os.WriteFile(configFile, out, 0644); err != nil {
			return "", fmt.Errorf("写入 config.toml 失败: %v", err)
		}
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return "", fmt.Errorf("读取 %s 失败: %v", configFile, readErr)
	}

	// auth.json：只摘 API Key，保留 OAuth 的 tokens，避免把已登录的账号踢掉
	authFile := filepath.Join(codexDir, "auth.json")
	if data, readErr := os.ReadFile(authFile); readErr == nil && len(data) > 0 {
		payload, err := parseJSONLikeObject(data)
		if err != nil {
			return "", fmt.Errorf("解析 %s 失败，为保护原文件已中止写入: %v", authFile, err)
		}
		delete(payload, "OPENAI_API_KEY")
		if len(payload) == 0 {
			// 只有 API Key 时直接删文件，codex 会引导重新登录
			if err := os.Remove(authFile); err != nil && !os.IsNotExist(err) {
				return "", fmt.Errorf("清理 auth.json 失败: %v", err)
			}
		} else {
			out, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				return "", fmt.Errorf("序列化 auth.json 失败: %v", err)
			}
			if err := os.WriteFile(authFile, out, 0600); err != nil {
				return "", fmt.Errorf("写入 auth.json 失败: %v", err)
			}
		}
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return "", fmt.Errorf("读取 %s 失败: %v", authFile, readErr)
	}

	return "已切换到 Codex 官方登录；如未登录过请运行 codex login", nil
}

func (a *App) officialLoginAntigravity() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户目录失败: %v", err)
	}
	geminiDir := filepath.Join(homeDir, ".gemini")

	// .env 里的密钥和端点摘掉；文件本身留着，用户可能写了别的变量
	envFile := filepath.Join(geminiDir, ".env")
	if data, readErr := os.ReadFile(envFile); readErr == nil && len(data) > 0 {
		kept := make([]string, 0, 8)
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			name := strings.TrimSpace(strings.SplitN(strings.TrimPrefix(trimmed, "export "), "=", 2)[0])
			if isAntigravityThirdPartyKey(name) {
				continue
			}
			kept = append(kept, line)
		}
		content := strings.Join(kept, "\n")
		if strings.TrimSpace(content) == "" {
			if err := os.Remove(envFile); err != nil && !os.IsNotExist(err) {
				return "", fmt.Errorf("清理 .env 失败: %v", err)
			}
		} else {
			if err := os.WriteFile(envFile, []byte(content+"\n"), 0644); err != nil {
				return "", fmt.Errorf("写入 .env 失败: %v", err)
			}
		}
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return "", fmt.Errorf("读取 %s 失败: %v", envFile, readErr)
	}

	// agy 只认进程环境变量，持久化到用户环境的那两个键必须一并清掉
	if err := syncAntigravityUserEnv(nil); err != nil {
		return "", fmt.Errorf("清理用户环境变量失败: %v", err)
	}

	// modelProvider 留着会让 agy 继续找 GEMINI_API_KEY，必须摘掉才会走 Google 账号登录
	for _, path := range []string{
		filepath.Join(geminiDir, "antigravity-cli", "settings.json"),
		filepath.Join(geminiDir, "settings.json"),
	} {
		removeJSONFileKeys(path, "modelProvider")
		if err := clearGeminiAuthSelection(path); err != nil {
			return "", err
		}
	}

	return "已切换到 Antigravity 官方登录；请新开终端后运行 agy 用 Google 账号登录", nil
}

func isAntigravityThirdPartyKey(name string) bool {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_GEMINI_BASE_URL", "GEMINI_MODEL",
		"GOOGLE_GENAI_USE_VERTEXAI", "GOOGLE_CLOUD_PROJECT", "GOOGLE_CLOUD_LOCATION":
		return true
	}
	return false
}

// clearGeminiAuthSelection 摘掉 security.auth.selectedType（值通常是 gemini-api-key），
// 让 agy 重新走账号登录选择流程。
func clearGeminiAuthSelection(path string) error {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal(data, &payload); err != nil || payload == nil {
		return nil
	}
	security, ok := payload["security"].(map[string]any)
	if !ok || security == nil {
		return nil
	}
	auth, ok := security["auth"].(map[string]any)
	if !ok || auth == nil {
		return nil
	}
	if _, exists := auth["selectedType"]; !exists {
		return nil
	}
	delete(auth, "selectedType")
	if len(auth) == 0 {
		delete(security, "auth")
	} else {
		security["auth"] = auth
	}
	if len(security) == 0 {
		delete(payload, "security")
	} else {
		payload["security"] = security
	}
	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 %s 失败: %v", path, err)
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("写入 %s 失败: %v", path, err)
	}
	return nil
}

func (a *App) officialLoginOpencode(env *EnvConfig) (string, error) {
	configFile := opencodeConfigFile(nil)
	data, readErr := os.ReadFile(configFile)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			a.setOpencodeOfficialCurrent(env)
			return "已切换到 OpenCode 官方登录；运行 opencode auth login 登录账号", nil
		}
		return "", fmt.Errorf("读取 %s 失败: %v", configFile, readErr)
	}
	payload, err := parseJSONLikeObject(data)
	if err != nil {
		return "", fmt.Errorf("解析 OpenCode 配置失败，为保护原文件已中止写入: %v", err)
	}

	// 把本工具接管过的自定义 provider 全部摘掉，官方登录不依赖它们
	managed := map[string]bool{}
	for i := range a.config.Environments {
		item := &a.config.Environments[i]
		if normalizeProviderID(item.Provider) != "opencode" || item.OfficialLogin {
			continue
		}
		if id := strings.TrimSpace(opencodeProviderID(item)); id != "" {
			managed[id] = true
		}
	}
	if providers, ok := payload["provider"].(map[string]any); ok && providers != nil {
		for id := range managed {
			delete(providers, id)
		}
		if len(providers) == 0 {
			delete(payload, "provider")
		} else {
			payload["provider"] = providers
		}
	}
	if model, ok := payload["model"].(string); ok {
		for id := range managed {
			if model == id || strings.HasPrefix(model, id+"/") {
				delete(payload, "model")
				break
			}
		}
	}

	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化 OpenCode 配置失败: %v", err)
	}
	if err := os.WriteFile(configFile, out, 0644); err != nil {
		return "", fmt.Errorf("写入 OpenCode 配置失败: %v", err)
	}
	a.setOpencodeOfficialCurrent(env)
	return "已切换到 OpenCode 官方登录；如未登录过请运行 opencode auth login", nil
}

// setOpencodeOfficialCurrent 官方登录是独占的：其它 OpenCode 配置的激活状态一并撤掉，
// 否则界面还显示着已经被摘掉的自定义 provider。
func (a *App) setOpencodeOfficialCurrent(env *EnvConfig) {
	if env == nil {
		return
	}
	a.config.CurrentEnvsOpencode = []string{env.Name}
	a.config.CurrentEnvOpencode = env.Name
}

func (a *App) officialLoginGrok(env *EnvConfig) (string, error) {
	configFile := grokConfigFile(env.Variables)
	data, readErr := os.ReadFile(configFile)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return "已切换到 Grok 官方登录；运行 grok 按提示用 xAI 账号登录", nil
		}
		return "", fmt.Errorf("读取 %s 失败: %v", configFile, readErr)
	}
	payload := map[string]any{}
	if len(data) > 0 {
		if err := toml.Unmarshal(data, &payload); err != nil {
			return "", fmt.Errorf("解析 %s 失败，为保护原文件已中止写入: %v", configFile, err)
		}
	}

	// 自定义模型表摘掉，Grok CLI 回落到自带的 xAI OAuth 登录
	if model, ok := payload["model"].(map[string]any); ok && model != nil {
		delete(model, "custom")
		if len(model) == 0 {
			delete(payload, "model")
		} else {
			payload["model"] = model
		}
	}
	if models, ok := payload["models"].(map[string]any); ok && models != nil {
		if def, ok := models["default"].(string); ok && strings.EqualFold(strings.TrimSpace(def), "custom") {
			delete(models, "default")
		}
		if len(models) == 0 {
			delete(payload, "models")
		} else {
			payload["models"] = models
		}
	}

	out, err := toml.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("序列化 Grok config.toml 失败: %v", err)
	}
	if err := os.WriteFile(configFile, out, 0644); err != nil {
		return "", fmt.Errorf("写入 Grok config.toml 失败: %v", err)
	}
	return "已切换到 Grok 官方登录；如未登录过请运行 grok 按提示登录", nil
}
