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

func TestHasTag(t *testing.T) {
	task := &Task{
		Tags: []Tag{
			{Name: "today"},
			{Name: "due", Value: "2026-03-15"},
		},
		TagSet: map[string]bool{"today": true, "due": true},
	}
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
