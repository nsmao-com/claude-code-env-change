package main

// 云端 OSS 配置同步：把环境/MCP/路由/Skills/监控配置与提示词打包上传，换电脑后凭同一套 OSS 凭证拉取。
// 历史版本与冲突检测见 cloud_history.go。
// 本地 API Key 仍明文存于各 json（与现有做法一致）；上传云端时若设置了口令则 AES-GCM 加密整包。

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
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
	// 以下由后端维护（SaveCloudConfig 不接受前端改写）
	DeviceID     string `json:"device_id,omitempty"`      // 本机标识，写进历史索引用于区分谁上传的
	LastRemoteAt int64  `json:"last_remote_at,omitempty"` // 上次同步时云端最新备份的导出时间
	LastSyncHash string `json:"last_sync_hash,omitempty"` // 上次同步时本机配置内容的指纹
}

// CloudSyncResult 一次上传/下载/测试的结果
type CloudSyncResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Latency int64  `json:"latency"`
	// Conflict 云端有别的电脑上传的新备份，本次未上传/未拉取
	Conflict bool `json:"conflict,omitempty"`
	// Skipped 没有需要做的事（例如启动时云端没有新备份）
	Skipped bool `json:"skipped,omitempty"`
	// 冲突时云端那份备份的来源与导出时间（毫秒），供界面按当前语言展示
	RemoteHost string `json:"remote_host,omitempty"`
	RemoteAt   int64  `json:"remote_at,omitempty"`
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
	Device     string                     `json:"device,omitempty"`
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
	// conflictNotified 已提示过的冲突（云端备份的导出时间），避免每次本地保存都弹一次
	conflictNotified int64
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
		result := cs.pullOnStartup()
		switch {
		case result.Skipped:
		case result.Conflict:
			cs.notifyConflict(result)
		case result.Success:
			cs.emit("cloud:pulled", result.Message)
		default:
			cs.emit("cloud:pull-failed", result.Message)
		}
	}
}

func (cs *CloudSyncService) emit(event, message string) {
	if cs.app != nil && cs.app.ctx != nil {
		wailsruntime.EventsEmit(cs.app.ctx, event, message)
	}
}

// notifyConflict 记下冲突并提示前端；同一份云端备份只提示一次
func (cs *CloudSyncService) notifyConflict(result CloudSyncResult) {
	cs.mu.Lock()
	cs.config.LastError = result.Message
	_ = cs.persistLocked()
	already := result.RemoteAt != 0 && cs.conflictNotified == result.RemoteAt
	cs.conflictNotified = result.RemoteAt
	cs.mu.Unlock()
	if !already && cs.app != nil && cs.app.ctx != nil {
		wailsruntime.EventsEmit(cs.app.ctx, "cloud:conflict", result)
	}
}

// deviceIDLocked 本机标识，首次使用时生成并立即落盘（每次启动都换的话会把自己上传的备份当成别人的）
func (cs *CloudSyncService) deviceIDLocked() string {
	if cs.config.DeviceID == "" {
		cs.config.DeviceID = newCloudDeviceID()
		_ = cs.persistLocked()
	}
	return cs.config.DeviceID
}

func cloudObjectKey(cfg CloudConfig) string {
	if strings.TrimSpace(cfg.ObjectKey) == "" {
		return defaultCloudKey
	}
	return cfg.ObjectKey
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
	cfg.DeviceID = cs.config.DeviceID
	cfg.LastRemoteAt = cs.config.LastRemoteAt
	cfg.LastSyncHash = cs.config.LastSyncHash
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
		return CloudSyncResult{Success: false, Message: tr("请先填写 Bucket、AccessKey 与 SecretKey"), Latency: time.Since(start).Milliseconds()}
	}
	client := newOSSObjectClient(cfg, cs.httpClient)
	key := cfg.ObjectKey
	if key == "" {
		key = defaultCloudKey
	}
	if err := client.Head(key); err != nil {
		return CloudSyncResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}
	return CloudSyncResult{Success: true, Message: tr("OSS 连接正常（凭证有效）"), Latency: time.Since(start).Milliseconds()}
}

// UploadToCloud 手动上传。云端有别的电脑的新备份时不覆盖，返回 Conflict 让界面确认
func (cs *CloudSyncService) UploadToCloud() CloudSyncResult {
	return cs.upload(false, false)
}

