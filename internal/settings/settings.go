// Package settings holds resolved configuration: the result of merging the
// built-in defaults, discovered .ruxx.toml files, CLI flags, and per-file
// overrides. Rules and passes read Settings; they never touch config files.
//
// This is intentionally small for now; it grows alongside the config package.
package settings

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ppreeper/ruxx/internal/walk"
)

// DefaultLineLength matches the spec's default (88).
const DefaultLineLength = 88

// DefaultCacheDir is the cache directory name relative to the working dir.
const DefaultCacheDir = ".ruxx_cache"

// DefaultIndentWidth is the canonical 4-space XML indentation.
const DefaultIndentWidth = 4

// DefaultAttributeSplit and DefaultEmptyElement are the canonical formatter
// modes; see the matching Settings fields.
const (
	DefaultAttributeSplit = "auto"
	DefaultEmptyElement   = "self-closing"
)

// DefaultPreserveBlankLines keeps a single author-written blank line between
// sibling nodes; set preserve-blank-lines = false to make the formatter close
// every gap it does not require.
const DefaultPreserveBlankLines = true

// DefaultAttributeSort leaves attributes in their source order.
const DefaultAttributeSort = "none"

// DefaultIndentStyle indents with spaces.
const DefaultIndentStyle = "space"

// DefaultLineEnding follows the line endings already present in the file.
const DefaultLineEnding = "auto"

// DefaultXMLDeclaration leaves the `<?xml ...?>` prolog exactly as written.
const DefaultXMLDeclaration = "preserve"

// DefaultEncodingStyle lower-cases the declaration's encoding value.
const DefaultEncodingStyle = "lower"

// DefaultFinalNewline makes the output end with exactly one newline.
const DefaultFinalNewline = true

// DefaultQWebAttributesFirst puts QWeb (t-*) attributes before plain ones.
const DefaultQWebAttributesFirst = true

// DefaultOdooSpacingTags are the elements that receive a preceding blank line
// while OdooTagSpacing is on.
var DefaultOdooSpacingTags = []string{"record", "menuitem", "template"}

// Settings is the resolved configuration for a run.
type Settings struct {
	// Select and Ignore are rule selectors (codes, prefixes, categories, ALL).
	Select []string
	Ignore []string

	// IgnoreSuppressions disables every `ruxx:` suppression directive for the
	// run (the --ignore-suppressions audit switch).
	IgnoreSuppressions bool

	// Fixable limits which rules may apply fixes (selectors). Empty means
	// every fixable rule may fix.
	Fixable []string
	// Unfixable removes rules from the fixable set; it wins over Fixable.
	Unfixable []string

	// Preview enables preview rules. A preview rule is never enabled by a
	// prefix, category, or ALL, and selecting one by code without Preview is
	// an error.
	Preview bool

	// LineLength is the maximum preferred line length (formatter + X010).
	LineLength int
	// IndentWidth is the number of spaces per indentation level.
	IndentWidth int
	// TargetVersion is "auto" or an Odoo series such as "17.0".
	TargetVersion string

	// OdooEnabled controls whether Odoo-aware rules run.
	OdooEnabled bool

	// ModulesPath lists roots for the opt-in cross-file index. Empty disables
	// all cross-file checks (O016, O018, QW012, SEC002, SEC003).
	ModulesPath []string

	// CacheDir is the directory used for cached lint results.
	CacheDir string

	// Include and Exclude select files during discovery. Include lists globs a
	// walked file must match; Exclude prunes matching files and directories.
	Include []string
	Exclude []string
	// RespectGitignore honours .gitignore files during discovery.
	RespectGitignore bool
	// ForceExclude applies Exclude even to explicit file arguments.
	ForceExclude bool

	// AttributeSplit decides when start-tag attributes go one per line:
	// "auto" (only when the inline tag exceeds LineLength), "always", or
	// "never".
	AttributeSplit string
	// AttributeSplitTags lists element names that always split, whatever
	// AttributeSplit says.
	AttributeSplitTags []string
	// CloseTagOnNewLine forces an end tag onto its own line even when the
	// element's content would fit inline.
	CloseTagOnNewLine bool
	// EmptyElement renders an element with no content either "self-closing"
	// (<x/>) or "expand" (<x></x>).
	EmptyElement string
	// OdooTagSpacing inserts a blank line before each tag in OdooSpacingTags.
	OdooTagSpacing bool
	// OdooSpacingTags lists the elements that receive a preceding blank line.
	OdooSpacingTags []string
	// PreserveBlankLines keeps one blank line where the author wrote one; runs
	// collapse to a single line. The spacing-tag blank line is added either
	// way.
	PreserveBlankLines bool
	// AttributeSort orders start-tag attributes: "none" (source order) or
	// "alphabetical" (Odoo-aware; see internal/attrorder).
	AttributeSort string

	// IndentStyle is "space" (IndentWidth spaces per level) or "tab" (one tab
	// per level). IndentWidth is still used to budget wrapped lines.
	IndentStyle string
	// LineEnding is "auto" (follow the file), "lf", or "crlf".
	LineEnding string
	// FinalNewline makes the output end with exactly one newline. When false,
	// the input's trailing-newline state is preserved.
	FinalNewline bool
	// XMLDeclaration controls the `<?xml ...?>` prolog: "preserve", "always"
	// (insert it when missing), or "never" (drop it).
	XMLDeclaration string
	// EncodingStyle sets the case of an existing declaration's encoding value:
	// "lower", "upper", or "preserve".
	EncodingStyle string
	// QWebAttributesFirst orders QWeb (t-*) attributes before plain ones.
	QWebAttributesFirst bool
}

