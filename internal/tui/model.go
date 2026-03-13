package tui

import (
	"fmt"
	"path/filepath"
	gosort "sort"
	"strings"
	"time"

	"pike/internal/config"
	"pike/internal/editor"
	"pike/internal/filter"
	"pike/internal/model"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RefreshMsg triggers a re-scan of task files.
type RefreshMsg struct{}

// EditorFinishedMsg is sent after the editor process exits.
type EditorFinishedMsg struct{ Err error }

// viewMode tracks the current display mode.
type viewMode int

const (
	modeDashboard viewMode = iota
	modeAllTasks
	modeTagSearch
)

// Model is the main Bubbletea model for the tasks TUI.
type Model struct {
	config      *config.Config
	allTasks    []model.Task
	sections     []filter.ViewResult
	hiddenCounts []int // per-section count of @hidden tasks that were removed
	cursor       int   // index into flat task list across all sections
	focusedView string // "" = dashboard, otherwise title of focused section
	showSummary bool
	filterInput textinput.Model
	filtering   bool
	filterText  string
	mode        viewMode
	tagList     []string // unique tags for tag search mode
	tagCursor   int      // cursor in tag list
	showHidden  bool     // whether to show @hidden tasks
	width       int
	height      int
	err         error
	scanFunc    func() ([]model.Task, error) // injected for refresh
	editorCmd   string
	tagColors   map[string]string
	keys        KeyMap
	now         func() time.Time // injectable for testing
}

// NewModel creates a new TUI model with the given configuration and initial tasks.
func NewModel(cfg *config.Config, tasks []model.Task, scanFunc func() ([]model.Task, error)) Model {
	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 256
	ti.Prompt = "/ "
	ti.PromptStyle = lipgloss.NewStyle().Bold(true)
	ti.PlaceholderStyle = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("7"))

	m := Model{
		config:      cfg,
		allTasks:    tasks,
		focusedView: "",
		filterInput: ti,
		scanFunc:    scanFunc,
		editorCmd:   cfg.Editor,
		tagColors:   cfg.TagColors,
		keys:        DefaultKeyMap(),
		now:         time.Now,
	}

	m.rebuildSections()
	m.clampCursor()

	return m
}

