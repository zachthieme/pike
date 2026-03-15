package model

import "time"

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
	State       TaskState       // Open or Completed
	File        string          // Relative path from notes_dir
	Line        int             // 1-based line number
	Tags        []Tag           // Parsed @tag tokens
	TagSet      map[string]bool // O(1) tag lookup by name, populated at parse time
	Due         *time.Time      // Parsed from @due(YYYY-MM-DD), nil if absent
	Completed   *time.Time      // Parsed from @completed(YYYY-MM-DD), nil if absent
	HasCheckbox bool            // true if line had - [ ] or - [x], false for plain bullets
}

// HasTag returns true if the task has a tag with the given name (O(1) lookup).
func (t *Task) HasTag(name string) bool {
	return t.TagSet[name]
}
