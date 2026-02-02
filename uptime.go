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
}

type RotationGroup struct {
	Name             string   `json:"name"`
	Provider         string   `json:"provider"` // claude | codex | gemini
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
	Settings UptimeSettings            `json:"settings"`
	Groups   []RotationGroup           `json:"groups"`
	History  map[string][]UptimeCheck  `json:"history"`
	URLs     map[string]string         `json:"urls"` // 便于前端展示当前检查 URL
	Now      int64                     `json:"now"`
}

type uptimeStore struct {
	Settings UptimeSettings           `json:"settings"`
	Groups   []RotationGroup          `json:"groups"`
	History  map[string][]UptimeCheck `json:"history"`
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
	return us.saveStore(store)
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

	return us.saveStore(store)
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

	return us.saveStore(store)
}

// RunOnce 执行一次检查并（可选）触发轮换
func (us *UptimeService) RunOnce() (UptimeSnapshot, error) {
	us.mu.Lock()
	defer us.mu.Unlock()

	store, err := us.loadStore()
	if err != nil {
		return UptimeSnapshot{}, err
	}

	if !store.Settings.Enabled {
		return us.buildSnapshot(store), nil
	}

	config := us.app.GetConfig()

	timeout := time.Duration(store.Settings.TimeoutSeconds) * time.Second
	client := &http.Client{Timeout: timeout}

	urls := make(map[string]string)
	for _, env := range config.Environments {
		url := deriveEnvURL(env)
		if strings.TrimSpace(url) == "" {
			continue
		}
		urls[env.Name] = url
	}

	if store.History == nil {
		store.History = map[string][]UptimeCheck{}
	}

	// 逐个检查（避免并发导致 UI 卡顿/过多连接）
	for name, url := range urls {
		check := runUptimeCheck(client, url)
		store.History[name] = appendAndTrim(store.History[name], check, store.Settings.KeepLast)
	}

	// 轮换：按组评估当前激活环境的连续失败次数
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

		history := store.History[activeName]
		failCount := consecutiveFailures(history)
		if failCount < group.FailureThreshold {
			continue
		}

		nextName := pickNextHealthy(group.EnvNames, currentIndex, store.History)
		if nextName == "" || nextName == activeName {
			continue
		}

		// 切换并应用（沿用现有逻辑：SwitchToEnv + ApplyCurrentEnv）
		_ = us.app.SwitchToEnv(nextName)
		_, _ = us.app.ApplyCurrentEnv()

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
		urls[env.Name] = url
	}

	if store.History == nil {
		store.History = map[string][]UptimeCheck{}
	}
	if store.Groups == nil {
		store.Groups = []RotationGroup{}
	}

	return UptimeSnapshot{
		Settings: normalizeUptimeSettings(store.Settings),
		Groups:   store.Groups,
		History:  store.History,
		URLs:     urls,
		Now:      time.Now().Unix(),
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
	return os.WriteFile(path, data, 0o644)
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
	return out
}

func normalizeRotationGroup(group RotationGroup) RotationGroup {
	group.Name = strings.TrimSpace(group.Name)
	group.Provider = strings.ToLower(strings.TrimSpace(group.Provider))
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
	if group.Provider != "claude" && group.Provider != "codex" && group.Provider != "gemini" {
		return fmt.Errorf("轮换组 provider 必须是 claude/codex/gemini")
	}
	if len(group.EnvNames) == 0 {
		return fmt.Errorf("轮换组必须至少包含 1 个配置")
	}
	if group.FailureThreshold <= 0 {
		return fmt.Errorf("失败阈值必须 >= 1")
	}

	config := us.app.GetConfig()
	envProvider := map[string]string{}
	for _, env := range config.Environments {
		envProvider[env.Name] = strings.ToLower(strings.TrimSpace(env.Provider))
	}

	for _, name := range group.EnvNames {
		p, ok := envProvider[name]
		if !ok {
			return fmt.Errorf("轮换组包含不存在的配置：%s", name)
		}
		if p == "" {
			p = "claude"
		}
		if p != group.Provider {
			return fmt.Errorf("轮换组配置 provider 不一致：%s 不是 %s", name, group.Provider)
		}
	}

	return nil
}

func deriveEnvURL(env EnvConfig) string {
	vars := env.Variables
	provider := strings.ToLower(strings.TrimSpace(env.Provider))
	if provider == "" {
		provider = "claude"
	}
	switch provider {
	case "claude":
		if v := strings.TrimSpace(vars["ANTHROPIC_BASE_URL"]); v != "" {
			return v
		}
		if v := strings.TrimSpace(vars["API_BASE_URL"]); v != "" {
			return v
		}
		return ""
	case "codex":
		return strings.TrimSpace(vars["base_url"])
	case "gemini":
		return strings.TrimSpace(vars["GOOGLE_GEMINI_BASE_URL"])
	default:
		return ""
	}
}

func runUptimeCheck(client *http.Client, url string) UptimeCheck {
	start := time.Now()
	check := UptimeCheck{At: start.Unix()}

	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		check.Success = false
		check.Error = err.Error()
		return check
	}

	resp, err := client.Do(req)
	if err != nil {
		// fallback GET
		req, reqErr := http.NewRequest(http.MethodGet, url, nil)
		if reqErr != nil {
			check.Success = false
			check.Error = err.Error()
			return check
		}
		resp, err = client.Do(req)
	}
	if err != nil {
		check.Success = false
		check.Error = err.Error()
		check.LatencyMs = time.Since(start).Milliseconds()
		return check
	}
	defer resp.Body.Close()

	check.Success = true
	check.StatusCode = resp.StatusCode
	check.LatencyMs = time.Since(start).Milliseconds()
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
	case "codex":
		return config.CurrentEnvCodex
	case "gemini":
		return config.CurrentEnvGemini
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

func pickNextHealthy(values []string, currentIndex int, history map[string][]UptimeCheck) string {
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
		h := history[name]
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
