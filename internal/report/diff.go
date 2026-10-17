package report

import (
	"fmt"
	"strings"
)

// UnifiedDiff renders a `diff -u` style patch between before and after: three
// lines of context around each change, hunks grouped the way GNU diff groups
// them, and `\ No newline at end of file` when a side lacks the final newline.
// It returns "" when the inputs are identical. path labels both sides.
func UnifiedDiff(path, before, after string) string {
	return formatUnified(path, diffLines(splitDiffLines(before), splitDiffLines(after)))
}

// op is one line of the edit script: ' ' common, '-' removed, '+' added.
type op struct {
	kind byte
	text string
}

// nonlSentinel marks a line that is not terminated by a newline. Appending it
// to the text keeps a "no trailing newline" difference visible to the line
// comparison, which otherwise sees identical strings.
const nonlSentinel = "\x00nonl"

// splitDiffLines splits s into lines without their terminator, tagging the last
// line when the input does not end with a newline.
func splitDiffLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if !strings.HasSuffix(s, "\n") {
		lines[len(lines)-1] += nonlSentinel
	}
	return lines
}

// diffLines builds an edit script with a patience-style algorithm: the common
// prefix and suffix are matched directly, then the remaining region is split at
// lines that occur exactly once on both sides (their longest increasing
// subsequence). A region with no unique line is reported as a replacement.
//
// The script is not always the shortest one, but it is small on real input and
// never needs quadratic memory, which matters for whole-file formatter diffs.
func diffLines(a, b []string) []op {
	var out []op
	diffSegment(a, b, &out)
	return out
}

func diffSegment(a, b []string, out *[]op) {
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	for _, line := range a[:prefix] {
		*out = append(*out, op{' ', line})
	}
	a, b = a[prefix:], b[prefix:]

	suffix := 0
	for suffix < len(a) && suffix < len(b) && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	tail := a[len(a)-suffix:]
	a, b = a[:len(a)-suffix], b[:len(b)-suffix]

	switch {
	case len(a) == 0:
		for _, line := range b {
			*out = append(*out, op{'+', line})
		}
	case len(b) == 0:
		for _, line := range a {
			*out = append(*out, op{'-', line})
		}
	default:
		anchors := uniquePairs(a, b)
		if len(anchors) == 0 {
			for _, line := range a {
				*out = append(*out, op{'-', line})
			}
			for _, line := range b {
				*out = append(*out, op{'+', line})
			}
			break
		}
		ai, bi := 0, 0
		for _, an := range anchors {
			diffSegment(a[ai:an.a], b[bi:an.b], out)
			*out = append(*out, op{' ', a[an.a]})
			ai, bi = an.a+1, an.b+1
		}
		diffSegment(a[ai:], b[bi:], out)
	}

	for _, line := range tail {
		*out = append(*out, op{' ', line})
	}
}

// pair ties a line position in a to a line position in b.
type pair struct{ a, b int }

// uniquePairs returns the lines that occur exactly once in both inputs, in the
// longest increasing order of their b positions.
func uniquePairs(a, b []string) []pair {
	countA := make(map[string]int, len(a))
	for _, line := range a {
		countA[line]++
	}
	countB := make(map[string]int, len(b))
	for _, line := range b {
		countB[line]++
	}
	posB := make(map[string]int, len(b))
	for i, line := range b {
		if countB[line] == 1 {
			posB[line] = i
		}
	}
	var candidates []pair
	for i, line := range a {
		if countA[line] != 1 || countB[line] != 1 {
			continue
		}
		candidates = append(candidates, pair{i, posB[line]})
	}
	return longestIncreasing(candidates)
}

// longestIncreasing keeps the longest subsequence of pairs whose b positions
// strictly increase (patience sorting, O(n log n)).
func longestIncreasing(ps []pair) []pair {
	if len(ps) == 0 {
		return nil
	}
	tails := make([]int, 0, len(ps)) // indices into ps
	prev := make([]int, len(ps))
	for i := range prev {
		prev[i] = -1
	}
	for i, p := range ps {
		lo, hi := 0, len(tails)
		for lo < hi {
			mid := int(uint(lo+hi) >> 1)
			if ps[tails[mid]].b < p.b {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		if lo > 0 {
			prev[i] = tails[lo-1]
		}
		if lo == len(tails) {
			tails = append(tails, i)
		} else {
			tails[lo] = i
		}
	}
	out := make([]pair, len(tails))
	k := tails[len(tails)-1]
	for i := len(tails) - 1; i >= 0; i-- {
		out[i] = ps[k]
		k = prev[k]
	}
	return out
}

// diffContext is the number of unchanged lines shown around each change.
const diffContext = 3

// formatUnified groups the edit script into hunks and renders them.
func formatUnified(path string, ops []op) string {
	var changed []int
	for i, o := range ops {
		if o.kind != ' ' {
			changed = append(changed, i)
		}
	}
	if len(changed) == 0 {
		return ""
	}

	type group struct{ first, last int }
	var groups []group
	for _, ci := range changed {
		if n := len(groups); n > 0 && ci-groups[n-1].last-1 <= 2*diffContext {
			groups[n-1].last = ci
			continue
		}
		groups = append(groups, group{first: ci, last: ci})
	}

	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", path, path)

	beforeLine, afterLine := 1, 1
	advance := func(o op) {
		if o.kind != '+' {
			beforeLine++
		}
		if o.kind != '-' {
			afterLine++
		}
	}
	pos := 0
	for _, g := range groups {
		lo := max(0, g.first-diffContext)
		hi := min(len(ops)-1, g.last+diffContext)
		for ; pos < lo; pos++ {
			advance(ops[pos])
		}
		aStart, bStart := beforeLine, afterLine
		aCount, bCount := 0, 0
		for _, o := range ops[lo : hi+1] {
			if o.kind != '+' {
				aCount++
			}
			if o.kind != '-' {
				bCount++
			}
		}
		// GNU diff reports an empty side as the line before the hunk, or 0
		// when the hunk is at the very start.
		if aCount == 0 {
			aStart--
		}
		if bCount == 0 {
			bStart--
		}
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", aStart, aCount, bStart, bCount)
		for ; pos <= hi; pos++ {
			o := ops[pos]
			text, noNewline := strings.CutSuffix(o.text, nonlSentinel)
			b.WriteByte(o.kind)
			b.WriteString(text)
			b.WriteByte('\n')
			if noNewline {
				b.WriteString("\\ No newline at end of file\n")
			}
			advance(o)
		}
	}
	return b.String()
}
