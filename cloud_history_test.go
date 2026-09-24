package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeObjectStore 内存里的 S3 兼容对象存储（path-style：/bucket/key），不校验签名
type fakeObjectStore struct {
	*httptest.Server
	mu      sync.Mutex
	objects map[string][]byte
}

func newFakeObjectStore(t *testing.T) *fakeObjectStore {
	t.Helper()
	fs := &fakeObjectStore{objects: map[string][]byte{}}
	fs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/bucket/")
		fs.mu.Lock()
		defer fs.mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			data, _ := io.ReadAll(r.Body)
			fs.objects[key] = data
		case http.MethodGet, http.MethodHead:
			data, ok := fs.objects[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.Method == http.MethodGet {
				w.Write(data)
			}
		case http.MethodDelete:
			delete(fs.objects, key)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(fs.Close)
	return fs
}

func (fs *fakeObjectStore) count(prefix string) int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	n := 0
	for key := range fs.objects {
		if strings.HasPrefix(key, prefix) {
			n++
		}
	}
	return n
}

// cloudDevice 一台"电脑"：独立的本机配置目录与设备标识，共用同一个存储桶。
// 各服务按 HOME 找本机文件，操作前用 on() 切到这台电脑的目录
type cloudDevice struct {
	*CloudSyncService
	home string
	t    *testing.T
}

func (d cloudDevice) on() cloudDevice {
	d.t.Setenv("HOME", d.home)
	d.t.Setenv("USERPROFILE", d.home)
	return d
}

func newCloudDevice(t *testing.T, store *fakeObjectStore, passphrase string) (cloudDevice, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CODEX_HOME", "")
	cs := &CloudSyncService{httpClient: store.Client()}
	cs.config = CloudConfig{
		Enabled: true, Provider: "minio", Endpoint: store.URL, Bucket: "bucket",
		ObjectKey: "env/backup.bin", AccessKey: "ak", SecretKey: "sk", PathStyle: true,
		Passphrase: passphrase,
	}
	return cloudDevice{CloudSyncService: cs, home: home, t: t}, home
}

func writeLocalConfig(t *testing.T, home, content string) {
	t.Helper()
	dir := filepath.Join(home, mcpStoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, mainConfigFile), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readLocalConfig(t *testing.T, home string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, mcpStoreDir, mainConfigFile))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// 每次上传都留一份历史，只保留最近 10 份，多出来的对象会被删掉
func TestCloudUploadKeepsHistory(t *testing.T) {
	store := newFakeObjectStore(t)
	cs, home := newCloudDevice(t, store, "pass")
	for i := 0; i < cloudHistoryKeep+2; i++ {
		writeLocalConfig(t, home, `{"environments":[],"n":"`+string(rune('a'+i))+`"}`)
		if r := cs.on().UploadToCloud(); !r.Success {
			t.Fatalf("第 %d 次上传失败: %s", i+1, r.Message)
		}
	}
	versions, err := cs.on().ListCloudVersions()
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != cloudHistoryKeep {
		t.Fatalf("应保留 %d 份历史，实际 %d", cloudHistoryKeep, len(versions))
	}
	if got := store.count("env/backup.bin" + cloudHistoryPrefix); got != cloudHistoryKeep {
		t.Fatalf("超出的历史对象应被删除，存储里还有 %d 份", got)
	}
	for i := 1; i < len(versions); i++ {
		if versions[i-1].ExportedAt <= versions[i].ExportedAt {
			t.Fatalf("历史应按上传顺序新的在前，且导出时间严格递增: %+v", versions)
		}
	}
	// 设了口令时索引同样加密，存储里看不到主机名
	store.mu.Lock()
	idx := string(store.objects["env/backup.bin"+cloudIndexSuffix])
	store.mu.Unlock()
	if !strings.HasPrefix(idx, cloudMagic) {
		t.Fatalf("设了口令时历史索引也应加密")
	}
}

