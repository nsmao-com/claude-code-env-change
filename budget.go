package main

// 花费预算：按全部平台 / 单个平台 / 单个配置设每日或每月上限（美元，与统计页同一套价格估算）。
// 后台每 10 分钟算一次，花费达到提醒比例（默认 80%）与超出上限时各提醒一次
// （界面通知 + Windows 托盘通知），同一周期内同一级别不重复提醒。只提醒，不拦截请求。

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	budgetStoreFile     = "budget.json"
	budgetCheckInterval = 10 * time.Minute
	defaultWarnPercent  = 80
)

// BudgetRule 一条预算
type BudgetRule struct {
	ID       string  `json:"id"`
	Scope    string  `json:"scope"`              // all | provider | env
	Provider string  `json:"provider,omitempty"` // scope 为 provider / env 时必填
	EnvName  string  `json:"env_name,omitempty"` // scope 为 env 时必填
	Period   string  `json:"period"`             // day | month
	Limit    float64 `json:"limit"`              // 美元
	Enabled  bool    `json:"enabled"`
}

// BudgetSettings 预算设置
type BudgetSettings struct {
	Rules       []BudgetRule `json:"rules"`
	WarnPercent int          `json:"warn_percent"`
}

// BudgetStatus 一条预算在当前周期的花费
type BudgetStatus struct {
	Rule    BudgetRule `json:"rule"`
	Spent   float64    `json:"spent"`
	Percent float64    `json:"percent"`
	Level   string     `json:"level"` // ok | warn | over
	Since   string     `json:"since"` // 本周期起点（本地时间）
}

type budgetAlertState struct {
	Period string `json:"period"`
	Level  string `json:"level"`
}

type budgetStore struct {
	BudgetSettings
	// Alerts 每条预算在当前周期已提醒到的级别，重启后不重复提醒
	Alerts map[string]budgetAlertState `json:"alerts,omitempty"`
}

// BudgetService 预算
type BudgetService struct {
	mu   sync.Mutex
	app  *App
	logs *LogService
	stop chan struct{}

	// 以下可在测试中替换
	now         func() time.Time
	records     func(days int) []UsageRecord
	activations func() map[string][]EnvActivationEvent
	notify      func(BudgetStatus)
}

func NewBudgetService(app *App, logs *LogService) *BudgetService {
	bs := &BudgetService{app: app, logs: logs, now: time.Now}
	bs.records = func(days int) []UsageRecord {
		if bs.logs == nil {
			return nil
		}
		return bs.logs.loadRecordsForPlatform(days, "all")
	}
	bs.activations = func() map[string][]EnvActivationEvent {
		events, err := LoadEnvActivations()
		if err != nil {
			return map[string][]EnvActivationEvent{}
		}
		return events
	}
	bs.notify = bs.defaultNotify
	return bs
}

// OnStartup 启动后稍等再算第一次（避开启动时读日志的高峰），之后定时检查
func (bs *BudgetService) OnStartup() {
	bs.mu.Lock()
	if bs.stop != nil {
		bs.mu.Unlock()
		return
	}
	bs.stop = make(chan struct{})
	stop := bs.stop
	bs.mu.Unlock()
	go func() {
		timer := time.NewTimer(time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-stop:
				return
			case <-timer.C:
				bs.check()
				timer.Reset(budgetCheckInterval)
			}
		}
	}()
}

func budgetStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, budgetStoreFile), nil
}

func loadBudgetStore() (budgetStore, error) {
	store := budgetStore{BudgetSettings: BudgetSettings{WarnPercent: defaultWarnPercent}}
	path, err := budgetStorePath()
	if err != nil {
		return store, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return store, err
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return store, errorf("解析 %s 失败: %v", path, err)
	}
	if store.WarnPercent <= 0 || store.WarnPercent >= 100 {
		store.WarnPercent = defaultWarnPercent
	}
	return store, nil
}

func saveBudgetStore(store budgetStore) error {
	path, err := budgetStorePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o600)
}

