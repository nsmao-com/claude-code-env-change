//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 结束标记被用户删掉时，只能去掉块头和托管的 export 行，块头之后的用户配置必须保留
func TestReplaceManagedBlockOrphanBeginKeepsUserLines(t *testing.T) {
	content := "export PATH=$HOME/bin:$PATH\n" +
		antigravityEnvBlockBegin + "\n" +
		"export GEMINI_API_KEY='old'\n" +
		"export GOOGLE_GEMINI_BASE_URL='https://old'\n" +
		"alias ll='ls -la'\n" +
		"source ~/.nvm/nvm.sh\n"
	block := antigravityEnvBlockBegin + "\nexport GEMINI_API_KEY='new'\n" + antigravityEnvBlockEnd

	got := replaceManagedBlock(content, block)
	for _, keep := range []string{"export PATH=$HOME/bin:$PATH", "alias ll='ls -la'", "source ~/.nvm/nvm.sh"} {
		if !strings.Contains(got, keep) {
			t.Fatalf("用户配置 %q 被删除:\n%s", keep, got)
		}
	}
	if strings.Contains(got, "'old'") || strings.Contains(got, "https://old") {
		t.Fatalf("旧的托管变量未清理:\n%s", got)
	}
	if strings.Count(got, antigravityEnvBlockBegin) != 1 || !strings.Contains(got, "export GEMINI_API_KEY='new'") {
		t.Fatalf("新托管块写入不正确:\n%s", got)
	}

	removed := replaceManagedBlock(content, "")
	if strings.Contains(removed, antigravityEnvBlockBegin) || !strings.Contains(removed, "source ~/.nvm/nvm.sh") {
		t.Fatalf("移除托管块结果不正确:\n%s", removed)
	}
}

func TestReplaceManagedBlockReplacesCompleteBlock(t *testing.T) {
	content := "# mine\n" + antigravityEnvBlockBegin + "\nexport GEMINI_API_KEY='old'\n" + antigravityEnvBlockEnd + "\nalias g=git\n"
	block := antigravityEnvBlockBegin + "\nexport GEMINI_API_KEY='new'\n" + antigravityEnvBlockEnd
	got := replaceManagedBlock(content, block)
	if !strings.Contains(got, "# mine") || !strings.Contains(got, "alias g=git") || strings.Contains(got, "'old'") {
		t.Fatalf("替换结果不正确:\n%s", got)
	}
}

func TestLoginShellRCName(t *testing.T) {
	cases := []struct{ shell, goos, want string }{
		{"/bin/zsh", "linux", ".zshrc"},
		{"/usr/local/bin/bash", "darwin", ".bashrc"},
		{"", "darwin", ".zshrc"},
		{"/usr/bin/fish", "linux", ".bashrc"},
	}
	for _, tc := range cases {
		if got := loginShellRCName(tc.shell, tc.goos); got != tc.want {
			t.Errorf("loginShellRCName(%q, %q) = %q，期望 %q", tc.shell, tc.goos, got, tc.want)
		}
	}
}

// 只有 .bashrc 的账户在用 zsh 时，托管块也要写进 .zshrc
func TestSyncAntigravityUserEnvWritesLoginShellRC(t *testing.T) {
	home := withHomeRoot(t)
	t.Setenv("SHELL", "/bin/zsh")
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("alias g=git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncAntigravityUserEnv(map[string]string{"GEMINI_API_KEY": "k"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".zshrc", ".bashrc"} {
		data, err := os.ReadFile(filepath.Join(home, name))
		if err != nil || !strings.Contains(string(data), "export GEMINI_API_KEY='k'") {
			t.Fatalf("%s 缺少托管变量: %s (%v)", name, data, err)
		}
	}
	if got := readAntigravityUserEnv()["GEMINI_API_KEY"]; got != "k" {
		t.Fatalf("读回的值 = %q", got)
	}
}
