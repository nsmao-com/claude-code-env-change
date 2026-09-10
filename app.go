package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"strconv"

	"github.com/pelletier/go-toml/v2"
	json5 "github.com/titanous/json5"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// EnvConfig 环境配置
type EnvConfig struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Variables   map[string]string `json:"variables"`
	Provider    string            `json:"provider"`            // "claude", "claude_desktop", "codex", "antigravity", "opencode", "grok"
	Templates   map[string]string `json:"templates,omitempty"` // 自定义模板内容，key为文件名
	Icon        string            `json:"icon,omitempty"`      // emoji 图标
	// 上游 API 格式："" 原生直连；chat_completions / anthropic_messages / responses 需本地路由转换
	UpstreamFormat string `json:"upstream_format,omitempty"`
	// Claude Code 特有配置 (值为 "0" 或 "1"，空字符串表示不设置)
	AttributionHeader          string `json:"attribution_header"`
	DisableNonessentialTraffic string `json:"disable_nonessential_traffic"`
}

// Config 主配置
type Config struct {
	CurrentEnv              string      `json:"current_env"` // Deprecated: 兼容旧版本
	CurrentEnvClaude        string      `json:"current_env_claude"`
	CurrentEnvClaudeDesktop string      `json:"current_env_claude_desktop"`
	CurrentEnvCodex         string      `json:"current_env_codex"`
	CurrentEnvAntigravity   string      `json:"current_env_antigravity"` // 旧版本为 gemini，加载时自动迁移
	CurrentEnvOpencode      string      `json:"current_env_opencode"`
	CurrentEnvsOpencode     []string    `json:"current_envs_opencode"`
	CurrentEnvGrok          string      `json:"current_env_grok"`
	Environments            []EnvConfig `json:"environments"`
}

// App struct
type App struct {
	ctx        context.Context
	configPath string
	config     Config
	// configLoadErr 记录配置文件读取/解析失败的原因。
	// 只要它不为空，saveConfig 就拒绝写盘，避免用一份空配置覆盖用户真实数据。
	configLoadErr error
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
	initOutboundProxy()
	go cleanupStaleUpdateTemp()
	if err := a.loadConfig(); err != nil {
		// 配置读不出来时绝对不能继续往下写盘：早期版本会在这里直接 saveConfig，
		// 把一份空配置盖回用户的 config.json，所有环境配置就此丢失。
		runtime.LogErrorf(ctx, "配置加载失败，已暂停写入: %v", err)
		return
	}
	a.syncOpencodeAppliedFromDisk()
	_ = a.saveConfig()
	_ = RecordEnvActivation("claude", a.config.CurrentEnvClaude, time.Now())
	_ = RecordEnvActivation("claude_desktop", a.config.CurrentEnvClaudeDesktop, time.Now())
	_ = RecordEnvActivation("codex", a.config.CurrentEnvCodex, time.Now())
	_ = RecordEnvActivation("antigravity", a.config.CurrentEnvAntigravity, time.Now())
	_ = RecordEnvActivation("opencode", a.config.CurrentEnvOpencode, time.Now())
	_ = RecordEnvActivation("grok", a.config.CurrentEnvGrok, time.Now())
}

// GetConfig 获取配置
func (a *App) GetConfig() Config {
	a.syncOpencodeAppliedFromDisk()
	cfg := a.config
	cfg.CurrentEnvsOpencode = append([]string(nil), a.config.CurrentEnvsOpencode...)
	if cfg.CurrentEnvsOpencode == nil {
		cfg.CurrentEnvsOpencode = []string{}
	}
	return cfg
}

