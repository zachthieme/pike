package tui

import (
	"context"
	"path/filepath"

	"github.com/zachthieme/pike/internal/editor"
	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/toggle"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Tag search mode: delegate all keys to TagSearch sub-model.
	if m.mode == modeTagSearch {
		var cmd tea.Cmd
		m.tagSearch, cmd = m.tagSearch.Update(msg)
		return m, cmd
	}

	// Recently-completed: intercept Escape before FilterBar.
	if m.mode == modeRecentlyCompleted && m.filterBar.Active() && key.Matches(msg, m.keys.Escape) {
		m.exitToDashboard()
		return m, nil
	}

	// FilterBar active: delegate to specialized handlers.
	if m.filterBar.Active() {
		if m.filterBar.InputFocused() {
			return m.handleKeyFilterInput(msg)
		}
		return m.handleKeyFilterResults(msg)
	}

	// Custom bindings — checked before built-in keys so custom wins on conflict.
	if !m.viewLocked {
		if model, cmd, handled := m.handleKeyCustomBinding(msg); handled {
			return model, cmd
		}
	}

	return m.handleKeyDashboard(msg)
}

// handleKeyFilterInput handles keys when the FilterBar is active and the text
// input is focused. Rune keys go to FilterBar for typing; only arrow keys and
// ctrl shortcuts navigate.
func (m Model) handleKeyFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type != tea.KeyRunes {
		switch {
		case key.Matches(msg, m.keys.Down):
			m.nav.CursorDown(m.displaySections())
			return m, nil
		case key.Matches(msg, m.keys.Up):
			m.nav.CursorUp()
			return m, nil
		case key.Matches(msg, m.keys.PageDown):
			m.nav.PageScroll(1, m.displaySections())
			return m, tea.ClearScreen
		case key.Matches(msg, m.keys.PageUp):
			m.nav.PageScroll(-1, m.displaySections())
			return m, tea.ClearScreen
		}
	}
	var cmd tea.Cmd
	m.filterBar, cmd = m.filterBar.Update(msg)
	return m.processFilterOutput(cmd)
}

// handleKeyFilterResults handles keys when the FilterBar is active but the
// text input is not focused (user is navigating filtered results).
func (m Model) handleKeyFilterResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Filter bar keys (escape, tab, /, ?) — must be checked before general
	// navigation because NextSection (tab) drives mode cycling in the filter bar.
	switch {
	case key.Matches(msg, m.keys.Escape), key.Matches(msg, m.keys.NextSection),
		key.Matches(msg, m.keys.Filter), key.Matches(msg, m.keys.Query):
		var cmd tea.Cmd
		m.filterBar, cmd = m.filterBar.Update(msg)
		return m.processFilterOutput(cmd)
	case key.Matches(msg, m.keys.Quit):
		m.exitToDashboard()
		return m, nil
	}
	if mdl, cmd, handled := m.handleTaskAction(msg); handled {
		return mdl, cmd
	}
	if mdl, cmd, handled := m.handleCursorMovement(msg); handled {
		return mdl, cmd
	}
	return m, nil
}

// handleKeyCustomBinding checks user-configured custom key bindings via O(1) map lookup.
// Returns handled=true if a binding matched.
func (m Model) handleKeyCustomBinding(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	i, ok := m.customKeyIndex[msg.String()]
	if !ok {
		return m, nil, false
	}
	cb := m.customBindings[i]
	if cb.View != "" {
		for _, sec := range m.visibleSections() {
			if sec.Title == cb.View {
				m.mode = modeFocused
				m.focusedView = cb.View
				m.rebuildSections()
				m.nav.JumpToTop()
				break
			}
		}
		return m, nil, true
	}
	if cb.Query != "" {
		cmd := m.enterQueryMode(cb.Query, cb.Sort)
		return m, cmd, true
	}
	return m, nil, true
}

