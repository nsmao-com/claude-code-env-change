package main

import (
	"fmt"
	"strconv"
	"strings"
)

func injectCodexExtras(payload map[string]any, vars map[string]string) {
	if payload == nil {
		return
	}
	setString := func(key, varKey string) {
		if v := strings.TrimSpace(vars[varKey]); v != "" {
			payload[key] = v
		}
	}
	setInt := func(key, varKey string) {
		if v := strings.TrimSpace(vars[varKey]); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				payload[key] = n
			}
		}
	}
	setOrDelete := func(key, varKey string) {
		if v := strings.TrimSpace(vars[varKey]); v != "" {
			payload[key] = v
		} else {
			delete(payload, key)
		}
	}
	setOrDelete("model_reasoning_effort", "model_reasoning_effort")
	setOrDelete("model_reasoning_summary", "model_reasoning_summary")
	setOrDelete("plan_mode_reasoning_effort", "plan_mode_reasoning_effort")
	setOrDelete("model_verbosity", "model_verbosity")
	setString("approval_policy", "approval_policy")
	setString("sandbox_mode", "sandbox_mode")
	setInt("model_context_window", "model_context_window")
	setInt("model_max_output_tokens", "model_max_output_tokens")
	setInt("project_doc_max_bytes", "project_doc_max_bytes")
}

// sanitizeCodexConfigPayload removes legacy keys that Codex no longer reads
// and normalizes MCP entries to the current config.toml schema.
func sanitizeCodexConfigPayload(payload map[string]any) {
	if payload == nil {
		return
	}
	delete(payload, "network_access")
	delete(payload, "disable_response_storage")

	sanitizeEntry := func(entry map[string]any) {
		if entry == nil {
			return
		}
		delete(entry, "type")
		if _, ok := entry["http_headers"]; !ok {
			if headers, ok := entry["headers"]; ok {
				entry["http_headers"] = headers
			}
		}
		delete(entry, "headers")
	}

	switch servers := payload["mcp_servers"].(type) {
	case map[string]any:
		for name, raw := range servers {
			if entry, ok := raw.(map[string]any); ok {
				sanitizeEntry(entry)
				servers[name] = entry
			}
		}
	case map[string]map[string]any:
		for name, entry := range servers {
			sanitizeEntry(entry)
			servers[name] = entry
		}
	}
}

func appendMissingEnvLines(content string, vars map[string]string, keys []string) string {
	out := content
	for _, key := range keys {
		v := strings.TrimSpace(vars[key])
		if v == "" {
			continue
		}
		if strings.Contains(out, key+"=") || strings.Contains(out, "{{"+key+"}}") {
			if strings.Contains(out, "{{"+key+"}}") {
				out = strings.ReplaceAll(out, "{{"+key+"}}", v)
			}
			continue
		}
		if !strings.HasSuffix(out, "\n") && strings.TrimSpace(out) != "" {
			out += "\n"
		}
		out += key + "=" + v + "\n"
	}
	return out
}

func injectOpencodeExtras(payload map[string]any, vars map[string]string) {
	if payload == nil {
		return
	}
	if v := strings.TrimSpace(vars["OPENCODE_SMALL_MODEL"]); v != "" {
		payload["small_model"] = v
	}
	if v := strings.TrimSpace(vars["OPENCODE_USERNAME"]); v != "" {
		payload["username"] = v
	}
	if v := strings.TrimSpace(vars["OPENCODE_SHARE"]); v != "" {
		payload["share"] = v
	}
	if v := strings.TrimSpace(vars["OPENCODE_AUTOUPDATE"]); v != "" {
		payload["autoupdate"] = v == "1" || strings.EqualFold(v, "true")
	}
	if v := strings.TrimSpace(vars["OPENCODE_SNAPSHOT"]); v != "" {
		payload["snapshot"] = v == "1" || strings.EqualFold(v, "true")
	}
	injectOpencodeThinking(payload, vars)
}