// SetFocusedView sets the focused view by section title and rebuilds sections.
func (m *Model) SetFocusedView(title string) {
	m.focusedView = title
	m.rebuildSections()
	m.clampCursor()
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.config != nil && m.config.RefreshInterval > 0 {
		return tea.Tick(m.config.RefreshInterval, func(time.Time) tea.Msg {
			return RefreshMsg{}
		})
	}
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case RefreshMsg:
		var nextTick tea.Cmd
		if m.config != nil && m.config.RefreshInterval > 0 {
			nextTick = tea.Tick(m.config.RefreshInterval, func(time.Time) tea.Msg {
				return RefreshMsg{}
			})
		}
		if m.scanFunc != nil {
			tasks, err := m.scanFunc()
			if err != nil {
				m.err = err
				return m, nextTick
			}
			m.allTasks = tasks
			if m.mode == modeTagSearch {
				m.buildTagList()
			}
			m.rebuildSections()
			m.clampCursor()
		}
		return m, nextTick

	case EditorFinishedMsg:
		if msg.Err != nil {
			m.err = msg.Err
		}
		return m, func() tea.Msg { return RefreshMsg{} }

	case tea.KeyMsg:
		m.err = nil // clear error on any key press
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Tag search mode: navigate/filter tag list.
	if m.mode == modeTagSearch {
		return m.handleTagSearchKey(msg)
	}

	// If filtering, handle text input first.
	if m.filtering {
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.clearFilter()
			if m.mode == modeAllTasks {
				m.mode = modeDashboard
			}
			m.rebuildSections()
			m.clampCursor()
			return m, nil
		case msg.Type == tea.KeyEnter:
			return m.openEditor()
		case msg.Type == tea.KeyDown || msg.Type == tea.KeyCtrlN:
			flatTasks := m.flatTasks()
			if len(flatTasks) > 0 && m.cursor < len(flatTasks)-1 {
				m.cursor++
			}
			return m, nil
		case msg.Type == tea.KeyUp || msg.Type == tea.KeyCtrlP:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case key.Matches(msg, m.keys.NextSection):
			m.jumpToNextSection()
			return m, nil
		case key.Matches(msg, m.keys.PrevSection):
			m.jumpToPrevSection()
			return m, nil
		default:
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.filterText = m.filterInput.Value()
			m.rebuildSections()
			m.clampCursor()
			return m, cmd
		}
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Escape):
		// Escape priority: dismiss summary -> exit mode -> exit focus -> do nothing
		if m.showSummary {
			m.showSummary = false
		} else if m.mode != modeDashboard {
			m.mode = modeDashboard
			m.rebuildSections()
			m.clampCursor()
		} else if m.focusedView != "" {
			m.focusedView = ""
			m.rebuildSections()
			m.clampCursor()
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		flatTasks := m.flatTasks()
		if len(flatTasks) > 0 && m.cursor < len(flatTasks)-1 {
			m.cursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Top):
		m.cursor = 0
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		flatTasks := m.flatTasks()
		if len(flatTasks) > 0 {
			m.cursor = len(flatTasks) - 1
		}
		return m, nil

	case key.Matches(msg, m.keys.NextSection):
		m.jumpToNextSection()
		return m, nil

	case key.Matches(msg, m.keys.PrevSection):
		m.jumpToPrevSection()
		return m, nil

	case key.Matches(msg, m.keys.Summary):
		m.showSummary = !m.showSummary
		return m, nil

	case key.Matches(msg, m.keys.Filter):
		m.filtering = true
		cmd := m.filterInput.Focus()
		return m, cmd

	case key.Matches(msg, m.keys.AllTasks):
		m.mode = modeAllTasks
		m.filtering = true
		m.filterInput.SetValue("")
		m.filterText = ""
		m.filterInput.Prompt = "> "
		m.filterInput.Placeholder = "search all open tasks..."
		focusCmd := m.filterInput.Focus()
		m.cursor = 0
		m.rebuildSections()
		m.clampCursor()
		return m, tea.Batch(focusCmd, func() tea.Msg { return tea.ClearScreen() })

	case key.Matches(msg, m.keys.TagSearch):
		m.mode = modeTagSearch
		m.buildTagList()
		m.filterInput.SetValue("")
		m.filterText = ""
		m.filterInput.Prompt = "> "
		m.filterInput.Placeholder = "search tags..."
		focusCmd := m.filterInput.Focus()
		m.filtering = true
		m.tagCursor = 0
		return m, tea.Batch(focusCmd, func() tea.Msg { return tea.ClearScreen() })

	case key.Matches(msg, m.keys.ToggleHidden):
		m.showHidden = !m.showHidden
		m.rebuildSections()
		m.clampCursor()
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		return m, func() tea.Msg { return RefreshMsg{} }

	case key.Matches(msg, m.keys.Enter):
		return m.openEditor()
	}

	// Check focus section keys 1-9.
	for i := 0; i < 9; i++ {
		if key.Matches(msg, m.keys.FocusSection[i]) {
			sections := m.visibleSections()
			if i < len(sections) {
				m.focusedView = sections[i].Title
				m.rebuildSections()
				m.cursor = 0
			}
			return m, nil
		}
	}

	return m, nil
}

// openEditor launches the editor for the task at the current cursor position.
func (m Model) openEditor() (tea.Model, tea.Cmd) {
	flatTasks := m.flatTasks()
	if len(flatTasks) == 0 || m.cursor >= len(flatTasks) {
		return m, nil
	}

	task := flatTasks[m.cursor]
	editorName := editor.ResolveEditor(m.editorCmd)

	filePath := task.File
	if m.config != nil && m.config.NotesDir != "" {
		filePath = filepath.Join(m.config.NotesDir, task.File)
	}

	cmd := editor.Command(editorName, filePath, task.Line)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return EditorFinishedMsg{Err: err}
	})
}

// View implements tea.Model.
func (m Model) View() string {
	var errLine string
	if m.err != nil {
		errLine = ErrorStyle().Render("Error: "+m.err.Error()) + "\n"
	}

	if m.showSummary {
		return errLine + m.viewSummary()
	}

	switch m.mode {
	case modeAllTasks:
		return errLine + m.viewAllTasks()
	case modeTagSearch:
		return errLine + m.viewTagSearch()
	}

	if m.focusedView != "" {
		return errLine + m.viewFocused()
	}

	return errLine + m.viewDashboard()
}

