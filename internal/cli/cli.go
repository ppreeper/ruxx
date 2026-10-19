// Package cli implements the ruxx command-line interface.
//
// The entry point is Run, which takes the argument vector (without the program
// name) and explicit stdin/stdout/stderr streams. This keeps the CLI fully
// testable in-process: no subprocesses, no globals. cmd/ruxx/main.go is a thin
// wrapper that calls Run with the real process streams.
//
// Exit-code contract (mirrors ESLint/Prettier/Ruff): 0 = clean, 1 = findings
// (or files that would change under --check), 2 = abnormal (bad usage, bad
// config, I/O error).
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ppreeper/ruxx/internal/cache"
	"github.com/ppreeper/ruxx/internal/config"
	"github.com/ppreeper/ruxx/internal/diagnostic"
	"github.com/ppreeper/ruxx/internal/fix"
	"github.com/ppreeper/ruxx/internal/formatter"
	"github.com/ppreeper/ruxx/internal/index"
	"github.com/ppreeper/ruxx/internal/linter"
	"github.com/ppreeper/ruxx/internal/lsp"
	"github.com/ppreeper/ruxx/internal/report"
	"github.com/ppreeper/ruxx/internal/rules"
	"github.com/ppreeper/ruxx/internal/settings"
	"github.com/ppreeper/ruxx/internal/source"
	"github.com/ppreeper/ruxx/internal/suppress"
	"github.com/ppreeper/ruxx/internal/syntax"
	"github.com/ppreeper/ruxx/internal/version"
	"github.com/ppreeper/ruxx/internal/walk"
)

// Exit codes returned by Run.
const (
	ExitClean    = 0
	ExitFindings = 1
	ExitAbnormal = 2
)

// Run parses args, executes the requested command, writes to stdout/stderr,
// and returns the process exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// No subcommand defaults to check.
	if len(args) == 0 {
		return runCheck(nil, stdin, stdout, stderr)
	}

	switch args[0] {
	case "check":
		return runCheck(args[1:], stdin, stdout, stderr)
	case "format":
		return runFormat(args[1:], stdin, stdout, stderr)
	case "clean":
		return runClean(args[1:], stdout, stderr)
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "rule":
		return runRule(args[1:], stdout, stderr)
	case "config":
		return runConfig(args[1:], stdout, stderr)
	case "server":
		return runServer(args[1:], stdin, stdout, stderr)
	case "generate-shell-completion":
		return runGenerateShellCompletion(args[1:], stdout, stderr)
	case "version", "--version", "-V":
		fmt.Fprintln(stdout, "ruxx "+version.String())
		return ExitClean
	case "help", "--help", "-h":
		usage(stdout)
		return ExitClean
	default:
		// Allow flags without an explicit subcommand (e.g. `ruxx --select O004`).
		if strings.HasPrefix(args[0], "-") {
			return runCheck(args, stdin, stdout, stderr)
		}
		fmt.Fprintf(stderr, "ruxx: unknown command %q\n", args[0])
		usage(stderr)
		return ExitAbnormal
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `ruxx - a fast XML linter and formatter with Odoo awareness

Usage:
  ruxx check [flags] [paths...]    Lint XML files (default command)
  ruxx format [flags] [paths...]   Format XML files
  ruxx init [dir] [--force]        Write a starter .ruxx.toml
  ruxx rule [CODE|SELECTOR]        Explain a rule, or list the rules
  ruxx config [KEY]                List or describe configuration keys
  ruxx clean [flags] [paths...]    Clear the lint cache
  ruxx server                      Run the language server (LSP over stdio)
  ruxx generate-shell-completion <SHELL>  Print a completion script
  ruxx version                     Print the version
  ruxx help                        Print this help

Run "ruxx check --help" or "ruxx format --help" for command flags.
`)
}

// options holds parsed command-line flags shared by check and format.
type options struct {
	paths            []string
	exitZero         bool
	diff             bool
	fix              bool
	unsafeFixes      bool
	check            bool // format --check
	isolated         bool
	configFile       string
	selectCSV        string
	ignoreCSV        string
	stdinFilename    string
	outputFormat     string
	statistics       bool
	noCache          bool
	cacheDir         string
	watch            bool
	addIgnore        bool
	colorMode        string
	outputFile       string
	excludeCSV       string
	extendExcludeCSV string
	forceExclude     bool
	noGitignore      bool
	showFiles        bool

	extendSelectCSV    string
	extendIgnoreCSV    string
	targetVersion      string
	fixOnly            bool
	showFixes          bool
	exitNonZeroOnFix   bool
	ignoreSuppressions bool
	perFileIgnores     []globCodes

	fixableCSV   string
	unfixableCSV string
	preview      bool
	showSettings bool
	quiet        bool
	silent       bool
	verbose      bool
	help         bool

	// exitNonZeroOnFormat makes `ruxx format` exit 1 when it rewrote a file.
	exitNonZeroOnFormat bool

	// formatRange is the parsed --range selection; it only applies to
	// `ruxx format`.
	formatRange *rangeLines

	// lspStdio and lspClientPID record transport flags that LSP clients add
	// when they launch `ruxx server` (vscode-languageclient sends
	// `--stdio --clientProcessId=<pid>`). The server always speaks stdio, so
	// both are accepted and ignored; they are rejected for every other
	// subcommand.
	lspStdio     bool
	lspClientPID bool
}

