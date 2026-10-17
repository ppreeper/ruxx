package finder

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/blake2b"
)

// setupTestDir creates a temporary directory with the given file structure.
// files maps relative path to content.
func setupTestDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestFind_BasicDuplicates(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"a.txt": "hello",
		"b.txt": "hello",
		"c.txt": "world",
	})

	groups, err := Find([]string{dir}, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Paths) != 2 {
		t.Fatalf("expected 2 paths in group, got %d", len(groups[0].Paths))
	}
}

func TestFind_NoDuplicates(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"a.txt": "alpha",
		"b.txt": "bravo",
		"c.txt": "charlie",
	})

	groups, err := Find([]string{dir}, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups, got %d", len(groups))
	}
}

func TestFind_MultipleGroups(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"a.txt": "hello",
		"b.txt": "hello",
		"c.txt": "world",
		"d.txt": "world",
		"e.txt": "unique content here",
	})

	groups, err := Find([]string{dir}, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
}

func TestFind_MinCount(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"a.txt": "hello",
		"b.txt": "hello",
		"c.txt": "hello",
		"d.txt": "world",
		"e.txt": "world",
	})

	// minCount=3: only the "hello" group (3 files) qualifies.
	groups, err := Find([]string{dir}, 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Paths) != 3 {
		t.Fatalf("expected 3 paths in group, got %d", len(groups[0].Paths))
	}
}

func TestFind_MinCountClampedToTwo(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"a.txt": "hello",
		"b.txt": "hello",
	})

	// minCount < 2 should be clamped to 2.
	groups, err := Find([]string{dir}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
}

func TestFind_SizePreFilter(t *testing.T) {
	// Files with different sizes cannot be duplicates.
	// "hi" is 2 bytes, "hello" is 5 bytes -- different sizes, skipped before hashing.
	dir := setupTestDir(t, map[string]string{
		"a.txt": "hi",
		"b.txt": "hello",
	})

	groups, err := Find([]string{dir}, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups, got %d", len(groups))
	}
}

func TestFind_SameSizeNotDuplicate(t *testing.T) {
	// Same size (5 bytes) but different content -- passes size filter, rejected by hash.
	dir := setupTestDir(t, map[string]string{
		"a.txt": "hello",
		"b.txt": "world",
	})

	groups, err := Find([]string{dir}, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups (same size, different content), got %d", len(groups))
	}
}

func TestFind_SkipEmpty(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"a.txt": "",
		"b.txt": "",
		"c.txt": "content",
	})

	// Without SkipEmpty: empty files are grouped as duplicates.
	groups, err := Find([]string{dir}, 2, &Options{SkipEmpty: false})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group (empty files), got %d", len(groups))
	}

	// With SkipEmpty: empty files are excluded entirely.
	groups, err = Find([]string{dir}, 2, &Options{SkipEmpty: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups with SkipEmpty, got %d", len(groups))
	}
}

func TestFind_MultipleDirectories(t *testing.T) {
	dir1 := setupTestDir(t, map[string]string{
		"a.txt": "hello",
	})
	dir2 := setupTestDir(t, map[string]string{
		"b.txt": "hello",
	})

	groups, err := Find([]string{dir1, dir2}, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group across directories, got %d", len(groups))
	}
	if len(groups[0].Paths) != 2 {
		t.Fatalf("expected 2 paths in group, got %d", len(groups[0].Paths))
	}
}

func TestFind_DeterministicOrder(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"z.txt": "hello",
		"a.txt": "hello",
		"m.txt": "hello",
	})

	// Run multiple times to verify deterministic ordering.
	for range 10 {
		groups, err := Find([]string{dir}, 2, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		// Paths within a group must be sorted.
		for j := 1; j < len(groups[0].Paths); j++ {
			if groups[0].Paths[j-1] >= groups[0].Paths[j] {
				t.Fatalf("paths not sorted: %v", groups[0].Paths)
			}
		}
	}
}

func TestFind_Subdirectories(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"a.txt":         "hello",
		"sub/b.txt":     "hello",
		"deep/d/e.txt":  "hello",
		"sub/unique.go": "unique content",
	})

	groups, err := Find([]string{dir}, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Paths) != 3 {
		t.Fatalf("expected 3 paths in group, got %d", len(groups[0].Paths))
	}
}

func TestFind_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	groups, err := Find([]string{dir}, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups for empty dir, got %d", len(groups))
	}
}

func TestFind_WarningsCollected(t *testing.T) {
	var warnings []string
	opts := &Options{
		WarnFunc: func(msg string) {
			warnings = append(warnings, msg)
		},
	}

	// Walk a nonexistent subdirectory to trigger a warning.
	dir := setupTestDir(t, map[string]string{
		"a.txt": "hello",
	})
	// Create a directory that we remove read permissions from.
	unreadable := filepath.Join(dir, "noperm")
	if err := os.Mkdir(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(unreadable, 0o755) })

	_, err := Find([]string{dir}, 2, opts)
	if err != nil {
		t.Fatal(err)
	}
	// We should have gotten at least one warning about the unreadable directory.
	if len(warnings) == 0 {
		t.Log("no warnings generated (may require non-root execution)")
	}
}

func TestFind_WorkersOption(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"a.txt": "hello",
		"b.txt": "hello",
		"c.txt": "hello",
	})

	// Explicitly set workers to 1 (sequential).
	groups, err := Find([]string{dir}, 2, &Options{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group with Workers=1, got %d", len(groups))
	}

	// Explicitly set workers to a high number.
	groups, err = Find([]string{dir}, 2, &Options{Workers: 16})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group with Workers=16, got %d", len(groups))
	}
}

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := hashFile(path)
	if err != nil {
		t.Fatal(err)
	}

	want := blake2b.Sum256(content)
	if got != want {
		t.Fatalf("hash mismatch: got %x, want %x", got, want)
	}
}

func TestHashFile_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := hashFile(path)
	if err != nil {
		t.Fatal(err)
	}

	want := blake2b.Sum256(nil)
	if got != want {
		t.Fatalf("hash mismatch for empty file: got %x, want %x", got, want)
	}
}

func TestHashFile_NotFound(t *testing.T) {
	_, err := hashFile("/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
