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
	platAntigravity  = "antigravity"
	platOpencode     = "opencode"
	platGrok         = "grok"
	grokDirName      = ".grok"
	grokTomlName     = "config.toml"
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
	Name                 string            `json:"name"`
	Type                 string            `json:"type"` // stdio 或 http
	Command              string            `json:"command,omitempty"`
	Args                 []string          `json:"args,omitempty"`
	Env                  map[string]string `json:"env,omitempty"`
	URL                  string            `json:"url,omitempty"`
	Headers              map[string]string `json:"headers,omitempty"`
	Website              string            `json:"website,omitempty"`
	Tips                 string            `json:"tips,omitempty"`
	EnablePlatform       []string          `json:"enable_platform"`
	EnabledInClaude      bool              `json:"enabled_in_claude"`
	EnabledInCodex       bool              `json:"enabled_in_codex"`
	EnabledInAntigravity bool              `json:"enabled_in_antigravity"`
	EnabledInOpencode    bool              `json:"enabled_in_opencode"`
	EnabledInGrok        bool              `json:"enabled_in_grok"`
	MissingPlaceholders  []string          `json:"missing_placeholders"`
}

// rawMCPServer 内部存储格式
type rawMCPServer struct {
	Type           string            `json:"type"`
	Command        string            `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	URL            string            `json:"url,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
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
	Headers map[string]string `json:"headers,omitempty"`
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
	antigravityEnabled := loadAntigravityEnabledServers()
	opencodeEnabled := loadOpencodeEnabledServers()
	grokEnabled := loadGrokEnabledServers()

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
			Name:                 name,
			Type:                 typ,
			Command:              strings.TrimSpace(entry.Command),
			Args:                 cloneArgs(entry.Args),
			Env:                  cloneEnv(entry.Env),
			URL:                  strings.TrimSpace(entry.URL),
			Headers:              cloneEnv(entry.Headers),
			Website:              strings.TrimSpace(entry.Website),
			Tips:                 strings.TrimSpace(entry.Tips),
			EnablePlatform:       platforms,
			EnabledInClaude:      containsNormalized(claudeEnabled, name),
			EnabledInCodex:       containsNormalized(codexEnabled, name),
			EnabledInAntigravity: containsNormalized(antigravityEnabled, name),
			EnabledInOpencode:    containsNormalized(opencodeEnabled, name),
			EnabledInGrok:        containsNormalized(grokEnabled, name),
		}
		server.MissingPlaceholders = detectPlaceholders(server.URL, server.Args, headerValues(server.Headers)...)
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

		headers := cleanEnv(server.Headers)
		normalized[i] = MCPServer{
			Name:                 name,
			Type:                 typ,
			Command:              command,
			Args:                 args,
			Env:                  env,
			URL:                  url,
			Headers:              headers,
			Website:              strings.TrimSpace(server.Website),
			Tips:                 strings.TrimSpace(server.Tips),
			EnablePlatform:       platforms,
			EnabledInClaude:      server.EnabledInClaude,
			EnabledInCodex:       server.EnabledInCodex,
			EnabledInAntigravity: server.EnabledInAntigravity,
		}

		raw[name] = rawMCPServer{
			Type:           typ,
			Command:        command,
			Args:           args,
			Env:            env,
			URL:            url,
			Headers:        headers,
			Website:        normalized[i].Website,
			Tips:           normalized[i].Tips,
			EnablePlatform: platforms,
		}

		placeholders := detectPlaceholders(url, args, headerValues(headers)...)
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

	if err := ms.syncAntigravityServers(normalized); err != nil {
		return err
	}
	if err := ms.syncGrokServers(normalized); err != nil {
		return err
	}
	if err := ms.syncOpencodeServers(normalized); err != nil {
		return err
	}

	notifyCloudSync()
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
	if imported, err := ms.importFromAntigravity(payload); err == nil {
		if ms.mergeImportedServers(payload, imported) {
			changed = true
		}
	}

	if imported, err := ms.importFromOpencode(payload); err == nil {
		if ms.mergeImportedServers(payload, imported) {
			changed = true
		}
	}

	if imported, err := ms.importFromGrok(payload); err == nil {
		if ms.mergeImportedServers(payload, imported) {
			changed = true
		}
	}

	if ms.reconcilePlatformsFromDisk(payload) {
		changed = true
	}

	// 清理不再存在于任何平台配置中的服务器
	if ms.cleanupDeletedServers(payload) {
		changed = true
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
			Headers:        cleanEnv(entry.Headers),
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
			// 解析失败时中止而非清空重建，避免误删 ~/.claude.json 中的其他配置（projects 历史等）
			return fmt.Errorf("解析 %s 失败，为保护原文件已中止同步: %v", path, err)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// 直接使用 desired，不保留任何现有服务器
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
			// 解析失败时中止而非清空重建，避免覆盖 config.toml 中的其他配置
			return fmt.Errorf("解析 %s 失败，为保护原文件已中止同步: %v", path, err)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	sanitizeCodexConfigPayload(payload)
	// 直接使用 desired，不保留任何现有服务器
	payload["mcp_servers"] = desired
	data, err := toml.Marshal(payload)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// syncAntigravityServers 同步到 Antigravity CLI MCP 配置（~/.gemini/config/mcp_config.json）
func (ms *MCPService) syncAntigravityServers(servers []MCPServer) error {
	path, err := antigravityMcpConfigPath()
	if err != nil {
		return err
	}

	desired := make(map[string]antigravityMcpServer)
	for _, server := range servers {
		if !platformContains(server.EnablePlatform, platAntigravity) {
			continue
		}
		desired[server.Name] = buildAntigravityEntry(server)
	}

	payload := make(map[string]any)
	existingServers := make(map[string]json.RawMessage)
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			payload = make(map[string]any)
		}
		// 读取现有的 mcpServers
		var mcpPayload claudeMcpFilePayload
		if err := json.Unmarshal(data, &mcpPayload); err == nil && len(mcpPayload.Servers) > 0 {
			existingServers = mcpPayload.Servers
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// 构建所有被管理的服务器集合
	managed := map[string]struct{}{}
	for _, server := range servers {
		name := strings.TrimSpace(server.Name)
		if name == "" {
			continue
		}
		managed[strings.ToLower(name)] = struct{}{}
	}

	// 合并服务器：保留外部手动添加的，添加/更新管理的
	merged := make(map[string]antigravityMcpServer)

	// 先添加现有的外部服务器
	for name, rawEntry := range existingServers {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		// 如果在管理列表中，跳过（由 desired 决定）
		if _, ok := managed[strings.ToLower(trimmed)]; ok {
			continue
		}
		// 解析并保留外部服务器
		var entry antigravityMcpServer
		if err := json.Unmarshal(rawEntry, &entry); err == nil {
			merged[name] = entry
		}
	}

	// 添加所有启用 Antigravity 的服务器
	for name, entry := range desired {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		merged[trimmed] = entry
	}

	payload["mcpServers"] = merged
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

		// Codex uses http_headers; accept the old headers key when importing.
		headers := make(map[string]string)
		for _, headerKey := range []string{"http_headers", "headers"} {
			if headerRaw, ok := entry[headerKey].(map[string]interface{}); ok {
				for k, v := range headerRaw {
					if s, ok := v.(string); ok {
						headers[k] = s
					}
				}
				if len(headers) > 0 {
					break
				}
			}
		}

		result[trimmedName] = rawMCPServer{
			Type:           typ,
			Command:        strings.TrimSpace(command),
			Args:           cleanArgs(args),
			Env:            cleanEnv(env),
			URL:            strings.TrimSpace(url),
			Headers:        cleanEnv(headers),
			EnablePlatform: []string{platCodex},
		}
	}
	return result, nil
}

