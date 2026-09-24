package main

// Claude Desktop 的 MCP 服务器写在 claude_desktop_config.json 的 mcpServers 里。
// 这个文件只认本地 stdio 服务器：远程（http/sse）服务器通过 mcp-remote 桥接成
// 一个 npx 进程；导入时再把本工具认得的桥接还原成远程服务器。

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	claudeDesktopMcpFile     = "claude_desktop_config.json"
	mcpRemotePackage         = "mcp-remote"
	mcpRemoteHeaderEnvPrefix = "MCP_REMOTE_HEADER_"
)

// claudeDesktopMcpConfigPath 返回 Claude Desktop 读取 MCP 的配置文件。
// 与环境配置不同：3P 版本的 configLibrary 只管推理网关，MCP 始终在这个文件里。
func claudeDesktopMcpConfigPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv("CLAUDE_DESKTOP_MCP_CONFIG")); override != "" {
		return override, nil
	}
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "Claude", claudeDesktopMcpFile), nil
	case "windows":
		// 微软商店（MSIX）版把 %APPDATA% 写入重定向到包目录；那里已有配置时必须写那份，
		// 否则应用读到的是虚拟化副本，改了真实 %APPDATA% 也不生效
		if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
			pattern := filepath.Join(local, "Packages", "Claude_*", "LocalCache", "Roaming", "Claude", claudeDesktopMcpFile)
			if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
				sort.Strings(matches)
				return matches[0], nil
			}
		}
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Claude", claudeDesktopMcpFile), nil
}

// buildClaudeDesktopMcpEntry 生成 Claude Desktop 的服务器条目
func buildClaudeDesktopMcpEntry(server MCPServer) map[string]any {
	if server.Type != "http" && server.Type != "sse" {
		entry := map[string]any{"command": server.Command}
		if len(server.Args) > 0 {
			entry["args"] = server.Args
		}
		if len(server.Env) > 0 {
			entry["env"] = server.Env
		}
		return entry
	}

	args := []string{"-y", mcpRemotePackage, server.URL}
	env := map[string]string{}
	names := make([]string, 0, len(server.Headers))
	for name := range server.Headers {
		names = append(names, name)
	}
	sort.Strings(names)
	for i, name := range names {
		// 头的值放进 env 再用 ${VAR} 引用：Claude Desktop 在 Windows 上调用 npx 时
		// 不转义参数里的空格，"Authorization: Bearer xxx" 直接写进 args 会被拆坏
		key := fmt.Sprintf("%s%d", mcpRemoteHeaderEnvPrefix, i+1)
		args = append(args, "--header", name+":${"+key+"}")
		env[key] = server.Headers[name]
	}
	if server.Type == "sse" {
		args = append(args, "--transport", "sse-only")
	}
	entry := map[string]any{"command": "npx", "args": args}
	if len(env) > 0 {
		entry["env"] = env
	}
	return entry
}

// parseMcpRemoteBridge 识别 mcp-remote 桥接条目并还原成远程服务器。
// 只还原完全认得的参数组合；带其它参数（OAuth 端口、--allow-http 等）的保持原样，
// 避免同步回去时丢掉用户手动加的选项。
func parseMcpRemoteBridge(command string, args []string, env map[string]string) (url string, headers map[string]string, sse bool, ok bool) {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(command)))
	base = strings.TrimSuffix(strings.TrimSuffix(base, ".cmd"), ".exe")
	if base != "npx" {
		return "", nil, false, false
	}
	i := 0
	for i < len(args) && (args[i] == "-y" || args[i] == "--yes") {
		i++
	}
	if i >= len(args) {
		return "", nil, false, false
	}
	pkg := args[i]
	if pkg != mcpRemotePackage && !strings.HasPrefix(pkg, mcpRemotePackage+"@") {
		return "", nil, false, false
	}
	i++
	if i >= len(args) {
		return "", nil, false, false
	}
	url = args[i]
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "", nil, false, false
	}
	i++
	headers = map[string]string{}
	for i < len(args) {
		switch args[i] {
		case "--header":
			if i+1 >= len(args) {
				return "", nil, false, false
			}
			name, value, found := strings.Cut(args[i+1], ":")
			name = strings.TrimSpace(name)
			if !found || name == "" {
				return "", nil, false, false
			}
			value = strings.TrimSpace(value)
			if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
				resolved, exists := env[value[2:len(value)-1]]
				if !exists {
					return "", nil, false, false
				}
				value = resolved
			}
			headers[name] = value
			i += 2
		case "--transport":
			if i+1 >= len(args) {
				return "", nil, false, false
			}
			switch args[i+1] {
			case "sse-only", "sse-first":
				sse = true
			case "http-only", "http-first":
			default:
				return "", nil, false, false
			}
			i += 2
		default:
			return "", nil, false, false
		}
	}
	// env 里只能有桥接自己的头变量，否则说明用户另有用途，按 stdio 原样保留
	for key := range env {
		if !strings.HasPrefix(key, mcpRemoteHeaderEnvPrefix) {
			return "", nil, false, false
		}
	}
	return url, headers, sse, true
}

