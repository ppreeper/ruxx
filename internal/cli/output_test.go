package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

func TestCheck_OutputFormatJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", badO004)
	code, stdout, _ := run(t, "", "check", "--output-format", "json", dir)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d", code, cli.ExitFindings)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	if len(got) != 1 || got[0]["code"] != "O004" || got[0]["severity"] != "error" {
		t.Fatalf("unexpected json: %v", got)
	}
	if got[0]["path"] == "" {
		t.Fatal("json should carry the file path")
	}
}

func TestCheck_OutputFormatGitHub(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", badO004)
	code, stdout, _ := run(t, "", "check", "--output-format", "github", dir)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d", code, cli.ExitFindings)
	}
	if !strings.HasPrefix(stdout, "::error file=") || !strings.Contains(stdout, "::O004 ") {
		t.Fatalf("github output = %q", stdout)
	}
}

func TestCheck_OutputFormatSARIF(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", badO004)
	code, stdout, _ := run(t, "", "check", "--output-format", "sarif", dir)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d", code, cli.ExitFindings)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("sarif: %v", err)
	}
	if got["version"] != "2.1.0" {
		t.Fatalf("sarif version = %v", got["version"])
	}
}

func TestCheck_OutputFormatCleanIsEmpty(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", cleanO004)
	if code, stdout, _ := run(t, "", "check", "--output-format", "json", dir); code != cli.ExitClean || strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("exit=%d stdout=%q, want 0 and []", code, stdout)
	}
	if code, stdout, _ := run(t, "", "check", dir); code != cli.ExitClean || stdout != "" {
		t.Fatalf("exit=%d stdout=%q, want 0 and empty", code, stdout)
	}
}

func TestCheck_UnknownOutputFormat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", badO004)
	code, _, stderr := run(t, "", "check", "--output-format", "nope", dir)
	if code != cli.ExitAbnormal {
		t.Fatalf("exit = %d, want %d", code, cli.ExitAbnormal)
	}
	if !strings.Contains(stderr, "nope") {
		t.Fatalf("stderr = %q, want the bad format named", stderr)
	}
}
