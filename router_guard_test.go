package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 网关会注入真实 API Key：来自网页的跨站请求、DNS 重绑定（Host 非回环）必须被拒，
// CLI 客户端（无 Origin / Sec-Fetch-Site、Host 为回环地址）必须照常放行
func TestGatewayGuard(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		headers map[string]string
		allowed bool
	}{
		{name: "CLI 直连 127.0.0.1", host: "127.0.0.1:8790", allowed: true},
		{name: "CLI 直连 localhost", host: "localhost:8790", allowed: true},
		{name: "IPv6 回环", host: "[::1]:8790", allowed: true},
		{name: "Node fetch 自带 sec-fetch-mode", host: "127.0.0.1:8790", headers: map[string]string{"Sec-Fetch-Mode": "cors"}, allowed: true},
		{name: "用户直接在地址栏打开", host: "127.0.0.1:8790", headers: map[string]string{"Sec-Fetch-Site": "none"}, allowed: true},
		{name: "本机页面同源", host: "localhost:8790", headers: map[string]string{"Origin": "http://localhost:5173"}, allowed: true},
		{name: "DNS 重绑定", host: "evil.example.com:8790", allowed: false},
		{name: "局域网 IP", host: "192.168.1.5:8790", allowed: false},
		{name: "跨站网页 POST", host: "127.0.0.1:8790", headers: map[string]string{"Origin": "https://evil.example.com"}, allowed: false},
		{name: "沙箱 iframe 的 null Origin", host: "127.0.0.1:8790", headers: map[string]string{"Origin": "null"}, allowed: false},
		{name: "no-cors GET 只带 Sec-Fetch-Site", host: "127.0.0.1:8790", headers: map[string]string{"Sec-Fetch-Site": "cross-site"}, allowed: false},
		{name: "同站子域", host: "127.0.0.1:8790", headers: map[string]string{"Sec-Fetch-Site": "same-site"}, allowed: false},
	}

	var reached bool
	handler := gatewayGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reached = false
			req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8790/claude/v1/messages", strings.NewReader(`{}`))
			req.Host = tc.host
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if tc.allowed {
				if !reached || rec.Code != http.StatusOK {
					t.Fatalf("应放行，实际状态 %d，body=%s", rec.Code, rec.Body.String())
				}
				return
			}
			if reached {
				t.Fatalf("应拦截，但请求进入了路由处理")
			}
			if rec.Code != http.StatusForbidden {
				t.Fatalf("拦截应返回 403，实际 %d", rec.Code)
			}
		})
	}
}
