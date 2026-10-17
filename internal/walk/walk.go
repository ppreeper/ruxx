// Package walk discovers the files ruxx should analyze.
//
// It expands CLI roots (files and directories) into a sorted, de-duplicated
// list, honouring include/exclude globs and .gitignore files. Mirroring Ruff,
// an explicit file argument is always analyzed unless force-exclude applies,
// while a directory is walked and filtered.
package walk

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Options control discovery.
type Options struct {
	// Include holds globs; a walked file must match at least one. An empty
	// Include means "every file".
	Include []string
	// Exclude holds globs; a matching file is skipped and a matching
	// directory is pruned.
	Exclude []string
	// RespectGitignore enables ignore-file handling: `.ignore` and
	// `.gitignore` files inside the walked root, the repository's
	// `.git/info/exclude`, and the user-level gitignore. Files above the
	// walked root are not read, so a parent repository's rules do not leak in.
	RespectGitignore bool
	// ForceExclude applies Exclude even to explicit file arguments.
	ForceExclude bool
}

// DefaultInclude is the built-in file selection.
var DefaultInclude = []string{"*.xml", "*.xml.template", "*.qweb"}

// DefaultExclude is the built-in prune list.
var DefaultExclude = []string{
	".git", ".hg", ".svn", "node_modules", ".ruxx_cache",
	"build", "dist", "__pycache__", ".venv", "venv",
}

// Collect expands roots into the files to analyze. A root that names a file is
// taken as-is (subject to ForceExclude); a root that names a directory is
// walked. Non-existent roots are an error.
func Collect(roots []string, opts Options) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}

	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if opts.ForceExclude && matchAny(opts.Exclude, root, filepath.Dir(root)) {
				continue
			}
			add(root)
			continue
		}

		w := &walker{opts: opts, root: filepath.Clean(root), cache: map[string]dirIgnores{}}
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			if d.IsDir() {
				if path == root {
					return nil
				}
				if filepath.Base(path) == ".git" {
					return fs.SkipDir
				}
				if matchAny(opts.Exclude, path, root) {
					return fs.SkipDir
				}
				if opts.RespectGitignore && w.ignored(path, true) {
					return fs.SkipDir
				}
				return nil
			}
			if len(opts.Include) > 0 && !matchAny(opts.Include, path, root) {
				return nil
			}
			if matchAny(opts.Exclude, path, root) {
				return nil
			}
			if opts.RespectGitignore && w.ignored(path, false) {
				return nil
			}
			add(path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Strings(out)
	return out, nil
}

// --- glob matching ----------------------------------------------------------

func matchAny(patterns []string, path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	base := filepath.Base(path)
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if matchPattern(p, rel, base) {
			return true
		}
	}
	return false
}

func matchPattern(pattern, rel, base string) bool {
	p := strings.TrimSuffix(pattern, "/")
	if p == "" {
		return false
	}
	if !strings.Contains(p, "/") {
		// A bare name matches any path component (file or directory).
		for _, seg := range strings.Split(rel, "/") {
			if ok, err := filepath.Match(p, seg); err == nil && ok {
				return true
			}
		}
		return false
	}
	if ok, err := filepath.Match(p, rel); err == nil && ok {
		return true
	}
	if strings.HasPrefix(p, "**/") {
		if ok, err := filepath.Match(p[3:], rel); err == nil && ok {
			return true
		}
	}
	if strings.Contains(p, "**") {
		if re, err := compileGlob(p); err == nil && re.MatchString(rel) {
			return true
		}
	}
	return false
}

