package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestHistoryRedactionAndStaleRestore(t *testing.T) {
	home := withHomeRoot(t)
	p := filepath.Join(home, "config.json")
	t.Setenv("CLAUDIA_CONFIG_PATH", p)
	a := NewApp()
	if e := a.AddEnv(EnvConfig{Name: "test", Provider: "codex", Variables: map[string]string{"OPENAI_API_KEY": "secret-one"}, Templates: map[string]string{"config.toml": "experimental_bearer_token = \"secret-template\""}}); e != nil {
		t.Fatal(e)
	}
	if e := a.AddEnv(EnvConfig{Name: "test", Provider: "codex", Variables: map[string]string{"OPENAI_API_KEY": "secret-two"}}); e != nil {
		t.Fatal(e)
	}
	history, e := a.ListConfigHistory()
	if e != nil || len(history) == 0 {
		t.Fatal("history absent", e)
	}
	preview, e := a.PreviewConfigHistory(history[0].ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range preview.Changes {
		if strings.Contains(c.Before+c.After, "secret-one") || strings.Contains(c.After, "secret-template") {
			t.Fatal("secret leaked in preview")
		}
	}
	if e = a.AddEnv(EnvConfig{Name: "changed", Provider: "claude", Variables: map[string]string{}}); e != nil {
		t.Fatal(e)
	}
	if e = a.RestoreConfigHistory(history[0].ID, preview.Token); e == nil {
		t.Fatal("stale preview restored")
	}
}
func TestUniversalProviderCollisionIsAtomic(t *testing.T) {
	home := withHomeRoot(t)
	t.Setenv("CLAUDIA_CONFIG_PATH", filepath.Join(home, "config.json"))
	a := NewApp()
	if e := a.AddEnv(EnvConfig{Name: "shared", Provider: "codex", Variables: map[string]string{}}); e != nil {
		t.Fatal(e)
	}
	w := NewWorkbenchService(a, nil, nil, nil)
	_, e := w.SaveUniversalProvider(UniversalProvider{Name: "shared", BaseURL: "https://api.example.com/v1", APIKey: "fixture", Format: "responses", Providers: []string{"claude", "codex"}})
	if e == nil {
		t.Fatal("collision accepted")
	}
	if a.findEnvIn("claude", "shared") != nil {
		t.Fatal("failed operation partially modified config")
	}
}
func TestCodexMixedProfileRoundTripHasNoDrift(t *testing.T) {
	home := withHomeRoot(t)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("CLAUDIA_CONFIG_PATH", filepath.Join(home, "config.json"))
	a := NewApp()
	env := EnvConfig{Name: "profile", Provider: "codex", Variables: map[string]string{"base_url": "http://127.0.0.1:19871/v1", "OPENAI_API_KEY": "fixture", "model": "fixture", "AI_ENV_AUTH_MODE": "mixed", "model_context_window": "128000", "model_auto_compact_token_limit": "96000"}}
	if e := a.AddEnv(env); e != nil {
		t.Fatal(e)
	}
	if _, e := a.ApplyEnv(env.Name, env.Provider); e != nil {
		t.Fatal(e)
	}
	if got := a.GetConfigDrift(); len(got) > 0 {
		t.Fatalf("false drift after applying: %v", got)
	}
	read := a.GetCodexSettings()
	if read["OPENAI_API_KEY"] != "fixture" || read["model_auto_compact_token_limit"] != "96000" {
		t.Fatal("provider-scoped settings lost")
	}
}
func TestCCSwitchLinkImportSupportsModelsAndEndpoints(t *testing.T) {
	w := &WorkbenchService{}
	envs, e := w.PreviewExternalImport("ccswitch://v1/import?resource=provider&app=claude&name=shared&endpoint=https%3A%2F%2Fa.example.com%2Chttps%3A%2F%2Fb.example.com&apiKey=fixture&model=main&haikuModel=small")
	if e != nil || len(envs) != 2 {
		t.Fatal("link import failed", e)
	}
	if envs[0].Variables["ANTHROPIC_DEFAULT_HAIKU_MODEL"] != "small" || envs[0].Name == envs[1].Name {
		t.Fatal("import lost model mapping or unique name")
	}
}

// withHomeRoot 把 HOME/USERPROFILE 指到临时目录，让 apply*/sync* 写进隔离环境
func withHomeRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

// 覆写保护 1：应用 Claude 配置时，用户自加的 env 变量保留、托管键被新配置接管
func TestApplyClaudeEnvPreservesUserEnvKeys(t *testing.T) {
	home := withHomeRoot(t)
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := map[string]any{
		"permissions": map[string]any{"allow": []any{"Bash"}},
		"env": map[string]any{
			"ANTHROPIC_BASE_URL": "https://old.example.com",
			"HTTP_PROXY":         "http://127.0.0.1:7890",
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	app := &App{}
	env := &EnvConfig{
		Provider: "claude",
		Variables: map[string]string{
			"ANTHROPIC_BASE_URL": "https://new.example.com",
			"ANTHROPIC_API_KEY":  "sk-test",
		},
	}
	if _, err := app.applyClaudeEnv(env); err != nil {
		t.Fatalf("应用失败: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(out, &settings); err != nil {
		t.Fatalf("写回的不是合法 JSON: %v", err)
	}
	if _, ok := settings["permissions"]; !ok {
		t.Errorf("用户的 permissions 被清掉")
	}
	envMap, _ := settings["env"].(map[string]any)
	if envMap == nil {
		t.Fatalf("env 字段丢失: %s", out)
	}
	if envMap["HTTP_PROXY"] != "http://127.0.0.1:7890" {
		t.Errorf("用户自加的 HTTP_PROXY 被覆写丢失: %v", envMap)
	}
	if envMap["ANTHROPIC_BASE_URL"] != "https://new.example.com" {
		t.Errorf("托管键应更新为新配置: %v", envMap["ANTHROPIC_BASE_URL"])
	}
}

// 覆写保护 2：Codex config.toml 重建时保留模板未覆盖的用户段，并合并 auth.json 的 OAuth tokens
func TestCodexApplyPreservesUserSectionsAndTokens(t *testing.T) {
	home := withHomeRoot(t)
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existingToml := `
model = "old-model"
model_provider = "duckcoding"

[features]
web_search = true

[model_providers.duckcoding]
name = "duckcoding"
base_url = "https://old.example.com"
`
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(existingToml), 0o644); err != nil {
		t.Fatal(err)
	}
	existingAuth := map[string]any{
		"OPENAI_API_KEY": "sk-old",
		"tokens":         map[string]any{"access_token": "oauth-token"},
	}
	authData, _ := json.MarshalIndent(existingAuth, "", "  ")
	if err := os.WriteFile(filepath.Join(codexDir, "auth.json"), authData, 0o600); err != nil {
		t.Fatal(err)
	}

	app := &App{}
	env := &EnvConfig{
		Provider: "codex",
		Variables: map[string]string{
			"base_url":       "https://new.example.com",
			"OPENAI_API_KEY": "sk-new",
			"model":          "gpt-5.2",
		},
	}
	if _, err := app.applyCodexEnv(env); err != nil {
		t.Fatalf("应用失败: %v", err)
	}

	tomlData, err := os.ReadFile(filepath.Join(codexDir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := toml.Unmarshal(tomlData, &payload); err != nil {
		t.Fatalf("写回的不是合法 TOML: %v", err)
	}
	if payload["model"] != "gpt-5.2" {
		t.Errorf("模型应更新为新值: %v", payload["model"])
	}
	features, _ := payload["features"].(map[string]any)
	if features == nil || features["web_search"] != true {
		t.Errorf("用户自定义段 [features] 被覆写丢失: %v", payload["features"])
	}

	authOut, err := os.ReadFile(filepath.Join(codexDir, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	var auth map[string]json.RawMessage
	if err := json.Unmarshal(authOut, &auth); err != nil {
		t.Fatalf("auth.json 不是合法 JSON: %v", err)
	}
	var newKey string
	_ = json.Unmarshal(auth["OPENAI_API_KEY"], &newKey)
	if newKey != "sk-new" {
		t.Errorf("OPENAI_API_KEY 应更新: %s", newKey)
	}
	if _, ok := auth["tokens"]; !ok {
		t.Errorf("OAuth tokens 被覆写清掉: %s", authOut)
	}
}

// 覆写保护 3：Grok 深合并保留全部用户自定义顶层段；解析失败中止而不是清空重建
func TestGrokMergePreservesUserSectionsAndAbortsOnCorrupt(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.toml")
	existing := `
[models]
default = "custom"

[theme]
dark = true
`
	if err := os.WriteFile(configFile, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	incoming := `
[models]
default = "custom"

[model.custom]
model = "grok-4.6"
base_url = "https://new.example.com"
`
	if err := mergeWriteGrokConfig(configFile, incoming, map[string]string{}); err != nil {
		t.Fatalf("合并写入失败: %v", err)
	}
	data, _ := os.ReadFile(configFile)
	var payload map[string]any
	if err := toml.Unmarshal(data, &payload); err != nil {
		t.Fatalf("写回的不是合法 TOML: %v", err)
	}
	theme, _ := payload["theme"].(map[string]any)
	if theme == nil || theme["dark"] != true {
		t.Errorf("用户自定义段 [theme] 被覆写丢失: %v", payload)
	}

	// 损坏文件：必须中止并且原文件一个字节都不动
	corrupt := "this is [not toml"
	if err := os.WriteFile(configFile, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mergeWriteGrokConfig(configFile, incoming, map[string]string{}); err == nil {
		t.Fatalf("解析失败时应中止写入")
	}
	after, _ := os.ReadFile(configFile)
	if string(after) != corrupt {
		t.Errorf("中止后原文件被改动: %q", string(after))
	}
}

// 覆写保护 4：MCP 同步遇到解析不了的平台文件时中止，原文件保持原样
func TestMcpSyncAbortsOnCorruptPlatformFile(t *testing.T) {
	home := withHomeRoot(t)
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	corrupt := `{"mcpServers": broken`
	if err := os.WriteFile(filepath.Join(home, claudeMcpFile), []byte(corrupt), 0o600); err != nil {
		t.Fatal(err)
	}

	ms := NewMCPService()
	err := ms.syncClaudeServers([]MCPServer{{
		Name:           "demo",
		Type:           "stdio",
		Command:        "node",
		EnablePlatform: []string{platClaudeCode},
	}})
	if err == nil {
		t.Fatalf("平台文件损坏时应中止同步")
	}
	if !strings.Contains(err.Error(), "中止") {
		t.Errorf("错误信息应说明已中止: %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(home, claudeMcpFile))
	if string(after) != corrupt {
		t.Errorf("中止后原文件被改动: %q", string(after))
	}
}

// 覆写保护：Gemini/Antigravity settings.json 带注释时按 JSON5 合并；
// 实在解析不了时中止，不能用模板覆盖掉用户的全部设置
func TestWriteGeminiStyleSettingsKeepsUserSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	withComments := "{\n  // 用户注释\n  \"theme\": \"Dracula\",\n  \"ide\": {\"enabled\": false, \"port\": 1234},\n}\n"
	if err := os.WriteFile(path, []byte(withComments), 0o644); err != nil {
		t.Fatal(err)
	}
	desired := map[string]any{"ide": map[string]any{"enabled": true}}
	if err := writeGeminiStyleSettings(path, desired, nil); err != nil {
		t.Fatalf("带注释的 settings.json 应能合并: %v", err)
	}
	var got map[string]any
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("写回的不是合法 JSON: %v", err)
	}
	if got["theme"] != "Dracula" {
		t.Fatalf("用户设置 theme 丢失: %v", got)
	}
	ide, _ := got["ide"].(map[string]any)
	if ide["enabled"] != true || ide["port"] != float64(1234) {
		t.Fatalf("ide 未正确合并: %v", ide)
	}

	broken := []byte(`{"theme": "Dracula", `)
	if err := os.WriteFile(path, broken, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeGeminiStyleSettings(path, desired, nil); err == nil {
		t.Fatal("无法解析时应中止写入")
	}
	if data, _ := os.ReadFile(path); string(data) != string(broken) {
		t.Fatalf("原文件被改写: %s", data)
	}
}
