// Package cache stores per-file lint results keyed by a content hash and a
// settings fingerprint (see settings.Settings.Key). The cache is purely an
// optimization: a miss falls back to a full run, and any read/parse error is
// treated as a miss. Entries live as JSON files under a .ruxx_cache directory.
package cache

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"golang.org/x/crypto/blake2b"

	"github.com/ppreeper/ruxx/internal/diagnostic"
)

// DefaultDir is the cache directory name relative to the working directory.
const DefaultDir = ".ruxx_cache"

// Cache is an on-disk store of lint results.
type Cache struct{ dir string }

// Open returns a cache rooted at dir, creating it if necessary. An empty dir
// uses DefaultDir.
func Open(dir string) (*Cache, error) {
	if dir == "" {
		dir = DefaultDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Cache{dir: dir}, nil
}

// Dir reports the cache directory.
func (c *Cache) Dir() string { return c.dir }

// Get returns the cached diagnostics for content under the given settings key.
// The boolean reports whether an entry was found and read successfully.
func (c *Cache) Get(content []byte, settingsKey string) ([]diagnostic.Diagnostic, bool) {
	data, err := os.ReadFile(c.entryPath(content, settingsKey))
	if err != nil {
		return nil, false
	}
	var diags []diagnostic.Diagnostic
	if err := json.Unmarshal(data, &diags); err != nil {
		return nil, false
	}
	return diags, true
}

// Put stores diagnostics for content under the given settings key. Caching an
// empty result is meaningful: it records that the file was clean.
func (c *Cache) Put(content []byte, settingsKey string, diags []diagnostic.Diagnostic) error {
	data, err := json.Marshal(diags)
	if err != nil {
		return err
	}
	return os.WriteFile(c.entryPath(content, settingsKey), data, 0o644)
}

// Clean removes the cache directory and every entry in it. It is not an error
// for the directory to be absent.
func (c *Cache) Clean() error { return os.RemoveAll(c.dir) }

func (c *Cache) entryPath(content []byte, settingsKey string) string {
	return filepath.Join(c.dir, key(content, settingsKey)+".json")
}

// key derives the entry name from the settings fingerprint and the file
// contents (BLAKE2b-256), so both must match for a hit.
func key(content []byte, settingsKey string) string {
	h, _ := blake2b.New256(nil)
	h.Write([]byte(settingsKey))
	h.Write([]byte{0})
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}
