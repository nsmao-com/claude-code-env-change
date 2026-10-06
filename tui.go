package main

// `claude-env-switcher tui`：终端里的环境切换界面。←→ 切换工具，↑↓ 选环境，Enter 应用。
// 不依赖额外的界面库，直接用 ANSI 转义序列绘制；Windows 上会打开一个独立的控制台窗口。

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"time"
	"unicode"
)

var tuiTools = []struct{ id, name string }{
	{"claude", "Claude Code"}, {"claude_desktop", "Claude Desktop"}, {"codex", "Codex"},
	{"antigravity", "Antigravity"}, {"opencode", "OpenCode"}, {"grok", "Grok"},
}

type tuiState struct {
	app     *App
	router  *RouterService
	tool    int
	cursor  map[int]int
	message string
}

func runTUI() error {
	in, out, restore, err := tuiTerminal()
	if err != nil {
		return err
	}
	defer restore()
	st := &tuiState{app: NewApp(), router: NewRouterService(), cursor: map[int]int{}}
	fmt.Fprint(out, "\x1b[?1049h\x1b[?25l")
	defer fmt.Fprint(out, "\x1b[?25h\x1b[?1049l")
	keys := bufio.NewReader(in)
	for {
		st.draw(out)
		key, err := tuiReadKey(keys)
		if err != nil {
			return nil
		}
		envs := st.envs()
		cur := st.cursor[st.tool]
		switch key {
		case "q", "esc", "ctrl-c":
			return nil
		case "left", "h":
			st.tool = (st.tool + len(tuiTools) - 1) % len(tuiTools)
		case "right", "l", "tab":
			st.tool = (st.tool + 1) % len(tuiTools)
		case "up", "k":
			if cur > 0 {
				st.cursor[st.tool] = cur - 1
			}
		case "down", "j":
			if cur < len(envs)-1 {
				st.cursor[st.tool] = cur + 1
			}
		case "r":
			st.app = NewApp()
			st.message = "已刷新"
		case "enter":
			if cur < len(envs) {
				e := envs[cur]
				st.message = "正在应用 " + e.Name + "…"
				st.draw(out)
				msg, err := st.app.ApplyEnv(e.Name, e.Provider)
				switch {
				case err != nil:
					st.message = "应用失败：" + err.Error()
				case msg == "" || msg == "unapplied":
					st.message = "已应用 " + e.Name
				default:
					st.message = msg
				}
			}
		}
	}
}

func (st *tuiState) envs() []EnvConfig {
	cfg := st.app.GetConfig()
	tool := tuiTools[st.tool].id
	var out []EnvConfig
	for _, e := range cfg.Environments {
		if e.Provider == tool {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (st *tuiState) active() map[string]bool {
	cfg := st.app.GetConfig()
	cur := map[string]string{
		"claude": cfg.CurrentEnvClaude, "claude_desktop": cfg.CurrentEnvClaudeDesktop, "codex": cfg.CurrentEnvCodex,
		"antigravity": cfg.CurrentEnvAntigravity, "opencode": cfg.CurrentEnvOpencode, "grok": cfg.CurrentEnvGrok,
	}
	out := map[string]bool{cur[tuiTools[st.tool].id]: true}
	if tuiTools[st.tool].id == "opencode" {
		for _, n := range cfg.CurrentEnvsOpencode {
			out[n] = true
		}
	}
	return out
}

func (st *tuiState) gatewayLine() string {
	port := routerPort(st.router)
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 150*time.Millisecond)
	if err != nil {
		return fmt.Sprintf("网关 :%d 未运行", port)
	}
	c.Close()
	return fmt.Sprintf("网关 :%d 运行中", port)
}

func (st *tuiState) draw(w io.Writer) {
	var b strings.Builder
	b.WriteString("\x1b[H\x1b[2J")
	b.WriteString("\x1b[1m AI ENV\x1b[0m · 环境切换    \x1b[2m" + st.gatewayLine() + "\x1b[0m\r\n\r\n ")
	for i, t := range tuiTools {
		if i == st.tool {
			b.WriteString("\x1b[7m " + t.name + " \x1b[0m ")
		} else {
			b.WriteString("\x1b[2m " + t.name + " \x1b[0m ")
		}
	}
	b.WriteString("\r\n\r\n")
	envs := st.envs()
	active := st.active()
	cur := st.cursor[st.tool]
	if cur >= len(envs) {
		cur = 0
		st.cursor[st.tool] = 0
	}
	if len(envs) == 0 {
		b.WriteString("   \x1b[2m这个工具还没有环境，请在 AI ENV 窗口里添加\x1b[0m\r\n")
	}
	for i, e := range envs {
		mark := "  "
		if active[e.Name] && e.Name != "" {
			mark = "\x1b[32m●\x1b[0m "
		}
		base, _, _ := upstreamVarsForEnv(&e)
		if e.OfficialLogin {
			base = "官方登录"
		}
		line := tuiPad(e.Name, 28) + " " + clipText(base, 60)
		if i == cur {
			b.WriteString(" " + mark + "\x1b[7m " + line + " \x1b[0m\r\n")
		} else {
			b.WriteString(" " + mark + " " + line + "\r\n")
		}
	}
	b.WriteString("\r\n\x1b[2m ←→ 工具  ↑↓ 选择  Enter 应用  r 刷新  q 退出\x1b[0m\r\n")
	if st.message != "" {
		b.WriteString("\r\n " + st.message + "\r\n")
	}
	_, _ = io.WriteString(w, b.String())
}

// tuiPad 按显示宽度补空格（中日韩字符算两格）
func tuiPad(s string, width int) string {
	n := 0
	var out strings.Builder
	for _, r := range s {
		rw := 1
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || r >= 0xFF00 && r <= 0xFFEF {
			rw = 2
		}
		if n+rw > width {
			break
		}
		n += rw
		out.WriteRune(r)
	}
	return out.String() + strings.Repeat(" ", width-n)
}

// tuiReadKey 读一个按键：方向键是 ESC [ A/B/C/D
func tuiReadKey(r *bufio.Reader) (string, error) {
	c, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	switch c {
	case 3:
		return "ctrl-c", nil
	case '\r', '\n':
		return "enter", nil
	case '\t':
		return "tab", nil
	case 27:
		if r.Buffered() == 0 {
			time.Sleep(30 * time.Millisecond)
			if r.Buffered() == 0 {
				return "esc", nil
			}
		}
		next, _ := r.ReadByte()
		if next != '[' && next != 'O' {
			return "esc", nil
		}
		code, _ := r.ReadByte()
		switch code {
		case 'A':
			return "up", nil
		case 'B':
			return "down", nil
		case 'C':
			return "right", nil
		case 'D':
			return "left", nil
		}
		return "", nil
	}
	return strings.ToLower(string(c)), nil
}
