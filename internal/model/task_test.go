package model

import "testing"

func TestTaskStateValues(t *testing.T) {
	if Open == Completed {
		t.Fatal("Open and Completed should be different values")
	}
}

func TestTaskStateString(t *testing.T) {
	tests := []struct {
		state TaskState
		want  string
	}{
		{Open, "open"},
		{Completed, "completed"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("TaskState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestNewTask(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		file        string
		line        int
		state       TaskState
		hasCheckbox bool
		wantLower   string
	}{
		{"open checkbox", "Buy groceries @today", "notes.md", 5, Open, true, "buy groceries @today"},
		{"completed checkbox", "Ship feature", "work.md", 12, Completed, true, "ship feature"},
		{"plain bullet", "Review PR @risk", "dev.md", 1, Open, false, "review pr @risk"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := NewTask(tt.text, tt.file, tt.line, tt.state, tt.hasCheckbox)
			if task.Text != tt.text {
				t.Errorf("Text = %q, want %q", task.Text, tt.text)
			}
			if task.LowerText != tt.wantLower {
				t.Errorf("LowerText = %q, want %q", task.LowerText, tt.wantLower)
			}
			if task.File != tt.file {
				t.Errorf("File = %q, want %q", task.File, tt.file)
			}
			if task.Line != tt.line {
				t.Errorf("Line = %d, want %d", task.Line, tt.line)
			}
			if task.State != tt.state {
				t.Errorf("State = %v, want %v", task.State, tt.state)
			}
			if task.HasCheckbox != tt.hasCheckbox {
				t.Errorf("HasCheckbox = %v, want %v", task.HasCheckbox, tt.hasCheckbox)
			}
			if task.tagSet == nil {
				t.Error("tagSet should be initialized, got nil")
			}
		})
	}
}

func TestAddTag(t *testing.T) {
	tests := []struct {
		name    string
		tags    []Tag
		hasTag  string
		wantHas bool
	}{
		{"present tag", []Tag{{Name: "today"}, {Name: "due", Value: "2026-03-15"}}, "today", true},
		{"present valued tag", []Tag{{Name: "today"}, {Name: "due", Value: "2026-03-15"}}, "due", true},
		{"absent tag", []Tag{{Name: "today"}, {Name: "due", Value: "2026-03-15"}}, "risk", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := NewTask("test", "f.md", 1, Open, true)
			for _, tag := range tt.tags {
				task.AddTag(tag)
			}
			if len(task.Tags) != len(tt.tags) {
				t.Fatalf("Tags len = %d, want %d", len(task.Tags), len(tt.tags))
			}
			if got := task.HasTag(tt.hasTag); got != tt.wantHas {
				t.Errorf("HasTag(%q) = %v, want %v", tt.hasTag, got, tt.wantHas)
			}
		})
	}
}

func TestHasTag(t *testing.T) {
	task := NewTask("task @today @due(2026-03-15)", "f.md", 1, Open, true)
	task.AddTag(Tag{Name: "today"})
	task.AddTag(Tag{Name: "due", Value: "2026-03-15"})

	if !task.HasTag("today") {
		t.Error("expected HasTag('today') to be true")
	}
	if !task.HasTag("due") {
		t.Error("expected HasTag('due') to be true")
	}
	if task.HasTag("risk") {
		t.Error("expected HasTag('risk') to be false")
	}
}
