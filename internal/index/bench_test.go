package index_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/index"
)

const (
	benchModules = 12
	benchFiles   = 25
)

// benchTree writes a synthetic project: benchModules modules, each with a
// manifest listing benchFiles view files that define one xmlid apiece.
func benchTree(b *testing.B) string {
	b.Helper()
	root := b.TempDir()
	for m := 0; m < benchModules; m++ {
		mod := filepath.Join(root, fmt.Sprintf("mod_%02d", m))
		views := filepath.Join(mod, "views")
		if err := os.MkdirAll(views, 0o755); err != nil {
			b.Fatal(err)
		}
		var data []string
		for f := 0; f < benchFiles; f++ {
			name := fmt.Sprintf("v%02d.xml", f)
			data = append(data, "views/"+name)
			src := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<odoo>
    <record id="view_%02d_%02d" model="ir.ui.view">
        <field name="name">view %02d %02d</field>
        <field name="model">res.partner</field>
        <field name="arch" type="xml"><form/></field>
    </record>
</odoo>
`, m, f, m, f)
			if err := os.WriteFile(filepath.Join(views, name), []byte(src), 0o644); err != nil {
				b.Fatal(err)
			}
		}
		var q strings.Builder
		fmt.Fprintf(&q, "{\n    \"name\": \"mod_%02d\",\n    \"data\": [\n", m)
		for _, d := range data {
			fmt.Fprintf(&q, "        %q,\n", d)
		}
		q.WriteString("    ],\n}\n")
		if err := os.WriteFile(filepath.Join(mod, "__manifest__.py"), []byte(q.String()), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	return root
}

func BenchmarkBuild(b *testing.B) {
	root := benchTree(b)
	roots := []string{root}

	b.Run("cold", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := index.BuildWith(roots, &index.Options{}); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("cold-single-worker", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := index.BuildWith(roots, &index.Options{Workers: 1}); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("cached", func(b *testing.B) {
		dir := b.TempDir()
		if _, err := index.BuildWith(roots, &index.Options{CacheDir: dir}); err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := index.BuildWith(roots, &index.Options{CacheDir: dir}); err != nil {
				b.Fatal(err)
			}
		}
	})
}
