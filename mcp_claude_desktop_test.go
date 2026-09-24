package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// withMcpHome 把所有平台配置都指到临时目录，避免测试读写真实用户文件
func withMcpHome(t *testing.T) (home, desktopConfig string) {
	t.Helper()
	home = withHomeRoot(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("CODEX_HOME", "")
	t.Setenv("GROK_HOME", "")
	t.Setenv("OPENCODE_CONFIG", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	desktopConfig = filepath.Join(home, "Claude", claudeDesktopMcpFile)
	t.Setenv("CLAUDE_DESKTOP_MCP_CONFIG", desktopConfig)
	return home, desktopConfig
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("解析 %s 失败: %v", path, err)
	}
	return out
}

// 删除中央存储里的服务器后，保留外部条目的平台（Antigravity）也必须把它删掉，
// 否则下次加载会从平台文件把它重新导入回来
func TestMcpDeleteRemovesServerFromMergingPlatforms(t *testing.T) {
	home, _ := withMcpHome(t)
	ms := NewMCPService()

	servers := []MCPServer{{
		Name:           "fetch",
		Type:           "stdio",
		Command:        "uvx",
		Args:           []string{"mcp-server-fetch"},
		EnablePlatform: []string{platAntigravity},
	}}
	if err := ms.SaveServers(servers); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	agPath := filepath.Join(home, geminiDirName, "config", "mcp_config.json")
	if _, ok := readJSONFile(t, agPath)["mcpServers"].(map[string]any)["fetch"]; !ok {
		t.Fatalf("fetch 应已写入 Antigravity")
	}

	if err := ms.SaveServers([]MCPServer{}); err != nil {
		t.Fatalf("删除后保存失败: %v", err)
	}
	if _, ok := readJSONFile(t, agPath)["mcpServers"].(map[string]any)["fetch"]; ok {
		t.Fatalf("删除后 fetch 仍留在 Antigravity 配置里")
	}
	listed, err := ms.ListServers()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range listed {
		if s.Name == "fetch" {
			t.Fatalf("已删除的 fetch 又被从平台文件导入回来了")
		}
	}
}

// 启用 Claude Desktop：stdio 原样写入，远程服务器走 mcp-remote 桥接（头放 env），
// 文件里的其它设置与不归本工具管理的服务器保持不动
func TestMcpSyncToClaudeDesktop(t *testing.T) {
	home, desktopConfig := withMcpHome(t)
	if err := os.MkdirAll(filepath.Dir(desktopConfig), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"preferences":{"sidebarMode":"chat"},"mcpServers":{"manual":{"command":"node","args":["server.js"]}}}`
	if err := os.WriteFile(desktopConfig, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	ms := NewMCPService()
	servers := []MCPServer{
		{Name: "manual", Type: "stdio", Command: "node", Args: []string{"server.js"}, EnablePlatform: []string{platClaudeDesktop}},
		{Name: "fs", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"}, EnablePlatform: []string{platClaudeDesktop}},
		{Name: "remote", Type: "http", URL: "https://mcp.example.com/mcp", Headers: map[string]string{"Authorization": "Bearer abc 123"}, EnablePlatform: []string{platClaudeDesktop, platClaudeCode}},
	}
	if err := ms.SaveServers(servers); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	cfg := readJSONFile(t, desktopConfig)
	if prefs, ok := cfg["preferences"].(map[string]any); !ok || prefs["sidebarMode"] != "chat" {
		t.Fatalf("preferences 被改动: %v", cfg["preferences"])
	}
	mcpServers := cfg["mcpServers"].(map[string]any)
	fs := mcpServers["fs"].(map[string]any)
	if fs["command"] != "npx" || len(fs["args"].([]any)) != 3 {
		t.Fatalf("stdio 条目写入不正确: %v", fs)
	}
	remote := mcpServers["remote"].(map[string]any)
	args := remote["args"].([]any)
	if remote["command"] != "npx" || args[1] != mcpRemotePackage || args[2] != "https://mcp.example.com/mcp" {
		t.Fatalf("远程服务器应通过 mcp-remote 桥接: %v", remote)
	}
	if args[3] != "--header" || args[4] != "Authorization:${MCP_REMOTE_HEADER_1}" {
		t.Fatalf("请求头应以 env 引用写入 args，避免空格被拆: %v", args)
	}
	if env := remote["env"].(map[string]any); env["MCP_REMOTE_HEADER_1"] != "Bearer abc 123" {
		t.Fatalf("请求头的值应放在 env: %v", env)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex")); !os.IsNotExist(err) {
		t.Fatalf("没有服务器启用 Codex 时不应创建 ~/.codex")
	}

	listed, err := ms.ListServers()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]MCPServer{}
	for _, s := range listed {
		byName[s.Name] = s
	}
	if !byName["remote"].EnabledInClaudeDesktop || !platformContains(byName["remote"].EnablePlatform, platClaudeDesktop) {
		t.Fatalf("remote 应显示为已启用 Claude Desktop: %+v", byName["remote"])
	}
	if byName["remote"].Type != "http" {
		t.Fatalf("重新加载后 remote 仍应是 http 类型，实际 %s", byName["remote"].Type)
	}

	// 从存储删掉 remote：Claude Desktop 里的桥接条目也要删掉
	if err := ms.SaveServers(servers[:2]); err != nil {
		t.Fatalf("删除后保存失败: %v", err)
	}
	if _, ok := readJSONFile(t, desktopConfig)["mcpServers"].(map[string]any)["remote"]; ok {
		t.Fatalf("删除后 remote 仍留在 Claude Desktop 配置里")
	}
}

// Claude Desktop 里手动加的服务器会被导入；本工具认得的 mcp-remote 桥接还原成远程服务器
func TestMcpImportFromClaudeDesktop(t *testing.T) {
	_, desktopConfig := withMcpHome(t)
	if err := os.MkdirAll(filepath.Dir(desktopConfig), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"mcpServers":{
		"local":{"command":"uvx","args":["mcp-server-time"]},
		"bridged":{"command":"npx","args":["mcp-remote","https://api.example.com/sse","--header","X-Token:${MCP_REMOTE_HEADER_1}","--transport","sse-only"],"env":{"MCP_REMOTE_HEADER_1":"t0k"}},
		"custom-bridge":{"command":"npx","args":["mcp-remote","https://api.example.com/mcp","3334"]}
	}}`
	if err := os.WriteFile(desktopConfig, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	listed, err := NewMCPService().ListServers()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]MCPServer{}
	for _, s := range listed {
		byName[s.Name] = s
	}
	local, ok := byName["local"]
	if !ok || local.Type != "stdio" || local.Command != "uvx" || !platformContains(local.EnablePlatform, platClaudeDesktop) {
		t.Fatalf("stdio 服务器导入不正确: %+v", local)
	}
	bridged := byName["bridged"]
	if bridged.Type != "sse" || bridged.URL != "https://api.example.com/sse" || bridged.Headers["X-Token"] != "t0k" {
		t.Fatalf("mcp-remote 桥接应还原为远程服务器: %+v", bridged)
	}
	custom := byName["custom-bridge"]
	if custom.Type != "stdio" || custom.Command != "npx" || len(custom.Args) != 3 {
		t.Fatalf("带未知参数的桥接应按 stdio 原样保留: %+v", custom)
	}
}

func TestParseMcpRemoteBridgeRoundTrip(t *testing.T) {
	server := MCPServer{
		Name:    "r",
		Type:    "http",
		URL:     "https://x.example.com/mcp",
		Headers: map[string]string{"Authorization": "Bearer a b", "X-Team": "core"},
	}
	entry := buildClaudeDesktopMcpEntry(server)
	args := entry["args"].([]string)
	env := entry["env"].(map[string]string)
	url, headers, sse, ok := parseMcpRemoteBridge(entry["command"].(string), args, env)
	if !ok || sse || url != server.URL {
		t.Fatalf("桥接往返失败: ok=%v sse=%v url=%s", ok, sse, url)
	}
	for k, v := range server.Headers {
		if headers[k] != v {
			t.Fatalf("请求头 %s 往返后为 %q，期望 %q", k, headers[k], v)
		}
	}
	if _, _, _, ok := parseMcpRemoteBridge("node", []string{"mcp-remote", "https://a"}, nil); ok {
		t.Fatalf("非 npx 命令不应识别为桥接")
	}
}

// 没装 Claude Desktop、也没有服务器启用它时，不能凭空创建配置文件
func TestMcpSyncSkipsMissingClaudeDesktop(t *testing.T) {
	_, desktopConfig := withMcpHome(t)
	err := NewMCPService().SaveServers([]MCPServer{{Name: "a", Type: "stdio", Command: "node", EnablePlatform: []string{platClaudeCode}}})
	if err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if _, err := os.Stat(desktopConfig); !os.IsNotExist(err) {
		t.Fatalf("不应创建 Claude Desktop 配置文件")
	}
}

// 从 JSON / 市场导入的远程服务器必须保留请求头（此前 AddServers 会丢掉 Headers）
func TestMcpAddServersKeepsHeaders(t *testing.T) {
	home, _ := withMcpHome(t)
	ms := NewMCPService()
	err := ms.AddServers([]MCPServer{{
		Name:           "remote",
		Type:           "http",
		URL:            "https://mcp.example.com",
		Headers:        map[string]string{"Authorization": "Bearer k"},
		EnablePlatform: []string{platClaudeCode},
	}})
	if err != nil {
		t.Fatalf("添加失败: %v", err)
	}
	cfg := readJSONFile(t, filepath.Join(home, claudeMcpFile))
	remote := cfg["mcpServers"].(map[string]any)["remote"].(map[string]any)
	if headers, ok := remote["headers"].(map[string]any); !ok || headers["Authorization"] != "Bearer k" {
		t.Fatalf("请求头丢失: %v", remote)
	}
}

// 旧版 Claude Desktop 的环境配置与 MCP 同在 claude_desktop_config.json：
// 用导入模板应用环境、清除环境，都不能动到 MCP 页面同步过去的 mcpServers
func TestClaudeDesktopEnvKeepsMcpServers(t *testing.T) {
	_, desktopConfig := withMcpHome(t)
	t.Setenv("CLAUDE_DESKTOP_CONFIG", desktopConfig)
	if err := os.MkdirAll(filepath.Dir(desktopConfig), 0o755); err != nil {
		t.Fatal(err)
	}
	current := `{"preferences":{"a":1},"mcpServers":{"live":{"command":"node"}}}`
	if err := os.WriteFile(desktopConfig, []byte(current), 0o600); err != nil {
		t.Fatal(err)
	}

	app := &App{configPath: filepath.Join(t.TempDir(), "config.json")}
	env := &EnvConfig{
		Name:      "desk",
		Provider:  "claude_desktop",
		Variables: map[string]string{"ANTHROPIC_BASE_URL": "https://relay.example.com"},
		Templates: map[string]string{"claude_desktop_config.json": `{"mcpServers":{"stale":{"command":"old"}}}`},
	}
	if _, err := app.applyClaudeDesktopEnv(env); err != nil {
		t.Fatalf("应用失败: %v", err)
	}
	cfg := readJSONFile(t, desktopConfig)
	servers := cfg["mcpServers"].(map[string]any)
	if _, ok := servers["live"]; !ok {
		t.Fatalf("模板盖掉了当前的 mcpServers: %v", servers)
	}
	if _, ok := servers["stale"]; ok {
		t.Fatalf("模板里的旧 MCP 快照不应写回: %v", servers)
	}
	if cfg["env"] == nil {
		t.Fatalf("env 应写入")
	}

	app.configMu.Lock()
	err := app.clearClaudeDesktopSettingsLocked()
	app.configMu.Unlock()
	if err != nil {
		t.Fatalf("清除失败: %v", err)
	}
	cfg = readJSONFile(t, desktopConfig)
	if cfg["env"] != nil {
		t.Fatalf("清除后 env 应被摘掉: %v", cfg)
	}
	if _, ok := cfg["mcpServers"].(map[string]any)["live"]; !ok {
		t.Fatalf("清除环境不应删掉 MCP 服务器: %v", cfg)
	}
}

// 服务器没有变化时不重写 Claude Desktop 配置（不重排用户的键、不刷新 .bak）
func TestMcpSyncClaudeDesktopNoopWhenUnchanged(t *testing.T) {
	_, desktopConfig := withMcpHome(t)
	if err := os.MkdirAll(filepath.Dir(desktopConfig), 0o755); err != nil {
		t.Fatal(err)
	}
	original := `{"zeta":1,"alpha":{"b":2,"a":1},"mcpServers":{"fs":{"command":"npx","args":["-y","pkg"]}}}`
	if err := os.WriteFile(desktopConfig, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	ms := NewMCPService()
	servers := []MCPServer{{Name: "fs", Type: "stdio", Command: "npx", Args: []string{"-y", "pkg"}, EnablePlatform: []string{platClaudeDesktop}}}
	if err := ms.syncClaudeDesktopServers(servers, nil); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(desktopConfig)
	if string(after) != original {
		t.Fatalf("内容未变时不应重写文件:\n%s", after)
	}
	if _, err := os.Stat(desktopConfig + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("内容未变时不应生成 .bak")
	}

	// 没有任何服务器、原文件也没有 mcpServers：同样不写
	noServers := `{"preferences":{}}`
	if err := os.WriteFile(desktopConfig, []byte(noServers), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ms.syncClaudeDesktopServers(nil, nil); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(desktopConfig); string(after) != noServers {
		t.Fatalf("不应凭空加上 mcpServers: %s", after)
	}
}

// Claude Desktop 配置文件损坏时：没给它启用服务器就照常保存；给它启用了则中止并保留原文件
func TestMcpCorruptClaudeDesktopConfig(t *testing.T) {
	home, desktopConfig := withMcpHome(t)
	if err := os.MkdirAll(filepath.Dir(desktopConfig), 0o755); err != nil {
		t.Fatal(err)
	}
	corrupt := `{"mcpServers": oops`
	if err := os.WriteFile(desktopConfig, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	ms := NewMCPService()
	if err := ms.SaveServers([]MCPServer{{Name: "a", Type: "stdio", Command: "node", EnablePlatform: []string{platClaudeCode}}}); err != nil {
		t.Fatalf("未启用 Claude Desktop 时不应被它的坏配置挡住: %v", err)
	}
	if _, ok := readJSONFile(t, filepath.Join(home, claudeMcpFile))["mcpServers"].(map[string]any)["a"]; !ok {
		t.Fatalf("Claude Code 应已同步")
	}
	err := ms.SaveServers([]MCPServer{{Name: "a", Type: "stdio", Command: "node", EnablePlatform: []string{platClaudeDesktop}}})
	if err == nil {
		t.Fatalf("要写入 Claude Desktop 时遇到坏配置应报错")
	}
	if after, _ := os.ReadFile(desktopConfig); string(after) != corrupt {
		t.Fatalf("损坏的配置文件不应被改写")
	}
}
