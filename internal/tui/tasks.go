package tui

import (
	"time"

	"pike/internal/filter"
	"pike/internal/model"
)

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

func (m Model) nowFunc() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}
