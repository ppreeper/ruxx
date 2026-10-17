// Package source provides byte-offset ranges and line/column mapping over a
// source text. It is the coordinate system every other package speaks: rules
// report spans, the fix engine rewrites spans, and editors map spans back to
// line/column.
package source

import (
	"strings"
	"unicode/utf8"
)

// Text is a source being analyzed. It exists so that, later, non-string
// sources (e.g. a mapped/edited buffer) can flow through the same APIs.
type Text interface {
	String() string
	Len() int
}

// Str is a string-backed Text.
type Str string

// String returns the underlying text.
func (s Str) String() string { return string(s) }

// Len returns the byte length of the text.
func (s Str) Len() int { return len(s) }

// Range is a half-open byte range [Start, End) into a source text.
type Range struct {
	Start int
	End   int
}

// NewRange builds a Range.
func NewRange(start, end int) Range { return Range{Start: start, End: end} }

// Len returns the number of bytes in the range.
func (r Range) Len() int { return r.End - r.Start }

// Empty reports whether the range covers no bytes.
func (r Range) Empty() bool { return r.End <= r.Start }

// Contains reports whether byte offset off lies within [Start, End).
func (r Range) Contains(off int) bool { return off >= r.Start && off < r.End }

// Locator maps byte offsets to 1-based line/column positions. It is built once
// per source and answers queries lazily. Columns are counted in Unicode code
// points (runes), so multi-byte UTF-8 characters advance the column by one.
type Locator struct {
	src        string
	lineStarts []int
}

// NewLocator indexes the line starts of src.
func NewLocator(src string) *Locator {
	starts := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &Locator{src: src, lineStarts: starts}
}

// NumLines returns the number of lines (always >= 1).
func (l *Locator) NumLines() int { return len(l.lineStarts) }

// LineStart returns the byte offset at which 1-based line begins.
// Out-of-range lines are clamped to [1, NumLines].
func (l *Locator) LineStart(line int) int {
	if line < 1 {
		line = 1
	}
	if line > len(l.lineStarts) {
		line = len(l.lineStarts)
	}
	return l.lineStarts[line-1]
}

// LineCol maps a byte offset to a 1-based line and rune-column. Offsets past
// the end of the source clamp to the final position.
func (l *Locator) LineCol(offset int) (line, col int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(l.src) {
		offset = len(l.src)
	}
	// Binary search for the greatest lineStart <= offset.
	lo, hi := 0, len(l.lineStarts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if l.lineStarts[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	start := l.lineStarts[lo]
	col = len([]rune(l.src[start:offset])) + 1
	return lo + 1, col
}

// Offset returns the byte offset of a 1-based line and rune-column, the inverse
// of LineCol. The column clamps to the end of the line (excluding its
// terminator) and the line clamps to the source bounds.
func (l *Locator) Offset(line, col int) int {
	if line < 1 {
		line = 1
	}
	if line > len(l.lineStarts) {
		line = len(l.lineStarts)
	}
	start := l.lineStarts[line-1]
	end := len(l.src)
	if line < len(l.lineStarts) {
		end = l.lineStarts[line]
	}
	// Never return an offset inside the line terminator: the end of a line is
	// the position just before it.
	for end > start && (l.src[end-1] == '\n' || l.src[end-1] == '\r') {
		end--
	}
	if col < 1 {
		col = 1
	}
	off := start
	for c := 1; c < col && off < end; c++ {
		_, size := utf8.DecodeRuneInString(l.src[off:end])
		off += size
	}
	return off
}

// LineText returns the text of 1-based line without its trailing newline.
// Out-of-range lines clamp to [1, NumLines].
func (l *Locator) LineText(line int) string {
	if line < 1 {
		line = 1
	}
	if line > len(l.lineStarts) {
		line = len(l.lineStarts)
	}
	start := l.lineStarts[line-1]
	end := len(l.src)
	if line < len(l.lineStarts) {
		end = l.lineStarts[line]
	}
	return strings.TrimRight(l.src[start:end], "\r\n")
}
