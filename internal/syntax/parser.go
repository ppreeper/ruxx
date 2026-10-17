package syntax

import (
	"strings"

	"github.com/ppreeper/ruxx/internal/source"
)

// parser is a single-pass, error-tolerant XML scanner. It never stops early:
// malformed spans are recorded in p.errors and turned into KindSyntaxError
// nodes so the tree still covers the entire input.
type parser struct {
	src    string
	pos    int
	errors []SyntaxError
}

// Parse parses src into a lossless tree. It never returns nil and never panics
// on malformed input; problems are recorded in the returned tree's Errors.
func Parse(src string) *Tree {
	p := &parser{src: src}
	root := p.parseDocument()
	return &Tree{Src: src, Root: root, Errors: p.errors}
}

// Unparse reconstructs the original source text from the tree. It is the exact
// inverse of Parse for every input:
//
//	Unparse(Parse(src)) == src
//
// It walks the tree and emits any source bytes not covered by a child, which
// reproduces start tags, attribute text, and end tags verbatim.
func Unparse(t *Tree) string {
	if t == nil {
		return ""
	}
	if t.Root == nil {
		return t.Src
	}
	var b strings.Builder
	b.Grow(len(t.Src))
	unparseNode(&b, t.Src, t.Root)
	return b.String()
}

