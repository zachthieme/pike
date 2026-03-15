package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"pike/internal/config"
	"pike/internal/model"

	tea "github.com/charmbracelet/bubbletea"
)

func timePtr(t time.Time) *time.Time {
	return &t
}

var testNow = time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

func testTasks() []model.Task {
	return []model.Task{
		{
			Text:  "Overdue task @due(2026-03-10)",
			State: model.Open,
			File:  "notes/todo.md",
			Line:  1,
			Tags:  []model.Tag{{Name: "due", Value: "2026-03-10"}},
			Due:   timePtr(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)),
		},
		{
			Text:  "Today task @today",
			State: model.Open,
			File:  "notes/todo.md",
			Line:  2,
			Tags:  []model.Tag{{Name: "today"}},
		},
		{
			Text:  "Future task @due(2026-03-20)",
			State: model.Open,
			File:  "notes/todo.md",
			Line:  3,
			Tags:  []model.Tag{{Name: "due", Value: "2026-03-20"}},
			Due:   timePtr(time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)),
		},
		{
			Text:      "Done task @completed(2026-03-12)",
			State:     model.Completed,
			File:      "notes/todo.md",
			Line:      4,
			Tags:      []model.Tag{{Name: "completed", Value: "2026-03-12"}},
			Completed: timePtr(time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)),
		},
	}
}

func testViews() []config.ViewConfig {
	return []config.ViewConfig{
		{Title: "Overdue", Query: "open and @due < today", Sort: "due_asc", Color: "red", Order: 1},
		{Title: "Open", Query: "open", Sort: "file", Color: "green", Order: 2},
		{Title: "Completed", Query: "completed", Sort: "file", Color: "blue", Order: 3},
	}
}

func testModel(tasks []model.Task, views []config.ViewConfig) Model {
	cfg := &config.Config{
		Editor:    "vi",
		TagColors: map[string]string{"due": "red", "today": "green", "_default": "cyan"},
		Views:     views,
	}

	m := NewModel(cfg, tasks, nil)
	m.now = func() time.Time { return testNow }
	m.rebuildSections()
	m.clampCursor()
	return m
}

func sendKey(m tea.Model, keyStr string) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)})
}

func sendSpecialKey(m tea.Model, keyType tea.KeyType) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: keyType})
}

func TestCursorMovementDown(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendKey(m, "j")
	m2 := updated.(Model)
	if m2.cursor != 1 {
		t.Errorf("expected cursor at 1 after j, got %d", m2.cursor)
	}
}

func TestCursorMovementUp(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.cursor = 2

	updated, _ := sendKey(m, "k")
	m2 := updated.(Model)
	if m2.cursor != 1 {
		t.Errorf("expected cursor at 1 after k, got %d", m2.cursor)
	}
}

func TestCursorDoesNotGoNegative(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.cursor = 0

	updated, _ := sendKey(m, "k")
	m2 := updated.(Model)
	if m2.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", m2.cursor)
	}
}

func TestCursorDoesNotExceedTasks(t *testing.T) {
	m := testModel(testTasks(), testViews())
	flatLen := len(m.flatTasks())
	m.cursor = flatLen - 1

	updated, _ := sendKey(m, "j")
	m2 := updated.(Model)
	if m2.cursor != flatLen-1 {
		t.Errorf("expected cursor to stay at %d, got %d", flatLen-1, m2.cursor)
	}
}

func TestGotoTop(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.cursor = 3

	updated, _ := sendKey(m, "g")
	m2 := updated.(Model)
	if m2.cursor != 0 {
		t.Errorf("expected cursor at 0 after g, got %d", m2.cursor)
	}
}

func TestGotoBottom(t *testing.T) {
	m := testModel(testTasks(), testViews())
	flatLen := len(m.flatTasks())

	updated, _ := sendKey(m, "G")
	m2 := updated.(Model)
	if m2.cursor != flatLen-1 {
		t.Errorf("expected cursor at %d after G, got %d", flatLen-1, m2.cursor)
	}
}

