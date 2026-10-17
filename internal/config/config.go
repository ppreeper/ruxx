// Package config discovers and parses `.ruxx.toml` configuration.
//
// Discovery is Ruff-style "closest wins": for a given file, walk up the
// directory tree and use the nearest `.ruxx.toml` (or, failing that,
// `ruxx.toml`). Configs are NOT merged across directories; a directory-level
// file replaces the parent entirely unless it uses `extend`.
//
// Relative paths inside a config (exclude globs, extend, src) resolve against
// the directory containing that config file.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/ppreeper/ruxx/internal/settings"
)

// Override is a glob-scoped settings patch declared with [[overrides]].
type Override struct {
	// Include/Exclude select which files the override applies to.
	Include []string
	Exclude []string
	// Settings is the patch applied on top of the base settings.
	Settings *settings.Settings
}

// Config is one resolved configuration (a file, or the built-in defaults when
// Path is empty).
type Config struct {
	// Path is the absolute path of the config file; empty for defaults.
	Path string
	// Dir is the directory relative paths resolve against ("" for defaults).
	Dir string

	Settings       *settings.Settings
	PerFileIgnores map[string][]string
	Overrides      []Override

	// Extends is the resolved absolute path named by `extend`, or "".
	Extends string

	// patches holds the field-level override data needed to merge [[overrides]]
	// onto the base settings in SettingsFor.
	patches []overridePatch
}

// overridePatch records which setting fields a [[overrides]] entry set, so that
// unset fields inherit from the base settings rather than a default.
type overridePatch struct {
	include, exclude []string
	selects, ignores []string
	lineLength       *int
	indentWidth      *int
	targetVersion    string
}

// Default returns the built-in defaults with no config file.
func Default() *Config {
	return &Config{Settings: settings.Default()}
}

// tomlConfig mirrors the `.ruxx.toml` schema.
type tomlConfig struct {
	LineLength       *int           `toml:"line-length"`
	IndentWidth      *int           `toml:"indent-width"`
	TargetVersion    string         `toml:"target-version"`
	CacheDir         string         `toml:"cache-dir"`
	Include          []string       `toml:"include"`
	ExtendInclude    []string       `toml:"extend-include"`
	Exclude          []string       `toml:"exclude"`
	ExtendExclude    []string       `toml:"extend-exclude"`
	RespectGitignore *bool          `toml:"respect-gitignore"`
	ForceExclude     bool           `toml:"force-exclude"`
	Preview          *bool          `toml:"preview"`
	Extend           string         `toml:"extend"`
	Lint             tomlLint       `toml:"lint"`
	Odoo             tomlOdoo       `toml:"odoo"`
	Format           tomlFormat     `toml:"format"`
	Overrides        []tomlOverride `toml:"overrides"`
}

type tomlOdoo struct {
	ModulesPath []string `toml:"modules-path"`
}

// tomlFormat mirrors the `[format]` section. Pointer fields distinguish
// "absent" from an explicit false.
type tomlFormat struct {
	AttributeSplit      string   `toml:"attribute-split"`
	AttributeSplitTags  []string `toml:"split-attributes-tags"`
	CloseTagOnNewLine   *bool    `toml:"close-tag-on-new-line"`
	EmptyElement        string   `toml:"empty-element"`
	OdooTagSpacing      *bool    `toml:"odoo-tag-spacing"`
	OdooSpacingTags     []string `toml:"odoo-spacing-tags"`
	PreserveBlankLines  *bool    `toml:"preserve-blank-lines"`
	AttributeSort       string   `toml:"attribute-sort"`
	IndentStyle         string   `toml:"indent-style"`
	LineEnding          string   `toml:"line-ending"`
	FinalNewline        *bool    `toml:"final-newline"`
	XMLDeclaration      string   `toml:"xml-declaration"`
	EncodingStyle       string   `toml:"encoding-style"`
	QWebAttributesFirst *bool    `toml:"qweb-attributes-first"`
	PreserveComments    *bool    `toml:"preserve-comments"`
}

type tomlLint struct {
	Select             []string            `toml:"select"`
	Ignore             []string            `toml:"ignore"`
	Fixable            []string            `toml:"fixable"`
	Unfixable          []string            `toml:"unfixable"`
	PerFileIgnores     map[string][]string `toml:"per-file-ignores"`
	IgnoreSuppressions *bool               `toml:"ignore-suppressions"`
}