func unparseNode(b *strings.Builder, src string, n *Node) {
	limit := len(src)
	start := clamp(n.Range.Start, 0, limit)
	pos := start
	for _, c := range n.Children {
		cs := clamp(c.Range.Start, pos, limit)
		ce := clamp(c.Range.End, cs, limit)
		if cs > pos {
			b.WriteString(src[pos:cs])
		}
		unparseNode(b, src, c)
		pos = ce
	}
	end := clamp(n.Range.End, pos, limit)
	if end > pos {
		b.WriteString(src[pos:end])
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (p *parser) errorAt(start, end int, msg string) {
	p.errors = append(p.errors, SyntaxError{Range: source.NewRange(start, end), Message: msg})
}

func (p *parser) eof() bool { return p.pos >= len(p.src) }

func (p *parser) hasPrefix(s string) bool {
	return strings.HasPrefix(p.src[p.pos:], s)
}

func (p *parser) peekByte() byte {
	if p.eof() {
		return 0
	}
	return p.src[p.pos]
}

func (p *parser) skipWS() {
	for !p.eof() && isXMLSpace(p.src[p.pos]) {
		p.pos++
	}
}

func isXMLSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isNameStart(c byte) bool {
	return c == '_' || c == ':' ||
		(c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		c >= 0x80
}

func isNameChar(c byte) bool {
	return isNameStart(c) || c == '-' || c == '.' || (c >= '0' && c <= '9')
}

func (p *parser) scanName() string {
	start := p.pos
	if p.eof() || !isNameStart(p.src[p.pos]) {
		return ""
	}
	p.pos++
	for !p.eof() && isNameChar(p.src[p.pos]) {
		p.pos++
	}
	return p.src[start:p.pos]
}

// parseDocument fills the document node with prolog, root element, and epilog.
func (p *parser) parseDocument() *Node {
	doc := &Node{Kind: KindDocument, Range: source.NewRange(0, len(p.src))}
	for !p.eof() {
		if p.hasPrefix("<") {
			doc.Children = append(doc.Children, p.parseMarkup())
		} else {
			// Top-level text/whitespace (e.g. the newline after the prolog or
			// trailing whitespace) is character data, not an element.
			doc.Children = append(doc.Children, p.parseText())
		}
	}
	return doc
}

// parseMarkup parses one markup construct at p.pos (which must point at '<').
func (p *parser) parseMarkup() *Node {
	switch {
	case p.hasPrefix("<!--"):
		return p.parseComment()
	case p.hasPrefix("<![CDATA["):
		return p.parseCDATA()
	case p.hasPrefix("<?"):
		return p.parsePI()
	case p.hasPrefix("<!"):
		return p.parseDoctype()
	case p.hasPrefix("</"):
		return p.parseStrayCloseTag()
	default:
		return p.parseElement()
	}
}

func (p *parser) parseText() *Node {
	start := p.pos
	for !p.eof() && p.src[p.pos] != '<' {
		p.pos++
	}
	kind := KindText
	if isAllSpace(p.src[start:p.pos]) {
		kind = KindWhitespace
	}
	return &Node{Kind: kind, Range: source.NewRange(start, p.pos)}
}

func isAllSpace(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isXMLSpace(s[i]) {
			return false
		}
	}
	return true
}

func (p *parser) parseComment() *Node {
	start := p.pos
	idx := strings.Index(p.src[start+4:], "-->")
	if idx < 0 {
		p.pos = len(p.src)
		p.errorAt(start, p.pos, "unterminated comment")
		return &Node{Kind: KindComment, Range: source.NewRange(start, p.pos)}
	}
	end := start + 4 + idx + 3
	p.pos = end
	return &Node{Kind: KindComment, Range: source.NewRange(start, end)}
}

func (p *parser) parseCDATA() *Node {
	start := p.pos
	idx := strings.Index(p.src[start+9:], "]]>")
	if idx < 0 {
		p.pos = len(p.src)
		p.errorAt(start, p.pos, "unterminated CDATA section")
		return &Node{Kind: KindCDATA, Range: source.NewRange(start, p.pos)}
	}
	end := start + 9 + idx + 3
	p.pos = end
	return &Node{Kind: KindCDATA, Range: source.NewRange(start, end)}
}

func (p *parser) parsePI() *Node {
	start := p.pos
	idx := strings.Index(p.src[start+2:], "?>")
	if idx < 0 {
		p.pos = len(p.src)
		p.errorAt(start, p.pos, "unterminated processing instruction")
		return &Node{Kind: KindProcessingInstruction, Range: source.NewRange(start, p.pos)}
	}
	end := start + 2 + idx + 2
	p.pos = end
	return &Node{Kind: KindProcessingInstruction, Range: source.NewRange(start, end)}
}

func (p *parser) parseDoctype() *Node {
	start := p.pos
	p.pos += 2 // consume "<!"
	depth := 0
	for !p.eof() {
		c := p.src[p.pos]
		switch {
		case c == '[':
			depth++
		case c == ']' && depth > 0:
			depth--
		case c == '>' && depth == 0:
			p.pos++
			return &Node{Kind: KindDoctype, Range: source.NewRange(start, p.pos)}
		}
		p.pos++
	}
	p.errorAt(start, p.pos, "unterminated doctype")
	return &Node{Kind: KindDoctype, Range: source.NewRange(start, p.pos)}
}

func (p *parser) parseStrayCloseTag() *Node {
	start := p.pos
	p.pos += 2
	name := p.scanName()
	p.skipWS()
	if p.hasPrefix(">") {
		p.pos++
	}
	if name == "" {
		p.errorAt(start, p.pos, "malformed closing tag")
	} else {
		p.errorAt(start, p.pos, "unexpected closing tag </"+name+">")
	}
	return &Node{Kind: KindSyntaxError, Range: source.NewRange(start, p.pos)}
}

func (p *parser) parseElement() *Node {
	start := p.pos
	p.pos++ // consume '<'
	name := p.scanName()
	if name == "" {
		// Not a valid start tag (e.g. a lone '<').
		if !p.eof() {
			p.pos++
		}
		p.errorAt(start, p.pos, "expected element name")
		return &Node{Kind: KindSyntaxError, Range: source.NewRange(start, p.pos)}
	}

	el := &Node{Kind: KindElement, Name: name}
	for {
		p.skipWS()
		if p.eof() {
			p.errorAt(start, len(p.src), "unclosed element <"+name+">")
			el.Range = source.NewRange(start, len(p.src))
			return el
		}
		if p.hasPrefix("/>") {
			p.pos += 2
			el.SelfClosed = true
			el.Range = source.NewRange(start, p.pos)
			return el
		}
		if p.hasPrefix(">") {
			p.pos++
			break
		}
		el.Attrs = append(el.Attrs, p.parseAttribute())
	}

	// Parse children until the matching end tag.
	for {
		if p.eof() {
			p.errorAt(start, len(p.src), "unclosed element <"+name+">")
			el.Range = source.NewRange(start, len(p.src))
			return el
		}
		if p.hasPrefix("</") {
			closeStart := p.pos
			p.pos += 2
			cname := p.scanName()
			p.skipWS()
			if p.hasPrefix(">") {
				p.pos++
			} else {
				p.errorAt(closeStart, p.pos, "malformed closing tag")
			}
			if cname == name {
				el.Range = source.NewRange(start, p.pos)
				return el
			}
			p.errorAt(closeStart, p.pos, "mismatched closing tag </"+cname+">; expected </"+name+">")
			el.Range = source.NewRange(start, p.pos)
			return el
		}
		if p.hasPrefix("<") {
			el.Children = append(el.Children, p.parseMarkup())
		} else {
			el.Children = append(el.Children, p.parseText())
		}
	}
}

func (p *parser) parseAttribute() *Node {
	start := p.pos
	name := p.scanName()
	if name == "" {
		// Cannot make progress on a stray byte; consume it to avoid looping.
		p.pos++
		p.errorAt(start, p.pos, "malformed start tag")
		return &Node{Kind: KindSyntaxError, Range: source.NewRange(start, p.pos)}
	}

	attr := &Node{Kind: KindAttribute, AttrName: name}
	p.skipWS()
	if !p.hasPrefix("=") {
		p.errorAt(start, p.pos, `attribute "`+name+`" is missing a value`)
		attr.Range = source.NewRange(start, p.pos)
		return attr
	}
	p.pos++ // consume '='
	p.skipWS()

	q := p.peekByte()
	if q != '"' && q != '\'' {
		p.errorAt(start, p.pos, `attribute "`+name+`" has no value`)
		attr.Range = source.NewRange(start, p.pos)
		return attr
	}
	attr.AttrQuote = q
	p.pos++ // consume opening quote
	valStart := p.pos
	for !p.eof() && p.src[p.pos] != q {
		p.pos++
	}
	if p.eof() {
		attr.AttrValue = p.src[valStart:p.pos]
		attr.Range = source.NewRange(start, p.pos)
		p.errorAt(start, p.pos, "unterminated attribute value")
		return attr
	}
	attr.AttrValue = p.src[valStart:p.pos]
	p.pos++ // consume closing quote
	attr.Range = source.NewRange(start, p.pos)
	return attr
}
