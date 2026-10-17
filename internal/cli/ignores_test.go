package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

// treeArch is an ir.ui.view record whose arch uses <tree>, for the
// version-gated O010 rule.
const treeArch = `<?xml version="1.0"?>
<odoo>
    <record id="v" model="ir.ui.view">
        <field name="arch" type="xml">
            <tree>
                <field name="name"/>
            </tree>
        </field>
    </record>
</odoo>
`

// extendSrc has both an O004 finding and an ST002 finding, so extending the
// default set is observable.
const extendSrc = `<?xml version="1.0"?>
<odoo>
    <record id="a">
        <field name="x"></field>
    </record>
</odoo>
`

// suppressedSrc carries a file-wide suppression for O004.
const suppressedSrc = `<?xml version="1.0"?>
<!-- ruxx: ignore[O004] -->
<odoo>
    <record id="a"/>
</odoo>
`

// trailingSrc has trailing whitespace (X007, safe fix) and a record with no
// model (O004, no fix).
const trailingSrc = "<?xml version=\"1.0\"?>\n<odoo>   \n</odoo>\n"

const fixableSrc = "<?xml version=\"1.0\"?>\n<odoo>   \n    <record id=\"a\"/>\n</odoo>\n"

// TestCheck_PerFileIgnoresConfig is the regression test for
// [lint.per-file-ignores]: the config used to be parsed but never applied.
func TestCheck_PerFileIgnoresConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".ruxx.toml",
		"[lint]\nselect = [\"O004\"]\n\n[lint.per-file-ignores]\n\"skip/*.xml\" = [\"O004\"]\n")
	if err := os.MkdirAll(filepath.Join(dir, "skip"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "skip/a.xml", badO004)
	writeFile(t, dir, "b.xml", badO004)

	code, stdout, stderr := run(t, "", "check", dir)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitFindings, stderr)
	}
	if strings.Contains(stdout, "skip/a.xml") {
		t.Errorf("skip/a.xml should be ignored, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "b.xml") {
		t.Errorf("b.xml should be reported, got:\n%s", stdout)
	}
}

// TestCheck_PerFileIgnoresFlag covers the CLI equivalent.
func TestCheck_PerFileIgnoresFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "skip"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "skip/a.xml", badO004)
	writeFile(t, dir, "b.xml", badO004)

	code, stdout, stderr := run(t, "", "--select", "O004", "--per-file-ignores", "**/skip/*.xml:O004", dir)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitFindings, stderr)
	}
	if strings.Contains(stdout, "a.xml") {
		t.Errorf("skip/a.xml should be ignored, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "b.xml") {
		t.Errorf("b.xml should be reported, got:\n%s", stdout)
	}
}

func TestCheck_PerFileIgnoresFlagInvalid(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", badO004)
	code, _, stderr := run(t, "", "--per-file-ignores", "bogus", f)
	if code != cli.ExitAbnormal {
		t.Fatalf("exit = %d, want %d", code, cli.ExitAbnormal)
	}
	if !strings.Contains(stderr, "per-file-ignores") {
		t.Errorf("stderr = %q, want it to mention per-file-ignores", stderr)
	}
}

// TestCheck_TargetVersionFlag checks the version gate can be set from the CLI,
// which is the only way to enable it for a tree without a .ruxx.toml.
func TestCheck_TargetVersionFlag(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "v.xml", treeArch)

	code, stdout, stderr := run(t, "", "--select", "O010", "--target-version", "18.0", f)
	if code != cli.ExitFindings {
		t.Fatalf("18.0: exit = %d, want %d (stderr %q)", code, cli.ExitFindings, stderr)
	}
	if !strings.Contains(stdout, "O010") {
		t.Errorf("18.0: stdout = %q, want an O010 finding", stdout)
	}

	code, stdout, _ = run(t, "", "--select", "O010", "--target-version", "17.0", f)
	if code != cli.ExitClean || stdout != "" {
		t.Errorf("17.0: exit = %d stdout = %q, want clean", code, stdout)
	}
}

