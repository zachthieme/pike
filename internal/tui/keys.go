package tui

import (
	"context"
	"path/filepath"

	"pike/internal/editor"
	"pike/internal/model"
	"pike/internal/toggle"

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

	// FilterBar active + input focused: rune keys (j, k, q, etc.) go to
	// FilterBar for typing. Only arrow keys and ctrl shortcuts navigate.
	if m.filterBar.Active() && m.filterBar.InputFocused() {
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

	// FilterBar active + results focused: only certain keys to FilterBar.
	if m.filterBar.Active() {
		switch {
		case key.Matches(msg, m.keys.Escape), key.Matches(msg, m.keys.NextSection),
			key.Matches(msg, m.keys.Filter), key.Matches(msg, m.keys.Query):
			var cmd tea.Cmd
			m.filterBar, cmd = m.filterBar.Update(msg)
			return m.processFilterOutput(cmd)
		case key.Matches(msg, m.keys.Quit):
			m.exitToDashboard()
			return m, nil
		case key.Matches(msg, m.keys.Toggle):
			return m.toggleTask()
		case key.Matches(msg, m.keys.ToggleHiddenTag):
			return m.toggleHiddenTag()
		case key.Matches(msg, m.keys.ToggleHidden):
			m.showHidden = !m.showHidden
			m.rebuildSections()
			m.nav.ClampCursor(m.displaySections())
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.nav.CursorDown(m.displaySections())
			return m, nil
		case key.Matches(msg, m.keys.Up):
			m.nav.CursorUp()
			return m, nil
		case key.Matches(msg, m.keys.Top):
			m.nav.JumpToTop()
			return m, nil
		case key.Matches(msg, m.keys.Bottom):
			m.nav.JumpToBottom(m.displaySections())
			return m, nil
		case key.Matches(msg, m.keys.PageDown):
			m.nav.PageScroll(1, m.displaySections())
			return m, tea.ClearScreen
		case key.Matches(msg, m.keys.PageUp):
			m.nav.PageScroll(-1, m.displaySections())
			return m, tea.ClearScreen
		case key.Matches(msg, m.keys.PrevSection):
			m.nav.JumpToPrevSection(m.displaySections())
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			return m.openEditor()
		}
		return m, nil
	}

	// Custom bindings — checked before built-in keys so custom wins on conflict.
	// Suppressed when viewLocked (--view flag) to match the behavior of 1-9 focus keys.
	if !m.viewLocked {
		for i, cb := range m.customBindings {
			if key.Matches(msg, m.customKeys[i]) {
				if cb.View != "" {
					for _, sec := range m.visibleSections() {
						if sec.Title == cb.View {
							m.focusedView = cb.View
							m.rebuildSections()
							m.nav.JumpToTop()
							break
						}
					}
					return m, nil
				}
				if cb.Query != "" {
					cmd := m.enterQueryMode(cb.Query, cb.Sort)
					return m, cmd
				}
				return m, nil
			}
		}
	}

	// Dashboard/navigation keys.
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Escape):
		// Escape priority: dismiss summary -> exit mode -> exit focus -> do nothing
		switch {
		case m.showSummary:
			m.showSummary = false
		case m.mode != modeDashboard:
			m.exitToDashboard()
		case m.focusedView != "" && !m.viewLocked:
			m.focusedView = ""
			m.rebuildSections()
			m.nav.ClampCursor(m.displaySections())
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.nav.CursorDown(m.displaySections())
		return m, nil

	case key.Matches(msg, m.keys.Up):
		m.nav.CursorUp()
		return m, nil

	case key.Matches(msg, m.keys.Top):
		m.nav.JumpToTop()
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		m.nav.JumpToBottom(m.displaySections())
		return m, nil

	case key.Matches(msg, m.keys.PageDown):
		m.nav.PageScroll(1, m.displaySections())
		return m, tea.ClearScreen

	case key.Matches(msg, m.keys.PageUp):
		m.nav.PageScroll(-1, m.displaySections())
		return m, tea.ClearScreen

	case key.Matches(msg, m.keys.NextSection):
		m.nav.JumpToNextSection(m.displaySections())
		return m, nil

	case key.Matches(msg, m.keys.PrevSection):
		m.nav.JumpToPrevSection(m.displaySections())
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

	case key.Matches(msg, m.keys.ToggleHidden):
		m.showHidden = !m.showHidden
		m.rebuildSections()
		m.nav.ClampCursor(m.displaySections())
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		return m, func() tea.Msg { return RefreshMsg{} }

	case key.Matches(msg, m.keys.Enter):
		return m.openEditor()

	case key.Matches(msg, m.keys.Toggle):
		return m.toggleTask()

	case key.Matches(msg, m.keys.ToggleHiddenTag):
		return m.toggleHiddenTag()
	}

	// Check focus section keys 1-9.
	if !m.viewLocked && len(m.customBindings) == 0 {
		for i := range 9 {
			if key.Matches(msg, m.keys.FocusSection[i]) {
				sections := m.visibleSections()
				if i < len(sections) {
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

