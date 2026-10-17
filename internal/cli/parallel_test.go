package cli_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

// TestCheck_ParallelOutputIsDeterministic lints a directory of many files and
// asserts the diagnostics arrive in sorted path order, identically across runs
// (the worker pool must not reorder output).
func TestCheck_ParallelOutputIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	const n = 12
	for i := 0; i < n; i++ {
		writeFile(t, dir, fmt.Sprintf("f%02d.xml", i), badO004)
	}

	first := ""
	for attempt := 0; attempt < 3; attempt++ {
		code, stdout, _ := run(t, "", "check", dir)
		if code != cli.ExitFindings {
			t.Fatalf("exit = %d, want %d", code, cli.ExitFindings)
		}
		if attempt == 0 {
			first = stdout
			continue
		}
		if stdout != first {
			t.Fatalf("output changed between runs:\n--- first ---\n%s\n--- again ---\n%s", first, stdout)
		}
	}

	lines := strings.Split(strings.TrimSpace(first), "\n")
	if len(lines) != n {
		t.Fatalf("got %d diagnostics, want %d:\n%s", len(lines), n, first)
	}
	for i, line := range lines {
		want := fmt.Sprintf("%s:3:5:", filepath.Join(dir, fmt.Sprintf("f%02d.xml", i)))
		if !strings.HasPrefix(line, want) {
			t.Fatalf("line %d = %q, want prefix %q", i, line, want)
		}
	}
}

// TestFormat_ParallelOutputIsDeterministic is the formatter counterpart: the
// --check verdicts must be emitted in sorted path order.
func TestFormat_ParallelOutputIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	const n = 10
	for i := 0; i < n; i++ {
		writeFile(t, dir, fmt.Sprintf("f%02d.xml", i), "<odoo>\n    <field name=\"x\"></field>\n</odoo>\n")
	}

	code, stdout, _ := run(t, "", "format", "--check", dir)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d", code, cli.ExitFindings)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != n {
		t.Fatalf("got %d verdicts, want %d:\n%s", len(lines), n, stdout)
	}
	for i, line := range lines {
		want := "would reformat " + filepath.Join(dir, fmt.Sprintf("f%02d.xml", i))
		if line != want {
			t.Fatalf("line %d = %q, want %q", i, line, want)
		}
	}
}