// importFromAntigravity 从 Antigravity CLI 配置导入（新 mcp_config.json 优先，回退旧 settings.json）
func (ms *MCPService) importFromAntigravity(existing map[string]rawMCPServer) (map[string]rawMCPServer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	result := map[string]rawMCPServer{}
	for _, path := range antigravityMcpConfigCandidates(home) {
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			continue
		}
		var payload struct {
			Servers map[string]struct {
				claudeDesktopServer
				ServerURL string `json:"serverUrl"`
			} `json:"mcpServers"`
		}
		if json.Unmarshal(data, &payload) != nil {
			continue
		}
		for name, raw := range payload.Servers {
			trimmedName := strings.TrimSpace(name)
			if trimmedName == "" {
				continue
			}
			if _, exists := existing[trimmedName]; exists {
				continue
			}
			if _, exists := result[trimmedName]; exists {
				continue
			}

			entry := raw
			url := strings.TrimSpace(entry.URL)
			if url == "" {
				url = strings.TrimSpace(entry.ServerURL)
			}

			typeHint := entry.Type
			if strings.TrimSpace(typeHint) == "" {
				if url != "" {
					typeHint = "http"
				}
			}
			if strings.TrimSpace(typeHint) == "" {
				typeHint = "stdio"
			}

			typ := normalizeServerType(typeHint)
			if (typ == "http" || typ == "sse") && url == "" {
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
				URL:            url,
				EnablePlatform: []string{platAntigravity},
			}
		}
	}
	return result, nil
}

