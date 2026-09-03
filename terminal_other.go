//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// openTerminalWithEnv 打开一个新的终端窗口，并把 vars 注入其环境。
// macOS/Linux 上托管的 export 已写入 shell 配置，新开的终端本身就能读到，
// 这里仍然注入一份，保证立即生效、不依赖配置文件加载顺序。
func openTerminalWithEnv(vars map[string]string) error {
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
		return fmt.Errorf("启动终端失败: %v", lastErr)
	}
	return fmt.Errorf("未找到可用的终端程序，请手动打开终端使用")
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
