package report_test

import (
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/report"
)

func diff(t *testing.T, before, after string) string {
	t.Helper()
	return report.UnifiedDiff("f.xml", before, after)
}

func TestUnifiedDiff_NoChange(t *testing.T) {
	if got := diff(t, "a\nb\n", "a\nb\n"); got != "" {
		t.Errorf("diff = %q, want empty", got)
	}
	if got := diff(t, "", ""); got != "" {
		t.Errorf("diff of two empty inputs = %q, want empty", got)
	}
}

func TestUnifiedDiff_SingleChange(t *testing.T) {
	got := diff(t, "a\nb\nc\n", "a\nB\nc\n")
	want := "--- f.xml\n+++ f.xml\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n"
	if got != want {
		t.Errorf("diff =\n%q\nwant\n%q", got, want)
	}
}

func TestUnifiedDiff_Insertion(t *testing.T) {
	got := diff(t, "a\nb\n", "a\nnew\nb\n")
	want := "--- f.xml\n+++ f.xml\n@@ -1,2 +1,3 @@\n a\n+new\n b\n"
	if got != want {
		t.Errorf("diff =\n%q\nwant\n%q", got, want)
	}
}

func TestUnifiedDiff_Deletion(t *testing.T) {
	got := diff(t, "a\nx\nb\n", "a\nb\n")
	want := "--- f.xml\n+++ f.xml\n@@ -1,3 +1,2 @@\n a\n-x\n b\n"
	if got != want {
		t.Errorf("diff =\n%q\nwant\n%q", got, want)
	}
}

func TestUnifiedDiff_EmptyBefore(t *testing.T) {
	got := diff(t, "", "x\n")
	want := "--- f.xml\n+++ f.xml\n@@ -0,0 +1,1 @@\n+x\n"
	if got != want {
		t.Errorf("diff =\n%q\nwant\n%q", got, want)
	}
}

func TestUnifiedDiff_NoTrailingNewline(t *testing.T) {
	got := diff(t, "a\nb", "a\nc")
	want := "--- f.xml\n+++ f.xml\n@@ -1,2 +1,2 @@\n a\n-b\n\\ No newline at end of file\n+c\n\\ No newline at end of file\n"
	if got != want {
		t.Errorf("diff =\n%q\nwant\n%q", got, want)
	}
}

func TestUnifiedDiff_AddedTrailingNewline(t *testing.T) {
	got := diff(t, "a\nb", "a\nb\n")
	want := "--- f.xml\n+++ f.xml\n@@ -1,2 +1,2 @@\n a\n-b\n\\ No newline at end of file\n+b\n"
	if got != want {
		t.Errorf("diff =\n%q\nwant\n%q", got, want)
	}
}

// TestUnifiedDiff_TwoHunks checks that distant changes become separate hunks
// with the unchanged middle elided.
func TestUnifiedDiff_TwoHunks(t *testing.T) {
	var before, after []string
	for i := 1; i <= 20; i++ {
		line := "L" + string(rune('A'+i-1))
		before = append(before, line)
		if i == 1 || i == 20 {
			after = append(after, "X"+string(rune('A'+i-1)))
			continue
		}
		after = append(after, line)
	}
	got := diff(t, strings.Join(before, "\n")+"\n", strings.Join(after, "\n")+"\n")
	if !strings.Contains(got, "@@ -1,4 +1,4 @@\n-LA\n+XA\n LB\n LC\n LD\n") {
		t.Errorf("missing first hunk:\n%s", got)
	}
	if !strings.Contains(got, "@@ -17,4 +17,4 @@\n LQ\n LR\n LS\n-LT\n+XT\n") {
		t.Errorf("missing second hunk:\n%s", got)
	}
	if strings.Contains(got, " LM\n") {
		t.Errorf("elided context leaked into the diff:\n%s", got)
	}
	if n := strings.Count(got, "@@ -"); n != 2 {
		t.Errorf("hunks = %d, want 2:\n%s", n, got)
	}
}

// TestUnifiedDiff_NearbyChangesMerge checks changes within 2*context lines
// collapse into one hunk.
func TestUnifiedDiff_NearbyChangesMerge(t *testing.T) {
	got := diff(t, "a\nb\nc\n", "A\nb\nC\n")
	want := "--- f.xml\n+++ f.xml\n@@ -1,3 +1,3 @@\n-a\n+A\n b\n-c\n+C\n"
	if got != want {
		t.Errorf("diff =\n%q\nwant\n%q", got, want)
	}
}

// TestUnifiedDiff_KeepsLinesShort checks a whole-file reformat stays proportional
// to the change instead of dumping both files.
func TestUnifiedDiff_KeepsLinesShort(t *testing.T) {
	var before, after []string
	for i := 0; i < 40; i++ {
		line := "<field name=\"f" + string(rune('a'+i%26)) + "\"/>"
		before = append(before, line)
		if i == 20 {
			after = append(after, "<field", "    name=\"f\"", "/>")
			continue
		}
		after = append(after, line)
	}
	got := diff(t, strings.Join(before, "\n")+"\n", strings.Join(after, "\n")+"\n")
	if n := strings.Count(got, "\n"); n > 14 {
		t.Errorf("diff has %d lines, want a small hunk:\n%s", n, got)
	}
	if !strings.Contains(got, "@@ -18,7 +18,9 @@") {
		t.Errorf("unexpected hunk header:\n%s", got)
	}
}

// TestUnifiedDiff_RepeatedLines checks the algorithm terminates and stays sane
// when no line is unique (worst case for anchoring).
func TestUnifiedDiff_RepeatedLines(t *testing.T) {
	before := strings.Repeat("x\ny\n", 200)
	after := strings.Repeat("y\nx\n", 200)
	got := diff(t, before, after)
	if !strings.HasPrefix(got, "--- f.xml\n+++ f.xml\n@@ ") {
		t.Fatalf("unexpected diff head:\n%.200s", got)
	}
	if !strings.Contains(got, "\n+x\n") || !strings.Contains(got, "\n-y\n") {
		t.Errorf("diff does not describe the change:\n%.200s", got)
	}
}
