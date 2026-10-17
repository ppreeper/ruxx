package rules_test

import (
	"testing"

	"github.com/ppreeper/ruxx/internal/diagnostic"
	"github.com/ppreeper/ruxx/internal/rules"
)

// TestFixApplicability_Count guards the fix table against silent drift: it must
// keep naming rules that actually exist in the catalog.
func TestFixApplicability_Count(t *testing.T) {
	n := 0
	for _, r := range rules.All() {
		if _, ok := rules.FixApplicability(r.Code); ok {
			n++
		}
	}
	if n != 24 {
		t.Fatalf("%d catalogued rules carry a fix, want 24", n)
	}
}

func TestFixApplicability_KnownEntries(t *testing.T) {
	cases := []struct {
		code string
		want diagnostic.Applicability
		ok   bool
	}{
		{"X002", diagnostic.ApplicabilitySafe, true},
		{"X007", diagnostic.ApplicabilitySafe, true},
		{"O002", diagnostic.ApplicabilitySafe, true},
		{"O019", diagnostic.ApplicabilitySafe, true},
		{"ST002", diagnostic.ApplicabilitySafe, true},
		{"XP004", diagnostic.ApplicabilitySafe, true},
		{"X014", diagnostic.ApplicabilityUnsafe, true},
		{"QW001", diagnostic.ApplicabilityUnsafe, true},
		{"O004", 0, false},
		{"X001", 0, false},
		{"ZZ999", 0, false},
	}
	for _, c := range cases {
		got, ok := rules.FixApplicability(c.code)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("FixApplicability(%q) = (%v, %v), want (%v, %v)", c.code, got, ok, c.want, c.ok)
		}
	}
}
