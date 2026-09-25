package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

type rpcReply struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
type mcpProbeTransport struct {
	ctx                         context.Context
	server                      MCPServer
	client                      *http.Client
	session, protocol, endpoint string
	in                          io.WriteCloser
	replies                     chan []byte
	failures                    chan error
	close                       func()
}

func probeMCP(server MCPServer) (r MCPTestResult) {
	start := time.Now()
	r = MCPTestResult{Stage: "connect", Tools: []string{}}
	defer func() { r.Latency = time.Since(start).Milliseconds() }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	t, err := newMCPProbe(ctx, server)
	if err != nil {
		r.Message = err.Error()
		r.Latency = time.Since(start).Milliseconds()
		return r
	}
	defer t.close()
	defer func() { cancel() }()
	init, err := t.call(1, "initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "AI ENV diagnostics", "version": appVersion}})
	if err != nil {
		r.Message = "初始化失败: " + err.Error()
		r.Latency = time.Since(start).Milliseconds()
		return r
	}
	var initialized struct {
		Protocol     string                     `json:"protocolVersion"`
		Capabilities map[string]json.RawMessage `json:"capabilities"`
	}
	if err = json.Unmarshal(init, &initialized); err != nil || initialized.Protocol == "" {
		r.Message = "服务器未返回有效的 MCP 初始化结果"
		return r
	}
	t.protocol = initialized.Protocol
	r.Protocol = t.protocol
	r.Stage = "initialize"
	if _, err = t.call(0, "notifications/initialized", nil); err != nil {
		r.Message = "初始化通知失败: " + err.Error()
		return r
	}
	if _, ok := initialized.Capabilities["tools"]; ok {
		cursor := ""
		seen := map[string]bool{}
		for page := 0; page < 50; page++ {
			params := map[string]any{}
			if cursor != "" {
				params["cursor"] = cursor
			}
			b, e := t.call(page+2, "tools/list", params)
			if e != nil {
				r.Message = "读取工具列表失败: " + e.Error()
				return r
			}
			var list struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
				Cursor string `json:"nextCursor"`
			}
			if e = json.Unmarshal(b, &list); e != nil {
				r.Message = "工具列表格式无效"
				return r
			}
			for _, tool := range list.Tools {
				if tool.Name != "" {
					r.Tools = append(r.Tools, tool.Name)
				}
			}
			if list.Cursor == "" {
				break
			}
			if seen[list.Cursor] || page == 49 {
				r.Message = "工具列表分页异常或超过 50 页"
				return r
			}
			seen[list.Cursor] = true
			cursor = list.Cursor
		}
	}
	r.Success = true
	r.Stage = "tools"
	r.Latency = time.Since(start).Milliseconds()
	r.Message = fmt.Sprintf("MCP 握手通过，可读取 %d 个工具", len(r.Tools))
	return r
}
func newMCPProbe(ctx context.Context, s MCPServer) (*mcpProbeTransport, error) {
	t := &mcpProbeTransport{ctx: ctx, server: s, client: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, endpoint: s.URL, replies: make(chan []byte, 16), failures: make(chan error, 1), close: func() {}}
	if s.Type == "http" || s.Type == "sse" {
		u, e := url.Parse(s.URL)
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, fmt.Errorf("请输入有效的 HTTP(S) MCP 地址")
		}
		if s.Type == "sse" {
			if e = t.startSSE(); e != nil {
				return nil, e
			}
		}
		return t, nil
	}
	bin, e := exec.LookPath(s.Command)
	if e != nil {
		return nil, fmt.Errorf("未找到 %s，请先安装对应运行环境", s.Command)
	}
	cmd := startToolCommand(ctx, bin, s.Args, nil)
	env := map[string]string{}
	for _, pair := range os.Environ() {
		k, v, ok := strings.Cut(pair, "=")
		if ok {
			env[k] = v
		}
	}
	for k, v := range s.Env {
		env[k] = v
	}
	cmd.Env = []string{}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	in, e := cmd.StdinPipe()
	if e != nil {
		return nil, e
	}
	out, e := cmd.StdoutPipe()
	if e != nil {
		in.Close()
		return nil, e
	}
	cmd.Stderr = io.Discard
	if e = cmd.Start(); e != nil {
		in.Close()
		out.Close()
		return nil, e
	}
	t.in = in
	t.close = func() { in.Close(); killCmd(cmd); _ = cmd.Wait() }
	go func() {
		scanner := bufio.NewScanner(out)
		scanner.Buffer(make([]byte, 4096), 4<<20)
		for scanner.Scan() {
			data := append([]byte(nil), scanner.Bytes()...)
			select {
			case t.replies <- data:
			case <-ctx.Done():
				return
			}
		}
		e := scanner.Err()
		if e == nil {
			e = fmt.Errorf("MCP 进程已退出或关闭输出")
		}
		select {
		case t.failures <- e:
		default:
		}
	}()
	return t, nil
}
func (t *mcpProbeTransport) startSSE() error {
	req, e := http.NewRequestWithContext(t.ctx, "GET", t.server.URL, nil)
	if e != nil {
		return e
	}
	for k, v := range t.server.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, e := t.client.Do(req)
	if e != nil {
		return e
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return fmt.Errorf("SSE 连接失败 (HTTP %d)", resp.StatusCode)
	}
	endpoint := make(chan string, 1)
	t.close = func() { resp.Body.Close() }
	go func() {
		e := readMCPEvents(resp.Body, func(event, data string) bool {
			if event == "endpoint" {
				select {
				case endpoint <- data:
				default:
				}
			} else if event == "message" || event == "" {
				select {
				case t.replies <- []byte(data):
				case <-t.ctx.Done():
					return false
				}
			}
			return true
		})
		if e == nil {
			e = io.EOF
		}
		select {
		case t.failures <- e:
		default:
		}
	}()
	select {
	case raw := <-endpoint:
		base, _ := url.Parse(t.server.URL)
		rel, e := url.Parse(raw)
		if e != nil {
			return e
		}
		target := base.ResolveReference(rel)
		if !strings.EqualFold(target.Host, base.Host) || target.Scheme != base.Scheme {
			t.close()
			return fmt.Errorf("拒绝向不同来源发送 MCP 凭证")
		}
		t.endpoint = target.String()
		return nil
	case e := <-t.failures:
		t.close()
		return e
	case <-t.ctx.Done():
		t.close()
		return t.ctx.Err()
	}
}
func readMCPEvents(reader io.Reader, handle func(string, string) bool) error {
	s := bufio.NewScanner(reader)
	s.Buffer(make([]byte, 4096), 4<<20)
	event := ""
	data := []string{}
	for s.Scan() {
		line := s.Text()
		if line == "" {
			if len(data) > 0 && !handle(event, strings.Join(data, "\n")) {
				return nil
			}
			event = ""
			data = nil
			continue
		}
		if v, ok := strings.CutPrefix(line, "event:"); ok {
			event = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "data:"); ok {
			data = append(data, strings.TrimPrefix(v, " "))
		}
	}
	if len(data) > 0 {
		handle(event, strings.Join(data, "\n"))
	}
	return s.Err()
}
func parseRPCResponse(data []byte, id int) (json.RawMessage, bool, error) {
	var r rpcReply
	if e := json.Unmarshal(data, &r); e != nil {
		return nil, false, e
	}
	if string(r.ID) != fmt.Sprint(id) {
		return nil, false, nil
	}
	if r.Error != nil {
		return nil, true, fmt.Errorf("JSON-RPC %d: %s", r.Error.Code, r.Error.Message)
	}
	if len(r.Result) == 0 {
		return nil, true, fmt.Errorf("缺少 JSON-RPC result")
	}
	return r.Result, true, nil
}
func (t *mcpProbeTransport) call(id int, method string, params any) (json.RawMessage, error) {
	p := map[string]any{"jsonrpc": "2.0", "method": method}
	if id > 0 {
		p["id"] = id
	}
	if params != nil {
		p["params"] = params
	}
	data, e := json.Marshal(p)
	if e != nil {
		return nil, e
	}
	if t.in != nil {
		if _, e = t.in.Write(append(data, '\n')); e != nil {
			return nil, e
		}
	} else {
		req, e := http.NewRequestWithContext(t.ctx, "POST", t.endpoint, bytes.NewReader(data))
		if e != nil {
			return nil, e
		}
		for k, v := range t.server.Headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Mcp-Method", method)
		if t.session != "" {
			req.Header.Set("Mcp-Session-Id", t.session)
		}
		if t.protocol != "" {
			req.Header.Set("MCP-Protocol-Version", t.protocol)
		}
		resp, e := t.client.Do(req)
		if e != nil {
			return nil, e
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("HTTP %d（请检查地址、请求头与鉴权）", resp.StatusCode)
		}
		if session := resp.Header.Get("Mcp-Session-Id"); session != "" {
			t.session = session
		}
		if id == 0 {
			return nil, nil
		}
		if t.server.Type != "sse" {
			if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
				var result json.RawMessage
				var responseErr error
				found := false
				e = readMCPEvents(resp.Body, func(_, event string) bool {
					var err error
					result, found, err = parseRPCResponse([]byte(event), id)
					if err != nil {
						responseErr = err
						return false
					}
					return !found
				})
				if responseErr != nil {
					return nil, responseErr
				}
				if e != nil {
					return nil, e
				}
				if !found {
					return nil, fmt.Errorf("SSE 未返回对应响应")
				}
				return result, nil
			}
			b, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
			if e != nil {
				return nil, e
			}
			result, found, e := parseRPCResponse(b, id)
			if e != nil {
				return nil, e
			}
			if !found {
				return nil, fmt.Errorf("MCP 响应 ID 不匹配")
			}
			return result, nil
		}
	}
	if id == 0 {
		return nil, nil
	}
	for {
		select {
		case b := <-t.replies:
			r, found, e := parseRPCResponse(b, id)
			if e != nil {
				return nil, e
			}
			if found {
				return r, nil
			}
		case e := <-t.failures:
			return nil, e
		case <-t.ctx.Done():
			return nil, fmt.Errorf("握手超时，请检查服务日志与凭证")
		}
	}
}
