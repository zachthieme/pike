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
	// Tag search mode: navigate/filter tag list.
	if m.mode == modeTagSearch {
		return m.handleTagSearchKey(msg)
	}

	// If filtering, handle keys based on whether the query bar or results have focus.
	if m.filtering {
		switch {
		case key.Matches(msg, m.keys.Escape):
			// If results are focused, return focus to query bar.
			if !m.filterInput.Focused() {
				cmd := m.filterInput.Focus()
				return m, cmd
			}
			// If query bar has content, clear it; otherwise exit filter mode.
			if m.filterInput.Value() != "" {
				m.filterInput.SetValue("")
				m.filterText = ""
				// If we came from tag search, return there instead of showing empty results.
				if m.showAll && m.mode != modeRecentlyCompleted {
					focusCmd := m.enterTagSearchMode()
					return m, focusCmd
				}
				m.rebuildSections()
				m.clampCursor()
				return m, nil
			}
			m.clearFilter()
			if m.mode == modeAllTasks || m.mode == modeRecentlyCompleted {
				m.mode = modeDashboard
			}
			m.rebuildSections()
			m.clampCursor()
			return m, nil
		case key.Matches(msg, m.keys.NextSection):
			// Tab: toggle focus between query bar and results.
			if m.filterInput.Focused() {
				m.filterInput.Blur()
			} else {
				cmd := m.filterInput.Focus()
				return m, cmd
			}
			return m, nil
		case msg.Type == tea.KeyEnter:
			if m.filterInput.Focused() {
				// Submit query: move focus to results.
				m.filterInput.Blur()
				return m, nil
			}
			return m.openEditor()
		case msg.Type == tea.KeyDown || msg.Type == tea.KeyCtrlN:
			m.cursorDown()
			return m, nil
		case msg.Type == tea.KeyUp || msg.Type == tea.KeyCtrlP:
			m.cursorUp()
			return m, nil
		case msg.Type == tea.KeyCtrlD:
			m.pageScroll(1)
			return m, tea.ClearScreen
		case msg.Type == tea.KeyCtrlU:
			m.pageScroll(-1)
			return m, tea.ClearScreen
		}

		// When results are focused, handle navigation and action keys directly.
		if !m.filterInput.Focused() {
			switch {
			case key.Matches(msg, m.keys.Toggle):
				return m.toggleTask()
			case key.Matches(msg, m.keys.ToggleHiddenTag):
				return m.toggleHiddenTag()
			case key.Matches(msg, m.keys.Up):
				m.cursorUp()
				return m, nil
			case key.Matches(msg, m.keys.Down):
				m.cursorDown()
				return m, nil
			case key.Matches(msg, m.keys.Top):
				m.cursor = 0
				return m, nil
			case key.Matches(msg, m.keys.Bottom):
				m.cursor = max(0, m.countFlatTasks()-1)
				return m, nil
			case key.Matches(msg, m.keys.Filter):
				return m, m.setFilterMode(filterSubstring)
			case key.Matches(msg, m.keys.Query):
				return m, m.setFilterMode(filterQuery)
			case key.Matches(msg, m.keys.PrevSection):
				m.jumpToPrevSection()
				return m, nil
			}
			// Ignore other keys when results are focused (don't type into input).
			return m, nil
		}

		// Query bar is focused — route to text input.
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.filterText = m.filterInput.Value()
		// If we came from tag search and filter is now empty, return to tag search.
		if m.showAll && m.filterText == "" && m.mode != modeRecentlyCompleted {
			focusCmd := m.enterTagSearchMode()
			return m, tea.Batch(cmd, focusCmd)
		}
		m.rebuildSections()
		m.clampCursor()
		return m, cmd
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
			m.showAll = false
			m.rebuildSections()
			m.clampCursor()
		} else if m.focusedView != "" {
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

	case msg.Type == tea.KeyCtrlD:
		m.pageScroll(1)
		return m, tea.ClearScreen

	case msg.Type == tea.KeyCtrlU:
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
		return m, m.setFilterMode(filterSubstring)

	case key.Matches(msg, m.keys.Query):
		return m, m.setFilterMode(filterQuery)

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

// toggleTask completes or uncompletes the task at the cursor.
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

	var err error
	if task.State == model.Open {
		now := m.nowFunc()
		err = toggle.Complete(filePath, task.Line, now)
	} else {
		err = toggle.Uncomplete(filePath, task.Line)
	}
	if err != nil {
		m.err = err
		return m, nil
	}
	return m, func() tea.Msg { return RefreshMsg{} }
}

// toggleHiddenTag adds or removes @hidden from the task at the cursor.
func (m Model) toggleHiddenTag() (tea.Model, tea.Cmd) {
	flatTasks := m.flatTasks()
	if len(flatTasks) == 0 || m.cursor >= len(flatTasks) {
		return m, nil
	}
	task := flatTasks[m.cursor]
	filePath := m.resolveFilePath(task.File)

	err := toggle.ToggleHidden(filePath, task.Line)
	if err != nil {
		m.err = err
		return m, nil
	}
	return m, func() tea.Msg { return RefreshMsg{} }
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

	case key.Matches(msg, m.keys.NextSection) || msg.Type == tea.KeyDown || msg.Type == tea.KeyCtrlN:
		// Tab / Down / Ctrl-N: cycle forward through matched tags.
		tags := m.filteredTags()
		if len(tags) > 0 {
			m.tagCursor = (m.tagCursor + 1) % len(tags)
		}
		return m, nil

	case key.Matches(msg, m.keys.PrevSection) || msg.Type == tea.KeyUp || msg.Type == tea.KeyCtrlP:
		// Shift-Tab / Up / Ctrl-P: cycle backward through matched tags.
		tags := m.filteredTags()
		if len(tags) > 0 {
			m.tagCursor = (m.tagCursor - 1 + len(tags)) % len(tags)
		}
		return m, nil

	case msg.Type == tea.KeyEnter:
		// Select tag → switch to all-tasks mode filtered to @tag (including completed).
		tags := m.filteredTags()
		if m.tagCursor < len(tags) {
			selectedTag := tags[m.tagCursor]
			if selectedTag == "hidden" {
				m.showHidden = true
			}
			cmd := m.enterAllTasksMode(true, "@"+selectedTag)
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
