package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

func TestFormatCheck_OutputFormatJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", unformatted)
	code, stdout, _ := run(t, "", "format", "--check", "--output-format", "json", dir)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d", code, cli.ExitFindings)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	if len(got) != 1 || got[0]["wouldReformat"] != true {
		t.Fatalf("unexpected json: %v", got)
	}
	if p, _ := got[0]["path"].(string); !strings.HasSuffix(p, "a.xml") {
		t.Fatalf("path = %v, want a file ending in a.xml", got[0]["path"])
	}
}

func TestFormatCheck_CleanIsEmptyJSONArray(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", formatted)
	code, stdout, _ := run(t, "", "format", "--check", "--output-format", "json", dir)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d", code, cli.ExitClean)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("stdout = %q, want []", stdout)
	}
}

func TestCheck_OutputFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xml", badO004)
	reportPath := filepath.Join(dir, "report.json")
	code, stdout, _ := run(t, "", "check", "--output-file", reportPath, "--output-format", "json", dir)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d", code, cli.ExitFindings)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("stdout = %q, want empty when --output-file is used", stdout)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("report json: %v", err)
	}
	if len(got) == 0 || got[0]["code"] != "O004" {
		t.Fatalf("unexpected report: %v", got)
	}
}
