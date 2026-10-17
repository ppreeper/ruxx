// Package lsp implements a small Language Server Protocol endpoint for ruxx.
//
// It speaks JSON-RPC 2.0 over stdio using Content-Length framing and supports:
//
//   - initialize / initialized / shutdown / exit lifecycle messages
//   - textDocument/didOpen, didChange, didClose -> textDocument/publishDiagnostics
//   - textDocument/formatting            -> a whole-document TextEdit
//   - textDocument/codeAction            -> quickfix CodeActions for fixable diagnostics
//
// The server is deliberately narrow: full-document sync only, edits confined to
// the single document being edited, and no LSP features unrelated to lint/format.
// Diagnostics are produced by the same linter used on the command line, so the
// editor and CI agree.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/ppreeper/ruxx/internal/diagnostic"
	"github.com/ppreeper/ruxx/internal/formatter"
	"github.com/ppreeper/ruxx/internal/index"
	"github.com/ppreeper/ruxx/internal/linter"
	"github.com/ppreeper/ruxx/internal/settings"
	"github.com/ppreeper/ruxx/internal/source"
	"github.com/ppreeper/ruxx/internal/syntax"
	"github.com/ppreeper/ruxx/internal/version"
)

// Version is reported in the initialize result's serverInfo. The value is
// stamped into internal/version at build time.
var Version = version.Version

// Options configures a Server. Both hooks are optional: when SettingsFor is nil
// the built-in defaults are used, and when IndexFor is nil no cross-file index
// is built (so index-dependent rules stay silent).
type Options struct {
	SettingsFor func(path string) *settings.Settings
	IndexFor    func(s *settings.Settings) *index.Index
}

// Run serves LSP on the given streams until the client exits or the stream
// ends, returning a process exit code: 0 for a clean shutdown or EOF, 1 when
// the client sends "exit" without a preceding "shutdown" (per the LSP spec).
func Run(in io.Reader, out io.Writer, opts Options) int {
	s := &Server{conn: NewConn(in, out), opts: opts, docs: map[string]string{}}
	return s.Serve()
}

// message is a JSON-RPC request, notification, or response.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// response is a successful JSON-RPC response. Result is always serialized
// (even when nil, as `"result":null`) so strict clients see a well-formed reply.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result"`
}

