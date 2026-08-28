package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const mcpRegistryBase = "https://registry.modelcontextprotocol.io"

// McpMarketItem 官方 MCP Registry 里的一条可导入服务器
type McpMarketItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Website     string   `json:"website"`
	Version     string   `json:"version"`
	Type        string   `json:"type"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	URL         string   `json:"url"`
	Hint        string   `json:"hint"`
}

// McpMarketPage 市场分页
type McpMarketPage struct {
	Items   []McpMarketItem `json:"items"`
	Next    string          `json:"next"`
	Warning string          `json:"warning"`
}

func (ms *MCPService) SearchMcpMarketplace(query, cursor string) (McpMarketPage, error) {
	query = strings.TrimSpace(query)
	cursor = strings.TrimSpace(cursor)
	page, err := fetchMcpRegistry(query, cursor)
	if err != nil {
		fallback := filterMcpCurated(query)
		if cursor != "" {
			return McpMarketPage{Warning: err.Error()}, nil
		}
		return McpMarketPage{Items: fallback, Warning: "官方市场暂不可用，已显示常用 MCP。 " + err.Error()}, nil
	}
	if len(page.Items) == 0 && cursor == "" && query != "" {
		page.Items = filterMcpCurated(query)
		if len(page.Items) > 0 {
			page.Warning = "官方结果为空，已附带常用 MCP"
		}
	}
	return page, nil
}

func (ms *MCPService) ImportMcpMarketplace(id string, platforms []string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("未选择 MCP")
	}
	item, err := resolveMcpMarketItem(id)
	if err != nil {
		return err
	}
	server, err := mcpMarketToServer(item, platforms)
	if err != nil {
		return err
	}
	return ms.AddServers([]MCPServer{server})
}

func fetchMcpRegistry(query, cursor string) (McpMarketPage, error) {
	params := url.Values{}
	params.Set("limit", "30")
	if query != "" {
		params.Set("search", query)
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	rawURL := mcpRegistryBase + "/v0.1/servers?" + params.Encode()
	data, err := marketHTTPGet(rawURL, 15*time.Second)
	if err != nil {
		data, err = marketHTTPGet(mcpRegistryBase+"/v0/servers?"+params.Encode(), 15*time.Second)
		if err != nil {
			return McpMarketPage{}, err
		}
	}
	return parseMcpRegistryList(data)
}

func parseMcpRegistryList(data []byte) (McpMarketPage, error) {
	var payload struct {
		Servers  json.RawMessage `json:"servers"`
		Metadata struct {
			NextCursor string `json:"nextCursor"`
			Next       string `json:"next"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return McpMarketPage{}, err
	}
	page := McpMarketPage{Next: firstNonEmpty(payload.Metadata.NextCursor, payload.Metadata.Next)}
	if len(payload.Servers) == 0 {
		return page, nil
	}
	var wrapped []struct {
		Server json.RawMessage `json:"server"`
	}
	if err := json.Unmarshal(payload.Servers, &wrapped); err == nil && len(wrapped) > 0 && len(wrapped[0].Server) > 0 {
		for _, row := range wrapped {
			if item, ok := parseMcpRegistryServer(row.Server); ok {
				page.Items = append(page.Items, item)
			}
		}
		return page, nil
	}
	var direct []json.RawMessage
	if err := json.Unmarshal(payload.Servers, &direct); err != nil {
		return McpMarketPage{}, err
	}
	for _, row := range direct {
		if item, ok := parseMcpRegistryServer(row); ok {
			page.Items = append(page.Items, item)
		}
	}
	return page, nil
}

func parseMcpRegistryServer(raw json.RawMessage) (McpMarketItem, bool) {
	var body struct {
		Name        string `json:"name"`
		Title       string `json:"title"`
		Description string `json:"description"`
		WebsiteURL  string `json:"websiteUrl"`
		Repository  struct {
			URL string `json:"url"`
		} `json:"repository"`
		Version  string `json:"version"`
		Packages []struct {
			RegistryType string `json:"registryType"`
			Identifier   string `json:"identifier"`
			Transport    struct {
				Type string `json:"type"`
				URL  string `json:"url"`
			} `json:"transport"`
		} `json:"packages"`
		Remotes []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"remotes"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return McpMarketItem{}, false
	}
	id := strings.TrimSpace(body.Name)
	if id == "" {
		return McpMarketItem{}, false
	}
	item := McpMarketItem{
		ID:          id,
		Name:        slugMarketName(lastPathSegment(id)),
		Title:       firstNonEmpty(strings.TrimSpace(body.Title), lastPathSegment(id)),
		Description: strings.TrimSpace(body.Description),
		Website:     firstNonEmpty(strings.TrimSpace(body.WebsiteURL), strings.TrimSpace(body.Repository.URL)),
		Version:     strings.TrimSpace(body.Version),
	}
	for _, remote := range body.Remotes {
		if strings.TrimSpace(remote.URL) == "" {
			continue
		}
		item.Type = "http"
		if strings.EqualFold(remote.Type, "sse") {
			item.Type = "sse"
		}
		item.URL = strings.TrimSpace(remote.URL)
		item.Hint = item.URL
		break
	}
	if item.URL == "" {
		for _, pkg := range body.Packages {
			ident := strings.TrimSpace(pkg.Identifier)
			if ident == "" {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(pkg.RegistryType)) {
			case "npm":
				item.Type = "stdio"
				item.Command = "npx"
				item.Args = []string{"-y", ident}
			case "pypi":
				item.Type = "stdio"
				item.Command = "uvx"
				item.Args = []string{ident}
			default:
				if strings.TrimSpace(pkg.Transport.URL) != "" {
					item.Type = "http"
					item.URL = strings.TrimSpace(pkg.Transport.URL)
					item.Hint = item.URL
					break
				}
				continue
			}
			item.Hint = strings.TrimSpace(item.Command + " " + strings.Join(item.Args, " "))
			break
		}
	}
	if item.Type == "" {
		item.Type = "stdio"
	}
	return item, true
}

