package main

// 供应商余额：按环境配置里的 Base URL 自动识别 DeepSeek、Moonshot、OpenRouter、
// SiliconFlow、StepFun、AiHubMix 等官方余额接口；中转站可选 new-api / one-api 接口，
// 其余供应商可自定义同域名的余额地址与 JSON 字段。Key 只会发往该环境 Base URL 的同一主机。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BalanceCard 一个 Key 的余额（同一 Key 被多个环境使用时合并为一张）
type BalanceCard struct {
	ID           string   `json:"id"`
	Adapter      string   `json:"adapter"`
	Vendor       string   `json:"vendor"`
	Host         string   `json:"host"`
	KeyHint      string   `json:"key_hint"`
	Environments []string `json:"environments"` // provider/name
	Amount       float64  `json:"amount"`
	Currency     string   `json:"currency"`
	Display      string   `json:"display"`
	Unlimited    bool     `json:"unlimited,omitempty"`
	AlertBelow   float64  `json:"alert_below,omitempty"`
	Low          bool     `json:"low,omitempty"`
	Configured   bool     `json:"configured,omitempty"` // 来自手动配置的余额来源
	Error        string   `json:"error,omitempty"`
	ReadAt       int64    `json:"read_at,omitempty"`
}

var balanceAdapters = map[string]string{
	"auto":        "自动识别",
	"openrouter":  "OpenRouter",
	"deepseek":    "DeepSeek",
	"moonshot":    "Moonshot",
	"siliconflow": "SiliconFlow",
	"stepfun":     "StepFun",
	"aihubmix":    "AiHubMix",
	"newapi":      "New API",
	"oneapi":      "One API",
	"custom":      "自定义",
}

// balanceAdapterForHost 按主机名识别官方余额接口
func balanceAdapterForHost(host string) string {
	switch strings.ToLower(host) {
	case "api.deepseek.com":
		return "deepseek"
	case "api.moonshot.cn", "api.moonshot.ai":
		return "moonshot"
	case "openrouter.ai":
		return "openrouter"
	case "api.siliconflow.cn", "api.siliconflow.com":
		return "siliconflow"
	case "api.stepfun.com", "api.stepfun.ai":
		return "stepfun"
	case "aihubmix.com", "api.aihubmix.com":
		return "aihubmix"
	}
	return ""
}

type balanceReading struct {
	amount    float64
	currency  string
	display   string
	unlimited bool
}

func currencySymbol(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "CNY", "RMB", "¥":
		return "¥"
	case "USD", "$", "":
		return "$"
	case "EUR":
		return "€"
	}
	return strings.ToUpper(code) + " "
}

func formatMoney(amount float64, currency string) string {
	return currencySymbol(currency) + strconv.FormatFloat(math.Round(amount*100)/100, 'f', 2, 64)
}

func balanceNumber(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, !math.IsNaN(x) && !math.IsInf(x, 0)
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return n, err == nil && !math.IsNaN(n) && !math.IsInf(n, 0)
	case json.Number:
		n, err := x.Float64()
		return n, err == nil
	}
	return 0, false
}

