package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/zachthieme/pike/internal/filter"
	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/query"
	tasksort "github.com/zachthieme/pike/internal/sort"

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

}

// rebuildSingleSection builds a single-section view from the given tasks,
// applying the active filter (substring or DSL) and pin partitioning.
func (m *Model) rebuildSingleSection(title, color string, tasks []model.Task, now time.Time) {
	m.unfilteredSections = nil
	filtered, ok := m.filterTasks(tasks, now)
	if !ok {
		return
	}
	if m.sortOverride != "" {
		if err := tasksort.Sort(filtered, m.sortOverride); err != nil {
			m.err = err
			return
		}
	}
	tasks = tasksort.StablePartitionPinned(filtered)
	tasks = m.regroupChildren(tasks)
	m.sections = []filter.ViewResult{{Title: title, Color: color, Tasks: tasks}}
	m.applyHiddenFilter()
	m.applyCollapseFilter()
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
			filtered, ok := m.filterTasks(results[i].Tasks, now)
			if !ok {
				return
			}
			if filtered == nil {
				filtered = []model.Task{}
			}
			results[i].Tasks = tasksort.StablePartitionPinned(filtered)
		}
	}

	// Regroup children next to their parents after sort
	for i := range results {
		results[i].Tasks = m.regroupChildren(results[i].Tasks)
	}

	m.sections = results
	m.applyHiddenFilter()
	m.applyCollapseFilter()
}

// filterTasks applies the active filter (substring or DSL) to a task slice.
// Returns the filtered tasks and true, or nil and false if a DSL parse error
// occurred (the error is set on the filter bar).
func (m *Model) filterTasks(tasks []model.Task, now time.Time) ([]model.Task, bool) {
	if m.filterBar.Text() == "" {
		return tasks, true
	}
	if m.filterBar.Mode() == filterQuery {
		filtered, err := applyDSLFilter(tasks, m.filterBar.Text(), now)
		if err != nil {
			m.filterBar, _ = m.filterBar.Update(FilterSetErrorMsg{Err: err})
			return nil, false
		}
		return filtered, true
	}
	return applySubstringFilter(tasks, m.filterBar.Text()), true
}

// regroupChildren moves children adjacent to their parent within a section.
// Children whose parent is not in the section remain in their sorted position.
func (m *Model) regroupChildren(tasks []model.Task) []model.Task {
	if len(tasks) == 0 {
		return tasks
	}

	// Build set of parent keys present in this task list
	parentKeys := make(map[string]bool)
	for _, t := range tasks {
		if len(t.Children) > 0 {
			parentKeys[fmt.Sprintf("%s:%d", t.File, t.Line)] = true
		}
	}

	// Separate children (whose parent is in this list) from the rest
	childrenOf := make(map[string][]model.Task)
	var ordered []model.Task
	for _, t := range tasks {
		if t.Indent > 0 && t.ParentIndex >= 0 {
			parent := m.allTasks[t.ParentIndex]
			key := fmt.Sprintf("%s:%d", parent.File, parent.Line)
			if parentKeys[key] {
				childrenOf[key] = append(childrenOf[key], t)
				continue
			}
		}
		ordered = append(ordered, t)
	}

	// Reinsert children after their parents
	result := make([]model.Task, 0, len(tasks))
	for _, t := range ordered {
		result = append(result, t)
		key := fmt.Sprintf("%s:%d", t.File, t.Line)
		if children, ok := childrenOf[key]; ok {
			result = append(result, children...)
		}
	}
	return result
}

// applyCollapseFilter removes children of collapsed parents from sections.
// A parent is collapsed by default; expanded parents are in m.expanded.
// Children whose parent is not in the same section are not affected.
func (m *Model) applyCollapseFilter() {
	for i, sec := range m.sections {
		// Build set of tasks present in this section
		present := make(map[string]bool)
		for _, t := range sec.Tasks {
			present[fmt.Sprintf("%s:%d", t.File, t.Line)] = true
		}

		var kept []model.Task
		for _, t := range sec.Tasks {
			if t.Indent > 0 && t.ParentIndex >= 0 {
				parent := m.allTasks[t.ParentIndex]
				parentKey := fmt.Sprintf("%s:%d", parent.File, parent.Line)
				if present[parentKey] && !m.expanded[parentKey] {
					continue // parent visible and collapsed — skip child
				}
			}
			kept = append(kept, t)
		}
		m.sections[i].Tasks = kept
	}
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
		match := true
		for _, tok := range tokens {
			if !strings.Contains(t.LowerText, tok) {
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

// enterAllTasksMode switches to all-tasks mode with a focused filter input.
// When showAll is true (from tag search), completed tasks and tagged bullets are included.
func (m *Model) enterAllTasksMode(showAll bool, initialFilter string) tea.Cmd {
	m.mode = modeAllTasks
	m.showAll = showAll
	m.sortOverride = ""
	m.nav.JumpToTop()
	var cmd tea.Cmd
	m.filterBar, cmd = m.filterBar.Update(FilterActivateMsg{
		Mode:         filterSubstring,
		InitialValue: initialFilter,
		Placeholder:  "search tasks...",
	})
	m.rebuildSections()
	m.nav.ClampCursor(m.displaySections())
	return cmd
}

// enterQueryMode switches to all-tasks mode with a pre-filled DSL query
// and an optional sort override from custom bindings.
func (m *Model) enterQueryMode(query string, sortOrder string) tea.Cmd {
	m.mode = modeAllTasks
	m.showAll = true
	m.sortOverride = sortOrder
	m.nav.JumpToTop()
	var cmd tea.Cmd
	m.filterBar, cmd = m.filterBar.Update(FilterActivateMsg{
		Mode:         filterQuery,
		InitialValue: query,
		Placeholder:  "query...",
	})
	m.rebuildSections()
	m.nav.ClampCursor(m.displaySections())
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
	m.nav.JumpToTop()
	var cmd tea.Cmd
	m.filterBar, cmd = m.filterBar.Update(FilterActivateMsg{
		Mode:         filterQuery,
		InitialValue: queryStr,
		Placeholder:  "type to filter...",
	})
	m.rebuildSections()
	m.nav.ClampCursor(m.displaySections())
	return cmd
}

// exitToDashboard resets all mode state back to the base dashboard view.
func (m *Model) exitToDashboard() {
	m.mode = modeDashboard
	m.focusedView = ""
	m.filterBar, _ = m.filterBar.Update(FilterDeactivateMsg{})
	m.showAll = false
	m.sortOverride = ""
	m.rebuildSections()
	m.nav.ClampCursor(m.displaySections())
}
