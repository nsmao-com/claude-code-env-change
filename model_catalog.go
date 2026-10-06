package main

// models.dev 模型目录：下载 https://models.dev/api.json，按模型 ID 去重后精简缓存到
// ~/.claude-env-switcher/models-catalog.json，用于给模型档案补全上下文 / 输出上限与公开价格，
// 以及在没有模型档案、内置价目也查不到时估算费用。目录超过 7 天会在后台自动刷新。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	modelCatalogFile   = "models-catalog.json"
	modelCatalogMaxAge = 7 * 24 * time.Hour
)

var modelsDevURL = "https://models.dev/api.json"

// CatalogModel 目录中的一个模型（价格单位：美元 / 百万 Token）
type CatalogModel struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Maker      string  `json:"maker"`
	Context    int     `json:"context,omitempty"`
	Output     int     `json:"output,omitempty"`
	Input      float64 `json:"input,omitempty"`
	OutputCost float64 `json:"output_cost,omitempty"`
	CacheRead  float64 `json:"cache_read,omitempty"`
	CacheWrite float64 `json:"cache_write,omitempty"`
	Priced     bool    `json:"priced,omitempty"`
	Reasoning  bool    `json:"reasoning,omitempty"`
	Images     bool    `json:"images,omitempty"`
	Released   string  `json:"released,omitempty"`
}

type CatalogStatus struct {
	Count     int    `json:"count"`
	UpdatedAt int64  `json:"updated_at"`
	Syncing   bool   `json:"syncing"`
	Error     string `json:"error,omitempty"`
}

type modelCatalogFileData struct {
	UpdatedAt int64          `json:"updated_at"`
	Models    []CatalogModel `json:"models"`
}

var modelCatalog = struct {
	sync.Mutex
	loaded  bool
	byID    map[string]CatalogModel
	updated int64
	syncing bool
	lastErr string
}{}

type mdModel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Reasoning  bool   `json:"reasoning"`
	Released   string `json:"release_date"`
	Canonical  string `json:"canonical_model_id"`
	Modalities struct {
		Input []string `json:"input"`
	} `json:"modalities"`
	Limit struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
	Cost *struct {
		Input      float64 `json:"input"`
		Output     float64 `json:"output"`
		CacheRead  float64 `json:"cache_read"`
		CacheWrite float64 `json:"cache_write"`
	} `json:"cost"`
}

// firstPartyProviders 模型厂商自己的 models.dev 条目：价格与上限以它们为准，而不是转售平台
var firstPartyProviders = map[string]bool{
	"anthropic": true, "openai": true, "google": true, "deepseek": true, "moonshotai": true, "moonshotai-cn": true,
	"zai": true, "zhipuai": true, "xai": true, "mistral": true, "alibaba": true, "minimax": true, "cohere": true, "stepfun": true,
}

// catalogKey 统一模型 ID：去掉 "openrouter/anthropic/" 一类的前缀，小写
func catalogKey(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if i := strings.LastIndex(model, "/"); i >= 0 {
		model = model[i+1:]
	}
	return strings.TrimSuffix(model, ":free")
}

