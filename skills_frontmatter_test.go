package main

import (
	"strings"
	"testing"
)

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
