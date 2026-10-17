// Package formatter renders XML into a canonical, Odoo-friendly layout.
//
// Two guarantees hold for every output:
//
//   - Idempotency: Format(Format(src)) == Format(src).
//   - Lint-safety: formatting reorganizes layout only; it never removes
//     content or introduces a well-formedness error.
//
// The formatter works from the lossless syntax tree produced by
// internal/syntax: whitespace and layout are recomputed here, while
// elements, attributes, text, comments, and the prolog are preserved.
package formatter

import (
	"regexp"
	"strings"

	"github.com/ppreeper/ruxx/internal/attrorder"
	"github.com/ppreeper/ruxx/internal/settings"
	"github.com/ppreeper/ruxx/internal/source"
	"github.com/ppreeper/ruxx/internal/syntax"
)

// Format renders src in canonical layout using s for indentation and the
// line-length budget. It returns the formatted source and a nil error; the
// error return exists for API symmetry with future formatting modes.
func Format(src string, s *settings.Settings) (string, error) {
	f := newFormatter(src, s)
	body := f.document(syntax.Parse(src))
	if body == "" {
		return src, nil
	}
	return f.finish(body, src), nil
}

// FormatRange formats only the smallest element that encloses rng, leaving every
// byte outside that element untouched. It is the engine behind `ruxx format
// --range` and editor range formatting; the selection's own indentation is kept,
// so only the layout inside it is recomputed.
func FormatRange(src string, s *settings.Settings, rng source.Range) (string, error) {
	f := newFormatter(src, s)
	n, depth := enclosingElement(syntax.Parse(src), rng)
	if n == nil {
		return src, nil
	}
	rendered := f.element(n, depth)
	// The renderer indents the first line; the caller keeps whatever
	// whitespace already precedes the selection.
	rendered = strings.TrimPrefix(rendered, f.indent(depth))
	// Keep the file's own line ending so a range edit cannot introduce mixed
	// endings.
	if eol := f.eol(); eol != "\n" {
		rendered = strings.ReplaceAll(toLF(rendered), "\n", eol)
	}
	return src[:n.Range.Start] + rendered + src[n.Range.End:], nil
}

// newFormatter builds a formatter, filling the zero value of the enum fields
// with the built-in default so callers that do not go through
// settings.Default() still get a sane layout.
func newFormatter(src string, s *settings.Settings) *formatter {
	if s == nil {
		s = settings.Default()
	}
	f := &formatter{
		src:               src,
		indentWidth:       s.IndentWidth,
		lineLength:        s.LineLength,
		attrSplit:         s.AttributeSplit,
		attrSplitTags:     s.AttributeSplitTags,
		closeTagOnNewLine: s.CloseTagOnNewLine,
		emptyElement:      s.EmptyElement,
		odooTagSpacing:    s.OdooTagSpacing,
		spacingTags:       s.OdooSpacingTags,
		preserveBlank:     s.PreserveBlankLines,
		attrSort:          s.AttributeSort,
		indentStyle:       s.IndentStyle,
		lineEnding:        s.LineEnding,
		finalNewline:      s.FinalNewline,
		xmlDecl:           s.XMLDeclaration,
		encodingStyle:     s.EncodingStyle,
		qwebFirst:         s.QWebAttributesFirst,
	}
	if f.attrSplit == "" {
		f.attrSplit = settings.DefaultAttributeSplit
	}
	if f.emptyElement == "" {
		f.emptyElement = settings.DefaultEmptyElement
	}
	if f.spacingTags == nil {
		f.spacingTags = settings.DefaultOdooSpacingTags
	}
	if f.attrSort == "" {
		f.attrSort = settings.DefaultAttributeSort
	}
	if f.indentStyle == "" {
		f.indentStyle = settings.DefaultIndentStyle
	}
	if f.lineEnding == "" {
		f.lineEnding = settings.DefaultLineEnding
	}
	if f.xmlDecl == "" {
		f.xmlDecl = settings.DefaultXMLDeclaration
	}
	if f.encodingStyle == "" {
		f.encodingStyle = settings.DefaultEncodingStyle
	}
	return f
}

// finish applies the file-level output options: line endings and the trailing
// newline. It works in LF space and converts at the end, so text that was
// carried over verbatim (comments, CDATA, mixed content) is normalised too.
func (f *formatter) finish(body, src string) string {
	body = strings.TrimRight(toLF(body), "\n")
	if f.finalNewline || endsWithNewline(src) {
		body += "\n"
	}
	if f.eol() == "\r\n" {
		body = strings.ReplaceAll(body, "\n", "\r\n")
	}
	return body
}

