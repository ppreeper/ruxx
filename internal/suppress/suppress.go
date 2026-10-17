// Package suppress parses and applies ruxx suppression directives, which are
// written as XML comments:
//
//	<!-- ruxx: ignore[O004, O005] -->   suppress the listed rules file-wide
//	<!-- ruxx: ignore -->               suppress all rules file-wide
//	<!-- ruxx: disable[O004] -->        begin a suppressed region
//	<!-- ruxx: enable[O004] -->         end a suppressed region
//	<!-- fmt: off --> / <!-- fmt: on -->   control the formatter
//	<!-- fmt: skip -->                  skip the next element
//
// Parsing works on raw text by scanning comment spans, so it needs no tree.
package suppress

import (
	"sort"
	"strings"

	"github.com/ppreeper/ruxx/internal/source"
)

// Kind identifies a directive.
type Kind uint8

const (
	// Ignore suppresses the listed rules (or all rules) file-wide.
	Ignore Kind = iota
	// Disable begins a suppressed region.
	Disable
	// Enable ends a suppressed region.
	Enable
	// FmtOff disables formatting until FmtOn.
	FmtOff
	// FmtOn re-enables formatting.
	FmtOn
	// FmtSkip skips formatting the next element.
	FmtSkip
)

// String returns the lowercase name of the directive kind.
func (k Kind) String() string {
	switch k {
	case Ignore:
		return "ignore"
	case Disable:
		return "disable"
	case Enable:
		return "enable"
	case FmtOff:
		return "fmt-off"
	case FmtOn:
		return "fmt-on"
	case FmtSkip:
		return "fmt-skip"
	default:
		return "unknown"
	}
}

// Directive is one parsed suppression comment.
type Directive struct {
	Kind  Kind
	Range source.Range `json:"-"`
	// Codes lists the affected rule codes; empty means all rules.
	Codes []string `json:",omitempty"`
}

// Parse returns every suppression directive in src, in source order.
func Parse(src string) []Directive {
	var out []Directive
	i := 0
	for {
		rel := strings.Index(src[i:], "<!--")
		if rel < 0 {
			break
		}
		start := i + rel
		endRel := strings.Index(src[start+4:], "-->")
		if endRel < 0 {
			break
		}
		inner := src[start+4 : start+4+endRel]
		if d, ok := parseDirective(strings.TrimSpace(inner), start); ok {
			out = append(out, d)
		}
		i = start + 4 + endRel + 3
	}
	return out
}

func parseDirective(text string, start int) (Directive, bool) {
	lower := strings.ToLower(text)
	switch {
	case strings.HasPrefix(lower, "ruxx:"), strings.HasPrefix(lower, "noqa:"):
		rest := strings.TrimSpace(text[strings.Index(text, ":")+1:])
		return parseRuxx(rest, start)
	case strings.HasPrefix(lower, "fmt:"):
		rest := strings.ToLower(strings.TrimSpace(text[strings.Index(text, ":")+1:]))
		switch rest {
		case "off":
			return Directive{Kind: FmtOff, Range: source.NewRange(start, start)}, true
		case "on":
			return Directive{Kind: FmtOn, Range: source.NewRange(start, start)}, true
		case "skip":
			return Directive{Kind: FmtSkip, Range: source.NewRange(start, start)}, true
		}
	}
	return Directive{}, false
}

func parseRuxx(rest string, start int) (Directive, bool) {
	verbs := []struct {
		name string
		kind Kind
	}{
		{"ignore", Ignore},
		{"disable", Disable},
		{"enable", Enable},
	}
	for _, v := range verbs {
		if rest != v.name && !strings.HasPrefix(rest, v.name+"[") {
			continue
		}
		d := Directive{Kind: v.kind, Range: source.NewRange(start, start)}
		if strings.HasPrefix(rest, v.name+"[") {
			open := strings.Index(rest, "[")
			close := strings.Index(rest, "]")
			if close < 0 {
				return Directive{}, false
			}
			for _, c := range strings.Split(rest[open+1:close], ",") {
				c = strings.TrimSpace(c)
				if c != "" {
					d.Codes = append(d.Codes, c)
				}
			}
		}
		return d, true
	}
	return Directive{}, false
}

// Suppresses reports whether the diagnostic for code at byte offset off is
// suppressed by src's directives.
func Suppresses(src string, offset int, code string) bool {
	dirs := Parse(src)

	// Ignore directives are file-wide.
	for _, d := range dirs {
		if d.Kind == Ignore && (len(d.Codes) == 0 || contains(d.Codes, code)) {
			return true
		}
	}

	// Disable/enable regions only cover offsets after the directive.
	activeAll := false
	active := map[string]bool{}
	for _, d := range dirs {
		if d.Range.Start >= offset {
			continue
		}
		switch d.Kind {
		case Disable:
			if len(d.Codes) == 0 {
				activeAll = true
			} else {
				for _, c := range d.Codes {
					active[c] = true
				}
			}
		case Enable:
			if len(d.Codes) == 0 {
				activeAll = false
				active = map[string]bool{}
			} else {
				for _, c := range d.Codes {
					delete(active, c)
				}
			}
		}
	}
	return activeAll || active[code]
}

func contains(codes []string, code string) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

// AddIgnore inserts (or extends) a file-wide ignore directive in src for the
// given rule codes and returns the new source. Duplicate codes are dropped and
// codes are kept sorted. When src already carries a blanket
// `<!-- ruxx: ignore -->`, src is returned unchanged because everything is
// already suppressed.
//
// The directive is placed just after the XML declaration when one is present,
// so the file keeps starting with the declaration, and at the very top
// otherwise.
func AddIgnore(src string, codes []string) string {
	codes = uniqueCodes(codes)
	if len(codes) == 0 {
		return src
	}
	for _, d := range Parse(src) {
		if d.Kind != Ignore {
			continue
		}
		if len(d.Codes) == 0 {
			return src
		}
		rel := strings.Index(src[d.Range.Start:], "-->")
		if rel < 0 {
			continue
		}
		end := d.Range.Start + rel + len("-->")
		merged := uniqueCodes(append(append([]string{}, d.Codes...), codes...))
		comment := "<!-- ruxx: ignore[" + strings.Join(merged, ", ") + "] -->"
		return src[:d.Range.Start] + comment + src[end:]
	}
	comment := "<!-- ruxx: ignore[" + strings.Join(codes, ", ") + "] -->\n"
	if off := declarationEnd(src); off > 0 {
		return src[:off] + comment + src[off:]
	}
	return comment + src
}

// declarationEnd returns the offset just past the XML declaration, or 0 when
// src does not begin with one.
func declarationEnd(src string) int {
	if !strings.HasPrefix(src, "<?xml") {
		return 0
	}
	rel := strings.Index(src, "?>")
	if rel < 0 {
		return 0
	}
	end := rel + len("?>")
	switch {
	case strings.HasPrefix(src[end:], "\r\n"):
		return end + 2
	case strings.HasPrefix(src[end:], "\n"):
		return end + 1
	default:
		return end
	}
}

// uniqueCodes trims, drops empties and duplicates, and sorts codes.
func uniqueCodes(codes []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, c := range codes {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
