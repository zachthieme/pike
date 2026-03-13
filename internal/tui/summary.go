package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderSummary renders a centered summary overlay with task counts.
// The overdue count is red when > 0.
func RenderSummary(open, overdue, dueThisWeek, completedThisWeek int, width int) string {
	const labelWidth = 24

	formatLine := func(label string, count int, style lipgloss.Style) string {
		countStr := fmt.Sprintf("%d", count)
		padding := labelWidth - len(label)
		if padding < 1 {
			padding = 1
		}
		line := fmt.Sprintf("  %s%s%s", label, strings.Repeat(" ", padding), countStr)
		if style.Value() != lipgloss.NewStyle().Value() {
			return style.Render(line)
		}
		return line
	}

	noStyle := lipgloss.NewStyle()
	redStyle := lipgloss.NewStyle().Foreground(resolveColor("red"))

	var lines []string
	lines = append(lines, "")
	lines = append(lines, formatLine("Open tasks", open, noStyle))

	overdueStyle := noStyle
	if overdue > 0 {
		overdueStyle = redStyle
	}
	lines = append(lines, formatLine("Overdue", overdue, overdueStyle))
	lines = append(lines, formatLine("Due this week", dueThisWeek, noStyle))
	lines = append(lines, formatLine("Completed this week", completedThisWeek, noStyle))
	lines = append(lines, "")

	content := strings.Join(lines, "\n")

	boxStyle := SummaryStyle()
	if width > 0 {
		boxWidth := 40
		if boxWidth > width-4 {
			boxWidth = width - 4
		}
		boxStyle = boxStyle.Width(boxWidth)
	}

	box := boxStyle.Render(content)

	// Center horizontally.
	if width > 0 {
		box = lipgloss.PlaceHorizontal(width, lipgloss.Center, box)
	}

	return box
}
