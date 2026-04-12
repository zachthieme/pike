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
// When toggling a child task, cascades to the parent:
//   - Completing last open child -> auto-completes parent (using toggle date)
//   - Uncompleting a child of a completed parent -> uncompletes parent
//
// Auto-complete only applies to parents with HasCheckbox.
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

	// Capture parent info for auto-complete cascade
	var parentFile string
	var parentLine int
	var parentState model.TaskState
	var parentHasCheckbox bool
	var siblingsDone, siblingsTotal int
	hasParent := task.Indent > 0 && task.ParentIndex >= 0 && task.ParentIndex < len(m.allTasks)

	if hasParent {
		parent := m.allTasks[task.ParentIndex]
		parentFile = parent.File
		parentLine = parent.Line
		parentState = parent.State
		parentHasCheckbox = parent.HasCheckbox
		siblingsDone, siblingsTotal = parent.Progress(m.allTasks)
	}

	notesDir := ""
	if m.config != nil {
		notesDir = m.config.NotesDir
	}

	return m, func() tea.Msg {
		ctx := context.Background()
		if state == model.Open {
			if err := toggle.Complete(ctx, filePath, line, now); err != nil {
				return toggleResultMsg{Err: err}
			}
			// Auto-complete parent if this was the last open child
			if hasParent && parentHasCheckbox && parentState == model.Open && siblingsDone+1 == siblingsTotal {
				parentPath := parentFile
				if notesDir != "" {
					parentPath = filepath.Join(notesDir, parentFile)
				}
				_ = toggle.Complete(ctx, parentPath, parentLine, now)
			}
		} else {
			if err := toggle.Uncomplete(ctx, filePath, line); err != nil {
				return toggleResultMsg{Err: err}
			}
			// Auto-uncomplete parent if it was completed
			if hasParent && parentHasCheckbox && parentState == model.Completed {
				parentPath := parentFile
				if notesDir != "" {
					parentPath = filepath.Join(notesDir, parentFile)
				}
				_ = toggle.Uncomplete(ctx, parentPath, parentLine)
			}
		}
		return toggleResultMsg{Err: nil}
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

// toggleCollapse expands or collapses the subtask list for the parent at the cursor.
// No-op if the cursor is on a child or a task with no children.
func (m *Model) toggleCollapse() {
	tasks := flatTasks(m.displaySections())
	if len(tasks) == 0 || m.nav.Cursor() >= len(tasks) {
		return
	}
	task := tasks[m.nav.Cursor()]
	if !task.HasChildren() {
		return // only toggle on parents
	}
	key := task.Key()
	if m.expanded[key] {
		delete(m.expanded, key)
	} else {
		m.expanded[key] = true
	}
	m.rebuildSections()
	m.nav.ClampCursor(m.displaySections())
}
