package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ppreeper/ruxx/internal/config"
	"github.com/ppreeper/ruxx/internal/settings"
)

// write creates a file (and parents) under dir and returns its path.
func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestDiscover_NoConfig(t *testing.T) {
	dir := t.TempDir()
	file := write(t, dir, "a.xml", "<x/>")

	c, err := config.Discover(file)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if c.Path != "" {
		t.Fatalf("Path = %q, want empty (defaults)", c.Path)
	}
	if c.Settings.LineLength != settings.DefaultLineLength {
		t.Fatalf("LineLength = %d, want %d", c.Settings.LineLength, settings.DefaultLineLength)
	}
}

func TestDiscover_NearestWins(t *testing.T) {
	base := t.TempDir()
	write(t, base, ".ruxx.toml", "line-length = 100\n")
	write(t, filepath.Join(base, "sub"), ".ruxx.toml", "line-length = 120\n")

	outer := write(t, base, "a.xml", "<x/>")
	inner := write(t, filepath.Join(base, "sub"), "b.xml", "<x/>")

	co, err := config.Discover(outer)
	if err != nil {
		t.Fatal(err)
	}
	if co.Settings.LineLength != 100 {
		t.Fatalf("outer LineLength = %d, want 100", co.Settings.LineLength)
	}

	ci, err := config.Discover(inner)
	if err != nil {
		t.Fatal(err)
	}
	if ci.Settings.LineLength != 120 {
		t.Fatalf("inner LineLength = %d, want 120", ci.Settings.LineLength)
	}
}

func TestDiscover_FileNamePrecedence(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".ruxx.toml", "line-length = 111\n")
	write(t, dir, "ruxx.toml", "line-length = 222\n")
	file := write(t, dir, "a.xml", "<x/>")

	c, err := config.Discover(file)
	if err != nil {
		t.Fatal(err)
	}
	if c.Settings.LineLength != 111 {
		t.Fatalf("LineLength = %d, want 111 (.ruxx.toml wins over ruxx.toml)", c.Settings.LineLength)
	}
}

func TestDiscover_ParentFallback(t *testing.T) {
	base := t.TempDir()
	write(t, base, "ruxx.toml", "line-length = 90\n")
	file := write(t, filepath.Join(base, "deep", "nested"), "a.xml", "<x/>")

	c, err := config.Discover(file)
	if err != nil {
		t.Fatal(err)
	}
	if c.Settings.LineLength != 90 {
		t.Fatalf("LineLength = %d, want 90 (inherited from parent)", c.Settings.LineLength)
	}
}

func TestLoad_TopLevel(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, ".ruxx.toml", "line-length = 100\nindent-width = 2\ntarget-version = \"17.0\"\n")

	c, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Settings.LineLength != 100 {
		t.Errorf("LineLength = %d, want 100", c.Settings.LineLength)
	}
	if c.Settings.IndentWidth != 2 {
		t.Errorf("IndentWidth = %d, want 2", c.Settings.IndentWidth)
	}
	if c.Settings.TargetVersion != "17.0" {
		t.Errorf("TargetVersion = %q, want 17.0", c.Settings.TargetVersion)
	}
}

func TestLoad_LintSelectIgnore(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, ".ruxx.toml", "[lint]\nselect = [\"O\", \"X\"]\nignore = [\"O004\"]\n")

	c, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Settings.Select, []string{"O", "X"}) {
		t.Errorf("Select = %v, want [O X]", c.Settings.Select)
	}
	if !reflect.DeepEqual(c.Settings.Ignore, []string{"O004"}) {
		t.Errorf("Ignore = %v, want [O004]", c.Settings.Ignore)
	}
}

func TestLoad_Extend(t *testing.T) {
	base := t.TempDir()
	write(t, base, "ruxx.toml", "line-length = 100\n")
	sub := filepath.Join(base, "sub")
	p := write(t, sub, ".ruxx.toml", "extend = \"../ruxx.toml\"\nindent-width = 2\n")

	c, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Settings.LineLength != 100 {
		t.Errorf("LineLength = %d, want 100 (inherited via extend)", c.Settings.LineLength)
	}
	if c.Settings.IndentWidth != 2 {
		t.Errorf("IndentWidth = %d, want 2 (local override)", c.Settings.IndentWidth)
	}
	if want := filepath.Join(base, "ruxx.toml"); c.Extends != want {
		t.Errorf("Extends = %q, want %q", c.Extends, want)
	}
}

func TestLoad_PerFileIgnores(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, ".ruxx.toml", "[lint.per-file-ignores]\n\"**/odoo/**\" = [\"X010\"]\n\"views/*.xml\" = [\"O004\", \"O005\"]\n")

	c, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"**/odoo/**":  {"X010"},
		"views/*.xml": {"O004", "O005"},
	}
	if !reflect.DeepEqual(c.PerFileIgnores, want) {
		t.Errorf("PerFileIgnores = %v, want %v", c.PerFileIgnores, want)
	}
}

func TestLoad_Overrides(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, ".ruxx.toml", "[[overrides]]\ninclude = [\"**/views/*.xml\"]\nselect = [\"VW001\"]\n")

	c, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Overrides) != 1 {
		t.Fatalf("len(Overrides) = %d, want 1", len(c.Overrides))
	}
	ov := c.Overrides[0]
	if !reflect.DeepEqual(ov.Include, []string{"**/views/*.xml"}) {
		t.Errorf("Include = %v, want [**/views/*.xml]", ov.Include)
	}
	if ov.Settings == nil || !reflect.DeepEqual(ov.Settings.Select, []string{"VW001"}) {
		t.Errorf("override Select = %v, want [VW001]", ov.Settings)
	}
}

func TestSettingsFor_Override(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, ".ruxx.toml", "[lint]\nselect = [\"X\"]\n\n[[overrides]]\ninclude = [\"**/views/*.xml\"]\nselect = [\"O004\"]\n")

	c, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}

	// A file matching the override gets the override's select.
	views := filepath.Join(dir, "views", "a.xml")
	if got := c.SettingsFor(views).Select; !reflect.DeepEqual(got, []string{"O004"}) {
		t.Errorf("SettingsFor(views).Select = %v, want [O004]", got)
	}

	// A file not matching keeps the base select.
	other := filepath.Join(dir, "data", "b.xml")
	if got := c.SettingsFor(other).Select; !reflect.DeepEqual(got, []string{"X"}) {
		t.Errorf("SettingsFor(other).Select = %v, want [X]", got)
	}
}
