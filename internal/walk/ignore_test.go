package walk_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ppreeper/ruxx/internal/walk"
)

// TestMain pins XDG_CONFIG_HOME so the developer's real
// ~/.config/git/ignore can never influence these tests.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ruxx-walk-xdg")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("XDG_CONFIG_HOME", dir); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestCollect_DotIgnoreWinsOverGitignore(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "a.xml"), "<odoo/>")
	touch(t, filepath.Join(dir, "b.xml"), "<odoo/>")
	touch(t, filepath.Join(dir, ".gitignore"), "a.xml\nb.xml\n")
	// .ignore takes precedence: it un-ignores a.xml and ignores c.xml only.
	touch(t, filepath.Join(dir, ".ignore"), "!a.xml\n")

	got, err := walk.Collect([]string{dir}, opts())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.xml"}
	if have := rel(t, dir, got); !reflect.DeepEqual(have, want) {
		t.Errorf("files = %v, want %v", have, want)
	}
}

func TestCollect_DotIgnoreDirectoryRule(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "keep.xml"), "<odoo/>")
	touch(t, filepath.Join(dir, "skip", "x.xml"), "<odoo/>")
	touch(t, filepath.Join(dir, ".ignore"), "skip/\n")

	got, err := walk.Collect([]string{dir}, opts())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"keep.xml"}
	if have := rel(t, dir, got); !reflect.DeepEqual(have, want) {
		t.Errorf("files = %v, want %v", have, want)
	}
}

func TestCollect_RepoInfoExclude(t *testing.T) {
	repo := t.TempDir()
	touch(t, filepath.Join(repo, ".git", "info", "exclude"), "gen/\n")
	root := filepath.Join(repo, "sub")
	touch(t, filepath.Join(root, "a.xml"), "<odoo/>")
	touch(t, filepath.Join(root, "gen", "x.xml"), "<odoo/>")

	got, err := walk.Collect([]string{root}, opts())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.xml"}
	if have := rel(t, root, got); !reflect.DeepEqual(have, want) {
		t.Errorf("files = %v, want %v", have, want)
	}
}

func TestCollect_GlobalGitignore(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	touch(t, filepath.Join(xdg, "git", "ignore"), "vendor/\n")

	dir := t.TempDir()
	touch(t, filepath.Join(dir, "a.xml"), "<odoo/>")
	touch(t, filepath.Join(dir, "vendor", "x.xml"), "<odoo/>")

	got, err := walk.Collect([]string{dir}, opts())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.xml"}
	if have := rel(t, dir, got); !reflect.DeepEqual(have, want) {
		t.Errorf("files = %v, want %v", have, want)
	}
}

func TestCollect_AboveRootDoesNotLeak(t *testing.T) {
	parent := t.TempDir()
	touch(t, filepath.Join(parent, ".gitignore"), "a.xml\n")
	root := filepath.Join(parent, "sub")
	touch(t, filepath.Join(root, "a.xml"), "<odoo/>")

	got, err := walk.Collect([]string{root}, opts())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.xml"}
	if have := rel(t, root, got); !reflect.DeepEqual(have, want) {
		t.Errorf("files = %v, want %v", have, want)
	}
}

func TestCollect_IgnoreFilesDisabled(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "a.xml"), "<odoo/>")
	touch(t, filepath.Join(dir, ".ignore"), "a.xml\n")

	o := opts()
	o.RespectGitignore = false
	got, err := walk.Collect([]string{dir}, o)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.xml"}
	if have := rel(t, dir, got); !reflect.DeepEqual(have, want) {
		t.Errorf("files = %v, want %v", have, want)
	}
}
