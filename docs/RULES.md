# ruxx — Rule Catalog

Companion to [`SPEC.md`](./SPEC.md). Each rule lists: **code**, **name**,
default **severity** (`error` / `warning` / `hint`), **fix** applicability
(`safe`, `unsafe`, `—`), **category**, whether it is in the **default set**
(yes = `correctness`), and a source/notes reference.

> Codes are `PREFIX` + three digits. Programs **should not** infer ordering
> from the final digits except within a family; new rules append at the next
> free number in their family.

Legend: **Sev** = default severity; **Fix** = fix applicability; **Cat** =
category; **Def** = enabled by default.

---

## X — XML core (`xml`)

Lexical form, well-formedness, and file hygiene. Domain-agnostic.

| Code | Name | Sev | Fix | Cat | Def | Notes |
|---|---|---|---|---|---|---|
| `X001` | `parse-error` | error | — | correctness | yes | file must be well-formed XML; parse errors reported with span |
| `X002` | `missing-xml-declaration` | hint | safe | style | no | add `<?xml version="1.0" encoding="utf-8"?>` |
| `X003` | `encoding-style` | hint | safe | style | no | encoding should be lower-case `utf-8` (configurable) |
| `X004` | `byte-order-mark` | warning | safe | suspicious | no | strip UTF-8 BOM |
| `X005` | `crlf-line-ending` | hint | safe | style | no | prefer LF (configurable) |
| `X006` | `tab-indentation` | warning | safe | style | no | indent with spaces |
| `X007` | `trailing-whitespace` | hint | safe | style | no | no trailing spaces/tabs |
| `X008` | `indent-unit` | hint | safe | style | no | indentation is a multiple of `indent-width` |
| `X009` | `mixed-indentation` | warning | unsafe | suspicious | no | a file must not mix tabs and spaces |
| `X010` | `line-too-long` | hint | — | style | no | exceeds `line-length`; formatter handles wrapping |
| `X011` | `conflict-markers` | error | — | correctness | yes | `<<<<<<<`/`=======`/`>>>>>>>` in a committed file |
| `X012` | `missing-final-newline` | hint | safe | style | no | file should end with exactly one newline |
| `X013` | `attribute-spacing` | hint | safe | style | no | no spaces around `=`; one space before attrs |
| `X014` | `duplicate-id-in-file` | error | unsafe | correctness | yes | two records/menuitems/templates share an `id` in one file |
| `X015` | `xmlid-multiple-dots` | error | — | correctness | yes | an xml id may contain at most one dot |
| `X016` | `empty-element-whitespace` | hint | safe | style | no | `<x />` / `<x >` spacing normalized |
| `X017` | `unclosed-cdata-or-comment` | error | — | correctness | yes | unterminated `-->` or `]]>` (surfaced via `X001` when possible) |

---

## O — Odoo data (`odoo`)

Records, fields, menus, and data semantics. Apply to `odoo.data` /
`odoo.view` kinds (or when `[odoo].enabled = true`).

