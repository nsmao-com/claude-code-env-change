package main

// RTK（rtk-ai.app）：让 Agent 执行的 git、ls、测试等命令输出先经 rtk 精简再交给模型，节省 Token。
// 开启时运行 rtk 自带的安装器（rtk init -g）写入钩子；关闭时只删除 rtk 写入的钩子、@RTK.md 引用与 RTK.md，
// 用户自己的其它钩子与内容保持不变。

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
)

const rtkHomepage = "https://www.rtk-ai.app"

type RTKAgent struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type RTKStatus struct {
	Installed    bool       `json:"installed"`
	Path         string     `json:"path,omitempty"`
	Version      string     `json:"version,omitempty"`
	CanInstall   bool       `json:"can_install"`
	InstallHint  string     `json:"install_hint"`
	Agents       []RTKAgent `json:"agents"`
	Homepage     string     `json:"homepage"`
	InstallError string     `json:"install_error,omitempty"`
}

func rtkClaudeDir() string { return claudeConfigHome() }

func rtkCodexDir() string { return resolveCodexHome(homeDir()) }

func fileContains(path, s string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(b), s)
}

func rtkAgentEnabled(id string) bool {
	switch id {
	case "claude":
		return fileContains(filepath.Join(rtkClaudeDir(), "settings.json"), "rtk hook claude")
	case "codex":
		return fileContains(filepath.Join(rtkCodexDir(), "hooks.json"), "rtk hook codex")
	}
	return false
}

func rtkInstallerCommand() []string {
	if goruntime.GOOS == "windows" {
		if _, err := exec.LookPath("winget"); err == nil {
			return []string{"winget", "install", "--id", "rtk-ai.rtk", "--exact", "--silent", "--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"}
		}
		return nil
	}
	if _, err := exec.LookPath("brew"); err == nil {
		return []string{"brew", "install", "rtk"}
	}
	return nil
}

// GetRTKStatus rtk 是否安装、版本与各 Agent 是否已启用
func (w *WorkbenchService) GetRTKStatus() RTKStatus {
	st := RTKStatus{Homepage: rtkHomepage, Agents: []RTKAgent{
		{ID: "claude", Name: "Claude Code", Enabled: rtkAgentEnabled("claude")},
		{ID: "codex", Name: "Codex", Enabled: rtkAgentEnabled("codex")},
	}}
	if bin := findCliBinary("rtk"); bin != "" {
		st.Installed, st.Path = true, bin
		if out, err := runToolRaw(5*time.Second, bin, "--version"); err == nil {
			st.Version = strings.TrimSpace(firstLine(out))
		}
	}
	st.CanInstall = rtkInstallerCommand() != nil
	if goruntime.GOOS == "windows" {
		st.InstallHint = "通过 winget 安装 rtk-ai.rtk"
	} else {
		st.InstallHint = "通过 Homebrew 安装，或按官网说明安装"
	}
	return st
}

// InstallRTK 用系统包管理器安装 rtk
func (w *WorkbenchService) InstallRTK() (RTKStatus, error) {
	cmd := rtkInstallerCommand()
	if cmd == nil {
		return w.GetRTKStatus(), fmt.Errorf("没有可用的安装方式，请按 %s 的说明手动安装", rtkHomepage)
	}
	out, err := runToolRaw(10*time.Minute, cmd[0], cmd[1:]...)
	augmentPathAfterInstall()
	st := w.GetRTKStatus()
	if err != nil && !st.Installed {
		return st, fmt.Errorf("安装 rtk 失败: %s", clipText(strings.TrimSpace(out), 300))
	}
	if !st.Installed {
		return st, fmt.Errorf("安装命令已执行，但还没找到 rtk；可能需要重新打开 AI ENV 让新的 PATH 生效")
	}
	return st, nil
}

// augmentPathAfterInstall winget 安装后的新 PATH 当前进程看不到，补上常见目录
func augmentPathAfterInstall() {
	if goruntime.GOOS != "windows" {
		return
	}
	local := os.Getenv("LOCALAPPDATA")
	dirs := []string{filepath.Join(local, "Microsoft", "WinGet", "Links"), filepath.Join(homeDir(), ".local", "bin")}
	path := os.Getenv("PATH")
	for _, d := range dirs {
		if dirExists(d) && !strings.Contains(strings.ToLower(path), strings.ToLower(d)) {
			path = d + string(os.PathListSeparator) + path
		}
	}
	_ = os.Setenv("PATH", path)
}

