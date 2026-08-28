package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func resolveGrokHome(vars map[string]string) string {
	home, _ := os.UserHomeDir()
	defaultHome := filepath.Join(home, ".grok")
	if vars != nil {
		if v := strings.TrimSpace(vars["GROK_HOME"]); v != "" {
			return expandAndNormalizePath(v, home, defaultHome)
		}
	}
	if v := strings.TrimSpace(os.Getenv("GROK_HOME")); v != "" {
		return expandAndNormalizePath(v, home, defaultHome)
	}
	return defaultHome
}

func grokConfigFile(vars map[string]string) string {
	return filepath.Join(resolveGrokHome(vars), "config.toml")
}

func grokSkillsRoot() string {
	return filepath.Join(resolveGrokHome(nil), "skills")
}

func (a *App) applyGrokEnv(env *EnvConfig) (string, error) {
	dir := resolveGrokHome(env.Variables)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建 Grok 配置目录失败: %v", err)
	}

	configFile := filepath.Join(dir, "config.toml")
	var content string
	if tmpl := strings.TrimSpace(env.Templates["config.toml"]); tmpl != "" {
		content = applyGrokTemplate(tmpl, env)
	} else {
		content = defaultGrokToml(env)
	}

	if err := mergeWriteGrokConfig(configFile, content, env.Variables); err != nil {
		return "", err
	}
	return fmt.Sprintf("Grok 配置已应用到 %s", configFile), nil
}

func defaultGrokToml(env *EnvConfig) string {
	model := strings.TrimSpace(env.Variables["XAI_MODEL"])
	if model == "" {
		model = "grok-4.6"
	}
	baseURL := strings.TrimSpace(env.Variables["XAI_BASE_URL"])
	if baseURL == "" {
		baseURL = "https://api.x.ai/v1"
	}
	backend := strings.TrimSpace(env.Variables["XAI_API_BACKEND"])
	if backend == "" {
		backend = "responses"
	}
	apiKey := strings.TrimSpace(env.Variables["XAI_API_KEY"])
	name := strings.TrimSpace(env.Variables["XAI_MODEL_NAME"])
	if name == "" {
		name = "Grok"
	}

	b := strings.Builder{}
	b.WriteString("[models]\n")
	b.WriteString("default = \"custom\"\n\n")
	b.WriteString("[model.custom]\n")
	b.WriteString(fmt.Sprintf("model = %q\n", model))
	b.WriteString(fmt.Sprintf("base_url = %q\n", baseURL))
	b.WriteString(fmt.Sprintf("name = %q\n", name))
	b.WriteString(fmt.Sprintf("api_backend = %q\n", backend))
	if apiKey != "" {
		b.WriteString(fmt.Sprintf("api_key = %q\n", apiKey))
	}
	return b.String()
}

func applyGrokTemplate(tmpl string, env *EnvConfig) string {
	out := tmpl
	repl := map[string]string{
		"{{XAI_MODEL}}":        env.Variables["XAI_MODEL"],
		"{{XAI_BASE_URL}}":     env.Variables["XAI_BASE_URL"],
		"{{XAI_API_KEY}}":      env.Variables["XAI_API_KEY"],
		"{{XAI_API_BACKEND}}":  env.Variables["XAI_API_BACKEND"],
		"{{XAI_MODEL_NAME}}":   env.Variables["XAI_MODEL_NAME"],
		"{{model}}":            env.Variables["XAI_MODEL"],
		"{{base_url}}":         env.Variables["XAI_BASE_URL"],
		"{{api_key}}":          env.Variables["XAI_API_KEY"],
	}
	for k, v := range repl {
		out = strings.ReplaceAll(out, k, v)
	}
	return out
}

func mergeWriteGrokConfig(configFile, incoming string, vars map[string]string) error {
	existing := map[string]any{}
	if data, err := os.ReadFile(configFile); err == nil && len(data) > 0 {
		_ = toml.Unmarshal(data, &existing)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取 Grok config.toml 失败: %v", err)
	}

	next := map[string]any{}
	if err := toml.Unmarshal([]byte(incoming), &next); err != nil || next == nil {
		next = map[string]any{}
	}

	preserve := []string{"mcp_servers", "skills", "plugins", "ui", "hooks", "compat", "mcp", "cli", "agent", "session", "permission"}
	for _, key := range preserve {
		if v, ok := existing[key]; ok {
			if _, already := next[key]; !already {
				next[key] = v
			}
		}
	}
	injectGrokExtras(next, vars)

	data, err := toml.Marshal(next)
	if err != nil {
		return fmt.Errorf("序列化 Grok config.toml 失败: %v", err)
	}
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("写入 Grok config.toml 失败: %v", err)
	}
	return nil
}

func (a *App) GetGrokSettings() map[string]string {
	vars := map[string]string{}
	if env := a.findEnv(a.config.CurrentEnvGrok); env != nil {
		vars = env.Variables
	}
	configFile := grokConfigFile(vars)
	result := map[string]string{
		"GROK_HOME":         resolveGrokHome(vars),
		"GROK_CONFIG_PATH":  configFile,
		"XAI_BASE_URL":      "https://api.x.ai/v1",
		"XAI_MODEL":         "",
		"XAI_API_BACKEND":   "responses",
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		if env := a.findEnv(a.config.CurrentEnvGrok); env != nil {
			for _, key := range []string{"XAI_BASE_URL", "XAI_MODEL", "XAI_API_KEY", "XAI_API_BACKEND"} {
				if v := strings.TrimSpace(env.Variables[key]); v != "" {
					result[key] = v
				}
			}
		}
		return result
	}

	payload := map[string]any{}
	if err := toml.Unmarshal(data, &payload); err != nil {
		result["GROK_CONFIG_PARSE_ERROR"] = err.Error()
		return result
	}

	if models, ok := payload["models"].(map[string]any); ok {
		if def, ok := models["default"].(string); ok {
			result["XAI_MODEL"] = strings.TrimSpace(def)
		}
	}
	if modelMap, ok := payload["model"].(map[string]any); ok {
		if custom, ok := modelMap["custom"].(map[string]any); ok {
			if v, ok := custom["model"].(string); ok && strings.TrimSpace(v) != "" {
				result["XAI_MODEL"] = strings.TrimSpace(v)
			}
			if v, ok := custom["base_url"].(string); ok && strings.TrimSpace(v) != "" {
				result["XAI_BASE_URL"] = strings.TrimSpace(v)
			}
			if v, ok := custom["api_backend"].(string); ok && strings.TrimSpace(v) != "" {
				result["XAI_API_BACKEND"] = strings.TrimSpace(v)
			}
			if v, ok := custom["api_key"].(string); ok && strings.TrimSpace(v) != "" {
				result["XAI_API_KEY"] = strings.TrimSpace(v)
			}
		}
	}
	return result
}

func (a *App) ClearGrokSettings() error {
	vars := map[string]string{}
	if env := a.findEnv(a.config.CurrentEnvGrok); env != nil {
		vars = env.Variables
	}
	configFile := grokConfigFile(vars)
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	payload := map[string]any{}
	if err := toml.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("解析 Grok config.toml 失败: %v", err)
	}

	if modelMap, ok := payload["model"].(map[string]any); ok {
		if custom, ok := modelMap["custom"].(map[string]any); ok {
			delete(custom, "api_key")
			modelMap["custom"] = custom
			payload["model"] = modelMap
		}
	}

	out, err := toml.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, out, 0644)
}
