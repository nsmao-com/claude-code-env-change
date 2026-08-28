//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func init() {
	augmentWindowsPath()
}

func configureHiddenCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

func extraBinDirs() []string {
	home, _ := os.UserHomeDir()
	local := os.Getenv("LOCALAPPDATA")
	roaming := os.Getenv("APPDATA")
	return []string{
		filepath.Join(local, "pnpm"),
		filepath.Join(roaming, "npm"),
		filepath.Join(home, ".grok", "bin"),
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".kimi-code", "bin"),
		filepath.Join(local, "agy", "bin"),
		filepath.Join(local, "fnm"),
		filepath.Join(local, "Volta", "bin"),
		filepath.Join(home, "scoop", "shims"),
		filepath.Join(roaming, "nvm"),
		`D:\nodejs`,
		filepath.Join(filepath.VolumeName(home), "nodejs"),
	}
}

func wrapIfScript(path string, args []string) (string, []string) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".cmd", ".bat":
		comspec := os.Getenv("ComSpec")
		if comspec == "" {
			comspec = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
		}
		argv := append([]string{"/d", "/s", "/c", path}, args...)
		return comspec, argv
	case ".ps1":
		ps := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		argv := append([]string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", path}, args...)
		return ps, argv
	default:
		return path, args
	}
}

func listCommandCopies(name string) []string {
	found := scanKnownBins(name)
	where := filepath.Join(os.Getenv("SystemRoot"), "System32", "where.exe")
	if where == filepath.Join("System32", "where.exe") {
		where = `C:\Windows\System32\where.exe`
	}
	if out, err := runToolRaw(2*time.Second, where, name); err == nil {
		found = append(found, splitNonEmptyLines(out)...)
	}
	if out, err := runToolRaw(2*time.Second, where, name+".exe"); err == nil {
		found = append(found, splitNonEmptyLines(out)...)
	}
	if out, err := runToolRaw(2*time.Second, where, name+".cmd"); err == nil {
		found = append(found, splitNonEmptyLines(out)...)
	}
	return uniquePaths(found)
}

func augmentWindowsPath() {
	var extras []string
	for _, dir := range extraBinDirs() {
		if dirExists(dir) {
			extras = append(extras, dir)
		}
	}
	system32 := filepath.Join(os.Getenv("SystemRoot"), "System32")
	if system32 != filepath.Join("System32") {
		extras = append(extras, system32)
	}
	seen := map[string]struct{}{}
	var parts []string
	for _, dir := range append(extras, strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))...) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(dir))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		parts = append(parts, dir)
	}
	_ = os.Setenv("PATH", strings.Join(parts, string(os.PathListSeparator)))
}

func openInFileManager(path string) error {
	cmd := exec.Command("explorer.exe", path)
	return cmd.Start()
}

func revealInFileManager(path string) error {
	cmd := exec.Command("explorer.exe", "/select,"+path)
	return cmd.Start()
}
