package main

// 命令行模式：同一个程序带子命令运行时不打开窗口，直接在终端里完成查询或操作。
// 写入类命令改的是同一份配置文件，正在运行的 AI ENV 会在几秒内自动重新加载。

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

var cliCommands = map[string]bool{
	"help": true, "-h": true, "--help": true, "version": true, "--version": true,
	"list": true, "ls": true, "use": true, "quota": true, "balance": true,
	"gateway": true, "gateway-key": true, "sessions": true, "mcp": true, "catalog": true,
	"web": true, "serve": true, "tui": true,
}

// isCLIInvocation 第一个参数是已知子命令时走命令行模式（aienv:// 链接等仍交给窗口）
func isCLIInvocation(args []string) bool {
	return len(args) > 0 && cliCommands[strings.ToLower(args[0])]
}

const cliUsage = `AI ENV 命令行

用法：
  claude-env-switcher <命令> [参数]

命令：
  list                                   列出各工具的环境与当前激活项
  use <工具> <环境名>                     应用环境（工具：claude / claude_desktop / codex / antigravity / opencode / grok）
  quota [--json]                         订阅额度（Claude Code / Codex / Copilot）
  quota wait <claude|codex|copilot> [--timeout 6h] [--below 100]
                                         等到该订阅所有窗口低于给定用量（默认 100%）再退出，可接在长任务的重试循环里
  balance [--json]                       供应商余额
  sessions [--days 30] [--tool claude|codex] [--json]
                                         会话使用洞察
  gateway                                网关端口、局域网共享与密钥概览
  gateway-key list                       列出网关密钥
  gateway-key add <名称>                 新建网关密钥并打印一次
  gateway-key rotate <id>                轮换密钥
  gateway-key remove <id>                删除密钥
  gateway-key limit <id> <day|week|month|off> [--tokens 2m] [--cost 5]
                                         设置密钥的周期上限
  catalog sync                           同步 models.dev 模型目录
  mcp image | mcp search                 以 stdio MCP 服务器运行内置的生成图片 / 联网搜索工具
  tui                                    终端里的环境切换界面（←→ 工具，↑↓ 环境，Enter 应用）
  web [--addr 127.0.0.1:3430] [--password 口令]
                                         浏览器模式：不开窗口，用网页提供同一套界面（NAS、服务器、Docker）；
                                         对外监听时必须设置口令，也可用环境变量 AIENV_WEB_ADDR / AIENV_WEB_PASSWORD
  version                                显示版本
`

