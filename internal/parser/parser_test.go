package parser

import (
	"tasks/internal/model"
	"testing"
	"time"
)

func date(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func TestParseLine(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		wantNil       bool
		wantText      string
		wantState     model.TaskState
		wantTags      []model.Tag
		wantDue       *time.Time
		wantCompleted *time.Time
	}{
		{
			name:      "open task",
			line:      "- [ ] Buy groceries",
			wantText:  "Buy groceries",
			wantState: model.Open,
		},
		{
			name:      "completed task lowercase x",
			line:      "- [x] Ship feature",
			wantText:  "Ship feature",
			wantState: model.Completed,
		},
		{
			name:      "completed task uppercase X",
			line:      "- [X] Ship feature",
			wantText:  "Ship feature",
			wantState: model.Completed,
		},
		{
			name:      "task with bare tags",
			line:      "- [ ] Fix auth bug @today @risk",
			wantText:  "Fix auth bug @today @risk",
			wantState: model.Open,
			wantTags: []model.Tag{
				{Name: "today", Value: ""},
				{Name: "risk", Value: ""},
			},
		},
		{
			name:      "task with due date tag",
			line:      "- [ ] Write report @due(2026-03-15)",
			wantText:  "Write report @due(2026-03-15)",
			wantState: model.Open,
			wantTags: []model.Tag{
				{Name: "due", Value: "2026-03-15"},
			},
			wantDue: date(2026, time.March, 15),
		},
		{
			name:      "task with completed date tag",
			line:      "- [x] Done task @completed(2026-03-10)",
			wantText:  "Done task @completed(2026-03-10)",
			wantState: model.Completed,
			wantTags: []model.Tag{
				{Name: "completed", Value: "2026-03-10"},
			},
			wantCompleted: date(2026, time.March, 10),
		},
		{
			name:      "task with both bare and date tags",
			line:      "- [ ] Prepare deck @talk @due(2026-03-14)",
			wantText:  "Prepare deck @talk @due(2026-03-14)",
			wantState: model.Open,
			wantTags: []model.Tag{
				{Name: "talk", Value: ""},
				{Name: "due", Value: "2026-03-14"},
			},
			wantDue: date(2026, time.March, 14),
		},
		{
			name:      "indented task",
			line:      "  - [ ] Nested item",
			wantText:  "Nested item",
			wantState: model.Open,
		},
		{
			name:    "non-task plain text",
			line:    "This is just a paragraph.",
			wantNil: true,
		},
		{
			name:    "non-task header",
			line:    "## Section Header",
			wantNil: true,
		},
		{
			name:    "bullet without checkbox and no tag",
			line:    "- Just a bullet point",
			wantNil: true,
		},
		{
			name:      "plain bullet with tag treated as open task",
			line:      "- Reserve moving to core platform @horizon",
			wantText:  "Reserve moving to core platform @horizon",
			wantState: model.Open,
			wantTags: []model.Tag{
				{Name: "horizon", Value: ""},
			},
		},
		{
			name:      "plain bullet with multiple tags",
			line:      "- ISK missing security controls @risk @horizon",
			wantText:  "ISK missing security controls @risk @horizon",
			wantState: model.Open,
			wantTags: []model.Tag{
				{Name: "risk", Value: ""},
				{Name: "horizon", Value: ""},
			},
		},
		{
			name:      "indented plain bullet with tag",
			line:      "  - Saransh leave ends 4/16 @horizon",
			wantText:  "Saransh leave ends 4/16 @horizon",
			wantState: model.Open,
			wantTags: []model.Tag{
				{Name: "horizon", Value: ""},
			},
		},
		{
			name:      "invalid date format in due tag",
			line:      "- [ ] Bad date @due(not-a-date)",
			wantText:  "Bad date @due(not-a-date)",
			wantState: model.Open,
			wantTags: []model.Tag{
				{Name: "due", Value: ""},
			},
			wantDue: nil,
		},
		{
			name:      "multiple tags of various kinds",
			line:      "- [ ] Big task @today @risk @due(2026-04-01) @horizon",
			wantText:  "Big task @today @risk @due(2026-04-01) @horizon",
			wantState: model.Open,
			wantTags: []model.Tag{
				{Name: "today", Value: ""},
				{Name: "risk", Value: ""},
				{Name: "due", Value: "2026-04-01"},
				{Name: "horizon", Value: ""},
			},
			wantDue: date(2026, time.April, 1),
		},
		{
			name:      "empty task text after checkbox",
			line:      "- [ ] ",
			wantText:  "",
			wantState: model.Open,
		},
		{
			name:      "task text with special characters",
			line:      "- [ ] Fix bug #123 -- use `foo()` & bar <baz>",
			wantText:  "Fix bug #123 -- use `foo()` & bar <baz>",
			wantState: model.Open,
		},
		{
			name:      "task with due and completed tags",
			line:      "- [x] Finished @due(2026-03-01) @completed(2026-03-02)",
			wantText:  "Finished @due(2026-03-01) @completed(2026-03-02)",
			wantState: model.Completed,
			wantTags: []model.Tag{
				{Name: "due", Value: "2026-03-01"},
				{Name: "completed", Value: "2026-03-02"},
			},
			wantDue:       date(2026, time.March, 1),
			wantCompleted: date(2026, time.March, 2),
		},
		{
			name:    "empty line",
			line:    "",
			wantNil: true,
		},
		{
			name:      "deeply indented task",
			line:      "      - [x] Very nested @weekly",
			wantText:  "Very nested @weekly",
			wantState: model.Completed,
			wantTags: []model.Tag{
				{Name: "weekly", Value: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := ParseLine(tt.line, "test.md", 1)

			if tt.wantNil {
				if task != nil {
					t.Fatalf("expected nil, got %+v", task)
				}
				return
			}

			if task == nil {
				t.Fatal("expected non-nil task, got nil")
			}

			if task.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", task.Text, tt.wantText)
			}

			if task.State != tt.wantState {
				t.Errorf("State = %v, want %v", task.State, tt.wantState)
			}

			if task.File != "test.md" {
				t.Errorf("File = %q, want %q", task.File, "test.md")
			}

			if task.Line != 1 {
				t.Errorf("Line = %d, want %d", task.Line, 1)
			}

			// Check tags
			if tt.wantTags != nil {
				if len(task.Tags) != len(tt.wantTags) {
					t.Fatalf("Tags count = %d, want %d; got %+v", len(task.Tags), len(tt.wantTags), task.Tags)
				}
				for i, wantTag := range tt.wantTags {
					if task.Tags[i].Name != wantTag.Name {
						t.Errorf("Tags[%d].Name = %q, want %q", i, task.Tags[i].Name, wantTag.Name)
					}
					if task.Tags[i].Value != wantTag.Value {
						t.Errorf("Tags[%d].Value = %q, want %q", i, task.Tags[i].Value, wantTag.Value)
					}
				}
			}

			// Check Due
			if tt.wantDue == nil {
				if task.Due != nil {
					t.Errorf("Due = %v, want nil", task.Due)
				}
			} else {
				if task.Due == nil {
					t.Errorf("Due = nil, want %v", tt.wantDue)
				} else if !task.Due.Equal(*tt.wantDue) {
					t.Errorf("Due = %v, want %v", task.Due, tt.wantDue)
				}
			}

			// Check Completed
			if tt.wantCompleted == nil {
				if task.Completed != nil {
					t.Errorf("Completed = %v, want nil", task.Completed)
				}
			} else {
				if task.Completed == nil {
					t.Errorf("Completed = nil, want %v", tt.wantCompleted)
				} else if !task.Completed.Equal(*tt.wantCompleted) {
					t.Errorf("Completed = %v, want %v", task.Completed, tt.wantCompleted)
				}
			}
		})
	}
}