// Default returns the zero-configuration settings: correctness rules only,
// line length 88, 4-space indent, Odoo detection "auto".
func Default() *Settings {
	return &Settings{
		Select:        nil, // nil means the default set (correctness)
		Ignore:        nil,
		LineLength:    DefaultLineLength,
		IndentWidth:   DefaultIndentWidth,
		TargetVersion: "auto",
		OdooEnabled:   true,
		CacheDir:      DefaultCacheDir,

		Include:          walk.DefaultInclude,
		Exclude:          walk.DefaultExclude,
		RespectGitignore: true,

		AttributeSplit:  DefaultAttributeSplit,
		EmptyElement:    DefaultEmptyElement,
		OdooTagSpacing:  true,
		OdooSpacingTags: DefaultOdooSpacingTags,

		PreserveBlankLines: DefaultPreserveBlankLines,
		AttributeSort:      DefaultAttributeSort,

		IndentStyle:         DefaultIndentStyle,
		LineEnding:          DefaultLineEnding,
		FinalNewline:        DefaultFinalNewline,
		XMLDeclaration:      DefaultXMLDeclaration,
		EncodingStyle:       DefaultEncodingStyle,
		QWebAttributesFirst: DefaultQWebAttributesFirst,
	}
}

// Key returns a stable fingerprint of the settings that affect diagnostics.
// It forms part of the cache key, so any settings change invalidates cached
// results. A nil Settings yields "".
func (s *Settings) Key() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("sel=%s;ign=%s;ll=%d;iw=%d;tv=%s;odoo=%t;mp=%s;cache=%s;inc=%s;exc=%s;gi=%t;fe=%t;as=%s;ast=%s;ct=%t;ee=%s;ots=%t;ost=%s;pbl=%t;aso=%s;sup=%t;is=%s;le=%s;fn=%t;xd=%s;es=%s;qwaf=%t;fx=%s;ufx=%s;pv=%t",
		joinSorted(s.Select), joinSorted(s.Ignore), s.LineLength, s.IndentWidth,
		s.TargetVersion, s.OdooEnabled, joinSorted(s.ModulesPath), s.CacheDir,
		joinSorted(s.Include), joinSorted(s.Exclude), s.RespectGitignore, s.ForceExclude,
		s.AttributeSplit, joinSorted(s.AttributeSplitTags), s.CloseTagOnNewLine,
		s.EmptyElement, s.OdooTagSpacing, joinSorted(s.OdooSpacingTags),
		s.PreserveBlankLines, s.AttributeSort, s.IgnoreSuppressions,
		s.IndentStyle, s.LineEnding, s.FinalNewline, s.XMLDeclaration,
		s.EncodingStyle, s.QWebAttributesFirst,
		joinSorted(s.Fixable), joinSorted(s.Unfixable), s.Preview)
}

// joinSorted joins a copy of v in sorted order so that equivalent selector
// sets (in any order) produce the same key.
func joinSorted(v []string) string {
	if len(v) == 0 {
		return ""
	}
	c := append([]string(nil), v...)
	sort.Strings(c)
	return strings.Join(c, ",")
}
