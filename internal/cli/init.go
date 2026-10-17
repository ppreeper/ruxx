package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// configTemplate is the starter .ruxx.toml written by `ruxx init`. It is valid
// TOML and loads cleanly through internal/config; keys ruxx does not read yet
// appear only as comments so the file never lies about what is supported.
const configTemplate = `# .ruxx.toml — ruxx configuration.
#
# ruxx discovers the nearest .ruxx.toml (or ruxx.toml) walking up from each
# file, so one config at the project root covers the whole tree. Relative paths
# resolve against the directory holding this file.
# Full reference: docs/SPEC.md §3.

line-length = 88
indent-width = 4

# Odoo series the project targets. "auto" infers it from the path (a "18.0"
# segment, say); pin it to settle the version-gated rules on unversioned trees.
# Values: "auto", "15.0", "16.0", "17.0", "18.0", "19.0", "20.0".
target-version = "auto"

# File discovery. include/exclude REPLACE the built-in lists;
# extend-include/extend-exclude ADD to them.
include = ["*.xml"]
exclude = [".git", ".hg", ".svn", "node_modules", ".ruxx_cache", "build", "dist", "__pycache__", ".venv", "venv"]
respect-gitignore = true
# force-exclude = false

[lint]
# Selectors: exact codes (O004), numeric bands (XP0), families (O, XP, QW),
# categories (correctness, suspicious, style, security, pedantic, restriction,
# formatting), or "ALL". The default set is correctness.
select = ["correctness"]
ignore = []

# Per-pattern exceptions, e.g.:
# [lint.per-file-ignores]
# "**/demo/**" = ["O004"]

[odoo]
# Opt in to the cross-file checks (O016, O018, QW012, SEC002, SEC003) by
# pointing at your addons roots. Leave unset to skip project-wide analysis.
# modules-path = ["addons", "enterprise"]

# [format] — formatter options (all optional; the values below are defaults).
# [format]
# attribute-split        = "auto"        # auto | always | never
# split-attributes-tags  = []            # tags whose attrs always split
# close-tag-on-new-line  = false         # put a leaf's closing tag on its own line
# preserve-blank-lines   = true          # keep one author-written blank line
# attribute-sort         = "none"        # none | alphabetical
# indent-style           = "space"       # space | tab
# line-ending            = "auto"        # auto | lf | crlf
# final-newline          = true          # end the file with exactly one newline
# xml-declaration        = "preserve"    # preserve | always | never
# encoding-style         = "lower"       # lower | upper | preserve
# qweb-attributes-first  = true          # put t-* attributes before plain ones
# empty-element          = "self-closing" # self-closing | expand
# odoo-tag-spacing       = true          # blank line before grouped Odoo blocks
# odoo-spacing-tags      = ["record", "menuitem", "template"]
`

// runInit writes a starter .ruxx.toml.
func runInit(args []string, stdout, stderr io.Writer) int {
	dir, force, err := parseInitArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "ruxx: %v\n", err)
		return ExitAbnormal
	}
	target := filepath.Join(dir, ".ruxx.toml")
	if _, err := os.Stat(target); err == nil && !force {
		fmt.Fprintf(stderr, "ruxx: %s already exists (use --force to overwrite)\n", target)
		return ExitAbnormal
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(stderr, "ruxx: %v\n", err)
		return ExitAbnormal
	}
	if err := os.WriteFile(target, []byte(configTemplate), 0o644); err != nil {
		fmt.Fprintf(stderr, "ruxx: %v\n", err)
		return ExitAbnormal
	}
	fmt.Fprintf(stdout, "created %s\n", target)
	return ExitClean
}

// parseInitArgs reads `ruxx init [dir] [--force]`.
func parseInitArgs(args []string) (dir string, force bool, err error) {
	dir = "."
	for _, a := range args {
		switch {
		case a == "--force" || a == "-f":
			force = true
		case a == "-h" || a == "--help":
			return "", false, fmt.Errorf("usage: ruxx init [dir] [--force]")
		case strings.HasPrefix(a, "-"):
			return "", false, fmt.Errorf("unknown flag %q", a)
		default:
			if dir != "." {
				return "", false, fmt.Errorf("usage: ruxx init [dir] [--force]")
			}
			dir = a
		}
	}
	return dir, force, nil
}
