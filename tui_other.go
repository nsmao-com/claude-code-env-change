//go:build !windows && !linux && !darwin

package main

import (
	"errors"
	"io"
)

func tuiTerminal() (io.Reader, io.Writer, func(), error) {
	return nil, nil, nil, errors.New("当前系统不支持终端界面，请使用 list / use 命令")
}
