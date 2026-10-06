//go:build !windows

package main

// registerURLProtocol macOS / Linux 的协议注册由应用包（Info.plist / .desktop）声明，这里不做处理
func registerURLProtocol() error { return nil }
