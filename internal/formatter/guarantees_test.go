package formatter_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/diagnostic"
	"github.com/ppreeper/ruxx/internal/fix"
	"github.com/ppreeper/ruxx/internal/formatter"
	"github.com/ppreeper/ruxx/internal/linter"
	"github.com/ppreeper/ruxx/internal/rules"
	"github.com/ppreeper/ruxx/internal/settings"
	"github.com/ppreeper/ruxx/internal/syntax"
)

func readAll(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// caseDirs returns every directory under root that holds an input.xml.
func caseDirs(t *testing.T, root string) []string {
	t.Helper()
	var dirs []string
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "input.xml")); err == nil {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// signature reduces a tree to the content that formatting must preserve:
// element names, their attributes (order-insensitive), non-blank character
// data, comments, and CDATA.
func signature(src string) string {
	tree := syntax.Parse(src)
	var b strings.Builder
	sl := func(n *syntax.Node) string {
		start, end := n.Range.Start, n.Range.End
		if start < 0 {
			start = 0
		}
		if end > len(src) {
			end = len(src)
		}
		if start > end {
			return ""
		}
		return src[start:end]
	}
	var walk func(n *syntax.Node)
	walk = func(n *syntax.Node) {
		switch n.Kind {
		case syntax.KindElement:
			var attrs []string
			for _, a := range n.Attrs {
				attrs = append(attrs, a.AttrName+"="+a.AttrValue)
			}
			sort.Strings(attrs)
			b.WriteString("<" + n.Name)
			for _, a := range attrs {
				b.WriteString(" " + a)
			}
			b.WriteString(">")
			for _, c := range n.Children {
				walk(c)
			}
			b.WriteString("</" + n.Name + ">")
		case syntax.KindText:
			if txt := strings.TrimSpace(sl(n)); txt != "" {
				b.WriteString("T:" + txt + ";")
			}
		case syntax.KindComment:
			b.WriteString("C:" + strings.TrimSpace(sl(n)) + ";")
		case syntax.KindCDATA:
			b.WriteString("D:" + strings.TrimSpace(sl(n)) + ";")
		}
	}
	if tree.Root != nil {
		for _, c := range tree.Root.Children {
			walk(c)
		}
	}
	return b.String()
}

// TestFormat_PreservesContent is the "reorganizes layout only" guarantee:
// element names, attributes, text, comments, and CDATA must all survive.
func TestFormat_PreservesContent(t *testing.T) {
	for _, dir := range caseDirs(t, "testdata/format") {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			in := readAll(t, filepath.Join(dir, "input.xml"))
			out, err := formatter.Format(in, settings.Default())
			if err != nil {
				t.Fatalf("format: %v", err)
			}
			if got, want := signature(out), signature(in); got != want {
				t.Fatalf("formatting changed content:\n got %s\nwant %s", got, want)
			}
		})
	}
}

// TestFormat_IntroducesNoNewLintCodes checks that formatting never turns a clean
// file dirty: every code present after formatting was already present before.
//
// Style-category rules are exempt: the formatter deliberately owns those
// concerns (that overlap is exactly what RUX103 reports when they are enabled),
// so e.g. the split-attribute self-close style may trip X016/ST006. No rule from
// any other category may appear.
func TestFormat_IntroducesNoNewLintCodes(t *testing.T) {
	styleCodes := map[string]bool{}
	for _, r := range rules.All() {
		if r.Category == rules.Style {
			styleCodes[r.Code] = true
		}
	}
	s := settings.Default()
	s.Select = []string{"ALL"}
	for _, dir := range caseDirs(t, "testdata/format") {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			in := readAll(t, filepath.Join(dir, "input.xml"))
			out, err := formatter.Format(in, s)
			if err != nil {
				t.Fatalf("format: %v", err)
			}
			before := codeSet(linter.Check(syntax.Parse(in), s))
			for code := range codeSet(linter.Check(syntax.Parse(out), s)) {
				if before[code] || styleCodes[code] {
					continue
				}
				t.Errorf("formatting introduced %s", code)
			}
		})
	}
}

// codeSet collects the set of rule codes in a diagnostic list.
func codeSet(diags []diagnostic.Diagnostic) map[string]bool {
	set := make(map[string]bool, len(diags))
	for _, d := range diags {
		set[d.Code] = true
	}
	return set
}

// TestFormat_FixInterop runs every fix fixture through fix + format and checks
// the fix sticks: the rule that fired must be gone afterwards, and formatting
// the fixed output must be idempotent.
func TestFormat_FixInterop(t *testing.T) {
	root := filepath.Join("..", "fix", "testdata")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	for _, ruleEntry := range entries {
		if !ruleEntry.IsDir() {
			continue
		}
		code := ruleEntry.Name()
		for _, dir := range caseDirs(t, filepath.Join(root, code)) {
			t.Run(code+"/"+filepath.Base(dir), func(t *testing.T) {
				in := readAll(t, filepath.Join(dir, "input.xml"))
				s := settings.Default()
				s.Select = []string{code}
				fixed := fix.Apply(in, s, fix.Options{Unsafe: true}).Source

				out, err := formatter.Format(fixed, settings.Default())
				if err != nil {
					t.Fatalf("format: %v", err)
				}
				if again, _ := formatter.Format(out, settings.Default()); again != out {
					t.Fatalf("not idempotent:\n first %q\nsecond %q", out, again)
				}
				for _, d := range linter.Check(syntax.Parse(out), s) {
					if d.Code == code {
						t.Fatalf("%s still fires after fix+format:\n%s", code, out)
					}
				}
			})
		}
	}
}
