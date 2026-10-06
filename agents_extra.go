package main

// 更多 Agent 接入：把 Crush、Kimi Code、Droid、Pi、Cline CLI、Qwen Code 指向本机网关。
// 每个 Agent 使用一条独立路由（agent-<id>），上游取自所选环境，协议由网关转换；
// 写入前保存历史快照与 .bak，断开时删除本程序写入的条目并恢复原来的默认模型。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/titanous/json5"
)

const agentProviderID = "aienv"

// ExtraAgentStatus 一个 Agent 的接入状态
type ExtraAgentStatus struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Installed   bool   `json:"installed"`
	ConfigPath  string `json:"config_path"`
	Connected   bool   `json:"connected"`
	Provider    string `json:"provider,omitempty"`
	Environment string `json:"environment,omitempty"`
	Model       string `json:"model,omitempty"`
	Note        string `json:"note,omitempty"`
}

type agentLink struct {
	Provider    string                     `json:"provider"`
	Environment string                     `json:"environment"`
	Model       string                     `json:"model"`
	Previous    map[string]json.RawMessage `json:"previous,omitempty"` // 被替换的原值，null 表示原来没有
}

type extraAgent struct {
	id, name, command, note string
	paths                   func() []string // 第一个是主配置文件
	connect                 func(gw, model string, prev map[string]json.RawMessage) error
	disconnect              func(prev map[string]json.RawMessage) error
	connected               func() (bool, string)
	warn                    func() string // 可能让接入不生效的情况，显示在说明里
}

var agentStashMu sync.Mutex

func loadAgentStash() map[string]agentLink {
	out := map[string]agentLink{}
	if p, err := storePath("agents.json"); err == nil {
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &out)
		}
	}
	return out
}

func saveAgentStash(m map[string]agentLink) error {
	p, err := storePath("agents.json")
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(p, b, 0o600)
}

// ===== JSON / TOML 文件编辑 =====

func readJSONDoc(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return map[string]any{}, nil
	}
	doc := map[string]any{}
	if err := json.Unmarshal(b, &doc); err != nil {
		// 带注释的 JSONC：宽松解析，写回时注释会丢失（写入前已备份）
		if err5 := json5.Unmarshal(b, &doc); err5 != nil {
			return nil, fmt.Errorf("%s 不是有效的 JSON，未做任何修改: %v", path, err)
		}
	}
	return doc, nil
}

func writeAgentFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_ = captureBeforeWrite(path, data)
	if _, err := backupFile(path); err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o600)
}

func writeJSONDoc(path string, doc map[string]any) error {
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return writeAgentFile(path, append(b, '\n'))
}

