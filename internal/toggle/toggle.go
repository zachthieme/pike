package toggle

import (
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

// fileMu provides per-file locking to prevent concurrent mutations from racing.
var fileMu sync.Map // map[string]*sync.Mutex

func lockFile(path string) *sync.Mutex {
	v, _ := fileMu.LoadOrStore(path, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu
}

// Complete marks an open checkbox task as completed by modifying the source file.
// Replaces - [ ] with - [x] and appends @completed(YYYY-MM-DD).
// Returns an error if the line doesn't contain - [ ] (stale data).
func Complete(filePath string, line int, date time.Time) error {
	mu := lockFile(filePath)
	defer mu.Unlock()

	lines, err := readLines(filePath)
	if err != nil {
		return err
	}
	if line < 1 || line > len(lines) {
		return fmt.Errorf("%w: line %d (file has %d lines)", ErrLineOutOfRange, line, len(lines))
	}

	idx := line - 1
	l := lines[idx]
	if !strings.Contains(l, "- [ ]") {
		return fmt.Errorf("%w: line %d does not contain '- [ ]'", ErrStaleData, line)
	}

	originalLine := l
	l = strings.Replace(l, "- [ ]", "- [x]", 1)
	l += fmt.Sprintf(" @completed(%s)", date.Format("2006-01-02"))
	lines[idx] = l

	if err := verifyUnmodified(filePath, line, originalLine); err != nil {
		return err
	}
	return writeLines(filePath, lines)
}

// Uncomplete marks a completed checkbox task as open by modifying the source file.
// Replaces - [x] with - [ ] and removes @completed(...) tag.
// Returns an error if the line doesn't contain - [x] (stale data).
func Uncomplete(filePath string, line int) error {
	mu := lockFile(filePath)
	defer mu.Unlock()

	lines, err := readLines(filePath)
	if err != nil {
		return err
	}
	if line < 1 || line > len(lines) {
		return fmt.Errorf("%w: line %d (file has %d lines)", ErrLineOutOfRange, line, len(lines))
	}

	idx := line - 1
	l := lines[idx]
	if !strings.Contains(l, "- [x]") {
		return fmt.Errorf("%w: line %d does not contain '- [x]'", ErrStaleData, line)
	}

	originalLine := l
	l = strings.Replace(l, "- [x]", "- [ ]", 1)

	// Remove @completed(...) tag. The regex may match mid-line or at end.
	// If the match ends with whitespace, replace with a single space to
	// avoid joining adjacent content. If at end of string, remove entirely.
	l = completedTagRe.ReplaceAllStringFunc(l, func(match string) string {
		if strings.HasSuffix(match, " ") || strings.HasSuffix(match, "\t") {
			return " "
		}
		return ""
	})
	l = strings.TrimRight(l, " \t")

	lines[idx] = l

	if err := verifyUnmodified(filePath, line, originalLine); err != nil {
		return err
	}
	return writeLines(filePath, lines)
}

// ToggleHidden adds @hidden to a task line if absent, or removes it if present.
func ToggleHidden(filePath string, line int) error {
	mu := lockFile(filePath)
	defer mu.Unlock()

	lines, err := readLines(filePath)
	if err != nil {
		return err
	}
	if line < 1 || line > len(lines) {
		return fmt.Errorf("%w: line %d (file has %d lines)", ErrLineOutOfRange, line, len(lines))
	}

	idx := line - 1
	l := lines[idx]
	originalLine := l

	if strings.Contains(l, "@hidden") {
		// Remove @hidden tag
		l = hiddenTagRe.ReplaceAllStringFunc(l, func(match string) string {
			if strings.HasSuffix(match, " ") || strings.HasSuffix(match, "\t") {
				return " "
			}
			return ""
		})
		l = strings.TrimRight(l, " \t")
	} else {
		// Append @hidden tag
		l += " @hidden"
	}

	lines[idx] = l
	if err := verifyUnmodified(filePath, line, originalLine); err != nil {
		return err
	}
	return writeLines(filePath, lines)
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
func writeLines(path string, lines []string) error {
	content := strings.Join(lines, "\n") + "\n"
	tmp := path + ".pike-tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp) // best-effort cleanup; rename error takes precedence
		return err
	}
	return nil
}
