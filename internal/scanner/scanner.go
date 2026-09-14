// Package scanner walks directories for markdown files with mtime-based caching.
package scanner

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/parser"
)

// TaskSource is the interface for anything that can provide tasks via an
// initial scan and incremental refresh. [Scanner] satisfies this interface.
// Consumers that only need task data (e.g. the TUI) should accept a
// TaskSource to decouple from filesystem details and enable test doubles.
type TaskSource interface {
	Scan(ctx context.Context) ([]model.Task, error)
	Refresh(ctx context.Context) ([]model.Task, error)
}

// maxLineSize is the maximum length of a single markdown line the scanner will
// read. Lines longer than this are split by bufio.Scanner, which may cause
// missed tasks — but 1MB is well beyond any reasonable markdown line.
const maxLineSize = 1 << 20

// Scanner walks a directory tree, finds files matching include/exclude globs,
// and parses them for task lines.
//
// Scan and Refresh are safe for concurrent use: each call builds its result
// against a snapshot of the cached state and publishes it atomically, so
// overlapping callers each get a complete, consistent task list and never
// observe another caller's half-updated cache. Reads of Warnings should go
// through [Scanner.Warns] for the same guarantee.
type Scanner struct {
	root    string
	include []string // glob patterns like "**/*.md"
	exclude []string // glob patterns like "archive/**"

	// mu guards the cached scan state (mtimes, tasks, Warnings). It is only
	// held to snapshot or publish that state, never across file I/O, so
	// concurrent scans do not serialize on disk work.
	mu       sync.Mutex
	mtimes   map[string]time.Time    // relPath -> last mtime
	tasks    map[string][]model.Task // relPath -> tasks from that file
	Warnings []model.Warning         // populated during Scan/Refresh; read via Warns
}

// matchedFile holds info about a file discovered during a directory walk.
type matchedFile struct {
	absPath string
	relPath string
	modTime time.Time
}

// New creates a Scanner for the given root directory with include and exclude
// glob patterns. Returns an error if any glob pattern is invalid.
func New(root string, include, exclude []string) (*Scanner, error) {
	// Validate all glob patterns up front.
	for _, pattern := range include {
		if !doublestar.ValidatePattern(pattern) {
			return nil, fmt.Errorf("invalid include glob pattern: %q", pattern)
		}
	}
	for _, pattern := range exclude {
		if !doublestar.ValidatePattern(pattern) {
			return nil, fmt.Errorf("invalid exclude glob pattern: %q", pattern)
		}
	}
	return &Scanner{
		root:    root,
		include: include,
		exclude: exclude,
		mtimes:  make(map[string]time.Time),
		tasks:   make(map[string][]model.Task),
	}, nil
}

// Scan performs a full scan of all matching files. Returns all tasks found.
func (s *Scanner) Scan(ctx context.Context) ([]model.Task, error) {
	mtimes := make(map[string]time.Time)
	tasks := make(map[string][]model.Task)
	var warnings []model.Warning

	err := s.walkMatching(ctx, func(mf matchedFile) error {
		return parseFileInto(mf.absPath, mf.relPath, mf.modTime, mtimes, tasks, &warnings)
	})
	if err != nil {
		return nil, err
	}

	s.publish(mtimes, tasks, warnings)
	return allTasks(tasks), nil
}

