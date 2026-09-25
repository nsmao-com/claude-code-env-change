package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ModelProfile struct {
	Provider        string  `json:"provider"`
	Environment     string  `json:"environment"`
	Model           string  `json:"model"`
	Context         int     `json:"context"`
	Output          int     `json:"output"`
	Compact         int     `json:"compact"`
	InputPrice      float64 `json:"input_price"`
	OutputPrice     float64 `json:"output_price"`
	CacheReadPrice  float64 `json:"cache_read_price"`
	CacheWritePrice float64 `json:"cache_write_price"`
}
type PromptPreset struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}
type ProjectPreset struct {
	Name        string   `json:"name"`
	Directory   string   `json:"directory"`
	Provider    string   `json:"provider"`
	Environment string   `json:"environment"`
	Model       string   `json:"model"`
	MCP         []string `json:"mcp"`
	Skills      []string `json:"skills"`
	Prompt      string   `json:"prompt"`
}
type CostSettings struct {
	DailyBudget    float64            `json:"daily_budget"`
	MonthlyBudget  float64            `json:"monthly_budget"`
	Multipliers    map[string]float64 `json:"multipliers"`
	BalanceSources []BalanceSource    `json:"balance_sources"`
}
type BalanceSource struct {
	Provider    string `json:"provider"`
	Environment string `json:"environment"`
	Adapter     string `json:"adapter"`
}
type WorkbenchConfig struct {
	Models           []ModelProfile  `json:"models"`
	Prompts          []PromptPreset  `json:"prompts"`
	Projects         []ProjectPreset `json:"projects"`
	Costs            CostSettings    `json:"costs"`
	ArchivedSessions []string        `json:"archived_sessions"`
}

var workbenchMu sync.Mutex

type WorkbenchService struct {
	app    *App
	mcp    *MCPService
	skills *SkillService
	router *RouterService
}