func TestTabNextSection(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendSpecialKey(m, tea.KeyTab)
	m2 := updated.(Model)

	sections := m2.displaySections()
	firstSectionTasks := 0
	for _, sec := range sections {
		if len(sec.Tasks) > 0 {
			firstSectionTasks = len(sec.Tasks)
			break
		}
	}
	if m2.cursor != firstSectionTasks {
		t.Errorf("expected cursor at %d after Tab, got %d", firstSectionTasks, m2.cursor)
	}
}

func TestFocusSectionByNumber(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendKey(m, "1")
	m2 := updated.(Model)
	// The first visible section title is "Overdue".
	if m2.focusedView != "Overdue" {
		t.Errorf("expected focusedView %q after pressing '1', got %q", "Overdue", m2.focusedView)
	}
}

func TestSummaryToggle(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendKey(m, "s")
	m2 := updated.(Model)
	if !m2.showSummary {
		t.Error("expected showSummary to be true after pressing 's'")
	}

	updated, _ = sendKey(m2, "s")
	m3 := updated.(Model)
	if m3.showSummary {
		t.Error("expected showSummary to be false after pressing 's' again")
	}
}

func TestFilterActivation(t *testing.T) {
	m := testModel(testTasks(), testViews())

	if m.filtering {
		t.Fatal("expected filtering to start false")
	}

	updated, _ := sendKey(m, "/")
	m2 := updated.(Model)
	if !m2.filtering {
		t.Error("expected filtering to be true after pressing '/'")
	}
}

func TestFilterAtTopWhenActive(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.width = 80

	// Activate filter
	updated, _ := sendKey(m, "/")
	m2 := updated.(Model)

	view := m2.View()
	lines := strings.Split(view, "\n")
	if len(lines) == 0 || !strings.Contains(lines[0], "/") {
		t.Errorf("expected filter input at top of view, got first line: %q", lines[0])
	}
}

func TestFilterNotShownWhenInactive(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.width = 80

	view := m.View()
	lines := strings.Split(view, "\n")
	// First line should not be the filter prompt
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "/ ") {
		t.Error("expected filter input to not be shown when inactive")
	}
}

func TestFilterNarrowsResults(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Activate filter
	updated, _ := sendKey(m, "/")
	m = updated.(Model)

	// Type "overdue"
	for _, ch := range "overdue" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}

	flat := m.flatTasks()
	for _, task := range flat {
		if !strings.Contains(strings.ToLower(task.Text), "overdue") {
			t.Errorf("expected all tasks to match filter, got %q", task.Text)
		}
	}
}

func TestFilterNavigationWithArrows(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Activate filter
	updated, _ := sendKey(m, "/")
	m = updated.(Model)

	// Arrow down should move cursor, not type into filter
	updated, _ = sendSpecialKey(m, tea.KeyDown)
	m2 := updated.(Model)
	if m2.cursor != 1 {
		t.Errorf("expected cursor at 1 after down arrow in filter mode, got %d", m2.cursor)
	}
}

func TestEscapeDismissesFilter(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Activate filter and type something
	updated, _ := sendKey(m, "/")
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("test")})
	m = updated.(Model)

	// First Escape clears query text but stays in filter mode.
	updated, _ = sendSpecialKey(m, tea.KeyEscape)
	m = updated.(Model)
	if !m.filtering {
		t.Error("expected filtering to still be true after first Esc")
	}
	if m.filterText != "" {
		t.Errorf("expected filterText to be empty, got %q", m.filterText)
	}

	// Second Escape exits filter mode entirely.
	updated, _ = sendSpecialKey(m, tea.KeyEscape)
	m2 := updated.(Model)
	if m2.filtering {
		t.Error("expected filtering to be false after second Esc")
	}
}

func TestEscapeDismissesSummary(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.showSummary = true

	updated, _ := sendSpecialKey(m, tea.KeyEscape)
	m2 := updated.(Model)
	if m2.showSummary {
		t.Error("expected showSummary to be false after Esc")
	}
}

func TestEscapeExitsFocus(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.focusedView = "Overdue"

	updated, _ := sendSpecialKey(m, tea.KeyEscape)
	m2 := updated.(Model)
	if m2.focusedView != "" {
		t.Errorf("expected focusedView %q after Esc, got %q", "", m2.focusedView)
	}
}

