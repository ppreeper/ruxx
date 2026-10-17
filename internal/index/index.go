// Package index builds a lightweight, opt-in cross-file Odoo project index.
//
// It records, per module directory: the xmlids the module defines (and the
// files defining them), the data files its manifest lists, and whether an
// access-control csv exists. It powers the cross-file rules (O016, O018,
// QW012, SEC002, SEC003) and is only consulted when [odoo].modules-path is
// configured. Without an index those rules stay silent, so a plain
// single-file lint never produces cross-file false positives.
package index

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/ppreeper/ruxx/internal/syntax"
)

// Module is the indexed facts about one Odoo module directory.
type Module struct {
	Name        string
	Dir         string
	HasManifest bool
	// Data holds the manifest's data-file entries (verbatim, relative paths).
	Data []string
	// Defs maps a fully-qualified xmlid to every file defining it.
	Defs map[string][]string
	// Groups holds the fully-qualified xmlids of res.groups records defined
	// by this module.
	Groups map[string]bool
	// AccessCSV is the path to security/ir.model.access.csv when it exists.
	AccessCSV string
}

// Index is the built project index.
type Index struct {
	modules map[string]*Module
}

// Module returns the module with the given name.
func (ix *Index) Module(name string) (*Module, bool) {
	m, ok := ix.modules[name]
	return m, ok
}

// HasModule reports whether the module name is present in the index.
func (ix *Index) HasModule(name string) bool {
	_, ok := ix.modules[name]
	return ok
}

// HasXMLID reports whether the fully-qualified xmlid is defined anywhere.
func (ix *Index) HasXMLID(id string) bool {
	m, ok := ix.Module(moduleOfXMLID(id))
	if !ok {
		return false
	}
	_, ok = m.Defs[id]
	return ok
}

// Modules returns every indexed module, sorted by name.
func (ix *Index) Modules() []*Module {
	out := make([]*Module, 0, len(ix.modules))
	for _, m := range ix.modules {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// moduleOfXMLID splits "module.name" into "module" (or the whole string when
// there is no dot).
func moduleOfXMLID(id string) string {
	if i := strings.IndexByte(id, '.'); i >= 0 {
		return id[:i]
	}
	return id
}

var manifestNames = []string{"__manifest__.py", "__openerp__.py"}

// Options configures index construction.
type Options struct {
	// CacheDir, when non-empty, enables a persistent cache of parsed module
	// facts keyed by every module file's size and modification time. A module
	// whose files are unchanged is reused verbatim instead of re-parsed, so
	// repeated builds (watch mode, editor sessions) skip the expensive part.
	// The cache lives at <CacheDir>/index.json and is removed by `ruxx clean`.
	CacheDir string
	// Workers caps how many modules are parsed in parallel. Zero means
	// runtime.NumCPU(); one forces a sequential build.
	Workers int
}

// Build walks each root, treating every immediate subdirectory as a module.
// Roots that do not exist are skipped so a stale config never hard-fails.
func Build(roots []string) (*Index, error) { return BuildWith(roots, nil) }

// BuildWith is Build with an optional persistent cache and worker cap.
func BuildWith(roots []string, opts *Options) (*Index, error) {
	cacheDir := ""
	var prev map[string]cachedModule
	workers := 0
	if opts != nil {
		cacheDir = opts.CacheDir
		workers = opts.Workers
		prev = loadCache(cacheDir, roots)
	}
	ix, cur, err := build(roots, prev, workers)
	if err != nil {
		return nil, err
	}
	if cacheDir != "" {
		_ = saveCache(cacheDir, roots, cur)
	}
	return ix, nil
}

// build indexes every module under roots, reusing prev entries whose files are
// unchanged. It returns the index plus the cache entries describing this build.
func build(roots []string, prev map[string]cachedModule, workers int) (*Index, map[string]cachedModule, error) {
	type job struct{ dir, name string }
	var jobs []job
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			jobs = append(jobs, job{dir: filepath.Join(root, e.Name()), name: e.Name()})
		}
	}
	mods := make([]*Module, len(jobs))
	entries := make([]cachedModule, len(jobs))
	parallel(len(jobs), workers, func(i int) {
		mods[i], entries[i] = buildModule(jobs[i].dir, jobs[i].name, prev[jobs[i].dir])
	})
	ix := &Index{modules: make(map[string]*Module, len(jobs))}
	cur := make(map[string]cachedModule, len(jobs))
	for i := range mods {
		ix.modules[mods[i].Name] = mods[i]
		cur[mods[i].Dir] = entries[i]
	}
	return ix, cur, nil
}

