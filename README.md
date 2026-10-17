# ruxx

[![CI](https://github.com/ppreeper/ruxx/actions/workflows/ci.yml/badge.svg)](https://github.com/ppreeper/ruxx/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ppreeper/ruxx)](https://github.com/ppreeper/ruxx/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**XML linter + formatter, in the spirit of [Ruff](https://docs.astral.sh/ruff/) — written in Go, with first-class Odoo XML support.**

`ruxx` is a single, fast, self-contained binary that lints and formats XML.
It brings the ergonomics developers expect from Ruff to XML:

- one `check` and one `format` command sharing one lossless parser
- Flake8-style rule codes (`X001`, `O004`, `QW001`, …), selectable by code,
  prefix, group, or category
- safe/unsafe automatic fixes, `--fix`, `--diff`, stdin, caching (`.ruxx_cache`), watch mode (`-w`)
- `--fixable`/`--unfixable` control over which rules may fix, `--add-ignore`
  suppression authoring, and `--show-files`/`--show-settings` introspection
- a self-documenting CLI: `-h`/`--help` on every subcommand, `ruxx rule` to
  explain a rule, `ruxx config` to list every setting, and `ruxx
  generate-shell-completion` for bash/zsh/fish/PowerShell
- a `.ruxx.toml` config file with hierarchical discovery, scaffolded by `ruxx init`
- gitignore-aware file discovery (`include`/`exclude` globs, `--exclude`,
  `--extend-exclude`, `--force-exclude`, `--show-files`), reading `.ignore`,
  `.gitignore`, `.git/info/exclude`, and the user-level gitignore
- a formatter with the odoo-shortcuts option surface (`attribute-split`,
  `attribute-sort`, `indent-style`, `line-ending`, `xml-declaration`,
  `encoding-style`, `empty-element`, tag spacing) plus `ruxx format --range`
- an opt-in cross-file index (`[odoo].modules-path`) for duplicate-xmlid,
  uninstalled-module, `t-call` target, and security-file checks; the index is
  persisted to `.ruxx_cache/index.json` and reused until a module's files change
- a language server (`ruxx server`) for editor integration, with a bundled
  [VS Code client](editors/vscode) for diagnostics, quick fixes, and formatting
- `ruxx rule CODE` to explain any rule and `ruxx config` to list every setting's
  default, plus [editor & CI setup](docs/EDITOR.md) and
  [pre-commit hooks](.pre-commit-hooks.yaml) (`ruxx-check`, `ruxx-format`)

On top of generic XML quality, `ruxx` understands **Odoo**: records, fields,
menus, XPath view inheritance, QWeb/OWL templates, and version-sensitive rules
for Odoo 15.0–20.0. The Odoo hints are distilled from the
[`odoo-shortcuts`](https://github.com/mjavint/odoo-shortcuts) extension and
Odoo's own loader/validation source.

## Install

```console
# with Go 1.27+
go install github.com/ppreeper/ruxx/cmd/ruxx@latest
```

Prebuilt binaries for Linux, macOS, and Windows (amd64 and arm64) are attached
to each [release](https://github.com/ppreeper/ruxx/releases), with a
`checksums.txt` to verify them.

Pre-commit ([`.pre-commit-hooks.yaml`](.pre-commit-hooks.yaml)) builds the
binary from source, so nothing needs to be installed first:

```yaml
repos:
  - repo: https://github.com/ppreeper/ruxx
    rev: v0.1.0
    hooks:
      - id: ruxx-check
      - id: ruxx-format
```

For editors, build the VS Code client in [`editors/vscode`](editors/vscode)
(`npm install && npm run package`, then install the `.vsix`) or run it from
source with F5; Neovim and Helix setups are in [`docs/EDITOR.md`](docs/EDITOR.md).

## Status

**Implemented.** `ruxx check` and `ruxx format` share one lossless parser and
all 83 catalogued rules are live. The fix engine is bounded and reverts on
breakage, the formatter is idempotent, results are cacheable, `--output-format`
covers ten CI-friendly formats, and `ruxx server` speaks LSP over stdio. The
test suite is green across 17 packages.

## Documentation

- [`docs/SPEC.md`](docs/SPEC.md) — the full specification: CLI, config,
  architecture, formatter, testing, roadmap.
- [`docs/RULES.md`](docs/RULES.md) — the rule catalog (83 rules).
- [`docs/PERFORMANCE.md`](docs/PERFORMANCE.md) — benchmarks and corpus timings.
- [`docs/EDITOR.md`](docs/EDITOR.md) — editor (LSP), CI, and pre-commit integration.
- [`docs/ODOO-HINTS.md`](docs/ODOO-HINTS.md) — Odoo-specific hints and their
  provenance.

## Quick example

```console
$ ruxx check addons/sale/views/sale_order_views.xml
addons/sale/views/sale_order_views.xml:12:5: O004 <record> is missing the required "model" attribute
addons/sale/views/sale_order_views.xml:40:9: XP001 <xpath> is missing a non-empty "expr"

$ ruxx check --fix .
$ ruxx check --statistics .
   3  X007
   1  O004

$ ruxx format --check addons/
would reformat addons/sale/views/sale_order_views.xml
```

## Language server

`ruxx server` speaks the Language Server Protocol over stdio and can back
lint/format in any LSP-aware editor:

- `textDocument/publishDiagnostics` on open/change (same rules as `check`)
- `textDocument/formatting` and `textDocument/rangeFormatting` (idempotent)
- `textDocument/codeAction` quick-fixes for fixable diagnostics

A VS Code client lives in [`editors/vscode`](editors/vscode); Neovim, Helix and
the CLI-in-a-task setup are in [`docs/EDITOR.md`](docs/EDITOR.md), which also
covers the [`pre-commit` hooks](.pre-commit-hooks.yaml) (`ruxx-check`,
`ruxx-format`) for gating a repository.

## Configuration (`.ruxx.toml`)

```toml
line-length = 88
target-version = "17.0"

[odoo]
modules-path = ["addons", "enterprise"]

[lint]
select = ["correctness", "odoo", "qweb", "xpath"]
fixable = ["ALL"]        # which rules may apply fixes
unfixable = []           # removed from the fixable set (wins)

[format]
attribute-split = "auto"
split-attributes-tags = []
attribute-sort = "none"
indent-style = "space"
line-ending = "auto"
final-newline = true
xml-declaration = "preserve"
encoding-style = "lower"
qweb-attributes-first = true
close-tag-on-new-line = false
preserve-blank-lines = true
odoo-tag-spacing = true
empty-element = "self-closing"
```

See [`docs/SPEC.md`](docs/SPEC.md) §3 for the complete schema.

## Development

```console
make build      # bin/ruxx, with the version stamped in from git
make test       # the full suite
make race       # ...under the race detector
make lint       # gofmt check + go vet
make bench      # micro-benchmarks (+ a corpus sweep when RXX_CORPUS is set)
```

Releases are cut by pushing a `v*` tag:

```console
git tag -a v0.1.0 -m "ruxx v0.1.0"
git push origin v0.1.0
```

The [release workflow](.github/workflows/release.yml) then runs goreleaser and
publishes archives for Linux, macOS, and Windows on amd64 and arm64, plus
`checksums.txt`. `ruxx version` reports the version and commit stamped into
the binary (see `internal/version`).

## License

See [LICENSE](LICENSE).
