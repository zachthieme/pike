package tui

import (
	"fmt"
	"strings"

	"pike/internal/filter"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model.
func (m Model) View() string {
	var errLine string
	if m.err != nil {
		errLine = ErrorStyle().Render("Error: "+m.err.Error()) + "\n"
	}

	if m.showSummary {
		return errLine + m.viewSummary()
	}

	switch m.mode {
	case modeAllTasks:
		return errLine + m.viewAllTasks()
	case modeTagSearch:
		return errLine + m.viewTagSearch()
	}

	if m.focusedView != "" {
		return errLine + m.viewFocused()
	}

	return errLine + m.viewDashboard()
}

func (m Model) viewDashboard() string {
	body, _ := m.renderSections()

	openCount := m.countOpen()
	footer := FooterStyle().Render(fmt.Sprintf("%s %d open tasks", strings.Repeat("\u2500", max(0, m.width-16)), openCount))

	full := body + "\n" + footer
	return m.truncateView(full)
}

func (m Model) viewFocused() string {
	body, count := m.renderSections()
	if count == 0 {
		return body + "\nNo tasks"
	}
	return m.truncateView(body)
}

func (m Model) viewSummary() string {
	open := m.countOpen()
	overdue := m.countOverdue()
	dueThisWeek := m.countDueThisWeek()
	completedThisWeek := m.countCompletedThisWeek()

	return RenderSummary(open, overdue, dueThisWeek, completedThisWeek, m.width)
}

// displaySections returns the sections to display based on focus mode.
func (m Model) displaySections() []filter.ViewResult {
	if m.focusedView != "" {
		// Find the matching section in m.sections (which may be filtered) by title.
		for _, sec := range m.sections {
			if sec.Title == m.focusedView {
				return []filter.ViewResult{sec}
			}
		}
		// Fallback: check the unfiltered visible sections.
		for _, sec := range m.visibleSections() {
			if sec.Title == m.focusedView {
				return []filter.ViewResult{sec}
			}
		}
		return nil
	}
	return m.sections
}

// viewAllTasks renders all open tasks in a single section with filter bar.
// Tasks are windowed to fit the terminal height, keeping the cursor visible.
func (m Model) viewAllTasks() string {
	var parts []string

	// Always show the search bar in all-tasks mode.
	parts = append(parts, m.filterInput.View())
	parts = append(parts, "")

	sections := m.displaySections()
	if len(sections) == 0 || len(sections[0].Tasks) == 0 {
		parts = append(parts, "  No matching tasks")
		return strings.Join(parts, "\n")
	}

	sec := sections[0]
	tasks := sec.Tasks

	// Calculate available lines for task rows inside the border box.
	// Overhead: search bar (1) + blank line (1) + section header (1)
	//           + border top (1) + border bottom (1)
	//           + scroll footer (1) + bubbletea final line (1) = 7
	overhead := 7

	maxVisible := m.height - overhead
	if maxVisible < 3 {
		maxVisible = 3
	}

	hiddenCount := m.hiddenCountFor(sec.Title)

	if len(tasks) <= maxVisible {
		rendered := m.renderSection(sec.Title, tasks, sec.Color, 0, hiddenCount)
		parts = append(parts, rendered)
	} else {
		start, end := scrollWindow(m.cursor, len(tasks), maxVisible)
		rendered := m.renderSection(sec.Title, tasks[start:end], sec.Color, start, hiddenCount)
		parts = append(parts, rendered)
		parts = append(parts, FooterStyle().Render(fmt.Sprintf("  %d–%d of %d tasks", start+1, end, len(tasks))))
	}

	return strings.Join(parts, "\n")
}

// viewTagSearch renders the tag picker with filter bar.
// Tags are displayed in a flow-wrapped line with matched tags highlighted.
func (m Model) viewTagSearch() string {
	var parts []string

	parts = append(parts, m.filterInput.View())
	parts = append(parts, "")

	filtered := m.filteredTags()
	if len(m.tagList) == 0 {
		parts = append(parts, "  No tags found")
		return strings.Join(parts, "\n")
	}

	// Build a set of matched tag names for quick lookup.
	matchedSet := make(map[string]bool, len(filtered))
	for _, tag := range filtered {
		matchedSet[tag] = true
	}

	// Determine which filtered tag is currently selected.
	selectedTag := ""
	if len(filtered) > 0 && m.tagCursor < len(filtered) {
		selectedTag = filtered[m.tagCursor]
	}

	// Render all tags in a flow-wrapped line.
	// Matched tags are highlighted with their configured color.
	// The selected tag (via Tab) gets reverse video.
	// Non-matching tags are rendered faint.
	faintStyle := lipgloss.NewStyle().Faint(true)
	delim := faintStyle.Render(" · ")
	var tagParts []string
	for _, tag := range m.tagList {
		if tag == selectedTag {
			tagParts = append(tagParts, TaskStyle(true).Render(tag))
		} else if matchedSet[tag] {
			if color := m.resolveTagColor(tag); color != "" {
				tagParts = append(tagParts, TagStyle(color).Render(tag))
			} else {
				tagParts = append(tagParts, tag)
			}
		} else {
			tagParts = append(tagParts, faintStyle.Render(tag))
		}
	}

	// Flow-wrap the tags to fit the terminal width.
	if m.width > 0 {
		parts = append(parts, flowWrap(tagParts, delim, m.width-2))
	} else {
		parts = append(parts, "  "+strings.Join(tagParts, delim))
	}

	if len(filtered) == 0 && m.filterText != "" {
		parts = append(parts, "")
		parts = append(parts, "  No matching tags")
	}

	return strings.Join(parts, "\n")
}

// flowWrap joins styled parts with a delimiter, wrapping to new lines
// when the visible width exceeds maxWidth.
func flowWrap(parts []string, delim string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 80
	}

	// Use lipgloss width measurement which handles ANSI correctly.
	visibleLen := lipgloss.Width

	delimVisible := visibleLen(delim)
	var lines []string
	currentLine := "  " // indent
	currentWidth := 2

	for i, part := range parts {
		partWidth := visibleLen(part)
		needsDelim := i > 0

		addedWidth := partWidth
		if needsDelim {
			addedWidth += delimVisible
		}

		if needsDelim && currentWidth+addedWidth > maxWidth {
			lines = append(lines, currentLine)
			currentLine = "  " + part
			currentWidth = 2 + partWidth
		} else {
			if needsDelim {
				currentLine += delim
				currentWidth += delimVisible
			}
			currentLine += part
			currentWidth += partWidth
		}
	}
	if currentLine != "  " {
		lines = append(lines, currentLine)
	}

	return strings.Join(lines, "\n")
}

