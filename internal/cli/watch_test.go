package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSnapshotChanged(t *testing.T) {
	prev := map[string]fileStamp{
		"a.xml": {size: 10, modTime: 100},
		"b.xml": {size: 20, modTime: 200},
		"c.xml": {size: 30, modTime: 300},
	}
	cur := map[string]fileStamp{
		"a.xml": {size: 10, modTime: 100}, // unchanged
		"b.xml": {size: 21, modTime: 200}, // size changed
		"d.xml": {size: 40, modTime: 400}, // added (c removed)
	}
	got := snapshotChanged(prev, cur)
	want := []string{"b.xml", "c.xml", "d.xml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshotChanged = %v, want %v", got, want)
	}
}

func TestSnapshotChanged_NoChanges(t *testing.T) {
	s := map[string]fileStamp{"a.xml": {size: 1, modTime: 2}}
	if got := snapshotChanged(s, s); len(got) != 0 {
		t.Fatalf("snapshotChanged = %v, want empty", got)
	}
}

func TestFileSnapshot(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.xml", "<odoo/>")
	write("b.xml", "<odoo></odoo>")
	write("notes.txt", "ignored")

	// "-" (stdin) is skipped; only .xml files under dirs are collected.
	got := fileSnapshot([]string{"-", dir}, &options{})

	if _, ok := got["-"]; ok {
		t.Fatalf("fileSnapshot should skip \"-\"")
	}
	want := []string{filepath.Join(dir, "a.xml"), filepath.Join(dir, "b.xml")}
	for _, p := range want {
		st, ok := got[p]
		if !ok {
			t.Fatalf("fileSnapshot missing %q (got %v)", p, got)
		}
		if st.size == 0 {
			t.Fatalf("fileSnapshot %q has zero size", p)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("fileSnapshot has %d entries, want %d (%v)", len(got), len(want), got)
	}
}
