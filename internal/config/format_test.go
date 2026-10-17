package config_test

import (
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/config"
	"github.com/ppreeper/ruxx/internal/settings"
)

// TestLoad_FormatOptions checks that every [format] key reaches Settings.
func TestLoad_FormatOptions(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, ".ruxx.toml", `
[format]
attribute-split = "always"
split-attributes-tags = ["field", "record"]
close-tag-on-new-line = true
empty-element = "expand"
odoo-tag-spacing = false
odoo-spacing-tags = ["field"]
attribute-sort = "alphabetical"
indent-style = "tab"
line-ending = "crlf"
final-newline = false
xml-declaration = "always"
encoding-style = "upper"
qweb-attributes-first = false
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s := cfg.Settings
	if s.AttributeSplit != "always" {
		t.Errorf("AttributeSplit = %q, want always", s.AttributeSplit)
	}
	if len(s.AttributeSplitTags) != 2 || s.AttributeSplitTags[0] != "field" {
		t.Errorf("AttributeSplitTags = %v, want [field record]", s.AttributeSplitTags)
	}
	if !s.CloseTagOnNewLine {
		t.Error("CloseTagOnNewLine = false, want true")
	}
	if s.EmptyElement != "expand" {
		t.Errorf("EmptyElement = %q, want expand", s.EmptyElement)
	}
	if s.OdooTagSpacing {
		t.Error("OdooTagSpacing = true, want false")
	}
	if len(s.OdooSpacingTags) != 1 || s.OdooSpacingTags[0] != "field" {
		t.Errorf("OdooSpacingTags = %v, want [field]", s.OdooSpacingTags)
	}
	if s.AttributeSort != "alphabetical" {
		t.Errorf("AttributeSort = %q, want alphabetical", s.AttributeSort)
	}
	if s.IndentStyle != "tab" {
		t.Errorf("IndentStyle = %q, want tab", s.IndentStyle)
	}
	if s.LineEnding != "crlf" {
		t.Errorf("LineEnding = %q, want crlf", s.LineEnding)
	}
	if s.FinalNewline {
		t.Error("FinalNewline = true, want false")
	}
	if s.XMLDeclaration != "always" {
		t.Errorf("XMLDeclaration = %q, want always", s.XMLDeclaration)
	}
	if s.EncodingStyle != "upper" {
		t.Errorf("EncodingStyle = %q, want upper", s.EncodingStyle)
	}
	if s.QWebAttributesFirst {
		t.Error("QWebAttributesFirst = true, want false")
	}
}

// TestLoad_FormatDefaults checks that an absent [format] section leaves the
// formatter defaults untouched.
func TestLoad_FormatDefaults(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, ".ruxx.toml", "line-length = 100\n")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s := cfg.Settings
	if s.AttributeSplit != settings.DefaultAttributeSplit {
		t.Errorf("AttributeSplit = %q, want %q", s.AttributeSplit, settings.DefaultAttributeSplit)
	}
	if s.EmptyElement != settings.DefaultEmptyElement {
		t.Errorf("EmptyElement = %q, want %q", s.EmptyElement, settings.DefaultEmptyElement)
	}
	if !s.OdooTagSpacing {
		t.Error("OdooTagSpacing = false, want true by default")
	}
	if len(s.OdooSpacingTags) != len(settings.DefaultOdooSpacingTags) {
		t.Errorf("OdooSpacingTags = %v, want %v", s.OdooSpacingTags, settings.DefaultOdooSpacingTags)
	}
	if s.CloseTagOnNewLine {
		t.Error("CloseTagOnNewLine = true, want false by default")
	}
	if !s.PreserveBlankLines {
		t.Error("PreserveBlankLines = false, want true by default")
	}
	if s.AttributeSort != settings.DefaultAttributeSort {
		t.Errorf("AttributeSort = %q, want %q", s.AttributeSort, settings.DefaultAttributeSort)
	}
	if s.IndentStyle != settings.DefaultIndentStyle {
		t.Errorf("IndentStyle = %q, want %q", s.IndentStyle, settings.DefaultIndentStyle)
	}
	if s.LineEnding != settings.DefaultLineEnding {
		t.Errorf("LineEnding = %q, want %q", s.LineEnding, settings.DefaultLineEnding)
	}
	if !s.FinalNewline {
		t.Error("FinalNewline = false, want true by default")
	}
	if s.XMLDeclaration != settings.DefaultXMLDeclaration {
		t.Errorf("XMLDeclaration = %q, want %q", s.XMLDeclaration, settings.DefaultXMLDeclaration)
	}
	if s.EncodingStyle != settings.DefaultEncodingStyle {
		t.Errorf("EncodingStyle = %q, want %q", s.EncodingStyle, settings.DefaultEncodingStyle)
	}
	if !s.QWebAttributesFirst {
		t.Error("QWebAttributesFirst = false, want true by default")
	}
}

// TestLoad_PreserveBlankLines checks the blank-line policy can be turned off.
func TestLoad_PreserveBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, ".ruxx.toml", "[format]\npreserve-blank-lines = false\n")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Settings.PreserveBlankLines {
		t.Error("PreserveBlankLines = true, want false")
	}
}

// TestLoad_FormatInvalid checks the enumerations are validated.
func TestLoad_FormatInvalid(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"attribute-split", "[format]\nattribute-split = \"often\"\n", "attribute-split must be auto, always, or never"},
		{"empty-element", "[format]\nempty-element = \"shrug\"\n", "empty-element must be self-closing or expand"},
		{"attribute-sort", "[format]\nattribute-sort = \"by-length\"\n", "attribute-sort must be none or alphabetical"},
		{"indent-style", "[format]\nindent-style = \"tabs\"\n", "indent-style must be space or tab"},
		{"line-ending", "[format]\nline-ending = \"cr\"\n", "line-ending must be auto, lf, or crlf"},
		{"xml-declaration", "[format]\nxml-declaration = \"maybe\"\n", "xml-declaration must be preserve, always, or never"},
		{"encoding-style", "[format]\nencoding-style = \"title\"\n", "encoding-style must be lower, upper, or preserve"},
		{"preserve-comments", "[format]\npreserve-comments = false\n", "preserve-comments must be true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := write(t, dir, ".ruxx.toml", tc.body)
			_, err := config.Load(path)
			if err == nil {
				t.Fatalf("Load succeeded, want error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}