// ForceUploadToCloud 用户确认后覆盖云端（被覆盖的那份仍保留在历史版本里）
func (cs *CloudSyncService) ForceUploadToCloud() CloudSyncResult {
	return cs.upload(true, false)
}

// upload auto 为本地保存触发的自动上传：内容与上次同步相同时不传，免得无变化的保存刷掉历史版本
func (cs *CloudSyncService) upload(force, auto bool) CloudSyncResult {
	start := time.Now()
	fail := func(msg string) CloudSyncResult {
		cs.recordError(msg)
		return CloudSyncResult{Success: false, Message: msg, Latency: time.Since(start).Milliseconds()}
	}
	cs.mu.Lock()
	if cs.applying {
		cs.mu.Unlock()
		return CloudSyncResult{Success: true, Skipped: true, Message: tr("正在从云端恢复，跳过上传"), Latency: 0}
	}
	if cs.pushing {
		// 自动上传与手动上传并发时会互相覆盖历史索引
		cs.mu.Unlock()
		return CloudSyncResult{Success: false, Message: tr("正在上传，请稍后再试"), Latency: 0}
	}
	cfg := cs.config
	device := cs.deviceIDLocked()
	cs.pushing = true
	cs.mu.Unlock()
	defer func() {
		cs.mu.Lock()
		cs.pushing = false
		cs.mu.Unlock()
	}()

	if !csConfigured(cfg) {
		return CloudSyncResult{Success: false, Message: tr("请先填写 Bucket 与密钥"), Latency: time.Since(start).Milliseconds()}
	}

	bundle, err := cs.buildBundle()
	if err != nil {
		return fail(err.Error())
	}
	bundle.Device = device
	contentHash := bundleContentHash(bundle.Files)
	if auto && cfg.LastSyncHash != "" && contentHash == cfg.LastSyncHash {
		return CloudSyncResult{Success: true, Skipped: true, Message: tr("本机配置没有变化，无需上传"), Latency: time.Since(start).Milliseconds()}
	}

	client := newOSSObjectClient(cfg, cs.httpClient)
	key := cloudObjectKey(cfg)
	idx, err := readCloudIndex(client, key, cfg.Passphrase)
	if err != nil {
		return fail(tr("读取云端历史失败: ") + err.Error())
	}
	if !force {
		if other := idx.newerRemote(cfg.LastRemoteAt, device); other != nil {
			return CloudSyncResult{
				Success:    false,
				Conflict:   true,
				Message:    cloudConflictMessage(other) + tr("，已暂停上传。可以先拉取云端，或确认后覆盖（被覆盖的备份仍保留在历史版本里）"),
				Latency:    time.Since(start).Milliseconds(),
				RemoteHost: other.Hostname,
				RemoteAt:   other.ExportedAt,
			}
		}
	}
	// 导出时间同时是历史对象名，必须比云端最新一份大：同一毫秒连传两次、
	// 或别的电脑时钟偏快时，都不能撞名覆盖历史
	if latest := idx.latest(); latest != nil && bundle.ExportedAt <= latest.ExportedAt {
		bundle.ExportedAt = latest.ExportedAt + 1
	}

	payload, err := json.Marshal(bundle)
	if err != nil {
		return CloudSyncResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}
	contentType := "application/json"
	if strings.TrimSpace(cfg.Passphrase) != "" {
		enc, err := encryptCloudPayload(payload, cfg.Passphrase)
		if err != nil {
			return fail(tr("加密失败: ") + err.Error())
		}
		payload = enc
		contentType = "application/octet-stream"
	}

	// 顺序：历史对象 → 索引 → 主对象。索引写失败就不动主对象；
	// 主对象写失败时索引里已有本机这份，下次上传不会误判成别人的备份
	versionKey := cloudVersionKey(key, bundle.ExportedAt)
	if err := client.Put(versionKey, payload, contentType); err != nil {
		return fail(err.Error())
	}
	entry := CloudVersion{
		Key:        versionKey,
		ExportedAt: bundle.ExportedAt,
		Hostname:   bundle.Hostname,
		Device:     device,
		Files:      len(bundle.Files),
		Size:       len(payload),
	}
	idx.Versions = append([]CloudVersion{entry}, idx.Versions...)
	var dropped []CloudVersion
	if len(idx.Versions) > cloudHistoryKeep {
		dropped = append(dropped, idx.Versions[cloudHistoryKeep:]...)
		idx.Versions = idx.Versions[:cloudHistoryKeep]
	}
	if err := writeCloudIndex(client, key, cfg.Passphrase, idx); err != nil {
		return fail(tr("更新云端历史失败: ") + err.Error())
	}
	if err := client.Put(key, payload, contentType); err != nil {
		return fail(err.Error())
	}
	for _, v := range dropped {
		if isCloudVersionKey(key, v.Key) {
			_ = client.Delete(v.Key)
		}
	}

	cs.mu.Lock()
	cs.config.LastPushAt = time.Now().UnixMilli()
	cs.config.LastError = ""
	cs.config.LastRemoteAt = bundle.ExportedAt
	cs.config.LastSyncHash = contentHash
	cs.conflictNotified = 0
	_ = cs.persistLocked()
	cs.mu.Unlock()

	return CloudSyncResult{
		Success: true,
		Message: sprintf("已上传 %d 个配置文件到 %s（云端保留最近 %d 份历史）", len(bundle.Files), key, cloudHistoryKeep),
		Latency: time.Since(start).Milliseconds(),
	}
}

