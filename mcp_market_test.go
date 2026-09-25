package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestMCPProbeInitializesAndListsTools(t *testing.T) {
	var mu sync.Mutex
	methods := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if e := json.NewDecoder(r.Body).Decode(&req); e != nil {
			t.Error(e)
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		methods = append(methods, req.Method)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "session-test")
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{"tools": map[string]any{}}}})
		case "notifications/initialized":
			if r.Header.Get("Mcp-Session-Id") != "session-test" {
				t.Error("session header absent")
			}
			w.WriteHeader(202)
		case "tools/list":
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"tools": []map[string]string{{"name": "read_file"}, {"name": "search"}}}})
		default:
			t.Errorf("unexpected method %s", req.Method)
		}
	}))
	defer srv.Close()
	r := probeMCP(MCPServer{Type: "http", URL: srv.URL})
	if !r.Success || len(r.Tools) != 2 || r.Protocol != "2025-11-25" {
		t.Fatalf("probe failed: %+v", r)
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(methods, ",") != "initialize,notifications/initialized,tools/list" {
		t.Fatalf("not an MCP handshake: %v", methods)
	}
}
func TestMCPProbeRejectsGenericHTTP200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"ok"}`)) }))
	defer srv.Close()
	if r := probeMCP(MCPServer{Type: "http", URL: srv.URL}); r.Success {
		t.Fatal("generic HTTP response reported as MCP success")
	}
}

func TestParseMcpRegistryServerURLPackageKeepsHint(t *testing.T) {
	raw := []byte(`{
		"name": "io.example/remote-only",
		"packages": [
			{"registryType": "oci", "identifier": "ghcr.io/example/remote", "transport": {"type": "streamable-http", "url": "https://mcp.example.com/mcp"}}
		]
	}`)
	item, ok := parseMcpRegistryServer(raw)
	if !ok {
		t.Fatal("解析失败")
	}
	if item.Type != "http" || item.URL != "https://mcp.example.com/mcp" {
		t.Fatalf("type/url = %q/%q", item.Type, item.URL)
	}
	if item.Hint != item.URL {
		t.Fatalf("hint 应显示远程地址，实际 %q", item.Hint)
	}
}

func TestParseMcpRegistryServerNpmHint(t *testing.T) {
	raw := []byte(`{"name": "io.example/npm", "packages": [{"registryType": "npm", "identifier": "@example/mcp"}]}`)
	item, ok := parseMcpRegistryServer(raw)
	if !ok {
		t.Fatal("解析失败")
	}
	if item.Command != "npx" || strings.Join(item.Args, " ") != "-y @example/mcp" {
		t.Fatalf("command/args = %q %v", item.Command, item.Args)
	}
	if item.Hint != "npx -y @example/mcp" {
		t.Fatalf("hint = %q", item.Hint)
	}
}
