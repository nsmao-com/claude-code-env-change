package main

// 项目级配置：Claude Code 除了全局的 ~/.claude/settings.json 与 ~/.claude.json，
// 还会读项目目录里的三样东西，这里按项目管理它们：
//   - .mcp.json：项目共享的 MCP 服务器（通常随仓库提交）
//   - .claude/settings.local.json：只在本机生效的项目设置，用来让这个项目走另一套接入配置
//   - CLAUDE.md：项目提示词
//
// 写项目目录时不留 .bak（会出现在 git status 里）；settings.local.json 含 API Key，
// 项目是 git 仓库时会把它加进 .git/info/exclude，避免被误提交。

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	projectsStoreFile = "projects.json"
	projectMcpFile    = ".mcp.json"
	projectClaudeMD   = "CLAUDE.md"
)

var projectLocalSettings = filepath.Join(".claude", "settings.local.json")

// ProjectInfo 项目列表里的一项
type ProjectInfo struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Exists      bool   `json:"exists"`
	Pinned      bool   `json:"pinned"` // 手动添加；否则是从 ~/.claude.json 发现的
	McpCount    int    `json:"mcp_count"`
	HasClaudeMD bool   `json:"has_claude_md"`
	AppliedEnv  string `json:"applied_env,omitempty"`
	// EnvOverride 项目 settings.local.json 里设置了接入变量（不管是不是本工具写的）
	EnvOverride bool `json:"env_override"`
}

// ProjectMcpServer 项目 .mcp.json 里的一个服务器
type ProjectMcpServer struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	URL       string   `json:"url,omitempty"`
	InLibrary bool     `json:"in_library"` // 全局 MCP 库里有同名服务器
}

// ProjectEnvVar 项目 settings.local.json 里的接入变量（密钥已打码）
type ProjectEnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ProjectDetail 项目详情
type ProjectDetail struct {
	ProjectInfo
	McpServers   []ProjectMcpServer `json:"mcp_servers"`
	EnvVars      []ProjectEnvVar    `json:"env_vars"`
	ClaudeMD     string             `json:"claude_md"`
	McpPath      string             `json:"mcp_path"`
	SettingsPath string             `json:"settings_path"`
	ClaudeMDPath string             `json:"claude_md_path"`
}

type projectRecord struct {
	Path       string `json:"path"`
	AppliedEnv string `json:"applied_env,omitempty"`
	AppliedAt  int64  `json:"applied_at,omitempty"`
}

type projectStore struct {
	Projects []projectRecord `json:"projects"`
	// Hidden 从列表里移除的"发现的项目"（它们来自 ~/.claude.json，删不掉，只能隐藏）
	Hidden []string `json:"hidden,omitempty"`
}

// ProjectService 项目级配置
type ProjectService struct {
	mu  sync.Mutex
	app *App
	mcp *MCPService
}

func NewProjectService(app *App, mcp *MCPService) *ProjectService {
	return &ProjectService{app: app, mcp: mcp}
}

func projectStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, projectsStoreFile), nil
}

func loadProjectStore() (projectStore, error) {
	var store projectStore
	path, err := projectStorePath()
	if err != nil {
		return store, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return store, err
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return store, errorf("解析 %s 失败: %v", path, err)
	}
	return store, nil
}

func saveProjectStore(store projectStore) error {
	path, err := projectStorePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFileAtomic(path, data, 0o600); err != nil {
		return err
	}
	notifyCloudSync()
	return nil
}

// cleanProjectPath 统一路径写法（~/.claude.json 在 Windows 上也用正斜杠）
func cleanProjectPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Clean(filepath.FromSlash(path))
}

func sameProjectPath(a, b string) bool {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(cleanProjectPath(a), cleanProjectPath(b))
	}
	return cleanProjectPath(a) == cleanProjectPath(b)
}

func (store *projectStore) find(path string) *projectRecord {
	for i := range store.Projects {
		if sameProjectPath(store.Projects[i].Path, path) {
			return &store.Projects[i]
		}
	}
	return nil
}

func (store *projectStore) hidden(path string) bool {
	for _, h := range store.Hidden {
		if sameProjectPath(h, path) {
			return true
		}
	}
	return false
}