// rangeLines is a parsed `--range start_line:col-end_line:col` selection, with
// 1-based lines and rune columns. A missing column means column 1 for the
// start and the end of the line for the end.
type rangeLines struct {
	startLine, startCol int
	endLine, endCol     int
}

// endOfLineColumn is the column used when a range position names only a line.
const endOfLineColumn = 1 << 30

// parseRangeSpec parses "start_line:col-end_line:col".
func parseRangeSpec(spec string) (rangeLines, error) {
	bad := func(msg string) (rangeLines, error) {
		return rangeLines{}, fmt.Errorf("invalid --range %q: %s", spec, msg)
	}
	start, end, ok := strings.Cut(spec, "-")
	if !ok {
		return bad("want start_line:col-end_line:col")
	}
	sl, sc, err := parseRangePos(start)
	if err != nil {
		return bad(err.Error())
	}
	el, ec, err := parseRangePos(end)
	if err != nil {
		return bad(err.Error())
	}
	if el < sl || (el == sl && ec < sc) {
		return bad("end is before start")
	}
	return rangeLines{startLine: sl, startCol: sc, endLine: el, endCol: ec}, nil
}

// parseRangePos parses one "line:col" position, or a bare line.
func parseRangePos(s string) (line, col int, err error) {
	s = strings.TrimSpace(s)
	lineStr, colStr, hasCol := strings.Cut(s, ":")
	line, err = strconv.Atoi(lineStr)
	if err != nil || line < 1 {
		return 0, 0, fmt.Errorf("bad position %q", s)
	}
	if !hasCol {
		return line, endOfLineColumn, nil
	}
	col, err = strconv.Atoi(colStr)
	if err != nil || col < 1 {
		return 0, 0, fmt.Errorf("bad position %q", s)
	}
	return line, col, nil
}

// resolve maps the selection to byte offsets in text.
func (r rangeLines) resolve(text string) source.Range {
	loc := source.NewLocator(text)
	return source.NewRange(loc.Offset(r.startLine, r.startCol), loc.Offset(r.endLine, r.endCol))
}

// formatText formats text, honouring the --range selection when one is set.
func formatText(text string, s *settings.Settings, o *options) (string, error) {
	if o.formatRange != nil {
		return formatter.FormatRange(text, s, o.formatRange.resolve(text))
	}
	return formatter.Format(text, s)
}

// globCodes is one --per-file-ignores entry: a glob and the codes it ignores.
type globCodes struct {
	glob  string
	codes []string
}

