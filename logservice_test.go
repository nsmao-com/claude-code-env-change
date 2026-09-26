package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStatsOverviewMatchesLegacy 验证合并接口与旧接口聚合结果一致。
// 使用固定临时日志，避免运行中的本机会话改变两次读取之间的数据。
func TestStatsOverviewMatchesLegacy(t *testing.T) {
	home := withHomeRoot(t)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	dir := filepath.Join(home, ".claude", "projects", "fixture")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data := fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"id":"fixture","model":"claude-sonnet-4-20250514","usage":{"input_tokens":100,"output_tokens":20}}}`+"\n", time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(dir, "session.jsonl"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	ls := NewLogService()

	overview, err := ls.GetStatsOverview(7, 182, "all")
	if err != nil {
		t.Fatalf("GetStatsOverview: %v", err)
	}
	if overview.Stats.TotalRequests != 1 {
		t.Fatalf("expected one fixture request, got %d", overview.Stats.TotalRequests)
	}

	legacyStats, err := ls.GetUsageStats(7, "all")
	if err != nil {
		t.Fatalf("GetUsageStats: %v", err)
	}
	legacyHeatmap, err := ls.GetHeatmapData(182, "all")
	if err != nil {
		t.Fatalf("GetHeatmapData: %v", err)
	}

	if overview.Stats.TotalRequests != legacyStats.TotalRequests {
		t.Errorf("requests mismatch: overview=%d legacy=%d", overview.Stats.TotalRequests, legacyStats.TotalRequests)
	}
	if overview.Stats.TotalInputTokens != legacyStats.TotalInputTokens {
		t.Errorf("input tokens mismatch: overview=%d legacy=%d", overview.Stats.TotalInputTokens, legacyStats.TotalInputTokens)
	}
	if overview.Stats.TotalOutputTokens != legacyStats.TotalOutputTokens {
		t.Errorf("output tokens mismatch: overview=%d legacy=%d", overview.Stats.TotalOutputTokens, legacyStats.TotalOutputTokens)
	}
	// 并发读取时记录累加顺序不定，浮点成本可能有极小误差，用容差比较
	if math.Abs(overview.Stats.TotalCost-legacyStats.TotalCost) > 1e-6 {
		t.Errorf("cost mismatch: overview=%.6f legacy=%.6f", overview.Stats.TotalCost, legacyStats.TotalCost)
	}
	if len(overview.Stats.Series) != len(legacyStats.Series) {
		t.Errorf("series length mismatch: overview=%d legacy=%d", len(overview.Stats.Series), len(legacyStats.Series))
	}

	if len(overview.Heatmap) != len(legacyHeatmap) {
		t.Fatalf("heatmap length mismatch: overview=%d legacy=%d", len(overview.Heatmap), len(legacyHeatmap))
	}
	for i := range overview.Heatmap {
		a, b := overview.Heatmap[i], legacyHeatmap[i]
		if a.Date != b.Date || a.Requests != b.Requests || a.Tokens != b.Tokens || math.Abs(a.Cost-b.Cost) > 1e-6 {
			t.Errorf("heatmap[%d] mismatch: overview=%+v legacy=%+v", i, a, b)
		}
	}

	t.Logf("overview: requests=%d series=%d heatmapDays=%d logDir=%s",
		overview.Stats.TotalRequests, len(overview.Stats.Series), len(overview.Heatmap), overview.LogDirectory)

	for _, platform := range []string{"claude_desktop", "opencode", "grok", "unknown"} {
		t.Run(platform+"_does_not_fall_back_to_all", func(t *testing.T) {
			filtered, err := ls.GetStatsOverview(7, 182, platform)
			if err != nil {
				t.Fatal(err)
			}
			if filtered.Stats.TotalRequests != 0 || len(filtered.Heatmap) != 0 || len(filtered.EnvSummary) != 0 {
				t.Fatalf("unsupported platform returned another tool's usage: %+v", filtered)
			}
			logs, err := ls.GetRecentLogs(50, platform)
			if err != nil || len(logs) != 0 {
				t.Fatalf("unsupported platform returned logs: %v, %v", logs, err)
			}
		})
	}
}
