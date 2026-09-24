package main

import (
	"path/filepath"
	"testing"
)

// 设置了 CODEX_HOME 时，环境写入、MCP、Skills、提示词必须都落到同一个目录
func TestResolveCodexHomeHonorsEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	t.Setenv("CODEX_HOME", "")
	if got, want := resolveCodexHome(home), filepath.Join(home, ".codex"); got != want {
		t.Fatalf("未设置 CODEX_HOME 时应为 %s，实际 %s", want, got)
	}

	custom := filepath.Join(home, "custom-codex")
	t.Setenv("CODEX_HOME", custom)
	if got := resolveCodexHome(home); got != custom {
		t.Fatalf("应遵循 CODEX_HOME=%s，实际 %s", custom, got)
	}

	cfg, err := codexConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(custom, "config.toml"); cfg != want {
		t.Fatalf("MCP 同步的 config.toml 应为 %s，实际 %s", want, cfg)
	}

	prompt, err := promptFilePath("codex")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(custom, "AGENTS.md"); prompt != want {
		t.Fatalf("提示词应为 %s，实际 %s", want, prompt)
	}

	t.Setenv("CODEX_HOME", "~/elsewhere")
	if got, want := resolveCodexHome(home), filepath.Join(home, "elsewhere"); got != want {
		t.Fatalf("CODEX_HOME 中的 ~ 应展开为用户目录：期望 %s，实际 %s", want, got)
	}
}

func TestSkillSyncWritesToCodexHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	custom := filepath.Join(home, "codex-home")
	t.Setenv("CODEX_HOME", custom)

	ss := NewSkillService()
	entry := rawSkill{Content: "---\nname: demo\n---\nbody", EnablePlatform: []string{platCodex}}
	if err := ss.syncSkill("demo", entry); err != nil {
		t.Fatalf("同步 Skill 失败: %v", err)
	}
	if !fileExists(filepath.Join(custom, "skills", "demo", "SKILL.md")) {
		t.Fatalf("Skill 应写入 CODEX_HOME/skills")
	}
	if fileExists(filepath.Join(home, ".codex", "skills", "demo", "SKILL.md")) {
		t.Fatalf("设置 CODEX_HOME 后不应再写入 ~/.codex/skills")
	}
}
