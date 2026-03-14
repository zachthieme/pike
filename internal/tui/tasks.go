package tui

import (
	gosort "sort"
	"strings"
	"time"

	"pike/internal/filter"
	"pike/internal/model"
)

// rebuildSections recomputes sections from allTasks, applying filter text.
func (m *Model) rebuildSections() {
	// In all-tasks mode, show tasks in a single section.
	// When showAll is true (e.g. from tag search), include completed tasks too.
	if m.mode == modeAllTasks {
		var tasks []model.Task
		for _, t := range m.allTasks {
			if m.showAll || t.State != model.Completed {
				tasks = append(tasks, t)
			}
		}

		if m.filterText != "" {
			tokens := strings.Fields(m.filterText)
			var filtered []model.Task
			for _, t := range tasks {
				if matchesFilter(t, tokens) {
					filtered = append(filtered, t)
				}
			}
			tasks = filtered
		}

		title := "All Open Tasks"
		if m.showAll {
			title = "All Tasks"
		}
		sections := []filter.ViewResult{{
			Title: title,
			Color: "cyan",
			Tasks: tasks,
		}}
		m.sections = sections
		m.applyHiddenFilter()
		return
	}

	now := m.nowFunc()
	results, err := filter.ApplyViews(m.allTasks, m.config.Views, now)
	if err != nil {
		m.err = err
		return
	}

	// Apply text filter if active. Tokens are space-separated and ANDed:
	//   "foo bar"  → must contain both "foo" and "bar"
	//   "!bob"     → must NOT contain "bob"
	//   "@tag"     → must have a tag named "tag"
	//   "!@tag"    → must NOT have a tag named "tag"
	if m.filterText != "" {
		tokens := strings.Fields(m.filterText)
		for i := range results {
			var filtered []model.Task
			for _, t := range results[i].Tasks {
				if matchesFilter(t, tokens) {
					filtered = append(filtered, t)
				}
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

// matchesFilter checks whether a task matches all filter tokens.
func matchesFilter(t model.Task, tokens []string) bool {
	lower := strings.ToLower(t.Text)
	for _, tok := range tokens {
		negate := false
		term := tok

		if strings.HasPrefix(term, "!") {
			negate = true
			term = term[1:]
		}
		if term == "" {
			continue
		}

		var match bool
		if strings.HasPrefix(term, "@") {
			// Tag match: check parsed tags by name (partial substring match).
			tagName := strings.ToLower(term[1:])
			for _, tag := range t.Tags {
				if strings.Contains(strings.ToLower(tag.Name), tagName) {
					match = true
					break
				}
			}
		} else {
			// Substring match on task text.
			match = strings.Contains(lower, strings.ToLower(term))
		}

		if negate && match {
			return false
		}
		if !negate && !match {
			return false
		}
	}
	return true
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
	gosort.Strings(m.tagList)
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

// countOpen returns the total count of open tasks.
func (m Model) countOpen() int {
	count := 0
	for _, t := range m.allTasks {
		if t.State == model.Open {
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

func (m Model) nowFunc() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}
