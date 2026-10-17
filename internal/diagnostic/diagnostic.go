// Package diagnostic defines the rule-independent result types: what a rule
// reports (Diagnostic), where it points (source.Range), and how it can be
// repaired (Fix = ordered Edits with an Applicability).
//
// The model is deliberately free of any rule, parser, or output concerns so
// that linter, fix engine, and serializers can share it.
package diagnostic

import (
	"sort"

	"github.com/ppreeper/ruxx/internal/source"
)

// Severity ranks a diagnostic.
type Severity uint8

const (
	// SeverityError marks a defect that makes the file wrong (unloadable XML
	// or a construct Odoo will reject).
	SeverityError Severity = iota
	// SeverityWarning marks a likely mistake or a deprecated construct.
	SeverityWarning
	// SeverityHint marks an advisory style/suggestion.
	SeverityHint
)

// String returns the lowercase name of the severity, as used in output.
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}

// Applicability describes how safe a Fix is to apply automatically.
type Applicability uint8

const (
	// ApplicabilitySafe fixes preserve behavior and are applied by default.
	ApplicabilitySafe Applicability = iota
	// ApplicabilityUnsafe fixes may change behavior or drop comments; they are
	// applied only with --unsafe-fixes.
	ApplicabilityUnsafe
	// ApplicabilityDisplay fixes have no automatic edit (report only).
	ApplicabilityDisplay
)

// String returns the lowercase name of the applicability.
func (a Applicability) String() string {
	switch a {
	case ApplicabilitySafe:
		return "safe"
	case ApplicabilityUnsafe:
		return "unsafe"
	case ApplicabilityDisplay:
		return "display"
	default:
		return "unknown"
	}
}

// Edit is a single, non-overlapping source rewrite: replace the bytes in Range
// with NewText. An empty Range (Start == End) is an insertion; an empty NewText
// is a deletion.
type Edit struct {
	Range   source.Range
	NewText string
}

// Fix is an ordered set of Edits that resolves one Diagnostic. Edits within a
// single Fix must not overlap each other and must be applied in ascending
// Range.Start order.
type Fix struct {
	// Message is a short human description of the change (e.g. "add name=\"\"").
	Message string
	// Applicability says whether the fix is safe to apply automatically.
	Applicability Applicability
	// Edits are ordered by Range.Start.
	Edits []Edit
}

// Diagnostic is a single finding from a rule.
type Diagnostic struct {
	// Code is the stable rule code, e.g. "O004".
	Code string
	// Name is the kebab-case rule name, e.g. "record-missing-model".
	Name string
	// Severity ranks the finding.
	Severity Severity
	// Range points at the offending source span (byte offsets).
	Range source.Range
	// Message is the primary description.
	Message string
	// Help is optional secondary guidance (rendered as "= help: ...").
	Help string
	// Fix is optional; when present the diagnostic is auto-fixable.
	Fix *Fix
}

// Fixable reports whether the diagnostic carries an automatic fix and, if so,
// its applicability.
func (d Diagnostic) Fixable() (Applicability, bool) {
	if d.Fix == nil {
		return ApplicabilityDisplay, false
	}
	return d.Fix.Applicability, true
}

// Sort orders diagnostics deterministically: by Range.Start, then Range.End,
// then Code — the order callers and golden tests rely on.
func Sort(diags []Diagnostic) {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i], diags[j]
		if a.Range.Start != b.Range.Start {
			return a.Range.Start < b.Range.Start
		}
		if a.Range.End != b.Range.End {
			return a.Range.End < b.Range.End
		}
		return a.Code < b.Code
	})
}
