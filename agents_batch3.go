package main

// 更多 Agent（第三批）：Qoder、Qoder CN、MiMo Code、OmO、fx、T3 Code、Muse Code、
// Hermes Agent、MiniMax Code、Mister Morph、omp。配置写法参考 yetone/magpie；
// 与前两批一样，每个 Agent 一条独立路由，写入前备份，断开时只删本程序写入的内容、还原原值。

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// piLikeAgent Pi 及其分支（OmO）：settings.json 的默认服务商 / 模型 + models.json 里的 aienv 服务商
func piLikeAgent(id, name, command string, dir func() string) extraAgent {
	return extraAgent{
		id: id, name: name, command: command,
		paths: func() []string {
			return []string{filepath.Join(dir(), "settings.json"), filepath.Join(dir(), "models.json")}
		},
		connect: func(gw, model string, prev map[string]json.RawMessage) error {
			modelsPath, settingsPath := filepath.Join(dir(), "models.json"), filepath.Join(dir(), "settings.json")
			models, err := readJSONDoc(modelsPath)
			if err != nil {
				return err
			}
			settings, err := readJSONDoc(settingsPath)
			if err != nil {
				return err
			}
			ctx, out := modelLimits(model)
			jsonSet(models, "providers."+agentProviderID, map[string]any{
				"baseUrl": gw, "api": "openai-completions", "apiKey": agentProviderID,
				"models": []any{map[string]any{"id": model, "name": model, "reasoning": false, "contextWindow": ctx, "maxTokens": out}},
			})
			stashJSON(prev, "provider", settings, "defaultProvider")
			stashJSON(prev, "model", settings, "defaultModel")
			settings["defaultProvider"], settings["defaultModel"] = agentProviderID, model
			if err := writeJSONDoc(modelsPath, models); err != nil {
				return err
			}
			return writeJSONDoc(settingsPath, settings)
		},
		disconnect: func(prev map[string]json.RawMessage) error {
			modelsPath, settingsPath := filepath.Join(dir(), "models.json"), filepath.Join(dir(), "settings.json")
			if settings, err := readJSONDoc(settingsPath); err == nil {
				restoreJSON(prev, "provider", settings, "defaultProvider")
				restoreJSON(prev, "model", settings, "defaultModel")
				if err := writeJSONDocOrRemove(settingsPath, settings); err != nil {
					return err
				}
			}
			models, err := readJSONDoc(modelsPath)
			if err != nil {
				return err
			}
			jsonDelete(models, "providers."+agentProviderID)
			return writeJSONDocOrRemove(modelsPath, models)
		},
		connected: func() (bool, string) {
			settings, err := readJSONDoc(filepath.Join(dir(), "settings.json"))
			if err != nil || settings["defaultProvider"] != agentProviderID {
				return false, ""
			}
			m, _ := settings["defaultModel"].(string)
			return true, m
		},
	}
}

func absEnvDir(keys ...string) string {
	for _, k := range keys {
		if d := strings.TrimSpace(os.Getenv(k)); d != "" && filepath.IsAbs(d) {
			return filepath.Clean(d)
		}
	}
	return ""
}

func omoDir() string {
	if d := absEnvDir("OMO_CODING_AGENT_DIR", "SENPI_CODING_AGENT_DIR"); d != "" {
		return d
	}
	return filepath.Join(homeDir(), ".omo", "agent")
}

func xdgConfigDir() string { return envOr("XDG_CONFIG_HOME", filepath.Join(homeDir(), ".config")) }

// openCodeStyleFile OpenCode 形状的配置（MiMo Code）：已有的 jsonc / json / config.json 优先
func openCodeStyleFile(dir, id string) string {
	for _, name := range []string{id + ".jsonc", id + ".json", "config.json"} {
		if fileExists(filepath.Join(dir, name)) {
			return filepath.Join(dir, name)
		}
	}
	return filepath.Join(dir, id+".json")
}

func mimocodeDir() string {
	if h := absEnvDir("MIMOCODE_HOME"); h != "" {
		return filepath.Join(h, "config")
	}
	return filepath.Join(xdgConfigDir(), "mimocode")
}

