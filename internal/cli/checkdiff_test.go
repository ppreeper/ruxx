package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

// TestCheck_DiffPrintsPatchAndDoesNotWrite checks `ruxx check --diff` applies
// fixes in memory, prints a unified diff, and leaves the file untouched.
//
// O019 is selected explicitly: the default correctness set has no safe-fixable
// rule that fires here (O010 is version-gated, X014's rename is unsafe).
func TestCheck_DiffPrintsPatchAndDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	src := "<?xml version=\"1.0\"?>\n<odoo>\n    <record model=\"ir.ui.view\" id=\"a\"/>\n</odoo>\n"
	path := filepath.Join(dir, "f.xml")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := run(t, "", "check", "--diff", "--select", "O019", path)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, cli.ExitClean, stderr)
	}
	if !strings.HasPrefix(stdout, "--- "+path+"\n+++ "+path+"\n") {
		t.Fatalf("missing file headers:\n%s", stdout)
	}
	want := "@@ -1,4 +1,4 @@\n <?xml version=\"1.0\"?>\n <odoo>\n-    <record model=\"ir.ui.view\" id=\"a\"/>\n+    <record id=\"a\" model=\"ir.ui.view\"/>\n </odoo>\n"
	if !strings.Contains(stdout, want) {
		t.Errorf("diff =\n%s\nwant to contain\n%s", stdout, want)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != src {
		t.Errorf("--diff modified the file (err %v)", err)
	}
}

// TestCheck_DiffOnCleanFileIsSilent checks there is no diff when nothing is
// fixable.
func TestCheck_DiffOnCleanFileIsSilent(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "f.xml", "<?xml version=\"1.0\"?>\n<odoo>\n    <record id=\"a\" model=\"ir.ui.view\"/>\n</odoo>\n")
	code, stdout, stderr := run(t, "", "check", "--diff", "--select", "O019", path)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want clean (stderr: %s)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}
