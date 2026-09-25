package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type meteredBody struct {
	io.ReadCloser
	trace    *gatewayTrace
	start    time.Time
	stream   bool
	buffer   []byte
	overflow bool
}

func (b *meteredBody) Read(p []byte) (int, error) {
	n, e := b.ReadCloser.Read(p)
	if n > 0 && !b.overflow {
		b.buffer = append(b.buffer, p[:n]...)
		if b.stream {
			for {
				index := bytes.IndexByte(b.buffer, '\n')
				if index < 0 {
					break
				}
				line := bytes.TrimSpace(b.buffer[:index])
				b.buffer = b.buffer[index+1:]
				if bytes.HasPrefix(line, []byte("data:")) {
					b.consume(bytes.TrimSpace(line[5:]))
				}
			}
		}
		if len(b.buffer) > 4<<20 {
			b.buffer = nil
			b.overflow = true
		}
	}
	if e == io.EOF && !b.overflow {
		if b.stream {
			line := bytes.TrimSpace(b.buffer)
			if bytes.HasPrefix(line, []byte("data:")) {
				b.consume(bytes.TrimSpace(line[5:]))
			}
		} else {
			b.consume(b.buffer)
		}
		b.buffer = nil
	}
	return n, e
}
func number(m map[string]any, k string) int {
	v, _ := m[k].(float64)
	if v < 0 {
		return 0
	}
	return int(v)
}
func object(m map[string]any, k string) map[string]any { v, _ := m[k].(map[string]any); return v }
func (b *meteredBody) consume(data []byte) {
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return
	}
	t := b.trace
	if model := asString(m["model"]); model != "" {
		t.model = model
	}
	kind := asString(m["type"])
	hasDelta := false
	choices, _ := m["choices"].([]any)
	for _, raw := range choices {
		choice, _ := raw.(map[string]any)
		delta := object(choice, "delta")
		if asString(delta["content"]) != "" || asString(delta["reasoning_content"]) != "" || lenArray(delta["tool_calls"]) > 0 {
			hasDelta = true
		}
	}
	if t.firstToken == 0 && (kind == "content_block_delta" || kind == "response.output_text.delta" || hasDelta) {
		t.firstToken = time.Since(b.start).Milliseconds()
		if t.firstToken == 0 {
			t.firstToken = 1
		}
	}
	if msg := object(m, "message"); msg != nil {
		b.consumeObject(msg)
	}
	if resp := object(m, "response"); resp != nil {
		b.consumeObject(resp)
	}
	b.consumeObject(m)
}
func lenArray(v any) int { a, _ := v.([]any); return len(a) }
func (b *meteredBody) consumeObject(m map[string]any) {
	t := b.trace
	if model := asString(m["model"]); model != "" {
		t.model = model
	}
	u := object(m, "usage")
	gemini := false
	if u == nil {
		u = object(m, "usageMetadata")
		gemini = true
	}
	if u == nil {
		return
	}
	t.reported = true
	if gemini {
		t.cacheRead = number(u, "cachedContentTokenCount")
		t.input = max(0, number(u, "promptTokenCount")-t.cacheRead)
		t.output = number(u, "candidatesTokenCount") + number(u, "thoughtsTokenCount")
		return
	}
	if _, ok := u["input_tokens"]; ok {
		t.input = number(u, "input_tokens")
	}
	if _, ok := u["output_tokens"]; ok {
		t.output = number(u, "output_tokens")
	}
	if _, ok := u["cache_read_input_tokens"]; ok {
		t.cacheRead = number(u, "cache_read_input_tokens")
	}
	if _, ok := u["cache_creation_input_tokens"]; ok {
		t.cacheWrite = number(u, "cache_creation_input_tokens")
	}
	if _, ok := u["prompt_tokens"]; ok {
		t.cacheRead = number(object(u, "prompt_tokens_details"), "cached_tokens")
		t.input = max(0, number(u, "prompt_tokens")-t.cacheRead)
		t.output = number(u, "completion_tokens")
	}
	if d := object(u, "input_tokens_details"); d != nil {
		t.cacheRead = number(d, "cached_tokens")
		t.input = max(0, t.input-t.cacheRead)
	}
}

var usageLedgerMu sync.Mutex
var usageLedgerMonth = map[string]string{}

// Keep a current-month ledger and the two preceding monthly archives.
func rotateUsageLedger(p string, now time.Time) error {
	month := now.Format("2006-01")
	if usageLedgerMonth[p] == month {
		return nil
	}
	f, err := os.Open(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		var first RouterLogEntry
		err = json.NewDecoder(f).Decode(&first)
		f.Close()
		if err != nil && err != io.EOF {
			return err
		}
		if len(first.Time) >= 7 && first.Time[:7] != month {
			if _, err := time.Parse("2006-01", first.Time[:7]); err != nil {
				return err
			}
			if err := os.Rename(p, p+"."+first.Time[:7]); err != nil {
				return err
			}
		}
	}
	files, _ := filepath.Glob(p + ".????-??")
	cutoff := now.AddDate(0, -2, 0).Format("2006-01")
	for _, file := range files {
		if suffix := strings.TrimPrefix(file, p+"."); suffix < cutoff {
			_ = os.Remove(file)
		}
	}
	usageLedgerMonth[p] = month
	return nil
}

func persistGatewayUsage(e RouterLogEntry) {
	if !e.UsageReported {
		return
	}
	usageLedgerMu.Lock()
	defer usageLedgerMu.Unlock()
	p, err := storePath("gateway-usage.jsonl")
	if err != nil {
		return
	}
	if rotateUsageLedger(p, time.Now()) != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	data, err := json.Marshal(e)
	if err == nil {
		_, _ = f.Write(append(data, '\n'))
	}
}
