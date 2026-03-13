package render

import (
	"fmt"
	"strings"
	"testing"

	"pike/internal/model"
)

const (
	red   = "\033[31m"
	green = "\033[32m"
	cyan  = "\033[36m"
	reset = "\033[0m"
)

func TestFormatTask(t *testing.T) {
	tagColors := map[string]string{
		"today":    "green",
		"risk":     "red",
		"due":      "red",
		"completed": "green",
		"_default": "cyan",
	}

	tests := []struct {
		name      string
		task      model.Task
		tagColors map[string]string
		noColor   bool
		want      string
	}{
		{
			name: "open task with file:line prefix",
			task: model.Task{
				Text:  "Buy groceries @today",
				State: model.Open,
				File:  "notes/todo.md",
				Line:  5,
				Tags:        []model.Tag{{Name: "today"}},
			HasCheckbox: true,
			},
			tagColors: tagColors,
			noColor:   true,
			want:      "notes/todo.md:5  - [ ] Buy groceries @today",
		},
		{
			name: "colorized @today tag in green",
			task: model.Task{
				Text:  "Buy groceries @today",
				State: model.Open,
				File:  "notes/todo.md",
				Line:  5,
				Tags:        []model.Tag{{Name: "today"}},
			HasCheckbox: true,
			},
			tagColors: tagColors,
			noColor:   false,
			want:      fmt.Sprintf("notes/todo.md:5  - [ ] Buy groceries %s@today%s", green, reset),
		},
		{
			name: "colorized @risk tag in red",
			task: model.Task{
				Text:  "Fix critical bug @risk",
				State: model.Open,
				File:  "work/bugs.md",
				Line:  12,
				Tags:        []model.Tag{{Name: "risk"}},
			HasCheckbox: true,
			},
			tagColors: tagColors,
			noColor:   false,
			want:      fmt.Sprintf("work/bugs.md:12  - [ ] Fix critical bug %s@risk%s", red, reset),
		},
		{
			name: "colorized @due(...) tag in red",
			task: model.Task{
				Text:  "Submit report @due(2026-03-15)",
				State: model.Open,
				File:  "work/tasks.md",
				Line:  3,
				Tags:        []model.Tag{{Name: "due", Value: "2026-03-15"}},
			HasCheckbox: true,
			},
			tagColors: tagColors,
			noColor:   false,
			want:      fmt.Sprintf("work/tasks.md:3  - [ ] Submit report %s@due(2026-03-15)%s", red, reset),
		},
		{
			name: "no-color mode skips ANSI codes",
			task: model.Task{
				Text:  "Submit report @due(2026-03-15) @risk",
				State: model.Open,
				File:  "work/tasks.md",
				Line:  3,
				Tags:        []model.Tag{{Name: "due", Value: "2026-03-15"}, {Name: "risk"}},
			HasCheckbox: true,
			},
			tagColors: tagColors,
			noColor:   true,
			want:      "work/tasks.md:3  - [ ] Submit report @due(2026-03-15) @risk",
		},
		{
			name: "completed task shows [x]",
			task: model.Task{
				Text:  "Write tests @completed(2026-03-10)",
				State: model.Completed,
				File:  "notes/done.md",
				Line:  8,
				Tags:        []model.Tag{{Name: "completed", Value: "2026-03-10"}},
			HasCheckbox: true,
			},
			tagColors: tagColors,
			noColor:   true,
			want:      "notes/done.md:8  - [x] Write tests @completed(2026-03-10)",
		},
		{
			name: "completed task with color",
			task: model.Task{
				Text:  "Write tests @completed(2026-03-10)",
				State: model.Completed,
				File:  "notes/done.md",
				Line:  8,
				Tags:        []model.Tag{{Name: "completed", Value: "2026-03-10"}},
			HasCheckbox: true,
			},
			tagColors: tagColors,
			noColor:   false,
			want:      fmt.Sprintf("notes/done.md:8  - [x] Write tests %s@completed(2026-03-10)%s", green, reset),
		},
		{
			name: "unknown tag uses _default color",
			task: model.Task{
				Text:  "Research topic @someothertag",
				State: model.Open,
				File:  "notes/ideas.md",
				Line:  1,
				Tags:        []model.Tag{{Name: "someothertag"}},
			HasCheckbox: true,
			},
			tagColors: tagColors,
			noColor:   false,
			want:      fmt.Sprintf("notes/ideas.md:1  - [ ] Research topic %s@someothertag%s", cyan, reset),
		},
		{
			name: "multiple tags colorized independently",
			task: model.Task{
				Text:  "Deploy service @risk @today",
				State: model.Open,
				File:  "ops/deploy.md",
				Line:  22,
				Tags:        []model.Tag{{Name: "risk"}, {Name: "today"}},
			HasCheckbox: true,
			},
			tagColors: tagColors,
			noColor:   false,
			want:      fmt.Sprintf("ops/deploy.md:22  - [ ] Deploy service %s@risk%s %s@today%s", red, reset, green, reset),
		},
		{
			name: "hex color tag",
			task: model.Task{
				Text:  "Custom color @special",
				State: model.Open,
				File:  "notes/todo.md",
				Line:  1,
				Tags:        []model.Tag{{Name: "special"}},
			HasCheckbox: true,
			},
			tagColors: map[string]string{
				"special": "#FF5733",
			},
			noColor: false,
			want:    fmt.Sprintf("notes/todo.md:1  - [ ] Custom color %s@special%s", "\033[38;2;255;87;51m", reset),
		},
		{
			name: "nil tagColors uses no coloring",
			task: model.Task{
				Text:        "Plain task @today",
				State:       model.Open,
				File:        "notes/todo.md",
				Line:        1,
				Tags:        []model.Tag{{Name: "today"}},
				HasCheckbox: true,
			},
			tagColors: nil,
			noColor:   false,
			want:      "notes/todo.md:1  - [ ] Plain task @today",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTask(tt.task, tt.tagColors, tt.noColor)
			if got != tt.want {
				t.Errorf("FormatTask() =\n  %q\nwant:\n  %q", got, tt.want)
			}
		})
	}
}

