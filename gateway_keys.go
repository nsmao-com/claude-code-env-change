package main

// 局域网共享与网关密钥：开启共享后网关监听 0.0.0.0，其它电脑必须携带网关密钥访问；
// 每个密钥可单独停用、轮换、限定可用路由，并按日 / 周 / 月设置 Token 或费用上限。
// 本机请求保持原有规则（无需密钥），携带密钥时同样计入该密钥的用量与限额。

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// GatewayKey 供其它设备 / 客户端调用网关的密钥（与上游供应商的 API Key 无关）
type GatewayKey struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Key         string   `json:"key"`
	Enabled     bool     `json:"enabled"`
	CreatedAt   int64    `json:"created_at"`
	Routes      []string `json:"routes,omitempty"`       // 允许访问的路由，空表示全部
	LimitPeriod string   `json:"limit_period,omitempty"` // "" | day | week | month
	LimitTokens int64    `json:"limit_tokens,omitempty"`
	LimitCost   float64  `json:"limit_cost,omitempty"` // USD，按模型档案价格估算
	// Usage 仅在 GetGatewayAccess 返回时填充，不写入 router.json
	Usage *GatewayKeyUsage `json:"usage,omitempty"`
}

// GatewayKeyUsage 密钥在当前周期内的用量
type GatewayKeyUsage struct {
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
	Requests int     `json:"requests"`
	ResetsAt int64   `json:"resets_at,omitempty"`
	Exceeded bool    `json:"exceeded"`
}

type GatewayAccessInfo struct {
	LANShare  bool         `json:"lan_share"`
	Port      int          `json:"port"`
	Running   bool         `json:"running"`
	Addresses []string     `json:"addresses"`
	Keys      []GatewayKey `json:"keys"`
	Routes    []string     `json:"routes"`
}

type callerUse struct {
	at     time.Time
	tokens int64
	cost   float64
}

var callerLedger = struct {
	sync.Mutex
	loaded bool
	uses   map[string][]callerUse // key ID -> 最近 32 天的用量
}{uses: map[string][]callerUse{}}

type callerKeyCtx struct{}

type callerInfo struct {
	id, name, client string
}

func callerFrom(r *http.Request) *callerInfo {
	if r == nil {
		return nil
	}
	c, _ := r.Context().Value(callerKeyCtx{}).(*callerInfo)
	return c
}

func newGatewayKeySecret() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "sk-aienv-" + hex.EncodeToString(b), nil
}

func newGatewayKeyID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// gatewayTokenFrom 依次读取 Bearer、x-api-key、x-goog-api-key 与 ?key=
func gatewayTokenFrom(r *http.Request) (token string, fromQuery bool) {
	if v := strings.TrimSpace(r.Header.Get("Authorization")); v != "" {
		if len(v) > 7 && strings.EqualFold(v[:7], "bearer ") {
			return strings.TrimSpace(v[7:]), false
		}
		return v, false
	}
	if v := strings.TrimSpace(r.Header.Get("X-Api-Key")); v != "" {
		return v, false
	}
	if v := strings.TrimSpace(r.Header.Get("X-Goog-Api-Key")); v != "" {
		return v, false
	}
	if v := strings.TrimSpace(r.URL.Query().Get("key")); v != "" {
		return v, true
	}
	return "", false
}

func isLoopbackRemoteAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// rejectBrowserRequest 局域网请求同样不接受网页发起的跨站调用
func rejectBrowserRequest(r *http.Request) string {
	switch strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))) {
	case "", "none", "same-origin":
	default:
		return "网关拒绝来自网页的跨站请求"
	}
	if strings.TrimSpace(r.Header.Get("Origin")) != "" {
		return "网关拒绝来自网页的跨站请求"
	}
	return ""
}

