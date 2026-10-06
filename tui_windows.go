//go:build windows

package main

import (
	"io"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// tuiTerminal 桌面版是 GUI 程序，终端不会等它退出，与终端抢输入会乱：没有控制台时开一个独立窗口
func tuiTerminal() (io.Reader, io.Writer, func(), error) {
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	if w, _, _ := kernel.NewProc("GetConsoleWindow").Call(); w == 0 {
		if r, _, err := kernel.NewProc("AllocConsole").Call(); r == 0 {
			return nil, nil, nil, err
		}
		if title, err := windows.UTF16PtrFromString("AI ENV"); err == nil {
			_, _, _ = kernel.NewProc("SetConsoleTitleW").Call(uintptr(unsafe.Pointer(title)))
		}
	}
	in, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, nil, err
	}
	out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		in.Close()
		return nil, nil, nil, err
	}
	inH, outH := windows.Handle(in.Fd()), windows.Handle(out.Fd())
	var inMode, outMode uint32
	_ = windows.GetConsoleMode(inH, &inMode)
	_ = windows.GetConsoleMode(outH, &outMode)
	_ = windows.SetConsoleMode(inH, windows.ENABLE_VIRTUAL_TERMINAL_INPUT|windows.ENABLE_EXTENDED_FLAGS)
	_ = windows.SetConsoleMode(outH, outMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING|windows.ENABLE_PROCESSED_OUTPUT)
	_ = windows.SetConsoleOutputCP(65001)
	restore := func() {
		_ = windows.SetConsoleMode(inH, inMode)
		_ = windows.SetConsoleMode(outH, outMode)
		in.Close()
		out.Close()
	}
	return in, out, restore, nil
}
