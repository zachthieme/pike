// Package toggle performs atomic file mutations for task completion and visibility.
package toggle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Sentinel errors for programmatic handling.
var (
	ErrStaleData      = errors.New("stale data: file changed externally")
	ErrLineOutOfRange = errors.New("line number out of range")
	ErrAmbiguousTag   = errors.New("ambiguous tag: line carries more than one")
)

var completedTagRe = regexp.MustCompile(`\s*@completed(\([^)]*\))?(?:\s|$)`)
var hiddenTagRe = regexp.MustCompile(`\s*@hidden(?:\s|$)`)

// tagTokenRe matches one @name or @name(value) token, mirroring the parser's
// tag grammar so text rewrites keep exactly the tokens pike recognises.
var tagTokenRe = regexp.MustCompile(`@\w+(?:\([^)]*\))?`)

// taskPrefixRe matches a task line's leading marker: indentation, the bullet,
// and an optional checkbox. The body (text and tags) is everything after it.
var taskPrefixRe = regexp.MustCompile(`^\s*- (?:\[[ xX]\] )?`)

// fileMutexMap provides type-safe per-file locking to prevent concurrent
// mutations from racing. Each file path gets its own mutex so that operations
// on different files proceed in parallel.
type fileMutexMap struct {
	mu sync.Mutex
	m  map[string]*sync.Mutex
}

func newFileMutexMap() fileMutexMap {
	return fileMutexMap{m: make(map[string]*sync.Mutex)}
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

// Toggler performs atomic file mutations with its own per-file lock map.
// Use [NewToggler] to create an isolated instance (useful for testing),
// or use the package-level [Complete], [Uncomplete], and [ToggleHidden]
// functions which share a default instance.
type Toggler struct {
	locks fileMutexMap
}

// NewToggler creates a Toggler with its own lock state, enabling isolated
// concurrent usage (e.g. parallel test cases).
func NewToggler() *Toggler {
	return &Toggler{locks: newFileMutexMap()}
}

// defaultToggler is the package-level instance used by the convenience functions.
var defaultToggler = NewToggler()

// mutateFile reads a file, calls mutate on the target line, verifies the file
// wasn't modified externally, and atomically writes the result. This is the
// shared plumbing for Complete, Uncomplete, and ToggleHidden.
func (t *Toggler) mutateFile(ctx context.Context, filePath string, line int, mutate func(string) (string, error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mu := t.locks.lock(filePath)
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
func (t *Toggler) Complete(ctx context.Context, filePath string, line int, date time.Time) error {
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
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
func (t *Toggler) Uncomplete(ctx context.Context, filePath string, line int) error {
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
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
func (t *Toggler) ToggleHidden(ctx context.Context, filePath string, line int) error {
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
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

// Complete marks an open checkbox task as completed using the default Toggler.
func Complete(ctx context.Context, filePath string, line int, date time.Time) error {
	return defaultToggler.Complete(ctx, filePath, line, date)
}

// Uncomplete marks a completed checkbox task as open using the default Toggler.
func Uncomplete(ctx context.Context, filePath string, line int) error {
	return defaultToggler.Uncomplete(ctx, filePath, line)
}

// ToggleHidden toggles @hidden on a task using the default Toggler.
func ToggleHidden(ctx context.Context, filePath string, line int) error {
	return defaultToggler.ToggleHidden(ctx, filePath, line)
}

// AppendTag appends a tag (e.g. "@hey(h1)") to a task line, leaving the rest
// of the line byte-for-byte unchanged. wantText is the task text the caller
// observed when it scanned the line; if the current line no longer contains it,
// the line changed since the scan and AppendTag returns [ErrStaleData] without
// writing. This is the same stale-line guard Complete and Uncomplete rely on.
func AppendTag(ctx context.Context, filePath string, line int, wantText, tag string) error {
	return defaultToggler.AppendTag(ctx, filePath, line, wantText, tag)
}

// AppendTag appends a tag to a task line using this Toggler's lock state.
func (t *Toggler) AppendTag(ctx context.Context, filePath string, line int, wantText, tag string) error {
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
		if !strings.Contains(l, wantText) {
			return "", fmt.Errorf("%w: line %d no longer contains %q", ErrStaleData, line, wantText)
		}
		return l + " " + tag, nil
	})
}

// SetText rewrites a task line's text to newText while keeping its tags,
// re-appending them at the end in their original order. The leading marker
// (indentation, bullet, and checkbox state) is left untouched. wantText is the
// task text the caller observed when it scanned the line; if the current line
// no longer contains it, the line changed since the scan and SetText returns
// [ErrStaleData] without writing, the same stale-line guard AppendTag uses.
func SetText(ctx context.Context, filePath string, line int, wantText, newText string) error {
	return defaultToggler.SetText(ctx, filePath, line, wantText, newText)
}

// SetText rewrites a task line's text keeping its tags, using this Toggler's
// lock state.
func (t *Toggler) SetText(ctx context.Context, filePath string, line int, wantText, newText string) error {
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
		if !strings.Contains(l, wantText) {
			return "", fmt.Errorf("%w: line %d no longer contains %q", ErrStaleData, line, wantText)
		}
		prefix := taskPrefixRe.FindString(l)
		body := l[len(prefix):]
		newBody := newText
		if tags := tagTokenRe.FindAllString(body, -1); len(tags) > 0 {
			newBody = newText + " " + strings.Join(tags, " ")
		}
		return prefix + newBody, nil
	})
}

// RemoveTag strips a single @name (or @name(value)) token from a task line,
// collapsing the surrounding whitespace and leaving the rest of the line
// unchanged. It is the supported way to un-link a Task from HEY. A line with no
// such tag is treated as stale ([ErrStaleData]) and a line carrying two or more
// is ambiguous ([ErrAmbiguousTag]); in both cases nothing is written.
func RemoveTag(ctx context.Context, filePath string, line int, name string) error {
	return defaultToggler.RemoveTag(ctx, filePath, line, name)
}

// RemoveTag strips a tag from a task line using this Toggler's lock state.
func (t *Toggler) RemoveTag(ctx context.Context, filePath string, line int, name string) error {
	re := tagRemovalRe(name)
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
		matches := re.FindAllString(l, -1)
		switch len(matches) {
		case 0:
			return "", fmt.Errorf("%w: line %d has no @%s tag", ErrStaleData, line, name)
		case 1:
			l = re.ReplaceAllStringFunc(l, func(match string) string {
				if strings.HasSuffix(match, " ") || strings.HasSuffix(match, "\t") {
					return " "
				}
				return ""
			})
			return strings.TrimRight(l, " \t"), nil
		default:
			return "", fmt.Errorf("%w: line %d has %d @%s tags", ErrAmbiguousTag, line, len(matches), name)
		}
	})
}

