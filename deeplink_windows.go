//go:build windows

package main

import (
	"golang.org/x/sys/windows/registry"
)

// registerURLProtocol 在 HKCU 注册 aienv:// 协议，指向当前程序；路径未变时不重复写入
func registerURLProtocol() error {
	exe, err := currentExePath()
	if err != nil {
		return err
	}
	command := `"` + exe + `" "%1"`
	base := `Software\Classes\` + deepLinkScheme
	if key, err := registry.OpenKey(registry.CURRENT_USER, base+`\shell\open\command`, registry.QUERY_VALUE); err == nil {
		current, _, _ := key.GetStringValue("")
		key.Close()
		if current == command {
			return nil
		}
	}
	root, _, err := registry.CreateKey(registry.CURRENT_USER, base, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.SetStringValue("", "URL:AI ENV"); err != nil {
		return err
	}
	if err := root.SetStringValue("URL Protocol", ""); err != nil {
		return err
	}
	if icon, _, err := registry.CreateKey(registry.CURRENT_USER, base+`\DefaultIcon`, registry.SET_VALUE); err == nil {
		_ = icon.SetStringValue("", `"`+exe+`",0`)
		icon.Close()
	}
	cmd, _, err := registry.CreateKey(registry.CURRENT_USER, base+`\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer cmd.Close()
	return cmd.SetStringValue("", command)
}
