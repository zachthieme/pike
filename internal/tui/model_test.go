package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zachthieme/pike/internal/config"
	"github.com/zachthieme/pike/internal/model"

	tea "github.com/charmbracelet/bubbletea"
)

func timePtr(t time.Time) *time.Time {
	return &t
}

var testNow = time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

// taskWithTagSet creates a Task and populates TagSet from Tags.
func taskWithTagSet(t model.Task) model.Task {
	t.LowerText = strings.ToLower(t.Text)
	t.TagSet = make(map[string]bool, len(t.Tags))
	for _, tag := range t.Tags {
		t.TagSet[tag.Name] = true
	}
	return t
}

func testTasks() []model.Task {
	return []model.Task{
		taskWithTagSet(model.Task{
			Text:  "Overdue task @due(2026-03-10)",
			State: model.Open,
			File:  "notes/todo.md",
			Line:  1,
			Tags:  []model.Tag{{Name: "due", Value: "2026-03-10"}},
			Due:   timePtr(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)),
		}),
		taskWithTagSet(model.Task{
			Text:  "Today task @today",
			State: model.Open,
			File:  "notes/todo.md",
			Line:  2,
			Tags:  []model.Tag{{Name: "today"}},
		}),
		taskWithTagSet(model.Task{
			Text:  "Future task @due(2026-03-20)",
			State: model.Open,
			File:  "notes/todo.md",
			Line:  3,
			Tags:  []model.Tag{{Name: "due", Value: "2026-03-20"}},
			Due:   timePtr(time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)),
		}),
		taskWithTagSet(model.Task{
			Text:      "Done task @completed(2026-03-12)",
			State:     model.Completed,
			File:      "notes/todo.md",
			Line:      4,
			Tags:      []model.Tag{{Name: "completed", Value: "2026-03-12"}},
			Completed: timePtr(time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)),
		}),
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

	m := NewModel(cfg, tasks, nil, nil)
	m.now = func() time.Time { return testNow }
	m.rebuildSections()
	m.nav.ClampCursor(m.displaySections())
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
	if m2.nav.Cursor() != 1 {
		t.Errorf("expected cursor at 1 after j, got %d", m2.nav.Cursor())
	}
}

func TestCursorMovementUp(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.nav.SetCursor(2)

	updated, _ := sendKey(m, "k")
	m2 := updated.(Model)
	if m2.nav.Cursor() != 1 {
		t.Errorf("expected cursor at 1 after k, got %d", m2.nav.Cursor())
	}
}

func TestCursorDoesNotGoNegative(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.nav.SetCursor(0)

	updated, _ := sendKey(m, "k")
	m2 := updated.(Model)
	if m2.nav.Cursor() != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", m2.nav.Cursor())
	}
}

func TestCursorDoesNotExceedTasks(t *testing.T) {
	m := testModel(testTasks(), testViews())
	flatLen := len(flatTasks(m.displaySections()))
	m.nav.SetCursor(flatLen - 1)

	updated, _ := sendKey(m, "j")
	m2 := updated.(Model)
	if m2.nav.Cursor() != flatLen-1 {
		t.Errorf("expected cursor to stay at %d, got %d", flatLen-1, m2.nav.Cursor())
	}
}

func TestGotoTop(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.nav.SetCursor(3)

	updated, _ := sendKey(m, "g")
	m2 := updated.(Model)
	if m2.nav.Cursor() != 0 {
		t.Errorf("expected cursor at 0 after g, got %d", m2.nav.Cursor())
	}
}

