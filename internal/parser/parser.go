// Package parser extracts tasks and tags from markdown checkbox lines.
package parser

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/zachthieme/pike/internal/model"
)

var (
	taskLineRe  = regexp.MustCompile(`^\s*- \[([ xX])\] (.*)$`)
	plainLineRe = regexp.MustCompile(`^\s*- (.+)$`)
	tagRe       = regexp.MustCompile(`@(\w+)(?:\(([^)]*)\))?`)
)

func normalizeDate(s string) (string, bool) {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ".", "-")
	parts := strings.Split(s, "-")
	if len(parts) != 3 {
		return "", false
	}
	if len(parts[1]) == 1 {
		parts[1] = "0" + parts[1]
	}
	if len(parts[2]) == 1 {
		parts[2] = "0" + parts[2]
	}
	normalized := parts[0] + "-" + parts[1] + "-" + parts[2]
	_, err := time.Parse("2006-01-02", normalized)
	if err != nil {
		return "", false
	}
	return normalized, true
}

// lineMatch holds the result of matching a markdown line against task patterns.
type lineMatch struct {
	text        string
	checkbox    string // " " for open, "x"/"X" for completed
	hasCheckbox bool
}

// matchLine attempts to match a line as a checkbox item or a tagged plain bullet.
// Returns nil if the line is not a task.
func matchLine(line string) *lineMatch {
	if m := taskLineRe.FindStringSubmatch(line); m != nil {
		return &lineMatch{
			text:        strings.TrimRight(m[2], " "),
			checkbox:    m[1],
			hasCheckbox: true,
		}
	}
	pm := plainLineRe.FindStringSubmatch(line)
	if pm == nil {
		return nil
	}
	candidate := strings.TrimRight(pm[1], " ")
	if !tagRe.MatchString(candidate) {
		return nil
	}
	return &lineMatch{text: candidate, checkbox: " ", hasCheckbox: false}
}

// extractTags parses @tag tokens from the task text, normalizes date values
// for @due and @completed, and populates the task's tag list, Due, Completed,
// and State fields. Returns any warnings from invalid date values.
func extractTags(task *model.Task, file string, lineNum int) []model.Warning {
	var warnings []model.Warning
	for _, tm := range tagRe.FindAllStringSubmatch(task.Text, -1) {
		tagName := tm[1]
		tagValue := tm[2]
		tag := model.Tag{Name: tagName, Value: tagValue}

		if tagValue != "" && (tagName == "due" || tagName == "completed") {
			normalized, ok := normalizeDate(tagValue)
			if ok {
				tag.Value = normalized
			} else {
				warnings = append(warnings, model.Warning{
					File: file, Line: lineNum,
					Message: fmt.Sprintf("@%s value %q is not a valid date (expected YYYY-MM-DD)", tagName, tagValue),
				})
				tag.Value = ""
			}
		}

		task.AddTag(tag)

		if tagName == "due" && tag.Value != "" {
			t, _ := time.Parse("2006-01-02", tag.Value) //nolint:errcheck // validated by normalizeDate
			task.Due = &t
		}
		if tagName == "completed" {
			if !task.HasCheckbox {
				task.State = model.Completed
			}
			if tag.Value != "" {
				t, _ := time.Parse("2006-01-02", tag.Value) //nolint:errcheck // validated by normalizeDate
				task.Completed = &t
			}
		}
	}
	return warnings
}

// LinkSubtasks builds single-level parent-child relationships among tasks.
// Tasks must be ordered by file then line number (as returned by scanner).
// Modifies tasks in place: sets ParentIndex on children, appends to
// Children on parents. Does not reorder or remove tasks.
// Only links one level deep — a task that is already a child cannot be a parent.
func LinkSubtasks(tasks []model.Task) {
	for i := range tasks {
		if tasks[i].Indent == 0 {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if tasks[j].File != tasks[i].File {
				break // crossed file boundary
			}
			if tasks[j].Indent < tasks[i].Indent {
				if tasks[j].ParentIndex != -1 {
					break // tasks[j] is already a child; tasks[i] would be a grandchild — stop
				}
				tasks[i].ParentIndex = j
				tasks[j].Children = append(tasks[j].Children, &tasks[i])
				break
			}
		}
	}
}

// ParseLine parses a single line of text and returns a Task if the line
// matches the task format, or nil if it does not.
// Matches checkbox lines (- [ ] / - [x]) and plain bullet lines (- text)
// that contain at least one @tag.
// Also returns any non-fatal warnings encountered during parsing.
func ParseLine(line string, file string, lineNum int) (*model.Task, []model.Warning) {
	lm := matchLine(line)
	if lm == nil {
		return nil, nil
	}

	state := model.Open
	if lm.checkbox != " " {
		state = model.Completed
	}

	task := model.NewTask(lm.text, file, lineNum, state, lm.hasCheckbox)
	task.Indent = len(line) - len(strings.TrimLeft(line, " \t"))
	warnings := extractTags(task, file, lineNum)
	return task, warnings
}
