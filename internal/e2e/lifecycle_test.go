package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestInit_ScaffoldsAWorkingConfig is the first-run story: init, then lint the
// tree with the scaffolded configuration.
func TestInit_ScaffoldsAWorkingConfig(t *testing.T) {
	dir := t.TempDir()

	got := run(t, dir, "", "init")
	if got.code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, ".ruxx.toml") {
		t.Errorf("stdout = %q, want the created path", got.stdout)
	}
	cfg := read(t, filepath.Join(dir, ".ruxx.toml"))
	if !strings.Contains(cfg, "[lint]") || !strings.Contains(cfg, "[format]") {
		t.Fatalf("scaffolded config =\n%s\nwant [lint] and [format] sections", cfg)
	}

	write(t, dir, "bad.xml", badView)
	if code := run(t, dir, "", "check", "bad.xml").code; code != 1 {
		t.Fatalf("exit = %d, want 1 against the scaffolded config", code)
	}
}

// TestInit_RefusesToClobber covers the safety check and its override.
func TestInit_RefusesToClobber(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".ruxx.toml", "# mine\n")

	got := run(t, dir, "", "init")
	if got.code != 2 {
		t.Fatalf("exit = %d, want 2", got.code)
	}
	if !strings.Contains(got.stderr, "already exists") {
		t.Errorf("stderr = %q, want an overwrite warning", got.stderr)
	}
	if read(t, filepath.Join(dir, ".ruxx.toml")) != "# mine\n" {
		t.Error("init overwrote an existing config without --force")
	}

	forced := run(t, dir, "", "init", "--force")
	if forced.code != 0 {
		t.Fatalf("exit = %d, want 0 with --force", forced.code)
	}
	if !strings.Contains(read(t, filepath.Join(dir, ".ruxx.toml")), "[lint]") {
		t.Error("--force did not rewrite the config")
	}
}

// TestClean_RemovesTheCaches drives the two-cache cleanup.
func TestClean_RemovesTheCaches(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "bad.xml", badView)

	if code := run(t, dir, "", "check", "bad.xml").code; code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	cacheDir := filepath.Join(dir, ".ruxx_cache")
	if !exists(cacheDir) {
		t.Fatalf("check did not populate %s", cacheDir)
	}

	got := run(t, dir, "", "clean")
	if got.code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "removed") {
		t.Errorf("stdout = %q, want a removal notice", got.stdout)
	}
	if exists(cacheDir) {
		t.Errorf("%s still exists after clean", cacheDir)
	}
}

// TestVersion_ReportsBuildIdentity checks the shape the Makefile and goreleaser
// stamp into the binary.
func TestVersion_ReportsBuildIdentity(t *testing.T) {
	dir := t.TempDir()

	for _, args := range [][]string{{"version"}, {"--version"}, {"-V"}} {
		got := run(t, dir, "", args...)
		if got.code != 0 {
			t.Fatalf("%v: exit = %d, want 0", args, got.code)
		}
		out := strings.TrimSpace(got.stdout)
		if !strings.HasPrefix(out, "ruxx ") {
			t.Errorf("%v: stdout = %q, want a \"ruxx <version>\" line", args, out)
		}
		if strings.Contains(out, "\n") {
			t.Errorf("%v: stdout = %q, want a single line", args, out)
		}
	}
}

// TestHelp_CoversEverySubcommand is the discoverability contract.
func TestHelp_CoversEverySubcommand(t *testing.T) {
	dir := t.TempDir()

	top := run(t, dir, "", "help")
	if top.code != 0 {
		t.Fatalf("exit = %d, want 0", top.code)
	}
	for _, cmd := range []string{"check", "format", "init", "rule", "config", "clean", "server", "generate-shell-completion", "version", "help"} {
		if !strings.Contains(top.stdout, cmd) {
			t.Errorf("top-level help is missing %q:\n%s", cmd, top.stdout)
		}
	}

	for _, cmd := range []string{"check", "format", "clean", "server"} {
		got := run(t, dir, "", cmd, "--help")
		if got.code != 0 {
			t.Fatalf("%s --help: exit = %d, want 0", cmd, got.code)
		}
		if !strings.Contains(got.stdout, "usage: ruxx "+cmd) {
			t.Errorf("%s --help: stdout = %q, want a usage line", cmd, got.stdout)
		}
	}
}

// TestUnknownCommand_ExitsAbnormal keeps typos loud.
func TestUnknownCommand_ExitsAbnormal(t *testing.T) {
	dir := t.TempDir()

	got := run(t, dir, "", "lint")
	if got.code != 2 {
		t.Fatalf("exit = %d, want 2", got.code)
	}
	if !strings.Contains(got.stderr, "lint") {
		t.Errorf("stderr = %q, want it to name the bad command", got.stderr)
	}
}

// TestShellCompletion_Scripts checks the generated scripts for the shell idioms
// each one needs.
func TestShellCompletion_Scripts(t *testing.T) {
	dir := t.TempDir()

	cases := map[string]string{
		"bash":       "complete -F _ruxx ruxx",
		"zsh":        "#compdef ruxx",
		"fish":       "complete -c ruxx",
		"powershell": "Register-ArgumentCompleter",
	}
	for shell, want := range cases {
		got := run(t, dir, "", "generate-shell-completion", shell)
		if got.code != 0 {
			t.Fatalf("%s: exit = %d, want 0 (stderr %q)", shell, got.code, got.stderr)
		}
		if !strings.Contains(got.stdout, want) {
			t.Errorf("%s: script is missing %q", shell, want)
		}
		if !strings.Contains(got.stdout, "generate-shell-completion") {
			t.Errorf("%s: script does not complete the subcommands", shell)
		}
	}

	bad := run(t, dir, "", "generate-shell-completion", "nushell")
	if bad.code != 2 {
		t.Fatalf("exit = %d, want 2 for an unsupported shell", bad.code)
	}
	if !strings.Contains(bad.stderr, "bash") {
		t.Errorf("stderr = %q, want the supported shells listed", bad.stderr)
	}
}
