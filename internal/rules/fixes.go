package rules

import "github.com/ppreeper/ruxx/internal/diagnostic"

// fixApplicability lists the rules that carry an automatic fix and how safe
// that fix is. A rule absent from this table produces no fix.
//
// Keep it in step with the rule bodies: fixes_test.go cross-checks every entry
// against the registry, and the fix goldens in internal/fix cover the rewrites.
var fixApplicability = map[string]diagnostic.Applicability{
	// XML hygiene.
	"X002": diagnostic.ApplicabilitySafe,   // insert the declaration
	"X003": diagnostic.ApplicabilitySafe,   // lower-case the encoding
	"X004": diagnostic.ApplicabilitySafe,   // strip the BOM
	"X005": diagnostic.ApplicabilitySafe,   // CRLF to LF
	"X006": diagnostic.ApplicabilitySafe,   // tabs to spaces
	"X007": diagnostic.ApplicabilitySafe,   // trim trailing whitespace
	"X008": diagnostic.ApplicabilitySafe,   // round indentation to the unit
	"X012": diagnostic.ApplicabilitySafe,   // add the final newline
	"X013": diagnostic.ApplicabilitySafe,   // drop whitespace around '='
	"X014": diagnostic.ApplicabilityUnsafe, // rename a duplicate id
	"X016": diagnostic.ApplicabilitySafe,   // tighten the start tag

	// Odoo structure.
	"O002": diagnostic.ApplicabilitySafe, // <openerp> to <odoo>
	"O010": diagnostic.ApplicabilitySafe, // tree/list rename
	"O019": diagnostic.ApplicabilitySafe, // id before model

	// QWeb.
	"QW001": diagnostic.ApplicabilityUnsafe, // t-esc to t-out
	"QW003": diagnostic.ApplicabilitySafe,   // t-* attributes first
	"QW009": diagnostic.ApplicabilitySafe,   // xml:space on <templates>

	// XPath.
	"XP002": diagnostic.ApplicabilitySafe, // add an explicit position
	"XP004": diagnostic.ApplicabilitySafe, // expr before position

	// Style.
	"ST001": diagnostic.ApplicabilitySafe, // blank line before an Odoo tag
	"ST002": diagnostic.ApplicabilitySafe, // self-close empty elements
	"ST003": diagnostic.ApplicabilitySafe, // sort attributes
	"ST005": diagnostic.ApplicabilitySafe, // collapse blank-line runs
	"ST006": diagnostic.ApplicabilitySafe, // drop the space in " />"
}

// FixApplicability reports whether a rule carries an automatic fix and, if so,
// how safe that fix is.
func FixApplicability(code string) (diagnostic.Applicability, bool) {
	a, ok := fixApplicability[code]
	return a, ok
}
