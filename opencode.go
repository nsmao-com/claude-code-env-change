package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// resolveOpencodeConfigDir 解析 OpenCode 配置目录（默认 ~/.config/opencode）。
// OPENCODE_CONFIG_DIR 与 OpenCode CLI 的官方约定一致，可覆盖目录位置。
func resolveOpencodeConfigDir(vars map[string]string) string {
	home, _ := os.UserHomeDir()
	defaultDir := filepath.Join(home, ".config", "opencode")
	if vars != nil {
		if v := strings.TrimSpace(vars["OPENCODE_CONFIG_DIR"]); v != "" {
			return expandAndNormalizePath(v, home, defaultDir)
		}
	}
	if v := strings.TrimSpace(os.Getenv("OPENCODE_CONFIG_DIR")); v != "" {
		return expandAndNormalizePath(v, home, defaultDir)
	}
	return defaultDir
}

// opencodeConfigFile 解析 OpenCode 配置文件路径（默认 <dir>/opencode.json）。
// OPENCODE_CONFIG 与 OpenCode CLI 的官方约定一致，可指向具体配置文件。
func opencodeConfigFile(vars map[string]string) string {
	home, _ := os.UserHomeDir()
	configDir := resolveOpencodeConfigDir(vars)
	if vars != nil {
		if v := strings.TrimSpace(vars["OPENCODE_CONFIG"]); v != "" {
			return expandAndNormalizePath(v, home, configDir)
		}
	}
	if v := strings.TrimSpace(os.Getenv("OPENCODE_CONFIG")); v != "" {
		return expandAndNormalizePath(v, home, configDir)
	}
	jsonPath := filepath.Join(configDir, "opencode.json")
	jsoncPath := filepath.Join(configDir, "opencode.jsonc")
	if fileExists(jsonPath) {
		return jsonPath
	}
	if fileExists(jsoncPath) {
		return jsoncPath
	}
	return jsonPath
}

func opencodeSkillsRoot() string {
	return filepath.Join(resolveOpencodeConfigDir(nil), "skills")
}

func (a *App) applyOpencodeEnv(env *EnvConfig) (string, error) {
	if env.Variables == nil {
		env.Variables = map[string]string{}
	}
	if strings.TrimSpace(env.Variables["OPENCODE_PROVIDER_ID"]) == "" {
		env.Variables["OPENCODE_PROVIDER_ID"] = opencodeProviderID(env)
	}
	a.persistOpencodeProviderID(env.Name, env.Variables["OPENCODE_PROVIDER_ID"])
	configFile := opencodeConfigFile(env.Variables)
	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return "", fmt.Errorf("创建 OpenCode 配置目录失败: %v", err)
	}

	var content string
	if tmpl := strings.TrimSpace(env.Templates["opencode.json"]); tmpl != "" {
		content = applyOpencodeTemplate(tmpl, env)
	} else {
		content = defaultOpencodeConfig(env)
	}

	if err := mergeWriteOpencodeConfig(configFile, content, env.Variables); err != nil {
		return "", err
	}
	return fmt.Sprintf("OpenCode 配置已应用到 %s", configFile), nil
}

// defaultOpencodeConfig 生成默认 opencode.json：
// 填了 Base URL 时通过 OpenAI 兼容自定义 provider 接入网关，否则直接设置原生 model。
func defaultOpencodeConfig(env *EnvConfig) string {
	model := strings.TrimSpace(env.Variables["OPENCODE_MODEL"])
	baseURL := strings.TrimSpace(env.Variables["OPENCODE_BASE_URL"])
	apiKey := strings.TrimSpace(env.Variables["OPENCODE_API_KEY"])

	payload := map[string]any{
		"$schema": "https://opencode.ai/config.json",
	}

	injectOpencodeExtras(payload, env.Variables)

	providerID := opencodeProviderID(env)
	providerName := strings.TrimSpace(env.Variables["OPENCODE_PROVIDER_NAME"])
	npmPkg := strings.TrimSpace(env.Variables["OPENCODE_NPM"])
	if providerName == "" {
		if env != nil && strings.TrimSpace(env.Name) != "" {
			providerName = strings.TrimSpace(env.Name)
		} else {
			providerName = "Custom"
		}
	}
	if npmPkg == "" {
		npmPkg = "@ai-sdk/openai-compatible"
	}

	if baseURL != "" || apiKey != "" || model != "" {
		modelID := model
		if idx := strings.LastIndex(modelID, "/"); idx >= 0 && modelID[:idx] == providerID {
			modelID = modelID[idx+1:]
		} else if idx := strings.LastIndex(modelID, "/"); idx >= 0 && providerID == "custom" {
			modelID = modelID[idx+1:]
		}
		if modelID == "" {
			modelID = "custom-model"
		}
		payload["model"] = providerID + "/" + modelID

		options := map[string]any{}
		if baseURL != "" {
			options["baseURL"] = baseURL
		}
		if apiKey != "" {
			options["apiKey"] = apiKey
		}
		models := map[string]any{}
		ids := []string{}
		for _, id := range strings.Split(env.Variables["OPENCODE_MODELS"], ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			ids = []string{modelID}
		}
		for _, id := range ids {
			models[id] = map[string]any{"name": id}
		}
		entry := map[string]any{
			"npm":     npmPkg,
			"name":    providerName,
			"models":  models,
		}
		if len(options) > 0 {
			entry["options"] = options
		}
		payload["provider"] = map[string]any{providerID: entry}
	} else if model != "" {
		payload["model"] = model
	}

	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data)
}

