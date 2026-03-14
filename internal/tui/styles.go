package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// namedColorMap maps color names to lipgloss ANSI color codes.
var namedColorMap = map[string]string{
	"red":     "1",
	"green":   "2",
	"yellow":  "3",
	"blue":    "4",
	"magenta": "5",
	"cyan":    "6",
	"white":   "7",
}

// resolveColor converts a color name or hex string to a lipgloss.Color.
// Supports named colors ("red", "green", etc.) and hex ("#FF5733").
func resolveColor(color string) lipgloss.Color {
	if code, ok := namedColorMap[color]; ok {
		return lipgloss.Color(code)
	}
	if strings.HasPrefix(color, "#") {
		return lipgloss.Color(color)
	}
	// Fallback: try using the string directly as a lipgloss color.
	return lipgloss.Color(color)
}

// SectionHeaderStyle returns a bold style with the given color for section headers.
func SectionHeaderStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(resolveColor(color))
}

// TaskStyle returns a style for rendering task lines.
// If selected is true, the task is highlighted with a reverse video effect.
func TaskStyle(selected bool) lipgloss.Style {
	s := lipgloss.NewStyle()
	if selected {
		s = s.Reverse(true)
	}
	return s
}

// TagStyle returns a colored style for rendering tag tokens.
func TagStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(resolveColor(color))
}

// SummaryStyle returns a style for the summary overlay box.
func SummaryStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		Padding(1, 2).
		Align(lipgloss.Center)
}

// FooterStyle returns a style for the footer bar.
func FooterStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Faint(true)
}

// ErrorStyle returns a style for rendering error messages.
func ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(resolveColor("red")).
		Bold(true)
}

// LinkStyle returns a bold style with the given color for rendering links.
func LinkStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(resolveColor(color))
}
