package main

import (
	"math"
	"testing"
)

// TestStatsOverviewMatchesLegacy 验证合并接口与旧接口聚合结果一致。
// 在真实机器上会带着本机日志数据运行；无日志数据时两者都为空，同样通过。
func TestStatsOverviewMatchesLegacy(t *testing.T) {
	ls := NewLogService()

	overview, err := ls.GetStatsOverview(7, 182, "all")
	if err != nil {
		t.Fatalf("GetStatsOverview: %v", err)
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
}