// Refresh does an incremental scan. Only re-parses files whose mtime has
// changed since the last scan. Removes tasks from deleted files.
//
// It works against an independent snapshot of the cached state taken up front
// and publishes the fully-built result atomically at the end, so it is safe to
// call from several goroutines at once (see [Scanner]).
func (s *Scanner) Refresh(ctx context.Context) ([]model.Task, error) {
	// Snapshot the cache into fresh maps this call owns exclusively. The
	// snapshot is shallow, but parseFileInto only ever replaces a file's
	// entry wholesale (never mutates a stored slice in place), so the shared
	// task slices are read-only here.
	mtimes, tasks := s.snapshot()
	var warnings []model.Warning

	// Collect the set of files currently on disk that match our patterns
	onDisk := make(map[string]bool)

	err := s.walkMatching(ctx, func(mf matchedFile) error {
		onDisk[mf.relPath] = true

		prevMtime, seen := mtimes[mf.relPath]
		if !seen || mf.modTime.After(prevMtime) {
			// File is new or modified — re-parse
			return parseFileInto(mf.absPath, mf.relPath, mf.modTime, mtimes, tasks, &warnings)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Remove tasks for files that no longer exist
	for relPath := range tasks {
		if !onDisk[relPath] {
			delete(tasks, relPath)
			delete(mtimes, relPath)
		}
	}

	s.publish(mtimes, tasks, warnings)
	return allTasks(tasks), nil
}

// snapshot returns independent copies of the cached mtimes and tasks maps that
// the caller may mutate freely without affecting the shared state.
func (s *Scanner) snapshot() (map[string]time.Time, map[string][]model.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.mtimes), maps.Clone(s.tasks)
}

// publish atomically replaces the cached scan state with the given result.
func (s *Scanner) publish(mtimes map[string]time.Time, tasks map[string][]model.Task, warnings []model.Warning) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mtimes = mtimes
	s.tasks = tasks
	s.Warnings = warnings
}

// Warns returns the warnings collected by the most recent completed Scan or
// Refresh. It is safe to call concurrently with a scan in progress.
func (s *Scanner) Warns() []model.Warning {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Warnings
}

// walkMatching walks the root directory and calls fn for each file matching
// include/exclude patterns. Respects context cancellation.
func (s *Scanner) walkMatching(ctx context.Context, fn func(matchedFile) error) error {
	return filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip problematic entries, continue scanning
		}
		// Check for cancellation periodically
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		// Normalize to forward slashes for glob matching
		relPath = filepath.ToSlash(relPath)

		if !s.matchesInclude(relPath) {
			return nil
		}
		if s.matchesExclude(relPath) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil // skip problematic entries
		}

		return fn(matchedFile{
			absPath: path,
			relPath: relPath,
			modTime: info.ModTime(),
		})
	})
}

// parseFileInto reads a file, extracts tasks, and stores the results into the
// provided maps, appending any parse warnings to warnings. The modTime parameter
// is the file's modification time obtained during the directory walk, avoiding a
// TOCTOU race from re-statting the file. It writes only into caller-owned state
// (the maps and slice passed in), never the Scanner's shared cache, so it is
// safe to call from concurrent scans.
func parseFileInto(absPath, relPath string, modTime time.Time, mtimes map[string]time.Time, tasks map[string][]model.Task, warnings *[]model.Warning) error {
	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // read-only file; close error not actionable

	var fileTasks []model.Task
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxLineSize)
	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := sc.Text()
		task, warns := parser.ParseLine(line, relPath, lineNum)
		if task != nil {
			fileTasks = append(fileTasks, *task)
		}
		*warnings = append(*warnings, warns...)
	}
	if err := sc.Err(); err != nil {
		return err
	}

	tasks[relPath] = fileTasks
	mtimes[relPath] = modTime
	return nil
}

// matchesInclude returns true if the relPath matches any include pattern.
// Patterns are validated at Scanner creation time, so errors are not expected.
func (s *Scanner) matchesInclude(relPath string) bool {
	for _, pattern := range s.include {
		matched, _ := doublestar.Match(pattern, relPath) //nolint:errcheck // patterns validated at construction time
		if matched {
			return true
		}
	}
	return false
}

// matchesExclude returns true if the relPath matches any exclude pattern.
// Patterns are validated at Scanner creation time, so errors are not expected.
func (s *Scanner) matchesExclude(relPath string) bool {
	for _, pattern := range s.exclude {
		matched, _ := doublestar.Match(pattern, relPath) //nolint:errcheck // patterns validated at construction time
		if matched {
			return true
		}
	}
	return false
}

// allTasks collects all tasks from the given map in a stable order (sorted by
// file path). It operates on the caller's snapshot, not the shared cache.
func allTasks(tasks map[string][]model.Task) []model.Task {
	// Get sorted file paths
	paths := make([]string, 0, len(tasks))
	for p := range tasks {
		paths = append(paths, p)
	}
	slices.Sort(paths)

	var all []model.Task
	for _, p := range paths {
		all = append(all, tasks[p]...)
	}
	return all
}