// renderSections renders the filter bar (if active) and all non-empty sections.
// Returns the rendered string and the total number of tasks rendered.
func (m Model) renderSections() (string, int) {
	var parts []string

	if m.filtering {
		parts = append(parts, m.filterInput.View())
		parts = append(parts, "")
	}

	sections := m.displaySections()
	flatIdx := 0
	for _, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		hiddenCount := m.hiddenCountFor(sec.Title)
		rendered := m.renderSection(sec.Title, sec.Tasks, sec.Color, flatIdx, hiddenCount)
		if rendered != "" {
			parts = append(parts, rendered)
		}
		flatIdx += len(sec.Tasks)
	}

	return strings.Join(parts, "\n"), flatIdx
}

// truncateView ensures the rendered output fits within the terminal height.
// It finds the line containing the cursor marker (reverse video) and shows a
// window of m.height lines centered on it.
func (m Model) truncateView(s string) string {
	return m.truncateViewPinTop(s, 0)
}

// truncateViewPinTop ensures the rendered output fits within the terminal
// height, keeping the first pinnedTop lines always visible. The remaining
// lines scroll to keep the cursor-selected line on screen.
func (m Model) truncateViewPinTop(s string, pinnedTop int) string {
	if m.height <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	maxLines := m.height - 1
	if len(lines) <= maxLines {
		return s
	}

	if pinnedTop >= maxLines {
		pinnedTop = 0
	}

	top := lines[:pinnedTop]
	rest := lines[pinnedTop:]

	// Find the line with the selected cursor (reverse-video escape).
	cursorIdx := 0
	for i, line := range rest {
		if strings.Contains(line, "\x1b[7m") {
			cursorIdx = i
			break
		}
	}

	available := maxLines - pinnedTop
	start, end := scrollWindow(cursorIdx, len(rest), available)
	result := append(top, rest[start:end]...)
	return strings.Join(result, "\n")
}

// scrollWindow computes the start/end indices of a task window of size
// maxVisible, centered on cursor. Clamps to [0, total).
func scrollWindow(cursor, total, maxVisible int) (start, end int) {
	start = cursor - maxVisible/2
	if start < 0 {
		start = 0
	}
	end = start + maxVisible
	if end > total {
		end = total
		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}
	return
}
