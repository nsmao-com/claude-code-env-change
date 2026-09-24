//go:build !windows

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// macOS 终端脚本：变量按原值导出（含单引号、$ 等特殊字符），非法变量名跳过，脚本执行后自删
func TestWriteMacTerminalScriptExportsVars(t *testing.T) {
	vars := map[string]string{
		"ANTHROPIC_API_KEY":  "sk-it's-$HOME-`x`",
		"ANTHROPIC_BASE_URL": "http://127.0.0.1:3456",
		"BAD NAME":           "skip",
		"EMPTY":              "  ",
	}
	path, err := writeMacTerminalScript(vars)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("脚本权限 = %o，期望 700", info.Mode().Perm())
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "BAD NAME") || strings.Contains(string(data), "EMPTY") {
		t.Fatalf("不应导出非法或空变量:\n%s", data)
	}

	// 去掉最后的 exec 登录 shell，改为打印变量，验证 sh 解析出的值与原值一致
	body := strings.Replace(string(data), `exec "${SHELL:-/bin/zsh}" -l`, `printf '%s\n%s' "$ANTHROPIC_API_KEY" "$ANTHROPIC_BASE_URL"`, 1)
	body = strings.Replace(body, "clear\n", "", 1)
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("/bin/sh", path).Output()
	if err != nil {
		t.Fatalf("执行脚本失败: %v", err)
	}
	want := vars["ANTHROPIC_API_KEY"] + "\n" + vars["ANTHROPIC_BASE_URL"]
	if string(out) != want {
		t.Fatalf("导出的值 = %q，期望 %q", out, want)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("脚本执行后应删除自身")
	}
}