// beginApply 进入"从云端恢复"状态；正在上传时拒绝
func (cs *CloudSyncService) beginApply() (CloudConfig, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.pushing || cs.applying {
		return CloudConfig{}, false
	}
	cs.applying = true
	return cs.config, true
}

func (cs *CloudSyncService) endApply() {
	cs.mu.Lock()
	cs.applying = false
	cs.mu.Unlock()
}

// DownloadFromCloud 手动拉取云端最新备份（界面已确认覆盖本机）
func (cs *CloudSyncService) DownloadFromCloud() CloudSyncResult {
	start := time.Now()
	cfg, ok := cs.beginApply()
	if !ok {
		return CloudSyncResult{Success: false, Message: tr("正在上传或恢复备份，请稍后再拉取"), Latency: 0}
	}
	defer cs.endApply()
	if !csConfigured(cfg) {
		return CloudSyncResult{Success: false, Message: tr("请先填写 Bucket 与密钥"), Latency: time.Since(start).Milliseconds()}
	}
	return cs.restoreObject(cfg, cloudObjectKey(cfg), 0, start)
}

// pullOnStartup 启动时自动拉取：云端没有新备份就跳过；
// 本机有未上传的修改、云端又有别的电脑的新备份时不覆盖，交给用户决定
func (cs *CloudSyncService) pullOnStartup() CloudSyncResult {
	start := time.Now()
	cfg, ok := cs.beginApply()
	if !ok {
		return CloudSyncResult{Success: false, Message: tr("正在上传或恢复备份"), Latency: 0}
	}
	defer cs.endApply()
	if !csConfigured(cfg) {
		return CloudSyncResult{Success: false, Message: tr("请先填写 Bucket 与密钥"), Latency: time.Since(start).Milliseconds()}
	}
	cs.mu.Lock()
	device := cs.deviceIDLocked()
	cs.mu.Unlock()

	client := newOSSObjectClient(cfg, cs.httpClient)
	key := cloudObjectKey(cfg)
	idx, err := readCloudIndex(client, key, cfg.Passphrase)
	if err != nil {
		cs.recordError(err.Error())
		return CloudSyncResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}
	dirty := cs.localDirty(cfg)
	if latest := idx.latest(); latest != nil {
		other := idx.newerRemote(cfg.LastRemoteAt, device)
		if other == nil {
			return CloudSyncResult{Success: true, Skipped: true, Message: tr("云端没有新的备份"), Latency: time.Since(start).Milliseconds()}
		}
		if dirty {
			return CloudSyncResult{
				Success:    false,
				Conflict:   true,
				Message:    cloudConflictMessage(other) + tr("，本机也有未上传的修改，已跳过启动时的自动拉取。请在云同步页选择拉取云端或上传本机"),
				Latency:    time.Since(start).Milliseconds(),
				RemoteHost: other.Hostname,
				RemoteAt:   other.ExportedAt,
			}
		}
	} else if dirty {
		// 云端没有历史索引（旧版本上传的备份），判断不了是不是新的；本机有改动时不冒险覆盖
		return CloudSyncResult{
			Success:  false,
			Conflict: true,
			Message:  tr("本机有未上传的修改，已跳过启动时的自动拉取。请在云同步页选择拉取云端或上传本机"),
			Latency:  time.Since(start).Milliseconds(),
		}
	}
	return cs.restoreObject(cfg, key, 0, start)
}