| Code | Name | Sev | Fix | Cat | Def | Notes |
|---|---|---|---|---|---|---|
| `O001` | `invalid-root-tag` | error | — | correctness | yes | root must be `<odoo>`, `<data>`, or `<openerp>` |
| `O002` | `deprecated-openerp-root` | warning | safe | suspicious | no | migrate `<openerp>` → `<odoo>` |
| `O003` | `record-missing-id` | warning | — | suspicious | no | `<record>` should define `id` (empty xmlid otherwise) |
| `O004` | `record-missing-model` | error | — | correctness | yes | `<record>` requires `model` |
| `O005` | `field-missing-name` | error | — | correctness | yes | `<field>` requires `name` |
| `O006` | `arch-missing-type` | hint | safe | style | no | `<field name="arch">` should carry `type="xml"` |
| `O007` | `noupdate-on-record` | warning | unsafe | suspicious | no | `noupdate` only reads on `<odoo>`/`<data>`; ignored on `<record>` |
| `O008` | `deprecated-act-window` | error | — | correctness | yes | `<act_window>` sugar is not a loader tag |
| `O009` | `deprecated-report-tag` | error | — | correctness | yes | `<report>` sugar is not a loader tag |
| `O010` | `tree-vs-list` | warning | safe | correctness | yes | `<tree>` for ≤17, `<list>` for ≥18 (version-gated) |
| `O011` | `view-mode-version` | warning | — | correctness | yes | `view_mode` uses `tree`, renamed to `list` in Odoo 18.0 (version-gated) |
| `O012` | `deprecated-attrs` | warning | — | suspicious | no | `attrs=` removed in 17.0 (version-gated) |
| `O013` | `deprecated-states` | warning | — | suspicious | no | `states=` removed in 17.0 (version-gated) |
| `O014` | `group-typo` | warning | safe | suspicious | no | `group=` is a typo for `groups=` |
| `O015` | `menuitem-missing-id` | error | — | correctness | yes | `<menuitem>` requires `id` |
| `O016` | `xmlid-uninstalled-module` | error | — | correctness | no | reference targets an uninstalled module (needs index) |
| `O017` | `noupdate-not-boolean` | warning | safe | suspicious | no | `noupdate` should be `0`/`1`/`true`/`false` |
| `O018` | `duplicate-xmlid-module` | warning | — | suspicious | no | same `module.id` defined twice across a module (needs index) |
| `O019` | `record-attribute-order` | hint | safe | style | no | `<record id=… model=…>`: `id` before `model` (Odoo hint) |
| `O020` | `record-id-qualified` | hint | — | suspicious | no | in-module records should not qualify their own id with the module |
| `O021` | `template-id-missing` | error | — | correctness | yes | `<template>` at data level requires an `id`/`t-name` |
| `O022` | `ir-ui-view-field-order` | hint | unsafe | style | no | arch view fields ordered `name, model, inherit_id, priority, arch` |

---

## QW — QWeb / OWL (`qweb`)

| Code | Name | Sev | Fix | Cat | Def | Notes |
|---|---|---|---|---|---|---|
| `QW001` | `deprecated-tesc` | warning | unsafe | suspicious | no | prefer `t-out` over `t-esc` (removed in 20.0) |
| `QW002` | `deprecated-traw` | warning | — | suspicious | no | `t-raw` removed in 20.0 |
| `QW003` | `qweb-attributes-first` | hint | safe | style | no | `t-*` attributes precede plain attributes |
| `QW004` | `template-missing-name` | warning | — | correctness | no | template needs `t-name` (or a usable `id`) |
| `QW005` | `tname-module-qualified` | hint | — | style | no | `t-name` should be `module.Name` |
| `QW006` | `owl-directive-forbidden` | error | — | correctness | yes | OWL directives only valid in QWeb-based views |
| `QW007` | `comp-directive-forbidden` | error | — | correctness | yes | `__comp__` is forbidden in arch |
| `QW008` | `data-tooltip-forbidden` | error | — | correctness | yes | `data-tooltip` forbidden in arch |
| `QW009` | `templates-root-space` | hint | safe | style | no | `<templates>` root should set `xml:space="preserve"` |
| `QW010` | `directive-order` | hint | unsafe | style | no | canonical directive order (scoping-sensitive) |
| `QW011` | `deprecated-tjquery` | warning | — | suspicious | no | `t-jquery` is obsolete |
| `QW012` | `tcall-unknown-template` | warning | — | suspicious | no | `t-call` target not found (needs index) |

---

## XP — XPath inheritance (`xpath`)

