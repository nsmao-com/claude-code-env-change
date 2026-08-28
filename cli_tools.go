package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type cliSpec struct {
	ID         string
	Name       string
	Command    string
	NpmPackage string
	UpdateArgs []string
}

type CliToolStatus struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Command        string   `json:"command"`
	Installed      bool     `json:"installed"`
	Runnable       bool     `json:"runnable"`
	CurrentVersion string   `json:"current_version"`
	LatestVersion  string   `json:"latest_version"`
	InstallPath    string   `json:"install_path"`
	InstallMethod  string   `json:"install_method"`
	ConfigDir      string   `json:"config_dir"`
	ConfigExists   bool     `json:"config_exists"`
	Platform       string   `json:"platform"`
	Upgradable     bool     `json:"upgradable"`
	Error          string   `json:"error"`
	ExtraPaths     []string `json:"extra_paths"`
	NpmPackage     string   `json:"npm_package"`
}

type CliUpgradeResult struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Log     string `json:"log"`
}

type ConfigDirInfo struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Dir     string          `json:"dir"`
	Exists  bool            `json:"exists"`
	Files   []ConfigDirFile `json:"files"`
}

type ConfigDirFile struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

var versionNumberRe = regexp.MustCompile(`\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.]+)?`)

func cliCatalog() []cliSpec {
	return []cliSpec{
		{ID: "claude", Name: "Claude Code", Command: "claude", NpmPackage: "@anthropic-ai/claude-code", UpdateArgs: []string{"update"}},
		{ID: "codex", Name: "Codex", Command: "codex", NpmPackage: "@openai/codex", UpdateArgs: []string{"update"}},
		{ID: "gemini", Name: "Gemini CLI", Command: "gemini", NpmPackage: "@google/gemini-cli"},
		{ID: "opencode", Name: "OpenCode", Command: "opencode", NpmPackage: "opencode-ai"},
		{ID: "grok", Name: "Grok", Command: "grok", NpmPackage: "@xai-official/grok", UpdateArgs: []string{"update"}},
	}
}

func platformLabel() string {
	switch runtime.GOOS {
	case "windows":
		return "Win"
	case "darwin":
		return "Mac"
	default:
		return "Linux"
	}
}

func (a *App) ListCliTools() []CliToolStatus {
	specs := cliCatalog()
	out := make([]CliToolStatus, len(specs))
	var wg sync.WaitGroup
	for i, spec := range specs {
		wg.Add(1)
		go func(i int, spec cliSpec) {
			defer wg.Done()
			out[i] = a.inspectCliTool(spec)
		}(i, spec)
	}
	wg.Wait()
	return out
}

func (a *App) ListConfigDirs() []ConfigDirInfo {
	home, _ := os.UserHomeDir()
	appData := os.Getenv("APPDATA")
	localApp := os.Getenv("LOCALAPPDATA")
	items := []ConfigDirInfo{
		configDirInfo("claude", "Claude Code", firstExistingDir(
			filepath.Join(home, ".claude"),
			filepath.Join(appData, ".claude"),
			filepath.Join(localApp, ".claude"),
		), []string{"settings.json"}),
		configDirInfo("codex", "Codex", firstExistingDir(
			filepath.Join(home, ".codex"),
			filepath.Join(appData, ".codex"),
		), []string{"config.toml", "auth.json"}),
		configDirInfo("gemini", "Gemini CLI", firstExistingDir(
			filepath.Join(home, ".gemini"),
			filepath.Join(appData, ".gemini"),
		), []string{".env", "settings.json"}),
		configDirInfo("opencode", "OpenCode", firstExistingDir(
			resolveOpencodeConfigDir(nil),
			filepath.Join(home, ".config", "opencode"),
			filepath.Join(appData, "opencode"),
		), []string{"opencode.json"}),
		configDirInfo("grok", "Grok", firstExistingDir(
			resolveGrokHome(nil),
			filepath.Join(home, ".grok"),
		), []string{"config.toml"}),
	}
	if a != nil && strings.TrimSpace(a.configPath) != "" {
		items = append(items, configDirInfo("ai-env", "AI ENV", filepath.Dir(a.configPath), []string{"config.json", "outbound-proxy.json"}))
	}
	return items
}

func configDirInfo(id, name, dir string, files []string) ConfigDirInfo {
	info := ConfigDirInfo{ID: id, Name: name, Dir: dir, Exists: dirExists(dir)}
	for _, file := range files {
		path := filepath.Join(dir, file)
		info.Files = append(info.Files, ConfigDirFile{Name: file, Path: path, Exists: fileExists(path)})
	}
	return info
}

