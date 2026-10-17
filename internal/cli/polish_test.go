package cli_test

import (
	"os"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

func TestCheck_AddIgnore(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.xml", badO004)

	code, _, stderr := run(t, "", "check", "--add-ignore", path)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d (stderr %q), want %d", code, stderr, cli.ExitClean)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "ruxx: ignore[O004]") {
		t.Fatalf("file should carry a suppression comment:\n%s", data)
	}

	// A second run must be clean and must not add a duplicate suppression.
	if code, _, _ := run(t, "", "check", path); code != cli.ExitClean {
		t.Fatalf("second run exit = %d, want clean", code)
	}
	again, _ := os.ReadFile(path)
	if strings.Count(string(again), "ruxx: ignore") != 1 {
		t.Fatalf("suppression should be idempotent:\n%s", again)
	}
}

func TestCheck_ColorAlways(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", badO004)
	_, stdout, _ := run(t, "", "check", "--color", "always", dir)
	if !strings.Contains(stdout, "\x1b[31mO004\x1b[0m") {
		t.Fatalf("expected a red O004, got %q", stdout)
	}
}

func TestCheck_ColorNever(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", badO004)
	_, stdout, _ := run(t, "", "check", "--color", "never", dir)
	if strings.Contains(stdout, "\x1b[") {
		t.Fatalf("no escape codes expected, got %q", stdout)
	}
}

func TestCheck_ColorAutoDefaultIsPlainForNonTTY(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", badO004)
	_, stdout, _ := run(t, "", "check", dir)
	if strings.Contains(stdout, "\x1b[") {
		t.Fatalf("auto should be plain for a non-tty, got %q", stdout)
	}
}

func TestCheck_ColorForceEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "1")
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", badO004)
	_, stdout, _ := run(t, "", "check", dir)
	if !strings.Contains(stdout, "\x1b[31m") {
		t.Fatalf("FORCE_COLOR should enable colour, got %q", stdout)
	}
}

func TestCheck_NoColorEnvWins(t *testing.T) {
	t.Setenv("FORCE_COLOR", "1")
	t.Setenv("NO_COLOR", "1")
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", badO004)
	_, stdout, _ := run(t, "", "check", dir)
	if strings.Contains(stdout, "\x1b[") {
		t.Fatalf("NO_COLOR should win, got %q", stdout)
	}
}

func TestCheck_ColorNotAppliedToJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", badO004)
	_, stdout, _ := run(t, "", "check", "--color", "always", "--output-format", "json", dir)
	if strings.Contains(stdout, "\x1b[") {
		t.Fatalf("json output must never be coloured, got %q", stdout)
	}
}