func NewWorkbenchService(a *App, m *MCPService, s *SkillService, r *RouterService) *WorkbenchService {
	return &WorkbenchService{a, m, s, r}
}
func loadWorkbench() (WorkbenchConfig, error) {
	c := WorkbenchConfig{Models: []ModelProfile{}, Prompts: []PromptPreset{}, Projects: []ProjectPreset{}, ArchivedSessions: []string{}, Costs: CostSettings{Multipliers: map[string]float64{}, BalanceSources: []BalanceSource{}}}
	p, e := storePath("workbench.json")
	if e != nil {
		return c, e
	}
	b, e := os.ReadFile(p)
	if os.IsNotExist(e) {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	if c.Costs.Multipliers == nil {
		c.Costs.Multipliers = map[string]float64{}
	}
	return c, e
}
func saveWorkbench(c WorkbenchConfig) error {
	p, e := storePath("workbench.json")
	if e != nil {
		return e
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	if e = writeFileAtomic(p, b, 0600); e == nil {
		notifyCloudSync()
	}
	return e
}
func (w *WorkbenchService) GetWorkbench() (WorkbenchConfig, error) {
	workbenchMu.Lock()
	defer workbenchMu.Unlock()
	return loadWorkbench()
}
func (w *WorkbenchService) env(provider, name string) (EnvConfig, error) {
	w.app.configMu.Lock()
	defer w.app.configMu.Unlock()
	e := w.app.findEnvIn(provider, name)
	if e == nil {
		return EnvConfig{}, fmt.Errorf("请选择仍然存在的环境配置")
	}
	return *cloneEnvConfig(e), nil
}
func (w *WorkbenchService) PickProjectDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(w.app.ctx, runtime.OpenDialogOptions{Title: "选择项目目录"})
}
func (w *WorkbenchService) PickImportFile(kind string) (string, error) {
	pattern := "*.json;*.txt"
	if kind == "zip" {
		pattern = "*.zip"
	}
	return runtime.OpenFileDialog(w.app.ctx, runtime.OpenDialogOptions{Title: "选择导入文件", Filters: []runtime.FileFilter{{DisplayName: "配置 / 技能文件", Pattern: pattern}}})
}
func (w *WorkbenchService) SavePromptPreset(p PromptPreset) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len(p.Name) > 100 || strings.TrimSpace(p.Content) == "" || len(p.Content) > 1<<20 {
		return fmt.Errorf("请填写名称和不超过 1 MB 的提示词")
	}
	workbenchMu.Lock()
	defer workbenchMu.Unlock()
	c, e := loadWorkbench()
	if e != nil {
		return e
	}
	found := false
	for i := range c.Prompts {
		if c.Prompts[i].Name == p.Name {
			c.Prompts[i] = p
			found = true
		}
	}
	if !found {
		c.Prompts = append(c.Prompts, p)
	}
	return saveWorkbench(c)
}
func (w *WorkbenchService) DeletePromptPreset(name string) error {
	workbenchMu.Lock()
	defer workbenchMu.Unlock()
	c, e := loadWorkbench()
	if e != nil {
		return e
	}
	for _, p := range c.Projects {
		if p.Prompt == name {
			return fmt.Errorf("项目套装 %s 正在使用此提示词", p.Name)
		}
	}
	out := []PromptPreset{}
	for _, p := range c.Prompts {
		if p.Name != name {
			out = append(out, p)
		}
	}
	c.Prompts = out
	return saveWorkbench(c)
}
func (w *WorkbenchService) ApplyPromptPreset(name, provider string) error {
	c, e := w.GetWorkbench()
	if e != nil {
		return e
	}
	for _, p := range c.Prompts {
		if p.Name == name {
			return w.app.SavePromptFile(provider, p.Content)
		}
	}
	return fmt.Errorf("提示词不存在")
}
func (w *WorkbenchService) SaveProjectPreset(p ProjectPreset) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len(p.Name) > 100 {
		return fmt.Errorf("请填写项目套装名称")
	}
	if !filepath.IsAbs(p.Directory) || !dirExists(p.Directory) {
		return fmt.Errorf("请选择存在的项目目录")
	}
	if _, e := w.env(p.Provider, p.Environment); e != nil {
		return e
	}
	if e := w.validateProjectTools(p); e != nil {
		return e
	}
	workbenchMu.Lock()
	defer workbenchMu.Unlock()
	c, e := loadWorkbench()
	if e != nil {
		return e
	}
	if p.Prompt != "" {
		found := false
		for _, x := range c.Prompts {
			if x.Name == p.Prompt {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("提示词模板不存在")
		}
	}
	found := false
	for i := range c.Projects {
		if c.Projects[i].Name == p.Name {
			c.Projects[i] = p
			found = true
		}
	}
	if !found {
		c.Projects = append(c.Projects, p)
	}
	return saveWorkbench(c)
}
func (w *WorkbenchService) validateProjectTools(p ProjectPreset) error {
	servers, e := w.mcp.ListServers()
	if e != nil {
		return e
	}
	known := map[string]bool{}
	for _, s := range servers {
		known[s.Name] = true
	}
	for _, name := range p.MCP {
		if !known[name] {
			return fmt.Errorf("MCP %s 已不存在", name)
		}
	}
	skills, e := w.skills.ListSkills()
	if e != nil {
		return e
	}
	known = map[string]bool{}
	for _, s := range skills {
		known[s.Name] = true
	}
	for _, name := range p.Skills {
		if !known[name] {
			return fmt.Errorf("技能 %s 已不存在", name)
		}
	}
	return nil
}
func (w *WorkbenchService) DeleteProjectPreset(name string) error {
	workbenchMu.Lock()
	defer workbenchMu.Unlock()
	c, e := loadWorkbench()
	if e != nil {
		return e
	}
	out := []ProjectPreset{}
	for _, p := range c.Projects {
		if p.Name != name {
			out = append(out, p)
		}
	}
	c.Projects = out
	return saveWorkbench(c)
}
func (w *WorkbenchService) ApplyProjectPreset(name string) (string, error) {
	c, e := w.GetWorkbench()
	if e != nil {
		return "", e
	}
	var p *ProjectPreset
	for i := range c.Projects {
		if c.Projects[i].Name == name {
			p = &c.Projects[i]
		}
	}
	if p == nil {
		return "", fmt.Errorf("项目套装不存在")
	}
	if e = w.validateProjectTools(*p); e != nil {
		return "", e
	}
	if !dirExists(p.Directory) {
		return "", fmt.Errorf("项目目录已不存在")
	}
	if _, e = w.snapshot("应用项目套装前 · " + name); e != nil {
		return "", e
	}
	env, e := w.env(p.Provider, p.Environment)
	if e != nil {
		return "", e
	}
	if p.Model != "" {
		setEnvironmentModel(&env, p.Model)
		if e = w.app.UpdateEnv(env.Name, env.Provider, env); e != nil {
			return "", e
		}
	}
	message, e := w.app.ApplyEnv(p.Environment, p.Provider)
	if e != nil {
		return "", e
	}
	platform, ok := normalizePlatform(p.Provider)
	if !ok {
		return "", fmt.Errorf("平台不支持工具配置")
	}
	servers, e := w.mcp.ListServers()
	if e != nil {
		return "", e
	}
	selected := map[string]bool{}
	for _, n := range p.MCP {
		selected[n] = true
	}
	for i := range servers {
		servers[i].EnablePlatform = withPlatformSelection(servers[i].EnablePlatform, platform, selected[servers[i].Name])
	}
	if e = w.mcp.SaveServers(servers); e != nil {
		return "", fmt.Errorf("环境已应用，MCP 应用失败，可从配置历史恢复: %w", e)
	}
	skills, e := w.skills.ListSkills()
	if e != nil {
		return "", e
	}
	selected = map[string]bool{}
	for _, n := range p.Skills {
		selected[n] = true
	}
	for _, s := range skills {
		s.EnablePlatform = withPlatformSelection(s.EnablePlatform, platform, selected[s.Name])
		if e = w.skills.SaveSkill(s); e != nil {
			return "", fmt.Errorf("部分套装已应用，技能 %s 失败，可从历史恢复: %w", s.Name, e)
		}
	}
	if p.Prompt != "" {
		if e = w.ApplyPromptPreset(p.Prompt, p.Provider); e != nil {
			return "", e
		}
	}
	return message + "；已应用工具与提示词（影响该工具的当前全局配置）", nil
}
func withPlatformSelection(items []string, platform string, on bool) []string {
	out := []string{}
	for _, x := range items {
		if x != platform {
			out = append(out, x)
		}
	}
	if on {
		out = append(out, platform)
	}
	return out
}
func setEnvironmentModel(e *EnvConfig, model string) {
	if e.Variables == nil {
		e.Variables = map[string]string{}
	}
	key := map[string]string{"claude": "ANTHROPIC_MODEL", "claude_desktop": "ANTHROPIC_MODEL", "codex": "model", "antigravity": "GEMINI_MODEL", "opencode": "OPENCODE_MODEL", "grok": "XAI_MODEL"}[e.Provider]
	if key != "" {
		e.Variables[key] = model
	}
}
