package config_test

import (
	"path/filepath"
	"testing"

	"github.com/ppreeper/ruxx/internal/config"
)

func TestLoad_CacheDir(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".ruxx.toml", "cache-dir = \".cache\"\n")

	cfg, err := config.Load(filepath.Join(dir, ".ruxx.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := filepath.Join(dir, ".cache")
	if cfg.Settings.CacheDir != want {
		t.Errorf("CacheDir = %q, want %q", cfg.Settings.CacheDir, want)
	}
}

func TestLoad_CacheDirDefault(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".ruxx.toml", "line-length = 100\n")

	cfg, err := config.Load(filepath.Join(dir, ".ruxx.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Settings.CacheDir == "" {
		t.Error("CacheDir should retain the default when unset")
	}
}
