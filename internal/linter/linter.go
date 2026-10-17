// Package linter orchestrates rule execution over a parsed tree and returns
// diagnostics. It owns the rule registry and the select/ignore resolution; it
// knows nothing about output, fixing, or the filesystem.
package linter

import (
	"github.com/ppreeper/ruxx/internal/diagnostic"
	"github.com/ppreeper/ruxx/internal/index"
	"github.com/ppreeper/ruxx/internal/rules"
	"github.com/ppreeper/ruxx/internal/settings"
	"github.com/ppreeper/ruxx/internal/syntax"
)

// Check runs the selected rules over tree and returns diagnostics in a stable
// order (see diagnostic.Sort).
func Check(tree *syntax.Tree, s *settings.Settings) []diagnostic.Diagnostic {
	return rules.Check(tree, s)
}

// CheckFile is Check with the file's path, so that Odoo-specific rules only run
// for Odoo source files. An empty path runs every rule.
func CheckFile(tree *syntax.Tree, path string, s *settings.Settings, ix *index.Index) []diagnostic.Diagnostic {
	return rules.CheckFile(tree, path, s, ix)
}
