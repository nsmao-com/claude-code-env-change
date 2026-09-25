package main

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"sync"
	"time"
)

type UpstreamMetric struct {
	Route       string `json:"route"`
	Host        string `json:"host"`
	Requests    int    `json:"requests"`
	Failures    int    `json:"failures"`
	Consecutive int    `json:"consecutive"`
	Latency     int64  `json:"latency"`
	CoolUntil   int64  `json:"cool_until"`
	Probing     bool   `json:"probing"`
}

var policyState = struct {
	sync.Mutex
	metrics  map[string]*UpstreamMetric
	counters map[string]uint64
}{metrics: map[string]*UpstreamMetric{}, counters: map[string]uint64{}}

func metricLocked(r APIRoute, u RouteUpstream) *UpstreamMetric {
	k := upstreamHealthKey(r.Name, u)
	m := policyState.metrics[k]
	if m == nil {
		m = &UpstreamMetric{Route: r.Name, Host: upstreamHost(u.BaseURL)}
		policyState.metrics[k] = m
	}
	return m
}
func claimUpstream(r APIRoute, u RouteUpstream) bool {
	if r.FailureThreshold <= 0 {
		return true
	}
	policyState.Lock()
	defer policyState.Unlock()
	m := metricLocked(r, u)
	if m.CoolUntil > time.Now().UnixMilli() || m.Probing {
		return false
	}
	if m.CoolUntil != 0 {
		m.Probing = true
	}
	return true
}
func releaseUpstream(r APIRoute, u RouteUpstream, ok bool, latency int64) {
	policyState.Lock()
	m := metricLocked(r, u)
	m.Probing = false
	m.Requests++
	m.Latency = latency
	if ok {
		m.Consecutive = 0
		m.CoolUntil = 0
	} else {
		m.Failures++
		m.Consecutive++
		threshold := r.FailureThreshold
		if threshold < 1 {
			threshold = 1
		}
		if m.Consecutive >= threshold {
			seconds := r.CooldownSeconds
			if seconds < 1 {
				seconds = 60
			}
			m.CoolUntil = time.Now().Add(time.Duration(seconds) * time.Second).UnixMilli()
		}
	}
	cool := m.CoolUntil
	policyState.Unlock()
	if ok {
		routeUpstreamHealth.markOK(r.Name, u)
	} else if cool > 0 {
		routeUpstreamHealth.mu.Lock()
		routeUpstreamHealth.coolUntil[upstreamHealthKey(r.Name, u)] = time.UnixMilli(cool)
		routeUpstreamHealth.mu.Unlock()
	}
}
func policyUpstreams(r APIRoute, req *http.Request) []RouteUpstream {
	all := orderedUpstreams(r, time.Now())
	if len(all) < 2 || r.Strategy == "" || r.Strategy == "priority" {
		return all
	}
	policyState.Lock()
	n := policyState.counters[r.Name]
	policyState.counters[r.Name]++
	policyState.Unlock()
	if r.Strategy == "session" && req != nil {
		key := req.Header.Get("X-Session-ID")
		if key == "" {
			key = req.Header.Get("Session_id")
		}
		if key != "" {
			h := fnv.New64a()
			_, _ = h.Write([]byte(key))
			n = h.Sum64()
		}
	}
	total := 0
	for _, u := range all {
		weight := u.Weight
		if weight < 1 {
			weight = 1
		}
		if weight > 100 {
			weight = 100
		}
		total += weight
	}
	slot := int(n % uint64(total))
	index := 0
	for i, u := range all {
		weight := u.Weight
		if weight < 1 {
			weight = 1
		}
		if weight > 100 {
			weight = 100
		}
		if slot < weight {
			index = i
			break
		}
		slot -= weight
	}
	return append(all[index:], all[:index]...)
}
func (rs *RouterService) GetUpstreamHealth() []UpstreamMetric {
	policyState.Lock()
	defer policyState.Unlock()
	out := []UpstreamMetric{}
	for _, m := range policyState.metrics {
		out = append(out, *m)
	}
	return out
}
func validateRoutePolicy(r APIRoute) error {
	if r.FailureThreshold < 0 || r.FailureThreshold > 100 || r.CooldownSeconds < 0 || r.CooldownSeconds > 86400 || r.Weight < 0 || r.Weight > 100 {
		return fmt.Errorf("故障阈值、冷却时间或权重超出范围")
	}
	if r.Strategy != "" && r.Strategy != "priority" && r.Strategy != "weighted" && r.Strategy != "session" {
		return fmt.Errorf("未知分流策略")
	}
	for _, u := range r.Fallbacks {
		if u.Weight < 0 || u.Weight > 100 {
			return fmt.Errorf("上游权重必须在 0–100 之间")
		}
	}
	return nil
}
