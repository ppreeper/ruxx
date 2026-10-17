package syntax_test

import (
	"testing"

	"github.com/ppreeper/ruxx/internal/syntax"
)

// FuzzParseRoundTrip asserts the central invariant for arbitrary input:
// Unparse(Parse(src)) == src, even for malformed XML.
func FuzzParseRoundTrip(f *testing.F) {
	seeds := []string{
		"",
		"   ",
		"<a/>",
		"<a></a>",
		"<a b=\"c\">text</a>",
		"<?xml version=\"1.0\"?><odoo/>",
		"<!-- x --><a/>",
		"<a><![CDATA[x]]></a>",
		"<?xml version=\"1.0\"?><!DOCTYPE odoo><odoo/>",
		"<a>",
		"<a><b></a>",
		"<a b=\"c",
		"<a b=>",
		"pointer<",
		"</a>",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		got := syntax.Unparse(syntax.Parse(src))
		if got != src {
			t.Fatalf("round-trip mismatch\n src: %q\n got: %q", src, got)
		}
	})
}
