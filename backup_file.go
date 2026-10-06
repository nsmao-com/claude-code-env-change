package main

// 本地加密备份文件：不依赖对象存储，把环境、MCP、Skills、路由、监控与工作台配置
// 用口令加密（scrypt + AES-256-GCM，与云同步相同格式）导出成一个文件，
// 换电脑时导入，预览差异后按文件选择恢复。

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const backupFileExt = ".aienv-backup"

var backupFilePending = struct {
	sync.Mutex
	token     string
	localHash string
	bundle    cloudBundle
	at        time.Time
}{}

func validateBackupPassphrase(p string) error {
	if len([]rune(strings.TrimSpace(p))) < 8 {
		return fmt.Errorf("加密口令至少 8 个字符")
	}
	return nil
}

// ExportBackupFile 导出加密备份文件，返回保存路径（取消时为空）
func (cs *CloudSyncService) ExportBackupFile(passphrase string) (string, error) {
	if err := validateBackupPassphrase(passphrase); err != nil {
		return "", err
	}
	if cs.app == nil || cs.app.ctx == nil {
		return "", fmt.Errorf("应用未就绪")
	}
	bundle, err := cs.buildBundle()
	if err != nil {
		return "", err
	}
	plain, err := json.Marshal(bundle)
	if err != nil {
		return "", err
	}
	sealed, err := encryptCloudPayload(plain, passphrase)
	if err != nil {
		return "", err
	}
	target, err := runtime.SaveFileDialog(cs.app.ctx, runtime.SaveDialogOptions{
		Title:           "导出加密备份",
		DefaultFilename: "aienv-" + time.Now().Format("20060102-1504") + backupFileExt,
		Filters:         []runtime.FileFilter{{DisplayName: "AI ENV 备份", Pattern: "*" + backupFileExt}},
	})
	if err != nil || target == "" {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(target), backupFileExt) {
		target += backupFileExt
	}
	return target, writeFileAtomic(target, sealed, 0o600)
}

// PreviewBackupFile 选择备份文件并解密，返回与本机配置的差异预览
func (cs *CloudSyncService) PreviewBackupFile(passphrase string) (HistoryPreview, error) {
	if strings.TrimSpace(passphrase) == "" {
		return HistoryPreview{}, fmt.Errorf("请填写导出时设置的加密口令")
	}
	if cs.app == nil || cs.app.ctx == nil {
		return HistoryPreview{}, fmt.Errorf("应用未就绪")
	}
	path, err := runtime.OpenFileDialog(cs.app.ctx, runtime.OpenDialogOptions{
		Title:   "选择加密备份",
		Filters: []runtime.FileFilter{{DisplayName: "AI ENV 备份", Pattern: "*" + backupFileExt}},
	})
	if err != nil || path == "" {
		return HistoryPreview{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return HistoryPreview{}, err
	}
	if info.Size() > 64<<20 {
		return HistoryPreview{}, fmt.Errorf("备份文件超过 64 MB，不是有效的 AI ENV 备份")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return HistoryPreview{}, err
	}
	if len(raw) > 0 && raw[0] == '{' {
		return HistoryPreview{}, fmt.Errorf("这不是加密备份文件；普通 JSON 配置请用「导入配置」")
	}
	plain, err := decryptCloudPayload(raw, passphrase)
	if err != nil {
		return HistoryPreview{}, err
	}
	var bundle cloudBundle
	if json.Unmarshal(plain, &bundle) != nil || bundle.Version != cloudBackupVersion {
		return HistoryPreview{}, fmt.Errorf("备份格式或版本不受支持")
	}
	local, err := cs.buildBundle()
	if err != nil {
		return HistoryPreview{}, err
	}
	preview := HistoryPreview{Token: newRecordID(), Changes: []FileChange{}}
	allowed := map[string]bool{"config.json": true, "mcp.json": true, "router.json": true, "skills.json": true, "uptime.json": true, "workbench.json": true}
	for name, data := range bundle.Files {
		if !allowed[name] {
			delete(bundle.Files, name)
			continue
		}
		if !json.Valid(data) {
			return preview, fmt.Errorf("%s 内容损坏", name)
		}
		preview.Changes = append(preview.Changes, FileChange{name, redactConfig(local.Files[name]), redactConfig(data), contentHash(local.Files[name]) != contentHash(data)})
	}
	if len(preview.Changes) == 0 {
		return preview, fmt.Errorf("备份不含可恢复配置")
	}
	if err := validateCloudBundle(bundle); err != nil {
		return preview, err
	}
	sort.Slice(preview.Changes, func(i, j int) bool { return preview.Changes[i].Path < preview.Changes[j].Path })
	backupFilePending.Lock()
	backupFilePending.token, backupFilePending.localHash = preview.Token, bundleFingerprint(*local)
	backupFilePending.bundle, backupFilePending.at = bundle, time.Now()
	backupFilePending.Unlock()
	return preview, nil
}

// ConfirmBackupFileRestore 恢复预览中选中的文件
func (cs *CloudSyncService) ConfirmBackupFileRestore(token string, names []string) CloudSyncResult {
	backupFilePending.Lock()
	valid := backupFilePending.token != "" && backupFilePending.token == token && time.Since(backupFilePending.at) <= 10*time.Minute
	bundle, localHash := backupFilePending.bundle, backupFilePending.localHash
	backupFilePending.Unlock()
	if !valid {
		return CloudSyncResult{Message: "预览已失效，请重新选择备份文件"}
	}
	if len(names) == 0 {
		return CloudSyncResult{Message: "请选择要恢复的配置"}
	}
	cs.mu.Lock()
	if cs.applying || cs.pushing {
		cs.mu.Unlock()
		return CloudSyncResult{Message: "云同步进行中，请稍后重试"}
	}
	cs.applying = true
	cs.mu.Unlock()
	defer func() { cs.mu.Lock(); cs.applying = false; cs.mu.Unlock() }()
	local, err := cs.buildBundle()
	if err != nil {
		return CloudSyncResult{Message: err.Error()}
	}
	if bundleFingerprint(*local) != localHash {
		return CloudSyncResult{Message: "本地在预览后发生变化，请重新预览"}
	}
	selected := cloudBundle{Version: cloudBackupVersion, Files: map[string]json.RawMessage{}}
	for _, n := range names {
		data, ok := bundle.Files[n]
		if !ok {
			return CloudSyncResult{Message: "选择的文件不在预览中"}
		}
		selected.Files[n] = data
	}
	msg, err := cs.restoreBundle(selected)
	if err != nil {
		return CloudSyncResult{Message: err.Error()}
	}
	backupFilePending.Lock()
	backupFilePending.token = ""
	backupFilePending.Unlock()
	return CloudSyncResult{Success: true, Message: strings.Replace(msg, "从云端", "从备份文件", 1)}
}