func TestQuit(t *testing.T) {
	m := testModel(testTasks(), testViews())

	_, cmd := sendKey(m, "q")
	if cmd == nil {
		t.Fatal("expected a non-nil cmd for quit")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestEmptySectionsHiddenInView(t *testing.T) {
	tasks := []model.Task{
		{Text: "Just a task", State: model.Open, File: "test.md", Line: 1},
	}
	views := []config.ViewConfig{
		{Title: "Overdue", Query: "open and @due < today", Sort: "file", Color: "red", Order: 1},
		{Title: "Open", Query: "open", Sort: "file", Color: "green", Order: 2},
	}

	m := testModel(tasks, views)
	m.width = 80
	view := m.View()

	if strings.Contains(view, "Overdue") {
		t.Error("expected empty 'Overdue' section to be hidden in view")
	}
	if !strings.Contains(view, "Open") {
		t.Error("expected 'Open' section to appear in view")
	}
}

func TestRefreshMsg(t *testing.T) {
	initialTasks := []model.Task{
		{Text: "Original task", State: model.Open, File: "test.md", Line: 1},
	}
	views := []config.ViewConfig{
		{Title: "All", Query: "open", Sort: "file", Color: "green"},
	}
	newTasks := []model.Task{
		{Text: "Updated task 1", State: model.Open, File: "test.md", Line: 1},
		{Text: "Updated task 2", State: model.Open, File: "test.md", Line: 2},
	}

	m := testModel(initialTasks, views)
	m.scanFunc = func() ([]model.Task, error) { return newTasks, nil }

	if len(m.flatTasks()) != 1 {
		t.Fatalf("expected 1 task initially, got %d", len(m.flatTasks()))
	}

	updated, _ := m.Update(RefreshMsg{})
	m2 := updated.(Model)
	if len(m2.flatTasks()) != 2 {
		t.Fatalf("expected 2 tasks after refresh, got %d", len(m2.flatTasks()))
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	if m2.width != 120 || m2.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", m2.width, m2.height)
	}
}

func TestViewRendersWithSummary(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.showSummary = true
	m.width = 80

	view := m.View()
	if !strings.Contains(view, "Open tasks") {
		t.Error("expected summary to contain 'Open tasks'")
	}
}

func TestArrowKeys(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendSpecialKey(m, tea.KeyDown)
	m2 := updated.(Model)
	if m2.cursor != 1 {
		t.Errorf("expected cursor at 1 after down arrow, got %d", m2.cursor)
	}

	m2.cursor = 2
	updated, _ = sendSpecialKey(m2, tea.KeyUp)
	m3 := updated.(Model)
	if m3.cursor != 1 {
		t.Errorf("expected cursor at 1 after up arrow, got %d", m3.cursor)
	}
}

func TestFilterPartialTagMatch(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Activate filter and type partial tag "@du" (should match @due)
	updated, _ := sendKey(m, "/")
	m = updated.(Model)
	for _, ch := range "@du" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}

	flat := m.flatTasks()
	if len(flat) == 0 {
		t.Fatal("expected partial tag @du to match tasks with @due, got 0 results")
	}
	for _, task := range flat {
		if !task.HasTag("due") {
			t.Errorf("expected all filtered tasks to have @due tag, got %q", task.Text)
		}
	}
}

func TestFilterFullTagMatch(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Activate filter and type full tag "@today"
	updated, _ := sendKey(m, "/")
	m = updated.(Model)
	for _, ch := range "@today" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}

	flat := m.flatTasks()
	if len(flat) == 0 {
		t.Fatal("expected @today to match tasks, got 0 results")
	}
	for _, task := range flat {
		if !task.HasTag("today") {
			t.Errorf("expected all filtered tasks to have @today tag, got %q", task.Text)
		}
	}
}

func TestTagSearchWithAtPrefix(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Enter tag search mode
	updated, _ := sendKey(m, "t")
	m = updated.(Model)

	// Type "@du" — should match "due" tag even with @ prefix
	for _, ch := range "@du" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}

	tags := m.filteredTags()
	if len(tags) == 0 {
		t.Fatal("expected @du in tag search to match 'due', got 0 results")
	}
	found := false
	for _, tag := range tags {
		if tag == "due" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'due' in filtered tags, got %v", tags)
	}
}

