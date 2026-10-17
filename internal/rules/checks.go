package rules

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ppreeper/ruxx/internal/attrorder"
	"github.com/ppreeper/ruxx/internal/diagnostic"
	"github.com/ppreeper/ruxx/internal/index"
	"github.com/ppreeper/ruxx/internal/settings"
	"github.com/ppreeper/ruxx/internal/source"
	"github.com/ppreeper/ruxx/internal/suppress"
	"github.com/ppreeper/ruxx/internal/syntax"
)

// ctx carries everything a rule needs while it runs.
type ctx struct {
	src      string
	tree     *syntax.Tree
	loc      *source.Locator
	meta     *metaInfo
	path     string
	odoo     bool // whether Odoo-specific rules should run for this input
	settings *settings.Settings
	index    *index.Index
	set      *Set
}

// metaInfo is available only to meta rules (RUX).
type metaInfo struct {
	dirs       []suppress.Directive
	suppressed map[int]map[string]bool // directive index -> codes it suppressed
}

// check is a registered rule implementation.
type check struct {
	code string
	meta bool
	odoo bool // rule only applies to Odoo data/view files
	run  func(*ctx) []diagnostic.Diagnostic
}

// Check runs every enabled rule over tree and returns diagnostics in stable
// order, applying suppression directives and then the meta (RUX) rules.
func Check(tree *syntax.Tree, s *settings.Settings) []diagnostic.Diagnostic {
	return CheckFile(tree, "", s, nil)
}

// CheckFile is Check with the file's path, used to decide whether Odoo-specific
// rules apply. An empty path means "run everything" (used by tests and by
// callers that have no meaningful path).
func CheckFile(tree *syntax.Tree, path string, s *settings.Settings, ix *index.Index) []diagnostic.Diagnostic {
	if tree == nil {
		return nil
	}
	var sel, ign []string
	odooEnabled := true
	previewOn := false
	if s != nil {
		sel, ign = s.Select, s.Ignore
		odooEnabled = s.OdooEnabled
		previewOn = s.Preview
	}
	set, err := ResolveWith(sel, ign, previewOn)
	if err != nil {
		return nil
	}
	c := &ctx{
		src:      tree.Src,
		tree:     tree,
		loc:      source.NewLocator(tree.Src),
		path:     path,
		odoo:     odooEnabled && odooContext(path, tree),
		settings: s,
		index:    ix,
		set:      set,
	}

	// Non-meta rules first.
	var raw []diagnostic.Diagnostic
	for _, chk := range checks {
		if chk.meta || !set.Enabled(chk.code) {
			continue
		}
		if chk.odoo && !c.odoo {
			continue
		}
		raw = append(raw, chk.run(c)...)
	}

	// Apply suppression directives. --ignore-suppressions (audit mode) skips
	// them entirely; the directive-policing meta rules then see no directives
	// and report nothing.
	dirs := []suppress.Directive(nil)
	suppressed := map[int]map[string]bool{}
	var emitted []diagnostic.Diagnostic
	if c.settings != nil && c.settings.IgnoreSuppressions {
		emitted = append(emitted, raw...)
	} else {
		dirs = suppress.Parse(c.src)
		for _, d := range raw {
			if i := suppressingIndex(dirs, d.Range.Start, d.Code); i >= 0 {
				if suppressed[i] == nil {
					suppressed[i] = map[string]bool{}
				}
				suppressed[i][d.Code] = true
				continue
			}
			emitted = append(emitted, d)
		}
	}

	// Meta rules.
	c.meta = &metaInfo{dirs: dirs, suppressed: suppressed}
	for _, chk := range checks {
		if !chk.meta || !set.Enabled(chk.code) {
			continue
		}
		emitted = append(emitted, chk.run(c)...)
	}

	diagnostic.Sort(emitted)
	return emitted
}

// suppressingIndex returns the index of the directive suppressing code at off,
// or -1 if none does.
func suppressingIndex(dirs []suppress.Directive, off int, code string) int {
	for i, d := range dirs {
		if d.Kind == suppress.Ignore && (len(d.Codes) == 0 || containsStr(d.Codes, code)) {
			return i
		}
	}
	activeAll := -1
	active := map[string]int{}
	for i, d := range dirs {
		if d.Range.Start >= off {
			continue
		}
		switch d.Kind {
		case suppress.Disable:
			if len(d.Codes) == 0 {
				activeAll = i
			} else {
				for _, c := range d.Codes {
					active[c] = i
				}
			}
		case suppress.Enable:
			if len(d.Codes) == 0 {
				activeAll = -1
				active = map[string]int{}
			} else {
				for _, c := range d.Codes {
					delete(active, c)
				}
			}
		}
	}
	if i, ok := active[code]; ok {
		return i
	}
	return activeAll
}

// ---- helpers ---------------------------------------------------------------

func diag(code string, rng source.Range, msg string) diagnostic.Diagnostic {
	name := code
	sev := diagnostic.SeverityError
	if r, ok := ByCode(code); ok {
		name = r.Name
	}
	return diagnostic.Diagnostic{Code: code, Name: name, Severity: sev, Range: rng, Message: msg}
}

func diagFix(code string, rng source.Range, msg string, fix *diagnostic.Fix) diagnostic.Diagnostic {
	d := diag(code, rng, msg)
	d.Fix = fix
	return d
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// walkElements visits every element, passing its parent element (nil at root).
func walkElements(n *syntax.Node, parent *syntax.Node, fn func(el, parent *syntax.Node)) {
	for _, c := range n.Children {
		if c.Kind != syntax.KindElement {
			continue
		}
		fn(c, parent)
		walkElements(c, c, fn)
	}
}

// walkArch visits every element, tracking whether it is inside a <field
// name="arch" type="xml"> subtree.
func walkArch(root *syntax.Node, fn func(el *syntax.Node, inArch bool)) {
	var rec func(n *syntax.Node, inArch bool)
	rec = func(n *syntax.Node, inArch bool) {
		for _, ch := range n.Children {
			if ch.Kind != syntax.KindElement {
				continue
			}
			ia := inArch
			if ch.Name == "field" {
				if v, ok := ch.Attr("name"); ok && v == "arch" {
					ia = true
				}
			}
			fn(ch, ia)
			rec(ch, ia)
		}
	}
	rec(root, false)
}

func firstElementChild(n *syntax.Node) *syntax.Node {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if c.Kind == syntax.KindElement {
			return c
		}
	}
	return nil
}

// isDataRootName reports whether name is one of the Odoo data-file root
// elements under which records define external ids.
func isDataRootName(name string) bool {
	switch name {
	case "odoo", "data", "openerp":
		return true
	}
	return false
}

// isIdDefiner reports whether an element with the given parent defines an
// external id (i.e. it is a direct child of a data root, or the root itself),
// as opposed to a nested DOM id such as <field id="...">.
func isIdDefiner(parent *syntax.Node) bool {
	return parent == nil || isDataRootName(parent.Name)
}

// odooContext reports whether Odoo-specific rules should run for a file at
// path. An empty path runs everything. Otherwise a file qualifies when its root
// is a data root, or it lives in a conventional Odoo source directory and is
// not a known non-Odoo format (namespaced EDI/ODF documents, test fixtures).
func odooContext(path string, tree *syntax.Tree) bool {
	if path == "" {
		return true
	}
	// The path wins over the root element name: client-asset and test trees are
	// never Odoo data, even when they happen to use an <odoo> root.
	if isNonOdooPath(path) {
		return false
	}
	name := ""
	if root := firstElementChild(tree.Root); root != nil {
		name = root.Name
	}
	if isDataRootName(name) {
		return true
	}
	if isNonOdooRoot(name) {
		return false
	}
	return isDataDir(path)
}

// isNonOdooPath reports whether path lies in a client-asset or test tree, where
// Odoo server-side rules do not apply regardless of the root element name.
func isNonOdooPath(path string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(path), "/") {
		switch seg {
		case "static", "tests", "test":
			return true
		}
	}
	return false
}

// isDataDir reports whether path sits in a conventional Odoo source directory,
// excluding test and static-asset trees.
func isDataDir(path string) bool {
	segs := strings.Split(filepath.ToSlash(path), "/")
	for _, seg := range segs {
		switch seg {
		case "static", "tests", "test":
			return false
		}
	}
	for _, seg := range segs {
		switch seg {
		case "views", "data", "security", "report", "wizard", "demo":
			return true
		}
	}
	return false
}

// isNonOdooRoot reports whether a root element is clearly not an Odoo data
// root (namespaced documents and known EDI/ODF roots).
func isNonOdooRoot(name string) bool {
	if strings.Contains(name, ":") {
		return true
	}
	switch name {
	case "Invoice", "CreditNote", "InvoiceData":
		return true
	}
	return false
}

