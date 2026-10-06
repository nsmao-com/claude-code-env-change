package main

// 浏览器模式：`claude-env-switcher web` 不开窗口，用内置网页服务提供同一套界面，
// 适合 NAS、Linux 服务器或 Docker。界面调用后端方法走 /__aienv/call（按方法名反射分发，
// 与 Wails 绑定的是同一批对象），后端事件走 /__aienv/events（SSE）。
// 对外监听（非本机地址）时必须设置访问口令。

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed webui/bridge.js
var webBridgeJS []byte

const webDefaultAddr = "127.0.0.1:3430"

// ===== 事件流 =====

type webHub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

var webHubCurrent atomic.Pointer[webHub]

func currentWebHub() *webHub { return webHubCurrent.Load() }

func (h *webHub) broadcast(name string, data []interface{}) {
	if data == nil {
		data = []interface{}{}
	}
	b, err := json.Marshal(map[string]any{"name": name, "data": data})
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c <- b:
		default: // 页面卡住时丢弃，不阻塞后端
		}
	}
}

func (h *webHub) subscribe() chan []byte {
	c := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	return c
}

func (h *webHub) unsubscribe(c chan []byte) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

// ===== 服务 =====

type webServer struct {
	password  string
	devURL    *url.URL
	services  map[string]reflect.Value
	hub       *webHub
	tmp       string
	sessions  sync.Map // 会话令牌 → 过期时间
	downloads sync.Map // 下载令牌 → 文件路径
	uploads   sync.Map // 上传令牌 → chan string
}

