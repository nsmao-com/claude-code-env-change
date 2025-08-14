package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// EnvConfig 环境配置
type EnvConfig struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Variables   map[string]string `json:"variables"`
}

// Config 主配置
type Config struct {
	CurrentEnv    string      `json:"current_env"`
	Environments  []EnvConfig `json:"environments"`
}

// App struct
type App struct {
	ctx        context.Context
	configPath string
	config     Config
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		configPath: "config.json",
	}
}

// OnStartup is called when the app starts up
func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
	a.loadConfig()
}

// GetConfig 获取配置
func (a *App) GetConfig() Config {
	return a.config
}

// GetEnvVar 获取环境变量
func (a *App) GetEnvVar(key string) string {
	// 在Windows系统上，直接从注册表读取最新值
	if runtime.GOOS == "windows" {
		return a.getWindowsEnvVar(key)
	}
	return os.Getenv(key)
}

// getWindowsEnvVar 从Windows注册表读取环境变量
func (a *App) getWindowsEnvVar(key string) string {
	// 使用reg query命令查询用户环境变量
	cmd := exec.Command("reg", "query", "HKCU\\Environment", "/v", key)
	output, err := cmd.Output()
	if err != nil {
		// 如果用户环境变量不存在，尝试读取进程环境变量
		return os.Getenv(key)
	}
	
	// 解析reg命令的输出
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 查找包含变量名和REG_SZ的行
		if strings.Contains(line, key) && strings.Contains(line, "REG_SZ") {
			// 使用正则表达式或字符串分割来解析
			// 格式通常是: "变量名    REG_SZ    值"
			regSzIndex := strings.Index(line, "REG_SZ")
			if regSzIndex != -1 {
				// 获取REG_SZ后面的值
				valueStart := regSzIndex + len("REG_SZ")
				if valueStart < len(line) {
					value := strings.TrimSpace(line[valueStart:])
					return value
				}
			}
		}
	}
	
	// 如果解析失败，返回进程环境变量
	return os.Getenv(key)
}

// SetEnvVar 设置环境变量
func (a *App) SetEnvVar(key, value string) error {
	// 设置当前进程的环境变量
	err := os.Setenv(key, value)
	if err != nil {
		return fmt.Errorf("设置环境变量失败: %v", err)
	}
	
	// 在Windows系统上，同时设置系统环境变量
	if runtime.GOOS == "windows" {
		return a.setWindowsEnvVar(key, value)
	}
	
	return nil
}

