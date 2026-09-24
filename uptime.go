package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const uptimeStoreFile = "uptime.json"

// UptimeService 负责可用性监控与轮换组（Failover）
type UptimeService struct {
	mu  sync.Mutex
	app *App
}

func NewUptimeService(app *App) *UptimeService {
	return &UptimeService{app: app}
}

type UptimeSettings struct {
	Enabled         bool `json:"enabled"`
	IntervalSeconds int  `json:"interval_seconds"`
	TimeoutSeconds  int  `json:"timeout_seconds"`
	KeepLast        int  `json:"keep_last"`
	// ProbeMode reachability：只测 Base URL 可达；auth：带 Key 请求列模型接口，
	// 能发现 Key 失效/余额不足
	ProbeMode string `json:"probe_mode"`
}

type RotationGroup struct {
	Name             string   `json:"name"`
	Provider         string   `json:"provider"` // claude | codex | gemini | opencode | grok
	EnvNames         []string `json:"env_names"`
	Enabled          bool     `json:"enabled"`
	FailureThreshold int      `json:"failure_threshold"`
}

type UptimeCheck struct {
	At         int64  `json:"at"`
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int64  `json:"latency_ms"`
	Error      string `json:"error,omitempty"`
}

type UptimeSnapshot struct {
	Settings UptimeSettings           `json:"settings"`
	Groups   []RotationGroup          `json:"groups"`
	History  map[string][]UptimeCheck `json:"history"`
	URLs     map[string]string        `json:"urls"` // 便于前端展示当前检查 URL
	Now      int64                    `json:"now"`
	// 最近一次自动轮换的说明/失败原因，供面板提示"已自动切换/切换失败"
	LastRotation      string `json:"last_rotation,omitempty"`
	LastRotationError string `json:"last_rotation_error,omitempty"`
	LastRotationAt    int64  `json:"last_rotation_at,omitempty"`
}

type uptimeStore struct {
	Settings          UptimeSettings           `json:"settings"`
	Groups            []RotationGroup          `json:"groups"`
	History           map[string][]UptimeCheck `json:"history"`
	LastRotation      string                   `json:"last_rotation,omitempty"`
	LastRotationError string                   `json:"last_rotation_error,omitempty"`
	LastRotationAt    int64                    `json:"last_rotation_at,omitempty"`
}

func (us *UptimeService) GetSnapshot() (UptimeSnapshot, error) {
	us.mu.Lock()
	defer us.mu.Unlock()

	store, err := us.loadStore()
	if err != nil {
		return UptimeSnapshot{}, err
	}

	return us.buildSnapshot(store), nil
}

func (us *UptimeService) SaveSettings(settings UptimeSettings) error {
	us.mu.Lock()
	defer us.mu.Unlock()

	store, err := us.loadStore()
	if err != nil {
		return err
	}

	store.Settings = normalizeUptimeSettings(settings)
	if err := us.saveStore(store); err != nil {
		return err
	}
	notifyCloudSync()
	return nil
}

func (us *UptimeService) SaveRotationGroup(group RotationGroup) error {
	us.mu.Lock()
	defer us.mu.Unlock()

	store, err := us.loadStore()
	if err != nil {
		return err
	}

	if err := us.validateRotationGroup(group); err != nil {
		return err
	}

	trimmedName := strings.TrimSpace(group.Name)
	updated := false
	for i := range store.Groups {
		if strings.EqualFold(strings.TrimSpace(store.Groups[i].Name), trimmedName) {
			store.Groups[i] = normalizeRotationGroup(group)
			updated = true
			break
		}
	}
	if !updated {
		store.Groups = append(store.Groups, normalizeRotationGroup(group))
	}

	sort.SliceStable(store.Groups, func(i, j int) bool {
		return strings.ToLower(strings.TrimSpace(store.Groups[i].Name)) < strings.ToLower(strings.TrimSpace(store.Groups[j].Name))
	})

	if err := us.saveStore(store); err != nil {
		return err
	}
	notifyCloudSync()
	return nil
}

func (us *UptimeService) DeleteRotationGroup(name string) error {
	us.mu.Lock()
	defer us.mu.Unlock()

	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("轮换组名称不能为空")
	}

	store, err := us.loadStore()
	if err != nil {
		return err
	}

	next := make([]RotationGroup, 0, len(store.Groups))
	for _, g := range store.Groups {
		if strings.EqualFold(strings.TrimSpace(g.Name), trimmed) {
			continue
		}
		next = append(next, g)
	}
	store.Groups = next

	if err := us.saveStore(store); err != nil {
		return err
	}
	notifyCloudSync()
	return nil
}

