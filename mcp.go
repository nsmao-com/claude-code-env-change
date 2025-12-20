package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)
const (
	mcpStoreDir      = ".claude-env-switcher"
	mcpStoreFile     = "mcp.json"
	claudeMcpFile    = ".claude.json"
	codexDirName     = ".codex"
	codexConfigFile  = "config.toml"
	geminiDirName    = ".gemini"
	geminiConfigFile = "settings.json"
	platClaudeCode   = "claude-code"
	platCodex        = "codex"
	platGemini       = "gemini"
)

var placeholderPattern = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

// MCPService MCP 服务管理
type MCPService struct {
	mu sync.Mutex
}

// NewMCPService 创建 MCP 服务实例
func NewMCPService() *MCPService {
	return &MCPService{}
}

// MCPServer MCP 服务器配置
type MCPServer struct {
	Name                string            `json:"name"`
	Type                string            `json:"type"` // stdio 或 http
	Command             string            `json:"command,omitempty"`
	Args                []string          `json:"args,omitempty"`
	Env                 map[string]string `json:"env,omitempty"`
	URL                 string            `json:"url,omitempty"`
	Website             string            `json:"website,omitempty"`
	Tips                string            `json:"tips,omitempty"`
	EnablePlatform      []string          `json:"enable_platform"`
	EnabledInClaude     bool              `json:"enabled_in_claude"`
	EnabledInCodex      bool              `json:"enabled_in_codex"`
	EnabledInGemini     bool              `json:"enabled_in_gemini"`
	MissingPlaceholders []string          `json:"missing_placeholders"`
}