func injectOpencodeThinking(payload map[string]any, vars map[string]string) {
	effort := strings.TrimSpace(vars["OPENCODE_REASONING_EFFORT"])
	summary := strings.TrimSpace(vars["OPENCODE_REASONING_SUMMARY"])
	budget := strings.TrimSpace(vars["OPENCODE_THINKING_BUDGET"])
	if effort == "" && summary == "" && budget == "" {
		return
	}
	providers, _ := payload["provider"].(map[string]any)
	if providers == nil {
		return
	}
	target := strings.TrimSpace(vars["OPENCODE_PROVIDER_ID"])
	for id, raw := range providers {
		if target != "" && id != target {
			continue
		}
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		models, _ := entry["models"].(map[string]any)
		if models == nil {
			continue
		}
		for id, modelRaw := range models {
			model, _ := modelRaw.(map[string]any)
			if model == nil {
				model = map[string]any{}
			}
			options, _ := model["options"].(map[string]any)
			if options == nil {
				options = map[string]any{}
			}
			if effort != "" {
				options["reasoningEffort"] = effort
			}
			if summary != "" {
				options["reasoningSummary"] = summary
			}
			if budget != "" {
				if n, err := strconv.Atoi(budget); err == nil {
					options["thinking"] = map[string]any{
						"type":         "enabled",
						"budgetTokens": n,
					}
				}
			}
			model["options"] = options
			models[id] = model
		}
		entry["models"] = models
	}
}

func injectGrokExtras(payload map[string]any, vars map[string]string) {
	if payload == nil {
		return
	}
	modelMap, _ := payload["model"].(map[string]any)
	if modelMap == nil {
		modelMap = map[string]any{}
		payload["model"] = modelMap
	}
	custom, _ := modelMap["custom"].(map[string]any)
	if custom == nil {
		custom = map[string]any{}
		modelMap["custom"] = custom
	}
	if v := strings.TrimSpace(vars["XAI_MODEL_NAME"]); v != "" {
		custom["name"] = v
	}
	if n, err := strconv.Atoi(strings.TrimSpace(vars["XAI_CONTEXT_WINDOW"])); err == nil && n > 0 {
		custom["context_window"] = n
	}
	if n, err := strconv.Atoi(strings.TrimSpace(vars["XAI_MAX_TOKENS"])); err == nil && n > 0 {
		custom["max_output_tokens"] = n
	}
	if f, err := strconv.ParseFloat(strings.TrimSpace(vars["XAI_TEMPERATURE"]), 64); err == nil {
		custom["temperature"] = f
	}
	if v := strings.TrimSpace(vars["XAI_REASONING_EFFORT"]); v != "" {
		custom["reasoning_effort"] = v
		models, _ := payload["models"].(map[string]any)
		if models == nil {
			models = map[string]any{}
		}
		models["default_reasoning_effort"] = v
		payload["models"] = models
	}
}

func injectGeminiSettingsExtras(settings map[string]any, vars map[string]string) {
	if settings == nil {
		return
	}
	if v := strings.TrimSpace(vars["GEMINI_MAX_SESSION_TURNS"]); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			settings["maxSessionTurns"] = n
		}
	}
	if v := strings.TrimSpace(vars["GEMINI_COMPRESSION_THRESHOLD"]); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cc, _ := settings["chatCompression"].(map[string]any)
			if cc == nil {
				cc = map[string]any{}
			}
			cc["contextPercentageThreshold"] = f
			settings["chatCompression"] = cc
		}
	}
	injectGeminiThinking(settings, vars)
}

func injectGeminiThinking(settings map[string]any, vars map[string]string) {
	level := strings.TrimSpace(vars["GEMINI_THINKING_LEVEL"])
	budget := strings.TrimSpace(vars["GEMINI_THINKING_BUDGET"])
	if level == "" && budget == "" {
		return
	}
	thinking := map[string]any{"includeThoughts": true}
	if level != "" {
		thinking["thinkingLevel"] = strings.ToUpper(level)
	} else if n, err := strconv.Atoi(budget); err == nil {
		thinking["thinkingBudget"] = n
	} else {
		return
	}
	model := strings.TrimSpace(vars["GEMINI_MODEL"])
	if model == "" {
		model = "*"
	}
	mc, _ := settings["modelConfigs"].(map[string]any)
	if mc == nil {
		mc = map[string]any{}
	}
	raw, _ := mc["customOverrides"].([]any)
	next := make([]any, 0, len(raw)+1)
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			next = append(next, item)
			continue
		}
		if strings.TrimSpace(fmt.Sprint(entry["model"])) == model {
			continue
		}
		next = append(next, item)
	}
	next = append(next, map[string]any{
		"model": model,
		"generateContentConfig": map[string]any{
			"thinkingConfig": thinking,
		},
	})
	mc["customOverrides"] = next
	settings["modelConfigs"] = mc
}
