package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zachthieme/pike/internal/config"
	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/style"
)

// viewTestTasks returns tasks with HasCheckbox set so all-tasks mode includes them.
func viewTestTasks() []model.Task {
	return []model.Task{
		model.TaskWith(model.Task{
			Text:        "Overdue task @due(2026-03-10)",
			State:       model.Open,
			File:        "notes/todo.md",
			Line:        1,
			Tags:        []model.Tag{{Name: "due", Value: "2026-03-10"}},
			Due:         timePtr(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)),
			HasCheckbox: true,
		}),
		model.TaskWith(model.Task{
			Text:        "Today task @today",
			State:       model.Open,
			File:        "notes/todo.md",
			Line:        2,
			Tags:        []model.Tag{{Name: "today"}},
			HasCheckbox: true,
		}),
		model.TaskWith(model.Task{
			Text:        "Future task @due(2026-03-20)",
			State:       model.Open,
			File:        "notes/todo.md",
			Line:        3,
			Tags:        []model.Tag{{Name: "due", Value: "2026-03-20"}},
			Due:         timePtr(time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)),
			HasCheckbox: true,
		}),
		model.TaskWith(model.Task{
			Text:        "Done task @completed(2026-03-12)",
			State:       model.Completed,
			File:        "notes/todo.md",
			Line:        4,
			Tags:        []model.Tag{{Name: "completed", Value: "2026-03-12"}},
			Completed:   timePtr(time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)),
			HasCheckbox: true,
		}),
	}
}

// viewTestModel creates a model with a terminal size set so View() produces output.
func viewTestModel(tasks []model.Task, views []config.ViewConfig) Model {
	cfg := &config.Config{
		Editor:                "vi",
		TagColors:             map[string]string{"due": "red", "today": "green", "_default": "cyan"},
		Views:                 views,
		RecentlyCompletedDays: 7,
	}
	m := NewModel(cfg, tasks, nil, nil)
	m.now = func() time.Time { return testNow }
	m.width = 80
	m.height = 40
	m.nav.SetHeight(40)
	m.rebuildSections()
	m.nav.ClampCursor(m.displaySections())
	return m
}

// stripped returns the View() output with ANSI escapes removed for assertion.
func stripped(m Model) string {
	return style.StripANSI(m.View())
}

func TestViewDashboardContainsSectionHeaders(t *testing.T) {
	m := viewTestModel(viewTestTasks(), testViews())
	output := stripped(m)

	for _, title := range []string{"Overdue", "Open", "Completed"} {
		if !strings.Contains(output, title) {
			t.Errorf("dashboard View() missing section header %q", title)
		}
	}
}

func TestViewDashboardContainsTaskText(t *testing.T) {
	m := viewTestModel(viewTestTasks(), testViews())
	output := stripped(m)

	if !strings.Contains(output, "Overdue task") {
		t.Error("dashboard View() missing 'Overdue task' text")
	}
	if !strings.Contains(output, "Done task") {
		t.Error("dashboard View() missing 'Done task' text")
	}
}

func TestViewDashboardContainsTaskMarkers(t *testing.T) {
	m := viewTestModel(viewTestTasks(), testViews())
	output := stripped(m)

	// TUI mode uses ○ for open and ● for completed.
	if !strings.Contains(output, "○") {
		t.Error("dashboard View() missing open task marker ○")
	}
	if !strings.Contains(output, "●") {
		t.Error("dashboard View() missing completed task marker ●")
	}
}

func TestViewFocusedShowsSingleSection(t *testing.T) {
	m := viewTestModel(viewTestTasks(), testViews())

	// Press "1" to focus on "Overdue" section.
	updated, _ := sendKey(m, "1")
	m2 := updated.(Model)
	output := stripped(m2)

	if !strings.Contains(output, "Overdue") {
		t.Error("focused View() missing 'Overdue' header")
	}
	if !strings.Contains(output, "Overdue task") {
		t.Error("focused View() missing overdue task text")
	}
}

func TestViewAllTasksMode(t *testing.T) {
	m := viewTestModel(viewTestTasks(), testViews())

	// Enter all-tasks mode directly instead of via key to avoid filterBar state.
	m.mode = modeAllTasks
	m.showAll = false
	m.rebuildSections()
	m.nav.ClampCursor(m.displaySections())
	output := stripped(m)

	if !strings.Contains(output, "All Open Tasks") {
		t.Errorf("all-tasks View() missing 'All Open Tasks' header; got:\n%s", output)
	}
}

func TestViewSummaryShowsVersion(t *testing.T) {
	m := viewTestModel(viewTestTasks(), testViews())
	m.version = "v1.2.3"

	// Press "s" to toggle summary.
	updated, _ := sendKey(m, "s")
	m2 := updated.(Model)
	output := stripped(m2)

	if !strings.Contains(output, "v1.2.3") {
		t.Error("summary View() missing version string")
	}
}

func TestViewCursorMarkerPresent(t *testing.T) {
	m := viewTestModel(viewTestTasks(), testViews())
	output := stripped(m)

	// The first task's marker should be present in the output.
	// In TUI mode, open tasks get ○ and completed get ●.
	if !strings.Contains(output, "○") {
		t.Error("View() missing cursor task marker")
	}
}

func TestViewEmptyTasksRendersWithoutPanic(t *testing.T) {
	// Verify that rendering with no matching tasks does not panic.
	m := viewTestModel(nil, []config.ViewConfig{
		{Title: "Empty", Query: "open and @nonexistent", Sort: "file", Color: "cyan", Order: 1},
	})
	_ = stripped(m)
}

func TestViewWindowSizeAffectsOutput(t *testing.T) {
	m := viewTestModel(viewTestTasks(), testViews())
	small := m
	small.width = 40
	small.height = 10
	small.nav.SetHeight(10)

	large := m
	large.width = 120
	large.height = 50
	large.nav.SetHeight(50)

	smallOut := stripped(small)
	largeOut := stripped(large)

	if smallOut == "" {
		t.Error("small viewport View() returned empty")
	}
	if largeOut == "" {
		t.Error("large viewport View() returned empty")
	}
}

func TestViewRecentlyCompletedMode(t *testing.T) {
	m := viewTestModel(viewTestTasks(), testViews())

	// Enter recently-completed mode directly to avoid filterBar cmd issues.
	m.mode = modeRecentlyCompleted
	m.filterBar, _ = m.filterBar.Update(FilterActivateMsg{
		Mode:         filterQuery,
		InitialValue: "completed and @completed >= today-7d",
		Placeholder:  "type to filter...",
	})
	m.rebuildSections()
	m.nav.ClampCursor(m.displaySections())
	output := stripped(m)

	if !strings.Contains(output, "Recently Completed") {
		t.Errorf("recently-completed View() missing 'Recently Completed' header; got:\n%s", output)
	}
}

func TestViewErrorDisplayed(t *testing.T) {
	m := viewTestModel(viewTestTasks(), testViews())
	m.err = errors.New("something broke")
	output := stripped(m)

	if !strings.Contains(output, "Error:") {
		t.Error("View() with error should display 'Error:' prefix")
	}
}
