package cli_test

import (
	"strings"
	"testing"
)

// Every subcommand answers -h/--help with a usage screen and exit 0, before
// any work happens.

func TestCheck_Help(t *testing.T) {
	code, stdout, stderr := run(t, "", "check", "--help")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", code, stderr)
	}
	for _, want := range []string{
		"usage: ruxx check",
		"--fix",
		"--unsafe-fixes",
		"--select",
		"--show-settings",
		"--fixable",
		"Exit codes:",
		"Selectors:",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("check --help output missing %q", want)
		}
	}
	if stderr != "" {
		t.Errorf("check --help wrote to stderr: %q", stderr)
	}
}

func TestCheck_HelpShortFlag(t *testing.T) {
	code, stdout, _ := run(t, "", "check", "-h")
	if code != 0 || !strings.Contains(stdout, "usage: ruxx check") {
		t.Fatalf("check -h: exit = %d, stdout = %q", code, stdout)
	}
}

func TestFormat_Help(t *testing.T) {
	code, stdout, _ := run(t, "", "format", "--help")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{
		"usage: ruxx format",
		"--check",
		"--diff",
		"--range",
		"--exit-non-zero-on-format",
		"Exit codes:",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("format --help output missing %q", want)
		}
	}
}

func TestClean_Help(t *testing.T) {
	code, stdout, _ := run(t, "", "clean", "--help")
	if code != 0 || !strings.Contains(stdout, "usage: ruxx clean") {
		t.Fatalf("clean --help: exit = %d, stdout = %q", code, stdout)
	}
	if !strings.Contains(stdout, "--cache-dir") {
		t.Errorf("clean --help missing --cache-dir")
	}
}

func TestServer_Help(t *testing.T) {
	code, stdout, _ := run(t, "", "server", "--help")
	if code != 0 || !strings.Contains(stdout, "usage: ruxx server") {
		t.Fatalf("server --help: exit = %d, stdout = %q", code, stdout)
	}
	if !strings.Contains(stdout, "LSP") {
		t.Errorf("server --help should mention LSP")
	}
}

// --help wins over the work: no linting, no diagnostics, exit 0.
func TestHelp_DoesNotLint(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", badO004)
	code, stdout, _ := run(t, "", "check", "--help", dir)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.Contains(stdout, "a.xml:") {
		t.Errorf("--help should not lint, got %q", stdout)
	}
}

// An unknown flag is still an error, not a help request.
func TestHelp_UnknownFlagStaysAbnormal(t *testing.T) {
	code, _, stderr := run(t, "", "check", "--bogus")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "--bogus") {
		t.Errorf("stderr should name the flag: %q", stderr)
	}
}
