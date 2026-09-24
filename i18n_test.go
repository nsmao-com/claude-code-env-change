package main

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

var fmtVerbPattern = regexp.MustCompile(`%(?:\[\d+\])?[-+# 0]*(?:\d+|\*)?(?:\.(?:\d+|\*))?[a-zA-Z%]`)

func fmtVerbs(s string) []string {
	var out []string
	for _, v := range fmtVerbPattern.FindAllString(s, -1) {
		if v != "%%" {
			out = append(out, v)
		}
	}
	return out
}

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

type i18nUse struct {
	pos    string
	fn     string
	format string
	args   int // 格式串之后的参数个数；-1 表示 args... 展开，无法计数
}

// collectI18nUses 扫描本包全部非测试源码（含各平台文件）
func collectI18nUses(t *testing.T) (uses []i18nUse, direct []string) {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			name := ""
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				name = fn.Name
			case *ast.SelectorExpr:
				if x, ok := fn.X.(*ast.Ident); ok {
					name = x.Name + "." + fn.Sel.Name
				}
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil || !hasHan(value) {
				return true
			}
			pos := fset.Position(lit.Pos()).String()
			switch name {
			case "tr", "trError":
				uses = append(uses, i18nUse{pos: pos, fn: name, format: value, args: -1})
			case "errorf", "sprintf":
				args := len(call.Args) - 1
				if call.Ellipsis.IsValid() {
					args = -1
				}
				uses = append(uses, i18nUse{pos: pos, fn: name, format: value, args: args})
			case "fmt.Errorf", "fmt.Sprintf", "errors.New":
				direct = append(direct, pos+" "+name+"("+value+")")
			}
			return true
		})
	}
	return uses, direct
}

// 每一处面向用户的中文都有英文，占位符一致、参数个数对得上
func TestBackendMessagesTranslated(t *testing.T) {
	uses, direct := collectI18nUses(t)
	if len(uses) < 400 {
		t.Fatalf("只扫到 %d 处，扫描可能出错了", len(uses))
	}
	for _, d := range direct {
		t.Errorf("中文格式串请改用 errorf / sprintf / trError 以便翻译: %s", d)
	}

	missing := map[string]bool{}
	for _, u := range uses {
		zhVerbs := fmtVerbs(u.format)
		if u.args >= 0 && len(zhVerbs) != u.args {
			t.Errorf("%s: %s 的占位符 %d 个，参数 %d 个: %q", u.pos, u.fn, len(zhVerbs), u.args, u.format)
		}
		en, ok := enMessages[u.format]
		if !ok {
			missing[u.format] = true
			continue
		}
		if strings.Join(fmtVerbs(en), " ") != strings.Join(zhVerbs, " ") {
			t.Errorf("%s: 英文占位符与中文不一致:\n  zh %q\n  en %q", u.pos, u.format, en)
		}
		if hasHan(en) {
			t.Errorf("%s: 英文对照里还有中文: %q", u.pos, en)
		}
	}
	// 包级表：用到时才经 tr 翻译
	for _, meta := range officialLoginCatalog {
		for _, s := range []string{meta.name, meta.desc} {
			if _, ok := enMessages[s]; !ok {
				missing[s] = true
			}
		}
	}
	for _, p := range providerPresets {
		for _, s := range []string{p.Name, p.Description} {
			if _, ok := enMessages[s]; !ok && hasHan(s) {
				missing[s] = true
			}
		}
	}
	if len(missing) > 0 {
		keys := make([]string, 0, len(missing))
		for k := range missing {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if dump := os.Getenv("I18N_DUMP"); dump != "" {
			data, _ := json.MarshalIndent(keys, "", "  ")
			_ = os.WriteFile(dump, data, 0o644)
		}
		t.Errorf("%d 条中文没有英文对照，例如 %q", len(keys), keys[0])
	}
}

// 对照表里不应留着源码已经不用的旧条目
func TestBackendMessagesNoStaleEntries(t *testing.T) {
	uses, _ := collectI18nUses(t)
	used := map[string]bool{}
	for _, u := range uses {
		used[u.format] = true
	}
	for _, meta := range officialLoginCatalog {
		used[meta.name], used[meta.desc] = true, true
	}
	for _, p := range providerPresets {
		used[p.Name], used[p.Description] = true, true
	}
	for _, extra := range enMessagesNoHan {
		used[extra] = true
	}
	for key := range enMessages {
		if !used[key] {
			t.Errorf("对照表条目已无人使用: %q", key)
		}
	}
}

func TestBackendLanguageSwitch(t *testing.T) {
	withHomeRoot(t)
	defer setBackendLanguage("zh")
	app := &App{}

	err := errorf("找不到环境配置 %q", "x")
	if !strings.Contains(err.Error(), "找不到") {
		t.Fatalf("默认应为中文: %v", err)
	}
	if err := app.SetLanguage("en-US"); err != nil {
		t.Fatal(err)
	}
	if app.GetLanguage() != "en" {
		t.Fatalf("en-US 应归一为 en")
	}
	err = errorf("找不到环境配置 %q", "x")
	if hasHan(err.Error()) || !strings.Contains(err.Error(), `"x"`) {
		t.Fatalf("切到英文后应输出英文且保留参数: %v", err)
	}
	// %w 包装在翻译后仍可解开
	wrapped := errorf("写入 %s 失败: %w", "a.json", os.ErrPermission)
	if !errors.Is(wrapped, os.ErrPermission) {
		t.Fatalf("翻译后 %%w 仍应可 errors.Is: %v", wrapped)
	}
	// 包级错误变量按调用时的语言输出，errors.Is 不受影响
	if hasHan(errOSSNotFound.Error()) {
		t.Fatalf("包级错误应随语言输出英文: %v", errOSSNotFound)
	}
	if !errors.Is(errorf("下载失败: %w", errOSSNotFound), errOSSNotFound) {
		t.Fatalf("errors.Is 应能识别包级错误")
	}

	// 语言落盘，下次启动沿用
	setBackendLanguage("zh")
	loadBackendLanguage()
	if backendLanguage() != "en" {
		t.Fatalf("启动时应读回上次的语言")
	}
}
