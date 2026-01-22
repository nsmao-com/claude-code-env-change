package main

import (
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

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "Claude Code 环境管理器",
		Width:  1200,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.OnStartup,
		OnDomReady:       nil,
		OnBeforeClose:    nil,
		OnShutdown:       nil,
		WindowStartState: options.Normal,
		Frameless:        true, // 启用无边框模式
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
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
