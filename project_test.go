package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestProject(t *testing.T, home string) string {
	t.Helper()
	dir := filepath.Join(home, "work", "demo")
	if err := os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("%s 不是有效 JSON: %v", path, err)
	}
	return obj
}

// 列表：手动添加的在前，~/.claude.json 里发现的在后；不存在的目录与主目录不列；移除发现的项目改为隐藏
func TestProjectListIncludesDiscovered(t *testing.T) {
	home, _ := withMcpHome(t)
	pinned := newTestProject(t, home)
	found := filepath.Join(home, "code", "alpha")
	os.MkdirAll(found, 0o755)
	claudeJSON := map[string]any{"projects": map[string]any{
		filepath.ToSlash(found):                       map[string]any{},
		filepath.ToSlash(filepath.Join(home, "gone")): map[string]any{},
		filepath.ToSlash(home):                        map[string]any{},
	}}
	data, _ := json.Marshal(claudeJSON)
	os.WriteFile(filepath.Join(home, claudeMcpFile), data, 0o600)

	ps := NewProjectService(nil, nil)
	if _, err := ps.AddProject(pinned); err != nil {
		t.Fatal(err)
	}
	if _, err := ps.AddProject(home); err == nil {
		t.Fatalf("主目录不能作为项目")
	}
	list, err := ps.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Path != pinned || !list[0].Pinned || list[1].Path != found || list[1].Pinned {
		t.Fatalf("列表不符合预期: %+v", list)
	}
	if err := ps.RemoveProject(found); err != nil {
		t.Fatal(err)
	}
	if list, _ := ps.ListProjects(); len(list) != 1 {
		t.Fatalf("移除发现的项目后应不再显示: %+v", list)
	}
	// 再手动添加回来会取消隐藏
	if _, err := ps.AddProject(found); err != nil {
		t.Fatal(err)
	}
	if list, _ := ps.ListProjects(); len(list) != 2 {
		t.Fatalf("重新添加后应显示: %+v", list)
	}
}

// .mcp.json：从全局库加入会保留文件里已有的条目与其它字段；有未填占位符的拒绝
func TestProjectMcpFromLibrary(t *testing.T) {
	home, _ := withMcpHome(t)
	dir := newTestProject(t, home)
	os.WriteFile(filepath.Join(dir, projectMcpFile), []byte(`{"mcpServers":{"team-db":{"command":"db-mcp"}},"note":"keep"}`), 0o644)

	ms := NewMCPService()
	if err := ms.SaveServers([]MCPServer{
		{Name: "fetch", Type: "stdio", Command: "uvx", Args: []string{"mcp-server-fetch"}},
		{Name: "linear", Type: "http", URL: "https://mcp.linear.app/mcp", Headers: map[string]string{"Authorization": "Bearer x"}},
		{Name: "needs-key", Type: "http", URL: "https://api.example.com/{API_KEY}"},
	}); err != nil {
		t.Fatal(err)
	}
	ps := NewProjectService(nil, ms)
	if err := ps.AddLibraryMcpToProject(dir, []string{"fetch", "linear"}); err != nil {
		t.Fatal(err)
	}
	obj := readJSON(t, filepath.Join(dir, projectMcpFile))
	servers := obj["mcpServers"].(map[string]any)
	if obj["note"] != "keep" || servers["team-db"] == nil {
		t.Fatalf("已有条目与其它字段应保留: %v", obj)
	}
	fetch := servers["fetch"].(map[string]any)
	if fetch["type"] != "stdio" || fetch["command"] != "uvx" {
		t.Fatalf("stdio 条目格式不对: %v", fetch)
	}
	linear := servers["linear"].(map[string]any)
	if linear["url"] != "https://mcp.linear.app/mcp" || linear["headers"] == nil {
		t.Fatalf("http 条目应带 url 与 headers: %v", linear)
	}
	if err := ps.AddLibraryMcpToProject(dir, []string{"needs-key"}); err == nil || !strings.Contains(err.Error(), "占位符") {
		t.Fatalf("有未填占位符的服务器应拒绝，实际 %v", err)
	}

	detail, err := ps.GetProjectDetail(dir)
	if err != nil {
		t.Fatal(err)
	}
	if detail.McpCount != 3 || len(detail.McpServers) != 3 {
		t.Fatalf("详情应列出 3 个服务器: %+v", detail.McpServers)
	}
	for _, s := range detail.McpServers {
		if want := s.Name != "team-db"; s.InLibrary != want {
			t.Fatalf("%s 的 InLibrary 应为 %v（全局库里有 fetch、linear，没有 team-db）", s.Name, want)
		}
	}
	if err := ps.RemoveProjectMcp(dir, "fetch"); err != nil {
		t.Fatal(err)
	}
	if servers := readJSON(t, filepath.Join(dir, projectMcpFile))["mcpServers"].(map[string]any); servers["fetch"] != nil || servers["team-db"] == nil {
		t.Fatalf("只应删掉 fetch: %v", servers)
	}
}

