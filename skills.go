package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

const skillsStoreFile = "skills.json"

var skillDirNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// SkillService Skills 管理服务
type SkillService struct {
	mu sync.Mutex
}

func NewSkillService() *SkillService {
	return &SkillService{}
}

// Skill 前端展示/编辑结构
type Skill struct {
	Name            string   `json:"name"`
	Content         string   `json:"content"`
	EnablePlatform  []string `json:"enable_platform"`
	EnabledInClaude bool     `json:"enabled_in_claude"`
	EnabledInCodex  bool     `json:"enabled_in_codex"`
	EnabledInGemini bool     `json:"enabled_in_gemini"`

	// 仅用于展示（从 Content 解析）
	FrontmatterName  string `json:"frontmatter_name"`
	Description      string `json:"description"`
	HasFrontmatter   bool   `json:"has_frontmatter"`
	HasName          bool   `json:"has_name"`
	HasDescription   bool   `json:"has_description"`
	FrontmatterError string `json:"frontmatter_error"`
}

type rawSkill struct {
	Content        string   `json:"content"`
	EnablePlatform []string `json:"enable_platform"`
}

func (ss *SkillService) ListSkills() ([]Skill, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	config, changed, err := ss.loadConfigWithImport()
	if err != nil {
		return nil, err
	}
	if changed {
		if err := ss.saveConfig(config); err != nil {
			return nil, err
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(config))
	for name := range config {
		names = append(names, name)
	}
	sort.Strings(names)

	skills := make([]Skill, 0, len(names))
	for _, name := range names {
		entry := normalizeRawSkill(config[name])
		content := entry.Content
		meta := parseSkillFrontmatter(content)

		skills = append(skills, Skill{
			Name:            name,
			Content:         content,
			EnablePlatform:  entry.EnablePlatform,
			EnabledInClaude: fileExists(filepath.Join(home, ".claude", "skills", name, "SKILL.md")),
			EnabledInCodex:  fileExists(filepath.Join(home, ".codex", "skills", name, "SKILL.md")),
			EnabledInGemini: fileExists(filepath.Join(home, ".gemini", "skills", name, "SKILL.md")),

			FrontmatterName:  meta.Name,
			Description:      meta.Description,
			HasFrontmatter:   meta.HasFrontmatter,
			HasName:          meta.HasName,
			HasDescription:   meta.HasDescription,
			FrontmatterError: meta.Error,
		})
	}

	return skills, nil
}

func (ss *SkillService) SaveSkill(skill Skill) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	name := strings.TrimSpace(skill.Name)
	if name == "" {
		return fmt.Errorf("技能名称不能为空")
	}
	if !skillDirNamePattern.MatchString(name) {
		return fmt.Errorf("技能名称格式不正确：只允许小写字母/数字/连字符，且长度 1-64")
	}

	enablePlatform := normalizePlatforms(skill.EnablePlatform)
	if len(enablePlatform) == 0 {
		return fmt.Errorf("请至少选择一个平台")
	}

	content := strings.TrimSpace(skill.Content)
	if content == "" {
		return fmt.Errorf("SKILL.md 内容不能为空")
	}

	meta := parseSkillFrontmatter(content)
	if !meta.HasFrontmatter {
		return fmt.Errorf("SKILL.md 必须以 YAML frontmatter 开头（--- ... ---）")
	}
	if !meta.HasName || strings.TrimSpace(meta.Name) == "" {
		return fmt.Errorf("SKILL.md frontmatter 必须包含 name")
	}
	if !meta.HasDescription || strings.TrimSpace(meta.Description) == "" {
		return fmt.Errorf("SKILL.md frontmatter 必须包含 description")
	}
	if strings.TrimSpace(meta.Name) != name {
		return fmt.Errorf("SKILL.md frontmatter name(%s) 与技能目录名(%s) 不一致", strings.TrimSpace(meta.Name), name)
	}

	config, err := ss.loadConfig()
	if err != nil {
		return err
	}

	config[name] = rawSkill{
		Content:        content,
		EnablePlatform: enablePlatform,
	}

	if err := ss.saveConfig(config); err != nil {
		return err
	}

	return ss.syncSkill(name, config[name])
}

func (ss *SkillService) DeleteSkill(name string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("技能名称不能为空")
	}

	config, err := ss.loadConfig()
	if err != nil {
		return err
	}

	delete(config, trimmed)
	if err := ss.saveConfig(config); err != nil {
		return err
	}

	return ss.removeSkillFromAllPlatforms(trimmed)
}

func (ss *SkillService) configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, skillsStoreFile), nil
}

func (ss *SkillService) loadConfig() (map[string]rawSkill, error) {
	path, err := ss.configPath()
	if err != nil {
		return nil, err
	}

	payload := map[string]rawSkill{}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return payload, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return payload, nil
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	normalized := make(map[string]rawSkill, len(payload))
	for name, entry := range payload {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			continue
		}
		normalized[trimmedName] = normalizeRawSkill(entry)
	}
	return normalized, nil
}

