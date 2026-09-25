//go:build windows

package main

import (
	"context"
	"log"
	goruntime "runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

// 系统托盘（Windows）：
//   - 托盘图标跑在独立线程（自建消息窗口 + 消息循环），左键切换主窗口显隐；
//   - 右键打开自定义面板（traypanel_windows.go，独立 WebView2 窗口）；
//   - 面板创建失败（缺 WebView2 运行时等）时右键回退到系统原生菜单。
//
// 不用 getlantern/systray：它只支持原生菜单，无法承载自定义样式的右键面板。

const (
	wmTrayCallback   = 0x8000 + 1 // 托盘图标鼠标消息（WM_APP+1）
	wmDispatch       = 0x8000 + 2 // 面板线程任务派发
	wmPrecreatePanel = 0x8000 + 3 // 启动后预创建面板
	wmTrayShutdown   = 0x8000 + 4 // 退出：销毁托盘

	wmActivate   = 0x0006
	wmDestroy    = 0x0002
	wmClose      = 0x0010
	wmSize       = 0x0005
	wmSetFocus   = 0x0007
	wmDpiChanged = 0x02E0

	wmLButtonUp = 0x0202
	wmRButtonUp = 0x0205

	wsPopup        = 0x80000000
	wsExTopMost    = 0x00000008
	wsExToolWindow = 0x00000080

	swHide = 0
	swShow = 5

	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
	swpShowWindow = 0x0040

	monitorDefaultToNearest = 2

	idcArrow       = 32512
	tpmReturnCmd   = 0x0100
	tpmRightButton = 0x0002
	tpmNoNotify    = 0x0080
	mfString       = 0x0000
	mfSeparator    = 0x0800

	nimAdd    = 0
	nimDelete = 2
	nifIcon   = 0x02
	nifTip    = 0x04

	msgfltAllow = 1

	gwlUserData = ^uintptr(20) // GWLP_USERDATA (-21)
	hwndTopmost = ^uintptr(0)  // HWND_TOPMOST (-1)
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procSendMessageW        = user32.NewProc("SendMessageW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procGetWindowLongPtrW   = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW   = user32.NewProc("SetWindowLongPtrW")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procMonitorFromPoint    = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procGetDpiForWindow     = user32.NewProc("GetDpiForWindow")
	procCreateIconFromRes   = user32.NewProc("CreateIconFromResourceEx")
	procDestroyIcon         = user32.NewProc("DestroyIcon")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procRegWindowMessageW   = user32.NewProc("RegisterWindowMessageW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenuEx    = user32.NewProc("TrackPopupMenuEx")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procChangeWndMsgFilter  = user32.NewProc("ChangeWindowMessageFilterEx")

	shell32              = windows.NewLazySystemDLL("shell32.dll")
	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")

	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	ole32        = windows.NewLazySystemDLL("ole32.dll")
	procCoInitEx = ole32.NewProc("CoInitializeEx")
)

func init() {
	// 32 位系统上没有 *PtrW 系列导出
	if goarch386() {
		procGetWindowLongPtrW = user32.NewProc("GetWindowLongW")
		procSetWindowLongPtrW = user32.NewProc("SetWindowLongW")
	}
}

func goarch386() bool { return unsafe.Sizeof(uintptr(0)) == 4 }

type winPoint struct{ X, Y int32 }

type winRect struct{ Left, Top, Right, Bottom int32 }

type winMsg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      winPoint
}

type winMonitorInfo struct {
	CbSize    uint32
	RcMonitor winRect
	RcWork    winRect
	DwFlags   uint32
}

type wndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  uintptr
	LpszClassName uintptr
	HIconSm       uintptr
}

// notifyIconData 与 NOTIFYICONDATAW 的 x64/arm64 内存布局一致
type notifyIconData struct {
	CbSize           uint32
	_                [4]byte
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	_                [4]byte
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [128]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     uintptr
}

type trayManager struct {
	ctx    context.Context
	app    *App
	router *RouterService

	hostHwnd       uintptr
	hIcon          uintptr
	taskbarCreated uintptr
	hostProc       uintptr

	// mainVisible 跟踪主窗口显隐（托盘上的"关闭"= 隐藏）
	mainVisible bool
	quitting    bool

	// 面板在独立线程创建，跨线程读写都要持锁
	mu            sync.Mutex
	panel         *trayPanel
	panelStarting bool
	panelFailed   bool

	updateAvailable bool
}

var tray *trayManager

// StartTray 启动系统托盘。app/router 供面板动作调用，ctx 用于 wails runtime。
func StartTray(a *App, ctx context.Context, router *RouterService) {
	tray = &trayManager{ctx: ctx, app: a, router: router, mainVisible: true}
	go func() {
		goruntime.LockOSThread()
		tray.run()
	}()
	// 启动 3 秒后预创建面板（WebView2 初始化需要几百毫秒），首次右键即可秒开
	time.AfterFunc(3*time.Second, func() {
		if tray != nil {
			tray.post(wmPrecreatePanel, 0, 0)
		}
	})
}

// StopTray 退出前清理托盘图标与窗口。
func StopTray() {
	if tray == nil || tray.hostHwnd == 0 {
		return
	}
	procSendMessageW.Call(tray.hostHwnd, wmTrayShutdown, 0, 0)
	tray = nil
}

// trayAllowQuit 程序主动退出（如在线更新）前调用。runtime.Quit 同样会经过
// OnBeforeClose，不先放行就会被改成隐藏到托盘，进程不退出，等它退出的安装器也就不会启动。
func trayAllowQuit() {
	if tray != nil {
		tray.quitting = true
	}
}

// trayShouldHideOnClose 主窗口收到关闭请求时调用：返回 true 表示改为隐藏到托盘。
func trayShouldHideOnClose() bool {
	if tray == nil || tray.quitting {
		return false
	}
	tray.hideMain()
	return true
}

func (t *trayManager) run() {
	hInst, _, _ := procGetModuleHandleW.Call(0)
	t.hostProc = windows.NewCallback(t.hostWndProc)
	registerWindowClass("AIEnvTrayHost", t.hostProc, hInst, 0)

	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uptr("AIEnvTrayHost"),
		uptr("AI ENV Tray"),
		0, 0, 0, 0, 0,
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		log.Printf("托盘宿主窗口创建失败: %v", err)
		return
	}
	t.hostHwnd = hwnd
	procSetWindowLongPtrW.Call(hwnd, gwlUserData, uintptr(unsafe.Pointer(t)))
	t.taskbarCreated, _, _ = procRegWindowMessageW.Call(uptr("TaskbarCreated"))
	// 以管理员身份运行时（如被提权的安装器完成页拉起），UIPI 会拦下 Explorer（普通权限）
	// 发来的托盘点击和 TaskbarCreated：托盘左右键全无反应，explorer 重启后图标也回不来
	for _, m := range []uintptr{wmTrayCallback, t.taskbarCreated} {
		if m != 0 {
			procChangeWndMsgFilter.Call(hwnd, m, msgfltAllow, 0)
		}
	}

	t.loadTrayIcon()
	t.addTrayIcon()

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

	t.removeTrayIcon()
	if t.hIcon != 0 {
		procDestroyIcon.Call(t.hIcon)
	}
}