func TestCheck_ExtendSelect(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", extendSrc)

	// The default (correctness) set reports O004 but not ST002.
	_, stdout, _ := run(t, "", "check", f)
	if !strings.Contains(stdout, "O004") || strings.Contains(stdout, "ST002") {
		t.Fatalf("default stdout = %q, want O004 only", stdout)
	}

	code, stdout, stderr := run(t, "", "--extend-select", "ST002", f)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitFindings, stderr)
	}
	if !strings.Contains(stdout, "ST002") || !strings.Contains(stdout, "O004") {
		t.Errorf("stdout = %q, want both O004 and ST002", stdout)
	}
}

func TestCheck_ExtendIgnore(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", badO004)

	code, stdout, stderr := run(t, "", "--extend-ignore", "O004", f)
	if code != cli.ExitClean || stdout != "" {
		t.Errorf("exit = %d stdout = %q, want clean (stderr %q)", code, stdout, stderr)
	}
}

// TestCheck_IgnoreSuppressions audits a tree while ignoring ruxx: directives.
func TestCheck_IgnoreSuppressions(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", suppressedSrc)

	code, stdout, stderr := run(t, "", "check", f)
	if code != cli.ExitClean || stdout != "" {
		t.Fatalf("suppressed: exit = %d stdout = %q, want clean (stderr %q)", code, stdout, stderr)
	}

	code, stdout, _ = run(t, "", "--ignore-suppressions", f)
	if code != cli.ExitFindings {
		t.Fatalf("--ignore-suppressions: exit = %d, want %d", code, cli.ExitFindings)
	}
	if !strings.Contains(stdout, "O004") {
		t.Errorf("--ignore-suppressions: stdout = %q, want O004", stdout)
	}
}

// TestCheck_FixOnly applies fixes and reports nothing, even when unfixable
// findings remain.
func TestCheck_FixOnly(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", fixableSrc)

	code, stdout, stderr := run(t, "", "--fix", "--select", "X007,O004", f)
	if code != cli.ExitFindings {
		t.Fatalf("--fix: exit = %d, want %d (stderr %q)", code, cli.ExitFindings, stderr)
	}
	if !strings.Contains(stdout, "O004") {
		t.Errorf("--fix: stdout = %q, want the remaining O004", stdout)
	}

	writeFile(t, dir, "a.xml", fixableSrc)
	code, stdout, stderr = run(t, "", "--fix-only", "--select", "X007,O004", f)
	if code != cli.ExitClean {
		t.Fatalf("--fix-only: exit = %d, want %d (stderr %q)", code, cli.ExitClean, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("--fix-only: stdout = %q, want empty", stdout)
	}
	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "   \n") {
		t.Errorf("--fix-only did not apply the X007 fix:\n%q", data)
	}
}

func TestCheck_ShowFixes(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", trailingSrc)

	code, stdout, stderr := run(t, "", "--fix", "--show-fixes", "--select", "X007", f)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitClean, stderr)
	}
	if !strings.Contains(stdout, "Fixed 1 issue(s):") || !strings.Contains(stdout, "X007") {
		t.Errorf("stdout = %q, want a fix summary naming X007", stdout)
	}
}

func TestCheck_ExitNonZeroOnFix(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", trailingSrc)

	code, _, stderr := run(t, "", "--fix", "--exit-non-zero-on-fix", "--select", "X007", f)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitFindings, stderr)
	}
}

// TestCheck_NoFixNegation checks --no-fix wins when it comes last.
func TestCheck_NoFixNegation(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", trailingSrc)

	code, _, _ := run(t, "", "--fix", "--no-fix", "--select", "X007", f)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d", code, cli.ExitFindings)
	}
	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != trailingSrc {
		t.Errorf("--no-fix rewrote the file: %q", data)
	}
}
