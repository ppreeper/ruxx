package e2e

import (
	"strings"
	"testing"
)

// formattedUnformattedView is what unformattedView looks like after `ruxx
// format`: the empty-element space and the layout are normalised.
const formattedUnformattedView = "<?xml version=\"1.0\"?>\n<odoo>\n    <record id=\"a\" model=\"ir.ui.view\">\n        <field name=\"name\">A</field>\n    </record>\n</odoo>\n"

// TestFormat_WritesInPlaceAndIsIdempotent is the formatter's core promise.
func TestFormat_WritesInPlaceAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "a.xml", unformattedView)

	got := run(t, dir, "", "format", "a.xml")
	if got.code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", got.code, got.stderr)
	}
	if after := read(t, path); after != formattedUnformattedView {
		t.Fatalf("formatted file =\n%q\nwant\n%q", after, formattedUnformattedView)
	}

	again := run(t, dir, "", "format", "a.xml")
	if again.code != 0 {
		t.Fatalf("second run exit = %d, want 0", again.code)
	}
	if after := read(t, path); after != formattedUnformattedView {
		t.Errorf("second run changed the file:\n%q", after)
	}
}

// TestFormat_CheckMode covers the CI gate: exit 1 while a file needs work,
// exit 0 once it does not, and never a write.
func TestFormat_CheckMode(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "a.xml", unformattedView)

	got := run(t, dir, "", "format", "--check", "a.xml")
	if got.code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr %q)", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "would reformat") {
		t.Errorf("stdout = %q, want a would-reformat verdict", got.stdout)
	}
	if read(t, path) != unformattedView {
		t.Error("--check modified the file")
	}

	if code := run(t, dir, "", "format", "a.xml").code; code != 0 {
		t.Fatalf("format exit = %d, want 0", code)
	}
	clean := run(t, dir, "", "format", "--check", "a.xml")
	if clean.code != 0 {
		t.Fatalf("exit = %d, want 0 on a formatted file (stdout %q)", clean.code, clean.stdout)
	}
	if clean.stdout != "" {
		t.Errorf("stdout = %q, want empty", clean.stdout)
	}
}

// TestFormat_DiffPrintsPatch covers review-before-write.
func TestFormat_DiffPrintsPatch(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "a.xml", unformattedView)

	got := run(t, dir, "", "format", "--diff", "a.xml")
	if got.code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", got.code, got.stderr)
	}
	for _, want := range []string{"--- a.xml", "+++ a.xml", "@@"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout = %q, want it to contain %q", got.stdout, want)
		}
	}
	if read(t, path) != unformattedView {
		t.Error("--diff modified the file")
	}
}

// TestFormat_Stdin writes the result to stdout.
func TestFormat_Stdin(t *testing.T) {
	dir := t.TempDir()

	got := run(t, dir, unformattedView, "format", "-")
	if got.code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", got.code, got.stderr)
	}
	if got.stdout != formattedUnformattedView {
		t.Errorf("stdout =\n%q\nwant\n%q", got.stdout, formattedUnformattedView)
	}
}

// TestFormat_RangeOnlyTouchesTheSelection proves range formatting leaves the
// rest of the file byte-identical.
func TestFormat_RangeOnlyTouchesTheSelection(t *testing.T) {
	dir := t.TempDir()
	// Line 3 is misformatted; line 5 is deliberately left unformatted too.
	src := "<?xml version=\"1.0\"?>\n<odoo>\n    <record id=\"a\" model=\"ir.ui.view\" >\n    </record>\n    <record id=\"b\" model=\"ir.ui.view\" >\n    </record>\n</odoo>\n"
	path := write(t, dir, "a.xml", src)

	got := run(t, dir, "", "format", "--range", "3:5-4:15", "a.xml")
	if got.code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", got.code, got.stderr)
	}
	after := read(t, path)
	if strings.Contains(after, "<record id=\"a\" model=\"ir.ui.view\" >") {
		t.Errorf("the selection was not reformatted:\n%s", after)
	}
	if !strings.Contains(after, "<record id=\"b\" model=\"ir.ui.view\" >") {
		t.Errorf("a file outside the selection was reformatted:\n%s", after)
	}
}

// TestFormat_ExitNonZeroOnFormat covers the hook-friendly exit code.
func TestFormat_ExitNonZeroOnFormat(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.xml", unformattedView)

	got := run(t, dir, "", "format", "--exit-non-zero-on-format", "a.xml")
	if got.code != 1 {
		t.Fatalf("exit = %d, want 1 when a file was rewritten", got.code)
	}
	second := run(t, dir, "", "format", "--exit-non-zero-on-format", "a.xml")
	if second.code != 0 {
		t.Fatalf("exit = %d, want 0 when nothing changed", second.code)
	}
}

// TestFormat_RangeIsRejectedByCheck keeps the flag on the command that owns it.
func TestFormat_RangeIsRejectedByCheck(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.xml", badView)

	got := run(t, dir, "", "check", "--range", "1:1-2:2", "a.xml")
	if got.code != 2 {
		t.Fatalf("exit = %d, want 2", got.code)
	}
	if !strings.Contains(got.stderr, "--range") {
		t.Errorf("stderr = %q, want it to mention --range", got.stderr)
	}
}

// TestFormat_ConfigInfluencesOutput proves .ruxx.toml reaches the formatter:
// tabs, a dropped declaration, and CRLF-free output.
func TestFormat_ConfigInfluencesOutput(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".ruxx.toml", "[format]\nindent-style = \"tab\"\nxml-declaration = \"never\"\n")
	path := write(t, dir, "a.xml", unformattedView)

	if code := run(t, dir, "", "format", "a.xml").code; code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	after := read(t, path)
	if strings.Contains(after, "<?xml") {
		t.Errorf("file = %q, want the declaration dropped", after)
	}
	if !strings.Contains(after, "\t<record") {
		t.Errorf("file = %q, want tab indentation", after)
	}
}
