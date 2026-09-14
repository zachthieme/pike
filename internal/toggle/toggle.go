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

// staleLine reports that the line at a scanned position no longer matches the
// full line the caller scanned, so a Sync mutation writes nothing. Every
// line-verifying mutation returns this when its byte-for-byte guard fails.
func staleLine(line int) error {
	return fmt.Errorf("%w: line %d no longer matches the scanned task", ErrStaleData, line)
}

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
// Returns an error if the line doesn't contain - [ ] (stale data). This is the
// TUI's own toggle: it trusts the line number and its checkbox marker, so a Task
// toggled in the dashboard completes exactly the line under the cursor.
func (t *Toggler) Complete(ctx context.Context, filePath string, line int, date time.Time) error {
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
		if !strings.Contains(l, "- [ ]") {
			return "", fmt.Errorf("%w: line %d does not contain '- [ ]'", ErrStaleData, line)
		}
		return completeLine(l, date), nil
	})
}

// Uncomplete marks a completed checkbox task as open by modifying the source file.
// Replaces - [x]/- [X] with - [ ] and removes @completed(...) tag.
// Returns an error if the line doesn't contain - [x] or - [X] (stale data). Like
// [Toggler.Complete], this is the TUI's own toggle and trusts the line number.
func (t *Toggler) Uncomplete(ctx context.Context, filePath string, line int) error {
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
		if !strings.Contains(l, "- [x]") && !strings.Contains(l, "- [X]") {
			return "", fmt.Errorf("%w: line %d does not contain '- [x]'", ErrStaleData, line)
		}
		return uncompleteLine(l), nil
	})
}

// completeLine turns an open checkbox line into a completed one: it swaps the
// marker and appends @completed(date).
func completeLine(l string, date time.Time) string {
	l = strings.Replace(l, "- [ ]", "- [x]", 1)
	return l + fmt.Sprintf(" @completed(%s)", date.Format("2006-01-02"))
}

// uncompleteLine turns a completed checkbox line into an open one: it swaps the
// marker and strips the @completed tag, collapsing the surrounding whitespace.
func uncompleteLine(l string) string {
	l = strings.Replace(l, "- [x]", "- [ ]", 1)
	l = strings.Replace(l, "- [X]", "- [ ]", 1)
	l = completedTagRe.ReplaceAllStringFunc(l, func(match string) string {
		if strings.HasSuffix(match, " ") || strings.HasSuffix(match, "\t") {
			return " "
		}
		return ""
	})
	return strings.TrimRight(l, " \t")
}

// CompleteLine marks an open checkbox task as completed, but only when the line
// at the given position is still byte-for-byte equal to wantLine — the full line
// the caller scanned. Otherwise the line changed since the scan and CompleteLine
// returns [ErrStaleData] without writing. This is the guarded completion a Sync
// uses so it never completes a line that has become a different Task.
func CompleteLine(ctx context.Context, filePath string, line int, wantLine string, date time.Time) error {
	return defaultToggler.CompleteLine(ctx, filePath, line, wantLine, date)
}

// CompleteLine marks an open checkbox task as completed using this Toggler's lock state.
func (t *Toggler) CompleteLine(ctx context.Context, filePath string, line int, wantLine string, date time.Time) error {
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
		if l != wantLine {
			return "", staleLine(line)
		}
		return completeLine(l, date), nil
	})
}

// UncompleteLine reopens a completed checkbox task, but only when the line at the
// given position is still byte-for-byte equal to wantLine — the full line the
// caller scanned. Otherwise the line changed since the scan and UncompleteLine
// returns [ErrStaleData] without writing. This is the guarded reopen a Sync uses
// so it never un-links or reopens a line that has become a different Task.
func UncompleteLine(ctx context.Context, filePath string, line int, wantLine string) error {
	return defaultToggler.UncompleteLine(ctx, filePath, line, wantLine)
}

// UncompleteLine reopens a completed checkbox task using this Toggler's lock state.
func (t *Toggler) UncompleteLine(ctx context.Context, filePath string, line int, wantLine string) error {
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
		if l != wantLine {
			return "", staleLine(line)
		}
		return uncompleteLine(l), nil
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
// of the line byte-for-byte unchanged. wantLine is the full line the caller
// observed when it scanned the task; unless the line at that position is still
// byte-for-byte equal to it, the line changed since the scan and AppendTag
// returns [ErrStaleData] without writing. This is the stale-line guard every
// Sync mutation shares.
func AppendTag(ctx context.Context, filePath string, line int, wantLine, tag string) error {
	return defaultToggler.AppendTag(ctx, filePath, line, wantLine, tag)
}

// AppendTag appends a tag to a task line using this Toggler's lock state.
func (t *Toggler) AppendTag(ctx context.Context, filePath string, line int, wantLine, tag string) error {
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
		if l != wantLine {
			return "", staleLine(line)
		}
		return l + " " + tag, nil
	})
}

