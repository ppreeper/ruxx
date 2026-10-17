package fix_test

import (
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/fix"
	"github.com/ppreeper/ruxx/internal/settings"
)

// X014's rename assist is deliberately unsafe: it invents a new identifier, so
// it must not fire unless the caller asks for unsafe fixes.
const dupSrc = `<?xml version="1.0"?>
<odoo>
    <record id="dup" model="ir.ui.view"/>
    <record id="dup" model="ir.ui.view"/>
</odoo>
`

func TestApply_UnsafeRenameDuplicate(t *testing.T) {
	s := settings.Default()
	s.Select = []string{"X014"}

	safe := fix.Apply(dupSrc, s, fix.Options{})
	if safe.Source != dupSrc {
		t.Fatalf("safe-only should not change the source:\n%s", safe.Source)
	}

	unsafe := fix.Apply(dupSrc, s, fix.Options{Unsafe: true})
	if unsafe.Reverted {
		t.Fatal("the rename should not trigger a revert")
	}
	if !strings.Contains(unsafe.Source, `id="dup"`) {
		t.Fatalf("the first definition must be kept:\n%s", unsafe.Source)
	}
	if !strings.Contains(unsafe.Source, `id="dup_2"`) {
		t.Fatalf("the duplicate should be renamed:\n%s", unsafe.Source)
	}
	if strings.Count(unsafe.Source, `id="dup"`) != 1 {
		t.Fatalf("expected exactly one surviving id=\"dup\":\n%s", unsafe.Source)
	}
	// The renamed source must now be clean for X014.
	if again := fix.Apply(unsafe.Source, s, fix.Options{Unsafe: true}); again.Source != unsafe.Source {
		t.Fatalf("not idempotent:\n%s", again.Source)
	}
}

func TestApply_UnsafeRenameThirdDuplicate(t *testing.T) {
	src := `<?xml version="1.0"?>
<odoo>
    <record id="a" model="m"/>
    <record id="a" model="m"/>
    <record id="a" model="m"/>
</odoo>
`
	s := settings.Default()
	s.Select = []string{"X014"}
	got := fix.Apply(src, s, fix.Options{Unsafe: true}).Source
	for _, want := range []string{`id="a"`, `id="a_2"`, `id="a_3"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in:\n%s", want, got)
		}
	}
}
