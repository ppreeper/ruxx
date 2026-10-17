package syntax_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/syntax"
)

// benchDoc builds a synthetic Odoo data file: one form view per record, with
// nested fields, mixed attribute quoting, and a CDATA-free body. It is meant
// to be representative of the files ruxx is pointed at, not to be minimal.
func benchDoc(records int) string {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<odoo>\n")
	for i := 0; i < records; i++ {
		fmt.Fprintf(&b, `    <record id="view_%d" model="ir.ui.view">
        <field name="name">partner form %d</field>
        <field name="model">res.partner</field>
        <field name="arch" type="xml">
            <form string="Partner">
                <sheet>
                    <group>
                        <field name="name" widget="char" readonly="1" options='{"no_open": True}'/>
                        <field name="email" placeholder="Email"/>
                        <field name="partner_id" domain="[('active', '=', True)]"/>
                    </group>
                </sheet>
            </form>
        </field>
    </record>
`, i, i)
	}
	b.WriteString("</odoo>\n")
	return b.String()
}

func benchmarkParse(b *testing.B, records int) {
	src := benchDoc(records)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = syntax.Parse(src)
	}
}

func BenchmarkParse(b *testing.B)      { benchmarkParse(b, 50) }
func BenchmarkParseLarge(b *testing.B) { benchmarkParse(b, 1000) }

func BenchmarkUnparse(b *testing.B) {
	src := benchDoc(50)
	tree := syntax.Parse(src)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = syntax.Unparse(tree)
	}
}

func BenchmarkRoundTrip(b *testing.B) {
	src := benchDoc(50)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if syntax.Unparse(syntax.Parse(src)) != src {
			b.Fatal("round trip lost data")
		}
	}
}
