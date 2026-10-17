package lsp_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/lsp"
	"github.com/ppreeper/ruxx/internal/settings"
)

const badO004 = "<?xml version=\"1.0\"?>\n<odoo>\n    <record id=\"foo\">\n    </record>\n</odoo>\n"

type outMsg struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func frame(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), data)
}

// runServer frames each raw JSON message, serves it, and returns the exit code
// plus every message the server wrote.
func runServer(t *testing.T, opts lsp.Options, reqs ...string) (int, []outMsg) {
	t.Helper()
	var in strings.Builder
	for _, r := range reqs {
		in.WriteString(frame(t, json.RawMessage(r)))
	}
	var out bytes.Buffer
	code := lsp.Run(strings.NewReader(in.String()), &out, opts)
	return code, readAll(t, out.String())
}

func readAll(t *testing.T, s string) []outMsg {
	t.Helper()
	r := bufio.NewReader(strings.NewReader(s))
	var msgs []outMsg
	for {
		length := -1
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return msgs
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if name, value, ok := strings.Cut(line, ":"); ok &&
				strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
				length, _ = strconv.Atoi(strings.TrimSpace(value))
			}
		}
		if length < 0 {
			return msgs
		}
		buf := make([]byte, length)
		if _, err := io.ReadFull(r, buf); err != nil {
			t.Fatalf("short frame: %v", err)
		}
		var m outMsg
		if err := json.Unmarshal(buf, &m); err != nil {
			t.Fatalf("bad json frame %q: %v", buf, err)
		}
		msgs = append(msgs, m)
	}
}

func raw(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func didOpen(uri, text string) string {
	return raw(map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{"uri": uri, "text": text}},
	})
}