func (t *trayManager) hostWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	if msg == wmTrayCallback {
		switch lParam & 0xFFFF {
		case wmLButtonUp:
			t.toggleMain()
		case wmRButtonUp:
			t.showPanelRequest()
		}
		return 0
	}
	switch msg {
	case wmPrecreatePanel:
		t.ensurePanelAsync()
	case wmTrayShutdown:
		if p := t.getPanel(); p != nil {
			p.requestClose()
		}
		procDestroyWindow.Call(hwnd)
	case wmDestroy:
		procPostQuitMessage.Call(0)
	default:
		if t.taskbarCreated != 0 && msg == t.taskbarCreated {
			// explorer 重启会清掉托盘图标，重新注册
			t.addTrayIcon()
		} else {
			r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
			return r
		}
	}
	return 0
}

func (t *trayManager) loadTrayIcon() {
	// 托盘只认 HICON：把 PNG 直接交给 CreateIconFromResourceEx 解码
	// （Vista+ 支持 PNG 数据），按系统 DPI 缩放到托盘小图标尺寸。
	dpi := t.systemDPI()
	size := 16 * dpi / 96
	if size < 16 {
		size = 16
	}
	h, _, err := procCreateIconFromRes.Call(
		uintptr(unsafe.Pointer(&appIcon[0])), uintptr(len(appIcon)),
		1, 0x00030000, size, size, 0,
	)
	if h == 0 {
		log.Printf("托盘图标加载失败(%v)，使用默认图标", err)
		h, _, _ = procLoadIconW.Call(0, 32512)
	}
	t.hIcon = h
}

func (t *trayManager) addTrayIcon() {
	if t.hostHwnd == 0 {
		return
	}
	var nid notifyIconData
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = t.hostHwnd
	nid.UFlags = nifTip
	nid.UCallbackMessage = wmTrayCallback
	if t.hIcon != 0 {
		nid.UFlags |= nifIcon
		nid.HIcon = t.hIcon
	}
	if tip, err := windows.UTF16FromString("AI ENV - AI CLI 环境与配置管理"); err == nil {
		copy(nid.SzTip[:], tip)
	}
	procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
}

func (t *trayManager) removeTrayIcon() {
	if t.hostHwnd == 0 {
		return
	}
	var nid notifyIconData
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = t.hostHwnd
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
}

func (t *trayManager) post(msg, wParam, lParam uintptr) {
	if t.hostHwnd != 0 {
		procPostMessageW.Call(t.hostHwnd, msg, wParam, lParam)
	}
}

func (t *trayManager) systemDPI() uintptr {
	if t.hostHwnd != 0 {
		if dpi, _, _ := procGetDpiForWindow.Call(t.hostHwnd); dpi > 0 {
			return dpi
		}
	}
	if dpi, _, _ := procGetDpiForWindow.Call(0); dpi > 0 {
		return dpi
	}
	return 96
}

