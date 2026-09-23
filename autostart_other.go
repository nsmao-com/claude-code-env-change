//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func enableAutostart() error {
	exe, err := currentExePath()
	if err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		return enableAutostartDarwin(exe)
	}
	return enableAutostartLinux(exe)
}

func disableAutostart() error {
	if runtime.GOOS == "darwin" {
		return removeAutostartFile(autostartLaunchAgentPath)
	}
	return removeAutostartFile(autostartDesktopPath)
}

func autostartEnabled() (bool, error) {
	if runtime.GOOS == "darwin" {
		return autostartFileExists(autostartLaunchAgentPath)
	}
	return autostartFileExists(autostartDesktopPath)
}

func removeAutostartFile(pathFn func() (string, error)) error {
	path, err := pathFn()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func autostartFileExists(pathFn func() (string, error)) (bool, error) {
	path, err := pathFn()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, nil
}

// darwin: LaunchAgent plist（RunAtLoad，登录即启动）
func enableAutostartDarwin(exe string) error {
	path, err := autostartLaunchAgentPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + autostartName + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + exe + `</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`
	return writeFileAtomic(path, []byte(plist), 0o644)
}

// linux: XDG autostart .desktop
func enableAutostartLinux(exe string) error {
	path, err := autostartDesktopPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	entry := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=AI ENV\n" +
		"Comment=AI CLI 环境与配置管理工具\n" +
		"Exec=" + desktopExecValue(exe) + "\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n"
	return writeFileAtomic(path, []byte(entry), 0o644)
}

// desktopExecValue Exec 字段里的引号要转义，含空格的路径必须整体加引号
func desktopExecValue(exe string) string {
	exe = strings.ReplaceAll(exe, `"`, `\"`)
	if strings.ContainsAny(exe, " \t") {
		return `"` + exe + `"`
	}
	return exe
}
