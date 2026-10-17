# ruxx — Specification

> An extremely fast XML linter and formatter, written in Go, in the spirit of
> [Ruff](https://docs.astral.sh/ruff/), with first-class support for Odoo XML.

Status: **draft v0.1** — this document is the authoritative design. It is
intentionally opinionated. Sections marked **[decision]** record a choice made
here; sections marked **[open]** are unresolved and need review before
implementation.

Companion documents:

- [`RULES.md`](./RULES.md) — the full rule catalog (codes, severity, fixes).
- [`ODOO-HINTS.md`](./ODOO-HINTS.md) — Odoo-specific hints and their provenance.

---

## 1. Overview

`ruxx` is a single, self-contained Go binary exposing two tools behind one
interface:

- **`ruxx check`** — a linter. It parses XML into a lossless tree, runs a large
  set of rules organized by namespace, and reports diagnostics with byte-exact
  spans. Many diagnostics carry **safe** or **unsafe** automatic fixes.
- **`ruxx format`** — an opinionated formatter. It reprints XML from a
  lossless parse tree using a width-aware document model. It is designed to be
  **idempotent** and to **never introduce new lint errors**.

Around those two cores sit the Ruff-isms that make the tool pleasant:

- A single `.ruxx.toml` configuration file with hierarchical discovery.
- Flake8-style **rule codes** (`X001`, `O004`, `QW001`, …) selectable by code,
  prefix, group, or category.
- Suppression directives written as XML comments.
- `--fix`, safe/unsafe fix taxonomy, `--diff`, stdin (`-`), caching, watch mode.
- A stable exit-code contract (`0` clean, `1` findings, `2` abnormal).
- Machine-readable output (JSON, SARIF, GitHub, GitLab, JUnit).

### 1.1 Goals

1. **Correctness first.** Catch the XML and Odoo mistakes that actually break
   module loading or views, before the server does.
2. **Speed.** Fast enough to run on a whole Odoo addons tree on every save.
   Target: parse + lint + format a 20k-file tree in well under a few seconds.
3. **One binary, no runtime.** No Python, no Node, no plugins. `go build` and
   ship.
4. **Opinionated, batteries-included defaults.** Zero config catches real bugs;
   deeper style guidance is one `extend-select` away.
5. **Odoo-aware, not Odoo-locked.** Generic XML quality works everywhere; the
   Odoo rules/hints activate for Odoo files.

### 1.2 Non-goals (v1)

- **Not** a general-purpose schema validator (no XSD/RNG validation).
- **Not** a replacement for `xmllint`'s canonicalization, nor an XSLT engine.
- **Not** a full Odoo ORM emulator. Cross-file checks are best-effort and
  opt-in (they need a project index).
- **Not** a Python/CSV/JS formatter; those file kinds are out of scope except
  as data for cross-file checks (`__manifest__.py`, `ir.model.access.csv`).

### 1.3 Name

Binary and config stem: `ruxx`. Module path:
`github.com/ppreeper/ruxx`. Cache dir: `.ruxx_cache/`. Suppression prefix:
`ruxx:`. **[decision]**

---

## 2. Command-line interface

### 2.1 Subcommands

```
ruxx check      [PATH]...        # lint (default when no subcommand)
ruxx format     [PATH]...        # format
ruxx rule       [CODE|SELECTOR]  # explain a rule, or list rules — implemented
ruxx config     [KEY]            # list or describe configuration keys — implemented
ruxx init                        # write a starter .ruxx.toml — implemented
ruxx clean                       # clear cache(s)
ruxx server                      # language server (LSP over stdio) — implemented
ruxx version
ruxx help
ruxx generate-shell-completion <bash|zsh|fish|powershell>
```

Every subcommand accepts `-h`/`--help` and prints a usage screen (exit 0) that
documents its flags, exit codes and examples; `ruxx help` lists the commands.
`ruxx generate-shell-completion` prints a completion script covering the
subcommands, every flag, the output formats, the target versions, the colour
modes, and rule selectors (ALL, categories, families, and rule codes).

When invoked with no subcommand, `ruxx` behaves as `ruxx check`. **[decision]**

### 2.2 `ruxx check` flags

| Flag | Meaning |
|---|---|
| `--fix` / `--no-fix` | apply fixes (safe by default) — **implemented** |
| `--unsafe-fixes` / `--no-unsafe-fixes` | include unsafe fixes — **implemented** |
| `--show-fixes` | summarize fixes applied — **implemented** |
| `--diff` | print a unified diff instead of writing (implies `--fix-only`) — **implemented** |
| `--fix-only` | apply fixes, do not report remaining diagnostics — **implemented** |
| `--select <SEL>` | comma list of codes/prefixes/groups/categories/`ALL` — **implemented** |
| `--ignore <SEL>` | remove from the selected set — **implemented** |
| `--extend-select <SEL>` | add to the selected set — **implemented** |
| `--extend-ignore <SEL>` | add to the ignored set — **implemented** |
| `--per-file-ignores <GLOB:CODE[,CODE...]>` | per-glob ignores from the CLI; repeat the flag for several globs — **implemented** |
| `--fixable <SEL>` / `--unfixable <SEL>` | limit which rules may fix; `--unfixable` wins — **implemented** |
| `--add-ignore [REASON]` / `--add-noqa [REASON]` | author suppressions for existing findings — **implemented** |
| `--ignore-suppressions` | ignore all `ruxx:` directives (audit mode) — **implemented** |
| `--output-format <FMT>` | `concise`,`full`,`json`,`json-lines`,`github`,`gitlab`,`junit`,`sarif`,`grouped`,`azure` |
| `-o, --output-file <FILE>` | write report to file (env `RUXX_OUTPUT_FILE`) |
| `--statistics` | per-rule counts |
| `--exit-zero` | always exit 0 on findings |
| `--exit-non-zero-on-fix` | exit 1 if anything was fixed — **implemented** |
| `--target-version <V>` | Odoo series, e.g. `17.0` — **implemented** |
| `--exclude` / `--extend-exclude` / `--force-exclude` | file selection; `--force-exclude` also applies `exclude` to paths given explicitly |
| `--respect-gitignore` / `--no-respect-gitignore` | gitignore handling |
| `-n, --no-cache` / `--cache-dir <DIR>` | cache control (env `RUXX_NO_CACHE`, `RUXX_CACHE_DIR`) |
| `-w, --watch` | re-run on file changes |
| `--color <auto\|always\|never>` | colour control (also `--no-color`) |
| `--stdin-filename <NAME>` | logical path when reading from `-` |
| `--show-files` / `--show-settings` | print the selected files, or the effective settings per file — **implemented** |
| `--preview` | gate for experimental rules: never enabled by prefix/category/`ALL`, and selecting one by code without it is an error — **implemented** |
| `--isolated` | ignore all config files |
| `--config <FILE|KEY=VALUE>` | override config |
| `-v/-q/-s` | verbosity: `-v` writes a timing summary to stderr, `-q` hides the fix summary, `-s` silences stdout (the exit code is unchanged) — **implemented** |

### 2.3 `ruxx format` flags

`--check`, `--diff`, `--line-length`, `--indent-width`,
`--range <start_line>:<col>-<end_line>:<col>` (range/selection formatting),
`--stdin-filename`, `--exit-non-zero-on-format`, plus the shared file-selection,
cache, config, and global flags. **[decision]**

`--check` and `--diff` accept `--output-format` and `-o/--output-file` too, so a
CI job can consume the formatter verdict (e.g. `ruxx format --check
--output-format json`) from the same ten formats the linter uses.
**[implemented]**

Both `ruxx check --diff` and `ruxx format --diff` print a real unified diff —
`--- path` / `+++ path` headers, `@@ -l,c +l,c @@` hunks with three lines of
context, and `\ No newline at end of file` where it applies — instead of
writing. `check --diff` implies `--fix`: the fixes are applied in memory and
shown as a patch. **[implemented]**

`--range start_line:col-end_line:col` (1-based lines and rune columns) formats
only the smallest element enclosing the selection and leaves every other byte
untouched; a bare line means the whole line. `--check` and `--diff` compose with
it, and the language server exposes the same engine as
`textDocument/rangeFormatting`. **[implemented]**

### 2.4 Exit codes

Mirrors Ruff / ESLint / Prettier. **[decision]**

| Code | `check` | `format` | `format --check` |
|---|---|---|---|
| `0` | no findings (or all fixed) | success | nothing would change |
| `1` | findings | only with `--exit-non-zero-on-format` | files would be reformatted |
| `2` | abnormal (bad config/CLI/internal) | abnormal | abnormal |

### 2.5 stdin contract

Pass `-` as the path and `--stdin-filename` for the logical path. The logical
path drives config resolution, per-file-ignores, and file-kind detection.
Formatted/fixed content is written to stdout. **[decision]**

---

## 3. Configuration

### 3.1 File name and discovery

Primary config file: **`.ruxx.toml`**. **[decision]**

Discovery, from highest to lowest precedence within a directory:

1. `.ruxx.toml`
2. `ruxx.toml`

(No `pyproject.toml` support — `.ruxx.toml`/`ruxx.toml` only. **[decision]**)

Rules:

- **Hierarchical, closest-wins.** For each file, the nearest config up the
  directory tree applies. Configuration files do **not** merge across
  directories (like Ruff, unlike ESLint).
- Relative paths (`exclude`, `include`, `src`, `cache-dir`) resolve against the
  config file's directory (the "project root").
- `extend = "../.ruxx.toml"` gives explicit inheritance; the extending file's
  directory is the project root.
- `--config FILE` applies to every file; `--config "key = value"` sets a
  one-off TOML override. CLI flags beat all config files.
- `--isolated` ignores every config file.
- Fallback: built-in defaults; then `$XDG_CONFIG_HOME/ruxx/.ruxx.toml` if
  present.

### 3.2 Schema

```toml
# ---------- shared (lint + format) ----------
line-length      = 88          # default; see §9.4
indent-width     = 4
target-version   = "auto"      # Odoo series: auto|"15.0".."20.0"
preview          = false
cache-dir        = ".ruxx_cache"
respect-gitignore = true
force-exclude    = false
extend           = ""          # path to a parent config
exclude          = [".git", ".hg", ".svn", "node_modules", ".ruxx_cache",
                    "build", "dist", "__pycache__", ".venv", "venv"]
extend-exclude   = []
include          = ["*.xml", "*.xml.template", "*.qweb"]
extend-include   = []

# ---------- Odoo ----------
[odoo]
enabled        = "auto"        # true | false | "auto" (detect Odoo files)
# target-version lives at top level; kept here only as a read-through alias
modules-path   = []            # roots for cross-file xmlid/ref checks (opt-in)
attribute-order = "preserve"   # preserve | canonical
spacing-tags   = ["record", "menuitem", "template"]

# ---------- linter ----------
[lint]
select          = ["correctness"]
extend-select   = []
ignore          = []
extend-ignore   = []
fixable         = ["ALL"]
unfixable       = []
per-file-ignores = { }
dummy-ref-rgx   = ""           # names exempt from unused-xmlid checks (later)
severity        = { }          # optional per-code severity remap, e.g. { O019 = "warning" }
exclude         = []
extend-exclude  = []

# ---------- formatter ----------
# Implemented keys: line-length, indent-width, indent-style, line-ending,
# final-newline, xml-declaration, encoding-style, attribute-split,
# split-attributes-tags, attribute-sort, empty-element, close-tag-on-new-line,
# odoo-tag-spacing, odoo-spacing-tags, preserve-blank-lines,
# qweb-attributes-first. `preserve-comments` is accepted but must be true
# (ruxx never removes comments); `exclude`/`extend-exclude` are planned.
[format]
line-length            = 88
indent-width           = 4
indent-style           = "space"          # space | tab  (tab is discouraged)
line-ending            = "auto"           # auto | lf | crlf
final-newline          = true
xml-declaration        = "preserve"       # preserve | always | never
encoding-style         = "lower"          # lower | upper | preserve
attribute-split        = "auto"           # auto | always | never
split-attributes-tags  = []               # tags whose attrs always split
attribute-sort         = "none"           # none | alphabetical
empty-element          = "self-closing"   # self-closing | expand
close-tag-on-new-line  = false
odoo-tag-spacing       = true
odoo-spacing-tags      = ["record", "menuitem", "template"]
preserve-blank-lines   = true             # keep one author-written blank line
qweb-attributes-first  = true
preserve-comments      = true
exclude                = []
extend-exclude         = []

# ---------- overrides ----------
[[overrides]]
files = ["**/static/src/xml/**"]
[lint]
select = ["correctness", "style"]
```

### 3.3 Selector grammar

`select` / `ignore` / `extend-select` / `fixable` / `unfixable` accept a
comma-separated list where each item is one of:

- a full code: `O004`
- a prefix: `O`, `O0`, `XP`
- a group/family name: `xml`, `odoo`, `qweb`, `xpath`, `view`, `security`,
  `style`, `ruxx`
- a category: `correctness`, `suspicious`, `style`, `security`, `pedantic`,
  `restriction`, `formatting`
- `ALL`

Precedence (broadest → narrowest): `ALL` < category < group < prefix < code.
Reconciled across sources: CLI > current config > extended config. **[decision]**

Default `select = ["correctness"]` (see §7.3).

### 3.4 Per-file ignores

```toml
[lint.per-file-ignores]
"**/static/src/xml/**" = ["X010"]      # no line-length in OWL templates
"**/tests/**" = ["O003"]
```

Globs are matched against the path relative to the project root. **[decision]**

---

## 4. File discovery & kinds

- **[implemented]** Default includes: `*.xml`, `*.xml.template`, `*.qweb`;
  replaced by `include` and extended by `extend-include`.
- **[implemented]** Default excludes: `.git`, `.hg`, `.svn`, `node_modules`,
  `.ruxx_cache`, `build`, `dist`, `__pycache__`, `.venv`, `venv`; replaced by
  `exclude` and extended by `extend-exclude`. A bare name matches any path
  component (so `vendor` prunes every `vendor/`), and `**` is supported.
- **[implemented]** An explicit CLI path that names a file is always analyzed
  unless `force-exclude = true` (or `--force-exclude`); directories are walked.
  Excludes are matched against the path as given, so a bare name like `vendor`
  also excludes an explicit `vendor/a.xml`.
- **[implemented]** Ignore files are honored when `respect-gitignore` is on (the
  default): every `.ignore` and `.gitignore` inside the walked tree (including
  `!` negation and trailing-`/` directory rules; `.ignore` beats `.gitignore`
  in the same directory, a nested file wins over its parents), the repository's
  `.git/info/exclude`, and the user-level gitignore
  (`$XDG_CONFIG_HOME/git/ignore`, else `~/.config/git/ignore`). Ignore files
  above the walked root do not leak in, and `--no-respect-gitignore` disables
  the whole feature.
- **[implemented]** `--show-files` prints the selected files and exits without
  linting.
- File **kind** is derived from path + content and drives whether Odoo rules
  apply:

| Kind | Detection |
|---|---|
| `odoo.data` | root is `<odoo>`/`<data>`/`<openerp>` |
| `odoo.templates` | root is `<templates>` (OWL/client QWeb) |
| `odoo.view` | within an `odoo.data` file, a `<record model="ir.ui.view">` or `<template>` scope |
| `generic.xml` | anything else |

**[decision]**

---

## 5. Diagnostics model

A diagnostic carries:

- a **code** (`O004`) and rule **name** (`record-missing-model`),
- a **severity** (`error` | `warning` | `hint`),
- a primary **span** (`TextRange`, byte offsets) and optional secondary spans,
- a **message**, and an optional `help` note,
- an optional **Fix**: an ordered list of **Edits**, each a
  `{range, replacement}` (empty range = insert, empty replacement = delete),
- an **applicability**: `safe` | `unsafe` | `display`,
- provenance (which linter pass produced it).

Rendered forms:

- **concise**: `path:line:col: CODE message` with a `[*]` marker and `[fixable]`
  hint when a fix is available.
- **full**: annotated snippet with a caret underline and `= help:` line.
- Machine formats as listed in §2.2.

Severity defaults are per-rule (see `RULES.md`) and remappable via
`[lint.severity]`.

---

## 6. Suppression directives

XML has no line comments, so directives live in XML comments. Canonical prefix
`ruxx:`; `noqa:` is accepted as an alias. **[decision]**

| Directive | Scope |
|---|---|
| `<!-- ruxx: ignore[O004] -->` | the element on the same line, or the next element if on its own line |
| `<!-- ruxx: ignore -->` | blanket for the element/line |
| `<!-- ruxx: disable[O004] -->` … `<!-- ruxx: enable[O004] -->` | range between the two comments (indentation-nested) |
| `<!-- ruxx: noqa -->` / `<!-- ruxx: file-ignore[O004] -->` | whole file |
| `<!-- ruxx: fmt: off -->` / `<!-- ruxx: fmt: on -->` | formatter off/on range |
| `<!-- ruxx: fmt: skip -->` | skip the next element |

Meta rules police the directives themselves: `RUX100` flags a suppression that
suppresses nothing (auto-fixable: remove it), `RUX101` flags an unknown code,
`RUX102` flags a deprecated code. `--add-ignore` / `--add-noqa` insert
directives for existing findings (migration aid).

---

## 7. Rule system

### 7.1 Families (namespaces)

| Prefix | Group | Domain |
|---|---|---|
| `X` | `xml` | XML lexical form, well-formedness, file hygiene |
| `O` | `odoo` | Odoo records, fields, menus, data semantics |
| `QW` | `qweb` | QWeb/OWL templates |
| `XP` | `xpath` | XPath view inheritance |
| `VW` | `view` | View architecture semantics |
| `SEC` | `security` | Security model (`ir.rule`, groups, access) |
| `ST` | `style` | Style/format-facing rules (off by default; see §7.4) |
| `RUX` | `ruxx` | Meta: suppressions, deprecations |

Codes are `PREFIX` + three digits (`X001`, `QW012`). **[decision]**

### 7.2 Rule metadata

Each rule declares: code, kebab-case name, group, category, default severity,
`LintSource` (the cheapest pass that can decide it), fix availability and
safety, and stable/preview status. `LintSource` ∈
`{ token, tree, lines, file, odoo }` — rules run at the cheapest stage, mirroring
Ruff's `check_tokens → check_logical_lines → check_ast → check_physical_lines →
check_noqa`.

### 7.3 Categories & defaults

Categories: `correctness`, `suspicious`, `style`, `security`, `pedantic`,
`restriction`, `formatting`.

- **Default set** = `correctness` — currently **29 of the 83 rules** (see
  [`RULES.md`](./RULES.md), whose summary counts and
  *implementation-validated refinements* are kept in sync with the registry).
- Everything else is opt-in via `select`/`extend-select`.
- Preview rules require `--preview` (or `preview = true`) and are never enabled
  by prefix/category/`ALL` unless preview is on.

**[decision]**

### 7.4 Linter vs formatter separation

Like Ruff, ruxx keeps the two tools disjoint: `ruxx format` must never
introduce a `ruxx check` finding. `ST` style rules exist only for people who
run the linter without the formatter, **are off by default**, and enabling them
alongside `format` emits a warning (`RUX103`-style advisory) that the two may
fight. **[decision]**

### 7.5 Fix safety

- **safe** — preserves meaning; applied by `--fix`.
- **unsafe** — may change meaning or remove information (e.g. reordering
  directives with scoping effects, or renaming a duplicate `id`); requires
  `--unsafe-fixes`. X014 carries one: the later duplicate `id="x"` is renamed to
  `id="x_2"`.
- **display** — no automatic fix.

Fixes are applied by a non-overlapping, greedy engine with a bounded iterative
loop (see §11.4).

### 7.6 Full catalog

See [`RULES.md`](./RULES.md). Odoo hints and their sources are in
[`ODOO-HINTS.md`](./ODOO-HINTS.md).

Odoo-structure families (`O`, `QW`, `XP`, `VW`, `SEC`) are additionally gated by
**path**: they are skipped under `static/`, `tests/`, and `test/` trees (client
assets and fixtures) even when the root element is `<odoo>`, and any root that is
a known non-Odoo document (namespaced EDI/ODF, invoice fragments) is skipped
outright. **[decision]**

---

## 8. Odoo support

### 8.1 Activation

`[odoo].enabled`:

- `"auto"` (default): Odoo rules apply when the file's kind is an Odoo kind
  (§4) or the file sits under a directory containing `__manifest__.py`.
- `true`: Odoo rules always apply.
- `false`: only generic `X` rules and the formatter apply.

### 8.2 Target version

`target-version` gates version-sensitive rules so we do not report false
positives against the wrong Odoo series. Default `"auto"` infers the series
from the path (a directory named `15.0`…`20.0`), a `__manifest__.py`
`version` key, or falls back to the latest known series. Values:
`"15.0"`,`"16.0"`,`"17.0"`,`"18.0"`,`"19.0"`,`"20.0"` (aliases `"15"`…`"20"`).

Version-sensitive facts (see `ODOO-HINTS.md` for citations):

| Feature | ≤16 | 17 | 18 | 19 | 20 |
|---|---|---|---|---|---|
| `<tree>` list view | ok | ok | deprecated | deprecated | deprecated |
| `<list>` list view | no | no | ok | ok | ok |
| `attrs=` | ok | removed | removed | removed | removed |
| `states=` | ok | removed | removed | removed | removed |
| `t-esc` / `t-raw` | ok | ok | deprecated | deprecated | removed |
| `<openerp>` root | deprecated | deprecated | deprecated | deprecated | deprecated |

### 8.3 Cross-file checks (opt-in, implemented)

When `[odoo].modules-path` is set (roots are addons directories, resolved
relative to the config file), ruxx builds a light project index
(`internal/index`): per module, the xmlids it defines and the files defining
them, the `__manifest__.py` `data` list, `res.groups` xmlids, and whether
`security/ir.model.access.csv` exists. The index powers five rules:

| Rule | Check |
| --- | --- |
| `O016` | a `ref="module.id"` targets a module absent from the index |
| `O018` | an xmlid is defined in more than one file of the same module |
| `QW012` | a `t-call` target is not a known template (core `web.*` exempt) |
| `SEC002` | the module has an access csv the manifest `data` does not list |
| `SEC003` | `groups="module.group"` references a group the module does not define |

They are off by default and stay silent when no index is configured, so a
plain single-file lint never produces cross-file false positives.
`modules-path` should include every addons root whose modules are referenced.
The index is built in parallel across modules (bounded by `runtime.NumCPU` and
the module count) and persisted to `<cache-dir>/index.json`, keyed by the roots
plus a `size:mtime` stamp per file, so a later run re-parses only the modules
whose file set changed ([`PERFORMANCE.md`](PERFORMANCE.md) has the timings).

### 8.4 Hints

"Hint" rules are the advisory, best-practice diagnostics distilled from the
`odoo-shortcuts` extension (attribute order, tag spacing, QWeb `t-*` first,
`expr` before `position`, self-closing empties, and so on). They carry severity
`hint`, live in the Odoo families, and are catalogued in `ODOO-HINTS.md` with a
citation for each.

---

## 9. Formatter

### 9.1 Pipeline

`source → lossless parse tree → document IR → width-aware printer → text`.

The parse tree is **lossless**: `Unparse(tree) == source` for every input
(property-tested). All trivia — whitespace, newlines, comments, CDATA,
processing instructions, DOCTYPE — is attached, so formatting is a
re-decoration, never a re-serialization that loses information.

### 9.2 Document IR

A Ruff/Prettier-style IR: `Text`, `Line`/`SoftLine`/`HardLine`, `Group`,
`Indent`, `IfBreak`, `Fill`, `LineSuffix`. The printer measures groups against
`line-length` and breaks the outermost group that overflows. This keeps
formatting linear and idempotent.

### 9.3 Rules

**[implemented]** Indentation (`indent-width`, `indent-style`), element layout
(`attribute-split`, `split-attributes-tags`, `close-tag-on-new-line`),
attribute sorting (`attribute-sort`), empty elements (`empty-element`), tag
spacing (`odoo-tag-spacing`, `odoo-spacing-tags`), blank line preservation
(`preserve-blank-lines`), QWeb `t-*`-first ordering (`qweb-attributes-first`),
prolog control (`xml-declaration`, `encoding-style`), output line endings
(`line-ending`, `final-newline`), comment preservation, and range formatting
are live.

1. **Indentation** — `indent-width` (default 4) spaces per level.
2. **Element layout** — an element is printed on one line if it fits and it is
   not in `split-attributes-tags`; otherwise attributes go one-per-line, with
   the closing delimiter aligned.
3. **Empty elements** — `empty-element = self-closing` prints `<x/>`; `expand`
   prints `<x></x>`.
4. **Attribute order** — `attribute-sort = none` (default) preserves source
   order, except that QWeb `t-*` attributes always lead. `alphabetical` sorts
   the rest with Odoo-aware priorities: `<record id model>`,
   `<xpath expr position>`, `<attribute name>`, `<field name>`,
   `<menuitem id name>`, `<template id name>`, then the remaining attributes
   alphabetically. `t-*` attributes keep their source order (their relative
   order is style, not ours to shuffle). The same order drives the `ST003` lint
   rule and its fix, so `ruxx format` and `ruxx check --select ST003 --fix`
   agree.
5. **Tag spacing** — a blank line before `odoo-spacing-tags`
   (`record`, `menuitem`, `template`) at the same nesting level.
6. **QWeb** — `t-*` attributes precede plain attributes when
   `qweb-attributes-first = true`.
7. **Declaration & encoding** — `xml-declaration = preserve` (default) keeps the
   prolog as written, `always` inserts `<?xml version="1.0" encoding="utf-8"?>`
   when it is missing, and `never` drops it. `encoding-style` re-cases an
   existing declaration's encoding value: `lower` (default), `upper`, or
   `preserve`.
8. **Line endings & trailing newline** — `line-ending = auto` (default) follows
   the file; `lf` and `crlf` force one. `final-newline = true` (default) ends
   the output with exactly one newline; `false` preserves the input's trailing
   state. Both apply to the whole output, including text carried over verbatim.
9. **Comments** — preserved in place (`preserve-comments`).
10. **Blank lines** — with `preserve-blank-lines = true` (default) a single
    author-written blank line between siblings survives, at any depth and at
    document level (e.g. between the license comment and the root); longer runs
    collapse to one. The spacing-tag blank line is still added when absent.
11. **Range formatting** — `ruxx format --range start_line:col-end_line:col`
    formats only the smallest element enclosing the selection and leaves every
    other byte alone; the language server offers the same through the
    `textDocument/rangeFormatting` request. **[implemented]**

### 9.4 Line length

Default `line-length = 88`, matching Ruff/Black and the `odoo-shortcuts`
recommendation. The `odoo-shortcuts` option *table* lists 120, but its
*recommended config* and changelog say 88; we follow 88. **[decision]**

### 9.5 Suppression

`<!-- ruxx: fmt: off/on -->` disables formatting for a range;
`<!-- ruxx: fmt: skip -->` skips the next element. Both leave a
re-parse-stable file.

### 9.6 Idempotency & safety

- `Format(Format(x)) == Format(x)` (property-tested).
- Formatting must not change the parse tree modulo trivia.
- If formatting would change element order or attribute *set* (it never does),
  that is a bug, not an option.

---

## 10. Output, caching, watch

- **Output formats**: `concise`, `full`, `json`, `json-lines`, `github`,
  `gitlab`, `junit`, `sarif`, `azure`, `grouped`. **[implemented]** Selected
  with `--output-format`; the aggregating formats (`json`, `sarif`, `junit`, …)
  are rendered once, after every file has been linted.
- **Cache**: `.ruxx_cache/` keyed by content hash + settings hash; `--no-cache`
  bypasses; `ruxx clean` clears. **[implemented]** Entries are
  `.ruxx_cache/<blake2b-256>.json`, the digest covering the settings fingerprint
  plus the file bytes; a hit returns the cached diagnostics without re-parsing.
  `--cache-dir <dir>` and `cache-dir` in `.ruxx.toml` override the location, and
  caching is skipped under `--fix`. The cross-file index shares the cache
  directory: it is persisted to `<cache-dir>/index.json` and reused while every
  module's file set (`size:mtime` per file) is unchanged, so the first indexed
  run parses the tree and later runs only `stat` it. `--no-cache` disables both
  caches; `ruxx clean` removes both.
- **Watch**: `-w` re-runs on change; intended for editor/CI loops.
  **[implemented]** Polls a size+mtime snapshot every 300 ms, prints
  `[ruxx] changed: <path>` for each changed file, then re-runs the pass.
- **Colour**: respects `NO_COLOR`, `FORCE_COLOR`, `CLICOLOR`, and
  `CLICOLOR_FORCE`; `--color auto|always|never` overrides the detection.
  **[implemented]** Only the human-readable formats (concise, full, grouped)
  are coloured; machine-readable output is never decorated.

---

## 11. Architecture

Deep modules: a large amount of behaviour behind small interfaces.

```
cmd/ruxx/                  thin main() → cli.Main
internal/cli/              arg parsing, subcommand dispatch, exit codes
internal/config/           .ruxx.toml discovery, parsing, hierarchy, overrides
internal/settings/         resolved Settings (shared/lint/format/odoo)
internal/source/           Text, Locator (byte↔line/col), TextRange
internal/syntax/           lossless lexer + error-recovering parser → Tree
internal/tree/             node kinds, trivia attachment, visitor
internal/index/            comment ranges, directive index, element index
internal/diagnostic/       Diagnostic, Edit, Fix, Applicability, Severity
internal/linter/           pass orchestration; rule registry; checkers
internal/rules/            one file per rule family (x_*.go, odoo_*.go, …)
internal/odoo/             file-kind detection, version facts, semantic model
internal/odoo/knowledge/   embedded canonical data (positions, translated attrs,
                           forbidden attrs, version feature maps)
internal/fix/              greedy non-overlapping apply + iterative loop + map
internal/formatter/        document IR + printer + format rules
internal/output/           all output serializers
internal/walk/             file discovery: directory traversal (seeded from
                           finder), includes/excludes, globs, gitignore,
                           per-file-kind selection
internal/cache/            content/settings-keyed cache (BLAKE2b-256)
internal/lsp/              LSP server over stdio (implemented)
internal/report/           output formats + unified diff (implemented)
internal/attrorder/        canonical attribute order shared by `attribute-sort`
                           and the ST003 rule (implemented)
internal/version/          build identity, stamped with -ldflags (implemented)
internal/linttest/         test harness (diagnostics assertions)
internal/formatest/        test harness (format golden assertions)
```

### 11.0 Existing scaffolding

The repo already contains `internal/finder` (adapted from the author's `fdup`
project) and a `cmd/ruxx/main.go` CLI that currently exposes duplicate-file
finding. ruxx reuses **only the directory/file traversal** from this code:

- **Take the walk, drop the duplicate machinery.** `internal/walk` is seeded
  from `finder`'s `filepath.WalkDir` traversal (regular-file check, error
  warnings, `Options.Workers` pool style). The size-grouping, BLAKE2b hashing,
  worker-pool hashing, and duplicate grouping are **not** part of ruxx and are
  dropped. `internal/cache` computes its own content hash (BLAKE2b-256) — it
  does not depend on `finder`.
- **Reconcile the entry point.** `cmd/ruxx/main.go` is repurposed to dispatch
  ruxx's `check`/`format` subcommands; the duplicate-finding output is dropped.
- **Do not clobber author work.** `internal/finder` is left in place (and can
  serve as the traversal reference) until `internal/walk` supersedes it; remove
  it only with explicit agreement.

### 11.1 Core interfaces

```go
// internal/source
type Text interface { String() string; Len() int }
type Range struct { Start, End int } // byte offsets

// internal/syntax — one deep module: text in, lossless tree + errors out.
func Parse(src string) (*Tree, []SyntaxError)

// internal/tree
func Unparse(t *Tree) string // guarantees Unparse(Parse(src)) == src

// internal/linter — one deep module: tree in, diagnostics out.
func Check(t *Tree, s *settings.Settings) []Diagnostic

// internal/fix
func ApplyFix(src string, diags []Diagnostic, ap Applicability) (string, bool, error)

// internal/formatter
func Format(t *Tree, s *settings.Format) string
```

The **seam** is the lossless `Tree`. Linter, formatter, and fix engine all
speak it; tests cross the same seam.

### 11.2 Lint passes (cheapest first)

1. `check_file` — path/kind, conflicts markers, declaration, BOM, encoding.
2. `check_tokens` — trivia: tabs, trailing whitespace, line endings, indent.
3. `check_tree` — the bulk: XML structure and Odoo/QWeb/XPath/view rules.
4. `check_odoo` — semantic cross-checks when an Odoo model/index is available.
5. `check_suppressions` — enforce directives; emit `RUX100`.
6. `check_lines` — `X010` line length.

### 11.3 Parser requirements

- Error recovery: never abort; record `SyntaxError`s and keep parsing.
- If syntax is invalid: emit `X001`, **strip all fixes** (never fix a file we
  could not fully parse).
- Trivia preservation is mandatory (round-trip property).
- Fast: single forward scan, `memchr`-style byte scanning for `<`, no regex in
  hot paths.

### 11.4 Fix engine

- Sort fixes by `(start, then rule stability)`; greedy non-overlap apply;
  isolation groups (`Group(id)`) allow only one fix per group.
- Iterate: parse → lint → apply → repeat until stable, capped at
  `maxIterations = 100`; if not converged, abort with an internal error.
- If a fix loop introduces a *new* syntax error where the original parsed
  cleanly, **revert all changes** and report abnormal.
- `--diff` prints the diff instead of writing.

### 11.5 Concurrency

**[implemented]** Files are analyzed in parallel with a bounded worker pool
(`runtime.NumCPU` workers, capped at the number of files; no `errgroup`
dependency). Each file is single-threaded internally (one parse, one traverse),
so locking is unnecessary. Workers only read and compute; every mutation
(writing fixed or `--add-ignore` files, accumulating totals, building the
report) happens in a single sequential merge pass, and results are addressed by
input index, keeping output deterministic (diagnostics sorted by path then
range). Both `ruxx check` and `ruxx format` use it; with `GOMAXPROCS=1` or a
single file the pool degrades to a sequential path.

The workers share one per-run `runState`: the resolved config memoised per
directory, the diagnostics cache, and the cross-file index, each behind its own
mutex. Sharing matters most for the index — it is built once per run (and then
persisted to `.ruxx_cache/index.json`), so N workers do not each walk and parse
the whole tree. Without that sharing the indexed path was slower than no index
at all (4.3 GB RSS and a 17× file-open amplification on a 45k-file sweep);
[`PERFORMANCE.md`](PERFORMANCE.md) records the before/after numbers.

### 11.6 Dependencies

- TOML: `github.com/BurntSushi/toml` (or `pelletier/go-toml/v2`).
- Glob matching: `github.com/bmatcuk/doublestar/v4`.
- gitignore: `github.com/go-git/go-git`'s gitignore or a small in-house matcher.
- CLI: `spf13/cobra` + `pflag` (or stdlib `flag` with a custom parser for
  Ruff-style comma lists).
- A `harmonica`/`go-runewidth`-class width lib for column/width math.

**[open]** final dependency selection.

---

## 12. Testing strategy

Tests come **before** implementation (the next milestone). The harness is part
of the spec.

### 12.1 Layout

```
internal/linttest/                       harness
internal/formatest/                      harness
testdata/
  rules/<CODE>/<case>/input.xml          lint/format input
                    expected.xml         expected output after --fix (if any)
                    expected.txt         expected concise diagnostics (golden)
  format/<case>/input.xml
                expected.xml             golden formatted output
  corpus/                                pinned real Odoo snippets (small subset)
  config/<case>/.ruxx.toml + tree/       config resolution cases
  cli/<case>/args + files + expected     CLI/end-to-end cases
```

Golden files are regenerated with `-update`.

### 12.2 Harness API

```go
// internal/linttest
func Run(t *testing.T, src string, opts Options) []diagnostic.Diagnostic
func AssertDiagnostics(t *testing.T, got []Diagnostic, golden string)
func AssertFix(t *testing.T, src string, opts Options, golden string)

// internal/formatest
func Format(t *testing.T, src string, opts Options) string
func AssertFormat(t *testing.T, src, golden string, opts Options)
```

Tests are table-driven; each rule gets a `TestRule_<CODE>` with subtests named
`finds_*`, `ok_*`, `fix_*`.

### 12.3 Test types

1. **Rule tests** — every rule has at least one positive and one negative case,
   plus a fix case when fixable.
2. **Formatter goldens** — input/expected pairs per layout rule; idempotency
   asserted on every golden (`Format(expected) == expected`).
3. **Property tests** —
   - `Unparse(Parse(src)) == src` (losslessness) over a corpus.
   - `Format(Format(x)) == Format(x)` (idempotency).
   - Formatting does not change the element/attribute structure.
4. **Fuzz tests** — `FuzzParse` (no panics; round-trip on valid inputs) and
   `FuzzFormat` (idempotency), run in CI for a fixed budget.
5. **Config tests** — hierarchical discovery, `extend`, overrides,
   `per-file-ignores`, selectors.
6. **CLI/e2e tests** — exit codes, `--fix`, `--diff`, `--check`, stdin,
   `--output-format`, `--add-ignore`, caching.
7. **Corpus tests** — run `check` and `format --check` over a pinned copy of
   representative Odoo XML to guard against regressions and count false
   positives. Kept as a separate, opt-in package because the corpus is large.
8. **Golden-update discipline** — `-update` rewrites goldens; reviewers read
   the diff.

### 12.4 TDD order

1. Land `source`, `syntax.Parse`, and the round-trip property test.
2. Land `linttest`/`formatest` harnesses with a single trivial rule.
3. For each rule (in `RULES.md` order): add failing fixtures → implement → green.
4. Land the formatter IR with a minimal printer; grow rule-by-rule with
   goldens.
5. Land `fix` engine; add `--fix` fixtures.
6. Land CLI/e2e with exit-code tests.
7. Land cache/watch/output.

---

## 13. Roadmap

| Phase | Deliverable | Status |
|---|---|---|
| 0 | Spec (this document) + rule catalog + Odoo hints | done |
| 1 | `source`, lossless `syntax.Parse`, round-trip property, fuzz | done |
| 2 | Test harness + `X` family rules + `fix` engine | done |
| 3 | Formatter IR + printer + `format` | done |
| 4 | `O`/`QW`/`XP`/`VW` families; `[odoo]` detection & version gates | done |
| 5 | CLI polish: output formats, cache, watch, `--add-ignore` | done |
| 6 | `SEC` + cross-file index (opt-in) | done |
| 7 | LSP `ruxx server`; editor integration | done: server, VS Code client (`editors/vscode`), pre-commit hooks |
| 8 | Packaging: `Makefile`, CI (`.github/workflows/ci.yml`), goreleaser release, `internal/version` | done |

---

## 14. Resolved decisions & open questions

Resolved:

- Rule-code letters: `X`, `O`, `QW`, `XP`, `VW`, `SEC`, `ST`, `RUX`.
- Default `select`: `["correctness"]` only.
- `ST` style rules ship, off by default.
- Config files: `.ruxx.toml` and `ruxx.toml` only (no `pyproject.toml`).
- Odoo series floor: 15.0.
- Line length default: 88.
- Suppression directives use `<!-- ruxx: … -->` (with `noqa:` alias).
- `internal/finder`: reuse only its directory/file **traversal**; drop the
  duplicate-finding/hashing. `cmd/ruxx/main.go` is repurposed to the
  `check`/`format` dispatcher (§11.0).

Still open:

- **[open]** Dependency choices in §11.6.
- **[open]** Cross-file index scope for v1 (deferred to phase 6).
- **[open]** Whether `target-version` should live at the top level or inside
  `[odoo]` (currently top-level, read-through alias in `[odoo]`).