// ---- X family --------------------------------------------------------------

func checkX001(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	for _, e := range c.tree.Errors {
		if strings.HasPrefix(e.Message, "unterminated comment") || strings.HasPrefix(e.Message, "unterminated CDATA") {
			continue // owned by X017
		}
		out = append(out, diag("X001", e.Range, e.Message))
	}
	return out
}

// xmlDeclaration is the header inserted by the X002 fix.
const xmlDeclaration = `<?xml version="1.0" encoding="utf-8"?>` + "\n"

func checkX002(c *ctx) []diagnostic.Diagnostic {
	for _, ch := range c.tree.Root.Children {
		if ch.Kind != syntax.KindProcessingInstruction {
			continue
		}
		target := strings.ToLower(c.src[ch.Range.Start:ch.Range.End])
		if strings.HasPrefix(target, "<?xml") {
			return nil
		}
	}
	fix := &diagnostic.Fix{
		Applicability: diagnostic.ApplicabilitySafe,
		Edits:         []diagnostic.Edit{{Range: source.NewRange(0, 0), NewText: xmlDeclaration}},
	}
	return []diagnostic.Diagnostic{
		diagFix("X002", source.NewRange(0, 0), "missing XML declaration", fix),
	}
}

func checkX007(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	off := 0
	for off <= len(c.src) {
		nl := strings.IndexByte(c.src[off:], '\n')
		end := len(c.src)
		if nl >= 0 {
			end = off + nl
		}
		trimEnd := end
		if trimEnd > off && c.src[trimEnd-1] == '\r' {
			trimEnd--
		}
		start := trimEnd
		for start > off && (c.src[start-1] == ' ' || c.src[start-1] == '\t') {
			start--
		}
		if start < trimEnd {
			fix := &diagnostic.Fix{
				Applicability: diagnostic.ApplicabilitySafe,
				Edits:         []diagnostic.Edit{{Range: source.NewRange(start, trimEnd), NewText: ""}},
			}
			out = append(out, diagFix("X007", source.NewRange(start, trimEnd), "trailing whitespace", fix))
		}
		if nl < 0 {
			break
		}
		off = end + 1
	}
	return out
}

func checkX011(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	off := 0
	for {
		nl := strings.IndexByte(c.src[off:], '\n')
		end := len(c.src)
		if nl >= 0 {
			end = off + nl
		}
		text := strings.TrimSpace(c.src[off:end])
		if strings.HasPrefix(text, "<<<<<<<") || strings.HasPrefix(text, "|||||||") ||
			strings.HasPrefix(text, ">>>>>>>") || text == "=======" {
			out = append(out, diag("X011", source.NewRange(off, off+len(text)), "merge conflict marker found"))
		}
		if nl < 0 {
			break
		}
		off = end + 1
	}
	return out
}

func checkX014(c *ctx) []diagnostic.Diagnostic {
	seen := map[string]int{}
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, parent *syntax.Node) {
		if !isIdDefiner(parent) {
			return
		}
		id, ok := n.Attr("id")
		if !ok {
			return
		}
		first, dup := seen[id]
		if !dup {
			seen[id] = n.Range.Start
			return
		}
		firstLine, _ := c.loc.LineCol(first)
		msg := fmt.Sprintf("duplicate id %q in this file (first defined at line %d)", id, firstLine)
		newID := uniqueID(id, seen)
		edit, ok := attrValueEdit(c, n, "id", newID)
		if !ok {
			out = append(out, diag("X014", n.Range, msg))
			return
		}
		seen[newID] = n.Range.Start
		fix := &diagnostic.Fix{
			Applicability: diagnostic.ApplicabilityUnsafe,
			Message:       fmt.Sprintf("rename duplicate id %q to %q", id, newID),
			Edits:         []diagnostic.Edit{edit},
		}
		out = append(out, diagFix("X014", n.Range, msg, fix))
	})
	return out
}

// uniqueID returns a free identifier derived from base (base_2, base_3, ...).
func uniqueID(base string, taken map[string]int) string {
	for i := 2; i < 1000; i++ {
		cand := fmt.Sprintf("%s_%d", base, i)
		if _, ok := taken[cand]; !ok {
			return cand
		}
	}
	return ""
}

// attrValueEdit builds an edit that replaces only the text inside the quotes of
// the named attribute, preserving the original quote character.
func attrValueEdit(c *ctx, n *syntax.Node, attrName, newValue string) (diagnostic.Edit, bool) {
	a := attrNode(n, attrName)
	if a == nil {
		return diagnostic.Edit{}, false
	}
	raw := c.src[a.Range.Start:a.Range.End]
	eq := strings.IndexByte(raw, '=')
	if eq < 0 {
		return diagnostic.Edit{}, false
	}
	rest := raw[eq+1:]
	if len(rest) < 2 || (rest[0] != '"' && rest[0] != '\'') {
		return diagnostic.Edit{}, false
	}
	closeRel := strings.LastIndexByte(raw, rest[0])
	if closeRel <= eq+1 {
		return diagnostic.Edit{}, false
	}
	return diagnostic.Edit{
		Range:   source.NewRange(a.Range.Start+eq+2, a.Range.Start+closeRel),
		NewText: newValue,
	}, true
}

func checkX015(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, parent *syntax.Node) {
		if !isIdDefiner(parent) {
			return
		}
		id, ok := n.Attr("id")
		if ok && strings.Count(id, ".") > 1 {
			out = append(out, diag("X015", n.Range, fmt.Sprintf("external id %q must contain at most one dot", id)))
		}
	})
	return out
}

func checkX017(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	for _, e := range c.tree.Errors {
		if strings.HasPrefix(e.Message, "unterminated comment") {
			out = append(out, diag("X017", e.Range, e.Message))
		}
	}
	return out
}

// ---- O family --------------------------------------------------------------

// ---- X family (style & hygiene) --------------------------------------------

// indentWidth returns the configured indentation width.
func (c *ctx) indentWidth() int {
	if c.settings != nil && c.settings.IndentWidth > 0 {
		return c.settings.IndentWidth
	}
	return settings.DefaultIndentWidth
}

// lineLength returns the configured maximum line length.
func (c *ctx) lineLength() int {
	if c.settings != nil && c.settings.LineLength > 0 {
		return c.settings.LineLength
	}
	return settings.DefaultLineLength
}

func checkX003(c *ctx) []diagnostic.Diagnostic {
	for _, ch := range c.tree.Root.Children {
		if ch.Kind != syntax.KindProcessingInstruction {
			continue
		}
		text := c.src[ch.Range.Start:ch.Range.End]
		if !strings.HasPrefix(strings.ToLower(text), "<?xml") {
			return nil
		}
		if strings.Contains(text, `encoding="utf-8"`) {
			return nil
		}
		fix := &diagnostic.Fix{
			Applicability: diagnostic.ApplicabilitySafe,
			Edits: []diagnostic.Edit{
				{Range: ch.Range, NewText: `<?xml version="1.0" encoding="utf-8"?>`},
			},
		}
		return []diagnostic.Diagnostic{
			diagFix("X003", ch.Range, `XML declaration should use encoding="utf-8"`, fix),
		}
	}
	return nil
}

func checkX004(c *ctx) []diagnostic.Diagnostic {
	const bom = "\ufeff"
	if !strings.HasPrefix(c.src, bom) {
		return nil
	}
	rng := source.NewRange(0, len(bom))
	fix := &diagnostic.Fix{
		Applicability: diagnostic.ApplicabilitySafe,
		Edits:         []diagnostic.Edit{{Range: rng, NewText: ""}},
	}
	return []diagnostic.Diagnostic{diagFix("X004", rng, "UTF-8 byte order mark at start of file", fix)}
}

func checkX005(c *ctx) []diagnostic.Diagnostic {
	i := strings.Index(c.src, "\r\n")
	if i < 0 {
		return nil
	}
	rng := source.NewRange(i, i+2)
	fix := &diagnostic.Fix{
		Applicability: diagnostic.ApplicabilitySafe,
		Edits: []diagnostic.Edit{
			{Range: source.NewRange(0, len(c.src)), NewText: strings.ReplaceAll(c.src, "\r\n", "\n")},
		},
	}
	return []diagnostic.Diagnostic{diagFix("X005", rng, "CRLF line ending; use LF", fix)}
}

func checkX006(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	w := c.indentWidth()
	forEachLine(c.src, func(off, end int) {
		lead := leadingWhitespace(c.src, off, end)
		ws := c.src[off : off+lead]
		if !strings.ContainsRune(ws, '\t') {
			return
		}
		fixed := strings.ReplaceAll(ws, "\t", strings.Repeat(" ", w))
		rng := source.NewRange(off, off+lead)
		fix := &diagnostic.Fix{
			Applicability: diagnostic.ApplicabilitySafe,
			Edits:         []diagnostic.Edit{{Range: rng, NewText: fixed}},
		}
		out = append(out, diagFix("X006", rng, "tab used for indentation; use spaces", fix))
	})
	return out
}

