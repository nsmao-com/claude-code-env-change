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
		{"claude", "anthropic_messages", false}, // 同协议直连
		{"claude", "responses", false},          // 暂不支持的组合，前端不提供
		{"codex", "", false},
		{"codex", "responses", false},
		{"codex", "chat_completions", true},
		{"codex", "anthropic_messages", true},
		{"gemini", "chat_completions", false}, // gemini 不参与路由
	}
	for _, c := range cases {
		env := &EnvConfig{Provider: c.provider, UpstreamFormat: c.format}
		if got := needsRouting(env); got != c.want {
			t.Errorf("needsRouting(%s,%s) = %v, want %v", c.provider, c.format, got, c.want)
		}
	}
}
