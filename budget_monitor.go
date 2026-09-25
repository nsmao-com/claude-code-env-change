package main

import (
	"context"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"time"
)

func (w *WorkbenchService) startBudgetMonitor(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		notified := map[string]bool{}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c, e := w.GetWorkbench()
				if e != nil || c.Costs.DailyBudget <= 0 && c.Costs.MonthlyBudget <= 0 {
					continue
				}
				usage, e := w.GetCostOverview()
				if e != nil {
					continue
				}
				now := time.Now()
				for key, exceeded := range map[string]bool{"day/" + now.Format("2006-01-02"): usage.DailyExceeded, "month/" + now.Format("2006-01"): usage.MonthlyExceeded} {
					if exceeded && !notified[key] {
						notified[key] = true
						runtime.EventsEmit(ctx, "cost:budget", usage)
					}
				}
				if len(notified) > 40 {
					for key := range notified {
						if key != "day/"+now.Format("2006-01-02") && key != "month/"+now.Format("2006-01") {
							delete(notified, key)
						}
					}
				}
			}
		}
	}()
}
