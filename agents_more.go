package main

// 更多 Agent（第二批）：Gemini CLI、VS Code Chat、Zed、Goose、Command Code、Empryo、ZCode。
// 配置文件的写法参考 yetone/magpie 对各 Agent 的接入；与第一批一样，每个 Agent 一条独立路由，
// 写入前备份，断开时只删本程序写入的条目、还原原值。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

// gatewayRoot 去掉 /v1：Gemini、ZCode 等会自己补上版本路径
func gatewayRoot(gw string) string { return strings.TrimSuffix(strings.TrimRight(gw, "/"), "/v1") }

// ===== .env 文件（KEY=VALUE，保留注释与其它行） =====

func envFileGet(path, key string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && strings.TrimSpace(strings.TrimPrefix(k, "export ")) == key {
			return strings.Trim(strings.TrimSpace(v), `"'`), true
		}
	}
	return "", false
}

// envFileApply 设置或删除若干键；值为 nil 表示删除
func envFileApply(path string, changes map[string]*string) error {
	var lines []string
	if b, err := os.ReadFile(path); err == nil {
		lines = strings.Split(strings.TrimRight(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n"), "\n")
		if len(lines) == 1 && lines[0] == "" {
			lines = nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	done := map[string]bool{}
	out := make([]string, 0, len(lines)+len(changes))
	for _, line := range lines {
		k, _, ok := strings.Cut(strings.TrimSpace(line), "=")
		key := strings.TrimSpace(strings.TrimPrefix(k, "export "))
		if v, change := changes[key]; ok && change {
			done[key] = true
			if v != nil {
				out = append(out, key+"="+*v)
			}
			continue
		}
		out = append(out, line)
	}
	for key, v := range changes {
		if !done[key] && v != nil {
			out = append(out, key+"="+*v)
		}
	}
	if len(out) == 0 {
		if _, err := backupFile(path); err != nil {
			return err
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeAgentFile(path, []byte(strings.Join(out, "\n")+"\n"))
}

func strPtr(s string) *string { return &s }

// stashEnv 记下 .env 里某键的原值（null 表示原来没有）
func stashEnv(prev map[string]json.RawMessage, name, path, key string) {
	if _, done := prev[name]; done {
		return
	}
	if v, ok := envFileGet(path, key); ok {
		b, _ := json.Marshal(v)
		prev[name] = b
	} else {
		prev[name] = json.RawMessage("null")
	}
}

func restoredEnv(prev map[string]json.RawMessage, name string) *string {
	raw, ok := prev[name]
	if !ok || string(raw) == "null" {
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return nil
	}
	return &s
}

// ===== YAML 顶层的简单键（Goose 的 config.yaml） =====

func yamlTopGet(path, key string) (string, bool) {
	raw, ok := yamlTopRaw(path, key)
	return strings.Trim(raw, `"'`), ok
}

// yamlTopRaw 顶层键冒号后的原文（含引号），还原时原样写回
func yamlTopRaw(path, key string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+":")), true
		}
	}
	return "", false
}

func yamlQuote(s string) string {
	q, _ := json.Marshal(s)
	return string(q)
}

// yamlTopApply 设置或删除顶层键（值为 YAML 原文，nil 表示删除），其余内容原样保留
func yamlTopApply(path string, changes map[string]*string) error {
	var lines []string
	if b, err := os.ReadFile(path); err == nil {
		lines = strings.Split(strings.TrimRight(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n"), "\n")
		if len(lines) == 1 && lines[0] == "" {
			lines = nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	done := map[string]bool{}
	out := make([]string, 0, len(lines)+len(changes))
	for _, line := range lines {
		matched := ""
		for key := range changes {
			if strings.HasPrefix(line, key+":") {
				matched = key
				break
			}
		}
		if matched == "" {
			out = append(out, line)
			continue
		}
		done[matched] = true
		if v := changes[matched]; v != nil {
			out = append(out, matched+": "+*v)
		}
	}
	for key, v := range changes {
		if !done[key] && v != nil {
			out = append(out, key+": "+*v)
		}
	}
	return writeAgentFile(path, []byte(strings.Join(out, "\n")+"\n"))
}

// ===== 顶层带点号的 JSON 键（VS Code 的 chat.defaultModel） =====

func stashTop(prev map[string]json.RawMessage, name string, doc map[string]any, key string) {
	if _, done := prev[name]; done {
		return
	}
	if v, ok := doc[key]; ok {
		b, _ := json.Marshal(v)
		prev[name] = b
	} else {
		prev[name] = json.RawMessage("null")
	}
}

func restoreTop(prev map[string]json.RawMessage, name string, doc map[string]any, key string) {
	raw, ok := prev[name]
	if !ok {
		return
	}
	if string(raw) == "null" {
		delete(doc, key)
		return
	}
	var v any
	if json.Unmarshal(raw, &v) == nil {
		doc[key] = v
	}
}

// ===== 各 Agent 的路径 =====

func geminiCLIDir() string { return envOr("GEMINI_CLI_HOME", filepath.Join(homeDir(), ".gemini")) }

func vscodeUserDir() string {
	switch goruntime.GOOS {
	case "windows":
		return filepath.Join(envOr("APPDATA", filepath.Join(homeDir(), "AppData", "Roaming")), "Code", "User")
	case "darwin":
		return filepath.Join(homeDir(), "Library", "Application Support", "Code", "User")
	}
	return filepath.Join(envOr("XDG_CONFIG_HOME", filepath.Join(homeDir(), ".config")), "Code", "User")
}

func zedSettingsPath() string {
	switch goruntime.GOOS {
	case "windows":
		return filepath.Join(envOr("APPDATA", filepath.Join(homeDir(), "AppData", "Roaming")), "Zed", "settings.json")
	}
	return filepath.Join(envOr("XDG_CONFIG_HOME", filepath.Join(homeDir(), ".config")), "zed", "settings.json")
}

func gooseConfigPath() string {
	if goruntime.GOOS == "windows" {
		if app := os.Getenv("APPDATA"); app != "" {
			return filepath.Join(app, "Block", "goose", "config", "config.yaml")
		}
	}
	return filepath.Join(envOr("XDG_CONFIG_HOME", filepath.Join(homeDir(), ".config")), "goose", "config.yaml")
}

func gooseProviderFile() string {
	return filepath.Join(filepath.Dir(gooseConfigPath()), "custom_providers", agentProviderID+".json")
}

func zcodeDir() string { return filepath.Join(homeDir(), ".zcode", "v2") }

const vscodeDefaultModelKey = "chat.defaultModel"

func moreAgents() []extraAgent {
	return []extraAgent{
		{
			// Gemini CLI 只说 Gemini 协议：网关把它的请求转成路由上游的协议
			id: "gemini", name: "Gemini CLI", command: "gemini",
			note: "经网关的 Gemini 协议入口转换，可用 Claude / OpenAI / 订阅等任意上游",
			paths: func() []string {
				return []string{filepath.Join(geminiCLIDir(), "settings.json"), filepath.Join(geminiCLIDir(), ".env")}
			},
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				settings, envPath := filepath.Join(geminiCLIDir(), "settings.json"), filepath.Join(geminiCLIDir(), ".env")
				doc, err := readJSONDoc(settings)
				if err != nil {
					return err
				}
				stashJSON(prev, "auth", doc, "security.auth.selectedType")
				stashJSON(prev, "model", doc, "model.name")
				stashEnv(prev, "env.base", envPath, "GOOGLE_GEMINI_BASE_URL")
				stashEnv(prev, "env.key", envPath, "GEMINI_API_KEY")
				if err := envFileApply(envPath, map[string]*string{
					"GOOGLE_GEMINI_BASE_URL": strPtr(gatewayRoot(gw)),
					"GEMINI_API_KEY":         strPtr(agentProviderID),
				}); err != nil {
					return err
				}
				jsonSet(doc, "security.auth.selectedType", "gemini-api-key")
				jsonSet(doc, "model.name", model)
				return writeJSONDoc(settings, doc)
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				settings, envPath := filepath.Join(geminiCLIDir(), "settings.json"), filepath.Join(geminiCLIDir(), ".env")
				if err := envFileApply(envPath, map[string]*string{
					"GOOGLE_GEMINI_BASE_URL": restoredEnv(prev, "env.base"),
					"GEMINI_API_KEY":         restoredEnv(prev, "env.key"),
				}); err != nil {
					return err
				}
				doc, err := readJSONDoc(settings)
				if err != nil {
					return err
				}
				restoreJSON(prev, "auth", doc, "security.auth.selectedType")
				restoreJSON(prev, "model", doc, "model.name")
				return writeJSONDocOrRemove(settings, doc)
			},
			warn: func() string {
				// 进程环境变量优先于 ~/.gemini/.env（Antigravity 的配置会写到用户环境变量里）
				for _, key := range []string{"GOOGLE_GEMINI_BASE_URL", "GEMINI_API_KEY"} {
					if v := firstNonEmpty(readAntigravityUserEnv()[key], os.Getenv(key)); v != "" && !strings.Contains(v, "/agent-gemini") && v != agentProviderID {
						return "系统环境变量里已有 " + key + "（多半来自 Antigravity 的配置），Gemini CLI 会优先用它而不是 ~/.gemini/.env，接入后可能不生效"
					}
				}
				return ""
			},
			connected: func() (bool, string) {
				base, _ := envFileGet(filepath.Join(geminiCLIDir(), ".env"), "GOOGLE_GEMINI_BASE_URL")
				if !strings.Contains(base, "/agent-gemini") {
					return false, ""
				}
				doc, _ := readJSONDoc(filepath.Join(geminiCLIDir(), "settings.json"))
				m, _ := jsonGet(doc, "model.name")
				ms, _ := m.(string)
				return true, ms
			},
		},
		{
			// VS Code 1.122+ 的 Chat 自定义端点：模型出现在 Chat 的模型选择器里，不需要 Copilot 登录
			id: "vscode", name: "VS Code Chat", command: "code",
			note: "写入 chatLanguageModels.json 的自定义端点（需 VS Code 1.122+），改完在 VS Code 里执行 Reload Window",
			paths: func() []string {
				return []string{filepath.Join(vscodeUserDir(), "chatLanguageModels.json"), filepath.Join(vscodeUserDir(), "settings.json")}
			},
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				modelsPath, settings := filepath.Join(vscodeUserDir(), "chatLanguageModels.json"), filepath.Join(vscodeUserDir(), "settings.json")
				groups, err := readJSONList(modelsPath)
				if err != nil {
					return err
				}
				ctx, out := modelLimits(model)
				group := map[string]any{"name": "AI ENV", "vendor": "customendpoint", "apiType": "chat-completions", "models": []any{map[string]any{
					"id": model, "name": "AI ENV · " + model, "url": strings.TrimRight(gw, "/") + "/chat/completions",
					"toolCalling": true, "vision": true, "contextWindow": ctx, "maxOutputTokens": min(out, ctx/4),
					"requestHeaders": map[string]string{"Authorization": "Bearer " + agentProviderID},
				}}}
				kept := []any{group}
				for _, g := range groups {
					if m, ok := g.(map[string]any); ok && m["vendor"] == "customendpoint" && m["name"] == "AI ENV" {
						continue
					}
					kept = append(kept, g)
				}
				if err := writeJSONList(modelsPath, kept); err != nil {
					return err
				}
				doc, err := readJSONDoc(settings)
				if err != nil {
					return err
				}
				stashTop(prev, "model", doc, vscodeDefaultModelKey)
				doc[vscodeDefaultModelKey] = model
				return writeJSONDoc(settings, doc)
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				modelsPath, settings := filepath.Join(vscodeUserDir(), "chatLanguageModels.json"), filepath.Join(vscodeUserDir(), "settings.json")
				groups, err := readJSONList(modelsPath)
				if err != nil {
					return err
				}
				kept := []any{}
				for _, g := range groups {
					if m, ok := g.(map[string]any); ok && m["vendor"] == "customendpoint" && m["name"] == "AI ENV" {
						continue
					}
					kept = append(kept, g)
				}
				if err := writeJSONList(modelsPath, kept); err != nil {
					return err
				}
				doc, err := readJSONDoc(settings)
				if err != nil {
					return err
				}
				restoreTop(prev, "model", doc, vscodeDefaultModelKey)
				return writeJSONDoc(settings, doc)
			},
			connected: func() (bool, string) {
				groups, _ := readJSONList(filepath.Join(vscodeUserDir(), "chatLanguageModels.json"))
				for _, g := range groups {
					if m, ok := g.(map[string]any); ok && m["vendor"] == "customendpoint" && m["name"] == "AI ENV" {
						doc, _ := readJSONDoc(filepath.Join(vscodeUserDir(), "settings.json"))
						cur, _ := doc[vscodeDefaultModelKey].(string)
						return true, cur
					}
				}
				return false, ""
			},
		},
		{
			// Zed 的 OpenAI 兼容服务商；Windows 上 API Key（占位值）写进凭据管理器，其它系统首次使用时在 Zed 里随便填一个
			id: "zed", name: "Zed", command: "zed",
			note:  "OpenAI 兼容服务商 aienv；非 Windows 系统首次使用需在 Zed 的 Agent 设置里为 aienv 随便填一个 API Key",
			paths: func() []string { return []string{zedSettingsPath()} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				path := zedSettingsPath()
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				stashJSON(prev, "model", doc, "agent.default_model")
				ctx, out := modelLimits(model)
				jsonSet(doc, "language_models.openai_compatible."+agentProviderID, map[string]any{
					"api_url": gw,
					"available_models": []any{map[string]any{
						"name": model, "display_name": "AI ENV · " + model, "max_tokens": ctx + out, "max_output_tokens": out,
						"capabilities": map[string]any{"tools": true, "images": true, "parallel_tool_calls": false, "prompt_cache_key": false, "chat_completions": true},
					}},
				})
				jsonSet(doc, "agent.default_model", map[string]any{"provider": agentProviderID, "model": model})
				if err := writeJSONDoc(path, doc); err != nil {
					return err
				}
				_ = saveZedCredential(gw)
				return nil
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				path := zedSettingsPath()
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				if v, ok := jsonGet(doc, "language_models.openai_compatible."+agentProviderID+".api_url"); ok {
					if s, _ := v.(string); s != "" {
						_ = deleteZedCredential(s)
					}
				}
				jsonDelete(doc, "language_models.openai_compatible."+agentProviderID)
				restoreJSON(prev, "model", doc, "agent.default_model")
				return writeJSONDocOrRemove(path, doc)
			},
			connected: func() (bool, string) {
				doc, err := readJSONDoc(zedSettingsPath())
				if err != nil {
					return false, ""
				}
				if _, ok := jsonGet(doc, "language_models.openai_compatible."+agentProviderID); !ok {
					return false, ""
				}
				m, _ := jsonGet(doc, "agent.default_model.model")
				ms, _ := m.(string)
				return true, ms
			},
		},
		{
			id: "goose", name: "Goose", command: "goose",
			note:  "自定义服务商写在 custom_providers/aienv.json，重开 Goose 生效",
			paths: func() []string { return []string{gooseConfigPath(), gooseProviderFile()} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				path := gooseConfigPath()
				ctx, _ := modelLimits(model)
				provider := map[string]any{
					"name": agentProviderID, "engine": "openai", "display_name": "AI ENV",
					"description": "AI ENV 本机网关", "api_key_env": "", "base_url": gw,
					"headers":                  map[string]any{"Authorization": "Bearer " + agentProviderID},
					"requires_auth":            false,
					"skip_canonical_filtering": true,
					"dynamic_models":           false,
					"models":                   []any{map[string]any{"name": model, "context_limit": ctx}},
				}
				b, _ := json.MarshalIndent(provider, "", "  ")
				if err := writeAgentFile(gooseProviderFile(), append(b, '\n')); err != nil {
					return err
				}
				for _, key := range []string{"GOOSE_PROVIDER", "GOOSE_MODEL"} {
					stashEnvLike(prev, key, func() (string, bool) { return yamlTopRaw(path, key) })
				}
				return yamlTopApply(path, map[string]*string{"GOOSE_PROVIDER": strPtr(yamlQuote(agentProviderID)), "GOOSE_MODEL": strPtr(yamlQuote(model))})
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				if err := os.Remove(gooseProviderFile()); err != nil && !os.IsNotExist(err) {
					return err
				}
				return yamlTopApply(gooseConfigPath(), map[string]*string{
					"GOOSE_PROVIDER": restoredEnv(prev, "GOOSE_PROVIDER"),
					"GOOSE_MODEL":    restoredEnv(prev, "GOOSE_MODEL"),
				})
			},
			connected: func() (bool, string) {
				if p, _ := yamlTopGet(gooseConfigPath(), "GOOSE_PROVIDER"); p != agentProviderID {
					return false, ""
				}
				m, _ := yamlTopGet(gooseConfigPath(), "GOOSE_MODEL")
				return true, m
			},
		},
		{
			id: "commandcode", name: "Command Code", command: "command-code",
			paths: func() []string {
				dir := filepath.Join(homeDir(), ".commandcode")
				return []string{filepath.Join(dir, "settings.json"), filepath.Join(dir, "providers.json")}
			},
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				dir := filepath.Join(homeDir(), ".commandcode")
				providers, err := readJSONDoc(filepath.Join(dir, "providers.json"))
				if err != nil {
					return err
				}
				jsonSet(providers, "provider."+agentProviderID, map[string]any{"name": "AI ENV", "api": "openai-completions", "baseURL": gw, "apiKey": false,
					"models": map[string]any{model: map[string]any{"name": "AI ENV · " + model}}})
				if err := writeJSONDoc(filepath.Join(dir, "providers.json"), providers); err != nil {
					return err
				}
				settings, err := readJSONDoc(filepath.Join(dir, "settings.json"))
				if err != nil {
					return err
				}
				stashJSON(prev, "model", settings, "model")
				stashJSON(prev, "modelProvider", settings, "modelProvider")
				jsonSet(settings, "model", model)
				jsonSet(settings, "modelProvider", agentProviderID)
				return writeJSONDoc(filepath.Join(dir, "settings.json"), settings)
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				dir := filepath.Join(homeDir(), ".commandcode")
				providers, err := readJSONDoc(filepath.Join(dir, "providers.json"))
				if err != nil {
					return err
				}
				jsonDelete(providers, "provider."+agentProviderID)
				if err := writeJSONDocOrRemove(filepath.Join(dir, "providers.json"), providers); err != nil {
					return err
				}
				settings, err := readJSONDoc(filepath.Join(dir, "settings.json"))
				if err != nil {
					return err
				}
				restoreJSON(prev, "model", settings, "model")
				restoreJSON(prev, "modelProvider", settings, "modelProvider")
				return writeJSONDocOrRemove(filepath.Join(dir, "settings.json"), settings)
			},
			connected: func() (bool, string) {
				settings, err := readJSONDoc(filepath.Join(homeDir(), ".commandcode", "settings.json"))
				if err != nil {
					return false, ""
				}
				if p, _ := jsonGet(settings, "modelProvider"); p != agentProviderID {
					return false, ""
				}
				m, _ := jsonGet(settings, "model")
				ms, _ := m.(string)
				return true, ms
			},
		},
		{
			id: "empryo", name: "Empryo", command: "empryo",
			paths: func() []string { return []string{filepath.Join(homeDir(), ".empryo", "config.json")} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				path := filepath.Join(homeDir(), ".empryo", "config.json")
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				stashJSON(prev, "model", doc, "defaultModel")
				list, _ := doc["providers"].([]any)
				kept := []any{}
				for _, p := range list {
					if m, ok := p.(map[string]any); ok && m["id"] == agentProviderID {
						continue
					}
					kept = append(kept, p)
				}
				doc["providers"] = append(kept, map[string]any{"id": agentProviderID, "name": "AI ENV", "baseURL": gw, "modelsAPI": gw + "/models"})
				doc["defaultModel"] = agentProviderID + "/" + model
				return writeJSONDoc(path, doc)
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				path := filepath.Join(homeDir(), ".empryo", "config.json")
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				list, _ := doc["providers"].([]any)
				kept := []any{}
				for _, p := range list {
					if m, ok := p.(map[string]any); ok && m["id"] == agentProviderID {
						continue
					}
					kept = append(kept, p)
				}
				if len(kept) == 0 {
					delete(doc, "providers")
				} else {
					doc["providers"] = kept
				}
				restoreJSON(prev, "model", doc, "defaultModel")
				return writeJSONDocOrRemove(path, doc)
			},
			connected: func() (bool, string) {
				doc, err := readJSONDoc(filepath.Join(homeDir(), ".empryo", "config.json"))
				if err != nil {
					return false, ""
				}
				m, _ := doc["defaultModel"].(string)
				ref, ok := strings.CutPrefix(m, agentProviderID+"/")
				return ok, ref
			},
		},
		{
			// ZCode（智谱桌面端）：Anthropic 协议的自定义服务商，新版读 provider_config.json 的规则，旧版读 config.json
			id: "zcode", name: "ZCode", command: "zcode",
			note: "模型在 ZCode 的模型选择器里选，重开 ZCode 生效",
			paths: func() []string {
				return []string{filepath.Join(zcodeDir(), "config.json"), filepath.Join(zcodeDir(), "provider_config.json")}
			},
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				base := gatewayRoot(gw)
				ctx, out := modelLimits(model)
				path := filepath.Join(zcodeDir(), "config.json")
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				jsonSet(doc, "provider."+agentProviderID, map[string]any{
					"name": "AI ENV", "kind": "anthropic", "enabled": true, "source": "custom",
					"options": map[string]any{"apiKey": agentProviderID, "baseURL": base},
					"models": map[string]any{model: map[string]any{"name": model, "limit": map[string]any{"context": ctx, "output": min(out, 128000)},
						"modalities": map[string]any{"input": []string{"text", "image"}, "output": []string{"text"}}}},
				})
				if err := writeJSONDoc(path, doc); err != nil {
					return err
				}
				return zcodeApplyRules(base, model, ctx, min(out, 128000), true)
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				path := filepath.Join(zcodeDir(), "config.json")
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				jsonDelete(doc, "provider."+agentProviderID)
				if err := writeJSONDoc(path, doc); err != nil {
					return err
				}
				return zcodeApplyRules("", "", 0, 0, false)
			},
			connected: func() (bool, string) {
				doc, err := readJSONDoc(filepath.Join(zcodeDir(), "config.json"))
				if err != nil {
					return false, ""
				}
				v, ok := jsonGet(doc, "provider."+agentProviderID+".models")
				if !ok {
					return false, ""
				}
				for id := range asMap(v) {
					return true, id
				}
				return true, ""
			},
		},
	}
}

func asMap(v any) map[string]any { m, _ := v.(map[string]any); return m }

// stashEnvLike 记下某个简单键的原值（null 表示原来没有）
func stashEnvLike(prev map[string]json.RawMessage, name string, get func() (string, bool)) {
	if _, done := prev[name]; done {
		return
	}
	if v, ok := get(); ok {
		b, _ := json.Marshal(v)
		prev[name] = b
	} else {
		prev[name] = json.RawMessage("null")
	}
}

// readJSONList 读取顶层是数组的 JSON 文件
func readJSONList(path string) ([]any, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) || len(strings.TrimSpace(string(b))) == 0 {
		return []any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var list []any
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("%s 不是 JSON 数组，未做任何修改: %v", path, err)
	}
	return list, nil
}

func writeJSONList(path string, list []any) error {
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return writeAgentFile(path, append(b, '\n'))
}

// zcodeApplyRules 新版 ZCode 的 provider_config.json：加入或移除 aienv 的服务商规则与模型规则
func zcodeApplyRules(base, model string, ctx, out int, on bool) error {
	path := filepath.Join(zcodeDir(), "provider_config.json")
	doc, err := readJSONDoc(path)
	if err != nil {
		return err
	}
	if len(doc) == 0 && !on {
		return nil
	}
	if doc["schemaVersion"] == nil {
		doc["schemaVersion"] = 1
	}
	obj := func(m map[string]any, k string) map[string]any {
		o, ok := m[k].(map[string]any)
		if !ok {
			o = map[string]any{}
			m[k] = o
		}
		return o
	}
	list := func(m map[string]any, k string) []any { l, _ := m[k].([]any); return l }
	mine := func(r any) bool { m, _ := r.(map[string]any); return m != nil && m["providerId"] == agentProviderID }
	cfg := obj(doc, "config")
	pcr, mcr := obj(cfg, "providerConfigRules"), obj(cfg, "modelConfigRules")
	providers, models := []any{}, []any{}
	for _, r := range list(pcr, "providerRules") {
		if !mine(r) {
			providers = append(providers, r)
		}
	}
	for _, r := range list(mcr, "providerModelRules") {
		if !mine(r) {
			models = append(models, r)
		}
	}
	if on {
		providers = append(providers, map[string]any{"providerId": agentProviderID, "providerName": "AI ENV", "enabled": true,
			"config": map[string]any{
				"group":            "standard-personal",
				"access":           map[string]any{"type": "api-key", "apiKey": agentProviderID},
				"api":              map[string]any{"type": "anthropic-messages", "baseUrl": base},
				"personalModelIds": []string{model}, "modelOrder": []string{model},
			}})
		models = append(models, map[string]any{"providerId": agentProviderID, "modelId": model, "config": map[string]any{
			"properties":  map[string]any{"contextWindow": ctx, "inputFormat": map[string]any{"supportsImage": true}},
			"optionSpecs": map[string]any{"maxOutputTokens": map[string]any{"max": out}},
		}})
	}
	pcr["providerRules"] = providers
	mcr["providerModelRules"] = models
	if list(mcr, "manualProviderModelRules") == nil {
		mcr["manualProviderModelRules"] = []any{}
	}
	return writeJSONDoc(path, doc)
}