// handleKeyDashboard handles keys in the default dashboard/navigation mode.
func (m Model) handleKeyDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Escape):
		// Escape priority: dismiss summary → unfocus section → exit mode → do nothing.
		switch {
		case m.showSummary:
			m.showSummary = false
		case m.mode == modeFocused && !m.viewLocked:
			m.mode = modeDashboard
			m.focusedView = ""
			m.rebuildSections()
			m.nav.ClampCursor(m.displaySections())
		case m.mode != modeDashboard && m.mode != modeFocused:
			m.exitToDashboard()
		}
		return m, nil

	case key.Matches(msg, m.keys.Summary):
		m.showSummary = !m.showSummary
		return m, nil

	case key.Matches(msg, m.keys.Filter):
		var cmd tea.Cmd
		m.filterBar, cmd = m.filterBar.Update(FilterActivateMsg{
			Mode:         filterSubstring,
			InitialValue: "",
			Placeholder:  "type to filter...",
		})
		return m, cmd

	case key.Matches(msg, m.keys.Query):
		var cmd tea.Cmd
		m.filterBar, cmd = m.filterBar.Update(FilterActivateMsg{
			Mode:         filterQuery,
			InitialValue: "",
			Placeholder:  "type to filter...",
		})
		return m, cmd

	case key.Matches(msg, m.keys.AllTasks):
		focusCmd := m.enterAllTasksMode(false, "")
		return m, tea.Batch(focusCmd, tea.ClearScreen)

	case key.Matches(msg, m.keys.TagSearch):
		focusCmd := m.enterTagSearchMode()
		return m, tea.Batch(focusCmd, tea.ClearScreen)

	case key.Matches(msg, m.keys.RecentlyCompleted):
		if m.mode == modeRecentlyCompleted {
			return m, nil
		}
		focusCmd := m.enterRecentlyCompletedMode()
		return m, tea.Batch(focusCmd, tea.ClearScreen)

	case key.Matches(msg, m.keys.NextSection):
		m.nav.JumpToNextSection(m.displaySections())
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		return m, func() tea.Msg { return RefreshMsg{} }
	}

	if mdl, cmd, handled := m.handleTaskAction(msg); handled {
		return mdl, cmd
	}
	if mdl, cmd, handled := m.handleCursorMovement(msg); handled {
		return mdl, cmd
	}

	// Check focus section keys 1-9.
	if !m.viewLocked && len(m.customBindings) == 0 {
		for i := range 9 {
			if key.Matches(msg, m.keys.FocusSection[i]) {
				sections := m.visibleSections()
				if i < len(sections) {
					m.mode = modeFocused
					m.focusedView = sections[i].Title
					m.rebuildSections()
					m.nav.JumpToTop()
				}
				return m, nil
			}
		}
	}

	return m, nil
}

// handleCursorMovement handles cursor movement keys shared across modes.
// NextSection is intentionally excluded: it has mode-specific behavior
// (section jump in dashboard, filter-bar mode cycling in filter results)
// and is handled by each caller's own switch block.
// Returns handled=true if a movement key was matched.
func (m Model) handleCursorMovement(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keys.Down):
		m.nav.CursorDown(m.displaySections())
		return m, nil, true
	case key.Matches(msg, m.keys.Up):
		m.nav.CursorUp()
		return m, nil, true
	case key.Matches(msg, m.keys.Top):
		m.nav.JumpToTop()
		return m, nil, true
	case key.Matches(msg, m.keys.Bottom):
		m.nav.JumpToBottom(m.displaySections())
		return m, nil, true
	case key.Matches(msg, m.keys.PageDown):
		m.nav.PageScroll(1, m.displaySections())
		return m, tea.ClearScreen, true
	case key.Matches(msg, m.keys.PageUp):
		m.nav.PageScroll(-1, m.displaySections())
		return m, tea.ClearScreen, true
	case key.Matches(msg, m.keys.PrevSection):
		m.nav.JumpToPrevSection(m.displaySections())
		return m, nil, true
	}
	return m, nil, false
}

// handleTaskAction handles task-level action keys shared across modes.
// Returns handled=true if an action key was matched.
func (m Model) handleTaskAction(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keys.Toggle):
		mdl, cmd := m.toggleTask()
		return mdl, cmd, true
	case key.Matches(msg, m.keys.ToggleHiddenTag):
		mdl, cmd := m.toggleHiddenTag()
		return mdl, cmd, true
	case key.Matches(msg, m.keys.ToggleHidden):
		m.showHidden = !m.showHidden
		m.rebuildSections()
		m.nav.ClampCursor(m.displaySections())
		return m, nil, true
	case key.Matches(msg, m.keys.Enter):
		mdl, cmd := m.openEditor()
		return mdl, cmd, true
	}
	return m, nil, false
}