func checkX008(c *ctx) []diagnostic.Diagnostic {
	w := c.indentWidth()
	if w <= 0 {
		return nil
	}
	var out []diagnostic.Diagnostic
	forEachLine(c.src, func(off, end int) {
		lead := 0
		for off+lead < end && c.src[off+lead] == ' ' {
			lead++
		}
		if lead == 0 || lead%w == 0 {
			return
		}
		trim := lead % w
		rng := source.NewRange(off, off+lead)
		fix := &diagnostic.Fix{
			Applicability: diagnostic.ApplicabilitySafe,
			Edits:         []diagnostic.Edit{{Range: source.NewRange(off+lead-trim, off+lead), NewText: ""}},
		}
		out = append(out, diagFix("X008", rng,
			fmt.Sprintf("indentation is not a multiple of %d spaces", w), fix))
	})
	return out
}

func checkX009(c *ctx) []diagnostic.Diagnostic {
	firstTab, firstSpace := -1, -1
	forEachLine(c.src, func(off, end int) {
		lead := leadingWhitespace(c.src, off, end)
		ws := c.src[off : off+lead]
		if firstTab < 0 && strings.ContainsRune(ws, '\t') {
			firstTab = off
		}
		if firstSpace < 0 && strings.ContainsRune(ws, ' ') {
			firstSpace = off
		}
	})
	if firstTab < 0 || firstSpace < 0 {
		return nil
	}
	at := firstTab
	if firstSpace < at {
		at = firstSpace
	}
	return []diagnostic.Diagnostic{diag("X009", source.NewRange(at, at), "file mixes tabs and spaces for indentation")}
}

func checkX010(c *ctx) []diagnostic.Diagnostic {
	lim := c.lineLength()
	var out []diagnostic.Diagnostic
	forEachLine(c.src, func(off, end int) {
		n := end - off
		if n <= lim {
			return
		}
		out = append(out, diag("X010", source.NewRange(off+lim, off+n),
			fmt.Sprintf("line length %d exceeds %d", n, lim)))
	})
	return out
}

func checkX012(c *ctx) []diagnostic.Diagnostic {
	if len(c.src) == 0 || strings.HasSuffix(c.src, "\n") {
		return nil
	}
	rng := source.NewRange(len(c.src), len(c.src))
	fix := &diagnostic.Fix{
		Applicability: diagnostic.ApplicabilitySafe,
		Edits:         []diagnostic.Edit{{Range: rng, NewText: "\n"}},
	}
	return []diagnostic.Diagnostic{diagFix("X012", rng, "file does not end with a newline", fix)}
}

func checkX013(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		for _, a := range n.Attrs {
			text := c.src[a.Range.Start:a.Range.End]
			eq := strings.IndexByte(text, '=')
			if eq < 0 {
				continue
			}
			namePart := text[:eq]
			valPart := text[eq+1:]
			if !endsWithSpace(namePart) && !startsWithSpace(valPart) {
				continue
			}
			fixed := strings.TrimRight(namePart, " \t") + "=" + strings.TrimLeft(valPart, " \t")
			fix := &diagnostic.Fix{
				Applicability: diagnostic.ApplicabilitySafe,
				Edits:         []diagnostic.Edit{{Range: a.Range, NewText: fixed}},
			}
			out = append(out, diagFix("X013", a.Range, "unexpected whitespace around '='", fix))
		}
	})
	return out
}

func checkX016(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		gt := startTagEnd(c.src, n)
		if gt < 0 {
			return
		}
		pos := n.Range.Start + 1 + len(n.Name)
		if len(n.Attrs) > 0 {
			pos = n.Attrs[len(n.Attrs)-1].Range.End
		}
		if pos < 0 || pos > gt {
			return
		}
		between := c.src[pos:gt]
		// Whitespace containing a line break is the formatter's own multi-line
		// layout (e.g. a split tag ending in "\n    />"), not a spacing
		// mistake, so only same-line whitespace is reported.
		if strings.ContainsAny(between, "\n\r") {
			return
		}
		var want string
		if n.SelfClosed {
			if between == "/" {
				return
			}
			want = "/"
		} else {
			if between == "" {
				return
			}
			want = ""
		}
		rng := source.NewRange(pos, gt+1)
		fix := &diagnostic.Fix{
			Applicability: diagnostic.ApplicabilitySafe,
			Edits:         []diagnostic.Edit{{Range: rng, NewText: want + ">"}},
		}
		out = append(out, diagFix("X016", rng, "unexpected whitespace inside the start tag", fix))
	})
	return out
}

// forEachLine calls fn with the byte range [off,end) of each line, excluding
// the trailing newline.
func forEachLine(src string, fn func(off, end int)) {
	off := 0
	for off <= len(src) {
		nl := strings.IndexByte(src[off:], '\n')
		end := len(src)
		if nl >= 0 {
			end = off + nl
		}
		fn(off, end)
		if nl < 0 {
			return
		}
		off = end + 1
	}
}

// leadingWhitespace returns the count of leading spaces/tabs in src[off:end].
func leadingWhitespace(src string, off, end int) int {
	n := 0
	for off+n < end {
		b := src[off+n]
		if b != ' ' && b != '\t' {
			break
		}
		n++
	}
	return n
}

func startsWithSpace(s string) bool { return len(s) > 0 && (s[0] == ' ' || s[0] == '\t') }
func endsWithSpace(s string) bool {
	return len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t')
}

// startTagEnd returns the index of the '>' that closes an element's start tag,
// or -1 when it cannot be located.
func startTagEnd(src string, n *syntax.Node) int {
	pos := n.Range.Start + 1 + len(n.Name)
	if len(n.Attrs) > 0 {
		pos = n.Attrs[len(n.Attrs)-1].Range.End
	}
	if pos < 0 || pos > len(src) {
		return -1
	}
	i := strings.IndexByte(src[pos:], '>')
	if i < 0 {
		return -1
	}
	return pos + i
}

func checkO001(c *ctx) []diagnostic.Diagnostic {
	root := firstElementChild(c.tree.Root)
	if root == nil {
		return nil
	}
	if isDataRootName(root.Name) {
		return nil
	}
	if !hasOdooDataChild(root) {
		return nil // not an Odoo data file (e.g. an EDI/ODF or test fragment)
	}
	return []diagnostic.Diagnostic{
		diag("O001", root.Range, fmt.Sprintf("invalid root element <%s>; expected <odoo> or <data>", root.Name)),
	}
}

// odooDataTags are the record-defining elements that make a document Odoo data.
var odooDataTags = map[string]bool{
	"record": true, "menuitem": true, "template": true, "data": true,
	"asset": true, "act_window": true, "report": true, "function": true,
	"delete": true,
}

// hasOdooDataChild reports whether n has a direct child that defines Odoo data.
func hasOdooDataChild(n *syntax.Node) bool {
	for _, ch := range n.ChildElements() {
		if odooDataTags[ch.Name] {
			return true
		}
	}
	return false
}

func checkO002(c *ctx) []diagnostic.Diagnostic {
	root := firstElementChild(c.tree.Root)
	if root == nil || root.Name != "openerp" {
		return nil
	}
	name := root.Name
	edits := []diagnostic.Edit{
		{Range: source.NewRange(root.Range.Start+1, root.Range.Start+1+len(name)), NewText: "odoo"},
	}
	if !root.SelfClosed {
		endTagStart := root.Range.End - (len(name) + 3)
		if endTagStart > root.Range.Start {
			edits = append(edits, diagnostic.Edit{
				Range:   source.NewRange(endTagStart+2, endTagStart+2+len(name)),
				NewText: "odoo",
			})
		}
	}
	fix := &diagnostic.Fix{Applicability: diagnostic.ApplicabilitySafe, Edits: edits}
	return []diagnostic.Diagnostic{
		diagFix("O002", root.Range, "deprecated <openerp> root; use <odoo>", fix),
	}
}

func checkO004(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name == "record" {
			if _, ok := n.Attr("model"); !ok {
				out = append(out, diag("O004", n.Range, `<record> is missing the required "model" attribute`))
			}
		}
	})
	return out
}

func checkO005(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name == "field" {
			if _, ok := n.Attr("name"); !ok {
				out = append(out, diag("O005", n.Range, `<field> is missing the required "name" attribute`))
			}
		}
	})
	return out
}

func checkO008(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name == "act_window" {
			out = append(out, diag("O008", n.Range, `deprecated <act_window> sugar; use <record model="ir.actions.act_window">`))
		}
	})
	return out
}

