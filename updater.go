package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// 与 wails.json info.productVersion 保持一致
const appVersion = "2.3.0"

const (
	githubOwner = "nsmao-com"
	githubRepo  = "claude-code-env-change"
	updateEvent = "update:progress"
)

var updateMu sync.Mutex

// UpdateInfo GitHub 更新检查结果
type UpdateInfo struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	ReleaseName    string `json:"release_name"`
	ReleaseNotes   string `json:"release_notes"`
	PublishedAt    string `json:"published_at"`
	DownloadURL    string `json:"download_url"`
	AssetName      string `json:"asset_name"`
	AssetSize      int64  `json:"asset_size"`
	AssetDigest    string `json:"asset_digest"`
	ReleaseURL     string `json:"release_url"`
	CanApply       bool   `json:"can_apply"`
	IsDev          bool   `json:"is_dev"`
	Message        string `json:"message"`
}

// UpdateProgress 应用内更新进度（EventsEmit update:progress）
type UpdateProgress struct {
	Phase    string  `json:"phase"`
	Percent  float64 `json:"percent"`
	Received int64   `json:"received"`
	Total    int64   `json:"total"`
	Message  string  `json:"message"`
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt string    `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	Assets      []ghAsset `json:"assets"`
	Message     string    `json:"message"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
	Digest             string `json:"digest"`
	State              string `json:"state"`
}

func (a *App) emitUpdateProgress(p UpdateProgress) {
	if a == nil || a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, updateEvent, p)
}

// GetAppVersion 当前软件版本
func (a *App) GetAppVersion() string {
	return appVersion
}

// OpenReleasePage 打开 GitHub Releases 页面
func (a *App) OpenReleasePage() {
	if a == nil || a.ctx == nil {
		return
	}
	runtime.BrowserOpenURL(a.ctx, fmt.Sprintf("https://github.com/%s/%s/releases/latest", githubOwner, githubRepo))
}

// CheckForUpdate 查询 GitHub Releases 最新版本
func (a *App) CheckForUpdate() (UpdateInfo, error) {
	info := UpdateInfo{
		CurrentVersion: appVersion,
		IsDev:          isDevBuild(),
	}

	rel, err := fetchLatestRelease()
	if err != nil {
		return info, err
	}

	latest := strings.TrimSpace(rel.TagName)
	info.LatestVersion = latest
	info.ReleaseName = strings.TrimSpace(rel.Name)
	info.ReleaseNotes = strings.TrimSpace(rel.Body)
	info.PublishedAt = rel.PublishedAt
	info.ReleaseURL = rel.HTMLURL
	if info.ReleaseName == "" {
		info.ReleaseName = latest
	}

	asset := pickReleaseAsset(rel.Assets)
	if asset != nil {
		info.DownloadURL = asset.BrowserDownloadURL
		info.AssetName = asset.Name
		info.AssetSize = asset.Size
		info.AssetDigest = asset.Digest
	}

	cmp := compareVersions(latest, appVersion)
	switch {
	case cmp > 0:
		info.Available = true
		info.CanApply = goruntime.GOOS == "windows" && info.DownloadURL != "" && !info.IsDev
		if info.IsDev {
			info.Message = "发现新版本。开发模式不能覆盖当前程序，请使用正式构建更新，或前往 GitHub 下载。"
		} else if info.DownloadURL == "" {
			info.Message = "发现新版本，但没有匹配当前系统的安装包，请前往 GitHub 下载。"
		} else if goruntime.GOOS != "windows" {
			info.Message = "发现新版本。当前系统请从 GitHub 下载安装包后手动更新。"
		} else {
			info.Message = "发现新版本，可在软件内下载并安装。"
		}
	case cmp < 0:
		info.Message = "当前版本高于 GitHub 最新发布。"
	default:
		info.Message = "当前已是最新版本。"
	}
	return info, nil
}

