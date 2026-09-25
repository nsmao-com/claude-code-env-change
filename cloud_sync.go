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

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/scrypt"
)

const (
	cloudStoreFile     = "cloud.json"
	cloudBackupVersion = 1
	cloudMagic         = "CEB2"
	cloudMagicLegacy   = "CEB1"
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
	Enabled    bool   `json:"enabled"`
	Provider   string `json:"provider"` // s3 | aliyun | tencent | r2 | minio | custom
	Endpoint   string `json:"endpoint"`
	Region     string `json:"region"`
	Bucket     string `json:"bucket"`
	ObjectKey  string `json:"object_key"`
	AccessKey  string `json:"access_key"`
	SecretKey  string `json:"secret_key"`
	PathStyle  bool   `json:"path_style"`
	Passphrase string `json:"passphrase,omitempty"`
	// ClearSecrets 仅作为 SaveCloudConfig 的入参标志：置 true 时清空已保存的
	// SecretKey/Passphrase（持久化前会复位，不落盘）
	ClearSecrets    bool   `json:"clear_secrets,omitempty"`
	AutoPush        bool   `json:"auto_push"`
	AutoPullOnStart bool   `json:"auto_pull_on_start"`
	LastPushAt      int64  `json:"last_push_at,omitempty"`
	LastPullAt      int64  `json:"last_pull_at,omitempty"`
	LastError       string `json:"last_error,omitempty"`
	RemoteETag      string `json:"remote_etag,omitempty"`
	LocalBaseline   string `json:"local_baseline,omitempty"`
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
	mcp        *MCPService
	skills     *SkillService
	httpClient *http.Client
	timer      *time.Timer
	pushing    bool
	applying   bool
	pending    *cloudRestorePending
}

