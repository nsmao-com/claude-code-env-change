package main

// 云端 OSS 配置同步：把环境/MCP/路由/Skills/监控配置打包上传，换电脑后凭同一套 OSS 凭证拉取。
// 本地 API Key 仍明文存于各 json（与现有做法一致）；上传云端时若设置了口令则 AES-GCM 加密整包。

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	cloudStoreFile     = "cloud.json"
	cloudBackupVersion = 1
	cloudMagic         = "CEB1"
	cloudDebounce      = 2 * time.Second
	defaultCloudKey    = "claude-env-switcher/backup.bin"
)

var (
	cloudSyncInst *CloudSyncService
)

func notifyCloudSync() {
	if cloudSyncInst == nil {
		return
	}
	cloudSyncInst.schedulePush()
}

// CloudConfig OSS 同步配置（凭证仅存本机 cloud.json）
type CloudConfig struct {
	Enabled         bool   `json:"enabled"`
	Provider        string `json:"provider"` // s3 | aliyun | tencent | r2 | minio | custom
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	ObjectKey       string `json:"object_key"`
	AccessKey       string `json:"access_key"`
	SecretKey       string `json:"secret_key"`
	PathStyle       bool   `json:"path_style"`
	Passphrase      string `json:"passphrase,omitempty"`
	AutoPush        bool   `json:"auto_push"`
	AutoPullOnStart bool   `json:"auto_pull_on_start"`
	LastPushAt      int64  `json:"last_push_at,omitempty"`
	LastPullAt      int64  `json:"last_pull_at,omitempty"`
	LastError       string `json:"last_error,omitempty"`
}

// CloudSyncResult 一次上传/下载/测试的结果
type CloudSyncResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Latency int64  `json:"latency"`
}

// CloudSyncStatus 状态快照
type CloudSyncStatus struct {
	Enabled    bool   `json:"enabled"`
	Configured bool   `json:"configured"`
	Pushing    bool   `json:"pushing"`
	LastPushAt int64  `json:"last_push_at,omitempty"`
	LastPullAt int64  `json:"last_pull_at,omitempty"`
	LastError  string `json:"last_error,omitempty"`
	ObjectKey  string `json:"object_key,omitempty"`
	Provider   string `json:"provider,omitempty"`
}

type cloudBundle struct {
	Version    int                        `json:"version"`
	ExportedAt int64                      `json:"exported_at"`
	Hostname   string                     `json:"hostname,omitempty"`
	Files      map[string]json.RawMessage `json:"files"`
}

// CloudSyncService 云同步服务
type CloudSyncService struct {
	mu         sync.Mutex
	config     CloudConfig
	app        *App
	router     *RouterService
	httpClient *http.Client
	timer      *time.Timer
	pushing    bool
	applying   bool
}

func NewCloudSyncService(app *App, router *RouterService) *CloudSyncService {
	cs := &CloudSyncService{
		app:        app,
		router:     router,
		httpClient: &http.Client{Timeout: 45 * time.Second},
	}
	_ = cs.loadConfig()
	cs.applyEnvOverrides()
	cloudSyncInst = cs
	return cs
}

func (cs *CloudSyncService) OnStartup() {
	cs.mu.Lock()
	pull := cs.config.Enabled && cs.config.AutoPullOnStart && cs.isConfiguredLocked()
	cs.mu.Unlock()
	if pull {
		_ = cs.DownloadFromCloud()
	}
}

func (cs *CloudSyncService) configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, cloudStoreFile), nil
}

func (cs *CloudSyncService) loadConfig() error {
	path, err := cs.configPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cs.config = CloudConfig{
				Provider:  "aliyun",
				ObjectKey: defaultCloudKey,
				AutoPush:  true,
			}
			return nil
		}
		return err
	}
	var cfg CloudConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.ObjectKey) == "" {
		cfg.ObjectKey = defaultCloudKey
	}
	if strings.TrimSpace(cfg.Provider) == "" {
		cfg.Provider = "aliyun"
	}
	cs.config = cfg
	return nil
}

