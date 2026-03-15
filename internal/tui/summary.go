package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderSummary renders a full-screen summary with version, description, and keybindings.
func RenderSummary(version string, width int) string {
	faintStyle := lipgloss.NewStyle().Faint(true)
	boldStyle := lipgloss.NewStyle().Bold(true)
	headerStyle := lipgloss.NewStyle().Bold(true).Underline(true)

	var lines []string

	// Header
	versionStr := ""
	if version != "" {
		versionStr = " " + version
	}
	lines = append(lines, headerStyle.Render("pike"+versionStr))
	lines = append(lines, "")
	lines = append(lines, faintStyle.Render("A long pointed tool, used to pick through things"))
	lines = append(lines, faintStyle.Render("quickly and with precision. Your tasks are scattered"))
	lines = append(lines, faintStyle.Render("across dozens of markdown files. Pike reaches in"))
	lines = append(lines, faintStyle.Render("and pulls them out."))
	lines = append(lines, "")
	lines = append(lines, "")

	// Keybinding sections
	type section struct {
		title string
		keys  []struct{ key, desc string }
	}

	sections := []section{
		{
			title: "Navigation",
			keys: []struct{ key, desc string }{
				{"j / k", "move down / up"},
				{"g / G", "jump to top / bottom"},
				{"Tab", "next section / toggle focus"},
				{"Shift+Tab", "previous section"},
				{"Ctrl+D / U", "page down / up"},
				{"1-9", "focus section by number"},
			},
		},
		{
			title: "Actions",
			keys: []struct{ key, desc string }{
				{"Enter", "open task in editor"},
				{"x", "toggle task complete"},
				{"H", "toggle @hidden tag on task"},
				{"h", "show / hide @hidden tasks"},
			},
		},
		{
			title: "Search & Filter",
			keys: []struct{ key, desc string }{
				{"/", "substring search"},
				{"?", "query DSL search"},
				{"a", "all open tasks"},
				{"t", "tag search"},
				{"c", "recently completed"},
				{"Esc", "clear filter / exit"},
			},
		},
		{
			title: "Other",
			keys: []struct{ key, desc string }{
				{"s", "toggle this summary"},
				{"r", "refresh tasks"},
				{"q", "quit"},
			},
		},
	}

	// Find the widest key across all sections for uniform alignment.
	maxKeyLen := 0
	for _, sec := range sections {
		for _, k := range sec.keys {
			if len(k.key) > maxKeyLen {
				maxKeyLen = len(k.key)
			}
		}
	}
	colWidth := maxKeyLen + 4

	for i, sec := range sections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, boldStyle.Render(sec.title))
		lines = append(lines, "")
		for _, k := range sec.keys {
			paddedKey := fmt.Sprintf("  %-*s", colWidth, k.key)
			lines = append(lines, paddedKey+faintStyle.Render(k.desc))
		}
	}

	content := strings.Join(lines, "\n")

	if width > 0 {
		content = lipgloss.PlaceHorizontal(width, lipgloss.Center,
			lipgloss.NewStyle().Width(min(60, width)).Render(content))
	}

	return content
}
