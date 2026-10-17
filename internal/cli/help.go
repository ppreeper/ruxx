package cli

import (
	"fmt"
	"io"
)

// This file holds the per-command help screens. `ruxx <command> --help` (or
// -h) prints one and exits 0. The wording of the shared flags deliberately
// matches docs/SPEC.md §2.2 so the spec stays the source of truth.

type flagHelp struct {
	name string
	desc string
}

type flagGroup struct {
	title string
	flags []flagHelp
}

func writeFlagGroups(w io.Writer, groups []flagGroup) {
	for _, g := range groups {
		fmt.Fprintf(w, "\n%s:\n", g.title)
		for _, f := range g.flags {
			fmt.Fprintf(w, "  %-26s %s\n", f.name, f.desc)
		}
	}
}

func checkUsage(w io.Writer) {
	fmt.Fprint(w, `usage: ruxx check [flags] [paths...]

Lint XML files. With no paths, the current directory is linted. Paths may be
files or directories; "-" reads from stdin (see --stdin-filename).
`)
	writeFlagGroups(w, []flagGroup{
		{"Fixing", []flagHelp{
			{"--fix", "apply fixes (safe fixes only)"},
			{"--unsafe-fixes", "also apply fixes that may change behaviour"},
			{"--diff", "print a unified diff instead of writing (implies --fix)"},
			{"--fix-only", "apply fixes and report nothing"},
			{"--show-fixes", "summarise the fixes that were applied"},
			{"--add-ignore", `author suppressions for existing findings`},
			{"--fixable <SEL>", "limit which rules may fix"},
			{"--unfixable <SEL>", "remove rules from the fixable set (wins)"},
		}},
		{"Rule selection", []flagHelp{
			{"--select <SEL>", "replace the selected set (default: correctness)"},
			{"--extend-select <SEL>", "add to the selected set"},
			{"--ignore <SEL>", "remove from the selected set"},
			{"--extend-ignore <SEL>", "add to the ignored set"},
			{"--per-file-ignores <G=C>", "ignore rules for paths matching a glob"},
			{"--target-version <V>", `Odoo series, e.g. "17.0" (default: auto)`},
			{"--preview", "enable preview rules (never enabled in bulk)"},
			{"--ignore-suppressions", "report findings that ruxx: comments silence"},
		}},
		{"Output", []flagHelp{
			{"--output-format <FMT>", "concise, full, json, json-lines, github, gitlab, junit, sarif, azure, grouped"},
			{"-o, --output-file <FILE>", "write the report to a file (env RUXX_OUTPUT_FILE)"},
			{"--statistics", "print per-rule finding counts"},
			{"--show-files", "print the files that would be linted and exit"},
			{"--show-settings", "print the effective settings per file and exit"},
			{"--color <WHEN>", "auto, always, or never (also --no-color)"},
			{"-q, --quiet", "hide the fix summary"},
			{"-s, --silent", "write nothing to stdout (the exit code is unchanged)"},
			{"-v, --verbose", "write a timing summary to stderr"},
		}},
		{"File selection", []flagHelp{
			{"--exclude <GLOB>", "replace the default exclude list"},
			{"--extend-exclude <GLOB>", "add to the exclude list"},
			{"--force-exclude", "apply exclusions to explicit paths too"},
			{"--respect-gitignore", "honour ignore files (default)"},
			{"--no-respect-gitignore", "do not read ignore files"},
		}},
		{"Misc", []flagHelp{
			{"--config <FILE>", "use this config file for every path"},
			{"--isolated", "ignore all config files"},
			{"-n, --no-cache", "bypass the cache (env RUXX_NO_CACHE)"},
			{"--cache-dir <DIR>", "cache location (env RUXX_CACHE_DIR)"},
			{"-w, --watch", "re-run when files change"},
			{"--stdin-filename <NAME>", `the logical name of stdin ("-")`},
			{"--exit-zero", "exit 0 even when findings are reported"},
			{"--exit-non-zero-on-fix", "exit 1 when fixes were applied"},
			{"-h, --help", "print this help"},
		}},
	})
	fmt.Fprint(w, `
Exit codes:
  0  no findings (or every finding was fixed)
  1  findings were reported
  2  abnormal termination (bad flag, unreadable file, bad config)

Examples:
  ruxx check .
  ruxx check --select O004,ST001 --fix views/
  ruxx check --statistics --output-format sarif -o report.sarif .

Selectors: a rule code (O004), a prefix (O, XP0), a family, a category
(correctness, style, ...), or ALL. See "ruxx rule" and "ruxx config".
`)
}

