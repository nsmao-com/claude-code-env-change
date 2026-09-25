package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DiscoveredModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func credentialClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}
func validEndpoint(raw string) error {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("请输入不含账号密码的 HTTP(S) 地址")
	}
	return nil
}
func (w *WorkbenchService) DiscoverModels(provider, name string) ([]DiscoveredModel, error) {
	env, e := w.env(provider, name)
	if e != nil {
		return nil, e
	}
	if env.OfficialLogin {
		return nil, fmt.Errorf("官方登录环境由原应用管理模型，请选择 API 环境")
	}
	base, key, _ := upstreamVarsForEnv(&env)
	if e = validEndpoint(base); e != nil {
		return nil, e
	}
	endpoint := joinModelsURL(base, "/v1/models")
	if provider == "antigravity" && normalizeUpstreamFormat(env.UpstreamFormat) == "" {
		endpoint = joinModelsURL(base, "/v1beta/models")
	}
	req, e := http.NewRequest("GET", endpoint, nil)
	if e != nil {
		return nil, e
	}
	format := targetFormatForEnv(&env)
	if format == "anthropic" {
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else if provider == "antigravity" && normalizeUpstreamFormat(env.UpstreamFormat) == "" {
		req.Header.Set("x-goog-api-key", key)
	} else {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, e := credentialClient(20 * time.Second).Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("模型查询失败 (HTTP %d)，部分供应商不提供模型目录，可手动添加", resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if e != nil {
		return nil, e
	}
	var payload struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"display_name"`
		} `json:"data"`
		Models []struct {
			Name    string `json:"name"`
			Display string `json:"displayName"`
		} `json:"models"`
	}
	if e = json.Unmarshal(b, &payload); e != nil {
		return nil, fmt.Errorf("上游模型目录不是有效 JSON")
	}
	out := []DiscoveredModel{}
	seen := map[string]bool{}
	for _, m := range payload.Data {
		if m.ID != "" && !seen[m.ID] {
			out = append(out, DiscoveredModel{m.ID, firstNonEmpty(m.Name, m.ID)})
			seen[m.ID] = true
		}
	}
	for _, m := range payload.Models {
		id := strings.TrimPrefix(m.Name, "models/")
		if id != "" && !seen[id] {
			out = append(out, DiscoveredModel{id, firstNonEmpty(m.Display, id)})
			seen[id] = true
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (w *WorkbenchService) TestModel(provider, name, model string) RouterTestResult {
	env, e := w.env(provider, name)
	if e != nil {
		return RouterTestResult{Message: e.Error()}
	}
	if strings.TrimSpace(model) == "" {
		return RouterTestResult{Message: "请选择测试模型"}
	}
	base, key, _ := upstreamVarsForEnv(&env)
	return w.router.testUpstream(APIRoute{BaseURL: base, APIKey: key, DefaultModel: model, TargetFormat: targetFormatForEnv(&env)})
}
func (w *WorkbenchService) SaveModelProfile(p ModelProfile) error {
	if _, e := w.env(p.Provider, p.Environment); e != nil {
		return e
	}
	p.Model = strings.TrimSpace(p.Model)
	if p.Model == "" || len(p.Model) > 200 {
		return fmt.Errorf("请填写有效模型名称")
	}
	if p.Context < 0 || p.Output < 0 || p.Compact < 0 || p.Context > 100000000 || p.Output > 100000000 || p.Compact > 100000000 {
		return fmt.Errorf("Token 限制应为 0～100000000")
	}
	if p.Context > 0 && (p.Compact > p.Context || p.Output > p.Context) {
		return fmt.Errorf("输出上限和压缩阈值不能超过上下文窗口")
	}
	for _, n := range []float64{p.InputPrice, p.OutputPrice, p.CacheReadPrice, p.CacheWritePrice} {
		if n < 0 || math.IsNaN(n) || math.IsInf(n, 0) || n > 1000000 {
			return fmt.Errorf("价格应为有效的非负数")
		}
	}
	workbenchMu.Lock()
	defer workbenchMu.Unlock()
	c, e := loadWorkbench()
	if e != nil {
		return e
	}
	found := false
	for i, x := range c.Models {
		if x.Provider == p.Provider && x.Environment == p.Environment && x.Model == p.Model {
			c.Models[i] = p
			found = true
		}
	}
	if !found {
		c.Models = append(c.Models, p)
	}
	return saveWorkbench(c)
}
func (w *WorkbenchService) DeleteModelProfile(provider, name, model string) error {
	workbenchMu.Lock()
	defer workbenchMu.Unlock()
	c, e := loadWorkbench()
	if e != nil {
		return e
	}
	out := []ModelProfile{}
	for _, p := range c.Models {
		if p.Provider != provider || p.Environment != name || p.Model != model {
			out = append(out, p)
		}
	}
	c.Models = out
	return saveWorkbench(c)
}
func (w *WorkbenchService) ApplyModelProfile(provider, name, model string) error {
	c, e := w.GetWorkbench()
	if e != nil {
		return e
	}
	env, e := w.env(provider, name)
	if e != nil {
		return e
	}
	for _, p := range c.Models {
		if p.Provider != provider || p.Environment != name || p.Model != model {
			continue
		}
		setEnvironmentModel(&env, model)
		if provider == "claude" {
			if p.Output > 0 {
				env.Variables["CLAUDE_CODE_MAX_OUTPUT_TOKENS"] = strconv.Itoa(p.Output)
			}
			if p.Compact > 0 && p.Context > 0 {
				env.Variables["CLAUDE_AUTOCOMPACT_PCT_OVERRIDE"] = strconv.Itoa(p.Compact * 100 / p.Context)
			}
		}
		if provider == "antigravity" && p.Compact > 0 && p.Context > 0 {
			env.Variables["GEMINI_COMPRESSION_THRESHOLD"] = strconv.FormatFloat(float64(p.Compact)/float64(p.Context), 'f', 4, 64)
		}
		if provider == "opencode" {
			env.Variables["AI_ENV_MODEL_CONTEXT"] = strconv.Itoa(p.Context)
			env.Variables["AI_ENV_MODEL_OUTPUT"] = strconv.Itoa(p.Output)
		}
		if provider == "codex" {
			env.Variables["model_context_window"] = strconv.Itoa(p.Context)
			env.Variables["model_max_output_tokens"] = strconv.Itoa(p.Output)
			env.Variables["model_auto_compact_token_limit"] = strconv.Itoa(p.Compact)
			for _, key := range []string{"model_context_window", "model_max_output_tokens", "model_auto_compact_token_limit"} {
				if env.Variables[key] == "0" {
					delete(env.Variables, key)
				}
			}
		}
		if provider == "grok" {
			env.Variables["XAI_CONTEXT_WINDOW"] = strconv.Itoa(p.Context)
			env.Variables["XAI_MAX_TOKENS"] = strconv.Itoa(p.Output)
		}
		if provider == "codex" {
			catalog, err := writeCodexCatalog(env, c.Models)
			if err != nil {
				return err
			}
			env.Variables["model_catalog_json"] = catalog
		}
		if e = w.app.UpdateEnv(name, provider, env); e != nil {
			return e
		}
		_, e = w.app.ApplyEnv(name, provider)
		return e
	}
	return fmt.Errorf("请先保存模型档案")
}

type DiagnosticItem struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Action  string `json:"action"`
}

func (w *WorkbenchService) RunDiagnostics(provider, name string, network bool) ([]DiagnosticItem, error) {
	items := []DiagnosticItem{}
	env, e := w.env(provider, name)
	if e != nil {
		return items, e
	}
	home, e := os.UserHomeDir()
	if e != nil {
		return nil, e
	}
	items = append(items, DiagnosticItem{"配置目录", "ok", home, ""})
	for _, d := range w.app.ListConfigDirs() {
		if d.ID == provider {
			status := "ok"
			if !d.Exists {
				status = "warning"
			}
			items = append(items, DiagnosticItem{"CLI 配置", status, d.Dir, "可在设置 → 配置目录中打开"})
			for _, f := range d.Files {
				if !f.Exists {
					continue
				}
				if filepath.Ext(f.Path) == ".json" {
					b, err := os.ReadFile(f.Path)
					if err == nil && !json.Valid(b) {
						items = append(items, DiagnosticItem{f.Name, "error", "JSON 格式无效", "从配置历史恢复，或修复原文件后重试"})
					}
				}
			}
		}
	}
	for _, spec := range cliCatalog() {
		if spec.ID == provider {
			copies := listCommandCopies(spec.Command)
			status := "ok"
			msg := "已找到 " + spec.Command
			if len(copies) == 0 {
				status = "error"
				msg = "未找到 CLI"
			}
			if len(copies) > 1 {
				status = "warning"
				msg = "检测到多个安装路径: " + strings.Join(copies, "、")
			}
			items = append(items, DiagnosticItem{"CLI 安装", status, msg, "在设置 → CLI 中检查安装"})
			break
		}
	}
	for _, p := range w.app.GetConfigDrift() {
		label := map[string]string{"claude": "Claude Code", "claude_desktop": "Claude Desktop", "codex": "Codex", "antigravity": "Antigravity", "opencode": "OpenCode", "grok": "Grok"}[provider]
		if sameProvider(provider, p) || p == label {
			items = append(items, DiagnosticItem{"配置一致性", "warning", "本机配置与当前环境不同", "确认来源后重新应用环境"})
		}
	}
	proxy := loadOutboundProxy()
	msg := "使用系统默认网络"
	if proxy.Enabled {
		msg = "已启用应用出站代理"
	}
	items = append(items, DiagnosticItem{"代理", "ok", msg, ""})
	base, key, model := upstreamVarsForEnv(&env)
	if !env.OfficialLogin {
		if e = validEndpoint(base); e != nil {
			items = append(items, DiagnosticItem{"API 地址", "error", e.Error(), "编辑环境中的 Base URL"})
		}
		if key == "" {
			items = append(items, DiagnosticItem{"API 凭证", "warning", "尚未配置 API Key", "在环境编辑页填写凭证"})
		}
		if network {
			models, err := w.DiscoverModels(provider, name)
			if err != nil {
				items = append(items, DiagnosticItem{"模型目录", "warning", err.Error(), "模型目录不受支持时仍可单独测试指定模型"})
			} else {
				items = append(items, DiagnosticItem{"模型目录", "ok", fmt.Sprintf("发现 %d 个模型", len(models)), ""})
			}
			if model != "" {
				r := w.TestModel(provider, name, model)
				status := "ok"
				if !r.Success {
					status = "error"
				}
				items = append(items, DiagnosticItem{"模型请求", status, r.Message, "检查模型名称、协议及供应商权限"})
			}
		}
	}
	if provider == "codex" {
		items = append(items, DiagnosticItem{"认证模式", "ok", firstNonEmpty(env.Variables["AI_ENV_AUTH_MODE"], "兼容现有 API 模式"), "在模型中心选择 API / 保留官方登录模式"})
	}
	if network {
		servers, err := w.mcp.ListServers()
		if err != nil {
			return items, err
		}
		platform, _ := normalizePlatform(provider)
		for _, s := range servers {
			if !platformContains(s.EnablePlatform, platform) {
				continue
			}
			r := w.mcp.TestServer(s)
			status := "ok"
			if !r.Success {
				status = "error"
			}
			items = append(items, DiagnosticItem{"MCP · " + s.Name, status, r.Message, "在 MCP 页检查命令、环境变量和请求头"})
		}
	}
	return items, nil
}
