package e2e

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestOutputFormats drives every renderer through the CLI. Each format is
// chosen because CI consumes it, so a regression here breaks somebody's
// pipeline rather than just a snapshot.
func TestOutputFormats(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.xml", badView)

	cases := []struct {
		format string
		wants  []string
	}{
		{"concise", []string{"a.xml:3:5: O004"}},
		{"full", []string{"a.xml:3:5: O004", "^"}},
		{"grouped", []string{"a.xml:", "3:5", "O004"}},
		{"json", []string{`"code": "O004"`, `"path": "a.xml"`}},
		{"json-lines", []string{`"code":"O004"`}},
		{"github", []string{"::error file=a.xml,line=3,col=5", "O004"}},
		{"gitlab", []string{`"fingerprint"`, "O004"}},
		{"junit", []string{"<testsuite", "<testcase", "O004"}},
		{"sarif", []string{`"version": "2.1.0"`, `"ruleId": "O004"`}},
		{"azure", []string{"##vso[task.logissue", "sourcepath=a.xml", "O004"}},
	}
	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			got := run(t, dir, "", "check", "--output-format", tc.format, "a.xml")
			if got.code != 1 {
				t.Fatalf("exit = %d, want 1 (stderr %q)", got.code, got.stderr)
			}
			for _, want := range tc.wants {
				if !strings.Contains(got.stdout, want) {
					t.Errorf("stdout = %q, want it to contain %q", got.stdout, want)
				}
			}
		})
	}
}

// TestOutputFormat_JSONIsMachineReadable parses the JSON report so a schema
// change cannot pass unnoticed.
func TestOutputFormat_JSONIsMachineReadable(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.xml", badView)

	got := run(t, dir, "", "check", "--output-format", "json", "a.xml")
	var report []map[string]any
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("json is not valid: %v\n%s", err, got.stdout)
	}
	if len(report) != 1 {
		t.Fatalf("len(report) = %d, want 1", len(report))
	}
	if code, _ := report[0]["code"].(string); code != "O004" {
		t.Errorf("code = %v, want O004", report[0]["code"])
	}
	if path, _ := report[0]["path"].(string); path != "a.xml" {
		t.Errorf("path = %v, want a.xml", report[0]["path"])
	}
}

// TestOutputFormat_SARIFDriverIdentity keeps the SARIF tool block honest: CI
// systems key suppressions off it.
func TestOutputFormat_SARIFDriverIdentity(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.xml", badView)

	got := run(t, dir, "", "check", "--output-format", "sarif", "a.xml")
	var log struct {
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				} `json:"driver"`
			} `json:"tool"`
		} `json:"runs"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &log); err != nil {
		t.Fatalf("sarif is not valid: %v\n%s", err, got.stdout)
	}
	if log.Version != "2.1.0" {
		t.Errorf("version = %q, want 2.1.0", log.Version)
	}
	if len(log.Runs) != 1 || log.Runs[0].Tool.Driver.Name != "ruxx" {
		t.Fatalf("runs = %+v, want one run with driver ruxx", log.Runs)
	}
	if log.Runs[0].Tool.Driver.Version == "" {
		t.Error("driver version is empty, want the build identity")
	}
}

// TestOutputFile_WritesToDisk covers -o and the RUXX_OUTPUT_FILE escape hatch,
// both with stdout left empty for pipeline cleanliness.
func TestOutputFile_WritesToDisk(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.xml", badView)

	t.Run("flag", func(t *testing.T) {
		got := run(t, dir, "", "check", "--output-format", "json", "-o", "report.json", "a.xml")
		if got.code != 1 {
			t.Fatalf("exit = %d, want 1", got.code)
		}
		if got.stdout != "" {
			t.Errorf("stdout = %q, want empty when -o is used", got.stdout)
		}
		body := read(t, filepath.Join(dir, "report.json"))
		if !strings.Contains(body, "O004") {
			t.Errorf("report.json = %q, want the finding", body)
		}
	})

	t.Run("environment", func(t *testing.T) {
		got := runEnv(t, dir, "", []string{"RUXX_OUTPUT_FILE=env.json"}, "check", "--output-format", "json", "a.xml")
		if got.code != 1 {
			t.Fatalf("exit = %d, want 1", got.code)
		}
		if got.stdout != "" {
			t.Errorf("stdout = %q, want empty when RUXX_OUTPUT_FILE is set", got.stdout)
		}
		if body := read(t, filepath.Join(dir, "env.json")); !strings.Contains(body, "O004") {
			t.Errorf("env.json = %q, want the finding", body)
		}
	})
}

// TestOutputFormat_MachineOutputIsNeverColoured guards the contract that
// --color only decorates the human-readable renderers.
func TestOutputFormat_MachineOutputIsNeverColoured(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.xml", badView)

	for _, format := range []string{"json", "json-lines", "sarif", "github", "azure", "gitlab", "junit"} {
		got := run(t, dir, "", "check", "--color", "always", "--output-format", format, "a.xml")
		if strings.Contains(got.stdout, "\x1b[") {
			t.Errorf("%s output contains ANSI escapes with --color always", format)
		}
	}

	human := run(t, dir, "", "check", "--color", "always", "a.xml")
	if !strings.Contains(human.stdout, "\x1b[") {
		t.Errorf("concise output = %q, want colour with --color always", human.stdout)
	}
}

// TestEnvironment_CacheDirAndNoCache exercises the cache switches end to end.
func TestEnvironment_CacheDirAndNoCache(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.xml", badView)

	t.Run("RUXX_CACHE_DIR", func(t *testing.T) {
		got := runEnv(t, dir, "", []string{"RUXX_CACHE_DIR=custom-cache"}, "check", "a.xml")
		if got.code != 1 {
			t.Fatalf("exit = %d, want 1", got.code)
		}
		if !exists(filepath.Join(dir, "custom-cache")) {
			t.Error("RUXX_CACHE_DIR was ignored")
		}
	})

	t.Run("RUXX_NO_CACHE", func(t *testing.T) {
		other := t.TempDir()
		write(t, other, "a.xml", badView)
		got := runEnv(t, other, "", []string{"RUXX_NO_CACHE=1"}, "check", "a.xml")
		if got.code != 1 {
			t.Fatalf("exit = %d, want 1", got.code)
		}
		if exists(filepath.Join(other, ".ruxx_cache")) {
			t.Error("RUXX_NO_CACHE still wrote a cache")
		}
	})
}
