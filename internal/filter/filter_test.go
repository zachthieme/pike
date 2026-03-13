package filter

import (
	"tasks/internal/config"
	"tasks/internal/model"
	"testing"
	"time"
)

func timePtr(t time.Time) *time.Time {
	return &t
}

func makeTasks() []model.Task {
	return []model.Task{
		{
			Text:  "Buy groceries @due(2026-03-10)",
			State: model.Open,
			File:  "notes/todo.md",
			Line:  1,
			Tags:  []model.Tag{{Name: "due", Value: "2026-03-10"}},
			Due:   timePtr(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)),
		},
		{
			Text:      "Write report @due(2026-03-20) @completed(2026-03-11)",
			State:     model.Completed,
			File:      "notes/todo.md",
			Line:      2,
			Tags:      []model.Tag{{Name: "due", Value: "2026-03-20"}, {Name: "completed", Value: "2026-03-11"}},
			Due:       timePtr(time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)),
			Completed: timePtr(time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)),
		},
		{
			Text:  "Call dentist @due(2026-03-15)",
			State: model.Open,
			File:  "notes/health.md",
			Line:  5,
			Tags:  []model.Tag{{Name: "due", Value: "2026-03-15"}},
			Due:   timePtr(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)),
		},
		{
			Text:  "Fix bug @due(2026-03-05)",
			State: model.Open,
			File:  "notes/dev.md",
			Line:  3,
			Tags:  []model.Tag{{Name: "due", Value: "2026-03-05"}},
			Due:   timePtr(time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)),
		},
		{
			Text:  "Read book",
			State: model.Open,
			File:  "notes/personal.md",
			Line:  1,
			Tags:  nil,
			Due:   nil,
		},
	}
}

func TestApplyQueryOpen(t *testing.T) {
	tasks := makeTasks()
	now := time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

	result, err := Apply(tasks, "open", "", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 4 {
		t.Fatalf("expected 4 open tasks, got %d", len(result))
	}

	for _, task := range result {
		if task.State != model.Open {
			t.Errorf("expected open task, got state %v for %q", task.State, task.Text)
		}
	}
}

func TestApplyQueryCompleted(t *testing.T) {
	tasks := makeTasks()
	now := time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

	result, err := Apply(tasks, "completed", "", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 completed task, got %d", len(result))
	}

	if result[0].State != model.Completed {
		t.Errorf("expected completed task, got state %v", result[0].State)
	}
	if result[0].Text != "Write report @due(2026-03-20) @completed(2026-03-11)" {
		t.Errorf("unexpected task text: %q", result[0].Text)
	}
}

func TestApplyQueryOverdue(t *testing.T) {
	tasks := makeTasks()
	now := time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

	result, err := Apply(tasks, "open and @due < today", "", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Tasks with due dates before 2026-03-13: "Buy groceries" (Mar 10), "Fix bug" (Mar 5)
	if len(result) != 2 {
		t.Fatalf("expected 2 overdue open tasks, got %d", len(result))
	}

	for _, task := range result {
		if task.State != model.Open {
			t.Errorf("expected open task, got state %v for %q", task.State, task.Text)
		}
		if task.Due == nil || !task.Due.Before(now) {
			t.Errorf("expected task with due date before %v, got %v for %q", now, task.Due, task.Text)
		}
	}
}

func TestApplySortDueAsc(t *testing.T) {
	tasks := makeTasks()
	now := time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

	result, err := Apply(tasks, "open", "due_asc", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 4 {
		t.Fatalf("expected 4 open tasks, got %d", len(result))
	}

	// Tasks with due dates should come first sorted ascending, nil due last
	// Fix bug (Mar 5), Buy groceries (Mar 10), Call dentist (Mar 15), Read book (nil)
	expected := []string{
		"Fix bug @due(2026-03-05)",
		"Buy groceries @due(2026-03-10)",
		"Call dentist @due(2026-03-15)",
		"Read book",
	}
	for i, task := range result {
		if task.Text != expected[i] {
			t.Errorf("position %d: expected %q, got %q", i, expected[i], task.Text)
		}
	}
}

func TestApplySortFile(t *testing.T) {
	tasks := makeTasks()
	now := time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

	result, err := Apply(tasks, "open", "file", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 4 {
		t.Fatalf("expected 4 open tasks, got %d", len(result))
	}

	// Sorted by file path then line:
	// notes/dev.md:3, notes/health.md:5, notes/personal.md:1, notes/todo.md:1
	expected := []struct {
		file string
		line int
	}{
		{"notes/dev.md", 3},
		{"notes/health.md", 5},
		{"notes/personal.md", 1},
		{"notes/todo.md", 1},
	}
	for i, task := range result {
		if task.File != expected[i].file || task.Line != expected[i].line {
			t.Errorf("position %d: expected %s:%d, got %s:%d",
				i, expected[i].file, expected[i].line, task.File, task.Line)
		}
	}
}

func TestApplyViewsMultipleViews(t *testing.T) {
	tasks := makeTasks()
	now := time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

	views := []config.ViewConfig{
		{Title: "Open", Query: "open", Sort: "file", Color: "green"},
		{Title: "Completed", Query: "completed", Sort: "file", Color: "blue"},
	}

	results, err := ApplyViews(tasks, views, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 view results, got %d", len(results))
	}

	// First view: open tasks
	if results[0].Title != "Open" {
		t.Errorf("expected title 'Open', got %q", results[0].Title)
	}
	if results[0].Color != "green" {
		t.Errorf("expected color 'green', got %q", results[0].Color)
	}
	if len(results[0].Tasks) != 4 {
		t.Errorf("expected 4 open tasks, got %d", len(results[0].Tasks))
	}

	// Second view: completed tasks
	if results[1].Title != "Completed" {
		t.Errorf("expected title 'Completed', got %q", results[1].Title)
	}
	if results[1].Color != "blue" {
		t.Errorf("expected color 'blue', got %q", results[1].Color)
	}
	if len(results[1].Tasks) != 1 {
		t.Errorf("expected 1 completed task, got %d", len(results[1].Tasks))
	}
}

func TestApplyViewsEmptyResult(t *testing.T) {
	tasks := makeTasks()
	now := time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

	views := []config.ViewConfig{
		{Title: "Tagged Today", Query: "@today", Sort: "file", Color: "yellow"},
	}

	results, err := ApplyViews(tasks, views, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 view result, got %d", len(results))
	}

	if results[0].Title != "Tagged Today" {
		t.Errorf("expected title 'Tagged Today', got %q", results[0].Title)
	}
	if results[0].Tasks == nil {
		t.Errorf("expected non-nil empty Tasks slice, got nil")
	}
	if len(results[0].Tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(results[0].Tasks))
	}
}

func TestApplyInvalidQuery(t *testing.T) {
	tasks := makeTasks()
	now := time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

	_, err := Apply(tasks, "((( invalid", "", now)
	if err == nil {
		t.Fatal("expected error for invalid query, got nil")
	}
}

func TestApplyViewsInvalidQuery(t *testing.T) {
	tasks := makeTasks()
	now := time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

	views := []config.ViewConfig{
		{Title: "Good", Query: "open", Sort: "file", Color: "green"},
		{Title: "Bad", Query: "((( invalid", Sort: "file", Color: "red"},
	}

	_, err := ApplyViews(tasks, views, now)
	if err == nil {
		t.Fatal("expected error for invalid query in view, got nil")
	}
}
