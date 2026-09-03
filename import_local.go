package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// ImportLocalEnv 从本机 CLI 配置文件读取当前设置，并为指定平台新增一条环境配置。
// provider 为 claude/codex/gemini/opencode/grok；空或 all 则尝试全部平台。
func (a *App) ImportLocalEnv(provider string) ([]EnvConfig, error) {
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "" {
		p = "all"
	}

	targets := []string{p}
	if p == "all" {
		targets = []string{"claude", "codex", "antigravity", "opencode", "grok"}
	}

	added := make([]EnvConfig, 0, len(targets))
	var errs []string
	for _, item := range targets {
		envs, err := a.buildLocalEnvs(item)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		for _, env := range envs {
			env := env
			if err := a.AddEnv(env); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", item, err))
				continue
			}
			added = append(added, env)
		}
	}

	if len(added) == 0 {
		if len(errs) > 0 {
			return nil, fmt.Errorf("本机没有可导入的配置：%s", strings.Join(errs, "；"))
		}
		return nil, fmt.Errorf("本机没有可导入的配置")
	}
	return added, nil
}

func (a *App) buildLocalEnv(provider string) (*EnvConfig, error) {
	envs, err := a.buildLocalEnvs(provider)
	if err != nil {
		return nil, err
	}
	if len(envs) == 0 {
		return nil, nil
	}
	return &envs[0], nil
}

func (a *App) buildLocalEnvs(provider string) ([]EnvConfig, error) {
	switch provider {
	case "claude":
		env, err := a.buildLocalClaudeEnv()
		if err != nil || env == nil {
			return nil, err
		}
		return []EnvConfig{*env}, nil
	case "codex":
		env, err := a.buildLocalCodexEnv()
		if err != nil || env == nil {
			return nil, err
		}
		return []EnvConfig{*env}, nil
	case "antigravity":
		env, err := a.buildLocalAntigravityEnv()
		if err != nil || env == nil {
			return nil, err
		}
		return []EnvConfig{*env}, nil
	case "opencode":
		return a.buildLocalOpencodeEnvs()
	case "grok":
		env, err := a.buildLocalGrokEnv()
		if err != nil || env == nil {
			return nil, err
		}
		return []EnvConfig{*env}, nil
	default:
		return nil, fmt.Errorf("未知平台 %s", provider)
	}
}

func (a *App) uniqueEnvName(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "本机配置"
	}
	if a.findEnv(base) == nil {
		return base
	}
	for i := 2; i < 1000; i++ {
		name := fmt.Sprintf("%s %d", base, i)
		if a.findEnv(name) == nil {
			return name
		}
	}
	return fmt.Sprintf("%s %d", base, len(a.config.Environments)+1)
}

func (a *App) buildLocalClaudeEnv() (*EnvConfig, error) {
	vars := a.GetClaudeSettings()
	if len(vars) == 0 {
		return nil, nil
	}
	if v := vars["ANTHROPIC_BASE_URL"]; v != "" {
		vars["ANTHROPIC_BASE_URL"] = resolveImportedBaseURL(v)
	}
	if !hasAnyValue(vars, "ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY", "ANTHROPIC_MODEL") {
		return nil, nil
	}
	return &EnvConfig{
		Name:                       a.uniqueEnvName("本机 Claude"),
		Description:                "从本机 ~/.claude/settings.json 导入",
		Provider:                   "claude",
		Variables:                  vars,
		Icon:                       "💻",
		AttributionHeader:          vars["CLAUDE_CODE_ATTRIBUTION_HEADER"],
		DisableNonessentialTraffic: vars["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"],
	}, nil
}