func newBudgetID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("b%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

var budgetProviders = map[string]bool{"claude": true, "claude_desktop": true, "codex": true, "antigravity": true, "opencode": true, "grok": true}

func normalizeBudgetRule(rule BudgetRule) (BudgetRule, error) {
	rule.Scope = strings.ToLower(strings.TrimSpace(rule.Scope))
	rule.Provider = strings.ToLower(strings.TrimSpace(rule.Provider))
	rule.EnvName = strings.TrimSpace(rule.EnvName)
	rule.Period = strings.ToLower(strings.TrimSpace(rule.Period))
	if rule.Period != "day" && rule.Period != "month" {
		return rule, errorf("预算周期只能是每日或每月")
	}
	if math.IsNaN(rule.Limit) || math.IsInf(rule.Limit, 0) || rule.Limit <= 0 {
		return rule, errorf("预算金额必须大于 0")
	}
	switch rule.Scope {
	case "all":
		rule.Provider, rule.EnvName = "", ""
	case "provider":
		if !budgetProviders[rule.Provider] {
			return rule, errorf("请选择平台")
		}
		rule.EnvName = ""
	case "env":
		if !budgetProviders[rule.Provider] || rule.EnvName == "" {
			return rule, errorf("请选择配置")
		}
	default:
		return rule, errorf("未知的预算范围 %q", rule.Scope)
	}
	if strings.TrimSpace(rule.ID) == "" {
		rule.ID = newBudgetID()
	}
	return rule, nil
}

// GetBudgetSettings 读取预算设置
func (bs *BudgetService) GetBudgetSettings() (BudgetSettings, error) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	store, err := loadBudgetStore()
	if store.Rules == nil {
		store.Rules = []BudgetRule{}
	}
	return store.BudgetSettings, err
}

// SaveBudgetSettings 保存预算；改动过的预算清掉提醒记录，按新上限重新提醒
func (bs *BudgetService) SaveBudgetSettings(settings BudgetSettings) error {
	if settings.WarnPercent <= 0 || settings.WarnPercent >= 100 {
		settings.WarnPercent = defaultWarnPercent
	}
	rules := make([]BudgetRule, 0, len(settings.Rules))
	for _, r := range settings.Rules {
		normalized, err := normalizeBudgetRule(r)
		if err != nil {
			return err
		}
		rules = append(rules, normalized)
	}
	bs.mu.Lock()
	store, err := loadBudgetStore()
	if err != nil {
		bs.mu.Unlock()
		return err
	}
	old := map[string]BudgetRule{}
	for _, r := range store.Rules {
		old[r.ID] = r
	}
	alerts := map[string]budgetAlertState{}
	for _, r := range rules {
		if prev, ok := old[r.ID]; ok && prev == r && settings.WarnPercent == store.WarnPercent {
			if state, ok := store.Alerts[r.ID]; ok {
				alerts[r.ID] = state
			}
		}
	}
	store.BudgetSettings = BudgetSettings{Rules: rules, WarnPercent: settings.WarnPercent}
	store.Alerts = alerts
	err = saveBudgetStore(store)
	bs.mu.Unlock()
	if err != nil {
		return err
	}
	notifyCloudSync()
	// 新上限可能已经超了，立即检查一次
	go bs.check()
	return nil
}

// GetBudgetStatus 各条预算在当前周期的花费（只计算，不提醒）
func (bs *BudgetService) GetBudgetStatus() ([]BudgetStatus, error) {
	bs.mu.Lock()
	store, err := loadBudgetStore()
	bs.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return bs.compute(store.BudgetSettings), nil
}

func (bs *BudgetService) compute(settings BudgetSettings) []BudgetStatus {
	enabled := make([]BudgetRule, 0, len(settings.Rules))
	for _, r := range settings.Rules {
		if r.Enabled {
			enabled = append(enabled, r)
		}
	}
	if len(enabled) == 0 {
		return []BudgetStatus{}
	}
	now := bs.now()
	// 月度预算最多回看到上月最后一天之后，32 天足够覆盖
	return computeBudgetStatus(enabled, settings.WarnPercent, bs.records(32), bs.activations(), now)
}

func budgetPeriodStart(period string, now time.Time) time.Time {
	y, m, d := now.Date()
	if period == "month" {
		return time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
	}
	return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
}

func budgetPeriodKey(period string, now time.Time) string {
	if period == "month" {
		return now.Format("2006-01")
	}
	return now.Format("2006-01-02")
}