func applyOpencodeTemplate(tmpl string, env *EnvConfig) string {
	out := tmpl
	repl := map[string]string{
		"{{OPENCODE_MODEL}}":    env.Variables["OPENCODE_MODEL"],
		"{{OPENCODE_BASE_URL}}": env.Variables["OPENCODE_BASE_URL"],
		"{{OPENCODE_API_KEY}}":  env.Variables["OPENCODE_API_KEY"],
		"{{model}}":             env.Variables["OPENCODE_MODEL"],
		"{{base_url}}":          env.Variables["OPENCODE_BASE_URL"],
		"{{api_key}}":           env.Variables["OPENCODE_API_KEY"],
	}
	for k, v := range repl {
		out = strings.ReplaceAll(out, k, v)
	}
	return out
}

// mergeWriteOpencodeConfig 把这一套 provider 合并进 opencode.json。
// 其它已经存在的 provider / mcp / agent 原样保留，所以可以同时应用多套 OpenCode 配置。
func mergeWriteOpencodeConfig(configFile, incoming string, vars map[string]string) error {
	desired, err := parseJSONLikeObject([]byte(incoming))
	if err != nil {
		return fmt.Errorf("解析待写入的 OpenCode 配置失败: %v", err)
	}

	existing := map[string]any{}
	if data, err := os.ReadFile(configFile); err == nil && len(data) > 0 {
		parsed, parseErr := parseJSONLikeObject(data)
		if parseErr != nil {
			return fmt.Errorf("读取 OpenCode 配置失败: %v", parseErr)
		}
		existing = parsed
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取 OpenCode 配置失败: %v", err)
	}

	existProv, _ := existing["provider"].(map[string]any)
	if existProv == nil {
		existProv = map[string]any{}
	}
	desiredProv, _ := desired["provider"].(map[string]any)
	for key, value := range desired {
		if key == "provider" {
			continue
		}
		existing[key] = value
	}
	for id, entry := range desiredProv {
		existProv[id] = entry
	}
	existing["provider"] = existProv
	injectOpencodeExtras(existing, vars)
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 OpenCode 配置失败: %v", err)
	}
	return os.WriteFile(configFile, data, 0644)
}

func opencodeProviderID(env *EnvConfig) string {
	if env != nil && env.Variables != nil {
		if id := strings.TrimSpace(env.Variables["OPENCODE_PROVIDER_ID"]); id != "" {
			return id
		}
	}
	name := ""
	if env != nil {
		name = env.Name
	}
	return slugOpencodeProviderID(name)
}

func slugOpencodeProviderID(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		if (unicode.IsLetter(r) && r < 128) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if b.Len() > 0 && !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" || s == "custom" {
		h := fnv.New32a()
		_, _ = h.Write([]byte(name))
		return fmt.Sprintf("oc-%x", h.Sum32())
	}
	if len(s) > 32 {
		s = s[:32]
	}
	return s
}

func (a *App) persistOpencodeProviderID(name, id string) {
	name = strings.TrimSpace(name)
	id = strings.TrimSpace(id)
	if name == "" || id == "" {
		return
	}
	for i := range a.config.Environments {
		if a.config.Environments[i].Name != name {
			continue
		}
		if a.config.Environments[i].Variables == nil {
			a.config.Environments[i].Variables = map[string]string{}
		}
		if strings.TrimSpace(a.config.Environments[i].Variables["OPENCODE_PROVIDER_ID"]) == "" {
			a.config.Environments[i].Variables["OPENCODE_PROVIDER_ID"] = id
		}
		return
	}
}

func (a *App) opencodeDiskProviderIDs() map[string]bool {
	files := map[string]struct{}{}
	files[opencodeConfigFile(nil)] = struct{}{}
	for i := range a.config.Environments {
		env := &a.config.Environments[i]
		if !strings.EqualFold(strings.TrimSpace(env.Provider), "opencode") {
			continue
		}
		files[opencodeConfigFile(env.Variables)] = struct{}{}
	}
	ids := map[string]bool{}
	parsed := false
	for file := range files {
		data, err := os.ReadFile(file)
		if err != nil || len(data) == 0 {
			continue
		}
		payload, err := parseJSONLikeObject(data)
		if err != nil {
			continue
		}
		parsed = true
		for id := range opencodeProviderMap(payload) {
			id = strings.TrimSpace(id)
			if id != "" {
				ids[id] = true
			}
		}
	}
	if !parsed {
		return nil
	}
	return ids
}

