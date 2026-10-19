# Changelog

All notable changes to ruxx are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.1] - 2026-10-18

### Fixed

- `ruxx server` now accepts the `--stdio` and `--clientProcessId` flags that
  LSP clients add when they launch a stdio server. VS Code's
  `vscode-languageclient` appends `--stdio` to an `Executable`, so the server
  previously exited with `unknown flag "--stdio"`, which surfaced in the editor
  as `write EPIPE` and a restart loop.

## [0.1.0] - 2026-10-17

First release. `ruxx check` and `ruxx format` share one lossless XML parser,
so the linter and the formatter always agree about what a file says.

### Added

- **Linter** — 83 rules across eight families (`X` XML, `O` Odoo, `QW` QWeb,
  `XP` XPath, `VW` view structure, `SEC` security, `ST` style, `RUX` meta),
  29 of them on by default (the `correctness` category). Rule codes are
  Flake8-style and selectable by code, numeric prefix band, family, or
  category, with the precedence `ALL < category < family < prefix < code`.
- **Formatter** — idempotent, with `attribute-split` (`auto`/`always`/`never`),
  `split-attributes-tags`, `attribute-sort` (`none`/`alphabetical`),
  `empty-element` (`self-closing`/`expand`), `close-tag-on-new-line`,
  `odoo-tag-spacing`, `odoo-spacing-tags`, `preserve-blank-lines`,
  `indent-width`, `indent-style`, `line-ending`, `final-newline`,
  `xml-declaration`, `encoding-style`, and `qweb-attributes-first`.
- **Fixes** — safe and unsafe edits with `--fix`, `--unsafe-fixes`,
  `--diff` (a real unified diff), `--fix-only`, and `--show-fixes`.
  Two rules rename duplicates (`X014`) and migrate `t-esc` to `t-out` (`QW001`).
- **Configuration** — `.ruxx.toml` (then `ruxx.toml`) with closest-wins
  discovery, `extend`, `[[overrides]]`, per-file ignores, and `[lint]`,
  `[format]`, and `[odoo]` sections.
- **Suppressions** — `<!-- ruxx: ignore[...] -->`, `disable`/`enable` regions,
  `fmt: off`/`on`/`skip`, and `--add-ignore` to author them.
- **Cross-file checks** — an opt-in module index (`[odoo].modules-path`) for
  duplicate xmlids, references to uninstalled modules, unknown `t-call`
  targets, and security files missing from the manifest.
- **Odoo awareness** — path-aware gating (static/tests are skipped) and
  version-sensitive rules for Odoo 15.0–20.0 (`<tree>`/`<list>`,
  `attrs`/`states`, `t-esc`/`t-raw`).
- **CLI** — `check` (default), `format`, `init`, `rule`, `config`, `clean`,
  `server`, `version`, `help`; stdin via `-`; exit codes 0/1/2; ten output
  formats (concise, full, grouped, json, json-lines, github, gitlab, junit,
  sarif, azure); colour control and `--no-color`.
- **Range formatting** — `ruxx format --range start_line:col-end_line:col`.
- **Language server** — `ruxx server` speaks LSP over stdio (diagnostics,
  document and range formatting, quick fixes) with a VS Code client in
  `editors/vscode`.
- **Integrations** — pre-commit hooks (`ruxx-check`, `ruxx-format`).
- **Performance** — parallel file processing plus caching: lint results in
  `.ruxx_cache/<digest>.json` and the cross-file index in
  `.ruxx_cache/index.json`. 45,356 real Odoo XML files lint in ~7.7 s
  (see `docs/PERFORMANCE.md`).

### Notes

- Validated against ~45k Odoo XML files across 15.0–20.0 (community and
  enterprise). The findings that remain are genuine: duplicate xmlids,
  `data-tooltip` in an arch, and empty `position="attributes"` specs.
- Only `.gitignore` is honored for file selection; `.ignore`,
  `.git/info/exclude`, and the global gitignore are not read yet.

[Unreleased]: https://github.com/ppreeper/ruxx/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/ppreeper/ruxx/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/ppreeper/ruxx/releases/tag/v0.1.0