// RunOnce 执行一次检查并（可选）触发轮换。
// 网络探测在锁外执行：全程持锁会让 UI 的 GetSnapshot/SaveSettings 卡住整个探测周期。
func (us *UptimeService) RunOnce() (UptimeSnapshot, error) {
	us.mu.Lock()
	store, err := us.loadStore()
	if err != nil {
		us.mu.Unlock()
		return UptimeSnapshot{}, err
	}

	if !store.Settings.Enabled {
		snap := us.buildSnapshot(store)
		us.mu.Unlock()
		return snap, nil
	}

	config := us.app.GetConfig()

	timeout := time.Duration(store.Settings.TimeoutSeconds) * time.Second
	client := &http.Client{Timeout: timeout}

	type target struct {
		url   string
		probe authProbe
		auth  bool
	}
	authMode := store.Settings.ProbeMode == uptimeProbeAuth
	targets := make(map[string]target)
	for _, env := range config.Environments {
		url := deriveEnvURL(env)
		if strings.TrimSpace(url) == "" {
			continue
		}
		t := target{url: url}
		if authMode {
			// 没有 Key 的配置（官方登录等）退回可达性检测
			t.probe, t.auth = buildAuthProbe(env)
		}
		targets[uptimeEnvKey(env.Provider, env.Name)] = t
	}
	keepLast := store.Settings.KeepLast
	us.mu.Unlock()

	// 逐个检查（避免并发导致 UI 卡顿/过多连接）
	results := make(map[string]UptimeCheck, len(targets))
	for key, t := range targets {
		if t.auth {
			results[key] = runAuthCheck(client, t.probe)
		} else {
			results[key] = runUptimeCheck(client, t.url)
		}
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	// 探测期间用户可能改过设置，重读一遍再合并，避免整包覆盖
	store, err = us.loadStore()
	if err != nil {
		return UptimeSnapshot{}, err
	}
	if store.History == nil {
		store.History = map[string][]UptimeCheck{}
	}
	for key, check := range results {
		store.History[key] = appendAndTrim(store.History[key], check, keepLast)
	}

	// 轮换：按组评估当前激活环境的连续失败次数
	config = us.app.GetConfig()
	for _, group := range store.Groups {
		group = normalizeRotationGroup(group)
		if !group.Enabled {
			continue
		}
		if len(group.EnvNames) == 0 {
			continue
		}

		activeName := currentEnvNameByProvider(config, group.Provider)
		if strings.TrimSpace(activeName) == "" {
			continue
		}
		currentIndex := indexOfString(group.EnvNames, activeName)
		if currentIndex < 0 {
			continue
		}

		history := store.History[uptimeEnvKey(group.Provider, activeName)]
		failCount := consecutiveFailures(history)
		if failCount < group.FailureThreshold {
			continue
		}

		nextName := pickNextHealthy(group.EnvNames, currentIndex, store.History, group.Provider)
		if nextName == "" || nextName == activeName {
			continue
		}

		// 只切换并写回这一组所属的平台（ApplyCurrentEnv 会把所有平台都重写一遍）；
		// 失败要留痕，用户才知道自己还挂在故障环境上
		if _, err := us.app.ApplyEnv(nextName, group.Provider); err != nil {
			store.LastRotationError = fmt.Sprintf("切换到 %s 失败: %v", nextName, err)
			store.LastRotationAt = time.Now().Unix()
			continue
		}
		reason := fmt.Sprintf("连续 %d 次失败", failCount)
		if last := history[len(history)-1]; last.Error != "" {
			reason += "（" + last.Error + "）"
		}
		store.LastRotation = fmt.Sprintf("%s：%s，已自动切换到 %s", group.Name, reason, nextName)
		store.LastRotationError = ""
		store.LastRotationAt = time.Now().Unix()

		// 更新本地 config 快照，避免多个组使用旧值
		config = us.app.GetConfig()
	}

	if err := us.saveStore(store); err != nil {
		return UptimeSnapshot{}, err
	}

	return us.buildSnapshot(store), nil
}

func (us *UptimeService) buildSnapshot(store uptimeStore) UptimeSnapshot {
	config := us.app.GetConfig()
	urls := make(map[string]string)
	for _, env := range config.Environments {
		url := deriveEnvURL(env)
		if strings.TrimSpace(url) == "" {
			continue
		}
		urls[uptimeEnvKey(env.Provider, env.Name)] = url
	}

	if store.History == nil {
		store.History = map[string][]UptimeCheck{}
	}
	if store.Groups == nil {
		store.Groups = []RotationGroup{}
	}

	return UptimeSnapshot{
		Settings:          normalizeUptimeSettings(store.Settings),
		Groups:            store.Groups,
		History:           store.History,
		URLs:              urls,
		Now:               time.Now().Unix(),
		LastRotation:      store.LastRotation,
		LastRotationError: store.LastRotationError,
		LastRotationAt:    store.LastRotationAt,
	}
}

func (us *UptimeService) storePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, uptimeStoreFile), nil
}

