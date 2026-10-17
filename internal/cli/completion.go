package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/ppreeper/ruxx/internal/rules"
)

// completionCommand is the subcommand that prints completion scripts.
const completionCommand = "generate-shell-completion"

// completionCommands are the subcommands every completion script offers.
var completionCommands = []string{
	"check", "format", "init", "rule", "config", "clean", "server",
	completionCommand, "version", "help",
}

// completionFlags is the union of the flags accepted by the commands that take
// flags. Value-taking flags are completed by their own case in each script.
var completionFlags = []string{
	"--add-ignore", "--add-noqa", "--cache-dir", "--check", "--color", "--config",
	"--diff", "--exclude", "--exit-non-zero-on-fix", "--exit-non-zero-on-format",
	"--exit-zero", "--extend-exclude", "--extend-ignore", "--extend-select", "--fix",
	"--fix-only", "--fixable", "--force-exclude", "--help", "--ignore",
	"--ignore-suppressions", "--isolated", "--no-cache", "--no-color", "--no-fix",
	"--no-respect-gitignore", "--no-unsafe-fixes", "--output-file", "--output-format",
	"--per-file-ignores", "--preview", "--quiet", "--range", "--respect-gitignore",
	"--select", "--show-files", "--show-fixes", "--show-settings", "--silent",
	"--statistics", "--stdin-filename", "--target-version", "--unfixable", "--unsafe-fixes",
	"--verbose", "--watch", "-h", "-n", "-o", "-q", "-s", "-v", "-w",
}

// completionFormats are the --output-format values.
var completionFormats = []string{
	"concise", "full", "grouped", "json", "json-lines", "github", "gitlab",
	"junit", "sarif", "azure",
}

// completionColors are the --color values.
var completionColors = []string{"auto", "always", "never"}

// completionVersions are the --target-version values.
var completionVersions = []string{"auto", "15.0", "16.0", "17.0", "18.0", "19.0", "20.0"}

// completionSelectors lists every selector accepted by --select, --extend-select,
// --ignore, --extend-ignore, --fixable and --unfixable: ALL, the categories, the
// families, and each rule code.
func completionSelectors() []string {
	seen := map[string]bool{}
	out := make([]string, 0, 128)
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	add("ALL")
	for _, c := range []string{"correctness", "suspicious", "style", "security", "pedantic", "restriction", "formatting"} {
		add(c)
	}
	for _, r := range rules.All() {
		add(r.Family)
		add(r.Code)
	}
	return out
}

// completionScript returns the completion script for a shell.
func completionScript(shell string) (string, bool) {
	switch strings.ToLower(shell) {
	case "bash":
		return bashCompletion(), true
	case "zsh":
		return zshCompletion(), true
	case "fish":
		return fishCompletion(), true
	case "powershell":
		return powershellCompletion(), true
	default:
		return "", false
	}
}

func list(v []string) string { return strings.Join(v, " ") }

func completionUsage(w io.Writer) {
	fmt.Fprint(w, "usage: ruxx "+completionCommand+" <bash|zsh|fish|powershell>\n\n"+
		"Print a completion script for the given shell to stdout. Install it with:\n\n"+
		"  bash        ruxx "+completionCommand+" bash > /etc/bash_completion.d/ruxx\n"+
		"  zsh         ruxx "+completionCommand+" zsh  > \"${fpath[1]}/_ruxx\"\n"+
		"  fish        ruxx "+completionCommand+" fish > ~/.config/fish/completions/ruxx.fish\n"+
		"  powershell  ruxx "+completionCommand+" powershell >> $PROFILE\n\n"+
		"Completions cover the subcommands, every flag, the output formats, the\n"+
		"target versions, the colour modes, and rule selectors.\n")
}

// runGenerateShellCompletion prints a completion script, or usage on error.
func runGenerateShellCompletion(args []string, stdout, stderr io.Writer) int {
	shell := ""
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			completionUsage(stdout)
			return ExitClean
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "ruxx: unknown flag %q\n", a)
			return ExitAbnormal
		case shell != "":
			fmt.Fprintf(stderr, "ruxx: expected one shell name, got %q\n", a)
			return ExitAbnormal
		default:
			shell = a
		}
	}
	if shell == "" {
		completionUsage(stderr)
		return ExitAbnormal
	}
	script, ok := completionScript(shell)
	if !ok {
		fmt.Fprintf(stderr, "ruxx: unsupported shell %q (want bash, zsh, fish, or powershell)\n", shell)
		return ExitAbnormal
	}
	fmt.Fprint(stdout, script)
	return ExitClean
}

func bashCompletion() string {
	return fmt.Sprintf(`# bash completion for ruxx
# Install: ruxx %[1]s bash > /etc/bash_completion.d/ruxx
_ruxx() {
    local cur prev cmd i
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    cmd=""

    for ((i = 1; i < COMP_CWORD; i++)); do
        case "${COMP_WORDS[i]}" in
            -*) ;;
            *) cmd="${COMP_WORDS[i]}"; break ;;
        esac
    done

    case "$prev" in
        --output-format)
            COMPREPLY=( $(compgen -W "%[2]s" -- "$cur") ); return ;;
        --color)
            COMPREPLY=( $(compgen -W "%[3]s" -- "$cur") ); return ;;
        --target-version)
            COMPREPLY=( $(compgen -W "%[4]s" -- "$cur") ); return ;;
        --select|--extend-select|--ignore|--extend-ignore|--fixable|--unfixable)
            COMPREPLY=( $(compgen -W "%[5]s" -- "$cur") ); return ;;
        --config|--cache-dir|--output-file|--stdin-filename|--exclude|--extend-exclude|--range)
            COMPREPLY=( $(compgen -f -- "$cur") ); return ;;
    esac

    if [[ "$cur" == -* ]]; then
        COMPREPLY=( $(compgen -W "%[6]s" -- "$cur") )
        return
    fi

    if [[ -z "$cmd" ]]; then
        COMPREPLY=( $(compgen -W "%[7]s" -- "$cur") )
    fi
    COMPREPLY+=( $(compgen -f -- "$cur") )
}
complete -F _ruxx ruxx
`, completionCommand, list(completionFormats), list(completionColors), list(completionVersions),
		list(completionSelectors()), list(completionFlags), list(completionCommands))
}

