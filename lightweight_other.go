//go:build !windows

package main

// 其它平台没有托盘，轻量模式不起作用
func trimProcessTreeMemory() {}
