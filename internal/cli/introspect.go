package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ppreeper/ruxx/internal/rules"
	"github.com/ppreeper/ruxx/internal/settings"
)

// familyNames spell out the rule-code prefixes for humans.
var familyNames = map[string]string{
	"X":   "XML",
	"O":   "Odoo",
	"QW":  "QWeb",
	"XP":  "XPath",
	"VW":  "View",
	"SEC": "Security",
	"ST":  "Style",
	"RUX": "Meta",
}

// runRule implements `ruxx rule [selector]`.
func runRule(args []string, stdout, stderr io.Writer) int {
	selector := ""
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprintln(stdout, "usage: ruxx rule [CODE|SELECTOR]")
			return ExitClean
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "ruxx: unknown flag %q\n", a)
			return ExitAbnormal
		case selector != "":
			fmt.Fprintln(stderr, "usage: ruxx rule [CODE|SELECTOR]")
			return ExitAbnormal
		default:
			selector = a
		}
	}

	if selector == "" {
		printRuleList(stdout, rules.All())
		return ExitClean
	}

	if r, ok := rules.ByCode(selector); ok {
		printRuleDetail(stdout, r)
		return ExitClean
	}

	// Not an exact code: accept any selector (family, category, band, ALL).
	set, err := rules.Resolve([]string{selector}, nil)
	if err != nil {
		fmt.Fprintf(stderr, "ruxx: %v\n", err)
		return ExitAbnormal
	}
	matched := make([]rules.Rule, 0, len(set.Codes()))
	for _, code := range set.Codes() {
		if r, ok := rules.ByCode(code); ok {
			matched = append(matched, r)
		}
	}
	fmt.Fprintf(stdout, "%s selects %d rules:\n", selector, len(matched))
	printRuleList(stdout, matched)
	return ExitClean
}

func printRuleList(w io.Writer, rs []rules.Rule) {
	for _, r := range rs {
		fix := "-"
		if a, ok := rules.FixApplicability(r.Code); ok {
			fix = a.String()
		}
		def := "optional"
		if r.DefaultOn() {
			def = "default"
		}
		fmt.Fprintf(w, "%-6s %-32s %-12s %-9s %s\n", r.Code, r.Name, r.Category, def, fix)
	}
}

func printRuleDetail(w io.Writer, r rules.Rule) {
	family := familyNames[r.Family]
	if family == "" {
		family = r.Family
	}
	fix := "none"
	if a, ok := rules.FixApplicability(r.Code); ok {
		fix = a.String()
	}
	def := "optional (not in the default set)"
	if r.DefaultOn() {
		def = "enabled (in the default set)"
	}
	fmt.Fprintf(w, "%s  %s\n", r.Code, r.Name)
	fmt.Fprintf(w, "  family    %s (%s)\n", family, r.Family)
	fmt.Fprintf(w, "  category  %s\n", r.Category)
	fmt.Fprintf(w, "  default   %s\n", def)
	fmt.Fprintf(w, "  fix       %s\n", fix)
}

// configEntry describes one configuration key for `ruxx config`.
type configEntry struct {
	Key  string
	Desc string
	Val  func(s *settings.Settings) string
}

