//go:build windows

package main

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Zed 在 Windows 上从凭据管理器读取 OpenAI 兼容服务商的 API Key（目标名 zed:url=<api_url>）。
// 本机网关不校验这个值，写入占位值即可免去 Zed 的“未设置 API Key”提示。
type zedWindowsCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

func saveZedCredential(apiURL string) error {
	target, err := windows.UTF16PtrFromString("zed:url=" + apiURL)
	if err != nil {
		return err
	}
	user, _ := windows.UTF16PtrFromString("Bearer")
	key := []byte(agentProviderID)
	cred := zedWindowsCredential{
		Type: 1, TargetName: target, UserName: user, // CRED_TYPE_GENERIC
		CredentialBlobSize: uint32(len(key)), CredentialBlob: &key[0],
		Persist: 2, // CRED_PERSIST_LOCAL_MACHINE
	}
	ok, _, callErr := windows.NewLazySystemDLL("advapi32.dll").NewProc("CredWriteW").Call(uintptr(unsafe.Pointer(&cred)), 0)
	runtime.KeepAlive(cred)
	runtime.KeepAlive(key)
	if ok == 0 {
		return fmt.Errorf("CredWriteW: %w", callErr)
	}
	return nil
}

func deleteZedCredential(apiURL string) error {
	target, err := windows.UTF16PtrFromString("zed:url=" + apiURL)
	if err != nil {
		return err
	}
	ok, _, callErr := windows.NewLazySystemDLL("advapi32.dll").NewProc("CredDeleteW").Call(uintptr(unsafe.Pointer(target)), 1, 0)
	if ok == 0 {
		return fmt.Errorf("CredDeleteW: %w", callErr)
	}
	return nil
}
