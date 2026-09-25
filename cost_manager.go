package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

func repriceUsageRecords(records []UsageRecord) []UsageRecord {
	workbenchMu.Lock()
	c, e := loadWorkbench()
	workbenchMu.Unlock()
	if e != nil || len(c.Models) == 0 {
		return records
	}
	events, e := LoadEnvActivations()
	if e != nil {
		return records
	}
	for i := range records {
		r := &records[i]
		ts, e := parseTimestamp(r.Timestamp)
		if e != nil {
			continue
		}
		name := activeEnvAt(events[r.Provider], ts.Unix())
		for _, p := range c.Models {
			if p.Provider == r.Provider && p.Environment == name && p.Model == r.Model {
				multiplier := 1.0
				if n, ok := c.Costs.Multipliers[r.Provider+"/"+name]; ok {
					multiplier = n
				}
				r.TotalCost = (float64(r.InputTokens)*p.InputPrice + float64(r.OutputTokens)*p.OutputPrice + float64(r.CacheReadTokens)*p.CacheReadPrice + float64(r.CacheWriteTokens)*p.CacheWritePrice) / 1e6 * multiplier
				break
			}
		}
	}
	return records
}

type CostOverview struct {
	Today           float64            `json:"today"`
	Month           float64            `json:"month"`
	Requests        int                `json:"requests"`
	Unpriced        int                `json:"unpriced"`
	Input           int64              `json:"input"`
	Output          int64              `json:"output"`
	DailyExceeded   bool               `json:"daily_exceeded"`
	MonthlyExceeded bool               `json:"monthly_exceeded"`
	ByRoute         map[string]float64 `json:"by_route"`
}
type BalanceResult struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Message  string  `json:"message"`
}

func finiteNonnegative(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }
func (w *WorkbenchService) SaveCostSettings(s CostSettings) error {
	if !finiteNonnegative(s.DailyBudget) || !finiteNonnegative(s.MonthlyBudget) {
		return fmt.Errorf("预算必须是有效的非负金额")
	}
	for _, v := range s.Multipliers {
		if !finiteNonnegative(v) || v > 1000 {
			return fmt.Errorf("倍率必须在 0–1000 之间")
		}
	}
	for _, b := range s.BalanceSources {
		if b.Adapter != "openrouter" && b.Adapter != "deepseek" {
			return fmt.Errorf("不支持的余额适配器")
		}
	}
	workbenchMu.Lock()
	defer workbenchMu.Unlock()
	c, e := loadWorkbench()
	if e != nil {
		return e
	}
	c.Costs = s
	return saveWorkbench(c)
}
func (w *WorkbenchService) GetCostOverview() (CostOverview, error) {
	c, e := w.GetWorkbench()
	out := CostOverview{ByRoute: map[string]float64{}}
	if e != nil {
		return out, e
	}
	p, e := storePath("gateway-usage.jsonl")
	if e != nil {
		return out, e
	}
	f, e := os.Open(p)
	if os.IsNotExist(e) {
		return out, nil
	}
	if e != nil {
		return out, e
	}
	defer f.Close()
	now := time.Now()
	day := now.Format("2006-01-02")
	month := now.Format("2006-01")
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 4096), 1<<20)
	for scan.Scan() {
		var r RouterLogEntry
		if json.Unmarshal(scan.Bytes(), &r) != nil || !strings.HasPrefix(r.Time, month) {
			continue
		}
		out.Requests++
		out.Input += int64(r.InputTokens + r.CacheReadTokens + r.CacheWriteTokens)
		out.Output += int64(r.OutputTokens)
		var price *ModelProfile
		for i := range c.Models {
			m := &c.Models[i]
			if m.Model != r.Model {
				continue
			}
			env, err := w.env(m.Provider, m.Environment)
			if err != nil {
				continue
			}
			base, _, _ := upstreamVarsForEnv(&env)
			if upstreamHost(base) == r.Upstream {
				if price != nil {
					price = nil // Multiple environments match: do not guess their billing profile.
					break
				}
				price = m
			}
		}
		if price == nil {
			out.Unpriced++
			continue
		}
		multiplier := 1.0
		if v, ok := c.Costs.Multipliers[price.Provider+"/"+price.Environment]; ok {
			multiplier = v
		}
		cost := (float64(r.InputTokens)*price.InputPrice + float64(r.OutputTokens)*price.OutputPrice + float64(r.CacheReadTokens)*price.CacheReadPrice + float64(r.CacheWriteTokens)*price.CacheWritePrice) / 1e6 * multiplier
		out.Month += cost
		out.ByRoute[r.Route] += cost
		if strings.HasPrefix(r.Time, day) {
			out.Today += cost
		}
	}
	out.DailyExceeded = c.Costs.DailyBudget > 0 && out.Today >= c.Costs.DailyBudget
	out.MonthlyExceeded = c.Costs.MonthlyBudget > 0 && out.Month >= c.Costs.MonthlyBudget
	return out, scan.Err()
}
func (w *WorkbenchService) CheckBalance(source BalanceSource) (BalanceResult, error) {
	env, e := w.env(source.Provider, source.Environment)
	if e != nil {
		return BalanceResult{}, e
	}
	base, key, _ := upstreamVarsForEnv(&env)
	if e = validEndpoint(base); e != nil {
		return BalanceResult{}, e
	}
	path := ""
	switch source.Adapter {
	case "openrouter":
		path = "/credits"
	case "deepseek":
		path = "/user/balance"
	default:
		return BalanceResult{}, fmt.Errorf("请选择 OpenRouter 或 DeepSeek 余额接口")
	}
	req, e := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+path, nil)
	if e != nil {
		return BalanceResult{}, e
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, e := credentialClient(20 * time.Second).Do(req)
	if e != nil {
		return BalanceResult{}, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return BalanceResult{}, fmt.Errorf("余额接口返回 HTTP %d", resp.StatusCode)
	}
	var data map[string]any
	if e = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data); e != nil {
		return BalanceResult{}, e
	}
	if source.Adapter == "openrouter" {
		d := object(data, "data")
		total, ok := d["total_credits"].(float64)
		usage, ok2 := d["total_usage"].(float64)
		if !ok || !ok2 {
			return BalanceResult{}, fmt.Errorf("供应商未返回可识别余额")
		}
		return BalanceResult{total - usage, "USD", ""}, nil
	}
	items, _ := data["balance_infos"].([]any)
	if len(items) == 0 {
		return BalanceResult{}, fmt.Errorf("供应商未返回余额")
	}
	item, _ := items[0].(map[string]any)
	var amount float64
	if _, e = fmt.Sscan(asString(item["total_balance"]), &amount); e != nil {
		return BalanceResult{}, fmt.Errorf("余额格式无效")
	}
	return BalanceResult{amount, asString(item["currency"]), ""}, nil
}