func (rs *RouterService) matchGatewayKey(token string) (GatewayKey, bool) {
	if token == "" || !strings.HasPrefix(token, "sk-aienv-") {
		return GatewayKey{}, false
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for _, k := range rs.config.GatewayKeys {
		if k.Enabled && subtle.ConstantTimeCompare([]byte(k.Key), []byte(token)) == 1 {
			copied := k
			copied.Routes = append([]string(nil), k.Routes...)
			return copied, true
		}
	}
	return GatewayKey{}, false
}

// accessGuard 网关入口鉴权：本机沿用原有规则；局域网请求必须带有效网关密钥
func (rs *RouterService) accessGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.mu.Lock()
		lan := rs.config.LANShare
		rs.mu.Unlock()
		token, fromQuery := gatewayTokenFrom(r)
		key, matched := rs.matchGatewayKey(token)
		remote := lan && !isLoopbackRemoteAddr(r.RemoteAddr)
		if !remote {
			if reason := rejectGatewayRequest(r); reason != "" {
				writeJSONError(w, http.StatusForbidden, reason)
				return
			}
		} else {
			if reason := rejectBrowserRequest(r); reason != "" {
				writeJSONError(w, http.StatusForbidden, reason)
				return
			}
			if !matched {
				writeJSONError(w, http.StatusUnauthorized, "需要有效的网关密钥：在 AI ENV 的「路由 → 局域网共享」中创建，并作为 API Key 使用")
				return
			}
		}
		if matched {
			routeName := strings.Split(strings.Trim(r.URL.Path, "/"), "/")[0]
			if len(key.Routes) > 0 && !sliceHasFold(key.Routes, routeName) {
				writeJSONError(w, http.StatusForbidden, fmt.Sprintf("网关密钥「%s」不允许访问路由 %s", key.Name, routeName))
				return
			}
			if usage := gatewayKeyUsage(key, time.Now()); usage.Exceeded {
				if usage.ResetsAt > 0 {
					secs := time.Until(time.UnixMilli(usage.ResetsAt)).Seconds()
					w.Header().Set("Retry-After", fmt.Sprintf("%d", int64(secs)+1))
				}
				writeJSONError(w, http.StatusTooManyRequests, fmt.Sprintf("网关密钥「%s」已达到%s上限，%s 重置", key.Name,
					periodLabel(key.LimitPeriod), time.UnixMilli(usage.ResetsAt).Format("01-02 15:04")))
				return
			}
			if fromQuery {
				q := r.URL.Query()
				q.Del("key")
				r.URL.RawQuery = q.Encode()
			}
			client := ""
			if remote {
				client, _, _ = net.SplitHostPort(r.RemoteAddr)
			}
			r = r.WithContext(context.WithValue(r.Context(), callerKeyCtx{}, &callerInfo{id: key.ID, name: key.Name, client: client}))
		}
		r, id := withRequestID(r)
		keep := rs.captureEnabled()
		if keep || otelBodiesEnabled() {
			cw, cr, finish := captureRequest(w, r, id, keep)
			defer func() {
				item := finish()
				otelComplete(id, &item)
			}()
			w, r = cw, cr
		} else {
			defer otelComplete(id, nil)
		}
		next.ServeHTTP(w, r)
	})
}

func sliceHasFold(items []string, v string) bool {
	for _, it := range items {
		if strings.EqualFold(strings.TrimSpace(it), strings.TrimSpace(v)) {
			return true
		}
	}
	return false
}

func periodLabel(p string) string {
	switch p {
	case "day":
		return "今日"
	case "week":
		return "本周"
	case "month":
		return "本月"
	}
	return ""
}

// periodWindow 按本地时间的自然日 / 周（周一起）/ 月
func periodWindow(period string, now time.Time) (start, end time.Time, ok bool) {
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	switch period {
	case "day":
		return today, today.AddDate(0, 0, 1), true
	case "week":
		offset := (int(today.Weekday()) + 6) % 7
		start = today.AddDate(0, 0, -offset)
		return start, start.AddDate(0, 0, 7), true
	case "month":
		start = time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
		return start, start.AddDate(0, 1, 0), true
	}
	return time.Time{}, time.Time{}, false
}

