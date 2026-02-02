package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// LogService 日志统计服务
type LogService struct{}

// NewLogService 创建日志服务
func NewLogService() *LogService {
	return &LogService{}
}

// GetLogDirectory 获取日志目录路径 (供前端调试)
func (ls *LogService) GetLogDirectory() string {
	return ls.getClaudeProjectsDir()
}

// UsageRecord 单条使用记录
type UsageRecord struct {
	Timestamp       string  `json:"timestamp"`
	Model           string  `json:"model"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	CacheReadTokens int     `json:"cache_read_tokens"`
	CacheWriteTokens int    `json:"cache_write_tokens"`
	TotalCost       float64 `json:"total_cost"`
	SessionID       string  `json:"session_id"`
	ProjectPath     string  `json:"project_path"`
}

// ModelStats 模型统计
type ModelStats struct {
	Requests int     `json:"requests"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
}

// UsageStats 使用统计
type UsageStats struct {
	TotalRequests     int                   `json:"total_requests"`
	TotalInputTokens  int64                 `json:"total_input_tokens"`
	TotalOutputTokens int64                 `json:"total_output_tokens"`
	TotalCacheRead    int64                 `json:"total_cache_read"`
	TotalCacheWrite   int64                 `json:"total_cache_write"`
	TotalCost         float64               `json:"total_cost"`
	ByModel           map[string]ModelStats `json:"by_model"`
	Series            []HourlyStat          `json:"series"`
}

// HourlyStat 小时统计
type HourlyStat struct {
	Hour         string  `json:"hour"`
	Requests     int     `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	Cost         float64 `json:"cost"`
}

// HeatmapData 热力图数据
type HeatmapData struct {
	Date     string  `json:"date"`
	Requests int     `json:"requests"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
}

