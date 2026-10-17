package cli_test

import (
	"os"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

// rangeDoc has two badly spaced records; a range selection must only rewrite
// the one it covers.
const rangeDoc = `<?xml version="1.0"?>
<odoo>
    <record id="a" model="ir.ui.view"   >
        <field name="name">A</field>
    </record>
    <record id="b" model="ir.ui.view"   >
        <field name="name">B</field>
    </record>
</odoo>
`

// TestFormat_RangeOnlyTouchesSelection checks that --range rewrites the element
// enclosing the selection and leaves every other byte alone.
func TestFormat_RangeOnlyTouchesSelection(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.xml", rangeDoc)
	code, _, stderr := run(t, "", "format", "--range", "3:5-3:30", path)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d (stderr %q), want %d", code, stderr, cli.ExitClean)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `<?xml version="1.0"?>
<odoo>
    <record id="a" model="ir.ui.view">
        <field name="name">A</field>
    </record>
    <record id="b" model="ir.ui.view"   >
        <field name="name">B</field>
    </record>
</odoo>
`
	if string(got) != want {
		t.Errorf("formatted file =\n%s\nwant\n%s", got, want)
	}
}

// TestFormat_RangeAlreadyCanonicalIsNoOp checks an already-formatted selection
// reports nothing to do.
func TestFormat_RangeAlreadyCanonicalIsNoOp(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.xml", rangeDoc)
	code, stdout, stderr := run(t, "", "format", "--range", "4:9-4:37", "--check", path)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d (stderr %q), want %d", code, stderr, cli.ExitClean)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty (selection is already formatted)", stdout)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != rangeDoc {
		t.Errorf("--check modified the file")
	}
}

// TestFormat_RangeCheckReportsChanges checks --range --check reports the file
// that would change.
func TestFormat_RangeCheckReportsChanges(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.xml", rangeDoc)
	code, stdout, stderr := run(t, "", "format", "--range", "3:5-3:30", "--check", path)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d (stderr %q), want %d", code, stderr, cli.ExitFindings)
	}
	if !strings.Contains(stdout, "would reformat "+path) {
		t.Errorf("stdout = %q, want it to name %s", stdout, path)
	}
}

// TestFormat_RangeInvalid checks a malformed selection is a usage error.
func TestFormat_RangeInvalid(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.xml", rangeDoc)
	for _, spec := range []string{"nonsense", "3:5", "9:1-3:2", "3:x-4:y"} {
		t.Run(spec, func(t *testing.T) {
			code, _, stderr := run(t, "", "format", "--range", spec, path)
			if code != cli.ExitAbnormal {
				t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitAbnormal, stderr)
			}
			if !strings.Contains(stderr, "--range") {
				t.Errorf("stderr = %q, want it to mention --range", stderr)
			}
		})
	}
}

// TestCheck_RangeRejected checks --range is only meaningful for format.
func TestCheck_RangeRejected(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.xml", rangeDoc)
	code, _, stderr := run(t, "", "check", "--range", "3:5-3:30", path)
	if code != cli.ExitAbnormal {
		t.Fatalf("exit = %d, want %d", code, cli.ExitAbnormal)
	}
	if !strings.Contains(stderr, "only supported by ruxx format") {
		t.Errorf("stderr = %q, want it to reject --range", stderr)
	}
}

// TestFormat_RangeKeepsFileLineEndings checks a range edit cannot introduce a
// line ending the file did not use.
func TestFormat_RangeKeepsFileLineEndings(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.xml", strings.ReplaceAll(rangeDoc, "\n", "\r\n"))
	code, _, stderr := run(t, "", "format", "--range", "3:5-3:30", path)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d (stderr %q), want %d", code, stderr, cli.ExitClean)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ReplaceAll(string(got), "\r\n", ""), "\n") {
		t.Errorf("range edit introduced a bare LF into a CRLF file:\n%q", got)
	}
	if !strings.Contains(string(got), "model=\"ir.ui.view\">\r\n") {
		t.Errorf("selection not formatted:\n%q", got)
	}
}

// TestFormat_RangeStdin checks the selection applies to stdin too.
func TestFormat_RangeStdin(t *testing.T) {
	code, stdout, stderr := run(t, rangeDoc, "format", "--range", "3:5-3:30", "-")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d (stderr %q), want %d", code, stderr, cli.ExitClean)
	}
	if !strings.Contains(stdout, `model="ir.ui.view">`) {
		t.Errorf("stdout = %q, want the first record normalised", stdout)
	}
	if !strings.Contains(stdout, `model="ir.ui.view"   >`) {
		t.Errorf("stdout = %q, want the second record untouched", stdout)
	}
}
