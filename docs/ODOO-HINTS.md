# ruxx — Odoo Hints

This document collects the Odoo-specific knowledge that `ruxx` applies on top
of generic XML linting/formatting. Most hints originate in the
[`odoo-shortcuts`](https://github.com/mjavint/odoo-shortcuts) VS Code extension
and in Odoo's own source (`convert.py`, `template_inheritance.py`, `ir_ui_view.py`,
`ir_qweb.py`, `translate.py`). Each entry cites where the behaviour is defined
or observed, and maps to the `ruxx` rule(s) that enforce it.

Terminology: a **hint** is an advisory rule (`severity = hint`); a **rule** is
any diagnostic. All entries below are Odoo-family rules (`O*`, `QW*`, `XP*`,
`VW*`, `SEC*`).

---

## 1. File structure & kind

| Hint | Detail | ruxx |
|---|---|---|
| Root element | Odoo data files load only under `<odoo>`, `<data>`, or `<openerp>`; the loader asserts `Root xml tag must be <openerp>, <odoo> or <data>` (`tools/convert.py`, `DATA_ROOTS`). | `O001` |
| Client templates | OWL/QWeb client templates use a `<templates xml:space="preserve">` root. | `QW009` |
| Deprecated root | `<openerp>` is legacy; prefer `<odoo>`. | `O002` |
| Declaration | Recommended header is `<?xml version="1.0" encoding="utf-8"?>` (lower-case `utf-8`), with an optional license comment block. | `X002`, `X003` |
| Directory convention | `views/`, `data/`, `security/`, `report/`, `wizard/` hold data XML; `static/src/xml/` holds client QWeb. | kind detection (§4 of `SPEC.md`) |
| Excludes | The `odoo-shortcuts` formatter skips `**/odoo/**` and `**/enterprise/**` by default; `ruxx` does **not** exclude these by default but documents them as a suggested `extend-exclude`. | config |

---

## 2. Records, fields, menus

| Hint | Detail | ruxx |
|---|---|---|
| `<record id=… model=…>` | Both attributes are effectively required. `id`-before-`model` is the dominant order (32,546 vs 1,114 records in Odoo 17.0), though core itself is not perfectly consistent. | `O004` (error), `O019` (hint) |
| `<record>` without `id` | Allowed by the loader but yields an empty xmlid; 0 occurrences in core. | `O003` |
| `<field name=…>` | `name` is mandatory; view arch raises `Field tag must have a "name" attribute defined`. | `O005` |
| `<field name="arch" type="xml">` | `type="xml"` should be present (only 2 of 3,387 omit it in 17.0). | `O006` |
| `<menuitem>` | Requires `id`; attributes typically `id, name, parent, action, groups, sequence`. | `O015` |
| `<template id=…>` | Sugar for an `ir.ui.view` record of `type=qweb`; needs an id or `t-name`. | `O021` |
| `noupdate` | Read from the **root** element only (`<odoo>`/`<data>`); placing it on `<record>` is silently ignored. | `O007`, `O017` |
| View field order | Canonical `name, model, inherit_id, priority, arch`. | `O022` |
| Deprecated sugar | `<act_window>` and `<report>` are **not** loader tags in modern Odoo; use `<record model="ir.actions.act_window">` / `ir.actions.report`. | `O008`, `O009` |
| Versioned list view | `<tree>` for ≤17; `<list>` for ≥18. | `O010`, `O011` |
| Deprecated view attrs | `attrs=`/`states=` were removed in 17.0 (499/32 files used them in 15.0, 0 in 17.0+). | `O012`, `O013` |
| `group=` typo | Odoo warns when `group=` is used instead of `groups=`. | `O014` |

---

## 3. XPath inheritance

| Hint | Detail | ruxx |
|---|---|---|
| Valid positions | `before`, `after`, `inside` (default), `replace`, `attributes`, `move`; anything else raises `Invalid position attribute`. | `XP003` |
| `expr` required | `locate_node` needs an `expr`; a missing one fails. | `XP001` |
| Explicit position | `position` defaults to `inside`; only 10 usages in 17.0 relied on the default, so require it explicitly. | `XP002` |
| Attribute order | `expr` must come before `position`; the `odoo-shortcuts` changelog explicitly fixed this "following Odoo best practices". 100% of 17.0 `<xpath … position>` tags put `expr` first. | `XP004` |
| `position="attributes"` | Children must be `<attribute name="…" [add=] [remove=] [separator=]>…</attribute>`. | `XP007` |
| `<attribute>` semantics | `position=` on `<attribute>` is ignored; use `add=`/`remove=`/text. Real bug seen in `base_vat/views/res_partner_views.xml`. | `XP008` |
| `@class` selector | Avoid `@class` in `expr`; use `hasclass(*classes)`. | `XP005` |
| Translated selectors | A translated attribute (e.g. `string`, `help`, `title`, `alt`, `placeholder`) must not be used as a selector. | `XP006` |
| Locatability | For generic tags (`<field>`, `<div>`, …), matching uses all other attributes; a spec with no identifying attribute matches the first of its kind. | `XP009` |
| Inherited record `groups` | An inherited view record must not define `groups` outside the arch. | `XP010` |

---

## 4. QWeb / OWL

