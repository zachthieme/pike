package scanner

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"tasks/internal/model"
	"tasks/internal/parser"
)

// Scanner walks a directory tree, finds files matching include/exclude globs,
// and parses them for task lines.
type Scanner struct {
	root    string
	include []string            // glob patterns like "**/*.md"
	exclude []string            // glob patterns like "archive/**"
	mtimes  map[string]time.Time // relPath -> last mtime
	tasks   map[string][]model.Task // relPath -> tasks from that file
}

// New creates a Scanner for the given root directory with include and exclude
// glob patterns.
func New(root string, include, exclude []string) *Scanner {
	return &Scanner{
		root:    root,
		include: include,
		exclude: exclude,
		mtimes:  make(map[string]time.Time),
		tasks:   make(map[string][]model.Task),
	}
}

// Scan performs a full scan of all matching files. Returns all tasks found.
func (s *Scanner) Scan() ([]model.Task, error) {
	s.mtimes = make(map[string]time.Time)
	s.tasks = make(map[string][]model.Task)

	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
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

		return s.parseFile(path, relPath)
	})
	if err != nil {
		return nil, err
	}

	return s.allTasks(), nil
}

// Refresh does an incremental scan. Only re-parses files whose mtime has
// changed since the last scan. Removes tasks from deleted files.
func (s *Scanner) Refresh() ([]model.Task, error) {
	// Collect the set of files currently on disk that match our patterns
	onDisk := make(map[string]bool)

	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
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
			return err
		}
		mtime := info.ModTime()

		prevMtime, seen := s.mtimes[relPath]
		if !seen || mtime.After(prevMtime) {
			// File is new or modified — re-parse
			return s.parseFile(path, relPath)
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

// parseFile reads a file, extracts tasks, and stores the results.
func (s *Scanner) parseFile(absPath, relPath string) error {
	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	var tasks []model.Task
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		task := parser.ParseLine(line, relPath, lineNum)
		if task != nil {
			tasks = append(tasks, *task)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	s.tasks[relPath] = tasks
	s.mtimes[relPath] = info.ModTime()
	return nil
}

// matchesInclude returns true if the relPath matches any include pattern.
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
	sort.Strings(paths)

	var all []model.Task
	for _, p := range paths {
		all = append(all, s.tasks[p]...)
	}
	return all
}