func (a *App) buildLocalCodexEnv() (*EnvConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	codexDir := filepath.Join(home, ".codex")
	settings := a.GetCodexSettings()
	if v := settings["base_url"]; v != "" {
		settings["base_url"] = resolveImportedBaseURL(v)
	}

	variables := map[string]string{}
	copyIfSet(variables, settings, "base_url", "OPENAI_API_KEY", "model",
		"model_context_window", "model_max_output_tokens", "model_reasoning_effort",
		"model_reasoning_summary", "plan_mode_reasoning_effort", "approval_policy", "sandbox_mode", "model_verbosity")

	templates := map[string]string{}
	if data, err := os.ReadFile(filepath.Join(codexDir, "config.toml")); err == nil && len(data) > 0 {
		tmpl := string(data)
		if orig := strings.TrimSpace(settings["base_url"]); orig != "" {
			tmpl = strings.ReplaceAll(tmpl, orig, "{{base_url}}")
		}
		if orig := strings.TrimSpace(settings["model"]); orig != "" {
			tmpl = replaceTOMLQuoted(tmpl, "model", orig, "{{model}}")
		}
		templates["config.toml"] = tmpl
	}
	if data, err := os.ReadFile(filepath.Join(codexDir, "auth.json")); err == nil && len(data) > 0 {
		tmpl := string(data)
		if key := strings.TrimSpace(settings["OPENAI_API_KEY"]); key != "" {
			tmpl = strings.ReplaceAll(tmpl, key, "{{OPENAI_API_KEY}}")
		}
		templates["auth.json"] = tmpl
	}

	if !hasAnyValue(variables, "base_url", "OPENAI_API_KEY", "model") && len(templates) == 0 {
		return nil, nil
	}
	return &EnvConfig{
		Name:        a.uniqueEnvName("本机 Codex"),
		Description: "从本机 ~/.codex 导入",
		Provider:    "codex",
		Variables:   variables,
		Templates:   templates,
		Icon:        "💻",
	}, nil
}

func (a *App) buildLocalAntigravityEnv() (*EnvConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	geminiDir := filepath.Join(home, ".gemini")
	settings := a.GetAntigravitySettings()
	if v := settings["GOOGLE_GEMINI_BASE_URL"]; v != "" {
		settings["GOOGLE_GEMINI_BASE_URL"] = resolveImportedBaseURL(v)
	}

	variables := map[string]string{}
	copyIfSet(variables, settings,
		"GOOGLE_GEMINI_BASE_URL", "GEMINI_API_KEY", "GEMINI_MODEL", "GOOGLE_API_KEY",
		"GOOGLE_CLOUD_PROJECT", "GOOGLE_CLOUD_LOCATION", "GOOGLE_GENAI_USE_VERTEXAI",
		"GEMINI_SANDBOX")

	templates := map[string]string{}
	if data, err := os.ReadFile(filepath.Join(geminiDir, ".env")); err == nil && len(data) > 0 {
		tmpl := string(data)
		for _, key := range []string{"GOOGLE_GEMINI_BASE_URL", "GEMINI_API_KEY", "GEMINI_MODEL"} {
			if v := strings.TrimSpace(variables[key]); v != "" {
				tmpl = strings.ReplaceAll(tmpl, v, "{{"+key+"}}")
			}
		}
		templates[".env"] = tmpl
	}
	// 优先读取 agy 的新位置 settings.json，回退到旧位置
	settingsFile := filepath.Join(geminiDir, "settings.json")
	if _, err := os.Stat(filepath.Join(geminiDir, "antigravity-cli", "settings.json")); err == nil {
		settingsFile = filepath.Join(geminiDir, "antigravity-cli", "settings.json")
	}
	if data, err := os.ReadFile(settingsFile); err == nil && len(data) > 0 {
		templates["settings.json"] = string(data)
		var payload map[string]any
		if json.Unmarshal(data, &payload) == nil {
			if n, ok := asInt(payload["maxSessionTurns"]); ok {
				variables["GEMINI_MAX_SESSION_TURNS"] = strconv.Itoa(n)
			}
			if cc, ok := payload["chatCompression"].(map[string]any); ok {
				if v, ok := asFloat(cc["contextPercentageThreshold"]); ok {
					variables["GEMINI_COMPRESSION_THRESHOLD"] = strconv.FormatFloat(v, 'f', -1, 64)
				}
			}
		}
	}

	if !hasAnyValue(variables, "GOOGLE_GEMINI_BASE_URL", "GEMINI_API_KEY", "GEMINI_MODEL") && len(templates) == 0 {
		return nil, nil
	}
	return &EnvConfig{
		Name:        a.uniqueEnvName("本机 Antigravity"),
		Description: "从本机 ~/.gemini (Antigravity CLI) 导入",
		Provider:    "antigravity",
		Variables:   variables,
		Templates:   templates,
		Icon:        "💻",
	}, nil
}