// GetOpencodeAppliedNames 当前同时挂在 opencode.json 里的 OpenCode 配置名
func (a *App) GetOpencodeAppliedNames() []string {
	a.syncOpencodeAppliedFromDisk()
	names := a.opencodeCurrentNames()
	if names == nil {
		return []string{}
	}
	return names
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
func (a *App) SwitchToEnv(name string, provider string) error {
	// 名称只在同一服务商内唯一，必须由调用方指明服务商
	found := false
	for _, env := range a.config.Environments {
		if env.Name == name && sameProvider(env.Provider, provider) {
			provider = env.Provider
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("environment '%s' (%s) not found", name, provider)
	}

	// 根据 Provider 更新对应的 CurrentEnv
	switch provider {
	case "codex":
		a.config.CurrentEnvCodex = name
	case "claude_desktop":
		a.config.CurrentEnvClaudeDesktop = name
	case "antigravity":
		a.config.CurrentEnvAntigravity = name
	case "opencode":
		a.addOpencodeCurrent(name)
	case "grok":
		a.config.CurrentEnvGrok = name
	default:
		a.config.CurrentEnvClaude = name
	}

	// 兼容旧字段
	a.config.CurrentEnv = name

	return a.saveConfig()
}

// ApplyEnv 仅应用指定服务商的一条配置，避免点击单个平台时重写其它平台的本机配置。
func (a *App) ApplyEnv(name, provider string) (string, error) {
	env := a.findEnvIn(provider, name)
	if env == nil {
		return "", fmt.Errorf("找不到环境配置 %q (%s)", name, provider)
	}
	if err := a.SwitchToEnv(name, provider); err != nil {
		return "", err
	}
	message, err := a.applyEnvByProvider(env)
	if err != nil {
		return "", err
	}
	if err := a.saveConfig(); err != nil {
		return "", err
	}
	_ = RecordEnvActivation(env.Provider, name, time.Now())
	return message, nil
}

// UnapplyEnv 停用一条已应用的配置。OpenCode 可同时挂多套，停用只拿掉这一套，其它继续留在 opencode.json。
func (a *App) UnapplyEnv(name string) error {
	env := a.findEnvIn("opencode", name)
	if env == nil {
		return fmt.Errorf("environment '%s' not found", name)
	}
	if env.Provider != "opencode" {
		return fmt.Errorf("只有 OpenCode 支持停用单套配置")
	}
	if !a.isOpencodeCurrent(name) {
		return nil
	}
	if err := a.stripOpencodeProvider(env); err != nil {
		return err
	}
	a.removeOpencodeCurrent(name)
	if err := a.saveConfig(); err != nil {
		return err
	}
	if last := a.config.CurrentEnvOpencode; last != "" {
		if remaining := a.findEnvIn("opencode", last); remaining != nil {
			if _, err := a.applyOpencodeEnv(remaining); err != nil {
				return fmt.Errorf("已停用 %s，但刷新默认模型失败: %v", name, err)
			}
		}
	}
	return nil
}

// AddEnv adds a new environment configuration
// 名称只需在同一服务商内唯一；同名但不同服务商的配置各自独立存在
func (a *App) AddEnv(env EnvConfig) error {
	// Check if environment already exists (same provider)
	for i, existing := range a.config.Environments {
		if existing.Name == env.Name && sameProvider(existing.Provider, env.Provider) {
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
func (a *App) UpdateEnv(oldName string, oldProvider string, newEnv EnvConfig) error {
	idx := -1
	for i, existing := range a.config.Environments {
		if existing.Name == oldName && sameProvider(existing.Provider, oldProvider) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("environment '%s' (%s) not found", oldName, oldProvider)
	}

	// 目标 (name, provider) 不能与其他配置冲突（同服务商内唯一）
	for i, existing := range a.config.Environments {
		if i != idx && existing.Name == newEnv.Name && sameProvider(existing.Provider, newEnv.Provider) {
			return fmt.Errorf("配置名称 %q 在 %s 下已存在", newEnv.Name, newEnv.Provider)
		}
	}

	// Update in place to maintain order
	a.config.Environments[idx] = newEnv

	providerChanged := !sameProvider(oldProvider, newEnv.Provider)
	renamed := oldName != newEnv.Name
	if providerChanged {
		// 换了服务商：旧服务商的当前环境引用随之清空
		a.clearCurrentEnvRef(oldProvider, oldName)
	} else if renamed {
		// 只迁移该服务商自己的当前环境引用，避免误改其他服务商的同名配置
		if a.config.CurrentEnv == oldName {
			a.config.CurrentEnv = newEnv.Name
		}
		a.renameProviderCurrentRef(newEnv.Provider, oldName, newEnv.Name)
	}

	if err := a.saveConfig(); err != nil {
		return err
	}
	if !providerChanged && a.isCurrentEnvFor(newEnv.Provider, newEnv.Name) {
		if _, err := a.applyEnvByProvider(&newEnv); err != nil {
			return fmt.Errorf("配置已保存，但写回本机失败: %v", err)
		}
	}
	return nil
}

// sameProvider 服务商比较（大小写不敏感，空值归一为 claude）
func sameProvider(a, b string) bool {
	norm := func(p string) string {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			return "claude"
		}
		return p
	}
	return norm(a) == norm(b)
}

// isCurrentEnvFor 某条配置是否是其服务商当前激活的环境
func (a *App) isCurrentEnvFor(provider, name string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "codex":
		return a.config.CurrentEnvCodex == name
	case "claude_desktop":
		return a.config.CurrentEnvClaudeDesktop == name
	case "antigravity":
		return a.config.CurrentEnvAntigravity == name
	case "opencode":
		return a.isOpencodeCurrent(name)
	case "grok":
		return a.config.CurrentEnvGrok == name
	default:
		return a.config.CurrentEnvClaude == name
	}
}

// clearCurrentEnvRef 清掉某服务商下指向该名称的当前环境引用
func (a *App) clearCurrentEnvRef(provider, name string) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "claude_desktop":
		if a.config.CurrentEnvClaudeDesktop == name {
			a.config.CurrentEnvClaudeDesktop = ""
		}
	case "codex":
		if a.config.CurrentEnvCodex == name {
			a.config.CurrentEnvCodex = ""
		}
	case "antigravity":
		if a.config.CurrentEnvAntigravity == name {
			a.config.CurrentEnvAntigravity = ""
		}
	case "opencode":
		a.removeOpencodeCurrent(name)
	case "grok":
		if a.config.CurrentEnvGrok == name {
			a.config.CurrentEnvGrok = ""
		}
	default:
		if a.config.CurrentEnvClaude == name {
			a.config.CurrentEnvClaude = ""
		}
	}
	if a.config.CurrentEnv == name {
		a.config.CurrentEnv = ""
	}
}

// clearProviderCurrent 清掉某服务商当前激活的配置引用（不关心具体是哪一条）。
// 清除本机配置后必须调用，否则界面仍显示"已应用"，且下次启动的配置差异检测
// 会把刚清掉的配置又同步回去，看起来就像清除没生效。
func (a *App) clearProviderCurrent(provider string) {
	var cleared string
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "claude_desktop":
		cleared = a.config.CurrentEnvClaudeDesktop
		a.config.CurrentEnvClaudeDesktop = ""
	case "codex":
		cleared = a.config.CurrentEnvCodex
		a.config.CurrentEnvCodex = ""
	case "antigravity":
		cleared = a.config.CurrentEnvAntigravity
		a.config.CurrentEnvAntigravity = ""
	case "opencode":
		cleared = a.config.CurrentEnvOpencode
		a.config.CurrentEnvOpencode = ""
		a.config.CurrentEnvsOpencode = nil
	case "grok":
		cleared = a.config.CurrentEnvGrok
		a.config.CurrentEnvGrok = ""
	default:
		cleared = a.config.CurrentEnvClaude
		a.config.CurrentEnvClaude = ""
	}
	if cleared != "" && a.config.CurrentEnv == cleared {
		a.config.CurrentEnv = ""
	}
}

// renameProviderCurrentRef 服务商内改名时迁移当前环境引用
func (a *App) renameProviderCurrentRef(provider, oldName, newName string) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "codex":
		if a.config.CurrentEnvCodex == oldName {
			a.config.CurrentEnvCodex = newName
		}
	case "antigravity":
		if a.config.CurrentEnvAntigravity == oldName {
			a.config.CurrentEnvAntigravity = newName
		}
	case "opencode":
		a.renameOpencodeCurrent(oldName, newName)
	case "grok":
		if a.config.CurrentEnvGrok == oldName {
			a.config.CurrentEnvGrok = newName
		}
	case "claude_desktop":
		if a.config.CurrentEnvClaudeDesktop == oldName {
			a.config.CurrentEnvClaudeDesktop = newName
		}
	default:
		if a.config.CurrentEnvClaude == oldName {
			a.config.CurrentEnvClaude = newName
		}
	}
}

func (a *App) applyEnvByProvider(env *EnvConfig) (string, error) {
	live, err := prepareLiveEnv(env)
	if err != nil {
		return "", err
	}
	switch live.Provider {
	case "claude_desktop":
		return a.applyClaudeDesktopEnv(live)
	case "codex":
		return a.applyCodexEnv(live)
	case "antigravity":
		return a.applyAntigravityEnv(live)
	case "opencode":
		return a.applyOpencodeEnv(live)
	case "grok":
		return a.applyGrokEnv(live)
	default:
		return a.applyClaudeEnv(live)
	}
}

// DeleteEnv deletes an environment configuration by name
func (a *App) DeleteEnv(name string, provider string) error {
	for i, env := range a.config.Environments {
		if env.Name == name && sameProvider(env.Provider, provider) {
			if env.Provider == "opencode" && a.isOpencodeCurrent(name) {
				_ = a.stripOpencodeProvider(&env)
			}
			// Remove environment from slice
			a.config.Environments = append(a.config.Environments[:i], a.config.Environments[i+1:]...)

			// Clear current env references（仅该服务商自己的引用）
			a.clearCurrentEnvRef(env.Provider, name)

			return a.saveConfig()
		}
	}
	return fmt.Errorf("environment '%s' (%s) not found", name, provider)
}

