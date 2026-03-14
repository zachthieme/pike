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

	// If filtering, handle text input first.
	if m.filtering {
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.clearFilter()
			if m.mode == modeAllTasks || m.mode == modeRecentlyCompleted {
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
		default:
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.filterText = m.filterInput.Value()
			// If we came from tag search and filter is now empty, return to tag search.
			if m.showAll && m.filterText == "" && m.mode != modeRecentlyCompleted {
				m.enterTagSearchMode()
				return m, cmd
			}
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
		m.filtering = true
		cmd := m.filterInput.Focus()
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

	filePath := task.File
	if m.config != nil && m.config.NotesDir != "" {
		filePath = filepath.Join(m.config.NotesDir, task.File)
	}

	var err error
	if task.State == model.Open {
		now := m.nowFunc()
		err = toggle.Complete(filePath, task.Line, now)
	} else {
		err = toggle.Uncomplete(filePath, task.Line)
	}
	if err != nil {
		m.err = err
		return m, func() tea.Msg { return RefreshMsg{} }
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
			cmd := m.enterAllTasksMode(true, "@"+tags[m.tagCursor])
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
