package main

// 内置 MCP 工具：由本程序以 `mcp image` / `mcp search` 子命令作为 stdio MCP 服务器运行，
// 给 Claude Code、Codex 等 Agent 提供「生成图片」和「联网搜索」两个工具。
// 设置保存在 workbench.json 的 tools 字段，每次调用时读取，修改后无需重启 Agent。

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// BuiltinToolSettings 内置工具设置
type BuiltinToolSettings struct {
	ImageProvider  string `json:"image_provider,omitempty"` // 生成图片使用的环境（provider/name）
	ImageEnv       string `json:"image_env,omitempty"`
	ImageModel     string `json:"image_model,omitempty"`
	ImageSize      string `json:"image_size,omitempty"`
	SearchProvider string `json:"search_provider,omitempty"` // tavily | brave | exa | bocha
	SearchKey      string `json:"search_key,omitempty"`
}

const (
	builtinImageServer  = "aienv-image"
	builtinSearchServer = "aienv-search"
)

// ===== JSON-RPC over stdio =====

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpToolResult struct {
	Content []map[string]any `json:"content"`
	IsError bool             `json:"isError,omitempty"`
}

func textResult(text string, isErr bool) mcpToolResult {
	return mcpToolResult{Content: []map[string]any{{"type": "text", "text": text}}, IsError: isErr}
}

// serveMCP 运行一个最小的 MCP stdio 服务器
func serveMCP(name string, tools []mcpTool, call func(tool string, args map[string]any) mcpToolResult) error {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 64<<10), 16<<20)
	out := json.NewEncoder(os.Stdout)
	respond := func(id json.RawMessage, result any, rpcErr map[string]any) {
		msg := map[string]any{"jsonrpc": "2.0", "id": id}
		if rpcErr != nil {
			msg["error"] = rpcErr
		} else {
			msg["result"] = result
		}
		_ = out.Encode(msg)
	}
	for in.Scan() {
		var req mcpRequest
		if json.Unmarshal(in.Bytes(), &req) != nil || len(req.ID) == 0 {
			continue // 通知（notifications/*）不需要回复
		}
		switch req.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			_ = json.Unmarshal(req.Params, &p)
			version := p.ProtocolVersion
			if version == "" {
				version = "2025-06-18"
			}
			respond(req.ID, map[string]any{
				"protocolVersion": version,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": name, "version": appVersion},
			}, nil)
		case "ping":
			respond(req.ID, map[string]any{}, nil)
		case "tools/list":
			respond(req.ID, map[string]any{"tools": tools}, nil)
		case "tools/call":
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &p); err != nil {
				respond(req.ID, nil, map[string]any{"code": -32602, "message": "参数无效"})
				continue
			}
			respond(req.ID, call(p.Name, p.Arguments), nil)
		default:
			respond(req.ID, nil, map[string]any{"code": -32601, "message": "不支持的方法: " + req.Method})
		}
	}
	return in.Err()
}

func loadBuiltinTools() BuiltinToolSettings {
	wb, _ := loadWorkbenchLocked()
	return wb.Tools
}

func argString(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return strings.TrimSpace(v)
}

// ===== 生成图片 =====

var imageTools = []mcpTool{{
	Name: "generate_image",
	Description: "Generate an image from a text prompt with the image model configured in AI ENV, save it as a PNG file in the project " +
		"(generated-images/ unless path says otherwise) and return the file path. Describe the subject, style, composition and colors.",
	InputSchema: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{"type": "string", "description": "What to draw: subject, style, composition, colors."},
			"size":   map[string]any{"type": "string", "description": "Image size such as 1024x1024, 1536x1024 or 1024x1536. Default: the size set in AI ENV."},
			"path":   map[string]any{"type": "string", "description": "Where to save: a .png file or a folder, relative to the project or absolute. Existing files are never overwritten."},
		},
		"required": []string{"prompt"},
	},
}}

func imagesEndpoint(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/v1") {
		return base + "/images/generations"
	}
	return base + "/v1/images/generations"
}

var slugRE = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func imageTarget(cwd, path, prompt string) (string, error) {
	slug := strings.Trim(slugRE.ReplaceAllString(strings.ToLower(prompt), "-"), "-")
	if len(slug) > 40 {
		slug = slug[:40]
	}
	if slug == "" {
		slug = "image"
	}
	name := time.Now().Format("20060102-150405") + "-" + slug + ".png"
	target := filepath.Join(cwd, "generated-images", name)
	if path != "" {
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
		if strings.EqualFold(filepath.Ext(path), ".png") {
			target = path
		} else {
			target = filepath.Join(path, name)
		}
	}
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("文件已存在，不会覆盖: %s", target)
	}
	return target, os.MkdirAll(filepath.Dir(target), 0o755)
}

