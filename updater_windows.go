//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func applyUpdateAndRestart(a *App, newExe, currentExe string) error {
	if a == nil || a.ctx == nil {
		return fmt.Errorf("应用未就绪")
	}
	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("claude-env-update-%d.ps1", os.Getpid()))
	script := buildReplaceScript(os.Getpid(), newExe, currentExe)
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return fmt.Errorf("写入更新脚本失败: %v", err)
	}

	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-WindowStyle", "Hidden",
		"-File", scriptPath,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000008 | 0x00000200 | 0x08000000, // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW
	}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("启动更新进程失败: %v", err)
	}
	time.Sleep(250 * time.Millisecond)
	return nil
}

func buildReplaceScript(pid int, src, dst string) string {
	var b strings.Builder
	b.WriteString("\ufeff") // UTF-8 BOM，兼容 Windows PowerShell 5.1
	b.WriteString("$ErrorActionPreference = 'Continue'\n")
	b.WriteString(fmt.Sprintf("$targetPid = %d\n", pid))
	b.WriteString(fmt.Sprintf("$src = %s\n", psQuote(src)))
	b.WriteString(fmt.Sprintf("$dst = %s\n", psQuote(dst)))
	b.WriteString(fmt.Sprintf("$log = %s\n", psQuote(updateResultLogPath())))
	b.WriteString("$script = $MyInvocation.MyCommand.Path\n")
	b.WriteString("New-Item -ItemType Directory -Force -Path (Split-Path -Parent $log) | Out-Null\n")
	b.WriteString("for ($i = 0; $i -lt 80; $i++) {\n")
	b.WriteString("  $proc = Get-Process -Id $targetPid -ErrorAction SilentlyContinue\n")
	b.WriteString("  if (-not $proc) { break }\n")
	b.WriteString("  Start-Sleep -Milliseconds 250\n")
	b.WriteString("}\n")
	b.WriteString("Start-Sleep -Milliseconds 800\n")
	b.WriteString("$workDir = Split-Path -Parent $dst\n")
	b.WriteString("$copied = $false\n")
	b.WriteString("$lastErr = ''\n")
	b.WriteString("for ($i = 0; $i -lt 20; $i++) {\n")
	b.WriteString("  try {\n")
	b.WriteString("    Copy-Item -LiteralPath $src -Destination $dst -Force\n")
	b.WriteString("    $copied = $true\n")
	b.WriteString("    break\n")
	b.WriteString("  } catch {\n")
	b.WriteString("    $lastErr = $_.Exception.Message\n")
	b.WriteString("    Start-Sleep -Milliseconds 400\n")
	b.WriteString("  }\n")
	b.WriteString("}\n")
	b.WriteString("if (-not $copied) {\n")
	b.WriteString("  Set-Content -LiteralPath $log -Value ('ERROR: ' + $lastErr) -Encoding UTF8\n")
	b.WriteString("  # 覆盖失败：把旧程序拉回来，避免应用退出后“消失”\n")
	b.WriteString("  Start-Process -FilePath $dst -WorkingDirectory $workDir\n")
	b.WriteString("  exit 1\n")
	b.WriteString("}\n")
	b.WriteString("Set-Content -LiteralPath $log -Value 'OK' -Encoding UTF8\n")
	b.WriteString("Start-Process -FilePath $dst -WorkingDirectory $workDir\n")
	b.WriteString("Remove-Item -LiteralPath $src -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("$parent = Split-Path -Parent $src\n")
	b.WriteString("Remove-Item -LiteralPath $parent -Recurse -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("Remove-Item -LiteralPath $script -Force -ErrorAction SilentlyContinue\n")
	return b.String()
}

func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