func (us *UptimeService) loadStore() (uptimeStore, error) {
	path, err := us.storePath()
	if err != nil {
		return uptimeStore{}, err
	}

	defaultStore := uptimeStore{
		Settings: normalizeUptimeSettings(UptimeSettings{}),
		Groups:   []RotationGroup{},
		History:  map[string][]UptimeCheck{},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultStore, nil
		}
		return uptimeStore{}, err
	}
	if len(data) == 0 {
		return defaultStore, nil
	}

	var store uptimeStore
	if err := json.Unmarshal(data, &store); err != nil {
		// 损坏时先留一份 .bak，避免下一次保存把用户的轮换组配置永远覆盖掉
		backupFile(path)
		return defaultStore, nil
	}

	store.Settings = normalizeUptimeSettings(store.Settings)
	if store.Groups == nil {
		store.Groups = []RotationGroup{}
	}
	if store.History == nil {
		store.History = map[string][]UptimeCheck{}
	}
	for k, v := range store.History {
		if len(v) > store.Settings.KeepLast {
			store.History[k] = v[len(v)-store.Settings.KeepLast:]
		} else {
			store.History[k] = v
		}
	}

	return store, nil
}

func (us *UptimeService) saveStore(store uptimeStore) error {
	path, err := us.storePath()
	if err != nil {
		return err
	}
	store.Settings = normalizeUptimeSettings(store.Settings)
	if store.Groups == nil {
		store.Groups = []RotationGroup{}
	}
	if store.History == nil {
		store.History = map[string][]UptimeCheck{}
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func normalizeUptimeSettings(settings UptimeSettings) UptimeSettings {
	out := settings
	if out.IntervalSeconds <= 0 {
		out.IntervalSeconds = 300
	}
	if out.TimeoutSeconds <= 0 {
		out.TimeoutSeconds = 8
	}
	if out.KeepLast <= 0 {
		out.KeepLast = 10
	}
	if out.KeepLast > 50 {
		out.KeepLast = 50
	}
	out.ProbeMode = normalizeUptimeProbeMode(out.ProbeMode)
	return out
}

func normalizeRotationGroup(group RotationGroup) RotationGroup {
	group.Name = strings.TrimSpace(group.Name)
	group.Provider = strings.ToLower(strings.TrimSpace(group.Provider))
	if group.Provider == "openclaw" {
		// 旧值归一到 opencode，避免轮换时误读 Claude 的当前环境
		group.Provider = "opencode"
	}
	if group.Provider == "gemini" {
		// 旧平台名，归一到 antigravity
		group.Provider = "antigravity"
	}
	group.EnvNames = normalizeStringList(group.EnvNames)
	if group.FailureThreshold <= 0 {
		group.FailureThreshold = 3
	}
	return group
}

func normalizeStringList(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func (us *UptimeService) validateRotationGroup(group RotationGroup) error {
	group = normalizeRotationGroup(group)
	if group.Name == "" {
		return fmt.Errorf("轮换组名称不能为空")
	}
	if group.Provider != "claude" && group.Provider != "claude_desktop" && group.Provider != "codex" && group.Provider != "antigravity" && group.Provider != "opencode" && group.Provider != "grok" {
		return fmt.Errorf("轮换组 provider 必须是 claude/claude_desktop/codex/antigravity/opencode/grok")
	}
	if len(group.EnvNames) == 0 {
		return fmt.Errorf("轮换组必须至少包含 1 个配置")
	}
	if group.FailureThreshold <= 0 {
		return fmt.Errorf("失败阈值必须 >= 1")
	}

	config := us.app.GetConfig()
	known := map[string]bool{}
	for _, env := range config.Environments {
		p := strings.ToLower(strings.TrimSpace(env.Provider))
		if p == "" {
			p = "claude"
		}
		known[p+"\x00"+env.Name] = true
	}

	for _, name := range group.EnvNames {
		if !known[group.Provider+"\x00"+name] {
			return fmt.Errorf("轮换组包含不存在的配置：%s", name)
		}
	}

	return nil
}

// uptimeEnvKey 检测历史的复合键：配置名可能跨服务商重复
func uptimeEnvKey(provider, name string) string {
	return strings.ToLower(strings.TrimSpace(provider)) + "/" + name
}

func deriveEnvURL(env EnvConfig) string {
	vars := env.Variables
	provider := strings.ToLower(strings.TrimSpace(env.Provider))
	if provider == "" {
		provider = "claude"
	}
	switch provider {
	case "claude", "claude_desktop":
		if v := strings.TrimSpace(vars["ANTHROPIC_BASE_URL"]); v != "" {
			return v
		}
		if v := strings.TrimSpace(vars["API_BASE_URL"]); v != "" {
			return v
		}
		return ""
	case "codex":
		return strings.TrimSpace(vars["base_url"])
	case "antigravity":
		return strings.TrimSpace(vars["GOOGLE_GEMINI_BASE_URL"])
	case "opencode":
		return strings.TrimSpace(vars["OPENCODE_BASE_URL"])
	case "grok":
		if v := strings.TrimSpace(vars["XAI_BASE_URL"]); v != "" {
			return v
		}
		return "https://api.x.ai/v1"
	default:
		return ""
	}
}

func runUptimeCheck(client *http.Client, url string) UptimeCheck {
	start := time.Now()
	check := UptimeCheck{At: start.Unix()}

	normalized, err := normalizeProbeURL(url)
	if err != nil {
		check.Success = false
		check.Error = err.Error()
		return check
	}

	resp, err := probeURL(client, http.MethodHead, normalized)
	if err != nil {
		resp, err = probeURL(client, http.MethodGet, normalized)
	}
	if err != nil {
		check.Success = false
		check.Error = err.Error()
		check.LatencyMs = time.Since(start).Milliseconds()
		return check
	}
	defer resp.Body.Close()

	check.StatusCode = resp.StatusCode
	check.LatencyMs = time.Since(start).Milliseconds()
	// 服务端错误（5xx）与限流（429）必须算失败，否则故障轮换永远不会触发；
	// 可达性检测不带 Key，401/403 说明服务本身在线，不算故障（Key 失效要靠鉴权探测发现）
	check.Success = resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests
	if !check.Success {
		check.Error = fmt.Sprintf("上游返回 %s", resp.Status)
	}
	return check
}

func appendAndTrim(history []UptimeCheck, check UptimeCheck, keep int) []UptimeCheck {
	if keep <= 0 {
		keep = 10
	}
	if check.At != 0 || check.Success || check.StatusCode != 0 || check.LatencyMs != 0 || check.Error != "" {
		history = append(history, check)
	}
	if len(history) > keep {
		history = history[len(history)-keep:]
	}
	return history
}

func consecutiveFailures(history []UptimeCheck) int {
	count := 0
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Success {
			break
		}
		count++
	}
	return count
}

func currentEnvNameByProvider(config Config, provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "claude_desktop":
		return config.CurrentEnvClaudeDesktop
	case "codex":
		return config.CurrentEnvCodex
	case "antigravity":
		return config.CurrentEnvAntigravity
	case "opencode":
		return config.CurrentEnvOpencode
	case "grok":
		return config.CurrentEnvGrok
	default:
		return config.CurrentEnvClaude
	}
}

func indexOfString(values []string, target string) int {
	target = strings.TrimSpace(target)
	for i, v := range values {
		if strings.TrimSpace(v) == target {
			return i
		}
	}
	return -1
}

func pickNextHealthy(values []string, currentIndex int, history map[string][]UptimeCheck, provider string) string {
	if len(values) == 0 {
		return ""
	}
	if currentIndex < 0 || currentIndex >= len(values) {
		return ""
	}

	// 优先挑选最近一次成功或尚未检测过的
	for offset := 1; offset <= len(values); offset++ {
		idx := (currentIndex + offset) % len(values)
		name := values[idx]
		h := history[uptimeEnvKey(provider, name)]
		if len(h) == 0 {
			return name
		}
		if h[len(h)-1].Success {
			return name
		}
	}

	// 都失败：退化为顺序轮换
	return values[(currentIndex+1)%len(values)]
}
