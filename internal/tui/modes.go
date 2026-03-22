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

// enterAllTasksMode switches to all-tasks mode with a focused filter input.
// When showAll is true (from tag search), completed tasks and tagged bullets are included.
func (m *Model) enterAllTasksMode(showAll bool, initialFilter string) tea.Cmd {
	m.mode = modeAllTasks
	m.showAll = showAll
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

// enterQueryMode switches to all-tasks mode with a pre-filled DSL query.
// sortOrder is accepted for future use but not yet applied (uses default file sort).
func (m *Model) enterQueryMode(query string, sortOrder string) tea.Cmd {
	m.mode = modeAllTasks
	m.showAll = true
	m.nav.JumpToTop()
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

// exitToDashboard resets the mode, clears any active filter, and rebuilds.
func (m *Model) exitToDashboard() {
	m.mode = modeDashboard
	m.filterBar, _ = m.filterBar.Update(FilterDeactivateMsg{})
	m.showAll = false
	m.rebuildSections()
	m.nav.ClampCursor(m.displaySections())
}