func t3SettingsPath() string {
	if h := absEnvDir("T3CODE_HOME"); h != "" {
		return filepath.Join(h, "userdata", "settings.json")
	}
	return filepath.Join(homeDir(), ".t3", "userdata", "settings.json")
}

func hermesConfigPath() string {
	if h := absEnvDir("HERMES_HOME"); h != "" {
		return filepath.Join(h, "config.yaml")
	}
	return filepath.Join(homeDir(), ".hermes", "config.yaml")
}

func minimaxConfigPath() string {
	if h := absEnvDir("MINIMAX_DATA_DIR"); h != "" {
		return filepath.Join(h, "config.yaml")
	}
	return filepath.Join(homeDir(), ".minimax", "config.yaml")
}

func morphConfigPath() string {
	if p := strings.TrimSpace(os.Getenv("MISTER_MORPH_CONFIG")); p != "" && filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(homeDir(), ".morph", "config.yaml")
}

func ompDir() string {
	if d := absEnvDir("PI_CODING_AGENT_DIR"); d != "" && strings.Contains(d, ".omp") {
		return d
	}
	return filepath.Join(homeDir(), ".omp", "agent")
}

// qoderAgent Qoder CLI 与 Qoder CN：settings.json 的 providers.aienv（OpenAI 协议）+ model.name
func qoderAgent(id, name, command, env, dirName string) extraAgent {
	path := func() string {
		if d := absEnvDir(env); d != "" {
			return filepath.Join(d, "settings.json")
		}
		return filepath.Join(homeDir(), dirName, "settings.json")
	}
	return extraAgent{
		id: id, name: name, command: command,
		note:  "需要已登录且套餐支持自带模型（BYOK）的 Qoder 账号",
		paths: func() []string { return []string{path()} },
		connect: func(gw, model string, prev map[string]json.RawMessage) error {
			doc, err := readJSONDoc(path())
			if err != nil {
				return err
			}
			stashJSON(prev, "model", doc, "model.name")
			ctx, out := modelLimits(model)
			jsonSet(doc, "providers."+agentProviderID, map[string]any{
				"displayName": "AI ENV", "protocol": "openai", "baseUrl": gw, "apiKey": agentProviderID, "model": model,
				"models": []any{map[string]any{"model": model, "displayName": "AI ENV · " + model, "contextWindow": ctx, "maxOutputTokens": out,
					"capabilities": map[string]any{"tools": true, "vision": true}}},
			})
			jsonSet(doc, "model.name", agentProviderID+"/"+model)
			return writeJSONDoc(path(), doc)
		},
		disconnect: func(prev map[string]json.RawMessage) error {
			doc, err := readJSONDoc(path())
			if err != nil {
				return err
			}
			jsonDelete(doc, "providers."+agentProviderID)
			restoreJSON(prev, "model", doc, "model.name")
			return writeJSONDocOrRemove(path(), doc)
		},
		connected: func() (bool, string) {
			doc, err := readJSONDoc(path())
			if err != nil {
				return false, ""
			}
			m, _ := jsonGet(doc, "model.name")
			ref, ok := strings.CutPrefix(asString(m), agentProviderID+"/")
			if _, has := jsonGet(doc, "providers."+agentProviderID); !has || !ok {
				return false, ""
			}
			return true, ref
		},
	}
}

func workbuddyModelsPath() string {
	if d := absEnvDir("WORKBUDDY_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "models.json")
	}
	return filepath.Join(homeDir(), ".workbuddy", "models.json")
}

func atomcodeConfigPath() string {
	if d := absEnvDir("ATOMCODE_HOME"); d != "" {
		return filepath.Join(d, "config.toml")
	}
	return filepath.Join(homeDir(), ".atomcode", "config.toml")
}