func (ms *MCPService) importFromOpencode(existing map[string]rawMCPServer) (map[string]rawMCPServer, error) {
	path := opencodeConfigFile(nil)
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
	payload, err := parseJSONLikeObject(data)
	if err != nil {
		return nil, err
	}
	mcp, _ := payload["mcp"].(map[string]any)
	result := make(map[string]rawMCPServer, len(mcp))
	for name, raw := range mcp {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			continue
		}
		if _, exists := existing[trimmedName]; exists {
			continue
		}
		entry, ok := parseOpencodeMcpEntry(raw)
		if !ok {
			continue
		}
		entry.EnablePlatform = []string{platOpencode}
		result[trimmedName] = entry
	}
	return result, nil
}

func parseOpencodeMcpEntry(raw any) (rawMCPServer, bool) {
	m, ok := raw.(map[string]any)
	if !ok || m == nil {
		return rawMCPServer{}, false
	}
	url := strings.TrimSpace(asString(m["url"]))
	typeHint := strings.ToLower(strings.TrimSpace(asString(m["type"])))
	cmdParts := anyToStringSlice(m["command"])
	command := ""
	args := []string{}
	if len(cmdParts) > 0 {
		command = cmdParts[0]
		args = cmdParts[1:]
	}
	if typeHint == "remote" || typeHint == "http" || typeHint == "sse" || url != "" {
		if url == "" {
			return rawMCPServer{}, false
		}
		typ := "http"
		if typeHint == "sse" {
			typ = "sse"
		}
		return rawMCPServer{
			Type:    typ,
			URL:     url,
			Headers: anyToStringMap(m["headers"]),
		}, true
	}
	if command == "" {
		return rawMCPServer{}, false
	}
	env := anyToStringMap(m["environment"])
	if len(env) == 0 {
		env = anyToStringMap(m["env"])
	}
	return rawMCPServer{
		Type:    "stdio",
		Command: command,
		Args:    cleanArgs(args),
		Env:     env,
	}, true
}

func (ms *MCPService) importFromGrok(existing map[string]rawMCPServer) (map[string]rawMCPServer, error) {
	path, err := grokMcpConfigPath()
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
		var args []string
		if argsRaw, ok := entry["args"].([]interface{}); ok {
			for _, arg := range argsRaw {
				if s, ok := arg.(string); ok {
					args = append(args, s)
				}
			}
		}
		env := make(map[string]string)
		if envRaw, ok := entry["env"].(map[string]interface{}); ok {
			for k, v := range envRaw {
				if s, ok := v.(string); ok {
					env[k] = s
				}
			}
		}
		headers := make(map[string]string)
		if headerRaw, ok := entry["headers"].(map[string]interface{}); ok {
			for k, v := range headerRaw {
				if s, ok := v.(string); ok {
					headers[k] = s
				}
			}
		}
		result[trimmedName] = rawMCPServer{
			Type:           typ,
			Command:        strings.TrimSpace(command),
			Args:           cleanArgs(args),
			Env:            cleanEnv(env),
			URL:            strings.TrimSpace(url),
			Headers:        cleanEnv(headers),
			EnablePlatform: []string{platGrok},
		}
	}
	return result, nil
}

func anyToStringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return cleanArgs(t)
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s := strings.TrimSpace(asString(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		if s := strings.TrimSpace(asString(v)); s != "" {
			return []string{s}
		}
	}
	return nil
}

func anyToStringMap(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, val := range m {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(asString(val))
	}
	return out
}

// mergeImportedServers 合并导入的服务器
func (ms *MCPService) mergeImportedServers(target, imported map[string]rawMCPServer) bool {
	changed := false
	for name, entry := range imported {
		entry = normalizeRawEntry(entry)
		if existing, ok := target[name]; ok {
			next := unionPlatforms(existing.EnablePlatform, entry.EnablePlatform)
			if samePlatforms(existing.EnablePlatform, next) {
				continue
			}
			existing.EnablePlatform = next
			target[name] = existing
			changed = true
			continue
		}
		target[name] = entry
		changed = true
	}
	return changed
}

func (ms *MCPService) reconcilePlatformsFromDisk(payload map[string]rawMCPServer) bool {
	claude := loadClaudeEnabledServers()
	codex := loadCodexEnabledServers()
	antigravity := loadAntigravityEnabledServers()
	opencode := loadOpencodeEnabledServers()
	grok := loadGrokEnabledServers()
	changed := false
	for name, entry := range payload {
		present := make([]string, 0, 5)
		if containsNormalized(claude, name) {
			present = append(present, platClaudeCode)
		}
		if containsNormalized(codex, name) {
			present = append(present, platCodex)
		}
		if containsNormalized(antigravity, name) {
			present = append(present, platAntigravity)
		}
		if containsNormalized(opencode, name) {
			present = append(present, platOpencode)
		}
		if containsNormalized(grok, name) {
			present = append(present, platGrok)
		}
		present = normalizePlatforms(present)
		if samePlatforms(entry.EnablePlatform, present) {
			continue
		}
		entry.EnablePlatform = present
		payload[name] = entry
		changed = true
	}
	return changed
}

// cleanupDeletedServers 清理已从所有平台配置中删除的服务器
func (ms *MCPService) cleanupDeletedServers(payload map[string]rawMCPServer) bool {
	// 获取所有平台当前的服务器列表
	claudeServers := ms.getCurrentClaudeServers()
	codexServers := ms.getCurrentCodexServers()
	antigravityServers := ms.getCurrentAntigravityServers()
	opencodeServers := loadOpencodeEnabledServers()
	grokServers := ms.getCurrentGrokServers()

	changed := false
	for name, entry := range payload {
		shouldDelete := true

		// 检查服务器是否在任何启用的平台中存在
		for _, platform := range entry.EnablePlatform {
			switch platform {
			case platClaudeCode:
				if _, exists := claudeServers[strings.ToLower(strings.TrimSpace(name))]; exists {
					shouldDelete = false
				}
			case platCodex:
				if _, exists := codexServers[strings.ToLower(strings.TrimSpace(name))]; exists {
					shouldDelete = false
				}
			case platAntigravity:
				if _, exists := antigravityServers[strings.ToLower(strings.TrimSpace(name))]; exists {
					shouldDelete = false
				}
			case platOpencode:
				if _, exists := opencodeServers[strings.ToLower(strings.TrimSpace(name))]; exists {
					shouldDelete = false
				}
			case platGrok:
				if _, exists := grokServers[strings.ToLower(strings.TrimSpace(name))]; exists {
					shouldDelete = false
				}
			}
		}

		// 如果服务器在所有启用的平台中都不存在，则删除
		if shouldDelete && len(entry.EnablePlatform) > 0 {
			delete(payload, name)
			changed = true
		}
	}
	return changed
}