// DownloadAndApplyUpdate 下载 GitHub 安装包并替换当前程序（Windows）
func (a *App) DownloadAndApplyUpdate() error {
	if !updateMu.TryLock() {
		return fmt.Errorf("已有更新任务在进行")
	}
	defer updateMu.Unlock()

	if isDevBuild() {
		return fmt.Errorf("开发模式无法覆盖当前程序，请使用正式构建更新")
	}
	if goruntime.GOOS != "windows" {
		return fmt.Errorf("当前系统请从 GitHub 下载安装包后手动更新")
	}

	info, err := a.CheckForUpdate()
	if err != nil {
		return err
	}
	if !info.Available {
		return fmt.Errorf("%s", info.Message)
	}
	if info.DownloadURL == "" {
		return fmt.Errorf("没有匹配当前系统的安装包")
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法定位当前程序: %v", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("无法解析程序路径: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "claude-env-update-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %v", err)
	}

	a.emitUpdateProgress(UpdateProgress{Phase: "download", Percent: 0, Message: "开始下载…"})

	archivePath := filepath.Join(tmpDir, sanitizeFileName(info.AssetName))
	if err := downloadWithProgress(a, info.DownloadURL, archivePath, info.AssetSize); err != nil {
		os.RemoveAll(tmpDir)
		a.emitUpdateProgress(UpdateProgress{Phase: "error", Message: err.Error()})
		return err
	}

	if err := verifyFileDigest(archivePath, info.AssetDigest); err != nil {
		os.RemoveAll(tmpDir)
		a.emitUpdateProgress(UpdateProgress{Phase: "error", Message: err.Error()})
		return err
	}

	a.emitUpdateProgress(UpdateProgress{Phase: "extract", Percent: 100, Message: "正在解压安装包…"})

	newExe := filepath.Join(tmpDir, "claude-env-switcher-update.exe")
	lowerName := strings.ToLower(info.AssetName)
	switch {
	case strings.HasSuffix(lowerName, ".zip"):
		if err := extractPrimaryExe(archivePath, newExe); err != nil {
			os.RemoveAll(tmpDir)
			a.emitUpdateProgress(UpdateProgress{Phase: "error", Message: err.Error()})
			return err
		}
		_ = os.Remove(archivePath)
	case strings.HasSuffix(lowerName, ".exe"):
		if err := os.Rename(archivePath, newExe); err != nil {
			os.RemoveAll(tmpDir)
			return fmt.Errorf("准备安装文件失败: %v", err)
		}
	default:
		os.RemoveAll(tmpDir)
		return fmt.Errorf("不支持的安装包格式: %s", info.AssetName)
	}

	a.emitUpdateProgress(UpdateProgress{Phase: "apply", Percent: 100, Message: "即将重启并完成安装…"})
	if err := applyUpdateAndRestart(a, newExe, exePath); err != nil {
		a.emitUpdateProgress(UpdateProgress{Phase: "error", Message: err.Error()})
		return err
	}
	runtime.Quit(a.ctx)
	return nil
}

func fetchLatestRelease() (*ghRelease, error) {
	path := fmt.Sprintf("/repos/%s/%s/releases/latest", githubOwner, githubRepo)
	urls := []string{
		"https://api.github.com" + path,
		"https://gh-proxy.com/https://api.github.com" + path,
	}
	var lastErr error
	for _, raw := range urls {
		body, err := httpGetBytes(raw, 20*time.Second, "application/vnd.github+json")
		if err != nil {
			lastErr = err
			continue
		}
		var rel ghRelease
		if err := json.Unmarshal(body, &rel); err != nil {
			lastErr = fmt.Errorf("解析 GitHub 响应失败")
			continue
		}
		if rel.Message != "" && rel.TagName == "" {
			lastErr = fmt.Errorf("GitHub: %s", rel.Message)
			continue
		}
		if rel.Draft || strings.TrimSpace(rel.TagName) == "" {
			lastErr = fmt.Errorf("未找到可用的 GitHub Release")
			continue
		}
		return &rel, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("无法连接 GitHub，请检查网络或代理")
	}
	return nil, lastErr
}

func httpGetBytes(url string, timeout time.Duration, accept string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "claude-env-switcher/"+appVersion)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := githubHTTPClient(timeout).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("GitHub API 请求过于频繁，请稍后再试")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 返回 %d", resp.StatusCode)
	}
	return body, nil
}

func githubHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func downloadWithProgress(a *App, url, dest string, expectedSize int64) error {
	urls := []string{url}
	if !strings.Contains(url, "gh-proxy.com/") {
		urls = append(urls, "https://gh-proxy.com/"+url)
	}
	var lastErr error
	for _, raw := range urls {
		if err := downloadOne(a, raw, dest, expectedSize); err != nil {
			lastErr = err
			_ = os.Remove(dest)
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("下载失败")
	}
	return lastErr
}

func downloadOne(a *App, url, dest string, expectedSize int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "claude-env-switcher/"+appVersion)
	req.Header.Set("Accept", "application/octet-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，服务器返回 %d", resp.StatusCode)
	}

	total := resp.ContentLength
	if total <= 0 {
		total = expectedSize
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("无法写入临时文件: %v", err)
	}
	defer out.Close()

	reader := &countingReader{
		r:     resp.Body,
		total: total,
		onUpdate: func(read, tot int64) {
			a.emitUpdateProgress(UpdateProgress{
				Phase:    "download",
				Percent:  progressPercent(read, tot),
				Received: read,
				Total:    tot,
				Message:  fmt.Sprintf("正在下载 %s / %s", formatBytes(read), formatBytes(tot)),
			})
		},
	}
	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("下载中断: %v", err)
	}
	return nil
}

