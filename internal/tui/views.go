package tui

import (
	"fmt"
	"strings"

	"github.com/zachthieme/pike/internal/filter"

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
	case modeFocused:
		return errLine + m.viewFocused()
	case modeAllTasks:
		return errLine + m.viewAllTasks()
	case modeTagSearch:
		return errLine + m.tagSearch.View(m.tagColors, m.width)
	case modeRecentlyCompleted:
		return errLine + m.viewAllTasks()
	default:
		return errLine + m.viewDashboard()
	}
}

func (m Model) viewDashboard() string {
	body, _ := m.renderSections()

	var footer string
	if m.filterBar.QueryErr() != nil {
		footer = "\n" + FooterStyle().Render("  "+m.filterBar.QueryErr().Error())
	}

	full := body + footer
	return m.truncateView(full)
}

func (m Model) viewFocused() string {
	body, count := m.renderSections()

	var footer string
	if m.filterBar.QueryErr() != nil {
		footer = "\n" + FooterStyle().Render("  "+m.filterBar.QueryErr().Error())
	}

	if count == 0 {
		return body + "\nNo results" + footer
	}

	full := body + footer
	return m.truncateView(full)
}

func (m Model) viewSummary() string {
	return RenderSummary(m.version, m.width, m.keys, m.customBindings)
}

// displaySections returns the sections to display based on the current mode.
func (m Model) displaySections() []filter.ViewResult {
	if m.mode == modeFocused {
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

// Layout constants for viewAllTasks windowing calculations.
const (
	allTasksChrome     = 3 // search bar + footer + bubbletea chrome
	sectionChrome      = 4 // section header + newline + top/bottom borders
	minAvailableHeight = 5
)

// taskViewport computes the windowing parameters for a task list:
// how many rows are available and how many tasks can be displayed.
type taskViewport struct {
	available int // total available height after chrome
	maxTasks  int // maximum number of tasks that fit
}

func (m Model) computeTaskViewport(taskCount int) taskViewport {
	available := m.height - allTasksChrome
	if available < minAvailableHeight {
		available = minAvailableHeight
	}
	maxTasks := min(taskCount, available-sectionChrome)
	if maxTasks < 1 {
		maxTasks = 1
	}
	return taskViewport{available: available, maxTasks: maxTasks}
}

// viewAllTasks renders a single-section task list with filter bar.
// Used for both all-tasks and recently-completed modes.
// Tasks are windowed to fit the terminal height, keeping the cursor visible.
func (m Model) viewAllTasks() string {
	var parts []string

	// Always show the search bar in all-tasks mode.
	parts = append(parts, m.filterBar.View())

	if m.filterBar.QueryErr() != nil {
		parts = append(parts, FooterStyle().Render("  "+m.filterBar.QueryErr().Error()))
	}

	sections := m.displaySections()
	if len(sections) == 0 || len(sections[0].Tasks) == 0 {
		parts = append(parts, "  No results")
		return strings.Join(parts, "\n")
	}

	sec := sections[0]
	tasks := sec.Tasks
	hiddenCount := m.hiddenCountFor(sec.Title)
	vp := m.computeTaskViewport(len(tasks))

	for vp.maxTasks > 1 {
		start, end := scrollWindow(m.nav.Cursor(), len(tasks), vp.maxTasks)
		rendered := m.renderSection(sec.Title, tasks[start:end], sec.Color, start, hiddenCount, len(tasks))
		renderedHeight := lipgloss.Height(rendered)
		needsFooter := end-start < len(tasks)
		if needsFooter {
			renderedHeight++ // footer line
		}
		if renderedHeight <= vp.available {
			parts = append(parts, rendered)
			if needsFooter {
				parts = append(parts, FooterStyle().Render(fmt.Sprintf("  %d–%d of %d results", start+1, end, len(tasks))))
			}
			return strings.Join(parts, "\n")
		}
		vp.maxTasks--
	}

	// Minimal case: 1 task.
	start, end := scrollWindow(m.nav.Cursor(), len(tasks), 1)
	rendered := m.renderSection(sec.Title, tasks[start:end], sec.Color, start, hiddenCount, len(tasks))
	parts = append(parts, rendered)
	if len(tasks) > 1 {
		parts = append(parts, FooterStyle().Render(fmt.Sprintf("  %d–%d of %d results", start+1, end, len(tasks))))
	}
	return strings.Join(parts, "\n")
}

// flowWrap joins styled parts with a delimiter, wrapping to new lines
// when the visible width exceeds maxWidth.
const defaultFlowWidth = 80

func flowWrap(parts []string, delim string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = defaultFlowWidth
	}

	// Use lipgloss width measurement which handles ANSI correctly.
	visibleLen := lipgloss.Width

	delimVisible := visibleLen(delim)
	var lines []string
	currentLine := ""
	currentWidth := 0

	for i, part := range parts {
		partWidth := visibleLen(part)
		needsDelim := i > 0

		addedWidth := partWidth
		if needsDelim {
			addedWidth += delimVisible
		}

		if needsDelim && currentWidth+addedWidth > maxWidth {
			lines = append(lines, currentLine)
			currentLine = part
			currentWidth = partWidth
		} else {
			if needsDelim {
				currentLine += delim
				currentWidth += delimVisible
			}
			currentLine += part
			currentWidth += partWidth
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	// Center each line within maxWidth.
	for i, line := range lines {
		lines[i] = lipgloss.PlaceHorizontal(maxWidth, lipgloss.Center, line)
	}

	return strings.Join(lines, "\n")
}

// renderSections renders the filter bar (if active) and all non-empty sections.
// Returns the rendered string and the total number of tasks rendered.
func (m Model) renderSections() (string, int) {
	var parts []string

	if m.filterBar.Active() {
		parts = append(parts, m.filterBar.View())
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
	if m.height <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	maxLines := m.height - 1
	if len(lines) <= maxLines {
		return s
	}

	// Find the line with the selected cursor (reverse-video escape).
	cursorIdx := 0
	for i, line := range lines {
		if strings.Contains(line, reverseVideoEsc) {
			cursorIdx = i
			break
		}
	}

	start, end := scrollWindow(cursorIdx, len(lines), maxLines)
	return strings.Join(lines[start:end], "\n")
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
