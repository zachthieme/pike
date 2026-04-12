// Package model defines the core data types for tasks, tags, and warnings.
package model

import (
	"strings"
	"time"
)

// TaskState represents whether a task is open or completed.
type TaskState int

const (
	Open TaskState = iota
	Completed
)

func (s TaskState) String() string {
	switch s {
	case Open:
		return "open"
	case Completed:
		return "completed"
	default:
		return "unknown"
	}
}

// Tag is a parsed @name or @name(value) token attached to a task.
type Tag struct {
	Name  string // e.g., "today", "due", "risk"
	Value string // e.g., "2026-03-15" for @due(2026-03-15), empty for bare tags
}

// Task is a single item parsed from a markdown checkbox or tagged bullet line.
// Use [NewTask] or [TaskWith] to construct and [AddTag] to append tags so that
// derived fields (LowerText, tagSet) stay consistent.
type Task struct {
	Text        string          // Full line text after "- [ ] " / "- [x] " or "- "
	LowerText   string          // Pre-lowered Text for efficient case-insensitive matching
	State       TaskState       // Open or Completed
	File        string          // Relative path from notes_dir
	Line        int             // 1-based line number
	Tags        []Tag           // Parsed @tag tokens
	tagSet      map[string]bool // O(1) tag lookup by name; use NewTask+AddTag to maintain
	Due         *time.Time      // Parsed from @due(YYYY-MM-DD), nil if absent
	Completed   *time.Time      // Parsed from @completed(YYYY-MM-DD), nil if absent
	HasCheckbox bool            // true if line had - [ ] or - [x], false for plain bullets
	Indent      int             // column count of leading whitespace (0 for top-level)
	Children    []*Task         // direct subtasks (single level only)
	ParentIndex int             // index of parent in flat task list, -1 if none
}

// NewTask creates a Task with pre-computed LowerText and an initialized tagSet.
// Use this instead of struct literals to ensure invariants are maintained.
func NewTask(text, file string, line int, state TaskState, hasCheckbox bool) *Task {
	return &Task{
		Text:        text,
		LowerText:   strings.ToLower(text),
		State:       state,
		File:        file,
		Line:        line,
		HasCheckbox: hasCheckbox,
		tagSet:      make(map[string]bool),
		ParentIndex: -1,
	}
}

// TaskWith constructs a Task from the given partial struct literal, ensuring
// the tagSet is properly initialized from Tags. This allows struct-literal style
// construction while maintaining invariants. Fields that are set on the input
// (Text, File, Line, State, HasCheckbox, Tags, Due, Completed) are copied;
// LowerText and tagSet are derived automatically.
func TaskWith(partial Task) Task {
	t := NewTask(partial.Text, partial.File, partial.Line, partial.State, partial.HasCheckbox)
	for _, tag := range partial.Tags {
		t.AddTag(tag)
	}
	t.Due = partial.Due
	t.Completed = partial.Completed
	t.Indent = partial.Indent
	t.ParentIndex = partial.ParentIndex
	return *t
}

// AddTag appends a tag and updates the tagSet for O(1) lookup.
func (t *Task) AddTag(tag Tag) {
	t.Tags = append(t.Tags, tag)
	t.tagSet[tag.Name] = true
}

// SetText sets Text and pre-computes LowerText.
func (t *Task) SetText(text string) {
	t.Text = text
	t.LowerText = strings.ToLower(text)
}

// HasTag returns true if the task has a tag with the given name (O(1) lookup).
func (t *Task) HasTag(name string) bool {
	return t.tagSet[name]
}

// Progress returns the count of completed vs. total children.
// Returns (0, 0) for leaf tasks (no children).
func (t *Task) Progress() (done, total int) {
	total = len(t.Children)
	for _, c := range t.Children {
		if c.State == Completed {
			done++
		}
	}
	return done, total
}

// Warning represents a non-fatal issue found during parsing.
type Warning struct {
	File    string
	Line    int
	Message string
}
