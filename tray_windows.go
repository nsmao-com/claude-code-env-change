//go:build windows

package main

import (
	"context"
	"sort"
	"strings"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// StartTray Windows 系统托盘：显示主窗口、快速应用 Claude 配置（● 标记当前）、退出。
// 在独立 goroutine 里运行（main.go OnStartup 时 go StartTray(...)），
// 菜单动作通过 wails runtime 与 App 方法执行，与主窗口事件循环互不阻塞。
// getlantern/systray 不支持子菜单，配置项平铺在分隔线之后（"Claude: 名称"）。
func StartTray(a *App, ctx context.Context) {
	systray.Run(func() {
		systray.SetIcon(appIcon)
		systray.SetTitle("AI ENV")
		systray.SetTooltip("AI ENV — AI CLI 环境与配置管理")

		showItem := systray.AddMenuItem("显示主窗口", "打开 AI ENV 主窗口")
		systray.AddSeparator()

		// 持锁快照配置列表，避免与 UI 并发修改竞争
		a.configMu.Lock()
		var names []string
		current := a.config.CurrentEnvClaude
		for _, env := range a.config.Environments {
			if strings.EqualFold(strings.TrimSpace(env.Provider), "claude") && !env.OfficialLogin {
				names = append(names, env.Name)
			}
		}
		a.configMu.Unlock()
		sort.Strings(names)

		items := make(map[string]*systray.MenuItem, len(names))
		for _, name := range names {
			label := "Claude: " + name
			if name == current {
				label = "● Claude: " + name
			}
			items[name] = systray.AddMenuItem(label, "应用 "+name)
		}
		if len(items) == 0 {
			none := systray.AddMenuItem("（暂无 Claude 配置）", "先在主窗口新建配置")
			none.Disable()
		}
		systray.AddSeparator()
		quitItem := systray.AddMenuItem("退出", "退出 AI ENV")

		go func() {
			for {
				select {
				case <-showItem.ClickedCh:
					runtime.WindowUnminimise(ctx)
					runtime.WindowShow(ctx)
				case <-quitItem.ClickedCh:
					systray.Quit()
					runtime.Quit(ctx)
					return
				case name := <-waitApply(items):
					if name == "" {
						continue
					}
					if _, err := a.ApplyEnv(name, "claude"); err != nil {
						runtime.LogWarningf(ctx, "托盘应用配置失败: %v", err)
					}
				}
			}
		}()
	}, func() {
		// 托盘退出回调
	})
}

// waitApply 把所有配置菜单项的点击汇聚到一个 channel；返回被点中的配置名。
func waitApply(items map[string]*systray.MenuItem) <-chan string {
	ch := make(chan string, len(items))
	for name, item := range items {
		go func(name string, item *systray.MenuItem) {
			<-item.ClickedCh
			select {
			case ch <- name:
			default:
			}
		}(name, item)
	}
	return ch
}
