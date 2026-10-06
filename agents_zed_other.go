//go:build !windows

package main

// 其它系统的 Zed 把 API Key 存在系统钥匙串，首次使用时在 Zed 里为 aienv 填任意值即可
func saveZedCredential(string) error { return nil }

func deleteZedCredential(string) error { return nil }
