# ruxx for VS Code

Odoo-aware XML diagnostics, quick fixes and formatting, powered by
[`ruxx server`](https://github.com/ppreeper/ruxx) (the ruxx language server).

The extension is a thin LSP client: all linting and formatting happens in the
`ruxx` binary you already have, so the extension never goes out of date
relative to the CLI.

## Features

- **Diagnostics as you type** for every XML file — the same rule catalog the CLI
  ships (`X`, `O`, `QW`, `XP`, `VW`, `SEC`, `ST`, `RUX` families), with
  Odoo-specific rules gated by path.
- **Quick fixes** — `Ctrl+.` / `Cmd+.` offers the rule's automatic fix, safe and
  unsafe alike, labelled with the diagnostic message.
- **Format document and selection** — `Shift+Alt+F` (or *Format Selection*) runs
  the ruxx formatter, honouring your `.ruxx.toml` `[format]` options.
- **Cross-file rules** work in the editor too: set `[odoo].modules-path` in
  `.ruxx.toml` and duplicate-xmlid / uninstalled-module / t-call checks light up.
- Diagnostics come from `.ruxx.toml` discovery per file, so nested projects with
  different settings behave as they do on the command line.

## Requirements

**Installed from the Marketplace or Open VSX: nothing to install.** Each
platform package (`linux-x64`, `linux-arm64`, `darwin-x64`, `darwin-arm64`,
`win32-x64`, `win32-arm64`) ships the matching `ruxx` binary, so the extension
works out of the box. On a platform with no specific package — the generic
package, or another architecture — the extension falls back to a `ruxx` on
`PATH`:

```sh
go install github.com/ppreeper/ruxx/cmd/ruxx@latest
```

or build it from a checkout (`go build -o ruxx ./cmd/ruxx`) and point
`ruxx.binaryPath` at it.

## Settings

| Setting | Default | Description |
| --- | --- | --- |
| `ruxx.binaryPath` | `""` | Path to the `ruxx` binary; empty uses a binary next to the extension, then `ruxx` on `PATH`. |
| `ruxx.configPath` | `""` | Passed as `--config <path>`; empty uses `.ruxx.toml` discovery. |
| `ruxx.trace.server` | `off` | `off` \| `messages` \| `verbose` JSON-RPC tracing. |

Commands: **ruxx: Restart language server**, **ruxx: Show output channel**.

## Build and install from source

```sh
cd editors/vscode
npm install
npm run build        # bundles src/ into dist/extension.js
npm run compile      # type-check only
npm run package          # generic package, no bundled binary
npm run package:targets  # six platform packages, each bundling the binary
code --install-extension ruxx-xml-0.1.1@linux-x64.vsix
```

To hack on the extension, open this folder in VS Code and press <kbd>F5</kbd>
(*Run ruxx extension*) — it launches an Extension Development Host with this
folder as the extension.

## Notes

- The client uses full-text synchronisation and the server pushes diagnostics,
  so there is nothing to configure to get started.
- VS Code's built-in XML support also offers a formatter. To make ruxx the
  default, add to your settings:

  ```json
  "[xml]": { "editor.defaultFormatter": "ppreeper.ruxx-xml" }
  ```

- Formatting a *selection* is supported through the LSP range-formatting
  request; on the command line the equivalent is
  `ruxx format --range start_line:col-end_line:col`.

## License

MIT — see [LICENSE](LICENSE).