// SetRTKAgent 为某个 Agent 开启或关闭 rtk
func (w *WorkbenchService) SetRTKAgent(agent string, on bool) (RTKStatus, error) {
	if agent != "claude" && agent != "codex" {
		return w.GetRTKStatus(), fmt.Errorf("不支持的 Agent")
	}
	if !on {
		return w.GetRTKStatus(), removeRTKHooks(agent)
	}
	bin := findCliBinary("rtk")
	if bin == "" {
		return w.GetRTKStatus(), fmt.Errorf("还没有安装 rtk")
	}
	args := []string{"init", "-g"}
	dir := rtkClaudeDir()
	if agent == "codex" {
		args = append(args, "--codex")
		dir = rtkCodexDir()
	} else {
		args = append(args, "--auto-patch")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return w.GetRTKStatus(), err
	}
	for _, f := range []string{"settings.json", "hooks.json", "CLAUDE.md", "AGENTS.md"} {
		if p := filepath.Join(dir, f); fileExists(p) {
			_, _ = backupFile(p)
		}
	}
	out, err := runToolRaw(2*time.Minute, bin, args...)
	if !rtkAgentEnabled(agent) {
		msg := clipText(strings.TrimSpace(out), 300)
		if err != nil && msg == "" {
			msg = err.Error()
		}
		return w.GetRTKStatus(), fmt.Errorf("rtk 安装器没有写入钩子：%s", msg)
	}
	return w.GetRTKStatus(), nil
}

// removeRTKHooks 只删除 rtk 写入的部分
func removeRTKHooks(agent string) error {
	dir, settings, mark, doc := rtkClaudeDir(), "settings.json", "rtk hook claude", "CLAUDE.md"
	if agent == "codex" {
		dir, settings, mark, doc = rtkCodexDir(), "hooks.json", "rtk hook codex", "AGENTS.md"
	}
	if err := dropHookCommands(filepath.Join(dir, settings), mark); err != nil {
		return err
	}
	if err := dropRTKReference(filepath.Join(dir, doc)); err != nil {
		return err
	}
	if p := filepath.Join(dir, "RTK.md"); fileExists(p) {
		_, _ = backupFile(p)
		if err := os.Remove(p); err != nil {
			return err
		}
	}
	return nil
}

// dropHookCommands 从 hooks.<事件> 列表中删除命令包含 mark 的钩子，变空的分组一并删除
func dropHookCommands(path, mark string) error {
	if !fileContains(path, mark) {
		return nil
	}
	doc, err := readJSONDoc(path)
	if err != nil {
		return err
	}
	hooks, _ := doc["hooks"].(map[string]any)
	for event, raw := range hooks {
		list, _ := raw.([]any)
		kept := []any{}
		for _, item := range list {
			group, _ := item.(map[string]any)
			if cmd, _ := group["command"].(string); strings.Contains(cmd, mark) {
				continue
			}
			if inner, ok := group["hooks"].([]any); ok {
				keptInner := []any{}
				for _, h := range inner {
					hm, _ := h.(map[string]any)
					if cmd, _ := hm["command"].(string); strings.Contains(cmd, mark) {
						continue
					}
					keptInner = append(keptInner, h)
				}
				if len(keptInner) == 0 {
					continue
				}
				group["hooks"] = keptInner
			}
			kept = append(kept, item)
		}
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}
	if len(hooks) == 0 {
		delete(doc, "hooks")
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return writeAgentFile(path, append(b, '\n'))
}

// dropRTKReference 删除 CLAUDE.md / AGENTS.md 中引用 RTK.md 的 @ 行
func dropRTKReference(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(raw), "\n")
	kept := make([]string, 0, len(lines))
	changed := false
	for _, line := range lines {
		t := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if strings.HasPrefix(t, "@") && (t == "@RTK.md" || strings.HasSuffix(t, "/RTK.md") || strings.HasSuffix(t, `\RTK.md`)) {
			changed = true
			continue
		}
		kept = append(kept, line)
	}
	if !changed {
		return nil
	}
	return writeAgentFile(path, []byte(strings.Join(kept, "\n")))
}