func (a *App) OpenConfigDir(id string) error {
	for _, item := range a.ListConfigDirs() {
		if item.ID != id {
			continue
		}
		if !item.Exists {
			if err := os.MkdirAll(item.Dir, 0o755); err != nil {
				return err
			}
		}
		return openInFileManager(item.Dir)
	}
	return fmt.Errorf("未知配置目录")
}

func (a *App) OpenConfigFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("路径为空")
	}
	if !fileExists(path) {
		return openInFileManager(filepath.Dir(path))
	}
	return revealInFileManager(path)
}

func (a *App) UpgradeCliTool(id string) CliUpgradeResult {
	spec, ok := findCliSpec(id)
	if !ok {
		return CliUpgradeResult{ID: id, Success: false, Message: "未知 CLI"}
	}
	a.emitCliProgress(id, "start", "开始更新 "+spec.Name)
	result := a.upgradeOne(spec)
	a.emitCliProgress(id, "done", result.Message)
	return result
}

func (a *App) UpgradeAllCliTools() []CliUpgradeResult {
	var results []CliUpgradeResult
	for _, spec := range cliCatalog() {
		status := a.inspectCliTool(spec)
		if !status.Installed || !status.Upgradable {
			continue
		}
		a.emitCliProgress(spec.ID, "start", "开始更新 "+spec.Name)
		result := a.upgradeOne(spec)
		a.emitCliProgress(spec.ID, "done", result.Message)
		results = append(results, result)
	}
	return results
}

func (a *App) inspectCliTool(spec cliSpec) CliToolStatus {
	status := CliToolStatus{
		ID:          spec.ID,
		Name:        spec.Name,
		Command:     spec.Command,
		Platform:    platformLabel(),
		NpmPackage:  spec.NpmPackage,
		ConfigDir:   configDirForCli(spec.ID),
		ExtraPaths:  []string{},
	}
	status.ConfigExists = dirExists(status.ConfigDir)

	copies := distinctBins(append(listCommandCopies(spec.Command), lookPathAll(spec.Command)...))
	if bin := pickBin(copies); bin != "" {
		status.Installed = true
		status.InstallPath = bin
		status.InstallMethod = detectInstallMethod(bin)
		status.ExtraPaths = copies
	}

	if status.Installed {
		bin := status.InstallPath
		if bin == "" {
			bin = spec.Command
		}
		out, runErr := runTool(8*time.Second, bin, "--version")
		if runErr != nil && strings.TrimSpace(out) == "" {
			out, runErr = runTool(8*time.Second, bin, "version")
		}
		status.CurrentVersion = extractVersion(out)
		if runErr != nil && status.CurrentVersion == "" {
			status.Runnable = false
			status.Error = truncateCliError(out, runErr)
		} else {
			status.Runnable = true
			if status.CurrentVersion == "" {
				status.CurrentVersion = strings.TrimSpace(firstLine(out))
			}
		}
	}

	if spec.NpmPackage != "" {
		if latest, err := npmLatestVersion(spec.NpmPackage); err == nil {
			status.LatestVersion = latest
		}
	}
	if status.Runnable && status.CurrentVersion != "" && status.LatestVersion != "" {
		status.Upgradable = versionLess(status.CurrentVersion, status.LatestVersion)
	}
	return status
}

func (a *App) upgradeOne(spec cliSpec) CliUpgradeResult {
	status := a.inspectCliTool(spec)
	installer := chooseInstaller(status, spec)
	if installer == "" {
		return CliUpgradeResult{ID: spec.ID, Success: false, Message: spec.Name + " 未安装，且没有可用的在线安装源"}
	}
	a.emitCliProgress(spec.ID, "start", spec.Name+" 使用 "+installer+" 更新中")

	var out string
	var err error
	switch installer {
	case "native":
		bin := status.InstallPath
		if bin == "" {
			bin = spec.Command
		}
		out, err = runTool(4*time.Minute, bin, spec.UpdateArgs...)
	case "pnpm":
		pnpm := findPnpmNear(status.InstallPath)
		if pnpm == "" {
			return CliUpgradeResult{ID: spec.ID, Success: false, Message: "未找到 pnpm，无法更新 " + spec.Name}
		}
		out, err = runTool(4*time.Minute, pnpm, "add", "-g", spec.NpmPackage+"@latest")
	case "npm":
		npm := findNpm()
		if npm == "" {
			return CliUpgradeResult{ID: spec.ID, Success: false, Message: "未找到 npm，无法更新 " + spec.Name}
		}
		out, err = runTool(4*time.Minute, npm, "install", "-g", spec.NpmPackage+"@latest")
	default:
		return CliUpgradeResult{ID: spec.ID, Success: false, Message: spec.Name + " 未安装，且没有可用的在线安装源"}
	}

	if err != nil {
		return CliUpgradeResult{ID: spec.ID, Success: false, Message: truncateCliError(out, err), Log: out}
	}
	after := a.inspectCliTool(spec)
	if status.LatestVersion != "" && after.CurrentVersion != "" && versionLess(after.CurrentVersion, status.LatestVersion) {
		return CliUpgradeResult{
			ID:      spec.ID,
			Success: false,
			Message: spec.Name + " 命令已执行，但当前仍是 " + after.CurrentVersion + "（目标 " + status.LatestVersion + "）。可能更新了另一份安装，当前 PATH 指向 " + after.InstallPath,
			Log:     out,
		}
	}
	msg := spec.Name + " 已更新"
	if after.CurrentVersion != "" {
		msg += "到 " + after.CurrentVersion
	}
	msg += "（" + installer + "）"
	return CliUpgradeResult{ID: spec.ID, Success: true, Message: msg, Log: out}
}