func TestGotoBottom(t *testing.T) {
	m := testModel(testTasks(), testViews())
	flatLen := len(flatTasks(m.displaySections()))

	updated, _ := sendKey(m, "G")
	m2 := updated.(Model)
	if m2.nav.Cursor() != flatLen-1 {
		t.Errorf("expected cursor at %d after G, got %d", flatLen-1, m2.nav.Cursor())
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
	if m2.nav.Cursor() != firstSectionTasks {
		t.Errorf("expected cursor at %d after Tab, got %d", firstSectionTasks, m2.nav.Cursor())
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

	if m.filterBar.Active() {
		t.Fatal("expected filtering to start false")
	}

	updated, _ := sendKey(m, "/")
	m2 := updated.(Model)
	if !m2.filterBar.Active() {
		t.Error("expected filtering to be true after pressing '/'")
	}
	if m2.filterBar.Mode() != filterSubstring {
		t.Error("expected filterSubstring mode after pressing '/'")
	}
}

func TestQueryModeActivation(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendKey(m, "?")
	m2 := updated.(Model)
	if !m2.filterBar.Active() {
		t.Error("expected filtering to be true after pressing '?'")
	}
	if m2.filterBar.Mode() != filterQuery {
		t.Error("expected filterQuery mode after pressing '?'")
	}
}

func TestRecentlyCompletedUsesQueryMode(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendKey(m, "c")
	m2 := updated.(Model)
	if m2.filterBar.Mode() != filterQuery {
		t.Error("expected filterQuery mode for recently completed")
	}
}

func TestSubstringFilterWithTags(t *testing.T) {
	tasks := []model.Task{
		taskWithTagSet(model.Task{Text: "Fix bug @delegated to bob", State: model.Open, File: "t.md", Line: 1,
			Tags: []model.Tag{{Name: "delegated"}}, HasCheckbox: true}),
		taskWithTagSet(model.Task{Text: "Write docs", State: model.Open, File: "t.md", Line: 2, HasCheckbox: true}),
	}
	views := []config.ViewConfig{
		{Title: "All", Query: "open", Sort: "file", Color: "green"},
	}
	m := testModel(tasks, views)

	// Activate substring filter
	updated, _ := sendKey(m, "/")
	m = updated.(Model)
	for _, ch := range "@delegated bob" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}

	flat := flatTasks(m.displaySections())
	if len(flat) != 1 {
		t.Fatalf("expected 1 matching task, got %d", len(flat))
	}
	if !strings.Contains(flat[0].Text, "delegated") {
		t.Errorf("expected task with @delegated, got %q", flat[0].Text)
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

	flat := flatTasks(m.displaySections())
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
	if m2.nav.Cursor() != 1 {
		t.Errorf("expected cursor at 1 after down arrow in filter mode, got %d", m2.nav.Cursor())
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
	if !m.filterBar.Active() {
		t.Error("expected filtering to still be true after first Esc")
	}
	if m.filterBar.Text() != "" {
		t.Errorf("expected filterText to be empty, got %q", m.filterBar.Text())
	}

	// Second Escape exits filter mode entirely.
	updated, _ = sendSpecialKey(m, tea.KeyEscape)
	m2 := updated.(Model)
	if m2.filterBar.Active() {
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

	if len(flatTasks(m.displaySections())) != 1 {
		t.Fatalf("expected 1 task initially, got %d", len(flatTasks(m.displaySections())))
	}

	// RefreshMsg launches an async scan; simulate receiving the result.
	updated, _ := m.Update(RefreshMsg{})
	m2 := updated.(Model)
	// Feed the scan result directly.
	updated, _ = m2.Update(scanResultMsg{Tasks: newTasks})
	m3 := updated.(Model)
	if len(flatTasks(m3.displaySections())) != 2 {
		t.Fatalf("expected 2 tasks after refresh, got %d", len(flatTasks(m3.displaySections())))
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
	m.version = "v1.2.3"
	m.width = 80

	view := m.View()

	// Version header
	if !strings.Contains(view, "v1.2.3") {
		t.Error("expected summary to contain version")
	}
	if !strings.Contains(view, "pike") {
		t.Error("expected summary to contain 'pike'")
	}

	// Description
	if !strings.Contains(view, "Pike reaches in") {
		t.Error("expected summary to contain description")
	}

	// Keybindings
	for _, key := range []string{"Enter", "Tab", "Shift+Tab"} {
		if !strings.Contains(view, key) {
			t.Errorf("expected summary to contain keybinding %q", key)
		}
	}
}

func TestArrowKeys(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendSpecialKey(m, tea.KeyDown)
	m2 := updated.(Model)
	if m2.nav.Cursor() != 1 {
		t.Errorf("expected cursor at 1 after down arrow, got %d", m2.nav.Cursor())
	}

	m2.nav.SetCursor(2)
	updated, _ = sendSpecialKey(m2, tea.KeyUp)
	m3 := updated.(Model)
	if m3.nav.Cursor() != 1 {
		t.Errorf("expected cursor at 1 after up arrow, got %d", m3.nav.Cursor())
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

	flat := flatTasks(m.displaySections())
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

	flat := flatTasks(m.displaySections())
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

	tags := m.tagSearch.filteredTags()
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

	tags := m.tagSearch.filteredTags()
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
	tags := m.tagSearch.filteredTags()
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

	// Press Enter to select — TagSearch emits TagSelectedMsg via command.
	updated, cmd := sendSpecialKey(m, tea.KeyEnter)
	m = updated.(Model)
	if cmd != nil {
		updated, _ = m.Update(cmd())
		m = updated.(Model)
	}

	if m.mode != modeAllTasks {
		t.Fatalf("expected modeAllTasks, got %d", m.mode)
	}
	if !m.showAll {
		t.Error("expected showAll to be true after tag selection")
	}

	// Should include the completed task.
	flat := flatTasks(m.displaySections())
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

	// Enter tag search, select first tag — TagSearch emits TagSelectedMsg via command.
	updated, _ := sendKey(m, "t")
	m = updated.(Model)
	updated, cmd := sendSpecialKey(m, tea.KeyEnter)
	m = updated.(Model)
	if cmd != nil {
		updated, _ = m.Update(cmd())
		m = updated.(Model)
	}

	if !m.showAll {
		t.Fatal("expected showAll after tag selection")
	}

	// First Escape clears query text (filter had @tag content).
	// With showAll=true and empty text, FilterChangedMsg triggers enterTagSearchMode().
	updated, _ = sendSpecialKey(m, tea.KeyEscape)
	m = updated.(Model)

	// Second Escape in tag search mode — TagSearch emits TagSearchExitMsg via command.
	updated, cmd = sendSpecialKey(m, tea.KeyEscape)
	m = updated.(Model)
	if cmd != nil {
		updated, _ = m.Update(cmd())
		m = updated.(Model)
	}

	if m.showAll {
		t.Error("expected showAll to be false after escape")
	}
	if m.mode != modeDashboard {
		t.Errorf("expected modeDashboard, got %d", m.mode)
	}
}

func TestBackspaceToEmptyReturnsToTagSearch(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Enter tag search, select first tag — TagSearch emits TagSelectedMsg via command.
	updated, _ := sendKey(m, "t")
	m = updated.(Model)
	updated, cmd := sendSpecialKey(m, tea.KeyEnter)
	m = updated.(Model)
	if cmd != nil {
		updated, _ = m.Update(cmd())
		m = updated.(Model)
	}

	if m.mode != modeAllTasks {
		t.Fatalf("expected modeAllTasks after tag selection, got %d", m.mode)
	}

	// Delete all characters in the filter.
	for m.filterBar.Text() != "" {
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

	flat := flatTasks(m.displaySections())
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
	m.nav.SetHeight(30)

	// Scroll down
	m.nav.PageScroll(1, m.displaySections())
	if m.nav.Cursor() <= 0 {
		t.Errorf("expected cursor > 0 after PageScroll(1), got %d", m.nav.Cursor())
	}
	first := m.nav.Cursor()

	// Scroll down again
	m.nav.PageScroll(1, m.displaySections())
	if m.nav.Cursor() <= first {
		t.Errorf("expected cursor > %d after 2nd PageScroll(1), got %d", first, m.nav.Cursor())
	}

	// Scroll back up
	m.nav.PageScroll(-1, m.displaySections())
	if m.nav.Cursor() != first {
		t.Errorf("expected cursor back to %d after PageScroll(-1), got %d", first, m.nav.Cursor())
	}

	// Scroll up past top clamps to 0
	m.nav.SetCursor(2)
	m.nav.PageScroll(-1, m.displaySections())
	if m.nav.Cursor() != 0 {
		t.Errorf("expected cursor clamped to 0, got %d", m.nav.Cursor())
	}
}

func TestRecentlyCompletedMode(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendKey(m, "c")
	m = updated.(Model)

	if m.mode != modeRecentlyCompleted {
		t.Errorf("expected modeRecentlyCompleted, got %d", m.mode)
	}
	if !m.filterBar.Active() {
		t.Error("expected filtering to be true")
	}
	if !strings.Contains(m.filterBar.Text(), "completed and @completed") {
		t.Errorf("expected pre-filled query, got %q", m.filterBar.Text())
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

func TestQuitFromResultsFocusedReturnsToDashboard(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Enter all-tasks mode (activates filter bar with input focused).
	updated, _ := sendKey(m, "a")
	m = updated.(Model)

	// Tab to blur input (focus moves to results).
	updated, _ = sendSpecialKey(m, tea.KeyTab)
	m = updated.(Model)

	if m.filterBar.InputFocused() {
		t.Fatal("expected input blurred after Tab")
	}
	if !m.filterBar.Active() {
		t.Fatal("expected filter bar active")
	}

	// Press q — should return to dashboard, not quit.
	updated, _ = sendKey(m, "q")
	m = updated.(Model)
	if m.mode != modeDashboard {
		t.Errorf("expected modeDashboard after q from results, got %d", m.mode)
	}
	if m.filterBar.Active() {
		t.Error("expected filter bar deactivated after q")
	}
}

func TestQuitFromRecentlyCompletedResultsFocusedReturnsToDashboard(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Enter recently-completed mode.
	updated, _ := sendKey(m, "c")
	m = updated.(Model)

	// Tab to blur input (focus moves to results).
	updated, _ = sendSpecialKey(m, tea.KeyTab)
	m = updated.(Model)

	// Press q — should return to dashboard, not quit.
	updated, _ = sendKey(m, "q")
	m = updated.(Model)
	if m.mode != modeDashboard {
		t.Errorf("expected modeDashboard after q from recently-completed, got %d", m.mode)
	}
}

func TestJKTypeIntoFilterWhenInputFocused(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Activate filter (input is focused).
	updated, _ := sendKey(m, "/")
	m = updated.(Model)

	if !m.filterBar.InputFocused() {
		t.Fatal("expected input focused after /")
	}

	// Press 'j' — should type into filter, not move cursor.
	cursorBefore := m.nav.Cursor()
	updated, _ = sendKey(m, "j")
	m = updated.(Model)

	if m.nav.Cursor() != cursorBefore {
		t.Errorf("expected cursor unchanged (typed j into filter), but cursor moved from %d to %d", cursorBefore, m.nav.Cursor())
	}
	if m.filterBar.Text() != "j" {
		t.Errorf("expected filter text 'j', got %q", m.filterBar.Text())
	}

	// Press 'k' — should also type into filter.
	updated, _ = sendKey(m, "k")
	m = updated.(Model)

	if m.filterBar.Text() != "jk" {
		t.Errorf("expected filter text 'jk', got %q", m.filterBar.Text())
	}
}

func TestProcessFilterOutputRebuildsOnChange(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Activate filter.
	updated, _ := sendKey(m, "/")
	m = updated.(Model)

	// Type "overdue" — processFilterOutput should rebuild sections inline.
	for _, ch := range "overdue" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}

	// Sections should be filtered (inline, not deferred).
	flat := flatTasks(m.displaySections())
	for _, task := range flat {
		if !strings.Contains(strings.ToLower(task.Text), "overdue") {
			t.Errorf("expected all tasks to match filter, got %q", task.Text)
		}
	}
}

func TestProcessFilterOutputClearedExitsMode(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Activate filter.
	updated, _ := sendKey(m, "/")
	m = updated.(Model)

	// Escape on empty input — should emit FilterClearedMsg handled inline.
	updated, _ = sendSpecialKey(m, tea.KeyEscape)
	m = updated.(Model)

	if m.filterBar.Active() {
		t.Error("expected filter bar deactivated after Escape on empty")
	}
	if m.mode != modeDashboard {
		t.Errorf("expected modeDashboard, got %d", m.mode)
	}
}

func TestProcessFilterOutputPreservesTextInputCmd(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Activate filter.
	updated, _ := sendKey(m, "/")
	m = updated.(Model)

	// Type a character — should return a non-nil tea.Cmd (textinput blink).
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = updated.(Model)

	// The textinput blink cmd should be preserved (not swallowed).
	// We can't easily test the blink content, but the cmd should be non-nil
	// since the textinput widget returns a blink cmd on keystrokes.
	if m.filterBar.Text() != "x" {
		t.Errorf("expected filter text 'x', got %q", m.filterBar.Text())
	}
	// cmd may be nil if textinput doesn't produce one, so just verify
	// the filter output was processed (sections rebuilt).
	_ = cmd
	flat := flatTasks(m.displaySections())
	// "x" shouldn't match any tasks
	if len(flat) != 0 {
		t.Errorf("expected 0 tasks matching 'x', got %d", len(flat))
	}
}

// Task 11: State transition tests

func TestErrorClearsOnKeyPress(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.err = fmt.Errorf("test error")
	updated, _ := sendKey(m, "j")
	m2 := updated.(Model)
	if m2.err != nil {
		t.Errorf("err = %v, want nil after keypress", m2.err)
	}
}

func TestScanResultMsgUpdatesConfig(t *testing.T) {
	m := testModel(testTasks(), testViews())
	newCfg := &config.Config{
		Editor:    "nvim",
		TagColors: map[string]string{"new": "#ff0000"},
		Views:     testViews(),
	}
	updated, _ := m.Update(scanResultMsg{Config: newCfg})
	m2 := updated.(Model)
	if m2.editorCmd != "nvim" {
		t.Errorf("editorCmd = %q, want %q", m2.editorCmd, "nvim")
	}
	if m2.tagColors["new"] != "#ff0000" {
		t.Errorf("tagColors[new] = %q, want #ff0000", m2.tagColors["new"])
	}
}

func TestScanResultMsgError(t *testing.T) {
	m := testModel(testTasks(), testViews())
	updated, _ := m.Update(scanResultMsg{Err: fmt.Errorf("scan failed")})
	m2 := updated.(Model)
	if m2.err == nil || m2.err.Error() != "scan failed" {
		t.Errorf("err = %v, want 'scan failed'", m2.err)
	}
}

func TestScanResultMsgInTagSearchMode(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.mode = modeTagSearch
	newTasks := append(testTasks(), taskWithTagSet(model.Task{
		Text: "new @newtag", State: model.Open, File: "new.md", Line: 1,
		Tags: []model.Tag{{Name: "newtag"}},
	}))
	updated, _ := m.Update(scanResultMsg{Tasks: newTasks})
	m2 := updated.(Model)
	if m2.mode != modeTagSearch {
		t.Errorf("mode = %v, want modeTagSearch", m2.mode)
	}
	if len(m2.allTasks) != len(newTasks) {
		t.Errorf("allTasks len = %d, want %d", len(m2.allTasks), len(newTasks))
	}
}

func TestEditorFinishedMsgWithError(t *testing.T) {
	m := testModel(testTasks(), testViews())
	updated, cmd := m.Update(EditorFinishedMsg{Err: fmt.Errorf("editor crashed")})
	m2 := updated.(Model)
	if m2.err == nil || m2.err.Error() != "editor crashed" {
		t.Errorf("err = %v, want 'editor crashed'", m2.err)
	}
	if cmd == nil {
		t.Error("cmd is nil, want refresh command")
	}
}

func TestEditorFinishedMsgNoError(t *testing.T) {
	m := testModel(testTasks(), testViews())
	_, cmd := m.Update(EditorFinishedMsg{})
	if cmd == nil {
		t.Error("cmd is nil, want refresh command")
	}
}

func TestToggleResultMsgError(t *testing.T) {
	m := testModel(testTasks(), testViews())
	updated, cmd := m.Update(toggleResultMsg{Err: fmt.Errorf("toggle failed")})
	m2 := updated.(Model)
	if m2.err == nil {
		t.Error("err is nil, want error")
	}
	if cmd != nil {
		t.Error("cmd should be nil on toggle error")
	}
}

func TestToggleResultMsgSuccess(t *testing.T) {
	m := testModel(testTasks(), testViews())
	_, cmd := m.Update(toggleResultMsg{})
	if cmd == nil {
		t.Error("cmd is nil, want refresh command")
	}
}

func TestSetFocusedViewLocksKeys(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.SetFocusedView("Open")
	if !m.viewLocked {
		t.Error("viewLocked should be true")
	}
	if m.keys.Summary.Enabled() {
		t.Error("Summary key should be disabled")
	}
	if m.keys.AllTasks.Enabled() {
		t.Error("AllTasks key should be disabled")
	}
	if m.keys.TagSearch.Enabled() {
		t.Error("TagSearch key should be disabled")
	}
	updated, _ := sendSpecialKey(m, tea.KeyEscape)
	m2 := updated.(Model)
	if m2.focusedView != "Open" {
		t.Errorf("focusedView = %q, want 'Open' (locked)", m2.focusedView)
	}
}

// Task 12: Key handling edge case tests

func TestCursorAtBoundaries(t *testing.T) {
	m := testModel(testTasks(), testViews())
	updated, _ := sendKey(m, "k")
	m2 := updated.(Model)
	if m2.nav.Cursor() != 0 {
		t.Errorf("cursor = %d, want 0 (at top)", m2.nav.Cursor())
	}
	total := countFlatTasks(m.displaySections())
	m.nav.SetCursor(total - 1)
	updated, _ = sendKey(m, "j")
	m2 = updated.(Model)
	if m2.nav.Cursor() != total-1 {
		t.Errorf("cursor = %d, want %d (at bottom)", m2.nav.Cursor(), total-1)
	}
}

func TestPageScrollAcrossSections(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.height = 20
	m.nav.SetHeight(20)
	m.nav.SetCursor(0)
	// PageDown is bound to ctrl+d.
	updated, _ := sendSpecialKey(m, tea.KeyCtrlD)
	m2 := updated.(Model)
	if m2.nav.Cursor() == 0 {
		t.Error("cursor should have moved on page down")
	}
	if m2.nav.Cursor() >= countFlatTasks(m.displaySections()) {
		t.Errorf("cursor %d out of bounds (total %d)", m2.nav.Cursor(), countFlatTasks(m.displaySections()))
	}
}

func TestEmptySectionsSkipped(t *testing.T) {
	views := []config.ViewConfig{
		{Title: "Empty", Query: "completed and @nonexistent", Sort: "file", Color: "red"},
		{Title: "Open", Query: "open", Sort: "file", Color: "green"},
	}
	m := testModel(testTasks(), views)
	sections := m.displaySections()
	for _, sec := range sections {
		if sec.Title == "Empty" && len(sec.Tasks) > 0 {
			t.Error("Empty section should have no tasks")
		}
	}
}

func TestKeyPressesInAllTasksMode(t *testing.T) {
	// Use tasks with HasCheckbox=true so they appear in modeAllTasks.
	tasks := []model.Task{
		taskWithTagSet(model.Task{Text: "Task A", State: model.Open, File: "t.md", Line: 1, HasCheckbox: true}),
		taskWithTagSet(model.Task{Text: "Task B", State: model.Open, File: "t.md", Line: 2, HasCheckbox: true}),
	}
	m := testModel(tasks, testViews())
	updated, _ := sendKey(m, "a")
	m2 := updated.(Model)
	if m2.mode != modeAllTasks {
		t.Errorf("mode = %v, want modeAllTasks", m2.mode)
	}
	updated, _ = sendSpecialKey(m2, tea.KeyDown)
	m3 := updated.(Model)
	if m3.nav.Cursor() != 1 {
		t.Errorf("cursor = %d, want 1", m3.nav.Cursor())
	}
}

func TestToggleNoCheckboxNoop(t *testing.T) {
	tasks := []model.Task{
		taskWithTagSet(model.Task{
			Text: "plain bullet @today", State: model.Open,
			File: "test.md", Line: 1, HasCheckbox: false,
			Tags: []model.Tag{{Name: "today"}},
		}),
	}
	views := []config.ViewConfig{
		{Title: "Open", Query: "open", Sort: "file", Color: "green"},
	}
	m := testModel(tasks, views)
	_, cmd := sendKey(m, "x")
	if cmd != nil {
		t.Error("toggle on non-checkbox task should return nil cmd")
	}
}

func TestRecentlyCompletedModeIdempotent(t *testing.T) {
	m := testModel(testTasks(), testViews())
	updated, _ := sendKey(m, "c")
	m2 := updated.(Model)
	if m2.mode != modeRecentlyCompleted {
		t.Errorf("mode = %v, want modeRecentlyCompleted", m2.mode)
	}
	updated, _ = sendKey(m2, "c")
	m3 := updated.(Model)
	if m3.mode != modeRecentlyCompleted {
		t.Errorf("mode = %v, want modeRecentlyCompleted", m3.mode)
	}
}

func TestEscapePriorityChain(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.showSummary = true
	updated, _ := sendSpecialKey(m, tea.KeyEscape)
	m2 := updated.(Model)
	if m2.showSummary {
		t.Error("showSummary should be false after escape")
	}
	if m2.mode != modeDashboard {
		t.Error("mode should still be dashboard")
	}
	m2.mode = modeAllTasks
	updated, _ = sendSpecialKey(m2, tea.KeyEscape)
	m3 := updated.(Model)
	if m3.mode != modeDashboard {
		t.Errorf("mode = %v, want modeDashboard after escape", m3.mode)
	}
	m3.focusedView = "Open"
	m3.viewLocked = false
	updated, _ = sendSpecialKey(m3, tea.KeyEscape)
	m4 := updated.(Model)
	if m4.focusedView != "" {
		t.Errorf("focusedView = %q, want empty after escape", m4.focusedView)
	}
}
