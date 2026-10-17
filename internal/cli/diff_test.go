package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

// TestFormat_DiffIsUnified checks `ruxx format --diff` prints a real unified
// diff: only the changed line and its context, not both whole files.
func TestFormat_DiffIsUnified(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\"?>\n<odoo>\n")
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&b, "    <field name=\"a%02d\"/>\n", i)
	}
	b.WriteString("    <field name=\"x\" />\n")
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&b, "    <field name=\"b%02d\"/>\n", i)
	}
	b.WriteString("</odoo>\n")
	src := b.String()
	path := filepath.Join(dir, "f.xml")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := run(t, "", "format", "--diff", path)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, cli.ExitClean, stderr)
	}
	if !strings.HasPrefix(stdout, "--- "+path+"\n+++ "+path+"\n") {
		t.Fatalf("missing file headers:\n%.200s", stdout)
	}
	if !strings.Contains(stdout, "@@ -10,7 +10,7 @@\n") {
		t.Errorf("missing hunk header:\n%s", stdout)
	}
	if !strings.Contains(stdout, "-    <field name=\"x\" />\n+    <field name=\"x\"/>\n") {
		t.Errorf("change not described:\n%s", stdout)
	}
	// Only three lines of context on each side, and nothing further out.
	if !strings.Contains(stdout, " name=\"a09\"/>\n") || !strings.Contains(stdout, " name=\"b00\"/>\n") {
		t.Errorf("context lines missing:\n%s", stdout)
	}
	if strings.Contains(stdout, "name=\"a00\"") || strings.Contains(stdout, "name=\"b09\"") {
		t.Errorf("elided lines leaked into the diff:\n%s", stdout)
	}
	if n := strings.Count(stdout, "\n"); n > 12 {
		t.Errorf("diff has %d lines, want a small hunk:\n%s", n, stdout)
	}
	// --diff must not rewrite the file.
	if got, err := os.ReadFile(path); err != nil || string(got) != src {
		t.Errorf("--diff modified the file (err %v)", err)
	}
}

// TestFormat_DiffNoChangeIsSilent checks a formatted file produces no output.
func TestFormat_DiffNoChangeIsSilent(t *testing.T) {
	dir := t.TempDir()
	const formattedDoc = "<?xml version=\"1.0\"?>\n<odoo>\n    <field name=\"name\"/>\n</odoo>\n"
	path := writeFile(t, dir, "f.xml", formattedDoc)
	code, stdout, stderr := run(t, "", "format", "--diff", path)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want clean (stderr: %s)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}