// eol returns the line ending to emit: the configured one, or the file's own
// dominant ending under "auto".
func (f *formatter) eol() string {
	switch f.lineEnding {
	case "crlf":
		return "\r\n"
	case "lf":
		return "\n"
	}
	return dominantEOL(f.src)
}

// toLF normalises CRLF and lone CR to LF.
func toLF(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// dominantEOL reports whether src uses CRLF more often than LF.
func dominantEOL(src string) string {
	crlf := strings.Count(src, "\r\n")
	if crlf > strings.Count(src, "\n")-crlf {
		return "\r\n"
	}
	return "\n"
}

// endsWithNewline reports whether src ends with a line break.
func endsWithNewline(src string) bool {
	return strings.HasSuffix(src, "\n") || strings.HasSuffix(src, "\r")
}

type formatter struct {
	src         string
	indentWidth int
	lineLength  int

	attrSplit         string
	attrSplitTags     []string
	closeTagOnNewLine bool
	emptyElement      string
	odooTagSpacing    bool
	spacingTags       []string
	preserveBlank     bool
	attrSort          string

	indentStyle   string
	lineEnding    string
	finalNewline  bool
	xmlDecl       string
	encodingStyle string
	qwebFirst     bool
}

// indent returns the leading whitespace for the given depth.
func (f *formatter) indent(depth int) string {
	if f.indentStyle == "tab" {
		return strings.Repeat("\t", depth)
	}
	return strings.Repeat(" ", depth*f.indentWidth)
}

// document formats the top level: the prolog (and any doctype/comments) are
// emitted verbatim, each on its own line, followed by the formatted root.
func (f *formatter) document(t *syntax.Tree) string {
	if t == nil || t.Root == nil {
		return f.src
	}
	var b strings.Builder
	emitted := false
	blank := false
	sawDecl := false
	for _, c := range t.Root.Children {
		switch c.Kind {
		case syntax.KindWhitespace, syntax.KindText:
			if f.preserveBlank && f.hasBlankLine(c) {
				blank = true
			}
		case syntax.KindProcessingInstruction:
			text := f.slice(c)
			if isXMLDecl(text) {
				if f.xmlDecl == "never" {
					continue
				}
				sawDecl = true
				text = restyleEncoding(text, f.encodingStyle)
			}
			if emitted && blank {
				b.WriteByte('\n')
			}
			blank = false
			b.WriteString(text)
			b.WriteByte('\n')
			emitted = true
		case syntax.KindDoctype, syntax.KindComment:
			if emitted && blank {
				b.WriteByte('\n')
			}
			blank = false
			b.WriteString(f.slice(c))
			b.WriteByte('\n')
			emitted = true
		case syntax.KindElement:
			if emitted && blank {
				b.WriteByte('\n')
			}
			blank = false
			b.WriteString(f.element(c, 0))
			b.WriteByte('\n')
			emitted = true
		}
	}
	out := b.String()
	if out == "" {
		return f.src
	}
	if f.xmlDecl == "always" && !sawDecl {
		out = declaration(f.encodingStyle) + "\n" + out
	}
	return out
}

// slice returns the raw source bytes of a node.
func (f *formatter) slice(n *syntax.Node) string {
	start, end := n.Range.Start, n.Range.End
	if start < 0 {
		start = 0
	}
	if end > len(f.src) {
		end = len(f.src)
	}
	if start > end {
		return ""
	}
	return f.src[start:end]
}

// hasBlankLine reports whether a whitespace node spans a blank line (two or
// more newlines).
func (f *formatter) hasBlankLine(n *syntax.Node) bool {
	return strings.Count(f.slice(n), "\n") >= 2
}

// element renders a single element at the given depth. The returned string
// begins with the element's indentation.
func (f *formatter) element(n *syntax.Node, depth int) string {
	if len(n.ChildElements()) == 0 {
		return f.leaf(n, depth)
	}
	if strings.TrimSpace(f.text(n)) != "" {
		// Mixed content: character data interleaved with elements (HTML-ish
		// fields). Re-emit the element verbatim so no text is lost or moved;
		// only its first line is re-indented.
		return f.indent(depth) + f.slice(n)
	}
	var b strings.Builder
	b.WriteString(f.indent(depth))
	b.WriteString(f.startTag(n, depth, false))
	b.WriteByte('\n')
	emitted := false
	blank := false
	for _, c := range n.Children {
		switch c.Kind {
		case syntax.KindWhitespace, syntax.KindText:
			if f.preserveBlank && f.hasBlankLine(c) {
				blank = true
			}
		case syntax.KindElement:
			if emitted && (blank || f.spacingTag(c.Name)) {
				b.WriteByte('\n')
			}
			blank = false
			b.WriteString(f.element(c, depth+1))
			b.WriteByte('\n')
			emitted = true
		case syntax.KindComment, syntax.KindCDATA:
			if emitted && blank {
				b.WriteByte('\n')
			}
			blank = false
			b.WriteString(f.indent(depth + 1))
			b.WriteString(strings.TrimSpace(f.slice(c)))
			b.WriteByte('\n')
			emitted = true
		}
	}
	b.WriteString(f.indent(depth))
	b.WriteString(closeTag(n))
	return b.String()
}

// leaf renders an element with no element children: an empty one self-closes, a
// short one stays inline, and text that would overflow the line budget is
// wrapped onto its own line.
func (f *formatter) leaf(n *syntax.Node, depth int) string {
	text := f.inner(n)
	if strings.TrimSpace(text) == "" {
		if f.emptyElement == "expand" {
			st := f.startTag(n, depth, false)
			if strings.Contains(st, "\n") {
				return f.indent(depth) + st + "\n" + f.indent(depth) + closeTag(n)
			}
			return f.indent(depth) + st + closeTag(n)
		}
		return f.indent(depth) + f.startTag(n, depth, true)
	}
	if strings.Contains(text, "<![CDATA[") {
		// CDATA blocks carry their own layout: keep them verbatim.
		return f.indent(depth) + f.slice(n)
	}
	st := f.startTag(n, depth, false)
	// A split start tag ends with a newline, so the text and the closing tag
	// each take their own line. Text with significant surrounding whitespace is
	// never moved: that would change the value.
	if strings.Contains(st, "\n") {
		if strings.TrimSpace(text) == text {
			return f.indent(depth) + st + "\n" + f.indent(depth+1) + text + "\n" + f.indent(depth) + closeTag(n)
		}
		return f.indent(depth) + st + text + closeTag(n)
	}
	inline := f.indent(depth) + st + text + closeTag(n)
	if strings.TrimSpace(text) == text && (f.closeTagOnNewLine || len(inline) > f.lineLength) {
		return f.indent(depth) + st + "\n" +
			f.indent(depth+1) + text + "\n" +
			f.indent(depth) + closeTag(n)
	}
	return inline
}

// text concatenates the character-data children of n.
func (f *formatter) text(n *syntax.Node) string {
	var b strings.Builder
	for _, c := range n.Children {
		if c.Kind == syntax.KindText {
			b.WriteString(f.slice(c))
		}
	}
	return b.String()
}

// inner concatenates the text and CDATA children of n.
func (f *formatter) inner(n *syntax.Node) string {
	var b strings.Builder
	for _, c := range n.Children {
		if c.Kind == syntax.KindText || c.Kind == syntax.KindCDATA {
			b.WriteString(f.slice(c))
		}
	}
	return b.String()
}

// closeTag returns the end tag for n.
func closeTag(n *syntax.Node) string { return "</" + n.Name + ">" }

// orderAttrs returns n's attributes in output order. Under attribute-sort =
// "alphabetical" the canonical order shared with the ST003 rule is used (QWeb
// directives lead). Otherwise QWeb directives lead and the rest keep source
// order, unless qweb-attributes-first is off, in which case source order is
// kept as-is.
func (f *formatter) orderAttrs(n *syntax.Node) []*syntax.Node {
	if f.attrSort == "alphabetical" {
		names := make([]string, len(n.Attrs))
		for i, a := range n.Attrs {
			names[i] = a.AttrName
		}
		out := make([]*syntax.Node, 0, len(n.Attrs))
		for _, i := range attrorder.Order(n.Name, names) {
			out = append(out, n.Attrs[i])
		}
		return out
	}
	if !f.qwebFirst {
		return n.Attrs
	}
	return orderAttrs(n.Attrs)
}

// startTagEnd returns the index of the '>' that closes n's start tag, or -1.
func startTagEnd(src string, n *syntax.Node) int {
	i := n.Range.Start
	if i < 0 || i >= len(src) || src[i] != '<' {
		return -1
	}
	var quote byte
	for ; i < len(src); i++ {
		c := src[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '>':
			return i
		}
	}
	return -1
}

// startTag renders "<name attr=..." -> ">". When the inline form would exceed
// the line-length budget, attributes are split one per line (indented one
// level) and the closing ">" or "/>" sits at the element's own indent. The
// result has no leading indentation for the first line.
func (f *formatter) startTag(n *syntax.Node, depth int, selfClose bool) string {
	attrs := f.orderAttrs(n)

	var inline strings.Builder
	inline.WriteByte('<')
	inline.WriteString(n.Name)
	for _, a := range attrs {
		inline.WriteByte(' ')
		writeAttr(&inline, a)
	}
	if selfClose {
		inline.WriteString("/>")
	} else {
		inline.WriteByte('>')
	}
	if !f.splitAttrs(n.Name, depth, inline.Len(), len(attrs)) {
		return inline.String()
	}

	var b strings.Builder
	b.WriteByte('<')
	b.WriteString(n.Name)
	for _, a := range attrs {
		b.WriteByte('\n')
		b.WriteString(f.indent(depth + 1))
		writeAttr(&b, a)
	}
	b.WriteByte('\n')
	b.WriteString(f.indent(depth))
	if selfClose {
		b.WriteString("/>")
	} else {
		b.WriteByte('>')
	}
	return b.String()
}

// writeAttr writes `name="value"`, reusing the quoting style found in the
// source. Rewriting a single-quoted value with double quotes would corrupt
// attributes like options='{"no_open": True}', whose text contains a '"'.
func writeAttr(b *strings.Builder, a *syntax.Node) {
	q := a.AttrQuote
	if q != '\'' {
		q = '"'
	}
	b.WriteString(a.AttrName)
	b.WriteByte('=')
	b.WriteByte(q)
	b.WriteString(a.AttrValue)
	b.WriteByte(q)
}

// splitAttrs decides whether an element's attributes are laid out one per
// line. A tag listed in AttributeSplitTags always splits, regardless of the
// mode; "always"/"never" then decide for every other tag; "auto" splits only
// when the inline tag would exceed the line-length budget at this depth.
func (f *formatter) splitAttrs(name string, depth, inlineLen, nattrs int) bool {
	if nattrs == 0 {
		return false
	}
	for _, t := range f.attrSplitTags {
		if t == name {
			return true
		}
	}
	switch f.attrSplit {
	case "always":
		return true
	case "never":
		return false
	}
	return depth*f.indentWidth+inlineLen > f.lineLength
}

// orderAttrs puts QWeb (t-*) attributes before plain ones, preserving relative
// order within each group.
func orderAttrs(attrs []*syntax.Node) []*syntax.Node {
	ordered := make([]*syntax.Node, 0, len(attrs))
	for _, a := range attrs {
		if strings.HasPrefix(a.AttrName, "t-") {
			ordered = append(ordered, a)
		}
	}
	for _, a := range attrs {
		if !strings.HasPrefix(a.AttrName, "t-") {
			ordered = append(ordered, a)
		}
	}
	return ordered
}

// spacingTag reports whether a blank line precedes this element when it
// follows a sibling (Odoo tag spacing).
func (f *formatter) spacingTag(name string) bool {
	if !f.odooTagSpacing {
		return false
	}
	for _, t := range f.spacingTags {
		if t == name {
			return true
		}
	}
	return false
}

// enclosingElement returns the deepest element that contains rng, with its
// depth, or nil when no element encloses it.
func enclosingElement(t *syntax.Tree, rng source.Range) (*syntax.Node, int) {
	if t == nil || t.Root == nil {
		return nil, -1
	}
	var chain []*syntax.Node
	var walk func(n *syntax.Node)
	walk = func(n *syntax.Node) {
		for _, c := range n.Children {
			if c.Kind != syntax.KindElement {
				continue
			}
			if c.Range.Start <= rng.Start && rng.Start <= c.Range.End {
				chain = append(chain, c)
				walk(c)
				return
			}
		}
	}
	walk(t.Root)
	for i := len(chain) - 1; i >= 0; i-- {
		if chain[i].Range.End >= rng.End {
			return chain[i], i
		}
	}
	return nil, -1
}

// isXMLDecl reports whether a processing instruction is the XML declaration.
func isXMLDecl(text string) bool {
	return strings.HasPrefix(strings.ToLower(text), "<?xml")
}

// declaration returns a fresh XML declaration. There is no source case to
// preserve, so "lower" and "preserve" both produce the canonical spelling.
func declaration(style string) string {
	enc := "utf-8"
	if style == "upper" {
		enc = "UTF-8"
	}
	return `<?xml version="1.0" encoding="` + enc + `"?>`
}

// encodingRE matches the encoding value in an XML declaration, keeping the
// author's quoting style.
var encodingRE = regexp.MustCompile(`(?i)(encoding\s*=\s*)(["'])([^"']*)(["'])`)

// restyleEncoding upper- or lower-cases the encoding value of an existing
// declaration. "preserve" (and any unknown value) leaves it alone.
func restyleEncoding(text, style string) string {
	if style != "lower" && style != "upper" {
		return text
	}
	return encodingRE.ReplaceAllStringFunc(text, func(m string) string {
		loc := encodingRE.FindStringSubmatchIndex(m)
		val := m[loc[6]:loc[7]]
		if style == "lower" {
			val = strings.ToLower(val)
		} else {
			val = strings.ToUpper(val)
		}
		return m[:loc[6]] + val + m[loc[7]:]
	})
}
