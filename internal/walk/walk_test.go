package walk_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ppreeper/ruxx/internal/walk"
)

func touch(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rel(t *testing.T, root string, files []string) []string {
	t.Helper()
	out := make([]string, 0, len(files))
	for _, f := range files {
		r, err := filepath.Rel(root, f)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(r))
	}
	return out
}

func opts() walk.Options {
	return walk.Options{
		Include:          walk.DefaultInclude,
		Exclude:          walk.DefaultExclude,
		RespectGitignore: true,
	}
}

func TestCollect_Defaults(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "a.xml"), "<a/>")
	touch(t, filepath.Join(root, "b.qweb"), "<b/>")
	touch(t, filepath.Join(root, "c.txt"), "nope")
	touch(t, filepath.Join(root, "sub", "d.xml"), "<d/>")

	got, err := walk.Collect([]string{root}, opts())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.xml", "b.qweb", "sub/d.xml"}
	if r := rel(t, root, got); !reflect.DeepEqual(r, want) {
		t.Fatalf("got %v, want %v", r, want)
	}
}

func TestCollect_ExcludeBareName(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "a.xml"), "<a/>")
	touch(t, filepath.Join(root, "vendor", "e.xml"), "<e/>")
	touch(t, filepath.Join(root, "node_modules", "n.xml"), "<n/>")

	o := opts()
	o.Exclude = append(append([]string{}, walk.DefaultExclude...), "vendor")
	got, err := walk.Collect([]string{root}, o)
	if err != nil {
		t.Fatal(err)
	}
	if r := rel(t, root, got); !reflect.DeepEqual(r, []string{"a.xml"}) {
		t.Fatalf("got %v, want [a.xml]", r)
	}
}

func TestCollect_ExcludeDoubleStar(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "a.xml"), "<a/>")
	touch(t, filepath.Join(root, "deep", "build", "f.xml"), "<f/>")

	o := opts()
	o.Exclude = append(append([]string{}, walk.DefaultExclude...), "**/build/**")
	got, err := walk.Collect([]string{root}, o)
	if err != nil {
		t.Fatal(err)
	}
	if r := rel(t, root, got); !reflect.DeepEqual(r, []string{"a.xml"}) {
		t.Fatalf("got %v, want [a.xml]", r)
	}
}

func TestCollect_ExplicitFileBypassesInclude(t *testing.T) {
	root := t.TempDir()
	txt := filepath.Join(root, "c.txt")
	touch(t, txt, "nope")
	touch(t, filepath.Join(root, "a.xml"), "<a/>")

	got, err := walk.Collect([]string{txt}, opts())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != txt {
		t.Fatalf("got %v, want [%s]", got, txt)
	}
}

func TestCollect_ForceExcludeOnExplicit(t *testing.T) {
	root := t.TempDir()
	txt := filepath.Join(root, "c.txt")
	touch(t, txt, "nope")

	o := opts()
	o.Exclude = []string{"*.txt"}
	o.ForceExclude = true
	got, err := walk.Collect([]string{txt}, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want none", got)
	}
}

func TestCollect_Gitignore(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, ".gitignore"), "ignored.xml\n/root_only.xml\n!keep.xml\nsub/\n")
	touch(t, filepath.Join(root, "ignored.xml"), "<i/>")
	touch(t, filepath.Join(root, "keep.xml"), "<k/>")
	touch(t, filepath.Join(root, "root_only.xml"), "<r/>")
	touch(t, filepath.Join(root, "a.xml"), "<a/>")
	touch(t, filepath.Join(root, "sub", "x.xml"), "<x/>")

	got, err := walk.Collect([]string{root}, opts())
	if err != nil {
		t.Fatal(err)
	}
	if r := rel(t, root, got); !reflect.DeepEqual(r, []string{"a.xml", "keep.xml"}) {
		t.Fatalf("got %v, want [a.xml keep.xml]", r)
	}
}

func TestCollect_GitignoreDisabled(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, ".gitignore"), "ignored.xml\n")
	touch(t, filepath.Join(root, "ignored.xml"), "<i/>")

	o := opts()
	o.RespectGitignore = false
	got, err := walk.Collect([]string{root}, o)
	if err != nil {
		t.Fatal(err)
	}
	if r := rel(t, root, got); !reflect.DeepEqual(r, []string{"ignored.xml"}) {
		t.Fatalf("got %v, want [ignored.xml]", r)
	}
}

func TestCollect_NestedGitignoreWins(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, ".gitignore"), "x.xml\n")
	touch(t, filepath.Join(root, "sub", ".gitignore"), "!x.xml\n")
	touch(t, filepath.Join(root, "sub", "x.xml"), "<x/>")

	got, err := walk.Collect([]string{root}, opts())
	if err != nil {
		t.Fatal(err)
	}
	if r := rel(t, root, got); !reflect.DeepEqual(r, []string{"sub/x.xml"}) {
		t.Fatalf("got %v, want [sub/x.xml]", r)
	}
}

func TestCollect_SkipsGitDir(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, ".git", "config.xml"), "<c/>")
	touch(t, filepath.Join(root, "a.xml"), "<a/>")

	got, err := walk.Collect([]string{root}, opts())
	if err != nil {
		t.Fatal(err)
	}
	if r := rel(t, root, got); !reflect.DeepEqual(r, []string{"a.xml"}) {
		t.Fatalf("got %v, want [a.xml]", r)
	}
}

func TestCollect_SortedAndDeduped(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "b.xml"), "<b/>")
	touch(t, filepath.Join(root, "a.xml"), "<a/>")

	got, err := walk.Collect([]string{root, root}, opts())
	if err != nil {
		t.Fatal(err)
	}
	if r := rel(t, root, got); !reflect.DeepEqual(r, []string{"a.xml", "b.xml"}) {
		t.Fatalf("got %v, want [a.xml b.xml]", r)
	}
}

func TestCollect_MissingRoot(t *testing.T) {
	if _, err := walk.Collect([]string{filepath.Join(t.TempDir(), "nope")}, opts()); err == nil {
		t.Fatal("expected an error for a missing root")
	}
}
