package rules_test

import (
	"errors"
	"testing"

	"github.com/ppreeper/ruxx/internal/rules"
)

func TestRegistry_ByCode(t *testing.T) {
	r, ok := rules.ByCode("O004")
	if !ok {
		t.Fatal("O004 not found")
	}
	if r.Name != "record-missing-model" {
		t.Errorf("O004 name = %q, want record-missing-model", r.Name)
	}
	if r.Family != "O" {
		t.Errorf("O004 family = %q, want O", r.Family)
	}
	if !r.DefaultOn() {
		t.Error("O004 should be default-on")
	}
	if _, ok := rules.ByCode("ZZZ999"); ok {
		t.Error("ZZZ999 should not exist")
	}
}

func TestRegistry_Families(t *testing.T) {
	cases := map[string]string{
		"X001":   "X",
		"O004":   "O",
		"QW006":  "QW",
		"XP001":  "XP",
		"VW001":  "VW",
		"SEC001": "SEC",
		"ST001":  "ST",
		"RUX100": "RUX",
	}
	for code, want := range cases {
		r, ok := rules.ByCode(code)
		if !ok {
			t.Fatalf("%s not found", code)
		}
		if r.Family != want {
			t.Errorf("%s family = %q, want %q", code, r.Family, want)
		}
	}
}

func TestResolve_ExactCode(t *testing.T) {
	set, err := rules.Resolve([]string{"O004"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !set.Enabled("O004") {
		t.Error("O004 should be enabled")
	}
	if set.Enabled("O005") {
		t.Error("O005 should not be enabled")
	}
	if got := set.Codes(); len(got) != 1 || got[0] != "O004" {
		t.Errorf("Codes() = %v, want [O004]", got)
	}
}

func TestResolve_Family(t *testing.T) {
	set, err := rules.Resolve([]string{"O"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !set.Enabled("O001") || !set.Enabled("O022") {
		t.Error("all O rules should be enabled")
	}
	if set.Enabled("X001") {
		t.Error("X rules should not be enabled")
	}
}

func TestResolve_Prefix(t *testing.T) {
	set, err := rules.Resolve([]string{"XP0"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !set.Enabled("XP001") || !set.Enabled("XP009") {
		t.Error("XP001..XP009 should be enabled")
	}
	if set.Enabled("XP010") {
		t.Error("XP010 should not match prefix XP0")
	}
}

func TestResolve_All(t *testing.T) {
	set, err := rules.Resolve([]string{"ALL"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rules.All() {
		if !set.Enabled(r.Code) {
			t.Errorf("ALL should enable %s", r.Code)
		}
	}
	if len(set.Codes()) != len(rules.All()) {
		t.Errorf("Codes() len = %d, want %d", len(set.Codes()), len(rules.All()))
	}
}

func TestResolve_Ignore(t *testing.T) {
	set, err := rules.Resolve([]string{"ALL"}, []string{"O004"})
	if err != nil {
		t.Fatal(err)
	}
	if set.Enabled("O004") {
		t.Error("O004 should be ignored")
	}
	if !set.Enabled("O005") {
		t.Error("O005 should remain enabled")
	}
}

func TestResolve_Default(t *testing.T) {
	set, err := rules.Resolve(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !set.Enabled("O004") {
		t.Error("O004 should be default-on")
	}
	if !set.Enabled("X001") {
		t.Error("X001 should be default-on")
	}
	if set.Enabled("O002") {
		t.Error("O002 should be off by default")
	}
	if set.Enabled("ST001") {
		t.Error("ST001 should be off by default")
	}
}

func TestResolve_Category(t *testing.T) {
	set, err := rules.Resolve([]string{"correctness"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !set.Enabled("O004") || !set.Enabled("XP001") {
		t.Error("correctness rules should be enabled")
	}
	if set.Enabled("O002") || set.Enabled("ST001") {
		t.Error("non-correctness rules should not be enabled")
	}
	// correctness must equal the default set
	def, _ := rules.Resolve(nil, nil)
	if len(set.Codes()) != len(def.Codes()) {
		t.Errorf("correctness set (%d) != default set (%d)", len(set.Codes()), len(def.Codes()))
	}
}

func TestResolve_IgnoreCategory(t *testing.T) {
	set, err := rules.Resolve([]string{"ALL"}, []string{"style"})
	if err != nil {
		t.Fatal(err)
	}
	if set.Enabled("ST001") || set.Enabled("X002") {
		t.Error("style rules should be ignored")
	}
	if !set.Enabled("O004") {
		t.Error("correctness rules should remain")
	}
}

func TestResolve_UnknownSelector(t *testing.T) {
	for _, sel := range []string{"ZZ999", "bogus", "Q"} {
		_, err := rules.Resolve([]string{sel}, nil)
		if err == nil {
			t.Errorf("Resolve(%q) should error", sel)
			continue
		}
		var uerr *rules.UnknownSelectorError
		if !errors.As(err, &uerr) {
			t.Errorf("Resolve(%q) error type = %T, want *UnknownSelectorError", sel, err)
		}
	}
	if _, err := rules.Resolve(nil, []string{"ZZ999"}); err == nil {
		t.Error("unknown ignore selector should error")
	}
}