func (a *App) buildLocalOpencodeEnv() (*EnvConfig, error) {
	envs, err := a.buildLocalOpencodeEnvs()
	if err != nil || len(envs) == 0 {
		return nil, err
	}
	return &envs[0], nil
}

func (a *App) buildLocalOpencodeEnvs() ([]EnvConfig, error) {
	configFile := opencodeConfigFile(nil)
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	payload, err := parseJSONLikeObject(data)
	if err != nil {
		return nil, fmt.Errorf("解析 OpenCode 配置失败: %v", err)
	}

	providers := opencodeProviderMap(payload)
	authKeys := loadOpencodeAuthKeys()
	defaultModel := ""
	if v, ok := payload["model"].(string); ok {
		defaultModel = strings.TrimSpace(v)
	}

	globals := map[string]string{
		"OPENCODE_CONFIG_DIR": resolveOpencodeConfigDir(nil),
		"OPENCODE_CONFIG":     configFile,
	}
	if v, ok := payload["small_model"].(string); ok {
		globals["OPENCODE_SMALL_MODEL"] = strings.TrimSpace(v)
	}
	if v, ok := payload["username"].(string); ok {
		globals["OPENCODE_USERNAME"] = strings.TrimSpace(v)
	}
	if v, ok := payload["share"].(string); ok {
		globals["OPENCODE_SHARE"] = strings.TrimSpace(v)
	}
	if v, ok := payload["autoupdate"].(bool); ok {
		globals["OPENCODE_AUTOUPDATE"] = bool01(v)
	}
	if v, ok := payload["snapshot"].(bool); ok {
		globals["OPENCODE_SNAPSHOT"] = bool01(v)
	}

	var out []EnvConfig
	if len(providers) > 0 {
		ids := make([]string, 0, len(providers))
		for id := range providers {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		matchedDefault := false
		for _, id := range ids {
			raw, _ := providers[id].(map[string]any)
			env := a.envFromOpencodeProvider(id, raw, authKeys[id], defaultModel, globals)
			if env == nil {
				continue
			}
			if defaultModel != "" && strings.HasPrefix(defaultModel, id+"/") {
				matchedDefault = true
				copyIfSet(env.Variables, globals, "OPENCODE_SMALL_MODEL", "OPENCODE_USERNAME", "OPENCODE_SHARE", "OPENCODE_AUTOUPDATE", "OPENCODE_SNAPSHOT")
			}
			out = append(out, *env)
		}
		if !matchedDefault && len(out) > 0 {
			copyIfSet(out[0].Variables, globals, "OPENCODE_SMALL_MODEL", "OPENCODE_USERNAME", "OPENCODE_SHARE", "OPENCODE_AUTOUPDATE", "OPENCODE_SNAPSHOT")
		}
	}

	if len(out) == 0 && (defaultModel != "" || len(authKeys) > 0) {
		variables := map[string]string{}
		copyIfSet(variables, globals, "OPENCODE_CONFIG_DIR", "OPENCODE_CONFIG", "OPENCODE_SMALL_MODEL", "OPENCODE_USERNAME", "OPENCODE_SHARE", "OPENCODE_AUTOUPDATE", "OPENCODE_SNAPSHOT")
		variables["OPENCODE_MODEL"] = defaultModel
		if key := firstAuthKey(authKeys); key != "" {
			variables["OPENCODE_API_KEY"] = key
		}
		out = append(out, EnvConfig{
			Name:        a.uniqueEnvName("本机 OpenCode"),
			Description: "从本机 OpenCode 配置导入",
			Provider:    "opencode",
			Variables:   variables,
			Icon:        "💻",
		})
	}
	return out, nil
}

func (a *App) envFromOpencodeProvider(id string, raw map[string]any, authKey, defaultModel string, globals map[string]string) *EnvConfig {
	if raw == nil {
		raw = map[string]any{}
	}
	display := strings.TrimSpace(asString(raw["name"]))
	if display == "" {
		display = id
	}
	npmPkg := strings.TrimSpace(asString(raw["npm"]))
	options, _ := raw["options"].(map[string]any)
	baseURL := ""
	apiKey := ""
	if options != nil {
		baseURL = resolveImportedBaseURL(asString(options["baseURL"]))
		apiKey = strings.TrimSpace(asString(options["apiKey"]))
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(authKey)
	}
	modelIDs := opencodeProviderModelIDs(raw["models"])
	model := ""
	if defaultModel != "" && (strings.HasPrefix(defaultModel, id+"/") || strings.HasPrefix(defaultModel, id+"\\")) {
		model = defaultModel
	} else if len(modelIDs) > 0 {
		model = id + "/" + modelIDs[0]
	}

	if baseURL == "" && apiKey == "" && model == "" && len(modelIDs) == 0 {
		return nil
	}

	variables := map[string]string{
		"OPENCODE_PROVIDER_ID":   id,
		"OPENCODE_PROVIDER_NAME": display,
	}
	copyIfSet(variables, globals, "OPENCODE_CONFIG_DIR", "OPENCODE_CONFIG")
	if baseURL != "" {
		variables["OPENCODE_BASE_URL"] = baseURL
	}
	if apiKey != "" {
		variables["OPENCODE_API_KEY"] = apiKey
	}
	if model != "" {
		variables["OPENCODE_MODEL"] = model
	}
	if npmPkg != "" {
		variables["OPENCODE_NPM"] = npmPkg
	}
	if len(modelIDs) > 0 {
		variables["OPENCODE_MODELS"] = strings.Join(modelIDs, ",")
	}
	return &EnvConfig{
		Name:        a.uniqueEnvName("本机 OpenCode · " + display),
		Description: "从本机 OpenCode provider " + id + " 导入",
		Provider:    "opencode",
		Variables:   variables,
		Icon:        "💻",
	}
}

func opencodeProviderMap(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	if p, ok := payload["provider"].(map[string]any); ok && len(p) > 0 {
		return p
	}
	if p, ok := payload["providers"].(map[string]any); ok && len(p) > 0 {
		return p
	}
	return nil
}

func opencodeProviderModelIDs(raw any) []string {
	models, ok := raw.(map[string]any)
	if !ok || len(models) == 0 {
		return nil
	}
	ids := make([]string, 0, len(models))
	for id := range models {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func opencodeAuthFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "opencode", "auth.json")
}

func loadOpencodeAuthKeys() map[string]string {
	out := map[string]string{}
	data, err := os.ReadFile(opencodeAuthFile())
	if err != nil || len(data) == 0 {
		return out
	}
	payload, err := parseJSONLikeObject(data)
	if err != nil {
		return out
	}
	for id, raw := range payload {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		key := strings.TrimSpace(asString(entry["key"]))
		if key == "" {
			key = strings.TrimSpace(asString(entry["apiKey"]))
		}
		if key != "" {
			out[id] = key
		}
	}
	return out
}

func firstAuthKey(keys map[string]string) string {
	ids := make([]string, 0, len(keys))
	for id := range keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if keys[id] != "" {
			return keys[id]
		}
	}
	return ""
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return ""
	}
}

