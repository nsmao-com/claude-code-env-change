package main

import (
	"fmt"
	"maps"
	"net/url"
	"strings"
)

// 上游格式取值：
//
//	""                     —— 原生直连（默认）
//	"chat_completions"     —— OpenAI Chat Completions（需开启该模型商路由才转换）
//	"anthropic_messages"   —— Anthropic Messages（需开启该模型商路由才转换）
//	"responses"            —— OpenAI Responses（需开启该模型商路由才转换）
const (
	UpstreamChatCompletions   = "chat_completions"
	UpstreamAnthropicMessages = "anthropic_messages"
	UpstreamResponses         = "responses"
)

func normalizeUpstreamFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case UpstreamChatCompletions:
		return UpstreamChatCompletions
	case UpstreamAnthropicMessages:
		return UpstreamAnthropicMessages
	case UpstreamResponses:
		return UpstreamResponses
	default:
		return ""
	}
}

func upstreamFormatLabel(format string) string {
	switch format {
	case UpstreamChatCompletions:
		return "Chat Completions"
	case UpstreamAnthropicMessages:
		return "Anthropic Messages"
	case UpstreamResponses:
		return "Responses"
	}
	return "原生"
}

func providerRouteName(provider string) string {
	if p, ok := knownProvider(provider); ok {
		return p
	}
	return "claude"
}

func sanitizeAutoRouteName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "env"
	}
	if len(out) > 50 {
		out = out[:50]
	}
	return "env-" + out
}

func defaultUpstreamURL(provider string) string {
	switch provider {
	case "claude":
		return "https://api.anthropic.com"
	case "codex":
		return "https://api.openai.com/v1"
	case "gemini":
		return "https://generativelanguage.googleapis.com"
	case "grok":
		return "https://api.x.ai/v1"
	default:
		return ""
	}
}

func upstreamVarsForEnv(env *EnvConfig) (baseURL, apiKey, model string) {
	switch env.Provider {
	case "claude":
		baseURL = env.Variables["ANTHROPIC_BASE_URL"]
		apiKey = env.Variables["ANTHROPIC_AUTH_TOKEN"]
		if apiKey == "" {
			apiKey = env.Variables["ANTHROPIC_API_KEY"]
		}
		model = env.Variables["ANTHROPIC_MODEL"]
	case "codex":
		baseURL = env.Variables["base_url"]
		apiKey = env.Variables["OPENAI_API_KEY"]
		model = env.Variables["model"]
	case "gemini":
		baseURL = env.Variables["GOOGLE_GEMINI_BASE_URL"]
		apiKey = env.Variables["GEMINI_API_KEY"]
		if apiKey == "" {
			apiKey = env.Variables["GOOGLE_API_KEY"]
		}
		model = env.Variables["GEMINI_MODEL"]
	case "opencode":
		baseURL = env.Variables["OPENCODE_BASE_URL"]
		apiKey = env.Variables["OPENCODE_API_KEY"]
		model = env.Variables["OPENCODE_MODEL"]
	case "grok":
		baseURL = env.Variables["XAI_BASE_URL"]
		apiKey = env.Variables["XAI_API_KEY"]
		model = env.Variables["XAI_MODEL"]
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultUpstreamURL(env.Provider)
	}
	return baseURL, apiKey, model
}

func needsConversion(env *EnvConfig) bool {
	if env == nil {
		return false
	}
	format := normalizeUpstreamFormat(env.UpstreamFormat)
	if format == "" {
		return false
	}
	switch env.Provider {
	case "claude":
		return format == UpstreamChatCompletions || format == UpstreamResponses
	case "codex", "grok":
		return format == UpstreamChatCompletions || format == UpstreamAnthropicMessages
	case "opencode":
		return format == UpstreamAnthropicMessages || format == UpstreamResponses
	case "gemini":
		return format == UpstreamChatCompletions || format == UpstreamAnthropicMessages || format == UpstreamResponses
	}
	return false
}

func needsRouting(env *EnvConfig) bool {
	return needsConversion(env)
}

func sourceFormatForEnv(env *EnvConfig) string {
	switch env.Provider {
	case "claude":
		return "anthropic"
	case "grok":
		if strings.EqualFold(strings.TrimSpace(env.Variables["XAI_API_BACKEND"]), "messages") {
			return "anthropic"
		}
		return "openai"
	default:
		return "openai"
	}
}

