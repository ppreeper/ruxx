package formatter_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/formatter"
	"github.com/ppreeper/ruxx/internal/settings"
)

// benchUnformatted builds a one-line-per-element Odoo file: the formatter has
// real work to do (nesting, attribute layout, tag spacing) on every run.
func benchUnformatted(records int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><odoo>`)
	for i := 0; i < records; i++ {
		fmt.Fprintf(&b, `<record id="view_%d" model="ir.ui.view"><field name="name">partner form %d</field><field name="model">res.partner</field><field name="arch" type="xml"><form string="Partner"><sheet><group><field name="name" widget="char" readonly="1" options='{"no_open": True}'/><field name="email" placeholder="Email"/></group></sheet></form></field></record>`, i, i)
	}
	b.WriteString(`</odoo>`)
	return b.String()
}

func BenchmarkFormat(b *testing.B) {
	src := benchUnformatted(50)
	s := settings.Default()
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := formatter.Format(src, s); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFormatIdempotent(b *testing.B) {
	s := settings.Default()
	src, err := formatter.Format(benchUnformatted(50), s)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := formatter.Format(src, s); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFormatAttributeSort(b *testing.B) {
	src := benchUnformatted(50)
	s := settings.Default()
	s.AttributeSort = "alphabetical"
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := formatter.Format(src, s); err != nil {
			b.Fatal(err)
		}
	}
}
