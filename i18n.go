package main

// 后端提示信息的多语言：界面切换语言时前端调用 SetLanguage，后端记住并落盘
// （托盘、启动时的自动拉取等在前端就绪之前就会产生提示）。
//
// 约定：面向用户的中文文字一律经过 tr / errorf / sprintf / trError，
// 英文对照在 messages_en.go，以中文原文为键。i18n_test.go 会扫描源码，
// 保证每一处中文都有英文、占位符一致、参数个数对得上。
// 日志、写进文件的数据（脚本、路由描述等）与用于匹配的字符串不翻译。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const uiSettingsFile = "ui.json"

var (
	backendLangMu sync.RWMutex
	backendLang   = "zh"
)

func normalizeLanguage(lang string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "en") {
		return "en"
	}
	return "zh"
}

func backendLanguage() string {
	backendLangMu.RLock()
	defer backendLangMu.RUnlock()
	return backendLang
}

func setBackendLanguage(lang string) {
	backendLangMu.Lock()
	backendLang = normalizeLanguage(lang)
	backendLangMu.Unlock()
}

// tr 把中文原文换成当前语言；没有对照时原样返回
func tr(zh string) string {
	if backendLanguage() == "en" {
		if en, ok := enMessages[zh]; ok {
			return en
		}
	}
	return zh
}

// errorf 与 fmt.Errorf 相同（%w 照常可用），格式串按当前语言翻译
func errorf(format string, args ...any) error {
	return fmt.Errorf(tr(format), args...)
}

// sprintf 与 fmt.Sprintf 相同，格式串按当前语言翻译
func sprintf(format string, args ...any) string {
	return fmt.Sprintf(tr(format), args...)
}

// untranslatedf 与 fmt.Sprintf 相同，明确标记"这是写进文件的数据，不翻译"
func untranslatedf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

// trError 包级错误变量用：Error() 调用时才翻译，errors.Is 比较不受语言影响
type trError string

func (e trError) Error() string { return tr(string(e)) }

func uiSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, mcpStoreDir, uiSettingsFile), nil
}

// loadBackendLanguage 启动时读取上次的界面语言
func loadBackendLanguage() {
	path, err := uiSettingsPath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var ui struct {
		Language string `json:"language"`
	}
	if json.Unmarshal(data, &ui) == nil && ui.Language != "" {
		setBackendLanguage(ui.Language)
	}
}

// SetLanguage 界面切换语言时调用：之后的提示信息、托盘文字都用这个语言
func (a *App) SetLanguage(lang string) error {
	lang = normalizeLanguage(lang)
	changed := backendLanguage() != lang
	setBackendLanguage(lang)
	if changed {
		trayLanguageChanged()
	}
	path, err := uiSettingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, _ := json.Marshal(map[string]string{"language": lang})
	return writeFileAtomic(path, data, 0o644)
}

// GetLanguage 后端当前使用的语言
func (a *App) GetLanguage() string {
	return backendLanguage()
}
