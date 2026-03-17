package tui

import (
	"pike/internal/filter"
	"pike/internal/model"
)

// pageScrollChrome is the approximate number of non-task lines on screen:
// search bar, section header, borders, footer, bubbletea chrome.
const pageScrollChrome = 8

// Navigator manages cursor state for navigating across task sections.
// Methods that need section data accept []filter.ViewResult as a parameter
// rather than storing a callback, because Bubble Tea's value semantics
// would cause a captured callback to bind to a stale Model copy.
type Navigator struct {
	cursor int
	height int
}

// Cursor returns the current cursor position.
func (n *Navigator) Cursor() int {
	return n.cursor
}

// SetCursor sets the cursor position directly.
func (n *Navigator) SetCursor(pos int) {
	n.cursor = pos
}

// SetHeight updates the viewport height for page scroll calculations.
func (n *Navigator) SetHeight(h int) {
	n.height = h
}

// ClampCursor ensures the cursor is within valid bounds.
func (n *Navigator) ClampCursor(sections []filter.ViewResult) {
	count := countFlatTasks(sections)
	if count == 0 {
		n.cursor = 0
		return
	}
	if n.cursor >= count {
		n.cursor = count - 1
	}
	if n.cursor < 0 {
		n.cursor = 0
	}
}

// CursorDown moves the cursor down one position if possible.
func (n *Navigator) CursorDown(sections []filter.ViewResult) {
	count := countFlatTasks(sections)
	if count > 0 && n.cursor < count-1 {
		n.cursor++
	}
}

// CursorUp moves the cursor up one position if possible.
func (n *Navigator) CursorUp() {
	if n.cursor > 0 {
		n.cursor--
	}
}

// JumpToTop moves the cursor to the first task.
func (n *Navigator) JumpToTop() {
	n.cursor = 0
}

// JumpToBottom moves the cursor to the last task.
func (n *Navigator) JumpToBottom(sections []filter.ViewResult) {
	count := countFlatTasks(sections)
	if count > 0 {
		n.cursor = count - 1
	}
}

// PageScroll moves the cursor by half the visible task window.
// direction is 1 for down, -1 for up.
func (n *Navigator) PageScroll(direction int, sections []filter.ViewResult) {
	visible := max(4, n.height-pageScrollChrome)
	n.cursor += direction * (visible / 2)
	n.ClampCursor(sections)
}

// cursorSection returns the index of the section the cursor is in, or -1.
func (n *Navigator) cursorSection(sections []filter.ViewResult) int {
	flatIdx := 0
	for i, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		if n.cursor >= flatIdx && n.cursor < flatIdx+len(sec.Tasks) {
			return i
		}
		flatIdx += len(sec.Tasks)
	}
	return -1
}

// JumpToNextSection moves the cursor to the first task of the next non-empty section.
func (n *Navigator) JumpToNextSection(sections []filter.ViewResult) {
	current := n.cursorSection(sections)
	flatIdx := 0
	for i, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		if i > current {
			n.cursor = flatIdx
			return
		}
		flatIdx += len(sec.Tasks)
	}
}

// JumpToPrevSection moves the cursor to the first task of the previous non-empty section.
func (n *Navigator) JumpToPrevSection(sections []filter.ViewResult) {
	current := n.cursorSection(sections)
	flatIdx := 0
	prevStart := -1
	for i, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		if i >= current {
			break
		}
		prevStart = flatIdx
		flatIdx += len(sec.Tasks)
	}
	if prevStart >= 0 {
		n.cursor = prevStart
	}
}

// FocusSection moves the cursor to the first task of the non-empty section at the given index.
func (n *Navigator) FocusSection(sections []filter.ViewResult, index int) {
	flatIdx := 0
	sectionIdx := 0
	for _, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		if sectionIdx == index {
			n.cursor = flatIdx
			return
		}
		flatIdx += len(sec.Tasks)
		sectionIdx++
	}
}

// flatTasks returns all tasks across displayed sections in order.
func flatTasks(sections []filter.ViewResult) []model.Task {
	var tasks []model.Task
	for _, sec := range sections {
		if len(sec.Tasks) > 0 {
			tasks = append(tasks, sec.Tasks...)
		}
	}
	return tasks
}

// countFlatTasks returns the total number of tasks across displayed sections.
func countFlatTasks(sections []filter.ViewResult) int {
	count := 0
	for _, sec := range sections {
		count += len(sec.Tasks)
	}
	return count
}
