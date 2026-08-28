package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type marketCacheEntry struct {
	at   time.Time
	data []byte
}

var marketCache sync.Map

func marketHTTPGet(rawURL string, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	if cached, ok := marketCacheLoad(rawURL); ok {
		return cached, nil
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AI-ENV/"+appVersion)
	req.Header.Set("Accept", "application/json, text/plain;q=0.9, */*;q=0.8")
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	marketCacheStore(rawURL, body)
	return body, nil
}

func githubFileURLs(repo, branch, rel string) []string {
	repo = strings.Trim(repo, "/")
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}
	rel = strings.TrimPrefix(strings.ReplaceAll(rel, "\\", "/"), "./")
	rel = strings.TrimPrefix(rel, "/")
	return []string{
		"https://cdn.jsdelivr.net/gh/" + repo + "@" + branch + "/" + rel,
		"https://fastly.jsdelivr.net/gh/" + repo + "@" + branch + "/" + rel,
		"https://raw.githubusercontent.com/" + repo + "/" + branch + "/" + rel,
		"https://gh-proxy.com/https://raw.githubusercontent.com/" + repo + "/" + branch + "/" + rel,
	}
}

func marketGetGitHubFile(repo, branch, rel string) ([]byte, error) {
	var last error
	for _, raw := range githubFileURLs(repo, branch, rel) {
		data, err := marketHTTPGet(raw, 18*time.Second)
		if err == nil && len(strings.TrimSpace(string(data))) > 0 {
			return data, nil
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("下载失败")
	}
	return nil, last
}

func marketGetGitHubRaw(path string) ([]byte, error) {
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, "/", 4)
	if len(parts) == 4 {
		return marketGetGitHubFile(parts[0]+"/"+parts[1], parts[2], parts[3])
	}
	urls := []string{
		"https://raw.githubusercontent.com/" + path,
		"https://gh-proxy.com/https://raw.githubusercontent.com/" + path,
	}
	var last error
	for _, raw := range urls {
		data, err := marketHTTPGet(raw, 18*time.Second)
		if err == nil {
			return data, nil
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("下载失败")
	}
	return nil, last
}

func marketGetGitHubAPI(path string) ([]byte, error) {
	path = strings.TrimPrefix(path, "/")
	urls := []string{
		"https://api.github.com/" + path,
		"https://gh-proxy.com/https://api.github.com/" + path,
	}
	var last error
	for _, raw := range urls {
		data, err := marketHTTPGet(raw, 15*time.Second)
		if err == nil {
			return data, nil
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("请求 GitHub 失败")
	}
	return nil, last
}

func marketCacheLoad(key string) ([]byte, bool) {
	raw, ok := marketCache.Load(key)
	if !ok {
		return nil, false
	}
	entry, ok := raw.(marketCacheEntry)
	if !ok || time.Since(entry.at) > 10*time.Minute {
		return nil, false
	}
	return entry.data, true
}

func marketCacheStore(key string, data []byte) {
	dup := append([]byte(nil), data...)
	marketCache.Store(key, marketCacheEntry{at: time.Now(), data: dup})
}

func slugMarketName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if b.Len() > 0 && !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "item"
	}
	if len(s) > 64 {
		s = s[:64]
		s = strings.Trim(s, "-")
	}
	return s
}

func containsFold(haystack, needle string) bool {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
