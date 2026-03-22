package tui

import (
	"context"
	"path/filepath"

	"github.com/zachthieme/pike/internal/editor"
	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/toggle"

	tea "github.com/charmbracelet/bubbletea"
)

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