func NewCloudSyncService(app *App, router *RouterService, mcp *MCPService, skills *SkillService) *CloudSyncService {
	cs := &CloudSyncService{
		app:        app,
		router:     router,
		mcp:        mcp,
		skills:     skills,
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
		// OnStartup 与前端加载并行：拉取可能在界面读完配置之后才完成，
		// 必须通知前端刷新，否则界面一直显示拉取前的旧数据；失败也要让用户知道
		result := cs.DownloadFromCloud()
		if cs.app != nil && cs.app.ctx != nil {
			if result.Success {
				wailsruntime.EventsEmit(cs.app.ctx, "cloud:pulled", result.Message)
			} else {
				wailsruntime.EventsEmit(cs.app.ctx, "cloud:pull-failed", result.Message)
			}
		}
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
	// cloud.json 含对象存储 SecretKey 与加密口令，仅本人可读
	return writeFileAtomic(path, data, 0o600)
}

func (cs *CloudSyncService) isConfiguredLocked() bool {
	return csConfigured(cs.config)
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
	if cfg.Provider == cs.config.Provider && cfg.Endpoint == cs.config.Endpoint && cfg.Bucket == cs.config.Bucket && cfg.ObjectKey == cs.config.ObjectKey {
		cfg.RemoteETag = cs.config.RemoteETag
		cfg.LocalBaseline = cs.config.LocalBaseline
	} else {
		cfg.RemoteETag = ""
		cfg.LocalBaseline = ""
	}
	if cfg.ClearSecrets {
		// 显式清除凭证：空 SecretKey 不再回落旧值，用户可以在 UI 里换号
		cfg.SecretKey = ""
		cfg.Passphrase = ""
		cfg.ClearSecrets = false
	} else if strings.TrimSpace(cfg.SecretKey) == "" {
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
	if !csConfigured(cfg) {
		return CloudSyncResult{Success: false, Message: "请先填写 Bucket、AccessKey 与 SecretKey", Latency: time.Since(start).Milliseconds()}
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
	if cs.applying || cs.pushing {
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
	etag, exists, err := client.Version(key)
	if err != nil {
		cs.recordError(err.Error())
		return CloudSyncResult{Message: err.Error()}
	}
	if exists && (cfg.RemoteETag == "" || cfg.RemoteETag != etag) {
		msg := "云端已有未确认的新版本，请先预览恢复并处理差异"
		cs.recordError(msg)
		return CloudSyncResult{Message: msg}
	}
	if exists && etag == "" {
		return CloudSyncResult{Message: "存储服务未返回 ETag，无法安全覆盖备份"}
	}
	client.etag = etag
	if !exists {
		client.etag = "*new*"
	}
	if err := client.Put(key, payload, contentType); err != nil {
		cs.recordError(err.Error())
		return CloudSyncResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}

	cs.mu.Lock()
	cs.config.RemoteETag = client.etag
	cs.config.LocalBaseline = bundleFingerprint(*bundle)
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
	preview, err := cs.PreviewCloudRestore()
	if err != nil {
		return CloudSyncResult{Message: err.Error()}
	}
	cs.mu.Lock()
	baseline := cs.config.LocalBaseline
	cs.mu.Unlock()
	local, err := cs.buildBundle()
	if err != nil {
		return CloudSyncResult{Message: err.Error()}
	}
	if baseline == "" || baseline != bundleFingerprint(*local) {
		return CloudSyncResult{Message: "本地有未确认的配置，请在云同步页预览并选择恢复文件"}
	}
	names := []string{}
	for _, c := range preview.Changes {
		names = append(names, c.Path)
	}
	return cs.ConfirmCloudRestore(preview.Token, names)
}

// restoreBundle 把备份写回本机：先覆盖中央存储，再让各服务重新加载，
// 最后把 MCP 与 Skills 写回各平台文件（否则换电脑后平台里没有这些条目，
// 下次加载会把启用标记全部清空）
func (cs *CloudSyncService) restoreBundle(bundle cloudBundle) (string, error) {
	n, err := cs.applyBundle(bundle)
	if err != nil {
		return "", err
	}
	var syncErrs []string
	if _, ok := bundle.Files["router.json"]; ok && cs.router != nil {
		if err := cs.router.ReloadFromDisk(); err != nil {
			syncErrs = append(syncErrs, "Router: "+err.Error())
		}
	}
	if _, ok := bundle.Files["config.json"]; ok && cs.app != nil {
		if err := cs.app.RefreshConfig(); err != nil {
			syncErrs = append(syncErrs, "Config: "+err.Error())
		}
	}

	message := fmt.Sprintf("已从云端恢复 %d 个配置文件", n)
	if _, ok := bundle.Files["mcp.json"]; ok && cs.mcp != nil {
		if err := cs.mcp.applyStoreToPlatforms(); err != nil {
			syncErrs = append(syncErrs, "MCP: "+err.Error())
		}
	}
	if _, ok := bundle.Files["skills.json"]; ok && cs.skills != nil {
		if err := cs.skills.applyStoreToPlatforms(); err != nil {
			syncErrs = append(syncErrs, "Skills: "+err.Error())
		}
	}
	if len(syncErrs) > 0 {
		return "", fmt.Errorf("%s；部分平台未生效，请修复后重试或从配置历史恢复: %s", message, strings.Join(syncErrs, "；"))
	}
	return message, nil
}

func csConfigured(cfg CloudConfig) bool {
	if cfg.Provider == "webdav" {
		return validEndpoint(cfg.Endpoint) == nil && strings.TrimSpace(cfg.AccessKey) != "" && cfg.SecretKey != ""
	}
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

	addFile := func(name, path string) error {
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			return fmt.Errorf("读取 %s 失败: %v", path, err)
		}
		if !json.Valid(data) {
			return fmt.Errorf("%s 不是有效 JSON，拒绝上传", path)
		}
		bundle.Files[name] = json.RawMessage(data)
		return nil
	}

	// config.json 是备份的核心：读不到时宁可不传，也不能用残缺包覆盖云端的好备份
	if cs.app != nil && strings.TrimSpace(cs.app.configPath) != "" {
		if err := addFile("config.json", cs.app.configPath); err != nil {
			return nil, err
		}
	} else {
		if err := addFile("config.json", filepath.Join(dir, mainConfigFile)); err != nil {
			return nil, err
		}
	}
	for _, name := range []string{mcpStoreFile, routerStoreFile, skillsStoreFile, uptimeStoreFile, "workbench.json"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		if err := addFile(name, p); err != nil {
			return nil, err
		}
	}

	if len(bundle.Files) == 0 {
		return nil, fmt.Errorf("没有可上传的本地配置")
	}
	return bundle, nil
}

func (cs *CloudSyncService) applyBundle(bundle cloudBundle) (int, error) {
	if err := validateCloudBundle(bundle); err != nil {
		return 0, err
	}
	dir, err := cs.storeDir()
	if err != nil {
		return 0, err
	}
	n := 0
	type previousFile struct {
		path   string
		data   []byte
		exists bool
	}
	previous := []previousFile{}
	rollback := func(cause error) (int, error) {
		errs := []string{}
		for i := len(previous) - 1; i >= 0; i-- {
			old := previous[i]
			var e error
			if old.exists {
				e = writeFileAtomic(old.path, old.data, 0600)
			} else {
				e = os.Remove(old.path)
			}
			if e != nil && !os.IsNotExist(e) {
				errs = append(errs, e.Error())
			}
		}
		if len(errs) > 0 {
			return 0, fmt.Errorf("%v；恢复原配置失败，请查看配置历史: %s", cause, strings.Join(errs, "；"))
		}
		return 0, fmt.Errorf("%v；已恢复原配置", cause)
	}
	write := func(path string, raw json.RawMessage) error {
		if len(raw) == 0 {
			return nil
		}
		var pretty any
		if json.Unmarshal(raw, &pretty) != nil {
			backupFile(path)
			return writeFileAtomic(path, raw, 0o600)
		}
		data, err := json.MarshalIndent(pretty, "", "  ")
		if err != nil {
			data = raw
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		// 云端内容即将覆盖本地文件，覆盖前留一份 .bak 作为最后防线；
		// 恢复出来的配置都可能含 API Key，仅本人可读
		backupFile(path)
		return writeFileAtomic(path, data, 0o600)
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
		case "workbench.json":
			path = filepath.Join(dir, "workbench.json")
		default:
			continue
		}
		old, readErr := os.ReadFile(path)
		if readErr != nil && !os.IsNotExist(readErr) {
			return rollback(readErr)
		}
		previous = append(previous, previousFile{path, old, readErr == nil})
		if err := write(path, raw); err != nil {
			return rollback(fmt.Errorf("写入 %s 失败: %v", name, err))
		}
		n++
	}
	if n == 0 {
		return 0, fmt.Errorf("备份里没有可识别的配置文件")
	}
	return n, nil
}

// 备份密文格式：magic(4) | salt(16) | nonce(12) | AES-256-GCM 密文。
// CEB2 用 scrypt 派生密钥；CEB1 是早期版本的 2 万轮 SHA-256 迭代，只保留解密兼容。
func encryptCloudPayload(plain []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key, err := deriveCloudKey([]byte(passphrase), salt)
	if err != nil {
		return nil, err
	}
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
	// magic 作为附加认证数据：篡改版本号把新密文降级成旧 KDF 解析会直接认证失败
	sealed := gcm.Seal(nil, nonce, plain, []byte(cloudMagic))
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
	hasPassphrase := strings.TrimSpace(passphrase) != ""
	if raw[0] == '{' {
		// 本机设了口令却拉到明文：可能是有人拿到了存储桶写权限，塞进一份
		// 改过 Base URL 的明文备份来截获 Key。宁可拒绝，也不静默套用。
		if hasPassphrase {
			return nil, fmt.Errorf("云端备份未加密，但本机设置了加密口令；为防篡改已拒绝恢复。确认备份可信时，请先清空口令再拉取")
		}
		return raw, nil
	}
	if len(raw) < 4+16+12+16 {
		return nil, fmt.Errorf("不是本工具的备份格式")
	}
	magic := string(raw[:4])
	if magic != cloudMagic && magic != cloudMagicLegacy {
		return nil, fmt.Errorf("不是本工具的备份格式")
	}
	if !hasPassphrase {
		return nil, fmt.Errorf("该备份已加密，请填写同样的加密口令")
	}
	salt := raw[4:20]
	var (
		key []byte
		aad []byte
		err error
	)
	if magic == cloudMagic {
		key, err = deriveCloudKey([]byte(passphrase), salt)
		if err != nil {
			return nil, err
		}
		aad = []byte(cloudMagic)
	} else {
		key = deriveLegacyCloudKey([]byte(passphrase), salt)
	}
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
	plain, err := gcm.Open(nil, nonce, sealed, aad)
	if err != nil {
		return nil, fmt.Errorf("解密失败，请确认加密口令")
	}
	return plain, nil
}

// deriveCloudKey 用 scrypt（N=2^15, r=8, p=1）从口令派生 AES-256 密钥，
// 单次约 32MB 内存，显著抬高离线暴力破解口令的成本。
func deriveCloudKey(passphrase, salt []byte) ([]byte, error) {
	return scrypt.Key(passphrase, salt, 1<<15, 8, 1, 32)
}

// deriveLegacyCloudKey CEB1 备份使用的旧 KDF，只用于解密旧备份
func deriveLegacyCloudKey(passphrase, salt []byte) []byte {
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