// buildModule indexes one module directory, reusing prev when every file it
// depends on still has the same size and modification time.
func buildModule(dir, name string, prev cachedModule) (*Module, cachedModule) {
	mod := &Module{Name: name, Dir: dir, Defs: map[string][]string{}, Groups: map[string]bool{}}

	files := map[string]string{}
	manifestPath := ""
	for _, mn := range manifestNames {
		p := filepath.Join(dir, mn)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			manifestPath = p
			files[p] = stamp(info)
			break
		}
	}
	accessCSV := ""
	if p := filepath.Join(dir, "security", "ir.model.access.csv"); fileExists(p) {
		accessCSV = p
		if info, err := os.Stat(p); err == nil {
			files[p] = stamp(info)
		}
	}
	var xmlFiles []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "__pycache__", ".git", "node_modules":
				if p != dir {
					return fs.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(p), ".xml") {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		xmlFiles = append(xmlFiles, p)
		files[p] = stamp(info)
		return nil
	})

	mod.HasManifest = manifestPath != ""
	mod.AccessCSV = accessCSV
	out := cachedModule{
		Dir:         dir,
		Name:        name,
		HasManifest: mod.HasManifest,
		AccessCSV:   accessCSV,
		Files:       files,
	}

	if sameFiles(prev.Files, files) {
		mod.Data, mod.Defs, mod.Groups = prev.Data, prev.Defs, prev.Groups
		if mod.Defs == nil {
			mod.Defs = map[string][]string{}
		}
		if mod.Groups == nil {
			mod.Groups = map[string]bool{}
		}
		out.Data, out.Defs, out.Groups = mod.Data, mod.Defs, mod.Groups
		return mod, out
	}

	if manifestPath != "" {
		if b, err := os.ReadFile(manifestPath); err == nil {
			mod.Data = parseManifestData(string(b))
		}
	}
	// Sorted so the per-xmlid file lists (and therefore rule messages) are
	// deterministic regardless of the filesystem's walk order.
	sort.Strings(xmlFiles)
	for _, p := range xmlFiles {
		collect(mod, p)
	}
	out.Data, out.Defs, out.Groups = mod.Data, mod.Defs, mod.Groups
	return mod, out
}

// stamp fingerprints a file by size and modification time.
func stamp(info fs.FileInfo) string {
	return strconv.FormatInt(info.Size(), 10) + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
}

// sameFiles reports whether two file→stamp maps describe the same file set.
func sameFiles(a, b map[string]string) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// parallel runs fn(i) for i in [0,n) on at most workers goroutines. Results are
// addressed by index, so the merged output is deterministic.
func parallel(n, workers int, fn func(int)) {
	if n == 0 {
		return
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > n {
		workers = n
	}
	if workers <= 1 {
		for i := 0; i < n; i++ {
			fn(i)
		}
		return
	}
	var wg sync.WaitGroup
	ch := make(chan int)
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := range ch {
				fn(i)
			}
		}()
	}
	for i := 0; i < n; i++ {
		ch <- i
	}
	close(ch)
	wg.Wait()
}

// cacheVersion bumps whenever cachedModule changes shape.
const cacheVersion = 1

