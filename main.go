package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()
	mcpService := NewMCPService()
	logService := NewLogService()
	skillService := NewSkillService()
	uptimeService := NewUptimeService(app)
	routerService := NewRouterService()
	cloudSyncService := NewCloudSyncService(app, routerService)

	onStartup := func(ctx context.Context) {
		app.OnStartup(ctx)
		routerService.OnStartup(ctx)
		cloudSyncService.OnStartup()
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "AI ENV",
		Width:  1200,
		Height: 800,
		MinWidth:  940,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 244, G: 244, B: 245, A: 1},
		OnStartup:        onStartup,
		OnDomReady:       nil,
		OnBeforeClose:    nil,
		OnShutdown:       nil,
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
		Bind: []interface{}{
			app,
			mcpService,
			logService,
			skillService,
			uptimeService,
			routerService,
			cloudSyncService,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