func checkO009(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name == "report" {
			out = append(out, diag("O009", n.Range, `deprecated <report> sugar; use <record model="ir.actions.report">`))
		}
	})
	return out
}

func checkO003(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, parent *syntax.Node) {
		if n.Name != "record" || !isIdDefiner(parent) {
			return
		}
		if _, ok := n.Attr("id"); !ok {
			out = append(out, diag("O003", n.Range, `<record> is missing the required "id" attribute`))
		}
	})
	return out
}

func checkO006(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name != "field" {
			return
		}
		if nm, ok := n.Attr("name"); !ok || nm != "arch" {
			return
		}
		if _, ok := n.Attr("type"); !ok {
			out = append(out, diag("O006", n.Range, `<field name="arch"> should declare type="xml"`))
		}
	})
	return out
}

func checkO007(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name != "record" {
			return
		}
		if _, ok := n.Attr("noupdate"); ok {
			out = append(out, diag("O007", n.Range, `"noupdate" belongs on <odoo>/<data>, not <record>`))
		}
	})
	return out
}

func checkO012(c *ctx) []diagnostic.Diagnostic {
	ver := c.targetVersion()
	if ver == "" || seriesMajor(ver) < 17 {
		return nil
	}
	var out []diagnostic.Diagnostic
	walkArch(c.tree.Root, func(el *syntax.Node, inArch bool) {
		if !inArch {
			return
		}
		for _, a := range el.Attrs {
			if a.AttrName == "attrs" {
				out = append(out, diag("O012", a.Range, `"attrs" was removed in Odoo 17.0; use invisible/readonly/required directly`))
			}
		}
	})
	return out
}

func checkO013(c *ctx) []diagnostic.Diagnostic {
	ver := c.targetVersion()
	if ver == "" || seriesMajor(ver) < 17 {
		return nil
	}
	var out []diagnostic.Diagnostic
	walkArch(c.tree.Root, func(el *syntax.Node, inArch bool) {
		if !inArch {
			return
		}
		for _, a := range el.Attrs {
			if a.AttrName == "states" {
				out = append(out, diag("O013", a.Range, `"states" was removed in Odoo 17.0; use invisible= instead`))
			}
		}
	})
	return out
}

func checkO014(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		for _, a := range n.Attrs {
			if a.AttrName == "group" {
				out = append(out, diag("O014", a.Range, `"group" is a typo for "groups"`))
			}
		}
	})
	return out
}

func checkO015(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name == "menuitem" {
			if _, ok := n.Attr("id"); !ok {
				out = append(out, diag("O015", n.Range, `<menuitem> is missing the required "id" attribute`))
			}
		}
	})
	return out
}

// targetVersion returns the effective Odoo series (e.g. "18.0") for a run, or
// "" when it cannot be determined. A concrete target-version wins; "auto"
// infers from a version-looking path segment such as "18.0".
func (c *ctx) targetVersion() string {
	if c.settings != nil {
		if v := c.settings.TargetVersion; v != "" && v != "auto" {
			return v
		}
	}
	for _, seg := range strings.Split(filepath.ToSlash(c.path), "/") {
		if isSeries(seg) {
			return seg
		}
	}
	return ""
}

// isSeries reports whether seg looks like an Odoo series ("15.0".."20.0").
func isSeries(seg string) bool {
	dot := strings.IndexByte(seg, '.')
	if dot <= 0 || dot == len(seg)-1 {
		return false
	}
	major, err := strconv.Atoi(seg[:dot])
	if err != nil || major < 10 {
		return false
	}
	if _, err := strconv.Atoi(seg[dot+1:]); err != nil {
		return false
	}
	return true
}

// seriesMajor returns the major number of a series string ("18.0" -> 18).
func seriesMajor(v string) int {
	if dot := strings.IndexByte(v, '.'); dot > 0 {
		v = v[:dot]
	}
	n, _ := strconv.Atoi(v)
	return n
}

// renameElementFix renames an element's start and end tags to `to`.
func renameElementFix(n *syntax.Node, to string) *diagnostic.Fix {
	edits := []diagnostic.Edit{
		{Range: source.NewRange(n.Range.Start+1, n.Range.Start+1+len(n.Name)), NewText: to},
	}
	if !n.SelfClosed {
		endTagStart := n.Range.End - (len(n.Name) + 3)
		if endTagStart > n.Range.Start {
			edits = append(edits, diagnostic.Edit{
				Range:   source.NewRange(endTagStart+2, endTagStart+2+len(n.Name)),
				NewText: to,
			})
		}
	}
	return &diagnostic.Fix{Applicability: diagnostic.ApplicabilitySafe, Edits: edits}
}

// checkO010 flags <tree>/<list> that do not match the target Odoo series.
func checkO010(c *ctx) []diagnostic.Diagnostic {
	ver := c.targetVersion()
	if ver == "" {
		return nil
	}
	major := seriesMajor(ver)
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		switch {
		case n.Name == "tree" && major >= 18:
			out = append(out, diagFix("O010", n.Range,
				"<tree> is deprecated in Odoo 18.0+; use <list>", renameElementFix(n, "list")))
		case n.Name == "list" && major <= 17:
			out = append(out, diagFix("O010", n.Range,
				"<list> requires Odoo 18.0+; use <tree>", renameElementFix(n, "tree")))
		}
	})
	return out
}

// checkO011 checks view_mode values against the target Odoo series.
func checkO011(c *ctx) []diagnostic.Diagnostic {
	ver := c.targetVersion()
	if ver == "" {
		return nil
	}
	major := seriesMajor(ver)
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if v, ok := n.Attr("view_mode"); ok {
			out = append(out, viewModeDiags(n, v, major)...)
		}
		if n.Name == "field" {
			if nm, ok := n.Attr("name"); ok && nm == "view_mode" {
				if txt := textOf(c.src, n); txt != "" {
					out = append(out, viewModeDiags(n, txt, major)...)
				}
			}
		}
	})
	return out
}

func viewModeDiags(n *syntax.Node, value string, major int) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	parts := strings.Split(value, ",")
	// Odoo aliases the old name in both directions (`list` -> `tree` before
	// 18.0, `tree` -> `list` from 18.0 on), so only the post-18 rename is worth
	// flagging: it is the documented, user-facing deprecation.
	if major >= 18 && containsStr(parts, "tree") {
		out = append(out, diag("O011", n.Range, `view_mode uses "tree", which was renamed to "list" in Odoo 18.0`))
	}
	return out
}

// attrNode returns the attribute node with the given name, or nil.
func attrNode(n *syntax.Node, name string) *syntax.Node {
	for _, a := range n.Attrs {
		if a.AttrName == name {
			return a
		}
	}
	return nil
}

// attrIndex returns the position of the named attribute in n.Attrs, or -1.
func attrIndex(n *syntax.Node, name string) int {
	for i, a := range n.Attrs {
		if a.AttrName == name {
			return i
		}
	}
	return -1
}

// attrReorderFix builds a safe fix that rewrites the attribute region of n so the
// attributes appear in the given order (a permutation of indices). The gaps
// between attributes keep their original positions, so multi-line attribute
// layouts survive reordering.
func attrReorderFix(c *ctx, n *syntax.Node, order []int) *diagnostic.Fix {
	if len(n.Attrs) < 2 || len(order) != len(n.Attrs) {
		return nil
	}
	var b strings.Builder
	for i, idx := range order {
		if idx < 0 || idx >= len(n.Attrs) {
			return nil
		}
		b.WriteString(c.src[n.Attrs[idx].Range.Start:n.Attrs[idx].Range.End])
		if i < len(order)-1 {
			b.WriteString(c.src[n.Attrs[i].Range.End:n.Attrs[i+1].Range.Start])
		}
	}
	return &diagnostic.Fix{
		Applicability: diagnostic.ApplicabilitySafe,
		Edits: []diagnostic.Edit{{
			Range:   source.NewRange(n.Attrs[0].Range.Start, n.Attrs[len(n.Attrs)-1].Range.End),
			NewText: b.String(),
		}},
	}
}

// moduleOf infers the Odoo module name from a file path, or "" when unknown.
func moduleOf(path string) string {
	segs := strings.Split(filepath.ToSlash(path), "/")
	for i, s := range segs {
		if (s == "addons" || s == "enterprise") && i+1 < len(segs) {
			return segs[i+1]
		}
	}
	for i, s := range segs {
		switch s {
		case "views", "data", "security", "report", "wizard", "demo":
			if i > 0 {
				return segs[i-1]
			}
		}
	}
	return ""
}

// viewFieldRank is the canonical order of ir.ui.view field elements.
var viewFieldRank = map[string]int{
	"name": 0, "model": 1, "inherit_id": 2, "priority": 3, "arch": 4,
}