func (t *trayManager) toggleMain() {
	if runtime.WindowIsMinimised(t.ctx) {
		t.showMain()
		return
	}
	if t.mainVisible {
		t.hideMain()
	} else {
		t.showMain()
	}
}

func (t *trayManager) showMain() {
	runtime.WindowUnminimise(t.ctx)
	runtime.WindowShow(t.ctx)
	t.mainVisible = true
}

func (t *trayManager) hideMain() {
	runtime.WindowHide(t.ctx)
	t.mainVisible = false
}

func (t *trayManager) quit() {
	t.quitting = true
	if p := t.getPanel(); p != nil {
		p.hide()
	}
	runtime.Quit(t.ctx)
}

func (t *trayManager) getPanel() *trayPanel {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.panel
}

func (t *trayManager) setPanel(p *trayPanel) {
	t.mu.Lock()
	t.panel = p
	t.mu.Unlock()
}

func (t *trayManager) isPanelFailed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.panelFailed
}

func (t *trayManager) isPanelStarting() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.panelStarting
}

// ensurePanelAsync 在独立线程创建面板：WebView2 Embed 会自旋泵消息，
// 就算卡住也不能拖累托盘图标的响应。
func (t *trayManager) ensurePanelAsync() {
	t.mu.Lock()
	if t.panel != nil || t.panelStarting || t.panelFailed {
		t.mu.Unlock()
		return
	}
	t.panelStarting = true
	t.mu.Unlock()

	// 兜底看门狗：Embed 异常卡死时，别让右键一直等面板
	time.AfterFunc(15*time.Second, func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.panel == nil && t.panelStarting {
			t.panelStarting = false
			t.panelFailed = true
		}
	})

	go func() {
		goruntime.LockOSThread()
		p, err := newTrayPanel(t)
		if err != nil {
			log.Printf("托盘面板创建失败，右键将使用系统菜单: %v", err)
			t.mu.Lock()
			t.panelFailed = true
			t.panelStarting = false
			t.mu.Unlock()
			return
		}
		t.setPanel(p)
		p.messageLoop()
		t.setPanel(nil)
	}()
}

// showPanelRequest 右键：面板就绪就显示；首次还在创建就等一小会儿；
// 创建失败则回退系统原生菜单。
func (t *trayManager) showPanelRequest() {
	t.mu.Lock()
	p := t.panel
	starting := t.panelStarting
	t.mu.Unlock()

	if p == nil {
		if !starting && !t.isPanelFailed() {
			t.ensurePanelAsync()
		}
		deadline := time.Now().Add(3 * time.Second)
		for p == nil && time.Now().Before(deadline) {
			time.Sleep(60 * time.Millisecond)
			if t.isPanelFailed() {
				break
			}
			if !t.isPanelStarting() {
				break
			}
			p = t.getPanel()
		}
		if p == nil {
			if t.isPanelFailed() {
				t.fallbackMenu()
			}
			return
		}
	}
	p.show()
	p.pushState()
}

// fallbackMenu 面板不可用（缺 WebView2 运行时等）时的原生右键菜单。
func (t *trayManager) fallbackMenu() {
	var pt winPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	hmenu, _, _ := procCreatePopupMenu.Call()
	if hmenu == 0 {
		return
	}
	appendFallbackItem(hmenu, 1, "显示主窗口")
	appendFallbackItem(hmenu, 2, "隐藏主窗口")
	appendFallbackSeparator(hmenu)
	appendFallbackItem(hmenu, 3, "全局设置")
	appendFallbackSeparator(hmenu)
	appendFallbackItem(hmenu, 4, "退出")
	procSetForegroundWindow.Call(t.hostHwnd)
	cmd, _, _ := procTrackPopupMenuEx.Call(hmenu, tpmReturnCmd|tpmRightButton|tpmNoNotify,
		uintptr(pt.X), uintptr(pt.Y), t.hostHwnd, 0)
	procDestroyMenu.Call(hmenu)
	switch cmd {
	case 1:
		t.showMain()
	case 2:
		t.hideMain()
	case 3:
		t.showMain()
		runtime.EventsEmit(t.ctx, "tray:navigate", "settings")
	case 4:
		t.quit()
	}
}

func appendFallbackItem(hmenu, id uintptr, text string) {
	procAppendMenuW.Call(hmenu, mfString, id, uptr(text))
}

func appendFallbackSeparator(hmenu uintptr) {
	procAppendMenuW.Call(hmenu, mfSeparator, 0, 0)
}

func registerWindowClass(name string, proc, hInst uintptr, style uint32) {
	var wc wndClassEx
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.Style = style
	wc.LpfnWndProc = proc
	wc.HInstance = hInst
	wc.HCursor, _, _ = procLoadCursorW.Call(0, idcArrow)
	wc.LpszClassName = uptr(name)
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
}

// uptr 字符串转 UTF-16 指针。
func uptr(s string) uintptr {
	p, err := windows.UTF16FromString(s)
	if err != nil || len(p) == 0 {
		return uintptr(unsafe.Pointer(&[]uint16{0}[0]))
	}
	return uintptr(unsafe.Pointer(&p[0]))
}