func zshCompletion() string {
	return fmt.Sprintf(`#compdef ruxx
# zsh completion for ruxx
# Install: ruxx %[1]s zsh > "${fpath[1]}/_ruxx"
_ruxx() {
    local cur prev cmd formats colors versions selectors flags commands
    formats=(%[2]s)
    colors=(%[3]s)
    versions=(%[4]s)
    selectors=(%[5]s)
    flags=(%[6]s)
    commands=(%[7]s)

    cur="${words[CURRENT]}"
    prev="${words[CURRENT-1]}"
    cmd=""
    local w
    for w in "${words[@]:1}"; do
        case "$w" in
            -*) ;;
            *) cmd="$w"; break ;;
        esac
    done

    case "$prev" in
        --output-format) compadd -a formats; return ;;
        --color) compadd -a colors; return ;;
        --target-version) compadd -a versions; return ;;
        --select|--extend-select|--ignore|--extend-ignore|--fixable|--unfixable)
            compadd -a selectors; return ;;
        --config|--cache-dir|--output-file|--stdin-filename|--exclude|--extend-exclude|--range)
            _files; return ;;
    esac

    if [[ "$cur" == -* ]]; then
        compadd -a flags
        return
    fi

    if [[ -z "$cmd" ]]; then
        compadd -a commands
    fi
    _files
}
_ruxx "$@"
`, completionCommand, list(completionFormats), list(completionColors), list(completionVersions),
		list(completionSelectors()), list(completionFlags), list(completionCommands))
}

func fishCompletion() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# fish completion for ruxx\n# Install: ruxx %s fish > ~/.config/fish/completions/ruxx.fish\n", completionCommand)
	b.WriteString("complete -c ruxx -f\n")
	for _, c := range completionCommands {
		fmt.Fprintf(&b, "complete -c ruxx -n '__fish_use_subcommand' -a %s\n", c)
	}
	for _, f := range completionFlags {
		long := strings.TrimLeft(f, "-")
		if strings.HasPrefix(f, "--") {
			fmt.Fprintf(&b, "complete -c ruxx -l %s\n", long)
		} else {
			fmt.Fprintf(&b, "complete -c ruxx -s %s\n", long)
		}
	}
	valueFlags := []struct {
		flag   string
		values []string
	}{
		{"output-format", completionFormats},
		{"color", completionColors},
		{"target-version", completionVersions},
		{"select", completionSelectors()},
		{"extend-select", completionSelectors()},
		{"ignore", completionSelectors()},
		{"extend-ignore", completionSelectors()},
		{"fixable", completionSelectors()},
		{"unfixable", completionSelectors()},
	}
	for _, vf := range valueFlags {
		fmt.Fprintf(&b, "complete -c ruxx -l %s -x -a '%s'\n", vf.flag, list(vf.values))
	}
	fmt.Fprintf(&b, "complete -c ruxx -l config -r\n")
	fmt.Fprintf(&b, "complete -c ruxx -l cache-dir -r\n")
	fmt.Fprintf(&b, "complete -c ruxx -l output-file -r\n")
	fmt.Fprintf(&b, "complete -c ruxx -l stdin-filename -r\n")
	fmt.Fprintf(&b, "complete -c ruxx -l exclude -r\n")
	fmt.Fprintf(&b, "complete -c ruxx -l extend-exclude -r\n")
	fmt.Fprintf(&b, "complete -c ruxx -l range -r\n")
	return b.String()
}

func powershellCompletion() string {
	return fmt.Sprintf(`# powershell completion for ruxx
# Install: ruxx %[1]s powershell >> $PROFILE
Register-ArgumentCompleter -Native -CommandName ruxx -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $commands = @(%[2]s)
    $flags = @(%[3]s)
    $formats = @(%[4]s)
    $colors = @(%[5]s)
    $versions = @(%[6]s)
    $selectors = @(%[7]s)

    $elements = $commandAst.CommandElements | ForEach-Object { $_.ToString() }
    $prev = if ($elements.Count -gt 0) { $elements[-1] } else { "" }
    if ($wordToComplete -ne "" -and $elements.Count -gt 1) { $prev = $elements[-2] }

    $candidates = switch -Regex ($prev) {
        '^--output-format$' { $formats; break }
        '^--color$' { $colors; break }
        '^--target-version$' { $versions; break }
        '^--(select|extend-select|ignore|extend-ignore|fixable|unfixable)$' { $selectors; break }
        default {
            if ($wordToComplete -like '-*') { $flags } else { $commands }
        }
    }

    $candidates |
        Where-Object { $_ -like "$wordToComplete*" } |
        ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) }
}
`, completionCommand,
		quoteAll(completionCommands), quoteAll(completionFlags), quoteAll(completionFormats),
		quoteAll(completionColors), quoteAll(completionVersions), quoteAll(completionSelectors()))
}

// quoteAll renders a PowerShell string array body: 'a', 'b', 'c'.
func quoteAll(v []string) string {
	q := make([]string, len(v))
	for i, s := range v {
		q[i] = "'" + s + "'"
	}
	return strings.Join(q, ", ")
}
