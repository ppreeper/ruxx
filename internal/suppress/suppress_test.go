package suppress_test

import (
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/suppress"
)

func TestParse_IgnoreCode(t *testing.T) {
	src := "<?xml version=\"1.0\"?>\n<odoo>\n    <!-- ruxx: ignore[O004] -->\n    <record/>\n</odoo>\n"
	got := suppress.Parse(src)
	if len(got) != 1 {
		t.Fatalf("got %d directives, want 1: %+v", len(got), got)
	}
	d := got[0]
	if d.Kind != suppress.Ignore {
		t.Errorf("kind = %v, want ignore", d.Kind)
	}
	if len(d.Codes) != 1 || d.Codes[0] != "O004" {
		t.Errorf("codes = %v, want [O004]", d.Codes)
	}
	if want := strings.Index(src, "<!--"); d.Range.Start != want {
		t.Errorf("range start = %d, want %d", d.Range.Start, want)
	}
}

func TestParse_IgnoreAll(t *testing.T) {
	src := "<!-- ruxx: ignore -->\n<odoo/>\n"
	got := suppress.Parse(src)
	if len(got) != 1 || got[0].Kind != suppress.Ignore {
		t.Fatalf("got %+v, want one ignore", got)
	}
	if len(got[0].Codes) != 0 {
		t.Errorf("codes = %v, want empty (all)", got[0].Codes)
	}
}

func TestParse_MultipleCodes(t *testing.T) {
	src := "<!-- ruxx: ignore[O004, O005] -->\n"
	got := suppress.Parse(src)
	if len(got) != 1 {
		t.Fatalf("got %d directives, want 1", len(got))
	}
	if len(got[0].Codes) != 2 || got[0].Codes[0] != "O004" || got[0].Codes[1] != "O005" {
		t.Errorf("codes = %v, want [O004 O005]", got[0].Codes)
	}
}

func TestParse_DisableEnable(t *testing.T) {
	src := "<!-- ruxx: disable[O004] -->\n<a/>\n<!-- ruxx: enable[O004] -->\n"
	got := suppress.Parse(src)
	if len(got) != 2 {
		t.Fatalf("got %d directives, want 2: %+v", len(got), got)
	}
	if got[0].Kind != suppress.Disable {
		t.Errorf("first kind = %v, want disable", got[0].Kind)
	}
	if got[1].Kind != suppress.Enable {
		t.Errorf("second kind = %v, want enable", got[1].Kind)
	}
}

func TestParse_Fmt(t *testing.T) {
	src := "<!-- fmt: off -->\n<a/>\n<!-- fmt: on -->\n<!-- fmt: skip -->\n"
	got := suppress.Parse(src)
	if len(got) != 3 {
		t.Fatalf("got %d directives, want 3: %+v", len(got), got)
	}
	want := []suppress.Kind{suppress.FmtOff, suppress.FmtOn, suppress.FmtSkip}
	for i, k := range want {
		if got[i].Kind != k {
			t.Errorf("directive %d kind = %v, want %v", i, got[i].Kind, k)
		}
	}
}

func TestParse_IgnoresOrdinaryComments(t *testing.T) {
	src := "<!-- just a comment -->\n<!-- ruxx: not a directive -->\n<a/>\n"
	if got := suppress.Parse(src); len(got) != 0 {
		t.Errorf("got %+v, want no directives", got)
	}
}

func TestSuppresses_FileIgnore(t *testing.T) {
	src := "<!-- ruxx: ignore[O004] -->\n<record/>\n"
	at := strings.Index(src, "<record/>")
	if !suppress.Suppresses(src, at, "O004") {
		t.Error("O004 should be suppressed")
	}
	if suppress.Suppresses(src, at, "O005") {
		t.Error("O005 should not be suppressed")
	}
}

func TestSuppresses_FileIgnoreAll(t *testing.T) {
	src := "<!-- ruxx: ignore -->\n<record/>\n"
	if !suppress.Suppresses(src, 0, "ANYTHING") {
		t.Error("all rules should be suppressed")
	}
}

func TestSuppresses_Region(t *testing.T) {
	src := "<!-- ruxx: disable[O004] -->\n<a/>\n<!-- ruxx: enable[O004] -->\n<b/>\n"
	in := strings.Index(src, "<a/>")
	out := strings.Index(src, "<b/>")
	if !suppress.Suppresses(src, in, "O004") {
		t.Error("O004 inside the region should be suppressed")
	}
	if suppress.Suppresses(src, out, "O004") {
		t.Error("O004 outside the region should not be suppressed")
	}
}

func TestSuppresses_None(t *testing.T) {
	src := "<!-- hello -->\n<record/>\n"
	if suppress.Suppresses(src, 0, "O004") {
		t.Error("nothing should be suppressed")
	}
}
