package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"pike/internal/config"
)

// RenderSummary renders a full-screen summary with version, description, and keybindings.
func RenderSummary(version string, width int, keys KeyMap, custom []config.CustomBinding) string {
	fs := FaintStyle()
	bs := BoldStyle()
	hs := HeaderStyle()

	var lines []string

	// Header
	versionStr := ""
	if version != "" {
		versionStr = " " + version
	}
	lines = append(lines, hs.Render("pike"+versionStr))
	lines = append(lines, "")
	lines = append(lines, fs.Render("A long pointed tool, used to pick through things"))
	lines = append(lines, fs.Render("quickly and with precision. Your tasks are scattered"))
	lines = append(lines, fs.Render("across dozens of markdown files. Pike reaches in"))
	lines = append(lines, fs.Render("and pulls them out."))
	lines = append(lines, "")
	lines = append(lines, "")

	// Keybinding sections
	type entry struct{ key, desc string }
	type section struct {
		title   string
		entries []entry
	}

	bindingEntry := func(b key.Binding) (entry, bool) {
		if !b.Enabled() {
			return entry{}, false
		}
		h := b.Help()
		return entry{key: h.Key, desc: h.Desc}, true
	}

	// Navigation
	var navEntries []entry
	for _, b := range []key.Binding{keys.Down, keys.Up, keys.Top, keys.Bottom, keys.PageDown, keys.PageUp, keys.NextSection, keys.PrevSection} {
		if e, ok := bindingEntry(b); ok {
			navEntries = append(navEntries, e)
		}
	}
	// FocusSection: show as "1-9 / focus section" only if at least the first is enabled.
	if keys.FocusSection[0].Enabled() {
		navEntries = append(navEntries, entry{key: "1-9", desc: "focus section"})
	}

	// Actions
	var actionEntries []entry
	for _, b := range []key.Binding{keys.Enter, keys.Toggle, keys.ToggleHiddenTag, keys.Refresh, keys.Quit} {
		if e, ok := bindingEntry(b); ok {
			actionEntries = append(actionEntries, e)
		}
	}

	// Views
	var viewEntries []entry
	for _, b := range []key.Binding{keys.AllTasks, keys.TagSearch, keys.RecentlyCompleted, keys.Summary, keys.ToggleHidden} {
		if e, ok := bindingEntry(b); ok {
			viewEntries = append(viewEntries, e)
		}
	}

	// Search
	var searchEntries []entry
	for _, b := range []key.Binding{keys.Filter, keys.Query, keys.Escape} {
		if e, ok := bindingEntry(b); ok {
			searchEntries = append(searchEntries, e)
		}
	}

	sections := []section{
		{title: "Navigation", entries: navEntries},
		{title: "Actions", entries: actionEntries},
		{title: "Views", entries: viewEntries},
		{title: "Search", entries: searchEntries},
	}

	// Custom shortcuts section
	if len(custom) > 0 {
		var shortcutEntries []entry
		for _, cb := range custom {
			desc := ""
			if cb.View != "" {
				desc = "focus " + cb.View
			} else if cb.Query != "" {
				desc = cb.Query
			}
			shortcutEntries = append(shortcutEntries, entry{key: cb.Key, desc: desc})
		}
		sections = append(sections, section{title: "Shortcuts", entries: shortcutEntries})
	}

	// Find the widest key across all sections for uniform alignment.
	maxKeyLen := 0
	for _, sec := range sections {
		for _, e := range sec.entries {
			if len(e.key) > maxKeyLen {
				maxKeyLen = len(e.key)
			}
		}
	}
	colWidth := maxKeyLen + 4

	for i, sec := range sections {
		if len(sec.entries) == 0 {
			continue
		}
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, bs.Render(sec.title))
		lines = append(lines, "")
		for _, e := range sec.entries {
			paddedKey := fmt.Sprintf("  %-*s", colWidth, e.key)
			lines = append(lines, paddedKey+fs.Render(e.desc))
		}
	}

	content := strings.Join(lines, "\n")

	if width > 0 {
		content = lipgloss.PlaceHorizontal(width, lipgloss.Center,
			lipgloss.NewStyle().Width(min(60, width)).Render(content))
	}

	return content
}