// parseArgs parses the shared flag set. Unknown flags are an error.
func parseArgs(args []string, cmd string) (*options, error) {
	o := &options{outputFormat: "concise", colorMode: "auto"}
	// Environment defaults; explicit flags win because they are parsed after.
	if v := os.Getenv("RUXX_OUTPUT_FILE"); v != "" {
		o.outputFile = v
	}
	if v := os.Getenv("RUXX_CACHE_DIR"); v != "" {
		o.cacheDir = v
	}
	if os.Getenv("RUXX_NO_CACHE") != "" {
		o.noCache = true
	}
	i := 0
	needValue := func(flag string, inline string) (string, error) {
		if inline != "" {
			return inline, nil
		}
		if i+1 >= len(args) {
			return "", fmt.Errorf("flag %s requires a value", flag)
		}
		i++
		return args[i], nil
	}
	for ; i < len(args); i++ {
		a := args[i]
		if a == "-" || !strings.HasPrefix(a, "-") {
			o.paths = append(o.paths, a)
			continue
		}
		name := a
		inline := ""
		if eq := strings.IndexByte(a, '='); eq >= 0 {
			name, inline = a[:eq], a[eq+1:]
		}
		switch name {
		case "--exit-zero":
			o.exitZero = true
		case "--diff":
			// A diff only makes sense against the fixed source, so --diff
			// implies --fix (without writing).
			o.diff = true
			o.fix = true
		case "--fix":
			o.fix = true
		case "--no-fix":
			o.fix = false
		case "--fix-only":
			// Apply fixes but report nothing else.
			o.fix = true
			o.fixOnly = true
		case "--show-fixes":
			o.showFixes = true
		case "--unsafe-fixes":
			o.unsafeFixes = true
		case "--no-unsafe-fixes":
			o.unsafeFixes = false
		case "--exit-non-zero-on-fix":
			o.exitNonZeroOnFix = true
		case "--ignore-suppressions":
			o.ignoreSuppressions = true
		case "--check":
			o.check = true
		case "--isolated":
			o.isolated = true
		case "--statistics":
			o.statistics = true
		case "--add-ignore", "--add-noqa":
			o.addIgnore = true
		case "--color":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.colorMode = v
		case "--no-color":
			o.colorMode = "never"
		case "-n", "--no-cache":
			o.noCache = true
		case "-w", "--watch":
			o.watch = true
		case "--cache-dir":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.cacheDir = v
		case "--config":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.configFile = v
		case "--select":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.selectCSV = v
		case "--extend-select":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.extendSelectCSV = v
		case "--ignore":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.ignoreCSV = v
		case "--extend-ignore":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.extendIgnoreCSV = v
		case "--target-version":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.targetVersion = v
		case "--per-file-ignores":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			entry, err := parsePerFileIgnores(v)
			if err != nil {
				return nil, err
			}
			o.perFileIgnores = append(o.perFileIgnores, entry)
		case "--stdin-filename":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.stdinFilename = v
		case "--output-format":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.outputFormat = v
		case "-o", "--output-file":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.outputFile = v
		case "--exclude":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.excludeCSV = v
		case "--extend-exclude":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.extendExcludeCSV = v
		case "--force-exclude":
			o.forceExclude = true
		case "--no-respect-gitignore":
			o.noGitignore = true
		case "--respect-gitignore":
			o.noGitignore = false
		case "--show-files":
			o.showFiles = true
		case "-h", "--help":
			o.help = true
		case "--range":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			rl, err := parseRangeSpec(v)
			if err != nil {
				return nil, err
			}
			o.formatRange = &rl
		case "--fixable":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.fixableCSV = v
		case "--unfixable":
			v, err := needValue(name, inline)
			if err != nil {
				return nil, err
			}
			o.unfixableCSV = v
		case "--preview":
			o.preview = true
		case "--show-settings":
			o.showSettings = true
		case "-q", "--quiet":
			o.quiet = true
		case "-s", "--silent":
			o.silent = true
		case "-v", "--verbose":
			o.verbose = true
		case "--exit-non-zero-on-format":
			o.exitNonZeroOnFormat = true
		case "--stdio":
			// LSP clients commonly pass --stdio to select the transport; the
			// server only speaks stdio, so accept and ignore it.
			o.lspStdio = true
		case "--clientProcessId":
			// vscode-languageclient passes --clientProcessId=<pid>; accept
			// and ignore it.
			if _, err := needValue(name, inline); err != nil {
				return nil, err
			}
			o.lspClientPID = true
		default:
			return nil, fmt.Errorf("unknown flag %q", a)
		}
	}
	if o.formatRange != nil && cmd != "format" {
		return nil, fmt.Errorf("--range is only supported by ruxx format")
	}
	if o.exitNonZeroOnFormat && cmd != "format" {
		return nil, fmt.Errorf("--exit-non-zero-on-format is only supported by ruxx format")
	}
	if o.lspStdio && cmd != "server" {
		return nil, fmt.Errorf("--stdio is only supported by ruxx server")
	}
	if o.lspClientPID && cmd != "server" {
		return nil, fmt.Errorf("--clientProcessId is only supported by ruxx server")
	}
	if len(o.paths) == 0 {
		o.paths = []string{"."}
	}
	return o, nil
}

// settingsFor resolves the effective settings for a single path, honoring
// --config, --isolated and --select/--ignore overrides. Callers that visit many
// files should share a runState instead, so the config is not re-resolved (nor
// the parent directories re-walked) once per file.
func settingsFor(o *options, path string) (*settings.Settings, error) {
	return newRunState(o).settings(path)
}

// runState holds the per-run shared state that is expensive to build and needed
// by every file: the resolved config, the lint cache, and the cross-file index.
// The parallel workers share one runState, so the lookups are guarded by mutexes
// and each artifact is built at most once per run.
type runState struct {
	o *options

	cfgMu    sync.Mutex
	confFile *config.Config
	dirs     map[string]*config.Config

	buildMu sync.Mutex
	caches  map[string]*cache.Cache
	indexes map[string]*index.Index
}

func newRunState(o *options) *runState {
	return &runState{
		o:       o,
		dirs:    map[string]*config.Config{},
		caches:  map[string]*cache.Cache{},
		indexes: map[string]*index.Index{},
	}
}