// ReorderEnvs 按给定顺序重排配置列表。
//
// 每一项形如 "provider::name"。名称只在同一服务商内唯一，只给名称时
// 跨服务商的同名配置无法区分，拖动排序会把配置排到别的服务商的位置上；
// 带上服务商前缀才能精确定位。为兼容旧调用，不含 "::" 的项仍按名称贪心匹配。
func (a *App) ReorderEnvs(keys []string) error {
	if len(keys) != len(a.config.Environments) {
		return fmt.Errorf("environment count mismatch")
	}

	used := make([]bool, len(a.config.Environments))
	newEnvs := make([]EnvConfig, 0, len(keys))
	for _, key := range keys {
		provider, name, hasProvider := strings.Cut(key, "::")
		if !hasProvider {
			name = key
		}
		found := -1
		for i, env := range a.config.Environments {
			if used[i] || env.Name != name {
				continue
			}
			if hasProvider && !sameProvider(env.Provider, provider) {
				continue
			}
			found = i
			break
		}
		if found < 0 {
			return fmt.Errorf("environment '%s' not found", name)
		}
		used[found] = true
		newEnvs = append(newEnvs, a.config.Environments[found])
	}

	a.config.Environments = newEnvs
	return a.saveConfig()
}

// TestLatency 测试 URL 延迟。能连上（含 4xx/5xx）即视为测速成功，返回往返毫秒。
func (a *App) TestLatency(urlStr string) (int64, error) {
	urlStr, err := normalizeProbeURL(urlStr)
	if err != nil {
		return 0, err
	}

	client := &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	start := time.Now()
	resp, err := probeURL(client, http.MethodHead, urlStr)
	if err != nil {
		resp, err = probeURL(client, http.MethodGet, urlStr)
	}
	if err != nil {
		return time.Since(start).Milliseconds(), fmt.Errorf("无法连接: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return time.Since(start).Milliseconds(), nil
}

func normalizeProbeURL(urlStr string) (string, error) {
	urlStr = strings.TrimSpace(urlStr)
	if urlStr == "" {
		return "", fmt.Errorf("URL 为空")
	}
	lower := strings.ToLower(urlStr)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		urlStr = "https://" + urlStr
	}
	return urlStr, nil
}

func probeURL(client *http.Client, method, urlStr string) (*http.Response, error) {
	req, err := http.NewRequest(method, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "AI-ENV/"+appVersion)
	req.Header.Set("Accept", "*/*")
	return client.Do(req)
}

// ApplyCurrentEnv 应用当前环境：把每个 Provider 各自激活的环境写入对应 CLI 配置文件
func (a *App) ApplyCurrentEnv() (string, error) {
	var msgs []string
	var errs []string

	apply := func(label, provider, envName string, applyFn func(*EnvConfig) (string, error)) {
		if envName == "" {
			return
		}
		env := a.findEnvIn(provider, envName)
		if env == nil {
			errs = append(errs, fmt.Sprintf("%s: 找不到环境配置 %q", label, envName))
			return
		}
		msg, err := applyFn(env)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", label, err))
			return
		}
		msgs = append(msgs, label+": "+msg)
	}

	apply("Claude", "claude", a.config.CurrentEnvClaude, a.applyEnvByProvider)
	apply("Claude Desktop", "claude_desktop", a.config.CurrentEnvClaudeDesktop, a.applyEnvByProvider)
	apply("Codex", "codex", a.config.CurrentEnvCodex, a.applyEnvByProvider)
	apply("Antigravity", "antigravity", a.config.CurrentEnvAntigravity, a.applyEnvByProvider)
	for _, name := range a.opencodeCurrentNames() {
		apply("OpenCode", "opencode", name, a.applyEnvByProvider)
	}
	apply("Grok", "grok", a.config.CurrentEnvGrok, a.applyEnvByProvider)

	a.syncOpencodeAppliedFromDisk()
	if err := a.saveConfig(); err != nil && len(errs) == 0 {
		return "", err
	}

	if len(msgs) == 0 && len(errs) == 0 {
		return "没有激活的环境可应用", nil
	}

	now := time.Now()
	_ = RecordEnvActivation("claude", a.config.CurrentEnvClaude, now)
	_ = RecordEnvActivation("claude_desktop", a.config.CurrentEnvClaudeDesktop, now)
	_ = RecordEnvActivation("codex", a.config.CurrentEnvCodex, now)
	_ = RecordEnvActivation("antigravity", a.config.CurrentEnvAntigravity, now)
	_ = RecordEnvActivation("opencode", a.config.CurrentEnvOpencode, now)
	_ = RecordEnvActivation("grok", a.config.CurrentEnvGrok, now)

	if len(msgs) == 0 {
		return "", fmt.Errorf("应用失败: %s", strings.Join(errs, "；"))
	}

	result := strings.Join(msgs, "；")
	if len(errs) > 0 {
		result += "；⚠ 部分失败: " + strings.Join(errs, "；")
	}
	return result, nil
}

func (a *App) findEnv(name string) *EnvConfig {
	for _, env := range a.config.Environments {
		if env.Name == name {
			return &env
		}
	}
	return nil
}