// runCLI 执行命令并返回退出码
func runCLI(args []string) int {
	initOutboundProxy()
	cmd := strings.ToLower(args[0])
	rest := args[1:]
	var err error
	code := 0
	switch cmd {
	case "help", "-h", "--help":
		fmt.Print(cliUsage)
	case "version", "--version":
		fmt.Println("AI ENV " + appVersion)
	case "list", "ls":
		err = cliList()
	case "use":
		err = cliUse(rest)
	case "quota":
		code, err = cliQuota(rest)
	case "balance":
		err = cliBalance(rest)
	case "sessions":
		err = cliSessions(rest)
	case "gateway":
		err = cliGateway()
	case "gateway-key":
		err = cliGatewayKey(rest)
	case "catalog":
		err = cliModelCatalog(rest)
	case "web", "serve":
		err = runWebMode(rest)
	case "tui":
		err = runTUI()
	case "mcp":
		switch {
		case len(rest) == 0:
			err = fmt.Errorf("用法：mcp image | mcp search")
		case rest[0] == "claude-bridge":
			err = runClaudeBridgeMCP(rest[1:])
		default:
			err = runBuiltinMCP(rest[0])
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

// cliFlags 解析 --name value 与 --flag 形式的参数，返回剩余的位置参数
func cliFlags(args []string, valued ...string) (map[string]string, []string) {
	want := map[string]bool{}
	for _, v := range valued {
		want[v] = true
	}
	flags := map[string]string{}
	pos := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "--") {
			name := strings.TrimPrefix(a, "--")
			if k, v, ok := strings.Cut(name, "="); ok {
				flags[k] = v
				continue
			}
			if want[name] && i+1 < len(args) {
				flags[name] = args[i+1]
				i++
				continue
			}
			flags[name] = "true"
			continue
		}
		pos = append(pos, a)
	}
	return flags, pos
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func cliList() error {
	app := NewApp()
	cfg := app.GetConfig()
	current := map[string]map[string]bool{
		"claude": {cfg.CurrentEnvClaude: true}, "claude_desktop": {cfg.CurrentEnvClaudeDesktop: true},
		"codex": {cfg.CurrentEnvCodex: true}, "antigravity": {cfg.CurrentEnvAntigravity: true},
		"opencode": {cfg.CurrentEnvOpencode: true}, "grok": {cfg.CurrentEnvGrok: true},
	}
	for _, n := range cfg.CurrentEnvsOpencode {
		current["opencode"][n] = true
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "工具\t环境\t当前\tBase URL")
	envs := append([]EnvConfig{}, cfg.Environments...)
	sort.SliceStable(envs, func(i, j int) bool { return envs[i].Provider < envs[j].Provider })
	for i := range envs {
		e := envs[i]
		mark := ""
		if current[e.Provider][e.Name] && e.Name != "" {
			mark = "●"
		}
		base, _, _ := upstreamVarsForEnv(&e)
		if e.OfficialLogin {
			base = "（官方登录）"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Provider, e.Name, mark, base)
	}
	return tw.Flush()
}

func cliUse(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("用法：use <工具> <环境名>")
	}
	provider, name := strings.ToLower(args[0]), strings.Join(args[1:], " ")
	app := NewApp()
	NewRouterService() // 让开启了应用路由的环境能写入网关路由
	if _, ok := app.findEnvCopy(provider, name); !ok {
		return fmt.Errorf("找不到 %s 的环境「%s」，可用 list 查看", provider, name)
	}
	msg, err := app.ApplyEnv(name, provider)
	if err != nil {
		return err
	}
	if msg == "" || msg == "unapplied" {
		msg = "已应用 " + name
	}
	fmt.Println(msg)
	return nil
}

func quotaWindowLine(w QuotaWindow) string {
	if w.Unlimited {
		return w.Name + " 不限量"
	}
	s := fmt.Sprintf("%s %.0f%%", w.Name, w.Used)
	if w.Display != "" {
		s += " (" + w.Display + ")"
	}
	if w.ResetsAt > 0 {
		s += " · " + time.UnixMilli(w.ResetsAt).Format("01-02 15:04") + " 重置"
	}
	return s
}

func cliQuota(args []string) (int, error) {
	flags, pos := cliFlags(args, "timeout", "below")
	if len(pos) > 0 && pos[0] == "wait" {
		return cliQuotaWait(pos[1:], flags)
	}
	qs := NewQuotaService(NewApp())
	quotas := qs.GetSubscriptionQuotas(true)
	if flags["json"] == "true" {
		return 0, printJSON(quotas)
	}
	for _, q := range quotas {
		head := q.Name
		if q.Plan != "" {
			head += " · " + q.Plan
		}
		if q.Account != "" {
			head += " · " + q.Account
		}
		fmt.Println(head)
		if len(q.Windows) == 0 {
			fmt.Println("  " + quotaStatusText(q))
		}
		for _, w := range q.Windows {
			fmt.Println("  " + quotaWindowLine(w))
		}
	}
	return 0, nil
}

func quotaStatusText(q SubscriptionQuota) string {
	switch q.Status {
	case "signed_out":
		return "未登录"
	case "not_installed":
		return "未安装 CLI"
	case "expired":
		return "登录已过期，打开一次 CLI 后再试"
	case "api_billing":
		return "按 API 计费，没有订阅额度窗口"
	case "error":
		return "读取失败：" + q.Error
	}
	return "没有额度窗口"
}

// cliQuotaWait 阻塞到订阅额度恢复：所有窗口用量都低于 below（默认 100）时退出 0；超时退出 1
func cliQuotaWait(args []string, flags map[string]string) (int, error) {
	if len(args) == 0 {
		return 2, fmt.Errorf("用法：quota wait <claude|codex|copilot> [--timeout 6h] [--below 100]")
	}
	provider := strings.ToLower(args[0])
	if provider != "claude" && provider != "codex" && provider != "copilot" {
		return 2, fmt.Errorf("未知订阅：%s", provider)
	}
	below := 100.0
	if v, err := strconv.ParseFloat(flags["below"], 64); err == nil && v > 0 {
		below = v
	}
	var deadline time.Time
	if flags["timeout"] != "" {
		d, err := time.ParseDuration(flags["timeout"])
		if err != nil {
			return 2, fmt.Errorf("--timeout 格式应如 30m、6h")
		}
		deadline = time.Now().Add(d)
	}
	qs := NewQuotaService(NewApp())
	wait := time.Minute
	for {
		var q *SubscriptionQuota
		for _, item := range qs.readQuotas(true, provider == "claude") {
			if item.Provider == provider {
				copied := item
				q = &copied
			}
		}
		if q == nil || q.Status == "signed_out" || q.Status == "not_installed" || q.Status == "api_billing" {
			status := "未登录"
			if q != nil {
				status = quotaStatusText(*q)
			}
			return 2, fmt.Errorf("%s：%s", provider, status)
		}
		blocked := false
		var soonest int64
		for _, w := range q.Windows {
			if !w.Unlimited && w.Used >= below {
				blocked = true
				if w.ResetsAt > 0 && (soonest == 0 || w.ResetsAt < soonest) {
					soonest = w.ResetsAt
				}
			}
		}
		if q.Status == "ok" && !blocked {
			fmt.Fprintf(os.Stderr, "%s 额度可用\n", q.Name)
			return 0, nil
		}
		next := time.Now().Add(wait)
		if soonest > 0 {
			reset := time.UnixMilli(soonest).Add(30 * time.Second)
			if reset.Before(time.Now().Add(10 * time.Minute)) {
				next = reset
			}
			fmt.Fprintf(os.Stderr, "等待 %s 额度恢复，预计 %s 重置\n", q.Name, time.UnixMilli(soonest).Format("01-02 15:04"))
		} else {
			fmt.Fprintf(os.Stderr, "等待 %s 额度恢复…\n", q.Name)
		}
		if !deadline.IsZero() && next.After(deadline) {
			if time.Now().After(deadline) {
				return 1, fmt.Errorf("等待超时")
			}
			next = deadline
		}
		time.Sleep(time.Until(next))
		if wait < 10*time.Minute {
			wait *= 2
		}
	}
}

func cliBalance(args []string) error {
	flags, _ := cliFlags(args)
	qs := NewQuotaService(NewApp())
	cards := qs.GetBalances(true)
	if flags["json"] == "true" {
		return printJSON(cards)
	}
	if len(cards) == 0 {
		fmt.Println("没有可识别余额接口的环境；可在界面「工作台 → 额度与余额」中为中转站设置接口类型")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "供应商\tKey\t余额\t环境")
	for _, c := range cards {
		value := c.Display
		if c.Error != "" {
			value = "失败：" + c.Error
		} else if c.Low {
			value += "（偏低）"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", c.Vendor, c.KeyHint, value, strings.Join(c.Environments, ", "))
	}
	return tw.Flush()
}

func cliSessions(args []string) error {
	flags, _ := cliFlags(args, "days", "tool")
	days := 30
	if v, err := strconv.Atoi(flags["days"]); err == nil && v >= 0 {
		days = v
	}
	ss := NewSessionService(NewApp())
	ins, err := ss.GetSessionInsights(SessionInsightQuery{Provider: flags["tool"], Days: days})
	if err != nil {
		return err
	}
	if flags["json"] == "true" {
		return printJSON(ins)
	}
	fmt.Printf("%s ~ %s：%d 个会话，%d 次提问，%d 次工具调用，%s Token（另有缓存读取 %s），活跃 %.1f 小时\n",
		ins.From, ins.To, ins.Sessions, ins.Prompts, ins.ToolCalls, compactInt(ins.InputTokens+ins.OutputTokens+ins.CacheWriteTokens),
		compactInt(ins.CacheReadTokens), float64(ins.ActiveMinutes)/60)
	section := func(title string, items []InsightCount, tokens bool) {
		if len(items) == 0 {
			return
		}
		fmt.Println("\n" + title)
		for i, it := range items {
			if i >= 8 {
				break
			}
			v := strconv.Itoa(it.Count)
			if tokens {
				v = compactInt(it.Tokens)
			}
			fmt.Printf("  %-40s %s\n", clipText(it.Name, 40), v)
		}
	}
	section("工具", ins.Tools, false)
	section("Skills", ins.Skills, false)
	section("MCP", ins.MCP, false)
	section("模型（Token）", ins.Models, true)
	section("项目", ins.Projects, false)
	return nil
}

func compactInt(n int64) string {
	switch {
	case n >= 1e9:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case n >= 1e6:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	}
	return strconv.FormatInt(n, 10)
}

func cliGateway() error {
	NewApp()
	rs := NewRouterService()
	info := rs.GetGatewayAccess()
	fmt.Printf("端口：%d\n局域网共享：%v\n", info.Port, map[bool]string{true: "开启", false: "关闭"}[info.LANShare])
	if info.LANShare {
		for _, a := range info.Addresses {
			fmt.Printf("  http://%s:%d/<路由>\n", a, info.Port)
		}
	}
	fmt.Printf("路由：%s\n网关密钥：%d 个\n", strings.Join(info.Routes, ", "), len(info.Keys))
	return nil
}

func parseTokenAmount(s string) (int64, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	mult := 1.0
	switch {
	case strings.HasSuffix(s, "k"):
		mult, s = 1e3, strings.TrimSuffix(s, "k")
	case strings.HasSuffix(s, "m"):
		mult, s = 1e6, strings.TrimSuffix(s, "m")
	case strings.HasSuffix(s, "b"):
		mult, s = 1e9, strings.TrimSuffix(s, "b")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("Token 数量格式应如 500k、2m")
	}
	return int64(v * mult), nil
}

func cliGatewayKey(args []string) error {
	flags, pos := cliFlags(args, "tokens", "cost")
	if len(pos) == 0 {
		pos = []string{"list"}
	}
	NewApp()
	rs := NewRouterService()
	find := func(id string) (GatewayKey, error) {
		for _, k := range rs.GetGatewayAccess().Keys {
			if k.ID == id || strings.EqualFold(k.Name, id) {
				return k, nil
			}
		}
		return GatewayKey{}, fmt.Errorf("找不到网关密钥 %s", id)
	}
	switch pos[0] {
	case "list", "ls":
		info := rs.GetGatewayAccess()
		if len(info.Keys) == 0 {
			fmt.Println("还没有网关密钥")
			return nil
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\t名称\t状态\t密钥\t本期用量\t上限")
		for _, k := range info.Keys {
			state := "启用"
			if !k.Enabled {
				state = "停用"
			}
			used, limit := "", "不限"
			if k.Usage != nil {
				used = compactInt(k.Usage.Tokens) + " Token"
			}
			if k.LimitPeriod != "" {
				parts := []string{}
				if k.LimitTokens > 0 {
					parts = append(parts, compactInt(k.LimitTokens)+" Token")
				}
				if k.LimitCost > 0 {
					parts = append(parts, fmt.Sprintf("$%g", k.LimitCost))
				}
				limit = periodLabel(k.LimitPeriod) + " " + strings.Join(parts, " / ")
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", k.ID, k.Name, state, maskSecretTail(k.Key), used, limit)
		}
		return tw.Flush()
	case "add":
		if len(pos) < 2 {
			return fmt.Errorf("用法：gateway-key add <名称>")
		}
		k, err := rs.AddGatewayKey(strings.Join(pos[1:], " "))
		if err != nil {
			return err
		}
		fmt.Printf("已创建「%s」（%s）\n%s\n", k.Name, k.ID, k.Key)
		return nil
	case "rotate":
		if len(pos) < 2 {
			return fmt.Errorf("用法：gateway-key rotate <id>")
		}
		k, err := find(pos[1])
		if err != nil {
			return err
		}
		k, err = rs.RotateGatewayKey(k.ID)
		if err != nil {
			return err
		}
		fmt.Printf("已轮换「%s」\n%s\n", k.Name, k.Key)
		return nil
	case "remove", "rm":
		if len(pos) < 2 {
			return fmt.Errorf("用法：gateway-key remove <id>")
		}
		k, err := find(pos[1])
		if err != nil {
			return err
		}
		if err := rs.DeleteGatewayKey(k.ID); err != nil {
			return err
		}
		fmt.Printf("已删除「%s」\n", k.Name)
		return nil
	case "limit":
		if len(pos) < 3 {
			return fmt.Errorf("用法：gateway-key limit <id> <day|week|month|off> [--tokens 2m] [--cost 5]")
		}
		k, err := find(pos[1])
		if err != nil {
			return err
		}
		k.Usage = nil
		if pos[2] == "off" {
			k.LimitPeriod, k.LimitTokens, k.LimitCost = "", 0, 0
		} else {
			k.LimitPeriod = pos[2]
			if v := flags["tokens"]; v != "" {
				if k.LimitTokens, err = parseTokenAmount(v); err != nil {
					return err
				}
			}
			if v := flags["cost"]; v != "" {
				if k.LimitCost, err = strconv.ParseFloat(v, 64); err != nil {
					return fmt.Errorf("--cost 应为美元金额")
				}
			}
		}
		if err := rs.UpdateGatewayKey(k); err != nil {
			return err
		}
		fmt.Println("已保存")
		return nil
	}
	return fmt.Errorf("未知子命令：%s", pos[0])
}

func cliModelCatalog(args []string) error {
	if len(args) == 0 || args[0] != "sync" {
		w := NewWorkbenchService(nil, nil, nil, nil)
		st := w.GetModelCatalogStatus()
		fmt.Printf("models.dev 目录：%d 个模型，更新于 %s\n", st.Count, time.UnixMilli(st.UpdatedAt).Format("2006-01-02 15:04"))
		return nil
	}
	fmt.Fprintln(os.Stderr, "正在下载 models.dev 目录…")
	w := NewWorkbenchService(nil, nil, nil, nil)
	st, err := w.SyncModelCatalog()
	if err != nil {
		return err
	}
	fmt.Printf("已同步 %d 个模型\n", st.Count)
	return nil
}