// setWindowsEnvVar 在Windows系统上设置环境变量
func (a *App) setWindowsEnvVar(key, value string) error {
	// 使用setx命令设置用户环境变量，不添加双引号
	cmd_exec := exec.Command("setx", key, value)
	
	output, err := cmd_exec.CombinedOutput()
	if err != nil {
		return fmt.Errorf("执行setx命令失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

// deleteWindowsEnvVar 在Windows系统上删除环境变量
func (a *App) deleteWindowsEnvVar(key string) error {
	// 使用reg命令删除用户环境变量
	cmd_exec := exec.Command("reg", "delete", "HKCU\\Environment", "/v", key, "/f")
	
	output, err := cmd_exec.CombinedOutput()
	if err != nil {
		// 如果环境变量不存在，reg命令会返回错误，但这不是真正的错误
		if !strings.Contains(string(output), "无法找到指定的注册表项或值") && !strings.Contains(string(output), "The system was unable to find the specified registry key or value") {
			return fmt.Errorf("执行reg delete命令失败: %v, 输出: %s", err, string(output))
		}
	}
	return nil
}

// SwitchToEnv 切换环境
func (a *App) SwitchToEnv(name string) error {
	a.config.CurrentEnv = name
	return a.saveConfig()
}

// AddEnv adds a new environment configuration
func (a *App) AddEnv(env EnvConfig) error {
	// Check if environment already exists
	for i, existing := range a.config.Environments {
		if existing.Name == env.Name {
			// Update existing environment
			a.config.Environments[i] = env
			return a.saveConfig()
		}
	}
	
	// Add new environment
	a.config.Environments = append(a.config.Environments, env)
	return a.saveConfig()
}

// DeleteEnv deletes an environment configuration by name
func (a *App) DeleteEnv(name string) error {
	for i, env := range a.config.Environments {
		if env.Name == name {
			// Remove environment from slice
			a.config.Environments = append(a.config.Environments[:i], a.config.Environments[i+1:]...)
			// If deleted environment is current, clear current
			if a.config.CurrentEnv == name {
				a.config.CurrentEnv = ""
			}
			return a.saveConfig()
		}
	}
	return fmt.Errorf("environment '%s' not found", name)
}

// ApplyCurrentEnv 应用当前环境
func (a *App) ApplyCurrentEnv() (string, error) {
	if a.config.CurrentEnv == "" {
		return "", fmt.Errorf("请先选择一个环境")
	}
	
	// 查找当前环境
	var currentEnv *EnvConfig
	for _, env := range a.config.Environments {
		if env.Name == a.config.CurrentEnv {
			currentEnv = &env
			break
		}
	}
	
	if currentEnv == nil {
		return "", fmt.Errorf("当前环境不存在")
	}
	
	// 生成环境变量设置脚本
	var script strings.Builder
	var filename string
	
	if runtime.GOOS == "windows" {
		script.WriteString("@echo off\n")
		script.WriteString("echo 正在设置 Claude Code 环境变量...\n")
		for key, value := range currentEnv.Variables {
			script.WriteString(fmt.Sprintf("set %s=%s\n", key, value))
		}
		script.WriteString("echo 环境变量设置完成！\n")
		script.WriteString("pause\n")
		filename = "claude_env_setup.bat"
	} else {
		script.WriteString("#!/bin/bash\n")
		script.WriteString("echo \"正在设置 Claude Code 环境变量...\"\n")
		for key, value := range currentEnv.Variables {
			script.WriteString(fmt.Sprintf("export %s=%s\n", key, value))
		}
		script.WriteString("echo \"环境变量设置完成！\"\n")
		filename = "claude_env_setup.sh"
	}
	
	// 保存脚本文件
	err := os.WriteFile(filename, []byte(script.String()), 0755)
	if err != nil {
		return "", fmt.Errorf("保存脚本文件失败: %v", err)
	}
	
	return filename, nil
}

// ClearEnvVar 清除特定环境变量
func (a *App) ClearEnvVar(key string) error {
	// 清除当前进程的环境变量
	err := os.Unsetenv(key)
	if err != nil {
		return fmt.Errorf("清除进程环境变量失败: %v", err)
	}
	
	// 在Windows系统上，同时删除系统环境变量
	if runtime.GOOS == "windows" {
		return a.deleteWindowsEnvVar(key)
	}
	
	return nil
}

// ClearAllEnv 清除所有相关环境变量
func (a *App) ClearAllEnv() error {
	// 定义需要清除的环境变量
	envVars := []string{
		"ANTHROPIC_BASE_URL",
		"ANTHROPIC_AUTH_TOKEN",
		"ANTHROPIC_API_KEY",
		"CLAUDE_MODEL",
		"API_BASE_URL",
		"CLAUDE_MAX_TOKENS",
		"CLAUDE_TEMPERATURE",
	}
	
	// 逐个清除环境变量
	for _, key := range envVars {
		err := a.ClearEnvVar(key)
		if err != nil {
			// 记录错误但继续清除其他变量
			fmt.Printf("清除环境变量 %s 时出错: %v\n", key, err)
		}
	}
	
	return nil
}

// RefreshConfig 刷新配置
func (a *App) RefreshConfig() error {
	return a.loadConfig()
}

func (a *App) loadConfig() error {
	// 如果配置文件不存在，创建默认配置
	if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
		a.config = Config{
			Environments: []EnvConfig{
				{
					Name:        "Development",
					Description: "开发环境",
					Variables: map[string]string{
						"ANTHROPIC_API_KEY": "your-dev-api-key",
						"CLAUDE_MODEL":      "claude-3-5-sonnet-20241022",
						"API_BASE_URL":      "https://api.anthropic.com",
					},
				},
				{
					Name:        "Production",
					Description: "生产环境",
					Variables: map[string]string{
						"ANTHROPIC_API_KEY": "your-prod-api-key",
						"CLAUDE_MODEL":      "claude-3-5-sonnet-20241022",
						"API_BASE_URL":      "https://api.anthropic.com",
						"CLAUDE_MAX_TOKENS": "4096",
					},
				},
			},
			CurrentEnv: "Development",
		}
		return a.saveConfig()
	}

	// 读取配置文件
	data, err := os.ReadFile(a.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	err = json.Unmarshal(data, &a.config)
	if err != nil {
		return fmt.Errorf("解析配置文件失败: %v", err)
	}
	
	return nil
}

func (a *App) saveConfig() error {
	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	err = os.WriteFile(a.configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("保存配置文件失败: %v", err)
	}
	
	return nil
}