// discoverClaudeProjects Claude Code 在 ~/.claude.json 的 projects 里记着用过的项目目录
func discoverClaudeProjects() []string {
	path, err := claudeConfigPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var payload struct {
		Projects map[string]json.RawMessage `json:"projects"`
	}
	if json.Unmarshal(data, &payload) != nil {
		return nil
	}
	out := make([]string, 0, len(payload.Projects))
	for p := range payload.Projects {
		if cleaned := cleanProjectPath(p); cleaned != "" && filepath.IsAbs(cleaned) {
			out = append(out, cleaned)
		}
	}
	return out
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// validateProjectDir 只接受已存在的绝对路径目录，且不能是用户主目录本身
// （主目录的 .claude/settings.local.json 与全局配置混在一起，容易误伤）
func validateProjectDir(path string) (string, error) {
	cleaned := cleanProjectPath(path)
	if cleaned == "" || !filepath.IsAbs(cleaned) {
		return "", errorf("项目路径必须是绝对路径")
	}
	if !isDir(cleaned) {
		return "", errorf("项目目录不存在: %s", cleaned)
	}
	if home, err := os.UserHomeDir(); err == nil && sameProjectPath(home, cleaned) {
		return "", errorf("不能把用户主目录当作项目，它的设置就是全局设置")
	}
	return cleaned, nil
}

func readJSONObjectFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}
	obj, err := parseJSONLikeObject(data)
	if err != nil {
		return nil, errorf("解析 %s 失败，为保护原文件已中止: %v", path, err)
	}
	return obj, nil
}

func writeJSONObjectFile(path string, obj map[string]any, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'), perm)
}

