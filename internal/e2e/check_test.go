package e2e

import (
	"strings"
	"testing"
)

// TestCheck_CleanExitsZero is the baseline contract: nothing to report means
// exit 0 and no output.
func TestCheck_CleanExitsZero(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "clean.xml", cleanView)

	got := run(t, dir, "", "check", "clean.xml")
	if got.code != 0 {
		t.Fatalf("exit = %d, want 0 (stdout %q stderr %q)", got.code, got.stdout, got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty", got.stdout)
	}
}

// TestCheck_FindingsExitOne pins the diagnostic format a user greps for:
// path:line:col: CODE message.
func TestCheck_FindingsExitOne(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "bad.xml", badView)

	got := run(t, dir, "", "check", "bad.xml")
	if got.code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr %q)", got.code, got.stderr)
	}
	want := "bad.xml:3:5: O004 <record> is missing the required \"model\" attribute"
	if !strings.Contains(got.stdout, want) {
		t.Errorf("stdout = %q, want it to contain %q", got.stdout, want)
	}
	// X007 is a style rule: off unless selected.
	if strings.Contains(got.stdout, "X007") {
		t.Errorf("stdout = %q, want no X007 by default", got.stdout)
	}
}

// TestCheck_AbnormalExitsTwo covers the third exit code: bad usage and
// unusable paths, not findings.
func TestCheck_AbnormalExitsTwo(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "bad.xml", badView)

	t.Run("unknown flag", func(t *testing.T) {
		got := run(t, dir, "", "check", "--bogus", "bad.xml")
		if got.code != 2 {
			t.Fatalf("exit = %d, want 2", got.code)
		}
		if !strings.Contains(got.stderr, "--bogus") {
			t.Errorf("stderr = %q, want it to name the flag", got.stderr)
		}
	})

	t.Run("missing path", func(t *testing.T) {
		got := run(t, dir, "", "check", "nope.xml")
		if got.code != 2 {
			t.Fatalf("exit = %d, want 2 (stdout %q)", got.code, got.stdout)
		}
		if got.stderr == "" {
			t.Error("stderr is empty, want an explanation")
		}
	})
}

// TestCheck_ExitZeroOverridesFindings is the CI escape hatch.
func TestCheck_ExitZeroOverridesFindings(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "bad.xml", badView)

	got := run(t, dir, "", "check", "--exit-zero", "bad.xml")
	if got.code != 0 {
		t.Fatalf("exit = %d, want 0", got.code)
	}
	if !strings.Contains(got.stdout, "O004") {
		t.Errorf("stdout = %q, want the finding to still be reported", got.stdout)
	}
}

// TestCheck_Stdin drives the editor/pre-commit path: a document on stdin with
// a logical name for the diagnostics.
func TestCheck_Stdin(t *testing.T) {
	dir := t.TempDir()

	got := run(t, dir, badView, "check", "--stdin-filename", "views/a.xml", "-")
	if got.code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr %q)", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "views/a.xml:3:5: O004") {
		t.Errorf("stdout = %q, want the stdin filename to be used", got.stdout)
	}
}

// TestCheck_Statistics checks the summary mode used in CI logs.
func TestCheck_Statistics(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.xml", badView)
	write(t, dir, "b.xml", badView)

	got := run(t, dir, "", "check", "--statistics", ".")
	if got.code != 1 {
		t.Fatalf("exit = %d, want 1", got.code)
	}
	if !strings.Contains(got.stdout, "O004") || !strings.Contains(got.stdout, "2") {
		t.Errorf("stdout = %q, want a count of 2 for O004", got.stdout)
	}
}

// TestCheck_ShowFiles lists the selection without linting.
func TestCheck_ShowFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.xml", badView)
	write(t, dir, "notes.txt", "not xml")

	got := run(t, dir, "", "check", "--show-files", ".")
	if got.code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "a.xml") {
		t.Errorf("stdout = %q, want a.xml", got.stdout)
	}
	if strings.Contains(got.stdout, "notes.txt") {
		t.Errorf("stdout = %q, want non-XML files skipped", got.stdout)
	}
	if strings.Contains(got.stdout, "O004") {
		t.Errorf("stdout = %q, want no linting under --show-files", got.stdout)
	}
}

