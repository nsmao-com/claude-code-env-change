//go:build windows

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"github.com/wailsapp/go-webview2/webviewloader"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

// 托盘右键面板：一个 WS_POPUP 无边框 WebView2 窗口，加载嵌入的
// traypanel/traypanel.html，样式复刻输入法状态栏的弹出菜单（圆角卡片、
// 头部信息、快捷开关、图标菜单）。页面与 Go 通过 window.external.invoke
// （JS→Go）和 chromium.Eval（Go→JS）通信。
//
// 窗口在专属线程上创建并泵消息；失去焦点（WM_ACTIVATE WA_INACTIVE）自动隐藏，
// 和输入法面板的交互一致。

//go:embed traypanel/traypanel.html
var trayPanelHTML string

const (
	trayPanelLogicalW  = 372 // 面板 CSS 像素宽度（与 traypanel.html 布局对应）
	trayPanelLogicalH  = 460 // 面板 CSS 像素高度
	trayPanelShadowPad = 18  // 窗口四周留白，给 CSS 阴影留空间
	projectRepoURL     = "https://github.com/nsmao-com/claude-code-env-change"
)

type trayPanel struct {
	mgr      *trayManager
	hwnd     uintptr
	chromium *edge.Chromium
	proc     uintptr
	scale    float64
	visible  bool

	// dispatchQueue 保证闭包在 PostMessage 派发期间不被 GC，且派发互不乱序
	mu            sync.Mutex
	dispatchQueue []*func()
}

// newTrayPanel 在调用线程上创建面板窗口与 WebView2（Embed 会泵消息直至就绪）。
func newTrayPanel(m *trayManager) (*trayPanel, error) {
	// WebView2 要求调用线程是 COM STA；面板线程是新锁的 OS 线程，必须自己初始化
	if hr, _, _ := procCoInitEx.Call(0, 0x2 /* COINIT_APARTMENTTHREADED */); int32(hr) < 0 {
		return nil, fmt.Errorf("CoInitializeEx 失败: 0x%08x", uint32(hr))
	}

	// 先探测 WebView2 运行时，缺失时直接失败走回退菜单，
	// 也避免 Embed 内部把失败当成挂起
	if version, err := webviewloader.GetAvailableCoreWebView2BrowserVersionString(""); err != nil || version == "" {
		return nil, fmt.Errorf("WebView2 运行时不可用: %v", err)
	}

	p := &trayPanel{mgr: m}
	dpi := m.systemDPI()
	p.scale = float64(dpi) / 96

	hInst, _, _ := procGetModuleHandleW.Call(0)
	p.proc = windows.NewCallback(p.wndProc)
	registerWindowClass("AIEnvTrayPanel", p.proc, hInst, 0)

	w := int32(float64(trayPanelLogicalW+trayPanelShadowPad*2) * p.scale)
	h := int32(float64(trayPanelLogicalH+trayPanelShadowPad*2) * p.scale)
	hwnd, _, err := procCreateWindowExW.Call(
		wsExTopMost|wsExToolWindow,
		uptr("AIEnvTrayPanel"),
		uptr("AI ENV"),
		wsPopup,
		0, 0, uintptr(w), uintptr(h),
		m.hostHwnd, 0, hInst, 0,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("创建面板窗口失败: %v", err)
	}
	p.hwnd = hwnd
	procSetWindowLongPtrW.Call(hwnd, gwlUserData, uintptr(unsafe.Pointer(p)))

	chromium := edge.NewChromium()
	chromium.DataPath = filepath.Join(os.Getenv("LocalAppData"), "ai-env-tray-panel")
	chromium.SetErrorCallback(func(err error) {
		log.Printf("托盘面板 WebView2 错误: %v", err)
	})
	chromium.MessageCallback = p.onMessage
	chromium.NavigationCompletedCallback = func(_ *edge.ICoreWebView2, _ *edge.ICoreWebView2NavigationCompletedEventArgs) {
		p.pushState()
	}
	if !chromium.Embed(hwnd) {
		return nil, fmt.Errorf("WebView2 初始化失败")
	}
	chromium.SetBackgroundColour(0, 0, 0, 0)
	if settings, serr := chromium.GetSettings(); serr == nil {
		_ = settings.PutAreDefaultContextMenusEnabled(false)
		_ = settings.PutAreDevToolsEnabled(false)
		_ = settings.PutIsStatusBarEnabled(false)
		_ = settings.PutAreBrowserAcceleratorKeysEnabled(false)
	}
	// WebView2 已随宿主窗口自动适配 DPI；页面缩放保持 100%，避免重复放大。
	chromium.PutZoomFactor(1)
	chromium.Resize()
	chromium.NavigateToString(trayPanelHTML)
	chromium.Hide()
	p.chromium = chromium
	return p, nil
}