// settings resolves the effective settings for path, applying CLI overrides.
func (r *runState) settings(path string) (*settings.Settings, error) {
	s, err := r.base(path)
	if err != nil {
		return nil, err
	}
	o := r.o
	if o.selectCSV != "" {
		s.Select = splitCSV(o.selectCSV)
	}
	if o.extendSelectCSV != "" {
		// Extending keeps the default set and adds to it.
		if s.Select == nil {
			s.Select = []string{"correctness"}
		}
		s.Select = append(append([]string{}, s.Select...), splitCSV(o.extendSelectCSV)...)
	}
	if o.ignoreCSV != "" {
		s.Ignore = splitCSV(o.ignoreCSV)
	}
	if o.extendIgnoreCSV != "" {
		s.Ignore = append(append([]string{}, s.Ignore...), splitCSV(o.extendIgnoreCSV)...)
	}
	for _, p := range o.perFileIgnores {
		if config.MatchGlob(p.glob, path) {
			s.Ignore = append(append([]string{}, s.Ignore...), p.codes...)
		}
	}
	if o.targetVersion != "" {
		s.TargetVersion = o.targetVersion
	}
	if o.ignoreSuppressions {
		s.IgnoreSuppressions = true
	}
	if o.fixableCSV != "" {
		s.Fixable = splitCSV(o.fixableCSV)
	}
	if o.unfixableCSV != "" {
		s.Unfixable = splitCSV(o.unfixableCSV)
	}
	if o.preview {
		s.Preview = true
	}
	return s, nil
}

// base resolves the config that governs path and derives its settings.
// SettingsFor returns a fresh value, so callers may mutate the result.
func (r *runState) base(path string) (*settings.Settings, error) {
	switch {
	case r.o.configFile != "":
		cfg, err := r.explicit()
		if err != nil {
			return nil, err
		}
		return cfg.SettingsFor(path), nil
	case r.o.isolated:
		return settings.Default(), nil
	}
	cfg, err := r.discover(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	return cfg.SettingsFor(path), nil
}

// explicit loads the --config file once per run.
func (r *runState) explicit() (*config.Config, error) {
	r.cfgMu.Lock()
	defer r.cfgMu.Unlock()
	if r.confFile == nil {
		cfg, err := config.Load(r.o.configFile)
		if err != nil {
			return nil, err
		}
		r.confFile = cfg
	}
	return r.confFile, nil
}

// discover finds the closest config for dir and remembers the answer for every
// directory on the way up. Without it a large tree re-walks (and stats) the same
// parent directories once per file.
func (r *runState) discover(dir string) (*config.Config, error) {
	r.cfgMu.Lock()
	defer r.cfgMu.Unlock()
	if cfg, ok := r.dirs[dir]; ok {
		return cfg, nil
	}
	cfg, err := config.DiscoverFrom(dir)
	if err != nil {
		return nil, err
	}
	d := dir
	for {
		r.dirs[d] = cfg
		// Stop at the directory that owns the config; above it discovery
		// could resolve differently. With defaults, stop at the root.
		if (cfg.Path != "" && d == cfg.Dir) || filepath.Dir(d) == d {
			break
		}
		d = filepath.Dir(d)
	}
	return cfg, nil
}

// cache returns the lint cache for the settings, or nil when caching is off.
func (r *runState) cache(s *settings.Settings) *cache.Cache {
	if r.o.noCache || r.o.fix || r.o.addIgnore {
		return nil
	}
	dir := r.o.cacheDir
	if dir == "" {
		dir = s.CacheDir
	}
	if dir == "" {
		return nil
	}
	r.buildMu.Lock()
	defer r.buildMu.Unlock()
	if c, ok := r.caches[dir]; ok {
		return c
	}
	c, err := cache.Open(dir)
	if err != nil {
		c = nil
	}
	r.caches[dir] = c
	return c
}

// index returns the cross-file index for the settings, building it at most once
// per (cache dir, modules path) pair. Without the guard every worker would race
// to build the whole index on its first file.
func (r *runState) index(s *settings.Settings) *index.Index {
	if len(s.ModulesPath) == 0 {
		return nil
	}
	dir := r.o.cacheDir
	if dir == "" {
		dir = s.CacheDir
	}
	key := strings.Join(s.ModulesPath, "\x00")
	if r.o.noCache {
		dir = ""
	} else {
		key = dir + "\x00" + key
	}
	r.buildMu.Lock()
	defer r.buildMu.Unlock()
	if ix, ok := r.indexes[key]; ok {
		return ix
	}
	ix, err := index.BuildWith(s.ModulesPath, &index.Options{CacheDir: dir})
	if err != nil {
		ix = nil
	}
	r.indexes[key] = ix
	return ix
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parsePerFileIgnores parses a --per-file-ignores value of the form
// GLOB:CODE[,CODE...]. Repeat the flag to cover more than one glob.
func parsePerFileIgnores(v string) (globCodes, error) {
	colon := strings.LastIndexByte(v, ':')
	if colon <= 0 || colon == len(v)-1 {
		return globCodes{}, fmt.Errorf("invalid --per-file-ignores %q: want GLOB:CODE[,CODE...]", v)
	}
	codes := splitCSV(v[colon+1:])
	if len(codes) == 0 {
		return globCodes{}, fmt.Errorf("invalid --per-file-ignores %q: no rule codes", v)
	}
	return globCodes{glob: v[:colon], codes: codes}, nil
}

// collectFiles expands a path into the files to process, honouring the
// include/exclude globs and the gitignore setting resolved for that path.
func collectFiles(path string, s *settings.Settings, forceExclude bool) ([]string, error) {
	return walk.Collect([]string{path}, walk.Options{
		Include:          s.Include,
		Exclude:          s.Exclude,
		RespectGitignore: s.RespectGitignore,
		ForceExclude:     forceExclude || s.ForceExclude,
	})
}

// applyCLIDiscovery folds the discovery flags (--exclude, --extend-exclude,
// --no-respect-gitignore) into s.
func applyCLIDiscovery(o *options, s *settings.Settings) {
	if o.excludeCSV != "" {
		s.Exclude = splitCSV(o.excludeCSV)
	}
	if o.extendExcludeCSV != "" {
		s.Exclude = append(append([]string{}, s.Exclude...), splitCSV(o.extendExcludeCSV)...)
	}
	if o.noGitignore {
		s.RespectGitignore = false
	}
	// Keep the list deterministic so it does not perturb the cache key.
	ex := append([]string{}, s.Exclude...)
	sort.Strings(ex)
	s.Exclude = ex
}

// parallelFiles applies fn to each file concurrently and returns the results in
// input order, so output stays deterministic regardless of scheduling. The
// worker count is bounded by runtime.NumCPU() and the number of files (small
// runs stay sequential).
func parallelFiles[T any](files []string, fn func(i int, file string) T) []T {
	out := make([]T, len(files))
	workers := runtime.NumCPU()
	if workers > len(files) {
		workers = len(files)
	}
	if workers <= 1 {
		for i, f := range files {
			out[i] = fn(i, f)
		}
		return out
	}
	var wg sync.WaitGroup
	work := make(chan int)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				out[i] = fn(i, files[i])
			}
		}()
	}
	for i := range files {
		work <- i
	}
	close(work)
	wg.Wait()
	return out
}

