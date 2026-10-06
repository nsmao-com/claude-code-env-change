package main

import (
	"context"
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// App icon for native Linux/macOS window metadata; also the Windows tray fallback.
// The Windows exe icon comes from build/windows/icon.ico.
//
//go:embed build/appicon.png
var appIcon []byte

func main() {
	// 带子命令运行时进入命令行模式，不启动窗口
	if args := os.Args[1:]; isCLIInvocation(args) {
		// mcp 子命令通过标准输入输出与 Agent 通信，不能借用控制台；tui 自己开控制台窗口
		if len(args) == 0 || args[0] != "mcp" && args[0] != "tui" {
			attachParentConsole()
		}
		os.Exit(runCLI(args))
	}
	svc := newAppServices()
	app := svc.app
	setPendingDeepLink(extractDeepLink(os.Args[1:]))
	onStartup := func(ctx context.Context) { svc.startup(ctx, true) }

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "AI ENV",
		Width:     1200,
		Height:    800,
		MinWidth:  940,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 244, G: 244, B: 245, A: 1},
		OnStartup:        onStartup,
		OnDomReady:       nil,
		// 点击关闭（Alt+F4 等）时隐藏到托盘继续运行；退出走托盘面板的"退出"
		OnBeforeClose: func(ctx context.Context) bool {
			return trayShouldHideOnClose()
		},
		OnShutdown: func(ctx context.Context) {
			StopTray()
			globalClaudeBridge.abortAll()
		},
		WindowStartState: options.Normal,
		Frameless:        true, // 启用无边框模式
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
			Theme:                             windows.SystemDefault,
		},
		Mac:   &mac.Options{About: &mac.AboutInfo{Title: "AI ENV", Message: "AI CLI 环境与配置管理工具", Icon: appIcon}},
		Linux: &linux.Options{Icon: appIcon, ProgramName: "AI ENV"},
		// 单实例：再次启动（含点击 aienv:// 导入链接）时唤起已运行的窗口
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               singleInstanceID(),
			OnSecondInstanceLaunch: app.handleSecondInstance,
		},
		Bind: svc.bind(),
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