// SetTagValue rewrites the value of a single @name tag in place, turning
// @name(old) into @name(newValue) and leaving the rest of the line byte-for-byte
// unchanged. It is how a Re-create moves a Link to a fresh Todo id. A line with
// no such tag is treated as stale ([ErrStaleData]) and a line carrying two or
// more is ambiguous ([ErrAmbiguousTag]); in both cases nothing is written.
func SetTagValue(ctx context.Context, filePath string, line int, name, newValue string) error {
	return defaultToggler.SetTagValue(ctx, filePath, line, name, newValue)
}

// SetTagValue rewrites a tag's value using this Toggler's lock state.
func (t *Toggler) SetTagValue(ctx context.Context, filePath string, line int, name, newValue string) error {
	re := regexp.MustCompile(`@` + regexp.QuoteMeta(name) + `(?:\([^)]*\))?`)
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
		switch matches := re.FindAllString(l, -1); len(matches) {
		case 0:
			return "", fmt.Errorf("%w: line %d has no @%s tag", ErrStaleData, line, name)
		case 1:
			return re.ReplaceAllString(l, "@"+name+"("+newValue+")"), nil
		default:
			return "", fmt.Errorf("%w: line %d has %d @%s tags", ErrAmbiguousTag, line, len(matches), name)
		}
	})
}

// tagRemovalRe matches one @name or @name(value) token together with any
// leading whitespace and its trailing separator, mirroring the parser's tag
// grammar so removal strips exactly the tokens pike recognises.
func tagRemovalRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`\s*@` + regexp.QuoteMeta(name) + `(?:\([^)]*\))?(?:\s|$)`)
}

// AppendTask appends a new checkbox task line to a file. Creates the file
// if it doesn't exist. The line is formatted as "- [ ] text".
// Returns an error if text is empty.
func AppendTask(ctx context.Context, filePath string, text string) error {
	return defaultToggler.AppendTask(ctx, filePath, text)
}

// AppendTask appends a new checkbox task line to a file.
func (t *Toggler) AppendTask(ctx context.Context, filePath string, text string) error {
	if text == "" {
		return fmt.Errorf("cannot append empty task")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	mu := t.locks.lock(filePath)
	defer mu.Unlock()

	line := "- [ ] " + text

	var lines []string
	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read file: %w", err)
		}
		lines = []string{line}
	} else {
		content := strings.TrimSuffix(string(data), "\n")
		if content == "" {
			lines = []string{line}
		} else {
			lines = append(strings.Split(content, "\n"), line)
		}
	}

	info, err := os.Stat(filePath)
	perm := os.FileMode(0o644)
	if err == nil {
		perm = info.Mode()
	}

	return writeLines(filePath, lines, perm)
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
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".pike-tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write([]byte(content)); err != nil {
		tmp.Close()        //nolint:errcheck // cleaning up on error
		os.Remove(tmpPath) //nolint:errcheck // cleaning up on error
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()        //nolint:errcheck // cleaning up on error
		os.Remove(tmpPath) //nolint:errcheck // cleaning up on error
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath) //nolint:errcheck // cleaning up on error
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) //nolint:errcheck // cleaning up on error; rename error takes precedence
		return err
	}
	return nil
}