func checkO017(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		v, ok := n.Attr("noupdate")
		if !ok {
			return
		}
		switch strings.ToLower(v) {
		case "0", "1", "true", "false":
			return
		}
		if a := attrNode(n, "noupdate"); a != nil {
			out = append(out, diag("O017", a.Range, `"noupdate" should be "0", "1", "true", or "false"`))
		}
	})
	return out
}

func checkO019(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name != "record" {
			return
		}
		mi, ii := attrIndex(n, "model"), attrIndex(n, "id")
		if mi >= 0 && ii >= 0 && mi < ii {
			order := make([]int, len(n.Attrs))
			for i := range order {
				order[i] = i
			}
			order[mi], order[ii] = order[ii], order[mi]
			out = append(out, diagFix("O019", n.Range, `<record> attributes should be ordered "id" before "model"`, attrReorderFix(c, n, order)))
		}
	})
	return out
}

func checkO020(c *ctx) []diagnostic.Diagnostic {
	mod := moduleOf(c.path)
	if mod == "" {
		return nil
	}
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, parent *syntax.Node) {
		if !isIdDefiner(parent) {
			return
		}
		id, ok := n.Attr("id")
		if !ok || !strings.HasPrefix(id, mod+".") {
			return
		}
		if a := attrNode(n, "id"); a != nil {
			out = append(out, diag("O020", a.Range,
				fmt.Sprintf("id %q is already in module %q; omit the module prefix", id, mod)))
		}
	})
	return out
}

func checkO022(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name != "record" {
			return
		}
		if mv, _ := n.Attr("model"); mv != "ir.ui.view" {
			return
		}
		last := -1
		for _, ch := range n.ChildElements() {
			if ch.Name != "field" {
				continue
			}
			fname, ok := ch.Attr("name")
			if !ok {
				continue
			}
			rank, known := viewFieldRank[fname]
			if !known {
				continue
			}
			if rank < last {
				out = append(out, diag("O022", ch.Range,
					fmt.Sprintf("view field %q is out of order (name, model, inherit_id, priority, arch)", fname)))
			}
			last = rank
		}
	})
	return out
}

// ---- cross-file (index-powered) rules -------------------------------------

func checkO016(c *ctx) []diagnostic.Diagnostic {
	if c.index == nil {
		return nil
	}
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		for _, a := range n.Attrs {
			if a.AttrName != "ref" {
				continue
			}
			ref := a.AttrValue
			i := strings.IndexByte(ref, '.')
			if i <= 0 {
				continue
			}
			mod := ref[:i]
			if c.index.HasModule(mod) {
				continue
			}
			out = append(out, diag("O016", a.Range,
				fmt.Sprintf("reference %q targets module %q, which is not in the index", ref, mod)))
		}
	})
	return out
}

func checkO018(c *ctx) []diagnostic.Diagnostic {
	if c.index == nil {
		return nil
	}
	mod := moduleOf(c.path)
	if mod == "" {
		return nil
	}
	m, ok := c.index.Module(mod)
	if !ok {
		return nil
	}
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, parent *syntax.Node) {
		if !isIdDefiner(parent) {
			return
		}
		id, ok := n.Attr("id")
		if !ok || id == "" {
			return
		}
		q := id
		if !strings.Contains(id, ".") {
			q = mod + "." + id
		}
		if files := m.Defs[q]; len(files) > 1 {
			out = append(out, diag("O018", n.Range,
				fmt.Sprintf("xmlid %q is defined in %d files of module %q", q, len(files), mod)))
		}
	})
	return out
}

// coreTemplates are ubiquitous QWeb templates that live outside the indexed
// project tree; references to them are never reported by QW012.
var coreTemplates = map[string]bool{
	"web.external_layout": true,
	"web.basic_layout":    true,
	"web.layout":          true,
	"web.html_container":  true,
}

func checkQW012(c *ctx) []diagnostic.Diagnostic {
	if c.index == nil {
		return nil
	}
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		v, ok := n.Attr("t-call")
		if !ok || v == "" || coreTemplates[v] {
			return
		}
		if c.index.HasXMLID(v) {
			return
		}
		out = append(out, diag("QW012", n.Range,
			fmt.Sprintf("t-call target %q was not found in the index", v)))
	})
	return out
}

func checkSEC002(c *ctx) []diagnostic.Diagnostic {
	if c.index == nil {
		return nil
	}
	mod := moduleOf(c.path)
	if mod == "" {
		return nil
	}
	m, ok := c.index.Module(mod)
	if !ok || m.AccessCSV == "" {
		return nil
	}
	for _, d := range m.Data {
		if d == "security/ir.model.access.csv" {
			return nil
		}
	}
	return []diagnostic.Diagnostic{diag("SEC002", source.NewRange(0, 0),
		`"security/ir.model.access.csv" is not listed in the manifest "data"`)}
}

func checkSEC003(c *ctx) []diagnostic.Diagnostic {
	if c.index == nil {
		return nil
	}
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		v, ok := n.Attr("groups")
		if !ok || v == "" {
			return
		}
		for _, g := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' }) {
			i := strings.IndexByte(g, '.')
			if i <= 0 {
				continue
			}
			mod := g[:i]
			m, ok := c.index.Module(mod)
			if !ok || m.Groups[g] {
				continue
			}
			out = append(out, diag("SEC003", n.Range,
				fmt.Sprintf("group %q is not defined in module %q", g, mod)))
		}
	})
	return out
}

func checkO021(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, parent *syntax.Node) {
		if n.Name != "template" {
			return
		}
		if parent != nil && !isDataRootName(parent.Name) {
			return // not a sugar <template id=...> record at a data root
		}
		if _, ok := n.Attr("id"); !ok {
			out = append(out, diag("O021", n.Range, `<template> is missing the required "id" attribute`))
		}
	})
	return out
}

// ---- QW family -------------------------------------------------------------

func isOWLDirective(name string) bool {
	if strings.HasPrefix(name, "t-on-") {
		return true
	}
	switch name {
	case "t-model", "t-model-value", "t-ref", "t-component", "t-props",
		"t-slot", "t-portal", "t-created", "t-mounted", "t-willstart", "t-willpatch",
		"t-willdestroy", "t-el":
		return true
	}
	return false
}

// qwebDirectiveRank orders QWeb directives for QW010 (lower = earlier).
var qwebDirectiveRank = map[string]int{
	"t-name": 0, "t-call": 1,
	"t-foreach": 10, "t-as": 11, "t-if": 12, "t-elif": 13, "t-else": 14,
	"t-set": 20, "t-value": 21, "t-field": 22, "t-esc": 23, "t-raw": 24,
	"t-out": 25, "t-options": 26,
}

// qwebRank returns the ordering rank of a QWeb attribute.
func qwebRank(name string) (int, bool) {
	switch {
	case strings.HasPrefix(name, "t-att"):
		return 27, true
	case strings.HasPrefix(name, "t-on-"):
		return 28, true
	}
	r, ok := qwebDirectiveRank[name]
	return r, ok
}

func checkQW001(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		for _, a := range n.Attrs {
			if a.AttrName != "t-esc" {
				continue
			}
			nameRange := source.NewRange(a.Range.Start, a.Range.Start+len("t-esc"))
			fix := &diagnostic.Fix{Applicability: diagnostic.ApplicabilityUnsafe,
				Edits: []diagnostic.Edit{{Range: nameRange, NewText: "t-out"}}}
			out = append(out, diagFix("QW001", a.Range, `"t-esc" is deprecated; use "t-out"`, fix))
		}
	})
	return out
}

func checkQW002(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		for _, a := range n.Attrs {
			if a.AttrName == "t-raw" {
				out = append(out, diag("QW002", a.Range, `"t-raw" was removed; use "t-out"`))
			}
		}
	})
	return out
}

func checkQW003(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		seenPlain := false
		var bad *syntax.Node
		var tIdx, plainIdx []int
		for i, a := range n.Attrs {
			if strings.HasPrefix(a.AttrName, "t-") {
				tIdx = append(tIdx, i)
				if seenPlain && bad == nil {
					bad = a
				}
			} else {
				plainIdx = append(plainIdx, i)
				seenPlain = true
			}
		}
		if bad != nil {
			order := append(append([]int{}, tIdx...), plainIdx...)
			out = append(out, diagFix("QW003", bad.Range, `QWeb "t-*" attributes should precede plain attributes`, attrReorderFix(c, n, order)))
		}
	})
	return out
}

func checkQW004(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, parent *syntax.Node) {
		if n.Name != "t" || parent == nil || parent.Name != "templates" {
			return
		}
		if _, ok := n.Attr("t-name"); !ok {
			out = append(out, diag("QW004", n.Range, `<t> template is missing a "t-name" attribute`))
		}
	})
	return out
}

