package parser

import (
	"regexp"
	"strings"
	"pike/internal/model"
	"time"
)

var (
	taskLineRe  = regexp.MustCompile(`^\s*- \[([ xX])\] (.*)$`)
	plainLineRe = regexp.MustCompile(`^\s*- (.+)$`)
	tagRe       = regexp.MustCompile(`@(\w+)(?:\(([^)]*)\))?`)
)

// ParseLine parses a single line of text and returns a Task if the line
// matches the task format, or nil if it does not.
// Matches checkbox lines (- [ ] / - [x]) and plain bullet lines (- text)
// that contain at least one @tag.
func ParseLine(line string, file string, lineNum int) *model.Task {
	m := taskLineRe.FindStringSubmatch(line)

	var checkbox string
	var text string

	hasCheckbox := m != nil

	if hasCheckbox {
		checkbox = m[1]
		text = strings.TrimRight(m[2], " ")
	} else {
		// Try plain bullet line — only match if it contains a @tag.
		pm := plainLineRe.FindStringSubmatch(line)
		if pm == nil {
			return nil
		}
		candidate := strings.TrimRight(pm[1], " ")
		if !tagRe.MatchString(candidate) {
			return nil
		}
		checkbox = " " // treat plain bullets as open
		text = candidate
	}

	task := &model.Task{
		Text:        text,
		File:        file,
		Line:        lineNum,
		HasCheckbox: hasCheckbox,
	}

	// Determine state
	if checkbox == " " {
		task.State = model.Open
	} else {
		task.State = model.Completed
	}

	// Extract tags
	tagMatches := tagRe.FindAllStringSubmatch(text, -1)
	for _, tm := range tagMatches {
		tagName := tm[1]
		tagValue := tm[2]

		tag := model.Tag{
			Name:  tagName,
			Value: tagValue,
		}

		// For date tags (due, completed), try to parse the value; if invalid, clear it.
		// For other tags, preserve the value as-is.
		if tagValue != "" && (tagName == "due" || tagName == "completed") {
			_, err := time.Parse("2006-01-02", tagValue)
			if err != nil {
				tag.Value = ""
			}
		}

		task.Tags = append(task.Tags, tag)

		// Set Due and Completed fields from well-known tags
		if tagName == "due" && tag.Value != "" {
			t, _ := time.Parse("2006-01-02", tag.Value)
			task.Due = &t
		}
		if tagName == "completed" {
			if !task.HasCheckbox {
				task.State = model.Completed
			}
			if tag.Value != "" {
				t, _ := time.Parse("2006-01-02", tag.Value)
				task.Completed = &t
			}
		}
	}

	return task
}
