package report_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/diagnostic"
	"github.com/ppreeper/ruxx/internal/report"
	"github.com/ppreeper/ruxx/internal/source"
)

const src = "<?xml version=\"1.0\"?>\n<odoo>\n    <record id=\"foo\">\n    </record>\n</odoo>\n"

// recordStart is the byte offset of '<' in `<record id="foo">` on line 3.
const recordStart = 33

func diag() diagnostic.Diagnostic {
	d := diagnostic.Diagnostic{
		Code:     "O004",
		Name:     "record-missing-model",
		Severity: diagnostic.SeverityError,
		Range:    source.NewRange(recordStart, recordStart+7),
		Message:  `<record> is missing the required "model" attribute`,
		Help:     "add a model attribute",
	}
	return d
}

func results(withFix bool) []report.Result {
	d := diag()
	if withFix {
		d.Fix = &diagnostic.Fix{
			Message:       "add model",
			Applicability: diagnostic.ApplicabilitySafe,
			Edits:         []diagnostic.Edit{{Range: source.NewRange(recordStart+8, recordStart+8), NewText: `model="x" `}},
		}
	}
	return []report.Result{{Path: "views/foo.xml", Source: src, Diagnostics: []diagnostic.Diagnostic{d}}}
}

func render(t *testing.T, format string, withFix bool) string {
	t.Helper()
	var buf bytes.Buffer
	if err := report.Render(&buf, results(withFix), format); err != nil {
		t.Fatalf("Render(%q): %v", format, err)
	}
	return buf.String()
}

func TestRender_Concise(t *testing.T) {
	got := render(t, "concise", false)
	want := "views/foo.xml:3:5: O004 <record> is missing the required \"model\" attribute\n"
	if got != want {
		t.Fatalf("concise = %q, want %q", got, want)
	}
}

func TestRender_DefaultIsConcise(t *testing.T) {
	if render(t, "", false) != render(t, "concise", false) {
		t.Fatal("empty format should behave like concise")
	}
}

func TestRender_Full(t *testing.T) {
	got := render(t, "full", true)
	for _, want := range []string{"views/foo.xml:3:5: O004", "3 |     <record", "^", "= help:", "= fix available (safe)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("full output missing %q:\n%s", want, got)
		}
	}
}

func TestRender_Grouped(t *testing.T) {
	got := render(t, "grouped", false)
	want := "views/foo.xml:\n  3:5  O004 <record> is missing the required \"model\" attribute\n"
	if got != want {
		t.Fatalf("grouped = %q, want %q", got, want)
	}
}

func TestRender_JSON(t *testing.T) {
	var got []map[string]any
	if err := json.Unmarshal([]byte(render(t, "json", true)), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0]["code"] != "O004" || got[0]["path"] != "views/foo.xml" || got[0]["severity"] != "error" {
		t.Fatalf("unexpected entry: %v", got[0])
	}
	rng := got[0]["range"].(map[string]any)["start"].(map[string]any)
	if rng["line"].(float64) != 3 || rng["column"].(float64) != 5 {
		t.Fatalf("unexpected range: %v", rng)
	}
	if _, ok := got[0]["fix"].(map[string]any); !ok {
		t.Fatalf("expected a fix object: %v", got[0])
	}
}

func TestRender_JSONEmptyIsArray(t *testing.T) {
	var buf bytes.Buffer
	if err := report.Render(&buf, nil, "json"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "[]") {
		t.Fatalf("empty json = %q, want []", buf.String())
	}
}

func TestRender_JSONLines(t *testing.T) {
	got := render(t, "json-lines", false)
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(lines))
	}
	var one map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &one); err != nil {
		t.Fatalf("json-lines: %v", err)
	}
	if one["code"] != "O004" {
		t.Fatalf("code = %v", one["code"])
	}
}

func TestRender_GitHub(t *testing.T) {
	got := render(t, "github", false)
	if !strings.HasPrefix(got, "::error file=views/foo.xml,line=3,col=5,endLine=3,endColumn=12::O004 ") {
		t.Fatalf("github = %q", got)
	}
}

func TestRender_Azure(t *testing.T) {
	got := render(t, "azure", false)
	if !strings.HasPrefix(got, "##vso[task.logissue type=error;sourcepath=views/foo.xml;linenumber=3;columnnumber=5;]O004 ") {
		t.Fatalf("azure = %q", got)
	}
}

func TestRender_GitLab(t *testing.T) {
	var got []map[string]any
	if err := json.Unmarshal([]byte(render(t, "gitlab", false)), &got); err != nil {
		t.Fatalf("gitlab: %v", err)
	}
	if len(got) != 1 || got[0]["severity"] != "major" || got[0]["check_name"] != "O004" {
		t.Fatalf("unexpected issue: %v", got)
	}
	if fp, _ := got[0]["fingerprint"].(string); fp == "" {
		t.Fatal("fingerprint should not be empty")
	}
}

func TestRender_JUnit(t *testing.T) {
	got := render(t, "junit", false)
	for _, want := range []string{"<testsuites", `name="views/foo.xml"`, "<failure", "O004"} {
		if !strings.Contains(got, want) {
			t.Fatalf("junit missing %q:\n%s", want, got)
		}
	}
}

func TestRender_SARIF(t *testing.T) {
	var got map[string]any
	if err := json.Unmarshal([]byte(render(t, "sarif", false)), &got); err != nil {
		t.Fatalf("sarif: %v", err)
	}
	if got["version"] != "2.1.0" {
		t.Fatalf("version = %v", got["version"])
	}
	runs := got["runs"].([]any)
	run := runs[0].(map[string]any)
	driver := run["tool"].(map[string]any)["driver"].(map[string]any)
	if driver["name"] != "ruxx" {
		t.Fatalf("driver name = %v", driver["name"])
	}
	res := run["results"].([]any)[0].(map[string]any)
	if res["ruleId"] != "O004" || res["level"] != "error" {
		t.Fatalf("unexpected result: %v", res)
	}
}

func TestRender_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := report.Render(&buf, nil, "nope"); err == nil {
		t.Fatal("expected an error for an unknown format")
	}
}
