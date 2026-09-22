package main

// 供应商预设：参考同类切换工具（cc-switch 等）的一键创建体验，
// 内置常见第三方供应商的官方 Anthropic 兼容端点，避免用户手抄 URL。
// API Key 一律留空由用户填写，不内置任何占位密钥。

// ProviderPreset 一个可一键填充的供应商模板
type ProviderPreset struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Provider     string            `json:"provider"` // 目标平台：claude / codex / ...
	Icon         string            `json:"icon"`
	Description  string            `json:"description"`
	Variables    map[string]string `json:"variables"`
	ModelChoices []string          `json:"model_choices,omitempty"`
	Website      string            `json:"website,omitempty"`
}

var providerPresets = []ProviderPreset{
	{
		ID:          "zhipu-glm",
		Name:        "智谱 GLM",
		Provider:    "claude",
		Icon:        "🧠",
		Website:     "https://open.bigmodel.cn",
		Description: "智谱 AI 开放平台，Anthropic 兼容端点，支持 GLM-4.6 等模型",
		Variables: map[string]string{
			"ANTHROPIC_BASE_URL": "https://open.bigmodel.cn/api/anthropic",
			"ANTHROPIC_MODEL":    "glm-4.6",
		},
		ModelChoices: []string{"glm-4.6", "glm-4.5", "glm-4.5-air"},
	},
	{
		ID:          "deepseek",
		Name:        "DeepSeek",
		Provider:    "claude",
		Icon:        "🐋",
		Website:     "https://platform.deepseek.com",
		Description: "DeepSeek 开放平台，Anthropic 兼容端点，支持 deepseek-chat / deepseek-reasoner",
		Variables: map[string]string{
			"ANTHROPIC_BASE_URL": "https://api.deepseek.com/anthropic",
			"ANTHROPIC_MODEL":    "deepseek-chat",
		},
		ModelChoices: []string{"deepseek-chat", "deepseek-reasoner"},
	},
	{
		ID:          "moonshot-kimi",
		Name:        "Kimi (Moonshot)",
		Provider:    "claude",
		Icon:        "🌙",
		Website:     "https://platform.moonshot.cn",
		Description: "月之暗面 Kimi 开放平台，Anthropic 兼容端点，K2 系列模型",
		Variables: map[string]string{
			"ANTHROPIC_BASE_URL": "https://api.moonshot.cn/anthropic",
			"ANTHROPIC_MODEL":    "kimi-k2-0905-preview",
		},
		ModelChoices: []string{"kimi-k2-0905-preview", "kimi-k2-turbo-preview"},
	},
	{
		ID:          "qwen-dashscope",
		Name:        "通义千问 Qwen",
		Provider:    "claude",
		Icon:        "☁️",
		Website:     "https://bailian.console.aliyun.com",
		Description: "阿里云百炼 Claude Code 代理端点，qwen3-coder 系列模型",
		Variables: map[string]string{
			"ANTHROPIC_BASE_URL": "https://dashscope.aliyuncs.com/api/v2/apps/claude-code-proxy",
			"ANTHROPIC_MODEL":    "qwen3-coder-plus",
		},
		ModelChoices: []string{"qwen3-coder-plus", "qwen3-coder-flash"},
	},
	{
		ID:          "packycode",
		Name:        "自定义中转",
		Provider:    "claude",
		Icon:        "🔗",
		Description: "通用的第三方中转/自建网关模板：填入中转站地址与 Key 即可",
		Variables: map[string]string{
			"ANTHROPIC_BASE_URL": "",
			"ANTHROPIC_MODEL":    "",
		},
	},
}

// GetProviderPresets 返回内置供应商预设目录（供前端"从模板新建"使用）
func (a *App) GetProviderPresets() []ProviderPreset {
	out := make([]ProviderPreset, len(providerPresets))
	copy(out, providerPresets)
	return out
}