| Hint | Detail | ruxx |
|---|---|---|
| `t-*` first | QWeb attributes should precede plain attributes (`odoo-shortcuts` guarantees this; 20,324 t-first vs 1,918 plain-first in source). | `QW003` |
| `t-esc` → `t-out` | `t-esc` is deprecated in favour of `t-out`; `t-raw` is obsolete (both removed in 20.0). | `QW001`, `QW002` |
| `t-name` | Templates should carry a module-qualified `t-name`. | `QW004`, `QW005` |
| `t-jquery` | Obsolete. | `QW011` |
| Forbidden directives | OWL/`__comp__`/`data-tooltip` directives are only valid in QWeb-based views, not in arch. | `QW006`, `QW007`, `QW008` |
| Directive order | Canonical style order: `t-name`, `t-call`, `t-foreach`, `t-if`, `t-set`, `t-out`. Reordering is scoping-sensitive, so the fix is unsafe. | `QW010` |

---

## 5. View architecture

| Hint | Detail | ruxx |
|---|---|---|
| `<page>` placement | A `<page>`'s direct parent must be `<notebook>`. | `VW001` |
| `<graph>` children | Only `<field>` is allowed. | `VW002` |
| List/tree children | Only a known set of tags is allowed. | `VW003` |
| `<search>` | Requires at least one `<field>`; at most one `<searchpanel>`. | `VW004`, `VW005` |
| `<img alt>` | `<img>` should define `alt`. | `VW006` |

---

## 6. Security

| Hint | Detail | ruxx |
|---|---|---|
| `ir.rule` | Should define `domain_force` and `perm_*`. | `SEC001` |
| Manifest listing | `security/*.xml` should be listed in the module manifest's `data`. | `SEC002` (needs index) |
| Access CSV | `ir.model.access.csv` must use the exact header `id,name,model_id:id,group_id:id,perm_read,perm_write,perm_create,perm_unlink` (out of XML scope, tracked). | — |

---

## 7. Formatting hints

Distilled from `odoo-shortcuts` `docs/features/formatter.md` and the README.

| Hint | Detail | ruxx formatter |
|---|---|---|
| Indentation | 4 spaces per level (`formatter.tabSize`). | `indent-width = 4`, `indent-style = "space"` (or `"tab"`) |
| Tag spacing | Blank line before `record`, `menuitem`, `template` (`odooTagSpacing`, `odooSpacingTags`). | `odoo-tag-spacing`, `odoo-spacing-tags` (also `ST001`) |
| Attribute splitting | One attribute per line when the line overflows, or for tags in `splitAttributesTags`. | `attribute-split`, `split-attributes-tags` |
| Line length | `maxLineLength` = 88 (recommended) — the option table lists 120; we follow 88. | `line-length = 88` |
| Empty elements | `emptyElementHandling` = `selfClosing` (default). | `empty-element = self-closing` |
| Attribute sorting | `sortAttributes` = false (default). | `attribute-sort = none` \| `alphabetical` (Odoo-aware; Odoo order is `<record id model>`, `<xpath expr position>`, `t-*` first) |
| Closing tag | `closeTagOnNewLine` = false (default). | `close-tag-on-new-line = false` |
| Comments | `preserveComments` = true (default). | `preserve-comments = true` |
| Prolog | The header template uses lower-case `utf-8`. | `xml-declaration = "preserve"`, `encoding-style = "lower"` |
| Line endings | LF with exactly one trailing newline. | `line-ending = "auto"`, `final-newline = true` (ruxx extension) |
| Blank lines | *(no odoo-shortcuts option)* — grouped sections survive. | `preserve-blank-lines = true` (ruxx extension) |
| QWeb attrs | QWeb `t-*` attributes come first. | `qweb-attributes-first = true` |
| Selection formatting | `Format Selection` (Ctrl+K Ctrl+F) is supported. | `ruxx format --range` (and LSP `textDocument/rangeFormatting`) |
| Exclusions | `excludePatterns` defaults to core/enterprise. | `extend-exclude` (documented, not default) |

---

## 8. Version feature map

Used by version-gated rules (`O010`, `O011`, `O012`, `O013`, `QW001`, `QW002`).

| Feature | ≤16 | 17 | 18 | 19 | 20 |
|---|---|---|---|---|---|
| `<tree>` | ok | ok | deprecated | deprecated | deprecated |
| `<list>` | no | no | ok | ok | ok |
| `attrs=` | ok | removed | removed | removed | removed |
| `states=` | ok | removed | removed | removed | removed |
| `t-esc`/`t-raw` | ok | ok | deprecated | deprecated | removed |
| `<openerp>` | deprecated | deprecated | deprecated | deprecated | deprecated |

`target-version = "auto"` infers the series from the path (a `15.0`…`20.0`
directory), a `__manifest__.py` `version` key, or the latest known series.

---

## 9. Sources

- `odoo-shortcuts`: `docs/features/formatter.md`, `docs/features/xpath-tools.md`,
  `README.md` §7, `docs/configuration.md`, `CHANGELOG.md`.
- Odoo: `odoo/tools/convert.py`, `odoo/tools/template_inheritance.py`,
  `odoo/tools/translate.py`, `odoo/addons/base/models/ir_ui_view.py`,
  `odoo/addons/base/models/ir_qweb.py`, `odoo/addons/base/models/ir_model.py`,
  `odoo/addons/base/rng/common.rng`, `odoo/addons/test_lint/tests/test_markers.py`.
- Empirical counts across Odoo 15.0/17.0/18.0/19.0/20.0 trees (~24,255 XML
  files).

Counts and citations are reproducible; rule severity/fix choices are
engineering judgements recorded in [`RULES.md`](./RULES.md).
