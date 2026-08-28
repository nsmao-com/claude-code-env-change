package main

import "testing"

func TestSanitizeAutoRouteName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"我的Claude配置", "env-claude"},
		{"My Config_01", "env-my-config_01"},
		{"  spaces  ", "env-spaces"},
		{"---", "env"},
		{"", "env"},
	}
	for _, c := range cases {
		if got := sanitizeAutoRouteName(c.in); got != c.want {
			t.Errorf("sanitizeAutoRouteName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if !routeNamePattern.MatchString(sanitizeAutoRouteName("任意名字!@#")) {
		t.Error("生成的路由名不满足 routeNamePattern")
	}
}

func TestNeedsRouting(t *testing.T) {
	cases := []struct {
		provider, format string
		want             bool
	}{
		{"claude", "", false},
		{"claude", "chat_completions", true},
		{"claude", "anthropic_messages", false},
		{"claude", "responses", true},
		{"codex", "", false},
		{"codex", "responses", false},
		{"codex", "chat_completions", true},
		{"codex", "anthropic_messages", true},
		{"gemini", "chat_completions", true},
		{"opencode", "anthropic_messages", true},
		{"grok", "chat_completions", true},
		{"grok", "", false},
	}
	for _, c := range cases {
		env := &EnvConfig{Provider: c.provider, UpstreamFormat: c.format}
		if got := needsRouting(env); got != c.want {
			t.Errorf("needsRouting(%s,%s) = %v, want %v", c.provider, c.format, got, c.want)
		}
	}
}