var configEntries = []configEntry{
	{"line-length", "maximum line width the linter reports and the formatter wraps at", func(s *settings.Settings) string { return strconv.Itoa(s.LineLength) }},
	{"indent-width", "spaces per indentation level", func(s *settings.Settings) string { return strconv.Itoa(s.IndentWidth) }},
	{"target-version", `Odoo series to check against; "auto" infers it from the path`, func(s *settings.Settings) string { return s.TargetVersion }},
	{"cache-dir", "where lint results are cached", func(s *settings.Settings) string { return s.CacheDir }},
	{"include", "globs to analyze (replaces the built-in list)", func(s *settings.Settings) string { return strings.Join(s.Include, ", ") }},
	{"exclude", "globs to skip (replaces the built-in list)", func(s *settings.Settings) string { return strings.Join(s.Exclude, ", ") }},
	{"extend-include", "globs to add to `include`", func(*settings.Settings) string { return "(none)" }},
	{"extend-exclude", "globs to add to `exclude`", func(*settings.Settings) string { return "(none)" }},
	{"respect-gitignore", "honour .gitignore files", func(s *settings.Settings) string { return strconv.FormatBool(s.RespectGitignore) }},
	{"force-exclude", "also exclude paths passed explicitly on the command line", func(s *settings.Settings) string { return strconv.FormatBool(s.ForceExclude) }},
	{"lint.select", "rule selectors to enable", func(s *settings.Settings) string { return defaultIfEmpty(s.Select, `["correctness"]`) }},
	{"lint.ignore", "rule selectors to switch off after `select`", func(s *settings.Settings) string { return defaultIfEmpty(s.Ignore, "[]") }},
	{"lint.per-file-ignores", "per-glob rule exceptions", func(*settings.Settings) string { return "(none)" }},
	{"odoo.modules-path", "addons roots for the cross-file index (opt-in)", func(s *settings.Settings) string { return defaultIfEmpty(s.ModulesPath, "(unset)") }},
	{"format.attribute-split", `layout attributes one per line: "auto", "always", or "never"`, func(s *settings.Settings) string { return s.AttributeSplit }},
	{"format.split-attributes-tags", "tags whose attributes always split", func(s *settings.Settings) string { return defaultIfEmpty(s.AttributeSplitTags, "(none)") }},
	{"format.close-tag-on-new-line", "put a leaf element's closing tag on its own line", func(s *settings.Settings) string { return strconv.FormatBool(s.CloseTagOnNewLine) }},
	{"format.empty-element", `render empty elements as "self-closing" or "expand"`, func(s *settings.Settings) string { return s.EmptyElement }},
	{"format.odoo-tag-spacing", "blank line before grouped Odoo blocks", func(s *settings.Settings) string { return strconv.FormatBool(s.OdooTagSpacing) }},
	{"format.odoo-spacing-tags", "tags that get a preceding blank line", func(s *settings.Settings) string { return strings.Join(s.OdooSpacingTags, ", ") }},
	{"format.preserve-blank-lines", "keep a single author-written blank line between nodes", func(s *settings.Settings) string { return strconv.FormatBool(s.PreserveBlankLines) }},
	{"format.attribute-sort", "attribute order: none or alphabetical", func(s *settings.Settings) string { return s.AttributeSort }},
	{"format.indent-style", `indent with "space" or "tab"`, func(s *settings.Settings) string { return s.IndentStyle }},
	{"format.line-ending", `line endings: "auto", "lf", or "crlf"`, func(s *settings.Settings) string { return s.LineEnding }},
	{"format.final-newline", "end the file with exactly one newline", func(s *settings.Settings) string { return strconv.FormatBool(s.FinalNewline) }},
	{"format.xml-declaration", `the <?xml ...?> prolog: "preserve", "always", or "never"`, func(s *settings.Settings) string { return s.XMLDeclaration }},
	{"format.encoding-style", `encoding case: "lower", "upper", or "preserve"`, func(s *settings.Settings) string { return s.EncodingStyle }},
	{"format.qweb-attributes-first", "put QWeb (t-*) attributes before plain ones", func(s *settings.Settings) string { return strconv.FormatBool(s.QWebAttributesFirst) }},
}

// clipValue shortens a rendered value so one long entry (a glob list, say)
// cannot stretch the whole table. The full value is always available via
// `ruxx config <key>`.
// printSettings writes the effective settings for a path, one key per line, in
// the same shape as `ruxx config` so the output is diffable.
func printSettings(w io.Writer, path string, s *settings.Settings) {
	fmt.Fprintf(w, "Resolved settings for %s:\n", path)
	for _, e := range configEntries {
		fmt.Fprintf(w, "  %-28s %s\n", e.Key, e.Val(s))
	}
}

func clipValue(v string, max int) string {
	if len(v) <= max {
		return v
	}
	if max < 4 {
		return v[:max]
	}
	return v[:max-3] + "..."
}

func defaultIfEmpty(v []string, empty string) string {
	if len(v) == 0 {
		return empty
	}
	return "[" + strings.Join(v, ", ") + "]"
}

// runConfig implements `ruxx config [key]`.
func runConfig(args []string, stdout, stderr io.Writer) int {
	key := ""
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprintln(stdout, "usage: ruxx config [KEY]")
			return ExitClean
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "ruxx: unknown flag %q\n", a)
			return ExitAbnormal
		case key != "":
			fmt.Fprintln(stderr, "usage: ruxx config [KEY]")
			return ExitAbnormal
		default:
			key = a
		}
	}

	s := settings.Default()
	if key == "" {
		fmt.Fprintln(stdout, "Configuration keys (built-in defaults; a .ruxx.toml may override them):")
		const valColMax = 40
		keyW, valW := 0, 0
		for _, e := range configEntries {
			if n := len(e.Key); n > keyW {
				keyW = n
			}
			n := len(e.Val(s))
			if n > valColMax {
				n = valColMax
			}
			if n > valW {
				valW = n
			}
		}
		for _, e := range configEntries {
			fmt.Fprintf(stdout, "%-*s  %-*s  %s\n", keyW, e.Key, valW, clipValue(e.Val(s), valW), e.Desc)
		}
		return ExitClean
	}

	for _, e := range configEntries {
		if e.Key == key {
			fmt.Fprintf(stdout, "%s = %s   (built-in default)\n  %s\n", e.Key, e.Val(s), e.Desc)
			return ExitClean
		}
	}
	fmt.Fprintf(stderr, "ruxx: unknown config key %q\n", key)
	return ExitAbnormal
}