// getCurrentClaudeServers 获取当前 Claude 配置中的服务器列表
func (ms *MCPService) getCurrentClaudeServers() map[string]struct{} {
	result := map[string]struct{}{}
	path, err := claudeConfigPath()
	if err != nil {
		return result
	}
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

// getCurrentCodexServers 获取当前 Codex 配置中的服务器列表
func (ms *MCPService) getCurrentCodexServers() map[string]struct{} {
	result := map[string]struct{}{}
	path, err := codexConfigPath()
	if err != nil {
		return result
	}
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

// getCurrentAntigravityServers 获取当前 Antigravity 配置中的服务器列表（新配置文件优先，回退旧 settings.json）
func (ms *MCPService) getCurrentAntigravityServers() map[string]struct{} {
	result := map[string]struct{}{}
	home, err := os.UserHomeDir()
	if err != nil {
		return result
	}
	for _, path := range antigravityMcpConfigCandidates(home) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var payload claudeMcpFilePayload
		if err := json.Unmarshal(data, &payload); err != nil {
			continue
		}
		for name := range payload.Servers {
			result[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
		}
	}
	return result
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
		// 旧平台名，归一到 antigravity
		return "antigravity", true
	case "antigravity":
		return "antigravity", true
	case "opencode", "openclaw":
		// openclaw 为旧值，归一到 opencode
		return "opencode", true
	case "grok":
		return "grok", true
	default:
		return "", false
	}
}

func unionPlatforms(primary, secondary []string) []string {
	combined := append([]string{}, primary...)
	combined = append(combined, secondary...)
	return normalizePlatforms(combined)
}

func samePlatforms(a, b []string) bool {
	left := normalizePlatforms(a)
	right := normalizePlatforms(b)
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]struct{}, len(left))
	for _, item := range left {
		seen[item] = struct{}{}
	}
	for _, item := range right {
		if _, ok := seen[item]; !ok {
			return false
		}
	}
	return true
}