func generateImage(args map[string]any) mcpToolResult {
	prompt := argString(args, "prompt")
	if prompt == "" {
		return textResult("prompt 不能为空", true)
	}
	s := loadBuiltinTools()
	if s.ImageEnv == "" {
		return textResult("还没有设置生成图片使用的环境：在 AI ENV 的「MCP → 内置工具」中选择一个支持 OpenAI 图片接口的环境。", true)
	}
	app := NewApp()
	env, ok := app.findEnvCopy(s.ImageProvider, s.ImageEnv)
	if !ok {
		return textResult("AI ENV 中找不到设置的图片环境「"+s.ImageEnv+"」", true)
	}
	base, key, _ := upstreamVarsForEnv(&env)
	if validEndpoint(base) != nil || key == "" {
		return textResult("图片环境缺少有效的 Base URL 或 API Key", true)
	}
	model := s.ImageModel
	if model == "" {
		model = "gpt-image-1"
	}
	size := argString(args, "size")
	if size == "" {
		size = s.ImageSize
	}
	if size == "" {
		size = "1024x1024"
	}
	body := map[string]any{"model": model, "prompt": prompt, "n": 1, "size": size}
	if strings.HasPrefix(model, "dall-e") {
		body["response_format"] = "b64_json"
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, imagesEndpoint(base), bytes.NewReader(data))
	if err != nil {
		return textResult(err.Error(), true)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return textResult("请求图片接口失败: "+strings.ReplaceAll(err.Error(), key, "••••"), true)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if resp.StatusCode >= 300 {
		return textResult(fmt.Sprintf("图片接口返回 HTTP %d: %s", resp.StatusCode, clipText(strings.Join(strings.Fields(string(raw)), " "), 300)), true)
	}
	var parsed struct {
		Data []struct {
			B64     string `json:"b64_json"`
			URL     string `json:"url"`
			Revised string `json:"revised_prompt"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &parsed) != nil || len(parsed.Data) == 0 {
		return textResult("图片接口没有返回图片", true)
	}
	img := parsed.Data[0]
	var png []byte
	switch {
	case img.B64 != "":
		png, err = base64.StdEncoding.DecodeString(img.B64)
	case img.URL != "":
		png, err = downloadImage(img.URL)
	default:
		err = fmt.Errorf("图片接口没有返回图片数据")
	}
	if err != nil {
		return textResult(err.Error(), true)
	}
	cwd, _ := os.Getwd()
	target, err := imageTarget(cwd, argString(args, "path"), prompt)
	if err != nil {
		return textResult(err.Error(), true)
	}
	if err := os.WriteFile(target, png, 0o644); err != nil {
		return textResult("保存图片失败: "+err.Error(), true)
	}
	text := "Image saved: " + target
	if img.Revised != "" {
		text += "\nRevised prompt: " + img.Revised
	}
	result := textResult(text, false)
	if len(png) <= 1<<20 {
		result.Content = append(result.Content, map[string]any{"type": "image", "data": base64.StdEncoding.EncodeToString(png), "mimeType": "image/png"})
	}
	return result
}

func downloadImage(u string) ([]byte, error) {
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, fmt.Errorf("图片地址无效")
	}
	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Get(u)
	if err != nil {
		return nil, fmt.Errorf("下载图片失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("下载图片返回 HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

// ===== 联网搜索 =====

var searchTools = []mcpTool{{
	Name:        "web_search",
	Description: "Search the web with the search API configured in AI ENV and return the top results with titles, URLs and snippets.",
	InputSchema: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":       map[string]any{"type": "string", "description": "What to search for."},
			"max_results": map[string]any{"type": "integer", "minimum": 1, "maximum": 10, "description": "How many results to return (default 5)."},
		},
		"required": []string{"query"},
	},
}}

type searchHit struct {
	Title, URL, Snippet string
}

func runWebSearch(provider, key, query string, n int) ([]searchHit, error) {
	if key == "" {
		return nil, fmt.Errorf("还没有设置搜索 API Key：在 AI ENV 的「MCP → 内置工具」中填写")
	}
	if n <= 0 || n > 10 {
		n = 5
	}
	client := &http.Client{Timeout: 30 * time.Second}
	do := func(req *http.Request, dst any) error {
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("搜索请求失败: %v", strings.ReplaceAll(err.Error(), key, "••••"))
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		if resp.StatusCode >= 300 {
			return fmt.Errorf("搜索接口返回 HTTP %d: %s", resp.StatusCode, clipText(strings.Join(strings.Fields(string(raw)), " "), 200))
		}
		return json.Unmarshal(raw, dst)
	}
	postJSON := func(endpoint string, body any) (*http.Request, error) {
		data, _ := json.Marshal(body)
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req, err
	}
	hits := []searchHit{}
	switch provider {
	case "tavily":
		req, err := postJSON("https://api.tavily.com/search", map[string]any{"query": query, "max_results": n, "search_depth": "basic"})
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+key)
		var r struct {
			Results []struct {
				Title, URL, Content string
			} `json:"results"`
		}
		if err := do(req, &r); err != nil {
			return nil, err
		}
		for _, x := range r.Results {
			hits = append(hits, searchHit{x.Title, x.URL, x.Content})
		}
	case "brave":
		req, err := http.NewRequest(http.MethodGet, "https://api.search.brave.com/res/v1/web/search?count="+fmt.Sprint(n)+"&q="+url.QueryEscape(query), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Subscription-Token", key)
		var r struct {
			Web struct {
				Results []struct {
					Title       string `json:"title"`
					URL         string `json:"url"`
					Description string `json:"description"`
				} `json:"results"`
			} `json:"web"`
		}
		if err := do(req, &r); err != nil {
			return nil, err
		}
		for _, x := range r.Web.Results {
			hits = append(hits, searchHit{x.Title, x.URL, x.Description})
		}
	case "exa":
		req, err := postJSON("https://api.exa.ai/search", map[string]any{"query": query, "numResults": n, "contents": map[string]any{"text": map[string]any{"maxCharacters": 800}}})
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", key)
		var r struct {
			Results []struct {
				Title string `json:"title"`
				URL   string `json:"url"`
				Text  string `json:"text"`
			} `json:"results"`
		}
		if err := do(req, &r); err != nil {
			return nil, err
		}
		for _, x := range r.Results {
			hits = append(hits, searchHit{x.Title, x.URL, x.Text})
		}
	case "bocha":
		req, err := postJSON("https://api.bochaai.com/v1/web-search", map[string]any{"query": query, "count": n, "summary": true})
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+key)
		var r struct {
			Data struct {
				WebPages struct {
					Value []struct {
						Name    string `json:"name"`
						URL     string `json:"url"`
						Snippet string `json:"snippet"`
						Summary string `json:"summary"`
					} `json:"value"`
				} `json:"webPages"`
			} `json:"data"`
		}
		if err := do(req, &r); err != nil {
			return nil, err
		}
		for _, x := range r.Data.WebPages.Value {
			snippet := x.Summary
			if snippet == "" {
				snippet = x.Snippet
			}
			hits = append(hits, searchHit{x.Name, x.URL, snippet})
		}
	default:
		return nil, fmt.Errorf("还没有选择搜索服务：在 AI ENV 的「MCP → 内置工具」中选择 Tavily、Brave、Exa 或博查")
	}
	if len(hits) > n {
		hits = hits[:n]
	}
	return hits, nil
}

func formatSearchHits(query string, hits []searchHit) string {
	if len(hits) == 0 {
		return "No results for: " + query
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Results for: %s\n", query)
	for i, h := range hits {
		fmt.Fprintf(&b, "\n%d. %s\n   %s\n   %s\n", i+1, h.Title, h.URL, clipText(strings.Join(strings.Fields(h.Snippet), " "), 600))
	}
	return b.String()
}

func webSearch(args map[string]any) mcpToolResult {
	query := argString(args, "query")
	if query == "" {
		return textResult("query 不能为空", true)
	}
	n := 5
	if v, ok := args["max_results"].(float64); ok {
		n = int(v)
	}
	s := loadBuiltinTools()
	hits, err := runWebSearch(s.SearchProvider, s.SearchKey, query, n)
	if err != nil {
		return textResult(err.Error(), true)
	}
	return textResult(formatSearchHits(query, hits), false)
}

// runBuiltinMCP 子命令入口：mcp image / mcp search
func runBuiltinMCP(kind string) error {
	switch kind {
	case "image":
		return serveMCP(builtinImageServer, imageTools, func(tool string, args map[string]any) mcpToolResult {
			if tool != "generate_image" {
				return textResult("未知工具: "+tool, true)
			}
			return generateImage(args)
		})
	case "search":
		return serveMCP(builtinSearchServer, searchTools, func(tool string, args map[string]any) mcpToolResult {
			if tool != "web_search" {
				return textResult("未知工具: "+tool, true)
			}
			return webSearch(args)
		})
	}
	return fmt.Errorf("未知的内置 MCP：%s（可用：image、search）", kind)
}

// ===== 界面设置 =====

// BuiltinToolsInfo 内置工具设置与安装状态
type BuiltinToolsInfo struct {
	Settings        BuiltinToolSettings `json:"settings"`
	ImageInstalled  bool                `json:"image_installed"`
	SearchInstalled bool                `json:"search_installed"`
	Command         string              `json:"command"`
}

func (w *WorkbenchService) GetBuiltinTools() (BuiltinToolsInfo, error) {
	wb, err := loadWorkbenchLocked()
	if err != nil {
		return BuiltinToolsInfo{}, err
	}
	info := BuiltinToolsInfo{Settings: wb.Tools}
	info.Command, _ = currentExePath()
	if w.mcp != nil {
		if servers, err := w.mcp.ListServers(); err == nil {
			for _, s := range servers {
				switch s.Name {
				case builtinImageServer:
					info.ImageInstalled = true
				case builtinSearchServer:
					info.SearchInstalled = true
				}
			}
		}
	}
	return info, nil
}

func (w *WorkbenchService) SaveBuiltinTools(s BuiltinToolSettings) error {
	switch s.SearchProvider {
	case "", "tavily", "brave", "exa", "bocha":
	default:
		return fmt.Errorf("不支持的搜索服务")
	}
	if s.ImageEnv != "" && w.app != nil {
		if _, ok := w.app.findEnvCopy(s.ImageProvider, s.ImageEnv); !ok {
			return fmt.Errorf("图片环境不存在")
		}
	}
	s.ImageModel, s.ImageSize, s.SearchKey = strings.TrimSpace(s.ImageModel), strings.TrimSpace(s.ImageSize), strings.TrimSpace(s.SearchKey)
	if s.ImageSize != "" && !regexp.MustCompile(`^\d{3,4}x\d{3,4}$|^auto$`).MatchString(s.ImageSize) {
		return fmt.Errorf("图片尺寸格式应为 1024x1024")
	}
	workbenchMu.Lock()
	defer workbenchMu.Unlock()
	c, err := loadWorkbench()
	if err != nil {
		return err
	}
	c.Tools = s
	return saveWorkbench(c)
}

// InstallBuiltinMCP 把内置工具登记为 MCP 服务器（默认在 Claude Code 与 Codex 启用），之后可在 MCP 页调整
func (w *WorkbenchService) InstallBuiltinMCP(kind string) error {
	if w.mcp == nil {
		return fmt.Errorf("MCP 服务未就绪")
	}
	exe, err := currentExePath()
	if err != nil {
		return err
	}
	name, tips := builtinImageServer, "AI ENV 内置：生成图片并保存到项目"
	if kind == "search" {
		name, tips = builtinSearchServer, "AI ENV 内置：联网搜索"
	} else if kind != "image" {
		return fmt.Errorf("未知的内置工具")
	}
	servers, err := w.mcp.ListServers()
	if err != nil {
		return err
	}
	for _, s := range servers {
		if s.Name == name {
			return fmt.Errorf("「%s」已经添加过了，可在 MCP 页调整启用平台", name)
		}
	}
	return w.mcp.AddServers([]MCPServer{{
		Name: name, Type: "stdio", Command: exe, Args: []string{"mcp", kind}, Tips: tips,
		EnablePlatform: []string{"claude", "codex"}, EnabledInClaude: true, EnabledInCodex: true,
	}})
}

// TestWebSearch 用当前（未保存的）设置搜索一次
func (w *WorkbenchService) TestWebSearch(provider, key, query string) (string, error) {
	if strings.TrimSpace(query) == "" {
		query = "AI ENV"
	}
	hits, err := runWebSearch(provider, strings.TrimSpace(key), query, 3)
	if err != nil {
		return "", err
	}
	return formatSearchHits(query, hits), nil
}