func (p *trayPanel) messageLoop() {
	var msg winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if r == 0 {
			break
		}
		if int32(r) == -1 {
			continue
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func (p *trayPanel) wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmActivate:
		// 点击面板以外的地方：像输入法面板一样自动收起
		if wParam&0xFFFF == 0 {
			p.hide()
		}
	case wmSize:
		if p.chromium != nil {
			p.chromium.Resize()
		}
	case wmSetFocus:
		if p.chromium != nil {
			p.chromium.Focus()
		}
	case wmDpiChanged:
		// DPI 变化：重算原生窗口尺寸；WebView2 自动调整渲染比例。
		if dpi := wParam & 0xFFFF; dpi > 0 {
			p.scale = float64(dpi) / 96
			w := int32(float64(trayPanelLogicalW+trayPanelShadowPad*2) * p.scale)
			h := int32(float64(trayPanelLogicalH+trayPanelShadowPad*2) * p.scale)
			procSetWindowPos.Call(hwnd, 0, 0, 0, uintptr(w), uintptr(h),
				swpNoZOrder|swpNoActivate|swpNoMove)
			if p.chromium != nil {
				p.chromium.Resize()
			}
		}
		return 0
	case wmDispatch:
		p.runDispatch(lParam)
		return 0
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

// show 显示面板，定位到光标所在显示器工作区的右下角（托盘一般在那里）。
// 可从托盘线程调用；涉及 WebView2 的部分统一走 dispatch。
func (p *trayPanel) show() {
	if p.hwnd == 0 {
		return
	}
	var pt winPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	// MonitorFromPoint 按值传 POINT（x64/arm64 打包成一个 8 字节参数）
	packed := *(*uint64)(unsafe.Pointer(&pt))
	mon, _, _ := procMonitorFromPoint.Call(uintptr(packed), monitorDefaultToNearest)
	var mi winMonitorInfo
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))

	w := int32(float64(trayPanelLogicalW+trayPanelShadowPad*2) * p.scale)
	h := int32(float64(trayPanelLogicalH+trayPanelShadowPad*2) * p.scale)
	margin := int32(10 * p.scale)
	x := mi.RcWork.Right - w - margin
	y := mi.RcWork.Bottom - h - margin
	if x < mi.RcWork.Left {
		x = mi.RcWork.Left
	}
	if y < mi.RcWork.Top {
		y = mi.RcWork.Top
	}
	procSetWindowPos.Call(p.hwnd, hwndTopmost, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpShowWindow)
	procShowWindow.Call(p.hwnd, swShow)
	procSetForegroundWindow.Call(p.hwnd)
	p.dispatch(func() {
		if p.chromium != nil {
			p.chromium.Show()
			p.chromium.Focus()
		}
	})
	p.visible = true
}

func (p *trayPanel) hide() {
	if p.hwnd == 0 {
		return
	}
	procShowWindow.Call(p.hwnd, swHide)
	p.dispatch(func() {
		if p.chromium != nil {
			p.chromium.Hide()
		}
	})
	p.visible = false
}

// requestClose 跨线程关闭面板（DestroyWindow 只能在窗口自己的线程调用，
// 所以发 WM_CLOSE 过去）。
func (p *trayPanel) requestClose() {
	if p.hwnd != 0 {
		procPostMessageW.Call(p.hwnd, wmClose, 0, 0)
	}
}