func checkQW005(c *ctx) []diagnostic.Diagnostic {
	mod := moduleOf(c.path)
	if mod == "" {
		return nil
	}
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		v, ok := n.Attr("t-name")
		if !ok || strings.Contains(v, ".") {
			return
		}
		if a := attrNode(n, "t-name"); a != nil {
			out = append(out, diag("QW005", a.Range,
				fmt.Sprintf("t-name %q should be qualified as %q", v, mod+"."+v)))
		}
	})
	return out
}

func checkQW009(c *ctx) []diagnostic.Diagnostic {
	root := firstElementChild(c.tree.Root)
	if root == nil || root.Name != "templates" || root.SelfClosed {
		return nil
	}
	if _, ok := root.Attr("xml:space"); ok {
		return nil
	}
	gt := startTagEnd(c.src, root)
	if gt < 0 {
		return nil
	}
	fix := &diagnostic.Fix{Applicability: diagnostic.ApplicabilitySafe,
		Edits: []diagnostic.Edit{{Range: source.NewRange(gt, gt), NewText: ` xml:space="preserve"`}}}
	return []diagnostic.Diagnostic{
		diagFix("QW009", root.Range, `<templates> root should set xml:space="preserve"`, fix),
	}
}

func checkQW010(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		last := -1
		for _, a := range n.Attrs {
			r, ok := qwebRank(a.AttrName)
			if !ok {
				continue
			}
			if r < last {
				out = append(out, diag("QW010", a.Range,
					fmt.Sprintf("%q is out of the canonical QWeb directive order", a.AttrName)))
				return
			}
			last = r
		}
	})
	return out
}

func checkQW011(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		for _, a := range n.Attrs {
			if a.AttrName == "t-jquery" {
				out = append(out, diag("QW011", a.Range, `"t-jquery" is obsolete`))
			}
		}
	})
	return out
}

func checkQW006(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkArch(c.tree.Root, func(el *syntax.Node, inArch bool) {
		if !inArch {
			return
		}
		for _, a := range el.Attrs {
			if isOWLDirective(a.AttrName) {
				out = append(out, diag("QW006", a.Range, fmt.Sprintf("OWL directive %q is not allowed in a view arch", a.AttrName)))
			}
		}
	})
	return out
}

func checkQW007(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkArch(c.tree.Root, func(el *syntax.Node, inArch bool) {
		if !inArch {
			return
		}
		for _, a := range el.Attrs {
			if a.AttrName == "__comp__" {
				out = append(out, diag("QW007", a.Range, `the "__comp__" attribute is not allowed in arch`))
			}
		}
	})
	return out
}

func checkQW008(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkArch(c.tree.Root, func(el *syntax.Node, inArch bool) {
		if !inArch {
			return
		}
		for _, a := range el.Attrs {
			if a.AttrName == "data-tooltip" {
				out = append(out, diag("QW008", a.Range, `the "data-tooltip" attribute is not allowed in arch; use tooltip`))
			}
		}
	})
	return out
}

// ---- XP family -------------------------------------------------------------

var validPositions = map[string]bool{
	"before": true, "after": true, "inside": true,
	"replace": true, "attributes": true, "move": true,
}

func checkXP001(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name != "xpath" {
			return
		}
		if v, ok := n.Attr("expr"); !ok || strings.TrimSpace(v) == "" {
			out = append(out, diag("XP001", n.Range, `<xpath> is missing a non-empty "expr"`))
		}
	})
	return out
}

func checkXP003(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkArch(c.tree.Root, func(n *syntax.Node, inArch bool) {
		if !inArch || n.Name == "attribute" {
			return // only view-inheritance specs; <attribute> uses add/remove (XP008)
		}
		v, ok := n.Attr("position")
		if !ok || validPositions[v] {
			return
		}
		out = append(out, diag("XP003", n.Range,
			fmt.Sprintf("invalid position %q; expected before, after, inside, replace, attributes, or move", v)))
	})
	return out
}

func checkXP007(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		v, ok := n.Attr("position")
		if !ok || v != "attributes" {
			return
		}
		has := false
		var find func(*syntax.Node)
		find = func(n *syntax.Node) {
			for _, ch := range n.ChildElements() {
				if ch.Name == "attribute" {
					if nm, ok := ch.Attr("name"); ok && nm != "" {
						has = true
					}
				}
				find(ch)
			}
		}
		find(n)
		if !has {
			out = append(out, diag("XP007", n.Range, `position="attributes" requires <attribute name="..."> children`))
		}
	})
	return out
}

// translatedSelectorAttrs are attributes Odoo translates; they must not be
// used as XPath selectors in a view inherited spec.
var translatedSelectorAttrs = []string{
	"string", "help", "sum", "avg", "confirm", "placeholder",
	"alt", "title", "aria-label", "data-tooltip", "label",
}

func checkXP002(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkArch(c.tree.Root, func(n *syntax.Node, inArch bool) {
		if !inArch || n.Name != "xpath" {
			return
		}
		if v, ok := n.Attr("expr"); !ok || strings.TrimSpace(v) == "" {
			return
		}
		if _, ok := n.Attr("position"); ok {
			return
		}
		gt := startTagEnd(c.src, n)
		if gt < 0 {
			out = append(out, diag("XP002", n.Range, `<xpath> should specify an explicit "position"`))
			return
		}
		fix := &diagnostic.Fix{
			Applicability: diagnostic.ApplicabilitySafe,
			Edits:         []diagnostic.Edit{{Range: source.NewRange(gt, gt), NewText: ` position="inside"`}},
		}
		out = append(out, diagFix("XP002", n.Range, `<xpath> should specify an explicit "position"`, fix))
	})
	return out
}

func checkXP004(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkArch(c.tree.Root, func(n *syntax.Node, inArch bool) {
		if !inArch {
			return
		}
		pe, pp := attrNode(n, "expr"), attrNode(n, "position")
		if pe == nil || pp == nil || pp.Range.Start > pe.Range.Start {
			return
		}
		exprText := c.src[pe.Range.Start:pe.Range.End]
		posText := c.src[pp.Range.Start:pp.Range.End]
		between := c.src[pp.Range.End:pe.Range.Start]
		fix := &diagnostic.Fix{
			Applicability: diagnostic.ApplicabilitySafe,
			Edits: []diagnostic.Edit{{
				Range:   source.NewRange(pp.Range.Start, pe.Range.End),
				NewText: exprText + between + posText,
			}},
		}
		out = append(out, diagFix("XP004", n.Range, `"expr" should come before "position"`, fix))
	})
	return out
}

func checkXP005(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkArch(c.tree.Root, func(n *syntax.Node, inArch bool) {
		if !inArch {
			return
		}
		v, ok := n.Attr("expr")
		if !ok || !strings.Contains(v, "@class") {
			return
		}
		if a := attrNode(n, "expr"); a != nil {
			out = append(out, diag("XP005", a.Range, `avoid "@class" in the expression; use hasclass(...)`))
		}
	})
	return out
}

func checkXP006(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkArch(c.tree.Root, func(n *syntax.Node, inArch bool) {
		if !inArch {
			return
		}
		v, ok := n.Attr("expr")
		if !ok {
			return
		}
		for _, name := range translatedSelectorAttrs {
			if strings.Contains(v, "@"+name) {
				if a := attrNode(n, "expr"); a != nil {
					out = append(out, diag("XP006", a.Range,
						fmt.Sprintf("expression uses translated attribute %q as a selector", "@"+name)))
				}
				return
			}
		}
	})
	return out
}

func checkXP008(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkArch(c.tree.Root, func(n *syntax.Node, inArch bool) {
		if !inArch || n.Name != "attribute" {
			return
		}
		if a := attrNode(n, "position"); a != nil {
			out = append(out, diag("XP008", a.Range, `<attribute> should use add=/remove= rather than position=`))
		}
	})
	return out
}

func checkXP009(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkArch(c.tree.Root, func(n *syntax.Node, inArch bool) {
		if !inArch {
			return
		}
		if _, ok := n.Attr("position"); !ok {
			return
		}
		switch n.Name {
		case "xpath", "attribute":
			return
		}
		if _, ok := n.Attr("id"); ok {
			return
		}
		if _, ok := n.Attr("name"); ok {
			return
		}
		if _, ok := n.Attr("expr"); ok {
			return
		}
		out = append(out, diag("XP009", n.Range,
			fmt.Sprintf(`"position" on <%s> without an identifying attribute`, n.Name)))
	})
	return out
}

