package rules

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
)

// categories is the set of valid category selector names.
var categories = map[string]bool{
	string(Correctness): true,
	string(Suspicious):  true,
	string(Style):       true,
	string(Security):    true,
	string(Pedantic):    true,
	string(Restriction): true,
	string(Formatting):  true,
}

// familiesOnce/familiesSet cache the set of known rule families, derived lazily
// from the catalog (which is populated in an init function, so a package-level
// initializer here would run too early).
var (
	familiesOnce sync.Once
	familiesSet  map[string]bool
)

func isFamily(s string) bool {
	familiesOnce.Do(func() {
		familiesSet = map[string]bool{}
		for _, r := range All() {
			familiesSet[r.Family] = true
		}
	})
	return familiesSet[s]
}

// Set is a resolved set of enabled rule codes.
type Set struct {
	enabled map[string]bool
}

// Enabled reports whether the given rule code is enabled.
func (s *Set) Enabled(code string) bool {
	if s == nil {
		return false
	}
	return s.enabled[code]
}

// Codes returns the enabled rule codes in sorted order.
func (s *Set) Codes() []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.enabled))
	for c := range s.enabled {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// UnknownSelectorError reports a selector that matches no known rule.
type UnknownSelectorError struct {
	Selector string
}

// Error implements error.
func (e *UnknownSelectorError) Error() string {
	return fmt.Sprintf("unknown rule selector %q", e.Selector)
}

// PreviewSelectorError reports a preview rule selected without preview mode.
type PreviewSelectorError struct {
	Selector string
}

// Error implements error.
func (e *PreviewSelectorError) Error() string {
	return fmt.Sprintf("rule %s is in preview; enable it with --preview (or preview = true)", e.Selector)
}

// Resolve computes the enabled rule set from select and ignore selectors, with
// preview rules off.
func Resolve(selectors, ignore []string) (*Set, error) {
	return ResolveWith(selectors, ignore, false)
}

// ResolveWith is Resolve with preview rules enabled or not.
//
// A selector matches rules by, in order of breadth: "ALL", a category name
// (for example "style"), a family (for example "O"), a code prefix (for
// example "XP0"), or an exact code. When select is empty the default set (the
// correctness category) is used; otherwise the selected rules are enabled.
// Ignore is applied afterwards and wins. A selector that matches no rule is an
// error.
//
// Broad selectors never enable preview rules; an exact preview code without
// preview is a PreviewSelectorError, and ignore may always name them.
func ResolveWith(selectors, ignore []string, previewOn bool) (*Set, error) {
	enabled := map[string]bool{}

	if len(selectors) == 0 {
		for _, r := range All() {
			if r.DefaultOn() && !IsPreview(r.Code) {
				enabled[r.Code] = true
			}
		}
	} else {
		for _, sel := range selectors {
			codes, ok, err := resolveSelector(sel, previewOn)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, &UnknownSelectorError{Selector: sel}
			}
			for _, c := range codes {
				enabled[c] = true
			}
		}
	}

	for _, sel := range ignore {
		// Ignoring never requires preview: a rule that is not enabled anyway
		// is simply a no-op.
		codes, ok, err := resolveSelector(sel, true)
		if err != nil || !ok {
			return nil, &UnknownSelectorError{Selector: sel}
		}
		for _, c := range codes {
			delete(enabled, c)
		}
	}

	return &Set{enabled: enabled}, nil
}

// resolveSelector expands one selector into the codes it matches. The ok result
// is false when the selector matches no rule; err is non-nil when the selector
// names a preview rule while preview is off.
func resolveSelector(sel string, previewOn bool) (codes []string, ok bool, err error) {
	// visible filters preview rules out of broad expansions.
	visible := func(code string) bool { return previewOn || !IsPreview(code) }

	if sel == "ALL" {
		out := make([]string, 0, len(All()))
		for _, r := range All() {
			if visible(r.Code) {
				out = append(out, r.Code)
			}
		}
		return out, true, nil
	}

	if categories[sel] {
		for _, r := range All() {
			if string(r.Category) == sel && visible(r.Code) {
				codes = append(codes, r.Code)
			}
		}
		return codes, len(codes) > 0, nil
	}

	if isFamily(sel) {
		for _, r := range All() {
			if r.Family == sel && visible(r.Code) {
				codes = append(codes, r.Code)
			}
		}
		return codes, len(codes) > 0, nil
	}

	// Exact code, checked before the numeric band so that e.g. "XP010" and
	// "O004" resolve to themselves rather than to a band.
	if r, found := ByCode(sel); found {
		if IsPreview(r.Code) && !previewOn {
			return nil, false, &PreviewSelectorError{Selector: sel}
		}
		return []string{r.Code}, true, nil
	}

	// Numeric band: a family followed by digits denotes a ten-wide band
	// indexed by the digits' value. "XP0" selects XP001..XP009 (values 0-9)
	// and "XP01" selects XP010..XP019.
	if fam, rest := splitFamily(sel); isFamily(fam) && rest != "" && allDigits(rest) {
		if n, err := strconv.Atoi(rest); err == nil {
			lo, hi := n*10, n*10+10
			for _, r := range All() {
				if r.Family != fam || !visible(r.Code) {
					continue
				}
				if v, ok := numericSuffix(r.Code); ok && v >= lo && v < hi {
					codes = append(codes, r.Code)
				}
			}
		}
		return codes, len(codes) > 0, nil
	}

	return nil, false, nil
}

// numericSuffix returns the integer value of a code's trailing digits.
func numericSuffix(code string) (int, bool) {
	_, rest := splitFamily(code)
	if rest == "" || !allDigits(rest) {
		return 0, false
	}
	n, err := strconv.Atoi(rest)
	if err != nil {
		return 0, false
	}
	return n, true
}

// splitFamily returns the leading run of uppercase letters and the remainder.
func splitFamily(s string) (fam, rest string) {
	i := 0
	for i < len(s) && s[i] >= 'A' && s[i] <= 'Z' {
		i++
	}
	return s[:i], s[i:]
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