// TestCheck_FixAppliesSafeFixesThenReportsRemaining is the core --fix story:
// the fixable finding goes away, the unfixable one is still reported.
func TestCheck_FixAppliesSafeFixesThenReportsRemaining(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "bad.xml", badView)

	got := run(t, dir, "", "check", "--select", "X007,O004", "--fix", "bad.xml")
	if got.code != 1 {
		t.Fatalf("exit = %d, want 1 (O004 is not fixable)", got.code)
	}
	if !strings.Contains(got.stdout, "O004") {
		t.Errorf("stdout = %q, want the remaining O004 finding", got.stdout)
	}
	fixed := read(t, path)
	if strings.Contains(fixed, "<odoo>   \n") {
		t.Errorf("trailing whitespace was not fixed:\n%s", fixed)
	}
	if !strings.Contains(fixed, "<odoo>\n") {
		t.Errorf("file = %q, want the X007 fix applied", fixed)
	}
}

// TestCheck_FixOnlyAppliesAndStaysQuiet covers the pre-commit mode: fix, report
// nothing, exit 0 when everything fixable is fixed.
func TestCheck_FixOnlyAppliesAndStaysQuiet(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "bad.xml", badView)

	got := run(t, dir, "", "check", "--select", "X007", "--fix-only", "bad.xml")
	if got.code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", got.code, got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty under --fix-only", got.stdout)
	}
	if strings.Contains(read(t, path), "   \n") {
		t.Errorf("file = %q, want the fix applied", read(t, path))
	}
}

// TestCheck_DiffPrintsPatchAndDoesNotWrite is the review-my-changes path.
func TestCheck_DiffPrintsPatchAndDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	before := badView
	path := write(t, dir, "bad.xml", before)

	got := run(t, dir, "", "check", "--select", "X007", "--diff", "bad.xml")
	if !strings.Contains(got.stdout, "--- bad.xml") || !strings.Contains(got.stdout, "+++ bad.xml") {
		t.Fatalf("stdout = %q, want unified diff headers", got.stdout)
	}
	if !strings.Contains(got.stdout, "@@") {
		t.Errorf("stdout = %q, want a hunk header", got.stdout)
	}
	if read(t, path) != before {
		t.Error("--diff modified the file")
	}
}

// TestCheck_AddIgnoreAuthorsSuppression covers the migration aid: the finding
// is suppressed in place, so the next run is clean.
func TestCheck_AddIgnoreAuthorsSuppression(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "bad.xml", badView)

	first := run(t, dir, "", "check", "--select", "O004", "--add-ignore", "bad.xml")
	if first.code != 0 {
		t.Fatalf("exit = %d, want 0 after suppressing the only finding (stderr %q)", first.code, first.stderr)
	}
	if !strings.Contains(read(t, path), "ruxx: ignore[O004]") {
		t.Fatalf("file = %q, want a ruxx suppression comment", read(t, path))
	}
	second := run(t, dir, "", "check", "--select", "O004", "bad.xml")
	if second.code != 0 {
		t.Fatalf("exit = %d, want 0 on the suppressed file (stdout %q)", second.code, second.stdout)
	}
}

// TestCheck_SelectAndIgnore covers the selector surface through the CLI.
func TestCheck_SelectAndIgnore(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "bad.xml", badView)

	all := run(t, dir, "", "check", "--select", "ALL", "bad.xml")
	if !strings.Contains(all.stdout, "X007") || !strings.Contains(all.stdout, "O004") {
		t.Errorf("stdout = %q, want both X007 and O004 under ALL", all.stdout)
	}

	ignored := run(t, dir, "", "check", "--select", "ALL", "--ignore", "O004", "bad.xml")
	if strings.Contains(ignored.stdout, "O004") {
		t.Errorf("stdout = %q, want O004 ignored", ignored.stdout)
	}
	if !strings.Contains(ignored.stdout, "X007") {
		t.Errorf("stdout = %q, want X007 still reported", ignored.stdout)
	}
}
