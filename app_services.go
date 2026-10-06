package main

import (
	"context"
	"fmt"
	"log"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// appServices 绑定给界面的全部服务：桌面窗口与浏览器模式共用
type appServices struct {
	app       *App
	mcp       *MCPService
	logs      *LogService
	skills    *SkillService
	uptime    *UptimeService
	router    *RouterService
	cloud     *CloudSyncService
	workbench *WorkbenchService
	sessions  *SessionService
	quota     *QuotaService
}

func newAppServices() *appServices {
	app := NewApp()
	s := &appServices{app: app}
	s.mcp = NewMCPService()
	s.logs = NewLogService()
	s.skills = NewSkillService()
	s.uptime = NewUptimeService(app)
	s.router = NewRouterService()
	s.cloud = NewCloudSyncService(app, s.router, s.mcp, s.skills)
	s.workbench = NewWorkbenchService(app, s.mcp, s.skills, s.router)
	s.sessions = NewSessionService(app)
	s.quota = NewQuotaService(app)
	return s
}

// bind 暴露给界面调用的对象（Wails 的 Bind 与浏览器模式的方法分发用同一份）
func (s *appServices) bind() []interface{} {
	return []interface{}{s.app, s.mcp, s.logs, s.skills, s.uptime, s.router, s.cloud, s.workbench, s.sessions, s.quota}
}

// startup 启动后台任务；tray 只在桌面窗口里需要
func (s *appServices) startup(ctx context.Context, tray bool) {
	s.app.OnStartup(ctx)
	s.workbench.startBudgetMonitor(ctx)
	s.router.OnStartup(ctx)
	s.cloud.OnStartup()
	s.quota.OnStartup(ctx)
	startModelCatalogRefresh(ctx)
	startConfigWatch(ctx, s.app, s.router)
	if tray {
		// Windows 系统托盘 + 右键面板（其它平台为空实现），独立 goroutine 不阻塞启动
		go StartTray(s.app, ctx, s.router)
	}
}

// emitAppEvent 发事件给界面：浏览器模式走网页的事件流，桌面窗口走 Wails；
// 没有窗口上下文时（命令行等）直接忽略，避免 Wails 因上下文缺失而退出进程
func emitAppEvent(ctx context.Context, name string, data ...interface{}) {
	if hub := currentWebHub(); hub != nil {
		hub.broadcast(name, data)
		return
	}
	if ctx == nil || ctx.Value("events") == nil {
		return
	}
	runtime.EventsEmit(ctx, name, data...)
}

// logAppError 写错误日志：桌面窗口进 Wails 日志，其它情况写标准日志
func logAppError(ctx context.Context, format string, args ...interface{}) {
	if ctx != nil && ctx.Value("logger") != nil {
		runtime.LogErrorf(ctx, format, args...)
		return
	}
	log.Print(fmt.Sprintf(format, args...))
}