func normalizeRawEntry(entry rawMCPServer) rawMCPServer {
	entry.Type = normalizeServerType(entry.Type)
	entry.Command = strings.TrimSpace(entry.Command)
	entry.URL = strings.TrimSpace(entry.URL)
	entry.Website = strings.TrimSpace(entry.Website)
	entry.Tips = strings.TrimSpace(entry.Tips)
	entry.Args = cleanArgs(entry.Args)
	entry.Env = cleanEnv(entry.Env)
	entry.Headers = cleanEnv(entry.Headers)
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

func loadAntigravityEnabledServers() map[string]struct{} {
	result := map[string]struct{}{}
	home, err := os.UserHomeDir()
	if err != nil {
		return result
	}
	for _, path := range antigravityMcpConfigCandidates(home) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var payload claudeMcpFilePayload
		if err := json.Unmarshal(data, &payload); err != nil {
			continue
		}
		for name := range payload.Servers {
			result[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
		}
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
		if len(server.Headers) > 0 {
			entry.Headers = server.Headers
		}
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

func buildMCPEntry(server MCPServer, headerKey string, includeType bool) map[string]any {
	entry := make(map[string]any)
	if includeType {
		entry["type"] = server.Type
	}
	if server.Type == "http" || server.Type == "sse" {
		entry["url"] = server.URL
		if len(server.Headers) > 0 {
			entry[headerKey] = server.Headers
		}
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

func buildCodexEntry(server MCPServer) map[string]any {
	return buildMCPEntry(server, "http_headers", false)
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

// antigravityMcpConfigPath Antigravity CLI 全局 MCP 配置：~/.gemini/config/mcp_config.json
func antigravityMcpConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, geminiDirName, "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "mcp_config.json"), nil
}

// antigravityMcpConfigCandidates 读取 Antigravity MCP 服务器时的候选文件（新位置优先，旧 settings.json 兜底）
func antigravityMcpConfigCandidates(home string) []string {
	return []string{
		filepath.Join(home, geminiDirName, "config", "mcp_config.json"),
		filepath.Join(home, geminiDirName, geminiConfigFile),
	}
}

// antigravityMcpServer Antigravity CLI（mcp_config.json）服务器格式：远程服务器用 serverUrl
type antigravityMcpServer struct {
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ServerURL string            `json:"serverUrl,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

func buildAntigravityEntry(server MCPServer) antigravityMcpServer {
	entry := antigravityMcpServer{}
	if server.Type == "http" || server.Type == "sse" {
		entry.ServerURL = server.URL
		if len(server.Headers) > 0 {
			entry.Headers = server.Headers
		}
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

func grokMcpConfigPath() (string, error) {
	dir := resolveGrokHome(nil)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, grokTomlName), nil
}

func (ms *MCPService) syncGrokServers(servers []MCPServer) error {
	path, err := grokMcpConfigPath()
	if err != nil {
		return err
	}

	desired := make(map[string]map[string]any)
	for _, server := range servers {
		if !platformContains(server.EnablePlatform, platGrok) {
			continue
		}
		desired[server.Name] = buildGrokMcpEntry(server)
	}

	payload := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := toml.Unmarshal(data, &payload); err != nil {
			return fmt.Errorf("解析 %s 失败，为保护原文件已中止同步: %v", path, err)
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

func (ms *MCPService) getCurrentGrokServers() map[string]struct{} {
	result := map[string]struct{}{}
	path, err := grokMcpConfigPath()
	if err != nil {
		return result
	}
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

func buildGrokMcpEntry(server MCPServer) map[string]any {
	entry := buildMCPEntry(server, "headers", true)
	entry["enabled"] = true
	return entry
}

func loadGrokEnabledServers() map[string]struct{} {
	result := map[string]struct{}{}
	path, err := grokMcpConfigPath()
	if err != nil {
		return result
	}
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

func loadOpencodeEnabledServers() map[string]struct{} {
	result := map[string]struct{}{}
	path := opencodeConfigFile(nil)
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	payload, err := parseJSONLikeObject(data)
	if err != nil {
		return result
	}
	mcp, _ := payload["mcp"].(map[string]any)
	for name := range mcp {
		trimmed := strings.ToLower(strings.TrimSpace(name))
		if trimmed != "" {
			result[trimmed] = struct{}{}
		}
	}
	return result
}

func (ms *MCPService) syncOpencodeServers(servers []MCPServer) error {
	path := opencodeConfigFile(nil)
	desired := map[string]any{}
	managed := map[string]struct{}{}
	for _, server := range servers {
		name := strings.TrimSpace(server.Name)
		if name == "" {
			continue
		}
		managed[strings.ToLower(name)] = struct{}{}
		if !platformContains(server.EnablePlatform, platOpencode) {
			continue
		}
		desired[server.Name] = buildOpencodeMcpEntry(server)
	}

	payload := map[string]any{}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if len(desired) == 0 {
			return nil
		}
	} else if len(data) > 0 {
		parsed, parseErr := parseJSONLikeObject(data)
		if parseErr != nil {
			return fmt.Errorf("解析 %s 失败，为保护原文件已中止同步: %v", path, parseErr)
		}
		payload = parsed
	}

	existMcp, _ := payload["mcp"].(map[string]any)
	if existMcp == nil {
		existMcp = map[string]any{}
	}
	for name := range existMcp {
		key := strings.ToLower(strings.TrimSpace(name))
		if _, ok := managed[key]; !ok {
			continue
		}
		keep := false
		for desiredName := range desired {
			if strings.EqualFold(desiredName, name) {
				keep = true
				break
			}
		}
		if !keep {
			delete(existMcp, name)
		}
	}
	for name, entry := range desired {
		existMcp[name] = entry
	}
	payload["mcp"] = existMcp
	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func buildOpencodeMcpEntry(server MCPServer) map[string]any {
	if server.Type == "http" || server.Type == "sse" {
		entry := map[string]any{
			"type":    "remote",
			"url":     server.URL,
			"enabled": true,
		}
		if len(server.Headers) > 0 {
			entry["headers"] = server.Headers
		}
		return entry
	}
	cmd := make([]string, 0, 1+len(server.Args))
	if strings.TrimSpace(server.Command) != "" {
		cmd = append(cmd, server.Command)
	}
	cmd = append(cmd, server.Args...)
	entry := map[string]any{
		"type":    "local",
		"command": cmd,
		"enabled": true,
	}
	if len(server.Env) > 0 {
		entry["environment"] = server.Env
	}
	return entry
}

func headerValues(headers map[string]string) []string {
	if len(headers) == 0 {
		return nil
	}
	out := make([]string, 0, len(headers))
	for _, v := range headers {
		out = append(out, v)
	}
	return out
}

func detectPlaceholders(url string, args []string, extra ...string) []string {
	set := make(map[string]struct{})
	collectPlaceholders(set, url)
	for _, arg := range args {
		collectPlaceholders(set, arg)
	}
	for _, item := range extra {
		collectPlaceholders(set, item)
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
			Headers:        cleanEnv(entry.Headers),
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
	if err := ms.syncAntigravityServers(servers); err != nil {
		return err
	}
	if err := ms.syncGrokServers(servers); err != nil {
		return err
	}
	if err := ms.syncOpencodeServers(servers); err != nil {
		return err
	}
	notifyCloudSync()
	return nil
}

// SyncToPlatforms 手动把中央存储的 MCP 配置强制重新下发到 Claude/Codex/Gemini，
// 用于平台配置被外部改动或损坏后恢复。
func (ms *MCPService) SyncToPlatforms() ([]MCPServer, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	config, err := ms.loadConfig()
	if err != nil {
		return nil, err
	}

	servers := ms.buildServersFromConfig(config)
	if err := ms.syncClaudeServers(servers); err != nil {
		return servers, err
	}
	if err := ms.syncCodexServers(servers); err != nil {
		return servers, err
	}
	if err := ms.syncAntigravityServers(servers); err != nil {
		return servers, err
	}
	if err := ms.syncGrokServers(servers); err != nil {
		return servers, err
	}
	if err := ms.syncOpencodeServers(servers); err != nil {
		return servers, err
	}
	return servers, nil
}

// ApplyToPlatform 把所有可写入的 MCP 一次性加入指定服务商配置。
func (ms *MCPService) ApplyToPlatform(platform string) (int, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	plat, ok := normalizePlatform(platform)
	if !ok {
		return 0, fmt.Errorf("未知平台")
	}

	config, err := ms.loadConfig()
	if err != nil {
		return 0, err
	}

	added := 0
	for name, entry := range config {
		entry = normalizeRawEntry(entry)
		placeholders := detectPlaceholders(entry.URL, entry.Args, headerValues(entry.Headers)...)
		if len(placeholders) > 0 {
			continue
		}
		if platformContains(entry.EnablePlatform, plat) {
			continue
		}
		entry.EnablePlatform = unionPlatforms(entry.EnablePlatform, []string{plat})
		config[name] = entry
		added++
	}
	if added == 0 {
		return 0, nil
	}
	if err := ms.saveConfig(config); err != nil {
		return 0, err
	}
	servers := ms.buildServersFromConfig(config)
	if err := ms.syncClaudeServers(servers); err != nil {
		return added, err
	}
	if err := ms.syncCodexServers(servers); err != nil {
		return added, err
	}
	if err := ms.syncAntigravityServers(servers); err != nil {
		return added, err
	}
	if err := ms.syncGrokServers(servers); err != nil {
		return added, err
	}
	if err := ms.syncOpencodeServers(servers); err != nil {
		return added, err
	}
	notifyCloudSync()
	return added, nil
}

// buildServersFromConfig 从配置构建服务器列表（内部使用，不加锁）
func (ms *MCPService) buildServersFromConfig(config map[string]rawMCPServer) []MCPServer {
	claudeEnabled := loadClaudeEnabledServers()
	codexEnabled := loadCodexEnabledServers()
	antigravityEnabled := loadAntigravityEnabledServers()
	opencodeEnabled := loadOpencodeEnabledServers()
	grokEnabled := loadGrokEnabledServers()

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
			Name:                 name,
			Type:                 typ,
			Command:              strings.TrimSpace(entry.Command),
			Args:                 cloneArgs(entry.Args),
			Env:                  cloneEnv(entry.Env),
			URL:                  strings.TrimSpace(entry.URL),
			Headers:              cloneEnv(entry.Headers),
			Website:              strings.TrimSpace(entry.Website),
			Tips:                 strings.TrimSpace(entry.Tips),
			EnablePlatform:       platforms,
			EnabledInClaude:      containsNormalized(claudeEnabled, name),
			EnabledInCodex:       containsNormalized(codexEnabled, name),
			EnabledInAntigravity: containsNormalized(antigravityEnabled, name),
			EnabledInOpencode:    containsNormalized(opencodeEnabled, name),
			EnabledInGrok:        containsNormalized(grokEnabled, name),
		}
		server.MissingPlaceholders = detectPlaceholders(server.URL, server.Args, headerValues(server.Headers)...)
		servers = append(servers, server)
	}

	return servers
}