// EnvUsageSummary 单个配置的用量汇总（按“配置切换时间线”近似归因）
type EnvUsageSummary struct {
	Provider        string  `json:"provider"`
	Requests        int     `json:"requests"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	CacheReadTokens int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
	TotalCost       float64 `json:"total_cost"`
	LastTimestamp   string  `json:"last_timestamp,omitempty"`
}

// Claude Code 日志条目结构
type claudeLogEntry struct {
	Type      string        `json:"type"`
	Timestamp string        `json:"timestamp"`
	Message   *claudeMessage `json:"message"`
}

type claudeMessage struct {
	Model string       `json:"model"`
	Usage *claudeUsage `json:"usage"`
}

type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	CacheRead    int `json:"cache_read_input_tokens"`
	CacheCreate  int `json:"cache_creation_input_tokens"`
}

// Model pricing (USD per 1M tokens)
// Cache pricing: CacheCreate = 1.25 × Input, CacheRead = 0.1 × Input
// Reference: https://docs.anthropic.com/en/docs/about-claude/models
var modelPricing = map[string]struct {
	Input       float64
	Output      float64
	CacheCreate float64
	CacheRead   float64
}{
	// Claude Opus 4.5 ($5/$25)
	"claude-opus-4-5-20251101":     {Input: 5.0, Output: 25.0, CacheCreate: 6.25, CacheRead: 0.50},
	// Claude Opus 4 / 4.1 ($15/$75)
	"claude-opus-4-20250514":       {Input: 15.0, Output: 75.0, CacheCreate: 18.75, CacheRead: 1.50},
	"claude-opus-4-1-20250805":     {Input: 15.0, Output: 75.0, CacheCreate: 18.75, CacheRead: 1.50},
	// Claude 3 Opus ($15/$75)
	"claude-3-opus-20240229":       {Input: 15.0, Output: 75.0, CacheCreate: 18.75, CacheRead: 1.50},
	// Claude Sonnet 4 / 4.5 / 3.7 / 3.5 ($3/$15)
	"claude-sonnet-4-20250514":     {Input: 3.0, Output: 15.0, CacheCreate: 3.75, CacheRead: 0.30},
	"claude-sonnet-4-5-20250929":   {Input: 3.0, Output: 15.0, CacheCreate: 3.75, CacheRead: 0.30},
	"claude-3-7-sonnet-20250219":   {Input: 3.0, Output: 15.0, CacheCreate: 3.75, CacheRead: 0.30},
	"claude-3-5-sonnet-20241022":   {Input: 3.0, Output: 15.0, CacheCreate: 3.75, CacheRead: 0.30},
	"claude-3-5-sonnet-20240620":   {Input: 3.0, Output: 15.0, CacheCreate: 3.75, CacheRead: 0.30},
	// Claude 3.5 Haiku ($0.80/$4)
	"claude-3-5-haiku-20241022":    {Input: 0.80, Output: 4.0, CacheCreate: 1.0, CacheRead: 0.08},
	// Claude 3 Haiku ($0.25/$1.25)
	"claude-3-haiku-20240307":      {Input: 0.25, Output: 1.25, CacheCreate: 0.3125, CacheRead: 0.025},
	// GPT-4 series
	"gpt-4":                        {Input: 30.0, Output: 60.0, CacheCreate: 0, CacheRead: 0},
	"gpt-4-turbo":                  {Input: 10.0, Output: 30.0, CacheCreate: 0, CacheRead: 0},
	"gpt-4o":                       {Input: 2.5, Output: 10.0, CacheCreate: 0, CacheRead: 0},
	"gpt-4o-mini":                  {Input: 0.15, Output: 0.6, CacheCreate: 0, CacheRead: 0},
	// Gemini series (https://ai.google.dev/gemini-api/docs/pricing)
	"gemini-2.5-pro":               {Input: 1.25, Output: 10.0, CacheCreate: 0.3125, CacheRead: 0},
	"gemini-2.5-flash":             {Input: 0.15, Output: 0.60, CacheCreate: 0.0375, CacheRead: 0},
	"gemini-2.0-flash":             {Input: 0.10, Output: 0.40, CacheCreate: 0.025, CacheRead: 0},
	"gemini-1.5-pro":               {Input: 1.25, Output: 5.0, CacheCreate: 0.3125, CacheRead: 0},
	"gemini-1.5-flash":             {Input: 0.075, Output: 0.3, CacheCreate: 0.01875, CacheRead: 0},
	"gemini-3-pro":                 {Input: 2.5, Output: 15.0, CacheCreate: 0.625, CacheRead: 0},
	// OpenAI Codex series (https://developers.openai.com/codex/pricing/)
	"gpt-5.2-codex":                {Input: 1.75, Output: 14.0, CacheCreate: 0, CacheRead: 0.175},
	"gpt-5.2":                      {Input: 1.75, Output: 14.0, CacheCreate: 0, CacheRead: 0.175},
	"gpt-5.1-codex-mini":           {Input: 0.30, Output: 1.20, CacheCreate: 0, CacheRead: 0.03},
	"gpt-5.1-codex-max":            {Input: 1.50, Output: 12.0, CacheCreate: 0, CacheRead: 0.15},
	"gpt-5.1-codex":                {Input: 1.50, Output: 12.0, CacheCreate: 0, CacheRead: 0.15},
	"gpt-5.1":                      {Input: 1.50, Output: 12.0, CacheCreate: 0, CacheRead: 0.15},
	"gpt-5-codex":                  {Input: 1.25, Output: 10.0, CacheCreate: 0, CacheRead: 0.125},
	"gpt-5":                        {Input: 1.25, Output: 10.0, CacheCreate: 0, CacheRead: 0.125},
	"codex-1":                      {Input: 1.25, Output: 10.0, CacheCreate: 0, CacheRead: 0.125},
}

// GetUsageStats 获取使用统计 (最近N天, 按平台筛选)
func (ls *LogService) GetUsageStats(days int, platform string) (UsageStats, error) {
	if days <= 0 {
		days = 7
	}

	stats := UsageStats{
		ByModel: make(map[string]ModelStats),
		Series:  make([]HourlyStat, 0),
	}

	var records []UsageRecord

	// 根据平台筛选
	switch platform {
	case "claude":
		claudeRecords, err := ls.readClaudeLogs(days)
		if err != nil {
			claudeRecords = []UsageRecord{}
		}
		records = claudeRecords
	case "gemini":
		geminiRecords, err := ls.readGeminiLogs(days)
		if err != nil {
			geminiRecords = []UsageRecord{}
		}
		records = geminiRecords
	case "codex":
		codexRecords, err := ls.readCodexLogs(days)
		if err != nil {
			codexRecords = []UsageRecord{}
		}
		records = codexRecords
	default: // "all" 或其他
		claudeRecords, err := ls.readClaudeLogs(days)
		if err != nil {
			claudeRecords = []UsageRecord{}
		}
		geminiRecords, err := ls.readGeminiLogs(days)
		if err != nil {
			geminiRecords = []UsageRecord{}
		}
		codexRecords, err := ls.readCodexLogs(days)
		if err != nil {
			codexRecords = []UsageRecord{}
		}
		records = append(claudeRecords, geminiRecords...)
		records = append(records, codexRecords...)
	}

	if len(records) == 0 {
		return stats, nil
	}

	// 按小时聚合
	hourlyMap := make(map[string]*HourlyStat)

	for _, record := range records {
		stats.TotalRequests++
		stats.TotalInputTokens += int64(record.InputTokens)
		stats.TotalOutputTokens += int64(record.OutputTokens)
		stats.TotalCacheRead += int64(record.CacheReadTokens)
		stats.TotalCacheWrite += int64(record.CacheWriteTokens)
		stats.TotalCost += record.TotalCost

		// 按模型聚合
		modelStat := stats.ByModel[record.Model]
		modelStat.Requests++
		modelStat.Tokens += int64(record.InputTokens + record.OutputTokens)
		modelStat.Cost += record.TotalCost
		stats.ByModel[record.Model] = modelStat

		// 按小时聚合
		if len(record.Timestamp) >= 13 {
			hour := record.Timestamp[:13] // "2025-01-15 14"
			if hourlyMap[hour] == nil {
				hourlyMap[hour] = &HourlyStat{Hour: hour}
			}
			hourlyMap[hour].Requests++
			hourlyMap[hour].InputTokens += int64(record.InputTokens)
			hourlyMap[hour].OutputTokens += int64(record.OutputTokens)
			hourlyMap[hour].Cost += record.TotalCost
		}
	}

	// 转换为有序列表
	hours := make([]string, 0, len(hourlyMap))
	for h := range hourlyMap {
		hours = append(hours, h)
	}
	sort.Strings(hours)

	for _, h := range hours {
		stats.Series = append(stats.Series, *hourlyMap[h])
	}

	return stats, nil
}

// GetHeatmapData 获取热力图数据 (最近N天, 按平台筛选)
func (ls *LogService) GetHeatmapData(days int, platform string) ([]HeatmapData, error) {
	if days <= 0 {
		days = 30
	}

	var records []UsageRecord

	// 根据平台筛选
	switch platform {
	case "claude":
		claudeRecords, err := ls.readClaudeLogs(days)
		if err != nil {
			claudeRecords = []UsageRecord{}
		}
		records = claudeRecords
	case "gemini":
		geminiRecords, err := ls.readGeminiLogs(days)
		if err != nil {
			geminiRecords = []UsageRecord{}
		}
		records = geminiRecords
	case "codex":
		codexRecords, err := ls.readCodexLogs(days)
		if err != nil {
			codexRecords = []UsageRecord{}
		}
		records = codexRecords
	default: // "all" 或其他
		claudeRecords, err := ls.readClaudeLogs(days)
		if err != nil {
			claudeRecords = []UsageRecord{}
		}
		geminiRecords, err := ls.readGeminiLogs(days)
		if err != nil {
			geminiRecords = []UsageRecord{}
		}
		codexRecords, err := ls.readCodexLogs(days)
		if err != nil {
			codexRecords = []UsageRecord{}
		}
		records = append(claudeRecords, geminiRecords...)
		records = append(records, codexRecords...)
	}

	if len(records) == 0 {
		return []HeatmapData{}, nil
	}

	// 按日期聚合
	dailyMap := make(map[string]*HeatmapData)

	for _, record := range records {
		if len(record.Timestamp) >= 10 {
			date := record.Timestamp[:10] // "2025-01-15"
			if dailyMap[date] == nil {
				dailyMap[date] = &HeatmapData{Date: date}
			}
			dailyMap[date].Requests++
			dailyMap[date].Tokens += int64(record.InputTokens + record.OutputTokens)
			dailyMap[date].Cost += record.TotalCost
		}
	}

	// 转换为有序列表
	dates := make([]string, 0, len(dailyMap))
	for d := range dailyMap {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	result := make([]HeatmapData, 0, len(dates))
	for _, d := range dates {
		result = append(result, *dailyMap[d])
	}

	return result, nil
}

// GetRecentLogs 获取最近的日志记录 (按平台筛选)
func (ls *LogService) GetRecentLogs(limit int, platform string) ([]UsageRecord, error) {
	if limit <= 0 {
		limit = 50
	}

	var records []UsageRecord

	// 根据平台筛选
	switch platform {
	case "claude":
		claudeRecords, err := ls.readClaudeLogs(7)
		if err != nil {
			claudeRecords = []UsageRecord{}
		}
		records = claudeRecords
	case "gemini":
		geminiRecords, err := ls.readGeminiLogs(7)
		if err != nil {
			geminiRecords = []UsageRecord{}
		}
		records = geminiRecords
	case "codex":
		codexRecords, err := ls.readCodexLogs(7)
		if err != nil {
			codexRecords = []UsageRecord{}
		}
		records = codexRecords
	default: // "all" 或其他
		claudeRecords, err := ls.readClaudeLogs(7)
		if err != nil {
			claudeRecords = []UsageRecord{}
		}
		geminiRecords, err := ls.readGeminiLogs(7)
		if err != nil {
			geminiRecords = []UsageRecord{}
		}
		codexRecords, err := ls.readCodexLogs(7)
		if err != nil {
			codexRecords = []UsageRecord{}
		}
		records = append(claudeRecords, geminiRecords...)
		records = append(records, codexRecords...)
	}

	if len(records) == 0 {
		return []UsageRecord{}, nil
	}

	// 按时间倒序
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp > records[j].Timestamp
	})

	if len(records) > limit {
		records = records[:limit]
	}

	return records, nil
}

// GetEnvUsageSummary 获取按“配置”聚合的用量统计（最近 N 天）
//
// 说明：日志本身不包含“使用的是哪个配置”，这里通过本软件记录的“切换/应用配置时间线”来近似归因。
// 如果你平时不是通过本软件切换配置，或历史记录不完整，结果会偏差。
func (ls *LogService) GetEnvUsageSummary(days int) (map[string]EnvUsageSummary, error) {
	if days <= 0 {
		days = 7
	}

	byEnv := map[string]EnvUsageSummary{}

	activations, err := LoadEnvActivations()
	if err != nil {
		activations = map[string][]EnvActivationEvent{}
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	cutoffUnix := cutoff.Unix()

	providers := []string{"claude", "codex", "gemini"}
	prepared := map[string][]EnvActivationEvent{}
	for _, p := range providers {
		events := activations[p]
		if len(events) == 0 {
			prepared[p] = events
			continue
		}
		// 若最早事件晚于 cutoff，则用最早 env 回填一个 cutoff 事件，让区间内的日志都有归属
		if events[0].At > cutoffUnix {
			events = append([]EnvActivationEvent{{At: cutoffUnix, Provider: p, EnvName: events[0].EnvName}}, events...)
		}
		prepared[p] = events
	}

	accumulate := func(provider string, records []UsageRecord) {
		events := prepared[provider]
		for _, record := range records {
			ts, _ := parseTimestamp(record.Timestamp)
			if ts.IsZero() || ts.Before(cutoff) {
				continue
			}
			envName := activeEnvAt(events, ts.Unix())
			if strings.TrimSpace(envName) == "" {
				continue
			}

			item := byEnv[envName]
			item.Provider = provider
			item.Requests++
			item.InputTokens += int64(record.InputTokens)
			item.OutputTokens += int64(record.OutputTokens)
			item.CacheReadTokens += int64(record.CacheReadTokens)
			item.CacheWriteTokens += int64(record.CacheWriteTokens)
			item.TotalCost += record.TotalCost
			if item.LastTimestamp == "" || record.Timestamp > item.LastTimestamp {
				item.LastTimestamp = record.Timestamp
			}
			byEnv[envName] = item
		}
	}

	claudeRecords, _ := ls.readClaudeLogs(days)
	accumulate("claude", claudeRecords)

	codexRecords, _ := ls.readCodexLogs(days)
	accumulate("codex", codexRecords)

	geminiRecords, _ := ls.readGeminiLogs(days)
	accumulate("gemini", geminiRecords)

	return byEnv, nil
}

// readClaudeLogs 读取 Claude Code 日志文件
func (ls *LogService) readClaudeLogs(days int) ([]UsageRecord, error) {
	projectsDir := ls.getClaudeProjectsDir()
	if projectsDir == "" {
		return []UsageRecord{}, nil
	}

	// 检查目录是否存在
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return []UsageRecord{}, nil
	}

	// 计算时间范围
	cutoff := time.Now().AddDate(0, 0, -days)

	var records []UsageRecord

	// 遍历 projects 目录
	err := filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误，继续遍历
		}

		// 只处理 .jsonl 文件
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".jsonl") {
			return nil
		}

		// 检查文件修改时间
		if info.ModTime().Before(cutoff) {
			return nil
		}

		// 从路径提取项目信息
		projectPath := extractProjectPath(path)

		// 读取文件
		fileRecords, err := ls.parseJSONLFile(path, projectPath, cutoff)
		if err != nil {
			return nil // 忽略解析错误
		}

		records = append(records, fileRecords...)
		return nil
	})

	if err != nil {
		return []UsageRecord{}, nil
	}

	if records == nil {
		records = []UsageRecord{}
	}

	return records, nil
}

func activeEnvAt(events []EnvActivationEvent, atUnix int64) string {
	if len(events) == 0 {
		return ""
	}
	// 找到第一个 events[i].At > atUnix 的位置，然后回退 1
	idx := sort.Search(len(events), func(i int) bool { return events[i].At > atUnix }) - 1
	if idx < 0 || idx >= len(events) {
		return ""
	}
	return events[idx].EnvName
}

// Gemini 会话文件结构
type geminiSession struct {
	SessionID   string           `json:"sessionId"`
	ProjectHash string           `json:"projectHash"`
	StartTime   string           `json:"startTime"`
	LastUpdated string           `json:"lastUpdated"`
	Messages    []geminiMessage  `json:"messages"`
}

type geminiMessage struct {
	ID        string       `json:"id"`
	Timestamp string       `json:"timestamp"`
	Type      string       `json:"type"` // "user" or "gemini"
	Content   string       `json:"content"`
	Model     string       `json:"model"`
	Tokens    *geminiTokens `json:"tokens"`
}

type geminiTokens struct {
	Input    int `json:"input"`
	Output   int `json:"output"`
	Cached   int `json:"cached"`
	Thoughts int `json:"thoughts"`
	Tool     int `json:"tool"`
	Total    int `json:"total"`
}

// readGeminiLogs 读取 Gemini CLI 日志文件
func (ls *LogService) readGeminiLogs(days int) ([]UsageRecord, error) {
	geminiDir := ls.getGeminiTmpDir()
	if geminiDir == "" {
		return []UsageRecord{}, nil
	}

	// 检查目录是否存在
	if _, err := os.Stat(geminiDir); os.IsNotExist(err) {
		return []UsageRecord{}, nil
	}

	// 计算时间范围
	cutoff := time.Now().AddDate(0, 0, -days)

	var records []UsageRecord

	// 遍历 tmp 目录下的所有项目 hash 目录
	entries, err := os.ReadDir(geminiDir)
	if err != nil {
		return []UsageRecord{}, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// 检查 chats 子目录
		chatsDir := filepath.Join(geminiDir, entry.Name(), "chats")
		if _, err := os.Stat(chatsDir); os.IsNotExist(err) {
			continue
		}

		// 遍历 session 文件
		sessionFiles, err := os.ReadDir(chatsDir)
		if err != nil {
			continue
		}

		for _, sessionFile := range sessionFiles {
			if sessionFile.IsDir() || !strings.HasSuffix(sessionFile.Name(), ".json") {
				continue
			}

			sessionPath := filepath.Join(chatsDir, sessionFile.Name())
			info, err := sessionFile.Info()
			if err != nil || info.ModTime().Before(cutoff) {
				continue
			}

			// 解析会话文件
			sessionRecords, err := ls.parseGeminiSession(sessionPath, entry.Name(), cutoff)
			if err != nil {
				continue
			}

			records = append(records, sessionRecords...)
		}
	}

	if records == nil {
		records = []UsageRecord{}
	}

	return records, nil
}

// parseGeminiSession 解析 Gemini 会话文件
func (ls *LogService) parseGeminiSession(path string, projectHash string, cutoff time.Time) ([]UsageRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session geminiSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	var records []UsageRecord

	for _, msg := range session.Messages {
		// 只处理有 token 统计的 gemini 响应
		if msg.Type != "gemini" || msg.Tokens == nil {
			continue
		}

		// 解析时间戳
		ts, err := parseTimestamp(msg.Timestamp)
		if err != nil || ts.Before(cutoff) {
			continue
		}

		model := msg.Model
		if model == "" {
			model = "gemini-2.5-pro" // 默认模型
		}

		// 计算成本
		cost := ls.calculateCost(
			model,
			msg.Tokens.Input,
			msg.Tokens.Output,
			0, // Gemini 没有 cache create 概念
			msg.Tokens.Cached,
		)

		record := UsageRecord{
			Timestamp:        ts.Format("2006-01-02 15:04:05"),
			Model:            model,
			InputTokens:      msg.Tokens.Input,
			OutputTokens:     msg.Tokens.Output,
			CacheReadTokens:  msg.Tokens.Cached,
			CacheWriteTokens: 0,
			TotalCost:        cost,
			SessionID:        session.SessionID,
			ProjectPath:      projectHash,
		}

		records = append(records, record)
	}

	return records, nil
}

// getGeminiTmpDir 获取 Gemini CLI 临时目录路径
func (ls *LogService) getGeminiTmpDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(homeDir, ".gemini", "tmp")
}

// getClaudeProjectsDir 获取 Claude Code 项目目录路径 (跨平台)
func (ls *LogService) getClaudeProjectsDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// 主要路径: ~/.claude/projects
	primaryPath := filepath.Join(homeDir, ".claude", "projects")

	// 检查主要路径
	if _, err := os.Stat(primaryPath); err == nil {
		return primaryPath
	}

	// Windows 备选路径
	if runtime.GOOS == "windows" {
		// 尝试 AppData\Roaming\.claude\projects
		appData := os.Getenv("APPDATA")
		if appData != "" {
			altPath := filepath.Join(appData, ".claude", "projects")
			if _, err := os.Stat(altPath); err == nil {
				return altPath
			}
		}

		// 尝试 AppData\Local\.claude\projects
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			altPath := filepath.Join(localAppData, ".claude", "projects")
			if _, err := os.Stat(altPath); err == nil {
				return altPath
			}
		}
	}

	// macOS 备选路径
	if runtime.GOOS == "darwin" {
		// 尝试 ~/Library/Application Support/Claude/projects
		altPath := filepath.Join(homeDir, "Library", "Application Support", "Claude", "projects")
		if _, err := os.Stat(altPath); err == nil {
			return altPath
		}
	}

	// Linux 备选路径
	if runtime.GOOS == "linux" {
		// 尝试 ~/.config/claude/projects
		altPath := filepath.Join(homeDir, ".config", "claude", "projects")
		if _, err := os.Stat(altPath); err == nil {
			return altPath
		}
	}

	// 返回主要路径（即使不存在，让调用者处理）
	return primaryPath
}

// parseJSONLFile 解析单个 JSONL 文件
func (ls *LogService) parseJSONLFile(path string, projectPath string, cutoff time.Time) ([]UsageRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []UsageRecord
	scanner := bufio.NewScanner(file)
	// 增大缓冲区以处理长行
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	sessionID := filepath.Base(path)
	sessionID = strings.TrimSuffix(sessionID, ".jsonl")

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry claudeLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// 只处理包含 message.usage 信息的条目
		if entry.Message == nil || entry.Message.Usage == nil {
			continue
		}

		// 解析时间戳
		ts, err := parseTimestamp(entry.Timestamp)
		if err != nil || ts.Before(cutoff) {
			continue
		}

		// 计算成本 (包含缓存成本)
		cost := ls.calculateCost(
			entry.Message.Model,
			entry.Message.Usage.InputTokens,
			entry.Message.Usage.OutputTokens,
			entry.Message.Usage.CacheCreate,
			entry.Message.Usage.CacheRead,
		)

		record := UsageRecord{
			Timestamp:        ts.Format("2006-01-02 15:04:05"),
			Model:            entry.Message.Model,
			InputTokens:      entry.Message.Usage.InputTokens,
			OutputTokens:     entry.Message.Usage.OutputTokens,
			CacheReadTokens:  entry.Message.Usage.CacheRead,
			CacheWriteTokens: entry.Message.Usage.CacheCreate,
			TotalCost:        cost,
			SessionID:        sessionID,
			ProjectPath:      projectPath,
		}

		records = append(records, record)
	}

	return records, nil
}

// calculateCost 计算成本 (包含缓存成本)
func (ls *LogService) calculateCost(model string, inputTokens, outputTokens, cacheCreateTokens, cacheReadTokens int) float64 {
	pricing, ok := modelPricing[model]
	if !ok {
		// 尝试模糊匹配
		for name, p := range modelPricing {
			if strings.Contains(strings.ToLower(model), strings.ToLower(name)) {
				pricing = p
				ok = true
				break
			}
		}
	}

	if !ok {
		// 默认使用 sonnet 定价
		pricing = modelPricing["claude-sonnet-4-20250514"]
	}

	// 计算成本 (价格是每百万 token)
	inputCost := float64(inputTokens) * pricing.Input / 1_000_000
	outputCost := float64(outputTokens) * pricing.Output / 1_000_000
	cacheCreateCost := float64(cacheCreateTokens) * pricing.CacheCreate / 1_000_000
	cacheReadCost := float64(cacheReadTokens) * pricing.CacheRead / 1_000_000

	return inputCost + outputCost + cacheCreateCost + cacheReadCost
}

// extractProjectPath 从文件路径提取项目路径
func extractProjectPath(path string) string {
	// 路径格式: ~/.claude/projects/<hash>/conversations/<session>.jsonl
	parts := strings.Split(path, string(os.PathSeparator))
	for i, part := range parts {
		if part == "projects" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// parseTimestamp 解析时间戳
func parseTimestamp(ts string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t, nil
		}
	}

	return time.Time{}, nil
}

// Codex CLI 日志条目结构
type codexLogEntry struct {
	Type      string        `json:"type"`
	Timestamp string        `json:"timestamp"`
	Payload   *codexPayload `json:"payload"`
}

type codexPayload struct {
	Type  string          `json:"type"`
	Info  *codexTokenInfo `json:"info"`
	Model string          `json:"model"` // for turn_context entries
}

type codexTokenInfo struct {
	TotalTokenUsage *codexTokenUsage `json:"total_token_usage"`
	LastTokenUsage  *codexTokenUsage `json:"last_token_usage"`
}

type codexTokenUsage struct {
	InputTokens           int `json:"input_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
	TotalTokens           int `json:"total_tokens"`
}

