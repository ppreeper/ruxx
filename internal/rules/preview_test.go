package rules

import (
	"errors"
	"testing"
)

func TestIsPreview_DefaultFalse(t *testing.T) {
	if IsPreview("X001") {
		t.Error("no rule is in preview yet")
	}
}

// TestResolveWith_PreviewGate pins the preview contract using a real catalog
// code marked preview for the duration of the test.
func TestResolveWith_PreviewGate(t *testing.T) {
	const code = "X014" // correctness, normally enabled by the default set
	preview[code] = true
	defer delete(preview, code)

	if !IsPreview(code) {
		t.Fatalf("IsPreview(%q) = false, want true", code)
	}

	// Broad selectors never pick up a preview rule without preview on.
	for _, sel := range []string{"ALL", "X", "correctness"} {
		set, err := ResolveWith([]string{sel}, nil, false)
		if err != nil {
			t.Fatalf("ResolveWith(%q): %v", sel, err)
		}
		if set.Enabled(code) {
			t.Errorf("%q enabled the preview rule without preview", sel)
		}
	}

	// The default set skips it too.
	set, err := ResolveWith(nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if set.Enabled(code) {
		t.Error("the default set enabled a preview rule")
	}

	// Selecting it by code without preview is an error we can explain.
	var pe *PreviewSelectorError
	if _, err := ResolveWith([]string{code}, nil, false); !errors.As(err, &pe) {
		t.Errorf("exact preview selection: err = %v, want *PreviewSelectorError", err)
	}

	// With preview on, exact and broad selection both work.
	set, err = ResolveWith([]string{code}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !set.Enabled(code) {
		t.Error("preview rule not enabled with preview on")
	}
	set, err = ResolveWith([]string{"ALL"}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !set.Enabled(code) {
		t.Error("ALL did not enable the preview rule with preview on")
	}

	// Ignoring a preview code never requires preview.
	if _, err := ResolveWith([]string{"ALL"}, []string{code}, false); err != nil {
		t.Errorf("ignoring a preview code errored: %v", err)
	}
}
