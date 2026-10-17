// Package linttest is the golden-file harness for rule tests.
//
// A rule's test cases live under a directory named after the rule code, one
// subdirectory per case:
//
//	testdata/rules/O004/
//	    missing-model/
//	        input.xml        (required)
//	        expected.txt     (optional; default: no diagnostics)
//	    valid/
//	        input.xml
//
// Each case is parsed, the rule is selected, and the rendered diagnostics are
// compared byte-for-byte with expected.txt. Run `go test ./... -update` to
// rewrite the golden files from current output.
package linttest

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/config"
	"github.com/ppreeper/ruxx/internal/diagnostic"
	"github.com/ppreeper/ruxx/internal/index"
	"github.com/ppreeper/ruxx/internal/linter"
	"github.com/ppreeper/ruxx/internal/settings"
	"github.com/ppreeper/ruxx/internal/source"
	"github.com/ppreeper/ruxx/internal/syntax"
)

var update = flag.Bool("update", false, "update expected.txt golden files")

// RunAll discovers every rule directory under root (each named after a rule
// code) and runs each via RunRule.
func RunAll(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read testdata root %s: %v", root, err)
	}
	var ran int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ran++
		ruleDir := filepath.Join(root, e.Name())
		t.Run(e.Name(), func(t *testing.T) { RunRule(t, ruleDir) })
	}
	if ran == 0 {
		t.Fatalf("no rule directories found under %s", root)
	}
}

// RunRule discovers the case subdirectories of ruleDir and runs each one. The
// rule code is the base name of ruleDir (for example "O004").
func RunRule(t *testing.T, ruleDir string) {
	t.Helper()
	code := filepath.Base(ruleDir)
	entries, err := os.ReadDir(ruleDir)
	if err != nil {
		t.Fatalf("read rule dir %s: %v", ruleDir, err)
	}
	var ran int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ran++
		caseDir := filepath.Join(ruleDir, e.Name())
		t.Run(e.Name(), func(t *testing.T) { runCase(t, code, caseDir) })
	}
	if ran == 0 {
		t.Fatalf("no cases found under %s", ruleDir)
	}
}

func runCase(t *testing.T, code, caseDir string) {
	t.Helper()

	srcBytes, err := os.ReadFile(filepath.Join(caseDir, "input.xml"))
	if err != nil {
		t.Fatalf("read input: %v", err)
	}
	src := string(srcBytes)

	s := settings.Default()
	if _, err := os.Stat(filepath.Join(caseDir, ".ruxx.toml")); err == nil {
		cfg, err := config.Load(filepath.Join(caseDir, ".ruxx.toml"))
		if err != nil {
			t.Fatalf("load case config: %v", err)
		}
		if cfg.Settings != nil {
			s = cfg.Settings
		}
	}
	s.Select = []string{code}

	var ix *index.Index
	if _, err := os.Stat(filepath.Join(caseDir, "modules")); err == nil {
		if built, err := index.Build([]string{filepath.Join(caseDir, "modules")}); err == nil {
			ix = built
		}
	}
	path := ""
	if b, err := os.ReadFile(filepath.Join(caseDir, "path.txt")); err == nil {
		path = strings.TrimSpace(string(b))
	}

	tree := syntax.Parse(src)
	diags := linter.CheckFile(tree, path, s, ix)
	diagnostic.Sort(diags)
	got := Render(src, diags)

	expectedPath := filepath.Join(caseDir, "expected.txt")
	if *update {
		if err := os.WriteFile(expectedPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write expected: %v", err)
		}
		return
	}

	wantBytes, err := os.ReadFile(expectedPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read expected: %v", err)
	}
	want := string(wantBytes)

	if got != want {
		t.Errorf("diagnostics mismatch for %s\n--- want ---\n%s--- got ---\n%s", caseDir, want, got)
	}
}

// Render formats diagnostics as stable golden text: one line per diagnostic,
// "CODE line:col message". Locations are 1-based.
func Render(src string, diags []diagnostic.Diagnostic) string {
	loc := source.NewLocator(src)
	var b strings.Builder
	for _, d := range diags {
		line, col := loc.LineCol(d.Range.Start)
		fmt.Fprintf(&b, "%s %d:%d %s\n", d.Code, line, col, d.Message)
	}
	return b.String()
}
