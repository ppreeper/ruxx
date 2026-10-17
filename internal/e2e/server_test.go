package e2e

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// frame renders one LSP message the way an editor puts it on the wire.
func frame(t *testing.T, v any) string {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

func lspRequest(id int, method string, params any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
}

func lspNotification(method string, params any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
}

// TestServer_HandshakePublishesDiagnostics runs the language server the way an
// editor does: initialize, open a document, shutdown, exit.
func TestServer_HandshakePublishesDiagnostics(t *testing.T) {
	dir := t.TempDir()
	const uri = "file:///ruxx-e2e/a.xml"

	msgs := []any{
		lspRequest(1, "initialize", map[string]any{
			"processId":    nil,
			"rootUri":      nil,
			"capabilities": map[string]any{},
		}),
		lspNotification("initialized", map[string]any{}),
		lspNotification("textDocument/didOpen", map[string]any{
			"textDocument": map[string]any{
				"uri": uri, "languageId": "xml", "version": 1, "text": badView,
			},
		}),
		lspRequest(2, "shutdown", nil),
		lspNotification("exit", nil),
	}
	var in strings.Builder
	for _, m := range msgs {
		in.WriteString(frame(t, m))
	}

	got := run(t, dir, in.String(), "server")
	if got.code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", got.code, got.stderr)
	}
	for _, want := range []string{
		`"serverInfo"`,
		`"documentFormattingProvider":true`,
		`"documentRangeFormattingProvider":true`,
		"textDocument/publishDiagnostics",
		"O004",
	} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, got.stdout)
		}
	}
}

// TestServer_CodeActionResult checks the quick-fix surface an editor shows.
func TestServer_CodeActionResult(t *testing.T) {
	dir := t.TempDir()
	const uri = "file:///ruxx-e2e/dup.xml"
	// Two records with the same id: X014 is in the default set and has a fix.
	dup := "<?xml version=\"1.0\"?>\n<odoo>\n    <record id=\"dup\" model=\"ir.ui.view\"/>\n    <record id=\"dup\" model=\"ir.ui.view\"/>\n</odoo>\n"

	msgs := []any{
		lspRequest(1, "initialize", map[string]any{"processId": nil, "capabilities": map[string]any{}}),
		lspNotification("initialized", map[string]any{}),
		lspNotification("textDocument/didOpen", map[string]any{
			"textDocument": map[string]any{"uri": uri, "languageId": "xml", "version": 1, "text": dup},
		}),
		lspRequest(2, "textDocument/codeAction", map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"range":        map[string]any{"start": map[string]int{"line": 2, "character": 4}, "end": map[string]int{"line": 2, "character": 8}},
			"context":      map[string]any{"diagnostics": []map[string]any{{"code": "X014"}}},
		}),
		lspRequest(3, "shutdown", nil),
		lspNotification("exit", nil),
	}
	var in strings.Builder
	for _, m := range msgs {
		in.WriteString(frame(t, m))
	}

	got := run(t, dir, in.String(), "server")
	if got.code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", got.code, got.stderr)
	}
	for _, want := range []string{"X014", "quickfix", "duplicate id"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, got.stdout)
		}
	}
}

// TestServer_FormattingRequest asserts the whole-document edit an editor asks
// for on save.
func TestServer_FormattingRequest(t *testing.T) {
	dir := t.TempDir()
	const uri = "file:///ruxx-e2e/unformatted.xml"

	msgs := []any{
		lspRequest(1, "initialize", map[string]any{"processId": nil, "capabilities": map[string]any{}}),
		lspNotification("initialized", map[string]any{}),
		lspNotification("textDocument/didOpen", map[string]any{
			"textDocument": map[string]any{"uri": uri, "languageId": "xml", "version": 1, "text": unformattedView},
		}),
		lspRequest(2, "textDocument/formatting", map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"options":      map[string]any{"tabSize": 4, "insertSpaces": true},
		}),
		lspRequest(3, "shutdown", nil),
		lspNotification("exit", nil),
	}
	var in strings.Builder
	for _, m := range msgs {
		in.WriteString(frame(t, m))
	}

	got := run(t, dir, in.String(), "server")
	if got.code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", got.code, got.stderr)
	}
	// The edit carries the formatted document; a record on its own line is the
	// easiest signal that the formatting happened inside the response.
	if !strings.Contains(got.stdout, `"newText"`) {
		t.Errorf("stdout is missing a TextEdit newText:\n%s", got.stdout)
	}
	if ok := strings.Contains(got.stdout, `\u003crecord id=\"a\" model=\"ir.ui.view\"\u003e`); !ok {
		t.Errorf("the edit does not contain the formatted record:\n%s", got.stdout)
	}
}
