package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// binPath is the freshly built ruxx binary shared by every test.
var binPath string

// buildErr is set when TestMain could not build the binary.
var buildErr error

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ruxx-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: temp dir:", err)
		os.Exit(1)
	}
	name := "ruxx"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binPath = filepath.Join(dir, name)

	// Build by import path so the test works from any working directory.
	cmd := exec.Command("go", "build", "-o", binPath, "github.com/ppreeper/ruxx/cmd/ruxx")
	if out, err := cmd.CombinedOutput(); err != nil {
		buildErr = fmt.Errorf("building ruxx: %v\n%s", err, out)
	}

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// result is what one run of the binary produced.
type result struct {
	code   int
	stdout string
	stderr string
}

// run executes the built binary with dir as the working directory. Relative
// arguments are therefore relative to dir, which keeps assertions free of
// absolute paths and platform path separators.
func run(t *testing.T, dir, stdin string, args ...string) result {
	t.Helper()
	return runEnv(t, dir, stdin, nil, args...)
}

// runEnv is run with extra environment variables appended to the inherited
// environment.
func runEnv(t *testing.T, dir, stdin string, env []string, args ...string) result {
	t.Helper()
	if buildErr != nil {
		t.Fatalf("%v", buildErr)
	}
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if ok := asExitError(err, &ee); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running %v: %v", args, err)
		}
	}
	return result{code: code, stdout: out.String(), stderr: errb.String()}
}

func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

// write creates dir/name (parents included) and returns the path.
func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// read returns the contents of path.
func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// exists reports whether path is present.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Fixtures. Each is a realistic minimal file so diagnostics land where a user
// would expect them.
const (
	// badView has a record without a model (O004 at 3:5) and trailing
	// whitespace on line 2 (X007).
	badView = "<?xml version=\"1.0\"?>\n<odoo>   \n    <record id=\"foo\">\n        <field name=\"name\">F</field>\n    </record>\n</odoo>\n"

	// cleanView lints clean under the default (correctness) rule set and is
	// already formatted.
	cleanView = "<?xml version=\"1.0\"?>\n<odoo>\n    <record id=\"foo\" model=\"ir.ui.view\">\n        <field name=\"name\">F</field>\n    </record>\n</odoo>\n"

	// unformattedView is valid and lint-clean but needs reformatting: an empty
	// element expanded, a redundant space before the self-close, and no blank
	// line before the record.
	unformattedView = "<?xml version=\"1.0\"?>\n<odoo><record id=\"a\" model=\"ir.ui.view\" >\n<field name=\"name\">A</field></record></odoo>\n"
)
