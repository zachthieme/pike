// Package toggle performs atomic file mutations for task completion and visibility.
package toggle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Sentinel errors for programmatic handling.
var (
	ErrStaleData      = errors.New("stale data: file changed externally")
	ErrLineOutOfRange = errors.New("line number out of range")
)

var completedTagRe = regexp.MustCompile(`\s*@completed(\([^)]*\))?(?:\s|$)`)
var hiddenTagRe = regexp.MustCompile(`\s*@hidden(?:\s|$)`)

// fileMutexMap provides type-safe per-file locking to prevent concurrent
// mutations from racing. Each file path gets its own mutex so that operations
// on different files proceed in parallel.
type fileMutexMap struct {
	mu sync.Mutex
	m  map[string]*sync.Mutex
}

func (fm *fileMutexMap) lock(path string) *sync.Mutex {
	fm.mu.Lock()
	fileMu, ok := fm.m[path]
	if !ok {
		fileMu = &sync.Mutex{}
		fm.m[path] = fileMu
	}
	fm.mu.Unlock()
	fileMu.Lock()
	return fileMu
}

var fileLocks = fileMutexMap{m: make(map[string]*sync.Mutex)}

// mutateFile reads a file, calls mutate on the target line, verifies the file
// wasn't modified externally, and atomically writes the result. This is the
// shared plumbing for Complete, Uncomplete, and ToggleHidden.
func mutateFile(ctx context.Context, filePath string, line int, mutate func(string) (string, error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mu := fileLocks.lock(filePath)
	defer mu.Unlock()

	lines, err := readLines(filePath)
	if err != nil {
		return err
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if line < 1 || line > len(lines) {
		return fmt.Errorf("%w: line %d (file has %d lines)", ErrLineOutOfRange, line, len(lines))
	}

	idx := line - 1
	originalLine := lines[idx]
	newLine, err := mutate(originalLine)
	if err != nil {
		return err
	}
	lines[idx] = newLine

	if err := verifyUnmodified(filePath, line, originalLine); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return writeLines(filePath, lines, info.Mode())
}

// Complete marks an open checkbox task as completed by modifying the source file.
// Replaces - [ ] with - [x] and appends @completed(YYYY-MM-DD).
// Returns an error if the line doesn't contain - [ ] (stale data).
func Complete(ctx context.Context, filePath string, line int, date time.Time) error {
	return mutateFile(ctx, filePath, line, func(l string) (string, error) {
		if !strings.Contains(l, "- [ ]") {
			return "", fmt.Errorf("%w: line %d does not contain '- [ ]'", ErrStaleData, line)
		}
		l = strings.Replace(l, "- [ ]", "- [x]", 1)
		l += fmt.Sprintf(" @completed(%s)", date.Format("2006-01-02"))
		return l, nil
	})
}

// Uncomplete marks a completed checkbox task as open by modifying the source file.
// Replaces - [x]/- [X] with - [ ] and removes @completed(...) tag.
// Returns an error if the line doesn't contain - [x] or - [X] (stale data).
func Uncomplete(ctx context.Context, filePath string, line int) error {
	return mutateFile(ctx, filePath, line, func(l string) (string, error) {
		if !strings.Contains(l, "- [x]") && !strings.Contains(l, "- [X]") {
			return "", fmt.Errorf("%w: line %d does not contain '- [x]'", ErrStaleData, line)
		}
		l = strings.Replace(l, "- [x]", "- [ ]", 1)
		l = strings.Replace(l, "- [X]", "- [ ]", 1)
		l = completedTagRe.ReplaceAllStringFunc(l, func(match string) string {
			if strings.HasSuffix(match, " ") || strings.HasSuffix(match, "\t") {
				return " "
			}
			return ""
		})
		l = strings.TrimRight(l, " \t")
		return l, nil
	})
}

// ToggleHidden adds @hidden to a task line if absent, or removes it if present.
func ToggleHidden(ctx context.Context, filePath string, line int) error {
	return mutateFile(ctx, filePath, line, func(l string) (string, error) {
		if strings.Contains(l, "@hidden") {
			l = hiddenTagRe.ReplaceAllStringFunc(l, func(match string) string {
				if strings.HasSuffix(match, " ") || strings.HasSuffix(match, "\t") {
					return " "
				}
				return ""
			})
			l = strings.TrimRight(l, " \t")
		} else {
			l += " @hidden"
		}
		return l, nil
	})
}

// verifyUnmodified re-reads the file and checks that the target line hasn't
// been modified by an external process since we first read it. This narrows
// the TOCTOU window to just the time between our two reads.
func verifyUnmodified(path string, lineNum int, originalLine string) error {
	lines, err := readLines(path)
	if err != nil {
		return fmt.Errorf("re-read for verification: %w", err)
	}
	if lineNum < 1 || lineNum > len(lines) {
		return fmt.Errorf("%w: file changed externally (line count changed)", ErrStaleData)
	}
	if lines[lineNum-1] != originalLine {
		return fmt.Errorf("%w: line %d modified externally between read and write", ErrStaleData, lineNum)
	}
	return nil
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := strings.TrimSuffix(string(data), "\n")
	return strings.Split(s, "\n"), nil
}

// writeLines writes lines atomically using write-to-temp + rename.
func writeLines(path string, lines []string, perm os.FileMode) error {
	content := strings.Join(lines, "\n") + "\n"
	tmp := path + ".pike-tmp"
	if err := os.WriteFile(tmp, []byte(content), perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp) // best-effort cleanup; rename error takes precedence
		return err
	}
	return nil
}