func TestTagSearchWithoutAtPrefix(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Enter tag search mode
	updated, _ := sendKey(m, "t")
	m = updated.(Model)

	// Type "tod" — should match "today" tag
	for _, ch := range "tod" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}

	tags := m.filteredTags()
	found := false
	for _, tag := range tags {
		if tag == "today" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'today' in filtered tags, got %v", tags)
	}
}

func TestTagSelectShowsCompletedTasks(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Enter tag search, select a tag that has completed tasks (@completed).
	updated, _ := sendKey(m, "t")
	m = updated.(Model)

	// Find the "completed" tag and move cursor to it.
	tags := m.filteredTags()
	idx := -1
	for i, tag := range tags {
		if tag == "completed" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("expected 'completed' tag in tag list")
	}
	// Tab to the right tag.
	for i := 0; i < idx; i++ {
		updated, _ = sendSpecialKey(m, tea.KeyTab)
		m = updated.(Model)
	}

	// Press Enter to select.
	updated, _ = sendSpecialKey(m, tea.KeyEnter)
	m = updated.(Model)

	if m.mode != modeAllTasks {
		t.Fatalf("expected modeAllTasks, got %d", m.mode)
	}
	if !m.showAll {
		t.Error("expected showAll to be true after tag selection")
	}

	// Should include the completed task.
	flat := m.flatTasks()
	found := false
	for _, task := range flat {
		if task.State == model.Completed {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected completed tasks to appear when filtering by tag from tag search")
	}
}

func TestTagSelectEscapeClearsShowAll(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Enter tag search, select first tag.
	updated, _ := sendKey(m, "t")
	m = updated.(Model)
	updated, _ = sendSpecialKey(m, tea.KeyEnter)
	m = updated.(Model)

	if !m.showAll {
		t.Fatal("expected showAll after tag selection")
	}

	// First Escape clears query text (filter had @tag content).
	updated, _ = sendSpecialKey(m, tea.KeyEscape)
	m = updated.(Model)

	// Second Escape exits filter mode and returns to dashboard.
	updated, _ = sendSpecialKey(m, tea.KeyEscape)
	m = updated.(Model)

	if m.showAll {
		t.Error("expected showAll to be false after escape")
	}
	if m.mode != modeDashboard {
		t.Errorf("expected modeDashboard, got %d", m.mode)
	}
}

func TestBackspaceToEmptyReturnsToTagSearch(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Enter tag search, select first tag.
	updated, _ := sendKey(m, "t")
	m = updated.(Model)
	updated, _ = sendSpecialKey(m, tea.KeyEnter)
	m = updated.(Model)

	if m.mode != modeAllTasks {
		t.Fatalf("expected modeAllTasks after tag selection, got %d", m.mode)
	}

	// Delete all characters in the filter.
	for m.filterText != "" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = updated.(Model)
	}

	if m.mode != modeTagSearch {
		t.Errorf("expected modeTagSearch after clearing filter, got %d", m.mode)
	}
	if m.showAll {
		t.Error("expected showAll to be false after returning to tag search")
	}
}

func TestNegationWithDSL(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Activate filter and type "not @du" — DSL negation with partial tag.
	updated, _ := sendKey(m, "/")
	m = updated.(Model)
	for _, ch := range "not @du" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}

	flat := m.flatTasks()
	for _, task := range flat {
		if task.HasTag("due") {
			t.Errorf("expected 'not @du' to exclude tasks with @due, but found %q", task.Text)
		}
	}
}

