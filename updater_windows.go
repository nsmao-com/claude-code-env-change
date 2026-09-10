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

// startUpdateScript 以无窗口方式拉起善后脚本。
//
// 注意：这里绝对不能带 DETACHED_PROCESS(0x8)。powershell.exe 是控制台程序，
// 在没有控制台的情况下会立即退出，脚本一行都不会执行——表现就是应用退出后
// 安装器不弹、旧程序也不回来。只用 CREATE_NO_WINDOW 即可隐藏窗口，
// 子进程默认不会随父进程退出而结束。
func startUpdateScript(scriptPath string) error {
	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000200 | 0x08000000, // CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// 正常情况下脚本会一直等旧进程退出，不可能秒退；
	// 秒退说明 PowerShell 没能跑起来，此时必须把失败暴露出来。
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("更新脚本进程异常退出: %v", err)
		}
		return fmt.Errorf("更新脚本进程异常退出")
	case <-time.After(700 * time.Millisecond):
		return nil
	}
}

// markUpdatePending 在拉起脚本前写入占位结果。脚本执行完会覆盖成 OK 或真实错误；
// 若脚本压根没跑起来，下次启动就能把这条占位信息提示给用户，而不是静默失败。
func markUpdatePending() {
	logPath := updateResultLogPath()
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	_ = os.WriteFile(logPath, []byte("ERROR: 更新脚本没有执行，安装器未启动；请到 GitHub 下载安装包手动更新"), 0o644)
}

func applyUpdateAndRestart(a *App, newExe, currentExe string) error {
	if a == nil || a.ctx == nil {
		return fmt.Errorf("应用未就绪")
	}
	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("claude-env-update-%d.ps1", os.Getpid()))
	script := buildReplaceScript(os.Getpid(), newExe, currentExe)
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return fmt.Errorf("写入更新脚本失败: %v", err)
	}

	markUpdatePending()
	if err := startUpdateScript(scriptPath); err != nil {
		_ = os.Remove(scriptPath)
		_ = os.Remove(updateResultLogPath())
		return fmt.Errorf("启动更新进程失败: %v", err)
	}
	return nil
}

func startInstallerAfterExit(a *App, installerPath, currentExe string) error {
	if a == nil || a.ctx == nil {
		return fmt.Errorf("应用未就绪")
	}
	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("claude-env-installer-%d.ps1", os.Getpid()))
	script := buildInstallerScript(os.Getpid(), installerPath, currentExe)
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return fmt.Errorf("写入安装脚本失败: %v", err)
	}

	markUpdatePending()
	if err := startUpdateScript(scriptPath); err != nil {
		_ = os.Remove(scriptPath)
		_ = os.Remove(updateResultLogPath())
		return fmt.Errorf("启动安装程序失败: %v", err)
	}
	return nil
}

func buildInstallerScript(pid int, installerPath, currentExe string) string {
	var b strings.Builder
	b.WriteString("\ufeff") // UTF-8 BOM，兼容 Windows PowerShell 5.1
	b.WriteString("$ErrorActionPreference = 'Stop'\n")
	b.WriteString(fmt.Sprintf("$targetPid = %d\n", pid))
	b.WriteString(fmt.Sprintf("$installer = %s\n", psQuote(installerPath)))
	b.WriteString(fmt.Sprintf("$currentExe = %s\n", psQuote(currentExe)))
	b.WriteString(fmt.Sprintf("$log = %s\n", psQuote(updateResultLogPath())))
	b.WriteString("$script = $MyInvocation.MyCommand.Path\n")
	b.WriteString("$installerDir = Split-Path -Parent $installer\n")
	b.WriteString("New-Item -ItemType Directory -Force -Path (Split-Path -Parent $log) -ErrorAction SilentlyContinue | Out-Null\n")
	b.WriteString("for ($i = 0; $i -lt 80; $i++) {\n")
	b.WriteString("  if (-not (Get-Process -Id $targetPid -ErrorAction SilentlyContinue)) { break }\n")
	b.WriteString("  Start-Sleep -Milliseconds 250\n")
	b.WriteString("}\n")
	b.WriteString("if (Get-Process -Id $targetPid -ErrorAction SilentlyContinue) {\n")
	b.WriteString("  Set-Content -LiteralPath $log -Value 'ERROR: 等待旧程序退出超时，安装程序未启动' -Encoding UTF8\n")
	b.WriteString("  Remove-Item -LiteralPath $installerDir -Recurse -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("  Remove-Item -LiteralPath $script -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("  exit 1\n")
	b.WriteString("}\n")
	b.WriteString("$installDir = Split-Path -Parent $currentExe\n")
	b.WriteString("try {\n")
	b.WriteString("  # NSIS /D 必须是最后一个参数且不能加引号；这里也不能设 -WorkingDirectory，\n")
	b.WriteString("  # 否则安装目录会被本进程占用，安装器覆盖文件时可能失败。\n")
	b.WriteString("  $proc = Start-Process -FilePath $installer -ArgumentList ('/D=' + $installDir) -Verb RunAs -PassThru\n")
	b.WriteString("  if (-not $proc) { throw '安装程序未能启动' }\n")
	b.WriteString("  $proc.WaitForExit()\n")
	b.WriteString("  if ($proc.ExitCode -ne 0) { throw ('安装程序退出码：' + $proc.ExitCode) }\n")
	b.WriteString("  if (-not (Test-Path -LiteralPath $currentExe)) { throw '安装完成后未找到目标程序' }\n")
	b.WriteString("  Set-Content -LiteralPath $log -Value 'OK' -Encoding UTF8\n")
	b.WriteString("} catch {\n")
	b.WriteString("  Set-Content -LiteralPath $log -Value ('ERROR: ' + $_.Exception.Message) -Encoding UTF8\n")
	b.WriteString("}\n")
	// 安装器完成页可能已经把程序拉起来了，这里只在确实没有实例时补一次，避免开出两个窗口。
	b.WriteString("try {\n")
	b.WriteString("  if (Test-Path -LiteralPath $currentExe) {\n")
	b.WriteString("    $exeName = [System.IO.Path]::GetFileNameWithoutExtension($currentExe)\n")
	b.WriteString("    Start-Sleep -Milliseconds 900\n")
	b.WriteString("    if (-not (Get-Process -Name $exeName -ErrorAction SilentlyContinue)) {\n")
	b.WriteString("      Start-Process -FilePath $currentExe -WorkingDirectory $installDir\n")
	b.WriteString("    }\n")
	b.WriteString("  }\n")
	b.WriteString("} catch { }\n")
	b.WriteString("Remove-Item -LiteralPath $installerDir -Recurse -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("Remove-Item -LiteralPath $script -Force -ErrorAction SilentlyContinue\n")
	return b.String()
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
	b.WriteString("New-Item -ItemType Directory -Force -Path (Split-Path -Parent $log) -ErrorAction SilentlyContinue | Out-Null\n")
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
