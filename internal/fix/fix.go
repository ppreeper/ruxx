// Package fix applies rule fixes to source text.
//
// Two layers:
//
//   - ApplyEdits is the pure, mechanical layer: it applies a set of
//     non-overlapping, sorted Edits to a string.
//   - Apply is the orchestration layer: lint -> collect fixes -> apply ->
//     reparse -> repeat until stable, mirroring Ruff's bounded fix loop.
//
// The loop guarantees: overlapping fixes are resolved deterministically, the
// process is bounded by MaxIterations, and if a change would introduce a
// well-formedness error into originally-valid source, all changes are reverted.
package fix

import (
	"fmt"
	"sort"

	"github.com/ppreeper/ruxx/internal/diagnostic"
	"github.com/ppreeper/ruxx/internal/linter"
	"github.com/ppreeper/ruxx/internal/rules"
	"github.com/ppreeper/ruxx/internal/settings"
	"github.com/ppreeper/ruxx/internal/syntax"
)

// MaxIterations bounds the lint/fix convergence loop.
const MaxIterations = 100

// Options controls fix application.
type Options struct {
	// Unsafe enables fixes whose Applicability is ApplicabilityUnsafe.
	Unsafe bool
}

// Result reports the outcome of Apply.
type Result struct {
	// Source is the final text (the original if nothing applied or reverted).
	Source string
	// Applied lists the codes of diagnostics whose fixes were applied, in the
	// order they were applied.
	Applied []string
	// Iterations is the number of lint/fix passes performed.
	Iterations int
	// Reverted is true when a pass introduced a syntax error and all changes
	// were rolled back.
	Reverted bool
}

// Apply runs the fix loop over src with settings s.
func Apply(src string, s *settings.Settings, opts Options) Result {
	if s == nil {
		s = settings.Default()
	}
	orig := src
	var applied []string
	iters := 0
	allow := fixableFilter(s)

	for iters < MaxIterations {
		iters++
		tree := syntax.Parse(src)
		diags := linter.Check(tree, s)

		var edits []diagnostic.Edit
		for _, d := range diags {
			if d.Fix == nil {
				continue
			}
			if allow != nil && !allow(d.Code) {
				continue
			}
			switch d.Fix.Applicability {
			case diagnostic.ApplicabilitySafe:
				// always applies
			case diagnostic.ApplicabilityUnsafe:
				if !opts.Unsafe {
					continue
				}
			default:
				continue // display-only
			}
			edits = append(edits, d.Fix.Edits...)
			applied = append(applied, d.Code)
		}

		if len(edits) == 0 {
			break
		}
		edits = dropOverlaps(edits)
		fixed, err := ApplyEdits(src, edits)
		if err != nil || fixed == src {
			break
		}
		src = fixed
	}

	// Revert if we broke originally-valid source.
	if syntax.Parse(orig).HasValidSyntax() && !syntax.Parse(src).HasValidSyntax() {
		return Result{Source: orig, Reverted: true, Iterations: iters, Applied: applied}
	}
	return Result{Source: src, Applied: applied, Iterations: iters}
}

// fixableFilter builds the predicate that decides whether a rule's fixes may be
// applied, honouring the Fixable/Unfixable selectors. It returns nil when no
// restriction is configured, meaning every fixable rule applies.
//
// Unfixable wins over Fixable. An unresolvable selector is treated as an empty
// restriction (the CLI validates selectors before running).
func fixableFilter(s *settings.Settings) func(code string) bool {
	if s == nil || (len(s.Fixable) == 0 && len(s.Unfixable) == 0) {
		return nil
	}
	var fixable, unfixable *rules.Set
	if len(s.Fixable) > 0 {
		if set, err := rules.Resolve(s.Fixable, nil); err == nil {
			fixable = set
		}
	}
	if len(s.Unfixable) > 0 {
		if set, err := rules.Resolve(s.Unfixable, nil); err == nil {
			unfixable = set
		}
	}
	return func(code string) bool {
		if fixable != nil && !fixable.Enabled(code) {
			return false
		}
		if unfixable != nil && unfixable.Enabled(code) {
			return false
		}
		return true
	}
}

// dropOverlaps keeps a deterministic, non-overlapping subset of edits. They are
// sorted by (start, end) and an edit is skipped when it overlaps one already
// kept, so several rules can contribute fixes in a single pass (e.g. O019 and
// ST003 both rewrapping the same start tag). Whatever is skipped is still
// reported and can be fixed on the next iteration of the loop.
func dropOverlaps(edits []diagnostic.Edit) []diagnostic.Edit {
	sorted := make([]diagnostic.Edit, len(edits))
	copy(sorted, edits)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Range.Start != sorted[j].Range.Start {
			return sorted[i].Range.Start < sorted[j].Range.Start
		}
		return sorted[i].Range.End < sorted[j].Range.End
	})
	out := make([]diagnostic.Edit, 0, len(sorted))
	pos := 0
	for _, e := range sorted {
		if e.Range.Start < pos {
			continue // overlaps an edit already kept
		}
		out = append(out, e)
		pos = e.Range.End
	}
	return out
}

// ApplyEdits applies edits to src. Edits must not overlap; they are applied in
// ascending Range.Start order. It returns an error if any two edits overlap.
func ApplyEdits(src string, edits []diagnostic.Edit) (string, error) {
	if len(edits) == 0 {
		return src, nil
	}
	sorted := make([]diagnostic.Edit, len(edits))
	copy(sorted, edits)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Range.Start != sorted[j].Range.Start {
			return sorted[i].Range.Start < sorted[j].Range.Start
		}
		return sorted[i].Range.End < sorted[j].Range.End
	})

	var b []byte
	pos := 0
	for _, e := range sorted {
		if e.Range.Start < pos {
			return "", fmt.Errorf("overlapping edits at offset %d", e.Range.Start)
		}
		if e.Range.Start > len(src) || e.Range.End > len(src) || e.Range.Start > e.Range.End {
			return "", fmt.Errorf("edit out of range")
		}
		b = append(b, src[pos:e.Range.Start]...)
		b = append(b, e.NewText...)
		pos = e.Range.End
	}
	b = append(b, src[pos:]...)
	return string(b), nil
}