// rawMCPServer 内部存储格式
type rawMCPServer struct {
	Type           string            `json:"type"`
	Command        string            `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	URL            string            `json:"url,omitempty"`
	Website        string            `json:"website,omitempty"`
	Tips           string            `json:"tips,omitempty"`
	EnablePlatform []string          `json:"enable_platform"`
}

// claudeMcpFilePayload Claude 配置文件格式
type claudeMcpFilePayload struct {
	Servers map[string]json.RawMessage `json:"mcpServers"`
}

// codexMcpFilePayload Codex 配置文件格式
type codexMcpFilePayload struct {
	Servers map[string]map[string]any `toml:"mcp_servers"`
}

// claudeDesktopServer Claude MCP 服务器格式
type claudeDesktopServer struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
}

// ListServers 列出所有 MCP 服务器
func (ms *MCPService) ListServers() ([]MCPServer, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	config, err := ms.loadConfig()
	if err != nil {
		return nil, err
	}

	claudeEnabled := loadClaudeEnabledServers()
	codexEnabled := loadCodexEnabledServers()
	geminiEnabled := loadGeminiEnabledServers()

	names := make([]string, 0, len(config))
	for name := range config {
		names = append(names, name)
	}
	sort.Strings(names)

	servers := make([]MCPServer, 0, len(names))
	for _, name := range names {
		entry := config[name]
		typ := normalizeServerType(entry.Type)
		platforms := normalizePlatforms(entry.EnablePlatform)
		server := MCPServer{
			Name:            name,
			Type:            typ,
			Command:         strings.TrimSpace(entry.Command),
			Args:            cloneArgs(entry.Args),
			Env:             cloneEnv(entry.Env),
			URL:             strings.TrimSpace(entry.URL),
			Website:         strings.TrimSpace(entry.Website),
			Tips:            strings.TrimSpace(entry.Tips),
			EnablePlatform:  platforms,
			EnabledInClaude: containsNormalized(claudeEnabled, name),
			EnabledInCodex:  containsNormalized(codexEnabled, name),
			EnabledInGemini: containsNormalized(geminiEnabled, name),
		}
		server.MissingPlaceholders = detectPlaceholders(server.URL, server.Args)
		servers = append(servers, server)
	}

	return servers, nil
}

// SaveServers 保存 MCP 服务器配置
func (ms *MCPService) SaveServers(servers []MCPServer) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	normalized := make([]MCPServer, len(servers))
	raw := make(map[string]rawMCPServer, len(servers))

	for i := range servers {
		server := servers[i]
		name := strings.TrimSpace(server.Name)
		if name == "" {
			return fmt.Errorf("服务器名称不能为空")
		}
		typ := normalizeServerType(server.Type)
		platforms := normalizePlatforms(server.EnablePlatform)
		args := cleanArgs(server.Args)
		env := cleanEnv(server.Env)
		command := strings.TrimSpace(server.Command)
		url := strings.TrimSpace(server.URL)

		if typ == "stdio" && command == "" {
			return fmt.Errorf("%s 需要提供 command", name)
		}
		if (typ == "http" || typ == "sse") && url == "" {
			return fmt.Errorf("%s 需要提供 url", name)
		}

		normalized[i] = MCPServer{
			Name:            name,
			Type:            typ,
			Command:         command,
			Args:            args,
			Env:             env,
			URL:             url,
			Website:         strings.TrimSpace(server.Website),
			Tips:            strings.TrimSpace(server.Tips),
			EnablePlatform:  platforms,
			EnabledInClaude: server.EnabledInClaude,
			EnabledInCodex:  server.EnabledInCodex,
			EnabledInGemini: server.EnabledInGemini,
		}

		raw[name] = rawMCPServer{
			Type:           typ,
			Command:        command,
			Args:           args,
			Env:            env,
			URL:            url,
			Website:        normalized[i].Website,
			Tips:           normalized[i].Tips,
			EnablePlatform: platforms,
		}

		placeholders := detectPlaceholders(url, args)
		normalized[i].MissingPlaceholders = placeholders
		if len(placeholders) > 0 {
			normalized[i].EnablePlatform = []string{}
			rawEntry := raw[name]
			rawEntry.EnablePlatform = []string{}
			raw[name] = rawEntry
		}
	}

	if err := ms.saveConfig(raw); err != nil {
		return err
	}
	if err := ms.syncClaudeServers(normalized); err != nil {
		return err
	}
	if err := ms.syncCodexServers(normalized); err != nil {
		return err
	}
	if err := ms.syncGeminiServers(normalized); err != nil {
		return err
	}
	return nil
}

// configPath 获取配置文件路径
func (ms *MCPService) configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, mcpStoreFile), nil
}

// loadConfig 加载配置
func (ms *MCPService) loadConfig() (map[string]rawMCPServer, error) {
	path, err := ms.configPath()
	if err != nil {
		return nil, err
	}

	payload := map[string]rawMCPServer{}
	if data, err := os.ReadFile(path); err == nil {
		if len(data) > 0 {
			if err := json.Unmarshal(data, &payload); err != nil {
				return nil, err
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	for name, entry := range payload {
		payload[name] = normalizeRawEntry(entry)
	}

	changed := false

	// 从 Claude 配置导入
	if imported, err := ms.importFromClaude(payload); err == nil {
		if ms.mergeImportedServers(payload, imported) {
			changed = true
		}
	}

	// 从 Codex 配置导入
	if imported, err := ms.importFromCodex(payload); err == nil {
		if ms.mergeImportedServers(payload, imported) {
			changed = true
		}
	}

	// 从 Gemini 配置导入
	if imported, err := ms.importFromGemini(payload); err == nil {
		if ms.mergeImportedServers(payload, imported) {
			changed = true
		}
	}

	if changed {
		if err := ms.saveConfig(payload); err != nil {
			return payload, err
		}
	}

	return payload, nil
}

// importFromClaude 从 Claude 配置导入
func (ms *MCPService) importFromClaude(existing map[string]rawMCPServer) (map[string]rawMCPServer, error) {
	path, err := claudeConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]rawMCPServer{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string]rawMCPServer{}, nil
	}

	var payload struct {
		Servers map[string]claudeDesktopServer `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	result := make(map[string]rawMCPServer, len(payload.Servers))
	for name, entry := range payload.Servers {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			continue
		}
		if _, exists := existing[trimmedName]; exists {
			continue
		}

		typeHint := entry.Type
		if strings.TrimSpace(typeHint) == "" {
			if strings.TrimSpace(entry.URL) != "" {
				typeHint = "http"
			}
		}
		if strings.TrimSpace(typeHint) == "" {
			typeHint = "stdio"
		}

		typ := normalizeServerType(typeHint)
		if (typ == "http" || typ == "sse") && entry.URL == "" {
			continue
		}
		if typ == "stdio" && entry.Command == "" {
			continue
		}

		result[trimmedName] = rawMCPServer{
			Type:           typ,
			Command:        strings.TrimSpace(entry.Command),
			Args:           cleanArgs(entry.Args),
			Env:            cleanEnv(entry.Env),
			URL:            strings.TrimSpace(entry.URL),
			EnablePlatform: []string{platClaudeCode},
		}
	}
	return result, nil
}