// loadCallerLedger 首次使用时从网关用量账本恢复各密钥最近的用量，重启后限额仍然有效
func loadCallerLedger() {
	callerLedger.Lock()
	defer callerLedger.Unlock()
	if callerLedger.loaded {
		return
	}
	callerLedger.loaded = true
	p, err := storePath("gateway-usage.jsonl")
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -32)
	files := []string{p}
	if archived, _ := filepath.Glob(p + ".????-??"); len(archived) > 0 {
		sort.Strings(archived)
		files = append(archived[max(0, len(archived)-1):], p)
	}
	wb, _ := loadWorkbenchLocked()
	envs := currentEnvsSnapshot()
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			continue
		}
		scan := bufio.NewScanner(f)
		scan.Buffer(make([]byte, 4096), 1<<20)
		for scan.Scan() {
			var e RouterLogEntry
			if json.Unmarshal(scan.Bytes(), &e) != nil || e.CallerKey == "" {
				continue
			}
			at, err := time.ParseInLocation("2006-01-02 15:04:05", e.Time, time.Local)
			if err != nil || at.Before(cutoff) {
				continue
			}
			cost, _ := gatewayEntryCost(wb, envs, e)
			callerLedger.uses[e.CallerKey] = append(callerLedger.uses[e.CallerKey], callerUse{at: at, tokens: callerTokens(e), cost: cost})
		}
		f.Close()
	}
}

func callerTokens(e RouterLogEntry) int64 {
	return int64(e.InputTokens + e.OutputTokens + e.CacheWriteTokens)
}

// recordCallerUse 请求结束时记入密钥用量
func recordCallerUse(e RouterLogEntry) {
	if e.CallerKey == "" || !e.UsageReported {
		return
	}
	loadCallerLedger()
	wb, _ := loadWorkbenchLocked()
	cost, _ := gatewayEntryCost(wb, currentEnvsSnapshot(), e)
	callerLedger.Lock()
	defer callerLedger.Unlock()
	cutoff := time.Now().AddDate(0, 0, -32)
	list := callerLedger.uses[e.CallerKey]
	kept := list[:0]
	for _, u := range list {
		if u.at.After(cutoff) {
			kept = append(kept, u)
		}
	}
	callerLedger.uses[e.CallerKey] = append(kept, callerUse{at: time.Now(), tokens: callerTokens(e), cost: cost})
}

func gatewayKeyUsage(k GatewayKey, now time.Time) GatewayKeyUsage {
	loadCallerLedger()
	start, end, ok := periodWindow(k.LimitPeriod, now)
	if !ok {
		// 未设周期时展示本月用量
		start, end, _ = periodWindow("month", now)
	}
	out := GatewayKeyUsage{ResetsAt: end.UnixMilli()}
	callerLedger.Lock()
	for _, u := range callerLedger.uses[k.ID] {
		if !u.at.Before(start) && u.at.Before(end) {
			out.Tokens += u.tokens
			out.Cost += u.cost
			out.Requests++
		}
	}
	callerLedger.Unlock()
	if ok {
		out.Exceeded = k.LimitTokens > 0 && out.Tokens >= k.LimitTokens || k.LimitCost > 0 && out.Cost >= k.LimitCost
	} else {
		out.ResetsAt = 0
	}
	return out
}

func loadWorkbenchLocked() (WorkbenchConfig, error) {
	workbenchMu.Lock()
	defer workbenchMu.Unlock()
	return loadWorkbench()
}

func currentEnvsSnapshot() []EnvConfig {
	if globalApp == nil {
		return nil
	}
	globalApp.configMu.Lock()
	defer globalApp.configMu.Unlock()
	return append([]EnvConfig{}, globalApp.config.Environments...)
}