// computeBudgetStatus 纯计算：按规则汇总本周期花费。
// 配置级预算按"记录发生时该平台启用的是哪套配置"归属，与统计页的配置用量一致。
func computeBudgetStatus(rules []BudgetRule, warnPercent int, records []UsageRecord, activations map[string][]EnvActivationEvent, now time.Time) []BudgetStatus {
	if warnPercent <= 0 || warnPercent >= 100 {
		warnPercent = defaultWarnPercent
	}
	out := make([]BudgetStatus, 0, len(rules))
	for _, rule := range rules {
		start := budgetPeriodStart(rule.Period, now)
		since := start.Format(recordTimeLayout)
		spent := 0.0
		for _, rec := range records {
			if rec.Timestamp < since {
				continue
			}
			switch rule.Scope {
			case "provider":
				if rec.Provider != rule.Provider {
					continue
				}
			case "env":
				if rec.Provider != rule.Provider {
					continue
				}
				ts, err := time.ParseInLocation(recordTimeLayout, rec.Timestamp, now.Location())
				if err != nil || activeEnvAt(activations[rule.Provider], ts.Unix()) != rule.EnvName {
					continue
				}
			}
			spent += rec.TotalCost
		}
		percent := 0.0
		if rule.Limit > 0 {
			percent = spent / rule.Limit * 100
		}
		level := "ok"
		switch {
		case percent >= 100:
			level = "over"
		case percent >= float64(warnPercent):
			level = "warn"
		}
		out = append(out, BudgetStatus{Rule: rule, Spent: spent, Percent: percent, Level: level, Since: since})
	}
	return out
}

var budgetLevelRank = map[string]int{"ok": 0, "warn": 1, "over": 2}

// check 计算并在级别上升时提醒；同一周期同一级别只提醒一次
func (bs *BudgetService) check() {
	bs.mu.Lock()
	store, err := loadBudgetStore()
	bs.mu.Unlock()
	if err != nil {
		return
	}
	statuses := bs.compute(store.BudgetSettings)
	if len(statuses) == 0 {
		return
	}
	now := bs.now()

	bs.mu.Lock()
	// 计算期间设置可能被改过，以最新文件为准，只更新仍存在且未改动的规则
	latest, err := loadBudgetStore()
	if err != nil {
		bs.mu.Unlock()
		return
	}
	current := map[string]BudgetRule{}
	for _, r := range latest.Rules {
		current[r.ID] = r
	}
	if latest.Alerts == nil {
		latest.Alerts = map[string]budgetAlertState{}
	}
	var fire []BudgetStatus
	changed := false
	for _, st := range statuses {
		if r, ok := current[st.Rule.ID]; !ok || r != st.Rule {
			continue
		}
		key := budgetPeriodKey(st.Rule.Period, now)
		state := latest.Alerts[st.Rule.ID]
		if state.Period != key {
			state = budgetAlertState{Period: key, Level: "ok"}
			changed = true
		}
		if budgetLevelRank[st.Level] > budgetLevelRank[state.Level] {
			state.Level = st.Level
			fire = append(fire, st)
			changed = true
		}
		latest.Alerts[st.Rule.ID] = state
	}
	if changed {
		_ = saveBudgetStore(latest)
	}
	bs.mu.Unlock()

	for _, st := range fire {
		bs.notify(st)
	}
}

func budgetProviderName(provider string) string {
	switch provider {
	case "claude":
		return "Claude Code"
	case "claude_desktop":
		return "Claude Desktop"
	case "codex":
		return "Codex"
	case "antigravity":
		return "Antigravity"
	case "opencode":
		return "OpenCode"
	case "grok":
		return "Grok"
	}
	return provider
}

// budgetAlertText 托盘通知的文字（界面通知由前端按当前语言排版）
func budgetAlertText(st BudgetStatus) (string, string) {
	scope := tr("全部平台")
	switch st.Rule.Scope {
	case "provider":
		scope = budgetProviderName(st.Rule.Provider)
	case "env":
		scope = fmt.Sprintf("%s / %s", budgetProviderName(st.Rule.Provider), st.Rule.EnvName)
	}
	period := tr("今日")
	if st.Rule.Period == "month" {
		period = tr("本月")
	}
	title := tr("花费接近预算")
	if st.Level == "over" {
		title = tr("花费已超出预算")
	}
	return title, sprintf("%s %s已花费 $%.2f，预算 $%.2f（%.0f%%）", scope, period, st.Spent, st.Rule.Limit, st.Percent)
}

func (bs *BudgetService) defaultNotify(st BudgetStatus) {
	title, body := budgetAlertText(st)
	trayNotify(title, body)
	if bs.app != nil && bs.app.ctx != nil {
		wailsruntime.EventsEmit(bs.app.ctx, "budget:alert", st)
	}
}