// parseModelsDev 把 models.dev 的 {provider: {models: {...}}} 按模型 ID 合并：
// 模型厂商自己的条目（canonical_model_id 的前缀等于 provider）优先，其次是带价格的条目
func parseModelsDev(raw []byte) ([]CatalogModel, error) {
	var providers map[string]struct {
		Models map[string]mdModel `json:"models"`
	}
	if err := json.Unmarshal(raw, &providers); err != nil {
		return nil, err
	}
	if len(providers) == 0 {
		return nil, fmt.Errorf("models.dev 返回内容为空")
	}
	type pick struct {
		m     CatalogModel
		score int
	}
	best := map[string]pick{}
	ids := make([]string, 0, len(providers))
	for pid := range providers {
		ids = append(ids, pid)
	}
	sort.Strings(ids)
	for _, pid := range ids {
		for id, x := range providers[pid].Models {
			if x.ID == "" {
				x.ID = id
			}
			key := catalogKey(x.ID)
			if key == "" {
				continue
			}
			maker := pid
			score := 0
			if canon := strings.ToLower(x.Canonical); canon != "" {
				if prefix, _, ok := strings.Cut(canon, "/"); ok {
					maker = prefix
					if prefix == pid {
						score += 4
					}
				}
			}
			if firstPartyProviders[pid] {
				maker = pid
				score += 4
			}
			cm := CatalogModel{ID: key, Name: x.Name, Maker: maker, Context: x.Limit.Context, Output: x.Limit.Output,
				Reasoning: x.Reasoning, Released: x.Released}
			for _, in := range x.Modalities.Input {
				if in == "image" {
					cm.Images = true
				}
			}
			if x.Cost != nil {
				cm.Input, cm.OutputCost, cm.CacheRead, cm.CacheWrite = x.Cost.Input, x.Cost.Output, x.Cost.CacheRead, x.Cost.CacheWrite
				cm.Priced = x.Cost.Input > 0 || x.Cost.Output > 0
				if cm.Priced {
					score += 2
				}
			}
			if cm.Context > 0 {
				score++
			}
			if prev, ok := best[key]; !ok || score > prev.score {
				best[key] = pick{cm, score}
			}
		}
	}
	out := make([]CatalogModel, 0, len(best))
	for _, p := range best {
		out = append(out, p.m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func loadModelCatalogLocked() {
	if modelCatalog.loaded {
		return
	}
	modelCatalog.loaded = true
	modelCatalog.byID = map[string]CatalogModel{}
	p, err := storePath(modelCatalogFile)
	if err != nil {
		return
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var data modelCatalogFileData
	if json.Unmarshal(b, &data) != nil {
		return
	}
	for _, m := range data.Models {
		modelCatalog.byID[m.ID] = m
	}
	modelCatalog.updated = data.UpdatedAt
}

// lookupCatalogModel 精确匹配模型 ID；带日期或后缀的变体（如 -20250929、-thinking）退到最长前缀
func lookupCatalogModel(model string) (CatalogModel, bool) {
	key := catalogKey(model)
	if key == "" {
		return CatalogModel{}, false
	}
	modelCatalog.Lock()
	defer modelCatalog.Unlock()
	loadModelCatalogLocked()
	if m, ok := modelCatalog.byID[key]; ok {
		return m, true
	}
	best, bestLen := CatalogModel{}, 0
	for id, m := range modelCatalog.byID {
		if len(id) > bestLen && len(id) >= 6 && strings.HasPrefix(key, id+"-") {
			best, bestLen = m, len(id)
		}
	}
	return best, bestLen > 0
}

// catalogPriceCost 按目录价格估算一条用量
func catalogPriceCost(model string, input, output, cacheRead, cacheWrite int) (float64, bool) {
	m, ok := lookupCatalogModel(model)
	if !ok || !m.Priced {
		return 0, false
	}
	return (float64(input)*m.Input + float64(output)*m.OutputCost + float64(cacheRead)*m.CacheRead + float64(cacheWrite)*m.CacheWrite) / 1e6, true
}

func syncModelCatalog(ctx context.Context) (int, error) {
	modelCatalog.Lock()
	if modelCatalog.syncing {
		modelCatalog.Unlock()
		return 0, fmt.Errorf("目录正在同步")
	}
	modelCatalog.syncing = true
	modelCatalog.Unlock()
	defer func() {
		modelCatalog.Lock()
		modelCatalog.syncing = false
		modelCatalog.Unlock()
	}()
	fail := func(err error) (int, error) {
		modelCatalog.Lock()
		modelCatalog.lastErr = err.Error()
		modelCatalog.Unlock()
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsDevURL, nil)
	if err != nil {
		return fail(err)
	}
	req.Header.Set("User-Agent", "AI-ENV")
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return fail(fmt.Errorf("下载 models.dev 目录失败: %v", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fail(fmt.Errorf("models.dev 返回 HTTP %d", resp.StatusCode))
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return fail(err)
	}
	models, err := parseModelsDev(raw)
	if err != nil {
		return fail(fmt.Errorf("models.dev 目录格式无法识别: %v", err))
	}
	data := modelCatalogFileData{UpdatedAt: time.Now().UnixMilli(), Models: models}
	b, err := json.Marshal(data)
	if err != nil {
		return fail(err)
	}
	p, err := storePath(modelCatalogFile)
	if err != nil {
		return fail(err)
	}
	if err := writeFileAtomic(p, b, 0o644); err != nil {
		return fail(err)
	}
	modelCatalog.Lock()
	modelCatalog.byID = map[string]CatalogModel{}
	for _, m := range models {
		modelCatalog.byID[m.ID] = m
	}
	modelCatalog.loaded, modelCatalog.updated, modelCatalog.lastErr = true, data.UpdatedAt, ""
	modelCatalog.Unlock()
	return len(models), nil
}

// startModelCatalogRefresh 启动时目录缺失或过期则后台刷新一次
func startModelCatalogRefresh(ctx context.Context) {
	modelCatalog.Lock()
	loadModelCatalogLocked()
	stale := time.Since(time.UnixMilli(modelCatalog.updated)) > modelCatalogMaxAge
	modelCatalog.Unlock()
	if !stale {
		return
	}
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(20 * time.Second):
		}
		c, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		_, _ = syncModelCatalog(c)
	}()
}

// GetModelCatalogStatus 目录条目数与更新时间
func (w *WorkbenchService) GetModelCatalogStatus() CatalogStatus {
	modelCatalog.Lock()
	defer modelCatalog.Unlock()
	loadModelCatalogLocked()
	return CatalogStatus{Count: len(modelCatalog.byID), UpdatedAt: modelCatalog.updated, Syncing: modelCatalog.syncing, Error: modelCatalog.lastErr}
}

// SyncModelCatalog 立即从 models.dev 同步目录
func (w *WorkbenchService) SyncModelCatalog() (CatalogStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if _, err := syncModelCatalog(ctx); err != nil {
		return w.GetModelCatalogStatus(), err
	}
	return w.GetModelCatalogStatus(), nil
}

// LookupModelCatalog 查询一个模型的目录信息；找不到时返回 nil
func (w *WorkbenchService) LookupModelCatalog(model string) *CatalogModel {
	if m, ok := lookupCatalogModel(model); ok {
		return &m
	}
	return nil
}

// SearchModelCatalog 按 ID 或名称搜索目录（用于模型名称候选）
func (w *WorkbenchService) SearchModelCatalog(query string, limit int) []CatalogModel {
	q := strings.ToLower(strings.TrimSpace(query))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	modelCatalog.Lock()
	loadModelCatalogLocked()
	out := []CatalogModel{}
	for _, m := range modelCatalog.byID {
		if q == "" || strings.Contains(m.ID, q) || strings.Contains(strings.ToLower(m.Name), q) {
			out = append(out, m)
		}
	}
	modelCatalog.Unlock()
	sort.Slice(out, func(i, j int) bool {
		// 前缀命中优先，其次较新的模型
		pi, pj := strings.HasPrefix(out[i].ID, q), strings.HasPrefix(out[j].ID, q)
		if pi != pj {
			return pi
		}
		if out[i].Released != out[j].Released {
			return out[i].Released > out[j].Released
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
