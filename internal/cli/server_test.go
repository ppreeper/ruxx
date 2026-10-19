package cli_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

// lspFrame wraps a JSON message in an LSP Content-Length frame.
func lspFrame(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), data)
}

func TestServer_Initialize(t *testing.T) {
	var in strings.Builder
	in.WriteString(lspFrame(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}}))
	in.WriteString(lspFrame(t, map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}}))
	in.WriteString(lspFrame(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "shutdown"}))
	in.WriteString(lspFrame(t, map[string]any{"jsonrpc": "2.0", "method": "exit"}))

	code, stdout, stderr := run(t, in.String(), "server")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "textDocumentSync") {
		t.Fatalf("stdout missing capabilities: %q", stdout)
	}
}

func TestServer_DidOpenPublishesDiagnostics(t *testing.T) {
	var in strings.Builder
	in.WriteString(lspFrame(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}}))
	in.WriteString(lspFrame(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{
		"textDocument": map[string]any{"uri": "file:///tmp/foo.xml", "text": badO004},
	}}))
	in.WriteString(lspFrame(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "shutdown"}))
	in.WriteString(lspFrame(t, map[string]any{"jsonrpc": "2.0", "method": "exit"}))

	code, stdout, _ := run(t, in.String(), "server")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stdout, `"O004"`) {
		t.Fatalf("stdout missing O004: %q", stdout)
	}
}

// vscode-languageclient appends `--stdio --clientProcessId=<pid>` when it
// launches an Executable server with the stdio transport; the server must
// accept (and ignore) both instead of exiting with "unknown flag".
func TestServer_AcceptsLSPClientFlags(t *testing.T) {
	var in strings.Builder
	in.WriteString(lspFrame(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}}))
	in.WriteString(lspFrame(t, map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}}))
	in.WriteString(lspFrame(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "shutdown"}))
	in.WriteString(lspFrame(t, map[string]any{"jsonrpc": "2.0", "method": "exit"}))

	code, stdout, stderr := run(t, in.String(), "server", "--stdio", "--clientProcessId=1234")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "textDocumentSync") {
		t.Fatalf("stdout missing capabilities: %q", stdout)
	}
}

// The compatibility flags belong to the server; other subcommands still
// reject unknown flags.
func TestServer_StdioFlagRejectedElsewhere(t *testing.T) {
	if code, _, _ := run(t, "", "check", "--stdio"); code != cli.ExitAbnormal {
		t.Errorf("check --stdio: exit = %d, want %d", code, cli.ExitAbnormal)
	}
}