func (ss *SkillService) saveConfig(config map[string]rawSkill) error {
	path, err := ss.configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (ss *SkillService) loadConfigWithImport() (map[string]rawSkill, bool, error) {
	config, err := ss.loadConfig()
	if err != nil {
		return nil, false, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return config, false, nil
	}

	changed := false

	imports := []struct {
		platform string
		root     string
	}{
		{platform: platClaudeCode, root: filepath.Join(home, ".claude", "skills")},
		{platform: platCodex, root: filepath.Join(home, ".codex", "skills")},
		{platform: platGemini, root: filepath.Join(home, ".gemini", "skills")},
	}

	for _, item := range imports {
		discovered := discoverSkillsFromRoot(item.root)
		for name, content := range discovered {
			trimmed := strings.TrimSpace(name)
			if trimmed == "" {
				continue
			}
			if _, exists := config[trimmed]; exists {
				continue
			}
			config[trimmed] = rawSkill{
				Content:        content,
				EnablePlatform: []string{item.platform},
			}
			changed = true
		}
	}

	return config, changed, nil
}

func normalizeRawSkill(entry rawSkill) rawSkill {
	entry.EnablePlatform = normalizePlatforms(entry.EnablePlatform)
	if entry.EnablePlatform == nil {
		entry.EnablePlatform = []string{}
	}
	return entry
}

func discoverSkillsFromRoot(root string) map[string]string {
	result := map[string]string{}
	entries, err := os.ReadDir(root)
	if err != nil {
		return result
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue
		}
		skillPath := filepath.Join(root, name, "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}
		result[name] = string(data)
	}
	return result
}

func (ss *SkillService) syncSkill(name string, entry rawSkill) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	entry = normalizeRawSkill(entry)

	roots := []struct {
		platform string
		root     string
	}{
		{platform: platClaudeCode, root: filepath.Join(home, ".claude", "skills")},
		{platform: platCodex, root: filepath.Join(home, ".codex", "skills")},
		{platform: platGemini, root: filepath.Join(home, ".gemini", "skills")},
	}

	for _, item := range roots {
		dir := filepath.Join(item.root, name)
		file := filepath.Join(dir, "SKILL.md")

		if platformContains(entry.EnablePlatform, item.platform) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(file, []byte(entry.Content), 0o644); err != nil {
				return err
			}
			if item.platform == platGemini {
				if err := ensureGeminiSkillsEnabled(); err != nil {
					return err
				}
			}
			continue
		}

		// 安全卸载：仅删除 SKILL.md（如目录为空则顺带删除目录）
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		_ = removeDirIfEmpty(dir)
	}

	return nil
}

func (ss *SkillService) removeSkillFromAllPlatforms(name string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	roots := []string{
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".codex", "skills"),
		filepath.Join(home, ".gemini", "skills"),
	}

	for _, root := range roots {
		dir := filepath.Join(root, name)
		file := filepath.Join(dir, "SKILL.md")
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		_ = removeDirIfEmpty(dir)
	}
	return nil
}

func removeDirIfEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(entries) != 0 {
		return nil
	}
	return os.Remove(dir)
}

func ensureGeminiSkillsEnabled() error {
	path, err := geminiConfigPath()
	if err != nil {
		return err
	}

	payload := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			payload = map[string]any{}
		}
	}

	experimental, ok := payload["experimental"].(map[string]any)
	if !ok || experimental == nil {
		experimental = map[string]any{}
	}
	experimental["skills"] = true
	payload["experimental"] = experimental

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type skillFrontmatter struct {
	Name          string
	Description   string
	HasFrontmatter bool
	HasName        bool
	HasDescription bool
	Error          string
}

func parseSkillFrontmatter(content string) skillFrontmatter {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.TrimPrefix(normalized, "\ufeff")

	lines := strings.Split(normalized, "\n")

	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) || strings.TrimSpace(lines[i]) != "---" {
		return skillFrontmatter{HasFrontmatter: false}
	}

	i++
	start := i
	for i < len(lines) && strings.TrimSpace(lines[i]) != "---" {
		i++
	}
	if i >= len(lines) {
		return skillFrontmatter{HasFrontmatter: true, Error: "frontmatter 未找到结束分隔符 ---"}
	}

	frontmatterLines := lines[start:i]

	meta := skillFrontmatter{HasFrontmatter: true}
	for idx := 0; idx < len(frontmatterLines); idx++ {
		line := frontmatterLines[idx]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		colon := strings.Index(trimmed, ":")
		if colon <= 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:colon])
		value := strings.TrimSpace(trimmed[colon+1:])

		switch key {
		case "name":
			if value != "" {
				meta.Name = strings.Trim(value, `"'`)
				meta.HasName = true
			}
		case "description":
			meta.HasDescription = true
			// 支持:
			// 1) description: xxx
			// 2) description: |
			//      multi...
			// 3) description:
			//      multi...
			if value == "" || strings.HasPrefix(value, "|") || strings.HasPrefix(value, ">") {
				block := []string{}
				for idx+1 < len(frontmatterLines) {
					next := frontmatterLines[idx+1]
					if next == "" {
						block = append(block, "")
						idx++
						continue
					}
					if strings.HasPrefix(next, " ") || strings.HasPrefix(next, "\t") {
						block = append(block, strings.TrimSpace(next))
						idx++
						continue
					}
					break
				}
				meta.Description = strings.TrimSpace(strings.Join(block, "\n"))
			} else {
				meta.Description = strings.Trim(value, `"'`)
			}
		}
	}

	return meta
}
