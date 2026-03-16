package tui

import (
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

	// FilterBar active + input focused: navigation keys move cursor,
	// all other keys go to FilterBar.
	if m.filterBar.Active() && m.filterBar.InputFocused() {
		switch {
		case key.Matches(msg, m.keys.Down):
			m.cursorDown()
			return m, nil
		case key.Matches(msg, m.keys.Up):
			m.cursorUp()
			return m, nil
		case key.Matches(msg, m.keys.PageDown):
			m.pageScroll(1)
			return m, tea.ClearScreen
		case key.Matches(msg, m.keys.PageUp):
			m.pageScroll(-1)
			return m, tea.ClearScreen
		}
		var cmd tea.Cmd
		m.filterBar, cmd = m.filterBar.Update(msg)
		return m.processFilterCmd(cmd)
	}

	// FilterBar active + results focused: only certain keys to FilterBar.
	if m.filterBar.Active() {
		switch {
		case key.Matches(msg, m.keys.Escape), key.Matches(msg, m.keys.NextSection),
			key.Matches(msg, m.keys.Filter), key.Matches(msg, m.keys.Query):
			var cmd tea.Cmd
			m.filterBar, cmd = m.filterBar.Update(msg)
			return m.processFilterCmd(cmd)
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Toggle):
			return m.toggleTask()
		case key.Matches(msg, m.keys.ToggleHiddenTag):
			return m.toggleHiddenTag()
		case key.Matches(msg, m.keys.ToggleHidden):
			m.showHidden = !m.showHidden
			m.rebuildSections()
			m.clampCursor()
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.cursorDown()
			return m, nil
		case key.Matches(msg, m.keys.Up):
			m.cursorUp()
			return m, nil
		case key.Matches(msg, m.keys.Top):
			m.cursor = 0
			return m, nil
		case key.Matches(msg, m.keys.Bottom):
			m.cursor = max(0, m.countFlatTasks()-1)
			return m, nil
		case key.Matches(msg, m.keys.PageDown):
			m.pageScroll(1)
			return m, tea.ClearScreen
		case key.Matches(msg, m.keys.PageUp):
			m.pageScroll(-1)
			return m, tea.ClearScreen
		case key.Matches(msg, m.keys.PrevSection):
			m.jumpToPrevSection()
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			return m.openEditor()
		}
		return m, nil
	}

	// Dashboard/navigation keys.
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Escape):
		// Escape priority: dismiss summary -> exit mode -> exit focus -> do nothing
		if m.showSummary {
			m.showSummary = false
		} else if m.mode != modeDashboard {
			m.exitToDashboard()
		} else if m.focusedView != "" && !m.viewLocked {
			m.focusedView = ""
			m.rebuildSections()
			m.clampCursor()
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.cursorDown()
		return m, nil

	case key.Matches(msg, m.keys.Up):
		m.cursorUp()
		return m, nil

	case key.Matches(msg, m.keys.Top):
		m.cursor = 0
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		m.cursor = max(0, m.countFlatTasks()-1)
		return m, nil

	case key.Matches(msg, m.keys.PageDown):
		m.pageScroll(1)
		return m, tea.ClearScreen

	case key.Matches(msg, m.keys.PageUp):
		m.pageScroll(-1)
		return m, tea.ClearScreen

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
		return m, tea.Batch(focusCmd, func() tea.Msg { return tea.ClearScreen() })

	case key.Matches(msg, m.keys.TagSearch):
		focusCmd := m.enterTagSearchMode()
		return m, tea.Batch(focusCmd, func() tea.Msg { return tea.ClearScreen() })

	case key.Matches(msg, m.keys.RecentlyCompleted):
		if m.mode == modeRecentlyCompleted {
			return m, nil
		}
		focusCmd := m.enterRecentlyCompletedMode()
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

	case key.Matches(msg, m.keys.Toggle):
		return m.toggleTask()

	case key.Matches(msg, m.keys.ToggleHiddenTag):
		return m.toggleHiddenTag()
	}

	// Check focus section keys 1-9.
	if !m.viewLocked {
		for i := range 9 {
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
	}

	return m, nil
}

// processFilterCmd executes the cmd returned by FilterBar.Update and handles
// filter output messages inline. Non-filter cmds are preserved for the runtime.
func (m Model) processFilterCmd(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	if cmd == nil {
		return m, nil
	}
	msg := cmd()
	if msg == nil {
		return m, nil
	}

	// Handle tea.BatchMsg: extract filter messages, keep the rest.
	if batch, ok := msg.(tea.BatchMsg); ok {
		var remaining []tea.Cmd
		for _, c := range batch {
			if c == nil {
				continue
			}
			batchMsg := c()
			if !m.handleFilterOutputMsg(batchMsg) {
				capturedMsg := batchMsg
				remaining = append(remaining, func() tea.Msg { return capturedMsg })
			}
		}
		switch len(remaining) {
		case 0:
			return m, nil
		case 1:
			return m, remaining[0]
		default:
			return m, tea.Batch(remaining...)
		}
	}

	// Single message.
	if m.handleFilterOutputMsg(msg) {
		return m, nil
	}
	capturedMsg := msg
	return m, func() tea.Msg { return capturedMsg }
}

// handleFilterOutputMsg processes a filter output message inline. Returns true if handled.
func (m *Model) handleFilterOutputMsg(msg tea.Msg) bool {
	switch fmsg := msg.(type) {
	case FilterChangedMsg:
		if m.showAll && fmsg.Text == "" && m.mode != modeRecentlyCompleted {
			m.filterBar, _ = m.filterBar.Update(FilterDeactivateMsg{})
			m.enterTagSearchMode()
			return true
		}
		m.rebuildSections()
		m.clampCursor()
		return true
	case FilterSubmittedMsg:
		return true
	case FilterClearedMsg:
		if m.showAll {
			m.filterBar, _ = m.filterBar.Update(FilterDeactivateMsg{})
			m.enterTagSearchMode()
			return true
		}
		m.filterBar, _ = m.filterBar.Update(FilterDeactivateMsg{})
		m.showAll = false
		if m.mode == modeAllTasks {
			m.mode = modeDashboard
		}
		m.rebuildSections()
		m.clampCursor()
		return true
	case FilterModeChangedMsg:
		m.rebuildSections()
		m.clampCursor()
		return true
	}
	return false
}

// openEditor launches the editor for the task at the current cursor position.
func (m Model) openEditor() (tea.Model, tea.Cmd) {
	flatTasks := m.flatTasks()
	if len(flatTasks) == 0 || m.cursor >= len(flatTasks) {
		return m, nil
	}

	task := flatTasks[m.cursor]
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
	flatTasks := m.flatTasks()
	if len(flatTasks) == 0 || m.cursor >= len(flatTasks) {
		return m, nil
	}
	task := flatTasks[m.cursor]
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
			err = toggle.Complete(filePath, line, now)
		} else {
			err = toggle.Uncomplete(filePath, line)
		}
		return toggleResultMsg{Err: err}
	}
}

// toggleHiddenTag adds or removes @hidden from the task at the cursor asynchronously.
func (m Model) toggleHiddenTag() (tea.Model, tea.Cmd) {
	flatTasks := m.flatTasks()
	if len(flatTasks) == 0 || m.cursor >= len(flatTasks) {
		return m, nil
	}
	task := flatTasks[m.cursor]
	filePath := m.resolveFilePath(task.File)
	line := task.Line

	return m, func() tea.Msg {
		return toggleResultMsg{Err: toggle.ToggleHidden(filePath, line)}
	}
}

