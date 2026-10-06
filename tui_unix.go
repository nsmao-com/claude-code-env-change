//go:build linux || darwin

package main

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// tuiTerminal 把终端切到原始模式（逐键读取、不回显），退出时还原
func tuiTerminal() (io.Reader, io.Writer, func(), error) {
	fd := int(os.Stdin.Fd())
	old, err := unix.IoctlGetTermios(fd, ioctlGetTermios)
	if err != nil {
		return nil, nil, nil, err
	}
	raw := *old
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, ioctlSetTermios, &raw); err != nil {
		return nil, nil, nil, err
	}
	return os.Stdin, os.Stdout, func() { _ = unix.IoctlSetTermios(fd, ioctlSetTermios, old) }, nil
}
