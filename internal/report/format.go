package report

import (
	"encoding/json"
	"fmt"
	"io"
)

// FormatFile is a file the formatter would change.
type FormatFile struct {
	Path   string
	Before string
	After  string
}

// FormatFormats lists the --output-format values supported by
// `ruxx format --check`.
var FormatFormats = Formats

// RenderFormatCheck writes the outcome of `ruxx format --check` in the
// requested format. An empty format means "concise". An unknown format is an
// error.
func RenderFormatCheck(w io.Writer, files []FormatFile, format string) error {
	if format == "" {
		format = "concise"
	}
	switch format {
	case "concise", "full", "grouped":
		for _, f := range files {
			if _, err := fmt.Fprintf(w, "would reformat %s\n", f.Path); err != nil {
				return err
			}
		}
		return nil
	case "json":
		data, err := json.MarshalIndent(formatJSON(files), "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "%s\n", data)
		return err
	case "json-lines":
		for _, f := range formatJSON(files) {
			data, err := json.Marshal(f)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "%s\n", data); err != nil {
				return err
			}
		}
		return nil
	case "github":
		for _, f := range files {
			if _, err := fmt.Fprintf(w, "::warning file=%s,line=1,col=1::would reformat\n", f.Path); err != nil {
				return err
			}
		}
		return nil
	case "azure":
		for _, f := range files {
			if _, err := fmt.Fprintf(w, "##vso[task.logissue type=warning;sourcepath=%s;linenumber=1;]would reformat\n", f.Path); err != nil {
				return err
			}
		}
		return nil
	case "gitlab":
		issues := []glIssue{}
		for _, f := range files {
			issues = append(issues, glIssue{
				Description: "would reformat",
				CheckName:   "format",
				Fingerprint: fingerprint(f.Path, "format"),
				Severity:    "minor",
				Location:    glLocation{Path: f.Path, Lines: glLines{Begin: 1}},
			})
		}
		data, err := json.MarshalIndent(issues, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "%s\n", data)
		return err
	case "junit":
		root := junitSuites{}
		for _, f := range files {
			suite := junitSuite{
				Name:     f.Path,
				Tests:    1,
				Failures: 1,
				Cases: []junitCase{{
					Name:      "format",
					Classname: f.Path,
					Failure:   &junitFailure{Message: "would reformat", Type: "format", Text: "would reformat " + f.Path},
				}},
			}
			root.Tests++
			root.Failures++
			root.Suites = append(root.Suites, suite)
		}
		return writeJUnit(w, root)
	case "sarif":
		log := sarifLog{
			Version: "2.1.0",
			Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
			Runs: []sarifRun{{
				Tool: sarifTool{Driver: sarifDriver{
					Name:    "ruxx",
					Version: Version,
					Rules:   []sarifRule{{ID: "format", Name: "format", ShortDescription: sarifText{Text: "format"}}},
				}},
				Results: []sarifResult{},
			}},
		}
		for _, f := range files {
			log.Runs[0].Results = append(log.Runs[0].Results, sarifResult{
				RuleID:  "format",
				Level:   "warning",
				Message: sarifText{Text: "would reformat"},
				Locations: []sarifLocation{{
					PhysicalLocation: sarifPhysical{
						ArtifactLocation: sarifArtifact{URI: f.Path},
						Region:           sarifRegion{StartLine: 1, StartColumn: 1},
					},
				}},
			})
		}
		data, err := json.MarshalIndent(log, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "%s\n", data)
		return err
	default:
		return fmt.Errorf("unknown output format %q", format)
	}
}

type formatJSONItem struct {
	Path          string `json:"path"`
	WouldReformat bool   `json:"wouldReformat"`
}

func formatJSON(files []FormatFile) []formatJSONItem {
	out := []formatJSONItem{}
	for _, f := range files {
		out = append(out, formatJSONItem{Path: f.Path, WouldReformat: true})
	}
	return out
}