func jsonGet(doc map[string]any, path string) (any, bool) {
	var cur any = doc
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func jsonSet(doc map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	cur := doc
	for _, part := range parts[:len(parts)-1] {
		next, ok := cur[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[part] = next
		}
		cur = next
	}
	cur[parts[len(parts)-1]] = value
}

// jsonDelete 删除路径上的键；删除后变空的父对象一并移除
func jsonDelete(doc map[string]any, path string) {
	parts := strings.Split(path, ".")
	chain := []map[string]any{doc}
	cur := doc
	for _, part := range parts[:len(parts)-1] {
		next, ok := cur[part].(map[string]any)
		if !ok {
			return
		}
		chain = append(chain, next)
		cur = next
	}
	delete(cur, parts[len(parts)-1])
	for i := len(chain) - 1; i > 0; i-- {
		if len(chain[i]) > 0 {
			break
		}
		delete(chain[i-1], parts[i-1])
	}
}

// writeJSONDocOrRemove 断开时使用：内容清空则删除文件（与文件不存在等价）
func writeJSONDocOrRemove(path string, doc map[string]any) error {
	if len(doc) == 0 {
		if _, err := backupFile(path); err != nil {
			return err
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeJSONDoc(path, doc)
}

// stashJSON 第一次接入时记下原值（之后重复接入不覆盖最初的原值）
func stashJSON(prev map[string]json.RawMessage, key string, doc map[string]any, path string) {
	if _, done := prev[key]; done {
		return
	}
	if v, ok := jsonGet(doc, path); ok {
		b, _ := json.Marshal(v)
		prev[key] = b
	} else {
		prev[key] = json.RawMessage("null")
	}
}

func restoreJSON(prev map[string]json.RawMessage, key string, doc map[string]any, path string) {
	raw, ok := prev[key]
	if !ok {
		return
	}
	if string(raw) == "null" {
		jsonDelete(doc, path)
		return
	}
	var v any
	if json.Unmarshal(raw, &v) == nil {
		jsonSet(doc, path, v)
	}
}

func modelLimits(model string) (int, int) {
	ctx, out := 128000, 16384
	if m, ok := lookupCatalogModel(model); ok {
		if m.Context > 0 {
			ctx = m.Context
		}
		if m.Output > 0 {
			out = m.Output
		}
	}
	if out > ctx {
		out = ctx
	}
	return ctx, out
}

// tomlTableHeader 是否为某个表头行（[a.b] / [[a]]）
func tomlTableHeader(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "[") {
		return "", false
	}
	if i := strings.Index(t, "#"); i > 0 {
		t = strings.TrimSpace(t[:i])
	}
	t = strings.TrimPrefix(strings.TrimSuffix(t, "]"), "[")
	t = strings.TrimPrefix(strings.TrimSuffix(t, "]"), "[")
	return strings.TrimSpace(t), true
}

// editKimiTOML 删除本程序写入的表，原位替换顶层 default_model（defaultModel 为已格式化的 TOML 值，
// 空串表示删除该行），再追加新的表；其余内容原样保留
func editKimiTOML(text string, drop func(header string) bool, defaultModel *string, appendix string) (string, string) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	skipping := false
	inTop := true
	oldDefault := ""
	defaultDone := false
	for _, line := range lines {
		if header, ok := tomlTableHeader(line); ok {
			if inTop && defaultModel != nil && !defaultDone && *defaultModel != "" {
				out = append(out, "default_model = "+*defaultModel)
				defaultDone = true
			}
			inTop = false
			skipping = drop(header)
			if skipping {
				continue
			}
		}
		if skipping {
			continue
		}
		if inTop {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "default_model") && strings.Contains(t, "=") {
				k, v, _ := strings.Cut(t, "=")
				if strings.TrimSpace(k) == "default_model" {
					oldDefault = strings.TrimSpace(v)
					if defaultModel != nil {
						if *defaultModel != "" && !defaultDone {
							out = append(out, "default_model = "+*defaultModel)
							defaultDone = true
						}
						continue
					}
				}
			}
		}
		out = append(out, line)
	}
	if inTop && defaultModel != nil && !defaultDone && *defaultModel != "" {
		out = append([]string{"default_model = " + *defaultModel}, out...)
	}
	result := strings.TrimRight(strings.Join(out, "\n"), "\n")
	if appendix != "" {
		if result != "" {
			result += "\n\n"
		}
		result += strings.TrimRight(appendix, "\n")
	}
	return result + "\n", oldDefault
}

func tomlQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// ===== 各 Agent =====

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func crushConfigPath() string {
	if goruntime.GOOS == "windows" {
		if app := os.Getenv("LOCALAPPDATA"); app != "" {
			return filepath.Join(app, "crush", "crush.json")
		}
	}
	return filepath.Join(envOr("XDG_CONFIG_HOME", filepath.Join(homeDir(), ".config")), "crush", "crush.json")
}

func kimiConfigDir() string {
	if d := os.Getenv("KIMI_CODE_HOME"); d != "" {
		return d
	}
	code := filepath.Join(homeDir(), ".kimi-code")
	if dirExists(code) {
		return code
	}
	if d := os.Getenv("KIMI_SHARE_DIR"); d != "" {
		return d
	}
	if d := filepath.Join(homeDir(), ".kimi"); dirExists(d) {
		return d
	}
	return code
}

func factoryDir() string {
	if d := os.Getenv("FACTORY_HOME_OVERRIDE"); d != "" {
		return filepath.Join(d, ".factory")
	}
	return filepath.Join(homeDir(), ".factory")
}

