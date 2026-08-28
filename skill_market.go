package main

// 技能市场：skills.sh 公开搜索、jsDelivr 仓库树、SkillsMP 中文/全网检索。

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// SkillMarketItem 在线/内置技能市场条目
type SkillMarketItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Repo        string `json:"repo"`
	Path        string `json:"path"`
	Builtin     bool   `json:"builtin"`
}

type skillMarketSource struct {
	ID     string
	Label  string
	Kind   string
	Repo   string
	Branch string
	Dir    string
}

func skillMarketSources() []skillMarketSource {
	return []skillMarketSource{
		{ID: "builtin", Label: "内置", Kind: "builtin"},
		{ID: "online", Label: "热门", Kind: "skillssh"},
		{ID: "anthropic", Label: "Anthropic", Kind: "github", Repo: "anthropics/skills", Branch: "main", Dir: "skills"},
		{ID: "baoyu", Label: "中文", Kind: "github", Repo: "JimLiu/baoyu-skills", Branch: "main", Dir: "skills"},
		{ID: "engineering", Label: "工程", Kind: "github", Repo: "alirezarezvani/claude-skills", Branch: "main"},
		{ID: "vercel", Label: "Vercel", Kind: "github", Repo: "vercel-labs/agent-skills", Branch: "main", Dir: "skills"},
		{ID: "skillsmp", Label: "全网", Kind: "skillsmp"},
	}
}

func skillSourceByID(id string) (skillMarketSource, bool) {
	for _, src := range skillMarketSources() {
		if src.ID == id {
			return src, true
		}
	}
	return skillMarketSource{}, false
}

func (ss *SkillService) ListSkillMarketSources() []map[string]string {
	out := make([]map[string]string, 0, 3)
	for _, src := range skillMarketSources() {
		out = append(out, map[string]string{"id": src.ID, "label": src.Label})
	}
	return out
}