func (cs *CloudSyncService) applyEnvOverrides() {
	set := func(dst *string, keys ...string) {
		if strings.TrimSpace(*dst) != "" {
			return
		}
		for _, k := range keys {
			if v := strings.TrimSpace(os.Getenv(k)); v != "" {
				*dst = v
				return
			}
		}
	}
	set(&cs.config.Endpoint, "CLAUDIA_OSS_ENDPOINT")
	set(&cs.config.Region, "CLAUDIA_OSS_REGION")
	set(&cs.config.Bucket, "CLAUDIA_OSS_BUCKET")
	set(&cs.config.AccessKey, "CLAUDIA_OSS_ACCESS_KEY")
	set(&cs.config.SecretKey, "CLAUDIA_OSS_SECRET_KEY")
	set(&cs.config.Passphrase, "CLAUDIA_OSS_PASSPHRASE")
	set(&cs.config.ObjectKey, "CLAUDIA_OSS_OBJECT_KEY")
	if p := strings.TrimSpace(os.Getenv("CLAUDIA_OSS_PROVIDER")); p != "" && cs.config.Provider == "aliyun" {
		cs.config.Provider = p
	}
}

func (cs *CloudSyncService) persistLocked() error {
	path, err := cs.configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cs.config, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (cs *CloudSyncService) isConfiguredLocked() bool {
	return strings.TrimSpace(cs.config.Bucket) != "" &&
		strings.TrimSpace(cs.config.AccessKey) != "" &&
		strings.TrimSpace(cs.config.SecretKey) != ""
}

func (cs *CloudSyncService) GetCloudConfig() CloudConfig {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.config
}

func (cs *CloudSyncService) GetCloudSyncStatus() CloudSyncStatus {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return CloudSyncStatus{
		Enabled:    cs.config.Enabled,
		Configured: cs.isConfiguredLocked(),
		Pushing:    cs.pushing,
		LastPushAt: cs.config.LastPushAt,
		LastPullAt: cs.config.LastPullAt,
		LastError:  cs.config.LastError,
		ObjectKey:  cs.config.ObjectKey,
		Provider:   cs.config.Provider,
	}
}

func (cs *CloudSyncService) SaveCloudConfig(cfg CloudConfig) error {
	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	if cfg.Provider == "" {
		cfg.Provider = "aliyun"
	}
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.ObjectKey = strings.TrimSpace(cfg.ObjectKey)
	if cfg.ObjectKey == "" {
		cfg.ObjectKey = defaultCloudKey
	}
	cfg.AccessKey = strings.TrimSpace(cfg.AccessKey)

	cs.mu.Lock()
	defer cs.mu.Unlock()
	cfg.LastPushAt = cs.config.LastPushAt
	cfg.LastPullAt = cs.config.LastPullAt
	if strings.TrimSpace(cfg.SecretKey) == "" {
		cfg.SecretKey = cs.config.SecretKey
	}
	cs.config = cfg
	return cs.persistLocked()
}

func (cs *CloudSyncService) TestCloudConnection() CloudSyncResult {
	start := time.Now()
	cs.mu.Lock()
	cfg := cs.config
	cs.mu.Unlock()
	if !cfg.Enabled && (cfg.Bucket == "" || cfg.AccessKey == "") {
		return CloudSyncResult{Success: false, Message: "请先填写 OSS 配置", Latency: time.Since(start).Milliseconds()}
	}
	client := newOSSObjectClient(cfg, cs.httpClient)
	key := cfg.ObjectKey
	if key == "" {
		key = defaultCloudKey
	}
	if err := client.Head(key); err != nil {
		return CloudSyncResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}
	return CloudSyncResult{Success: true, Message: "OSS 连接正常（凭证有效）", Latency: time.Since(start).Milliseconds()}
}

func (cs *CloudSyncService) UploadToCloud() CloudSyncResult {
	start := time.Now()
	cs.mu.Lock()
	if cs.applying {
		cs.mu.Unlock()
		return CloudSyncResult{Success: true, Message: "正在从云端恢复，跳过上传", Latency: 0}
	}
	cfg := cs.config
	cs.pushing = true
	cs.mu.Unlock()
	defer func() {
		cs.mu.Lock()
		cs.pushing = false
		cs.mu.Unlock()
	}()

	if !csConfigured(cfg) {
		return CloudSyncResult{Success: false, Message: "请先填写 Bucket 与密钥", Latency: time.Since(start).Milliseconds()}
	}

	bundle, err := cs.buildBundle()
	if err != nil {
		cs.recordError(err.Error())
		return CloudSyncResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}
	payload, err := json.Marshal(bundle)
	if err != nil {
		return CloudSyncResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}
	contentType := "application/json"
	if strings.TrimSpace(cfg.Passphrase) != "" {
		enc, err := encryptCloudPayload(payload, cfg.Passphrase)
		if err != nil {
			cs.recordError(err.Error())
			return CloudSyncResult{Success: false, Message: "加密失败: " + err.Error(), Latency: time.Since(start).Milliseconds()}
		}
		payload = enc
		contentType = "application/octet-stream"
	}

	client := newOSSObjectClient(cfg, cs.httpClient)
	key := cfg.ObjectKey
	if key == "" {
		key = defaultCloudKey
	}
	if err := client.Put(key, payload, contentType); err != nil {
		cs.recordError(err.Error())
		return CloudSyncResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}

	cs.mu.Lock()
	cs.config.LastPushAt = time.Now().UnixMilli()
	cs.config.LastError = ""
	_ = cs.persistLocked()
	cs.mu.Unlock()

	return CloudSyncResult{
		Success: true,
		Message: fmt.Sprintf("已上传 %d 个配置文件到 %s", len(bundle.Files), key),
		Latency: time.Since(start).Milliseconds(),
	}
}

func (cs *CloudSyncService) DownloadFromCloud() CloudSyncResult {
	start := time.Now()
	cs.mu.Lock()
	cfg := cs.config
	cs.applying = true
	cs.mu.Unlock()
	defer func() {
		cs.mu.Lock()
		cs.applying = false
		cs.mu.Unlock()
	}()

	if !csConfigured(cfg) {
		return CloudSyncResult{Success: false, Message: "请先填写 Bucket 与密钥", Latency: time.Since(start).Milliseconds()}
	}

	client := newOSSObjectClient(cfg, cs.httpClient)
	key := cfg.ObjectKey
	if key == "" {
		key = defaultCloudKey
	}
	raw, err := client.Get(key)
	if err != nil {
		cs.recordError(err.Error())
		return CloudSyncResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}

	payload, err := decryptCloudPayload(raw, cfg.Passphrase)
	if err != nil {
		cs.recordError(err.Error())
		return CloudSyncResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}

	var bundle cloudBundle
	if err := json.Unmarshal(payload, &bundle); err != nil {
		cs.recordError("备份内容无法解析")
		return CloudSyncResult{Success: false, Message: "备份内容无法解析，请确认加密口令是否正确", Latency: time.Since(start).Milliseconds()}
	}

	n, err := cs.applyBundle(bundle)
	if err != nil {
		cs.recordError(err.Error())
		return CloudSyncResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}

	cs.mu.Lock()
	cs.config.LastPullAt = time.Now().UnixMilli()
	cs.config.LastError = ""
	_ = cs.persistLocked()
	cs.mu.Unlock()

	if cs.router != nil {
		_ = cs.router.ReloadFromDisk()
	}
	if cs.app != nil {
		_ = cs.app.RefreshConfig()
	}

	return CloudSyncResult{
		Success: true,
		Message: fmt.Sprintf("已从云端恢复 %d 个配置文件", n),
		Latency: time.Since(start).Milliseconds(),
	}
}

func csConfigured(cfg CloudConfig) bool {
	return strings.TrimSpace(cfg.Bucket) != "" &&
		strings.TrimSpace(cfg.AccessKey) != "" &&
		strings.TrimSpace(cfg.SecretKey) != ""
}

func (cs *CloudSyncService) recordError(msg string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.config.LastError = msg
	_ = cs.persistLocked()
}

func (cs *CloudSyncService) schedulePush() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if !cs.config.Enabled || !cs.config.AutoPush || cs.applying || !cs.isConfiguredLocked() {
		return
	}
	if cs.timer != nil {
		cs.timer.Stop()
	}
	cs.timer = time.AfterFunc(cloudDebounce, func() {
		_ = cs.UploadToCloud()
	})
}

