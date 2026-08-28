package main

import (
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
	setString("model_reasoning_effort", "model_reasoning_effort")
	setString("model_reasoning_summary", "model_reasoning_summary")
	setString("approval_policy", "approval_policy")
	setString("sandbox_mode", "sandbox_mode")
	setString("model_verbosity", "model_verbosity")
	setInt("model_context_window", "model_context_window")
	setInt("model_max_output_tokens", "model_max_output_tokens")
	setInt("project_doc_max_bytes", "project_doc_max_bytes")
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
}
