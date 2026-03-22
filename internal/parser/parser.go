// Package parser extracts tasks and tags from markdown checkbox lines.
package parser

import (
	"fmt"
	"github.com/zachthieme/pike/internal/model"
	"regexp"
	"strings"
	"time"
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

// ParseLine parses a single line of text and returns a Task if the line
// matches the task format, or nil if it does not.
// Matches checkbox lines (- [ ] / - [x]) and plain bullet lines (- text)
// that contain at least one @tag.
// Also returns any non-fatal warnings encountered during parsing.
func ParseLine(line string, file string, lineNum int) (*model.Task, []model.Warning) {
	m := taskLineRe.FindStringSubmatch(line)
	var checkbox string
	var text string
	hasCheckbox := m != nil
	if hasCheckbox {
		checkbox = m[1]
		text = strings.TrimRight(m[2], " ")
	} else {
		pm := plainLineRe.FindStringSubmatch(line)
		if pm == nil {
			return nil, nil
		}
		candidate := strings.TrimRight(pm[1], " ")
		if !tagRe.MatchString(candidate) {
			return nil, nil
		}
		checkbox = " "
		text = candidate
	}
	state := model.Open
	if checkbox != " " {
		state = model.Completed
	}
	task := model.NewTask(text, file, lineNum, state, hasCheckbox)
	var warnings []model.Warning
	tagMatches := tagRe.FindAllStringSubmatch(text, -1)
	for _, tm := range tagMatches {
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
	return task, warnings
}
