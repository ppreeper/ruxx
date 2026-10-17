package cli_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

// everyShell is the list of shells the subcommand must support.
var everyShell = []string{"bash", "zsh", "fish", "powershell"}

func TestShellCompletion_Bash(t *testing.T) {
	code, stdout, stderr := run(t, "", "generate-shell-completion", "bash")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, cli.ExitClean, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	for _, want := range []string{
		"complete -F _ruxx ruxx",
		"COMPREPLY",
		"--output-format",
		"--select|--extend-select",
		"--stdin-filename|--exclude",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("bash script missing %q", want)
		}
	}
}

func TestShellCompletion_BashSyntax(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	_, stdout, _ := run(t, "", "generate-shell-completion", "bash")
	cmd := exec.Command(bash, "-n")
	cmd.Stdin = strings.NewReader(stdout)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bash -n rejected the script: %v\n%s", err, out)
	}
}

func TestShellCompletion_Zsh(t *testing.T) {
	code, stdout, _ := run(t, "", "generate-shell-completion", "zsh")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d", code, cli.ExitClean)
	}
	for _, want := range []string{"#compdef ruxx", "compadd", "--range", "_files"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("zsh script missing %q", want)
		}
	}
}

func TestShellCompletion_Fish(t *testing.T) {
	code, stdout, _ := run(t, "", "generate-shell-completion", "fish")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d", code, cli.ExitClean)
	}
	for _, want := range []string{
		"complete -c ruxx -f",
		"complete -c ruxx -n '__fish_use_subcommand' -a check",
		"complete -c ruxx -l output-format -x -a 'concise",
		"complete -c ruxx -l config -r",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("fish script missing %q", want)
		}
	}
}

func TestShellCompletion_PowerShell(t *testing.T) {
	code, stdout, _ := run(t, "", "generate-shell-completion", "powershell")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d", code, cli.ExitClean)
	}
	for _, want := range []string{"Register-ArgumentCompleter", "-CommandName ruxx", "'check'", "CompletionResult"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("powershell script missing %q", want)
		}
	}
}

func TestShellCompletion_CompletesCommandsAndSelectors(t *testing.T) {
	for _, shell := range everyShell {
		code, stdout, _ := run(t, "", "generate-shell-completion", shell)
		if code != cli.ExitClean {
			t.Fatalf("%s: exit = %d, want %d", shell, code, cli.ExitClean)
		}
		for _, want := range []string{
			"check", "format", "init", "rule", "config", "clean", "server",
			"generate-shell-completion", "version", "help",
			"ALL", "correctness", "X001", "O004", "XP007",
			"concise", "sarif", "auto", "20.0",
		} {
			if !strings.Contains(stdout, want) {
				t.Errorf("%s script missing %q", shell, want)
			}
		}
	}
}

func TestShellCompletion_UnknownShell(t *testing.T) {
	code, _, stderr := run(t, "", "generate-shell-completion", "nushell")
	if code != cli.ExitAbnormal {
		t.Fatalf("exit = %d, want %d", code, cli.ExitAbnormal)
	}
	if !strings.Contains(stderr, "unsupported shell") || !strings.Contains(stderr, "bash") {
		t.Errorf("stderr = %q, want an unsupported-shell message naming the shells", stderr)
	}
}

func TestShellCompletion_MissingShell(t *testing.T) {
	code, _, stderr := run(t, "", "generate-shell-completion")
	if code != cli.ExitAbnormal {
		t.Fatalf("exit = %d, want %d", code, cli.ExitAbnormal)
	}
	if !strings.Contains(stderr, "usage: ruxx generate-shell-completion") {
		t.Errorf("stderr = %q, want the usage line", stderr)
	}
}

func TestShellCompletion_Help(t *testing.T) {
	code, stdout, _ := run(t, "", "generate-shell-completion", "--help")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d", code, cli.ExitClean)
	}
	for _, want := range []string{"usage: ruxx generate-shell-completion", "bash", "zsh", "fish", "powershell"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestShellCompletion_Errors(t *testing.T) {
	if code, _, stderr := run(t, "", "generate-shell-completion", "--bogus"); code != cli.ExitAbnormal || !strings.Contains(stderr, "--bogus") {
		t.Errorf("unknown flag: exit = %d, stderr = %q", code, stderr)
	}
	if code, _, stderr := run(t, "", "generate-shell-completion", "bash", "zsh"); code != cli.ExitAbnormal || !strings.Contains(stderr, "one shell name") {
		t.Errorf("extra argument: exit = %d, stderr = %q", code, stderr)
	}
}

func TestShellCompletion_ListedInHelp(t *testing.T) {
	code, stdout, _ := run(t, "", "help")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d", code, cli.ExitClean)
	}
	if !strings.Contains(stdout, "generate-shell-completion") {
		t.Errorf("top-level help does not list generate-shell-completion:\n%s", stdout)
	}
}