func (ss *SkillService) SearchSkillMarketplace(source, query string) ([]SkillMarketItem, error) {
	source = strings.TrimSpace(strings.ToLower(source))
	if source == "" {
		source = "builtin"
	}
	src, ok := skillSourceByID(source)
	if !ok {
		return nil, fmt.Errorf("未知技能市场")
	}
	var (
		items []SkillMarketItem
		err   error
	)
	switch src.Kind {
	case "builtin":
		items = builtinSkillMarketItems()
	case "skillssh":
		items, err = fetchSkillsShCatalog(query)
		if err != nil {
			items, err = fetchSkillsMPCatalog(query, "")
		}
		return items, err
	case "skillsmp":
		return fetchSkillsMPCatalog(query, "")
	default:
		items, err = fetchGitHubSkillCatalog(src)
		if src.ID == "baoyu" {
			extra, extraErr := fetchSkillsMPCatalog(firstNonEmpty(query, "写作"), "zh")
			if extraErr == nil {
				items = mergeSkillMarketItems(items, extra)
				if len(items) > 0 {
					err = nil
				}
			} else if err != nil {
				err = extraErr
			}
		}
	}
	if err != nil && len(items) == 0 {
		return nil, err
	}
	if strings.TrimSpace(query) == "" || src.Kind == "skillsmp" {
		return items, nil
	}
	filtered := make([]SkillMarketItem, 0, len(items))
	for _, item := range items {
		if containsFold(item.Name+" "+item.Description+" "+item.Path, query) {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == 0 && src.ID == "baoyu" {
		return items, nil
	}
	return filtered, nil
}

func (ss *SkillService) ImportSkillMarketplace(id string) (Skill, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Skill{}, fmt.Errorf("未选择技能")
	}
	if strings.HasPrefix(id, "builtin:") {
		name := strings.TrimPrefix(id, "builtin:")
		for _, preset := range builtinSkillPresets {
			if preset.Name == name {
				return skillFromPreset(preset), nil
			}
		}
		return Skill{}, fmt.Errorf("找不到内置技能 %s", name)
	}
	if strings.HasPrefix(id, "skillssh:") {
		return importSkillsSh(strings.TrimPrefix(id, "skillssh:"))
	}
	item, err := lookupGitHubSkill(id)
	if err != nil {
		return Skill{}, err
	}
	body, err := downloadGitHubSkillMD(item.Repo, item.Path)
	if err != nil {
		return Skill{}, err
	}
	content := strings.TrimSpace(string(body))
	if content == "" {
		return Skill{}, fmt.Errorf("SKILL.md 为空")
	}
	name := slugMarketName(item.Name)
	if !skillDirNamePattern.MatchString(name) {
		name = slugMarketName(lastPathSegment(item.Path))
	}
	content = alignSkillFrontmatter(content, name, item.Description)
	meta := parseSkillFrontmatter(content)
	return Skill{
		Name:             name,
		Content:          content,
		EnablePlatform:   []string{platClaudeCode, platCodex, platGemini, platOpencode, platGrok},
		FrontmatterName:  meta.Name,
		Description:      firstNonEmpty(meta.Description, item.Description),
		HasFrontmatter:   meta.HasFrontmatter,
		HasName:          meta.HasName,
		HasDescription:   meta.HasDescription,
		FrontmatterError: meta.Error,
	}, nil
}

func builtinSkillMarketItems() []SkillMarketItem {
	out := make([]SkillMarketItem, 0, len(builtinSkillPresets))
	for _, preset := range builtinSkillPresets {
		out = append(out, SkillMarketItem{
			ID:          "builtin:" + preset.Name,
			Name:        preset.Name,
			Description: preset.Description,
			Source:      "builtin",
			Builtin:     true,
		})
	}
	return out
}

func skillFromPreset(preset SkillPreset) Skill {
	meta := parseSkillFrontmatter(preset.Content)
	return Skill{
		Name:            preset.Name,
		Content:         preset.Content,
		EnablePlatform:  []string{platClaudeCode, platCodex, platGemini, platOpencode, platGrok},
		FrontmatterName: firstNonEmpty(meta.Name, preset.Name),
		Description:     firstNonEmpty(meta.Description, preset.Description),
		HasFrontmatter:  meta.HasFrontmatter,
		HasName:         meta.HasName,
		HasDescription:  meta.HasDescription,
	}
}

func fetchGitHubSkillCatalog(src skillMarketSource) ([]SkillMarketItem, error) {
	branch := src.Branch
	if branch == "" {
		branch = "main"
	}
	var last error
	if data, err := marketGetGitHubFile(src.Repo, branch, ".claude-plugin/marketplace.json"); err == nil {
		if items := parseClaudeMarketplace(src, data); len(items) > 0 {
			return items, nil
		}
	} else {
		last = err
	}
	if items, err := fetchJsdelivrSkillTree(src, branch); err == nil && len(items) > 0 {
		return items, nil
	} else if err != nil {
		last = err
	}
	if items, err := fetchGitHubSkillTree(src, branch); err == nil && len(items) > 0 {
		return items, nil
	} else if err != nil {
		last = err
	}
	dir := src.Dir
	if dir == "" {
		dir = "skills"
	}
	data, err := marketGetGitHubAPI("repos/" + src.Repo + "/contents/" + dir)
	if err != nil {
		if last != nil {
			return nil, fmt.Errorf("读取 %s 失败: %v", src.Label, last)
		}
		return nil, fmt.Errorf("读取 %s 失败: %v", src.Label, err)
	}
	var entries []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	items := make([]SkillMarketItem, 0, len(entries))
	for _, entry := range entries {
		if entry.Type != "dir" || strings.HasPrefix(entry.Name, ".") {
			continue
		}
		name := slugMarketName(entry.Name)
		items = append(items, SkillMarketItem{
			ID:          "github:" + src.Repo + ":" + entry.Path,
			Name:        name,
			Description: src.Label + " / " + entry.Name,
			Source:      src.ID,
			Repo:        src.Repo,
			Path:        entry.Path,
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%s 没有公开技能", src.Label)
	}
	return items, nil
}

type jsdelivrNode struct {
	Type  string         `json:"type"`
	Name  string         `json:"name"`
	Files []jsdelivrNode `json:"files"`
}

func fetchJsdelivrSkillTree(src skillMarketSource, branch string) ([]SkillMarketItem, error) {
	data, err := marketHTTPGet("https://data.jsdelivr.com/v1/packages/gh/"+src.Repo+"@"+branch, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Files []jsdelivrNode `json:"files"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	paths := make([]string, 0)
	walkJsdelivrFiles(payload.Files, "", &paths)
	items := make([]SkillMarketItem, 0)
	seen := map[string]bool{}
	for _, path := range paths {
		if shouldSkipSkillPath(path) {
			continue
		}
		dir := strings.TrimSuffix(path, "/SKILL.md")
		dir = strings.TrimSuffix(dir, "/skill.md")
		if dir == "" || dir == path {
			continue
		}
		key := strings.ToLower(dir)
		if seen[key] {
			continue
		}
		seen[key] = true
		folder := lastPathSegment(dir)
		items = append(items, SkillMarketItem{
			ID:          "github:" + src.Repo + ":" + dir,
			Name:        slugMarketName(folder),
			Description: src.Label + " / " + dir,
			Source:      src.ID,
			Repo:        src.Repo,
			Path:        dir,
		})
		if len(items) >= 400 {
			break
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%s 没有 SKILL.md", src.Label)
	}
	return items, nil
}

func walkJsdelivrFiles(nodes []jsdelivrNode, prefix string, out *[]string) {
	for _, node := range nodes {
		path := node.Name
		if prefix != "" {
			path = prefix + "/" + node.Name
		}
		if strings.EqualFold(node.Type, "file") && strings.EqualFold(node.Name, "SKILL.md") {
			*out = append(*out, path)
			continue
		}
		if len(node.Files) > 0 {
			walkJsdelivrFiles(node.Files, path, out)
		}
	}
}

func fetchGitHubSkillTree(src skillMarketSource, branch string) ([]SkillMarketItem, error) {
	data, err := marketGetGitHubAPI("repos/" + src.Repo + "/git/trees/" + branch + "?recursive=1")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	items := make([]SkillMarketItem, 0)
	seen := map[string]bool{}
	for _, node := range payload.Tree {
		if node.Type != "blob" {
			continue
		}
		path := strings.ReplaceAll(node.Path, "\\", "/")
		if !strings.HasSuffix(strings.ToLower(path), "/skill.md") && !strings.EqualFold(path, "SKILL.md") {
			continue
		}
		if shouldSkipSkillPath(path) {
			continue
		}
		dir := strings.TrimSuffix(path, "/SKILL.md")
		dir = strings.TrimSuffix(dir, "/skill.md")
		if dir == "" || dir == path {
			continue
		}
		key := strings.ToLower(dir)
		if seen[key] {
			continue
		}
		seen[key] = true
		folder := lastPathSegment(dir)
		items = append(items, SkillMarketItem{
			ID:          "github:" + src.Repo + ":" + dir,
			Name:        slugMarketName(folder),
			Description: src.Label + " / " + dir,
			Source:      src.ID,
			Repo:        src.Repo,
			Path:        dir,
		})
		if len(items) >= 400 {
			break
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%s 树里没有 SKILL.md", src.Label)
	}
	return items, nil
}

func shouldSkipSkillPath(path string) bool {
	lower := strings.ToLower("/" + strings.TrimPrefix(path, "/"))
	for _, skip := range []string{
		"/.github/", "/node_modules/", "/template/", "/spec/",
		"/examples/", "/docs/", "/test/", "/tests/",
	} {
		if strings.Contains(lower, skip) {
			return true
		}
	}
	return false
}

func downloadGitHubSkillMD(repo, skillDir string) ([]byte, error) {
	skillDir = strings.TrimPrefix(strings.Trim(strings.ReplaceAll(skillDir, "\\", "/"), "/"), "./")
	candidates := []string{
		skillDir + "/SKILL.md",
		skillDir + "/skill.md",
	}
	if !strings.Contains(skillDir, "/") {
		candidates = append(candidates,
			"skills/"+skillDir+"/SKILL.md",
			".claude/skills/"+skillDir+"/SKILL.md",
			".agents/skills/"+skillDir+"/SKILL.md",
		)
	}
	var last error
	for _, rel := range candidates {
		for _, branch := range []string{"main", "master"} {
			body, err := marketGetGitHubFile(repo, branch, rel)
			if err == nil && len(bytesTrimSpace(body)) > 0 {
				return body, nil
			}
			last = err
		}
	}
	if last == nil {
		last = fmt.Errorf("下载 SKILL.md 失败")
	}
	return nil, last
}

func bytesTrimSpace(data []byte) []byte {
	return []byte(strings.TrimSpace(string(data)))
}

func fetchSkillsShCatalog(query string) ([]SkillMarketItem, error) {
	query = strings.TrimSpace(query)
	queries := []string{query}
	if query == "" {
		queries = []string{"skill", "code", "frontend", "pdf", "git", "claude"}
	}
	seen := map[string]bool{}
	items := make([]SkillMarketItem, 0)
	var last error
	for _, q := range queries {
		rawURL := "https://skills.sh/api/search?q=" + url.QueryEscape(q) + "&limit=40"
		data, err := marketHTTPGet(rawURL, 15*time.Second)
		if err != nil {
			last = err
			continue
		}
		for _, item := range parseSkillsShList(data) {
			if seen[item.ID] {
				continue
			}
			seen[item.ID] = true
			items = append(items, item)
		}
		if query != "" && len(items) > 0 {
			break
		}
	}
	if len(items) == 0 {
		if last != nil {
			return nil, fmt.Errorf("读取 skills.sh 失败: %v", last)
		}
		return nil, fmt.Errorf("skills.sh 没有匹配的技能")
	}
	if len(items) > 80 {
		items = items[:80]
	}
	return items, nil
}

func parseSkillsShList(data []byte) []SkillMarketItem {
	var payload struct {
		Data []json.RawMessage `json:"data"`
		Skills []json.RawMessage `json:"skills"`
		Results []json.RawMessage `json:"results"`
	}
	rows := []json.RawMessage{}
	if err := json.Unmarshal(data, &payload); err == nil {
		switch {
		case len(payload.Data) > 0:
			rows = payload.Data
		case len(payload.Skills) > 0:
			rows = payload.Skills
		case len(payload.Results) > 0:
			rows = payload.Results
		}
	}
	if len(rows) == 0 {
		_ = json.Unmarshal(data, &rows)
	}
	items := make([]SkillMarketItem, 0, len(rows))
	seen := map[string]bool{}
	for _, raw := range rows {
		item, ok := parseSkillsShItem(raw)
		if !ok || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		items = append(items, item)
	}
	return items
}

func parseSkillsShItem(raw json.RawMessage) (SkillMarketItem, bool) {
	var body struct {
		ID          string `json:"id"`
		SkillID     string `json:"skillId"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Source      string `json:"source"`
		Description string `json:"description"`
		Installs    int    `json:"installs"`
		URL         string `json:"url"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return SkillMarketItem{}, false
	}
	id := strings.TrimSpace(body.ID)
	if id == "" && body.Source != "" {
		slug := firstNonEmpty(body.Slug, body.SkillID)
		if slug != "" {
			id = strings.Trim(body.Source, "/") + "/" + strings.Trim(slug, "/")
		}
	}
	if id == "" {
		return SkillMarketItem{}, false
	}
	name := slugMarketName(firstNonEmpty(body.Slug, lastPathSegment(id), body.Name))
	desc := strings.TrimSpace(body.Description)
	if desc == "" {
		desc = firstNonEmpty(body.Name, body.Source)
	}
	if body.Installs > 0 {
		desc = fmt.Sprintf("%s · %d 安装", desc, body.Installs)
	}
	repo, path := splitSkillsShID(id)
	return SkillMarketItem{
		ID:          "skillssh:" + id,
		Name:        name,
		Description: desc,
		Source:      "online",
		Repo:        repo,
		Path:        path,
	}, true
}

func mergeSkillMarketItems(base, extra []SkillMarketItem) []SkillMarketItem {
	seen := map[string]bool{}
	out := make([]SkillMarketItem, 0, len(base)+len(extra))
	for _, item := range append(base, extra...) {
		key := strings.ToLower(item.ID)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func fetchSkillsMPCatalog(query, lang string) ([]SkillMarketItem, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		if lang == "zh" {
			query = "写作"
		} else {
			query = "claude"
		}
	}
	rawURL := "https://skillsmp.com/api/v1/skills/search?q=" + url.QueryEscape(query) + "&limit=40&sortBy=stars"
	if lang != "" {
		rawURL += "&language=" + url.QueryEscape(lang)
	}
	data, err := marketHTTPGet(rawURL, 18*time.Second)
	if err != nil {
		return nil, fmt.Errorf("读取 SkillsMP 失败: %v", err)
	}
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Skills []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Author      string `json:"author"`
				Description string `json:"description"`
				GitHubURL   string `json:"githubUrl"`
				Stars       int    `json:"stars"`
			} `json:"skills"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	items := make([]SkillMarketItem, 0)
	seen := map[string]bool{}
	for _, row := range payload.Data.Skills {
		repo, path, ok := parseGitHubRepoPath(row.GitHubURL)
		if !ok {
			continue
		}
		id := "github:" + repo + ":" + path
		if path == "" {
			id = "github:" + repo + ":" + slugMarketName(row.Name)
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		desc := strings.TrimSpace(row.Description)
		if desc == "" {
			desc = firstNonEmpty(row.Author, repo)
		}
		if row.Stars > 0 {
			desc = fmt.Sprintf("%s · %d★", desc, row.Stars)
		}
		items = append(items, SkillMarketItem{
			ID:          id,
			Name:        slugMarketName(firstNonEmpty(row.Name, lastPathSegment(path))),
			Description: desc,
			Source:      "skillsmp",
			Repo:        repo,
			Path:        path,
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("SkillsMP 没有可导入的技能")
	}
	return items, nil
}

func parseGitHubRepoPath(raw string) (repo, path string, ok bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "www.")
	if !strings.HasPrefix(raw, "github.com/") {
		return "", "", false
	}
	rest := strings.Trim(strings.TrimPrefix(raw, "github.com/"), "/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return "", "", false
	}
	repo = parts[0] + "/" + parts[1]
	if len(parts) >= 5 && (parts[2] == "tree" || parts[2] == "blob") {
		path = strings.Join(parts[4:], "/")
	} else if len(parts) > 2 {
		path = strings.Join(parts[2:], "/")
	}
	path = strings.TrimSuffix(path, "/SKILL.md")
	path = strings.TrimSuffix(path, "/skill.md")
	path = strings.Trim(path, "/")
	return repo, path, true
}

func splitSkillsShID(id string) (repo, path string) {
	parts := strings.Split(strings.Trim(id, "/"), "/")
	if len(parts) < 3 {
		return strings.TrimSpace(id), ""
	}
	return parts[0] + "/" + parts[1], strings.Join(parts[2:], "/")
}

func importSkillsSh(id string) (Skill, error) {
	id = strings.Trim(id, "/")
	if id == "" {
		return Skill{}, fmt.Errorf("技能 ID 无效")
	}
	data, err := marketHTTPGet("https://skills.sh/api/v1/skills/"+id, 15*time.Second)
	content := ""
	name := slugMarketName(lastPathSegment(id))
	desc := ""
	if err == nil {
		var detail struct {
			Name        string `json:"name"`
			Slug        string `json:"slug"`
			Description string `json:"description"`
			Files       []struct {
				Path     string `json:"path"`
				Contents string `json:"contents"`
			} `json:"files"`
		}
		if json.Unmarshal(data, &detail) == nil {
			desc = strings.TrimSpace(detail.Description)
			if strings.TrimSpace(detail.Slug) != "" {
				name = slugMarketName(detail.Slug)
			} else if strings.TrimSpace(detail.Name) != "" {
				name = slugMarketName(detail.Name)
			}
			for _, file := range detail.Files {
				if strings.EqualFold(lastPathSegment(file.Path), "SKILL.md") && strings.TrimSpace(file.Contents) != "" {
					content = strings.TrimSpace(file.Contents)
					break
				}
			}
		}
	}
	if content == "" {
		repo, path := splitSkillsShID(id)
		if repo != "" {
			if path == "" {
				path = lastPathSegment(id)
			}
			body, downErr := downloadGitHubSkillMD(repo, path)
			if downErr != nil {
				return Skill{}, fmt.Errorf("下载 SKILL.md 失败: %v", downErr)
			}
			content = strings.TrimSpace(string(body))
		}
	}
	if content == "" {
		return Skill{}, fmt.Errorf("没有找到 SKILL.md")
	}
	if !skillDirNamePattern.MatchString(name) {
		name = slugMarketName(lastPathSegment(id))
	}
	content = alignSkillFrontmatter(content, name, desc)
	meta := parseSkillFrontmatter(content)
	return Skill{
		Name:             name,
		Content:          content,
		EnablePlatform:   []string{platClaudeCode, platCodex, platGemini, platOpencode, platGrok},
		FrontmatterName:  meta.Name,
		Description:      firstNonEmpty(meta.Description, desc),
		HasFrontmatter:   meta.HasFrontmatter,
		HasName:          meta.HasName,
		HasDescription:   meta.HasDescription,
		FrontmatterError: meta.Error,
	}, nil
}

func parseClaudeMarketplace(src skillMarketSource, data []byte) []SkillMarketItem {
	var payload struct {
		Plugins []struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Skills      []string `json:"skills"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	seen := map[string]bool{}
	items := make([]SkillMarketItem, 0)
	for _, plugin := range payload.Plugins {
		for _, rel := range plugin.Skills {
			rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
			rel = strings.TrimPrefix(rel, "./")
			rel = strings.TrimPrefix(rel, "/")
			if rel == "" {
				continue
			}
			path := rel
			if !strings.Contains(rel, "/") && src.Dir != "" {
				path = strings.Trim(src.Dir, "/") + "/" + rel
			}
			key := strings.ToLower(path)
			if seen[key] {
				continue
			}
			seen[key] = true
			folder := lastPathSegment(path)
			items = append(items, SkillMarketItem{
				ID:          "github:" + src.Repo + ":" + path,
				Name:        slugMarketName(folder),
				Description: firstNonEmpty(strings.TrimSpace(plugin.Description), plugin.Name+" / "+folder),
				Source:      src.ID,
				Repo:        src.Repo,
				Path:        path,
			})
		}
	}
	return items
}

func lookupGitHubSkill(id string) (SkillMarketItem, error) {
	// github:owner/repo:path
	if !strings.HasPrefix(id, "github:") {
		return SkillMarketItem{}, fmt.Errorf("不支持的技能来源")
	}
	rest := strings.TrimPrefix(id, "github:")
	idx := strings.Index(rest, ":")
	if idx <= 0 {
		return SkillMarketItem{}, fmt.Errorf("技能 ID 无效")
	}
	repo := rest[:idx]
	path := rest[idx+1:]
	return SkillMarketItem{
		ID:   id,
		Name: lastPathSegment(path),
		Repo: repo,
		Path: path,
	}, nil
}

var skillNameLine = regexp.MustCompile(`(?m)^name:\s*.+$`)
var skillDescLine = regexp.MustCompile(`(?m)^description:\s*.+$`)

func alignSkillFrontmatter(content, name, fallbackDesc string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		desc := strings.TrimSpace(fallbackDesc)
		if desc == "" {
			desc = name
		}
		return "---\nname: " + name + "\ndescription: " + desc + "\n---\n\n" + content
	}
	if skillNameLine.MatchString(content) {
		content = skillNameLine.ReplaceAllString(content, "name: "+name)
	} else {
		content = strings.Replace(content, "---", "---\nname: "+name, 1)
	}
	if !skillDescLine.MatchString(content) && strings.TrimSpace(fallbackDesc) != "" {
		content = strings.Replace(content, "name: "+name, "name: "+name+"\ndescription: "+strings.TrimSpace(fallbackDesc), 1)
	}
	return content
}
