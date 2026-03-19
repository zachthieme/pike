// Package render formats tasks for stdout as plain text, styled ANSI, or JSON.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"pike/internal/model"
	"pike/internal/style"
)

// FormatTask formats a single task for non-interactive output.
func FormatTask(task model.Task, tagColors map[string]string, noColor bool) string {
	text := task.Text
	if !noColor && tagColors != nil {
		text = style.ColorizeTags(text, task.Tags, tagColors, style.ANSIStyleFunc())
	}

	marker := style.TaskMarker(task, false)
	if task.HasCheckbox {
		return fmt.Sprintf("- %s %s", marker, text)
	}
	return fmt.Sprintf("%s %s", marker, text)
}

// jsonTask is the JSON-serializable representation of a task.
type jsonTask struct {
	Text        string   `json:"text"`
	State       string   `json:"state"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Tags        []string `json:"tags,omitempty"`
	Due         string   `json:"due,omitempty"`
	Completed   string   `json:"completed,omitempty"`
	HasCheckbox bool     `json:"has_checkbox"`
}

// FormatJSON writes tasks as a JSON array to the writer.
func FormatJSON(w io.Writer, tasks []model.Task) error {
	out := make([]jsonTask, 0, len(tasks))
	for _, t := range tasks {
		jt := jsonTask{
			Text:        t.Text,
			State:       t.State.String(),
			File:        t.File,
			Line:        t.Line,
			HasCheckbox: t.HasCheckbox,
		}
		for _, tag := range t.Tags {
			jt.Tags = append(jt.Tags, style.TagToken(tag))
		}
		if t.Due != nil {
			jt.Due = t.Due.Format("2006-01-02")
		}
		if t.Completed != nil {
			jt.Completed = t.Completed.Format("2006-01-02")
		}
		out = append(out, jt)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// FormatSummary formats the task summary counts for non-interactive output.
func FormatSummary(open, overdue, dueThisWeek, completedThisWeek int, noColor bool) string {
	const summaryLabelWidth = 30
	const ansiReset = "\033[0m"

	header := "\u2550\u2550\u2550 Task Summary \u2550\u2550\u2550"

	formatLine := func(label string, count int) string {
		countStr := fmt.Sprintf("%d", count)
		padding := summaryLabelWidth - len(label) - len(countStr)
		if padding < 1 {
			padding = 1
		}
		return fmt.Sprintf("  %s%s%s", label, strings.Repeat(" ", padding), countStr)
	}

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")
	b.WriteString(formatLine("Open tasks", open))
	b.WriteString("\n")

	overdueLine := formatLine("Overdue", overdue)
	if !noColor && overdue > 0 {
		overdueLine = "\033[31m" + overdueLine + ansiReset
	}
	b.WriteString(overdueLine)
	b.WriteString("\n")

	b.WriteString(formatLine("Due this week", dueThisWeek))
	b.WriteString("\n")
	b.WriteString(formatLine("Completed this week", completedThisWeek))

	return b.String()
}
