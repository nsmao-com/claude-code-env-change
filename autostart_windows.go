//go:build windows

package main

import (
	"golang.org/x/sys/windows/registry"
)

// HKCU Run 键：登录后以当前用户身份启动，无需管理员权限
const autostartRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func enableAutostart() error {
	exe, err := currentExePath()
	if err != nil {
		return err
	}
	return enableAutostartWindows(exe)
}

func disableAutostart() error {
	return disableAutostartWindows()
}

func autostartEnabled() (bool, error) {
	return autostartEnabledWindows()
}

func enableAutostartWindows(exe string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartRunKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue(autostartName, `"`+exe+`"`)
}

func disableAutostartWindows() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartRunKey, registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	defer key.Close()
	// 值本身不存在视为已关闭；其它错误（如权限不足）必须上报，否则界面显示已关闭、实际仍会自启
	if err := key.DeleteValue(autostartName); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}

func autostartEnabledWindows() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartRunKey, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	defer key.Close()
	_, _, err = key.GetStringValue(autostartName)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