// cachedModule is one module's persisted facts plus the file stamps they were
// derived from.
type cachedModule struct {
	Dir         string              `json:"dir"`
	Name        string              `json:"name"`
	HasManifest bool                `json:"hasManifest,omitempty"`
	Data        []string            `json:"data,omitempty"`
	Defs        map[string][]string `json:"defs,omitempty"`
	Groups      map[string]bool     `json:"groups,omitempty"`
	AccessCSV   string              `json:"accessCSV,omitempty"`
	Files       map[string]string   `json:"files,omitempty"`
}

type cacheFile struct {
	Version int                     `json:"version"`
	Roots   []string                `json:"roots"`
	Modules map[string]cachedModule `json:"modules"`
}

func cachePath(dir string) string { return filepath.Join(dir, "index.json") }

// loadCache returns the cached modules for this exact root set, or nil.
func loadCache(dir string, roots []string) map[string]cachedModule {
	if dir == "" {
		return nil
	}
	b, err := os.ReadFile(cachePath(dir))
	if err != nil {
		return nil
	}
	var cf cacheFile
	if json.Unmarshal(b, &cf) != nil || cf.Version != cacheVersion {
		return nil
	}
	if len(cf.Roots) != len(roots) {
		return nil
	}
	for i := range roots {
		if cf.Roots[i] != roots[i] {
			return nil
		}
	}
	return cf.Modules
}

// saveCache writes the cache; a failure is never fatal to the caller.
func saveCache(dir string, roots []string, mods map[string]cachedModule) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(cacheFile{Version: cacheVersion, Roots: roots, Modules: mods})
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(dir), b, 0o644)
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// collect parses one XML file and records the xmlids it defines.
func collect(mod *Module, path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	tree := syntax.Parse(string(b))
	walk(tree.Root, nil, func(n, parent *syntax.Node) {
		if n.Name == "t" && parent != nil && parent.Name == "templates" {
			if name, ok := n.Attr("t-name"); ok && name != "" {
				mod.Defs[qualify(mod.Name, name)] = append(mod.Defs[qualify(mod.Name, name)], path)
				return
			}
		}
		if !isIdDefiner(parent) {
			return
		}
		id, ok := n.Attr("id")
		if !ok || id == "" {
			return
		}
		q := qualify(mod.Name, id)
		mod.Defs[q] = append(mod.Defs[q], path)
		if mv, _ := n.Attr("model"); mv == "res.groups" {
			mod.Groups[q] = true
		}
	})
}

func qualify(module, id string) string {
	if strings.Contains(id, ".") {
		return id
	}
	return module + "." + id
}

func isIdDefiner(parent *syntax.Node) bool {
	if parent == nil {
		return true
	}
	switch parent.Name {
	case "odoo", "data", "openerp":
		return true
	}
	return false
}

var (
	dataBlockRE = regexp.MustCompile(`(?s)["']data["']\s*:\s*\[(.*?)\]`)
	quoteRE     = regexp.MustCompile(`["']([^"']+)["']`)
)

// parseManifestData extracts the quoted entries of the manifest's data list.
func parseManifestData(src string) []string {
	var out []string
	for _, m := range dataBlockRE.FindAllStringSubmatch(src, -1) {
		for _, q := range quoteRE.FindAllStringSubmatch(m[1], -1) {
			out = append(out, q[1])
		}
	}
	return out
}

// walkElements is a minimal element walker (mirrors internal/rules).
func walkElements(n *syntax.Node, fn func(*syntax.Node)) {
	if n.Kind == syntax.KindElement || n.Kind == syntax.KindDocument {
		if n.Kind == syntax.KindElement {
			fn(n)
		}
		for _, c := range n.Children {
			walkElements(c, fn)
		}
	}
}

func walk(n *syntax.Node, parent *syntax.Node, fn func(*syntax.Node, *syntax.Node)) {
	if n.Kind == syntax.KindDocument {
		for _, c := range n.Children {
			walk(c, parent, fn)
		}
		return
	}
	if n.Kind != syntax.KindElement {
		return
	}
	fn(n, parent)
	for _, c := range n.Children {
		if c.Kind == syntax.KindElement {
			walk(c, n, fn)
		}
	}
}