func checkXP010(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name != "record" {
			return
		}
		if _, ok := n.Attr("groups"); !ok {
			return
		}
		for _, ch := range n.ChildElements() {
			if ch.Name == "field" {
				if nm, ok := ch.Attr("name"); ok && nm == "inherit_id" {
					out = append(out, diag("XP010", n.Range, `inherited view records must not define "groups"`))
					return
				}
			}
		}
	})
	return out
}

// ---- VW family -------------------------------------------------------------

func checkVW001(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	// A <page> is legitimate when it sits under a <notebook>, or when it is
	// inserted by view inheritance (a <page>/<xpath>/any element carrying a
	// `position` attribute in its arch ancestor chain) — such pages become
	// notebook children once inheritance is applied. Only a page that reaches
	// the arch root through none of those is misplaced.
	var walk func(n *syntax.Node, inArch, hasNotebook, hasInherit bool)
	walk = func(n *syntax.Node, inArch, hasNotebook, hasInherit bool) {
		if n.Kind != syntax.KindElement {
			return
		}
		nowArch := inArch
		if !inArch && n.Name == "field" {
			if v, ok := n.Attr("name"); ok && v == "arch" {
				nowArch = true
			}
		}
		_, hasPos := n.Attr("position")
		if nowArch {
			if n.Name == "notebook" {
				hasNotebook = true
			}
			if n.Name == "xpath" || hasPos {
				hasInherit = true
			}
			if n.Name == "page" && !hasNotebook && !hasInherit && !hasPos {
				out = append(out, diag("VW001", n.Range, `<page> must be a direct child of <notebook>`))
			}
		}
		for _, ch := range n.ChildElements() {
			walk(ch, nowArch, hasNotebook, hasInherit)
		}
	}
	for _, ch := range c.tree.Root.ChildElements() {
		walk(ch, false, false, false)
	}
	return out
}

func checkVW002(c *ctx) []diagnostic.Diagnostic {
	return childrenAllowed(c, "graph", "VW002", `<graph> may only contain <field> elements`, map[string]bool{"field": true})
}

// listChildren are the element names Odoo permits inside a <list> view.
// Mirrors ir_ui_view._validate_tag_list: allowed_tags = ('field', 'button',
// 'control', 'groupby', 'widget', 'header').
var listChildren = map[string]bool{
	"field":   true,
	"button":  true,
	"control": true,
	"groupby": true,
	"widget":  true,
	"header":  true,
	"column":  true, // Odoo 20.0 <list> column element
}

func checkVW003(c *ctx) []diagnostic.Diagnostic {
	return childrenAllowed(c, "list", "VW003", `<list> may only contain <field>, <button>, <control>, <groupby>, <widget>, or <header> elements`, listChildren)
}

func childrenAllowed(c *ctx, tag, code, msg string, allowed map[string]bool) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name != tag {
			return
		}
		if _, ok := n.Attr("position"); ok {
			return // inheritance spec such as <list position="attributes">
		}
		for _, ch := range n.ChildElements() {
			if !allowed[ch.Name] {
				out = append(out, diag(code, ch.Range, msg))
			}
		}
	})
	return out
}

// hasDescendantNamed reports whether any element descendant of n is named name.
func hasDescendantNamed(n *syntax.Node, name string) bool {
	for _, ch := range n.ChildElements() {
		if ch.Name == name || hasDescendantNamed(ch, name) {
			return true
		}
	}
	return false
}

func checkVW004(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name != "search" {
			return
		}
		if _, ok := n.Attr("position"); ok {
			return
		}
		if hasDescendantNamed(n, "field") {
			return
		}
		out = append(out, diag("VW004", n.Range, `<search> should contain at least one <field>`))
	})
	return out
}

func checkVW006(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name != "img" {
			return
		}
		if _, ok := n.Attr("alt"); !ok {
			out = append(out, diag("VW006", n.Range, `<img> is missing the "alt" attribute`))
		}
	})
	return out
}

func checkVW007(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name != "notebook" {
			return
		}
		for _, ch := range n.ChildElements() {
			if ch.Name == "page" {
				return
			}
		}
		out = append(out, diag("VW007", n.Range, `<notebook> should contain at least one <page>`))
	})
	return out
}

func checkVW008(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		seen := map[string]bool{}
		for _, ch := range n.ChildElements() {
			if ch.Name != "field" {
				continue
			}
			name, ok := ch.Attr("name")
			if !ok {
				continue
			}
			if seen[name] {
				out = append(out, diag("VW008", ch.Range,
					fmt.Sprintf("duplicate <field name=%q> among siblings", name)))
				continue
			}
			seen[name] = true
		}
	})
	return out
}

func checkVW005(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name != "search" {
			return
		}
		if _, ok := n.Attr("position"); ok {
			return // inheritance spec
		}
		seen := 0
		for _, ch := range n.ChildElements() {
			if ch.Name == "searchpanel" {
				seen++
				if seen > 1 {
					out = append(out, diag("VW005", ch.Range, `<search> may contain at most one <searchpanel>`))
				}
			}
		}
	})
	return out
}

// ---- ST family -------------------------------------------------------------

// lineStartOf returns the byte offset of the start of the line containing off.
func lineStartOf(src string, off int) int {
	if off > len(src) {
		off = len(src)
	}
	i := strings.LastIndexByte(src[:off], '\n')
	return i + 1
}

func checkSEC001(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.Name != "record" {
			return
		}
		if mv, _ := n.Attr("model"); mv != "ir.rule" {
			return
		}
		hasDomain, hasPerm := false, false
		for _, ch := range n.ChildElements() {
			if ch.Name != "field" {
				continue
			}
			fn, _ := ch.Attr("name")
			if fn == "domain_force" {
				hasDomain = true
			}
			if strings.HasPrefix(fn, "perm_") {
				hasPerm = true
			}
		}
		if !hasDomain && !hasPerm {
			out = append(out, diag("SEC001", n.Range,
				`<record model="ir.rule"> should define <field name="domain_force">`))
		}
	})
	return out
}

// hasPrecedingSibling reports whether any child before n in parent is not
// all-whitespace text.
func hasPrecedingSibling(c *ctx, parent, n *syntax.Node) bool {
	for _, ch := range parent.Children {
		if ch == n {
			return false
		}
		if ch.Kind == syntax.KindWhitespace || (ch.Kind == syntax.KindText && strings.TrimSpace(c.src[ch.Range.Start:ch.Range.End]) == "") {
			continue
		}
		return true
	}
	return false
}

func checkST001(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, parent *syntax.Node) {
		if parent == nil || !isDataRootName(parent.Name) {
			return
		}
		switch n.Name {
		case "record", "menuitem", "template":
		default:
			return
		}
		// The formatter separates sibling blocks with a blank line; the first
		// child sits directly under its opener, so requiring one there would
		// make the formatter's own output fail this rule.
		if !hasPrecedingSibling(c, parent, n) {
			return
		}
		ls := lineStartOf(c.src, n.Range.Start)
		if ls == 0 {
			return
		}
		prevLS := lineStartOf(c.src, ls-1)
		prevLine := c.src[prevLS : ls-1]
		if strings.TrimSpace(prevLine) == "" {
			return
		}
		fix := &diagnostic.Fix{Applicability: diagnostic.ApplicabilitySafe,
			Edits: []diagnostic.Edit{{Range: source.NewRange(ls, ls), NewText: "\n"}}}
		out = append(out, diagFix("ST001", n.Range,
			fmt.Sprintf("expected a blank line before <%s>", n.Name), fix))
	})
	return out
}

func checkST003(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if len(n.Attrs) < 2 {
			return
		}
		names := make([]string, len(n.Attrs))
		for i, a := range n.Attrs {
			names[i] = a.AttrName
		}
		// The canonical order is shared with the formatter's
		// attribute-sort = "alphabetical", so the ST003 fix never fights
		// `ruxx format`.
		if attrorder.Sorted(n.Name, names) {
			return
		}
		out = append(out, diagFix("ST003", n.Range, "attributes are not sorted alphabetically", attrReorderFix(c, n, attrorder.Order(n.Name, names))))
	})
	return out
}

func checkST004(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.SelfClosed || len(n.ChildElements()) == 0 {
			return
		}
		endTagStart := n.Range.End - (len(n.Name) + 3)
		if endTagStart <= n.Range.Start || endTagStart > len(c.src) {
			return
		}
		gap := c.src[:endTagStart]
		if j := strings.LastIndexByte(gap, '>'); j >= 0 {
			if !strings.ContainsAny(c.src[j:endTagStart], "\n") {
				out = append(out, diag("ST004", n.Range, "closing tag should be on its own line"))
			}
		}
	})
	return out
}