// jsonPathValue 读取点分路径（数组用下标）：data.balance、balance_infos.0.total_balance
func jsonPathValue(root any, path string) (any, bool) {
	cur := root
	for _, part := range strings.Split(strings.Trim(strings.TrimPrefix(strings.TrimSpace(path), "$"), "."), ".") {
		if part == "" {
			continue
		}
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[part]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(node) {
				return nil, false
			}
			cur = node[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

func originOf(raw string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", "", fmt.Errorf("Base URL 无效")
	}
	return u.Scheme + "://" + u.Host, u.Hostname(), nil
}

func balanceGet(rawURL, key string, dst any) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AI-ENV")
	resp, err := balanceClient(req.URL.Hostname()).Do(req)
	if err != nil {
		// 错误信息里可能带完整 URL，避免把 Key 原样带回界面
		return errors.New(strings.ReplaceAll(err.Error(), key, maskSecretTail(key)))
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	snippet := strings.Join(strings.Fields(string(b)), " ")
	isJSON := strings.HasPrefix(snippet, "{") || strings.HasPrefix(snippet, "[")
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := ""
		if isJSON {
			detail = "：" + clipText(snippet, 160)
		}
		switch {
		case !isJSON && resp.StatusCode == http.StatusForbidden || !isJSON && resp.StatusCode == http.StatusServiceUnavailable:
			return fmt.Errorf("站点对接口启用了网页安全验证（HTTP %d），无法自动查询余额", resp.StatusCode)
		case resp.StatusCode == http.StatusNotFound:
			return fmt.Errorf("站点没有这个余额接口（HTTP 404），请换一种接口类型")
		case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
			return fmt.Errorf("Key 无效或该接口不接受这个 Key（HTTP %d）%s", resp.StatusCode, detail)
		case resp.StatusCode >= 300 && resp.StatusCode < 400:
			return fmt.Errorf("余额接口跳转到了其它地址（HTTP %d），请换一种接口类型", resp.StatusCode)
		}
		return fmt.Errorf("余额接口返回 HTTP %d%s", resp.StatusCode, detail)
	}
	if !isJSON {
		return fmt.Errorf("余额接口返回的不是 JSON（可能是登录页或安全验证页），请换一种接口类型")
	}
	// new-api 风格的失败：HTTP 200 但 {"success":false,"message":"..."}
	var refusal struct {
		Success *bool  `json:"success"`
		Message string `json:"message"`
	}
	if json.Unmarshal(b, &refusal) == nil && refusal.Success != nil && !*refusal.Success && strings.TrimSpace(refusal.Message) != "" {
		return errors.New(clipText(strings.TrimSpace(refusal.Message), 160))
	}
	return json.Unmarshal(b, dst)
}

// balanceClient 只跟随同一主机的跳转（如补斜杠、http→https），Key 不会被带到其它域名
func balanceClient(host string) *http.Client {
	return &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !strings.EqualFold(req.URL.Hostname(), host) || via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
			return http.ErrUseLastResponse
		}
		return nil
	}}
}