func projectMcpServers(dir string) (map[string]any, error) {
	obj, err := readJSONObjectFile(filepath.Join(dir, projectMcpFile))
	if err != nil {
		return nil, err
	}
	servers, _ := obj["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	return servers, nil
}

// projectEnv 项目 settings.local.json 的 env
func projectEnv(dir string) map[string]any {
	obj, err := readJSONObjectFile(filepath.Join(dir, projectLocalSettings))
	if err != nil {
		return nil
	}
	env, _ := obj["env"].(map[string]any)
	return env
}

var projectEnvSignalKeys = []string{"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY", "ANTHROPIC_MODEL"}

func (ps *ProjectService) buildInfo(path string, pinned bool, record *projectRecord) ProjectInfo {
	info := ProjectInfo{Path: path, Name: filepath.Base(path), Pinned: pinned}
	info.Exists = isDir(path)
	if record != nil {
		info.AppliedEnv = record.AppliedEnv
	}
	if !info.Exists {
		return info
	}
	if servers, err := projectMcpServers(path); err == nil {
		info.McpCount = len(servers)
	}
	if _, err := os.Stat(filepath.Join(path, projectClaudeMD)); err == nil {
		info.HasClaudeMD = true
	}
	env := projectEnv(path)
	for _, key := range projectEnvSignalKeys {
		if v, ok := env[key]; ok && fmt.Sprint(v) != "" {
			info.EnvOverride = true
			break
		}
	}
	if !info.EnvOverride {
		// 本工具记的"已应用"与文件对不上（被手动清掉了）时不再显示
		info.AppliedEnv = ""
	}
	return info
}

// ListProjects 手动添加的项目在前，其后是 ~/.claude.json 里发现的项目（按名称排序）
func (ps *ProjectService) ListProjects() ([]ProjectInfo, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	store, err := loadProjectStore()
	if err != nil {
		return nil, err
	}
	out := make([]ProjectInfo, 0, len(store.Projects))
	for i := range store.Projects {
		out = append(out, ps.buildInfo(cleanProjectPath(store.Projects[i].Path), true, &store.Projects[i]))
	}
	var discovered []ProjectInfo
	home, _ := os.UserHomeDir()
	for _, path := range discoverClaudeProjects() {
		if store.find(path) != nil || store.hidden(path) || !isDir(path) || (home != "" && sameProjectPath(home, path)) {
			continue
		}
		dup := false
		for _, item := range discovered {
			if sameProjectPath(item.Path, path) {
				dup = true
				break
			}
		}
		if !dup {
			discovered = append(discovered, ps.buildInfo(path, false, nil))
		}
	}
	sort.Slice(discovered, func(i, j int) bool {
		return strings.ToLower(discovered[i].Name) < strings.ToLower(discovered[j].Name)
	})
	return append(out, discovered...), nil
}

// AddProject 手动添加项目目录
func (ps *ProjectService) AddProject(path string) (ProjectInfo, error) {
	dir, err := validateProjectDir(path)
	if err != nil {
		return ProjectInfo{}, err
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	store, err := loadProjectStore()
	if err != nil {
		return ProjectInfo{}, err
	}
	if store.find(dir) == nil {
		store.Projects = append(store.Projects, projectRecord{Path: dir})
	}
	hidden := store.Hidden[:0]
	for _, h := range store.Hidden {
		if !sameProjectPath(h, dir) {
			hidden = append(hidden, h)
		}
	}
	store.Hidden = hidden
	if err := saveProjectStore(store); err != nil {
		return ProjectInfo{}, err
	}
	return ps.buildInfo(dir, true, store.find(dir)), nil
}

// RemoveProject 从列表移除（不动项目里的任何文件）；发现的项目改为隐藏
func (ps *ProjectService) RemoveProject(path string) error {
	dir := cleanProjectPath(path)
	ps.mu.Lock()
	defer ps.mu.Unlock()
	store, err := loadProjectStore()
	if err != nil {
		return err
	}
	kept := store.Projects[:0]
	for _, p := range store.Projects {
		if !sameProjectPath(p.Path, dir) {
			kept = append(kept, p)
		}
	}
	store.Projects = kept
	if !store.hidden(dir) {
		store.Hidden = append(store.Hidden, dir)
	}
	return saveProjectStore(store)
}

// PickProjectDirectory 打开系统目录选择框；取消时返回空串
func (ps *ProjectService) PickProjectDirectory() (string, error) {
	if ps.app == nil || ps.app.ctx == nil {
		return "", errorf("窗口尚未就绪")
	}
	return wailsruntime.OpenDirectoryDialog(ps.app.ctx, wailsruntime.OpenDialogOptions{
		Title: tr("选择项目目录"),
	})
}

func maskSecret(key, value string) string {
	upper := strings.ToUpper(key)
	if !strings.Contains(upper, "KEY") && !strings.Contains(upper, "TOKEN") && !strings.Contains(upper, "SECRET") {
		return value
	}
	if len(value) <= 8 {
		return "••••"
	}
	return value[:4] + "••••" + value[len(value)-4:]
}

// GetProjectDetail 项目的 MCP、接入变量与 CLAUDE.md
func (ps *ProjectService) GetProjectDetail(path string) (ProjectDetail, error) {
	dir, err := validateProjectDir(path)
	if err != nil {
		return ProjectDetail{}, err
	}
	ps.mu.Lock()
	store, _ := loadProjectStore()
	record := store.find(dir)
	pinned := record != nil
	ps.mu.Unlock()

	detail := ProjectDetail{
		ProjectInfo:  ps.buildInfo(dir, pinned, record),
		McpServers:   []ProjectMcpServer{},
		EnvVars:      []ProjectEnvVar{},
		McpPath:      filepath.Join(dir, projectMcpFile),
		SettingsPath: filepath.Join(dir, projectLocalSettings),
		ClaudeMDPath: filepath.Join(dir, projectClaudeMD),
	}

	servers, err := projectMcpServers(dir)
	if err != nil {
		return detail, err
	}
	library := map[string]bool{}
	if ps.mcp != nil {
		if list, err := ps.mcp.ListServers(); err == nil {
			for _, s := range list {
				library[s.Name] = true
			}
		}
	}
	for name, raw := range servers {
		entry, _ := raw.(map[string]any)
		item := ProjectMcpServer{Name: name, InLibrary: library[name]}
		if entry != nil {
			item.Type, _ = entry["type"].(string)
			item.Command, _ = entry["command"].(string)
			item.URL, _ = entry["url"].(string)
			item.Args = anyToStringSlice(entry["args"])
		}
		if item.Type == "" {
			if item.URL != "" {
				item.Type = "http"
			} else {
				item.Type = "stdio"
			}
		}
		detail.McpServers = append(detail.McpServers, item)
	}
	sort.Slice(detail.McpServers, func(i, j int) bool { return detail.McpServers[i].Name < detail.McpServers[j].Name })

	env := projectEnv(dir)
	managed := map[string]bool{"CLAUDE_CODE_ATTRIBUTION_HEADER": true, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": true}
	for _, key := range claudeThirdPartyEnvKeys {
		managed[key] = true
	}
	for key, value := range env {
		if managed[key] {
			detail.EnvVars = append(detail.EnvVars, ProjectEnvVar{Key: key, Value: maskSecret(key, fmt.Sprint(value))})
		}
	}
	sort.Slice(detail.EnvVars, func(i, j int) bool { return detail.EnvVars[i].Key < detail.EnvVars[j].Key })

	if data, err := os.ReadFile(detail.ClaudeMDPath); err == nil {
		detail.ClaudeMD = string(data)
	}
	return detail, nil
}

// AddLibraryMcpToProject 把全局 MCP 库里的服务器写进项目 .mcp.json（同名覆盖，其它条目保留）
func (ps *ProjectService) AddLibraryMcpToProject(path string, names []string) error {
	dir, err := validateProjectDir(path)
	if err != nil {
		return err
	}
	if ps.mcp == nil {
		return errorf("MCP 服务未初始化")
	}
	library, err := ps.mcp.ListServers()
	if err != nil {
		return err
	}
	byName := map[string]MCPServer{}
	for _, s := range library {
		byName[s.Name] = s
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	file := filepath.Join(dir, projectMcpFile)
	obj, err := readJSONObjectFile(file)
	if err != nil {
		return err
	}
	servers, _ := obj["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	for _, name := range names {
		server, ok := byName[name]
		if !ok {
			return errorf("MCP 库里没有 %q", name)
		}
		if len(server.MissingPlaceholders) > 0 {
			return errorf("%s 还有未填写的占位符: %s", name, strings.Join(server.MissingPlaceholders, ", "))
		}
		servers[name] = buildMCPEntry(server, "headers", true)
	}
	obj["mcpServers"] = servers
	return writeJSONObjectFile(file, obj, 0o644)
}

// RemoveProjectMcp 从项目 .mcp.json 删除一个服务器
func (ps *ProjectService) RemoveProjectMcp(path, name string) error {
	dir, err := validateProjectDir(path)
	if err != nil {
		return err
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	file := filepath.Join(dir, projectMcpFile)
	obj, err := readJSONObjectFile(file)
	if err != nil {
		return err
	}
	servers, _ := obj["mcpServers"].(map[string]any)
	if _, ok := servers[name]; !ok {
		return nil
	}
	delete(servers, name)
	obj["mcpServers"] = servers
	return writeJSONObjectFile(file, obj, 0o644)
}

// ensureGitExcluded 项目是 git 仓库时把 settings.local.json 加进 .git/info/exclude（含 Key，不能提交）
func ensureGitExcluded(dir, relPath string) {
	gitDir := filepath.Join(dir, ".git")
	if !isDir(gitDir) {
		return
	}
	pattern := "/" + filepath.ToSlash(relPath)
	exclude := filepath.Join(gitDir, "info", "exclude")
	data, _ := os.ReadFile(exclude)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == pattern {
			return
		}
	}
	content := string(data)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "# AI ENV: 项目本机设置含 API Key，不提交\n" + pattern + "\n"
	if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		return
	}
	_ = writeFileAtomic(exclude, []byte(content), 0o644)
}

// ApplyEnvToProject 让这个项目使用指定的 Claude Code 配置（写入项目 .claude/settings.local.json 的 env），
// 全局配置不受影响
func (ps *ProjectService) ApplyEnvToProject(path, envName string) (string, error) {
	dir, err := validateProjectDir(path)
	if err != nil {
		return "", err
	}
	if ps.app == nil {
		return "", errorf("配置服务未初始化")
	}
	ps.app.configMu.Lock()
	env := ps.app.findEnvIn("claude", envName)
	ps.app.configMu.Unlock()
	if env == nil {
		return "", errorf("找不到 Claude Code 配置 %q", envName)
	}
	if env.OfficialLogin {
		return "", errorf("官方登录配置不能按项目应用：项目设置只能覆盖变量，无法撤销全局写入的接入信息")
	}
	if needsRouting(env) {
		return "", errorf("该配置需要经过本地路由转换协议，暂不支持按项目应用")
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()
	file := filepath.Join(dir, projectLocalSettings)
	settings, err := readJSONObjectFile(file)
	if err != nil {
		return "", err
	}
	mergeClaudeEnv(settings, env)
	if err := writeJSONObjectFile(file, settings, 0o600); err != nil {
		return "", err
	}
	ensureGitExcluded(dir, projectLocalSettings)

	store, err := loadProjectStore()
	if err != nil {
		return "", err
	}
	record := store.find(dir)
	if record == nil {
		store.Projects = append(store.Projects, projectRecord{Path: dir})
		record = &store.Projects[len(store.Projects)-1]
	}
	record.AppliedEnv = env.Name
	record.AppliedAt = time.Now().UnixMilli()
	if err := saveProjectStore(store); err != nil {
		return "", err
	}
	return sprintf("已让项目 %s 使用配置 %s（写入 %s）", filepath.Base(dir), env.Name, projectLocalSettings), nil
}

// ClearProjectEnv 移除项目 settings.local.json 里的接入变量，恢复使用全局配置
func (ps *ProjectService) ClearProjectEnv(path string) error {
	dir, err := validateProjectDir(path)
	if err != nil {
		return err
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	file := filepath.Join(dir, projectLocalSettings)
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	settings, err := readJSONObjectFile(file)
	if err != nil {
		return err
	}
	mergeClaudeEnv(settings, nil)
	if err := writeJSONObjectFile(file, settings, 0o600); err != nil {
		return err
	}
	store, err := loadProjectStore()
	if err != nil {
		return err
	}
	if record := store.find(dir); record != nil && record.AppliedEnv != "" {
		record.AppliedEnv = ""
		record.AppliedAt = 0
		return saveProjectStore(store)
	}
	return nil
}

// SaveProjectClaudeMD 保存项目 CLAUDE.md；内容为空时删除文件
func (ps *ProjectService) SaveProjectClaudeMD(path, content string) error {
	dir, err := validateProjectDir(path)
	if err != nil {
		return err
	}
	file := filepath.Join(dir, projectClaudeMD)
	if strings.TrimSpace(content) == "" {
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return writeFileAtomic(file, []byte(content), 0o644)
}
