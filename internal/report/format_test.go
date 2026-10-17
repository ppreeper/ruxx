package report_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/report"
)

func fmtFiles() []report.FormatFile {
	return []report.FormatFile{{Path: "views/a.xml", Before: "<a></a>", After: "<a/>"}}
}

func renderFormat(t *testing.T, format string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := report.RenderFormatCheck(&buf, fmtFiles(), format); err != nil {
		t.Fatalf("RenderFormatCheck(%q): %v", format, err)
	}
	return buf.String()
}

func TestRenderFormatCheck_Concise(t *testing.T) {
	if got := renderFormat(t, "concise"); got != "would reformat views/a.xml\n" {
		t.Fatalf("concise = %q", got)
	}
}

func TestRenderFormatCheck_DefaultIsConcise(t *testing.T) {
	if renderFormat(t, "") != renderFormat(t, "concise") {
		t.Fatal("empty format should behave like concise")
	}
}

func TestRenderFormatCheck_JSON(t *testing.T) {
	var got []map[string]any
	if err := json.Unmarshal([]byte(renderFormat(t, "json")), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(got) != 1 || got[0]["wouldReformat"] != true || got[0]["path"] != "views/a.xml" {
		t.Fatalf("unexpected json: %v", got)
	}
}

func TestRenderFormatCheck_JSONEmptyIsArray(t *testing.T) {
	var buf bytes.Buffer
	if err := report.RenderFormatCheck(&buf, nil, "json"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "[]") {
		t.Fatalf("empty json = %q, want []", buf.String())
	}
}

func TestRenderFormatCheck_JSONLines(t *testing.T) {
	got := strings.TrimSpace(renderFormat(t, "json-lines"))
	if strings.Count(got, "\n") != 0 || !strings.Contains(got, `"wouldReformat":true`) {
		t.Fatalf("json-lines = %q", got)
	}
}

func TestRenderFormatCheck_GitHub(t *testing.T) {
	if got := renderFormat(t, "github"); !strings.HasPrefix(got, "::warning file=views/a.xml,line=1,col=1::would reformat") {
		t.Fatalf("github = %q", got)
	}
}

func TestRenderFormatCheck_Azure(t *testing.T) {
	if got := renderFormat(t, "azure"); !strings.HasPrefix(got, "##vso[task.logissue type=warning;sourcepath=views/a.xml;linenumber=1;]would reformat") {
		t.Fatalf("azure = %q", got)
	}
}

func TestRenderFormatCheck_GitLab(t *testing.T) {
	var got []map[string]any
	if err := json.Unmarshal([]byte(renderFormat(t, "gitlab")), &got); err != nil {
		t.Fatalf("gitlab: %v", err)
	}
	if len(got) != 1 || got[0]["check_name"] != "format" || got[0]["severity"] != "minor" {
		t.Fatalf("unexpected issue: %v", got)
	}
}

func TestRenderFormatCheck_JUnit(t *testing.T) {
	got := renderFormat(t, "junit")
	for _, want := range []string{"<testsuites", "<failure", `name="format"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("junit missing %q:\n%s", want, got)
		}
	}
}

func TestRenderFormatCheck_SARIF(t *testing.T) {
	var got map[string]any
	if err := json.Unmarshal([]byte(renderFormat(t, "sarif")), &got); err != nil {
		t.Fatalf("sarif: %v", err)
	}
	if got["version"] != "2.1.0" {
		t.Fatalf("version = %v", got["version"])
	}
	run := got["runs"].([]any)[0].(map[string]any)
	res := run["results"].([]any)[0].(map[string]any)
	if res["level"] != "warning" || res["ruleId"] != "format" {
		t.Fatalf("unexpected result: %v", res)
	}
}

func TestRenderFormatCheck_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := report.RenderFormatCheck(&buf, fmtFiles(), "nope"); err == nil {
		t.Fatal("expected an error for an unknown format")
	}
}
