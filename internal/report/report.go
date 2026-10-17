// Package report renders lint results in the output formats supported by the
// `--output-format` flag: concise, full, json, json-lines, github, gitlab,
// junit, sarif, azure, and grouped.
//
// Every format is a pure function of the collected per-file results, so the
// CLI can lint first and render once at the end (which the aggregating formats
// such as json/sarif/junit require).
package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ppreeper/ruxx/internal/diagnostic"
	"github.com/ppreeper/ruxx/internal/source"
	"github.com/ppreeper/ruxx/internal/version"
)

// Version is reported by the machine-readable formats that carry tool
// metadata (sarif, gitlab). The value is stamped into internal/version at
// build time.
var Version = version.Version

// Result is the lint outcome for a single file.
type Result struct {
	Path        string
	Source      string
	Diagnostics []diagnostic.Diagnostic
}

// Formats lists every supported --output-format value.
var Formats = []string{
	"concise", "full", "json", "json-lines", "github", "gitlab",
	"junit", "sarif", "azure", "grouped",
}

// Options controls rendering.
type Options struct {
	// Format is one of Formats; an empty value means "concise".
	Format string
	// Color adds ANSI colour to the human-readable formats (concise, full,
	// grouped). Machine-readable formats are never coloured.
	Color bool
}

// Render writes results in the requested format (see Options.Format). An empty
// format means "concise" and an unknown format is an error. This is RenderWith
// with colour disabled.
func Render(w io.Writer, results []Result, format string) error {
	return RenderWith(w, results, Options{Format: format})
}

// RenderWith writes results according to opts.
func RenderWith(w io.Writer, results []Result, opts Options) error {
	format := opts.Format
	if format == "" {
		format = "concise"
	}
	switch format {
	case "concise":
		return renderConcise(w, results, false, opts.Color)
	case "full":
		return renderConcise(w, results, true, opts.Color)
	case "grouped":
		return renderGrouped(w, results, opts.Color)
	case "json":
		return renderJSON(w, results)
	case "json-lines":
		return renderJSONLines(w, results)
	case "github":
		return renderGitHub(w, results)
	case "azure":
		return renderAzure(w, results)
	case "gitlab":
		return renderGitLab(w, results)
	case "junit":
		return renderJUnit(w, results)
	case "sarif":
		return renderSARIF(w, results)
	default:
		return fmt.Errorf("unknown output format %q", format)
	}
}

// --- shared helpers ---------------------------------------------------------

// pos is a 1-based position range for one diagnostic.
type pos struct {
	line, col       int
	endLine, endCol int
}

func positions(r Result, d diagnostic.Diagnostic) pos {
	loc := source.NewLocator(r.Source)
	sl, sc := loc.LineCol(d.Range.Start)
	el, ec := loc.LineCol(d.Range.End)
	return pos{line: sl, col: sc, endLine: el, endCol: ec}
}