func (p *trayPanel) dispatch(fn func()) {
	pf := &fn
	p.mu.Lock()
	p.dispatchQueue = append(p.dispatchQueue, pf)
	p.mu.Unlock()
	if p.hwnd != 0 {
		procPostMessageW.Call(p.hwnd, wmDispatch, 0, uintptr(unsafe.Pointer(pf)))
	}
}

func (p *trayPanel) runDispatch(lp uintptr) {
	p.mu.Lock()
	for i, pf := range p.dispatchQueue {
		if uintptr(unsafe.Pointer(pf)) == lp {
			p.dispatchQueue = append(p.dispatchQueue[:i], p.dispatchQueue[i+1:]...)
			p.mu.Unlock()
			(*pf)()
			return
		}
	}
	p.mu.Unlock()
}

func (p *trayPanel) eval(js string) {
	p.dispatch(func() {
		if p.chromium != nil {
			p.chromium.Eval(js)
		}
	})
}

func (p *trayPanel) toastMsg(msg string, isErr bool) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	flag := ""
	if isErr {
		flag = ", true"
	}
	p.eval("__panelToast(" + string(data) + flag + ")")
}

func (p *trayPanel) resetBusy(action string) {
	p.eval("__panelBusy('" + action + "', false)")
}

// pushState 读取应用当前状态推给面板。耗时的锁操作放后台 goroutine，
// 结果通过 dispatch 回到面板线程再 Eval。
func (p *trayPanel) pushState() {
	m := p.mgr
	go func() {
		st := buildTrayPanelState(m)
		data, err := json.Marshal(st)
		if err != nil {
			return
		}
		p.eval("__panelApply(" + string(data) + ")")
	}()
}

// onMessage 面板 JS 通过 window.external.invoke 发来的动作。
// 在面板线程上执行；可能阻塞的动作（应用配置/检查更新）放 goroutine。
func (p *trayPanel) onMessage(message string, _ *edge.ICoreWebView2, _ *edge.ICoreWebView2WebMessageReceivedEventArgs) {
	var req struct {
		Action   string `json:"action"`
		Provider string `json:"provider"`
		Name     string `json:"name"`
		Page     string `json:"page"`
	}
	if err := json.Unmarshal([]byte(message), &req); err != nil {
		return
	}
	m := p.mgr
	switch req.Action {
	case "ready":
		p.pushState()
	case "hide-panel":
		p.hide()
	case "toggle-main":
		m.toggleMain()
		p.hide()
	case "apply-all":
		go func() {
			msg, err := m.app.ApplyCurrentEnv()
			p.resetBusy("apply-all")
			if err != nil {
				p.toastMsg("全部应用失败: "+err.Error(), true)
				return
			}
			runtime.EventsEmit(m.ctx, "tray:applied", msg)
			p.toastMsg(abbreviate(msg, 120), false)
			p.pushState()
		}()
	case "apply-env":
		go func() {
			_, err := m.app.ApplyEnv(req.Name, req.Provider)
			if err != nil {
				p.toastMsg("应用「"+req.Name+"」失败: "+err.Error(), true)
				return
			}
			runtime.EventsEmit(m.ctx, "tray:applied", "已应用 "+req.Name)
			p.toastMsg("已应用「"+req.Name+"」", false)
			p.pushState()
		}()
	case "router-toggle":
		go func() {
			var err error
			if m.router.GetGatewayStatus().Running {
				err = m.router.StopGateway()
			} else {
				err = m.router.StartGateway()
			}
			p.resetBusy("router-toggle")
			if err != nil {
				p.toastMsg("路由切换失败: "+err.Error(), true)
				return
			}
			runtime.EventsEmit(m.ctx, "tray:router-changed", m.router.GetGatewayStatus().Running)
			p.pushState()
		}()
	case "autostart-toggle":
		go func() {
			enabled := m.app.GetAutostartEnabled()
			err := m.app.SetAutostart(!enabled)
			p.resetBusy("autostart-toggle")
			if err != nil {
				p.toastMsg(err.Error(), true)
				return
			}
			p.toastMsg(map[bool]string{true: "已开启开机自启", false: "已关闭开机自启"}[!enabled], false)
			p.pushState()
		}()
	case "check-update":
		go func() {
			info, err := m.app.CheckForUpdate()
			p.resetBusy("check-update")
			if err != nil {
				p.toastMsg("检查更新失败: "+err.Error(), true)
				return
			}
			m.mu.Lock()
			m.updateAvailable = info.Available
			m.mu.Unlock()
			if info.Available {
				// tag 本身带 v（v2.6.15），不再重复拼接
				p.toastMsg("发现新版本 v"+strings.TrimPrefix(info.LatestVersion, "v")+"，可在主窗口中更新", false)
			} else {
				p.toastMsg("已是最新版本 v"+info.CurrentVersion, false)
			}
			runtime.EventsEmit(m.ctx, "tray:update-status", info.Available)
			p.pushState()
		}()
	case "open-page":
		if req.Page == "" {
			req.Page = "home"
		}
		m.showMain()
		runtime.EventsEmit(m.ctx, "tray:navigate", req.Page)
		p.hide()
	case "help":
		runtime.BrowserOpenURL(m.ctx, projectRepoURL)
		p.hide()
	case "quit":
		m.quit()
	}
}

