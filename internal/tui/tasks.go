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
		filtered, err := applyQueryFilter(tasks, m.filterText, now)
		if err != nil {
			m.queryErr = err
			return // preserve existing sections
		}
		tasks = filtered
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
			filtered, err := applyQueryFilter(results[i].Tasks, m.filterText, now)
			if err != nil {
				m.queryErr = err
				return // preserve existing sections
			}
			if filtered == nil {
				filtered = []model.Task{}
			}
			results[i].Tasks = filtered
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

// hasDSLTokens checks if input contains DSL-specific tokens that distinguish
// it from a plain text search. Uses word-boundary matching for keywords.
func hasDSLTokens(input string) bool {
	if strings.ContainsAny(input, "@<>") {
		return true
	}
	// Detect /regex/ patterns (paired slashes)
	if strings.Count(input, "/") >= 2 {
		return true
	}
	for _, word := range strings.Fields(input) {
		lower := strings.ToLower(word)
		if lower == "and" || lower == "or" || lower == "not" || lower == "open" || lower == "completed" {
			return true
		}
	}
	return false
}

// applyQueryFilter filters tasks using DSL parsing with fallback to substring matching.
// Returns (filtered tasks, parse error or nil).
// When DSL parsing fails and input has DSL tokens, returns (nil, error) to signal
// the caller should preserve existing sections.
func applyQueryFilter(tasks []model.Task, filterText string, now time.Time) ([]model.Task, error) {
	// Try DSL parsing first
	node, err := query.Parse(filterText)
	if err == nil && node != nil {
		opts := query.EvalOptions{PartialTags: true}
		var filtered []model.Task
		for _, t := range tasks {
			if query.EvalWithOptions(node, &t, now, opts) {
				filtered = append(filtered, t)
			}
		}
		return filtered, nil
	}

	// DSL parse failed — check if input looks like DSL
	if hasDSLTokens(filterText) {
		return nil, err // signal to preserve existing sections
	}

	// Fallback: simple substring matching (space-separated tokens, ANDed)
	tokens := strings.Fields(strings.ToLower(filterText))
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

// clampCursor ensures cursor is within valid bounds.
func (m *Model) clampCursor() {
	flatTasks := m.flatTasks()
	if len(flatTasks) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(flatTasks) {
		m.cursor = len(flatTasks) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// jumpToNextSection moves the cursor to the first task of the next non-empty section.
func (m *Model) jumpToNextSection() {
	sections := m.displaySections()
	if len(sections) == 0 {
		return
	}

	// Find which section the cursor is currently in.
	flatIdx := 0
	currentSection := -1
	for i, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		sectionEnd := flatIdx + len(sec.Tasks)
		if m.cursor >= flatIdx && m.cursor < sectionEnd {
			currentSection = i
			break
		}
		flatIdx += len(sec.Tasks)
	}

	// Find the next non-empty section.
	flatIdx = 0
	for i, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		if i > currentSection {
			m.cursor = flatIdx
			return
		}
		flatIdx += len(sec.Tasks)
	}
}

// jumpToPrevSection moves the cursor to the first task of the previous non-empty section.
func (m *Model) jumpToPrevSection() {
	sections := m.displaySections()
	if len(sections) == 0 {
		return
	}

	// Find which section the cursor is currently in.
	flatIdx := 0
	currentSection := -1
	for i, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		sectionEnd := flatIdx + len(sec.Tasks)
		if m.cursor >= flatIdx && m.cursor < sectionEnd {
			currentSection = i
			break
		}
		flatIdx += len(sec.Tasks)
	}

	// Find the previous non-empty section.
	flatIdx = 0
	prevStart := -1
	for i, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		if i >= currentSection {
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

// countOverdue returns the number of open tasks past their due date.
func (m Model) countOverdue() int {
	now := m.nowFunc()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	count := 0
	for _, t := range m.allTasks {
		if t.State == model.Open && t.Due != nil && t.Due.Before(today) {
			count++
		}
	}
	return count
}

// countDueThisWeek returns the number of open tasks due within 7 days.
func (m Model) countDueThisWeek() int {
	now := m.nowFunc()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfWeek := today.AddDate(0, 0, 7)
	count := 0
	for _, t := range m.allTasks {
		if t.State == model.Open && t.Due != nil && !t.Due.Before(today) && t.Due.Before(endOfWeek) {
			count++
		}
	}
	return count
}

// countCompletedThisWeek returns the number of tasks completed within the last 7 days.
func (m Model) countCompletedThisWeek() int {
	now := m.nowFunc()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekAgo := today.AddDate(0, 0, -7)
	count := 0
	for _, t := range m.allTasks {
		if t.State == model.Completed && t.Completed != nil && !t.Completed.Before(weekAgo) {
			count++
		}
	}
	return count
}

// clearFilter resets filter state and blurs the input.
func (m *Model) clearFilter() {
	m.filtering = false
	m.filterText = ""
	m.showAll = false
	m.filterInput.SetValue("")
	m.filterInput.Prompt = "/ "
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
	m.filterText = initialFilter
	m.filterInput.Prompt = "/ "
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
	m.filterInput.Prompt = "/ "
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
	m.filterInput.Prompt = "/ "
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