// workbuddyRead models.json 是模型数组，或 CodeBuddy 命令行写的 {"models": [...], "availableModels": [...]}
func workbuddyRead() ([]any, map[string]any, error) {
	b, err := os.ReadFile(workbuddyModelsPath())
	if os.IsNotExist(err) || err == nil && len(strings.TrimSpace(string(b))) == 0 {
		return []any{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var list []any
	if json.Unmarshal(b, &list) == nil {
		return list, nil, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil, nil, err
	}
	list, _ = obj["models"].([]any)
	return list, obj, nil
}

func workbuddyWrite(list []any, obj map[string]any, add string, remove []string) error {
	if obj == nil {
		if len(list) == 0 {
			if _, err := backupFile(workbuddyModelsPath()); err != nil {
				return err
			}
			if err := os.Remove(workbuddyModelsPath()); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		}
		b, _ := json.MarshalIndent(list, "", "  ")
		return writeAgentFile(workbuddyModelsPath(), append(b, '\n'))
	}
	obj["models"] = list
	if avail, ok := obj["availableModels"].([]any); ok {
		kept := []any{}
		for _, id := range avail {
			drop := false
			for _, r := range remove {
				if id == r {
					drop = true
				}
			}
			if !drop {
				kept = append(kept, id)
			}
		}
		if add != "" {
			kept = append(kept, add)
		}
		obj["availableModels"] = kept
	}
	return writeJSONDoc(workbuddyModelsPath(), obj)
}

func workbuddyOurs(list []any) (kept []any, ids []string) {
	kept = []any{}
	for _, item := range list {
		if m, ok := item.(map[string]any); ok && m["vendor"] == agentProviderID {
			ids = append(ids, asString(m["id"]))
			continue
		}
		kept = append(kept, item)
	}
	return kept, ids
}

func batch3Agents() []extraAgent {
	return []extraAgent{
		{
			// WorkBuddy（腾讯 CodeBuddy 桌面端）：models.json 里加一个自有模型，在它的模型选择器里选
			id: "workbuddy", name: "WorkBuddy", command: "workbuddy",
			note:  "模型出现在 WorkBuddy 的模型选择器里（custom-local），它会自动读到改动",
			paths: func() []string { return []string{workbuddyModelsPath()} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				list, obj, err := workbuddyRead()
				if err != nil {
					return err
				}
				kept, old := workbuddyOurs(list)
				ctx, out := modelLimits(model)
				kept = append(kept, map[string]any{
					"id": model, "name": "AI ENV · " + model, "vendor": agentProviderID, "apiKey": agentProviderID,
					"url": strings.TrimRight(gw, "/") + "/chat/completions", "maxInputTokens": ctx, "maxOutputTokens": out,
					"supportsToolCall": true, "supportsImages": true,
				})
				return workbuddyWrite(kept, obj, model, old)
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				list, obj, err := workbuddyRead()
				if err != nil {
					return err
				}
				kept, old := workbuddyOurs(list)
				return workbuddyWrite(kept, obj, "", old)
			},
			connected: func() (bool, string) {
				list, _, err := workbuddyRead()
				if err != nil {
					return false, ""
				}
				if _, ids := workbuddyOurs(list); len(ids) > 0 {
					return true, ids[0]
				}
				return false, ""
			},
		},
		{
			// AtomCode：[provider_accounts.aienv] + [models."aienv/<模型>"]，顶层 default_model 指向它
			id: "atomcode", name: "AtomCode", command: "atomcode",
			paths: func() []string { return []string{atomcodeConfigPath()} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				path := atomcodeConfigPath()
				raw, err := os.ReadFile(path)
				if err != nil && !os.IsNotExist(err) {
					return err
				}
				ctx, out := modelLimits(model)
				key := agentProviderID + "/" + model
				appendix := "[provider_accounts." + agentProviderID + "]\nprovider = \"openai-compatible\"\nbase_url = " + tomlQuote(gw) +
					"\napi_key = " + tomlQuote(agentProviderID) + "\n\n[models." + tomlQuote(key) + "]\naccount = " + tomlQuote(agentProviderID) +
					"\nmodel = " + tomlQuote(model) + "\ncontext_window = " + itoa(ctx) + "\nmax_tokens = " + itoa(out) + "\nsupports_vision = true\n"
				quoted := tomlQuote(key)
				text, old := editKimiTOML(string(raw), atomcodeOurTable, &quoted, appendix)
				if _, done := prev["default_model"]; !done {
					if old == "" || strings.Contains(old, agentProviderID+"/") {
						prev["default_model"] = json.RawMessage("null")
					} else {
						b, _ := json.Marshal(old)
						prev["default_model"] = b
					}
				}
				return writeAgentFile(path, []byte(text))
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				path := atomcodeConfigPath()
				raw, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				restore := ""
				var old string
				if json.Unmarshal(prev["default_model"], &old) == nil {
					restore = old
				}
				text, _ := editKimiTOML(string(raw), atomcodeOurTable, &restore, "")
				if strings.TrimSpace(text) == "" {
					if _, err := backupFile(path); err != nil {
						return err
					}
					return os.Remove(path)
				}
				return writeAgentFile(path, []byte(text))
			},
			connected: func() (bool, string) {
				raw, err := os.ReadFile(atomcodeConfigPath())
				if err != nil {
					return false, ""
				}
				for _, line := range strings.Split(string(raw), "\n") {
					t := strings.TrimSpace(line)
					if strings.HasPrefix(t, "[") {
						break
					}
					if k, v, ok := strings.Cut(t, "="); ok && strings.TrimSpace(k) == "default_model" {
						ref, ours := strings.CutPrefix(strings.Trim(strings.TrimSpace(v), `"'`), agentProviderID+"/")
						return ours, ref
					}
				}
				return false, ""
			},
		},
		qoderAgent("qoder", "Qoder CLI", "qodercli", "QODER_CONFIG_DIR", ".qoder"),
		qoderAgent("qoder-cn", "Qoder CN", "qoderclicn", "QODERCN_CONFIG_DIR", ".qoder-cn"),
		{
			// MiMo Code（小米）沿用 OpenCode 的配置形状
			id: "mimocode", name: "MiMo Code", command: "mimo",
			paths: func() []string { return []string{openCodeStyleFile(mimocodeDir(), "mimocode")} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				path := openCodeStyleFile(mimocodeDir(), "mimocode")
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				stashJSON(prev, "model", doc, "model")
				ctx, out := modelLimits(model)
				jsonSet(doc, "provider."+agentProviderID, map[string]any{
					"npm": "@ai-sdk/openai-compatible", "name": "AI ENV",
					"options": map[string]any{"baseURL": gw, "apiKey": agentProviderID},
					"models":  map[string]any{model: map[string]any{"name": model, "limit": map[string]any{"context": ctx, "output": out}}},
				})
				jsonSet(doc, "model", agentProviderID+"/"+model)
				return writeJSONDoc(path, doc)
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				path := openCodeStyleFile(mimocodeDir(), "mimocode")
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				jsonDelete(doc, "provider."+agentProviderID)
				restoreJSON(prev, "model", doc, "model")
				return writeJSONDocOrRemove(path, doc)
			},
			connected: func() (bool, string) {
				doc, err := readJSONDoc(openCodeStyleFile(mimocodeDir(), "mimocode"))
				if err != nil {
					return false, ""
				}
				ref, ok := strings.CutPrefix(asString(doc["model"]), agentProviderID+"/")
				return ok, ref
			},
		},
		piLikeAgent("omo", "OmO (oh-my-openagent)", "omo", omoDir),
		{
			// fx（Vercel Labs）：自有服务商 aienv，不带鉴权，Chat Completions 协议
			id: "fx", name: "fx", command: "fx",
			paths: func() []string { return []string{filepath.Join(homeDir(), ".fx", "settings.json")} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				path := filepath.Join(homeDir(), ".fx", "settings.json")
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				stashJSON(prev, "provider", doc, "provider")
				ctx, out := modelLimits(model)
				jsonSet(doc, "providers."+agentProviderID, map[string]any{
					"protocol": "openai-chat-completions", "base_url": gw, "auth": map[string]any{"type": "none"},
					"tool_choice_mode": "send",
					"model_metadata": map[string]any{model: map[string]any{
						"supports_tool_use": true, "supports_vision": true, "context_window": ctx, "max_output_tokens": out}},
				})
				jsonSet(doc, "provider", agentProviderID)
				jsonSet(doc, "models."+agentProviderID, model)
				return writeJSONDoc(path, doc)
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				path := filepath.Join(homeDir(), ".fx", "settings.json")
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				jsonDelete(doc, "providers."+agentProviderID)
				jsonDelete(doc, "models."+agentProviderID)
				restoreJSON(prev, "provider", doc, "provider")
				return writeJSONDocOrRemove(path, doc)
			},
			connected: func() (bool, string) {
				doc, err := readJSONDoc(filepath.Join(homeDir(), ".fx", "settings.json"))
				if err != nil || doc["provider"] != agentProviderID {
					return false, ""
				}
				m, _ := jsonGet(doc, "models."+agentProviderID)
				return true, asString(m)
			},
		},
		{
			// T3 Code：一个运行 Claude Code 的服务商实例，环境变量指向本机网关
			id: "t3code", name: "T3 Code", command: "t3code",
			note:  "在 T3 Code 里出现名为 AI ENV 的 Claude Code 服务商，模型在它的选择器里选",
			paths: func() []string { return []string{t3SettingsPath()} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				doc, err := readJSONDoc(t3SettingsPath())
				if err != nil {
					return err
				}
				config := map[string]any{"customModels": []any{map[string]any{"slug": model, "name": "AI ENV · " + model}}}
				for _, k := range []string{"binaryPath", "homePath"} {
					if v, ok := jsonGet(doc, "providerInstances.claudeAgent.config."+k); ok && strings.TrimSpace(asString(v)) != "" {
						config[k] = v
					}
				}
				jsonSet(doc, "providerInstances."+agentProviderID, map[string]any{
					"driver": "claudeAgent", "displayName": "AI ENV", "enabled": true,
					"environment": []any{
						map[string]any{"name": "ANTHROPIC_BASE_URL", "value": gatewayRoot(gw), "sensitive": false},
						map[string]any{"name": "ANTHROPIC_AUTH_TOKEN", "value": agentProviderID, "sensitive": false},
					},
					"config": config,
				})
				return writeJSONDoc(t3SettingsPath(), doc)
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				doc, err := readJSONDoc(t3SettingsPath())
				if err != nil {
					return err
				}
				jsonDelete(doc, "providerInstances."+agentProviderID)
				return writeJSONDocOrRemove(t3SettingsPath(), doc)
			},
			connected: func() (bool, string) {
				doc, err := readJSONDoc(t3SettingsPath())
				if err != nil {
					return false, ""
				}
				v, ok := jsonGet(doc, "providerInstances."+agentProviderID+".config.customModels")
				if !ok {
					return false, ""
				}
				if list, _ := v.([]any); len(list) > 0 {
					m, _ := list[0].(map[string]any)
					return true, asString(m["slug"])
				}
				return true, ""
			},
		},
		{
			// Muse Code：endpoint_transport 指向网关、不带鉴权；模型列表由网关的 /muse-code/models 提供
			id: "muse", name: "Muse Code", command: "muse",
			paths: func() []string { return []string{filepath.Join(xdgConfigDir(), "muse", "settings.json")} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				path := filepath.Join(xdgConfigDir(), "muse", "settings.json")
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				stashJSON(prev, "transport", doc, "endpoint_transport")
				stashJSON(prev, "model", doc, "model")
				if _, ok := doc["schema_version"]; !ok {
					doc["schema_version"] = 1
				}
				doc["endpoint_transport"] = map[string]any{"base_url": gw, "auth": "none"}
				doc["model"] = model
				return writeJSONDoc(path, doc)
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				path := filepath.Join(xdgConfigDir(), "muse", "settings.json")
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				restoreJSON(prev, "transport", doc, "endpoint_transport")
				restoreJSON(prev, "model", doc, "model")
				return writeJSONDoc(path, doc)
			},
			connected: func() (bool, string) {
				doc, err := readJSONDoc(filepath.Join(xdgConfigDir(), "muse", "settings.json"))
				if err != nil {
					return false, ""
				}
				base, _ := jsonGet(doc, "endpoint_transport.base_url")
				if !strings.Contains(asString(base), "/agent-muse/") {
					return false, ""
				}
				return true, asString(doc["model"])
			},
		},
		{
			// Hermes Agent（Nous Research）：providers.aienv + model.provider / model.default
			id: "hermes", name: "Hermes Agent", command: "hermes",
			paths: func() []string { return []string{hermesConfigPath()} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				d, err := loadYAMLDoc(hermesConfigPath())
				if err != nil {
					return err
				}
				d.stash(prev, "provider", "model.provider")
				d.stash(prev, "model", "model.default")
				if err := d.set("providers."+agentProviderID, map[string]any{
					"name": agentProviderID, "base_url": gw, "api_key": agentProviderID, "api_mode": "chat_completions",
					"models": []string{model},
				}); err != nil {
					return err
				}
				_ = d.set("model.provider", agentProviderID)
				_ = d.set("model.default", model)
				return d.save()
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				d, err := loadYAMLDoc(hermesConfigPath())
				if err != nil {
					return err
				}
				d.del("providers." + agentProviderID)
				if err := d.restore(prev, "provider", "model.provider"); err != nil {
					return err
				}
				if err := d.restore(prev, "model", "model.default"); err != nil {
					return err
				}
				return d.save()
			},
			connected: func() (bool, string) {
				d, err := loadYAMLDoc(hermesConfigPath())
				if err != nil || d.str("model.provider") != agentProviderID {
					return false, ""
				}
				return true, d.str("model.default")
			},
		},
		{
			// MiniMax Code（mcode 与桌面端）：custom_provider.aienv（Anthropic 协议）+ defaultModel
			id: "minimax", name: "MiniMax Code", command: "mcode",
			paths: func() []string { return []string{minimaxConfigPath()} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				d, err := loadYAMLDoc(minimaxConfigPath())
				if err != nil {
					return err
				}
				d.stash(prev, "model", "defaultModel")
				d.stash(prev, "variant", "defaultModelVariant")
				ctx, out := modelLimits(model)
				if err := d.set("custom_provider."+agentProviderID, map[string]any{
					"name": "AI ENV", "kind": "custom", "enabled": true, "api": "anthropic-messages",
					"options": map[string]any{"apiKey": agentProviderID, "baseURL": gatewayRoot(gw), "authMode": "api-key"},
					"models":  map[string]any{model: map[string]any{"name": model, "limit": map[string]any{"context": ctx, "output": out}}},
				}); err != nil {
					return err
				}
				_ = d.set("defaultModel", "custom_provider:"+agentProviderID+"/"+model)
				d.del("defaultModelVariant")
				return d.save()
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				d, err := loadYAMLDoc(minimaxConfigPath())
				if err != nil {
					return err
				}
				d.del("custom_provider." + agentProviderID)
				if err := d.restore(prev, "model", "defaultModel"); err != nil {
					return err
				}
				if err := d.restore(prev, "variant", "defaultModelVariant"); err != nil {
					return err
				}
				return d.save()
			},
			connected: func() (bool, string) {
				d, err := loadYAMLDoc(minimaxConfigPath())
				if err != nil {
					return false, ""
				}
				ref, ok := strings.CutPrefix(d.str("defaultModel"), "custom_provider:"+agentProviderID+"/")
				return ok, ref
			},
		},
		{
			// Mister Morph：没有服务商列表，接管 llm 下的几个键，按 Responses 协议请求网关
			id: "morph", name: "Mister Morph", command: "morph",
			paths: func() []string { return []string{morphConfigPath()} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				d, err := loadYAMLDoc(morphConfigPath())
				if err != nil {
					return err
				}
				for _, k := range []string{"inference_provider", "endpoint", "api_key", "model", "context_window_tokens"} {
					d.stash(prev, k, "llm."+k)
				}
				ctx, _ := modelLimits(model)
				_ = d.set("llm.inference_provider", "openai_response_compatible")
				_ = d.set("llm.endpoint", gw)
				_ = d.set("llm.api_key", agentProviderID)
				_ = d.set("llm.model", model)
				_ = d.set("llm.context_window_tokens", ctx)
				return d.save()
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				d, err := loadYAMLDoc(morphConfigPath())
				if err != nil {
					return err
				}
				for _, k := range []string{"inference_provider", "endpoint", "api_key", "model", "context_window_tokens"} {
					if err := d.restore(prev, k, "llm."+k); err != nil {
						return err
					}
				}
				return d.save()
			},
			connected: func() (bool, string) {
				d, err := loadYAMLDoc(morphConfigPath())
				if err != nil || !strings.Contains(d.str("llm.endpoint"), "/agent-morph/") {
					return false, ""
				}
				return true, d.str("llm.model")
			},
		},
		{
			// omp（oh-my-pi）：models.yml 里的服务商 aienv（不带鉴权）+ config.yml 的 modelRoles.default
			id: "omp", name: "omp (oh-my-pi)", command: "omp",
			paths: func() []string {
				return []string{filepath.Join(ompDir(), "config.yml"), filepath.Join(ompDir(), "models.yml")}
			},
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				models, err := loadYAMLDoc(filepath.Join(ompDir(), "models.yml"))
				if err != nil {
					return err
				}
				cfg, err := loadYAMLDoc(filepath.Join(ompDir(), "config.yml"))
				if err != nil {
					return err
				}
				ctx, out := modelLimits(model)
				if err := models.set("providers."+agentProviderID, map[string]any{
					"baseUrl": gw, "api": "openai-completions", "auth": "none",
					"models": []any{map[string]any{"id": model, "name": model, "reasoning": false, "contextWindow": ctx, "maxTokens": out}},
				}); err != nil {
					return err
				}
				cfg.stash(prev, "role", "modelRoles.default")
				_ = cfg.set("modelRoles.default", agentProviderID+"/"+model)
				if err := models.save(); err != nil {
					return err
				}
				return cfg.save()
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				cfg, err := loadYAMLDoc(filepath.Join(ompDir(), "config.yml"))
				if err != nil {
					return err
				}
				if err := cfg.restore(prev, "role", "modelRoles.default"); err != nil {
					return err
				}
				if err := cfg.save(); err != nil {
					return err
				}
				models, err := loadYAMLDoc(filepath.Join(ompDir(), "models.yml"))
				if err != nil {
					return err
				}
				models.del("providers." + agentProviderID)
				return models.save()
			},
			connected: func() (bool, string) {
				cfg, err := loadYAMLDoc(filepath.Join(ompDir(), "config.yml"))
				if err != nil {
					return false, ""
				}
				ref, ok := strings.CutPrefix(cfg.str("modelRoles.default"), agentProviderID+"/")
				return ok, ref
			},
		},
	}
}

func atomcodeOurTable(h string) bool {
	return h == "provider_accounts."+agentProviderID || strings.HasPrefix(h, `models."`+agentProviderID+"/") || strings.HasPrefix(h, `models.'`+agentProviderID+"/")
}

func itoa(n int) string { return strconv.Itoa(n) }

// serveMuseModels Muse Code 在网关主机的 /muse-code/models 读模型列表（不在路由路径下）
func (rs *RouterService) serveMuseModels(w http.ResponseWriter) {
	data := []any{}
	if route, ok := rs.findRoute(agentRouteName("muse")); ok {
		for _, id := range routeModelIDs(route) {
			ctx, out := modelLimits(id)
			data = append(data, map[string]any{"id": id, "object": "model", "created": 0, "owned_by": "aienv",
				"metadata": map[string]any{"muse-code": map[string]any{
					"name": id, "family": "aienv", "is_hidden": false, "attachment": true, "reasoning": false,
					"temperature": false, "tool_call": true,
					"modalities": map[string]any{"input": []string{"text", "image"}, "output": []string{"text"}},
					"options":    map[string]any{"include": []string{}}, "variants": map[string]any{},
					"limit":       map[string]any{"context": ctx, "output": out},
					"description": id + " via AI ENV",
				}}})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}