func opencodeEnvMatchesDisk(env *EnvConfig, ids map[string]bool) bool {
	if env == nil || len(ids) == 0 {
		return false
	}
	if id := opencodeProviderID(env); id != "" && ids[id] {
		return true
	}
	model := ""
	if env.Variables != nil {
		model = strings.TrimSpace(env.Variables["OPENCODE_MODEL"])
	}
	if idx := strings.Index(model, "/"); idx > 0 {
		if ids[strings.TrimSpace(model[:idx])] {
			return true
		}
	}
	return false
}

// syncOpencodeAppliedFromDisk 把已经写进 opencode.json 的 provider 也标成使用中，
// 这样多套共存不依赖只存最后一个名字的旧字段。
func (a *App) syncOpencodeAppliedFromDisk() {
	a.normalizeOpencodeCurrents()
	ids := a.opencodeDiskProviderIDs()
	if ids == nil {
		return
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(a.config.CurrentEnvsOpencode)+4)
	last := strings.TrimSpace(a.config.CurrentEnvOpencode)
	for _, name := range a.config.CurrentEnvsOpencode {
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	for i := range a.config.Environments {
		env := &a.config.Environments[i]
		if !strings.EqualFold(strings.TrimSpace(env.Provider), "opencode") || seen[env.Name] {
			continue
		}
		if !opencodeEnvMatchesDisk(env, ids) {
			continue
		}
		seen[env.Name] = true
		out = append(out, env.Name)
	}
	a.config.CurrentEnvsOpencode = out
	if last != "" && seen[last] {
		a.config.CurrentEnvOpencode = last
		return
	}
	if len(out) == 0 {
		a.config.CurrentEnvOpencode = ""
		return
	}
	a.config.CurrentEnvOpencode = out[len(out)-1]
}

func (a *App) opencodeCurrentNames() []string {
	a.normalizeOpencodeCurrents()
	if len(a.config.CurrentEnvsOpencode) > 0 {
		return append([]string(nil), a.config.CurrentEnvsOpencode...)
	}
	if strings.TrimSpace(a.config.CurrentEnvOpencode) != "" {
		return []string{a.config.CurrentEnvOpencode}
	}
	return []string{}
}

func (a *App) isOpencodeCurrent(name string) bool {
	for _, item := range a.opencodeCurrentNames() {
		if item == name {
			return true
		}
	}
	return false
}

func (a *App) normalizeOpencodeCurrents() {
	seen := map[string]bool{}
	out := make([]string, 0, len(a.config.CurrentEnvsOpencode)+1)
	for _, name := range a.config.CurrentEnvsOpencode {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] || a.findEnv(name) == nil {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	if cur := strings.TrimSpace(a.config.CurrentEnvOpencode); cur != "" && !seen[cur] && a.findEnv(cur) != nil {
		out = append(out, cur)
	}
	a.config.CurrentEnvsOpencode = out
	if len(out) == 0 {
		a.config.CurrentEnvOpencode = ""
		return
	}
	a.config.CurrentEnvOpencode = out[len(out)-1]
}

func (a *App) addOpencodeCurrent(name string) {
	a.normalizeOpencodeCurrents()
	out := make([]string, 0, len(a.config.CurrentEnvsOpencode)+1)
	for _, item := range a.config.CurrentEnvsOpencode {
		if item != name {
			out = append(out, item)
		}
	}
	out = append(out, name)
	a.config.CurrentEnvsOpencode = out
	a.config.CurrentEnvOpencode = name
}

func (a *App) removeOpencodeCurrent(name string) {
	a.normalizeOpencodeCurrents()
	out := a.config.CurrentEnvsOpencode[:0]
	for _, item := range a.config.CurrentEnvsOpencode {
		if item != name {
			out = append(out, item)
		}
	}
	a.config.CurrentEnvsOpencode = out
	if len(out) == 0 {
		a.config.CurrentEnvOpencode = ""
		return
	}
	a.config.CurrentEnvOpencode = out[len(out)-1]
}

func (a *App) renameOpencodeCurrent(oldName, newName string) {
	if oldName == newName {
		return
	}
	for i, item := range a.config.CurrentEnvsOpencode {
		if item == oldName {
			a.config.CurrentEnvsOpencode[i] = newName
		}
	}
	a.normalizeOpencodeCurrents()
}

func (a *App) stripOpencodeProvider(env *EnvConfig) error {
	if env == nil {
		return nil
	}
	configFile := opencodeConfigFile(env.Variables)
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	payload, err := parseJSONLikeObject(data)
	if err != nil {
		return fmt.Errorf("解析 OpenCode 配置失败: %v", err)
	}
	id := opencodeProviderID(env)
	if providers, ok := payload["provider"].(map[string]any); ok && providers != nil {
		delete(providers, id)
		payload["provider"] = providers
	}
	if model, ok := payload["model"].(string); ok {
		prefix := id + "/"
		if model == id || strings.HasPrefix(model, prefix) {
			delete(payload, "model")
		}
	}
	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, out, 0644)
}

func (a *App) GetOpencodeSettings() map[string]string {
	vars := map[string]string{}
	if env := a.findEnv(a.config.CurrentEnvOpencode); env != nil {
		vars = env.Variables
	}
	configFile := opencodeConfigFile(vars)
	result := map[string]string{
		"OPENCODE_CONFIG_DIR": resolveOpencodeConfigDir(vars),
		"OPENCODE_CONFIG":     configFile,
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		// 文件不存在时回退到当前激活的 OpenCode 环境变量
		for _, key := range []string{"OPENCODE_BASE_URL", "OPENCODE_MODEL", "OPENCODE_API_KEY"} {
			if v := strings.TrimSpace(vars[key]); v != "" {
				result[key] = v
			}
		}
		return result
	}

	payload, err := parseJSONLikeObject(data)
	if err != nil {
		result["OPENCODE_CONFIG_PARSE_ERROR"] = err.Error()
		return result
	}

	if v, ok := payload["model"].(string); ok {
		result["OPENCODE_MODEL"] = strings.TrimSpace(v)
	}
	providers := opencodeProviderMap(payload)
	if len(providers) == 0 {
		return result
	}
	chosenID := "custom"
	if model := result["OPENCODE_MODEL"]; model != "" {
		if idx := strings.Index(model, "/"); idx > 0 {
			chosenID = model[:idx]
		}
	}
	raw, ok := providers[chosenID].(map[string]any)
	if !ok {
		ids := make([]string, 0, len(providers))
		for id := range providers {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if m, good := providers[id].(map[string]any); good {
				chosenID = id
				raw = m
				break
			}
		}
	}
	if raw == nil {
		return result
	}
	result["OPENCODE_PROVIDER_ID"] = chosenID
	if v := strings.TrimSpace(asString(raw["name"])); v != "" {
		result["OPENCODE_PROVIDER_NAME"] = v
	}
	if v := strings.TrimSpace(asString(raw["npm"])); v != "" {
		result["OPENCODE_NPM"] = v
	}
	if options, ok := raw["options"].(map[string]any); ok && options != nil {
		if v, ok := options["baseURL"].(string); ok {
			result["OPENCODE_BASE_URL"] = strings.TrimSpace(v)
		}
		if v, ok := options["apiKey"].(string); ok {
			result["OPENCODE_API_KEY"] = strings.TrimSpace(v)
		}
	}
	if ids := opencodeProviderModelIDs(raw["models"]); len(ids) > 0 {
		result["OPENCODE_MODELS"] = strings.Join(ids, ",")
		if result["OPENCODE_MODEL"] == "" {
			result["OPENCODE_MODEL"] = chosenID + "/" + ids[0]
		}
	}
	return result
}

// ClearOpencodeSettings 仅清除本应用写入的 model / provider.custom 字段，保留其他配置
func (a *App) ClearOpencodeSettings() error {
	vars := map[string]string{}
	if env := a.findEnv(a.config.CurrentEnvOpencode); env != nil {
		vars = env.Variables
	}
	configFile := opencodeConfigFile(vars)

	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			a.config.CurrentEnvOpencode = ""
			a.config.CurrentEnvsOpencode = nil
			return a.saveConfig()
		}
		return err
	}

	payload, err := parseJSONLikeObject(data)
	if err != nil {
		return fmt.Errorf("解析 OpenCode 配置失败: %v", err)
	}

	changed := false
	if _, ok := payload["model"]; ok {
		delete(payload, "model")
		changed = true
	}
	if providers, ok := payload["provider"].(map[string]any); ok && providers != nil {
		for _, name := range a.opencodeCurrentNames() {
			if env := a.findEnv(name); env != nil {
				id := opencodeProviderID(env)
				if _, ok := providers[id]; ok {
					delete(providers, id)
					changed = true
				}
			}
		}
		if _, ok := providers["custom"]; ok {
			delete(providers, "custom")
			changed = true
		}
		payload["provider"] = providers
	}
	a.config.CurrentEnvOpencode = ""
	a.config.CurrentEnvsOpencode = nil
	if err := a.saveConfig(); err != nil {
		return err
	}
	if !changed {
		return nil
	}

	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, out, 0644)
}