// ListCloudVersions 云端保留的历史备份，新的在前
func (cs *CloudSyncService) ListCloudVersions() ([]CloudVersion, error) {
	cs.mu.Lock()
	cfg := cs.config
	cs.mu.Unlock()
	if !csConfigured(cfg) {
		return []CloudVersion{}, nil
	}
	idx, err := readCloudIndex(newOSSObjectClient(cfg, cs.httpClient), cloudObjectKey(cfg), cfg.Passphrase)
	if err != nil {
		return nil, err
	}
	if idx.Versions == nil {
		return []CloudVersion{}, nil
	}
	return idx.Versions, nil
}

// RestoreCloudVersion 恢复到某一份历史备份（只接受索引里列出的版本）
func (cs *CloudSyncService) RestoreCloudVersion(versionKey string) CloudSyncResult {
	start := time.Now()
	cfg, ok := cs.beginApply()
	if !ok {
		return CloudSyncResult{Success: false, Message: tr("正在上传或恢复备份，请稍后再试"), Latency: 0}
	}
	defer cs.endApply()
	if !csConfigured(cfg) {
		return CloudSyncResult{Success: false, Message: tr("请先填写 Bucket 与密钥"), Latency: time.Since(start).Milliseconds()}
	}
	key := cloudObjectKey(cfg)
	idx, err := readCloudIndex(newOSSObjectClient(cfg, cs.httpClient), key, cfg.Passphrase)
	if err != nil {
		return CloudSyncResult{Success: false, Message: err.Error(), Latency: time.Since(start).Milliseconds()}
	}
	if idx.find(versionKey) == nil {
		return CloudSyncResult{Success: false, Message: tr("云端没有这份历史备份，请刷新列表"), Latency: time.Since(start).Milliseconds()}
	}
	// 用户是看过最新版本后主动选的旧版本：同步基准记为最新版本，之后上传不会被当成冲突
	return cs.restoreObject(cfg, versionKey, idx.latest().ExportedAt, start)
}

// restoreObject 下载指定对象并恢复到本机。remoteAt 为 0 时以备份自身的导出时间作为同步基准
func (cs *CloudSyncService) restoreObject(cfg CloudConfig, objectKey string, remoteAt int64, start time.Time) CloudSyncResult {
	fail := func(msg string) CloudSyncResult {
		cs.recordError(msg)
		return CloudSyncResult{Success: false, Message: msg, Latency: time.Since(start).Milliseconds()}
	}
	raw, err := newOSSObjectClient(cfg, cs.httpClient).Get(objectKey)
	if err != nil {
		return fail(err.Error())
	}
	payload, err := decryptCloudPayload(raw, cfg.Passphrase)
	if err != nil {
		return fail(err.Error())
	}
	var bundle cloudBundle
	if err := json.Unmarshal(payload, &bundle); err != nil {
		cs.recordError(tr("备份内容无法解析"))
		return CloudSyncResult{Success: false, Message: tr("备份内容无法解析，请确认加密口令是否正确"), Latency: time.Since(start).Milliseconds()}
	}
	message, err := cs.restoreBundle(bundle)
	if err != nil {
		return fail(err.Error())
	}
	if remoteAt == 0 {
		remoteAt = bundle.ExportedAt
	}

	syncHash := ""
	if local, err := cs.buildBundle(); err == nil {
		syncHash = bundleContentHash(local.Files)
	}
	cs.mu.Lock()
	cs.config.LastPullAt = time.Now().UnixMilli()
	cs.config.LastError = ""
	cs.config.LastRemoteAt = remoteAt
	cs.config.LastSyncHash = syncHash
	cs.conflictNotified = 0
	_ = cs.persistLocked()
	cs.mu.Unlock()

	return CloudSyncResult{Success: true, Message: message, Latency: time.Since(start).Milliseconds()}
}

