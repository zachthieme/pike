package tui

import (
	"fmt"
	"strings"

	"pike/internal/filter"
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
	footer := FooterStyle().Render(fmt.Sprintf("%s %d open", strings.Repeat("\u2500", max(0, m.width-10)), openCount))

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
	//           + scroll footer (1) + bubbletea rendering (2) = 8
	overhead := 8

	maxVisible := m.height - overhead
	if maxVisible < 3 {
		maxVisible = 3
	}

	hiddenCount := m.hiddenCountFor(sec.Title)

	if len(tasks) <= maxVisible {
		rendered := RenderSection(sec.Title, tasks, sec.Color, m.cursor, 0, m.tagColors, m.width, m.config.LinkColor, hiddenCount)
		parts = append(parts, rendered)
	} else {
		start, end := scrollWindow(m.cursor, len(tasks), maxVisible)
		rendered := RenderSection(sec.Title, tasks[start:end], sec.Color, m.cursor, start, m.tagColors, m.width, m.config.LinkColor, hiddenCount)
		parts = append(parts, rendered)
		parts = append(parts, FooterStyle().Render(fmt.Sprintf("  %d–%d of %d tasks", start+1, end, len(tasks))))
	}

	// Pin the search bar (first 2 lines) and truncate the rest to fit.
	return m.truncateViewPinTop(strings.Join(parts, "\n"), 2)
}

// viewTagSearch renders the tag picker with filter bar.
func (m Model) viewTagSearch() string {
	var parts []string

	parts = append(parts, m.filterInput.View())
	parts = append(parts, "")

	tags := m.filteredTags()
	if len(tags) == 0 {
		parts = append(parts, "  No matching tags")
	} else {
		for i, tag := range tags {
			line := fmt.Sprintf("  @%s", tag)
			if i == m.tagCursor {
				line = TaskStyle(true).Render(fmt.Sprintf("▸ @%s", tag))
			}
			parts = append(parts, line)
		}
	}

	return strings.Join(parts, "\n")
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
		rendered := RenderSection(sec.Title, sec.Tasks, sec.Color, m.cursor, flatIdx, m.tagColors, m.width, m.config.LinkColor, hiddenCount)
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
