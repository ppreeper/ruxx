// Package attrorder computes the canonical start-tag attribute order shared by
// the formatter (`attribute-sort = "alphabetical"`) and the ST003 lint rule, so
// the two never disagree about what "sorted" means.
//
// The order is:
//
//  1. QWeb directives (`t-*`) first, keeping their source order — their
//     relative order carries style meaning (see QW010) and is not ours to
//     shuffle.
//  2. The element's Odoo priority attributes, in the order listed (for example
//     `<record id model>`, `<xpath expr position>`, `<attribute name>`).
//  3. Everything else, alphabetically, keeping source order for ties.
package attrorder

import (
	"sort"
	"strings"
)

// priorities maps an element name to the attributes that lead. These mirror the
// Odoo conventions (and odoo-shortcuts' `expr` before `position` fix).
var priorities = map[string][]string{
	"record":    {"id", "model"},
	"xpath":     {"expr", "position"},
	"attribute": {"name"},
	"field":     {"name"},
	"menuitem":  {"id", "name"},
	"template":  {"id", "name"},
}

// Priority returns the leading attributes for elem, or nil when the element has
// no special order.
func Priority(elem string) []string {
	return priorities[elem]
}

// Order returns the permutation that puts names into canonical order for elem.
// The result is a slice of indexes into names.
func Order(elem string, names []string) []int {
	prio := priorities[elem]
	var directives, rest []int
	for i, name := range names {
		if strings.HasPrefix(name, "t-") {
			directives = append(directives, i)
			continue
		}
		rest = append(rest, i)
	}
	rank := func(name string) int {
		for j, p := range prio {
			if p == name {
				return j
			}
		}
		return len(prio)
	}
	sort.SliceStable(rest, func(a, b int) bool {
		ra, rb := rank(names[rest[a]]), rank(names[rest[b]])
		if ra != rb {
			return ra < rb
		}
		return names[rest[a]] < names[rest[b]]
	})
	return append(directives, rest...)
}

// Sorted reports whether names are already in canonical order for elem.
func Sorted(elem string, names []string) bool {
	for i, idx := range Order(elem, names) {
		if idx != i {
			return false
		}
	}
	return true
}
