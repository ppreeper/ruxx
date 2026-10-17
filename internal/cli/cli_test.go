package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

const (
	badO004 = `<?xml version="1.0" encoding="utf-8"?>
<odoo>
    <record id="foo">
        <field name="name">Foo</field>
    </record>
</odoo>
`
	cleanO004 = `<?xml version="1.0" encoding="utf-8"?>
<odoo>
    <record id="foo" model="ir.ui.view">
        <field name="name">Foo</field>
    </record>
</odoo>
`
	unformatted = `<?xml version="1.0" encoding="utf-8"?>
<odoo>
    <field name="name"></field>
</odoo>
`
	formatted = `<?xml version="1.0" encoding="utf-8"?>
<odoo>
    <field name="name"/>
</odoo>
`
)

// run invokes the CLI in-process and captures its streams.
func run(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = cli.Run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

// writeFile creates a file in dir and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestCheck_Clean(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "clean.xml", cleanO004)

	code, out, _ := run(t, "", "check", path)
	if code != cli.ExitClean {
		t.Errorf("exit = %d, want %d\nstdout:\n%s", code, cli.ExitClean, out)
	}
	if strings.Contains(out, "O004") {
		t.Errorf("clean file reported a violation:\n%s", out)
	}
}

func TestCheck_Violation(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "bad.xml", badO004)

	code, out, _ := run(t, "", "check", path)
	if code != cli.ExitFindings {
		t.Errorf("exit = %d, want %d", code, cli.ExitFindings)
	}
	if !strings.Contains(out, "O004") {
		t.Errorf("stdout missing O004 diagnostic:\n%s", out)
	}
	if !strings.Contains(out, "3:5") {
		t.Errorf("stdout missing 3:5 location:\n%s", out)
	}
}

func TestCheck_ExitZero(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "bad.xml", badO004)

	code, out, _ := run(t, "", "check", "--exit-zero", path)
	if code != cli.ExitClean {
		t.Errorf("exit = %d, want %d with --exit-zero\nstdout:\n%s", code, cli.ExitClean, out)
	}
	if !strings.Contains(out, "O004") {
		t.Errorf("--exit-zero should still print the diagnostic:\n%s", out)
	}
}

func TestCheck_MissingFile(t *testing.T) {
	dir := t.TempDir()
	code, _, _ := run(t, "", "check", filepath.Join(dir, "nope.xml"))
	if code != cli.ExitAbnormal {
		t.Errorf("exit = %d, want %d for a missing file", code, cli.ExitAbnormal)
	}
}

func TestCheck_UnknownFlag(t *testing.T) {
	code, _, _ := run(t, "", "check", "--definitely-not-a-flag")
	if code != cli.ExitAbnormal {
		t.Errorf("exit = %d, want %d for an unknown flag", code, cli.ExitAbnormal)
	}
}

func TestCheck_Stdin(t *testing.T) {
	code, out, _ := run(t, badO004, "check", "-", "--stdin-filename", "views/foo.xml")
	if code != cli.ExitFindings {
		t.Errorf("exit = %d, want %d", code, cli.ExitFindings)
	}
	if !strings.Contains(out, "O004") {
		t.Errorf("stdout missing O004 for stdin input:\n%s", out)
	}
}

func TestFormat_Check_Unformatted(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "unformatted.xml", unformatted)

	code, _, _ := run(t, "", "format", "--check", path)
	if code != cli.ExitFindings {
		t.Errorf("exit = %d, want %d for a file that needs formatting", code, cli.ExitFindings)
	}
}

func TestFormat_Check_Formatted(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "formatted.xml", formatted)

	code, out, _ := run(t, "", "format", "--check", path)
	if code != cli.ExitClean {
		t.Errorf("exit = %d, want %d for an already-formatted file\nstdout:\n%s", code, cli.ExitClean, out)
	}
}