// lintText parses and lints text, optionally applying fixes and authoring
// suppressions. It returns the (possibly rewritten) source, whether it changed,
// and the diagnostics that remain. It performs no file IO itself, so it is safe
// to call from several goroutines.
func lintText(o *options, name, text string, s *settings.Settings, c *cache.Cache, ix *index.Index) (string, bool, []diagnostic.Diagnostic, []string, error) {
	orig := text
	var diags []diagnostic.Diagnostic
	if c != nil {
		if cached, ok := c.Get([]byte(text), s.Key()); ok {
			diags = cached
		} else {
			diags = linter.CheckFile(syntax.Parse(text), name, s, ix)
			_ = c.Put([]byte(text), s.Key(), diags)
		}
	} else {
		diags = linter.CheckFile(syntax.Parse(text), name, s, ix)
	}
	var applied []string
	if o.fix {
		res := fix.Apply(text, s, fix.Options{Unsafe: o.unsafeFixes})
		if res.Source != text {
			text = res.Source
			applied = uniqueStrings(res.Applied)
			diags = linter.CheckFile(syntax.Parse(text), name, s, ix)
		}
	}
	if o.addIgnore && len(diags) > 0 {
		codes := make([]string, 0, len(diags))
		for _, d := range diags {
			codes = append(codes, d.Code)
		}
		if updated := suppress.AddIgnore(text, codes); updated != text {
			text = updated
			diags = linter.CheckFile(syntax.Parse(text), name, s, ix)
		}
	}
	return text, text != orig, diags, applied, nil
}

