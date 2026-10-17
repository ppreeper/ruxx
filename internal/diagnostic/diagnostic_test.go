package diagnostic_test

import (
	"reflect"
	"testing"

	"github.com/ppreeper/ruxx/internal/diagnostic"
	"github.com/ppreeper/ruxx/internal/source"
)

func TestSeverity_String(t *testing.T) {
	cases := map[diagnostic.Severity]string{
		diagnostic.SeverityError:   "error",
		diagnostic.SeverityWarning: "warning",
		diagnostic.SeverityHint:    "hint",
		diagnostic.Severity(99):    "unknown",
	}
	for sev, want := range cases {
		if got := sev.String(); got != want {
			t.Errorf("Severity(%d).String() = %q, want %q", sev, got, want)
		}
	}
}

func TestApplicability_String(t *testing.T) {
	cases := map[diagnostic.Applicability]string{
		diagnostic.ApplicabilitySafe:    "safe",
		diagnostic.ApplicabilityUnsafe:  "unsafe",
		diagnostic.ApplicabilityDisplay: "display",
		diagnostic.Applicability(99):    "unknown",
	}
	for a, want := range cases {
		if got := a.String(); got != want {
			t.Errorf("Applicability(%d).String() = %q, want %q", a, got, want)
		}
	}
}

func TestDiagnostic_Fixable(t *testing.T) {
	// No fix.
	d := diagnostic.Diagnostic{Code: "X001"}
	if _, ok := d.Fixable(); ok {
		t.Fatal("diagnostic without Fix should not be fixable")
	}

	// Safe fix.
	d.Fix = &diagnostic.Fix{Applicability: diagnostic.ApplicabilitySafe}
	ap, ok := d.Fixable()
	if !ok || ap != diagnostic.ApplicabilitySafe {
		t.Fatalf("Fixable() = (%v,%v), want (safe,true)", ap, ok)
	}

	// Unsafe fix.
	d.Fix = &diagnostic.Fix{Applicability: diagnostic.ApplicabilityUnsafe}
	if ap, ok := d.Fixable(); !ok || ap != diagnostic.ApplicabilityUnsafe {
		t.Fatalf("Fixable() = (%v,%v), want (unsafe,true)", ap, ok)
	}
}

func TestSort(t *testing.T) {
	mk := func(code string, start, end int) diagnostic.Diagnostic {
		return diagnostic.Diagnostic{Code: code, Range: source.NewRange(start, end)}
	}
	got := []diagnostic.Diagnostic{
		mk("B001", 10, 20),
		mk("A001", 5, 9),
		mk("C001", 5, 15),
		mk("A002", 5, 15),
		mk("Z001", 5, 15),
	}
	diagnostic.Sort(got)

	want := []string{"A001", "A002", "C001", "Z001", "B001"}
	var codes []string
	for _, d := range got {
		codes = append(codes, d.Code)
	}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("sorted codes = %v, want %v", codes, want)
	}
}

func TestEdit_InsertAndDelete(t *testing.T) {
	ins := diagnostic.Edit{Range: source.NewRange(3, 3), NewText: "abc"}
	if ins.Range.Len() != 0 {
		t.Fatalf("insertion range should be empty, got %d", ins.Range.Len())
	}
	del := diagnostic.Edit{Range: source.NewRange(0, 3), NewText: ""}
	if del.Range.Len() != 3 {
		t.Fatalf("deletion range len = %d, want 3", del.Range.Len())
	}
}