func request(id int, method string, params any) string {
	return raw(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
}

func findResponse(t *testing.T, msgs []outMsg, id int) outMsg {
	t.Helper()
	want := json.RawMessage(strconv.Itoa(id))
	for _, m := range msgs {
		if bytes.Equal(m.ID, want) {
			return m
		}
	}
	t.Fatalf("no response with id %d in %+v", id, msgs)
	return outMsg{}
}

func findNotification(msgs []outMsg, method string) (outMsg, bool) {
	for _, m := range msgs {
		if m.Method == method {
			return m, true
		}
	}
	return outMsg{}, false
}

func TestInitialize(t *testing.T) {
	code, msgs := runServer(t, lsp.Options{},
		request(1, "initialize", map[string]any{}),
		request(2, "shutdown", nil),
		raw(map[string]any{"jsonrpc": "2.0", "method": "exit"}),
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var result struct {
		Capabilities struct {
			TextDocumentSync           int  `json:"textDocumentSync"`
			DocumentFormattingProvider bool `json:"documentFormattingProvider"`
			CodeActionProvider         bool `json:"codeActionProvider"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	resp := findResponse(t, msgs, 1)
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("result: %v", err)
	}
	if result.Capabilities.TextDocumentSync != 1 {
		t.Errorf("textDocumentSync = %d, want 1", result.Capabilities.TextDocumentSync)
	}
	if !result.Capabilities.DocumentFormattingProvider || !result.Capabilities.CodeActionProvider {
		t.Errorf("capabilities missing formatting/codeAction: %+v", result.Capabilities)
	}
	if result.ServerInfo.Name != "ruxx" {
		t.Errorf("serverInfo.name = %q, want ruxx", result.ServerInfo.Name)
	}
}

func TestDidOpenPublishesDiagnostics(t *testing.T) {
	_, msgs := runServer(t, lsp.Options{},
		didOpen("file:///tmp/a.xml", badO004),
	)
	n, ok := findNotification(msgs, "textDocument/publishDiagnostics")
	if !ok {
		t.Fatalf("no publishDiagnostics in %+v", msgs)
	}
	var p struct {
		URI         string `json:"uri"`
		Diagnostics []struct {
			Code     string `json:"code"`
			Severity int    `json:"severity"`
			Source   string `json:"source"`
			Message  string `json:"message"`
			Range    struct {
				Start struct{ Line, Character int } `json:"start"`
			} `json:"range"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(n.Params, &p); err != nil {
		t.Fatalf("params: %v", err)
	}
	if p.URI != "file:///tmp/a.xml" {
		t.Errorf("uri = %q", p.URI)
	}
	if len(p.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want 1", p.Diagnostics)
	}
	d := p.Diagnostics[0]
	if d.Code != "O004" || d.Severity != 1 || d.Source != "ruxx" {
		t.Errorf("diag = %+v, want O004/1/ruxx", d)
	}
	if d.Range.Start.Line != 2 || d.Range.Start.Character != 4 {
		t.Errorf("start = %d:%d, want 2:4", d.Range.Start.Line, d.Range.Start.Character)
	}
}

func TestFormatting(t *testing.T) {
	const src = `<odoo><record id="a" model="ir.ui.view"/></odoo>`
	uri := "file:///tmp/f.xml"
	_, msgs := runServer(t, lsp.Options{},
		didOpen(uri, src),
		request(2, "textDocument/formatting", map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"options":      map[string]any{"tabSize": 4, "insertSpaces": true},
		}),
	)
	resp := findResponse(t, msgs, 2)
	var edits []struct {
		Range   json.RawMessage `json:"range"`
		NewText string          `json:"newText"`
	}
	if err := json.Unmarshal(resp.Result, &edits); err != nil {
		t.Fatalf("result: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("edits = %+v, want 1", edits)
	}
	if !strings.Contains(edits[0].NewText, "\n    <record id=\"a\" model=\"ir.ui.view\"/>\n") {
		t.Errorf("formatted text not expanded:\n%s", edits[0].NewText)
	}
}

func TestFormatting_AlreadyFormattedIsNoOp(t *testing.T) {
	const src = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<odoo>\n    <record id=\"a\" model=\"ir.ui.view\"/>\n</odoo>\n"
	uri := "file:///tmp/g.xml"
	_, msgs := runServer(t, lsp.Options{},
		didOpen(uri, src),
		request(2, "textDocument/formatting", map[string]any{"textDocument": map[string]any{"uri": uri}}),
	)
	resp := findResponse(t, msgs, 2)
	var edits []any
	if err := json.Unmarshal(resp.Result, &edits); err != nil {
		t.Fatalf("result: %v", err)
	}
	if len(edits) != 0 {
		t.Errorf("edits = %+v, want none", edits)
	}
}

func TestCodeAction_QuickFix(t *testing.T) {
	const src = "<?xml version=\"1.0\"?>\n<odoo>\n    <field name=\"x\"></field>\n</odoo>\n"
	uri := "file:///tmp/c.xml"
	opts := lsp.Options{SettingsFor: func(path string) *settings.Settings {
		s := settings.Default()
		s.Select = []string{"ST002"}
		return s
	}}
	_, msgs := runServer(t, opts,
		didOpen(uri, src),
		request(3, "textDocument/codeAction", map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"range":        map[string]any{},
			"context": map[string]any{"diagnostics": []any{
				map[string]any{"code": "ST002"},
			}},
		}),
	)
	resp := findResponse(t, msgs, 3)
	var actions []struct {
		Title string `json:"title"`
		Kind  string `json:"kind"`
		Edit  *struct {
			Changes map[string][]struct {
				NewText string `json:"newText"`
			} `json:"changes"`
		} `json:"edit"`
	}
	if err := json.Unmarshal(resp.Result, &actions); err != nil {
		t.Fatalf("result: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions = %+v, want 1", actions)
	}
	if actions[0].Kind != "quickfix" || actions[0].Edit == nil {
		t.Fatalf("action = %+v", actions[0])
	}
	if got := actions[0].Edit.Changes[uri]; len(got) != 1 || got[0].NewText != "/>" {
		t.Errorf("edit = %+v, want single '/>'", got)
	}
}

func TestUnknownMethodReturnsError(t *testing.T) {
	_, msgs := runServer(t, lsp.Options{}, request(7, "workspace/whatever", map[string]any{}))
	resp := findResponse(t, msgs, 7)
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("resp = %+v, want -32601", resp)
	}
}

func TestExitWithoutShutdownReturnsOne(t *testing.T) {
	code, _ := runServer(t, lsp.Options{}, raw(map[string]any{"jsonrpc": "2.0", "method": "exit"}))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestDidCloseClearsDiagnostics(t *testing.T) {
	uri := "file:///tmp/d.xml"
	_, msgs := runServer(t, lsp.Options{},
		didOpen(uri, badO004),
		raw(map[string]any{"jsonrpc": "2.0", "method": "textDocument/didClose",
			"params": map[string]any{"textDocument": map[string]any{"uri": uri}}}),
	)
	// The last publishDiagnostics for this uri must be empty.
	var last string
	for _, m := range msgs {
		if m.Method == "textDocument/publishDiagnostics" {
			last = string(m.Params)
		}
	}
	if !strings.Contains(last, `"diagnostics":[]`) {
		t.Errorf("final publish = %s, want empty diagnostics", last)
	}
}

func TestInitialize_RangeFormattingCapability(t *testing.T) {
	_, msgs := runServer(t, lsp.Options{}, request(1, "initialize", map[string]any{}))
	resp := findResponse(t, msgs, 1)
	if !strings.Contains(string(resp.Result), "documentRangeFormattingProvider") {
		t.Errorf("capabilities missing range formatting: %s", resp.Result)
	}
}

func TestRangeFormatting(t *testing.T) {
	const src = "<?xml version=\"1.0\"?>\n<odoo>\n    <record id=\"a\" model=\"ir.ui.view\"   >\n        <field name=\"name\">A</field>\n    </record>\n    <record id=\"b\" model=\"ir.ui.view\"   >\n        <field name=\"name\">B</field>\n    </record>\n</odoo>\n"
	uri := "file:///tmp/r.xml"
	_, msgs := runServer(t, lsp.Options{},
		didOpen(uri, src),
		request(2, "textDocument/rangeFormatting", map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"range": map[string]any{
				"start": map[string]any{"line": 2, "character": 4},
				"end":   map[string]any{"line": 2, "character": 40},
			},
		}),
	)
	resp := findResponse(t, msgs, 2)
	var edits []struct {
		NewText string `json:"newText"`
	}
	if err := json.Unmarshal(resp.Result, &edits); err != nil {
		t.Fatalf("result: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("edits = %+v, want 1", edits)
	}
	if !strings.Contains(edits[0].NewText, "<record id=\"a\" model=\"ir.ui.view\">") {
		t.Errorf("selection not formatted:\n%s", edits[0].NewText)
	}
	if !strings.Contains(edits[0].NewText, "\"ir.ui.view\"   >") {
		t.Errorf("rest of the document changed:\n%s", edits[0].NewText)
	}
}
