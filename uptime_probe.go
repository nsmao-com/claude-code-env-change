package main

// 鉴权探测：带上配置里的 Key 请求"列模型"接口。不消耗 token，却能发现
// Key 失效、过期、余额不足——这些情况下 Base URL 照样可达，纯可达性检测看不出来，
// 轮换组也就永远不会切走。

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	uptimeProbeReachability = "reachability"
	uptimeProbeAuth         = "auth"
)

func normalizeUptimeProbeMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), uptimeProbeAuth) {
		return uptimeProbeAuth
	}
	return uptimeProbeReachability
}

type authProbe struct {
	URL      string
	Headers  map[string]string
	Protocol string // anthropic | openai | gemini
}

// buildAuthProbe 按配置的实际上游协议构造鉴权探测；没有 Key（官方登录等）时返回 false，
// 调用方退回可达性检测
func buildAuthProbe(env EnvConfig) (authProbe, bool) {
	baseURL := strings.TrimSpace(deriveEnvURL(env))
	if baseURL == "" {
		return authProbe{}, false
	}
	vars := env.Variables
	provider := strings.ToLower(strings.TrimSpace(env.Provider))
	if provider == "" {
		provider = "claude"
	}

	var key, bearer string
	switch provider {
	case "claude", "claude_desktop":
		bearer = strings.TrimSpace(vars["ANTHROPIC_AUTH_TOKEN"])
		key = strings.TrimSpace(vars["ANTHROPIC_API_KEY"])
	case "codex":
		key = strings.TrimSpace(vars["OPENAI_API_KEY"])
	case "antigravity":
		key = firstNonEmpty(strings.TrimSpace(vars["GEMINI_API_KEY"]), strings.TrimSpace(vars["GOOGLE_API_KEY"]))
	case "opencode":
		key = strings.TrimSpace(vars["OPENCODE_API_KEY"])
	case "grok":
		key = strings.TrimSpace(vars["XAI_API_KEY"])
	default:
		return authProbe{}, false
	}
	if key == "" && bearer == "" {
		return authProbe{}, false
	}

	protocol := authProbeProtocol(&env, provider)
	probe := authProbe{Protocol: protocol, Headers: map[string]string{}}
	switch protocol {
	case "anthropic":
		probe.URL = joinModelsURL(baseURL, "/v1/models")
		probe.Headers["anthropic-version"] = "2023-06-01"
		// 与 Claude Code 一致：AUTH_TOKEN 走 Bearer，API_KEY 走 x-api-key
		if bearer != "" {
			probe.Headers["Authorization"] = "Bearer " + bearer
		} else {
			probe.Headers["x-api-key"] = key
		}
	case "gemini":
		probe.URL = joinModelsURL(baseURL, "/v1beta/models")
		probe.Headers["x-goog-api-key"] = key
	default:
		probe.URL = joinModelsURL(baseURL, "/v1/models")
		probe.Headers["Authorization"] = "Bearer " + firstNonEmpty(key, bearer)
	}
	return probe, true
}

// authProbeProtocol 走本地路由转换的配置，Key 属于转换后的上游协议
func authProbeProtocol(env *EnvConfig, provider string) string {
	if needsConversion(env) {
		if normalizeUpstreamFormat(env.UpstreamFormat) == UpstreamAnthropicMessages {
			return "anthropic"
		}
		return "openai"
	}
	switch provider {
	case "claude", "claude_desktop":
		return "anthropic"
	case "antigravity":
		return "gemini"
	default:
		return "openai"
	}
}

// joinModelsURL 拼出列模型地址：Base URL 已带版本段（/v1、/v1beta）时不再重复
func joinModelsURL(baseURL, versionedPath string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	version := versionedPath[:strings.LastIndex(versionedPath, "/")]
	if strings.HasSuffix(strings.ToLower(base), strings.ToLower(version)) {
		return base + "/models"
	}
	return base + versionedPath
}

func runAuthCheck(client *http.Client, probe authProbe) UptimeCheck {
	start := time.Now()
	check := UptimeCheck{At: start.Unix()}

	req, err := http.NewRequest(http.MethodGet, probe.URL, nil)
	if err != nil {
		check.Error = err.Error()
		return check
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range probe.Headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	check.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		check.Error = err.Error()
		return check
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))

	check.StatusCode = resp.StatusCode
	check.Success, check.Error = classifyAuthProbe(resp.StatusCode, probe.Protocol, body)
	return check
}

// classifyAuthProbe 判定鉴权探测结果。列模型接口不存在（404/405 等）只说明
// 中转站没实现它，服务本身在线，按成功处理，避免误触发轮换。
func classifyAuthProbe(status int, protocol string, body []byte) (bool, string) {
	detail := upstreamErrorMessage(body)
	switch {
	case status >= 200 && status < 300:
		return true, ""
	case status == http.StatusUnauthorized, status == http.StatusPaymentRequired, status == http.StatusForbidden:
		return false, fmt.Sprintf("Key 无效、过期或余额不足（HTTP %d）%s", status, detail)
	case status == http.StatusBadRequest && protocol == "gemini":
		// Gemini 对无效 Key 返回 400 INVALID_ARGUMENT
		return false, fmt.Sprintf("Key 无效（HTTP 400）%s", detail)
	case status == http.StatusTooManyRequests, status >= 500:
		return false, fmt.Sprintf("上游返回 HTTP %d%s", status, detail)
	default:
		return true, ""
	}
}

func upstreamErrorMessage(body []byte) string {
	var payload struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(body, &payload) != nil || len(payload.Error) == 0 {
		return ""
	}
	var nested struct {
		Message string `json:"message"`
	}
	msg := ""
	if json.Unmarshal(payload.Error, &nested) == nil {
		msg = nested.Message
	} else {
		_ = json.Unmarshal(payload.Error, &msg)
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	if len([]rune(msg)) > 120 {
		msg = string([]rune(msg)[:120]) + "…"
	}
	return "：" + msg
}