// 两台电脑：B 没拉取过 A 的备份就上传，应判为冲突且不覆盖；确认后可强制上传，A 再上传时同样提示
func TestCloudUploadDetectsConflict(t *testing.T) {
	store := newFakeObjectStore(t)
	a, homeA := newCloudDevice(t, store, "")
	writeLocalConfig(t, homeA, `{"environments":[{"name":"from-a"}]}`)
	if r := a.on().UploadToCloud(); !r.Success {
		t.Fatalf("A 上传失败: %s", r.Message)
	}

	b, homeB := newCloudDevice(t, store, "")
	writeLocalConfig(t, homeB, `{"environments":[{"name":"from-b"}]}`)
	r := b.on().UploadToCloud()
	if r.Success || !r.Conflict {
		t.Fatalf("B 未拉取过就上传应判为冲突，实际 %+v", r)
	}
	store.mu.Lock()
	main := string(store.objects["env/backup.bin"])
	store.mu.Unlock()
	if !strings.Contains(main, "from-a") {
		t.Fatalf("冲突时不能覆盖云端主备份")
	}

	if r := b.on().ForceUploadToCloud(); !r.Success {
		t.Fatalf("确认后应能强制上传: %s", r.Message)
	}
	if versions, _ := b.on().ListCloudVersions(); len(versions) != 2 {
		t.Fatalf("被覆盖的 A 的备份应仍在历史里，实际 %d 份", len(versions))
	}

	writeLocalConfig(t, homeA, `{"environments":[{"name":"from-a-2"}]}`)
	if r := a.on().UploadToCloud(); !r.Conflict {
		t.Fatalf("B 覆盖后 A 再上传应提示冲突，实际 %+v", r)
	}
	// 本机自己连续上传不算冲突
	writeLocalConfig(t, homeB, `{"environments":[{"name":"from-b-2"}]}`)
	if r := b.on().UploadToCloud(); !r.Success {
		t.Fatalf("B 连续上传不应冲突: %s", r.Message)
	}
}

// 启动时自动拉取：新电脑直接拉；云端没新备份就跳过；本机有未上传的修改时不覆盖
func TestCloudPullOnStartup(t *testing.T) {
	store := newFakeObjectStore(t)
	a, homeA := newCloudDevice(t, store, "pw")
	writeLocalConfig(t, homeA, `{"environments":[{"name":"v1"}]}`)
	if r := a.on().UploadToCloud(); !r.Success {
		t.Fatal(r.Message)
	}

	b, homeB := newCloudDevice(t, store, "pw")
	writeLocalConfig(t, homeB, `{"environments":[]}`)
	if r := b.on().pullOnStartup(); !r.Success || r.Skipped {
		t.Fatalf("新电脑启动时应拉取云端备份，实际 %+v", r)
	}
	if !strings.Contains(readLocalConfig(t, homeB), "v1") {
		t.Fatalf("拉取后本机应是云端内容")
	}
	if r := b.on().pullOnStartup(); !r.Skipped {
		t.Fatalf("云端没有新备份时应跳过，实际 %+v", r)
	}

	// 只是换了格式、内容没变，不算本机修改
	writeLocalConfig(t, homeB, "{\n  \"environments\": [ { \"name\": \"v1\" } ]\n}")
	if b.on().localDirty(b.config) {
		t.Fatalf("仅格式变化不应判为本机有修改")
	}

	writeLocalConfig(t, homeA, `{"environments":[{"name":"v2"}]}`)
	if r := a.on().UploadToCloud(); !r.Success {
		t.Fatal(r.Message)
	}
	writeLocalConfig(t, homeB, `{"environments":[{"name":"b-local-edit"}]}`)
	r := b.on().pullOnStartup()
	if !r.Conflict {
		t.Fatalf("本机有未上传的修改且云端有新备份时应判为冲突，实际 %+v", r)
	}
	if !strings.Contains(readLocalConfig(t, homeB), "b-local-edit") {
		t.Fatalf("冲突时不能覆盖本机修改")
	}
}