func TestFlowWrap(t *testing.T) {
	// No ANSI codes — plain strings for predictable width measurement.
	tests := []struct {
		name     string
		parts    []string
		delim    string
		maxWidth int
		want     string
	}{
		{
			name:     "all fit on one line",
			parts:    []string{"today", "risk", "due"},
			delim:    " | ",
			maxWidth: 40,
			want:     "           today | risk | due           ",
		},
		{
			name:     "wraps to multiple lines",
			parts:    []string{"today", "risk", "due", "blocked"},
			delim:    " | ",
			maxWidth: 20,
			want:     " today | risk | due \n      blocked       ",
		},
		{
			name:     "single part",
			parts:    []string{"today"},
			delim:    " | ",
			maxWidth: 40,
			want:     "                 today                  ",
		},
		{
			name:     "empty parts",
			parts:    []string{},
			delim:    " | ",
			maxWidth: 40,
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flowWrap(tt.parts, tt.delim, tt.maxWidth)
			if got != tt.want {
				t.Errorf("flowWrap() =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestScrollWindow(t *testing.T) {
	tests := []struct {
		name       string
		cursor     int
		total      int
		maxVisible int
		wantStart  int
		wantEnd    int
	}{
		{"cursor at start", 0, 50, 20, 0, 20},
		{"cursor at end", 49, 50, 20, 30, 50},
		{"cursor in middle", 25, 50, 20, 15, 35},
		{"maxVisible > total", 5, 10, 20, 0, 10},
		{"maxVisible = 1", 5, 10, 1, 5, 6},
		{"total = 1", 0, 1, 20, 0, 1},
		{"cursor = total-1", 9, 10, 5, 5, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := scrollWindow(tt.cursor, tt.total, tt.maxVisible)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("scrollWindow(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.cursor, tt.total, tt.maxVisible, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestPageScroll(t *testing.T) {
	// Create enough tasks to scroll through.
	var tasks []model.Task
	for i := 0; i < 60; i++ {
		tasks = append(tasks, model.Task{
			Text:        fmt.Sprintf("Task %d", i+1),
			State:       model.Open,
			File:        "test.md",
			Line:        i + 1,
			HasCheckbox: true,
		})
	}
	views := []config.ViewConfig{
		{Title: "All", Query: "open", Sort: "file", Color: "green", Order: 1},
	}
	m := testModel(tasks, views)
	m.height = 30

	// Scroll down
	m.pageScroll(1)
	if m.cursor <= 0 {
		t.Errorf("expected cursor > 0 after pageScroll(1), got %d", m.cursor)
	}
	first := m.cursor

	// Scroll down again
	m.pageScroll(1)
	if m.cursor <= first {
		t.Errorf("expected cursor > %d after 2nd pageScroll(1), got %d", first, m.cursor)
	}

	// Scroll back up
	m.pageScroll(-1)
	if m.cursor != first {
		t.Errorf("expected cursor back to %d after pageScroll(-1), got %d", first, m.cursor)
	}

	// Scroll up past top clamps to 0
	m.cursor = 2
	m.pageScroll(-1)
	if m.cursor != 0 {
		t.Errorf("expected cursor clamped to 0, got %d", m.cursor)
	}
}

func TestRecentlyCompletedMode(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendKey(m, "c")
	m = updated.(Model)

	if m.mode != modeRecentlyCompleted {
		t.Errorf("expected modeRecentlyCompleted, got %d", m.mode)
	}
	if !m.filtering {
		t.Error("expected filtering to be true")
	}
	if !strings.Contains(m.filterText, "completed and @completed") {
		t.Errorf("expected pre-filled query, got %q", m.filterText)
	}
}

func TestRecentlyCompletedEscapeReturnsToDashboard(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendKey(m, "c")
	m = updated.(Model)

	// First Escape clears the pre-filled query.
	updated, _ = sendSpecialKey(m, tea.KeyEscape)
	m = updated.(Model)

	// Second Escape exits filter mode and returns to dashboard.
	updated, _ = sendSpecialKey(m, tea.KeyEscape)
	m = updated.(Model)

	if m.mode != modeDashboard {
		t.Errorf("expected modeDashboard after Esc, got %d", m.mode)
	}
}

func TestRecentlyCompletedNoOpWhenAlreadyActive(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendKey(m, "c")
	m = updated.(Model)

	// Press c again — should be no-op
	updated, _ = sendKey(m, "c")
	m2 := updated.(Model)
	if m2.mode != modeRecentlyCompleted {
		t.Errorf("expected to stay in modeRecentlyCompleted, got %d", m2.mode)
	}
}
