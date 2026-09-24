package main

import (
	"sync"
	"testing"
	"time"
)

func budgetRecord(ts time.Time, provider string, cost float64) UsageRecord {
	return UsageRecord{Timestamp: ts.Format(recordTimeLayout), Provider: provider, TotalCost: cost}
}

// 各范围的花费汇总：全部 / 平台 / 配置（按记录发生时启用的配置归属），只算本周期
func TestComputeBudgetStatus(t *testing.T) {
	now := time.Date(2026, 9, 24, 15, 0, 0, 0, time.Local)
	today := func(h int) time.Time { return time.Date(2026, 9, 24, h, 0, 0, 0, time.Local) }
	records := []UsageRecord{
		budgetRecord(today(9), "claude", 3),
		budgetRecord(today(11), "claude", 5),
		budgetRecord(today(12), "opencode", 2),
		budgetRecord(time.Date(2026, 9, 23, 22, 0, 0, 0, time.Local), "claude", 7),   // 昨天：只计入月度
		budgetRecord(time.Date(2026, 8, 31, 22, 0, 0, 0, time.Local), "claude", 100), // 上月：都不计
	}
	activations := map[string][]EnvActivationEvent{
		"claude": {
			{At: time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local).Unix(), Provider: "claude", EnvName: "relay-a"},
			{At: today(10).Unix(), Provider: "claude", EnvName: "relay-b"},
		},
	}
	rules := []BudgetRule{
		{ID: "all-day", Scope: "all", Period: "day", Limit: 10, Enabled: true},
		{ID: "claude-month", Scope: "provider", Provider: "claude", Period: "month", Limit: 100, Enabled: true},
		{ID: "b-day", Scope: "env", Provider: "claude", EnvName: "relay-b", Period: "day", Limit: 5, Enabled: true},
		{ID: "a-day", Scope: "env", Provider: "claude", EnvName: "relay-a", Period: "day", Limit: 10, Enabled: true},
	}
	got := computeBudgetStatus(rules, 80, records, activations, now)
	want := map[string]struct {
		spent float64
		level string
	}{
		"all-day":      {10, "over"}, // 3+5+2
		"claude-month": {15, "ok"},   // 3+5+7
		"b-day":        {5, "over"},  // 10 点后切到 relay-b 的 5
		"a-day":        {3, "ok"},    // 9 点仍是 relay-a
	}
	for _, st := range got {
		w := want[st.Rule.ID]
		if st.Spent != w.spent || st.Level != w.level {
			t.Errorf("%s: 花费 %.2f / 级别 %s，期望 %.2f / %s", st.Rule.ID, st.Spent, st.Level, w.spent, w.level)
		}
	}
}

// 提醒：达到提醒比例与超出上限各一次，同一周期不重复；进入新周期重新计；改了上限会重新提醒
func TestBudgetAlertsOncePerLevel(t *testing.T) {
	withHomeRoot(t)
	now := time.Date(2026, 9, 24, 15, 0, 0, 0, time.Local)
	var records []UsageRecord
	// SaveBudgetSettings 会在后台再检查一次，提醒回调可能来自另一个 goroutine
	var mu sync.Mutex
	var fired []string
	levels := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), fired...)
	}
	bs := NewBudgetService(nil, nil)
	bs.now = func() time.Time { return now }
	bs.records = func(int) []UsageRecord { return records }
	bs.activations = func() map[string][]EnvActivationEvent { return nil }
	bs.notify = func(st BudgetStatus) {
		mu.Lock()
		fired = append(fired, st.Level)
		mu.Unlock()
	}

	if err := saveBudgetStore(budgetStore{BudgetSettings: BudgetSettings{
		WarnPercent: 80,
		Rules:       []BudgetRule{{ID: "r1", Scope: "all", Period: "day", Limit: 10, Enabled: true}},
	}}); err != nil {
		t.Fatal(err)
	}
	add := func(cost float64) {
		records = append(records, budgetRecord(now.Add(-time.Minute), "claude", cost))
	}

	add(5)
	bs.check()
	add(3.5) // 85%
	bs.check()
	bs.check()
	add(2) // 105%
	bs.check()
	bs.check()
	if got := levels(); len(got) != 2 || got[0] != "warn" || got[1] != "over" {
		t.Fatalf("应依次提醒 warn、over 各一次，实际 %v", got)
	}

	// 第二天：花费清零，重新计
	now = now.Add(24 * time.Hour)
	records = nil
	add(9)
	bs.check()
	if got := levels(); len(got) != 3 || got[2] != "warn" {
		t.Fatalf("新的一天应重新提醒，实际 %v", got)
	}

	// 改了上限后按新上限重新提醒；保存触发的后台检查与这里的检查只会有一个真正提醒
	settings, _ := bs.GetBudgetSettings()
	settings.Rules[0].Limit = 5
	if err := bs.SaveBudgetSettings(settings); err != nil {
		t.Fatal(err)
	}
	bs.check()
	time.Sleep(50 * time.Millisecond)
	got := levels()
	if len(got) != 4 || got[3] != "over" {
		t.Fatalf("改低上限后应只提醒一次已超出，实际 %v", got)
	}
}

func TestBudgetRuleValidation(t *testing.T) {
	withHomeRoot(t)
	bs := NewBudgetService(nil, nil)
	bs.records = func(int) []UsageRecord { return nil }
	bs.activations = func() map[string][]EnvActivationEvent { return nil }
	bad := []BudgetRule{
		{Scope: "all", Period: "week", Limit: 1},
		{Scope: "all", Period: "day", Limit: 0},
		{Scope: "provider", Provider: "nope", Period: "day", Limit: 1},
		{Scope: "env", Provider: "claude", Period: "day", Limit: 1},
	}
	for _, r := range bad {
		if err := bs.SaveBudgetSettings(BudgetSettings{Rules: []BudgetRule{r}}); err == nil {
			t.Fatalf("应拒绝无效预算 %+v", r)
		}
	}
	if err := bs.SaveBudgetSettings(BudgetSettings{Rules: []BudgetRule{{Scope: "env", Provider: "claude", EnvName: "x", Period: "month", Limit: 20, Enabled: true}}}); err != nil {
		t.Fatal(err)
	}
	settings, _ := bs.GetBudgetSettings()
	if len(settings.Rules) != 1 || settings.Rules[0].ID == "" || settings.WarnPercent != defaultWarnPercent {
		t.Fatalf("保存后应补上 ID 与默认提醒比例: %+v", settings)
	}
}
