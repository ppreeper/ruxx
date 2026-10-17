package cli_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

const benchFileCount = 50

func benchDir(b *testing.B, contents string) string {
	b.Helper()
	dir := b.TempDir()
	for i := 0; i < benchFileCount; i++ {
		p := filepath.Join(dir, fmt.Sprintf("f%02d.xml", i))
		if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	return dir
}

// BenchmarkCheckFiles measures the whole `ruxx check` pipeline (discovery,
// parse, lint, render) over 50 files, skipping the cache so every iteration
// does the work.
func BenchmarkCheckFiles(b *testing.B) {
	dir := benchDir(b, badO004)
	args := []string{"check", "-n", dir}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if code := cli.Run(args, strings.NewReader(""), io.Discard, io.Discard); code != cli.ExitFindings {
			b.Fatalf("exit = %d, want %d", code, cli.ExitFindings)
		}
	}
}

// BenchmarkCheckFilesCached runs the same selection twice per iteration: the
// first pass fills the cache, the second is served from it.
func BenchmarkCheckFilesCached(b *testing.B) {
	dir := benchDir(b, badO004)
	cacheDir := b.TempDir()
	warm := []string{"check", "--cache-dir", cacheDir, dir}
	cached := append([]string{}, warm...)
	if code := cli.Run(warm, strings.NewReader(""), io.Discard, io.Discard); code != cli.ExitFindings {
		b.Fatalf("warm exit = %d, want %d", code, cli.ExitFindings)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if code := cli.Run(cached, strings.NewReader(""), io.Discard, io.Discard); code != cli.ExitFindings {
			b.Fatalf("exit = %d, want %d", code, cli.ExitFindings)
		}
	}
}

// BenchmarkFormatCheckFiles measures `ruxx format --check` over 50 files that
// all need reformatting.
func BenchmarkFormatCheckFiles(b *testing.B) {
	dir := benchDir(b, "<?xml version=\"1.0\"?><odoo><record id=\"a\" model=\"ir.ui.view\"><field name=\"name\">A</field></record></odoo>")
	args := []string{"format", "--check", "-n", dir}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if code := cli.Run(args, strings.NewReader(""), io.Discard, io.Discard); code != cli.ExitFindings {
			b.Fatalf("exit = %d, want %d", code, cli.ExitFindings)
		}
	}
}