func formatUsage(w io.Writer) {
	fmt.Fprint(w, `usage: ruxx format [flags] [paths...]

Format XML files in place. With no paths, the current directory is formatted.
Paths may be files or directories; "-" reads from stdin and writes stdout.
Formatting is idempotent and never introduces new lint findings.
`)
	writeFlagGroups(w, []flagGroup{
		{"Output mode", []flagHelp{
			{"--check", "do not write; exit 1 if any file would change"},
			{"--diff", "print a unified diff instead of writing"},
			{"--range <R>", "format only start_line:col-end_line:col"},
			{"--output-format <FMT>", "with --check: concise, json, github, sarif, ..."},
			{"-o, --output-file <FILE>", "write the report to a file (env RUXX_OUTPUT_FILE)"},
			{"--exit-non-zero-on-format", "exit 1 when a file was rewritten"},
			{"--exit-zero", "exit 0 even when files would change"},
			{"-q, --quiet", "hide the summary"},
			{"-s, --silent", "write nothing to stdout (the exit code is unchanged)"},
			{"-v, --verbose", "write a timing summary to stderr"},
		}},
		{"File selection", []flagHelp{
			{"--exclude <GLOB>", "replace the default exclude list"},
			{"--extend-exclude <GLOB>", "add to the exclude list"},
			{"--force-exclude", "apply exclusions to explicit paths too"},
			{"--respect-gitignore", "honour ignore files (default)"},
			{"--no-respect-gitignore", "do not read ignore files"},
		}},
		{"Misc", []flagHelp{
			{"--config <FILE>", "use this config file for every path"},
			{"--isolated", "ignore all config files"},
			{"--stdin-filename <NAME>", `the logical name of stdin ("-")`},
			{"-h, --help", "print this help"},
		}},
	})
	fmt.Fprint(w, `
Exit codes:
  0  nothing to do (or the file was written)
  1  --check found files that would change, or --exit-non-zero-on-format
     rewrote a file
  2  abnormal termination (bad flag, bad --range, unreadable file)

Examples:
  ruxx format .
  ruxx format --check --output-format json addons/
  ruxx format --diff views/account_view.xml

Layout is configured in .ruxx.toml under [format]; see "ruxx config".
`)
}

func cleanUsage(w io.Writer) {
	fmt.Fprint(w, `usage: ruxx clean [flags] [paths...]

Remove the ruxx caches (the diagnostics digest cache and the cross-file index)
for each path. With no paths, the current directory is used.
`)
	writeFlagGroups(w, []flagGroup{
		{"Misc", []flagHelp{
			{"--cache-dir <DIR>", "cache location to remove (env RUXX_CACHE_DIR)"},
			{"--config <FILE>", "resolve cache-dir from this config file"},
			{"-h, --help", "print this help"},
		}},
	})
	fmt.Fprint(w, `
Exit codes:
  0  the directories were removed (or did not exist)
  2  abnormal termination
`)
}

func serverUsage(w io.Writer) {
	fmt.Fprint(w, `usage: ruxx server [flags]

Run the language server: JSON-RPC 2.0 with LSP framing over stdio. The server
publishes diagnostics, formats documents and ranges, and offers quick fixes
for fixable findings. Editors should launch it as "ruxx server".

Flags configure how settings and the cross-file index are resolved:
`)
	writeFlagGroups(w, []flagGroup{
		{"Misc", []flagHelp{
			{"--config <FILE>", "use this config file for every document"},
			{"--isolated", "ignore all config files"},
			{"-n, --no-cache", "bypass the caches (env RUXX_NO_CACHE)"},
			{"--cache-dir <DIR>", "cache location (env RUXX_CACHE_DIR)"},
			{"-h, --help", "print this help"},
		}},
	})
	fmt.Fprint(w, `
See docs/EDITOR.md for Neovim, Helix, VS Code (editors/vscode) and pre-commit
setup.
`)
}
