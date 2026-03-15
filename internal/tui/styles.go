package tui

import (
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// reverseVideoEsc is the ANSI SGR escape for reverse video, used by
// TaskStyle(true) via lipgloss. Used to locate the cursor line in rendered output.
const reverseVideoEsc = "\x1b[7m"

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

// Static styles (no runtime parameters).
var (
	footerStyle   = lipgloss.NewStyle().Faint(true)
	errorStyle    = lipgloss.NewStyle().Foreground(resolveColor("red")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Reverse(true)
	normalStyle   = lipgloss.NewStyle()
	faintStyle    = lipgloss.NewStyle().Faint(true)
	boldStyle     = lipgloss.NewStyle().Bold(true)
)

// Parameterized style caches.
var (
	sectionHeaderCache sync.Map // color string → lipgloss.Style
	tagStyleCache      sync.Map // color string → lipgloss.Style
	linkStyleCache     sync.Map // color string → lipgloss.Style
)

// SectionHeaderStyle returns a bold style with the given color for section headers.
func SectionHeaderStyle(color string) lipgloss.Style {
	if v, ok := sectionHeaderCache.Load(color); ok {
		return v.(lipgloss.Style)
	}
	s := lipgloss.NewStyle().Bold(true).Foreground(resolveColor(color))
	sectionHeaderCache.Store(color, s)
	return s
}

// TaskStyle returns a style for rendering task lines.
// If selected is true, the task is highlighted with a reverse video effect.
func TaskStyle(selected bool) lipgloss.Style {
	if selected {
		return selectedStyle
	}
	return normalStyle
}

// TagStyle returns a colored style for rendering tag tokens.
func TagStyle(color string) lipgloss.Style {
	if v, ok := tagStyleCache.Load(color); ok {
		return v.(lipgloss.Style)
	}
	s := lipgloss.NewStyle().Foreground(resolveColor(color))
	tagStyleCache.Store(color, s)
	return s
}

// FooterStyle returns a style for the footer bar.
func FooterStyle() lipgloss.Style {
	return footerStyle
}

// ErrorStyle returns a style for rendering error messages.
func ErrorStyle() lipgloss.Style {
	return errorStyle
}

// LinkStyle returns a bold style with the given color for rendering links.
func LinkStyle(color string) lipgloss.Style {
	if v, ok := linkStyleCache.Load(color); ok {
		return v.(lipgloss.Style)
	}
	s := lipgloss.NewStyle().Bold(true).Foreground(resolveColor(color))
	linkStyleCache.Store(color, s)
	return s
}

// FaintStyle returns a faint style for dimmed text.
func FaintStyle() lipgloss.Style {
	return faintStyle
}

// BoldStyle returns a bold style.
func BoldStyle() lipgloss.Style {
	return boldStyle
}
