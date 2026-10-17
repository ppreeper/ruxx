package rules_test

import (
	"testing"

	"github.com/ppreeper/ruxx/internal/linter"
	"github.com/ppreeper/ruxx/internal/settings"
	"github.com/ppreeper/ruxx/internal/syntax"
)

const rux103Src = "<?xml version=\"1.0\"?>\n<odoo>\n    <record id=\"a\" model=\"ir.ui.view\"/>\n</odoo>\n"

func check(t *testing.T, s *settings.Settings) []string {
	t.Helper()
	var codes []string
	for _, d := range linter.Check(syntax.Parse(rux103Src), s) {
		codes = append(codes, d.Code)
	}
	return codes
}

func has(codes []string, want string) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

// TestRUX103_FiresWithStyleRule: enabling a style rule alongside the formatter
// is exactly the conflict RUX103 reports.
func TestRUX103_FiresWithStyleRule(t *testing.T) {
	s := settings.Default()
	s.Select = []string{"ST003", "RUX103"}
	if got := check(t, s); !has(got, "RUX103") {
		t.Fatalf("codes = %v, want RUX103", got)
	}
}

// TestRUX103_SilentByDefault: the default (correctness) set contains no style
// rules, so RUX103 stays quiet.
func TestRUX103_SilentByDefault(t *testing.T) {
	if got := check(t, settings.Default()); has(got, "RUX103") {
		t.Fatalf("codes = %v, RUX103 should not fire by default", got)
	}
}

// TestRUX103_SilentWhenStyleIgnored: style rules must actually be enabled for
// RUX103 to fire, so ignoring the whole category silences it.
func TestRUX103_SilentWhenStyleIgnored(t *testing.T) {
	s := settings.Default()
	s.Select = []string{"ALL"}
	s.Ignore = []string{"style"}
	if got := check(t, s); has(got, "RUX103") {
		t.Fatalf("codes = %v, RUX103 should not fire when style is ignored", got)
	}
}

// TestRUX103_FiresWithAll: selecting ALL enables style rules, so RUX103 fires.
func TestRUX103_FiresWithAll(t *testing.T) {
	s := settings.Default()
	s.Select = []string{"ALL"}
	if got := check(t, s); !has(got, "RUX103") {
		t.Fatalf("codes = %v, want RUX103", got)
	}
}
