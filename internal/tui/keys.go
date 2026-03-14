package tui

import (
	"path/filepath"

	"pike/internal/editor"

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