func targetFormatForEnv(env *EnvConfig) string {
	format := normalizeUpstreamFormat(env.UpstreamFormat)
	switch format {
	case UpstreamAnthropicMessages:
		return "anthropic"
	case UpstreamChatCompletions:
		return "openai"
	case UpstreamResponses:
		return "responses"
	}
	switch env.Provider {
	case "claude":
		return "anthropic"
	case "opencode":
		return "openai"
	case "gemini":
		return "openai"
	default:
		return "responses"
	}
}

func isAppRoutingOn(provider string) bool {
	rs := globalRouterService
	if rs == nil {
		return false
	}
	p, ok := knownProvider(provider)
	if !ok {
		return false
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rs.config.AppRouting == nil {
		return false
	}
	return rs.config.AppRouting[p]
}

func cloneEnvConfig(env *EnvConfig) *EnvConfig {
	if env == nil {
		return nil
	}
	out := *env
	out.Variables = maps.Clone(env.Variables)
	if out.Variables == nil {
		out.Variables = map[string]string{}
	}
	if env.Templates != nil {
		out.Templates = maps.Clone(env.Templates)
	}
	return &out
}

func routerPort(rs *RouterService) int {
	rs.mu.Lock()
	port := rs.config.Port
	rs.mu.Unlock()
	if port <= 0 {
		port = defaultRouterPort
	}
	return port
}

func routerLocalBase(rs *RouterService, provider string) string {
	return fmt.Sprintf("http://127.0.0.1:%d/%s", routerPort(rs), providerRouteName(provider))
}

func originalURLHasV1(original string) bool {
	u := strings.ToLower(strings.TrimRight(strings.TrimSpace(original), "/"))
	return strings.HasSuffix(u, "/v1") || strings.Contains(u, "/v1/")
}

func rewriteLiveBaseURL(env *EnvConfig, localBase string) {
	base := strings.TrimRight(localBase, "/")
	switch env.Provider {
	case "claude":
		env.Variables["ANTHROPIC_BASE_URL"] = base
	case "codex":
		env.Variables["base_url"] = base + "/v1"
	case "gemini":
		env.Variables["GOOGLE_GEMINI_BASE_URL"] = base
	case "opencode":
		orig := env.Variables["OPENCODE_BASE_URL"]
		if orig == "" || originalURLHasV1(orig) {
			env.Variables["OPENCODE_BASE_URL"] = base + "/v1"
		} else {
			env.Variables["OPENCODE_BASE_URL"] = base
		}
	case "grok":
		orig := env.Variables["XAI_BASE_URL"]
		if orig == "" || originalURLHasV1(orig) {
			env.Variables["XAI_BASE_URL"] = base + "/v1"
		} else {
			env.Variables["XAI_BASE_URL"] = base
		}
	}
}

func prepareLiveEnv(env *EnvConfig) (*EnvConfig, error) {
	live := cloneEnvConfig(env)
	if live == nil {
		return nil, fmt.Errorf("环境配置为空")
	}
	if isAppRoutingOn(live.Provider) {
		localBase, err := wireRouterForEnv(live)
		if err != nil {
			return nil, err
		}
		rewriteLiveBaseURL(live, localBase)
	} else {
		restoreOriginalRouting(live)
	}
	return live, nil
}

func wireRouterForEnv(env *EnvConfig) (string, error) {
	rs := globalRouterService
	if rs == nil {
		return "", fmt.Errorf("路由服务未初始化")
	}

	format := normalizeUpstreamFormat(env.UpstreamFormat)
	baseURL, apiKey, model := upstreamVarsForEnv(env)
	if strings.TrimSpace(baseURL) == "" {
		return "", fmt.Errorf("此配置未填写上游 Base URL，无法开启路由")
	}
	if _, err := url.Parse(baseURL); err != nil {
		return "", fmt.Errorf("上游 Base URL 无效: %v", err)
	}

	route := APIRoute{
		Name:         providerRouteName(env.Provider),
		Description:  fmt.Sprintf("配置 %q 的应用路由（上游格式: %s）", env.Name, upstreamFormatLabel(format)),
		SourceFormat: sourceFormatForEnv(env),
		TargetFormat: targetFormatForEnv(env),
		BaseURL:      strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		APIKey:       apiKey,
		DefaultModel: model,
		Enabled:      true,
	}

	if err := rs.upsertAutoRoute(route); err != nil {
		return "", fmt.Errorf("写入路由配置失败: %v", err)
	}
	_ = removeAutoRouteByName(sanitizeAutoRouteName(env.Name))

	rs.mu.Lock()
	running := rs.running
	rs.mu.Unlock()
	if !running {
		if err := rs.StartGateway(); err != nil {
			if needsConversion(env) {
				return "", fmt.Errorf("此供应商使用 %s 接口格式，需要路由服务才能正常工作，请先启动路由（%v）", upstreamFormatLabel(format), err)
			}
			return "", fmt.Errorf("启动路由网关失败: %v", err)
		}
	}

	return routerLocalBase(rs, env.Provider), nil
}

func restoreOriginalRouting(env *EnvConfig) {
	if env == nil {
		return
	}
	_ = removeAutoRouteByName(providerRouteName(env.Provider))
	_ = removeAutoRouteByName(sanitizeAutoRouteName(env.Name))
}

func removeAutoRouteByName(name string) error {
	rs := globalRouterService
	if rs == nil || strings.TrimSpace(name) == "" {
		return nil
	}
	rs.mu.Lock()
	filtered := make([]APIRoute, 0, len(rs.config.Routes))
	found := false
	for _, route := range rs.config.Routes {
		if strings.EqualFold(route.Name, name) {
			found = true
			continue
		}
		filtered = append(filtered, route)
	}
	if !found {
		rs.mu.Unlock()
		return nil
	}
	rs.config.Routes = filtered
	cfg := rs.config
	rs.mu.Unlock()
	return rs.SaveRouterConfig(cfg)
}

func (rs *RouterService) upsertAutoRoute(route APIRoute) error {
	rs.mu.Lock()
	config := rs.config
	replaced := false
	for i := range config.Routes {
		if strings.EqualFold(config.Routes[i].Name, route.Name) {
			config.Routes[i] = route
			replaced = true
			break
		}
	}
	if !replaced {
		config.Routes = append(config.Routes, route)
	}
	rs.mu.Unlock()
	return rs.SaveRouterConfig(config)
}

func (rs *RouterService) SetAppRouting(provider string, enabled bool) error {
	p, ok := knownProvider(provider)
	if !ok {
		return fmt.Errorf("未知模型商: %s", provider)
	}
	rs.mu.Lock()
	if rs.config.AppRouting == nil {
		rs.config.AppRouting = defaultAppRouting()
	}
	rs.config.AppRouting[p] = enabled
	cfg := rs.config
	rs.mu.Unlock()
	return rs.SaveRouterConfig(cfg)
}

func (a *App) currentEnvNameForProvider(provider string) string {
	switch provider {
	case "codex":
		return a.config.CurrentEnvCodex
	case "gemini":
		return a.config.CurrentEnvGemini
	case "opencode":
		return a.config.CurrentEnvOpencode
	case "grok":
		return a.config.CurrentEnvGrok
	default:
		return a.config.CurrentEnvClaude
	}
}

func (a *App) GetProviderRouting() map[string]bool {
	out := defaultAppRouting()
	rs := globalRouterService
	if rs == nil {
		return out
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for key, enabled := range rs.config.AppRouting {
		if p, ok := knownProvider(key); ok {
			out[p] = enabled
		}
	}
	return out
}

func (a *App) SetProviderRouting(provider string, enabled bool) error {
	p, ok := knownProvider(provider)
	if !ok {
		return fmt.Errorf("未知模型商: %s", provider)
	}
	rs := globalRouterService
	if rs == nil {
		return fmt.Errorf("路由服务未初始化")
	}
	if err := rs.SetAppRouting(p, enabled); err != nil {
		return err
	}
	if enabled {
		rs.mu.Lock()
		running := rs.running
		rs.mu.Unlock()
		if !running {
			if err := rs.StartGateway(); err != nil {
				return fmt.Errorf("启动路由网关失败: %v", err)
			}
		}
	}
	env := a.findEnv(a.currentEnvNameForProvider(p))
	if env == nil {
		if !enabled {
			_ = removeAutoRouteByName(p)
		}
		return nil
	}
	if _, err := a.applyEnvByProvider(env); err != nil {
		_ = rs.SetAppRouting(p, !enabled)
		return err
	}
	return nil
}

func (a *App) RefreshRoutedProviders() error {
	var errs []string
	for _, provider := range []string{"claude", "codex", "gemini", "opencode", "grok"} {
		if !isAppRoutingOn(provider) {
			continue
		}
		env := a.findEnv(a.currentEnvNameForProvider(provider))
		if env == nil {
			continue
		}
		if _, err := a.applyEnvByProvider(env); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", provider, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "；"))
	}
	return nil
}
