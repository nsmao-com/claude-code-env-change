//go:build windows

package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// trimProcessTreeMemory 把本进程与它启动的全部子孙进程（WebView2 的浏览器、渲染、GPU 进程等）
// 的工作集交还给系统；页面之后用到时再按需换回
func trimProcessTreeMemory() {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return
	}
	defer windows.CloseHandle(snap)
	children := map[uint32][]uint32{}
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		children[e.ParentProcessID] = append(children[e.ParentProcessID], e.ProcessID)
	}
	setWS := windows.NewLazySystemDLL("kernel32.dll").NewProc("SetProcessWorkingSetSize")
	self := uint32(os.Getpid())
	seen := map[uint32]bool{}
	queue := []uint32{self}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		if seen[pid] {
			continue
		}
		seen[pid] = true
		queue = append(queue, children[pid]...)
		h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
		if err != nil {
			continue
		}
		// 两个 -1：清空工作集（等同 EmptyWorkingSet）
		_, _, _ = setWS.Call(uintptr(h), ^uintptr(0), ^uintptr(0))
		windows.CloseHandle(h)
	}
}