func piDir() string {
	return envOr("PI_CODING_AGENT_DIR", filepath.Join(homeDir(), ".pi", "agent"))
}

func clineProvidersPath() string {
	return filepath.Join(envOr("CLINE_DIR", filepath.Join(homeDir(), ".cline")), "data", "settings", "providers.json")
}

func extraAgents() []extraAgent {
	return append([]extraAgent{
		{
			id: "crush", name: "Crush", command: "crush",
			paths: func() []string { return []string{crushConfigPath()} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				path := crushConfigPath()
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				stashJSON(prev, "large", doc, "models.large")
				stashJSON(prev, "small", doc, "models.small")
				ctx, out := modelLimits(model)
				jsonSet(doc, "providers."+agentProviderID, map[string]any{
					"type": "openai", "name": "AI ENV", "base_url": gw, "api_key": agentProviderID,
					"models": []any{map[string]any{"id": model, "name": model, "context_window": ctx, "default_max_tokens": out}},
				})
				pick := map[string]any{"provider": agentProviderID, "model": model}
				jsonSet(doc, "models.large", pick)
				jsonSet(doc, "models.small", pick)
				return writeJSONDoc(path, doc)
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				path := crushConfigPath()
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				jsonDelete(doc, "providers."+agentProviderID)
				restoreJSON(prev, "large", doc, "models.large")
				restoreJSON(prev, "small", doc, "models.small")
				return writeJSONDocOrRemove(path, doc)
			},
			connected: func() (bool, string) {
				doc, err := readJSONDoc(crushConfigPath())
				if err != nil {
					return false, ""
				}
				p, _ := jsonGet(doc, "models.large.provider")
				m, _ := jsonGet(doc, "models.large.model")
				ms, _ := m.(string)
				return p == agentProviderID, ms
			},
		},
		{
			id: "kimi", name: "Kimi Code", command: "kimi",
			paths: func() []string { return []string{filepath.Join(kimiConfigDir(), "config.toml")} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				path := filepath.Join(kimiConfigDir(), "config.toml")
				raw, err := os.ReadFile(path)
				if err != nil && !os.IsNotExist(err) {
					return err
				}
				ctx, _ := modelLimits(model)
				key := agentProviderID + "/" + model
				appendix := fmt.Sprintf("[providers.%s]\ntype = \"kimi\"\nbase_url = %s\napi_key = %s\n\n[models.%s]\nprovider = %s\nmodel = %s\nmax_context_size = %d\n",
					agentProviderID, tomlQuote(gw), tomlQuote(agentProviderID), tomlQuote(key), tomlQuote(agentProviderID), tomlQuote(model), ctx)
				drop := func(h string) bool {
					return h == "providers."+agentProviderID || strings.HasPrefix(h, `models."`+agentProviderID+"/") || strings.HasPrefix(h, `models.'`+agentProviderID+"/")
				}
				quoted := tomlQuote(key)
				text, old := editKimiTOML(string(raw), drop, &quoted, appendix)
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
				path := filepath.Join(kimiConfigDir(), "config.toml")
				raw, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				restore := ""
				var old string
				if json.Unmarshal(prev["default_model"], &old) == nil {
					restore = old // 原样保存的 TOML 值（含引号）
				}
				drop := func(h string) bool {
					return h == "providers."+agentProviderID || strings.HasPrefix(h, `models."`+agentProviderID+"/") || strings.HasPrefix(h, `models.'`+agentProviderID+"/")
				}
				text, _ := editKimiTOML(string(raw), drop, &restore, "")
				return writeAgentFile(path, []byte(text))
			},
			connected: func() (bool, string) {
				raw, err := os.ReadFile(filepath.Join(kimiConfigDir(), "config.toml"))
				if err != nil {
					return false, ""
				}
				for _, line := range strings.Split(string(raw), "\n") {
					t := strings.TrimSpace(line)
					if k, v, ok := strings.Cut(t, "="); ok && strings.TrimSpace(k) == "default_model" {
						v = strings.Trim(strings.TrimSpace(v), `"'`)
						if strings.HasPrefix(v, agentProviderID+"/") {
							return true, strings.TrimPrefix(v, agentProviderID+"/")
						}
						return false, ""
					}
					if strings.HasPrefix(t, "[") {
						break
					}
				}
				return false, ""
			},
		},
		{
			id: "droid", name: "Droid (Factory)", command: "droid",
			paths: func() []string { return []string{filepath.Join(factoryDir(), "settings.json")} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				path := filepath.Join(factoryDir(), "settings.json")
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				stashJSON(prev, "default", doc, "sessionDefaultSettings.model")
				list, _ := doc["customModels"].([]any)
				kept := []any{}
				for _, item := range list {
					if m, ok := item.(map[string]any); ok && strings.HasPrefix(fmt.Sprint(m["id"]), "custom:"+agentProviderID+"/") {
						continue
					}
					kept = append(kept, item)
				}
				_, out := modelLimits(model)
				id := "custom:" + agentProviderID + "/" + model
				kept = append(kept, map[string]any{"id": id, "model": model, "displayName": "AI ENV · " + model, "baseUrl": gw,
					"apiKey": agentProviderID, "provider": "generic-chat-completion-api", "maxOutputTokens": out})
				doc["customModels"] = kept
				jsonSet(doc, "sessionDefaultSettings.model", id)
				return writeJSONDoc(path, doc)
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				path := filepath.Join(factoryDir(), "settings.json")
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				list, _ := doc["customModels"].([]any)
				kept := []any{}
				for _, item := range list {
					if m, ok := item.(map[string]any); ok && strings.HasPrefix(fmt.Sprint(m["id"]), "custom:"+agentProviderID+"/") {
						continue
					}
					kept = append(kept, item)
				}
				doc["customModels"] = kept
				restoreJSON(prev, "default", doc, "sessionDefaultSettings.model")
				if s, ok := doc["sessionDefaultSettings"].(map[string]any); ok && len(s) == 0 {
					delete(doc, "sessionDefaultSettings")
				}
				return writeJSONDocOrRemove(path, doc)
			},
			connected: func() (bool, string) {
				doc, err := readJSONDoc(filepath.Join(factoryDir(), "settings.json"))
				if err != nil {
					return false, ""
				}
				v, _ := jsonGet(doc, "sessionDefaultSettings.model")
				s, _ := v.(string)
				if strings.HasPrefix(s, "custom:"+agentProviderID+"/") {
					return true, strings.TrimPrefix(s, "custom:"+agentProviderID+"/")
				}
				return false, ""
			},
		},
		{
			id: "pi", name: "Pi", command: "pi",
			paths: func() []string {
				return []string{filepath.Join(piDir(), "settings.json"), filepath.Join(piDir(), "models.json")}
			},
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				modelsPath, settingsPath := filepath.Join(piDir(), "models.json"), filepath.Join(piDir(), "settings.json")
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
				modelsPath, settingsPath := filepath.Join(piDir(), "models.json"), filepath.Join(piDir(), "settings.json")
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
				settings, err := readJSONDoc(filepath.Join(piDir(), "settings.json"))
				if err != nil || settings["defaultProvider"] != agentProviderID {
					return false, ""
				}
				m, _ := settings["defaultModel"].(string)
				return true, m
			},
		},
		{
			id: "cline", name: "Cline CLI", command: "cline",
			paths: func() []string { return []string{clineProvidersPath()} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				path := clineProvidersPath()
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				stashJSON(prev, "last", doc, "lastUsedProvider")
				stashJSON(prev, "entry", doc, "providers.openai-compatible")
				if _, ok := doc["version"]; !ok {
					doc["version"] = 1
				}
				jsonSet(doc, "providers.openai-compatible", map[string]any{
					"settings":    map[string]any{"provider": "openai-compatible", "apiKey": agentProviderID, "model": model, "baseUrl": gw},
					"updatedAt":   time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
					"tokenSource": "manual",
				})
				doc["lastUsedProvider"] = "openai-compatible"
				return writeJSONDoc(path, doc)
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				path := clineProvidersPath()
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				restoreJSON(prev, "entry", doc, "providers.openai-compatible")
				restoreJSON(prev, "last", doc, "lastUsedProvider")
				return writeJSONDocOrRemove(path, doc)
			},
			connected: func() (bool, string) {
				doc, err := readJSONDoc(clineProvidersPath())
				if err != nil || doc["lastUsedProvider"] != "openai-compatible" {
					return false, ""
				}
				key, _ := jsonGet(doc, "providers.openai-compatible.settings.apiKey")
				m, _ := jsonGet(doc, "providers.openai-compatible.settings.model")
				ms, _ := m.(string)
				return key == agentProviderID, ms
			},
		},
		{
			id: "qwen", name: "Qwen Code", command: "qwen",
			paths: func() []string { return []string{filepath.Join(homeDir(), ".qwen", "settings.json")} },
			connect: func(gw, model string, prev map[string]json.RawMessage) error {
				path := filepath.Join(homeDir(), ".qwen", "settings.json")
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				stashJSON(prev, "auth", doc, "security.auth.selectedType")
				stashJSON(prev, "model", doc, "model.name")
				list, _ := jsonGet(doc, "modelProviders.openai")
				items, _ := list.([]any)
				kept := []any{}
				for _, item := range items {
					if m, ok := item.(map[string]any); ok && m["envKey"] == "AIENV_GATEWAY_KEY" {
						continue
					}
					kept = append(kept, item)
				}
				ctx, _ := modelLimits(model)
				kept = append([]any{map[string]any{"id": model, "name": "AI ENV · " + model, "baseUrl": gw, "envKey": "AIENV_GATEWAY_KEY",
					"description": "AI ENV 本机网关", "generationConfig": map[string]any{"contextWindowSize": ctx}}}, kept...)
				jsonSet(doc, "modelProviders.openai", kept)
				jsonSet(doc, "env.AIENV_GATEWAY_KEY", agentProviderID)
				jsonSet(doc, "security.auth.selectedType", "openai")
				jsonSet(doc, "model.name", model)
				return writeJSONDoc(path, doc)
			},
			disconnect: func(prev map[string]json.RawMessage) error {
				path := filepath.Join(homeDir(), ".qwen", "settings.json")
				doc, err := readJSONDoc(path)
				if err != nil {
					return err
				}
				list, _ := jsonGet(doc, "modelProviders.openai")
				items, _ := list.([]any)
				kept := []any{}
				for _, item := range items {
					if m, ok := item.(map[string]any); ok && m["envKey"] == "AIENV_GATEWAY_KEY" {
						continue
					}
					kept = append(kept, item)
				}
				if len(kept) == 0 {
					jsonDelete(doc, "modelProviders.openai")
				} else {
					jsonSet(doc, "modelProviders.openai", kept)
				}
				jsonDelete(doc, "env.AIENV_GATEWAY_KEY")
				restoreJSON(prev, "auth", doc, "security.auth.selectedType")
				restoreJSON(prev, "model", doc, "model.name")
				return writeJSONDocOrRemove(path, doc)
			},
			connected: func() (bool, string) {
				doc, err := readJSONDoc(filepath.Join(homeDir(), ".qwen", "settings.json"))
				if err != nil {
					return false, ""
				}
				if _, ok := jsonGet(doc, "env.AIENV_GATEWAY_KEY"); !ok {
					return false, ""
				}
				m, _ := jsonGet(doc, "model.name")
				ms, _ := m.(string)
				return true, ms
			},
		},
	}, moreAgents()...)
}

