// Package syntax parses XML into a lossless tree.
//
// "Lossless" means the tree preserves every byte of the input, including
// whitespace, comments, CDATA, processing instructions, and the doctype. The
// invariant that every consumer relies on is:
//
//	Unparse(Parse(src)) == src
//
// The linter reads the tree for structure; the formatter reprints it from the
// same tree; the fix engine rewrites byte ranges within it. Parsing never
// fails hard: well-formedness problems are recorded and parsing continues so
// that the linter can still report useful diagnostics.
package syntax

import (
	"github.com/ppreeper/ruxx/internal/source"
)

// Kind enumerates node kinds in the lossless tree.
type Kind uint8

const (
	// KindDocument is the root node; its children are the prolog, the root
	// element, trailing comments, and trailing whitespace.
	KindDocument Kind = iota
	// KindElement is an XML element, including its start tag, children, and
	// end tag (when not self-closed).
	KindElement
	// KindAttribute is a single attribute inside a start tag.
	KindAttribute
	// KindText is character data between markup.
	KindText
	// KindComment is a <!-- ... --> comment.
	KindComment
	// KindCDATA is a <![CDATA[ ... ]]> section.
	KindCDATA
	// KindProcessingInstruction is <?target ... ?> (including the XML
	// declaration).
	KindProcessingInstruction
	// KindDoctype is a <!DOCTYPE ...> declaration.
	KindDoctype
	// KindWhitespace is insignificant whitespace that the formatter may
	// re-lay-out.
	KindWhitespace
	// KindSyntaxError marks a span the parser could not interpret.
	KindSyntaxError
)

// String returns a stable name for the kind.
func (k Kind) String() string {
	switch k {
	case KindDocument:
		return "document"
	case KindElement:
		return "element"
	case KindAttribute:
		return "attribute"
	case KindText:
		return "text"
	case KindComment:
		return "comment"
	case KindCDATA:
		return "cdata"
	case KindProcessingInstruction:
		return "pi"
	case KindDoctype:
		return "doctype"
	case KindWhitespace:
		return "whitespace"
	case KindSyntaxError:
		return "syntax-error"
	default:
		return "unknown"
	}
}

// IsTrivia reports whether the kind is layout-only (whitespace or comments).
func (k Kind) IsTrivia() bool {
	return k == KindWhitespace || k == KindComment
}

// Node is a lossless syntax tree node. Range is a half-open byte span into the
// original source. For KindText, KindComment, KindCDATA, and KindWhitespace the
// node's Range covers the raw source bytes verbatim.
type Node struct {
	Kind     Kind
	Range    source.Range
	Children []*Node

	// Element fields (Kind == KindElement).
	Name       string
	Attrs      []*Node // all KindAttribute nodes, in source order
	SelfClosed bool

	// Attribute fields (Kind == KindAttribute).
	AttrName  string
	AttrValue string
	// AttrQuote is the quote character used in the source ('"' or '\'').
	// It must be preserved: a value such as {"no_open": True} is only legal
	// when single-quoted, so re-quoting with '"' would corrupt the document.
	AttrQuote byte
}

// Attr returns the value of the named attribute and whether it was present.
func (n *Node) Attr(name string) (string, bool) {
	for _, a := range n.Attrs {
		if a.AttrName == name {
			return a.AttrValue, true
		}
	}
	return "", false
}

// ChildElements returns the element children of n, skipping trivia.
func (n *Node) ChildElements() []*Node {
	var out []*Node
	for _, c := range n.Children {
		if c.Kind == KindElement {
			out = append(out, c)
		}
	}
	return out
}

// SyntaxError describes a well-formedness problem found while parsing.
type SyntaxError struct {
	Range   source.Range
	Message string
}

// Error implements error.
func (e SyntaxError) Error() string { return e.Message }

// Tree is a parsed document. Errors holds well-formedness problems in source
// order; a document with errors still has a (best-effort) Root.
type Tree struct {
	Src    string
	Root   *Node
	Errors []SyntaxError
}

// HasValidSyntax reports whether parsing found no well-formedness errors.
func (t *Tree) HasValidSyntax() bool { return len(t.Errors) == 0 }

// Parse and Unparse live in parser.go: Parse builds the lossless tree and
// Unparse reconstructs the source by filling the gaps between child ranges.
