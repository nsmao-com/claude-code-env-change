//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// shell 配置里的托管块标记，整体替换/移除，保证块内容精确等于期望状态
const (
	antigravityEnvBlockBegin = "# >>> ai-env antigravity >>>"
	antigravityEnvBlockEnd   = "# <<< ai-env antigravity <<<"
)

// syncAntigravityUserEnv 声明式同步：把 state 里的变量以 export 写入 shell 配置托管块。
// agy 只从进程环境读取 GEMINI_API_KEY / GOOGLE_GEMINI_BASE_URL（官方明确不加载 .env），
// macOS/Linux 上按官方建议持久化到 shell 配置。state 为空时移除整个托管块。
func syncAntigravityUserEnv(state map[string]string) error {
	if len(state) == 0 {
		return rewriteAntigravityEnvBlock("")
	}
	names := make([]string, 0, len(state))
	for name := range state {
		names = append(names, name)
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names)+2)
	lines = append(lines, antigravityEnvBlockBegin)
	for _, name := range names {
		lines = append(lines, fmt.Sprintf("export %s=%q", name, state[name]))
	}
	lines = append(lines, antigravityEnvBlockEnd)
	return rewriteAntigravityEnvBlock(strings.Join(lines, "\n"))
}

// readAntigravityUserEnv 从 shell 配置托管块解析当前持久化的 agy 变量
func readAntigravityUserEnv() map[string]string {
	out := map[string]string{}
	home, err := os.UserHomeDir()
	if err != nil {
		return out
	}
	for _, name := range []string{".zshrc", ".bashrc", ".profile"} {
		data, err := os.ReadFile(filepath.Join(home, name))
		if err != nil {
			continue
		}
		content := string(data)
		start := strings.Index(content, antigravityEnvBlockBegin)
		if start < 0 {
			continue
		}
		endRel := strings.Index(content[start:], antigravityEnvBlockEnd)
		if endRel < 0 {
			continue
		}
		block := content[start : start+endRel]
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "export ") {
				continue
			}
			parts := strings.SplitN(strings.TrimPrefix(line, "export "), "=", 2)
			if len(parts) != 2 {
				continue
			}
			value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if value != "" {
				out[parts[0]] = value
			}
		}
	}
	return out
}

func rewriteAntigravityEnvBlock(block string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	candidates := []string{".zshrc", ".bashrc", ".profile"}
	targets := make([]string, 0, len(candidates))
	for _, name := range candidates {
		path := filepath.Join(home, name)
		if _, err := os.Stat(path); err == nil {
			targets = append(targets, path)
		}
	}
	if len(targets) == 0 {
		if block == "" {
			return nil
		}
		path := filepath.Join(home, ".bashrc")
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
			return err
		}
		targets = append(targets, path)
	}
	for _, path := range targets {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		next := replaceManagedBlock(string(data), block)
		if next == string(data) {
			continue
		}
		if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
			return fmt.Errorf("写入 %s 失败: %v", path, err)
		}
	}
	return nil
}

// replaceManagedBlock 替换内容中的托管块：已有则整体替换（block 为空表示移除），没有则追加到末尾
func replaceManagedBlock(content, block string) string {
	for {
		start := strings.Index(content, antigravityEnvBlockBegin)
		if start < 0 {
			break
		}
		endRel := strings.Index(content[start:], antigravityEnvBlockEnd)
		if endRel < 0 {
			// 有头无尾：从块头开始的剩余内容一并视为旧块
			content = strings.TrimRight(content[:start], "\n")
			break
		}
		after := content[start+endRel+len(antigravityEnvBlockEnd):]
		content = strings.TrimRight(content[:start], "\n") + "\n" + strings.TrimLeft(after, "\n")
		if content == "\n" {
			content = ""
		}
	}
	if block == "" {
		return content
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + block + "\n"
}
