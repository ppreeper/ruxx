package syntax_test

import (
	"testing"

	"github.com/ppreeper/ruxx/internal/syntax"
)

// roundtripInputs are inputs whose byte-for-byte re-serialization must equal
// the original. They deliberately include messy layout and edge cases.
var roundtripInputs = []string{
	"",
	"   ",
	"<a/>",
	"<a></a>",
	"<a b=\"c\"/>",
	"<a b='c' d=\"e\">text</a>",
	"<a>\n  <b>hi</b>\n</a>",
	"<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<odoo>\n  <record id=\"x\" model=\"m\"/>\n</odoo>\n",
	"<!-- a comment --><a/>",
	"<a><!-- inner --></a>",
	"<a><![CDATA[ raw < not markup ]]></a>",
	"<?xml version=\"1.0\"?><!DOCTYPE odoo SYSTEM \"odoo.dtd\"><odoo/>",
	"<a\n  b=\"1\"\n  c=\"2\"\n/>",
	"<a>&amp;&lt;&gt;&#65;</a>",
	"<a b=\"multi\nline\"/>",
	"<odoo><data noupdate=\"1\"><record id=\"a\" model=\"ir.ui.view\"><field name=\"arch\" type=\"xml\"><form><sheet/></form></field></record></data></odoo>",
	// malformed inputs must still round-trip losslessly
	"<a>",
	"</a>",
	"<a b=>",
	"<a b=\"c",
	"<a><b></a>",
	"plain text",
	"<a/", // truncated tag
	"<a></b></a>",
}

func TestParse_RoundTrip(t *testing.T) {
	for i, src := range roundtripInputs {
		got := syntax.Unparse(syntax.Parse(src))
		if got != src {
			t.Errorf("case %d: round-trip mismatch\n src: %q\n got: %q", i, src, got)
		}
	}
}

func TestParse_SimpleElement(t *testing.T) {
	tree := syntax.Parse("<field name=\"arch\" type=\"xml\"/>")
	if tree.Root == nil {
		t.Fatal("Root is nil")
	}
	if !tree.HasValidSyntax() {
		t.Fatalf("expected valid syntax, got errors: %v", tree.Errors)
	}
	root := tree.Root
	els := root.ChildElements()
	if len(els) == 0 {
		t.Fatal("no root element found")
	}
	el := els[0]
	if el.Name != "field" {
		t.Fatalf("element name = %q, want %q", el.Name, "field")
	}
	if !el.SelfClosed {
		t.Fatal("expected SelfClosed")
	}
	if v, ok := el.Attr("name"); !ok || v != "arch" {
		t.Fatalf("Attr(name) = (%q,%v), want (arch,true)", v, ok)
	}
	if v, ok := el.Attr("type"); !ok || v != "xml" {
		t.Fatalf("Attr(type) = (%q,%v), want (xml,true)", v, ok)
	}
	if _, ok := el.Attr("missing"); ok {
		t.Fatal("Attr(missing) should be absent")
	}
}

func TestParse_NestedElements(t *testing.T) {
	tree := syntax.Parse("<odoo><record id=\"a\"><field name=\"x\"/></record></odoo>")
	roots := tree.Root.ChildElements()
	if len(roots) != 1 || roots[0].Name != "odoo" {
		t.Fatalf("root elements = %v", roots)
	}
	recs := roots[0].ChildElements()
	if len(recs) != 1 || recs[0].Name != "record" {
		t.Fatalf("record elements = %v", recs)
	}
	fields := recs[0].ChildElements()
	if len(fields) != 1 || fields[0].Name != "field" {
		t.Fatalf("field elements = %v", fields)
	}
}

func TestParse_Errors(t *testing.T) {
	bad := []string{
		"<a>",        // unclosed element
		"<a><b></a>", // mismatched closing tag
		"pointer<",   // stray '<'
		"<a b=>",     // attribute without value
		"<a b=\"c",   // unterminated attribute value
	}
	for _, src := range bad {
		tree := syntax.Parse(src)
		if tree.HasValidSyntax() {
			t.Errorf("Parse(%q): expected syntax errors, got none", src)
		}
	}
}

func TestParse_Valid(t *testing.T) {
	good := []string{
		"<a/>",
		"<a></a>",
		"<a b=\"c\">text</a>",
		"<?xml version=\"1.0\"?><odoo/>",
		"<a><!-- c --></a>",
		"<a><![CDATA[x]]></a>",
	}
	for _, src := range good {
		tree := syntax.Parse(src)
		if !tree.HasValidSyntax() {
			t.Errorf("Parse(%q): unexpected errors: %v", src, tree.Errors)
		}
	}
}

func TestKind_IsTrivia(t *testing.T) {
	if !syntax.KindWhitespace.IsTrivia() || !syntax.KindComment.IsTrivia() {
		t.Fatal("whitespace and comment should be trivia")
	}
	if syntax.KindElement.IsTrivia() || syntax.KindText.IsTrivia() {
		t.Fatal("element and text are not trivia")
	}
}
