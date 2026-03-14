package scanner

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"pike/internal/model"
	"pike/internal/parser"
)

// Scanner walks a directory tree, finds files matching include/exclude globs,
// and parses them for task lines.
type Scanner struct {
	root    string
	include []string               // glob patterns like "**/*.md"
	exclude []string               // glob patterns like "archive/**"
	mtimes  map[string]time.Time   // relPath -> last mtime
	tasks   map[string][]model.Task // relPath -> tasks from that file
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
func (s *Scanner) Scan() ([]model.Task, error) {
	mtimes := make(map[string]time.Time)
	tasks := make(map[string][]model.Task)

	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip problematic entries, continue scanning
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
		mtime := info.ModTime()

		return s.parseFileInto(path, relPath, mtime, mtimes, tasks)
	})
	if err != nil {
		return nil, err
	}

	s.mtimes = mtimes
	s.tasks = tasks

	return s.allTasks(), nil
}

// Refresh does an incremental scan. Only re-parses files whose mtime has
// changed since the last scan. Removes tasks from deleted files.
func (s *Scanner) Refresh() ([]model.Task, error) {
	// Collect the set of files currently on disk that match our patterns
	onDisk := make(map[string]bool)

	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip problematic entries, continue scanning
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if !s.matchesInclude(relPath) {
			return nil
		}
		if s.matchesExclude(relPath) {
			return nil
		}

		onDisk[relPath] = true

		// Check mtime
		info, err := d.Info()
		if err != nil {
			return nil // skip problematic entries
		}
		mtime := info.ModTime()

		prevMtime, seen := s.mtimes[relPath]
		if !seen || mtime.After(prevMtime) {
			// File is new or modified — re-parse
			return s.parseFileInto(path, relPath, mtime, s.mtimes, s.tasks)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Remove tasks for files that no longer exist
	for relPath := range s.tasks {
		if !onDisk[relPath] {
			delete(s.tasks, relPath)
			delete(s.mtimes, relPath)
		}
	}

	return s.allTasks(), nil
}

// parseFileInto reads a file, extracts tasks, and stores the results into the
// provided maps. The modTime parameter is the file's modification time obtained
// during the directory walk, avoiding a TOCTOU race from re-statting the file.
func (s *Scanner) parseFileInto(absPath, relPath string, modTime time.Time, mtimes map[string]time.Time, tasks map[string][]model.Task) error {
	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var fileTasks []model.Task
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), 1024*1024) // allow lines up to 1MB
	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := sc.Text()
		task := parser.ParseLine(line, relPath, lineNum)
		if task != nil {
			fileTasks = append(fileTasks, *task)
		}
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
		matched, _ := doublestar.Match(pattern, relPath)
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
		matched, _ := doublestar.Match(pattern, relPath)
		if matched {
			return true
		}
	}
	return false
}

// allTasks collects all tasks from the map in a stable order (sorted by file path).
func (s *Scanner) allTasks() []model.Task {
	// Get sorted file paths
	paths := make([]string, 0, len(s.tasks))
	for p := range s.tasks {
		paths = append(paths, p)
	}
	slices.Sort(paths)

	var all []model.Task
	for _, p := range paths {
		all = append(all, s.tasks[p]...)
	}
	return all
}