// readClaudeDesktopMcpServers 读取 Claude Desktop 已配置的服务器（文件不存在时为空）
func readClaudeDesktopMcpServers() (map[string]claudeDesktopServer, error) {
	path, err := claudeDesktopMcpConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]claudeDesktopServer{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]claudeDesktopServer{}, nil
	}
	var payload struct {
		Servers map[string]claudeDesktopServer `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if payload.Servers == nil {
		payload.Servers = map[string]claudeDesktopServer{}
	}
	return payload.Servers, nil
}

// loadClaudeDesktopEnabledServers 读不了/解析不了时返回错误，调用方须跳过 reconcile
func loadClaudeDesktopEnabledServers() (map[string]struct{}, error) {
	result := map[string]struct{}{}
	servers, err := readClaudeDesktopMcpServers()
	if err != nil {
		return result, err
	}
	for name := range servers {
		result[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}
	return result, nil
}

// importFromClaudeDesktop 把 Claude Desktop 里有、中央存储里没有的服务器收进来
func (ms *MCPService) importFromClaudeDesktop(existing map[string]rawMCPServer) (map[string]rawMCPServer, error) {
	servers, err := readClaudeDesktopMcpServers()
	if err != nil {
		return nil, err
	}
	result := make(map[string]rawMCPServer, len(servers))
	for name, entry := range servers {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, exists := existing[trimmed]; exists {
			continue
		}
		if url, headers, sse, ok := parseMcpRemoteBridge(entry.Command, entry.Args, entry.Env); ok {
			typ := "http"
			if sse {
				typ = "sse"
			}
			result[trimmed] = rawMCPServer{
				Type:           typ,
				URL:            url,
				Headers:        cleanEnv(headers),
				EnablePlatform: []string{platClaudeDesktop},
			}
			continue
		}
		if strings.TrimSpace(entry.Command) == "" {
			continue
		}
		result[trimmed] = rawMCPServer{
			Type:           "stdio",
			Command:        strings.TrimSpace(entry.Command),
			Args:           cleanArgs(entry.Args),
			Env:            cleanEnv(entry.Env),
			EnablePlatform: []string{platClaudeDesktop},
		}
	}
	return result, nil
}

// syncClaudeDesktopServers 把启用了 Claude Desktop 的服务器写进 mcpServers，
// 保留文件里的其它设置（preferences 等）和不归本工具管理的服务器
func (ms *MCPService) syncClaudeDesktopServers(servers []MCPServer, removed map[string]struct{}) error {
	path, err := claudeDesktopMcpConfigPath()
	if err != nil {
		return err
	}

	desired := map[string]any{}
	managed := managedServerNames(servers, removed)
	for _, server := range servers {
		name := strings.TrimSpace(server.Name)
		if name != "" && platformContains(server.EnablePlatform, platClaudeDesktop) {
			desired[name] = buildClaudeDesktopMcpEntry(server)
		}
	}

	payload := map[string]any{}
	data, err := os.ReadFile(path)
	switch {
	case err == nil && len(strings.TrimSpace(string(data))) > 0:
		if err := json.Unmarshal(data, &payload); err != nil || payload == nil {
			return fmt.Errorf("解析 %s 失败，为保护原文件已中止同步: %v", path, err)
		}
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return err
	case err != nil && len(desired) == 0:
		// 没装 Claude Desktop 也没有要写的服务器：不凭空建配置文件
		return nil
	}

	merged := map[string]any{}
	if existing, ok := payload["mcpServers"].(map[string]any); ok {
		for name, entry := range existing {
			if _, isManaged := managed[strings.ToLower(strings.TrimSpace(name))]; isManaged {
				continue
			}
			merged[name] = entry
		}
	}
	for name, entry := range desired {
		merged[name] = entry
	}
	payload["mcpServers"] = merged

	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	backupFile(path)
	return writeFileAtomic(path, out, 0o600)
}
