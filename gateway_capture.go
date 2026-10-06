package main

// 请求内容查看：开启后在内存中保留最近 50 次网关请求的请求与响应内容（各最多 256 KB），
// 用于排查协议转换、参数被拒等问题。只存在内存里，不落盘；展示前会遮盖 Key / Token 一类的字段。

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	captureLimit   = 256 << 10
	captureEntries = 50
)

// RequestCapture 一次网关请求的原始内容
type RequestCapture struct {
	ID                string            `json:"id"`
	Time              string            `json:"time"`
	Method            string            `json:"method"`
	Path              string            `json:"path"`
	RequestHeaders    map[string]string `json:"request_headers"`
	RequestBody       string            `json:"request_body"`
	RequestTruncated  bool              `json:"request_truncated,omitempty"`
	Status            int               `json:"status"`
	ResponseHeaders   map[string]string `json:"response_headers"`
	ResponseBody      string            `json:"response_body"`
	ResponseTruncated bool              `json:"response_truncated,omitempty"`
}

var captureStore = struct {
	sync.Mutex
	items map[string]RequestCapture
	order []string
}{items: map[string]RequestCapture{}}

type requestIDKey struct{}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func requestIDFrom(r *http.Request) string {
	if r == nil {
		return ""
	}
	id, _ := r.Context().Value(requestIDKey{}).(string)
	return id
}

func withRequestID(r *http.Request) (*http.Request, string) {
	id := newRequestID()
	return r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)), id
}

// limitedBuffer 只保留前 limit 字节
type limitedBuffer struct {
	buf       bytes.Buffer
	truncated bool
}

func (l *limitedBuffer) Write(p []byte) {
	room := captureLimit - l.buf.Len()
	if room <= 0 {
		if len(p) > 0 {
			l.truncated = true
		}
		return
	}
	if len(p) > room {
		p = p[:room]
		l.truncated = true
	}
	l.buf.Write(p)
}

type captureReader struct {
	io.ReadCloser
	buf *limitedBuffer
}

func (c *captureReader) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	if n > 0 {
		c.buf.Write(p[:n])
	}
	return n, err
}

// captureWriter 记录写给客户端的响应；保留 Flush，流式响应照常逐块下发
type captureWriter struct {
	http.ResponseWriter
	status int
	buf    *limitedBuffer
}

func (c *captureWriter) WriteHeader(code int) {
	if c.status == 0 {
		c.status = code
	}
	c.ResponseWriter.WriteHeader(code)
}

func (c *captureWriter) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	c.buf.Write(p)
	return c.ResponseWriter.Write(p)
}

func (c *captureWriter) Flush() {
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap 让 http.NewResponseController 能找到底层连接
func (c *captureWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }

var (
	secretValueRE = regexp.MustCompile(`(?i)("(?:api[_-]?key|apikey|authorization|x-api-key|access[_-]?token|refresh[_-]?token|secret|password|token)"\s*:\s*")([^"]{4,})(")`)
	secretTokenRE = regexp.MustCompile(`\b(sk-[A-Za-z0-9_\-]{4})[A-Za-z0-9_\-]{8,}`)
)

// redactSecrets 遮盖 JSON 中的密钥字段与 sk- 开头的 Key
func redactSecrets(s string) string {
	s = secretValueRE.ReplaceAllString(s, `$1••••$3`)
	return secretTokenRE.ReplaceAllString(s, `$1••••`)
}

func captureHeaders(h http.Header) map[string]string {
	out := map[string]string{}
	for k, v := range h {
		value := strings.Join(v, ", ")
		switch strings.ToLower(k) {
		case "authorization", "x-api-key", "x-goog-api-key", "proxy-authorization", "cookie", "set-cookie":
			value = "••••"
		}
		out[k] = value
	}
	return out
}

func decodeCaptured(b []byte, encoding string) string {
	if strings.Contains(strings.ToLower(encoding), "gzip") {
		if zr, err := gzip.NewReader(bytes.NewReader(b)); err == nil {
			if plain, err := io.ReadAll(io.LimitReader(zr, captureLimit)); err == nil || len(plain) > 0 {
				b = plain
			}
		}
	}
	return redactSecrets(strings.ToValidUTF8(string(b), "�"))
}

func (rs *RouterService) captureEnabled() bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.config.CaptureBodies
}

// captureRequest 包装请求与响应；返回的 finish 在请求处理完毕后调用
func captureRequest(w http.ResponseWriter, r *http.Request, id string, keep bool) (http.ResponseWriter, *http.Request, func() RequestCapture) {
	reqBuf, respBuf := &limitedBuffer{}, &limitedBuffer{}
	if r.Body != nil {
		r.Body = &captureReader{ReadCloser: r.Body, buf: reqBuf}
	}
	cw := &captureWriter{ResponseWriter: w, buf: respBuf}
	started := time.Now()
	finish := func() RequestCapture {
		item := RequestCapture{
			ID:                id,
			Time:              started.Format("2006-01-02 15:04:05"),
			Method:            r.Method,
			Path:              r.URL.Path,
			RequestHeaders:    captureHeaders(r.Header),
			RequestBody:       decodeCaptured(reqBuf.buf.Bytes(), r.Header.Get("Content-Encoding")),
			RequestTruncated:  reqBuf.truncated,
			Status:            cw.status,
			ResponseHeaders:   captureHeaders(cw.Header()),
			ResponseBody:      decodeCaptured(respBuf.buf.Bytes(), cw.Header().Get("Content-Encoding")),
			ResponseTruncated: respBuf.truncated,
		}
		if !keep {
			return item
		}
		captureStore.Lock()
		captureStore.items[id] = item
		captureStore.order = append(captureStore.order, id)
		for len(captureStore.order) > captureEntries {
			delete(captureStore.items, captureStore.order[0])
			captureStore.order = captureStore.order[1:]
		}
		captureStore.Unlock()
		return item
	}
	return cw, r, finish
}

// GetRequestCapture 读取一次请求的内容；未开启记录或已被挤出最近 50 条时返回错误
func (rs *RouterService) GetRequestCapture(id string) (RequestCapture, error) {
	captureStore.Lock()
	defer captureStore.Unlock()
	item, ok := captureStore.items[id]
	if !ok {
		return RequestCapture{}, fmt.Errorf("没有这条请求的内容：只保留开启记录后最近 %d 次请求，且重启后清空", captureEntries)
	}
	return item, nil
}

// ListCapturedRequestIDs 当前保留了内容的请求 ID（最新在前）
func (rs *RouterService) ListCapturedRequestIDs() []string {
	captureStore.Lock()
	defer captureStore.Unlock()
	out := make([]string, 0, len(captureStore.order))
	for i := len(captureStore.order) - 1; i >= 0; i-- {
		out = append(out, captureStore.order[i])
	}
	return out
}

// SetCaptureBodies 开关请求内容记录；关闭时立即清空已保留的内容
func (rs *RouterService) SetCaptureBodies(on bool) error {
	err := rs.persistAccessConfig(func(c *RouterConfig) error {
		c.CaptureBodies = on
		return nil
	}, false)
	if err == nil && !on {
		captureStore.Lock()
		captureStore.items = map[string]RequestCapture{}
		captureStore.order = nil
		captureStore.Unlock()
	}
	return err
}
