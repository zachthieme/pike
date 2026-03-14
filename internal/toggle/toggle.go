package toggle

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var completedTagRe = regexp.MustCompile(`\s*@completed(\([^)]*\))?(?:\s|$)`)
var hiddenTagRe = regexp.MustCompile(`\s*@hidden(?:\s|$)`)

// Complete marks an open checkbox task as completed by modifying the source file.
// Replaces - [ ] with - [x] and appends @completed(YYYY-MM-DD).
// Returns an error if the line doesn't contain - [ ] (stale data).
func Complete(filePath string, line int, date time.Time) error {
	lines, err := readLines(filePath)
	if err != nil {
		return err
	}
	if line < 1 || line > len(lines) {
		return fmt.Errorf("line %d out of range (file has %d lines)", line, len(lines))
	}

	idx := line - 1
	l := lines[idx]
	if !strings.Contains(l, "- [ ]") {
		return fmt.Errorf("line %d does not contain '- [ ]': %q", line, l)
	}

	l = strings.Replace(l, "- [ ]", "- [x]", 1)
	l += fmt.Sprintf(" @completed(%s)", date.Format("2006-01-02"))
	lines[idx] = l

	return writeLines(filePath, lines)
}

// Uncomplete marks a completed checkbox task as open by modifying the source file.
// Replaces - [x] with - [ ] and removes @completed(...) tag.
// Returns an error if the line doesn't contain - [x] (stale data).
func Uncomplete(filePath string, line int) error {
	lines, err := readLines(filePath)
	if err != nil {
		return err
	}
	if line < 1 || line > len(lines) {
		return fmt.Errorf("line %d out of range (file has %d lines)", line, len(lines))
	}

	idx := line - 1
	l := lines[idx]
	if !strings.Contains(l, "- [x]") {
		return fmt.Errorf("line %d does not contain '- [x]': %q", line, l)
	}

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

	return writeLines(filePath, lines)
}

// ToggleHidden adds @hidden to a task line if absent, or removes it if present.
func ToggleHidden(filePath string, line int) error {
	lines, err := readLines(filePath)
	if err != nil {
		return err
	}
	if line < 1 || line > len(lines) {
		return fmt.Errorf("line %d out of range (file has %d lines)", line, len(lines))
	}

	idx := line - 1
	l := lines[idx]

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
	return writeLines(filePath, lines)
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := string(data)
	if strings.HasSuffix(s, "\n") {
		s = s[:len(s)-1]
	}
	return strings.Split(s, "\n"), nil
}

func writeLines(path string, lines []string) error {
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}