func (a *App) buildLocalGrokEnv() (*EnvConfig, error) {
	settings := a.GetGrokSettings()
	if v := settings["XAI_BASE_URL"]; v != "" {
		settings["XAI_BASE_URL"] = resolveImportedBaseURL(v)
	}
	variables := map[string]string{}
	copyIfSet(variables, settings,
		"XAI_BASE_URL", "XAI_API_KEY", "XAI_MODEL", "XAI_API_BACKEND",
		"GROK_HOME", "XAI_MODEL_NAME", "XAI_CONTEXT_WINDOW", "XAI_MAX_TOKENS", "XAI_TEMPERATURE", "XAI_REASONING_EFFORT")

	templates := map[string]string{}
	configFile := settings["GROK_CONFIG_PATH"]
	if configFile == "" {
		configFile = grokConfigFile(nil)
	}
	if data, err := os.ReadFile(configFile); err == nil && len(data) > 0 {
		tmpl := string(data)
		for _, key := range []string{"XAI_BASE_URL", "XAI_API_KEY", "XAI_MODEL"} {
			if v := strings.TrimSpace(variables[key]); v != "" {
				tmpl = strings.ReplaceAll(tmpl, v, "{{"+key+"}}")
			}
		}
		templates["config.toml"] = tmpl
		payload := map[string]any{}
		if toml.Unmarshal(data, &payload) == nil {
			if modelMap, ok := payload["model"].(map[string]any); ok {
				if custom, ok := modelMap["custom"].(map[string]any); ok {
					if v, ok := custom["name"].(string); ok {
						variables["XAI_MODEL_NAME"] = strings.TrimSpace(v)
					}
					if n, ok := asInt(custom["context_window"]); ok {
						variables["XAI_CONTEXT_WINDOW"] = strconv.Itoa(n)
					}
					if n, ok := asInt(custom["max_output_tokens"]); ok {
						variables["XAI_MAX_TOKENS"] = strconv.Itoa(n)
					}
					if f, ok := asFloat(custom["temperature"]); ok {
						variables["XAI_TEMPERATURE"] = strconv.FormatFloat(f, 'f', -1, 64)
					}
				}
			}
		}
	}

	if !hasAnyValue(variables, "XAI_BASE_URL", "XAI_API_KEY", "XAI_MODEL") && len(templates) == 0 {
		return nil, nil
	}
	return &EnvConfig{
		Name:        a.uniqueEnvName("本机 Grok"),
		Description: "从本机 ~/.grok/config.toml 导入",
		Provider:    "grok",
		Variables:   variables,
		Templates:   templates,
		Icon:        "💻",
	}, nil
}

func resolveImportedBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := strings.ToLower(u.Hostname())
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return raw
	}
	path := strings.Trim(u.Path, "/")
	path = strings.TrimSuffix(path, "/v1")
	path = strings.Trim(path, "/")
	if !strings.HasPrefix(path, "env-") {
		return raw
	}
	rs := globalRouterService
	if rs == nil {
		return raw
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for _, route := range rs.config.Routes {
		if strings.EqualFold(route.Name, path) && strings.TrimSpace(route.BaseURL) != "" {
			return strings.TrimRight(strings.TrimSpace(route.BaseURL), "/")
		}
	}
	return raw
}

func copyIfSet(dst, src map[string]string, keys ...string) {
	for _, key := range keys {
		if v := strings.TrimSpace(src[key]); v != "" {
			dst[key] = v
		}
	}
}

func hasAnyValue(vars map[string]string, keys ...string) bool {
	for _, key := range keys {
		if strings.TrimSpace(vars[key]) != "" {
			return true
		}
	}
	return false
}

func bool01(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		return i, err == nil
	}
	return 0, false
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	}
	return 0, false
}

func replaceTOMLQuoted(src, key, old, next string) string {
	if old == "" {
		return src
	}
	repls := []string{
		key + " = \"" + old + "\"",
		key + " = '" + old + "'",
	}
	out := src
	for _, item := range repls {
		out = strings.ReplaceAll(out, item, key+" = \""+next+"\"")
	}
	return out
}
