package main

// 网关请求账本导出：把 gateway-usage.jsonl（当月）与按月归档的账本导出为 CSV，
// 附带按模型档案估算的费用；没有档案时给出公开价目估算，单独成列，便于对账。

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// listPriceCost 按公开价目估算：内置价目精确命中 → models.dev 目录 → 内置价目模糊匹配
func listPriceCost(r RouterLogEntry) (float64, bool) {
	if _, exact := modelPricing[strings.ToLower(strings.TrimSpace(r.Model))]; !exact {
		if cost, ok := catalogPriceCost(r.Model, r.InputTokens, r.OutputTokens, r.CacheReadTokens, r.CacheWriteTokens); ok {
			return cost, true
		}
	}
	pricing, ok := lookupModelPricing(r.Model)
	if !ok {
		return 0, false
	}
	return (float64(r.InputTokens)*pricing.Input + float64(r.OutputTokens)*pricing.Output +
		float64(r.CacheReadTokens)*pricing.CacheRead + float64(r.CacheWriteTokens)*pricing.CacheCreate) / 1e6, true
}

func gatewayLedgerFile(month string) (string, error) {
	p, err := storePath("gateway-usage.jsonl")
	if err != nil {
		return "", err
	}
	if month == "" || month == time.Now().Format("2006-01") {
		return p, nil
	}
	if _, err := time.Parse("2006-01", month); err != nil {
		return "", fmt.Errorf("月份格式无效")
	}
	return p + "." + month, nil
}

// ListGatewayLedgerMonths 可导出的月份（当月在前）
func (w *WorkbenchService) ListGatewayLedgerMonths() []string {
	out := []string{time.Now().Format("2006-01")}
	p, err := storePath("gateway-usage.jsonl")
	if err != nil {
		return out
	}
	files, _ := filepath.Glob(p + ".????-??")
	months := []string{}
	for _, f := range files {
		m := strings.TrimPrefix(f, p+".")
		if _, err := time.Parse("2006-01", m); err == nil && m != out[0] {
			months = append(months, m)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(months)))
	return append(out, months...)
}

// ExportGatewayLedger 导出指定月份的网关请求账本为 CSV，返回保存路径（取消时为空）
func (w *WorkbenchService) ExportGatewayLedger(month string) (string, error) {
	if w.app == nil || w.app.ctx == nil {
		return "", fmt.Errorf("应用未就绪")
	}
	src, err := gatewayLedgerFile(month)
	if err != nil {
		return "", err
	}
	f, err := os.Open(src)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("该月份没有网关用量记录")
	}
	if err != nil {
		return "", err
	}
	defer f.Close()

	wb, _ := loadWorkbenchLocked()
	envs := currentEnvsSnapshot()
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM，便于 Excel 识别中文
	cw := csv.NewWriter(&buf)
	_ = cw.Write([]string{"time", "route", "model", "upstream", "status", "duration_ms", "first_token_ms",
		"input_tokens", "output_tokens", "cache_read_tokens", "cache_write_tokens",
		"cost_usd", "list_price_usd", "caller_key", "client", "failover", "error"})
	rows := 0
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 4096), 1<<20)
	for scan.Scan() {
		var r RouterLogEntry
		if json.Unmarshal(scan.Bytes(), &r) != nil {
			continue
		}
		cost, priced := gatewayEntryCost(wb, envs, r)
		costText, listText := "", ""
		if priced {
			costText = strconv.FormatFloat(cost, 'f', 6, 64)
		} else if lp, ok := listPriceCost(r); ok {
			listText = strconv.FormatFloat(lp, 'f', 6, 64)
		}
		_ = cw.Write([]string{r.Time, r.Route, r.Model, r.Upstream, strconv.Itoa(r.StatusCode),
			strconv.FormatInt(r.DurationMs, 10), strconv.FormatInt(r.FirstTokenMs, 10),
			strconv.Itoa(r.InputTokens), strconv.Itoa(r.OutputTokens), strconv.Itoa(r.CacheReadTokens), strconv.Itoa(r.CacheWriteTokens),
			costText, listText, r.CallerName, r.Client, r.Failover, r.Error})
		rows++
	}
	if err := scan.Err(); err != nil {
		return "", err
	}
	cw.Flush()
	if rows == 0 {
		return "", fmt.Errorf("该月份没有网关用量记录")
	}
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	target, err := runtime.SaveFileDialog(w.app.ctx, runtime.SaveDialogOptions{
		Title:           "导出网关请求账本",
		DefaultFilename: "aienv-gateway-" + month + ".csv",
		Filters:         []runtime.FileFilter{{DisplayName: "CSV", Pattern: "*.csv"}},
	})
	if err != nil || target == "" {
		return "", err
	}
	return target, writeFileAtomic(target, buf.Bytes(), 0o600)
}
