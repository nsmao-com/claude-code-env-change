package main

import (
	"strings"
	"testing"
)

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
