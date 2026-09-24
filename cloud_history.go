package main

// 云同步历史版本与冲突检测。
//
// 每次上传除了覆盖主对象（ObjectKey），还会另存一份 <ObjectKey>.history/<导出时间>.bin，
// 并维护索引 <ObjectKey>.index.json（设了口令时与备份一样加密），保留最近 cloudHistoryKeep 份。
//
// 冲突判断：本机记住上次同步（上传或拉取）时云端最新版本的导出时间 LastRemoteAt。
// 云端最新版本既不是这一份、也不是本机传的，说明别的电脑在那之后上传过：
// 自动上传此时暂停并提示，手动上传需确认；启动时的自动拉取遇到本机有未上传的修改也会跳过。
// 本机是否有未上传的修改按内容判断（规范化 JSON 后取哈希），不依赖保存事件，
// 格式化差异或没有实际变化的保存不会误判。

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	cloudHistoryKeep   = 10
	cloudIndexSuffix   = ".index.json"
	cloudHistoryPrefix = ".history/"
	cloudPromptsFile   = "prompts.json"
)

// CloudVersion 云端的一份历史备份
type CloudVersion struct {
	Key        string `json:"key"`
	ExportedAt int64  `json:"exported_at"`
	Hostname   string `json:"hostname,omitempty"`
	Device     string `json:"device,omitempty"`
	Files      int    `json:"files"`
	Size       int    `json:"size"`
}

type cloudIndex struct {
	Versions []CloudVersion `json:"versions"`
}

func (idx cloudIndex) latest() *CloudVersion {
	if len(idx.Versions) == 0 {
		return nil
	}
	return &idx.Versions[0]
}

// newerRemote 云端最新版本是别的电脑在本机上次同步之后上传的，返回它；否则返回 nil
func (idx cloudIndex) newerRemote(lastRemoteAt int64, device string) *CloudVersion {
	latest := idx.latest()
	if latest == nil || latest.ExportedAt == lastRemoteAt {
		return nil
	}
	if device != "" && latest.Device == device {
		// 本机自己传的（例如上次写完索引后主对象上传失败），不算冲突
		return nil
	}
	return latest
}

func (idx cloudIndex) find(key string) *CloudVersion {
	for i := range idx.Versions {
		if idx.Versions[i].Key == key {
			return &idx.Versions[i]
		}
	}
	return nil
}

func cloudIndexKey(objectKey string) string {
	return objectKey + cloudIndexSuffix
}

func cloudVersionKey(objectKey string, exportedAt int64) string {
	return fmt.Sprintf("%s%s%d.bin", objectKey, cloudHistoryPrefix, exportedAt)
}

// isCloudVersionKey 只允许操作本备份自己的历史对象，索引被改坏也删不到别的对象
func isCloudVersionKey(objectKey, key string) bool {
	return strings.HasPrefix(key, objectKey+cloudHistoryPrefix) && !strings.Contains(key, "..")
}

// readCloudIndex 读取历史索引。索引不存在（首次上传或旧版本上传的备份）返回空索引；
// 解不开（比如后来改了口令）也按空索引处理，历史从头记起，不影响主备份。
func readCloudIndex(client *ossObjectClient, objectKey, passphrase string) (cloudIndex, error) {
	raw, err := client.Get(cloudIndexKey(objectKey))
	if err != nil {
		if errors.Is(err, errOSSNotFound) {
			return cloudIndex{}, nil
		}
		return cloudIndex{}, err
	}
	payload, err := decryptCloudPayload(raw, passphrase)
	if err != nil {
		return cloudIndex{}, nil
	}
	var idx cloudIndex
	if json.Unmarshal(payload, &idx) != nil {
		return cloudIndex{}, nil
	}
	valid := idx.Versions[:0]
	for _, v := range idx.Versions {
		if isCloudVersionKey(objectKey, v.Key) {
			valid = append(valid, v)
		}
	}
	// 保持上传顺序（新的在前），不按时间排序：两台电脑时钟不一致时按时间排会认错"最新"
	idx.Versions = valid
	return idx, nil
}

func writeCloudIndex(client *ossObjectClient, objectKey, passphrase string, idx cloudIndex) error {
	payload, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	contentType := "application/json"
	if strings.TrimSpace(passphrase) != "" {
		if payload, err = encryptCloudPayload(payload, passphrase); err != nil {
			return err
		}
		contentType = "application/octet-stream"
	}
	return client.Put(cloudIndexKey(objectKey), payload, contentType)
}

func newCloudDeviceID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// canonicalJSON 去掉格式与键顺序差异（json.Marshal 会按键排序）
func canonicalJSON(raw json.RawMessage) []byte {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return raw
	}
	out, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return out
}

// bundleContentHash 备份内容的指纹，用来判断本机自上次同步以来是否真的改过
func bundleContentHash(files map[string]json.RawMessage) string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(canonicalJSON(files[name]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// promptProviders 有全局提示词文件的平台（Claude Desktop 没有）
var promptProviders = []string{"claude", "codex", "antigravity", "opencode", "grok"}

// collectPromptFiles 读取各平台的提示词文件，没有的平台不放进备份
func collectPromptFiles() map[string]string {
	out := map[string]string{}
	for _, provider := range promptProviders {
		path, err := promptFilePath(provider)
		if err != nil {
			continue
		}
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			out[provider] = string(data)
		}
	}
	return out
}

// restorePromptFiles 把备份里的提示词写回各平台；本机有、备份里没有的不删
func restorePromptFiles(raw json.RawMessage) error {
	var prompts map[string]string
	if err := json.Unmarshal(raw, &prompts); err != nil {
		return err
	}
	for _, provider := range promptProviders {
		content, ok := prompts[provider]
		if !ok {
			continue
		}
		path, err := promptFilePath(provider)
		if err != nil {
			return err
		}
		if data, err := os.ReadFile(path); err == nil && string(data) == content {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if _, err := backupFile(path); err != nil {
			return err
		}
		if err := writeFileAtomic(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func cloudConflictMessage(v *CloudVersion) string {
	host := strings.TrimSpace(v.Hostname)
	if host == "" {
		host = tr("另一台电脑")
	}
	return sprintf("云端有 %s 在 %s 上传的新备份，本机还没拉取过", host, time.UnixMilli(v.ExportedAt).Format("2006-01-02 15:04"))
}