// findEnvIn 按服务商 + 名称定位（配置名只在同一服务商内唯一）
func (a *App) findEnvIn(provider, name string) *EnvConfig {
	for _, env := range a.config.Environments {
		if env.Name == name && sameProvider(env.Provider, provider) {
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

// GetClaudeDesktopSettings 读取 Claude Desktop 本地配置并归一化为 Anthropic 字段。
// Claude Desktop 与 Claude Code 的文件格式不同，不能读取 ~/.claude/settings.json。
func (a *App) GetClaudeDesktopSettings() map[string]string {
	path, err := claudeDesktopConfigPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return readClaudeDesktopEnv(data)
}

// GetConfigDrift 返回当前激活环境与本机配置不一致的平台，启动时只用于询问用户是否同步。
func (a *App) GetConfigDrift() []string {
	var drift []string
	for _, item := range []struct{ provider, name, label string }{
		{"claude", a.config.CurrentEnvClaude, "Claude Code"},
		{"claude_desktop", a.config.CurrentEnvClaudeDesktop, "Claude Desktop"},
		{"codex", a.config.CurrentEnvCodex, "Codex"},
		{"antigravity", a.config.CurrentEnvAntigravity, "Antigravity"},
		{"opencode", a.config.CurrentEnvOpencode, "OpenCode"},
		{"grok", a.config.CurrentEnvGrok, "Grok"},
	} {
		if item.name == "" {
			continue
		}
		env := a.findEnvIn(item.provider, item.name)
		if env == nil {
			continue
		}
		var current map[string]string
		switch item.provider {
		case "claude":
			current = a.GetClaudeSettings()
		case "claude_desktop":
			current = a.GetClaudeDesktopSettings()
		case "codex":
			current = a.GetCodexSettings()
		case "antigravity":
			current = a.GetAntigravitySettings()
		case "opencode":
			current = a.GetOpencodeSettings()
		case "grok":
			current = a.GetGrokSettings()
		default:
			continue
		}
		expectedVariables := comparableEnvVariables(item.provider, env.Variables)
		if isAppRoutingOn(item.provider) {
			delete(expectedVariables, providerBaseVariable(item.provider))
		}
		for key, expected := range expectedVariables {
			actual := current[key]
			if item.provider == "claude_desktop" && (key == "ANTHROPIC_AUTH_TOKEN" || key == "ANTHROPIC_API_KEY") {
				// Desktop 3P 配置只有一个 gateway key；它在软件配置里可以
				// 表示为 Auth Token 或 API Key，两种写法都应视为同一个值。
				actual = firstNonEmpty(current["ANTHROPIC_AUTH_TOKEN"], current["ANTHROPIC_API_KEY"])
			}
			if strings.TrimSpace(expected) != strings.TrimSpace(actual) {
				drift = append(drift, item.label)
				break
			}
		}
	}
	return drift
}

func providerBaseVariable(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "claude", "claude_desktop":
		return "ANTHROPIC_BASE_URL"
	case "codex":
		return "base_url"
	case "antigravity":
		return "GOOGLE_GEMINI_BASE_URL"
	case "opencode":
		return "OPENCODE_BASE_URL"
	case "grok":
		return "XAI_BASE_URL"
	default:
		return ""
	}
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
			for _, key := range []string{"model_reasoning_effort", "model_reasoning_summary", "plan_mode_reasoning_effort", "approval_policy", "sandbox_mode", "model_verbosity"} {
				if v, ok := payload[key].(string); ok && strings.TrimSpace(v) != "" {
					result[key] = strings.TrimSpace(v)
				}
			}
			for _, key := range []string{"model_context_window", "model_max_output_tokens", "project_doc_max_bytes"} {
				if n, ok := asInt(payload[key]); ok {
					result[key] = strconv.Itoa(n)
				}
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

// GetAntigravitySettings 读取 Antigravity（原 Gemini CLI）配置
func (a *App) GetAntigravitySettings() map[string]string {
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

	// 覆盖为用户级环境变量里的实际值（agy 只认进程环境变量，这才是它真正读到的配置）
	for key, value := range readAntigravityUserEnv() {
		result[key] = value
	}

	return result
}

// OpenProviderTerminal 打开一个已注入该服务商当前生效环境变量的终端，
// 方便直接运行对应 CLI（尤其 agy 只认环境变量，这样不必等新终端继承用户环境）。
func (a *App) OpenProviderTerminal(provider string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	name := a.currentEnvNameForProvider(provider)
	if name == "" {
		return fmt.Errorf("该平台还没有已应用的配置")
	}
	env := a.findEnvIn(provider, name)
	if env == nil {
		return fmt.Errorf("找不到环境配置 %q", name)
	}
	vars := env.Variables
	// 路由开启时使用实际写入本机的 live 变量（例如指向本地网关的 base URL）
	if live, err := prepareLiveEnv(env); err == nil && live != nil {
		vars = live.Variables
	}
	if len(vars) == 0 {
		return fmt.Errorf("该配置没有可注入的环境变量")
	}
	return openTerminalWithEnv(vars)
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

	// 读取现有的 settings.json (如果存在)。
	// 解析失败时必须中止：早期版本会忽略解析错误直接当成空对象写回，
	// 用户的 permissions / hooks / model / statusLine 等设置会被整个抹掉。
	settings := map[string]interface{}{}
	if data, err := os.ReadFile(settingsFile); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		parsed, parseErr := parseJSONLikeObject(data)
		if parseErr != nil {
			return "", fmt.Errorf("解析 %s 失败，为保护原文件已中止写入: %v", settingsFile, parseErr)
		}
		settings = parsed
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("读取 %s 失败: %v", settingsFile, err)
	}

	// 更新 env 字段
	envMap := make(map[string]string)
	for key, value := range env.Variables {
		if value != "" {
			envMap[key] = value
		}
	}
	// 根据配置添加 Claude Code 优化选项
	if env.AttributionHeader != "" {
		envMap["CLAUDE_CODE_ATTRIBUTION_HEADER"] = env.AttributionHeader
	}
	if env.DisableNonessentialTraffic != "" {
		envMap["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] = env.DisableNonessentialTraffic
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

// applyClaudeDesktopEnv 写入 Claude Desktop 官方配置文件。
// 保留 mcpServers 等用户字段，仅声明式更新顶层 env；若配置模板存在则先使用模板作为基底。
func (a *App) applyClaudeDesktopEnv(env *EnvConfig) (string, error) {
	settingsFile, err := claudeDesktopConfigPath()
	if err != nil {
		return "", fmt.Errorf("获取 Claude Desktop 配置路径失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(settingsFile), 0o755); err != nil {
		return "", fmt.Errorf("创建 Claude Desktop 配置目录失败: %v", err)
	}

	settings := map[string]any{}
	if tmpl := strings.TrimSpace(env.Templates["claude_desktop_config.json"]); tmpl != "" {
		if err := json.Unmarshal([]byte(tmpl), &settings); err != nil {
			return "", fmt.Errorf("解析 Claude Desktop 配置模板失败: %v", err)
		}
		if settings == nil {
			return "", fmt.Errorf("Claude Desktop 配置模板必须是 JSON 对象")
		}
	} else if data, readErr := os.ReadFile(settingsFile); readErr == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return "", fmt.Errorf("解析 %s 失败，为保护原文件已中止写入: %v", settingsFile, err)
		}
		if settings == nil {
			return "", fmt.Errorf("解析 %s 失败：配置必须是 JSON 对象", settingsFile)
		}
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return "", fmt.Errorf("读取 %s 失败: %v", settingsFile, readErr)
	}

	// Claude Desktop 3P 的 configLibrary 使用 inference* 字段；旧版文件仍接受顶层 env。
	isThirdParty := strings.EqualFold(filepath.Base(filepath.Dir(settingsFile)), "configLibrary") ||
		settings["inferenceGatewayBaseUrl"] != nil || settings["inferenceGatewayApiKey"] != nil || settings["inferenceModels"] != nil
	if isThirdParty {
		baseURL := strings.TrimRight(strings.TrimSpace(env.Variables["ANTHROPIC_BASE_URL"]), "/")
		apiKey := strings.TrimSpace(env.Variables["ANTHROPIC_AUTH_TOKEN"])
		if apiKey == "" {
			apiKey = strings.TrimSpace(env.Variables["ANTHROPIC_API_KEY"])
		}
		model := strings.TrimSpace(env.Variables["ANTHROPIC_MODEL"])
		if baseURL != "" {
			settings["inferenceProvider"] = "gateway"
			settings["inferenceGatewayBaseUrl"] = baseURL
		} else {
			delete(settings, "inferenceGatewayBaseUrl")
		}
		if apiKey != "" {
			settings["inferenceGatewayApiKey"] = apiKey
			settings["inferenceGatewayAuthScheme"] = "bearer"
		} else {
			delete(settings, "inferenceGatewayApiKey")
			delete(settings, "inferenceGatewayAuthScheme")
		}
		if model != "" {
			settings["inferenceModels"] = updateClaudeDesktopModels(settings["inferenceModels"], model)
		} else {
			delete(settings, "inferenceModels")
		}
		if baseURL == "" && apiKey == "" {
			delete(settings, "inferenceProvider")
		}
	}

	envMap := map[string]string{}
	for key, value := range env.Variables {
		if strings.TrimSpace(value) != "" {
			envMap[key] = value
		}
	}
	if env.AttributionHeader != "" {
		envMap["CLAUDE_CODE_ATTRIBUTION_HEADER"] = env.AttributionHeader
	}
	if env.DisableNonessentialTraffic != "" {
		envMap["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] = env.DisableNonessentialTraffic
	}
	if len(envMap) > 0 && !isThirdParty {
		settings["env"] = envMap
	} else if !isThirdParty {
		delete(settings, "env")
	}

	content, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化 Claude Desktop 配置失败: %v", err)
	}
	if err := os.WriteFile(settingsFile, content, 0o600); err != nil {
		return "", fmt.Errorf("写入 Claude Desktop 配置失败: %v", err)
	}
	if isThirdParty {
		if err := writeClaudeDesktopMeta(settingsFile); err != nil {
			return "", err
		}
	}
	return "Claude Desktop 配置已应用到 " + settingsFile, nil
}

// comparableEnvVariables 只比较某个平台实际写入的字段，避免 Claude Desktop
// 因为 Claude Code 专属环境变量而被启动时误报为“配置有差异”。
func comparableEnvVariables(provider string, variables map[string]string) map[string]string {
	if variables == nil {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(provider), "claude_desktop") {
		out := make(map[string]string, 3)
		for _, key := range []string{"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY", "ANTHROPIC_MODEL"} {
			if value := strings.TrimSpace(variables[key]); value != "" {
				out[key] = value
			}
		}
		return out
	}
	out := make(map[string]string, len(variables))
	for key, value := range variables {
		if strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	return out
}

func writeClaudeDesktopMeta(configFile string) error {
	dir := filepath.Dir(configFile)
	if !strings.EqualFold(filepath.Base(dir), "configLibrary") {
		return nil
	}
	id := strings.TrimSuffix(filepath.Base(configFile), filepath.Ext(configFile))
	if !isClaudeDesktopConfigID(id) {
		return fmt.Errorf("Claude Desktop 配置文件名无效")
	}
	metaPath := filepath.Join(dir, "_meta.json")
	meta := map[string]any{"appliedId": id, "entries": []any{}}
	if data, err := os.ReadFile(metaPath); err == nil && len(data) > 0 {
		var existing map[string]any
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("解析 Claude Desktop 索引失败，为保护原索引已中止写入: %v", err)
		}
		for key, value := range existing {
			meta[key] = value
		}
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取 Claude Desktop 索引失败: %v", err)
	}
	meta["appliedId"] = id
	entries, _ := meta["entries"].([]any)
	found := false
	for i, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok || strings.TrimSpace(asString(entry["id"])) != id {
			continue
		}
		if strings.TrimSpace(asString(entry["name"])) == "" {
			entry["name"] = "AI ENV"
		}
		entries[i] = entry
		found = true
	}
	if !found {
		entries = append(entries, map[string]any{"id": id, "name": "AI ENV"})
	}
	meta["entries"] = entries
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 Claude Desktop 索引失败: %v", err)
	}
	if err := os.WriteFile(metaPath, data, 0o600); err != nil {
		return fmt.Errorf("写入 Claude Desktop 索引失败: %v", err)
	}
	return nil
}

func removeClaudeDesktopMetaEntry(dir, id string) error {
	if !isClaudeDesktopConfigID(id) {
		return nil
	}
	metaPath := filepath.Join(dir, "_meta.json")
	data, err := os.ReadFile(metaPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var meta map[string]any
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("解析 Claude Desktop 索引失败: %v", err)
	}
	entries, _ := meta["entries"].([]any)
	kept := make([]any, 0, len(entries))
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if ok && strings.TrimSpace(asString(entry["id"])) == id {
			continue
		}
		kept = append(kept, raw)
	}
	meta["entries"] = kept
	if strings.TrimSpace(asString(meta["appliedId"])) == id {
		if len(kept) > 0 {
			if first, ok := kept[0].(map[string]any); ok {
				meta["appliedId"] = strings.TrimSpace(asString(first["id"]))
			}
		} else {
			delete(meta, "appliedId")
		}
	}
	updated, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 Claude Desktop 索引失败: %v", err)
	}
	if len(kept) == 0 && len(meta) == 0 {
		return os.Remove(metaPath)
	}
	return os.WriteFile(metaPath, updated, 0o600)
}

func updateClaudeDesktopModels(raw any, model string) []map[string]string {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil
	}
	result := make([]map[string]string, 0, 1)
	if list, ok := raw.([]any); ok {
		for _, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			entry := map[string]string{}
			for _, key := range []string{"name", "labelOverride", "id"} {
				if value := strings.TrimSpace(asString(m[key])); value != "" {
					entry[key] = value
				}
			}
			if entry["name"] == "" && entry["id"] == "" {
				continue
			}
			result = append(result, entry)
		}
	}
	if len(result) == 0 {
		return []map[string]string{{"name": model}}
	}
	result[0]["name"] = model
	return result
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

	variables := env.Variables

	// 1. 处理 config.toml
	var configContent string
	if tmpl, ok := env.Templates["config.toml"]; ok && tmpl != "" {
		// 使用自定义模板，替换变量
		configContent = tmpl
		configContent = strings.ReplaceAll(configContent, "{{model}}", variables["model"])
		configContent = strings.ReplaceAll(configContent, "{{base_url}}", variables["base_url"])
	} else {
		// 使用默认模板
		configContent = fmt.Sprintf(`model_provider = "duckcoding"
model = "%s"
model_reasoning_effort = "high"

[model_providers.duckcoding]
name = "duckcoding"
base_url = "%s"
wire_api = "responses"
requires_openai_auth = true
`, variables["model"], variables["base_url"])
	}

	configFile := filepath.Join(codexDir, "config.toml")
	configData, err := buildCodexConfigData(configContent, configFile, variables)
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

