package tui

import (
	"fmt"
	"strings"
	"time"

	"pike/internal/filter"
	"pike/internal/model"
	"pike/internal/query"
	tasksort "pike/internal/sort"

	tea "github.com/charmbracelet/bubbletea"
)

// rebuildSections recomputes sections from allTasks, applying filter text.
func (m *Model) rebuildSections() {
	m.filterBar, _ = m.filterBar.Update(FilterSetErrorMsg{Err: nil})
	now := m.nowFunc()

	switch m.mode {
	case modeAllTasks:
		var tasks []model.Task
		for _, t := range m.allTasks {
			if m.showAll {
				tasks = append(tasks, t)
			} else if t.HasCheckbox && t.State != model.Completed {
				tasks = append(tasks, t)
			}
		}
		title := "All Open Tasks"
		if m.showAll {
			title = "Tagged"
		}
		m.rebuildSingleSection(title, "cyan", tasks, now)

	case modeRecentlyCompleted:
		m.rebuildSingleSection("Recently Completed", "blue", m.allTasks, now)

	default:
		m.rebuildDashboard(now)
	}

	// Cache counts so View() doesn't rescan.
	weekStart := startOfWeek(now, m.weekStartDay())

	openCount := 0
	completedThisWeek := 0
	for _, t := range m.allTasks {
		if t.HasCheckbox && t.State == model.Open {
			openCount++
		}
		if t.HasCheckbox && t.State == model.Completed && t.Completed != nil && !t.Completed.Before(weekStart) {
			completedThisWeek++
		}
	}
	m.openCount = openCount
	m.completedThisWeek = completedThisWeek
}

// startOfWeek returns midnight of the most recent occurrence of the given weekday.
// weekday is 0=Sunday, 1=Monday, ..., 6=Saturday.
func startOfWeek(now time.Time, weekday int) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	current := int(today.Weekday())
	diff := (current - weekday + 7) % 7
	return today.AddDate(0, 0, -diff)
}

// weekStartDay returns the configured week start day, defaulting to Sunday (0).
func (m Model) weekStartDay() int {
	if m.config != nil {
		return m.config.WeekStartDay
	}
	return 0
}

// rebuildSingleSection builds a single-section view from the given tasks,
// applying the active filter (substring or DSL) and pin partitioning.
func (m *Model) rebuildSingleSection(title, color string, tasks []model.Task, now time.Time) {
	m.unfilteredSections = nil
	if m.filterBar.Text() != "" {
		if m.filterBar.Mode() == filterQuery {
			filtered, err := applyDSLFilter(tasks, m.filterBar.Text(), now)
			if err != nil {
				m.filterBar, _ = m.filterBar.Update(FilterSetErrorMsg{Err: err})
				return
			}
			tasks = filtered
		} else {
			tasks = applySubstringFilter(tasks, m.filterBar.Text())
		}
	}
	tasks = tasksort.StablePartitionPinned(tasks)
	m.sections = []filter.ViewResult{{Title: title, Color: color, Tasks: tasks}}
	m.applyHiddenFilter()
}

// rebuildDashboard builds the multi-section dashboard view.
func (m *Model) rebuildDashboard(now time.Time) {
	results, err := filter.ApplyViews(m.allTasks, m.config.Views, now)
	if err != nil {
		m.err = err
		return
	}

	// Cache the unfiltered results so visibleSections() can reuse them.
	m.unfilteredSections = results

	if m.filterBar.Text() != "" {
		for i := range results {
			var filtered []model.Task
			if m.filterBar.Mode() == filterQuery {
				var qErr error
				filtered, qErr = applyDSLFilter(results[i].Tasks, m.filterBar.Text(), now)
				if qErr != nil {
					m.filterBar, _ = m.filterBar.Update(FilterSetErrorMsg{Err: qErr})
					return
				}
			} else {
				filtered = applySubstringFilter(results[i].Tasks, m.filterBar.Text())
			}
			if filtered == nil {
				filtered = []model.Task{}
			}
			results[i].Tasks = tasksort.StablePartitionPinned(filtered)
		}
	}

	m.sections = results
	m.applyHiddenFilter()
}