// readCodexLogs 读取 Codex CLI 日志文件
func (ls *LogService) readCodexLogs(days int) ([]UsageRecord, error) {
	codexDir := ls.getCodexDir()
	if codexDir == "" {
		return []UsageRecord{}, nil
	}

	// Codex 日志路径: ~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl
	sessionsDir := filepath.Join(codexDir, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return []UsageRecord{}, nil
	}

	// 计算时间范围
	cutoff := time.Now().AddDate(0, 0, -days)

	var records []UsageRecord

	// 遍历 sessions 目录下的日期文件夹
	err := filepath.Walk(sessionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误，继续遍历
		}

		// 只处理 .jsonl 文件
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".jsonl") {
			return nil
		}

		// 检查文件修改时间
		if info.ModTime().Before(cutoff) {
			return nil
		}

		// 解析会话文件
		sessionRecords, err := ls.parseCodexSession(path, info.Name(), cutoff)
		if err != nil {
			return nil // 忽略解析错误
		}

		records = append(records, sessionRecords...)
		return nil
	})

	if err != nil {
		return []UsageRecord{}, nil
	}

	if records == nil {
		records = []UsageRecord{}
	}

	return records, nil
}

// parseCodexSession 解析 Codex 会话 JSONL 文件
func (ls *LogService) parseCodexSession(path string, sessionID string, cutoff time.Time) ([]UsageRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []UsageRecord
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var lastTotalTokens *codexTokenUsage
	var currentModel string = "gpt-5-codex" // 默认模型

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// 先尝试解析为 event_msg 格式
		var eventEntry struct {
			Type      string `json:"type"`
			Timestamp string `json:"timestamp"`
			Payload   *struct {
				Type  string          `json:"type"`
				Info  *codexTokenInfo `json:"info"`
				Model string          `json:"model"`
			} `json:"payload"`
		}

		if err := json.Unmarshal([]byte(line), &eventEntry); err != nil {
			continue
		}

		// 解析时间戳
		ts, err := parseTimestamp(eventEntry.Timestamp)
		if err != nil || ts.Before(cutoff) {
			continue
		}

		// 处理 turn_context 类型 - 获取模型信息
		if eventEntry.Type == "turn_context" && eventEntry.Payload != nil && eventEntry.Payload.Model != "" {
			currentModel = eventEntry.Payload.Model
			continue
		}

		// 处理 event_msg 类型的 token_count
		if eventEntry.Type == "event_msg" && eventEntry.Payload != nil && eventEntry.Payload.Type == "token_count" {
			if eventEntry.Payload.Info == nil || eventEntry.Payload.Info.TotalTokenUsage == nil {
				continue
			}

			tc := eventEntry.Payload.Info.TotalTokenUsage

			// 计算增量 (Codex 日志是累计值)
			var inputDelta, cachedDelta, outputDelta int
			if lastTotalTokens != nil {
				inputDelta = tc.InputTokens - lastTotalTokens.InputTokens
				cachedDelta = tc.CachedInputTokens - lastTotalTokens.CachedInputTokens
				outputDelta = tc.OutputTokens - lastTotalTokens.OutputTokens
			} else {
				inputDelta = tc.InputTokens
				cachedDelta = tc.CachedInputTokens
				outputDelta = tc.OutputTokens
			}

			// 只记录有增量的条目
			if inputDelta > 0 || outputDelta > 0 {
				cost := ls.calculateCost(
					currentModel,
					inputDelta,
					outputDelta,
					0,
					cachedDelta,
				)

				record := UsageRecord{
					Timestamp:        ts.Format("2006-01-02 15:04:05"),
					Model:            currentModel,
					InputTokens:      inputDelta,
					OutputTokens:     outputDelta,
					CacheReadTokens:  cachedDelta,
					CacheWriteTokens: 0,
					TotalCost:        cost,
					SessionID:        sessionID,
					ProjectPath:      filepath.Base(path),
				}

				records = append(records, record)
			}

			lastTotalTokens = tc
		}
	}

	return records, nil
}

// getCodexDir 获取 Codex CLI 目录路径
func (ls *LogService) getCodexDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// 检查 CODEX_HOME 环境变量
	if codexHome := os.Getenv("CODEX_HOME"); codexHome != "" {
		return codexHome
	}

	return filepath.Join(homeDir, ".codex")
}
