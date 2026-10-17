package formatter_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/ppreeper/ruxx/internal/config"
	"github.com/ppreeper/ruxx/internal/formatter"
	"github.com/ppreeper/ruxx/internal/settings"
)

var update = flag.Bool("update", false, "update expected.xml golden files")

// TestFormatGoldens runs every case under testdata/format. Each case is a
// directory with input.xml and expected.xml. It also asserts idempotency:
// formatting the expected output again must not change it.
func TestFormatGoldens(t *testing.T) {
	root := "testdata/format"
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	var ran int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ran++
		dir := filepath.Join(root, e.Name())
		t.Run(e.Name(), func(t *testing.T) { runCase(t, dir) })
	}
	if ran == 0 {
		t.Fatalf("no format cases under %s", root)
	}
}

func runCase(t *testing.T, dir string) {
	t.Helper()
	input := mustRead(t, filepath.Join(dir, "input.xml"))
	s := settings.Default()

	// A case may carry its own .ruxx.toml to exercise formatter options.
	if _, err := os.Stat(filepath.Join(dir, ".ruxx.toml")); err == nil {
		cfg, err := config.Load(filepath.Join(dir, ".ruxx.toml"))
		if err != nil {
			t.Fatalf("load %s/.ruxx.toml: %v", dir, err)
		}
		s = cfg.Settings
	}

	got, err := formatter.Format(input, s)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	expectedPath := filepath.Join(dir, "expected.xml")
	if *update {
		if err := os.WriteFile(expectedPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write expected: %v", err)
		}
	} else {
		want := mustRead(t, expectedPath)
		if got != want {
			t.Errorf("format mismatch for %s\n--- want ---\n%s\n--- got ---\n%s", dir, want, got)
		}
	}

	// Idempotency: formatting the result must be a no-op.
	again, err := formatter.Format(got, s)
	if err != nil {
		t.Fatalf("Format (second pass): %v", err)
	}
	if again != got {
		t.Errorf("formatter is not idempotent for %s\n--- once ---\n%s\n--- twice ---\n%s", dir, got, again)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