func buildCodexConfigData(configContent, configFile string, vars map[string]string) ([]byte, error) {
	existingMcpServers := readCodexMcpServers(configFile)
	var payload map[string]any
	if err := toml.Unmarshal([]byte(configContent), &payload); err == nil && payload != nil {
		injectCodexExtras(payload, vars)
		if len(existingMcpServers) > 0 {
			if _, ok := payload["mcp_servers"]; !ok {
				payload["mcp_servers"] = existingMcpServers
			}
		}
		sanitizeCodexConfigPayload(payload)
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

// applyAntigravityEnv 应用 Antigravity CLI（原 Gemini CLI，命令 agy）配置
func (a *App) applyAntigravityEnv(env *EnvConfig) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户目录失败: %v", err)
	}

	geminiDir := filepath.Join(homeDir, ".gemini")
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
		return "", fmt.Errorf("创建 .gemini 目录失败: %v", err)
	}

	// 1. 处理 .env 文件（旧 Gemini CLI 兼容；本工具也通过它记录当前生效值）
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

	envContent = appendMissingEnvLines(envContent, env.Variables, []string{
		"GOOGLE_GEMINI_BASE_URL",
		"GEMINI_API_KEY",
		"GEMINI_MODEL",
		"GOOGLE_API_KEY",
		"GOOGLE_CLOUD_PROJECT",
		"GOOGLE_CLOUD_LOCATION",
		"GOOGLE_GENAI_USE_VERTEXAI",
		"GEMINI_SANDBOX",
	})

	envFile := filepath.Join(geminiDir, ".env")
	if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
		return "", fmt.Errorf("写入 .env 失败: %v", err)
	}

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

	// 2. 旧位置 ~/.gemini/settings.json（兼容仍装有 gemini 命令的旧环境）
	if err := writeGeminiStyleSettings(filepath.Join(geminiDir, "settings.json"), desiredSettings, env.Variables); err != nil {
		return "", err
	}

	// 3. 新位置 ~/.gemini/antigravity-cli/settings.json（agy 实际读取的配置）。
	//    官方规则：modelProvider 为 "gemini" 时必须有 GEMINI_API_KEY 环境变量，否则 agy 拒绝启动；
	//    没配 API Key 就不写 modelProvider，让 agy 走默认的 Google 账号登录。
	antigravityDir := filepath.Join(geminiDir, "antigravity-cli")
	if err := os.MkdirAll(antigravityDir, 0755); err != nil {
		return "", fmt.Errorf("创建 antigravity-cli 目录失败: %v", err)
	}
	apiKey := strings.TrimSpace(env.Variables["GEMINI_API_KEY"])
	antigravityDesired := maps.Clone(desiredSettings)
	if antigravityDesired == nil {
		antigravityDesired = map[string]any{}
	}
	if apiKey != "" {
		if _, ok := antigravityDesired["modelProvider"]; !ok {
			antigravityDesired["modelProvider"] = "gemini"
		}
	} else {
		delete(antigravityDesired, "modelProvider")
	}
	if err := writeGeminiStyleSettings(filepath.Join(antigravityDir, "settings.json"), antigravityDesired, env.Variables); err != nil {
		return "", err
	}
	// writeGeminiStyleSettings 是合并写入，key 为空时需要显式移除 modelProvider
	if apiKey == "" {
		removeJSONFileKeys(filepath.Join(antigravityDir, "settings.json"), "modelProvider")
	}

	// 4. agy 只从进程环境变量读取凭据和端点（官方明确不加载 .env，settings.json 也不存 key），
	//    把 GEMINI_API_KEY / GOOGLE_GEMINI_BASE_URL 持久化到用户级环境变量；
	//    声明式同步：未配置的键会被清掉，避免上一套配置的残留值串台。
	persistVars := map[string]string{}
	if apiKey != "" {
		persistVars["GEMINI_API_KEY"] = apiKey
	}
	if baseURL := strings.TrimSpace(env.Variables["GOOGLE_GEMINI_BASE_URL"]); baseURL != "" {
		persistVars["GOOGLE_GEMINI_BASE_URL"] = baseURL
	}
	if err := syncAntigravityUserEnv(persistVars); err != nil {
		return "", fmt.Errorf("配置已写入，但更新用户环境变量失败: %v", err)
	}

	if apiKey != "" {
		return "Antigravity CLI 配置已应用；密钥已写入用户环境变量，请新开终端后运行 agy", nil
	}
	return "Antigravity CLI 配置已应用", nil
}