func (cs *CloudSyncService) storeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func (cs *CloudSyncService) buildBundle() (*cloudBundle, error) {
	dir, err := cs.storeDir()
	if err != nil {
		return nil, err
	}
	host, _ := os.Hostname()
	bundle := &cloudBundle{
		Version:    cloudBackupVersion,
		ExportedAt: time.Now().UnixMilli(),
		Hostname:   host,
		Files:      map[string]json.RawMessage{},
	}

	addFile := func(name, path string) {
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			return
		}
		if !json.Valid(data) {
			return
		}
		bundle.Files[name] = json.RawMessage(data)
	}

	if cs.app != nil && strings.TrimSpace(cs.app.configPath) != "" {
		addFile("config.json", cs.app.configPath)
	} else {
		addFile("config.json", filepath.Join(dir, mainConfigFile))
	}
	addFile("mcp.json", filepath.Join(dir, mcpStoreFile))
	addFile("router.json", filepath.Join(dir, routerStoreFile))
	addFile("skills.json", filepath.Join(dir, skillsStoreFile))
	addFile("uptime.json", filepath.Join(dir, uptimeStoreFile))

	if len(bundle.Files) == 0 {
		return nil, fmt.Errorf("没有可上传的本地配置")
	}
	return bundle, nil
}

func (cs *CloudSyncService) applyBundle(bundle cloudBundle) (int, error) {
	dir, err := cs.storeDir()
	if err != nil {
		return 0, err
	}
	n := 0
	write := func(path string, raw json.RawMessage) error {
		if len(raw) == 0 {
			return nil
		}
		var pretty any
		if json.Unmarshal(raw, &pretty) != nil {
			return os.WriteFile(path, raw, 0o644)
		}
		data, err := json.MarshalIndent(pretty, "", "  ")
		if err != nil {
			data = raw
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return err
		}
		return os.Rename(tmp, path)
	}

	for name, raw := range bundle.Files {
		var path string
		switch name {
		case "config.json":
			if cs.app != nil && strings.TrimSpace(cs.app.configPath) != "" {
				path = cs.app.configPath
			} else {
				path = filepath.Join(dir, mainConfigFile)
			}
		case "mcp.json":
			path = filepath.Join(dir, mcpStoreFile)
		case "router.json":
			path = filepath.Join(dir, routerStoreFile)
		case "skills.json":
			path = filepath.Join(dir, skillsStoreFile)
		case "uptime.json":
			path = filepath.Join(dir, uptimeStoreFile)
		default:
			continue
		}
		if err := write(path, raw); err != nil {
			return n, fmt.Errorf("写入 %s 失败: %v", name, err)
		}
		n++
	}
	if n == 0 {
		return 0, fmt.Errorf("备份里没有可识别的配置文件")
	}
	return n, nil
}