type tomlOverride struct {
	Include       []string `toml:"include"`
	Exclude       []string `toml:"exclude"`
	Select        []string `toml:"select"`
	Ignore        []string `toml:"ignore"`
	LineLength    *int     `toml:"line-length"`
	IndentWidth   *int     `toml:"indent-width"`
	TargetVersion string   `toml:"target-version"`
}

// configNames are the recognized file names, in precedence order.
var configNames = []string{".ruxx.toml", "ruxx.toml"}

// Discover finds the config governing forPath by walking up from its directory.
// It returns Default() (Path == "") when no config file is found.
func Discover(forPath string) (*Config, error) {
	abs, err := filepath.Abs(forPath)
	if err != nil {
		return nil, err
	}
	return DiscoverFrom(filepath.Dir(abs))
}

// DiscoverFrom finds the closest config starting from a directory, walking up
// towards the filesystem root, and returns the built-in defaults when nothing
// is found. Callers that resolve many files can memoise per directory and skip
// the repeated parent stats.
func DiscoverFrom(dir string) (*Config, error) {
	for {
		for _, name := range configNames {
			candidate := filepath.Join(dir, name)
			if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
				return Load(candidate)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Default(), nil
		}
		dir = parent
	}
}

// Load parses a specific config file, applying `extend`.
func Load(path string) (*Config, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}

	var tc tomlConfig
	if _, err := toml.Decode(string(data), &tc); err != nil {
		return nil, err
	}

	cfg := &Config{
		Path:     abs,
		Dir:      filepath.Dir(abs),
		Settings: settings.Default(),
	}

	// `extend` inherits the named config's settings as the base.
	if tc.Extend != "" {
		ext := tc.Extend
		if !filepath.IsAbs(ext) {
			ext = filepath.Join(cfg.Dir, ext)
		}
		ext = filepath.Clean(ext)
		cfg.Extends = ext
		parent, err := Load(ext)
		if err != nil {
			return nil, err
		}
		inherited := *parent.Settings
		cfg.Settings = &inherited
	}

	// Apply the local (possibly inherited) settings on top.
	if tc.LineLength != nil {
		cfg.Settings.LineLength = *tc.LineLength
	}
	if tc.IndentWidth != nil {
		cfg.Settings.IndentWidth = *tc.IndentWidth
	}
	if tc.TargetVersion != "" {
		cfg.Settings.TargetVersion = tc.TargetVersion
	}
	if tc.CacheDir != "" {
		cd := tc.CacheDir
		if !filepath.IsAbs(cd) {
			cd = filepath.Join(cfg.Dir, cd)
		}
		cfg.Settings.CacheDir = filepath.Clean(cd)
	}
	if tc.Lint.Select != nil {
		cfg.Settings.Select = tc.Lint.Select
	}
	if tc.Lint.Ignore != nil {
		cfg.Settings.Ignore = tc.Lint.Ignore
	}
	if tc.Lint.PerFileIgnores != nil {
		cfg.PerFileIgnores = tc.Lint.PerFileIgnores
	}
	if tc.Lint.IgnoreSuppressions != nil {
		cfg.Settings.IgnoreSuppressions = *tc.Lint.IgnoreSuppressions
	}
	if tc.Lint.Fixable != nil {
		cfg.Settings.Fixable = append([]string{}, tc.Lint.Fixable...)
	}
	if tc.Lint.Unfixable != nil {
		cfg.Settings.Unfixable = append([]string{}, tc.Lint.Unfixable...)
	}
	if tc.Preview != nil {
		cfg.Settings.Preview = *tc.Preview
	}
	if len(tc.Odoo.ModulesPath) > 0 {
		paths := make([]string, 0, len(tc.Odoo.ModulesPath))
		for _, p := range tc.Odoo.ModulesPath {
			if !filepath.IsAbs(p) {
				p = filepath.Join(cfg.Dir, p)
			}
			paths = append(paths, filepath.Clean(p))
		}
		cfg.Settings.ModulesPath = paths
	}

	// File selection: `include`/`exclude` replace the built-in lists,
	// `extend-include`/`extend-exclude` append to them.
	if tc.Include != nil {
		cfg.Settings.Include = append([]string{}, tc.Include...)
	}
	if len(tc.ExtendInclude) > 0 {
		cfg.Settings.Include = append(append([]string{}, cfg.Settings.Include...), tc.ExtendInclude...)
	}
	if tc.Exclude != nil {
		cfg.Settings.Exclude = append([]string{}, tc.Exclude...)
	}
	if len(tc.ExtendExclude) > 0 {
		cfg.Settings.Exclude = append(append([]string{}, cfg.Settings.Exclude...), tc.ExtendExclude...)
	}
	if tc.RespectGitignore != nil {
		cfg.Settings.RespectGitignore = *tc.RespectGitignore
	}
	if tc.ForceExclude {
		cfg.Settings.ForceExclude = true
	}

	// Formatting: `[format]` mirrors the formatter options.
	if tc.Format.AttributeSplit != "" {
		cfg.Settings.AttributeSplit = tc.Format.AttributeSplit
	}
	if tc.Format.AttributeSplitTags != nil {
		cfg.Settings.AttributeSplitTags = append([]string{}, tc.Format.AttributeSplitTags...)
	}
	if tc.Format.CloseTagOnNewLine != nil {
		cfg.Settings.CloseTagOnNewLine = *tc.Format.CloseTagOnNewLine
	}
	if tc.Format.EmptyElement != "" {
		cfg.Settings.EmptyElement = tc.Format.EmptyElement
	}
	if tc.Format.OdooTagSpacing != nil {
		cfg.Settings.OdooTagSpacing = *tc.Format.OdooTagSpacing
	}
	if tc.Format.OdooSpacingTags != nil {
		cfg.Settings.OdooSpacingTags = append([]string{}, tc.Format.OdooSpacingTags...)
	}
	if tc.Format.PreserveBlankLines != nil {
		cfg.Settings.PreserveBlankLines = *tc.Format.PreserveBlankLines
	}
	if tc.Format.AttributeSort != "" {
		cfg.Settings.AttributeSort = tc.Format.AttributeSort
	}
	if tc.Format.IndentStyle != "" {
		cfg.Settings.IndentStyle = tc.Format.IndentStyle
	}
	if tc.Format.LineEnding != "" {
		cfg.Settings.LineEnding = tc.Format.LineEnding
	}
	if tc.Format.FinalNewline != nil {
		cfg.Settings.FinalNewline = *tc.Format.FinalNewline
	}
	if tc.Format.XMLDeclaration != "" {
		cfg.Settings.XMLDeclaration = tc.Format.XMLDeclaration
	}
	if tc.Format.EncodingStyle != "" {
		cfg.Settings.EncodingStyle = tc.Format.EncodingStyle
	}
	if tc.Format.QWebAttributesFirst != nil {
		cfg.Settings.QWebAttributesFirst = *tc.Format.QWebAttributesFirst
	}
	if tc.Format.PreserveComments != nil && !*tc.Format.PreserveComments {
		return nil, fmt.Errorf("config %s: preserve-comments must be true; ruxx never removes comments", cfg.Path)
	}
	switch cfg.Settings.AttributeSplit {
	case "auto", "always", "never":
	default:
		return nil, fmt.Errorf("config %s: attribute-split must be auto, always, or never (got %q)", cfg.Path, cfg.Settings.AttributeSplit)
	}
	switch cfg.Settings.EmptyElement {
	case "self-closing", "expand":
	default:
		return nil, fmt.Errorf("config %s: empty-element must be self-closing or expand (got %q)", cfg.Path, cfg.Settings.EmptyElement)
	}
	switch cfg.Settings.AttributeSort {
	case "none", "alphabetical":
	default:
		return nil, fmt.Errorf("config %s: attribute-sort must be none or alphabetical (got %q)", cfg.Path, cfg.Settings.AttributeSort)
	}
	switch cfg.Settings.IndentStyle {
	case "space", "tab":
	default:
		return nil, fmt.Errorf("config %s: indent-style must be space or tab (got %q)", cfg.Path, cfg.Settings.IndentStyle)
	}
	switch cfg.Settings.LineEnding {
	case "auto", "lf", "crlf":
	default:
		return nil, fmt.Errorf("config %s: line-ending must be auto, lf, or crlf (got %q)", cfg.Path, cfg.Settings.LineEnding)
	}
	switch cfg.Settings.XMLDeclaration {
	case "preserve", "always", "never":
	default:
		return nil, fmt.Errorf("config %s: xml-declaration must be preserve, always, or never (got %q)", cfg.Path, cfg.Settings.XMLDeclaration)
	}
	switch cfg.Settings.EncodingStyle {
	case "lower", "upper", "preserve":
	default:
		return nil, fmt.Errorf("config %s: encoding-style must be lower, upper, or preserve (got %q)", cfg.Path, cfg.Settings.EncodingStyle)
	}

	for _, o := range tc.Overrides {
		patch := *settings.Default()
		if o.Select != nil {
			patch.Select = o.Select
		}
		if o.Ignore != nil {
			patch.Ignore = o.Ignore
		}
		cfg.Overrides = append(cfg.Overrides, Override{
			Include:  o.Include,
			Exclude:  o.Exclude,
			Settings: &patch,
		})
		cfg.patches = append(cfg.patches, overridePatch{
			include:       o.Include,
			exclude:       o.Exclude,
			selects:       o.Select,
			ignores:       o.Ignore,
			lineLength:    o.LineLength,
			indentWidth:   o.IndentWidth,
			targetVersion: o.TargetVersion,
		})
	}

	return cfg, nil
}

