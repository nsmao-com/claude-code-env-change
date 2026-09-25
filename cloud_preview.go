package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

type cloudRestorePending struct {
	Token, LocalHash, ETag string
	Bundle                 cloudBundle
	At                     time.Time
	Connection             string
}

func bundleFingerprint(b cloudBundle) string {
	data, _ := json.Marshal(b.Files)
	return contentHash(data)
}
func (c *ossObjectClient) Version(key string) (string, bool, error) {
	req, e := c.newRequest(http.MethodHead, key, nil, "")
	if e != nil {
		return "", false, e
	}
	resp, e := c.client.Do(req)
	if e != nil {
		return "", false, e
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return "", false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, ossHTTPError("读取版本", resp)
	}
	return resp.Header.Get("ETag"), true, nil
}
func (cs *CloudSyncService) PreviewCloudRestore() (HistoryPreview, error) {
	cs.mu.Lock()
	cfg := cs.config
	busy := cs.applying || cs.pushing
	cs.mu.Unlock()
	if busy {
		return HistoryPreview{}, fmt.Errorf("同步进行中，请稍后重试")
	}
	if !csConfigured(cfg) {
		return HistoryPreview{}, fmt.Errorf("请先保存云端连接配置")
	}
	client := newOSSObjectClient(cfg, cs.httpClient)
	before, exists, e := client.Version(cfg.ObjectKey)
	if e != nil {
		return HistoryPreview{}, e
	}
	if !exists {
		return HistoryPreview{}, fmt.Errorf("云端还没有备份")
	}
	raw, e := client.Get(cfg.ObjectKey)
	if e != nil {
		return HistoryPreview{}, e
	}
	after, _, e := client.Version(cfg.ObjectKey)
	if e != nil {
		return HistoryPreview{}, e
	}
	if before != after || after == "" {
		return HistoryPreview{}, fmt.Errorf("云端版本变化或服务未提供 ETag，请重试")
	}
	plain, e := decryptCloudPayload(raw, cfg.Passphrase)
	if e != nil {
		return HistoryPreview{}, e
	}
	var bundle cloudBundle
	if json.Unmarshal(plain, &bundle) != nil || bundle.Version != cloudBackupVersion {
		return HistoryPreview{}, fmt.Errorf("备份格式或版本不受支持")
	}
	local, e := cs.buildBundle()
	if e != nil {
		return HistoryPreview{}, e
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
	if e := validateCloudBundle(bundle); e != nil {
		return preview, e
	}
	sort.Slice(preview.Changes, func(i, j int) bool { return preview.Changes[i].Path < preview.Changes[j].Path })
	cs.mu.Lock()
	cs.pending = &cloudRestorePending{preview.Token, bundleFingerprint(*local), after, bundle, time.Now(), cloudConnectionFingerprint(cfg)}
	cs.mu.Unlock()
	return preview, nil
}
func (cs *CloudSyncService) ConfirmCloudRestore(token string, names []string) CloudSyncResult {
	cs.mu.Lock()
	p := cs.pending
	cfg := cs.config
	if cs.applying || cs.pushing || p == nil || p.Token != token || time.Since(p.At) > 10*time.Minute || p.Connection != cloudConnectionFingerprint(cfg) {
		cs.mu.Unlock()
		return CloudSyncResult{Message: "预览已失效，请重新预览"}
	}
	cs.applying = true
	cs.mu.Unlock()
	defer func() { cs.mu.Lock(); cs.applying = false; cs.mu.Unlock() }()
	if len(names) == 0 {
		return CloudSyncResult{Message: "请选择要恢复的配置"}
	}
	local, e := cs.buildBundle()
	if e != nil {
		return CloudSyncResult{Message: e.Error()}
	}
	if bundleFingerprint(*local) != p.LocalHash {
		return CloudSyncResult{Message: "本地在预览后发生变化，请重新预览"}
	}
	etag, _, e := newOSSObjectClient(cfg, cs.httpClient).Version(cfg.ObjectKey)
	if e != nil {
		return CloudSyncResult{Message: e.Error()}
	}
	if etag != p.ETag {
		return CloudSyncResult{Message: "云端在预览后发生变化，请重新预览"}
	}
	selected := cloudBundle{Version: cloudBackupVersion, Files: map[string]json.RawMessage{}}
	for _, n := range names {
		data, ok := p.Bundle.Files[n]
		if !ok {
			return CloudSyncResult{Message: "选择的文件不在预览中"}
		}
		selected.Files[n] = data
	}
	msg, e := cs.restoreBundle(selected)
	if e != nil {
		return CloudSyncResult{Message: e.Error()}
	}
	updated, e := cs.buildBundle()
	if e != nil {
		return CloudSyncResult{Message: e.Error()}
	}
	cs.mu.Lock()
	cs.config.RemoteETag = p.ETag
	cs.config.LocalBaseline = bundleFingerprint(*updated)
	cs.config.LastPullAt = time.Now().UnixMilli()
	cs.config.LastError = ""
	cs.pending = nil
	e = cs.persistLocked()
	cs.mu.Unlock()
	if e != nil {
		return CloudSyncResult{Message: e.Error()}
	}
	return CloudSyncResult{Success: true, Message: msg}
}
func cloudConnectionFingerprint(c CloudConfig) string {
	data, _ := json.Marshal([]string{c.Provider, c.Endpoint, c.Region, c.Bucket, c.ObjectKey, c.AccessKey, c.SecretKey, c.Passphrase, fmt.Sprint(c.PathStyle)})
	return contentHash(data)
}
