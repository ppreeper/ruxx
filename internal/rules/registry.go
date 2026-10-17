package rules

// Category groups rules by intent. The default rule set is exactly the
// Correctness category; other categories are opt-in.
type Category string

const (
	// Correctness rules catch defects that make a file wrong or unloadable.
	Correctness Category = "correctness"
	// Suspicious rules flag likely mistakes.
	Suspicious Category = "suspicious"
	// Style rules are layout/formatting preferences (off by default).
	Style Category = "style"
	// Security rules concern access and rules data.
	Security Category = "security"
	// Pedantic rules are opinionated extras.
	Pedantic Category = "pedantic"
	// Restriction rules forbid specific constructs.
	Restriction Category = "restriction"
	// Formatting rules overlap with the formatter.
	Formatting Category = "formatting"
)

// Rule is the static metadata for one rule. Behaviour lives elsewhere; this is
// the data the selector resolver and `ruxx rule` read.
type Rule struct {
	// Code is the stable code, e.g. "O004".
	Code string
	// Name is the kebab-case name, e.g. "record-missing-model".
	Name string
	// Family is the leading letter group, e.g. "O", "QW", "RUX".
	Family string
	// Category groups the rule.
	Category Category
}

// DefaultOn reports whether the rule is enabled with no configuration.
func (r Rule) DefaultOn() bool { return r.Category == Correctness }

// familyOf returns the leading run of uppercase letters in a code.
func familyOf(code string) string {
	i := 0
	for i < len(code) && code[i] >= 'A' && code[i] <= 'Z' {
		i++
	}
	return code[:i]
}

type entry struct {
	code string
	name string
	cat  Category
}

// entries is the single source of truth for the rule catalog (see
// docs/RULES.md). Family and DefaultOn are derived.
var entries = []entry{
	// X — XML core.
	{"X001", "parse-error", Correctness},
	{"X002", "missing-xml-declaration", Style},
	{"X003", "encoding-style", Style},
	{"X004", "byte-order-mark", Style},
	{"X005", "crlf-line-ending", Style},
	{"X006", "tab-indentation", Style},
	{"X007", "trailing-whitespace", Style},
	{"X008", "indent-unit", Style},
	{"X009", "mixed-indentation", Style},
	{"X010", "line-too-long", Style},
	{"X011", "conflict-markers", Correctness},
	{"X012", "missing-final-newline", Style},
	{"X013", "attribute-spacing", Style},
	{"X014", "duplicate-id-in-file", Correctness},
	{"X015", "xmlid-multiple-dots", Correctness},
	{"X016", "empty-element-whitespace", Style},
	{"X017", "unclosed-cdata-or-comment", Correctness},

	// O — Odoo data.
	{"O001", "invalid-root-tag", Correctness},
	{"O002", "deprecated-openerp-root", Suspicious},
	{"O003", "record-missing-id", Suspicious},
	{"O004", "record-missing-model", Correctness},
	{"O005", "field-missing-name", Correctness},
	{"O006", "arch-missing-type", Suspicious},
	{"O007", "noupdate-on-record", Suspicious},
	{"O008", "deprecated-act-window", Correctness},
	{"O009", "deprecated-report-tag", Correctness},
	{"O010", "tree-vs-list", Correctness},
	{"O011", "view-mode-version", Correctness},
	{"O012", "deprecated-attrs", Suspicious},
	{"O013", "deprecated-states", Suspicious},
	{"O014", "group-typo", Suspicious},
	{"O015", "menuitem-missing-id", Correctness},
	{"O016", "xmlid-uninstalled-module", Suspicious},
	{"O017", "noupdate-not-boolean", Suspicious},
	{"O018", "duplicate-xmlid-module", Suspicious},
	{"O019", "record-attribute-order", Style},
	{"O020", "record-id-qualified", Style},
	{"O021", "template-id-missing", Correctness},
	{"O022", "ir-ui-view-field-order", Style},

	// QW — QWeb / OWL.
	{"QW001", "deprecated-tesc", Suspicious},
	{"QW002", "deprecated-traw", Suspicious},
	{"QW003", "qweb-attributes-first", Style},
	{"QW004", "template-missing-name", Suspicious},
	{"QW005", "tname-module-qualified", Style},
	{"QW006", "owl-directive-forbidden", Correctness},
	{"QW007", "comp-directive-forbidden", Correctness},
	{"QW008", "data-tooltip-forbidden", Correctness},
	{"QW009", "templates-root-space", Style},
	{"QW010", "directive-order", Style},
	{"QW011", "deprecated-tjquery", Suspicious},
	{"QW012", "tcall-unknown-template", Suspicious},

	// XP — XPath inheritance.
	{"XP001", "missing-expr", Correctness},
	{"XP002", "missing-position", Suspicious},
	{"XP003", "invalid-position", Correctness},
	{"XP004", "attribute-order", Style},
	{"XP005", "class-selector", Suspicious},
	{"XP006", "translated-selector", Suspicious},
	{"XP007", "attributes-needs-attribute", Correctness},
	{"XP008", "attribute-no-position", Style},
	{"XP009", "non-locatable-spec", Suspicious},
	{"XP010", "inherited-record-groups", Correctness},

	// VW — view architecture.
	{"VW001", "page-outside-notebook", Correctness},
	{"VW002", "graph-children", Correctness},
	{"VW003", "list-children", Correctness},
	{"VW004", "search-needs-field", Suspicious},
	{"VW005", "search-multiple-panels", Correctness},
	{"VW006", "img-missing-alt", Suspicious},
	{"VW007", "notebook-needs-page", Suspicious},
	{"VW008", "duplicate-field-in-view", Suspicious},

	// SEC — security.
	{"SEC001", "rule-missing-domain", Security},
	{"SEC002", "access-file-not-in-manifest", Security},
	{"SEC003", "group-implies-unknown", Security},

	// ST — style.
	{"ST001", "blank-line-before-odoo-tag", Style},
	{"ST002", "self-closing-empty-element", Style},
	{"ST003", "attribute-sort", Style},
	{"ST004", "close-tag-new-line", Style},
	{"ST005", "consecutive-blank-lines", Style},
	{"ST006", "space-before-self-close", Style},

	// RUX — meta.
	{"RUX100", "unused-suppression", Correctness},
	{"RUX101", "unknown-suppression-code", Correctness},
	{"RUX102", "deprecated-suppression-code", Pedantic},
	{"RUX103", "formatter-conflict", Correctness},
	{"RUX104", "unclosed-disable-region", Correctness},
}

var (
	byCode = make(map[string]Rule, len(entries))
	all    = make([]Rule, 0, len(entries))
)

func init() {
	for _, e := range entries {
		r := Rule{Code: e.code, Name: e.name, Family: familyOf(e.code), Category: e.cat}
		byCode[r.Code] = r
		all = append(all, r)
	}
}

// All returns every registered rule (in catalog order).
func All() []Rule { return all }

// ByCode looks a rule up by its exact code.
func ByCode(code string) (Rule, bool) {
	r, ok := byCode[code]
	return r, ok
}

// preview holds the codes that are gated behind the preview flag. A preview
// rule is never enabled by a broad selector (ALL, a category, a family, or a
// code band); selecting one by its exact code is an error unless preview is
// on. The catalog has no preview rules yet — this is the mechanism new rules
// use to ship behind the flag.
var preview = map[string]bool{}

// IsPreview reports whether code is a preview rule.
func IsPreview(code string) bool { return preview[code] }
