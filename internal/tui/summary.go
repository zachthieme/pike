package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderSummary renders a centered summary overlay with version, description, and keybindings.
func RenderSummary(version string, width int) string {
	faintStyle := lipgloss.NewStyle().Faint(true)
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

	// Keybindings
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

	// Find the widest key to align descriptions.
	maxKeyWidth := 0
	for _, k := range keys {
		if len(k.key) > maxKeyWidth {
			maxKeyWidth = len(k.key)
		}
	}
	colGap := 3

	for _, k := range keys {
		if k.key == "" {
			lines = append(lines, "")
			continue
		}
		padding := maxKeyWidth - len(k.key) + colGap
		lines = append(lines, fmt.Sprintf("  %s%s%s",
			boldStyle.Render(k.key),
			strings.Repeat(" ", padding),
			faintStyle.Render(k.desc)))
	}
	lines = append(lines, "")

	content := strings.Join(lines, "\n")

	boxStyle := SummaryStyle()
	if width > 0 {
		boxWidth := 48
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
