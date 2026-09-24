//go:build !windows

package main

import (
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// openTerminalWithEnv 打开一个新的终端窗口，并把 vars 注入其环境。
// macOS/Linux 上托管的 export 已写入 shell 配置，新开的终端本身就能读到，
// 这里仍然注入一份，保证立即生效、不依赖配置文件加载顺序。
func openTerminalWithEnv(vars map[string]string) error {
	if runtime.GOOS == "darwin" {
		return openMacTerminalWithEnv(vars)
	}
	env := mergedEnvWithOverrides(vars)
	candidates := [][]string{
		{"x-terminal-emulator"},
		{"gnome-terminal"},
		{"konsole"},
		{"alacritty"},
		{"kitty"},
		{"wezterm", "start"},
	}
	var lastErr error
	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(path, candidate[1:]...)
		cmd.Env = env
		if err := cmd.Start(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr != nil {
		return errorf("启动终端失败: %v", lastErr)
	}
	return errorf("未找到可用的终端程序，请手动打开终端使用")
}

func mergedEnvWithOverrides(vars map[string]string) []string {
	override := make(map[string]string, len(vars))
	for key, value := range vars {
		if strings.TrimSpace(value) == "" {
			continue
		}
		override[key] = value
	}
	env := make([]string, 0, len(os.Environ())+len(override))
	for _, entry := range os.Environ() {
		key := strings.SplitN(entry, "=", 2)[0]
		if _, ok := override[key]; ok {
			continue
		}
		env = append(env, entry)
	}
	for key, value := range override {
		env = append(env, key+"="+value)
	}
	return env
}

var shellVarNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// openMacTerminalWithEnv macOS 上的终端由 launchd 拉起，不继承本进程环境，上面那些
// Linux 终端也都不存在。改为生成一次性 .command 脚本：导出变量后进入登录 shell，
// 脚本一运行就删除自身（里面有 API Key）。
func openMacTerminalWithEnv(vars map[string]string) error {
	path, err := writeMacTerminalScript(vars)
	if err != nil {
		return err
	}
	if out, err := exec.Command("open", path).CombinedOutput(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("启动终端失败: %v %s", err, strings.TrimSpace(string(out)))
	}
	// 终端没能执行脚本时兜底清理，避免含密钥的脚本长期留在临时目录
	time.AfterFunc(2*time.Minute, func() { _ = os.Remove(path) })
	return nil
}

func writeMacTerminalScript(vars map[string]string) (string, error) {
	names := make([]string, 0, len(vars))
	for name, value := range vars {
		if shellVarNamePattern.MatchString(name) && strings.TrimSpace(value) != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("rm -f -- \"$0\"\n")
	for _, name := range names {
		b.WriteString("export " + name + "='" + strings.ReplaceAll(vars[name], "'", `'\''`) + "'\n")
	}
	b.WriteString("clear\n")
	b.WriteString("exec \"${SHELL:-/bin/zsh}\" -l\n")

	f, err := os.CreateTemp("", "ai-env-terminal-*.command")
	if err != nil {
		return "", fmt.Errorf("创建终端脚本失败: %v", err)
	}
	path := f.Name()
	_, werr := f.WriteString(b.String())
	cerr := f.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(path, 0o700)
	}
	if werr != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("写入终端脚本失败: %v", werr)
	}
	return path, nil
}