// localDirty 本机配置自上次同步以来是否改过（从没同步过时无从比较，按没改过处理）
func (cs *CloudSyncService) localDirty(cfg CloudConfig) bool {
	if cfg.LastSyncHash == "" {
		return false
	}
	local, err := cs.buildBundle()
	if err != nil {
		return false
	}
	return bundleContentHash(local.Files) != cfg.LastSyncHash
}

// restoreBundle 把备份写回本机：先覆盖中央存储，再让各服务重新加载，
// 最后把 MCP 与 Skills 写回各平台文件（否则换电脑后平台里没有这些条目，
// 下次加载会把启用标记全部清空）
func (cs *CloudSyncService) restoreBundle(bundle cloudBundle) (string, error) {
	n, err := cs.applyBundle(bundle)
	if err != nil {
		return "", err
	}
	if cs.router != nil {
		_ = cs.router.ReloadFromDisk()
	}
	if cs.app != nil {
		_ = cs.app.RefreshConfig()
	}

	message := sprintf("已从云端恢复 %d 个配置文件", n)
	var syncErrs []string
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
		message += tr("；⚠ 写回平台失败: ") + strings.Join(syncErrs, "；")
	}
	return message, nil
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
		if result := cs.upload(false, true); result.Conflict {
			cs.notifyConflict(result)
		}
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
			return errorf("读取 %s 失败: %v", path, err)
		}
		if !json.Valid(data) {
			return errorf("%s 不是有效 JSON，拒绝上传", path)
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
	addOptional := func(name, path string) {
		if err := addFile(name, path); err != nil {
			delete(bundle.Files, name)
		}
	}
	addOptional("mcp.json", filepath.Join(dir, mcpStoreFile))
	addOptional("router.json", filepath.Join(dir, routerStoreFile))
	addOptional("skills.json", filepath.Join(dir, skillsStoreFile))
	addOptional("uptime.json", filepath.Join(dir, uptimeStoreFile))
	addOptional(projectsStoreFile, filepath.Join(dir, projectsStoreFile))
	addOptional(budgetStoreFile, filepath.Join(dir, budgetStoreFile))
	if prompts := collectPromptFiles(); len(prompts) > 0 {
		if data, err := json.Marshal(prompts); err == nil {
			bundle.Files[cloudPromptsFile] = data
		}
	}

	if len(bundle.Files) == 0 {
		return nil, errorf("没有可上传的本地配置")
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
		case projectsStoreFile:
			path = filepath.Join(dir, projectsStoreFile)
		case budgetStoreFile:
			path = filepath.Join(dir, budgetStoreFile)
		case cloudPromptsFile:
			if err := restorePromptFiles(raw); err != nil {
				return n, errorf("写回提示词失败: %v", err)
			}
			n++
			continue
		default:
			continue
		}
		if err := write(path, raw); err != nil {
			return n, errorf("写入 %s 失败: %v", name, err)
		}
		n++
	}
	if n == 0 {
		return 0, errorf("备份里没有可识别的配置文件")
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
		return nil, errorf("云端备份为空")
	}
	hasPassphrase := strings.TrimSpace(passphrase) != ""
	if raw[0] == '{' {
		// 本机设了口令却拉到明文：可能是有人拿到了存储桶写权限，塞进一份
		// 改过 Base URL 的明文备份来截获 Key。宁可拒绝，也不静默套用。
		if hasPassphrase {
			return nil, errorf("云端备份未加密，但本机设置了加密口令；为防篡改已拒绝恢复。确认备份可信时，请先清空口令再拉取")
		}
		return raw, nil
	}
	if len(raw) < 4+16+12+16 {
		return nil, errorf("不是本工具的备份格式")
	}
	magic := string(raw[:4])
	if magic != cloudMagic && magic != cloudMagicLegacy {
		return nil, errorf("不是本工具的备份格式")
	}
	if !hasPassphrase {
		return nil, errorf("该备份已加密，请填写同样的加密口令")
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
		return nil, errorf("备份损坏")
	}
	nonce := raw[20 : 20+nonceSize]
	sealed := raw[20+nonceSize:]
	plain, err := gcm.Open(nil, nonce, sealed, aad)
	if err != nil {
		return nil, errorf("解密失败，请确认加密口令")
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