// processFilterOutput checks FilterBar.Output() for a pending action message
// and handles it inline. The tea.Cmd from FilterBar.Update is passed through
// to the Bubble Tea runtime without being eagerly executed. Returns any
// additional cmds from cross-model transitions (e.g., tag search focus).
func (m Model) processFilterOutput(filterCmd tea.Cmd) (tea.Model, tea.Cmd) {
	output := m.filterBar.Output()
	if output == nil {
		return m, filterCmd
	}

	var extraCmd tea.Cmd
	switch fmsg := output.(type) {
	case FilterChangedMsg:
		if m.showAll && fmsg.Text == "" && m.mode != modeRecentlyCompleted {
			m.filterBar, _ = m.filterBar.Update(FilterDeactivateMsg{})
			extraCmd = m.enterTagSearchMode()
		} else {
			m.rebuildSections()
			m.nav.ClampCursor(m.displaySections())
		}
	case FilterSubmittedMsg:
		// No-op — focus transition handled inside FilterBar.
	case FilterClearedMsg:
		if m.showAll {
			m.filterBar, _ = m.filterBar.Update(FilterDeactivateMsg{})
			extraCmd = m.enterTagSearchMode()
		} else {
			m.filterBar, _ = m.filterBar.Update(FilterDeactivateMsg{})
			m.showAll = false
			if m.mode == modeAllTasks {
				m.mode = modeDashboard
			}
			m.rebuildSections()
			m.nav.ClampCursor(m.displaySections())
		}
	case FilterModeChangedMsg:
		m.rebuildSections()
		m.nav.ClampCursor(m.displaySections())
	}

	// Combine the FilterBar's tea.Cmd (textinput blink/focus) with any
	// extra cmd from cross-model transitions (tag search focus).
	switch {
	case filterCmd == nil && extraCmd == nil:
		return m, nil
	case filterCmd == nil:
		return m, extraCmd
	case extraCmd == nil:
		return m, filterCmd
	default:
		return m, tea.Batch(filterCmd, extraCmd)
	}
}

// openEditor launches the editor for the task at the current cursor position.
func (m Model) openEditor() (tea.Model, tea.Cmd) {
	tasks := flatTasks(m.displaySections())
	if len(tasks) == 0 || m.nav.Cursor() >= len(tasks) {
		return m, nil
	}

	task := tasks[m.nav.Cursor()]
	editorName := editor.ResolveEditor(m.editorCmd)

	filePath := m.resolveFilePath(task.File)

	cmd := editor.Command(editorName, filePath, task.Line)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return EditorFinishedMsg{Err: err}
	})
}

// resolveFilePath returns the absolute path for a task's file.
func (m Model) resolveFilePath(relPath string) string {
	if m.config != nil && m.config.NotesDir != "" {
		return filepath.Join(m.config.NotesDir, relPath)
	}
	return relPath
}

// toggleTask completes or uncompletes the task at the cursor asynchronously.
func (m Model) toggleTask() (tea.Model, tea.Cmd) {
	tasks := flatTasks(m.displaySections())
	if len(tasks) == 0 || m.nav.Cursor() >= len(tasks) {
		return m, nil
	}
	task := tasks[m.nav.Cursor()]
	if !task.HasCheckbox {
		return m, nil
	}

	filePath := m.resolveFilePath(task.File)
	state := task.State
	line := task.Line
	now := m.nowFunc()

	return m, func() tea.Msg {
		var err error
		if state == model.Open {
			err = toggle.Complete(context.Background(), filePath, line, now)
		} else {
			err = toggle.Uncomplete(context.Background(), filePath, line)
		}
		return toggleResultMsg{Err: err}
	}
}

// toggleHiddenTag adds or removes @hidden from the task at the cursor asynchronously.
func (m Model) toggleHiddenTag() (tea.Model, tea.Cmd) {
	tasks := flatTasks(m.displaySections())
	if len(tasks) == 0 || m.nav.Cursor() >= len(tasks) {
		return m, nil
	}
	task := tasks[m.nav.Cursor()]
	filePath := m.resolveFilePath(task.File)
	line := task.Line

	return m, func() tea.Msg {
		return toggleResultMsg{Err: toggle.ToggleHidden(context.Background(), filePath, line)}
	}
}

