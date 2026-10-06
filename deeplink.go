package main

// aienv:// 导入链接：供应商或中转站可以把一份供应商配置做成链接，
// 点击后打开 AI ENV 并进入「工作台 → 供应商导入」的预览，用户确认后才会保存。
// 参数与 CC Switch 分享链接一致：aienv://import?app=claude&name=…&endpoint=…&apiKey=…&model=…

import (
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const deepLinkScheme = "aienv"

var pendingDeepLink struct {
	sync.Mutex
	link string
}

// extractDeepLink 从启动参数中找出 aienv:// 链接
func extractDeepLink(args []string) string {
	for _, arg := range args {
		arg = strings.Trim(strings.TrimSpace(arg), `"`)
		if len(arg) <= 64<<10 && strings.HasPrefix(strings.ToLower(arg), deepLinkScheme+"://") {
			return arg
		}
	}
	return ""
}

func setPendingDeepLink(link string) {
	if link == "" {
		return
	}
	pendingDeepLink.Lock()
	pendingDeepLink.link = link
	pendingDeepLink.Unlock()
}

// TakePendingDeepLink 前端启动后取走启动时带来的导入链接（取后清空）
func (a *App) TakePendingDeepLink() string {
	pendingDeepLink.Lock()
	defer pendingDeepLink.Unlock()
	link := pendingDeepLink.link
	pendingDeepLink.link = ""
	return link
}

// handleSecondInstance 再次启动时唤起已运行的窗口；带导入链接时交给前端预览
func (a *App) handleSecondInstance(data options.SecondInstanceData) {
	if a.ctx == nil {
		setPendingDeepLink(extractDeepLink(data.Args))
		return
	}
	lightweightOnShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
	if link := extractDeepLink(data.Args); link != "" {
		emitAppEvent(a.ctx, "deeplink:import", link)
	}
}

// normalizeDeepLink 把 aienv:// 链接转成 CC Switch 分享链接格式，复用同一套解析与校验
func normalizeDeepLink(text string) string {
	lower := strings.ToLower(text)
	if !strings.HasPrefix(lower, deepLinkScheme+"://") {
		return text
	}
	rest := text[len(deepLinkScheme)+3:]
	query := ""
	if i := strings.Index(rest, "?"); i >= 0 {
		query = rest[i+1:]
	}
	return "ccswitch://v1/import?" + query
}

// singleInstanceID 单实例锁 ID；wails dev（-tags dev）使用独立 ID，开发版可与已安装版本同时运行
func singleInstanceID() string {
	if devBuild {
		return "com.nsmao.aienv.desktop.dev"
	}
	return "com.nsmao.aienv.desktop"
}
