package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultOpencodeConfigWithGateway(t *testing.T) {
	env := &EnvConfig{Variables: map[string]string{
		"OPENCODE_BASE_URL": "https://gw.example.com/v1",
		"OPENCODE_API_KEY":  "sk-test",
		"OPENCODE_MODEL":    "anthropic/claude-sonnet-4",
	}}
	var payload map[string]any
	if err := json.Unmarshal([]byte(defaultOpencodeConfig(env)), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["model"] != "custom/claude-sonnet-4" {
		t.Errorf("model = %v", payload["model"])
	}
	provider, ok := payload["provider"].(map[string]any)
	if !ok {
		t.Fatalf("provider missing: %v", payload["provider"])
	}
	custom, ok := provider["custom"].(map[string]any)
	if !ok {
		t.Fatal("provider.custom missing")
	}
	if custom["npm"] != "@ai-sdk/openai-compatible" {
		t.Errorf("npm = %v", custom["npm"])
	}
	options, ok := custom["options"].(map[string]any)
	if !ok {
		t.Fatal("provider.custom.options missing")
	}
	if options["baseURL"] != "https://gw.example.com/v1" || options["apiKey"] != "sk-test" {
		t.Errorf("options = %v", options)
	}
	models, ok := custom["models"].(map[string]any)
	if !ok {
		t.Fatal("provider.custom.models missing")
	}
	if _, ok := models["claude-sonnet-4"]; !ok {
		t.Errorf("models missing bare id: %v", models)
	}
}

func TestDefaultOpencodeConfigNativeOnly(t *testing.T) {
	env := &EnvConfig{Variables: map[string]string{"OPENCODE_MODEL": "anthropic/claude-sonnet-4"}}
	var payload map[string]any
	if err := json.Unmarshal([]byte(defaultOpencodeConfig(env)), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["model"] != "anthropic/claude-sonnet-4" {
		t.Errorf("model = %v", payload["model"])
	}
	if _, has := payload["provider"]; has {
		t.Error("provider should be absent without base url")
	}
}

func TestApplyOpencodeTemplate(t *testing.T) {
	env := &EnvConfig{Variables: map[string]string{
		"OPENCODE_MODEL":    "gpt-4.1",
		"OPENCODE_BASE_URL": "https://gw/v1",
		"OPENCODE_API_KEY":  "k-1",
	}}
	out := applyOpencodeTemplate(
		`{"model":"{{OPENCODE_MODEL}}","u":"{{OPENCODE_BASE_URL}}","k":"{{OPENCODE_API_KEY}}","m":"{{model}}","b":"{{base_url}}","a":"{{api_key}}"}`,
		env,
	)
	for _, want := range []string{
		`"model":"gpt-4.1"`, `"u":"https://gw/v1"`, `"k":"k-1"`,
		`"m":"gpt-4.1"`, `"b":"https://gw/v1"`, `"a":"k-1"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("template output missing %s: %s", want, out)
		}
	}
}

func TestOpencodeConfigFileOverrides(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("USERPROFILE", fakeHome)
	t.Setenv("HOME", fakeHome)

	got := opencodeConfigFile(nil)
	want := filepath.Join(fakeHome, ".config", "opencode", "opencode.json")
	if got != want {
		t.Errorf("default = %s, want %s", got, want)
	}

	got = opencodeConfigFile(map[string]string{"OPENCODE_CONFIG_DIR": filepath.Join(fakeHome, "oc")})
	want = filepath.Join(fakeHome, "oc", "opencode.json")
	if got != want {
		t.Errorf("config dir override = %s, want %s", got, want)
	}

	// OPENCODE_CONFIG 指向文件时 ~ 应按用户主目录展开
	got = opencodeConfigFile(map[string]string{"OPENCODE_CONFIG": "~/custom/opencode.json"})
	want = filepath.Join(fakeHome, "custom", "opencode.json")
	if got != want {
		t.Errorf("~ expansion = %s, want %s", got, want)
	}
}

func TestMergeWriteOpencodeConfigPreservesOtherKeys(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "opencode.json")

	existing := `{"model":"old","mcp":{"fetch":{"type":"http"}},"agent":{"build":{}}}`
	if err := os.WriteFile(file, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}
	incoming := defaultOpencodeConfig(&EnvConfig{Variables: map[string]string{"OPENCODE_MODEL": "m-new"}})
	if err := mergeWriteOpencodeConfig(file, incoming, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("merged invalid: %v", err)
	}
	if payload["model"] != "m-new" {
		t.Errorf("model = %v", payload["model"])
	}
	if _, ok := payload["mcp"]; !ok {
		t.Error("mcp key lost")
	}
	if _, ok := payload["agent"]; !ok {
		t.Error("agent key lost")
	}
	if payload["$schema"] != "https://opencode.ai/config.json" {
		t.Errorf("schema = %v", payload["$schema"])
	}
}

func TestLegacyOpenclawNormalization(t *testing.T) {
	if p, ok := normalizePlatform(" openclaw "); !ok || p != "opencode" {
		t.Errorf("normalizePlatform(openclaw) = %s,%v", p, ok)
	}
	if p, ok := normalizePlatform("OpenCode"); !ok || p != "opencode" {
		t.Errorf("normalizePlatform(OpenCode) = %s,%v", p, ok)
	}
	if g := normalizeRotationGroup(RotationGroup{Provider: "OpenClaw", Name: "x"}); g.Provider != "opencode" {
		t.Errorf("rotation provider = %s", g.Provider)
	}
	if normalizeProvider("openclaw") != "opencode" {
		t.Error("usage provider mapping failed")
	}
}
