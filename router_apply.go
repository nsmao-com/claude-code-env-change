package main

import (
	"fmt"
	"net/url"
	"strings"
)

// 上游格式取值：
//   ""                     —— 原生直连（默认，不走路由）
//   "chat_completions"     —— OpenAI Chat Completions（需开启路由）
//   "anthropic_messages"   —— Anthropic Messages（需开启路由）
//   "responses"            —— OpenAI Responses（需开启路由）
const (
	UpstreamChatCompletions   = "chat_completions"
	UpstreamAnthropicMessages = "anthropic_messages"
	UpstreamResponses         = "responses"
)

// normalizeUpstreamFormat 归一化上游格式；不认识的值按原生直连处理
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

// upstreamFormatLabel 供错误提示使用
func upstreamFormatLabel(format string) string {
	switch format {
	case UpstreamChatCompletions:
		return "Chat Completions"
	case UpstreamAnthropicMessages:
		return "Anthropic Messages"
	case UpstreamResponses:
		return "Responses"
	}
	return ""
}

// sanitizeAutoRouteName 由配置名生成合法路由名（满足 routeNamePattern）
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

// upstreamVarsForEnv 取真实上游地址与密钥
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
	}
	return baseURL, apiKey, model
}

// needsRouting 判断该配置是否需要本地路由做协议转换
func needsRouting(env *EnvConfig) bool {
	format := normalizeUpstreamFormat(env.UpstreamFormat)
	if format == "" {
		return false
	}
	switch env.Provider {
	case "claude":
		// Claude Code 原生说 Anthropic Messages：只有 Chat Completions 上游需要转换
		// （Anthropic Messages 上游 = 直连；Responses 上游的转换暂不支持，前端不提供该选项）
		return format == UpstreamChatCompletions
	case "codex":
		// Codex 原生说 Responses：Chat Completions / Anthropic Messages 上游都需要转换
		return format == UpstreamChatCompletions || format == UpstreamAnthropicMessages
	}
	return false
}

// routerLocalBase 返回写入 CLI 配置的本地路由地址，如 http://127.0.0.1:8790/env-myconfig
func routerLocalBase(rs *RouterService, env *EnvConfig) string {
	rs.mu.Lock()
	port := rs.config.Port
	rs.mu.Unlock()
	if port <= 0 {
		port = defaultRouterPort
	}
	return fmt.Sprintf("http://127.0.0.1:%d/%s", port, sanitizeAutoRouteName(env.Name))
}

// wireRouterForEnv 按 cc-switch 模型为配置自动建路由并确保网关在运行。
// 返回应写入 CLI 的本地路由 base（已含 /v1 需求除外）。
func wireRouterForEnv(env *EnvConfig) (string, error) {
	rs := globalRouterService
	if rs == nil {
		return "", fmt.Errorf("路由服务未初始化")
	}

	format := normalizeUpstreamFormat(env.UpstreamFormat)
	baseURL, apiKey, model := upstreamVarsForEnv(env)
	if strings.TrimSpace(baseURL) == "" {
		return "", fmt.Errorf("此配置未填写上游 Base URL，无法开启路由转换")
	}
	if _, err := url.Parse(baseURL); err != nil {
		return "", fmt.Errorf("上游 Base URL 无效: %v", err)
	}

	// 目标协议：路由把请求转成上游说的协议
	target := "openai"
	if format == UpstreamAnthropicMessages {
		target = "anthropic"
	}

	route := APIRoute{
		Name:         sanitizeAutoRouteName(env.Name),
		Description:  fmt.Sprintf("配置 %q 的自动路由（上游格式: %s）", env.Name, upstreamFormatLabel(format)),
		SourceFormat: "anthropic",
		TargetFormat: target,
		BaseURL:      strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		APIKey:       apiKey,
		DefaultModel: model,
		Enabled:      true,
	}
	if env.Provider == "codex" {
		route.SourceFormat = "openai"
	}

	if err := rs.upsertAutoRoute(route); err != nil {
		return "", fmt.Errorf("写入路由配置失败: %v", err)
	}

	// 网关没在运行时自动拉起；失败则按 cc-switch 的口径提示
	rs.mu.Lock()
	running := rs.running
	rs.mu.Unlock()
	if !running {
		if err := rs.StartGateway(); err != nil {
			return "", fmt.Errorf("此供应商使用 %s 接口格式，需要路由服务才能正常工作，请先启动路由（%v）", upstreamFormatLabel(format), err)
		}
	}

	return routerLocalBase(rs, env), nil
}

// upsertAutoRoute 按名称替换/追加自动路由并落盘（网关运行中会热重启生效）
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