// queryBalance 用指定适配器查询一个 Key 的余额
func queryBalance(baseURL, key string, src BalanceSource) (balanceReading, error) {
	if strings.TrimSpace(key) == "" {
		return balanceReading{}, fmt.Errorf("该环境没有 API Key")
	}
	if err := validEndpoint(baseURL); err != nil {
		return balanceReading{}, err
	}
	origin, host, err := originOf(baseURL)
	if err != nil {
		return balanceReading{}, err
	}
	adapter := src.Adapter
	if adapter == "" || adapter == "auto" {
		adapter = balanceAdapterForHost(host)
		if adapter == "" {
			return balanceReading{}, fmt.Errorf("无法根据 %s 自动识别余额接口，请选择接口类型", host)
		}
	}
	cnHost := strings.HasSuffix(strings.ToLower(host), ".cn")
	var data map[string]any
	switch adapter {
	case "deepseek":
		if err := balanceGet(origin+"/user/balance", key, &data); err != nil {
			return balanceReading{}, err
		}
		items, _ := data["balance_infos"].([]any)
		parts := []string{}
		var first balanceReading
		for i, raw := range items {
			item, _ := raw.(map[string]any)
			n, ok := balanceNumber(item["total_balance"])
			if !ok {
				continue
			}
			cur := asString(item["currency"])
			if i == 0 || first.currency == "" {
				first = balanceReading{amount: n, currency: cur}
			}
			parts = append(parts, formatMoney(n, cur))
		}
		if len(parts) == 0 {
			return balanceReading{}, fmt.Errorf("供应商未返回余额")
		}
		first.display = strings.Join(parts, " · ")
		return first, nil
	case "moonshot":
		if err := balanceGet(origin+"/v1/users/me/balance", key, &data); err != nil {
			return balanceReading{}, err
		}
		n, ok := balanceNumber(object(data, "data")["available_balance"])
		if !ok {
			return balanceReading{}, fmt.Errorf("供应商未返回余额")
		}
		cur := "USD"
		if cnHost {
			cur = "CNY"
		}
		return balanceReading{amount: n, currency: cur}, nil
	case "openrouter":
		if err := balanceGet("https://openrouter.ai/api/v1/credits", key, &data); err != nil {
			return balanceReading{}, err
		}
		d := object(data, "data")
		total, ok := balanceNumber(d["total_credits"])
		used, ok2 := balanceNumber(d["total_usage"])
		if !ok || !ok2 {
			return balanceReading{}, fmt.Errorf("供应商未返回可识别余额")
		}
		return balanceReading{amount: total - used, currency: "USD"}, nil
	case "siliconflow":
		if err := balanceGet(origin+"/v1/user/info", key, &data); err != nil {
			return balanceReading{}, err
		}
		n, ok := balanceNumber(object(data, "data")["totalBalance"])
		if !ok {
			return balanceReading{}, fmt.Errorf("供应商未返回余额")
		}
		cur := "USD"
		if cnHost {
			cur = "CNY"
		}
		return balanceReading{amount: n, currency: cur}, nil
	case "stepfun":
		if err := balanceGet(origin+"/v1/accounts", key, &data); err != nil {
			return balanceReading{}, err
		}
		n, ok := balanceNumber(data["balance"])
		if !ok {
			return balanceReading{}, fmt.Errorf("供应商未返回余额")
		}
		cur := "CNY"
		if strings.HasSuffix(strings.ToLower(host), ".ai") {
			cur = "USD"
		}
		return balanceReading{amount: n, currency: cur}, nil
	case "aihubmix":
		if err := balanceGet("https://aihubmix.com/dashboard/billing/remain", key, &data); err != nil {
			return balanceReading{}, err
		}
		n, ok := balanceNumber(data["total_usage"])
		if !ok {
			return balanceReading{}, fmt.Errorf("供应商未返回余额")
		}
		if n < 0 {
			return balanceReading{unlimited: true, currency: "USD", display: "不限额 Key"}, nil
		}
		return balanceReading{amount: n, currency: "USD"}, nil
	case "newapi":
		if err := balanceGet(origin+"/api/usage/token", key, &data); err != nil {
			return balanceReading{}, err
		}
		d := object(data, "data")
		if d == nil {
			return balanceReading{}, fmt.Errorf("不是 new-api 的令牌用量接口")
		}
		if unlimited, _ := d["unlimited_quota"].(bool); unlimited {
			return balanceReading{unlimited: true, currency: "USD", display: "不限额令牌"}, nil
		}
		n, ok := balanceNumber(d["total_available"])
		if !ok {
			granted, ok1 := balanceNumber(d["total_granted"])
			used, ok2 := balanceNumber(d["total_used"])
			if !ok1 || !ok2 {
				return balanceReading{}, fmt.Errorf("new-api 未返回余额")
			}
			n = granted - used
		}
		return balanceReading{amount: n / 500000, currency: "USD"}, nil
	case "oneapi":
		var sub map[string]any
		if err := balanceGet(origin+"/v1/dashboard/billing/subscription", key, &sub); err != nil {
			return balanceReading{}, err
		}
		limit, ok := balanceNumber(sub["hard_limit_usd"])
		if !ok {
			return balanceReading{}, fmt.Errorf("不是 one-api 的额度接口")
		}
		if limit >= 100000000 {
			return balanceReading{unlimited: true, currency: "USD", display: "不限额令牌"}, nil
		}
		now := time.Now()
		usageURL := fmt.Sprintf("%s/v1/dashboard/billing/usage?start_date=%s&end_date=%s", origin,
			now.AddDate(0, 0, -99).Format("2006-01-02"), now.AddDate(0, 0, 1).Format("2006-01-02"))
		var usage map[string]any
		if err := balanceGet(usageURL, key, &usage); err != nil {
			return balanceReading{}, err
		}
		used, _ := balanceNumber(usage["total_usage"])
		return balanceReading{amount: limit - used/100, currency: "USD"}, nil
	case "custom":
		target := strings.TrimSpace(src.URL)
		if err := validEndpoint(target); err != nil {
			return balanceReading{}, fmt.Errorf("余额地址无效：%v", err)
		}
		u, _ := url.Parse(target)
		if !strings.EqualFold(u.Hostname(), host) {
			return balanceReading{}, fmt.Errorf("余额地址必须与环境 Base URL 同一主机（%s），以免 Key 发往其它网站", host)
		}
		var root any
		if err := balanceGet(target, key, &root); err != nil {
			return balanceReading{}, err
		}
		v, ok := jsonPathValue(root, src.Path)
		if !ok {
			return balanceReading{}, fmt.Errorf("返回内容里没有字段 %s", src.Path)
		}
		n, ok := balanceNumber(v)
		if !ok {
			return balanceReading{}, fmt.Errorf("字段 %s 不是数字", src.Path)
		}
		if src.Divisor > 0 {
			n /= src.Divisor
		}
		cur := src.Currency
		if cur == "" {
			cur = "USD"
		}
		return balanceReading{amount: n, currency: cur}, nil
	}
	return balanceReading{}, fmt.Errorf("不支持的余额接口类型")
}