func TestFormatSummary(t *testing.T) {
	tests := []struct {
		name           string
		open           int
		overdue        int
		dueThisWeek    int
		completedWeek  int
		noColor        bool
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:          "basic summary with all counts",
			open:          42,
			overdue:       3,
			dueThisWeek:   7,
			completedWeek: 12,
			noColor:       true,
			wantContains: []string{
				"Task Summary",
				"Open tasks",
				"42",
				"Overdue",
				"3",
				"Due this week",
				"7",
				"Completed this week",
				"12",
			},
			wantNotContain: []string{
				"\033[", // no ANSI codes in no-color mode
			},
		},
		{
			name:          "overdue > 0 highlights in red",
			open:          10,
			overdue:       5,
			dueThisWeek:   2,
			completedWeek: 1,
			noColor:       false,
			wantContains: []string{
				"Task Summary",
				red,   // overdue line should be red
				"5",
				reset, // reset after red
			},
		},
		{
			name:          "overdue = 0 has no red highlighting",
			open:          10,
			overdue:       0,
			dueThisWeek:   2,
			completedWeek: 1,
			noColor:       false,
			wantContains: []string{
				"Task Summary",
				"0",
			},
			wantNotContain: []string{
				red, // no red when overdue is 0
			},
		},
		{
			name:          "summary header uses box-drawing chars",
			open:          0,
			overdue:       0,
			dueThisWeek:   0,
			completedWeek: 0,
			noColor:       true,
			wantContains: []string{
				"\u2550", // ═ character
				"Task Summary",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSummary(tt.open, tt.overdue, tt.dueThisWeek, tt.completedWeek, tt.noColor)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("FormatSummary() missing %q in output:\n%s", want, got)
				}
			}
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(got, notWant) {
					t.Errorf("FormatSummary() should not contain %q in output:\n%s", notWant, got)
				}
			}
		})
	}
}

func TestFormatSummaryAlignment(t *testing.T) {
	got := FormatSummary(42, 3, 7, 12, true)
	lines := strings.Split(got, "\n")

	// Find the data lines (lines that contain numbers)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.Contains(trimmed, "═") {
			continue
		}
		// Each data line should have the label left-aligned and number right-aligned
		// within a consistent width
		if strings.Contains(line, "Open tasks") ||
			strings.Contains(line, "Overdue") ||
			strings.Contains(line, "Due this week") ||
			strings.Contains(line, "Completed this week") {
			// Verify the line has some padding (right-alignment of numbers)
			if len(line) < 20 {
				t.Errorf("Line too short, expected padded format: %q", line)
			}
		}
	}
}
