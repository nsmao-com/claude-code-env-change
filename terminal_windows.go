//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// openTerminalWithEnv 打开一个新的终端窗口，并把 vars 注入其环境。
// 用于「一键打开已配置的终端」：子终端继承注入的变量，agy 等只认环境变量的 CLI 可直接运行，
// 不必等待用户级环境变量传播到已运行的 Shell。
func openTerminalWithEnv(vars map[string]string) error {
	env := mergedEnvWithOverrides(vars)
	for _, name := range []string{"wt.exe", "wt"} {
		if path, err := exec.LookPath(name); err == nil {
			cmd := exec.Command(path)
			cmd.Env = env
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("启动 Windows Terminal 失败: %v", err)
			}
			return nil
		}
	}
	cmd := exec.Command("cmd.exe", "/K", "title AI ENV 终端")
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动终端失败: %v", err)
	}
	return nil
}

// mergedEnvWithOverrides 在当前环境基础上覆盖指定变量（同名旧值被替换，避免重复键）
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
