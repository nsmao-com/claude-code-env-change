package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// 配置文件是符号链接时，原子写入要写到真实文件，链接本身保持不变
func TestWriteFileAtomicFollowsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 创建符号链接需要额外权限")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "dotfiles", "settings.json")
	if err := os.MkdirAll(filepath.Dir(real), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, []byte(`{"old":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "settings.json")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	if err := writeFileAtomic(link, []byte(`{"new":true}`), 0o644); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("符号链接被替换成了普通文件")
	}
	if data, _ := os.ReadFile(real); string(data) != `{"new":true}` {
		t.Fatalf("真实文件未更新: %s", data)
	}
}