// applyHiddenFilter counts @hidden tasks per section and, when showHidden is
// false, strips them from the section task lists. hiddenCounts is always
// populated so the renderer can show visibility icons without rescanning.
func (m *Model) applyHiddenFilter() {
	m.hiddenCounts = make([]int, len(m.sections))
	for i, sec := range m.sections {
		hidden := 0
		for _, t := range sec.Tasks {
			if t.HasTag("hidden") {
				hidden++
			}
		}
		m.hiddenCounts[i] = hidden
		if hidden > 0 && !m.showHidden {
			kept := make([]model.Task, 0, len(sec.Tasks)-hidden)
			for _, t := range sec.Tasks {
				if !t.HasTag("hidden") {
					kept = append(kept, t)
				}
			}
			m.sections[i].Tasks = kept
		}
	}
}

// applySubstringFilter filters tasks using space-separated substring matching (ANDed).
func applySubstringFilter(tasks []model.Task, filterText string) []model.Task {
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(filterText)))
	if len(tokens) == 0 {
		return tasks
	}
	var filtered []model.Task
	for _, t := range tasks {
		lower := strings.ToLower(t.Text)
		match := true
		for _, tok := range tokens {
			if !strings.Contains(lower, tok) {
				match = false
				break
			}
		}
		if match {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// applyDSLFilter filters tasks using the query DSL.
func applyDSLFilter(tasks []model.Task, filterText string, now time.Time) ([]model.Task, error) {
	filterText = strings.TrimSpace(filterText)
	node, err := query.Parse(filterText)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return tasks, nil
	}
	opts := query.EvalOptions{PartialTags: true}
	var filtered []model.Task
	for _, t := range tasks {
		if query.EvalWithOptions(node, &t, now, opts) {
			filtered = append(filtered, t)
		}
	}
	return filtered, nil
}

// flatTasks returns all tasks across displayed sections in order.
func (m Model) flatTasks() []model.Task {
	var tasks []model.Task
	for _, sec := range m.displaySections() {
		if len(sec.Tasks) > 0 {
			tasks = append(tasks, sec.Tasks...)
		}
	}
	return tasks
}

// pageScroll moves the cursor by half the visible task window. direction is 1 for down, -1 for up.
func (m *Model) pageScroll(direction int) {
	visible := max(4, m.height-pageScrollChrome)
	m.cursor += direction * (visible / 2)
	m.clampCursor()
}

// countFlatTasks returns the total number of tasks across displayed sections.
func (m Model) countFlatTasks() int {
	n := 0
	for _, sec := range m.displaySections() {
		n += len(sec.Tasks)
	}
	return n
}

// clampCursor ensures cursor is within valid bounds.
func (m *Model) clampCursor() {
	n := m.countFlatTasks()
	if n == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// cursorDown moves the cursor down one position if possible.
func (m *Model) cursorDown() {
	n := m.countFlatTasks()
	if n > 0 && m.cursor < n-1 {
		m.cursor++
	}
}

// cursorUp moves the cursor up one position if possible.
func (m *Model) cursorUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// cursorSection returns the index of the section the cursor is currently in,
// or -1 if no section contains the cursor.
func (m Model) cursorSection() int {
	flatIdx := 0
	for i, sec := range m.displaySections() {
		if len(sec.Tasks) == 0 {
			continue
		}
		if m.cursor >= flatIdx && m.cursor < flatIdx+len(sec.Tasks) {
			return i
		}
		flatIdx += len(sec.Tasks)
	}
	return -1
}

// jumpToNextSection moves the cursor to the first task of the next non-empty section.
func (m *Model) jumpToNextSection() {
	sections := m.displaySections()
	current := m.cursorSection()
	flatIdx := 0
	for i, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		if i > current {
			m.cursor = flatIdx
			return
		}
		flatIdx += len(sec.Tasks)
	}
}

// jumpToPrevSection moves the cursor to the first task of the previous non-empty section.
func (m *Model) jumpToPrevSection() {
	current := m.cursorSection()
	flatIdx := 0
	prevStart := -1
	for i, sec := range m.displaySections() {
		if len(sec.Tasks) == 0 {
			continue
		}
		if i >= current {
			break
		}
		prevStart = flatIdx
		flatIdx += len(sec.Tasks)
	}
	if prevStart >= 0 {
		m.cursor = prevStart
	}
}

// visibleSections returns non-empty sections from the cached unfiltered view set.
func (m Model) visibleSections() []filter.ViewResult {
	// Use cached unfiltered results when available (dashboard mode).
	if m.unfilteredSections != nil {
		var visible []filter.ViewResult
		for _, r := range m.unfilteredSections {
			if len(r.Tasks) > 0 {
				visible = append(visible, r)
			}
		}
		return visible
	}
	// Fallback: recompute (non-dashboard modes).
	now := m.nowFunc()
	results, err := filter.ApplyViews(m.allTasks, m.config.Views, now)
	if err != nil {
		return nil
	}
	var visible []filter.ViewResult
	for _, r := range results {
		if len(r.Tasks) > 0 {
			visible = append(visible, r)
		}
	}
	return visible
}

// extractTagNames returns the unique tag names from the given tasks.
func extractTagNames(tasks []model.Task) []string {
	seen := make(map[string]bool)
	for _, t := range tasks {
		for _, tag := range t.Tags {
			seen[tag.Name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names
}

// hiddenCountFor returns the number of hidden tasks for the section with the given title.
func (m Model) hiddenCountFor(title string) int {
	for i, sec := range m.sections {
		if sec.Title == title && i < len(m.hiddenCounts) {
			return m.hiddenCounts[i]
		}
	}
	return 0
}

// exitToDashboard resets the mode, clears any active filter, and rebuilds.
func (m *Model) exitToDashboard() {
	m.mode = modeDashboard
	m.filterBar, _ = m.filterBar.Update(FilterDeactivateMsg{})
	m.showAll = false
	m.rebuildSections()
	m.clampCursor()
}

// enterAllTasksMode switches to all-tasks mode with a focused filter input.
// When showAll is true (from tag search), completed tasks and tagged bullets are included.
func (m *Model) enterAllTasksMode(showAll bool, initialFilter string) tea.Cmd {
	m.mode = modeAllTasks
	m.showAll = showAll
	m.cursor = 0
	var cmd tea.Cmd
	m.filterBar, cmd = m.filterBar.Update(FilterActivateMsg{
		Mode:         filterSubstring,
		InitialValue: initialFilter,
		Placeholder:  "search tasks...",
	})
	m.rebuildSections()
	m.clampCursor()
	return cmd
}

// enterQueryMode switches to all-tasks mode with a pre-filled DSL query.
// sortOrder is accepted for future use but not yet applied (uses default file sort).
func (m *Model) enterQueryMode(query string, sortOrder string) tea.Cmd {
	m.mode = modeAllTasks
	m.showAll = true
	m.cursor = 0
	if sortOrder == "" {
		sortOrder = "file"
	}
	_ = sortOrder // TODO: wire sort override into rebuildSingleSection
	var cmd tea.Cmd
	m.filterBar, cmd = m.filterBar.Update(FilterActivateMsg{
		Mode:         filterQuery,
		InitialValue: query,
		Placeholder:  "query...",
	})
	m.rebuildSections()
	m.clampCursor()
	return cmd
}

// enterTagSearchMode switches to tag search mode with a focused filter input.
func (m *Model) enterTagSearchMode() tea.Cmd {
	m.mode = modeTagSearch
	m.showAll = false
	tags := extractTagNames(m.allTasks)
	var cmd tea.Cmd
	m.tagSearch, cmd = m.tagSearch.Update(TagSearchActivateMsg{Tags: tags})
	return cmd
}

// enterRecentlyCompletedMode switches to recently-completed view with a pre-filled query.
func (m *Model) enterRecentlyCompletedMode() tea.Cmd {
	queryStr := fmt.Sprintf("completed and @completed >= today-%dd", m.config.RecentlyCompletedDays)
	m.mode = modeRecentlyCompleted
	m.cursor = 0
	var cmd tea.Cmd
	m.filterBar, cmd = m.filterBar.Update(FilterActivateMsg{
		Mode:         filterQuery,
		InitialValue: queryStr,
		Placeholder:  "type to filter...",
	})
	m.rebuildSections()
	m.clampCursor()
	return cmd
}

func (m Model) nowFunc() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}