// runWebMode `web [--addr 127.0.0.1:3430] [--password 口令] [--dev http://localhost:5199]`
func runWebMode(args []string) error {
	flags, _ := cliFlags(args, "addr", "password", "dev")
	addr := firstNonEmpty(flags["addr"], os.Getenv("AIENV_WEB_ADDR"), webDefaultAddr)
	password := firstNonEmpty(flags["password"], os.Getenv("AIENV_WEB_PASSWORD"))
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("监听地址无效（例：127.0.0.1:3430 或 0.0.0.0:3430）: %v", err)
	}
	if ip := net.ParseIP(host); !(host == "localhost" || ip != nil && ip.IsLoopback()) && password == "" {
		return errors.New("对外监听时必须设置访问口令：--password 或环境变量 AIENV_WEB_PASSWORD")
	}
	tmp, err := os.MkdirTemp("", "aienv-web-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	svc := newAppServices()
	hub := &webHub{clients: map[chan []byte]struct{}{}}
	webHubCurrent.Store(hub)
	ws := &webServer{password: password, hub: hub, tmp: tmp, services: map[string]reflect.Value{}}
	if dev := firstNonEmpty(flags["dev"], os.Getenv("AIENV_WEB_DEV_URL")); dev != "" {
		if ws.devURL, err = url.Parse(dev); err != nil {
			return fmt.Errorf("--dev 地址无效: %v", err)
		}
	}
	for _, obj := range svc.bind() {
		v := reflect.ValueOf(obj)
		ws.services[v.Elem().Type().Name()] = v
	}
	// Wails 的 runtime 函数按 ctx 里的 "frontend" 取窗口实现：换成浏览器版，
	// 打开链接、文件对话框、系统通知都转给网页处理
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), "frontend", &webFrontend{ws: ws}))
	defer cancel()
	svc.startup(ctx, false)

	srv := &http.Server{Addr: addr, Handler: ws.routes(), ReadHeaderTimeout: 15 * time.Second}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听 %s 失败: %v", addr, err)
	}
	shown := addr
	if host == "0.0.0.0" || host == "::" || host == "" {
		_, port, _ := net.SplitHostPort(addr)
		shown = "127.0.0.1:" + port
		for _, ip := range lanAddresses() {
			fmt.Printf("局域网访问：http://%s:%s\n", ip, port)
		}
	}
	fmt.Printf("AI ENV 浏览器模式已启动：http://%s\n", shown)
	if password == "" {
		fmt.Println("仅本机可访问；对外提供时请加 --addr 0.0.0.0:3430 --password <口令>")
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go func() {
		<-stop
		fmt.Println("正在退出…")
		globalClaudeBridge.abortAll()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = srv.Shutdown(shutdown)
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (ws *webServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/__aienv/bridge.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(webBridgeJS)
	})
	mux.HandleFunc("/__aienv/login", ws.serveLogin)
	mux.HandleFunc("/__aienv/call", ws.guard(ws.serveCall))
	mux.HandleFunc("/__aienv/events", ws.guard(ws.serveEvents))
	mux.HandleFunc("/__aienv/upload/", ws.guard(ws.serveUpload))
	mux.HandleFunc("/__aienv/download/", ws.guard(ws.serveDownload))
	mux.HandleFunc("/", ws.guard(ws.serveAssets))
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// guard 设置了口令时检查登录；改动类请求还要带自定义请求头，挡住跨站表单
func (ws *webServer) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ws.password != "" && !ws.authed(r) {
			if strings.HasPrefix(r.URL.Path, "/__aienv/") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/__aienv/login", http.StatusFound)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("X-AIENV-Web") != "1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (ws *webServer) authed(r *http.Request) bool {
	c, err := r.Cookie("aienv_session")
	if err != nil {
		return false
	}
	v, ok := ws.sessions.Load(c.Value)
	return ok && time.Now().Before(v.(time.Time))
}

func randomHexToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

const webLoginPage = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>AI ENV 登录</title><style>
:root{--bg:#f4f4f5;--card:#fff;--fg:#18181b;--muted:#71717a;--line:#e4e4e7;--accent:#18181b}
@media (prefers-color-scheme:dark){:root{--bg:#09090b;--card:#18181b;--fg:#fafafa;--muted:#a1a1aa;--line:#27272a;--accent:#fafafa}}
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;background:var(--bg);color:var(--fg);font:14px system-ui,-apple-system,"Segoe UI",sans-serif;padding:16px;box-sizing:border-box}
form{background:var(--card);border:1px solid var(--line);border-radius:14px;padding:24px;width:100%;max-width:340px}
h1{font-size:18px;margin:0 0 4px}p{color:var(--muted);margin:0 0 16px}
input{width:100%;box-sizing:border-box;padding:9px 11px;border:1px solid var(--line);border-radius:9px;background:transparent;color:var(--fg);font:inherit}
button{margin-top:12px;width:100%;padding:9px;border:0;border-radius:9px;background:var(--accent);color:var(--bg);font:inherit;font-weight:600;cursor:pointer}
.err{color:#dc2626;margin:10px 0 0}</style></head><body>
<form method="post" action="/__aienv/login"><h1>AI ENV</h1><p>输入启动浏览器模式时设置的访问口令</p>
<input type="password" name="password" autocomplete="current-password" autofocus required aria-label="访问口令">
<button type="submit">登录</button>{{ERROR}}</form></body></html>`

func (ws *webServer) serveLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method != http.MethodPost {
		_, _ = io.WriteString(w, strings.Replace(webLoginPage, "{{ERROR}}", "", 1))
		return
	}
	_ = r.ParseForm()
	got := r.PostForm.Get("password")
	if ws.password == "" || subtle.ConstantTimeCompare([]byte(got), []byte(ws.password)) != 1 {
		time.Sleep(800 * time.Millisecond) // 放慢猜口令
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, strings.Replace(webLoginPage, "{{ERROR}}", `<p class="err" role="alert">口令不对</p>`, 1))
		return
	}
	token := randomHexToken(24)
	ws.sessions.Store(token, time.Now().Add(30*24*time.Hour))
	http.SetCookie(w, &http.Cookie{Name: "aienv_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 30 * 24 * 3600, Secure: r.TLS != nil})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// serveCall 按 服务名.方法名 调用绑定的对象，参数与返回值的 JSON 约定与 Wails 相同
func (ws *webServer) serveCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Service string            `json:"service"`
		Method  string            `json:"method"`
		Args    []json.RawMessage `json:"args"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式不对"})
		return
	}
	obj, ok := ws.services[req.Service]
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"error": "未知的服务 " + req.Service})
		return
	}
	m := obj.MethodByName(req.Method)
	if !m.IsValid() || req.Method == "" || req.Method[0] < 'A' || req.Method[0] > 'Z' {
		writeJSON(w, http.StatusOK, map[string]any{"error": "未知的方法 " + req.Service + "." + req.Method})
		return
	}
	t := m.Type()
	in := make([]reflect.Value, t.NumIn())
	for i := 0; i < t.NumIn(); i++ {
		p := reflect.New(t.In(i))
		if i < len(req.Args) && len(req.Args[i]) > 0 && string(req.Args[i]) != "null" {
			if err := json.Unmarshal(req.Args[i], p.Interface()); err != nil {
				writeJSON(w, http.StatusOK, map[string]any{"error": fmt.Sprintf("第 %d 个参数格式不对: %v", i+1, err)})
				return
			}
		}
		in[i] = p.Elem()
	}
	var out []reflect.Value
	func() {
		defer func() {
			if p := recover(); p != nil {
				log.Printf("web call %s.%s panic: %v", req.Service, req.Method, p)
				out = nil
				writeJSON(w, http.StatusOK, map[string]any{"error": fmt.Sprint(p)})
			}
		}()
		out = m.Call(in)
	}()
	if out == nil && t.NumOut() > 0 {
		return
	}
	errType := reflect.TypeOf((*error)(nil)).Elem()
	resp := map[string]any{"result": nil}
	for i, v := range out {
		if t.Out(i) == errType {
			if !v.IsNil() {
				resp["error"] = v.Interface().(error).Error()
			}
			continue
		}
		resp["result"] = v.Interface()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (ws *webServer) serveEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	c := ws.hub.subscribe()
	defer ws.hub.unsubscribe(c)
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case b := <-c:
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// serveUpload 网页里选的文件：存到临时目录，交给等待中的“打开文件”对话框
func (ws *webServer) serveUpload(w http.ResponseWriter, r *http.Request) {
	token := path.Base(r.URL.Path)
	v, ok := ws.uploads.Load(token)
	if !ok {
		http.Error(w, "已过期", http.StatusGone)
		return
	}
	ch := v.(chan string)
	if r.Method == http.MethodDelete {
		select {
		case ch <- "":
		default:
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	name := filepath.Base(strings.ReplaceAll(r.URL.Query().Get("name"), "\\", "/"))
	if name == "" || name == "." || name == "/" {
		name = "upload"
	}
	dir := filepath.Join(ws.tmp, "up-"+token)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dst := filepath.Join(dir, name)
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = io.Copy(f, io.LimitReader(r.Body, 512<<20))
	_ = f.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	select {
	case ch <- dst:
	default:
	}
	w.WriteHeader(http.StatusNoContent)
}

// serveDownload “保存文件”对话框给出的路径：等后端写完再发给浏览器
func (ws *webServer) serveDownload(w http.ResponseWriter, r *http.Request) {
	token := path.Base(r.URL.Path)
	v, ok := ws.downloads.Load(token)
	if !ok {
		http.Error(w, "已过期", http.StatusGone)
		return
	}
	p := v.(string)
	var last int64 = -1
	stable := 0
	for i := 0; i < 300 && stable < 3; i++ {
		if fi, err := os.Stat(p); err == nil {
			if fi.Size() == last {
				stable++
			} else {
				stable, last = 0, fi.Size()
			}
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(150 * time.Millisecond):
		}
	}
	f, err := os.Open(p)
	if err != nil {
		http.Error(w, "文件还没生成", http.StatusNotFound)
		return
	}
	defer f.Close()
	name := filepath.Base(p)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = io.Copy(w, f)
}

// serveAssets 界面静态文件；index.html 里插入 bridge.js。--dev 时转发给 Vite 开发服务器
func (ws *webServer) serveAssets(w http.ResponseWriter, r *http.Request) {
	isIndex := r.URL.Path == "/" || r.URL.Path == "/index.html" || path.Ext(r.URL.Path) == ""
	if ws.devURL != nil {
		if !isIndex || strings.EqualFold(r.Header.Get("Upgrade"), "websocket") || strings.HasPrefix(r.URL.Path, "/@") || strings.HasPrefix(r.URL.Path, "/src/") || strings.HasPrefix(r.URL.Path, "/node_modules/") {
			httputil.NewSingleHostReverseProxy(ws.devURL).ServeHTTP(w, r)
			return
		}
		resp, err := http.Get(ws.devURL.String() + "/")
		if err != nil {
			http.Error(w, "连不上开发服务器: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		ws.writeIndex(w, b)
		return
	}
	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !isIndex {
		http.FileServer(http.FS(dist)).ServeHTTP(w, r)
		return
	}
	b, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		http.Error(w, "界面文件缺失：需要先构建前端", http.StatusInternalServerError)
		return
	}
	ws.writeIndex(w, b)
}

func (ws *webServer) writeIndex(w http.ResponseWriter, b []byte) {
	tag := []byte(`<script src="/__aienv/bridge.js"></script>`)
	if i := bytes.Index(b, []byte("<head>")); i >= 0 {
		b = append(b[:i+6:i+6], append(tag, b[i+6:]...)...)
	} else {
		b = append(tag, b...)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(b)
}

// ===== 浏览器版的 Wails 窗口实现 =====

// webFrontend 满足 Wails 的 Frontend 接口：窗口操作忽略，链接、文件、通知转给网页
type webFrontend struct{ ws *webServer }

func (f *webFrontend) emit(name string, v any) { f.ws.hub.broadcast(name, []interface{}{v}) }

func (f *webFrontend) Run(context.Context) error { return nil }
func (f *webFrontend) RunMainLoop()              {}
func (f *webFrontend) ExecJS(string)             {}
func (f *webFrontend) Hide()                     {}
func (f *webFrontend) Show()                     {}
func (f *webFrontend) Quit()                     {}

func (f *webFrontend) OpenFileDialog(o runtime.OpenDialogOptions) (string, error) {
	token := randomHexToken(16)
	ch := make(chan string, 1)
	f.ws.uploads.Store(token, ch)
	defer f.ws.uploads.Delete(token)
	var accept []string
	for _, flt := range o.Filters {
		for _, p := range strings.Split(flt.Pattern, ";") {
			if ext := strings.TrimPrefix(strings.TrimSpace(p), "*"); strings.HasPrefix(ext, ".") {
				accept = append(accept, ext)
			}
		}
	}
	f.emit("aienv:web:pick-file", map[string]any{"token": token, "title": o.Title, "accept": strings.Join(accept, ",")})
	select {
	case p := <-ch:
		return p, nil
	case <-time.After(10 * time.Minute):
		return "", nil
	}
}

func (f *webFrontend) OpenMultipleFilesDialog(o runtime.OpenDialogOptions) ([]string, error) {
	p, err := f.OpenFileDialog(o)
	if p == "" || err != nil {
		return nil, err
	}
	return []string{p}, nil
}

func (f *webFrontend) OpenDirectoryDialog(runtime.OpenDialogOptions) (string, error) {
	return "", errors.New("浏览器模式不能选择运行 AI ENV 那台机器上的文件夹，请直接填写路径")
}

func (f *webFrontend) SaveFileDialog(o runtime.SaveDialogOptions) (string, error) {
	token := randomHexToken(16)
	name := filepath.Base(firstNonEmpty(o.DefaultFilename, "download"))
	dir := filepath.Join(f.ws.tmp, "down-"+token)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(dir, name)
	f.ws.downloads.Store(token, p)
	time.AfterFunc(30*time.Minute, func() { f.ws.downloads.Delete(token); _ = os.RemoveAll(dir) })
	f.emit("aienv:web:download", map[string]any{"url": "/__aienv/download/" + token, "name": name})
	return p, nil
}

func (f *webFrontend) MessageDialog(runtime.MessageDialogOptions) (string, error) { return "", nil }

func (f *webFrontend) WindowSetTitle(string)                                   {}
func (f *webFrontend) WindowShow()                                             {}
func (f *webFrontend) WindowHide()                                             {}
func (f *webFrontend) WindowCenter()                                           {}
func (f *webFrontend) WindowToggleMaximise()                                   {}
func (f *webFrontend) WindowMaximise()                                         {}
func (f *webFrontend) WindowUnmaximise()                                       {}
func (f *webFrontend) WindowMinimise()                                         {}
func (f *webFrontend) WindowUnminimise()                                       {}
func (f *webFrontend) WindowSetAlwaysOnTop(bool)                               {}
func (f *webFrontend) WindowSetPosition(int, int)                              {}
func (f *webFrontend) WindowGetPosition() (int, int)                           { return 0, 0 }
func (f *webFrontend) WindowSetSize(int, int)                                  {}
func (f *webFrontend) WindowGetSize() (int, int)                               { return 0, 0 }
func (f *webFrontend) WindowSetMinSize(int, int)                               {}
func (f *webFrontend) WindowSetMaxSize(int, int)                               {}
func (f *webFrontend) WindowFullscreen()                                       {}
func (f *webFrontend) WindowUnfullscreen()                                     {}
func (f *webFrontend) WindowSetBackgroundColour(*options.RGBA)                 {}
func (f *webFrontend) WindowReload()                                           {}
func (f *webFrontend) WindowReloadApp()                                        {}
func (f *webFrontend) WindowSetSystemDefaultTheme()                            {}
func (f *webFrontend) WindowSetLightTheme()                                    {}
func (f *webFrontend) WindowSetDarkTheme()                                     {}
func (f *webFrontend) WindowIsMaximised() bool                                 { return false }
func (f *webFrontend) WindowIsMinimised() bool                                 { return false }
func (f *webFrontend) WindowIsNormal() bool                                    { return true }
func (f *webFrontend) WindowIsFullscreen() bool                                { return false }
func (f *webFrontend) WindowClose()                                            {}
func (f *webFrontend) WindowPrint()                                            {}
func (f *webFrontend) ScreenGetAll() ([]runtime.Screen, error)                 { return nil, nil }
func (f *webFrontend) MenuSetApplicationMenu(*menu.Menu)                       {}
func (f *webFrontend) MenuUpdateApplicationMenu()                              {}
func (f *webFrontend) Notify(name string, data ...interface{})                 { f.ws.hub.broadcast(name, data) }
func (f *webFrontend) BrowserOpenURL(u string)                                 { f.emit("aienv:web:open", u) }
func (f *webFrontend) ClipboardGetText() (string, error)                       { return "", nil }
func (f *webFrontend) ClipboardSetText(string) error                           { return nil }
func (f *webFrontend) InitializeNotifications() error                          { return nil }
func (f *webFrontend) CleanupNotifications()                                   {}
func (f *webFrontend) IsNotificationAvailable() bool                           { return true }
func (f *webFrontend) RequestNotificationAuthorization() (bool, error)         { return true, nil }
func (f *webFrontend) CheckNotificationAuthorization() (bool, error)           { return true, nil }
func (f *webFrontend) OnNotificationResponse(func(runtime.NotificationResult)) {}
func (f *webFrontend) SendNotification(o runtime.NotificationOptions) error {
	f.emit("aienv:web:notify", map[string]any{"title": o.Title, "body": o.Body})
	return nil
}
func (f *webFrontend) SendNotificationWithActions(o runtime.NotificationOptions) error {
	return f.SendNotification(o)
}
func (f *webFrontend) RegisterNotificationCategory(runtime.NotificationCategory) error { return nil }
func (f *webFrontend) RemoveNotificationCategory(string) error                         { return nil }
func (f *webFrontend) RemoveAllPendingNotifications() error                            { return nil }
func (f *webFrontend) RemovePendingNotification(string) error                          { return nil }
func (f *webFrontend) RemoveAllDeliveredNotifications() error                          { return nil }
func (f *webFrontend) RemoveDeliveredNotification(string) error                        { return nil }
func (f *webFrontend) RemoveNotification(string) error                                 { return nil }