func validateBalanceSource(b BalanceSource) error {
	if _, ok := balanceAdapters[b.Adapter]; !ok {
		return fmt.Errorf("不支持的余额接口类型 %q", b.Adapter)
	}
	if !finiteNonnegative(b.Divisor) || !finiteNonnegative(b.AlertBelow) {
		return fmt.Errorf("换算比例与提醒阈值必须是有效的非负数")
	}
	if b.Adapter == "custom" {
		if err := validEndpoint(strings.TrimSpace(b.URL)); err != nil {
			return fmt.Errorf("自定义余额地址无效：%v", err)
		}
		if strings.TrimSpace(b.Path) == "" {
			return fmt.Errorf("请填写余额字段路径，如 data.balance")
		}
	}
	return nil
}

func maskSecretTail(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 8 {
		return "••••"
	}
	return key[:3] + "…" + key[len(key)-4:]
}

// GetBalances 汇总所有可识别或已配置余额接口的环境，同一 Key 合并查询
func (qs *QuotaService) GetBalances(force bool) []BalanceCard {
	qs.balanceMu.Lock()
	if !force && qs.balances != nil && time.Since(qs.balanceAt) < quotaCacheTTL {
		out := append([]BalanceCard{}, qs.balances...)
		qs.balanceMu.Unlock()
		return out
	}
	qs.balanceMu.Unlock()

	workbenchMu.Lock()
	wb, _ := loadWorkbench()
	workbenchMu.Unlock()
	configured := map[string]BalanceSource{}
	for _, s := range wb.Costs.BalanceSources {
		configured[s.Provider+"/"+s.Environment] = s
	}

	type job struct {
		card BalanceCard
		base string
		key  string
		src  BalanceSource
	}
	jobs := map[string]*job{}
	order := []string{}
	if qs.app != nil {
		qs.app.configMu.Lock()
		envs := append([]EnvConfig{}, qs.app.config.Environments...)
		qs.app.configMu.Unlock()
		for i := range envs {
			env := envs[i]
			if env.OfficialLogin {
				continue
			}
			base, key, _ := upstreamVarsForEnv(&env)
			if strings.TrimSpace(key) == "" || validEndpoint(base) != nil {
				continue
			}
			_, host, err := originOf(base)
			if err != nil {
				continue
			}
			envID := env.Provider + "/" + env.Name
			src, explicit := configured[envID]
			if !explicit {
				if balanceAdapterForHost(host) == "" {
					continue
				}
				src = BalanceSource{Provider: env.Provider, Environment: env.Name, Adapter: "auto"}
			}
			adapter := src.Adapter
			if adapter == "" || adapter == "auto" {
				adapter = balanceAdapterForHost(host)
			}
			sum := sha256.Sum256([]byte(adapter + "\x00" + strings.ToLower(host) + "\x00" + key + "\x00" + src.URL + "\x00" + src.Path))
			id := hex.EncodeToString(sum[:8])
			if existing, ok := jobs[id]; ok {
				existing.card.Environments = append(existing.card.Environments, envID)
				if src.AlertBelow > existing.card.AlertBelow {
					existing.card.AlertBelow = src.AlertBelow
				}
				existing.card.Configured = existing.card.Configured || explicit
				continue
			}
			vendor := balanceAdapters[adapter]
			if vendor == "" {
				vendor = host
			}
			jobs[id] = &job{
				card: BalanceCard{ID: id, Adapter: adapter, Vendor: vendor, Host: host, KeyHint: maskSecretTail(key),
					Environments: []string{envID}, AlertBelow: src.AlertBelow, Configured: explicit},
				base: base, key: key, src: src,
			}
			order = append(order, id)
		}
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, id := range order {
		j := jobs[id]
		wg.Add(1)
		go func(j *job) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			r, err := queryBalance(j.base, j.key, j.src)
			j.card.ReadAt = time.Now().UnixMilli()
			if err != nil {
				j.card.Error = err.Error()
				return
			}
			j.card.Amount, j.card.Currency, j.card.Unlimited = r.amount, strings.ToUpper(r.currency), r.unlimited
			j.card.Display = r.display
			if j.card.Display == "" {
				j.card.Display = formatMoney(r.amount, r.currency)
			}
			j.card.Low = !r.unlimited && j.card.AlertBelow > 0 && r.amount < j.card.AlertBelow
		}(j)
	}
	wg.Wait()

	out := make([]BalanceCard, 0, len(order))
	for _, id := range order {
		c := jobs[id].card
		sort.Strings(c.Environments)
		out = append(out, c)
	}
	qs.balanceMu.Lock()
	qs.balances, qs.balanceAt = out, time.Now()
	qs.balanceMu.Unlock()
	return append([]BalanceCard{}, out...)
}

