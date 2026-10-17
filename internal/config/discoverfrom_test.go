package config_test

import (
	"path/filepath"
	"testing"

	"github.com/ppreeper/ruxx/internal/config"
	"github.com/ppreeper/ruxx/internal/settings"
)

func TestDiscoverFrom_UsesClosestConfig(t *testing.T) {
	base := t.TempDir()
	write(t, base, ".ruxx.toml", "line-length = 100\n")
	sub := filepath.Join(base, "sub")
	write(t, sub, ".ruxx.toml", "line-length = 120\n")

	cfg, err := config.DiscoverFrom(sub)
	if err != nil {
		t.Fatalf("DiscoverFrom: %v", err)
	}
	if cfg.Settings.LineLength != 120 {
		t.Errorf("sub line-length = %d, want 120", cfg.Settings.LineLength)
	}
	if cfg.Dir != sub {
		t.Errorf("Dir = %q, want %q", cfg.Dir, sub)
	}

	cfg, err = config.DiscoverFrom(base)
	if err != nil {
		t.Fatalf("DiscoverFrom: %v", err)
	}
	if cfg.Settings.LineLength != 100 {
		t.Errorf("base line-length = %d, want 100", cfg.Settings.LineLength)
	}
}

func TestDiscoverFrom_WalksUpAndDefaults(t *testing.T) {
	base := t.TempDir()
	write(t, base, "ruxx.toml", "line-length = 90\n")
	deep := filepath.Join(base, "a", "b", "c")
	write(t, deep, "keep.xml", "<odoo/>\n")

	cfg, err := config.DiscoverFrom(deep)
	if err != nil {
		t.Fatalf("DiscoverFrom: %v", err)
	}
	if cfg.Settings.LineLength != 90 {
		t.Errorf("line-length = %d, want 90", cfg.Settings.LineLength)
	}

	// A tree with no config at all falls back to the built-in defaults.
	empty := t.TempDir()
	cfg, err = config.DiscoverFrom(empty)
	if err != nil {
		t.Fatalf("DiscoverFrom: %v", err)
	}
	if cfg.Path != "" {
		t.Errorf("Path = %q, want empty", cfg.Path)
	}
	if cfg.Settings.LineLength != settings.DefaultLineLength {
		t.Errorf("line-length = %d, want default %d", cfg.Settings.LineLength, settings.DefaultLineLength)
	}
}