// gatewayEntryCost 按模型档案价格估算一条网关记录的费用；同一模型命中多个环境时不猜测
func gatewayEntryCost(c WorkbenchConfig, envs []EnvConfig, r RouterLogEntry) (float64, bool) {
	var price *ModelProfile
	for i := range c.Models {
		m := &c.Models[i]
		if m.Model != r.Model {
			continue
		}
		for j := range envs {
			env := envs[j]
			if env.Provider != m.Provider || env.Name != m.Environment {
				continue
			}
			base, _, _ := upstreamVarsForEnv(&env)
			if upstreamHost(base) == r.Upstream {
				if price != nil {
					return 0, false
				}
				price = m
			}
		}
	}
	if price == nil {
		return 0, false
	}
	multiplier := 1.0
	if v, ok := c.Costs.Multipliers[price.Provider+"/"+price.Environment]; ok {
		multiplier = v
	}
	return (float64(r.InputTokens)*price.InputPrice + float64(r.OutputTokens)*price.OutputPrice + float64(r.CacheReadTokens)*price.CacheReadPrice + float64(r.CacheWriteTokens)*price.CacheWritePrice) / 1e6 * multiplier, true
}

// lanAddresses 本机可供局域网访问的 IPv4 地址
func lanAddresses() []string {
	out := []string{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil || ipnet.IP.IsLinkLocalUnicast() {
				continue
			}
			out = append(out, ipnet.IP.String())
		}
	}
	// 常见家庭 / 办公网段排在前面，虚拟网卡（WSL、Hyper-V、代理 TUN）靠后
	rank := func(ip string) int {
		switch {
		case strings.HasPrefix(ip, "192.168."):
			return 0
		case strings.HasPrefix(ip, "10."):
			return 1
		case strings.HasPrefix(ip, "172."):
			return 2
		}
		return 3
	}
	sort.SliceStable(out, func(i, j int) bool {
		if rank(out[i]) != rank(out[j]) {
			return rank(out[i]) < rank(out[j])
		}
		return out[i] < out[j]
	})
	return out
}

// GetGatewayAccess 局域网共享状态与密钥列表（含当前周期用量）
func (rs *RouterService) GetGatewayAccess() GatewayAccessInfo {
	rs.mu.Lock()
	info := GatewayAccessInfo{LANShare: rs.config.LANShare, Port: rs.config.Port, Running: rs.running, Keys: []GatewayKey{}, Routes: []string{}}
	keys := append([]GatewayKey{}, rs.config.GatewayKeys...)
	for _, r := range rs.config.Routes {
		info.Routes = append(info.Routes, r.Name)
	}
	rs.mu.Unlock()
	info.Addresses = lanAddresses()
	now := time.Now()
	for _, k := range keys {
		usage := gatewayKeyUsage(k, now)
		k.Usage = &usage
		info.Keys = append(info.Keys, k)
	}
	return info
}

// persistAccessConfig 保存密钥类改动；不重启网关，避免中断进行中的流式请求
func (rs *RouterService) persistAccessConfig(mutate func(c *RouterConfig) error, restart bool) error {
	rs.mu.Lock()
	next := rs.config
	next.GatewayKeys = append([]GatewayKey{}, rs.config.GatewayKeys...)
	if err := mutate(&next); err != nil {
		rs.mu.Unlock()
		return err
	}
	path, err := rs.configPath()
	if err != nil {
		rs.mu.Unlock()
		return err
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		rs.mu.Unlock()
		return err
	}
	if err := writeFileAtomic(path, data, 0o600); err != nil {
		rs.mu.Unlock()
		return err
	}
	rs.config = next
	wasRunning := rs.running
	rs.mu.Unlock()
	notifyCloudSync()
	if restart && wasRunning {
		_ = rs.StopGateway()
		if err := rs.StartGateway(); err != nil {
			return fmt.Errorf("设置已保存，但网关重启失败: %v", err)
		}
	}
	return nil
}

// SetLANShare 开关局域网共享；首次开启时自动创建一个密钥
func (rs *RouterService) SetLANShare(on bool) error {
	return rs.persistAccessConfig(func(c *RouterConfig) error {
		c.LANShare = on
		if on && len(c.GatewayKeys) == 0 {
			secret, err := newGatewayKeySecret()
			if err != nil {
				return err
			}
			c.GatewayKeys = append(c.GatewayKeys, GatewayKey{ID: newGatewayKeyID(), Name: "默认", Key: secret, Enabled: true, CreatedAt: time.Now().UnixMilli()})
		}
		return nil
	}, true)
}