// 恢复历史版本：只接受索引里的版本；恢复后本机再上传不应被当成冲突
func TestCloudRestoreVersion(t *testing.T) {
	store := newFakeObjectStore(t)
	cs, home := newCloudDevice(t, store, "")
	writeLocalConfig(t, home, `{"environments":[{"name":"old"}]}`)
	if r := cs.on().UploadToCloud(); !r.Success {
		t.Fatal(r.Message)
	}
	writeLocalConfig(t, home, `{"environments":[{"name":"new"}]}`)
	if r := cs.on().UploadToCloud(); !r.Success {
		t.Fatal(r.Message)
	}
	versions, _ := cs.on().ListCloudVersions()
	if len(versions) != 2 {
		t.Fatalf("应有 2 份历史，实际 %d", len(versions))
	}

	if r := cs.on().RestoreCloudVersion("env/other.bin"); r.Success {
		t.Fatalf("不在索引里的对象不能恢复")
	}
	if r := cs.on().RestoreCloudVersion(versions[1].Key); !r.Success {
		t.Fatalf("恢复旧版本失败: %s", r.Message)
	}
	if !strings.Contains(readLocalConfig(t, home), "old") {
		t.Fatalf("恢复后本机应是旧版本内容")
	}
	writeLocalConfig(t, home, `{"environments":[{"name":"old-edited"}]}`)
	if r := cs.on().UploadToCloud(); !r.Success {
		t.Fatalf("恢复旧版本后继续上传不应冲突: %+v", r)
	}
}

// 提示词文件进备份；拉取时写回平台并留 .bak
func TestCloudBackupIncludesPrompts(t *testing.T) {
	store := newFakeObjectStore(t)
	a, homeA := newCloudDevice(t, store, "")
	writeLocalConfig(t, homeA, `{"environments":[]}`)
	claudeMD := filepath.Join(homeA, ".claude", "CLAUDE.md")
	os.MkdirAll(filepath.Dir(claudeMD), 0o755)
	os.WriteFile(claudeMD, []byte("# rules from A\n"), 0o644)
	if r := a.on().UploadToCloud(); !r.Success {
		t.Fatal(r.Message)
	}

	b, homeB := newCloudDevice(t, store, "")
	writeLocalConfig(t, homeB, `{"environments":[]}`)
	localMD := filepath.Join(homeB, ".claude", "CLAUDE.md")
	os.MkdirAll(filepath.Dir(localMD), 0o755)
	os.WriteFile(localMD, []byte("# local rules\n"), 0o644)
	agents := filepath.Join(homeB, ".codex", "AGENTS.md")
	os.MkdirAll(filepath.Dir(agents), 0o755)
	os.WriteFile(agents, []byte("# codex only on B\n"), 0o644)

	if r := b.on().DownloadFromCloud(); !r.Success {
		t.Fatalf("拉取失败: %s", r.Message)
	}
	if data, _ := os.ReadFile(localMD); string(data) != "# rules from A\n" {
		t.Fatalf("CLAUDE.md 应恢复为云端内容，实际 %q", data)
	}
	if data, _ := os.ReadFile(localMD + ".bak"); string(data) != "# local rules\n" {
		t.Fatalf("被覆盖的提示词应留 .bak，实际 %q", data)
	}
	if data, _ := os.ReadFile(agents); string(data) != "# codex only on B\n" {
		t.Fatalf("备份里没有的提示词不应被删改")
	}
}

// 自动上传：内容没变（包括只是格式变了）就不传，不会用重复内容刷掉历史
func TestCloudAutoUploadSkipsUnchanged(t *testing.T) {
	store := newFakeObjectStore(t)
	cs, home := newCloudDevice(t, store, "")
	writeLocalConfig(t, home, `{"environments":[{"name":"x"}]}`)
	if r := cs.on().upload(false, true); !r.Success || r.Skipped {
		t.Fatalf("首次自动上传应执行，实际 %+v", r)
	}
	writeLocalConfig(t, home, "{\n  \"environments\": [{\"name\": \"x\"}]\n}")
	if r := cs.on().upload(false, true); !r.Skipped {
		t.Fatalf("内容没变时自动上传应跳过，实际 %+v", r)
	}
	writeLocalConfig(t, home, `{"environments":[{"name":"y"}]}`)
	if r := cs.on().upload(false, true); !r.Success || r.Skipped {
		t.Fatalf("内容变了应上传，实际 %+v", r)
	}
	if versions, _ := cs.on().ListCloudVersions(); len(versions) != 2 {
		t.Fatalf("应只有 2 份历史，实际 %d", len(versions))
	}
}
