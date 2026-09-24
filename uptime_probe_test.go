package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAuthProbe(t *testing.T) {
	cases := []struct {
		name       string
		env        EnvConfig
		wantURL    string
		wantHeader [2]string
		ok         bool
	}{
		{
			name:       "Claude AUTH_TOKEN 走 Bearer",
			env:        EnvConfig{Provider: "claude", Variables: map[string]string{"ANTHROPIC_BASE_URL": "https://relay.example.com", "ANTHROPIC_AUTH_TOKEN": "tok"}},
			wantURL:    "https://relay.example.com/v1/models",
			wantHeader: [2]string{"Authorization", "Bearer tok"},
			ok:         true,
		},
		{
			name:       "Claude API_KEY 走 x-api-key",
			env:        EnvConfig{Provider: "claude", Variables: map[string]string{"ANTHROPIC_BASE_URL": "https://relay.example.com/", "ANTHROPIC_API_KEY": "sk"}},
			wantURL:    "https://relay.example.com/v1/models",
			wantHeader: [2]string{"x-api-key", "sk"},
			ok:         true,
		},
		{
			name:       "Codex Base URL 已带 /v1 不重复",
			env:        EnvConfig{Provider: "codex", Variables: map[string]string{"base_url": "https://api.example.com/v1", "OPENAI_API_KEY": "sk-o"}},
			wantURL:    "https://api.example.com/v1/models",
			wantHeader: [2]string{"Authorization", "Bearer sk-o"},
			ok:         true,
		},
		{
			name:       "Antigravity 用 Gemini 协议",
			env:        EnvConfig{Provider: "antigravity", Variables: map[string]string{"GOOGLE_GEMINI_BASE_URL": "https://g.example.com", "GEMINI_API_KEY": "g"}},
			wantURL:    "https://g.example.com/v1beta/models",
			wantHeader: [2]string{"x-goog-api-key", "g"},
			ok:         true,
		},
		{
			name:       "Claude 经路由转到 Chat Completions 上游，Key 按 OpenAI 协议校验",
			env:        EnvConfig{Provider: "claude", UpstreamFormat: UpstreamChatCompletions, Variables: map[string]string{"ANTHROPIC_BASE_URL": "https://oa.example.com/v1", "ANTHROPIC_AUTH_TOKEN": "k"}},
			wantURL:    "https://oa.example.com/v1/models",
			wantHeader: [2]string{"Authorization", "Bearer k"},
			ok:         true,
		},
		{
			name: "没有 Key 时退回可达性检测",
			env:  EnvConfig{Provider: "claude", Variables: map[string]string{"ANTHROPIC_BASE_URL": "https://relay.example.com"}},
			ok:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			probe, ok := buildAuthProbe(tc.env)
			if ok != tc.ok {
				t.Fatalf("ok=%v，期望 %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if probe.URL != tc.wantURL {
				t.Errorf("URL=%s，期望 %s", probe.URL, tc.wantURL)
			}
			if got := probe.Headers[tc.wantHeader[0]]; got != tc.wantHeader[1] {
				t.Errorf("%s=%q，期望 %q（全部头：%v）", tc.wantHeader[0], got, tc.wantHeader[1], probe.Headers)
			}
		})
	}
}

func TestClassifyAuthProbe(t *testing.T) {
	cases := []struct {
		status   int
		protocol string
		body     string
		success  bool
		contains string
	}{
		{200, "openai", `{"data":[]}`, true, ""},
		{401, "anthropic", `{"error":{"type":"authentication_error","message":"invalid x-api-key"}}`, false, "invalid x-api-key"},
		{402, "openai", `{"error":"余额不足"}`, false, "余额不足"},
		{403, "openai", ``, false, "HTTP 403"},
		{429, "openai", ``, false, "429"},
		{503, "openai", ``, false, "503"},
		{404, "openai", `not found`, true, ""},
		{400, "gemini", `{"error":{"message":"API key not valid"}}`, false, "API key not valid"},
		{400, "openai", ``, true, ""},
	}
	for _, tc := range cases {
		ok, msg := classifyAuthProbe(tc.status, tc.protocol, []byte(tc.body))
		if ok != tc.success {
			t.Errorf("HTTP %d (%s): success=%v，期望 %v", tc.status, tc.protocol, ok, tc.success)
		}
		if tc.contains != "" && !strings.Contains(msg, tc.contains) {
			t.Errorf("HTTP %d: 错误信息 %q 应包含 %q", tc.status, msg, tc.contains)
		}
	}
}