func validateGatewayKey(k GatewayKey) error {
	if strings.TrimSpace(k.Name) == "" || len([]rune(k.Name)) > 40 {
		return fmt.Errorf("密钥名称不能为空，且不超过 40 个字")
	}
	switch k.LimitPeriod {
	case "", "day", "week", "month":
	default:
		return fmt.Errorf("限额周期无效")
	}
	if k.LimitTokens < 0 || !finiteNonnegative(k.LimitCost) {
		return fmt.Errorf("限额必须是非负数")
	}
	if k.LimitPeriod == "" && (k.LimitTokens > 0 || k.LimitCost > 0) {
		return fmt.Errorf("设置限额前请选择限额周期")
	}
	return nil
}

// AddGatewayKey 新建密钥
func (rs *RouterService) AddGatewayKey(name string) (GatewayKey, error) {
	secret, err := newGatewayKeySecret()
	if err != nil {
		return GatewayKey{}, err
	}
	k := GatewayKey{ID: newGatewayKeyID(), Name: strings.TrimSpace(name), Key: secret, Enabled: true, CreatedAt: time.Now().UnixMilli()}
	if err := validateGatewayKey(k); err != nil {
		return GatewayKey{}, err
	}
	err = rs.persistAccessConfig(func(c *RouterConfig) error {
		if len(c.GatewayKeys) >= 50 {
			return fmt.Errorf("最多 50 个网关密钥")
		}
		c.GatewayKeys = append(c.GatewayKeys, k)
		return nil
	}, false)
	return k, err
}

// UpdateGatewayKey 修改名称、启用状态、可用路由与限额（不改密钥本身）
func (rs *RouterService) UpdateGatewayKey(update GatewayKey) error {
	if err := validateGatewayKey(update); err != nil {
		return err
	}
	return rs.persistAccessConfig(func(c *RouterConfig) error {
		for i := range c.GatewayKeys {
			if c.GatewayKeys[i].ID != update.ID {
				continue
			}
			k := &c.GatewayKeys[i]
			k.Name = strings.TrimSpace(update.Name)
			k.Enabled = update.Enabled
			k.LimitPeriod, k.LimitTokens, k.LimitCost = update.LimitPeriod, update.LimitTokens, update.LimitCost
			routes := []string{}
			for _, r := range update.Routes {
				if r = strings.TrimSpace(r); r != "" {
					routes = append(routes, r)
				}
			}
			k.Routes = routes
			k.Usage = nil
			return nil
		}
		return fmt.Errorf("网关密钥不存在")
	}, false)
}

// RotateGatewayKey 换一个新密钥值，保留名称、设置与用量
func (rs *RouterService) RotateGatewayKey(id string) (GatewayKey, error) {
	secret, err := newGatewayKeySecret()
	if err != nil {
		return GatewayKey{}, err
	}
	var out GatewayKey
	err = rs.persistAccessConfig(func(c *RouterConfig) error {
		for i := range c.GatewayKeys {
			if c.GatewayKeys[i].ID == id {
				c.GatewayKeys[i].Key = secret
				out = c.GatewayKeys[i]
				return nil
			}
		}
		return fmt.Errorf("网关密钥不存在")
	}, false)
	return out, err
}

// DeleteGatewayKey 删除密钥，持有它的设备立即失去访问权限
func (rs *RouterService) DeleteGatewayKey(id string) error {
	return rs.persistAccessConfig(func(c *RouterConfig) error {
		kept := c.GatewayKeys[:0]
		found := false
		for _, k := range c.GatewayKeys {
			if k.ID == id {
				found = true
				continue
			}
			kept = append(kept, k)
		}
		if !found {
			return fmt.Errorf("网关密钥不存在")
		}
		c.GatewayKeys = kept
		return nil
	}, false)
}
