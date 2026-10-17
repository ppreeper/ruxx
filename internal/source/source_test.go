package source_test

import (
	"testing"
	"unicode/utf8"

	"github.com/ppreeper/ruxx/internal/source"
)

func TestLocator_LineCol(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		offset int
		line   int
		col    int
	}{
		{"start", "abc\ndef", 0, 1, 1},
		{"mid line 1", "abc\ndef", 2, 1, 3},
		{"eol before nl", "abc\ndef", 3, 1, 4},
		{"line 2 start", "abc\ndef", 4, 2, 1},
		{"line 2 mid", "abc\ndef", 6, 2, 3},
		{"eof", "abc\ndef", 7, 2, 4},
		{"past eof clamps", "abc\ndef", 99, 2, 4},
		{"negative clamps", "abc\ndef", -5, 1, 1},
		{"empty source", "", 0, 1, 1},
		{"trailing newline", "a\n", 2, 2, 1},
		{"blank line", "a\n\nb", 2, 2, 1},
		{"runes count as one column", "héllo\nx", 7, 2, 1},
		{"rune mid", "héllo", 3, 1, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := source.NewLocator(tt.src)
			line, col := loc.LineCol(tt.offset)
			if line != tt.line || col != tt.col {
				t.Fatalf("LineCol(%d) = (%d,%d), want (%d,%d)",
					tt.offset, line, col, tt.line, tt.col)
			}
		})
	}
}

func TestLocator_NumLines(t *testing.T) {
	tests := []struct {
		src  string
		want int
	}{
		{"", 1},
		{"a", 1},
		{"a\n", 2},
		{"a\nb", 2},
		{"a\nb\n", 3},
		{"\n\n\n", 4},
	}
	for _, tt := range tests {
		if got := source.NewLocator(tt.src).NumLines(); got != tt.want {
			t.Errorf("NumLines(%q) = %d, want %d", tt.src, got, tt.want)
		}
	}
}

func TestLocator_LineText(t *testing.T) {
	loc := source.NewLocator("first\r\nsecond\nthird")
	tests := []struct {
		line int
		want string
	}{
		{1, "first"},
		{2, "second"},
		{3, "third"},
		{4, "third"}, // clamps to last line
		{0, "first"}, // clamps to first line
	}
	for _, tt := range tests {
		if got := loc.LineText(tt.line); got != tt.want {
			t.Errorf("LineText(%d) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

func TestRange(t *testing.T) {
	r := source.NewRange(3, 7)
	if r.Len() != 4 {
		t.Fatalf("Len = %d, want 4", r.Len())
	}
	if r.Empty() {
		t.Fatal("expected non-empty")
	}
	if !r.Contains(3) || !r.Contains(6) {
		t.Fatal("expected Contains(3) and Contains(6)")
	}
	if r.Contains(7) {
		t.Fatal("End is exclusive: Contains(7) must be false")
	}
	if !source.NewRange(5, 5).Empty() {
		t.Fatal("zero-width range should be empty")
	}
}

func TestStr(t *testing.T) {
	var s source.Text = source.Str("héllo")
	if s.Len() != 6 {
		t.Fatalf("Len = %d, want 6 bytes", s.Len())
	}
	if s.String() != "héllo" {
		t.Fatalf("String = %q", s.String())
	}
}

func TestLocator_Offset(t *testing.T) {
	// "abc\ndef\néx\n": offsets 0 a, 1 b, 2 c, 3 \n, 4 d, 5 e, 6 f, 7 \n,
	// 8-9 é, 10 x, 11 \n; line 4 is the empty line after the final newline.
	src := "abc\ndef\néx\n"
	tests := []struct {
		name string
		line int
		col  int
		want int
	}{
		{"line 1 start", 1, 1, 0},
		{"line 1 end", 1, 4, 3},
		{"line 2 mid", 2, 2, 5},
		{"col past line clamps", 2, 99, 7},
		{"rune counts as one column", 3, 2, 10},
		{"line 3 end", 3, 3, 11},
		{"last line", 4, 1, 12},
		{"col past eof clamps", 4, 9, 12},
		{"line past eof clamps", 99, 1, 12},
		{"line below range clamps", 0, 1, 0},
		{"col below range clamps", 2, -3, 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			loc := source.NewLocator(src)
			if got := loc.Offset(tc.line, tc.col); got != tc.want {
				t.Errorf("Offset(%d, %d) = %d, want %d", tc.line, tc.col, got, tc.want)
			}
		})
	}
}

// TestLocator_OffsetRoundTrip checks Offset is the inverse of LineCol.
func TestLocator_OffsetRoundTrip(t *testing.T) {
	src := "abc\ndef\néx\ny\n"
	loc := source.NewLocator(src)
	for off := 0; off <= len(src); off++ {
		// An offset inside a multi-byte rune has no column of its own, so it
		// cannot round-trip; every rune boundary must.
		if off < len(src) && !utf8.RuneStart(src[off]) {
			continue
		}
		line, col := loc.LineCol(off)
		if got := loc.Offset(line, col); got != off {
			t.Errorf("Offset(LineCol(%d)) = %d (line %d col %d), want %d", off, got, line, col, off)
		}
	}
}