// 端到端：Key 被上游拒绝（可达但 401）时，鉴权探测判失败并触发轮换；
// 可达性模式下同样的上游会被当成健康，这正是新增鉴权探测要解决的问题
func TestUptimeAuthProbeTriggersRotation(t *testing.T) {
	home := withHomeRoot(t)
	t.Setenv("CODEX_HOME", "")
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") == "Bearer good" {
			w.Write([]byte(`{"data":[]}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"quota exhausted"}}`))
	}))
	defer server.Close()

	app := &App{configPath: filepath.Join(home, "config.json")}
	app.config = Config{
		CurrentEnvCodex: "bad",
		Environments: []EnvConfig{
			{Name: "bad", Provider: "codex", Variables: map[string]string{"base_url": server.URL + "/v1", "OPENAI_API_KEY": "bad", "model": "gpt-4o"}},
			{Name: "good", Provider: "codex", Variables: map[string]string{"base_url": server.URL + "/v1", "OPENAI_API_KEY": "good", "model": "gpt-4o"}},
		},
	}
	us := NewUptimeService(app)
	if err := us.SaveRotationGroup(RotationGroup{Name: "codex-pool", Provider: "codex", EnvNames: []string{"bad", "good"}, Enabled: true, FailureThreshold: 1}); err != nil {
		t.Fatalf("保存轮换组失败: %v", err)
	}

	// 可达性模式：401 视为在线，不轮换
	if err := us.SaveSettings(UptimeSettings{Enabled: true, TimeoutSeconds: 5, ProbeMode: uptimeProbeReachability}); err != nil {
		t.Fatal(err)
	}
	if _, err := us.RunOnce(); err != nil {
		t.Fatalf("检测失败: %v", err)
	}
	if got := app.GetConfig().CurrentEnvCodex; got != "bad" {
		t.Fatalf("可达性模式不应轮换，当前 %s", got)
	}

	// 鉴权模式：bad 的 Key 被拒，切到 good
	if err := us.SaveSettings(UptimeSettings{Enabled: true, TimeoutSeconds: 5, ProbeMode: uptimeProbeAuth}); err != nil {
		t.Fatal(err)
	}
	snap, err := us.RunOnce()
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}
	if got := app.GetConfig().CurrentEnvCodex; got != "good" {
		t.Fatalf("鉴权模式应轮换到 good，当前 %s（%s）", got, snap.LastRotationError)
	}
	if !strings.Contains(snap.LastRotation, "quota exhausted") {
		t.Errorf("轮换说明应带上失败原因: %q", snap.LastRotation)
	}
	last := snap.History[uptimeEnvKey("codex", "bad")]
	if len(last) == 0 || last[len(last)-1].Success || last[len(last)-1].StatusCode != http.StatusUnauthorized {
		t.Errorf("bad 的最近一次检测应记为 401 失败: %+v", last)
	}
	if snap.Settings.ProbeMode != uptimeProbeAuth {
		t.Errorf("快照应带回探测模式: %q", snap.Settings.ProbeMode)
	}
}

func TestCurrentEnvNameByProviderClaudeDesktop(t *testing.T) {
	cfg := Config{CurrentEnvClaude: "code", CurrentEnvClaudeDesktop: "desk"}
	if got := currentEnvNameByProvider(cfg, "claude_desktop"); got != "desk" {
		t.Fatalf("Claude Desktop 轮换组应读取 Claude Desktop 的当前配置，实际 %q", got)
	}
}
