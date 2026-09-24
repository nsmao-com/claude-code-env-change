//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
		// 单引号包裹并转义内部单引号：Go 的 %q 不转义 $ 和反引号，
		// 双引号里的 $VAR 会被 shell 展开，非 ASCII 还会被转成 \uXXXX
		lines = append(lines, fmt.Sprintf("export %s='%s'", name, strings.ReplaceAll(state[name], "'", `'\''`)))
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
	// 当前登录 shell 的配置文件不存在时新建：macOS 默认 zsh，全新账户往往一个 rc 都没有，
	// 只有 .bashrc 的老账户切到 zsh 后也读不到，都得写进 .zshrc 才会生效
	if block != "" {
		preferred := filepath.Join(home, loginShellRCName(os.Getenv("SHELL"), runtime.GOOS))
		if _, err := os.Stat(preferred); os.IsNotExist(err) {
			if err := os.WriteFile(preferred, []byte(""), 0o644); err != nil {
				return err
			}
			targets = append(targets, preferred)
		}
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
		if err := writeFileAtomic(path, []byte(next), 0o644); err != nil {
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
			// 有头无尾（用户手动删掉了结束标记）：只去掉块头和紧随其后的托管 export 行，
			// 后面是用户自己的配置，不能一并删掉
			content = content[:start] + stripOrphanManagedLines(content[start:])
			continue
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

// stripOrphanManagedLines 去掉孤立的块头行，以及紧随其后、属于托管变量的 export 行
func stripOrphanManagedLines(rest string) string {
	lines := strings.SplitAfter(rest, "\n")
	i := 1 // 第 0 行是块头
	for ; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		managed := false
		for _, name := range antigravityManagedEnvVars {
			if strings.HasPrefix(line, "export "+name+"=") {
				managed = true
				break
			}
		}
		if !managed {
			break
		}
	}
	return strings.Join(lines[i:], "")
}

// loginShellRCName 按登录 shell 选择需要新建的 rc 文件
func loginShellRCName(shell, goos string) string {
	switch filepath.Base(strings.TrimSpace(shell)) {
	case "zsh":
		return ".zshrc"
	case "bash":
		return ".bashrc"
	}
	if goos == "darwin" {
		return ".zshrc"
	}
	return ".bashrc"
}