// saveConfig 保存配置
func (ms *MCPService) saveConfig(payload map[string]rawMCPServer) error {
	path, err := ms.configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// syncClaudeServers 同步到 Claude 配置
func (ms *MCPService) syncClaudeServers(servers []MCPServer) error {
	path, err := claudeConfigPath()
	if err != nil {
		return err
	}

	desired := make(map[string]claudeDesktopServer)
	for _, server := range servers {
		if !platformContains(server.EnablePlatform, platClaudeCode) {
			continue
		}
		desired[server.Name] = buildClaudeDesktopEntry(server)
	}

	payload := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			payload = make(map[string]any)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	payload["mcpServers"] = desired
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// syncCodexServers 同步到 Codex 配置
func (ms *MCPService) syncCodexServers(servers []MCPServer) error {
	path, err := codexConfigPath()
	if err != nil {
		return err
	}

	desired := make(map[string]map[string]any)
	for _, server := range servers {
		if !platformContains(server.EnablePlatform, platCodex) {
			continue
		}
		desired[server.Name] = buildCodexEntry(server)
	}

	payload := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := toml.Unmarshal(data, &payload); err != nil {
			payload = make(map[string]any)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	payload["mcp_servers"] = desired
	data, err := toml.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// syncGeminiServers 同步到 Gemini 配置
func (ms *MCPService) syncGeminiServers(servers []MCPServer) error {
	path, err := geminiConfigPath()
	if err != nil {
		return err
	}

	desired := make(map[string]claudeDesktopServer)
	for _, server := range servers {
		if !platformContains(server.EnablePlatform, platGemini) {
			continue
		}
		desired[server.Name] = buildClaudeDesktopEntry(server) // Gemini 使用相同的 JSON 格式
	}

	payload := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			payload = make(map[string]any)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	payload["mcpServers"] = desired
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// importFromCodex 从 Codex 配置导入
func (ms *MCPService) importFromCodex(existing map[string]rawMCPServer) (map[string]rawMCPServer, error) {
	path, err := codexConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]rawMCPServer{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string]rawMCPServer{}, nil
	}

	var payload codexMcpFilePayload
	if err := toml.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	result := make(map[string]rawMCPServer, len(payload.Servers))
	for name, entry := range payload.Servers {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			continue
		}
		if _, exists := existing[trimmedName]; exists {
			continue
		}

		// 解析 Codex 格式
		command, _ := entry["command"].(string)
		url, _ := entry["url"].(string)
		typeHint, _ := entry["type"].(string)

		if strings.TrimSpace(typeHint) == "" {
			if strings.TrimSpace(url) != "" {
				typeHint = "http"
			} else {
				typeHint = "stdio"
			}
		}

		typ := normalizeServerType(typeHint)
		if (typ == "http" || typ == "sse") && url == "" {
			continue
		}
		if typ == "stdio" && command == "" {
			continue
		}

		// 解析 args
		var args []string
		if argsRaw, ok := entry["args"].([]interface{}); ok {
			for _, arg := range argsRaw {
				if s, ok := arg.(string); ok {
					args = append(args, s)
				}
			}
		}

		// 解析 env
		env := make(map[string]string)
		if envRaw, ok := entry["env"].(map[string]interface{}); ok {
			for k, v := range envRaw {
				if s, ok := v.(string); ok {
					env[k] = s
				}
			}
		}

		result[trimmedName] = rawMCPServer{
			Type:           typ,
			Command:        strings.TrimSpace(command),
			Args:           cleanArgs(args),
			Env:            cleanEnv(env),
			URL:            strings.TrimSpace(url),
			EnablePlatform: []string{platCodex},
		}
	}
	return result, nil
}

// importFromGemini 从 Gemini 配置导入
func (ms *MCPService) importFromGemini(existing map[string]rawMCPServer) (map[string]rawMCPServer, error) {
	path, err := geminiConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]rawMCPServer{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string]rawMCPServer{}, nil
	}

	var payload struct {
		Servers map[string]claudeDesktopServer `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	result := make(map[string]rawMCPServer, len(payload.Servers))
	for name, entry := range payload.Servers {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			continue
		}
		if _, exists := existing[trimmedName]; exists {
			continue
		}

		typeHint := entry.Type
		if strings.TrimSpace(typeHint) == "" {
			if strings.TrimSpace(entry.URL) != "" {
				typeHint = "http"
			}
		}
		if strings.TrimSpace(typeHint) == "" {
			typeHint = "stdio"
		}

		typ := normalizeServerType(typeHint)
		if (typ == "http" || typ == "sse") && entry.URL == "" {
			continue
		}
		if typ == "stdio" && entry.Command == "" {
			continue
		}

		result[trimmedName] = rawMCPServer{
			Type:           typ,
			Command:        strings.TrimSpace(entry.Command),
			Args:           cleanArgs(entry.Args),
			Env:            cleanEnv(entry.Env),
			URL:            strings.TrimSpace(entry.URL),
			EnablePlatform: []string{platGemini},
		}
	}
	return result, nil
}

// mergeImportedServers 合并导入的服务器
func (ms *MCPService) mergeImportedServers(target, imported map[string]rawMCPServer) bool {
	changed := false
	for name, entry := range imported {
		entry = normalizeRawEntry(entry)
		if existing, ok := target[name]; ok {
			entry.EnablePlatform = unionPlatforms(existing.EnablePlatform, entry.EnablePlatform)
			if entry.Website == "" {
				entry.Website = existing.Website
			}
			if entry.Tips == "" {
				entry.Tips = existing.Tips
			}
		}
		target[name] = entry
		changed = true
	}
	return changed
}

// 辅助函数
func normalizeServerType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "http":
		return "http"
	case "sse":
		return "sse"
	default:
		return "stdio"
	}
}

func normalizePlatforms(values []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, raw := range values {
		if platform, ok := normalizePlatform(raw); ok {
			if _, exists := seen[platform]; exists {
				continue
			}
			seen[platform] = struct{}{}
			result = append(result, platform)
		}
	}
	return result
}

func normalizePlatform(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "claude", "claude_code", "claude-code":
		return "claude-code", true
	case "codex":
		return "codex", true
	case "gemini":
		return "gemini", true
	default:
		return "", false
	}
}

func unionPlatforms(primary, secondary []string) []string {
	combined := append([]string{}, primary...)
	combined = append(combined, secondary...)
	return normalizePlatforms(combined)
}

func normalizeRawEntry(entry rawMCPServer) rawMCPServer {
	entry.Type = normalizeServerType(entry.Type)
	entry.Command = strings.TrimSpace(entry.Command)
	entry.URL = strings.TrimSpace(entry.URL)
	entry.Website = strings.TrimSpace(entry.Website)
	entry.Tips = strings.TrimSpace(entry.Tips)
	entry.Args = cleanArgs(entry.Args)
	entry.Env = cleanEnv(entry.Env)
	entry.EnablePlatform = normalizePlatforms(entry.EnablePlatform)
	return entry
}

func cloneArgs(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	dup := make([]string, len(values))
	copy(dup, values)
	return dup
}

func cloneEnv(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	dup := make(map[string]string, len(values))
	for k, v := range values {
		dup[k] = v
	}
	return dup
}

func cleanArgs(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func cleanEnv(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		result[trimmedKey] = strings.TrimSpace(value)
	}
	return result
}

func containsNormalized(pool map[string]struct{}, value string) bool {
	if len(pool) == 0 {
		return false
	}
	_, ok := pool[strings.ToLower(strings.TrimSpace(value))]
	return ok
}

func loadClaudeEnabledServers() map[string]struct{} {
	result := map[string]struct{}{}
	home, err := os.UserHomeDir()
	if err != nil {
		return result
	}
	path := filepath.Join(home, claudeMcpFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	var payload claudeMcpFilePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return result
	}
	for name := range payload.Servers {
		result[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}
	return result
}

func loadCodexEnabledServers() map[string]struct{} {
	result := map[string]struct{}{}
	home, err := os.UserHomeDir()
	if err != nil {
		return result
	}
	path := filepath.Join(home, codexDirName, codexConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	var payload codexMcpFilePayload
	if err := toml.Unmarshal(data, &payload); err != nil {
		return result
	}
	for name := range payload.Servers {
		result[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}
	return result
}

func loadGeminiEnabledServers() map[string]struct{} {
	result := map[string]struct{}{}
	home, err := os.UserHomeDir()
	if err != nil {
		return result
	}
	path := filepath.Join(home, geminiDirName, geminiConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	var payload claudeMcpFilePayload // Gemini 使用相同的 mcpServers 格式
	if err := json.Unmarshal(data, &payload); err != nil {
		return result
	}
	for name := range payload.Servers {
		result[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}
	return result
}

func platformContains(platforms []string, target string) bool {
	for _, value := range platforms {
		if value == target {
			return true
		}
	}
	return false
}

func buildClaudeDesktopEntry(server MCPServer) claudeDesktopServer {
	entry := claudeDesktopServer{Type: server.Type}
	if server.Type == "http" || server.Type == "sse" {
		entry.URL = server.URL
	} else {
		entry.Command = server.Command
		if len(server.Args) > 0 {
			entry.Args = server.Args
		}
		if len(server.Env) > 0 {
			entry.Env = server.Env
		}
	}
	return entry
}

func buildCodexEntry(server MCPServer) map[string]any {
	entry := make(map[string]any)
	entry["type"] = server.Type
	if server.Type == "http" || server.Type == "sse" {
		entry["url"] = server.URL
	} else {
		entry["command"] = server.Command
		if len(server.Args) > 0 {
			entry["args"] = server.Args
		}
		if len(server.Env) > 0 {
			entry["env"] = server.Env
		}
	}
	return entry
}

func claudeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, claudeMcpFile), nil
}

func codexConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, codexDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, codexConfigFile), nil
}

func geminiConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, geminiDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, geminiConfigFile), nil
}

func detectPlaceholders(url string, args []string) []string {
	set := make(map[string]struct{})
	collectPlaceholders(set, url)
	for _, arg := range args {
		collectPlaceholders(set, arg)
	}
	if len(set) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(set))
	for key := range set {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func collectPlaceholders(set map[string]struct{}, value string) {
	if value == "" {
		return
	}
	matches := placeholderPattern.FindAllStringSubmatch(value, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		set[match[1]] = struct{}{}
	}
}

// MCPTestResult MCP 服务器测试结果
type MCPTestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Latency int64  `json:"latency"` // 毫秒
}

// TestServer 测试 MCP 服务器是否可用
func (ms *MCPService) TestServer(server MCPServer) MCPTestResult {
	start := time.Now()

	if server.Type == "http" || server.Type == "sse" {
		return ms.testHTTPServer(server.URL, start)
	}
	return ms.testStdioServer(server.Command, server.Args, server.Env, start)
}

// testHTTPServer 测试 HTTP 类型的 MCP 服务器
func (ms *MCPService) testHTTPServer(url string, start time.Time) MCPTestResult {
	if url == "" {
		return MCPTestResult{Success: false, Message: "URL 为空"}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return MCPTestResult{Success: false, Message: fmt.Sprintf("连接失败: %v", err), Latency: latency}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return MCPTestResult{Success: true, Message: fmt.Sprintf("连接成功 (HTTP %d)", resp.StatusCode), Latency: latency}
	}
	return MCPTestResult{Success: false, Message: fmt.Sprintf("服务器错误 (HTTP %d)", resp.StatusCode), Latency: latency}
}

// testStdioServer 测试 Stdio 类型的 MCP 服务器
func (ms *MCPService) testStdioServer(command string, args []string, env map[string]string, start time.Time) MCPTestResult {
	if command == "" {
		return MCPTestResult{Success: false, Message: "Command 为空"}
	}

	// 检查命令是否存在
	cmdPath, err := exec.LookPath(command)
	if err != nil {
		// 尝试常见的路径
		if runtime.GOOS == "windows" {
			// Windows 上尝试查找 npx, node 等
			possiblePaths := []string{
				filepath.Join(os.Getenv("APPDATA"), "npm", command+".cmd"),
				filepath.Join(os.Getenv("PROGRAMFILES"), "nodejs", command+".exe"),
			}
			found := false
			for _, p := range possiblePaths {
				if _, err := os.Stat(p); err == nil {
					cmdPath = p
					found = true
					break
				}
			}
			if !found {
				return MCPTestResult{Success: false, Message: fmt.Sprintf("命令未找到: %s", command), Latency: time.Since(start).Milliseconds()}
			}
		} else {
			return MCPTestResult{Success: false, Message: fmt.Sprintf("命令未找到: %s", command), Latency: time.Since(start).Milliseconds()}
		}
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdPath, args...)

	// 设置环境变量
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// 尝试启动命令（不等待完成，只检查是否能启动）
	err = cmd.Start()
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return MCPTestResult{Success: false, Message: fmt.Sprintf("启动失败: %v", err), Latency: latency}
	}

	// 立即终止进程
	if cmd.Process != nil {
		cmd.Process.Kill()
	}

	return MCPTestResult{Success: true, Message: "命令可执行", Latency: latency}
}

// ImportFromJSON 从 JSON 字符串导入 MCP 服务器配置
func (ms *MCPService) ImportFromJSON(jsonStr string) ([]MCPServer, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		return nil, fmt.Errorf("JSON 内容为空")
	}

	// 尝试解析为 Claude 格式的 mcpServers
	var claudeFormat struct {
		Servers map[string]claudeDesktopServer `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &claudeFormat); err == nil && len(claudeFormat.Servers) > 0 {
		return ms.parseClaudeFormat(claudeFormat.Servers)
	}

	// 尝试直接解析为 mcpServers 对象
	var mcpServersMap map[string]claudeDesktopServer
	if err := json.Unmarshal([]byte(jsonStr), &mcpServersMap); err == nil && len(mcpServersMap) > 0 {
		return ms.parseClaudeFormat(mcpServersMap)
	}

	// 尝试解析为单个服务器
	var singleServer claudeDesktopServer
	if err := json.Unmarshal([]byte(jsonStr), &singleServer); err == nil {
		if singleServer.Command != "" || singleServer.URL != "" {
			servers, err := ms.parseClaudeFormat(map[string]claudeDesktopServer{"imported": singleServer})
			if err == nil && len(servers) > 0 {
				servers[0].Name = "imported_server"
			}
			return servers, err
		}
	}

	// 尝试解析为服务器数组
	var serverArray []MCPServer
	if err := json.Unmarshal([]byte(jsonStr), &serverArray); err == nil && len(serverArray) > 0 {
		return serverArray, nil
	}

	return nil, fmt.Errorf("无法解析 JSON 格式，请检查格式是否正确")
}

