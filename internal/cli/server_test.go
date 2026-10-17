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