// SaveBalanceSource 为一个环境保存（或用 adapter 为空删除）余额接口配置
func (qs *QuotaService) SaveBalanceSource(src BalanceSource) error {
	src.Provider, src.Environment = strings.TrimSpace(src.Provider), strings.TrimSpace(src.Environment)
	src.URL, src.Path = strings.TrimSpace(src.URL), strings.TrimSpace(src.Path)
	if src.Provider == "" || src.Environment == "" {
		return fmt.Errorf("请选择环境")
	}
	remove := src.Adapter == ""
	if !remove {
		if err := validateBalanceSource(src); err != nil {
			return err
		}
	}
	workbenchMu.Lock()
	c, err := loadWorkbench()
	if err != nil {
		workbenchMu.Unlock()
		return err
	}
	next := make([]BalanceSource, 0, len(c.Costs.BalanceSources)+1)
	for _, b := range c.Costs.BalanceSources {
		if b.Provider != src.Provider || b.Environment != src.Environment {
			next = append(next, b)
		}
	}
	if !remove {
		next = append(next, src)
	}
	c.Costs.BalanceSources = next
	err = saveWorkbench(c)
	workbenchMu.Unlock()
	if err == nil {
		qs.balanceMu.Lock()
		qs.balanceAt = time.Time{}
		qs.balanceMu.Unlock()
	}
	return err
}

// TestBalanceSource 不保存，直接用该配置查询一次
func (qs *QuotaService) TestBalanceSource(src BalanceSource) (BalanceResult, error) {
	if err := validateBalanceSource(src); err != nil {
		return BalanceResult{}, err
	}
	if qs.app == nil {
		return BalanceResult{}, fmt.Errorf("应用未就绪")
	}
	env, ok := qs.app.findEnvCopy(src.Provider, src.Environment)
	if !ok {
		return BalanceResult{}, fmt.Errorf("环境不存在")
	}
	base, key, _ := upstreamVarsForEnv(&env)
	r, err := queryBalance(base, key, src)
	if err != nil {
		return BalanceResult{}, err
	}
	msg := r.display
	if msg == "" {
		msg = formatMoney(r.amount, r.currency)
	}
	return BalanceResult{Amount: r.amount, Currency: strings.ToUpper(r.currency), Message: msg}, nil
}

func (a *App) findEnvCopy(provider, name string) (EnvConfig, bool) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	for _, e := range a.config.Environments {
		if e.Provider == provider && e.Name == name {
			copied := e
			copied.Variables = map[string]string{}
			for k, v := range e.Variables {
				copied.Variables[k] = v
			}
			return copied, true
		}
	}
	return EnvConfig{}, false
}
