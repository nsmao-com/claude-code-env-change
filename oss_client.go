package main

// S3 兼容（AWS / 腾讯 COS / R2 / MinIO / 自定义）与阿里云 OSS 原生签名的对象存取。
// 仅依赖标准库。

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type ossObjectClient struct {
	provider  string
	endpoint  string
	region    string
	bucket    string
	accessKey string
	secretKey string
	pathStyle bool
	client    *http.Client
}

func newOSSObjectClient(cfg CloudConfig, httpClient *http.Client) *ossObjectClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &ossObjectClient{
		provider:  strings.ToLower(strings.TrimSpace(cfg.Provider)),
		endpoint:  strings.TrimSpace(cfg.Endpoint),
		region:    strings.TrimSpace(cfg.Region),
		bucket:    strings.TrimSpace(cfg.Bucket),
		accessKey: strings.TrimSpace(cfg.AccessKey),
		secretKey: cfg.SecretKey,
		pathStyle: cfg.PathStyle,
		client:    httpClient,
	}
}

func (c *ossObjectClient) Put(key string, body []byte, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req, err := c.newRequest(http.MethodPut, key, body, contentType)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ossHTTPError("上传", resp)
	}
	return nil
}

func (c *ossObjectClient) Get(key string) ([]byte, error) {
	req, err := c.newRequest(http.MethodGet, key, nil, "")
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("云端还没有备份（对象不存在）")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ossHTTPError("下载", resp)
	}
	return data, nil
}

func (c *ossObjectClient) Head(key string) error {
	req, err := c.newRequest(http.MethodHead, key, nil, "")
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return ossHTTPError("鉴权", resp)
	}
	if resp.StatusCode >= 400 {
		return ossHTTPError("连接", resp)
	}
	return nil
}

func ossHTTPError(action string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	msg := strings.TrimSpace(string(body))
	if len(msg) > 300 {
		msg = msg[:300] + "..."
	}
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("%s失败 (HTTP %d): %s", action, resp.StatusCode, msg)
}

func (c *ossObjectClient) newRequest(method, key string, body []byte, contentType string) (*http.Request, error) {
	if c.bucket == "" || c.accessKey == "" || c.secretKey == "" {
		return nil, fmt.Errorf("请填写 Bucket、Access Key、Secret Key")
	}
	key = strings.TrimPrefix(key, "/")
	if key == "" {
		return nil, fmt.Errorf("对象 Key 不能为空")
	}
	host, canonicalURI, fullURL := c.buildURL(key)
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequest(method, fullURL, reader)
	if err != nil {
		return nil, err
	}
	req.Host = host
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.provider == "aliyun" || c.provider == "oss" {
		c.signAliyun(req, method, key, body, contentType)
		return req, nil
	}
	c.signS3(req, method, host, canonicalURI, body, contentType)
	return req, nil
}

func (c *ossObjectClient) buildURL(key string) (host, canonicalURI, fullURL string) {
	endpoint := strings.TrimRight(c.endpoint, "/")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	scheme := "https"
	if strings.HasPrefix(strings.ToLower(c.endpoint), "http://") {
		scheme = "http"
	}

	encodedKey := encodeOSSPath(key)

	switch c.provider {
	case "s3", "aws":
		region := c.region
		if region == "" {
			region = "us-east-1"
		}
		if endpoint == "" {
			if c.pathStyle {
				host = fmt.Sprintf("s3.%s.amazonaws.com", region)
				canonicalURI = "/" + url.PathEscape(c.bucket) + "/" + encodedKey
				fullURL = fmt.Sprintf("%s://%s/%s/%s", scheme, host, url.PathEscape(c.bucket), encodedKey)
				return
			}
			host = fmt.Sprintf("%s.s3.%s.amazonaws.com", c.bucket, region)
			canonicalURI = "/" + encodedKey
			fullURL = fmt.Sprintf("%s://%s/%s", scheme, host, encodedKey)
			return
		}
	}

	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}

	if c.pathStyle {
		host = endpoint
		canonicalURI = "/" + url.PathEscape(c.bucket) + "/" + encodedKey
		fullURL = fmt.Sprintf("%s://%s/%s/%s", scheme, host, url.PathEscape(c.bucket), encodedKey)
		return
	}

	// 虚拟主机：bucket.endpoint
	if strings.HasPrefix(endpoint, c.bucket+".") {
		host = endpoint
	} else {
		host = c.bucket + "." + endpoint
	}
	canonicalURI = "/" + encodedKey
	fullURL = fmt.Sprintf("%s://%s/%s", scheme, host, encodedKey)
	return
}

func encodeOSSPath(key string) string {
	parts := strings.Split(key, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func (c *ossObjectClient) signS3(req *http.Request, method, host, canonicalURI string, body []byte, contentType string) {
	region := c.region
	if region == "" {
		region = "us-east-1"
	}
	if c.provider == "r2" && (region == "" || region == "auto") {
		region = "auto"
	}
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	payloadHash := sha256HexBytes(body)
	req.Header.Set("Host", host)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	headers := map[string]string{
		"host":                 host,
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           amzDate,
	}
	if contentType != "" {
		headers["content-type"] = contentType
	}
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	sort.Strings(names)
	var canonicalHeaders strings.Builder
	for _, n := range names {
		canonicalHeaders.WriteString(n)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(strings.TrimSpace(headers[n]))
		canonicalHeaders.WriteByte('\n')
	}
	signedHeaders := strings.Join(names, ";")
	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI,
		"",
		canonicalHeaders.String(),
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := dateStamp + "/" + region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256HexBytes([]byte(canonicalRequest)),
	}, "\n")

	signingKey := s3SigningKey(c.secretKey, dateStamp, region, "s3")
	sig := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.accessKey, scope, signedHeaders, sig,
	))
}

func s3SigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

func sha256HexBytes(data []byte) string {
	if data == nil {
		data = []byte{}
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (c *ossObjectClient) signAliyun(req *http.Request, method, key string, body []byte, contentType string) {
	now := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", now)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resource := "/" + c.bucket + "/" + key
	stringToSign := method + "\n\n" + contentType + "\n" + now + "\n" + resource
	mac := hmac.New(sha1.New, []byte(c.secretKey))
	mac.Write([]byte(stringToSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	req.Header.Set("Authorization", "OSS "+c.accessKey+":"+sig)
}