type countingReader struct {
	r        io.Reader
	read     int64
	total    int64
	last     time.Time
	onUpdate func(read, total int64)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.read += int64(n)
	now := time.Now()
	if c.onUpdate != nil && (now.Sub(c.last) > 120*time.Millisecond || err == io.EOF) {
		c.last = now
		c.onUpdate(c.read, c.total)
	}
	return n, err
}

func extractPrimaryExe(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("无法打开安装包: %v", err)
	}
	defer r.Close()

	var chosen *zip.File
	best := -1
	for i := range r.File {
		f := r.File[i]
		if f.FileInfo().IsDir() {
			continue
		}
		name := strings.ToLower(filepath.ToSlash(f.Name))
		if strings.Contains(name, "__macosx") {
			continue
		}
		if !strings.HasSuffix(name, ".exe") {
			continue
		}
		base := filepath.Base(name)
		score := 1
		if base == "claude-env-switcher.exe" {
			score = 20
		} else if strings.Contains(base, "claude-env-switcher") {
			score = 10
		}
		if strings.Contains(base, "uninstall") || strings.Contains(base, "setup") {
			score = 0
		}
		if score > best {
			best = score
			chosen = f
		}
	}
	if chosen == nil {
		return fmt.Errorf("安装包中未找到 Windows 可执行文件")
	}

	rc, err := chosen.Open()
	if err != nil {
		return fmt.Errorf("读取安装包失败: %v", err)
	}
	defer rc.Close()

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("无法写入更新文件: %v", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("解压失败: %v", err)
	}
	return nil
}

func verifyFileDigest(path, digest string) error {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return nil
	}
	algo, hexStr, ok := strings.Cut(digest, ":")
	if !ok || !strings.EqualFold(algo, "sha256") || hexStr == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("无法校验下载文件")
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("校验下载文件失败")
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, hexStr) {
		return fmt.Errorf("下载文件校验失败，请重试")
	}
	return nil
}

func pickReleaseAsset(assets []ghAsset) *ghAsset {
	var best *ghAsset
	bestScore := 0
	for i := range assets {
		a := &assets[i]
		if a.State != "" && a.State != "uploaded" {
			continue
		}
		score := scoreAsset(a.Name)
		if score > bestScore {
			bestScore = score
			best = a
		}
	}
	return best
}

func scoreAsset(name string) int {
	n := strings.ToLower(name)
	osKeys := map[string][]string{
		"windows": {"windows", "win64", "win32", "win"},
		"darwin":  {"macos", "darwin", "osx", "mac"},
		"linux":   {"linux"},
	}
	archKeys := map[string][]string{
		"amd64": {"amd64", "x86_64", "x64"},
		"arm64": {"arm64", "aarch64"},
		"386":   {"386", "i386", "x86"},
	}
	matchedOS := false
	score := 0
	for _, k := range osKeys[goruntime.GOOS] {
		if strings.Contains(n, k) {
			matchedOS = true
			score += 10
			break
		}
	}
	if !matchedOS {
		return 0
	}
	for _, k := range archKeys[goruntime.GOARCH] {
		if strings.Contains(n, k) {
			score += 5
			break
		}
	}
	switch {
	case strings.HasSuffix(n, ".zip"):
		score += 3
	case strings.HasSuffix(n, ".exe"):
		score += 2
	}
	if strings.Contains(n, "installer") || strings.Contains(n, "setup") || strings.Contains(n, "nsis") {
		score--
	}
	return score
}

func isDevBuild() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	base := strings.ToLower(filepath.Base(exe))
	return strings.Contains(base, "-dev")
}

func compareVersions(a, b string) int {
	pa := parseVersion(a)
	pb := parseVersion(b)
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var va, vb int
		if i < len(pa) {
			va = pa[i]
		}
		if i < len(pb) {
			vb = pb[i]
		}
		if va > vb {
			return 1
		}
		if va < vb {
			return -1
		}
	}
	return 0
}

func parseVersion(s string) []int {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		out = append(out, n)
	}
	if len(out) == 0 {
		return []int{0}
	}
	return out
}

func progressPercent(read, total int64) float64 {
	if total <= 0 {
		return 0
	}
	p := float64(read) * 100 / float64(total)
	if p > 100 {
		return 100
	}
	if p < 0 {
		return 0
	}
	return p
}

func formatBytes(n int64) string {
	if n <= 0 {
		return "0 B"
	}
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}

func sanitizeFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		return "update.bin"
	}
	return name
}
