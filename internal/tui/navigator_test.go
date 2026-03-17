package tui

import (
	"testing"

	"pike/internal/filter"
	"pike/internal/model"
)

func makeSections(counts ...int) []filter.ViewResult {
	var sections []filter.ViewResult
	for i, n := range counts {
		var tasks []model.Task
		for j := 0; j < n; j++ {
			tasks = append(tasks, model.Task{Text: "task"})
		}
		sections = append(sections, filter.ViewResult{
			Title: string(rune('A' + i)),
			Tasks: tasks,
		})
	}
	return sections
}

func TestNavigator_CursorDownUp(t *testing.T) {
	sections := makeSections(3, 2)
	var nav Navigator

	nav.CursorDown(sections)
	if nav.Cursor() != 1 {
		t.Errorf("after down: expected 1, got %d", nav.Cursor())
	}

	nav.CursorUp()
	if nav.Cursor() != 0 {
		t.Errorf("after up: expected 0, got %d", nav.Cursor())
	}

	nav.CursorUp()
	if nav.Cursor() != 0 {
		t.Errorf("after up at 0: expected 0, got %d", nav.Cursor())
	}
}

func TestNavigator_CursorBounds(t *testing.T) {
	sections := makeSections(2)
	var nav Navigator

	nav.CursorDown(sections)
	nav.CursorDown(sections)
	if nav.Cursor() != 1 {
		t.Errorf("expected cursor clamped at 1, got %d", nav.Cursor())
	}
}

func TestNavigator_JumpToNextSection(t *testing.T) {
	sections := makeSections(2, 3, 1)
	var nav Navigator

	nav.JumpToNextSection(sections)
	if nav.Cursor() != 2 {
		t.Errorf("expected cursor 2 (first of B), got %d", nav.Cursor())
	}

	nav.JumpToNextSection(sections)
	if nav.Cursor() != 5 {
		t.Errorf("expected cursor 5 (first of C), got %d", nav.Cursor())
	}

	nav.JumpToNextSection(sections)
	if nav.Cursor() != 5 {
		t.Errorf("expected cursor still 5, got %d", nav.Cursor())
	}
}

func TestNavigator_JumpToPrevSection(t *testing.T) {
	sections := makeSections(2, 3, 1)
	var nav Navigator
	nav.SetCursor(5)

	nav.JumpToPrevSection(sections)
	if nav.Cursor() != 2 {
		t.Errorf("expected cursor 2 (first of B), got %d", nav.Cursor())
	}

	nav.JumpToPrevSection(sections)
	if nav.Cursor() != 0 {
		t.Errorf("expected cursor 0 (first of A), got %d", nav.Cursor())
	}
}

func TestNavigator_JumpToTopBottom(t *testing.T) {
	sections := makeSections(3, 2)
	var nav Navigator

	nav.JumpToBottom(sections)
	if nav.Cursor() != 4 {
		t.Errorf("expected cursor 4, got %d", nav.Cursor())
	}

	nav.JumpToTop()
	if nav.Cursor() != 0 {
		t.Errorf("expected cursor 0, got %d", nav.Cursor())
	}
}

func TestNavigator_FocusSection(t *testing.T) {
	sections := makeSections(2, 3, 1)
	var nav Navigator

	nav.FocusSection(sections, 1)
	if nav.Cursor() != 2 {
		t.Errorf("expected cursor 2 (first of B), got %d", nav.Cursor())
	}

	nav.FocusSection(sections, 2)
	if nav.Cursor() != 5 {
		t.Errorf("expected cursor 5 (first of C), got %d", nav.Cursor())
	}
}

func TestNavigator_EmptySections(t *testing.T) {
	sections := makeSections(0, 0)
	var nav Navigator

	nav.CursorDown(sections)
	if nav.Cursor() != 0 {
		t.Errorf("expected 0 on empty, got %d", nav.Cursor())
	}
	if countFlatTasks(sections) != 0 {
		t.Errorf("expected 0 flat tasks, got %d", countFlatTasks(sections))
	}
}

func TestNavigator_PageScroll(t *testing.T) {
	sections := makeSections(30)
	nav := Navigator{height: 40}

	nav.PageScroll(1, sections)
	if nav.Cursor() == 0 {
		t.Error("expected cursor to move down on page scroll")
	}
	pos := nav.Cursor()
	nav.PageScroll(-1, sections)
	if nav.Cursor() >= pos {
		t.Error("expected cursor to move up on page scroll")
	}
}

func TestNavigator_ClampCursor(t *testing.T) {
	sections := makeSections(3)
	var nav Navigator
	nav.SetCursor(100)

	nav.ClampCursor(sections)
	if nav.Cursor() != 2 {
		t.Errorf("expected cursor clamped to 2, got %d", nav.Cursor())
	}
}

func TestFlatTasks(t *testing.T) {
	sections := makeSections(2, 1)
	tasks := flatTasks(sections)
	if len(tasks) != 3 {
		t.Errorf("expected 3 flat tasks, got %d", len(tasks))
	}
}

func TestCountFlatTasks(t *testing.T) {
	sections := makeSections(2, 3)
	if countFlatTasks(sections) != 5 {
		t.Errorf("expected 5, got %d", countFlatTasks(sections))
	}
}
