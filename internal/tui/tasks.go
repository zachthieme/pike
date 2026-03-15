package tui

import (
	"fmt"
	"slices"
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
	m.queryErr = nil
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
}

// rebuildSingleSection builds a single-section view from the given tasks,
// applying the query filter and pin partitioning.
func (m *Model) rebuildSingleSection(title, color string, tasks []model.Task, now time.Time) {
	if m.filterText != "" {
		if m.filterMode == filterQuery {
			filtered, err := applyDSLFilter(tasks, m.filterText, now)
			if err != nil {
				m.queryErr = err
				return
			}
			tasks = filtered
		} else {
			tasks = applySubstringFilter(tasks, m.filterText)
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

	if m.filterText != "" {
		for i := range results {
			var filtered []model.Task
			if m.filterMode == filterQuery {
				var qErr error
				filtered, qErr = applyDSLFilter(results[i].Tasks, m.filterText, now)
				if qErr != nil {
					m.queryErr = qErr
					return
				}
			} else {
				filtered = applySubstringFilter(results[i].Tasks, m.filterText)
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

// applyHiddenFilter strips @hidden tasks from each section when showHidden is
// false, and populates hiddenCounts so the renderer can show a lock icon.
func (m *Model) applyHiddenFilter() {
	m.hiddenCounts = make([]int, len(m.sections))
	if m.showHidden {
		return
	}
	for i, sec := range m.sections {
		var kept []model.Task
		hidden := 0
		for _, t := range sec.Tasks {
			if t.HasTag("hidden") {
				hidden++
			} else {
				kept = append(kept, t)
			}
		}
		if hidden > 0 {
			if kept == nil {
				kept = []model.Task{}
			}
			m.sections[i].Tasks = kept
			m.hiddenCounts[i] = hidden
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

// pageScrollChrome is the approximate number of non-task lines on screen:
// search bar, section header, borders, footer, bubbletea chrome.
const pageScrollChrome = 8

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

// visibleSections returns non-empty sections from the full (unfiltered) view set.
func (m Model) visibleSections() []filter.ViewResult {
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

// buildTagList extracts unique tag names from all tasks, sorted alphabetically.
func (m *Model) buildTagList() {
	seen := make(map[string]bool)
	for _, t := range m.allTasks {
		for _, tag := range t.Tags {
			seen[tag.Name] = true
		}
	}
	m.tagList = make([]string, 0, len(seen))
	for name := range seen {
		m.tagList = append(m.tagList, name)
	}
	slices.Sort(m.tagList)
}

// filteredTags returns the tag list filtered by current filter text.
func (m Model) filteredTags() []string {
	if m.filterText == "" {
		return m.tagList
	}
	// Strip leading @ so users can type "@due" or "due" interchangeably.
	lower := strings.ToLower(strings.TrimPrefix(m.filterText, "@"))
	var result []string
	for _, tag := range m.tagList {
		if strings.Contains(strings.ToLower(tag), lower) {
			result = append(result, tag)
		}
	}
	return result
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

// countOpen returns the total count of open checkbox tasks.
func (m Model) countOpen() int {
	count := 0
	for _, t := range m.allTasks {
		if t.HasCheckbox && t.State == model.Open {
			count++
		}
	}
	return count
}



// setFilterMode sets the filter mode, updates the prompt, and focuses the input.
func (m *Model) setFilterMode(mode filterMode) tea.Cmd {
	m.filtering = true
	m.filterMode = mode
	m.filterInput.Prompt = filterPrompt[mode]
	return m.filterInput.Focus()
}

// clearFilter resets filter state and blurs the input.
func (m *Model) clearFilter() {
	m.filtering = false
	m.filterText = ""
	m.filterMode = filterSubstring
	m.showAll = false
	m.filterInput.SetValue("")
	m.filterInput.Prompt = filterPrompt[filterSubstring]
	m.filterInput.Placeholder = "type to filter..."
	m.filterInput.Blur()
}

// enterAllTasksMode switches to all-tasks mode with a focused filter input.
// When showAll is true (from tag search), completed tasks and tagged bullets are included.
func (m *Model) enterAllTasksMode(showAll bool, initialFilter string) tea.Cmd {
	m.mode = modeAllTasks
	m.showAll = showAll
	m.filtering = true
	m.filterInput.SetValue(initialFilter)
	m.filterInput.CursorEnd()
	m.filterText = initialFilter
	m.filterMode = filterSubstring
	m.filterInput.Prompt = filterPrompt[filterSubstring]
	m.filterInput.Placeholder = "search tasks..."
	m.cursor = 0
	m.rebuildSections()
	m.clampCursor()
	return m.filterInput.Focus()
}

// enterTagSearchMode switches to tag search mode with a focused filter input.
func (m *Model) enterTagSearchMode() tea.Cmd {
	m.mode = modeTagSearch
	m.showAll = false
	m.buildTagList()
	m.filtering = true
	m.filterInput.SetValue("")
	m.filterText = ""
	m.filterMode = filterSubstring
	m.filterInput.Prompt = filterPrompt[filterSubstring]
	m.filterInput.Placeholder = "search tags..."
	m.tagCursor = 0
	return m.filterInput.Focus()
}

// enterRecentlyCompletedMode switches to recently-completed view with a pre-filled query.
func (m *Model) enterRecentlyCompletedMode() tea.Cmd {
	queryStr := fmt.Sprintf("completed and @completed >= today-%dd", m.config.RecentlyCompletedDays)

	m.mode = modeRecentlyCompleted
	m.filtering = true
	m.filterInput.SetValue(queryStr)
	m.filterText = queryStr
	m.filterMode = filterQuery
	m.filterInput.Prompt = filterPrompt[filterQuery]
	m.filterInput.Placeholder = "type to filter..."
	m.cursor = 0
	m.rebuildSections()
	m.clampCursor()
	return m.filterInput.Focus()
}

// resolveTagColor returns the configured color for a tag name, falling back to
// "_default". Returns empty string if no color is configured.
func (m Model) resolveTagColor(tagName string) string {
	if color, ok := m.tagColors[tagName]; ok {
		return color
	}
	if color, ok := m.tagColors["_default"]; ok {
		return color
	}
	return ""
}

func (m Model) nowFunc() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}
