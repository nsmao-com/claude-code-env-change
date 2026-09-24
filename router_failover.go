package main

// 路由多上游故障转移：一条路由除主上游外可配置若干备用上游（同一协议，
// 各自的 Base URL 与 Key）。请求在收到响应头之前失败，或上游返回
// 限流/鉴权/额度/服务端错误时，换下一个上游重发；失败过的上游冷却一段时间，
// 冷却期内排到队尾，避免每个请求都先撞一次坏节点。

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const upstreamCooldown = 60 * time.Second

// RouteUpstream 备用上游
type RouteUpstream struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key,omitempty"`
}

type upstreamHealth struct {
	mu        sync.Mutex
	coolUntil map[string]time.Time
}

var routeUpstreamHealth = &upstreamHealth{coolUntil: map[string]time.Time{}}

func upstreamHealthKey(routeName string, up RouteUpstream) string {
	return strings.ToLower(routeName) + "\x00" + up.BaseURL + "\x00" + up.APIKey
}

func (h *upstreamHealth) markFailed(routeName string, up RouteUpstream) {
	h.mu.Lock()
	h.coolUntil[upstreamHealthKey(routeName, up)] = time.Now().Add(upstreamCooldown)
	h.mu.Unlock()
}

func (h *upstreamHealth) markOK(routeName string, up RouteUpstream) {
	h.mu.Lock()
	delete(h.coolUntil, upstreamHealthKey(routeName, up))
	h.mu.Unlock()
}

func (h *upstreamHealth) cooling(routeName string, up RouteUpstream, now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	until, ok := h.coolUntil[upstreamHealthKey(routeName, up)]
	return ok && now.Before(until)
}

// upstreams 主上游在前、备用按配置顺序在后
func (rt *APIRoute) upstreams() []RouteUpstream {
	list := make([]RouteUpstream, 0, 1+len(rt.Fallbacks))
	list = append(list, RouteUpstream{BaseURL: rt.BaseURL, APIKey: rt.APIKey})
	for _, fb := range rt.Fallbacks {
		if strings.TrimSpace(fb.BaseURL) != "" {
			list = append(list, fb)
		}
	}
	return list
}

// orderedUpstreams 冷却中的上游挪到队尾（全部在冷却时仍按原顺序尝试）
func orderedUpstreams(route APIRoute, now time.Time) []RouteUpstream {
	all := route.upstreams()
	ready := make([]RouteUpstream, 0, len(all))
	cooling := make([]RouteUpstream, 0)
	for _, up := range all {
		if routeUpstreamHealth.cooling(route.Name, up, now) {
			cooling = append(cooling, up)
		} else {
			ready = append(ready, up)
		}
	}
	return append(ready, cooling...)
}

// withUpstream 返回把主上游替换成 up 的路由副本，复用现有的鉴权与拼 URL 逻辑
func (rt APIRoute) withUpstream(up RouteUpstream) APIRoute {
	rt.BaseURL = up.BaseURL
	rt.APIKey = up.APIKey
	return rt
}

// isFailoverStatus 值得换上游重试的状态：限流、鉴权/额度（换个 Key 可能就好）、
// 超时与服务端错误。400/404/413/422 是请求本身的问题，换上游也没用。
func isFailoverStatus(status int) bool {
	switch status {
	case http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden,
		http.StatusRequestTimeout, http.StatusTooManyRequests:
		return true
	}
	return status >= 500
}

// doWithFailover 依次尝试各上游。build 负责为给定上游构造请求（每次重建，保证请求体可重发）。
// 最后一个上游的响应无论成败都原样交回调用方处理。
func (rs *RouterService) doWithFailover(r *http.Request, route APIRoute, build func(APIRoute) (*http.Request, error)) (*http.Response, error) {
	candidates := orderedUpstreams(route, time.Now())
	trace := gatewayTraceFrom(r)
	var lastErr error
	for i, up := range candidates {
		last := i == len(candidates)-1
		req, err := build(route.withUpstream(up))
		if err != nil {
			lastErr = err
			if trace != nil {
				trace.skip(up, err.Error())
			}
			continue
		}
		if r != nil {
			req = req.WithContext(r.Context())
		}
		resp, err := rs.client.Do(req)
		if err != nil {
			routeUpstreamHealth.markFailed(route.Name, up)
			lastErr = fmt.Errorf("上游请求失败: %v", err)
			if r != nil && r.Context().Err() != nil {
				// 客户端已断开，没必要再换上游
				return nil, lastErr
			}
			if trace != nil && !last {
				trace.skip(up, err.Error())
			}
			continue
		}
		if isFailoverStatus(resp.StatusCode) {
			routeUpstreamHealth.markFailed(route.Name, up)
			if !last {
				_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
				resp.Body.Close()
				if trace != nil {
					trace.skip(up, fmt.Sprintf("HTTP %d", resp.StatusCode))
				}
				continue
			}
		} else {
			routeUpstreamHealth.markOK(route.Name, up)
		}
		if trace != nil {
			trace.upstream = upstreamHost(up.BaseURL)
		}
		return resp, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("路由 %s 没有可用的上游", route.Name)
	}
	return nil, lastErr
}

func upstreamHost(baseURL string) string {
	if u, err := url.Parse(strings.TrimSpace(baseURL)); err == nil && u.Host != "" {
		return u.Host
	}
	return baseURL
}

// gatewayTrace 记录一次网关请求实际用到的上游与被跳过的上游，写进请求日志
type gatewayTrace struct {
	upstream string
	skipped  []string
}

func (t *gatewayTrace) skip(up RouteUpstream, reason string) {
	t.skipped = append(t.skipped, upstreamHost(up.BaseURL)+" "+reason)
}

type gatewayTraceKey struct{}

func withGatewayTrace(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), gatewayTraceKey{}, &gatewayTrace{}))
}

func gatewayTraceFrom(r *http.Request) *gatewayTrace {
	if r == nil {
		return nil
	}
	trace, _ := r.Context().Value(gatewayTraceKey{}).(*gatewayTrace)
	return trace
}
