package index_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ppreeper/ruxx/internal/index"
)

// viewSrc returns a one-record view whose id is exactly three bytes long, so a
// replacement keeps the file size identical.
func viewSrc(id string) string {
	return `<?xml version="1.0"?>
<odoo>
    <record id="` + id + `" model="ir.ui.view"/>
</odoo>
`
}

// dump renders an index into a comparable, order-independent string.
func dump(ix *index.Index) string {
	var b strings.Builder
	for _, m := range ix.Modules() {
		fmt.Fprintf(&b, "%s|%s\n", m.Name, m.Dir)
		for _, id := range sortedKeys(m.Defs) {
			files := append([]string(nil), m.Defs[id]...)
			sort.Strings(files)
			for _, f := range files {
				fmt.Fprintf(&b, "  def %s=%s\n", id, filepath.Base(f))
			}
		}
		for _, id := range sortedKeys(m.Groups) {
			fmt.Fprintf(&b, "  grp %s\n", id)
		}
	}
	return b.String()
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestBuildWith_WritesCache(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	write(t, filepath.Join(root, "mod", "views"), "v.xml", viewSrc("aaa"))

	if _, err := index.BuildWith([]string{root}, &index.Options{CacheDir: cacheDir}); err != nil {
		t.Fatalf("BuildWith: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "index.json")); err != nil {
		t.Fatalf("cache file not written: %v", err)
	}
}

func TestBuildWith_NoCacheDirWritesNothing(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "mod", "views"), "v.xml", viewSrc("aaa"))

	if _, err := index.BuildWith([]string{root}, &index.Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "index.json")); err == nil {
		t.Error("index.json written without a CacheDir")
	}
}

func TestBuildWith_ReusesUnchangedModule(t *testing.T) {
	root := t.TempDir()
	mod := filepath.Join(root, "mod")
	cacheDir := filepath.Join(t.TempDir(), "cache")
	p := write(t, filepath.Join(mod, "views"), "v.xml", viewSrc("old"))

	ix, err := index.BuildWith([]string{root}, &index.Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatal(err)
	}
	if !ix.HasXMLID("mod.old") {
		t.Fatal("first build missed mod.old")
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}

	// Rewrite the file with a different xmlid but the same size, then restore
	// the modification time: the stamps are unchanged, so the cache is hit and
	// the stale facts are reused.
	if err := os.WriteFile(p, []byte(viewSrc("new")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	ix, err = index.BuildWith([]string{root}, &index.Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatal(err)
	}
	if !ix.HasXMLID("mod.old") || ix.HasXMLID("mod.new") {
		t.Errorf("expected the cached entry to be reused; old=%v new=%v",
			ix.HasXMLID("mod.old"), ix.HasXMLID("mod.new"))
	}

	// Touching the file invalidates it and the new content is parsed.
	now := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(p, now, now); err != nil {
		t.Fatal(err)
	}
	ix, err = index.BuildWith([]string{root}, &index.Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatal(err)
	}
	if ix.HasXMLID("mod.old") || !ix.HasXMLID("mod.new") {
		t.Errorf("expected a rebuild after touch; old=%v new=%v",
			ix.HasXMLID("mod.old"), ix.HasXMLID("mod.new"))
	}
}

func TestBuildWith_InvalidatedByChangedFileSet(t *testing.T) {
	root := t.TempDir()
	mod := filepath.Join(root, "mod")
	cacheDir := filepath.Join(t.TempDir(), "cache")
	write(t, filepath.Join(mod, "views"), "a.xml", viewSrc("aaa"))

	if _, err := index.BuildWith([]string{root}, &index.Options{CacheDir: cacheDir}); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(mod, "views"), "b.xml", viewSrc("bbb"))

	ix, err := index.BuildWith([]string{root}, &index.Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatal(err)
	}
	if !ix.HasXMLID("mod.bbb") {
		t.Error("a newly added file did not invalidate the cached module")
	}
}

func TestBuildWith_CacheIsRootScoped(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	write(t, filepath.Join(a, "mod_a", "views"), "a.xml", viewSrc("aaa"))
	write(t, filepath.Join(b, "mod_b", "views"), "b.xml", viewSrc("bbb"))

	if _, err := index.BuildWith([]string{a}, &index.Options{CacheDir: cacheDir}); err != nil {
		t.Fatal(err)
	}
	ix, err := index.BuildWith([]string{b}, &index.Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatal(err)
	}
	if ix.HasModule("mod_a") || !ix.HasModule("mod_b") {
		t.Errorf("cache leaked across root sets: %v", ix.Modules())
	}
}

func TestBuildWith_CorruptCacheFallsBack(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	write(t, filepath.Join(root, "mod", "views"), "v.xml", viewSrc("aaa"))
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A future/garbled cache version must never be trusted.
	if err := os.WriteFile(filepath.Join(cacheDir, "index.json"), []byte(`{"version":999,"modules":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	ix, err := index.BuildWith([]string{root}, &index.Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatal(err)
	}
	if !ix.HasXMLID("mod.aaa") {
		t.Error("corrupt cache was not rebuilt from source")
	}
}

func TestBuildWith_WorkersAgreeWithSequential(t *testing.T) {
	root := t.TempDir()
	for _, m := range []string{"m1", "m2", "m3"} {
		write(t, filepath.Join(root, m, "views"), "a.xml", viewSrc("a_"+m))
		write(t, filepath.Join(root, m, "views"), "b.xml", viewSrc("b_"+m))
	}
	seq, err := index.BuildWith([]string{root}, &index.Options{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	par, err := index.BuildWith([]string{root}, &index.Options{Workers: 8})
	if err != nil {
		t.Fatal(err)
	}
	if dump(seq) != dump(par) {
		t.Errorf("parallel index differs from sequential:\nseq:\n%s\npar:\n%s", dump(seq), dump(par))
	}
}
