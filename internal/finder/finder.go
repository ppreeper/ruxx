// Package finder provides duplicate file detection by content hash.
//
// It works in three phases:
//  1. Walk directories and group files by size (files with unique sizes cannot be duplicates).
//  2. Hash candidate files in parallel using streaming BLAKE2b-256.
//  3. Return groups of files with identical hashes, sorted deterministically.
package finder

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"sync"

	"golang.org/x/crypto/blake2b"
)

// Group represents a set of files with identical content.
type Group struct {
	Paths []string
}

// Options configures duplicate file detection behavior.
type Options struct {
	// SkipEmpty skips zero-length files. Default: false.
	SkipEmpty bool

	// Workers sets the number of parallel hashing goroutines.
	// Defaults to runtime.NumCPU() if <= 0.
	Workers int

	// WarnFunc is called with warning messages for non-fatal errors
	// encountered during scanning (e.g., permission denied).
	// If nil, warnings are silently discarded.
	WarnFunc func(msg string)
}

func (o *Options) warn(format string, args ...any) {
	if o.WarnFunc != nil {
		o.WarnFunc(fmt.Sprintf(format, args...))
	}
}

// Find walks the given directories and returns groups of duplicate files.
// minCount is the minimum number of files in a group (clamped to >= 2).
func Find(dirs []string, minCount int, opts *Options) ([]Group, error) {
	if minCount < 2 {
		minCount = 2
	}
	if opts == nil {
		opts = &Options{}
	}
	if opts.Workers <= 0 {
		opts.Workers = runtime.NumCPU()
	}

	// Phase 1: Walk all directories, group files by size.
	sizeGroups, err := collectBySize(dirs, opts)
	if err != nil {
		return nil, err
	}

	// Phase 2: Collect paths from size groups that have enough candidates
	// to potentially form a duplicate group.
	var toHash []string
	for _, paths := range sizeGroups {
		if len(paths) >= minCount {
			toHash = append(toHash, paths...)
		}
	}

	if len(toHash) == 0 {
		return nil, nil
	}

	// Phase 3: Hash candidate files in parallel.
	hashGroups := hashFiles(toHash, opts)

	// Phase 4: Filter to hash groups meeting minCount, sort deterministically.
	var groups []Group
	for _, paths := range hashGroups {
		if len(paths) >= minCount {
			slices.Sort(paths)
			groups = append(groups, Group{Paths: paths})
		}
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Paths[0] < groups[j].Paths[0]
	})

	return groups, nil
}

// collectBySize walks directories and groups regular file paths by file size.
// Symlinks, directories, and special files (devices, pipes, sockets) are skipped.
func collectBySize(dirs []string, opts *Options) (map[int64][]string, error) {
	sizeGroups := make(map[int64][]string)

	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				opts.warn("walk: %v", err)
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				opts.warn("stat %s: %v", path, err)
				return nil
			}
			if opts.SkipEmpty && info.Size() == 0 {
				return nil
			}
			sizeGroups[info.Size()] = append(sizeGroups[info.Size()], path)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking %s: %w", dir, err)
		}
	}

	return sizeGroups, nil
}

// digestSize is the size of a BLAKE2b-256 digest in bytes.
const digestSize = blake2b.Size256

// hashResult pairs a file path with its computed hash.
type hashResult struct {
	path string
	hash [digestSize]byte
	err  error
}

// hashFiles computes BLAKE2b-256 hashes for all given paths using a worker pool
// and returns a map from hash to list of paths with that hash.
func hashFiles(paths []string, opts *Options) map[[digestSize]byte][]string {
	jobs := make(chan string, opts.Workers*2)
	results := make(chan hashResult, opts.Workers*2)

	// Start hash workers.
	var wg sync.WaitGroup
	for range opts.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				hash, err := hashFile(path)
				results <- hashResult{path: path, hash: hash, err: err}
			}
		}()
	}

	// Collect results in a separate goroutine.
	hashGroups := make(map[[digestSize]byte][]string)
	var collectWg sync.WaitGroup
	collectWg.Add(1)
	go func() {
		defer collectWg.Done()
		for r := range results {
			if r.err != nil {
				opts.warn("hash %s: %v", r.path, r.err)
				continue
			}
			hashGroups[r.hash] = append(hashGroups[r.hash], r.path)
		}
	}()

	// Send jobs.
	for _, path := range paths {
		jobs <- path
	}
	close(jobs)
	wg.Wait()
	close(results)
	collectWg.Wait()

	return hashGroups
}

// hashFile computes the BLAKE2b-256 hash of a file using streaming I/O.
// Unlike reading the entire file into memory, this streams through a fixed buffer.
func hashFile(path string) ([digestSize]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return [digestSize]byte{}, err
	}
	defer f.Close()

	h, err := blake2b.New256(nil)
	if err != nil {
		return [digestSize]byte{}, err
	}
	if _, err := io.Copy(h, f); err != nil {
		return [digestSize]byte{}, err
	}

	var digest [digestSize]byte
	copy(digest[:], h.Sum(nil))
	return digest, nil
}