// removeJSONFileKeys 从 JSON 文件顶层移除指定键（文件不存在则忽略）
func removeJSONFileKeys(path string, keys ...string) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return
	}
	payload := map[string]any{}
	if json.Unmarshal(data, &payload) != nil {
		return
	}
	changed := false
	for _, key := range keys {
		if _, ok := payload[key]; ok {
			delete(payload, key)
			changed = true
		}
	}
	if !changed {
		return
	}
	if out, err := json.MarshalIndent(payload, "", "  "); err == nil {
		_ = os.WriteFile(path, out, 0644)
	}
}

// writeGeminiStyleSettings 将期望配置合并进现有 settings.json 并写回，保留用户已有的其他设置
func writeGeminiStyleSettings(settingsFile string, desiredSettings map[string]any, vars map[string]string) error {
	existingSettings := map[string]any{}
	if data, err := os.ReadFile(settingsFile); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &existingSettings); err != nil {
			existingSettings = map[string]any{}
		}
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取 %s 失败: %v", settingsFile, err)
	}

	deepMergeMap(existingSettings, desiredSettings)
	injectGeminiSettingsExtras(existingSettings, vars)
	settingsContent, err := json.MarshalIndent(existingSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 %s 失败: %v", settingsFile, err)
	}

	if err := os.WriteFile(settingsFile, settingsContent, 0644); err != nil {
		return fmt.Errorf("写入 %s 失败: %v", settingsFile, err)
	}
	return nil
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