// globBody translates a glob (supporting `**`) into a regex body.
func globBody(pattern string) string {
	var b strings.Builder
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '[':
			end := strings.IndexByte(pattern[i:], ']')
			if end > 0 {
				b.WriteString(pattern[i : i+end+1])
				i += end
			} else {
				b.WriteString("\\[")
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return b.String()
}

func compileGlob(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile("^" + globBody(pattern) + "$")
}

// --- .gitignore -------------------------------------------------------------

type giRule struct {
	re      *regexp.Regexp
	negate  bool
	dirOnly bool
}

type gitignoreFile struct {
	base  string
	rules []giRule
}

// dirIgnores holds the ignore files that live in one directory. A `.ignore`
// file takes precedence over a `.gitignore` in the same directory.
type dirIgnores struct {
	ignore    *gitignoreFile
	gitignore *gitignoreFile
}

type walker struct {
	opts  Options
	root  string
	cache map[string]dirIgnores

	// Repository (.git/info/exclude) and user-level ignore files are loaded at
	// most once per walk.
	repoSet, globalsSet bool
	repo, globals       *gitignoreFile
}

// ignored reports whether path is ignored. Inside the walked root a `.ignore`
// file beats a `.gitignore` in the same directory, and deeper files beat
// shallower ones; the repository's `.git/info/exclude` and the user-level
// gitignore apply last.
func (w *walker) ignored(path string, isDir bool) bool {
	dir := filepath.Dir(path)
	for {
		if di, ok := w.ignoresFor(dir); ok {
			if matched, ig := matchFile(di.ignore, path, isDir); matched {
				return ig
			}
			if matched, ig := matchFile(di.gitignore, path, isDir); matched {
				return ig
			}
		}
		if filepath.Clean(dir) == w.root {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir || len(parent) < len(w.root) {
			break
		}
		dir = parent
	}
	if matched, ig := matchFile(w.repoExcludes(), path, isDir); matched {
		return ig
	}
	if matched, ig := matchFile(w.globalIgnores(), path, isDir); matched {
		return ig
	}
	return false
}

// matchFile reports whether gi has an opinion about path and whether that
// opinion is "ignored". A nil file never matches. An empty base means the rules
// are matched against the absolute path, which is how the user-level gitignore
// is applied (its rules are not anchored to any particular directory).
func matchFile(gi *gitignoreFile, path string, isDir bool) (matched, ignored bool) {
	if gi == nil {
		return false, false
	}
	rel := filepath.ToSlash(path)
	if gi.base != "" {
		r, err := filepath.Rel(gi.base, path)
		if err != nil {
			return false, false
		}
		rel = filepath.ToSlash(r)
	}
	return gi.match(rel, isDir)
}

// ignoresFor returns the ignore files in dir, loading them on first use. The
// second result reports whether either file exists.
func (w *walker) ignoresFor(dir string) (dirIgnores, bool) {
	if di, ok := w.cache[dir]; ok {
		return di, di.ignore != nil || di.gitignore != nil
	}
	di := dirIgnores{
		ignore:    loadIgnoreFile(filepath.Join(dir, ".ignore"), dir),
		gitignore: loadIgnoreFile(filepath.Join(dir, ".gitignore"), dir),
	}
	w.cache[dir] = di
	return di, di.ignore != nil || di.gitignore != nil
}

// repoExcludes returns the repository's .git/info/exclude, looked up from the
// walked root upwards. It applies to every path inside the repository.
func (w *walker) repoExcludes() *gitignoreFile {
	if w.repoSet {
		return w.repo
	}
	w.repoSet = true
	for dir := w.root; ; {
		gitPath := filepath.Join(dir, ".git")
		if fi, err := os.Stat(gitPath); err == nil {
			gitDir := gitPath
			if !fi.IsDir() {
				gitDir = gitdirFromFile(gitPath, dir)
			}
			if gitDir != "" {
				w.repo = loadIgnoreFile(filepath.Join(gitDir, "info", "exclude"), dir)
			}
			return w.repo
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return w.repo
		}
		dir = parent
	}
}

// gitdirFromFile resolves the "gitdir: <path>" line of a .git file (worktrees
// and submodules) relative to the directory that holds it.
func gitdirFromFile(path, dir string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(data))
	line = strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
	if line == "" {
		return ""
	}
	if !filepath.IsAbs(line) {
		line = filepath.Join(dir, line)
	}
	return filepath.Clean(line)
}

// globalIgnores returns the user-level gitignore ($XDG_CONFIG_HOME/git/ignore,
// defaulting to ~/.config/git/ignore). Its rules are matched from the
// filesystem root, so an unanchored pattern matches at any depth.
func (w *walker) globalIgnores() *gitignoreFile {
	if w.globalsSet {
		return w.globals
	}
	w.globalsSet = true
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		dir = filepath.Join(home, ".config")
	}
	w.globals = loadIgnoreFile(filepath.Join(dir, "git", "ignore"), "")
	return w.globals
}

// loadIgnoreFile reads an ignore file; an absent or unreadable file yields nil.
func loadIgnoreFile(path, base string) *gitignoreFile {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseIgnore(data, base)
}

// parseIgnore parses gitignore syntax into rules matched relative to base.
func parseIgnore(data []byte, base string) *gitignoreFile {
	gi := &gitignoreFile{base: base}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		negate := false
		if strings.HasPrefix(line, "!") {
			negate = true
			line = line[1:]
		}
		if strings.HasPrefix(line, "\\#") || strings.HasPrefix(line, "\\!") {
			line = line[1:]
		}
		line = strings.TrimRight(line, " ")
		dirOnly := strings.HasSuffix(line, "/")
		line = strings.TrimSuffix(line, "/")
		if line == "" {
			continue
		}
		anchored := strings.Contains(line, "/") || strings.HasPrefix(line, "/")
		line = strings.TrimPrefix(line, "/")
		body := globBody(line)
		pattern := "(?:^|/)" + body + "(?:/.*)?$"
		if anchored {
			pattern = "^" + body + "(?:/.*)?$"
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		gi.rules = append(gi.rules, giRule{re: re, negate: negate, dirOnly: dirOnly})
	}
	return gi
}

// match reports whether rel matches a rule and whether that rule ignores it.
// The last matching rule wins.
func (gi *gitignoreFile) match(rel string, isDir bool) (matched, ignored bool) {
	for _, r := range gi.rules {
		if r.dirOnly && !isDir {
			continue
		}
		if r.re.MatchString(rel) {
			matched = true
			ignored = !r.negate
		}
	}
	return matched, ignored
}
