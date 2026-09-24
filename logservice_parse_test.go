package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func claudeLine(ts, msgID, reqID, blockType string, in, out, cacheRead int) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"requestId":%q,"message":{"id":%q,"model":"claude-sonnet-4-5-20250929","content":[{"type":%q}],"usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":0}}}`,
		ts, reqID, msgID, blockType, in, out, cacheRead)
}

// Claude Code 把一次响应的每个内容块各写一行（usage 相同），恢复会话还会把历史复制进新文件：
// 同一 message.id + requestId 只能计一次
func TestClaudeLogsDedupByMessageAndRequest(t *testing.T) {
	home := withHomeRoot(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	dir := filepath.Join(home, ".claude", "projects", "-work-demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	session := strings.Join([]string{
		claudeLine(ts, "msg_1", "req_1", "thinking", 10, 100, 1000),
		claudeLine(ts, "msg_1", "req_1", "text", 10, 100, 1000),
		claudeLine(ts, "msg_1", "req_1", "tool_use", 10, 100, 1000),
		claudeLine(ts, "msg_2", "req_2", "text", 5, 50, 0),
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(session), 0o644); err != nil {
		t.Fatal(err)
	}
	// 恢复出来的新会话文件复制了 msg_1
	resumed := claudeLine(ts, "msg_1", "req_1", "text", 10, 100, 1000)
	if err := os.WriteFile(filepath.Join(dir, "b.jsonl"), []byte(resumed), 0o644); err != nil {
		t.Fatal(err)
	}

	records, err := NewLogService().readClaudeLogs(30)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("应只有 2 次响应，实际 %d 条记录", len(records))
	}
	stats := aggregateUsageStats(records, "")
	if stats.TotalOutputTokens != 150 || stats.TotalCacheRead != 1000 {
		t.Fatalf("去重后输出应为 150、缓存读取 1000，实际 %d / %d", stats.TotalOutputTokens, stats.TotalCacheRead)
	}
}

func TestClaudeProjectsDirHonorsConfigDir(t *testing.T) {
	home := withHomeRoot(t)
	custom := filepath.Join(home, "claude-custom")
	if err := os.MkdirAll(filepath.Join(custom, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", custom)
	if got, want := NewLogService().getClaudeProjectsDir(), filepath.Join(custom, "projects"); got != want {
		t.Fatalf("应读取 CLAUDE_CONFIG_DIR 下的 projects：期望 %s，实际 %s", want, got)
	}
}

// 模糊匹配必须稳定地取最长的模型名；当前在用的 Claude 模型要有自己的价格
func TestLookupModelPricing(t *testing.T) {
	cases := []struct {
		model         string
		input, output float64
		cacheRead     float64
	}{
		{"gpt-5.1-codex-mini-2026-01-15", 0.30, 1.20, 0.03},
		{"claude-haiku-4-5-20251001", 1.0, 5.0, 0.10},
		{"claude-opus-4-8", 5.0, 25.0, 0.50},
		{"claude-opus-5-5", 4.0, 20.0, 0.20},
		{"claude-opus-5", 5.0, 25.0, 0.50},
		{"claude-fable-5-1", 10.0, 50.0, 0.25},
		{"claude-sonnet-5", 2.0, 10.0, 0.20},
		{"claude-opus-4-1-20250805", 15.0, 75.0, 1.50},
		{"us.anthropic.claude-sonnet-4-5-20250929-v1:0", 3.0, 15.0, 0.30},
	}
	for _, c := range cases {
		for i := 0; i < 30; i++ { // map 遍历顺序随机，多跑几次确认结果稳定
			p, ok := lookupModelPricing(c.model)
			if !ok || p.Input != c.input || p.Output != c.output || p.CacheRead != c.cacheRead {
				t.Fatalf("%s: 得到 %+v (ok=%v)，期望 input=%v output=%v cacheRead=%v", c.model, p, ok, c.input, c.output, c.cacheRead)
			}
		}
	}
}

// Codex 的 input_tokens 已包含 cached_input_tokens：缓存部分只能按缓存价算一次
func TestCodexCachedTokensNotDoubleBilled(t *testing.T) {
	dir := t.TempDir()
	ts := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	lines := strings.Join([]string{
		fmt.Sprintf(`{"type":"turn_context","timestamp":%q,"payload":{"model":"gpt-5-codex"}}`, ts),
		fmt.Sprintf(`{"type":"event_msg","timestamp":%q,"payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":800,"output_tokens":100,"total_tokens":1100}}}}`, ts),
	}, "\n")
	path := filepath.Join(dir, "rollout.jsonl")
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	records, err := NewLogService().parseCodexSession(path, "s1", time.Time{})
	if err != nil || len(records) != 1 {
		t.Fatalf("解析失败: %v %d", err, len(records))
	}
	r := records[0]
	if r.InputTokens != 200 || r.CacheReadTokens != 800 || r.OutputTokens != 100 {
		t.Fatalf("口径应为未缓存输入 200 / 缓存 800 / 输出 100，实际 %+v", r)
	}
	want := (200*1.25 + 100*10.0 + 800*0.125) / 1_000_000
	if math.Abs(r.TotalCost-want) > 1e-12 {
		t.Fatalf("花费应为 %.8f，实际 %.8f", want, r.TotalCost)
	}
}

// Gemini 的 input 已包含 cached，thoughts（推理）按输出计费
func TestGeminiThoughtsAndCachedTokens(t *testing.T) {
	dir := t.TempDir()
	ts := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	session := fmt.Sprintf(`{"sessionId":"g1","messages":[{"id":"m1","timestamp":%q,"type":"gemini","model":"gemini-2.5-pro","tokens":{"input":1000,"output":50,"cached":600,"thoughts":200,"tool":0,"total":1250}}]}`, ts)
	path := filepath.Join(dir, "session.json")
	if err := os.WriteFile(path, []byte(session), 0o644); err != nil {
		t.Fatal(err)
	}
	records, err := NewLogService().parseGeminiSession(path, "proj", time.Time{})
	if err != nil || len(records) != 1 {
		t.Fatalf("解析失败: %v %d", err, len(records))
	}
	r := records[0]
	if r.InputTokens != 400 || r.CacheReadTokens != 600 || r.OutputTokens != 250 {
		t.Fatalf("口径应为未缓存输入 400 / 缓存 600 / 输出(含推理) 250，实际 %+v", r)
	}
}