func parseJSONLikeObject(data []byte) (map[string]any, error) {
	payload := map[string]any{}
	if err := json.Unmarshal(data, &payload); err == nil {
		return payload, nil
	} else {
		payload = map[string]any{}
		if err5 := json5.Unmarshal(data, &payload); err5 == nil {
			return payload, nil
		} else {
			return nil, fmt.Errorf("配置文件不是有效 JSON/JSON5（json: %v; json5: %v）", err, err5)
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func expandAndNormalizePath(pathValue, homeDir, relativeBase string) string {
	expanded := strings.TrimSpace(pathValue)
	if expanded == "" {
		return ""
	}

	expanded = os.ExpandEnv(expanded)
	expanded = expandPercentEnv(expanded)

	if expanded == "~" {
		expanded = homeDir
	} else if strings.HasPrefix(expanded, "~\\") || strings.HasPrefix(expanded, "~/") {
		expanded = filepath.Join(homeDir, expanded[2:])
	}

	if !filepath.IsAbs(expanded) {
		expanded = filepath.Join(relativeBase, expanded)
	}

	return filepath.Clean(expanded)
}

var percentEnvVarPattern = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)

func expandPercentEnv(value string) string {
	return percentEnvVarPattern.ReplaceAllStringFunc(value, func(token string) string {
		name := strings.Trim(token, "%")
		if v := os.Getenv(name); strings.TrimSpace(v) != "" {
			return v
		}
		return token
	})
}

// ClearClaudeSettings 清除 Claude settings.json 中的 env 配置
func (a *App) ClearClaudeSettings() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %v", err)
	}

	settingsFile := filepath.Join(homeDir, ".claude", "settings.json")

	// 读取现有的 settings.json
	data, readErr := os.ReadFile(settingsFile)
	if readErr != nil || len(strings.TrimSpace(string(data))) == 0 {
		if readErr != nil && !os.IsNotExist(readErr) {
			return fmt.Errorf("读取 %s 失败: %v", settingsFile, readErr)
		}
		// 文件不存在，本机已经是干净的；激活状态仍要一起清掉
		a.clearProviderCurrent("claude")
		return a.saveConfig()
	}
	settings, parseErr := parseJSONLikeObject(data)
	if parseErr != nil {
		return fmt.Errorf("解析 %s 失败，为保护原文件已中止写入: %v", settingsFile, parseErr)
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

	a.clearProviderCurrent("claude")
	return a.saveConfig()
}

// ClearClaudeDesktopSettings 清除 Claude Desktop 当前用户配置。
func (a *App) ClearClaudeDesktopSettings() error {
	path, err := claudeDesktopConfigPath()
	if err != nil {
		return err
	}
	if strings.EqualFold(filepath.Base(filepath.Dir(path)), "configLibrary") {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := removeClaudeDesktopMetaEntry(filepath.Dir(path), strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))); err != nil {
			return err
		}
		oldName := a.config.CurrentEnvClaudeDesktop
		a.config.CurrentEnvClaudeDesktop = ""
		if a.config.CurrentEnv == oldName {
			a.config.CurrentEnv = ""
		}
		return a.saveConfig()
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	oldName := a.config.CurrentEnvClaudeDesktop
	a.config.CurrentEnvClaudeDesktop = ""
	if a.config.CurrentEnv == oldName {
		a.config.CurrentEnv = ""
	}
	if err := a.saveConfig(); err != nil {
		return err
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

	a.clearProviderCurrent("codex")
	return a.saveConfig()
}

// ClearAntigravitySettings 清除 Antigravity（原 Gemini CLI）配置文件
func (a *App) ClearAntigravitySettings() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %v", err)
	}

	geminiDir := filepath.Join(homeDir, ".gemini")

	// 删除配置文件
	os.Remove(filepath.Join(geminiDir, ".env"))

	// 移除持久化到用户环境的 agy 变量
	if err := syncAntigravityUserEnv(nil); err != nil {
		return fmt.Errorf("清理用户环境变量失败: %v", err)
	}

	// 清除 agy 配置里的认证信息（保留用户的其他偏好设置）
	antigravitySettings := filepath.Join(geminiDir, "antigravity-cli", "settings.json")
	if data, err := os.ReadFile(antigravitySettings); err == nil && len(data) > 0 {
		payload := map[string]any{}
		if json.Unmarshal(data, &payload) == nil {
			changed := false
			if _, ok := payload["modelProvider"]; ok {
				delete(payload, "modelProvider")
				changed = true
			}
			if security, ok := payload["security"].(map[string]any); ok {
				if _, ok := security["auth"]; ok {
					delete(security, "auth")
					changed = true
				}
			}
			if changed {
				if out, err := json.MarshalIndent(payload, "", "  "); err == nil {
					_ = os.WriteFile(antigravitySettings, out, 0644)
				}
			}
		}
	}

	a.clearProviderCurrent("antigravity")
	return a.saveConfig()
}

// ClearAllEnv 清除所有配置 (Claude/Codex/Antigravity/OpenCode/Grok)
func (a *App) ClearAllEnv() error {
	var errors []string

	if err := a.ClearClaudeSettings(); err != nil {
		errors = append(errors, fmt.Sprintf("Claude: %v", err))
	}
	if err := a.ClearClaudeDesktopSettings(); err != nil {
		errors = append(errors, fmt.Sprintf("Claude Desktop: %v", err))
	}

	if err := a.ClearCodexSettings(); err != nil {
		errors = append(errors, fmt.Sprintf("Codex: %v", err))
	}

	if err := a.ClearAntigravitySettings(); err != nil {
		errors = append(errors, fmt.Sprintf("Antigravity: %v", err))
	}

	if err := a.ClearOpencodeSettings(); err != nil {
		errors = append(errors, fmt.Sprintf("OpenCode: %v", err))
	}

	if err := a.ClearGrokSettings(); err != nil {
		errors = append(errors, fmt.Sprintf("Grok: %v", err))
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
	return a.mergeImportedConfigJSON(data)
}

// ImportConfigJSON 从 JSON 文本导入配置（拖拽/粘贴）
func (a *App) ImportConfigJSON(payload string) (int, error) {
	return a.mergeImportedConfigJSON([]byte(payload))
}

// ReadDroppedFile 读取用户拖入的本地文本文件（导入预览用）
func (a *App) ReadDroppedFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("路径为空")
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case "", ".json", ".txt", ".jsonc":
	default:
		return "", fmt.Errorf("只支持 JSON 文件")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %v", err)
	}
	if len(data) > 8<<20 {
		return "", fmt.Errorf("文件太大（上限 8MB）")
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	return string(data), nil
}

func (a *App) mergeImportedConfigJSON(data []byte) (int, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var importedConfig Config
	if err := json.Unmarshal(data, &importedConfig); err != nil {
		return 0, fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 名称只在同一服务商内唯一，重名判断必须带上 provider，
	// 否则 codex 下的同名配置会被无谓地改成 xxx_imported_1
	normProvider := func(p string) string {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			return "claude"
		}
		if p == "gemini" {
			return "antigravity"
		}
		return p
	}
	type envKey struct{ name, provider string }
	existingKeys := make(map[envKey]bool)
	for _, env := range a.config.Environments {
		existingKeys[envKey{env.Name, normProvider(env.Provider)}] = true
	}

	importCount := 0
	for _, importedEnv := range importedConfig.Environments {
		// 导入的配置可能没有 provider（旧版导出），不归一就会在按平台筛选时看不见
		importedEnv.Provider = normProvider(importedEnv.Provider)
		key := envKey{importedEnv.Name, importedEnv.Provider}
		if existingKeys[key] {
			suffix := 1
			for {
				newName := fmt.Sprintf("%s_imported_%d", importedEnv.Name, suffix)
				if !existingKeys[envKey{newName, importedEnv.Provider}] {
					importedEnv.Name = newName
					break
				}
				suffix++
			}
		}
		a.config.Environments = append(a.config.Environments, importedEnv)
		existingKeys[envKey{importedEnv.Name, importedEnv.Provider}] = true
		importCount++
	}

	if importCount == 0 {
		return 0, fmt.Errorf("文件里没有可导入的环境配置")
	}
	if err := a.saveConfig(); err != nil {
		return 0, fmt.Errorf("保存配置失败: %v", err)
	}
	return importCount, nil
}