// errorResponse is a failed JSON-RPC response. It never carries a result field.
type errorResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   *responseError  `json:"error"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// Conn reads and writes Content-Length framed JSON-RPC messages.
type Conn struct {
	r *bufio.Reader
	w io.Writer
}

// NewConn wraps the given streams.
func NewConn(r io.Reader, w io.Writer) *Conn {
	return &Conn{r: bufio.NewReader(r), w: w}
}

// Read reads one framed message. It returns io.EOF at the end of the stream.
func (c *Conn) Read() (*message, error) {
	length := -1
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			if err == io.EOF && line == "" {
				return nil, io.EOF
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if name, value, ok := strings.Cut(line, ":"); ok &&
			strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length %q", value)
			}
			length = n
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(c.r, buf); err != nil {
		return nil, err
	}
	var m message
	if err := json.Unmarshal(buf, &m); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	return &m, nil
}

// Write writes one framed message.
func (c *Conn) Write(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	_, err = c.w.Write(data)
	return err
}

// Server is an LSP endpoint over a single Conn.
type Server struct {
	conn     *Conn
	opts     Options
	docs     map[string]string // uri -> text
	shutdown bool
}

// Serve runs the message loop until exit or EOF.
func (s *Server) Serve() int {
	for {
		m, err := s.conn.Read()
		if err != nil {
			return 0
		}
		if m.Method == "" {
			continue // a response to one of our requests; nothing to do
		}
		switch m.Method {
		case "initialize":
			s.reply(m.ID, map[string]any{
				"capabilities": capabilities(),
				"serverInfo":   map[string]string{"name": "ruxx", "version": Version},
			})
		case "initialized":
			// notification, no reply
		case "shutdown":
			s.shutdown = true
			s.reply(m.ID, nil)
		case "exit":
			if s.shutdown {
				return 0
			}
			return 1
		case "textDocument/didOpen":
			s.didOpen(m.Params)
		case "textDocument/didChange":
			s.didChange(m.Params)
		case "textDocument/didClose":
			s.didClose(m.Params)
		case "textDocument/formatting":
			s.formatting(m.ID, m.Params)
		case "textDocument/rangeFormatting":
			s.rangeFormatting(m.ID, m.Params)
		case "textDocument/codeAction":
			s.codeAction(m.ID, m.Params)
		default:
			if isRequest(m.ID) {
				s.replyError(m.ID, -32601, "method not found: "+m.Method)
			}
		}
	}
}

func capabilities() map[string]any {
	return map[string]any{
		"textDocumentSync":                1, // full
		"documentFormattingProvider":      true,
		"documentRangeFormattingProvider": true,
		"codeActionProvider":              true,
	}
}

func isRequest(id json.RawMessage) bool {
	return len(id) > 0 && string(id) != "null"
}

func (s *Server) reply(id json.RawMessage, result any) {
	_ = s.conn.Write(response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) replyError(id json.RawMessage, code int, msg string) {
	_ = s.conn.Write(errorResponse{JSONRPC: "2.0", ID: id, Error: &responseError{Code: code, Message: msg}})
}

func (s *Server) notify(method string, params any) {
	_ = s.conn.Write(notification{JSONRPC: "2.0", Method: method, Params: params})
}

// --- document sync ---

type textDocumentItem struct {
	URI  string `json:"uri"`
	Text string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type didChangeParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

type didCloseParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

func (s *Server) didOpen(params json.RawMessage) {
	var p didOpenParams
	if json.Unmarshal(params, &p) != nil {
		return
	}
	s.docs[p.TextDocument.URI] = p.TextDocument.Text
	s.publish(p.TextDocument.URI)
}

func (s *Server) didChange(params json.RawMessage) {
	var p didChangeParams
	if json.Unmarshal(params, &p) != nil || len(p.ContentChanges) == 0 {
		return
	}
	// Full-document sync: the last change carries the whole document.
	s.docs[p.TextDocument.URI] = p.ContentChanges[len(p.ContentChanges)-1].Text
	s.publish(p.TextDocument.URI)
}

func (s *Server) didClose(params json.RawMessage) {
	var p didCloseParams
	if json.Unmarshal(params, &p) != nil {
		return
	}
	delete(s.docs, p.TextDocument.URI)
	s.notify("textDocument/publishDiagnostics", publishParams{URI: p.TextDocument.URI, Diagnostics: []lspDiagnostic{}})
}

// --- diagnostic mapping ---

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type lspDiagnostic struct {
	Range    lspRange `json:"range"`
	Severity int      `json:"severity,omitempty"`
	Code     string   `json:"code,omitempty"`
	Source   string   `json:"source,omitempty"`
	Message  string   `json:"message"`
}

type publishParams struct {
	URI         string          `json:"uri"`
	Diagnostics []lspDiagnostic `json:"diagnostics"`
}

func toRange(loc *source.Locator, r source.Range) lspRange {
	startLine, startCol := loc.LineCol(r.Start)
	endLine, endCol := loc.LineCol(r.End)
	return lspRange{
		Start: position{Line: startLine - 1, Character: startCol - 1},
		End:   position{Line: endLine - 1, Character: endCol - 1},
	}
}

func severityOf(sev diagnostic.Severity) int {
	switch sev {
	case diagnostic.SeverityWarning:
		return 2
	case diagnostic.SeverityHint:
		return 4
	default:
		return 1
	}
}

func (s *Server) settingsFor(path string) *settings.Settings {
	if s.opts.SettingsFor != nil {
		if st := s.opts.SettingsFor(path); st != nil {
			return st
		}
	}
	return settings.Default()
}

func (s *Server) indexFor(st *settings.Settings) *index.Index {
	if s.opts.IndexFor != nil {
		return s.opts.IndexFor(st)
	}
	return nil
}

func (s *Server) publish(uri string) {
	text, ok := s.docs[uri]
	if !ok {
		return
	}
	path := pathFromURI(uri)
	st := s.settingsFor(path)
	diags := linter.CheckFile(syntax.Parse(text), path, st, s.indexFor(st))
	loc := source.NewLocator(text)
	items := make([]lspDiagnostic, 0, len(diags))
	for _, d := range diags {
		items = append(items, lspDiagnostic{
			Range:    toRange(loc, d.Range),
			Severity: severityOf(d.Severity),
			Code:     d.Code,
			Source:   "ruxx",
			Message:  d.Message,
		})
	}
	s.notify("textDocument/publishDiagnostics", publishParams{URI: uri, Diagnostics: items})
}

// pathFromURI converts a file:// URI to a filesystem path, leaving anything
// else untouched.
func pathFromURI(uri string) string {
	const prefix = "file://"
	if !strings.HasPrefix(uri, prefix) {
		return uri
	}
	rest := strings.TrimPrefix(uri, prefix)
	if u, err := url.Parse(uri); err == nil && u.Path != "" {
		return u.Path
	}
	return rest
}

// --- formatting ---

type formattingParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

type textEdit struct {
	Range   lspRange `json:"range"`
	NewText string   `json:"newText"`
}

func (s *Server) formatting(id json.RawMessage, params json.RawMessage) {
	var p formattingParams
	if json.Unmarshal(params, &p) != nil {
		return
	}
	text, ok := s.docs[p.TextDocument.URI]
	if !ok {
		s.reply(id, []textEdit{})
		return
	}
	st := s.settingsFor(pathFromURI(p.TextDocument.URI))
	out, err := formatter.Format(text, st)
	if err != nil {
		s.replyError(id, -32603, err.Error())
		return
	}
	if out == text {
		s.reply(id, []textEdit{})
		return
	}
	loc := source.NewLocator(text)
	full := lspRange{Start: position{Line: 0, Character: 0}, End: toRange(loc, source.NewRange(len(text), len(text))).End}
	s.reply(id, []textEdit{{Range: full, NewText: out}})
}

type rangeFormattingParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Range lspRange `json:"range"`
}

// rangeFormatting formats only the element enclosing the requested range,
// leaving the rest of the document untouched.
func (s *Server) rangeFormatting(id json.RawMessage, params json.RawMessage) {
	var p rangeFormattingParams
	if json.Unmarshal(params, &p) != nil {
		return
	}
	uri := p.TextDocument.URI
	text, ok := s.docs[uri]
	if !ok {
		s.reply(id, []textEdit{})
		return
	}
	loc := source.NewLocator(text)
	// LSP positions are 0-based; source.Locator columns are 1-based.
	rng := source.NewRange(
		loc.Offset(p.Range.Start.Line+1, p.Range.Start.Character+1),
		loc.Offset(p.Range.End.Line+1, p.Range.End.Character+1),
	)
	st := s.settingsFor(pathFromURI(uri))
	out, err := formatter.FormatRange(text, st, rng)
	if err != nil {
		s.replyError(id, -32603, err.Error())
		return
	}
	if out == text {
		s.reply(id, []textEdit{})
		return
	}
	// The edit covers the whole document: applying a partial patch would mean
	// re-deriving offsets the client can compute just as well.
	full := lspRange{Start: position{Line: 0, Character: 0}, End: toRange(loc, source.NewRange(len(text), len(text))).End}
	s.reply(id, []textEdit{{Range: full, NewText: out}})
}

// --- code actions ---
type codeActionParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Context struct {
		Diagnostics []struct {
			Code string `json:"code"`
		} `json:"diagnostics"`
	} `json:"context"`
}

type workspaceEdit struct {
	Changes map[string][]textEdit `json:"changes"`
}

type codeAction struct {
	Title string         `json:"title"`
	Kind  string         `json:"kind"`
	Edit  *workspaceEdit `json:"edit,omitempty"`
}

func (s *Server) codeAction(id json.RawMessage, params json.RawMessage) {
	var p codeActionParams
	if json.Unmarshal(params, &p) != nil {
		s.reply(id, []codeAction{})
		return
	}
	uri := p.TextDocument.URI
	text, ok := s.docs[uri]
	if !ok {
		s.reply(id, []codeAction{})
		return
	}
	wanted := map[string]bool{}
	for _, d := range p.Context.Diagnostics {
		if d.Code != "" {
			wanted[d.Code] = true
		}
	}
	path := pathFromURI(uri)
	st := s.settingsFor(path)
	diags := linter.CheckFile(syntax.Parse(text), path, st, s.indexFor(st))
	loc := source.NewLocator(text)

	actions := []codeAction{}
	for _, d := range diags {
		if len(wanted) > 0 && !wanted[d.Code] {
			continue
		}
		if d.Fix == nil || len(d.Fix.Edits) == 0 {
			continue
		}
		edits := make([]textEdit, 0, len(d.Fix.Edits))
		for _, e := range d.Fix.Edits {
			edits = append(edits, textEdit{Range: toRange(loc, e.Range), NewText: e.NewText})
		}
		actions = append(actions, codeAction{
			Title: "ruxx: " + d.Message,
			Kind:  "quickfix",
			Edit:  &workspaceEdit{Changes: map[string][]textEdit{uri: edits}},
		})
	}
	s.reply(id, actions)
}
