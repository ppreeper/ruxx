package config_test

import (
	"testing"

	"github.com/ppreeper/ruxx/internal/config"
)

func TestLoad_FixableUnfixablePreview(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, ".ruxx.toml",
		"preview = true\n\n[lint]\nfixable = [\"X007\", \"O002\"]\nunfixable = [\"O002\"]\n")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.Settings
	if len(s.Fixable) != 2 || s.Fixable[0] != "X007" || s.Fixable[1] != "O002" {
		t.Errorf("Fixable = %v, want [X007 O002]", s.Fixable)
	}
	if len(s.Unfixable) != 1 || s.Unfixable[0] != "O002" {
		t.Errorf("Unfixable = %v, want [O002]", s.Unfixable)
	}
	if !s.Preview {
		t.Error("Preview = false, want true")
	}
}

func TestLoad_FixableDefaults(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, ".ruxx.toml", "line-length = 100\n")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.Settings
	if len(s.Fixable) != 0 {
		t.Errorf("Fixable = %v, want empty", s.Fixable)
	}
	if len(s.Unfixable) != 0 {
		t.Errorf("Unfixable = %v, want empty", s.Unfixable)
	}
	if s.Preview {
		t.Error("Preview = true, want false")
	}
}