func (m Model) viewDashboard() string {
	body, _ := m.renderSections()

	openCount := m.countOpen()
	footer := FooterStyle().Render(fmt.Sprintf("%s %d open", strings.Repeat("\u2500", max(0, m.width-10)), openCount))

	full := body + "\n" + footer
	return m.truncateView(full)
}

func (m Model) viewFocused() string {
	body, count := m.renderSections()
	if count == 0 {
		return body + "\nNo tasks"
	}
	return m.truncateView(body)
}

func (m Model) viewSummary() string {
	open := m.countOpen()
	overdue := m.countOverdue()
	dueThisWeek := m.countDueThisWeek()
	completedThisWeek := m.countCompletedThisWeek()

	return RenderSummary(open, overdue, dueThisWeek, completedThisWeek, m.width)
}

// displaySections returns the sections to display based on focus mode.
func (m Model) displaySections() []filter.ViewResult {
	if m.focusedView != "" {
		// Find the matching section in m.sections (which may be filtered) by title.
		for _, sec := range m.sections {
			if sec.Title == m.focusedView {
				return []filter.ViewResult{sec}
			}
		}
		// Fallback: check the unfiltered visible sections.
		for _, sec := range m.visibleSections() {
			if sec.Title == m.focusedView {
				return []filter.ViewResult{sec}
			}
		}
		return nil
	}
	return m.sections
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

// rebuildSections recomputes sections from allTasks, applying filter text.
func (m *Model) rebuildSections() {
	// In all-tasks mode, show all open (non-completed) tasks in a single section.
	if m.mode == modeAllTasks {
		var tasks []model.Task
		for _, t := range m.allTasks {
			if t.HasCheckbox && t.State != model.Completed {
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

		sections := []filter.ViewResult{{
			Title: "All Open Tasks",
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
			// Tag match: check parsed tags by name.
			tagName := strings.ToLower(term[1:])
			for _, tag := range t.Tags {
				if strings.ToLower(tag.Name) == tagName {
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
	m.filterInput.SetValue("")
	m.filterInput.Prompt = "/ "
	m.filterInput.Placeholder = "type to filter..."
	m.filterInput.Blur()
}

// renderSections renders the filter bar (if active) and all non-empty sections.
// Returns the rendered string and the total number of tasks rendered.
func (m Model) renderSections() (string, int) {
	var parts []string

	if m.filtering {
		parts = append(parts, m.filterInput.View())
		parts = append(parts, "")
	}

	sections := m.displaySections()
	flatIdx := 0
	for _, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		hiddenCount := m.hiddenCountFor(sec.Title)
		rendered := RenderSection(sec.Title, sec.Tasks, sec.Color, m.cursor, flatIdx, m.tagColors, m.width, m.config.LinkColor, hiddenCount)
		if rendered != "" {
			parts = append(parts, rendered)
		}
		flatIdx += len(sec.Tasks)
	}

	return strings.Join(parts, "\n"), flatIdx
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

// truncateView ensures the rendered output fits within the terminal height.
// It finds the line containing the cursor marker (reverse video) and shows a
// window of m.height lines centered on it.
func (m Model) truncateView(s string) string {
	return m.truncateViewPinTop(s, 0)
}

// truncateViewPinTop ensures the rendered output fits within the terminal
// height, keeping the first pinnedTop lines always visible. The remaining
// lines scroll to keep the cursor-selected line on screen.
func (m Model) truncateViewPinTop(s string, pinnedTop int) string {
	if m.height <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	maxLines := m.height - 1
	if len(lines) <= maxLines {
		return s
	}

	if pinnedTop >= maxLines {
		pinnedTop = 0
	}

	top := lines[:pinnedTop]
	rest := lines[pinnedTop:]

	// Find the line with the selected cursor (reverse-video escape).
	cursorIdx := 0
	for i, line := range rest {
		if strings.Contains(line, "\x1b[7m") {
			cursorIdx = i
			break
		}
	}

	available := maxLines - pinnedTop
	start, end := scrollWindow(cursorIdx, len(rest), available)
	result := append(top, rest[start:end]...)
	return strings.Join(result, "\n")
}

// scrollWindow computes the start/end indices of a task window of size
// maxVisible, centered on cursor. Clamps to [0, total).
func scrollWindow(cursor, total, maxVisible int) (start, end int) {
	start = cursor - maxVisible/2
	if start < 0 {
		start = 0
	}
	end = start + maxVisible
	if end > total {
		end = total
		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}
	return
}

func (m Model) nowFunc() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

// viewAllTasks renders all open tasks in a single section with filter bar.
// Tasks are windowed to fit the terminal height, keeping the cursor visible.
func (m Model) viewAllTasks() string {
	var parts []string

	// Always show the search bar in all-tasks mode.
	parts = append(parts, m.filterInput.View())
	parts = append(parts, "")

	sections := m.displaySections()
	if len(sections) == 0 || len(sections[0].Tasks) == 0 {
		parts = append(parts, "  No matching tasks")
		return strings.Join(parts, "\n")
	}

	sec := sections[0]
	tasks := sec.Tasks

	// Calculate available lines for task rows inside the border box.
	// Overhead: search bar (1) + blank line (1) + section header (1)
	//           + border top (1) + border bottom (1)
	//           + scroll footer (1) + bubbletea rendering (2) = 8
	overhead := 8

	maxVisible := m.height - overhead
	if maxVisible < 3 {
		maxVisible = 3
	}

	hiddenCount := m.hiddenCountFor(sec.Title)

	if len(tasks) <= maxVisible {
		rendered := RenderSection(sec.Title, tasks, sec.Color, m.cursor, 0, m.tagColors, m.width, m.config.LinkColor, hiddenCount)
		parts = append(parts, rendered)
	} else {
		start, end := scrollWindow(m.cursor, len(tasks), maxVisible)
		rendered := RenderSection(sec.Title, tasks[start:end], sec.Color, m.cursor, start, m.tagColors, m.width, m.config.LinkColor, hiddenCount)
		parts = append(parts, rendered)
		parts = append(parts, FooterStyle().Render(fmt.Sprintf("  %d–%d of %d tasks", start+1, end, len(tasks))))
	}

	// Pin the search bar (first 2 lines) and truncate the rest to fit.
	return m.truncateViewPinTop(strings.Join(parts, "\n"), 2)
}

// viewTagSearch renders the tag picker with filter bar.
func (m Model) viewTagSearch() string {
	var parts []string

	parts = append(parts, m.filterInput.View())
	parts = append(parts, "")

	tags := m.filteredTags()
	if len(tags) == 0 {
		parts = append(parts, "  No matching tags")
	} else {
		for i, tag := range tags {
			line := fmt.Sprintf("  @%s", tag)
			if i == m.tagCursor {
				line = TaskStyle(true).Render(fmt.Sprintf("▸ @%s", tag))
			}
			parts = append(parts, line)
		}
	}

	return strings.Join(parts, "\n")
}

// handleTagSearchKey handles key events in tag search mode.
func (m Model) handleTagSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.mode = modeDashboard
		m.clearFilter()
		m.rebuildSections()
		m.clampCursor()
		return m, nil

	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case msg.Type == tea.KeyDown || msg.Type == tea.KeyCtrlN:
		tags := m.filteredTags()
		if m.tagCursor < len(tags)-1 {
			m.tagCursor++
		}
		return m, nil

	case msg.Type == tea.KeyUp || msg.Type == tea.KeyCtrlP:
		if m.tagCursor > 0 {
			m.tagCursor--
		}
		return m, nil

	case msg.Type == tea.KeyEnter:
		// Select tag → switch to all-tasks mode filtered to @tag.
		tags := m.filteredTags()
		if m.tagCursor < len(tags) {
			selected := tags[m.tagCursor]
			m.mode = modeAllTasks
			m.filtering = true
			m.filterText = "@" + selected
			m.filterInput.SetValue("@" + selected)
			m.filterInput.Prompt = "> "
			m.filterInput.Placeholder = "search all open tasks..."
			cmd := m.filterInput.Focus()
			m.cursor = 0
			m.rebuildSections()
			m.clampCursor()
			return m, cmd
		}
		return m, nil

	default:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.filterText = m.filterInput.Value()
		m.tagCursor = 0
		return m, cmd
	}
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
	lower := strings.ToLower(m.filterText)
	var result []string
	for _, tag := range m.tagList {
		if strings.Contains(strings.ToLower(tag), lower) {
			result = append(result, tag)
		}
	}
	return result
}
