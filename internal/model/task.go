// Package model defines the core data types for tasks, tags, and warnings.
package model

import (
	"strings"
	"time"
)

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

type Tag struct {
	Name  string // e.g., "today", "due", "risk"
	Value string // e.g., "2026-03-15" for @due(2026-03-15), empty for bare tags
}

type Task struct {
	Text        string          // Full line text after "- [ ] " / "- [x] " or "- "
	LowerText   string          // Pre-lowered Text for efficient case-insensitive matching
	State       TaskState       // Open or Completed
	File        string          // Relative path from notes_dir
	Line        int             // 1-based line number
	Tags        []Tag           // Parsed @tag tokens
	TagSet      map[string]bool // O(1) tag lookup by name; use NewTask+AddTag to maintain
	Due         *time.Time      // Parsed from @due(YYYY-MM-DD), nil if absent
	Completed   *time.Time      // Parsed from @completed(YYYY-MM-DD), nil if absent
	HasCheckbox bool            // true if line had - [ ] or - [x], false for plain bullets
}

// NewTask creates a Task with pre-computed LowerText and an initialized TagSet.
// Use this instead of struct literals to ensure invariants are maintained.
func NewTask(text, file string, line int, state TaskState, hasCheckbox bool) *Task {
	return &Task{
		Text:        text,
		LowerText:   strings.ToLower(text),
		State:       state,
		File:        file,
		Line:        line,
		HasCheckbox: hasCheckbox,
		TagSet:      make(map[string]bool),
	}
}

// AddTag appends a tag and updates the TagSet for O(1) lookup.
func (t *Task) AddTag(tag Tag) {
	t.Tags = append(t.Tags, tag)
	t.TagSet[tag.Name] = true
}

// SetText sets Text and pre-computes LowerText.
func (t *Task) SetText(text string) {
	t.Text = text
	t.LowerText = strings.ToLower(text)
}

// HasTag returns true if the task has a tag with the given name (O(1) lookup).
func (t *Task) HasTag(name string) bool {
	return t.TagSet[name]
}

// Warning represents a non-fatal issue found during parsing.
type Warning struct {
	File    string
	Line    int
	Message string
}