// ---- 面板状态 ----

type trayEnvDto struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Official    bool   `json:"official"`
	Current     bool   `json:"current"`
}

type trayProviderDto struct {
	ID    string       `json:"id"`
	Label string       `json:"label"`
	Color string       `json:"color"`
	Envs  []trayEnvDto `json:"envs"`
}

type trayPanelState struct {
	Version         string            `json:"version"`
	ManagedCount    int               `json:"managedCount"`
	RouterRunning   bool              `json:"routerRunning"`
	RouterPort      int               `json:"routerPort"`
	AutostartOn     bool              `json:"autostartEnabled"`
	UpdateAvailable bool              `json:"updateAvailable"`
	Providers       []trayProviderDto `json:"providers"`
}

var trayProviderDefs = []struct{ id, label, color string }{
	{"claude", "Claude", "#D97757"},
	{"claude_desktop", "Claude Desktop", "#D97757"},
	{"codex", "Codex", "#1A1D21"},
	{"antigravity", "Antigravity", "#4F6BED"},
	{"opencode", "OpenCode", "#131010"},
	{"grok", "Grok", "#6B7280"},
}

func buildTrayPanelState(m *trayManager) trayPanelState {
	st := trayPanelState{Version: appVersion}
	cfg := m.app.GetConfig()
	st.ManagedCount = len(cfg.Environments)
	gs := m.router.GetGatewayStatus()
	st.RouterRunning = gs.Running
	st.RouterPort = gs.Port
	st.AutostartOn = m.app.GetAutostartEnabled()
	m.mu.Lock()
	st.UpdateAvailable = m.updateAvailable
	m.mu.Unlock()

	for _, def := range trayProviderDefs {
		dto := trayProviderDto{ID: def.id, Label: def.label, Color: def.color}
		for _, env := range cfg.Environments {
			if !sameProvider(env.Provider, def.id) {
				continue
			}
			e := trayEnvDto{
				Name:        env.Name,
				Description: abbreviate(env.Description, 48),
				Official:    env.OfficialLogin,
			}
			switch def.id {
			case "claude":
				e.Current = env.Name == cfg.CurrentEnvClaude
			case "claude_desktop":
				e.Current = env.Name == cfg.CurrentEnvClaudeDesktop
			case "codex":
				e.Current = env.Name == cfg.CurrentEnvCodex
			case "antigravity":
				e.Current = env.Name == cfg.CurrentEnvAntigravity
			case "grok":
				e.Current = env.Name == cfg.CurrentEnvGrok
			case "opencode":
				if env.Name == cfg.CurrentEnvOpencode {
					e.Current = true
				}
				for _, cur := range cfg.CurrentEnvsOpencode {
					if cur == env.Name {
						e.Current = true
						break
					}
				}
			}
			dto.Envs = append(dto.Envs, e)
		}
		st.Providers = append(st.Providers, dto)
	}
	return st
}

func abbreviate(s string, max int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
