package lsp_test

import (
	"testing"

	"github.com/ppreeper/ruxx/internal/lsp"
)

// TestShutdownReplyHasNullResult locks the JSON-RPC shape: a successful request
// whose payload is empty must still carry `"result":null` rather than omitting
// the field entirely.
func TestShutdownReplyHasNullResult(t *testing.T) {
	_, msgs := runServer(t, lsp.Options{},
		request(1, "initialize", map[string]any{}),
		request(2, "shutdown", nil),
	)
	r := findResponse(t, msgs, 2)
	if string(r.Result) != "null" {
		t.Fatalf("shutdown result = %q, want null", r.Result)
	}
}
