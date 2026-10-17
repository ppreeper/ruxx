# Editor & CI integration

`ruxx` is a single binary with two integration surfaces: the CLI (for CI and
hooks) and a language server (for editors). Both run the same 83 rules over the
same lossless parser, so an editor and CI never disagree.

## Scaffolding a project

```console
$ ruxx init                 # writes .ruxx.toml in the current directory
$ ruxx init addons/ --force # elsewhere, overwriting
```

The starter config is valid immediately and is what `ruxx` uses by default, so
you only edit it to *change* behaviour. Every key is commented where the value
is not yet consumed.

## CLI (CI, pre-commit, hooks)

```console
$ ruxx check .                        # lint; exit 1 on findings, 2 on error
$ ruxx check --output-format sarif -o ruxx.sarif .
$ ruxx check --statistics .
$ ruxx format --check .               # exit 1 if anything would change
$ ruxx check --add-ignore .            # write suppressions for existing findings
```

Exit codes are the ESLint/Prettier/Ruff contract: `0` clean, `1` findings,
`2` abnormal termination. `NO_COLOR` and `--color auto|always|never` control
colour; machine-readable formats are never decorated.

### pre-commit

The repository ships hooks, so pin a revision and get everything with no local
scripting:

```yaml
- repo: https://github.com/ppreeper/ruxx
  rev: v0.1.0
  hooks:
    - id: ruxx-check    # lint; non-zero exit on findings
    - id: ruxx-format   # rewrite with the formatter
```

`ruxx-check` runs `ruxx check --force-exclude` and `ruxx-format` runs
`ruxx format --force-exclude`, so your `exclude` patterns apply to the files
pre-commit hands over. Both use `language: golang`: pre-commit builds the binary
from source, so a Go toolchain is required but no pre-built `ruxx`.

To use the binary already on your `PATH` instead:

```yaml
- repo: local
  hooks:
    - id: ruxx
      name: ruxx
      entry: ruxx check --force-exclude
      language: system
      files: \.(xml|qweb|xml\.template)$
```

## Shell completions

`ruxx generate-shell-completion <bash|zsh|fish|powershell>` prints a completion
script covering the subcommands, every flag, the output formats, the target
versions, the colour modes, and rule selectors (`ALL`, categories, families and
rule codes).

```console
# bash
$ ruxx generate-shell-completion bash > /etc/bash_completion.d/ruxx   # or ~/.local/share/bash-completion/completions/ruxx

# zsh (first fpath entry)
$ ruxx generate-shell-completion zsh > "${fpath[1]}/_ruxx"

# fish
$ ruxx generate-shell-completion fish > ~/.config/fish/completions/ruxx.fish

# PowerShell
$ ruxx generate-shell-completion powershell >> $PROFILE
```

## Language server

`ruxx server` speaks LSP over stdio and provides:

- **diagnostics** — the same rules as `ruxx check`, published on open and change
- **formatting** — `ruxx format` as a whole-document edit
- **range formatting** — `ruxx format --range` for a selection; the rest of the
  document is returned untouched
- **code actions** — a `quickfix` for every fixable diagnostic

It is full-sync (whole document per change) and reads `.ruxx.toml`
per file, so per-directory configuration works. `[odoo].modules-path` is
honoured, so cross-file rules (`O016`, `O018`, `QW012`, `SEC002`, `SEC003`)
work in the editor too.

### Neovim (0.11+)

```lua
vim.lsp.config.ruxx = {
  cmd = { "ruxx", "server" },
  filetypes = { "xml" },
  root_markers = { ".ruxx.toml", "ruxx.toml", ".git" },
}
vim.lsp.enable("ruxx")
```

On older versions, start it per buffer:

```lua
vim.api.nvim_create_autocmd("FileType", {
  pattern = "xml",
  callback = function()
    vim.lsp.start({ name = "ruxx", cmd = { "ruxx", "server" }, root_dir = vim.fn.getcwd() })
  end,
})
```

### Helix

```toml
[[language]]
name = "xml"
language-servers = ["ruxx"]

[language-server.ruxx]
command = "ruxx"
args = ["server"]
```

### VS Code

A thin LSP client ships in [`editors/vscode`](../editors/vscode). It is a plain
stdio client for `ruxx server`, so it uses whatever `ruxx` binary you have:

```console
$ cd editors/vscode
$ npm install
$ npm run build          # bundles src/ into dist/extension.js
$ npm run package        # ruxx-xml-0.1.0.vsix
$ code --install-extension ruxx-xml-0.1.0.vsix
```

Or open `editors/vscode` in VS Code and press <kbd>F5</kbd> to run it in an
Extension Development Host.

Settings: `ruxx.binaryPath` (empty looks for a `ruxx` binary next to the
extension, then `ruxx` on `PATH`), `ruxx.configPath` (passed as `--config`), and
`ruxx.trace.server`. Commands: *ruxx: Restart language server* and
*ruxx: Show output channel*. To make ruxx the default XML formatter:

```json
"[xml]": { "editor.defaultFormatter": "ppreeper.ruxx-xml" }
```

Prefer the CLI in a task? Run `ruxx check ${relativeFile}` on save:

```json
{
  "task.allowAutomaticTasks": "on",
  "tasks": {
    "version": "2.0.0",
    "tasks": [
      {
        "label": "ruxx check",
        "type": "shell",
        "command": "ruxx check ${relativeFile}",
        "problemMatcher": []
      }
    ]
  }
}
```

`ruxx check --output-format github` emits `::error file=…,line=…::CODE message`
lines, which CI annotates directly; `sarif` and `gitlab`/`junit` are also
available for the usual dashboards.
