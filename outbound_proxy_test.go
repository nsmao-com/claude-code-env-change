package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// 切换出站代理与正在进行的请求并发，不能有数据竞争（go test -race 覆盖）；
// 所有默认 client 都应经过可切换的 Transport
func TestOutboundProxySwitchIsRaceFree(t *testing.T) {
	withHomeRoot(t)
	if _, ok := http.DefaultTransport.(*switchableTransport); !ok {
		t.Fatalf("http.DefaultTransport 应为可切换的 Transport，实际 %T", http.DefaultTransport)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				resp, err := http.Get(server.URL) // 本机地址始终直连
				if err != nil {
					t.Errorf("请求失败: %v", err)
					return
				}
				resp.Body.Close()
			}
		}()
	}
	for i := 0; i < 20; i++ {
		enabled := i%2 == 0
		if err := applyOutboundProxy(OutboundProxySettings{Enabled: enabled, URL: "http://127.0.0.1:1"}); err != nil {
			t.Fatalf("切换代理失败: %v", err)
		}
	}
	wg.Wait()
	_ = applyOutboundProxy(OutboundProxySettings{})
}
