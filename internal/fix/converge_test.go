package fix_test

import (
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/fix"
	"github.com/ppreeper/ruxx/internal/settings"
)

// TestApply_OverlappingRulesConverge covers two style rules that both rewrite
// the same start tag (O019 reorders id/model, ST003 sorts attributes). The
// engine must drop the redundant overlap instead of aborting the pass.
func TestApply_OverlappingRulesConverge(t *testing.T) {
	src := "<?xml version=\"1.0\"?>\n<odoo>\n    <record model=\"ir.ui.view\" id=\"a\"/>\n</odoo>\n"
	s := settings.Default()
	s.Select = []string{"O019", "ST003"}
	res := fix.Apply(src, s, fix.Options{})
	if res.Reverted {
		t.Fatal("unexpected revert")
	}
	want := `    <record id="a" model="ir.ui.view"/>`
	if !strings.Contains(res.Source, want) {
		t.Fatalf("got:\n%s\nwant to contain %q", res.Source, want)
	}
	if again := fix.Apply(res.Source, s, fix.Options{}); again.Source != res.Source {
		t.Fatalf("not idempotent:\n%q\n%q", res.Source, again.Source)
	}
}
