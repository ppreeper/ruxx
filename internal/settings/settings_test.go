package settings_test

import (
	"testing"

	"github.com/ppreeper/ruxx/internal/settings"
)

func TestDefault(t *testing.T) {
	s := settings.Default()
	if s.LineLength != settings.DefaultLineLength {
		t.Errorf("LineLength = %d, want %d", s.LineLength, settings.DefaultLineLength)
	}
	if s.IndentWidth != settings.DefaultIndentWidth {
		t.Errorf("IndentWidth = %d, want %d", s.IndentWidth, settings.DefaultIndentWidth)
	}
	if s.TargetVersion != "auto" {
		t.Errorf("TargetVersion = %q, want auto", s.TargetVersion)
	}
	if !s.OdooEnabled {
		t.Error("OdooEnabled = false, want true")
	}
	if s.CacheDir != settings.DefaultCacheDir {
		t.Errorf("CacheDir = %q, want %q", s.CacheDir, settings.DefaultCacheDir)
	}
	if s.Select != nil {
		t.Errorf("Select = %v, want nil (default set)", s.Select)
	}
}

func TestKey_OrderInsensitive(t *testing.T) {
	a := settings.Default()
	a.Select = []string{"O004", "X001"}
	a.LineLength = 100

	b := settings.Default()
	b.Select = []string{"X001", "O004"}
	b.LineLength = 100

	if a.Key() != b.Key() {
		t.Errorf("keys differ for reordered selectors:\n%s\n%s", a.Key(), b.Key())
	}
}

func TestKey_DistinguishesSettings(t *testing.T) {
	a := settings.Default()
	a.LineLength = 100
	b := settings.Default()
	b.LineLength = 88
	if a.Key() == b.Key() {
		t.Error("keys should differ when line length differs")
	}

	c := settings.Default()
	c.ModulesPath = []string{"a", "b"}
	d := settings.Default()
	d.ModulesPath = []string{"b", "a"}
	if c.Key() != d.Key() {
		t.Error("keys should not depend on modules-path order")
	}
}

func TestKey_NilSafe(t *testing.T) {
	var s *settings.Settings
	if got := s.Key(); got != "" {
		t.Errorf("nil Key = %q, want empty", got)
	}
}
