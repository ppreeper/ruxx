package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

const unformattedDoc = "<?xml version=\"1.0\"?>\n<odoo>\n    <record id=\"a\" model=\"ir.ui.view\"><field name=\"x\"></field></record>\n</odoo>\n"

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCheck_FixableRestrictsFixes(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", fixableSrc)

	// X007 fires but only O004 may fix: nothing changes.
	code, _, stderr := run(t, "", "--fix", "--select", "X007", "--fixable", "O004", f)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitFindings, stderr)
	}
	if read(t, f) != fixableSrc {
		t.Errorf("--fixable O004 should not have fixed X007:\n%q", read(t, f))
	}

	// X007 may fix: the trailing whitespace goes away and the run is clean.
	code, _, stderr = run(t, "", "--fix", "--select", "X007", "--fixable", "X007", f)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitClean, stderr)
	}
	if strings.Contains(read(t, f), "odoo>   ") {
		t.Errorf("X007 was not fixed:\n%q", read(t, f))
	}

	// --unfixable wins over --fixable.
	f2 := writeFile(t, dir, "b.xml", fixableSrc)
	code, _, _ = run(t, "", "--fix", "--select", "X007", "--fixable", "X007", "--unfixable", "X007", f2)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d", code, cli.ExitFindings)
	}
	if read(t, f2) != fixableSrc {
		t.Errorf("--unfixable should have won:\n%q", read(t, f2))
	}
}

func TestCheck_FixableUnknownSelector(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", fixableSrc)
	code, _, stderr := run(t, "", "--fix", "--fixable", "ZZ999", f)
	if code != cli.ExitAbnormal {
		t.Fatalf("exit = %d, want %d", code, cli.ExitAbnormal)
	}
	if !strings.Contains(stderr, "ZZ999") {
		t.Errorf("stderr = %q, want the bad selector named", stderr)
	}
}

func TestCheck_ShowSettings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".ruxx.toml", "line-length = 100\n")
	writeFile(t, dir, "a.xml", badO004)

	code, stdout, stderr := run(t, "", "--show-settings", dir)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitClean, stderr)
	}
	if !strings.Contains(stdout, "Resolved settings for") {
		t.Errorf("stdout does not look like settings output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "line-length") || !strings.Contains(stdout, "100") {
		t.Errorf("configured line-length missing from:\n%s", stdout)
	}
	if strings.Contains(stdout, "O004") {
		t.Errorf("--show-settings should not lint:\n%s", stdout)
	}
}

func TestCheck_QuietSuppressesFixSummary(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", fixableSrc)

	code, stdout, _ := run(t, "", "--fix", "--show-fixes", "--select", "X007", f)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d", code, cli.ExitClean)
	}
	if !strings.Contains(stdout, "Fixed") {
		t.Fatalf("expected a fix summary, got %q", stdout)
	}

	writeFile(t, dir, "b.xml", fixableSrc)
	code, stdout, _ = run(t, "", "--fix", "--show-fixes", "--quiet", "--select", "X007", filepath.Join(dir, "b.xml"))
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d", code, cli.ExitClean)
	}
	if strings.Contains(stdout, "Fixed") {
		t.Errorf("--quiet should hide the summary, got %q", stdout)
	}
}

func TestCheck_SilentSuppressesStdout(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", badO004)
	code, stdout, _ := run(t, "", "--silent", f)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d", code, cli.ExitFindings)
	}
	if stdout != "" {
		t.Errorf("--silent stdout = %q, want empty", stdout)
	}
}

func TestCheck_VerboseWritesStderr(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", badO004)
	_, _, stderr := run(t, "", "--verbose", f)
	if !strings.Contains(stderr, "[ruxx] checked") {
		t.Errorf("stderr = %q, want a verbose summary", stderr)
	}
}

func TestCheck_EnvOutputFile(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", badO004)
	out := filepath.Join(dir, "report.txt")
	t.Setenv("RUXX_OUTPUT_FILE", out)

	code, stdout, stderr := run(t, "", "check", f)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitFindings, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty (report goes to the file)", stdout)
	}
	if !strings.Contains(read(t, out), "O004") {
		t.Errorf("report file does not contain the finding:\n%s", read(t, out))
	}
}

func TestCheck_EnvCacheDir(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", badO004)
	cacheDir := filepath.Join(dir, "envcache")
	t.Setenv("RUXX_CACHE_DIR", cacheDir)

	if code, _, stderr := run(t, "", "check", f); code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitFindings, stderr)
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("RUXX_CACHE_DIR was not used: %v", err)
	}
	if len(entries) == 0 {
		t.Error("cache directory is empty")
	}
}

func TestCheck_EnvNoCache(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", badO004)
	cacheDir := filepath.Join(dir, "nocache")
	t.Setenv("RUXX_NO_CACHE", "1")
	t.Setenv("RUXX_CACHE_DIR", cacheDir)

	if code, _, _ := run(t, "", "check", f); code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d", code, cli.ExitFindings)
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Errorf("RUXX_NO_CACHE should prevent cache writes (err = %v)", err)
	}
}

func TestCheck_PreviewIsNoOpForStableRules(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", badO004)
	code, stdout, stderr := run(t, "", "--preview", f)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitFindings, stderr)
	}
	if !strings.Contains(stdout, "O004") {
		t.Errorf("stdout = %q, want the O004 finding", stdout)
	}
}

func TestFormat_ExitNonZeroOnFormat(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "a.xml", unformattedDoc)

	code, _, stderr := run(t, "", "format", "--exit-non-zero-on-format", f)
	if code != cli.ExitFindings {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitFindings, stderr)
	}
	if read(t, f) == unformattedDoc {
		t.Error("file was not reformatted")
	}

	// It is a format-only flag.
	if code, _, _ := run(t, "", "--exit-non-zero-on-format", f); code != cli.ExitAbnormal {
		t.Errorf("check --exit-non-zero-on-format: exit = %d, want %d", code, cli.ExitAbnormal)
	}

	// A second run has nothing to do and exits 0.
	if code, _, stderr := run(t, "", "format", "--exit-non-zero-on-format", f); code != cli.ExitClean {
		t.Errorf("second run: exit = %d, want %d (stderr %q)", code, cli.ExitClean, stderr)
	}
}