func chooseInstaller(status CliToolStatus, spec cliSpec) string {
	method := detectInstallMethod(status.InstallPath)
	if method == "pnpm" && spec.NpmPackage != "" {
		return "pnpm"
	}
	if method == "native" && status.Installed && len(spec.UpdateArgs) > 0 {
		return "native"
	}
	if spec.NpmPackage == "" {
		if status.Installed && len(spec.UpdateArgs) > 0 {
			return "native"
		}
		return ""
	}
	if method == "npm" {
		return "npm"
	}
	if findPnpmNear(status.InstallPath) != "" {
		return "pnpm"
	}
	if findNpm() != "" {
		return "npm"
	}
	return ""
}

func (a *App) emitCliProgress(id, phase, message string) {
	if a == nil || a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "cli:progress", map[string]string{
		"id":      id,
		"phase":   phase,
		"message": message,
	})
}

func findCliSpec(id string) (cliSpec, bool) {
	for _, spec := range cliCatalog() {
		if spec.ID == id {
			return spec, true
		}
	}
	return cliSpec{}, false
}

func configDirForCli(id string) string {
	home, _ := os.UserHomeDir()
	appData := os.Getenv("APPDATA")
	localApp := os.Getenv("LOCALAPPDATA")
	switch id {
	case "claude":
		return firstExistingDir(filepath.Join(home, ".claude"), filepath.Join(appData, ".claude"), filepath.Join(localApp, ".claude"))
	case "codex":
		return firstExistingDir(filepath.Join(home, ".codex"), filepath.Join(appData, ".codex"))
	case "gemini":
		return firstExistingDir(filepath.Join(home, ".gemini"), filepath.Join(appData, ".gemini"))
	case "opencode":
		return firstExistingDir(resolveOpencodeConfigDir(nil), filepath.Join(home, ".config", "opencode"))
	case "grok":
		return firstExistingDir(resolveGrokHome(nil), filepath.Join(home, ".grok"))
	default:
		return ""
	}
}

func detectInstallMethod(path string) string {
	p := strings.ToLower(filepath.ToSlash(path))
	switch {
	case strings.Contains(p, "/pnpm/") || strings.HasSuffix(p, "/pnpm") || strings.Contains(p, "/pnpm\\"):
		return "pnpm"
	case strings.Contains(p, "node_modules"), strings.Contains(p, "/npm/"), strings.Contains(p, "/nvm/"),
		strings.Contains(p, "fnm"), strings.Contains(p, "volta"):
		return "npm"
	case strings.Contains(p, "/.grok/bin"), strings.Contains(p, "/.local/bin"), strings.Contains(p, "program files"),
		strings.Contains(p, "windowsapps"):
		return "native"
	default:
		return "unknown"
	}
}

func findPnpmNear(installPath string) string {
	if dir := filepath.Dir(strings.TrimSpace(installPath)); dir != "" && dir != "." {
		for _, name := range []string{"pnpm.cmd", "pnpm.exe", "pnpm"} {
			candidate := filepath.Join(dir, name)
			if fileExists(candidate) && !dirExists(candidate) {
				return candidate
			}
		}
	}
	return findPnpm()
}

func findPnpm() string {
	return findPkgBin("pnpm")
}

func findNpm() string {
	return findPkgBin("npm")
}

func findPkgBin(name string) string {
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath(name + ".cmd"); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	if bin := pickBin(listCommandCopies(name)); bin != "" {
		return bin
	}
	return ""
}

