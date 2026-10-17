package cache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ppreeper/ruxx/internal/cache"
	"github.com/ppreeper/ruxx/internal/diagnostic"
	"github.com/ppreeper/ruxx/internal/source"
)

func TestOpen_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", ".ruxx_cache")
	c, err := cache.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if c.Dir() != dir {
		t.Errorf("Dir = %q, want %q", c.Dir(), dir)
	}
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		t.Fatalf("cache dir not created: %v", err)
	}
}

func TestPutGet_RoundTrip(t *testing.T) {
	c, err := cache.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("<odoo>\n</odoo>\n")
	key := "sel=O004;ll=88"
	diags := []diagnostic.Diagnostic{{
		Code:    "O004",
		Name:    "record-missing-model",
		Message: "boom",
		Range:   source.NewRange(1, 2),
	}}

	if err := c.Put(content, key, diags); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok := c.Get(content, key)
	if !ok {
		t.Fatal("expected a cache hit")
	}
	if len(got) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(got))
	}
	if got[0].Code != "O004" || got[0].Message != "boom" || got[0].Range.Start != 1 || got[0].Range.End != 2 {
		t.Errorf("round-trip mismatch: %+v", got[0])
	}
}

func TestGet_Absent(t *testing.T) {
	c, err := cache.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get([]byte("<a/>"), "key"); ok {
		t.Error("expected a miss for an absent entry")
	}
}

func TestGet_MissOnDifferentContent(t *testing.T) {
	c, err := cache.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Put([]byte("<a/>"), "key", nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get([]byte("<b/>"), "key"); ok {
		t.Error("expected a miss when content differs")
	}
}

func TestGet_MissOnDifferentSettings(t *testing.T) {
	c, err := cache.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Put([]byte("<a/>"), "ll=88", nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get([]byte("<a/>"), "ll=100"); ok {
		t.Error("expected a miss when settings differ")
	}
}

func TestPutGet_EmptyEligibleResult(t *testing.T) {
	c, err := cache.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Put([]byte("<a/>"), "key", []diagnostic.Diagnostic{}); err != nil {
		t.Fatal(err)
	}
	got, ok := c.Get([]byte("<a/>"), "key")
	if !ok {
		t.Fatal("expected a hit for a cached clean file")
	}
	if len(got) != 0 {
		t.Errorf("got %d diagnostics, want 0", len(got))
	}
}

func TestClean_RemovesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".ruxx_cache")
	c, err := cache.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Put([]byte("<a/>"), "key", nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Clean(); err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("cache dir still present after Clean: %v", err)
	}
	if _, ok := c.Get([]byte("<a/>"), "key"); ok {
		t.Error("expected a miss after Clean")
	}
}

func TestClean_AbsentDirIsNotAnError(t *testing.T) {
	c, err := cache.Open(filepath.Join(t.TempDir(), "created", "cache"))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Clean(); err != nil {
		t.Fatalf("Clean on a fresh dir: %v", err)
	}
	if err := c.Clean(); err != nil {
		t.Fatalf("second Clean: %v", err)
	}
}
