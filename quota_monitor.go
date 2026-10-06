package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// QuotaAlert 推送给前端的提醒
type QuotaAlert struct {
	Kind    string `json:"kind"` // quota | balance
	Title   string `json:"title"`
	Message string `json:"message"`
}

func defaultQuotaSettings() QuotaSettings {
	return QuotaSettings{AlertPercent: 80, MonitorMinutes: 0, DesktopNotify: true}
}

// GetQuotaSettings 读取额度提醒设置
func (qs *QuotaService) GetQuotaSettings() QuotaSettings {
	workbenchMu.Lock()
	c, _ := loadWorkbench()
	workbenchMu.Unlock()
	if !c.Quota.Saved {
		return defaultQuotaSettings()
	}
	return c.Quota
}

// SaveQuotaSettings 保存额度提醒设置
func (qs *QuotaService) SaveQuotaSettings(s QuotaSettings) error {
	if s.AlertPercent < 0 || s.AlertPercent > 100 {
		return fmt.Errorf("提醒阈值必须在 0–100 之间")
	}
	if s.MonitorMinutes < 0 || s.MonitorMinutes > 24*60 {
		return fmt.Errorf("检查间隔必须在 0–1440 分钟之间")
	}
	if s.MonitorMinutes > 0 && s.MonitorMinutes < 5 {
		return fmt.Errorf("检查间隔最少 5 分钟，避免频繁请求供应商接口")
	}
	if err := validateWarmup(s.Warmup); err != nil {
		return err
	}
	workbenchMu.Lock()
	defer workbenchMu.Unlock()
	c, err := loadWorkbench()
	if err != nil {
		return err
	}
	s.Saved = true
	c.Quota = s
	return saveWorkbench(c)
}

// OnStartup 启动后台额度监控
func (qs *QuotaService) OnStartup(ctx context.Context) {
	qs.ctx = ctx
	go qs.monitorLoop(ctx)
}

func (qs *QuotaService) monitorLoop(ctx context.Context) {
	// 启动后稍等再开始，不和界面首屏抢资源
	select {
	case <-ctx.Done():
		return
	case <-time.After(45 * time.Second):
	}
	var last time.Time
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		s := qs.GetQuotaSettings()
		if s.MonitorMinutes > 0 && time.Since(last) >= time.Duration(s.MonitorMinutes)*time.Minute {
			last = time.Now()
			qs.runMonitor(s)
		}
		qs.checkWarmup(time.Now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (qs *QuotaService) runMonitor(s QuotaSettings) {
	qs.mu.Lock()
	tried := qs.claudeTried
	qs.mu.Unlock()
	// Claude 只在它被使用过、且距上次读取超过 15 分钟时才重新运行 /usage
	readClaude := tried.IsZero() || time.Since(tried) >= 15*time.Minute && claudeUsedSince(tried)
	quotas := qs.readQuotas(true, readClaude)
	balances := qs.GetBalances(true)
	qs.checkAlerts(s, quotas, balances)
}

// checkAlerts 每个窗口在一个重置周期内只提醒一次；余额回升到阈值以上后才会再次提醒
func (qs *QuotaService) checkAlerts(s QuotaSettings, quotas []SubscriptionQuota, balances []BalanceCard) {
	alerts := []QuotaAlert{}
	qs.alertMu.Lock()
	if s.AlertPercent > 0 {
		for _, q := range quotas {
			if q.Status != "ok" {
				continue
			}
			for _, w := range q.Windows {
				if w.Unlimited {
					continue
				}
				key := "quota:" + q.Provider + ":" + w.Name
				if w.Used < float64(s.AlertPercent) {
					delete(qs.alerted, key)
					continue
				}
				if stamp, ok := qs.alerted[key]; ok && (stamp == w.ResetsAt || w.ResetsAt == 0) {
					continue
				}
				qs.alerted[key] = w.ResetsAt
				msg := fmt.Sprintf("%s 的 %s 窗口已用 %.0f%%", q.Name, w.Name, w.Used)
				if w.ResetsAt > 0 {
					msg += "，" + time.UnixMilli(w.ResetsAt).Format("01-02 15:04") + " 重置"
				}
				alerts = append(alerts, QuotaAlert{Kind: "quota", Title: "订阅额度提醒", Message: msg})
			}
		}
	}
	for _, b := range balances {
		key := "balance:" + b.ID
		if !b.Low {
			delete(qs.alerted, key)
			continue
		}
		if _, ok := qs.alerted[key]; ok {
			continue
		}
		qs.alerted[key] = 1
		alerts = append(alerts, QuotaAlert{Kind: "balance", Title: "余额不足提醒",
			Message: fmt.Sprintf("%s（%s）余额 %s，低于 %s", b.Vendor, strings.Join(b.Environments, "、"), b.Display, formatMoney(b.AlertBelow, b.Currency))})
	}
	qs.alertMu.Unlock()
	if len(alerts) == 0 || qs.ctx == nil {
		return
	}
	for _, a := range alerts {
		emitAppEvent(qs.ctx, "quota:alert", a)
		if s.DesktopNotify {
			sendDesktopNotification(qs.ctx, a.Title, a.Message)
		}
	}
}

var (
	notifyOnce sync.Once
	notifyOK   bool
)

// sendDesktopNotification 发送系统通知；不可用时静默跳过（应用内仍有提示）
func sendDesktopNotification(ctx context.Context, title, body string) {
	if ctx == nil {
		return
	}
	notifyOnce.Do(func() {
		notifyOK = runtime.InitializeNotifications(ctx) == nil && runtime.IsNotificationAvailable(ctx)
	})
	if !notifyOK {
		return
	}
	_ = runtime.SendNotification(ctx, runtime.NotificationOptions{
		ID:    fmt.Sprintf("aienv-%d", time.Now().UnixNano()),
		Title: title,
		Body:  body,
	})
}
