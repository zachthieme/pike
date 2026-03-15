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
	lines = append(lines, "")
	lines = append(lines, faintStyle.Render("  A long pointed tool, used to pick"))
	lines = append(lines, faintStyle.Render("  through things quickly and with"))
	lines = append(lines, faintStyle.Render("  precision. Your tasks are scattered"))
	lines = append(lines, faintStyle.Render("  across dozens of markdown files."))
	lines = append(lines, faintStyle.Render("  Pike reaches in and pulls them out."))
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

	// Find the widest key to compute the description column offset.
	maxKeyLen := 0
	for _, k := range keys {
		if len(k.key) > maxKeyLen {
			maxKeyLen = len(k.key)
		}
	}
	colWidth := maxKeyLen + 3

	for _, k := range keys {
		if k.key == "" {
			lines = append(lines, "")
			continue
		}
		// Right-pad the key to a fixed column width, then style each part.
		paddedKey := fmt.Sprintf("%-*s", colWidth, k.key)
		lines = append(lines, "  "+boldStyle.Render(paddedKey)+faintStyle.Render(k.desc))
	}
	lines = append(lines, "")

	content := strings.Join(lines, "\n")

	boxStyle := SummaryStyle()
	if width > 0 {
		boxWidth := min(48, width-4)
		boxStyle = boxStyle.Width(boxWidth)
	}

	box := boxStyle.Render(content)

	if width > 0 {
		box = lipgloss.PlaceHorizontal(width, lipgloss.Center, box)
	}

	return box
}