func checkST005(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	lines := strings.Split(c.src, "\n")
	run := 0
	start := 0
	emit := func(end int) {
		fix := &diagnostic.Fix{Applicability: diagnostic.ApplicabilitySafe,
			Edits: []diagnostic.Edit{{Range: source.NewRange(start, end), NewText: "\n\n"}}}
		out = append(out, diagFix("ST005", source.NewRange(start, end),
			"multiple consecutive blank lines", fix))
	}
	off := 0
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			if run == 0 {
				start = off
			}
			run++
		} else {
			if run > 2 {
				emit(off)
			}
			run = 0
		}
		off += len(ln) + 1
	}
	return out
}

func checkST006(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if !n.SelfClosed {
			return
		}
		text := c.src[n.Range.Start:n.Range.End]
		if !strings.HasSuffix(text, " />") {
			return
		}
		// Skip the formatter's multi-line layout: when the whitespace before
		// "/>" follows a line break (e.g. "\n    />") the spacing is
		// intentional, not a stray space.
		i := len(text) - 3 // the space immediately before "/>"
		for i > 0 && (text[i-1] == ' ' || text[i-1] == '\t') {
			i--
		}
		if i > 0 && (text[i-1] == '\n' || text[i-1] == '\r') {
			return
		}
		at := n.Range.End - 3
		fix := &diagnostic.Fix{Applicability: diagnostic.ApplicabilitySafe,
			Edits: []diagnostic.Edit{{Range: source.NewRange(at, at+1), NewText: ""}}}
		out = append(out, diagFix("ST006", source.NewRange(at, at+1),
			`unexpected space before "/>"`, fix))
	})
	return out
}

// RUX102 flags suppression comments that name a deprecated rule code. No codes
// are deprecated yet, so this is a no-op placeholder that keeps the code
// registered and documented.
func checkRUX102(c *ctx) []diagnostic.Diagnostic { return nil }

// RUX103 warns when a style rule is enabled alongside the formatter. The
// formatter already normalises indentation, spacing, attribute order and empty
// elements, so enabling the overlapping style rules makes lint and format fight
// each other.
func checkRUX103(c *ctx) []diagnostic.Diagnostic {
	if c.set == nil {
		return nil
	}
	for _, code := range c.set.Codes() {
		r, ok := ByCode(code)
		if !ok || r.Category != Style {
			continue
		}
		d := diag("RUX103", source.NewRange(0, 0),
			fmt.Sprintf("style rule %s is enabled; the formatter handles this (run ruxx format)", code))
		d.Severity = diagnostic.SeverityWarning
		return []diagnostic.Diagnostic{d}
	}
	return nil
}

func checkST002(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	walkElements(c.tree.Root, nil, func(n, _ *syntax.Node) {
		if n.SelfClosed || len(n.ChildElements()) > 0 {
			return
		}
		if strings.TrimSpace(textOf(c.src, n)) != "" {
			return
		}
		base := n.Range.Start + 1 + len(n.Name)
		if len(n.Attrs) > 0 {
			base = n.Attrs[len(n.Attrs)-1].Range.End
		}
		if base < 0 || base >= len(c.src) {
			return
		}
		gt := strings.IndexByte(c.src[base:], '>')
		if gt < 0 {
			return
		}
		gt += base
		if gt >= n.Range.End {
			return
		}
		fix := &diagnostic.Fix{
			Applicability: diagnostic.ApplicabilitySafe,
			Edits:         []diagnostic.Edit{{Range: source.NewRange(gt, n.Range.End), NewText: "/>"}},
		}
		out = append(out, diagFix("ST002", n.Range, "empty element should be self-closing", fix))
	})
	return out
}

func textOf(src string, n *syntax.Node) string {
	var b strings.Builder
	for _, ch := range n.Children {
		if ch.Kind == syntax.KindText || ch.Kind == syntax.KindCDATA {
			b.WriteString(src[ch.Range.Start:ch.Range.End])
		}
	}
	return b.String()
}

// ---- RUX meta family -------------------------------------------------------

func checkRUX100(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	for i, d := range c.meta.dirs {
		if d.Kind != suppress.Ignore {
			continue
		}
		for _, code := range d.Codes {
			if !c.meta.suppressed[i][code] {
				out = append(out, diag("RUX100", d.Range, fmt.Sprintf("unused suppression for %s", code)))
			}
		}
	}
	return out
}

func checkRUX101(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	for _, d := range c.meta.dirs {
		for _, code := range d.Codes {
			if _, ok := ByCode(code); !ok {
				out = append(out, diag("RUX101", d.Range, fmt.Sprintf("unknown rule code %q", code)))
			}
		}
	}
	return out
}

func checkRUX104(c *ctx) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	for i, d := range c.meta.dirs {
		if d.Kind != suppress.Disable {
			continue
		}
		for _, code := range d.Codes {
			if !hasMatchingEnable(c.meta.dirs, i, code) {
				out = append(out, diag("RUX104", d.Range, fmt.Sprintf("unclosed disable region for %s", code)))
			}
		}
	}
	return out
}

func hasMatchingEnable(dirs []suppress.Directive, from int, code string) bool {
	for j := from + 1; j < len(dirs); j++ {
		if dirs[j].Kind != suppress.Enable {
			continue
		}
		if len(dirs[j].Codes) == 0 || containsStr(dirs[j].Codes, code) {
			return true
		}
	}
	return false
}

// checks is the ordered registry of rule implementations.
var checks = []check{
	{"X001", false, false, checkX001},
	{"X002", false, false, checkX002},
	{"X007", false, false, checkX007},
	{"X011", false, false, checkX011},
	{"X014", false, false, checkX014},
	{"X015", false, false, checkX015},
	{"X017", false, false, checkX017},
	{"X003", false, false, checkX003},
	{"X004", false, false, checkX004},
	{"X005", false, false, checkX005},
	{"X006", false, false, checkX006},
	{"X008", false, false, checkX008},
	{"X009", false, false, checkX009},
	{"X010", false, false, checkX010},
	{"X012", false, false, checkX012},
	{"X013", false, false, checkX013},
	{"X016", false, false, checkX016},
	{"O001", false, true, checkO001},
	{"O002", false, true, checkO002},
	{"O004", false, true, checkO004},
	{"O005", false, true, checkO005},
	{"O008", false, true, checkO008},
	{"O009", false, true, checkO009},
	{"O003", false, true, checkO003},
	{"O006", false, true, checkO006},
	{"O007", false, true, checkO007},
	{"O012", false, true, checkO012},
	{"O013", false, true, checkO013},
	{"O014", false, true, checkO014},
	{"O017", false, true, checkO017},
	{"O019", false, true, checkO019},
	{"O020", false, true, checkO020},
	{"O022", false, true, checkO022},
	{"O016", false, true, checkO016},
	{"O018", false, true, checkO018},
	{"QW012", false, true, checkQW012},
	{"SEC002", false, true, checkSEC002},
	{"SEC003", false, true, checkSEC003},
	{"O010", false, true, checkO010},
	{"O011", false, true, checkO011},
	{"O015", false, true, checkO015},
	{"O021", false, true, checkO021},
	{"QW006", false, true, checkQW006},
	{"QW007", false, true, checkQW007},
	{"QW008", false, true, checkQW008},
	{"QW001", false, true, checkQW001},
	{"QW002", false, true, checkQW002},
	{"QW003", false, true, checkQW003},
	{"QW004", false, true, checkQW004},
	{"QW005", false, true, checkQW005},
	{"QW009", false, true, checkQW009},
	{"QW010", false, true, checkQW010},
	{"QW011", false, true, checkQW011},
	{"XP001", false, true, checkXP001},
	{"XP003", false, true, checkXP003},
	{"XP007", false, true, checkXP007},
	{"XP010", false, true, checkXP010},
	{"XP002", false, true, checkXP002},
	{"XP004", false, true, checkXP004},
	{"XP005", false, true, checkXP005},
	{"XP006", false, true, checkXP006},
	{"XP008", false, true, checkXP008},
	{"XP009", false, true, checkXP009},
	{"VW001", false, true, checkVW001},
	{"VW002", false, true, checkVW002},
	{"VW003", false, true, checkVW003},
	{"VW004", false, true, checkVW004},
	{"VW005", false, true, checkVW005},
	{"VW006", false, true, checkVW006},
	{"VW007", false, true, checkVW007},
	{"VW008", false, true, checkVW008},
	{"ST002", false, false, checkST002},
	{"ST001", false, false, checkST001},
	{"ST003", false, false, checkST003},
	{"ST004", false, false, checkST004},
	{"ST005", false, false, checkST005},
	{"ST006", false, false, checkST006},
	{"SEC001", false, true, checkSEC001},
	{"RUX100", true, false, checkRUX100},
	{"RUX101", true, false, checkRUX101},
	{"RUX102", true, false, checkRUX102},
	{"RUX103", true, false, checkRUX103},
	{"RUX104", true, false, checkRUX104},
}
