package suppress_test

import (
	"testing"

	"github.com/ppreeper/ruxx/internal/suppress"
)

func TestAddIgnore_InsertsAfterDeclaration(t *testing.T) {
	src := "<?xml version=\"1.0\"?>\n<odoo/>\n"
	got := suppress.AddIgnore(src, []string{"X007", "O004"})
	want := "<?xml version=\"1.0\"?>\n<!-- ruxx: ignore[O004, X007] -->\n<odoo/>\n"
	if got != want {
		t.Fatalf("AddIgnore = %q, want %q", got, want)
	}
}

func TestAddIgnore_TopWithoutDeclaration(t *testing.T) {
	got := suppress.AddIgnore("<odoo/>\n", []string{"O004"})
	want := "<!-- ruxx: ignore[O004] -->\n<odoo/>\n"
	if got != want {
		t.Fatalf("AddIgnore = %q, want %q", got, want)
	}
}

func TestAddIgnore_MergesExistingDirective(t *testing.T) {
	src := "<?xml version=\"1.0\"?>\n<!-- ruxx: ignore[O004] -->\n<odoo/>\n"
	got := suppress.AddIgnore(src, []string{"X007"})
	want := "<?xml version=\"1.0\"?>\n<!-- ruxx: ignore[O004, X007] -->\n<odoo/>\n"
	if got != want {
		t.Fatalf("AddIgnore = %q, want %q", got, want)
	}
}

func TestAddIgnore_BlanketIsNoOp(t *testing.T) {
	src := "<?xml version=\"1.0\"?>\n<!-- ruxx: ignore -->\n<odoo/>\n"
	if got := suppress.AddIgnore(src, []string{"O004"}); got != src {
		t.Fatalf("AddIgnore = %q, want unchanged", got)
	}
}

func TestAddIgnore_EmptyCodes(t *testing.T) {
	src := "<odoo/>\n"
	if got := suppress.AddIgnore(src, nil); got != src {
		t.Fatalf("AddIgnore = %q, want unchanged", got)
	}
}

func TestAddIgnore_DedupesAndSorts(t *testing.T) {
	got := suppress.AddIgnore("<odoo/>\n", []string{"X007", "O004", "X007", "  "})
	want := "<!-- ruxx: ignore[O004, X007] -->\n<odoo/>\n"
	if got != want {
		t.Fatalf("AddIgnore = %q, want %q", got, want)
	}
}

func TestAddIgnore_ResultIsSuppressed(t *testing.T) {
	src := "<?xml version=\"1.0\"?>\n<odoo>\n    <record id=\"a\" model=\"m\"/>  \n</odoo>\n"
	out := suppress.AddIgnore(src, []string{"X007"})
	if !suppress.Suppresses(out, len(out)-1, "X007") {
		t.Fatalf("expected X007 to be suppressed in %q", out)
	}
}
