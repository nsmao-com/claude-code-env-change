# Wails 窗口控制按钮自定义方案

## 概述

Wails v2 支持通过 **Frameless（无边框）模式** 来自定义窗口的外观，包括顶部标题栏和窗口控制按钮（最小化、最大化、关闭）。

## 实现方案

### 方案一：Frameless + 自定义标题栏（推荐）

这是最灵活的方案，可以完全自定义窗口的外观。

#### 1. 修改 main.go 配置

```go
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "Claude Code 环境管理器",
		Width:  1200,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.OnStartup,
		Frameless:        true, // 启用无边框模式
		Windows: &windows.Options{
			// Windows 特定配置
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			// 如果要去除圆角和阴影，设置为 true
			DisableFramelessWindowDecorations: false,
			// 自定义边框和标题栏颜色（Windows 11）
			Theme: windows.SystemDefault,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
```

#### 2. 在前端添加自定义标题栏

在 `frontend/index.html` 的 `<body>` 开头添加自定义标题栏：

```html
<!-- 自定义标题栏 -->
<div class="custom-titlebar" style="--wails-draggable:drag">
    <div class="titlebar-content">
        <div class="titlebar-title">
            <i class="fas fa-cube"></i>
            <span>Claude Code 环境管理器</span>
        </div>
        <div class="titlebar-controls" style="--wails-draggable:no-drag">
            <button class="titlebar-btn" onclick="minimizeWindow()">
                <i class="fas fa-window-minimize"></i>
            </button>
            <button class="titlebar-btn" onclick="toggleMaximize()">
                <i class="fas fa-window-maximize"></i>
            </button>
            <button class="titlebar-btn close-btn" onclick="closeWindow()">
                <i class="fas fa-times"></i>
            </button>
        </div>
    </div>
</div>
```

#### 3. 添加标题栏样式

在 `<style>` 标签中添加：

```css
/* 自定义标题栏 */
.custom-titlebar {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    height: 40px;
    background: var(--card);
    border-bottom: 1px solid var(--border);
    z-index: 1000;
    user-select: none;
}

.titlebar-content {
    display: flex;
    justify-content: space-between;
    align-items: center;
    height: 100%;
    padding: 0 15px;
}

.titlebar-title {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 14px;
    font-weight: 500;
    color: var(--foreground);
}

.titlebar-controls {
    display: flex;
    gap: 0;
}

.titlebar-btn {
    width: 46px;
    height: 40px;
    border: none;
    background: transparent;
    color: var(--foreground);
    cursor: pointer;
    transition: background-color 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
}

.titlebar-btn:hover {
    background-color: var(--accent);
}

.titlebar-btn.close-btn:hover {
    background-color: #e81123;
    color: white;
}

.titlebar-btn i {
    font-size: 12px;
}

/* 调整主内容区域，避免被标题栏遮挡 */
body {
    padding-top: 40px;
}
```

#### 4. 添加窗口控制 JavaScript 函数

在 `<script>` 标签中添加：

```javascript
// 引入 Wails Runtime
const { WindowMinimise, WindowToggleMaximise, Quit } = window.go.main.App || {};

// 最小化窗口
function minimizeWindow() {
    window.runtime.WindowMinimise();
}

// 切换最大化/还原
function toggleMaximize() {
    window.runtime.WindowToggleMaximise();
}

// 关闭窗口
function closeWindow() {
    window.runtime.Quit();
}
```

### 方案二：保留原生边框 + 自定义颜色（简单）

如果只想改变标题栏和边框的颜色，而不需要完全自定义，可以使用 Windows 特定的配置：

```go
Windows: &windows.Options{
    Theme: windows.Dark, // 或 windows.Light
    CustomTheme: &windows.ThemeSettings{
        // 暗色主题配置
        DarkModeTitleBar:   windows.RGB(27, 38, 54),
        DarkModeTitleText:  windows.RGB(255, 255, 255),
        DarkModeBorder:     windows.RGB(39, 39, 42),
        // 亮色主题配置
        LightModeTitleBar:  windows.RGB(255, 255, 255),
        LightModeTitleText: windows.RGB(0, 0, 0),
        LightModeBorder:    windows.RGB(228, 228, 231),
    },
}
```

## 实现步骤总结

### 推荐实现步骤（Frameless 模式）

1. ✅ 修改 `main.go`，添加 `Frameless: true` 和 Windows 配置
2. ✅ 在 `frontend/index.html` 顶部添加自定义标题栏 HTML
3. ✅ 添加标题栏 CSS 样式
4. ✅ 添加窗口控制 JavaScript 函数
5. ✅ 调整主内容区域的 padding-top
6. ✅ 运行 `wails dev` 测试效果

### 关键要点

- `style="--wails-draggable:drag"` - 使元素可拖动窗口
- `style="--wails-draggable:no-drag"` - 禁止子元素拖动（如按钮）
- 使用 `window.runtime` API 控制窗口行为
- Windows 11 支持更多自定义选项（圆角、阴影等）

## 注意事项

1. **macOS 和 Linux 兼容性**：Frameless 模式在不同平台表现可能不同，需要针对性调整
2. **拖拽区域**：确保标题栏可拖拽，但按钮不可拖拽
3. **窗口状态**：可能需要额外代码来检测和更新最大化/还原按钮的图标
4. **无障碍**：自定义按钮需要添加适当的 aria 标签

## 示例项目参考

- [Wails Frameless 官方示例](https://github.com/wailsapp/wails/tree/master/v2/examples/frameless)
- [Wails 讨论：自定义标题栏](https://github.com/wailsapp/wails/discussions/1067)

## 效果预览

实现后，您将获得：
- ✅ 完全自定义的标题栏样式
- ✅ 符合应用主题的窗口控制按钮
- ✅ 流畅的拖拽和窗口操作体验
- ✅ 现代化的无边框窗口外观
