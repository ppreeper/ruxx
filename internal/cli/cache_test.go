package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

// countCacheEntries returns the number of .json entries in dir (0 if absent).
func countCacheEntries(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}
	n := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			n++
		}
	}
	return n
}

func TestCheck_UsesCache(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, dir, "a.xml", badO004)
	cacheDir := filepath.Join(dir, "cache")

	code, first, stderr := run(t, "", "check", "--cache-dir", cacheDir, file)
	if code != cli.ExitFindings {
		t.Fatalf("first run: code = %d, want %d (stderr %q)", code, cli.ExitFindings, stderr)
	}
	if got := countCacheEntries(t, cacheDir); got != 1 {
		t.Fatalf("cache entries after first run = %d, want 1", got)
	}

	code, second, _ := run(t, "", "check", "--cache-dir", cacheDir, file)
	if code != cli.ExitFindings {
		t.Fatalf("second run: code = %d, want %d", code, cli.ExitFindings)
	}
	if second != first {
		t.Fatalf("cached output differs:\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestCheck_NoCacheWritesNothing(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, dir, "a.xml", badO004)
	cacheDir := filepath.Join(dir, "cache")

	code, _, stderr := run(t, "", "check", "-n", "--cache-dir", cacheDir, file)
	if code != cli.ExitFindings {
		t.Fatalf("code = %d, want %d (stderr %q)", code, cli.ExitFindings, stderr)
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Fatalf("cache dir %q should not exist with --no-cache (err = %v)", cacheDir, err)
	}
}

func TestCheck_CacheKeyedByContent(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, dir, "a.xml", badO004)
	cacheDir := filepath.Join(dir, "cache")

	if code, _, _ := run(t, "", "check", "--cache-dir", cacheDir, file); code != cli.ExitFindings {
		t.Fatalf("first run code = %d, want %d", code, cli.ExitFindings)
	}

	// Edit the file: one record gains model, so the file becomes clean.
	writeFile(t, dir, "a.xml", cleanO004)
	code, _, _ := run(t, "", "check", "--cache-dir", cacheDir, file)
	if code != cli.ExitClean {
		t.Fatalf("after edit code = %d, want %d", code, cli.ExitClean)
	}
	if got := countCacheEntries(t, cacheDir); got != 2 {
		t.Fatalf("cache entries = %d, want 2 (one per distinct content)", got)
	}
}

func TestCheck_FixDoesNotUseCache(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, dir, "a.xml", badO004)
	cacheDir := filepath.Join(dir, "cache")

	code, _, stderr := run(t, "", "check", "--fix", "--cache-dir", cacheDir, file)
	if code == cli.ExitAbnormal {
		t.Fatalf("--fix run abnormal: %q", stderr)
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Fatalf("cache dir %q should not exist when --fix is set", cacheDir)
	}
}

func TestClean_RemovesCache(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, dir, "a.xml", badO004)
	cacheDir := filepath.Join(dir, "cache")

	if code, _, _ := run(t, "", "check", "--cache-dir", cacheDir, file); code != cli.ExitFindings {
		t.Fatalf("check code = %d, want %d", code, cli.ExitFindings)
	}
	if _, err := os.Stat(cacheDir); err != nil {
		t.Fatalf("cache dir should exist before clean: %v", err)
	}

	code, stdout, stderr := run(t, "", "clean", "--cache-dir", cacheDir)
	if code != cli.ExitClean {
		t.Fatalf("clean code = %d, want %d (stderr %q)", code, cli.ExitClean, stderr)
	}
	if stdout == "" {
		t.Fatalf("clean should report what it removed")
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Fatalf("cache dir should be gone after clean (err = %v)", err)
	}
}

func TestClean_AbsentCacheIsClean(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "never-created")

	code, _, stderr := run(t, "", "clean", "--cache-dir", cacheDir)
	if code != cli.ExitClean {
		t.Fatalf("clean code = %d, want %d (stderr %q)", code, cli.ExitClean, stderr)
	}
}
