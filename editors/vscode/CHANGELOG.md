# Changelog

## 0.1.1

- Fixed the language server failing to start with `write EPIPE` in VS Code.
  `vscode-languageclient` appends `--stdio` when it launches the server; this
  release pairs with ruxx 0.1.1, whose server accepts the flag.
- Platform packages (`linux-x64`, `linux-arm64`, `darwin-x64`, `darwin-arm64`,
  `win32-x64`, `win32-arm64`) bundle the matching `ruxx` binary, so installing
  from the Marketplace or Open VSX needs no separate setup. The generic package
  keeps the old behaviour and uses a `ruxx` on `PATH`.
- Added the extension icon, homepage, issue tracker and gallery banner.

## 0.1.0

- Initial release: LSP client for `ruxx server`.
- Diagnostics for XML files (push model, full-text sync).
- Quick fixes via code actions, including unsafe fixes.
- Document and range formatting through the ruxx formatter.
- Settings: `ruxx.binaryPath`, `ruxx.configPath`, `ruxx.trace.server`.
- Commands: restart the language server, show the output channel.
