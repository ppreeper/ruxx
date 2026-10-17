package e2e

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// repoRoot locates the checkout from this test file's path, so the manifest
// check works whatever the working directory is.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// TestPreCommitManifest checks the hook manifest that consumers point
// pre-commit at: the ids, entries, language and file filter.
func TestPreCommitManifest(t *testing.T) {
	manifest := read(t, filepath.Join(repoRoot(t), ".pre-commit-hooks.yaml"))

	for _, want := range []string{
		"id: ruxx-check",
		"id: ruxx-format",
		"entry: ruxx check",
		"entry: ruxx format",
		"language: golang",
		"--force-exclude",
		`\.(xml|qweb|xml\.template)$`,
	} {
		if !strings.Contains(manifest, want) {
			t.Errorf("manifest is missing %q:\n%s", want, manifest)
		}
	}
}

// TestPreCommitFileFilter runs the manifest's own `files` pattern against the
// names it is meant to select, so the regex cannot silently rot.
func TestPreCommitFileFilter(t *testing.T) {
	manifest := read(t, filepath.Join(repoRoot(t), ".pre-commit-hooks.yaml"))
	re := regexp.MustCompile(`\.\(xml\|qweb\|xml\\\.template\)\$`)
	match := re.FindString(manifest)
	if match == "" {
		t.Fatalf("could not find the files pattern in the manifest:\n%s", manifest)
	}
	pattern, err := regexp.Compile(match)
	if err != nil {
		t.Fatalf("files pattern %q does not compile: %v", match, err)
	}

	selected := []string{"views/a.xml", "report/t.qweb", "x.xml.template"}
	for _, name := range selected {
		if !pattern.MatchString(name) {
			t.Errorf("files pattern does not select %q", name)
		}
	}
	skipped := []string{"a.txt", "a.xml.bak", "notes.md"}
	for _, name := range skipped {
		if pattern.MatchString(name) {
			t.Errorf("files pattern wrongly selects %q", name)
		}
	}
}

// TestPreCommitHookContract replays what pre-commit actually invokes: the hook
// entry plus its args, over the selected files.
func TestPreCommitHookContract(t *testing.T) {
	t.Run("check fails on findings", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "bad.xml", badView)
		write(t, dir, "ok.xml", cleanView)

		bad := run(t, dir, "", "check", "--force-exclude", "bad.xml")
		if bad.code != 1 {
			t.Fatalf("exit = %d, want 1 so the hook fails", bad.code)
		}
		if !strings.Contains(bad.stdout, "O004") {
			t.Errorf("stdout = %q, want the finding", bad.stdout)
		}

		ok := run(t, dir, "", "check", "--force-exclude", "ok.xml")
		if ok.code != 0 {
			t.Fatalf("exit = %d, want 0 for a clean file", ok.code)
		}
	})

	t.Run("format rewrites in place", func(t *testing.T) {
		dir := t.TempDir()
		path := write(t, dir, "a.xml", unformattedView)

		got := run(t, dir, "", "format", "--force-exclude", "a.xml")
		if got.code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr %q)", got.code, got.stderr)
		}
		if read(t, path) == unformattedView {
			t.Error("format left the file unchanged; pre-commit would not report it")
		}
	})

	t.Run("force-exclude still honours configured excludes", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, ".ruxx.toml", "exclude = [\"vendor\"]\n")
		write(t, dir, "vendor/a.xml", badView)

		got := run(t, dir, "", "check", "--force-exclude", "vendor/a.xml")
		if got.code != 0 {
			t.Fatalf("exit = %d, want 0: --force-exclude must apply the configured excludes (stdout %q)", got.code, got.stdout)
		}
		if got.stdout != "" {
			t.Errorf("stdout = %q, want the excluded file skipped", got.stdout)
		}
	})
}
