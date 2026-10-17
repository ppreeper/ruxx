package fix_test

import (
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/fix"
	"github.com/ppreeper/ruxx/internal/settings"
)

// trailing whitespace on line 2 -> X007 (safe fix); no other safe fix applies.
const fixX007Src = "<?xml version=\"1.0\"?>\n<odoo>   \n    <record id=\"a\" model=\"m\"/>\n</odoo>\n"

// deprecated root -> O002 (safe fix).
const fixO002Src = "<?xml version=\"1.0\"?>\n<openerp>\n    <record id=\"a\" model=\"m\"/>\n</openerp>\n"

func sel(codes ...string) *settings.Settings {
	s := settings.Default()
	s.Select = codes
	return s
}

func TestApply_FixableRestrictsWhichRulesFix(t *testing.T) {
	// O002 is selected but not fixable: the source must be untouched.
	s := sel("X007", "O002")
	s.Fixable = []string{"X007"}
	res := fix.Apply(fixO002Src, s, fix.Options{})
	if res.Source != fixO002Src {
		t.Errorf("unfixable rule was applied:\n%s", res.Source)
	}
	if len(res.Applied) != 0 {
		t.Errorf("Applied = %v, want none", res.Applied)
	}

	// Now allow only O002: the deprecated root is renamed.
	s = sel("X007", "O002")
	s.Fixable = []string{"O002"}
	res = fix.Apply(fixO002Src, s, fix.Options{})
	if !strings.Contains(res.Source, "<odoo>") || strings.Contains(res.Source, "openerp") {
		t.Errorf("fixable rule was not applied:\n%s", res.Source)
	}
}

func TestApply_FixableByPrefix(t *testing.T) {
	// A family selector is enough to allow a rule.
	s := sel("X007")
	s.Fixable = []string{"X"}
	res := fix.Apply(fixX007Src, s, fix.Options{})
	if strings.Contains(res.Source, "odoo>   ") {
		t.Errorf("X family should be fixable:\n%q", res.Source)
	}
}

func TestApply_UnfixableWinsOverFixable(t *testing.T) {
	s := sel("X007")
	s.Fixable = []string{"X007"}
	s.Unfixable = []string{"X007"}
	res := fix.Apply(fixX007Src, s, fix.Options{})
	if res.Source != fixX007Src {
		t.Errorf("unfixable should win:\n%q", res.Source)
	}
}

func TestApply_UnfixableAloneLeavesOtherRules(t *testing.T) {
	s := sel("X007", "O002")
	s.Unfixable = []string{"O002"}
	res := fix.Apply(fixO002Src+"\n", s, fix.Options{})
	// O002 may not fix, but nothing else should have changed either.
	if strings.Contains(res.Source, "<odoo>") {
		t.Errorf("unfixable rule was applied:\n%s", res.Source)
	}
}