// parseClaudeFormat 解析 Claude 格式的服务器配置
func (ms *MCPService) parseClaudeFormat(servers map[string]claudeDesktopServer) ([]MCPServer, error) {
	result := make([]MCPServer, 0, len(servers))

	for name, entry := range servers {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			continue
		}

		serverType := "stdio"
		if entry.Type != "" {
			serverType = normalizeServerType(entry.Type)
		} else if entry.URL != "" {
			serverType = "http"
		}

		server := MCPServer{
			Name:           trimmedName,
			Type:           serverType,
			Command:        strings.TrimSpace(entry.Command),
			Args:           cleanArgs(entry.Args),
			Env:            cleanEnv(entry.Env),
			URL:            strings.TrimSpace(entry.URL),
			EnablePlatform: []string{platClaudeCode}, // 默认启用 Claude
		}

		// 验证
		if (serverType == "http" || serverType == "sse") && server.URL == "" {
			continue
		}
		if serverType == "stdio" && server.Command == "" {
			continue
		}

		server.MissingPlaceholders = detectPlaceholders(server.URL, server.Args)
		result = append(result, server)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("没有找到有效的服务器配置")
	}

	return result, nil
}

// AddServers 添加服务器到现有列表（合并）
func (ms *MCPService) AddServers(newServers []MCPServer) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	config, err := ms.loadConfig()
	if err != nil {
		return err
	}

	// 添加新服务器
	for _, server := range newServers {
		name := strings.TrimSpace(server.Name)
		if name == "" {
			continue
		}

		// 如果名称已存在，添加后缀
		originalName := name
		suffix := 1
		for {
			if _, exists := config[name]; !exists {
				break
			}
			name = fmt.Sprintf("%s_%d", originalName, suffix)
			suffix++
		}
		server.Name = name

		typ := normalizeServerType(server.Type)
		platforms := normalizePlatforms(server.EnablePlatform)

		config[name] = rawMCPServer{
			Type:           typ,
			Command:        strings.TrimSpace(server.Command),
			Args:           cleanArgs(server.Args),
			Env:            cleanEnv(server.Env),
			URL:            strings.TrimSpace(server.URL),
			Website:        strings.TrimSpace(server.Website),
			Tips:           strings.TrimSpace(server.Tips),
			EnablePlatform: platforms,
		}
	}

	// 保存配置
	if err := ms.saveConfig(config); err != nil {
		return err
	}

	// 从 config 构建 servers 列表用于同步（不调用 ListServers 避免死锁）
	servers := ms.buildServersFromConfig(config)
	if err := ms.syncClaudeServers(servers); err != nil {
		return err
	}
	if err := ms.syncCodexServers(servers); err != nil {
		return err
	}
	return ms.syncGeminiServers(servers)
}