| Code | Name | Sev | Fix | Cat | Def | Notes |
|---|---|---|---|---|---|---|
| `XP001` | `missing-expr` | error | — | correctness | yes | `<xpath>` requires a non-empty `expr` |
| `XP002` | `missing-position` | hint | safe | style | no | explicit `position` recommended (default is `inside`) |
| `XP003` | `invalid-position` | error | — | correctness | yes | position ∈ {before, after, inside, replace, attributes, move} |
| `XP004` | `attribute-order` | hint | safe | style | no | `expr` before `position` (Odoo hint) |
| `XP005` | `class-selector` | warning | — | suspicious | no | avoid `@class` in `expr`; use `hasclass()` |
| `XP006` | `translated-selector` | error | — | correctness | yes | translated attribute (e.g. `string`) as selector |
| `XP007` | `attributes-needs-attribute` | error | — | correctness | yes | `position="attributes"` needs `<attribute name=…>` children |
| `XP008` | `attribute-no-position` | warning | safe | suspicious | no | `<attribute>` uses `position=`; use `add=`/`remove=`/text |
| `XP009` | `non-locatable-spec` | warning | — | suspicious | no | `position` on a generic tag with no identifying attribute |
| `XP010` | `inherited-record-groups` | error | — | correctness | yes | inherited view record must not carry `groups` |

---

## VW — View architecture (`view`)

| Code | Name | Sev | Fix | Cat | Def | Notes |
|---|---|---|---|---|---|---|
| `VW001` | `page-outside-notebook` | error | — | correctness | yes | `<page>` direct parent must be `<notebook>` |
| `VW002` | `graph-children` | error | — | correctness | yes | `<graph>` may contain only `<field>` |
| `VW003` | `list-children` | error | — | correctness | yes | list/tree view allows only known child tags |
| `VW004` | `search-needs-field` | warning | — | suspicious | no | `<search>` requires ≥1 `<field>` |
| `VW005` | `search-multiple-panels` | error | — | correctness | yes | `<search>` may have ≤1 `<searchpanel>` |
| `VW006` | `img-missing-alt` | warning | — | suspicious | no | `<img>` should define `alt` |
| `VW007` | `notebook-needs-page` | warning | — | suspicious | no | `<notebook>` with no `<page>` |
| `VW008` | `duplicate-field-in-view` | warning | — | suspicious | no | same `name` field twice under one parent |

---

## SEC — Security (`security`)

| Code | Name | Sev | Fix | Cat | Def | Notes |
|---|---|---|---|---|---|---|
| `SEC001` | `rule-missing-domain` | warning | — | security | no | `ir.rule` should define `domain_force`/`perm_*` |
| `SEC002` | `access-file-not-in-manifest` | warning | — | security | no | `security/*.xml` not listed in `__manifest__.py` (needs index) |
| `SEC003` | `group-implies-unknown` | warning | — | security | no | `implied_ids` targets an unknown group (needs index) |

Note: `ir.model.access.csv` header/column checks live outside XML; they are
tracked separately (§ `SEC` in `SPEC.md` §8.3) and not part of v1.

---

## ST — Style (`style`)

**Off by default.** These duplicate what `ruxx format` already does; they exist
only for linter-only workflows. Enabling them together with the formatter may
produce conflicting results (`SPEC.md` §7.4).

| Code | Name | Sev | Fix | Cat | Def | Notes |
|---|---|---|---|---|---|---|
| `ST001` | `blank-line-before-odoo-tag` | hint | safe | style | no | blank line before `record`/`menuitem`/`template` |
| `ST002` | `self-closing-empty-element` | hint | safe | style | no | empty elements self-close |
| `ST003` | `attribute-sort` | hint | safe | style | no | attributes are not in canonical order (QWeb `t-*` first, then `<record id model>`, `<xpath expr position>`, `<field name>`; shared with `attribute-sort = alphabetical`) |
| `ST004` | `close-tag-new-line` | hint | safe | style | no | closing tag on its own line |
| `ST005` | `consecutive-blank-lines` | hint | safe | style | no | collapse runs of blank lines |
| `ST006` | `space-before-self-close` | hint | safe | style | no | `<x/>` vs `<x />` per config |

---

## RUX — Meta (`ruxx`)