func runTool(timeout time.Duration, name string, args ...string) (string, error) {
	resolved := name
	if !filepath.IsAbs(name) && filepath.Base(name) == name {
		if p, err := exec.LookPath(name); err == nil {
			resolved = p
		} else if copies := listCommandCopies(name); len(copies) > 0 {
			resolved = pickBin(copies)
		}
	}
	return runToolRaw(timeout, resolved, args...)
}

func runToolRaw(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	bin, argv := wrapIfScript(name, args)
	cmd := exec.Command(bin, argv...)
	configureHiddenCmd(cmd)
	cmd.Env = os.Environ()
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		return buf.String(), err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return buf.String(), err
	case <-ctx.Done():
		killCmd(cmd)
		<-done
		return buf.String(), fmt.Errorf("命令超时")
	}
}

func killCmd(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		killer := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		configureHiddenCmd(killer)
		_ = killer.Run()
		return
	}
	_ = cmd.Process.Kill()
}

func lookPathAll(name string) []string {
	var out []string
	if p, err := exec.LookPath(name); err == nil {
		out = append(out, p)
	}
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath(name + ".cmd"); err == nil {
			out = append(out, p)
		}
		if p, err := exec.LookPath(name + ".exe"); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func distinctBins(paths []string) []string {
	best := map[string]string{}
	var order []string
	for _, path := range uniquePaths(paths) {
		resolved := path
		if real, err := filepath.EvalSymlinks(path); err == nil && strings.TrimSpace(real) != "" {
			resolved = real
		}
		dir := strings.ToLower(filepath.Clean(filepath.Dir(resolved)))
		if prev, ok := best[dir]; ok {
			best[dir] = pickBin([]string{prev, resolved})
			continue
		}
		best[dir] = resolved
		order = append(order, dir)
	}
	out := make([]string, 0, len(order))
	for _, dir := range order {
		out = append(out, best[dir])
	}
	return out
}

func scanKnownBins(name string) []string {
	var out []string
	for _, dir := range extraBinDirs() {
		if dir == "" || !dirExists(dir) {
			continue
		}
		for _, ext := range []string{".cmd", ".CMD", ".exe", ".bat", ".ps1", ""} {
			path := filepath.Join(dir, name+ext)
			if fileExists(path) && !dirExists(path) {
				out = append(out, path)
			}
		}
	}
	return out
}

func pickBin(paths []string) string {
	var exe, cmd, other string
	for _, path := range paths {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".exe":
			if exe == "" {
				exe = path
			}
		case ".cmd", ".bat":
			if cmd == "" {
				cmd = path
			}
		default:
			if other == "" {
				other = path
			}
		}
	}
	if exe != "" {
		return exe
	}
	if cmd != "" {
		return cmd
	}
	return other
}

func firstExistingDir(candidates ...string) string {
	fallback := ""
	for _, dir := range candidates {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if fallback == "" {
			fallback = dir
		}
		if dirExists(dir) {
			return dir
		}
	}
	return fallback
}

func npmLatestVersion(pkg string) (string, error) {
	raw := "https://registry.npmjs.org/" + strings.ReplaceAll(pkg, "/", "%2F") + "/latest"
	client := &http.Client{Timeout: 4 * time.Second}
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "AI-ENV/"+appVersion)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("npm registry %d", resp.StatusCode)
	}
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.Version), nil
}

func extractVersion(text string) string {
	return versionNumberRe.FindString(text)
}

func versionLess(current, latest string) bool {
	c := versionParts(current)
	l := versionParts(latest)
	n := len(c)
	if len(l) < n {
		n = len(l)
	}
	for i := 0; i < n; i++ {
		if c[i] != l[i] {
			return c[i] < l[i]
		}
	}
	return len(c) < len(l)
}

func versionParts(raw string) []int {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "v")
	match := versionNumberRe.FindString(raw)
	if match == "" {
		return nil
	}
	core := strings.SplitN(match, "-", 2)[0]
	var parts []int
	for _, piece := range strings.Split(core, ".") {
		n, _ := strconv.Atoi(piece)
		parts = append(parts, n)
	}
	return parts
}

func uniquePaths(items []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		cleaned := strings.TrimSpace(item)
		if cleaned == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(cleaned))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, filepath.Clean(cleaned))
	}
	return out
}

func splitNonEmptyLines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "\r"))
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func truncateCliError(out string, err error) string {
	msg := strings.TrimSpace(firstLine(out))
	if msg == "" && err != nil {
		msg = err.Error()
	}
	msg = strings.ReplaceAll(msg, "\n", " ")
	if len(msg) > 180 {
		return msg[:180] + "…"
	}
	return msg
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
