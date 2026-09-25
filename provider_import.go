package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type UniversalProvider struct {
	Name      string   `json:"name"`
	BaseURL   string   `json:"base_url"`
	APIKey    string   `json:"api_key"`
	Model     string   `json:"model"`
	Format    string   `json:"format"`
	Providers []string `json:"providers"`
}

func universalEnvironment(p UniversalProvider, provider string) EnvConfig {
	e := EnvConfig{Name: p.Name, Provider: provider, Description: "通用供应商 · " + p.Name, Variables: map[string]string{}, UniversalID: p.Name, UpstreamFormat: normalizeUpstreamFormat(p.Format)}
	switch provider {
	case "claude", "claude_desktop":
		e.Variables["ANTHROPIC_BASE_URL"] = p.BaseURL
		e.Variables["ANTHROPIC_AUTH_TOKEN"] = p.APIKey
	case "codex":
		e.Variables["base_url"] = p.BaseURL
		e.Variables["OPENAI_API_KEY"] = p.APIKey
		e.Variables["AI_ENV_AUTH_MODE"] = "mixed"
	case "antigravity":
		e.Variables["GOOGLE_GEMINI_BASE_URL"] = p.BaseURL
		e.Variables["GEMINI_API_KEY"] = p.APIKey
	case "opencode":
		e.Variables["OPENCODE_BASE_URL"] = p.BaseURL
		e.Variables["OPENCODE_API_KEY"] = p.APIKey
	case "grok":
		e.Variables["XAI_BASE_URL"] = p.BaseURL
		e.Variables["XAI_API_KEY"] = p.APIKey
	}
	setEnvironmentModel(&e, p.Model)
	return e
}
func (w *WorkbenchService) SaveUniversalProvider(p UniversalProvider) (int, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len(p.Name) > 100 || strings.TrimSpace(p.APIKey) == "" {
		return 0, fmt.Errorf("请填写供应商名称与 API Key")
	}
	if e := validEndpoint(p.BaseURL); e != nil {
		return 0, e
	}
	if len(p.Providers) == 0 {
		return 0, fmt.Errorf("请至少选择一个目标工具")
	}
	if normalizeUpstreamFormat(p.Format) == "" {
		return 0, fmt.Errorf("请选择上游协议")
	}
	seen := map[string]bool{}
	envs := []EnvConfig{}
	for _, provider := range p.Providers {
		if _, ok := knownProvider(provider); !ok && provider != "claude_desktop" {
			return 0, fmt.Errorf("不支持的平台 %s", provider)
		}
		if seen[provider] {
			continue
		}
		seen[provider] = true
		envs = append(envs, universalEnvironment(p, provider))
	}
	w.app.configMu.Lock()
	defer w.app.configMu.Unlock()
	original := append([]EnvConfig(nil), w.app.config.Environments...)
	for _, e := range envs {
		for _, old := range original {
			if sameProvider(old.Provider, e.Provider) && old.Name == e.Name && old.UniversalID != p.Name {
				return 0, fmt.Errorf("%s 下已有同名独立配置，请更换名称", e.Provider)
			}
		}
	}
	for _, e := range envs {
		found := false
		for i, old := range w.app.config.Environments {
			if sameProvider(old.Provider, e.Provider) && old.Name == e.Name {
				if old.UniversalID != p.Name {
					return 0, fmt.Errorf("%s 下已有同名独立配置，请更换名称", e.Provider)
				}
				w.app.config.Environments[i] = e
				found = true
				break
			}
		}
		if !found {
			w.app.config.Environments = append(w.app.config.Environments, e)
		}
	}
	if e := w.app.saveConfig(); e != nil {
		w.app.config.Environments = original
		return 0, e
	}
	return len(envs), nil
}
func importProviderName(raw string) string {
	switch strings.ToLower(raw) {
	case "claude-code", "claudecode":
		return "claude"
	case "gemini":
		return "antigravity"
	case "claude-desktop":
		return "claude_desktop"
	}
	return strings.ToLower(raw)
}
func parseCCProvider(provider string, raw map[string]any) (EnvConfig, error) {
	provider = importProviderName(provider)
	if _, ok := knownProvider(provider); !ok && provider != "claude_desktop" {
		return EnvConfig{}, fmt.Errorf("不支持的工具 %s", provider)
	}
	name := strings.TrimSpace(asString(raw["name"]))
	if name == "" {
		name = "导入供应商"
	}
	env := EnvConfig{Name: name, Provider: provider, Variables: map[string]string{}, Description: "从 CC Switch 导入"}
	cfg, _ := raw["settingsConfig"].(map[string]any)
	if cfg == nil {
		cfg, _ = raw["settings_config"].(map[string]any)
	}
	if cfg == nil {
		_ = json.Unmarshal([]byte(asString(raw["settings_config"])), &cfg)
	}
	if cfg == nil {
		cfg = raw
	}
	vars, _ := cfg["env"].(map[string]any)
	for k, v := range vars {
		if text, ok := v.(string); ok {
			env.Variables[k] = text
		}
	}
	if provider == "codex" {
		configText := asString(cfg["config"])
		var parsed map[string]any
		if configText != "" {
			if e := toml.Unmarshal([]byte(configText), &parsed); e != nil {
				return env, fmt.Errorf("%s 的 Codex TOML 无效", name)
			}
		}
		env.Variables["model"] = asString(parsed["model"])
		providers, _ := parsed["model_providers"].(map[string]any)
		active, _ := providers[asString(parsed["model_provider"])].(map[string]any)
		env.Variables["base_url"] = asString(active["base_url"])
		env.Variables["OPENAI_API_KEY"] = asString(active["experimental_bearer_token"])
		auth, _ := cfg["auth"].(map[string]any)
		if auth == nil {
			_ = json.Unmarshal([]byte(asString(cfg["auth"])), &auth)
		}
		if env.Variables["OPENAI_API_KEY"] == "" {
			env.Variables["OPENAI_API_KEY"] = asString(auth["OPENAI_API_KEY"])
		}
		env.Variables["AI_ENV_AUTH_MODE"] = "mixed"
		env.Templates = map[string]string{}
		if configText != "" {
			env.Templates["config.toml"] = configText
		}
	}
	if len(env.Variables) == 0 {
		return env, fmt.Errorf("%s 没有可识别的配置字段", name)
	}
	return env, nil
}
func (w *WorkbenchService) PreviewExternalImport(text string) ([]EnvConfig, error) {
	text = strings.TrimSpace(strings.TrimPrefix(text, "\ufeff"))
	if len(text) > 8<<20 {
		return nil, fmt.Errorf("导入内容不可超过 8 MB")
	}
	if strings.HasPrefix(strings.ToLower(text), "ccswitch://") {
		u, e := url.Parse(text)
		if e != nil {
			return nil, e
		}
		q := u.Query()
		if u.Host != "v1" || u.Path != "/import" {
			return nil, fmt.Errorf("不支持的 CC Switch 分享链接版本")
		}
		if q.Get("configUrl") != "" {
			return nil, fmt.Errorf("请下载配置文件后导入；不自动访问分享链接中的远程配置")
		}
		if raw := q.Get("config"); raw != "" {
			decoded, e := base64.StdEncoding.DecodeString(raw)
			if e != nil {
				decoded, e = base64.RawURLEncoding.DecodeString(raw)
			}
			if e != nil {
				return nil, fmt.Errorf("分享链接配置编码无效")
			}
			var obj map[string]any
			if q.Get("configFormat") == "toml" && importProviderName(q.Get("app")) == "codex" {
				obj = map[string]any{"config": string(decoded), "auth": map[string]any{"OPENAI_API_KEY": q.Get("apiKey")}}
			} else if json.Unmarshal(decoded, &obj) != nil {
				return nil, fmt.Errorf("分享链接配置格式无效")
			}
			if q.Get("name") != "" {
				obj["name"] = q.Get("name")
			}
			env, e := parseCCProvider(q.Get("app"), obj)
			return []EnvConfig{env}, e
		}
		if kind := q.Get("resource"); kind != "" && kind != "provider" {
			return nil, fmt.Errorf("此入口仅导入供应商链接，请在 MCP / Skills 页导入工具")
		}
		p := UniversalProvider{Name: q.Get("name"), BaseURL: firstNonEmpty(q.Get("endpoint"), q.Get("baseUrl")), APIKey: firstNonEmpty(q.Get("apiKey"), q.Get("api_key")), Model: q.Get("model")}
		if p.Name == "" || p.BaseURL == "" {
			return nil, fmt.Errorf("分享链接缺少名称或地址")
		}
		provider := importProviderName(q.Get("app"))
		if _, ok := knownProvider(provider); !ok && provider != "claude_desktop" {
			return nil, fmt.Errorf("分享链接工具不受支持")
		}
		out := []EnvConfig{}
		for i, endpoint := range strings.Split(p.BaseURL, ",") {
			copy := p
			copy.BaseURL = strings.TrimSpace(endpoint)
			if e = validEndpoint(copy.BaseURL); e != nil {
				return nil, e
			}
			if i > 0 {
				copy.Name = fmt.Sprintf("%s (%d)", p.Name, i+1)
			}
			env := universalEnvironment(copy, provider)
			if provider == "claude" || provider == "claude_desktop" {
				for field, key := range map[string]string{"haikuModel": "ANTHROPIC_DEFAULT_HAIKU_MODEL", "sonnetModel": "ANTHROPIC_DEFAULT_SONNET_MODEL", "opusModel": "ANTHROPIC_DEFAULT_OPUS_MODEL"} {
					if v := q.Get(field); v != "" {
						env.Variables[key] = v
					}
				}
			}
			out = append(out, env)
		}
		return out, nil
	}
	var root map[string]any
	if e := json.Unmarshal([]byte(text), &root); e != nil {
		return nil, fmt.Errorf("请选择 JSON 导出文件或粘贴 CC Switch 分享链接")
	}
	if _, ok := root["environments"]; ok {
		var cfg Config
		if e := json.Unmarshal([]byte(text), &cfg); e != nil {
			return nil, e
		}
		return cfg.Environments, nil
	}
	out := []EnvConfig{}
	add := func(provider string, value any) error {
		m, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		env, e := parseCCProvider(provider, m)
		if e != nil {
			return e
		}
		out = append(out, env)
		return nil
	}
	if rows, ok := root["providers"].([]any); ok {
		for _, row := range rows {
			m, ok := row.(map[string]any)
			if !ok {
				continue
			}
			provider := firstNonEmpty(asString(m["app_type"]), asString(m["appType"]))
			if e := add(provider, m); e != nil {
				return nil, e
			}
		}
	} else {
		for _, provider := range []string{"claude", "claude_desktop", "codex", "gemini", "antigravity", "opencode", "grok"} {
			group, _ := root[provider].(map[string]any)
			if group == nil {
				continue
			}
			providers, _ := group["providers"].(map[string]any)
			for _, raw := range providers {
				if e := add(provider, raw); e != nil {
					return nil, e
				}
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("没有找到支持的供应商。请使用 CC Switch JSON 导出或分享链接")
	}
	return out, nil
}
func (w *WorkbenchService) ReadExternalImport(file string) (string, error) {
	info, e := os.Stat(file)
	if e != nil {
		return "", e
	}
	if info.IsDir() || info.Size() > 8<<20 {
		return "", fmt.Errorf("请选择小于 8 MB 的配置文件")
	}
	b, e := os.ReadFile(file)
	return string(b), e
}
func (w *WorkbenchService) CommitExternalImport(items []EnvConfig) (int, error) {
	if len(items) == 0 || len(items) > 500 {
		return 0, fmt.Errorf("请选择 1～500 个环境")
	}
	for _, e := range items {
		if strings.TrimSpace(e.Name) == "" {
			return 0, fmt.Errorf("环境名称不能为空")
		}
		if _, ok := knownProvider(e.Provider); !ok && e.Provider != "claude_desktop" {
			return 0, fmt.Errorf("不支持的工具")
		}
	}
	w.app.configMu.Lock()
	defer w.app.configMu.Unlock()
	original := append([]EnvConfig(nil), w.app.config.Environments...)
	for _, e := range items {
		base := e.Name
		for i := 1; w.app.findEnvIn(e.Provider, e.Name) != nil; i++ {
			e.Name = fmt.Sprintf("%s (导入 %d)", base, i)
		}
		e.UniversalID = ""
		w.app.config.Environments = append(w.app.config.Environments, e)
	}
	if e := w.app.saveConfig(); e != nil {
		w.app.config.Environments = original
		return 0, e
	}
	return len(items), nil
}
