package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

// selectionDir lays out a directory with a violating a.xml, a violating
// skip/b.xml, and an ignored notes.txt.
func selectionDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", badO004)
	writeFile(t, dir, "notes.txt", "not xml")
	if err := os.MkdirAll(filepath.Join(dir, "skip"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "skip/b.xml", badO004)
	return dir
}

func TestCheck_DefaultIncludeSkipsNonXML(t *testing.T) {
	dir := selectionDir(t)
	_, stdout, _ := run(t, "", "check", dir)
	if !strings.Contains(stdout, "a.xml") || !strings.Contains(stdout, "b.xml") {
		t.Fatalf("expected both xml files to be linted:\n%s", stdout)
	}
	if strings.Contains(stdout, "notes.txt") {
		t.Fatalf("notes.txt should not be linted:\n%s", stdout)
	}
}

func TestCheck_ExcludeFlagPrunesDirectory(t *testing.T) {
	dir := selectionDir(t)
	code, stdout, _ := run(t, "", "check", "--exclude", "skip", dir)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d\n%s", code, cli.ExitFindings, stdout)
	}
	if strings.Contains(stdout, "b.xml") {
		t.Fatalf("skip/b.xml should have been excluded:\n%s", stdout)
	}
	if !strings.Contains(stdout, "a.xml") {
		t.Fatalf("a.xml should still be linted:\n%s", stdout)
	}
}

func TestCheck_ExtendExcludeFlag(t *testing.T) {
	dir := selectionDir(t)
	_, stdout, _ := run(t, "", "check", "--extend-exclude", "**/skip/**", dir)
	if strings.Contains(stdout, "b.xml") {
		t.Fatalf("skip/b.xml should have been excluded:\n%s", stdout)
	}
}

func TestCheck_GitignoreRespected(t *testing.T) {
	dir := selectionDir(t)
	writeFile(t, dir, ".gitignore", "skip/\n")
	_, stdout, _ := run(t, "", "check", dir)
	if strings.Contains(stdout, "b.xml") {
		t.Fatalf("skip/ is gitignored:\n%s", stdout)
	}
	if !strings.Contains(stdout, "a.xml") {
		t.Fatalf("a.xml should still be linted:\n%s", stdout)
	}
}

func TestCheck_NoRespectGitignore(t *testing.T) {
	dir := selectionDir(t)
	writeFile(t, dir, ".gitignore", "skip/\n")
	_, stdout, _ := run(t, "", "check", "--no-respect-gitignore", dir)
	if !strings.Contains(stdout, "b.xml") {
		t.Fatalf("gitignore should be ignored:\n%s", stdout)
	}
}

func TestCheck_ShowFiles(t *testing.T) {
	dir := selectionDir(t)
	code, stdout, _ := run(t, "", "check", "--show-files", dir)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d", code, cli.ExitClean)
	}
	if strings.Contains(stdout, "O004") {
		t.Fatalf("--show-files should not lint:\n%s", stdout)
	}
	if !strings.Contains(stdout, "a.xml") || !strings.Contains(stdout, "b.xml") {
		t.Fatalf("--show-files should list both xml files:\n%s", stdout)
	}
	if strings.Contains(stdout, "notes.txt") {
		t.Fatalf("--show-files should list only selected files:\n%s", stdout)
	}
}

func TestCheck_ForceExcludeExplicitFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.xml", badO004)
	_, stdout, _ := run(t, "", "check", "--exclude", "a.xml", "--force-exclude", path)
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("force-exclude should drop the explicit file:\n%s", stdout)
	}
	// Without --force-exclude an explicit file is always analyzed.
	_, stdout, _ = run(t, "", "check", "--exclude", "a.xml", path)
	if !strings.Contains(stdout, "O004") {
		t.Fatalf("an explicit file should bypass exclude:\n%s", stdout)
	}
}
