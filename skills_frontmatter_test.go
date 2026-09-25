package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillPackageIncludesAssetsAndProtectsLocalEdits(t *testing.T) {
	home := withHomeRoot(t)
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for path, content := range map[string]string{"repo/skills/demo/SKILL.md": "---\nname: demo\ndescription: fixture\n---\nbody", "repo/skills/demo/scripts/tool.py": "print('fixture')", "repo/skills/demo/references/guide.md": "guide"} {
		f, e := z.Create(path)
		if e != nil {
			t.Fatal(e)
		}
		f.Write([]byte(content))
	}
	z.Close()
	s, e := skillPackageFromZip(b.Bytes(), "skills/demo")
	if e != nil || len(s.Files) != 2 {
		t.Fatalf("package import: %+v %v", s, e)
	}
	service := NewSkillService()
	if e = service.SaveSkill(s); e != nil {
		t.Fatal(e)
	}
	dir := filepath.Join(home, ".claude", "skills", "demo")
	file := filepath.Join(dir, "scripts", "tool.py")
	if e = os.WriteFile(file, []byte("local edit"), 0600); e != nil {
		t.Fatal(e)
	}
	s.Files["scripts/tool.py"] = []byte("upstream edit")
	if e = service.SaveSkill(s); e != nil {
		t.Fatal(e)
	}
	got, _ := os.ReadFile(file)
	if string(got) != "local edit" {
		t.Fatal("local edit overwritten")
	}
	if e = service.DeleteSkill(s.Name); e != nil {
		t.Fatal(e)
	}
	if !fileExists(file) || fileExists(filepath.Join(dir, "references", "guide.md")) {
		t.Fatal("uninstall did not distinguish local and managed files")
	}
}
func TestSkillPackageRejectsTraversal(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	f, _ := z.Create("demo/SKILL.md")
	f.Write([]byte("---\nname: demo\ndescription: fixture\n---\nbody"))
	f, _ = z.Create("demo/../escape.txt")
	f.Write([]byte("escape"))
	z.Close()
	if _, e := skillPackageFromZip(b.Bytes(), "demo"); e == nil {
		t.Fatal("unsafe ZIP accepted")
	}
}

// metadata 等嵌套块里的 name/description 不能覆盖顶层字段
func TestParseSkillFrontmatterIgnoresNestedKeys(t *testing.T) {
	content := "---\n" +
		"name: pdf-tools\n" +
		"description: |\n" +
		"  Work with PDFs.\n" +
		"  Second line.\n" +
		"metadata:\n" +
		"  name: upstream-name\n" +
		"  description: nested\n" +
		"---\n\nbody\n"
	meta := parseSkillFrontmatter(content)
	if meta.Name != "pdf-tools" {
		t.Fatalf("name = %q，期望顶层的 pdf-tools", meta.Name)
	}
	if meta.Description != "Work with PDFs.\nSecond line." {
		t.Fatalf("description = %q", meta.Description)
	}
}

// 整体缩进的 frontmatter（合法 YAML）仍能读出字段
func TestParseSkillFrontmatterIndentedTopLevel(t *testing.T) {
	meta := parseSkillFrontmatter("---\n  name: a\n  description: b\n---\n")
	if meta.Name != "a" || meta.Description != "b" {
		t.Fatalf("name/description = %q/%q", meta.Name, meta.Description)
	}
}

// 带 BOM 的 SKILL.md 导入时应改写原 frontmatter，而不是在前面再套一层
func TestAlignSkillFrontmatterStripsBOM(t *testing.T) {
	content := "\ufeff---\nname: Original Name\ndescription: real description\n---\n\n# Title\n"
	got := alignSkillFrontmatter(content, "my-skill", "fallback")
	if strings.Count(got, "---\n") != 2 {
		t.Fatalf("出现了多层 frontmatter:\n%s", got)
	}
	meta := parseSkillFrontmatter(got)
	if meta.Name != "my-skill" || meta.Description != "real description" {
		t.Fatalf("name/description = %q/%q\n%s", meta.Name, meta.Description, got)
	}
}

// name 值为空时只替换这一行，不能把下一行的 description 吞掉
func TestAlignSkillFrontmatterEmptyNameKeepsNextLine(t *testing.T) {
	got := alignSkillFrontmatter("---\nname:\ndescription: keep me\n---\nbody", "my-skill", "")
	meta := parseSkillFrontmatter(got)
	if meta.Name != "my-skill" || meta.Description != "keep me" {
		t.Fatalf("name/description = %q/%q\n%s", meta.Name, meta.Description, got)
	}
}

func TestLocalSkillImportRejectsOversizedAsset(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "large.bin"), make([]byte, (8<<20)+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readLocalSkillAssets(dir); err == nil {
		t.Fatal("large attachment silently omitted")
	}
}
func TestSkillZipPreservesExecutableMetadata(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	f, _ := z.Create("demo/SKILL.md")
	f.Write([]byte("---\nname: demo\ndescription: fixture\n---\nbody"))
	h := &zip.FileHeader{Name: "demo/scripts/run.sh", Method: zip.Deflate}
	h.SetMode(0755)
	f, _ = z.CreateHeader(h)
	f.Write([]byte("#!/bin/sh\ntrue\n"))
	z.Close()
	skill, err := skillPackageFromZip(b.Bytes(), "demo")
	if err != nil || !skill.Executable["scripts/run.sh"] {
		t.Fatalf("executable metadata lost: %+v %v", skill.Executable, err)
	}
}