// uniqueStrings returns the distinct values of v in first-seen order.
func uniqueStrings(v []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range v {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// printFixes summarizes the automatic fixes applied during the run.
func printFixes(w io.Writer, counts map[string]int) {
	codes := make([]string, 0, len(counts))
	total := 0
	for c, n := range counts {
		codes = append(codes, c)
		total += n
	}
	if total == 0 {
		return
	}
	sort.Strings(codes)
	fmt.Fprintf(w, "Fixed %d issue(s):\n", total)
	for _, c := range codes {
		fmt.Fprintf(w, "  %s  %d\n", c, counts[c])
	}
}

// validateFixSelectors rejects --fixable/--unfixable values that do not name a
// known rule, so a typo is a configuration error rather than a silent no-op.
func validateFixSelectors(o *options) error {
	for _, csv := range []string{o.fixableCSV, o.unfixableCSV} {
		for _, sel := range splitCSV(csv) {
			if _, err := rules.ResolveWith([]string{sel}, nil, o.preview); err != nil {
				return fmt.Errorf("--fixable/--unfixable: %w", err)
			}
		}
	}
	return nil
}

func runCheck(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	o, err := parseArgs(args, "check")
	if err != nil {
		fmt.Fprintf(stderr, "ruxx: %v\n", err)
		return ExitAbnormal
	}
	if o.help {
		checkUsage(stdout)
		return ExitClean
	}
	if err := validateFixSelectors(o); err != nil {
		fmt.Fprintf(stderr, "ruxx: %v\n", err)
		return ExitAbnormal
	}
	code := runCheckPass(o, stdin, stdout, stderr)
	if !o.watch {
		return code
	}
	return watch(o, stdout, func() int { return runCheckPass(o, stdin, stdout, stderr) })
}

// runCheckPass performs one lint pass over o.paths.
func runCheckPass(o *options, stdin io.Reader, stdout, stderr io.Writer) int {
	found := false
	abnormal := false

	// The config, lint cache and cross-file index are shared by the parallel
	// workers: the run state builds each at most once and guards the lookups.
	st := newRunState(o)

	var results []report.Result
	counts := map[string]int{}
	fixedCounts := map[string]int{}
	fixedAny := false

	out, closeOut := stdout, func() error { return nil }
	if o.outputFile != "" && o.outputFile != "-" {
		f, err := os.Create(o.outputFile)
		if err != nil {
			fmt.Fprintf(stderr, "ruxx: %v\n", err)
			return ExitAbnormal
		}
		out, closeOut = f, f.Close
	}
	defer closeOut()
	if o.silent && o.outputFile == "" {
		out = io.Discard
	}
	start := time.Now()

	for _, path := range o.paths {
		if path == "-" {
			data, err := io.ReadAll(stdin)
			if err != nil {
				fmt.Fprintf(stderr, "ruxx: %v\n", err)
				abnormal = true
				continue
			}
			name := o.stdinFilename
			if name == "" {
				name = "-"
			}
			s, err := st.settings(name)
			if err != nil {
				fmt.Fprintf(stderr, "ruxx: %v\n", err)
				abnormal = true
				continue
			}
			if o.showSettings {
				printSettings(out, name, s)
				continue
			}
			text, changed, diags, applied, err := lintText(o, name, string(data), s, st.cache(s), st.index(s))
			if err != nil {
				fmt.Fprintf(stderr, "ruxx: %v\n", err)
				abnormal = true
				continue
			}
			if changed {
				if o.diff {
					fmt.Fprint(stdout, report.UnifiedDiff(name, string(data), text))
				} else {
					fmt.Fprint(stdout, text)
				}
			}
			for _, code := range applied {
				fixedCounts[code]++
				fixedAny = true
			}
			if o.fixOnly {
				diags = nil
			}
			if len(diags) > 0 {
				found = true
			}
			for _, d := range diags {
				counts[d.Code]++
			}
			results = append(results, report.Result{Path: name, Source: text, Diagnostics: diags})
			continue
		}
		rootSettings, err := settingsFor(o, path)
		if err != nil {
			fmt.Fprintf(stderr, "ruxx: %v\n", err)
			abnormal = true
			continue
		}
		applyCLIDiscovery(o, rootSettings)
		files, err := collectFiles(path, rootSettings, o.forceExclude)
		if err != nil {
			fmt.Fprintf(stderr, "ruxx: %v\n", err)
			abnormal = true
			continue
		}
		if o.showSettings {
			for _, file := range files {
				s, err := st.settings(file)
				if err != nil {
					fmt.Fprintf(stderr, "ruxx: %v\n", err)
					abnormal = true
					continue
				}
				printSettings(out, file, s)
			}
			continue
		}
		if o.showFiles {
			for _, file := range files {
				fmt.Fprintln(out, file)
			}
			continue
		}
		type checkOutcome struct {
			path    string
			before  string
			text    string
			changed bool
			diags   []diagnostic.Diagnostic
			applied []string
			err     error
		}
		outs := parallelFiles(files, func(_ int, file string) checkOutcome {
			data, err := os.ReadFile(file)
			if err != nil {
				return checkOutcome{path: file, err: err}
			}
			s, err := st.settings(file)
			if err != nil {
				return checkOutcome{path: file, err: err}
			}
			text, changed, diags, applied, err := lintText(o, file, string(data), s, st.cache(s), st.index(s))
			return checkOutcome{path: file, before: string(data), text: text, changed: changed, diags: diags, applied: applied, err: err}
		})
		for _, oc := range outs {
			if oc.err != nil {
				fmt.Fprintf(stderr, "ruxx: %v\n", oc.err)
				abnormal = true
				continue
			}
			if oc.changed {
				if o.diff {
					fmt.Fprint(out, report.UnifiedDiff(oc.path, oc.before, oc.text))
				} else if err := os.WriteFile(oc.path, []byte(oc.text), 0o644); err != nil {
					fmt.Fprintf(stderr, "ruxx: %v\n", err)
					abnormal = true
				}
			}
			for _, code := range oc.applied {
				fixedCounts[code]++
				fixedAny = true
			}
			diags := oc.diags
			if o.fixOnly {
				diags = nil
			}
			if len(diags) > 0 {
				found = true
			}
			for _, d := range diags {
				counts[d.Code]++
			}
			results = append(results, report.Result{Path: oc.path, Source: oc.text, Diagnostics: diags})
		}
	}

	if abnormal {
		return ExitAbnormal
	}
	if o.statistics {
		printStatistics(out, counts)
	} else if err := report.RenderWith(out, results, report.Options{Format: o.outputFormat, Color: wantColor(o.colorMode, out)}); err != nil {
		fmt.Fprintf(stderr, "ruxx: %v\n", err)
		return ExitAbnormal
	}
	if o.showFixes && !o.quiet {
		printFixes(out, fixedCounts)
	}
	if o.verbose {
		findings := 0
		for _, n := range counts {
			findings += n
		}
		fmt.Fprintf(stderr, "[ruxx] checked %d file(s) in %s: %d finding(s)\n",
			len(results), time.Since(start).Round(time.Millisecond), findings)
	}
	if found && !o.exitZero {
		return ExitFindings
	}
	if fixedAny && o.exitNonZeroOnFix {
		return ExitFindings
	}
	return ExitClean
}

func printStatistics(stdout io.Writer, counts map[string]int) {
	codes := make([]string, 0, len(counts))
	for c := range counts {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for _, c := range codes {
		fmt.Fprintf(stdout, "%d\t%s\n", counts[c], c)
	}
}

// wantColor decides whether to emit ANSI colour. mode is "auto" (default),
// "always", or "never"; the environment (NO_COLOR, CLICOLOR, FORCE_COLOR,
// CLICOLOR_FORCE) is consulted in auto mode, then whether w is a terminal.
func wantColor(mode string, w io.Writer) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("CLICOLOR") == "0" {
		return false
	}
	if v, ok := os.LookupEnv("FORCE_COLOR"); ok && v != "0" {
		return true
	}
	if v, ok := os.LookupEnv("CLICOLOR_FORCE"); ok && v != "0" {
		return true
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func runFormat(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	o, err := parseArgs(args, "format")
	if err != nil {
		fmt.Fprintf(stderr, "ruxx: %v\n", err)
		return ExitAbnormal
	}
	if o.help {
		formatUsage(stdout)
		return ExitClean
	}
	code := runFormatPass(o, stdin, stdout, stderr)
	if !o.watch {
		return code
	}
	return watch(o, stdout, func() int { return runFormatPass(o, stdin, stdout, stderr) })
}

// runFormatPass performs one formatting pass over o.paths.
func runFormatPass(o *options, stdin io.Reader, stdout, stderr io.Writer) int {
	wouldChange := false
	changedFiles := 0
	abnormal := false
	st := newRunState(o)

	w, closeOut := stdout, func() error { return nil }
	if o.outputFile != "" && o.outputFile != "-" {
		f, err := os.Create(o.outputFile)
		if err != nil {
			fmt.Fprintf(stderr, "ruxx: %v\n", err)
			return ExitAbnormal
		}
		w, closeOut = f, f.Close
	}
	defer closeOut()
	if o.silent && o.outputFile == "" {
		w = io.Discard
	}
	start := time.Now()

	var formats []report.FormatFile

	for _, path := range o.paths {
		if path == "-" {
			data, err := io.ReadAll(stdin)
			if err != nil {
				fmt.Fprintf(stderr, "ruxx: %v\n", err)
				abnormal = true
				continue
			}
			name := o.stdinFilename
			if name == "" {
				name = "-"
			}
			s, err := st.settings(name)
			if err != nil {
				fmt.Fprintf(stderr, "ruxx: %v\n", err)
				abnormal = true
				continue
			}
			out, err := formatText(string(data), s, o)
			if err != nil {
				fmt.Fprintf(stderr, "ruxx: %v\n", err)
				abnormal = true
				continue
			}
			fmt.Fprint(w, out)
			continue
		}
		rootSettings, err := settingsFor(o, path)
		if err != nil {
			fmt.Fprintf(stderr, "ruxx: %v\n", err)
			abnormal = true
			continue
		}
		applyCLIDiscovery(o, rootSettings)
		files, err := collectFiles(path, rootSettings, o.forceExclude)
		if err != nil {
			fmt.Fprintf(stderr, "ruxx: %v\n", err)
			abnormal = true
			continue
		}
		type formatOutcome struct {
			path    string
			before  string
			after   string
			changed bool
			err     error
		}
		outs := parallelFiles(files, func(_ int, file string) formatOutcome {
			data, err := os.ReadFile(file)
			if err != nil {
				return formatOutcome{path: file, err: err}
			}
			s, err := st.settings(file)
			if err != nil {
				return formatOutcome{path: file, err: err}
			}
			after, err := formatText(string(data), s, o)
			if err != nil {
				return formatOutcome{path: file, err: err}
			}
			return formatOutcome{path: file, before: string(data), after: after, changed: after != string(data)}
		})
		for _, oc := range outs {
			if oc.err != nil {
				fmt.Fprintf(stderr, "ruxx: %v\n", oc.err)
				abnormal = true
				continue
			}
			if !oc.changed {
				continue
			}
			wouldChange = true
			changedFiles++
			if o.check {
				formats = append(formats, report.FormatFile{Path: oc.path, Before: oc.before, After: oc.after})
				continue
			}
			if o.diff {
				fmt.Fprint(w, report.UnifiedDiff(oc.path, oc.before, oc.after))
				continue
			}
			if err := os.WriteFile(oc.path, []byte(oc.after), 0o644); err != nil {
				fmt.Fprintf(stderr, "ruxx: %v\n", err)
				abnormal = true
			}
		}
	}

	if abnormal {
		return ExitAbnormal
	}
	if o.verbose {
		fmt.Fprintf(stderr, "[ruxx] formatted in %s: %d file(s) changed\n",
			time.Since(start).Round(time.Millisecond), changedFiles)
	}
	if o.check {
		if err := report.RenderFormatCheck(w, formats, o.outputFormat); err != nil {
			fmt.Fprintf(stderr, "ruxx: %v\n", err)
			return ExitAbnormal
		}
	}
	if o.check && wouldChange {
		return ExitFindings
	}
	if o.exitNonZeroOnFormat && wouldChange {
		return ExitFindings
	}
	if o.exitZero {
		return ExitClean
	}
	return ExitClean
}

// watchInterval is the polling cadence for --watch. It is a variable so tests
// can shorten it.
var watchInterval = 300 * time.Millisecond

// watch re-runs run whenever a watched XML file changes, until the process is
// interrupted. It prints the changed paths before each re-run.
func watch(o *options, stdout io.Writer, run func() int) int {
	prev := fileSnapshot(o.paths, o)
	for {
		time.Sleep(watchInterval)
		cur := fileSnapshot(o.paths, o)
		changed := snapshotChanged(prev, cur)
		if len(changed) == 0 {
			continue
		}
		for _, f := range changed {
			fmt.Fprintf(stdout, "[ruxx] changed: %s\n", f)
		}
		run()
		prev = cur
	}
}

// fileStamp is the change-relevant metadata of one file.
type fileStamp struct {
	size    int64
	modTime int64
}

// fileSnapshot records the current stamp of every XML file under paths.
func fileSnapshot(paths []string, o *options) map[string]fileStamp {
	snap := map[string]fileStamp{}
	for _, p := range paths {
		if p == "-" {
			continue
		}
		s, err := settingsFor(o, p)
		if err != nil {
			s = settings.Default()
		}
		applyCLIDiscovery(o, s)
		files, err := collectFiles(p, s, o.forceExclude)
		if err != nil {
			continue
		}
		for _, f := range files {
			fi, err := os.Stat(f)
			if err != nil {
				continue
			}
			snap[f] = fileStamp{size: fi.Size(), modTime: fi.ModTime().UnixNano()}
		}
	}
	return snap
}

// snapshotChanged returns the sorted set of paths added, removed, or modified
// between two snapshots.
func snapshotChanged(prev, cur map[string]fileStamp) []string {
	var changed []string
	for p, c := range cur {
		if pc, ok := prev[p]; !ok || pc != c {
			changed = append(changed, p)
		}
	}
	for p := range prev {
		if _, ok := cur[p]; !ok {
			changed = append(changed, p)
		}
	}
	sort.Strings(changed)
	return changed
}

// runServer runs the LSP endpoint over the given stdin/stdout streams.
func runServer(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	o, err := parseArgs(args, "server")
	if err != nil {
		fmt.Fprintf(stderr, "ruxx: %v\n", err)
		return ExitAbnormal
	}
	if o.help {
		serverUsage(stdout)
		return ExitClean
	}

	st := newRunState(o)
	opts := lsp.Options{
		SettingsFor: func(path string) *settings.Settings {
			s, err := st.settings(path)
			if err != nil {
				return settings.Default()
			}
			return s
		},
		IndexFor: st.index,
	}
	return lsp.Run(stdin, stdout, opts)
}

// runClean removes the lint cache for the resolved settings of each path.
func runClean(args []string, stdout, stderr io.Writer) int {
	o, err := parseArgs(args, "clean")
	if err != nil {
		fmt.Fprintf(stderr, "ruxx: %v\n", err)
		return ExitAbnormal
	}
	if o.help {
		cleanUsage(stdout)
		return ExitClean
	}
	abnormal := false
	seen := map[string]bool{}
	for _, path := range o.paths {
		dir := o.cacheDir
		if dir == "" {
			s, err := settingsFor(o, path)
			if err != nil {
				fmt.Fprintf(stderr, "ruxx: %v\n", err)
				abnormal = true
				continue
			}
			dir = s.CacheDir
		}
		if dir == "" {
			dir = settings.DefaultCacheDir
		}
		if seen[dir] {
			continue
		}
		seen[dir] = true
		c, err := cache.Open(dir)
		if err != nil {
			fmt.Fprintf(stderr, "ruxx: %v\n", err)
			abnormal = true
			continue
		}
		if err := c.Clean(); err != nil {
			fmt.Fprintf(stderr, "ruxx: %v\n", err)
			abnormal = true
			continue
		}
		fmt.Fprintf(stdout, "removed %s\n", dir)
	}
	if abnormal {
		return ExitAbnormal
	}
	return ExitClean
}
