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

// 覆盖已有文件时不能放宽用户收紧过的权限，也不能超出调用方要求的权限
func TestWriteFileAtomicNeverLoosensPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不区分组/其他用户权限位")
	}
	dir := t.TempDir()
	cases := []struct {
		name     string
		existing os.FileMode
		perm     os.FileMode
		want     os.FileMode
	}{
		{"用户收紧为 600 时保持", 0o600, 0o644, 0o600},
		{"调用方要求 600 时收紧", 0o644, 0o600, 0o600},
		{"组可读保持", 0o640, 0o644, 0o640},
		{"不继承可执行位", 0o755, 0o644, 0o644},
	}
	for i, tc := range cases {
		path := filepath.Join(dir, tc.name+".json")
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, tc.existing); err != nil {
			t.Fatal(err)
		}
		if err := writeFileAtomic(path, []byte(`{"n":1}`), tc.perm); err != nil {
			t.Fatalf("case %d 写入失败: %v", i, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != tc.want {
			t.Errorf("%s: 权限 = %o，期望 %o", tc.name, got, tc.want)
		}
	}

	fresh := filepath.Join(dir, "new.json")
	if err := writeFileAtomic(fresh, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(fresh); info.Mode().Perm() != 0o644 {
		t.Errorf("新文件权限 = %o，期望 644", info.Mode().Perm())
	}
}
