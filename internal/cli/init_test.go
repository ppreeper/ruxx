package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
	"github.com/ppreeper/ruxx/internal/config"
)

func TestInit_CreatesLoadableConfig(t *testing.T) {
	dir := t.TempDir()
	code, stdout, stderr := run(t, "", "init", dir)
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitClean, stderr)
	}
	target := filepath.Join(dir, ".ruxx.toml")
	if !strings.Contains(stdout, target) {
		t.Fatalf("stdout = %q, want it to name %s", stdout, target)
	}
	// The scaffold must be valid TOML that config.Load accepts.
	cfg, err := config.Load(target)
	if err != nil {
		t.Fatalf("generated config does not load: %v", err)
	}
	if cfg.Settings.LineLength != 88 {
		t.Fatalf("line-length = %d, want 88", cfg.Settings.LineLength)
	}
	if cfg.Settings.TargetVersion != "auto" {
		t.Fatalf("target-version = %q, want auto", cfg.Settings.TargetVersion)
	}
}

func TestInit_RefusesToOverwrite(t *testing.T) {
	dir := t.TempDir()
	run(t, "", "init", dir)
	target := filepath.Join(dir, ".ruxx.toml")
	os.WriteFile(target, []byte("line-length = 120\n"), 0o644)

	code, _, stderr := run(t, "", "init", dir)
	if code != cli.ExitAbnormal {
		t.Fatalf("exit = %d, want %d", code, cli.ExitAbnormal)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Fatalf("stderr = %q, want it to refuse", stderr)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "line-length = 120\n" {
		t.Fatal("existing config must be left untouched")
	}
}

func TestInit_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".ruxx.toml")
	os.WriteFile(target, []byte("line-length = 120\n"), 0o644)

	if code, _, stderr := run(t, "", "init", "--force", dir); code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitClean, stderr)
	}
	data, _ := os.ReadFile(target)
	if !strings.Contains(string(data), "[lint]") {
		t.Fatalf("config not overwritten:\n%s", data)
	}
}

func TestInit_CreatesDirectory(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "sub", "project")
	if code, _, stderr := run(t, "", "init", nested); code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitClean, stderr)
	}
	if _, err := os.Stat(filepath.Join(nested, ".ruxx.toml")); err != nil {
		t.Fatalf("config not created: %v", err)
	}
}

func TestInit_UnknownFlag(t *testing.T) {
	if code, _, stderr := run(t, "", "init", "--nope"); code != cli.ExitAbnormal {
		t.Fatalf("exit = %d, want %d", code, cli.ExitAbnormal)
	} else if !strings.Contains(stderr, "nope") {
		t.Fatalf("stderr = %q, want it to name the flag", stderr)
	}
}

// TestInit_ScaffoldSelectsCorrectness locks the scaffold's lint default to the
// same set ruxx applies with no config at all.
func TestInit_ScaffoldSelectsCorrectness(t *testing.T) {
	dir := t.TempDir()
	run(t, "", "init", dir)
	cfg, err := config.Load(filepath.Join(dir, ".ruxx.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Settings.Select) != 1 || cfg.Settings.Select[0] != "correctness" {
		t.Fatalf("select = %v, want [correctness]", cfg.Settings.Select)
	}
}
