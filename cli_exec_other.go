//go:build !windows

package main

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"time"
)

func extraBinDirs() []string { return nil }

func wrapIfScript(path string, args []string) (string, []string) {
	return path, args
}

func startToolCommand(ctx context.Context, name string, args []string, extraEnv []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	return cmd
}

func configureHiddenCmd(cmd *exec.Cmd) {}

func listCommandCopies(name string) []string {
	out, err := runTool(2*time.Second, "sh", "-c", "command -v "+name+" || true")
	if err != nil {
		return nil
	}
	return splitNonEmptyLines(out)
}

func openInFileManager(path string) error {
	bin := "xdg-open"
	if runtime.GOOS == "darwin" {
		bin = "open"
	}
	return exec.Command(bin, path).Start()
}

func revealInFileManager(path string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", "-R", path).Start()
	}
	return openInFileManager(path)
}