func severityString(s diagnostic.Severity) string {
	switch s {
	case diagnostic.SeverityError:
		return "error"
	case diagnostic.SeverityWarning:
		return "warning"
	case diagnostic.SeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}

const (
	ansiReset  = "\x1b[0m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
	ansiDim    = "\x1b[2m"
)

// coloredCode returns the rule code wrapped in ANSI colour when color is set.
func coloredCode(d diagnostic.Diagnostic, color bool) string {
	if !color {
		return d.Code
	}
	c := ansiDim
	switch d.Severity {
	case diagnostic.SeverityError:
		c = ansiRed
	case diagnostic.SeverityWarning:
		c = ansiYellow
	}
	return c + d.Code + ansiReset
}

// --- concise / full ---------------------------------------------------------

func renderConcise(w io.Writer, results []Result, full, color bool) error {
	for _, r := range results {
		for _, d := range r.Diagnostics {
			p := positions(r, d)
			if _, err := fmt.Fprintf(w, "%s:%d:%d: %s %s\n", r.Path, p.line, p.col, coloredCode(d, color), d.Message); err != nil {
				return err
			}
			if !full {
				continue
			}
			if snippet := snippetFor(r.Source, p); snippet != "" {
				if _, err := io.WriteString(w, snippet); err != nil {
					return err
				}
			}
			if d.Help != "" {
				if _, err := fmt.Fprintf(w, "  = help: %s\n", d.Help); err != nil {
					return err
				}
			}
			if app, ok := d.Fixable(); ok {
				if _, err := fmt.Fprintf(w, "  = fix available (%s)\n", app); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func snippetFor(src string, p pos) string {
	line := source.NewLocator(src).LineText(p.line)
	if line == "" {
		return ""
	}
	caret := strings.Repeat(" ", p.col-1) + "^"
	if p.endLine == p.line && p.endCol > p.col {
		caret = strings.Repeat(" ", p.col-1) + strings.Repeat("^", p.endCol-p.col)
	}
	return fmt.Sprintf("%4d | %s\n     | %s\n", p.line, line, caret)
}

// --- grouped ----------------------------------------------------------------

func renderGrouped(w io.Writer, results []Result, color bool) error {
	for _, r := range results {
		if len(r.Diagnostics) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s:\n", r.Path); err != nil {
			return err
		}
		for _, d := range r.Diagnostics {
			p := positions(r, d)
			if _, err := fmt.Fprintf(w, "  %d:%d  %s %s\n", p.line, p.col, coloredCode(d, color), d.Message); err != nil {
				return err
			}
		}
	}
	return nil
}

// --- json / json-lines ------------------------------------------------------

type jsonLoc struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type jsonRange struct {
	Start jsonLoc `json:"start"`
	End   jsonLoc `json:"end"`
}

type jsonEdit struct {
	Range   jsonRange `json:"range"`
	NewText string    `json:"newText"`
}

type jsonFix struct {
	Applicability string     `json:"applicability"`
	Message       string     `json:"message,omitempty"`
	Edits         []jsonEdit `json:"edits"`
}

type jsonDiag struct {
	Path     string    `json:"path"`
	Code     string    `json:"code"`
	Name     string    `json:"name"`
	Severity string    `json:"severity"`
	Message  string    `json:"message"`
	Help     string    `json:"help,omitempty"`
	Range    jsonRange `json:"range"`
	Fix      *jsonFix  `json:"fix,omitempty"`
}

func toJSONDiag(r Result, d diagnostic.Diagnostic) jsonDiag {
	p := positions(r, d)
	jd := jsonDiag{
		Path:     r.Path,
		Code:     d.Code,
		Name:     d.Name,
		Severity: severityString(d.Severity),
		Message:  d.Message,
		Help:     d.Help,
		Range: jsonRange{
			Start: jsonLoc{p.line, p.col},
			End:   jsonLoc{p.endLine, p.endCol},
		},
	}
	if d.Fix != nil {
		f := &jsonFix{Applicability: d.Fix.Applicability.String(), Message: d.Fix.Message}
		for _, e := range d.Fix.Edits {
			loc := source.NewLocator(r.Source)
			sl, sc := loc.LineCol(e.Range.Start)
			el, ec := loc.LineCol(e.Range.End)
			f.Edits = append(f.Edits, jsonEdit{
				Range:   jsonRange{Start: jsonLoc{sl, sc}, End: jsonLoc{el, ec}},
				NewText: e.NewText,
			})
		}
		jd.Fix = f
	}
	return jd
}

func collectJSON(results []Result) []jsonDiag {
	out := []jsonDiag{}
	for _, r := range results {
		for _, d := range r.Diagnostics {
			out = append(out, toJSONDiag(r, d))
		}
	}
	return out
}

func renderJSON(w io.Writer, results []Result) error {
	data, err := json.MarshalIndent(collectJSON(results), "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func renderJSONLines(w io.Writer, results []Result) error {
	for _, jd := range collectJSON(results) {
		data, err := json.Marshal(jd)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "%s\n", data); err != nil {
			return err
		}
	}
	return nil
}

// --- github / azure ---------------------------------------------------------

func ghLevel(s diagnostic.Severity) string {
	switch s {
	case diagnostic.SeverityError:
		return "error"
	case diagnostic.SeverityWarning:
		return "warning"
	default:
		return "notice"
	}
}

func renderGitHub(w io.Writer, results []Result) error {
	for _, r := range results {
		for _, d := range r.Diagnostics {
			p := positions(r, d)
			if _, err := fmt.Fprintf(w,
				"::%s file=%s,line=%d,col=%d,endLine=%d,endColumn=%d::%s %s\n",
				ghLevel(d.Severity), r.Path, p.line, p.col, p.endLine, p.endCol,
				d.Code, d.Message); err != nil {
				return err
			}
		}
	}
	return nil
}

func azType(s diagnostic.Severity) string {
	if s == diagnostic.SeverityError {
		return "error"
	}
	return "warning"
}

func renderAzure(w io.Writer, results []Result) error {
	for _, r := range results {
		for _, d := range r.Diagnostics {
			p := positions(r, d)
			if _, err := fmt.Fprintf(w,
				"##vso[task.logissue type=%s;sourcepath=%s;linenumber=%d;columnnumber=%d;]%s %s\n",
				azType(d.Severity), r.Path, p.line, p.col, d.Code, d.Message); err != nil {
				return err
			}
		}
	}
	return nil
}

// --- gitlab code quality ----------------------------------------------------

type glLines struct {
	Begin int `json:"begin"`
}

type glLocation struct {
	Path  string  `json:"path"`
	Lines glLines `json:"lines"`
}

type glIssue struct {
	Description string     `json:"description"`
	CheckName   string     `json:"check_name"`
	Fingerprint string     `json:"fingerprint"`
	Severity    string     `json:"severity"`
	Location    glLocation `json:"location"`
}

func glSeverity(s diagnostic.Severity) string {
	switch s {
	case diagnostic.SeverityError:
		return "major"
	case diagnostic.SeverityWarning:
		return "minor"
	default:
		return "info"
	}
}

func fingerprint(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func renderGitLab(w io.Writer, results []Result) error {
	issues := []glIssue{}
	for _, r := range results {
		for _, d := range r.Diagnostics {
			p := positions(r, d)
			issues = append(issues, glIssue{
				Description: d.Message,
				CheckName:   d.Code,
				Fingerprint: fingerprint(r.Path, d.Code, d.Message, fmt.Sprint(p.line)),
				Severity:    glSeverity(d.Severity),
				Location:    glLocation{Path: r.Path, Lines: glLines{Begin: p.line}},
			})
		}
	}
	data, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

// --- junit ------------------------------------------------------------------

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
}

type junitSuite struct {
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

func renderJUnit(w io.Writer, results []Result) error {
	root := junitSuites{}
	for _, r := range results {
		if len(r.Diagnostics) == 0 {
			continue
		}
		suite := junitSuite{Name: r.Path, Tests: len(r.Diagnostics), Failures: len(r.Diagnostics)}
		for _, d := range r.Diagnostics {
			p := positions(r, d)
			suite.Cases = append(suite.Cases, junitCase{
				Name:      fmt.Sprintf("%s:%d:%d", d.Code, p.line, p.col),
				Classname: r.Path,
				Failure: &junitFailure{
					Message: d.Message,
					Type:    d.Code,
					Text:    fmt.Sprintf("%s %s", d.Code, d.Message),
				},
			})
		}
		root.Tests += suite.Tests
		root.Failures += suite.Failures
		root.Suites = append(root.Suites, suite)
	}
	return writeJUnit(w, root)
}

func writeJUnit(w io.Writer, root junitSuites) error {
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(root); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// --- sarif ------------------------------------------------------------------

type sarifText struct {
	Text string `json:"text"`
}

type sarifRule struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	ShortDescription sarifText `json:"shortDescription"`
}

type sarifDriver struct {
	Name    string      `json:"name"`
	Version string      `json:"version"`
	Rules   []sarifRule `json:"rules"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifArtifact struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
	EndLine     int `json:"endLine"`
	EndColumn   int `json:"endColumn"`
}

type sarifPhysical struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           sarifRegion   `json:"region"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysical `json:"physicalLocation"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifText       `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}

func sarifLevel(s diagnostic.Severity) string {
	switch s {
	case diagnostic.SeverityError:
		return "error"
	case diagnostic.SeverityWarning:
		return "warning"
	default:
		return "note"
	}
}

func renderSARIF(w io.Writer, results []Result) error {
	ruleIndex := map[string]sarifRule{}
	var results2 []sarifResult
	for _, r := range results {
		for _, d := range r.Diagnostics {
			p := positions(r, d)
			if _, ok := ruleIndex[d.Code]; !ok {
				ruleIndex[d.Code] = sarifRule{
					ID:               d.Code,
					Name:             d.Name,
					ShortDescription: sarifText{Text: d.Name},
				}
			}
			results2 = append(results2, sarifResult{
				RuleID:  d.Code,
				Level:   sarifLevel(d.Severity),
				Message: sarifText{Text: d.Message},
				Locations: []sarifLocation{{
					PhysicalLocation: sarifPhysical{
						ArtifactLocation: sarifArtifact{URI: r.Path},
						Region: sarifRegion{
							StartLine: p.line, StartColumn: p.col,
							EndLine: p.endLine, EndColumn: p.endCol,
						},
					},
				}},
			})
		}
	}
	rules := make([]sarifRule, 0, len(ruleIndex))
	for _, r := range ruleIndex {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })

	log := sarifLog{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []sarifRun{{
			Tool:    sarifTool{Driver: sarifDriver{Name: "ruxx", Version: Version, Rules: rules}},
			Results: results2,
		}},
	}
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}
