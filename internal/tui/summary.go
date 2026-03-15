package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderSummary renders a centered summary overlay with version, task counts, and keybindings.
func RenderSummary(version string, open, overdue, dueThisWeek, completedThisWeek int, width int) string {
	const labelWidth = 24

	formatLine := func(label string, count int, style lipgloss.Style) string {
		countStr := fmt.Sprintf("%d", count)
		padding := labelWidth - lipgloss.Width(label)
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
	faintStyle := lipgloss.NewStyle().Faint(true)
	redStyle := lipgloss.NewStyle().Foreground(resolveColor("red"))
	boldStyle := lipgloss.NewStyle().Bold(true)

	var lines []string

	// Header
	versionStr := ""
	if version != "" {
		versionStr = " " + version
	}
	lines = append(lines, boldStyle.Render("pike"+versionStr))
	lines = append(lines, faintStyle.Render("terminal task dashboard for markdown notes"))
	lines = append(lines, "")

	// Task counts
	lines = append(lines, formatLine("Open tasks", open, noStyle))
	overdueStyle := noStyle
	if overdue > 0 {
		overdueStyle = redStyle
	}
	lines = append(lines, formatLine("Overdue", overdue, overdueStyle))
	lines = append(lines, formatLine("Due this week", dueThisWeek, noStyle))
	lines = append(lines, formatLine("Completed this week", completedThisWeek, noStyle))
	lines = append(lines, "")

	// Keybindings
	lines = append(lines, boldStyle.Render("  Keys"))
	lines = append(lines, "")

	keys := []struct{ key, desc string }{
		{"j/k", "move up/down"},
		{"g/G", "top/bottom"},
		{"Enter", "open in editor"},
		{"x", "toggle complete"},
		{"", ""},
		{"/", "substring filter"},
		{"?", "query DSL filter"},
		{"a", "all tasks"},
		{"t", "tag search"},
		{"c", "recently completed"},
		{"", ""},
		{"Tab", "next section / toggle focus"},
		{"Shift+Tab", "prev section"},
		{"Ctrl+D/U", "page down/up"},
		{"H", "toggle @hidden tag"},
		{"h", "show/hide hidden"},
		{"s", "toggle this summary"},
		{"r", "refresh"},
		{"q", "quit"},
	}

	for _, k := range keys {
		if k.key == "" {
			lines = append(lines, "")
			continue
		}
		padding := 12 - len(k.key)
		if padding < 1 {
			padding = 1
		}
		lines = append(lines, fmt.Sprintf("  %s%s%s",
			boldStyle.Render(k.key),
			strings.Repeat(" ", padding),
			faintStyle.Render(k.desc)))
	}
	lines = append(lines, "")

	content := strings.Join(lines, "\n")

	boxStyle := SummaryStyle()
	if width > 0 {
		boxWidth := 44
		if boxWidth > width-4 {
			boxWidth = width - 4
		}
		boxStyle = boxStyle.Width(boxWidth)
	}

	box := boxStyle.Render(content)

	if width > 0 {
		box = lipgloss.PlaceHorizontal(width, lipgloss.Center, box)
	}

	return box
}