// 按项目应用配置：只写项目 settings.local.json，保留用户字段，加入 git exclude；清除后恢复全局
func TestProjectApplyEnv(t *testing.T) {
	home, _ := withMcpHome(t)
	dir := newTestProject(t, home)
	local := filepath.Join(dir, projectLocalSettings)
	os.MkdirAll(filepath.Dir(local), 0o755)
	os.WriteFile(local, []byte(`{"permissions":{"allow":["Bash(ls)"]},"env":{"MY_FLAG":"1","ANTHROPIC_MODEL":"old"}}`), 0o600)

	app := &App{configPath: filepath.Join(home, "config.json")}
	app.config.Environments = []EnvConfig{
		{Name: "glm", Provider: "claude", Variables: map[string]string{"ANTHROPIC_BASE_URL": "https://glm.example.com/api/anthropic", "ANTHROPIC_AUTH_TOKEN": "sk-glm-123456789"}},
		{Name: "official", Provider: "claude", OfficialLogin: true},
		{Name: "via-chat", Provider: "claude", UpstreamFormat: UpstreamChatCompletions, Variables: map[string]string{"ANTHROPIC_BASE_URL": "https://x"}},
		{Name: "codex-main", Provider: "codex", Variables: map[string]string{"base_url": "https://x"}},
	}
	ps := NewProjectService(app, nil)

	for _, bad := range []string{"official", "via-chat", "codex-main"} {
		if _, err := ps.ApplyEnvToProject(dir, bad); err == nil {
			t.Fatalf("%s 不应能按项目应用", bad)
		}
	}
	if _, err := ps.ApplyEnvToProject(dir, "glm"); err != nil {
		t.Fatal(err)
	}
	obj := readJSON(t, local)
	env := obj["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != "https://glm.example.com/api/anthropic" || env["MY_FLAG"] != "1" || env["ANTHROPIC_MODEL"] != nil {
		t.Fatalf("env 合并不对（应写入新配置、保留用户变量、摘掉旧托管键）: %v", env)
	}
	if obj["permissions"] == nil {
		t.Fatalf("permissions 等其它设置应保留")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "settings.json")); err == nil {
		t.Fatalf("按项目应用不能写全局 settings.json")
	}
	if data, _ := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude")); !strings.Contains(string(data), "/.claude/settings.local.json") {
		t.Fatalf("含 Key 的本机设置应加入 git exclude，实际 %q", data)
	}

	detail, err := ps.GetProjectDetail(dir)
	if err != nil {
		t.Fatal(err)
	}
	if detail.AppliedEnv != "glm" || !detail.EnvOverride {
		t.Fatalf("详情应显示已应用 glm: %+v", detail.ProjectInfo)
	}
	for _, v := range detail.EnvVars {
		if v.Key == "ANTHROPIC_AUTH_TOKEN" && strings.Contains(v.Value, "123456789") {
			t.Fatalf("密钥应打码: %s", v.Value)
		}
	}

	if err := ps.ClearProjectEnv(dir); err != nil {
		t.Fatal(err)
	}
	env = readJSON(t, local)["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != nil || env["MY_FLAG"] != "1" {
		t.Fatalf("清除后只应去掉接入变量: %v", env)
	}
	if detail, _ := ps.GetProjectDetail(dir); detail.AppliedEnv != "" || detail.EnvOverride {
		t.Fatalf("清除后不应再显示已应用: %+v", detail.ProjectInfo)
	}
}

func TestProjectClaudeMD(t *testing.T) {
	home, _ := withMcpHome(t)
	dir := newTestProject(t, home)
	ps := NewProjectService(nil, nil)
	if err := ps.SaveProjectClaudeMD(dir, "# rules\n"); err != nil {
		t.Fatal(err)
	}
	if detail, _ := ps.GetProjectDetail(dir); detail.ClaudeMD != "# rules\n" || !detail.HasClaudeMD {
		t.Fatalf("CLAUDE.md 应已保存: %+v", detail)
	}
	if err := ps.SaveProjectClaudeMD(dir, "  "); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, projectClaudeMD)); !os.IsNotExist(err) {
		t.Fatalf("内容为空时应删除 CLAUDE.md")
	}
	if _, err := os.Stat(filepath.Join(dir, projectClaudeMD+".bak")); !os.IsNotExist(err) {
		t.Fatalf("项目目录里不应留 .bak")
	}
}
