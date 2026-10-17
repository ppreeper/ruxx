package fix_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/ppreeper/ruxx/internal/diagnostic"
	"github.com/ppreeper/ruxx/internal/fix"
	"github.com/ppreeper/ruxx/internal/settings"
	"github.com/ppreeper/ruxx/internal/source"
)

var update = flag.Bool("update", false, "update expected.xml golden files")

// --- ApplyEdits: pure mechanics ---------------------------------------------

func TestApplyEdits_NoEdits(t *testing.T) {
	got, err := fix.ApplyEdits("hello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestApplyEdits_ReplaceInsertDelete(t *testing.T) {
	src := "abcdef"
	edits := []diagnostic.Edit{
		{Range: source.NewRange(0, 1), NewText: "X"}, // replace a -> X
		{Range: source.NewRange(3, 3), NewText: "-"}, // insert at 3
		{Range: source.NewRange(5, 6), NewText: ""},  // delete f
	}
	got, err := fix.ApplyEdits(src, edits)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "Xbc-de"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyEdits_Unordered(t *testing.T) {
	src := "abcdef"
	// Provide edits out of order; ApplyEdits must still produce correct output.
	edits := []diagnostic.Edit{
		{Range: source.NewRange(4, 5), NewText: "E"},
		{Range: source.NewRange(0, 1), NewText: "A"},
	}
	got, err := fix.ApplyEdits(src, edits)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "AbcdEf"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyEdits_OverlapErrors(t *testing.T) {
	src := "abcdef"
	edits := []diagnostic.Edit{
		{Range: source.NewRange(0, 3), NewText: "X"},
		{Range: source.NewRange(2, 4), NewText: "Y"},
	}
	if _, err := fix.ApplyEdits(src, edits); err == nil {
		t.Fatal("expected an overlap error, got nil")
	}
}

// --- Apply: golden loop ------------------------------------------------------

// testdata layout: testdata/<CODE>/<case>/{input.xml,expected.xml}. The rule
// code is the parent directory name; the harness selects just that rule so a
// case isolates one fix.
func TestApplyGoldens(t *testing.T) {
	root := "testdata"
	ruleDirs, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	for _, rd := range ruleDirs {
		if !rd.IsDir() {
			continue
		}
		code := rd.Name()
		ruleDir := filepath.Join(root, code)
		cases, err := os.ReadDir(ruleDir)
		if err != nil {
			t.Fatalf("read %s: %v", ruleDir, err)
		}
		for _, c := range cases {
			if !c.IsDir() {
				continue
			}
			caseDir := filepath.Join(ruleDir, c.Name())
			t.Run(code+"/"+c.Name(), func(t *testing.T) { runFixCase(t, code, caseDir) })
		}
	}
}

func runFixCase(t *testing.T, code, caseDir string) {
	t.Helper()

	input := mustRead(t, filepath.Join(caseDir, "input.xml"))

	s := settings.Default()
	s.Select = []string{code}

	res := fix.Apply(input, s, fix.Options{})

	expectedPath := filepath.Join(caseDir, "expected.xml")
	if *update {
		if err := os.WriteFile(expectedPath, []byte(res.Source), 0o644); err != nil {
			t.Fatalf("write expected: %v", err)
		}
		return
	}

	want := mustRead(t, expectedPath)
	if res.Source != want {
		t.Errorf("fixed output mismatch for %s\n--- want ---\n%s\n--- got ---\n%s", caseDir, want, res.Source)
	}
	if res.Reverted {
		t.Errorf("unexpected revert for %s", caseDir)
	}

	// Idempotency: fixing already-fixed output must be a no-op.
	again := fix.Apply(res.Source, s, fix.Options{})
	if again.Source != res.Source {
		t.Errorf("fix not idempotent for %s\n--- once ---\n%s\n--- twice ---\n%s", caseDir, res.Source, again.Source)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
