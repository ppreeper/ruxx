// Command ruxx is an XML linter and formatter, with first-class support for
// Odoo XML. See docs/SPEC.md for the design.
//
// Usage:
//
//	ruxx check [paths...]    lint (default when no subcommand is given)
//	ruxx format [paths...]   reformat
//
// Exit codes: 0 clean, 1 findings, 2 abnormal.
package main

import (
	"os"

	"github.com/ppreeper/ruxx/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
