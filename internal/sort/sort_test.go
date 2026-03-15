package sort

import (
	"pike/internal/model"
	"testing"
	"time"
)

func timePtr(t time.Time) *time.Time { return &t }
func date(y, m, d int) *time.Time {
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	return &t
}

func TestSort(t *testing.T) {
	tests := []struct {
		name      string
		order     string
		tasks     []model.Task
		wantTexts []string
		wantErr   bool
	}{
		{
			name:  "due_asc sorts by due date ascending, nil last",
			order: "due_asc",
			tasks: []model.Task{
				{Text: "no due", Due: nil},
				{Text: "mar 15", Due: date(2026, 3, 15)},
				{Text: "mar 10", Due: date(2026, 3, 10)},
				{Text: "mar 20", Due: date(2026, 3, 20)},
				{Text: "also no due", Due: nil},
			},
			wantTexts: []string{"mar 10", "mar 15", "mar 20", "no due", "also no due"},
		},
		{
			name:  "due_desc sorts by due date descending, nil last",
			order: "due_desc",
			tasks: []model.Task{
				{Text: "no due", Due: nil},
				{Text: "mar 10", Due: date(2026, 3, 10)},
				{Text: "mar 20", Due: date(2026, 3, 20)},
				{Text: "mar 15", Due: date(2026, 3, 15)},
				{Text: "also no due", Due: nil},
			},
			wantTexts: []string{"mar 20", "mar 15", "mar 10", "no due", "also no due"},
		},
		{
			name:  "completed_asc sorts by completed date ascending, nil last",
			order: "completed_asc",
			tasks: []model.Task{
				{Text: "no comp", Completed: nil},
				{Text: "mar 12", Completed: date(2026, 3, 12)},
				{Text: "mar 5", Completed: date(2026, 3, 5)},
				{Text: "mar 18", Completed: date(2026, 3, 18)},
				{Text: "also no comp", Completed: nil},
			},
			wantTexts: []string{"mar 5", "mar 12", "mar 18", "no comp", "also no comp"},
		},
		{
			name:  "completed_desc sorts by completed date descending, nil last",
			order: "completed_desc",
			tasks: []model.Task{
				{Text: "no comp", Completed: nil},
				{Text: "mar 5", Completed: date(2026, 3, 5)},
				{Text: "mar 18", Completed: date(2026, 3, 18)},
				{Text: "mar 12", Completed: date(2026, 3, 12)},
				{Text: "also no comp", Completed: nil},
			},
			wantTexts: []string{"mar 18", "mar 12", "mar 5", "no comp", "also no comp"},
		},
		{
			name:  "file sorts by file path then line number",
			order: "file",
			tasks: []model.Task{
				{Text: "b line 10", File: "notes/b.md", Line: 10},
				{Text: "a line 5", File: "notes/a.md", Line: 5},
				{Text: "b line 3", File: "notes/b.md", Line: 3},
				{Text: "a line 1", File: "notes/a.md", Line: 1},
			},
			wantTexts: []string{"a line 1", "a line 5", "b line 3", "b line 10"},
		},
		{
			name:  "alpha sorts alphabetically by text",
			order: "alpha",
			tasks: []model.Task{
				{Text: "Zebra task"},
				{Text: "Apple task"},
				{Text: "Mango task"},
			},
			wantTexts: []string{"Apple task", "Mango task", "Zebra task"},
		},
		{
			name:  "nil due dates go last in due_asc",
			order: "due_asc",
			tasks: []model.Task{
				{Text: "nil1", Due: nil},
				{Text: "nil2", Due: nil},
				{Text: "has due", Due: date(2026, 1, 1)},
			},
			wantTexts: []string{"has due", "nil1", "nil2"},
		},
		{
			name:  "nil due dates go last in due_desc",
			order: "due_desc",
			tasks: []model.Task{
				{Text: "nil1", Due: nil},
				{Text: "has due", Due: date(2026, 1, 1)},
				{Text: "nil2", Due: nil},
			},
			wantTexts: []string{"has due", "nil1", "nil2"},
		},
		{
			name:  "nil completed dates go last in completed_asc",
			order: "completed_asc",
			tasks: []model.Task{
				{Text: "nil1", Completed: nil},
				{Text: "has comp", Completed: date(2026, 1, 1)},
				{Text: "nil2", Completed: nil},
			},
			wantTexts: []string{"has comp", "nil1", "nil2"},
		},
		{
			name:  "nil completed dates go last in completed_desc",
			order: "completed_desc",
			tasks: []model.Task{
				{Text: "nil1", Completed: nil},
				{Text: "has comp", Completed: date(2026, 1, 1)},
				{Text: "nil2", Completed: nil},
			},
			wantTexts: []string{"has comp", "nil1", "nil2"},
		},
		{
			name:  "stability: equal sort keys maintain original order (due_asc)",
			order: "due_asc",
			tasks: []model.Task{
				{Text: "first", Due: date(2026, 3, 15)},
				{Text: "second", Due: date(2026, 3, 15)},
				{Text: "third", Due: date(2026, 3, 15)},
			},
			wantTexts: []string{"first", "second", "third"},
		},
		{
			name:  "stability: equal sort keys maintain original order (file)",
			order: "file",
			tasks: []model.Task{
				{Text: "first", File: "a.md", Line: 1},
				{Text: "second", File: "a.md", Line: 1},
				{Text: "third", File: "a.md", Line: 1},
			},
			wantTexts: []string{"first", "second", "third"},
		},
		{
			name:  "stability: equal sort keys maintain original order (alpha)",
			order: "alpha",
			tasks: []model.Task{
				{Text: "same", File: "a.md", Line: 1},
				{Text: "same", File: "b.md", Line: 2},
				{Text: "same", File: "c.md", Line: 3},
			},
			wantTexts: []string{"same", "same", "same"},
		},
		{
			name:      "empty slice does not panic",
			order:     "due_asc",
			tasks:     []model.Task{},
			wantTexts: []string{},
		},
		{
			name:  "single element does not panic",
			order: "due_asc",
			tasks: []model.Task{
				{Text: "only one"},
			},
			wantTexts: []string{"only one"},
		},
		{
			name:    "unknown sort order returns error",
			order:   "bogus",
			tasks:   []model.Task{{Text: "a"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy so we don't mutate the test table
			tasks := make([]model.Task, len(tt.tasks))
			copy(tasks, tt.tasks)

			err := Sort(tasks, tt.order)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(tasks) != len(tt.wantTexts) {
				t.Fatalf("got %d tasks, want %d", len(tasks), len(tt.wantTexts))
			}

			for i, want := range tt.wantTexts {
				if tasks[i].Text != want {
					t.Errorf("tasks[%d].Text = %q, want %q", i, tasks[i].Text, want)
				}
			}

			// For the stability test with identical text, verify by File field
			if tt.name == "stability: equal sort keys maintain original order (alpha)" {
				expectedFiles := []string{"a.md", "b.md", "c.md"}
				for i, want := range expectedFiles {
					if tasks[i].File != want {
						t.Errorf("tasks[%d].File = %q, want %q (stability check)", i, tasks[i].File, want)
					}
				}
			}
		})
	}
}

func TestStablePartitionPinned(t *testing.T) {
	tasks := []model.Task{
		{Text: "unpinned-a", TagSet: map[string]bool{}},
		{Text: "pinned-b", Tags: []model.Tag{{Name: "pin"}}, TagSet: map[string]bool{"pin": true}},
		{Text: "unpinned-c", TagSet: map[string]bool{}},
		{Text: "pinned-d", Tags: []model.Tag{{Name: "pin"}}, TagSet: map[string]bool{"pin": true}},
		{Text: "unpinned-e", TagSet: map[string]bool{}},
	}

	result := StablePartitionPinned(tasks)

	wantTexts := []string{"pinned-b", "pinned-d", "unpinned-a", "unpinned-c", "unpinned-e"}
	if len(result) != len(wantTexts) {
		t.Fatalf("got %d tasks, want %d", len(result), len(wantTexts))
	}
	for i, want := range wantTexts {
		if result[i].Text != want {
			t.Errorf("result[%d].Text = %q, want %q", i, result[i].Text, want)
		}
	}
}

func TestStablePartitionPinnedNoPins(t *testing.T) {
	tasks := []model.Task{
		{Text: "a", TagSet: map[string]bool{}},
		{Text: "b", TagSet: map[string]bool{}},
	}

	result := StablePartitionPinned(tasks)
	if result[0].Text != "a" || result[1].Text != "b" {
		t.Error("expected order unchanged when no pins")
	}
}

func TestStablePartitionPinnedEmpty(t *testing.T) {
	result := StablePartitionPinned(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}
