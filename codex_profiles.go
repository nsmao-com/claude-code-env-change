package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func (w *WorkbenchService) SetCodexAuthMode(name, mode string) error {
	if mode != "api" && mode != "mixed" {
		return fmt.Errorf("请选择 API 或保留官方登录模式")
	}
	env, e := w.env("codex", name)
	if e != nil {
		return e
	}
	if env.OfficialLogin {
		return fmt.Errorf("官方登录环境请直接在环境页切换")
	}
	env.Variables["AI_ENV_AUTH_MODE"] = mode
	return w.app.UpdateEnv(name, "codex", env)
}
func configureCodexProvider(payload map[string]any, vars map[string]string) error {
	key := strings.TrimSpace(vars["OPENAI_API_KEY"])
	if key == "" {
		return nil
	}
	id, _ := payload["model_provider"].(string)
	if id == "" {
		return fmt.Errorf("Codex API 配置缺少 model_provider")
	}
	providers, _ := payload["model_providers"].(map[string]any)
	if providers == nil {
		return fmt.Errorf("Codex API 配置缺少供应商表")
	}
	entry, _ := providers[id].(map[string]any)
	if entry == nil {
		return fmt.Errorf("Codex 未找到当前供应商 %s", id)
	}
	if id == "openai" || id == "ollama" || id == "lmstudio" || id == "amazon-bedrock-runtime" {
		newID := "ai_env_" + strings.ReplaceAll(id, "-", "_")
		if _, exists := providers[newID]; exists {
			return fmt.Errorf("保留供应商名称与 %s 冲突", newID)
		}
		delete(providers, id)
		providers[newID] = entry
		payload["model_provider"] = newID
		id = newID
	}
	if _, ok := entry["auth"]; ok {
		return fmt.Errorf("供应商使用外部认证命令，不能同时写入 API Key")
	}
	if envKey, ok := entry["env_key"].(string); ok && strings.TrimSpace(envKey) != "" {
		delete(entry, "env_key")
	}
	entry["experimental_bearer_token"] = key
	entry["requires_openai_auth"] = false
	if strings.TrimSpace(asString(entry["name"])) == "" {
		entry["name"] = id
	}
	entry["wire_api"] = "responses"
	for name, raw := range providers {
		if table, ok := raw.(map[string]any); ok && strings.TrimSpace(asString(table["name"])) == "" {
			table["name"] = name
		}
	}
	return nil
}
func writeCodexCatalog(env EnvConfig, models []ModelProfile) (string, error) {
	entries := []map[string]any{}
	for _, p := range models {
		if p.Provider != "codex" || p.Environment != env.Name {
			continue
		}
		m := map[string]any{"slug": p.Model, "display_name": p.Model, "description": "AI ENV model profile", "supported_reasoning_levels": []any{}, "shell_type": "shell_command", "visibility": "list", "supported_in_api": true, "priority": len(entries), "support_verbosity": false, "truncation_policy": map[string]any{"mode": "tokens", "limit": 10000}, "experimental_supported_tools": []string{}, "input_modalities": []string{"text"}}
		if p.Context > 0 {
			m["context_window"] = p.Context
		}
		if p.Compact > 0 {
			m["auto_compact_token_limit"] = p.Compact
		}
		entries = append(entries, m)
	}
	if len(entries) == 0 {
		return "", nil
	}
	home, e := os.UserHomeDir()
	if e != nil {
		return "", e
	}
	dir := filepath.Join(resolveCodexHome(home), "ai-env-models")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	p := filepath.Join(dir, contentHash([]byte(env.Name))[:20]+".json")
	b, e := json.MarshalIndent(map[string]any{"models": entries}, "", "  ")
	if e != nil {
		return "", e
	}
	return p, writeFileAtomic(p, b, 0600)
}
func validateCodexTOML(data []byte) error {
	var cfg map[string]any
	if e := toml.Unmarshal(data, &cfg); e != nil {
		return fmt.Errorf("Codex 配置格式无效: %w", e)
	}
	if _, ok := cfg["model_provider"].(string); !ok {
		return fmt.Errorf("缺少 Codex 供应商名称")
	}
	return nil
}