// buildServersFromConfig 从配置构建服务器列表（内部使用，不加锁）
func (ms *MCPService) buildServersFromConfig(config map[string]rawMCPServer) []MCPServer {
	claudeEnabled := loadClaudeEnabledServers()
	codexEnabled := loadCodexEnabledServers()
	geminiEnabled := loadGeminiEnabledServers()

	names := make([]string, 0, len(config))
	for name := range config {
		names = append(names, name)
	}
	sort.Strings(names)

	servers := make([]MCPServer, 0, len(names))
	for _, name := range names {
		entry := config[name]
		typ := normalizeServerType(entry.Type)
		platforms := normalizePlatforms(entry.EnablePlatform)
		server := MCPServer{
			Name:            name,
			Type:            typ,
			Command:         strings.TrimSpace(entry.Command),
			Args:            cloneArgs(entry.Args),
			Env:             cloneEnv(entry.Env),
			URL:             strings.TrimSpace(entry.URL),
			Website:         strings.TrimSpace(entry.Website),
			Tips:            strings.TrimSpace(entry.Tips),
			EnablePlatform:  platforms,
			EnabledInClaude: containsNormalized(claudeEnabled, name),
			EnabledInCodex:  containsNormalized(codexEnabled, name),
			EnabledInGemini: containsNormalized(geminiEnabled, name),
		}
		server.MissingPlaceholders = detectPlaceholders(server.URL, server.Args)
		servers = append(servers, server)
	}

	return servers
}
