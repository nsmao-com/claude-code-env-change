package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// writeFileAtomic 先写临时文件再原子替换目标文件，避免进程中断时把用户配置截断成半截
// JSON/TOML。Windows 上目标文件被占用时 rename 会失败，这里做有限次重试。
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := captureBeforeWrite(path, data); err != nil {
		return fmt.Errorf("创建配置历史失败，已停止写入: %w", err)
	}
	// 目标是符号链接时（dotfiles 仓库管理的配置很常见）写到链接指向的真实文件，
	// 否则 rename 会把链接本身替换成普通文件，用户的链接就断了
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	// 覆盖已有文件时不放宽组/其他用户权限：用户手动 chmod 600 的含密钥配置
	// 不能因为本工具写一次就变回所有人可读；属主读写位仍按调用方要求
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		perm = perm&info.Mode().Perm() | perm&0o700
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %v", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("写入临时文件失败: %v", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return err
	}

	var renameErr error
	for attempt := 0; attempt < 5; attempt++ {
		renameErr = os.Rename(tmpName, path)
		if renameErr == nil {
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	// 保留临时文件供人工恢复，绝不降级为非原子直写
	return fmt.Errorf("替换 %s 失败（原文件未动，临时文件保留在 %s）: %v", path, tmpName, renameErr)
}

// backupFile 把现有文件复制为 <name>.bak（覆盖上一份），用于覆盖删除前的最后防线。
// 文件不存在时视为无事可做。
func backupFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	backupPath := path + ".bak"
	if err := writeFileAtomic(backupPath, data, 0o600); err != nil {
		return "", err
	}
	return backupPath, nil
}