func findExtraAgent(id string) (extraAgent, bool) {
	for _, a := range extraAgents() {
		if a.id == id {
			return a, true
		}
	}
	return extraAgent{}, false
}

func agentRouteName(id string) string { return "agent-" + id }

// ListExtraAgents 各 Agent 的安装与接入状态
func (w *WorkbenchService) ListExtraAgents() []ExtraAgentStatus {
	agentStashMu.Lock()
	stash := loadAgentStash()
	agentStashMu.Unlock()
	out := []ExtraAgentStatus{}
	for _, a := range extraAgents() {
		paths := a.paths()
		st := ExtraAgentStatus{ID: a.id, Name: a.name, ConfigPath: paths[0], Note: a.note}
		st.Installed = findCliBinary(a.command) != "" || fileExists(paths[0]) || dirExists(filepath.Dir(paths[0]))
		st.Connected, st.Model = a.connected()
		if a.warn != nil {
			if w := a.warn(); w != "" {
				st.Note = strings.TrimSpace(st.Note + "\n⚠ " + w)
			}
		}
		if link, ok := stash[a.id]; ok && st.Connected {
			st.Provider, st.Environment = link.Provider, link.Environment
		}
		out = append(out, st)
	}
	return out
}

// ConnectExtraAgent 把 Agent 指向本机网关：为它建一条路由，上游取自所选环境
func (w *WorkbenchService) ConnectExtraAgent(agentID, provider, environment, model string) error {
	a, ok := findExtraAgent(agentID)
	if !ok {
		return fmt.Errorf("未知的 Agent")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return fmt.Errorf("请选择或填写模型")
	}
	if w.app == nil {
		return fmt.Errorf("应用未就绪")
	}
	env, ok := w.app.findEnvCopy(provider, environment)
	if !ok {
		return fmt.Errorf("环境不存在")
	}
	if env.OfficialLogin {
		return fmt.Errorf("官方登录环境没有可转发的 API 地址，请选择一个 API 环境")
	}
	base, key, _ := upstreamVarsForEnv(&env)
	if _, account := accountKind(base); !account && validEndpoint(base) != nil {
		return fmt.Errorf("该环境没有有效的 Base URL")
	}
	rs := globalRouterService
	if rs == nil {
		return fmt.Errorf("路由服务未就绪")
	}
	route := APIRoute{
		Name:         agentRouteName(a.id),
		Description:  fmt.Sprintf("%s 经 AI ENV 接入（环境 %s/%s）", a.name, env.Provider, env.Name),
		SourceFormat: "openai",
		TargetFormat: targetFormatForEnv(&env),
		BaseURL:      strings.TrimRight(strings.TrimSpace(base), "/"),
		APIKey:       key,
		DefaultModel: model,
		Enabled:      true,
	}
	if err := rs.upsertAutoRoute(route); err != nil {
		return fmt.Errorf("写入路由失败: %v", err)
	}
	cfg := rs.GetRouterConfig()
	if !cfg.AutoStart {
		// Agent 依赖网关，开机后也要能用
		cfg.AutoStart = true
		_ = rs.SaveRouterConfig(cfg)
	}
	if err := rs.StartGateway(); err != nil {
		return fmt.Errorf("启动本机网关失败: %v", err)
	}
	gw := fmt.Sprintf("http://127.0.0.1:%d/%s/v1", routerPort(rs), agentRouteName(a.id))

	agentStashMu.Lock()
	defer agentStashMu.Unlock()
	stash := loadAgentStash()
	link := stash[a.id]
	if link.Previous == nil {
		link.Previous = map[string]json.RawMessage{}
	}
	if err := a.connect(gw, model, link.Previous); err != nil {
		return err
	}
	link.Provider, link.Environment, link.Model = env.Provider, env.Name, model
	stash[a.id] = link
	return saveAgentStash(stash)
}

// DisconnectExtraAgent 删除写入的条目、恢复原默认模型，并移除对应路由
func (w *WorkbenchService) DisconnectExtraAgent(agentID string) error {
	a, ok := findExtraAgent(agentID)
	if !ok {
		return fmt.Errorf("未知的 Agent")
	}
	agentStashMu.Lock()
	defer agentStashMu.Unlock()
	stash := loadAgentStash()
	link := stash[a.id]
	if link.Previous == nil {
		link.Previous = map[string]json.RawMessage{}
	}
	if err := a.disconnect(link.Previous); err != nil {
		return err
	}
	delete(stash, a.id)
	if err := saveAgentStash(stash); err != nil {
		return err
	}
	return removeAutoRouteByName(agentRouteName(a.id))
}