func resolveMcpMarketItem(id string) (McpMarketItem, error) {
	for _, item := range curatedMcpServers() {
		if item.ID == id {
			return item, nil
		}
	}
	encoded := url.PathEscape(id)
	data, err := marketHTTPGet(mcpRegistryBase+"/v0.1/servers/"+encoded+"/versions/latest", 15*time.Second)
	if err != nil {
		data, err = marketHTTPGet(mcpRegistryBase+"/v0.1/servers/"+encoded, 15*time.Second)
	}
	if err != nil {
		return McpMarketItem{}, fmt.Errorf("读取 MCP 详情失败: %v", err)
	}
	var wrapped struct {
		Server json.RawMessage `json:"server"`
	}
	raw := data
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Server) > 0 {
		raw = wrapped.Server
	}
	item, ok := parseMcpRegistryServer(raw)
	if !ok {
		return McpMarketItem{}, fmt.Errorf("无法解析 MCP 详情")
	}
	return item, nil
}

func mcpMarketToServer(item McpMarketItem, platforms []string) (MCPServer, error) {
	name := slugMarketName(item.Name)
	if name == "" || name == "item" {
		name = slugMarketName(item.Title)
	}
	if name == "" {
		return MCPServer{}, fmt.Errorf("名称无效")
	}
	plats := normalizePlatforms(platforms)
	if len(plats) == 0 {
		plats = []string{platClaudeCode, platCodex, platGemini, platOpencode, platGrok}
	}
	server := MCPServer{
		Name:           name,
		Type:           normalizeServerType(item.Type),
		Command:        strings.TrimSpace(item.Command),
		Args:           cleanArgs(item.Args),
		URL:            strings.TrimSpace(item.URL),
		Website:        strings.TrimSpace(item.Website),
		Tips:           strings.TrimSpace(item.Description),
		EnablePlatform: plats,
	}
	if (server.Type == "http" || server.Type == "sse") && server.URL == "" {
		return MCPServer{}, fmt.Errorf("%s 没有可用的远程地址", item.Title)
	}
	if server.Type == "stdio" && server.Command == "" {
		return MCPServer{}, fmt.Errorf("%s 没有可用的安装命令", item.Title)
	}
	return server, nil
}

func lastPathSegment(name string) string {
	name = strings.TrimSpace(name)
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func filterMcpCurated(query string) []McpMarketItem {
	out := make([]McpMarketItem, 0)
	for _, item := range curatedMcpServers() {
		blob := item.Name + " " + item.Title + " " + item.Description + " " + item.Hint
		if containsFold(blob, query) {
			out = append(out, item)
		}
	}
	return out
}

func curatedMcpServers() []McpMarketItem {
	stdio := func(id, title, desc, pkg string) McpMarketItem {
		return McpMarketItem{
			ID: id, Name: id, Title: title, Description: desc,
			Type: "stdio", Command: "npx", Args: []string{"-y", pkg},
			Hint: "npx -y " + pkg, Website: "https://github.com/modelcontextprotocol",
		}
	}
	uvx := func(id, title, desc, pkg string) McpMarketItem {
		return McpMarketItem{
			ID: id, Name: id, Title: title, Description: desc,
			Type: "stdio", Command: "uvx", Args: []string{pkg},
			Hint: "uvx " + pkg,
		}
	}
	return []McpMarketItem{
		stdio("filesystem", "Filesystem", "本地文件读写", "@modelcontextprotocol/server-filesystem"),
		stdio("github", "GitHub", "仓库、issue、PR", "@modelcontextprotocol/server-github"),
		stdio("memory", "Memory", "跨会话记忆", "@modelcontextprotocol/server-memory"),
		stdio("sequential-thinking", "Sequential Thinking", "分步推理", "@modelcontextprotocol/server-sequential-thinking"),
		stdio("puppeteer", "Puppeteer", "浏览器自动化", "@modelcontextprotocol/server-puppeteer"),
		stdio("brave-search", "Brave Search", "网页搜索", "@modelcontextprotocol/server-brave-search"),
		stdio("context7", "Context7", "库文档检索", "@upstash/context7-mcp"),
		stdio("playwright", "Playwright", "浏览器 MCP", "@playwright/mcp"),
		stdio("exa", "Exa Search", "Exa 搜索", "exa-mcp-server"),
		uvx("fetch", "Fetch", "抓取网页", "mcp-server-fetch"),
		uvx("time", "Time", "时区与时间", "mcp-server-time"),
		uvx("git", "Git", "本地 git 操作", "mcp-server-git"),
	}
}
