package render

import (
	"fmt"
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
		return fmt.Sprintf("%s:%d  - %s %s", task.File, task.Line, marker, text)
	}
	return fmt.Sprintf("%s:%d  %s %s", task.File, task.Line, marker, text)
}

// FormatSummary formats the task summary counts for non-interactive output.
func FormatSummary(open, overdue, dueThisWeek, completedThisWeek int, noColor bool) string {
	const width = 30
	const ansiReset = "\033[0m"

	header := "\u2550\u2550\u2550 Task Summary \u2550\u2550\u2550"

	formatLine := func(label string, count int) string {
		countStr := fmt.Sprintf("%d", count)
		padding := width - len(label) - len(countStr)
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