func (a *App) loadConfig() error {
	a.configLoadErr = nil
	// 如果配置文件不存在，创建默认配置
	if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
		a.config = Config{
			Environments: []EnvConfig{
				{
					Name:        "Development",
					Description: "开发环境",
					Provider:    "claude",
					Variables: map[string]string{
						"ANTHROPIC_API_KEY": "your-dev-api-key",
						"CLAUDE_MODEL":      "claude-3-5-sonnet-20241022",
						"API_BASE_URL":      "https://api.anthropic.com",
					},
				},
				{
					Name:        "Production",
					Description: "生产环境",
					Provider:    "claude",
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
		a.configLoadErr = fmt.Errorf("读取配置文件失败 (%s): %v", a.configPath, err)
		return a.configLoadErr
	}

	// 先解析到临时变量，解析失败时不能污染已有的 a.config
	// （json.Unmarshal 出错前可能已经写进去一半字段）。
	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		a.configLoadErr = fmt.Errorf("解析配置文件失败 (%s): %v", a.configPath, err)
		return a.configLoadErr
	}
	a.config = loaded

	// 兼容旧配置：未设置 provider 时默认归到 claude；
	// gemini 平台已更名为 antigravity（Gemini CLI 于 2026-06 停服，由 Antigravity CLI 接替）
	for i := range a.config.Environments {
		provider := strings.TrimSpace(a.config.Environments[i].Provider)
		if provider == "" {
			a.config.Environments[i].Provider = "claude"
		} else if strings.EqualFold(provider, "gemini") {
			a.config.Environments[i].Provider = "antigravity"
		}
	}
	// 旧字段 current_env_gemini 迁移到 current_env_antigravity
	if a.config.CurrentEnvAntigravity == "" {
		var legacy struct {
			CurrentEnvGemini string `json:"current_env_gemini"`
		}
		if json.Unmarshal(data, &legacy) == nil && legacy.CurrentEnvGemini != "" {
			a.config.CurrentEnvAntigravity = legacy.CurrentEnvGemini
		}
	}
	// 移除已废弃的 openclaw 配置（provider 已替换为 opencode，旧变量无法直接迁移）
	kept := a.config.Environments[:0]
	for _, env := range a.config.Environments {
		if strings.EqualFold(strings.TrimSpace(env.Provider), "openclaw") {
			continue
		}
		kept = append(kept, env)
	}
	a.config.Environments = kept
	if strings.TrimSpace(a.config.CurrentEnvClaude) == "" && strings.TrimSpace(a.config.CurrentEnv) != "" {
		a.config.CurrentEnvClaude = a.config.CurrentEnv
	}
	a.normalizeOpencodeCurrents()

	return nil
}

func (a *App) saveConfig() error {
	if a.configLoadErr != nil {
		return fmt.Errorf("%v；为避免覆盖已有配置已暂停写入，请修复或备份后删除该文件再重启", a.configLoadErr)
	}

	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	dir := filepath.Dir(a.configPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建配置目录失败 (%s): %v", dir, err)
		}
	}

	// 先写临时文件再原子替换：中途崩溃/断电只会留下临时文件，
	// 不会把 config.json 截断成半截 JSON（那正是下次启动解析失败的根源）。
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("保存配置文件失败 (%s): %v", a.configPath, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("保存配置文件失败 (%s): %v", a.configPath, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("保存配置文件失败 (%s): %v", a.configPath, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("保存配置文件失败 (%s): %v", a.configPath, err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("保存配置文件失败 (%s): %v", a.configPath, err)
	}
	renameErr := os.Rename(tmpName, a.configPath)
	for i := 0; renameErr != nil && i < 4; i++ {
		// Windows 上配置文件可能被杀软或同步工具短暂占用，重试几次再说
		time.Sleep(60 * time.Millisecond)
		renameErr = os.Rename(tmpName, a.configPath)
	}
	if renameErr != nil {
		os.Remove(tmpName)
		if err := os.WriteFile(a.configPath, data, 0o644); err != nil {
			return fmt.Errorf("保存配置文件失败 (%s): %v", a.configPath, err)
		}
	}

	notifyCloudSync()
	return nil
}

// PromptFile 提示词文件信息
type PromptFile struct {
	Provider string `json:"provider"` // claude, codex, antigravity, opencode, grok
	Path     string `json:"path"`     // 文件路径
	Content  string `json:"content"`  // 文件内容
	Exists   bool   `json:"exists"`   // 文件是否存在
}

// GetPromptFiles 获取所有提示词文件
func (a *App) GetPromptFiles() ([]PromptFile, error) {
	// Claude Desktop 没有 Claude Code 那样的全局提示词文件；它的行为由
	// configLibrary / MCP 配置管理，因此不放进这个提示词文件列表。
	files := make([]PromptFile, 0, 5)
	for _, provider := range []string{"claude", "codex", "antigravity", "opencode", "grok"} {
		path, err := promptFilePath(provider)
		if err != nil {
			return nil, err
		}
		files = append(files, PromptFile{Provider: provider, Path: path})
	}

	for i := range files {
		if data, err := os.ReadFile(files[i].Path); err == nil {
			files[i].Content = string(data)
			files[i].Exists = true
		} else if os.IsNotExist(err) {
			files[i].Content = ""
			files[i].Exists = false
		} else {
			return nil, fmt.Errorf("读取 %s 失败: %v", files[i].Path, err)
		}
	}

	return files, nil
}

// GetPromptFile 获取单个提示词文件
func (a *App) GetPromptFile(provider string) (PromptFile, error) {
	filePath, err := promptFilePath(provider)
	if err != nil {
		return PromptFile{}, err
	}

	file := PromptFile{Provider: provider, Path: filePath}
	if data, err := os.ReadFile(filePath); err == nil {
		file.Content = string(data)
		file.Exists = true
	} else if !os.IsNotExist(err) {
		return PromptFile{}, fmt.Errorf("读取 %s 失败: %v", filePath, err)
	}

	return file, nil
}

// SavePromptFile 保存提示词文件
func (a *App) SavePromptFile(provider, content string) error {
	filePath, err := promptFilePath(provider)
	if err != nil {
		return err
	}
	dirPath := filepath.Dir(filePath)

	// 确保目录存在
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}
	// 读回校验，避免权限、同步软件或文件映射导致界面显示已保存但磁盘内容未覆盖。
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("保存后读取文件失败: %v", err)
	}
	if string(data) != content {
		return fmt.Errorf("保存后校验失败，文件内容未完整覆盖")
	}

	return nil
}

// DeletePromptFile 删除提示词文件
func (a *App) DeletePromptFile(provider string) error {
	filePath, err := promptFilePath(provider)
	if err != nil {
		return err
	}

	// 删除文件（如果不存在则忽略）
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除文件失败: %v", err)
	}

	return nil
}

func promptFilePath(provider string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户目录失败: %v", err)
	}
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "claude":
		return filepath.Join(homeDir, ".claude", "CLAUDE.md"), nil
	case "codex":
		codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME"))
		if codexHome == "" {
			codexHome = filepath.Join(homeDir, ".codex")
		} else {
			codexHome = expandAndNormalizePath(codexHome, homeDir, filepath.Join(homeDir, ".codex"))
		}
		return filepath.Join(codexHome, "AGENTS.md"), nil
	case "antigravity":
		return filepath.Join(homeDir, ".gemini", "GEMINI.md"), nil
	case "opencode":
		return filepath.Join(resolveOpencodeConfigDir(nil), "AGENTS.md"), nil
	case "grok":
		return filepath.Join(resolveGrokHome(nil), "GROK.md"), nil
	default:
		return "", fmt.Errorf("未知的 Provider: %s", provider)
	}
}
