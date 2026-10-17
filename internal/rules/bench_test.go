package rules_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/rules"
	"github.com/ppreeper/ruxx/internal/settings"
	"github.com/ppreeper/ruxx/internal/syntax"
)

// benchView builds a synthetic view file whose arch exercises the Odoo, QWeb,
// XPath and view-structure families (the ones that walk the element tree).
func benchView(records int) string {
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
                        <field name="name" widget="char" readonly="1"/>
                        <field name="email" placeholder="Email"/>
                        <xpath expr="//field[@name='email']" position="after">
                            <field name="phone"/>
                        </xpath>
                    </group>
                    <notebook>
                        <page string="Sales">
                            <field name="sale_order_count"/>
                        </page>
                    </notebook>
                </sheet>
            </form>
        </field>
    </record>
`, i, i)
	}
	b.WriteString("</odoo>\n")
	return b.String()
}

func BenchmarkCheck(b *testing.B) {
	tree := syntax.Parse(benchView(50))
	s := settings.Default()
	s.Select = []string{"ALL"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rules.Check(tree, s)
	}
}

func BenchmarkCheckDefault(b *testing.B) {
	tree := syntax.Parse(benchView(50))
	s := settings.Default()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rules.Check(tree, s)
	}
}