func encryptCloudPayload(plain []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key := deriveCloudKey([]byte(passphrase), salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nil, nonce, plain, nil)
	out := make([]byte, 0, 4+len(salt)+len(nonce)+len(sealed))
	out = append(out, []byte(cloudMagic)...)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

func decryptCloudPayload(raw []byte, passphrase string) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("云端备份为空")
	}
	if raw[0] == '{' {
		return raw, nil
	}
	if len(raw) < 4+16+12+16 || string(raw[:4]) != cloudMagic {
		return nil, fmt.Errorf("不是本工具的备份格式")
	}
	if strings.TrimSpace(passphrase) == "" {
		return nil, fmt.Errorf("该备份已加密，请填写同样的加密口令")
	}
	salt := raw[4:20]
	key := deriveCloudKey([]byte(passphrase), salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < 20+nonceSize {
		return nil, fmt.Errorf("备份损坏")
	}
	nonce := raw[20 : 20+nonceSize]
	sealed := raw[20+nonceSize:]
	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败，请确认加密口令")
	}
	return plain, nil
}

func deriveCloudKey(passphrase, salt []byte) []byte {
	key := passphrase
	for i := 0; i < 20000; i++ {
		h := sha256.New()
		h.Write(key)
		h.Write(salt)
		h.Write([]byte{byte(i), byte(i >> 8), byte(i >> 16)})
		key = h.Sum(nil)
	}
	return key[:32]
}
