package render

import (
	"fmt"
	"strconv"
	"strings"

	"pike/internal/model"
)

// ANSI color escape codes.
var namedColors = map[string]string{
	"red":     "\033[31m",
	"green":   "\033[32m",
	"yellow":  "\033[33m",
	"blue":    "\033[34m",
	"magenta": "\033[35m",
	"cyan":    "\033[36m",
	"white":   "\033[37m",
}

const ansiReset = "\033[0m"

// colorCode returns the ANSI escape sequence for a color name or hex value.
// Named colors: "red", "green", "yellow", "blue", "magenta", "cyan", "white".
// Hex colors: "#RRGGBB" converted to 24-bit ANSI escape sequences.
// Returns empty string if the color is not recognized.
func colorCode(color string) string {
	if code, ok := namedColors[color]; ok {
		return code
	}
	if strings.HasPrefix(color, "#") && len(color) == 7 {
		r, errR := strconv.ParseInt(color[1:3], 16, 64)
		g, errG := strconv.ParseInt(color[3:5], 16, 64)
		b, errB := strconv.ParseInt(color[5:7], 16, 64)
		if errR == nil && errG == nil && errB == nil {
			return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
		}
	}
	return ""
}

// tagToken returns the string representation of a tag as it appears in task text.
// Bare tags: "@name", parameterized tags: "@name(value)".
func tagToken(tag model.Tag) string {
	if tag.Value != "" {
		return "@" + tag.Name + "(" + tag.Value + ")"
	}
	return "@" + tag.Name
}

// FormatTask formats a single task for non-interactive output.
// Format: "file:line  - [state] text"
// Tags in the text are colorized based on tagColors unless noColor is true.
func FormatTask(task model.Task, tagColors map[string]string, noColor bool) string {
	state := " "
	if task.State == model.Completed {
		state = "x"
	}

	text := task.Text

	// Colorize tags if color is enabled and tagColors is provided.
	if !noColor && tagColors != nil {
		for _, tag := range task.Tags {
			token := tagToken(tag)
			color, ok := tagColors[tag.Name]
			if !ok {
				color = tagColors["_default"]
			}
			if color == "" {
				continue
			}
			code := colorCode(color)
			if code == "" {
				continue
			}
			colored := code + token + ansiReset
			text = strings.ReplaceAll(text, token, colored)
		}
	}

	if task.HasCheckbox {
		return fmt.Sprintf("%s:%d  - [%s] %s", task.File, task.Line, state, text)
	}
	return fmt.Sprintf("%s:%d  - %s", task.File, task.Line, text)
}

// FormatSummary formats the task summary counts for non-interactive output.
// The overdue count is highlighted in red when > 0 (unless noColor is true).
func FormatSummary(open, overdue, dueThisWeek, completedThisWeek int, noColor bool) string {
	const width = 30

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
