package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type SkillSource struct {
	Repo     string            `json:"repo"`
	Path     string            `json:"path"`
	Ref      string            `json:"ref"`
	Revision string            `json:"revision"`
	Hashes   map[string]string `json:"hashes"`
}
type SkillUpdate struct {
	Name              string       `json:"name"`
	Token             string       `json:"token"`
	Revision          string       `json:"revision"`
	Changes           []string     `json:"changes"`
	Conflicts         []string     `json:"conflicts"`
	PlatformConflicts []string     `json:"platform_conflicts"`
	Details           []FileChange `json:"details"`
	Skill             Skill        `json:"-"`
}

var pendingSkillUpdates = struct {
	sync.Mutex
	items map[string]SkillUpdate
}{items: map[string]SkillUpdate{}}

func safeSkillRelative(name string) bool {
	if name == "" || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".git" || part == "node_modules" || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if regexp.MustCompile(`^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])$`).MatchString(base) {
			return false
		}
	}
	return true
}
func validateSkillFiles(files map[string][]byte) error {
	if len(files) > 1000 {
		return fmt.Errorf("技能最多包含 1000 个附件")
	}
	total := 0
	seen := map[string]bool{}
	for name, b := range files {
		if !safeSkillRelative(name) || strings.EqualFold(name, "SKILL.md") || name == ".ai-env-manifest.json" {
			return fmt.Errorf("技能附件路径无效: %s", name)
		}
		key := strings.ToLower(name)
		if seen[key] {
			return fmt.Errorf("技能附件存在大小写冲突: %s", name)
		}
		seen[key] = true
		total += len(b)
		if len(b) > 8<<20 || total > 24<<20 {
			return fmt.Errorf("技能单文件不可超过 8 MB，总附件不可超过 24 MB")
		}
	}
	return nil
}
func readLocalSkillAssets(dir string) (map[string][]byte, map[string]bool, error) {
	files := map[string][]byte{}
	executable := map[string]bool{}
	total := int64(0)
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, nil, err
	}
	err = filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if e.IsDir() && (e.Name() == ".git" || e.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if e.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("技能附件不支持符号链接: %s", rel)
		}
		if !safeSkillRelative(rel) {
			return fmt.Errorf("技能附件路径不安全: %s", rel)
		}
		if e.IsDir() || strings.EqualFold(rel, "SKILL.md") || rel == ".ai-env-manifest.json" {
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		if info.Size() > 8<<20 || total+info.Size() > 24<<20 || len(files) >= 1000 {
			return fmt.Errorf("技能包超过限制，未导入: %s (单文件 8 MB、总附件 24 MB、1000 个附件)", rel)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[rel] = b
		if info.Mode()&0111 != 0 {
			executable[rel] = true
		}
		total += int64(len(b))
		return nil
	})
	return files, executable, err
}
func skillPackageFromZip(data []byte, requested string) (Skill, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Skill{}, fmt.Errorf("ZIP 文件无效: %w", err)
	}
	requested = strings.Trim(strings.ReplaceAll(requested, "\\", "/"), "/")
	candidates := []string{}
	for _, f := range z.File {
		if strings.EqualFold(path.Base(f.Name), "SKILL.md") {
			dir := path.Dir(f.Name)
			if requested == "" || dir == requested || strings.HasSuffix(dir, "/"+requested) {
				candidates = append(candidates, dir)
			}
		}
	}
	if len(candidates) != 1 {
		return Skill{}, fmt.Errorf("找到 %d 个匹配技能，请指定包含 SKILL.md 的目录", len(candidates))
	}
	root := candidates[0]
	files := map[string][]byte{}
	executable := map[string]bool{}
	content := ""
	total := uint64(0)
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rel := strings.TrimPrefix(f.Name, root+"/")
		if root == "." {
			rel = f.Name
		}
		if rel == f.Name && root != "." {
			continue
		}
		if !safeSkillRelative(rel) || f.Mode()&os.ModeSymlink != 0 {
			return Skill{}, fmt.Errorf("技能含不安全路径或符号链接: %s", rel)
		}
		total += f.UncompressedSize64
		if f.UncompressedSize64 > 8<<20 || total > 24<<20 || len(files) > 1000 {
			return Skill{}, fmt.Errorf("技能包超过大小或文件数量限制")
		}
		reader, e := f.Open()
		if e != nil {
			return Skill{}, e
		}
		b, e := io.ReadAll(io.LimitReader(reader, (8<<20)+1))
		reader.Close()
		if e != nil {
			return Skill{}, e
		}
		if len(b) > 8<<20 {
			return Skill{}, fmt.Errorf("技能文件过大")
		}
		if strings.EqualFold(rel, "SKILL.md") {
			content = string(b)
		} else {
			if _, exists := files[rel]; exists {
				return Skill{}, fmt.Errorf("ZIP 包含重复文件")
			}
			files[rel] = b
			if f.Mode()&0111 != 0 {
				executable[rel] = true
			}
		}
	}
	if err = validateSkillFiles(files); err != nil {
		return Skill{}, err
	}
	meta := parseSkillFrontmatter(content)
	name := slugMarketName(meta.Name)
	if name == "" {
		name = slugMarketName(path.Base(root))
	}
	if name == "" {
		name = "imported-skill"
	}
	content = alignSkillFrontmatter(content, name, meta.Description)
	return Skill{Name: name, Content: content, Files: files, Executable: executable, EnablePlatform: []string{platClaudeCode}, Description: meta.Description}, nil
}
func downloadSkillPackage(repo, dir, ref string) (Skill, error) {
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(repo) {
		return Skill{}, fmt.Errorf("仓库格式应为 owner/repo")
	}
	if ref == "" {
		ref = "main"
	}
	if strings.ContainsAny(ref, "\r\n?#") {
		return Skill{}, fmt.Errorf("版本格式无效")
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	metadata, err := marketGetGitHubAPI("repos/" + repo + "/commits/" + url.PathEscape(ref))
	if err != nil {
		return Skill{}, fmt.Errorf("读取仓库版本失败: %w", err)
	}
	if json.Unmarshal(metadata, &commit) != nil || !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(commit.SHA) {
		return Skill{}, fmt.Errorf("仓库未返回有效版本")
	}
	req, _ := http.NewRequest("GET", "https://codeload.github.com/"+repo+"/zip/"+commit.SHA, nil)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Skill{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return Skill{}, fmt.Errorf("下载技能包失败 (HTTP %d)", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (64<<20)+1))
	if err != nil {
		return Skill{}, err
	}
	if len(b) > 64<<20 {
		return Skill{}, fmt.Errorf("仓库压缩包超过 64 MB，请使用技能 ZIP 导入")
	}
	skill, err := skillPackageFromZip(b, dir)
	if err != nil {
		return skill, err
	}
	hashes := map[string]string{"SKILL.md": contentHash([]byte(strings.TrimSpace(skill.Content)))}
	for n, b := range skill.Files {
		hashes[n] = contentHash(b)
	}
	skill.Source = &SkillSource{repo, dir, ref, commit.SHA, hashes}
	return skill, nil
}
func (ss *SkillService) ImportSkillPackage(repo, dir, ref string) (Skill, error) {
	return downloadSkillPackage(strings.TrimSpace(repo), strings.TrimSpace(dir), strings.TrimSpace(ref))
}
func (ss *SkillService) ImportSkillZip(file, dir string) (Skill, error) {
	info, err := os.Stat(file)
	if err != nil {
		return Skill{}, err
	}
	if !strings.EqualFold(filepath.Ext(file), ".zip") || info.Size() > 64<<20 {
		return Skill{}, fmt.Errorf("请选择不超过 64 MB 的 ZIP 文件")
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return Skill{}, err
	}
	return skillPackageFromZip(b, dir)
}
func (ss *SkillService) ImportSkillDirectory(dir string) (Skill, error) {
	if !filepath.IsAbs(dir) || !dirExists(dir) {
		return Skill{}, fmt.Errorf("请选择存在的技能目录")
	}
	b, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return Skill{}, err
	}
	if len(b) > 8<<20 {
		return Skill{}, fmt.Errorf("SKILL.md 超过 8 MB")
	}
	meta := parseSkillFrontmatter(string(b))
	name := slugMarketName(meta.Name)
	if name == "" {
		name = slugMarketName(filepath.Base(dir))
	}
	files, executable, err := readLocalSkillAssets(dir)
	if err != nil {
		return Skill{}, err
	}
	if err = validateSkillFiles(files); err != nil {
		return Skill{}, err
	}
	return Skill{Name: name, Content: alignSkillFrontmatter(string(b), name, meta.Description), Files: files, Executable: executable, EnablePlatform: []string{platClaudeCode}, Description: meta.Description}, nil
}
func syncSkillAssets(dir string, files map[string][]byte, modes ...map[string]bool) error {
	if err := validateSkillFiles(files); err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(resolved, ".ai-env-manifest.json")
	previous := map[string]string{}
	if b, e := os.ReadFile(manifestPath); e == nil {
		_ = json.Unmarshal(b, &previous)
	}
	next := map[string]string{}
	for name, data := range files {
		target := filepath.Join(resolved, filepath.FromSlash(name))
		parent := filepath.Dir(target)
		for ancestor := parent; ancestor != resolved && pathWithin(resolved, ancestor); ancestor = filepath.Dir(ancestor) {
			if st, e := os.Lstat(ancestor); e == nil && st.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("附件父目录不能是符号链接: %s", name)
			}
		}
		if err = os.MkdirAll(parent, 0755); err != nil {
			return err
		}
		realParent, e := filepath.EvalSymlinks(parent)
		if e != nil || !pathWithin(resolved, realParent) {
			return fmt.Errorf("附件目录不能指向技能目录之外: %s", name)
		}
		if st, e := os.Lstat(target); e == nil && st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("附件不能是符号链接: %s", name)
		}
		existing, e := os.ReadFile(target)
		if e == nil && !bytes.Equal(existing, data) && (previous[name] == "" || previous[name] != contentHash(existing)) {
			next[name] = previous[name]
			continue
		}
		mode := os.FileMode(0644)
		if len(modes) > 0 && modes[0][name] {
			mode = 0755
		}
		if err = writeFileAtomic(target, data, mode); err != nil {
			return err
		}
		next[name] = contentHash(data)
	}
	for name, hash := range previous {
		if _, ok := files[name]; ok || !safeSkillRelative(name) {
			continue
		}
		target := filepath.Join(resolved, filepath.FromSlash(name))
		if b, e := os.ReadFile(target); e == nil && hash != "" && contentHash(b) == hash {
			if e = os.Remove(target); e != nil {
				return e
			}
		} else {
			next[name] = hash
		}
	}
	b, err := json.Marshal(next)
	if err != nil {
		return err
	}
	return writeFileAtomic(manifestPath, b, 0600)
}
func (ss *SkillService) PreviewSkillUpdate(name string) (SkillUpdate, error) {
	ss.mu.Lock()
	config, err := ss.loadConfig()
	ss.mu.Unlock()
	if err != nil {
		return SkillUpdate{}, err
	}
	old, ok := config[name]
	if !ok || old.Source == nil {
		return SkillUpdate{}, fmt.Errorf("该技能没有可更新的仓库来源")
	}
	fresh, err := downloadSkillPackage(old.Source.Repo, old.Source.Path, old.Source.Ref)
	if err != nil {
		return SkillUpdate{}, err
	}
	fresh.Name = name
	fresh.Content = alignSkillFrontmatter(fresh.Content, name, fresh.Description)
	fresh.Source.Hashes["SKILL.md"] = contentHash([]byte(strings.TrimSpace(fresh.Content)))
	fresh.EnablePlatform = old.EnablePlatform
	b, _ := json.Marshal(old)
	p := SkillUpdate{Name: name, Token: contentHash(b), Revision: fresh.Source.Revision, Changes: []string{}, Conflicts: []string{}, PlatformConflicts: []string{}, Details: []FileChange{}, Skill: fresh}
	before := map[string][]byte{"SKILL.md": []byte(strings.TrimSpace(old.Content))}
	after := map[string][]byte{"SKILL.md": []byte(strings.TrimSpace(fresh.Content))}
	for n, b := range old.Files {
		before[n] = b
	}
	for n, b := range fresh.Files {
		after[n] = b
	}
	names := map[string]bool{}
	for n := range before {
		names[n] = true
	}
	for n := range after {
		names[n] = true
	}
	for n := range names {
		if !bytes.Equal(before[n], after[n]) {
			p.Changes = append(p.Changes, n)
			p.Details = append(p.Details, FileChange{Path: n, Before: skillDiffContent(before[n]), After: skillDiffContent(after[n]), Changed: true})
		}
		hash, tracked := old.Source.Hashes[n]
		_, localExists := before[n]
		if (tracked && hash != contentHash(before[n])) || (!tracked && localExists) {
			p.Conflicts = append(p.Conflicts, n)
		}
	}
	home, _ := os.UserHomeDir()
	roots := map[string]string{platClaudeCode: filepath.Join(home, ".claude", "skills"), platCodex: filepath.Join(resolveCodexHome(home), "skills"), platAntigravity: antigravitySkillsScanRoot(home), platOpencode: opencodeSkillsRoot(), platGrok: grokSkillsRoot()}
	for platform, root := range roots {
		if root == "" || !platformContains(old.EnablePlatform, platform) {
			continue
		}
		for n, previous := range before {
			data, e := os.ReadFile(filepath.Join(root, name, filepath.FromSlash(n)))
			if e != nil && !os.IsNotExist(e) {
				return SkillUpdate{}, e
			}
			if e == nil && n == "SKILL.md" {
				data = bytes.TrimSpace(data)
			}
			if e == nil && !bytes.Equal(data, previous) {
				p.PlatformConflicts = append(p.PlatformConflicts, platform+"/"+n)
			}
		}
	}
	sort.Slice(p.Details, func(i, j int) bool { return p.Details[i].Path < p.Details[j].Path })
	sort.Strings(p.PlatformConflicts)
	sort.Strings(p.Changes)
	sort.Strings(p.Conflicts)
	pendingSkillUpdates.Lock()
	if len(pendingSkillUpdates.items) > 100 {
		pendingSkillUpdates.items = map[string]SkillUpdate{}
	}
	pendingSkillUpdates.items[name] = p
	pendingSkillUpdates.Unlock()
	return p, nil
}
func (ss *SkillService) ApplySkillUpdate(name, token string, keepLocal bool) error {
	pendingSkillUpdates.Lock()
	p, ok := pendingSkillUpdates.items[name]
	pendingSkillUpdates.Unlock()
	if !ok || token != p.Token {
		return fmt.Errorf("请先检查更新")
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	config, err := ss.loadConfig()
	if err != nil {
		return err
	}
	old := config[name]
	b, _ := json.Marshal(old)
	if contentHash(b) != token {
		return fmt.Errorf("技能已被修改，请重新检查更新")
	}
	if keepLocal {
		for _, n := range p.Conflicts {
			if n == "SKILL.md" {
				p.Skill.Content = old.Content
			} else if data, exists := old.Files[n]; exists {
				p.Skill.Files[n] = data
				if p.Skill.Executable == nil {
					p.Skill.Executable = map[string]bool{}
				}
				p.Skill.Executable[n] = old.Executable[n]
			} else {
				delete(p.Skill.Files, n)
			}
		}
	}
	if err = ss.saveSkillLocked(p.Skill, keepLocal); err != nil {
		return err
	}
	pendingSkillUpdates.Lock()
	delete(pendingSkillUpdates.items, name)
	pendingSkillUpdates.Unlock()
	return nil
}
func removeManagedSkillAssets(dir string) error {
	resolved, err := filepath.EvalSymlinks(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	manifest := filepath.Join(resolved, ".ai-env-manifest.json")
	data, err := os.ReadFile(manifest)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var hashes map[string]string
	if json.Unmarshal(data, &hashes) != nil {
		return fmt.Errorf("技能附件清单损坏，已停止卸载")
	}
	remaining := map[string]string{}
	for name, hash := range hashes {
		if !safeSkillRelative(name) {
			return fmt.Errorf("不安全的附件路径")
		}
		target := filepath.Join(resolved, filepath.FromSlash(name))
		real, err := filepath.EvalSymlinks(target)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !pathWithin(resolved, real) {
			return fmt.Errorf("附件路径越界")
		}
		b, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		if contentHash(b) != hash {
			remaining[name] = hash
			continue
		}
		if err = os.Remove(target); err != nil {
			return err
		}
		for parent := filepath.Dir(target); parent != resolved && pathWithin(resolved, parent); parent = filepath.Dir(parent) {
			if os.Remove(parent) != nil {
				break
			}
		}
	}
	if len(remaining) == 0 {
		return os.Remove(manifest)
	}
	b, _ := json.Marshal(remaining)
	return writeFileAtomic(manifest, b, 0600)
}

func skillDiffContent(data []byte) string {
	if !utf8.Valid(data) || bytes.ContainsRune(data, 0) {
		return fmt.Sprintf("[二进制附件 · %d bytes · SHA256 %s]", len(data), contentHash(data))
	}
	if len(data) > 200000 {
		return fmt.Sprintf("[大文件 · %d bytes · SHA256 %s]", len(data), contentHash(data))
	}
	return redactConfig(data)
}
