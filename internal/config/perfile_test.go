package config_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ppreeper/ruxx/internal/config"
)

// TestSettingsFor_PerFileIgnores checks that matching globs append their codes
// to Ignore and that non-matching files are left alone. Patterns resolve
// against the config directory, so they behave the same however the file was
// reached.
func TestSettingsFor_PerFileIgnores(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, ".ruxx.toml",
		"[lint.per-file-ignores]\n\"**/odoo/**\" = [\"X010\"]\n\"views/*.xml\" = [\"O004\", \"O005\"]\n")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got, want := cfg.SettingsFor(filepath.Join(dir, "views", "a.xml")).Ignore, []string{"O004", "O005"}; !reflect.DeepEqual(got, want) {
		t.Errorf("views/a.xml Ignore = %v, want %v", got, want)
	}
	if got := cfg.SettingsFor(filepath.Join(dir, "data", "b.xml")).Ignore; len(got) != 0 {
		t.Errorf("data/b.xml Ignore = %v, want none", got)
	}
	if got, want := cfg.SettingsFor(filepath.Join(dir, "addons", "x", "odoo", "c.xml")).Ignore, []string{"X010"}; !reflect.DeepEqual(got, want) {
		t.Errorf("addons/x/odoo/c.xml Ignore = %v, want %v", got, want)
	}
}

// TestSettingsFor_PerFileIgnoresDoesNotAlias guards the copy: adding ignores for
// one file must not leak into the base settings or another file.
func TestSettingsFor_PerFileIgnoresDoesNotAlias(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, ".ruxx.toml",
		"[lint]\nselect = [\"O004\"]\n\n[lint.per-file-ignores]\n\"views/*.xml\" = [\"O004\"]\n")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_ = cfg.SettingsFor(filepath.Join(dir, "views", "a.xml"))
	if got := cfg.SettingsFor(filepath.Join(dir, "data", "b.xml")).Ignore; len(got) != 0 {
		t.Errorf("data/b.xml Ignore = %v, want none", got)
	}
	if got := cfg.Settings.Ignore; len(got) != 0 {
		t.Errorf("base Ignore = %v, want none", got)
	}
}