// SettingsFor returns the effective settings for path: the base settings with
// any matching [[overrides]] applied, plus the codes of every matching
// [lint.per-file-ignores] glob appended to Ignore. Globs match the path as
// given and, when the path is inside the config directory, its path relative to
// that directory (the project root).
func (c *Config) SettingsFor(path string) *settings.Settings {
	out := *c.Settings
	rel := c.relative(path)
	if len(c.PerFileIgnores) > 0 {
		globs := make([]string, 0, len(c.PerFileIgnores))
		for glob := range c.PerFileIgnores {
			globs = append(globs, glob)
		}
		sort.Strings(globs) // deterministic, cache-stable ordering
		for _, glob := range globs {
			if !matchGlob(glob, rel) && !matchGlob(glob, path) {
				continue
			}
			// Copy before appending: out.Ignore aliases c.Settings.Ignore.
			out.Ignore = append(append([]string{}, out.Ignore...), c.PerFileIgnores[glob]...)
		}
	}
	for _, p := range c.patches {
		if !p.matches(rel) && !p.matches(path) {
			continue
		}
		if p.selects != nil {
			out.Select = p.selects
		}
		if p.ignores != nil {
			out.Ignore = p.ignores
		}
		if p.lineLength != nil {
			out.LineLength = *p.lineLength
		}
		if p.indentWidth != nil {
			out.IndentWidth = *p.indentWidth
		}
		if p.targetVersion != "" {
			out.TargetVersion = p.targetVersion
		}
	}
	return &out
}

// relative returns path relative to the config directory when path lies inside
// it, so globs resolve against the project root. Otherwise it returns path
// unchanged.
func (c *Config) relative(path string) string {
	if c.Dir == "" || path == "" {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(c.Dir, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}

// matches reports whether path is covered by this override.
func (p overridePatch) matches(path string) bool {
	if len(p.include) > 0 {
		ok := false
		for _, pat := range p.include {
			if matchGlob(pat, path) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	for _, pat := range p.exclude {
		if matchGlob(pat, path) {
			return false
		}
	}
	return true
}

// MatchGlob reports whether path matches a glob pattern supporting `*`, `?`,
// and `**` (which crosses directory separators). It is the matcher shared by
// [[overrides]] include/exclude, [lint.per-file-ignores], and the
// --per-file-ignores flag.
func MatchGlob(pattern, path string) bool {
	return matchGlob(pattern, path)
}

// matchGlob reports whether path matches a glob pattern supporting `*`, `?`,
// and `**` (which crosses directory separators).
func matchGlob(pattern, path string) bool {
	re := globToRegexp(pattern)
	if re == nil {
		return false
	}
	return re.MatchString(path)
}

func globToRegexp(pattern string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil
	}
	return re
}