| Code | Name | Sev | Fix | Cat | Def | Notes |
|---|---|---|---|---|---|---|
| `RUX100` | `unused-suppression` | warning | safe | correctness | yes | a `ruxx: ignore`/`disable` suppresses nothing |
| `RUX101` | `unknown-suppression-code` | warning | safe | correctness | yes | suppression names an unknown rule code |
| `RUX102` | `deprecated-suppression-code` | warning | safe | suspicious | no | suppression names a deprecated rule code |
| `RUX103` | `formatter-conflict` | warning | — | correctness | yes | a `style` rule is enabled; the formatter already covers it |
| `RUX104` | `unclosed-disable-region` | warning | — | suspicious | no | `ruxx: disable` with no matching `enable` |

---

## Summary counts

| Family | Rules | Default-on |
|---|---|---|
| X | 17 | 5 |
| O | 22 | 9 |
| QW | 12 | 3 |
| XP | 10 | 4 |
| VW | 8 | 4 |
| SEC | 3 | 0 |
| ST | 6 | 0 |
| RUX | 5 | 4 |
| **Total** | **83** | **29** |

The catalog is a starting point, not a cap. Rules are added per the TDD loop in
`SPEC.md` §12.4, each with fixtures, a fix case where applicable, and a source
citation in `ODOO-HINTS.md`.

---

## Implementation-validated refinements

The rules were exercised against ~40k real Odoo XML files (15.0–20.0 community
and enterprise). The following semantics were refined and are locked by
regression fixtures in `internal/rules/testdata/rules`:

- **X011** treats only `<<<<<<<`, `|||||||`, and `>>>>>>>` prefixes as conflict
  markers; a `=======` line counts only when it is exactly seven `=`, so long
  `=` comment separators are ignored.
- **X014 / X015** consider only *id-defining* elements — an element whose parent
  is a data root (`<odoo>`/`<data>`/`<openerp>`) or the root itself — so nested
  DOM ids such as `<field id="company">` are not treated as xmlids. Composite
  xmlids and genuine duplicate ids in Odoo core (e.g. `crm_case_15`) are real
  findings and remain default-on. X014 also names the first definition
  (`duplicate id "x" in this file (first defined at line N)`) and carries an
  **unsafe** fix renaming the later `id` to `x_2` (applied only under
  `--unsafe-fixes`). Across files, the index-powered **O018** reports the same
  `module.id` defined in more than one file of a module.
- **O001** fires only when the root is not a data root *and* has a direct child
  element named `record`, `menuitem`, `template`, `data`, `asset`,
  `act_window`, `report`, `function`, or `delete`, so non-Odoo documents that
  merely live under `data/` are ignored.
- **QW006** allows `t-key` (a legitimate QWeb directive); only true OWL
  directives are forbidden in an arch.
- **XP003** applies only inside `<field name="arch">` and skips `<attribute>`
  elements.
- **XP007** requires the `position="attributes"` element to have an
  `<attribute name="...">` *descendant*; a commented-out or absent one is
  flagged.
- **VW001** fires only when a `<page>` reaches the arch root with no
  `<notebook>`, `<xpath>`, or `position`-bearing ancestor, so view-inheritance
  specs are exempt.
- **VW003** allows the Odoo 20.0 `<column>` element inside `<list>`.
- **Odoo gating** excludes `static/`, `tests/`, and `test/` paths outright,
  even when a file's root element happens to be `<odoo>`.

---

## Cross-file index (opt-in)

Setting `[odoo].modules-path` (a list of addons roots, resolved relative to the
config file) builds a project index that enables five otherwise-silent rules:
`O016` (reference to a module absent from the index), `O018` (xmlid defined in
more than one file of a module), `QW012` (`t-call` target unknown; core `web.*`
layouts exempt), `SEC002` (access csv missing from the manifest `data`), and
`SEC003` (`groups` naming an undefined group). Without `modules-path` these
rules never fire, so single-file linting stays free of cross-file false
positives. Implementation: `internal/index`; exercised end-to-end by the CLI.

Every rule in the catalog is implemented. `RUX103` (formatter-conflict) fires
when a `style`-category rule is enabled: the formatter already covers those
concerns, so enabling them makes lint and format fight.