// SetText rewrites a task line's text to newText while keeping its tags,
// re-appending them at the end in their original order. The leading marker
// (indentation, bullet, and checkbox state) is left untouched. wantLine is the
// full line the caller observed when it scanned the task; unless the line at
// that position is still byte-for-byte equal to it, the line changed since the
// scan and SetText returns [ErrStaleData] without writing, the same stale-line
// guard AppendTag uses.
func SetText(ctx context.Context, filePath string, line int, wantLine, newText string) error {
	return defaultToggler.SetText(ctx, filePath, line, wantLine, newText)
}

// SetText rewrites a task line's text keeping its tags, using this Toggler's
// lock state.
func (t *Toggler) SetText(ctx context.Context, filePath string, line int, wantLine, newText string) error {
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
		if l != wantLine {
			return "", staleLine(line)
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
// unchanged. It is the supported way to un-link a Task from HEY. wantLine is the
// full line the caller scanned; unless the line at that position is still
// byte-for-byte equal to it — which verifies the exact @name(value) being
// stripped — the line changed since the scan and RemoveTag returns
// [ErrStaleData] without writing. A line with no such tag is likewise stale, and
// a line carrying two or more is ambiguous ([ErrAmbiguousTag]); in every case
// nothing is written.
func RemoveTag(ctx context.Context, filePath string, line int, wantLine, name string) error {
	return defaultToggler.RemoveTag(ctx, filePath, line, wantLine, name)
}

// RemoveTag strips a tag from a task line using this Toggler's lock state.
func (t *Toggler) RemoveTag(ctx context.Context, filePath string, line int, wantLine, name string) error {
	re := tagRemovalRe(name)
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
		if l != wantLine {
			return "", staleLine(line)
		}
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
// unchanged. It is how a Re-create moves a Link to a fresh Todo id. wantLine is
// the full line the caller scanned; unless the line at that position is still
// byte-for-byte equal to it — which verifies the exact @name(old) being
// replaced — the line changed since the scan and SetTagValue returns
// [ErrStaleData] without writing. A line with no such tag is likewise stale, and
// a line carrying two or more is ambiguous ([ErrAmbiguousTag]); in every case
// nothing is written.
func SetTagValue(ctx context.Context, filePath string, line int, wantLine, name, newValue string) error {
	return defaultToggler.SetTagValue(ctx, filePath, line, wantLine, name, newValue)
}

// SetTagValue rewrites a tag's value using this Toggler's lock state.
func (t *Toggler) SetTagValue(ctx context.Context, filePath string, line int, wantLine, name, newValue string) error {
	re := regexp.MustCompile(`@` + regexp.QuoteMeta(name) + `\b(?:\([^)]*\))?`)
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
		if l != wantLine {
			return "", staleLine(line)
		}
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

// dueTagRe matches an @due token with or without a value, mirroring the parser's
// tag grammar so a "set or replace" leaves exactly one @due token behind. The
// trailing \b keeps @due from matching the "@due" inside a longer name like
// @duedate, which the parser reads as the distinct tag "duedate".
var dueTagRe = regexp.MustCompile(`@due\b(?:\([^)]*\))?`)

// SetDue sets the task line's @due tag to date (formatted YYYY-MM-DD): it
// rewrites an existing @due value in place, or appends @due(date) when the line
// has none, leaving the rest of the line unchanged. wantLine is the full line
// the caller scanned; unless the line at that position is still byte-for-byte
// equal to it, the line changed since the scan and SetDue returns [ErrStaleData]
// without writing. A line carrying two or more @due tags is ambiguous
// ([ErrAmbiguousTag]) and nothing is written. This is the atomic line-write path
// a Sync uses to reschedule a Task from HEY's Week.
func SetDue(ctx context.Context, filePath string, line int, wantLine string, date time.Time) error {
	return defaultToggler.SetDue(ctx, filePath, line, wantLine, date)
}

// SetDue sets a task line's @due tag using this Toggler's lock state.
func (t *Toggler) SetDue(ctx context.Context, filePath string, line int, wantLine string, date time.Time) error {
	value := date.Format("2006-01-02")
	return t.mutateFile(ctx, filePath, line, func(l string) (string, error) {
		if l != wantLine {
			return "", staleLine(line)
		}
		switch matches := dueTagRe.FindAllString(l, -1); len(matches) {
		case 0:
			return l + " @due(" + value + ")", nil
		case 1:
			return dueTagRe.ReplaceAllString(l, "@due("+value+")"), nil
		default:
			return "", fmt.Errorf("%w: line %d has %d @due tags", ErrAmbiguousTag, line, len(matches))
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
