//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

// antigravityManagedEnvVars 本工具托管的 agy 环境变量。
// agy 只从进程环境读取凭据和端点（官方明确不加载 .env，settings.json 也不存 key），
// 因此应用配置时必须把它们写入用户级环境。
var antigravityManagedEnvVars = []string{"GEMINI_API_KEY", "GOOGLE_GEMINI_BASE_URL"}

// syncAntigravityUserEnv 声明式同步：让用户环境里的托管变量精确等于 state
// （state 里的键写入，不在 state 里的托管键删除）。写入后需新开终端才会生效。
func syncAntigravityUserEnv(state map[string]string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("打开用户环境变量失败: %v", err)
	}
	defer key.Close()

	changed := false
	for _, name := range antigravityManagedEnvVars {
		value, present := state[name]
		if present {
			if err := key.SetStringValue(name, value); err != nil {
				return fmt.Errorf("写入 %s 失败: %v", name, err)
			}
			changed = true
			continue
		}
		if err := key.DeleteValue(name); err != nil {
			if err == registry.ErrNotExist {
				continue
			}
			return fmt.Errorf("删除 %s 失败: %v", name, err)
		}
		changed = true
	}
	if changed {
		broadcastEnvChange()
	}
	return nil
}

// readAntigravityUserEnv 读取用户级环境变量里托管的 agy 变量（即 agy 实际能读到的值）
func readAntigravityUserEnv() map[string]string {
	out := map[string]string{}
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err != nil {
		return out
	}
	defer key.Close()
	for _, name := range antigravityManagedEnvVars {
		if value, _, err := key.GetStringValue(name); err == nil && value != "" {
			out[name] = value
		}
	}
	return out
}

// broadcastEnvChange 广播 WM_SETTINGCHANGE，让之后新开的终端继承最新用户环境变量
func broadcastEnvChange() {
	const (
		hwndBroadcast   = uintptr(0xFFFF)
		wmSettingChange = 0x001A
		smtoAbortIfHung = 0x0002
	)
	env, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	result := uintptr(0)
	syscall.NewLazyDLL("user32.dll").NewProc("SendMessageTimeoutW").Call(
		hwndBroadcast,
		wmSettingChange,
		0,
		uintptr(unsafe.Pointer(env)),
		smtoAbortIfHung,
		5000,
		uintptr(unsafe.Pointer(&result)),
	)
}
