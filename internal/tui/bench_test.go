package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zachthieme/pike/internal/config"
	"github.com/zachthieme/pike/internal/model"
)

// makeBenchTasks generates n tasks with varying tags and states.
func makeBenchTasks(n int) []model.Task {
	tags := []string{"today", "risk", "due", "weekly", "horizon", "talk"}
	tasks := make([]model.Task, n)
	now := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)

	for i := range n {
		state := model.Open
		if i%5 == 0 {
			state = model.Completed
		}

		taskTags := []model.Tag{{Name: tags[i%len(tags)]}}

		var due *time.Time
		if i%3 == 0 {
			d := now.AddDate(0, 0, i%14-7)
			due = &d
			taskTags = append(taskTags, model.Tag{Name: "due", Value: d.Format("2006-01-02")})
		}

		tasks[i] = model.TaskWith(model.Task{
			Text:        fmt.Sprintf("task %d do something important @%s", i, tags[i%len(tags)]),
			State:       state,
			File:        fmt.Sprintf("notes/file%d.md", i%10),
			Line:        i + 1,
			Tags:        taskTags,
			Due:         due,
			HasCheckbox: true,
		})
	}
	return tasks
}

func benchConfig() *config.Config {
	return &config.Config{
		NotesDir:        "/tmp/notes",
		Include:         []string{"**/*.md"},
		RefreshInterval: 5 * time.Second,
		Editor:          "hx",
		TagColors: map[string]string{
			"due":       "#f38ba8",
			"risk":      "#f38ba8",
			"today":     "#a6e3a1",
			"completed": "#a6e3a1",
			"weekly":    "#89b4fa",
			"horizon":   "#f9e2af",
			"talk":      "#cba6f7",
			"_default":  "#94e2d5",
		},
		LinkColor:    "#89b4fa",
		HiddenColor:  "#6c7086",
		VisibleColor: "#f5c2e7",
		Views: []config.ViewConfig{
			{Title: "Today", Query: "open and @today", Sort: "due_asc", Color: "#a6e3a1", Order: 1},
			{Title: "Overdue", Query: "open and @due < today", Sort: "due_asc", Color: "#f38ba8", Order: 2},
			{Title: "This Week", Query: "open and @due >= today and @due <= today+7d", Sort: "due_asc", Color: "#f9e2af", Order: 3},
		},
		RecentlyCompletedDays: 7,
	}
}

func BenchmarkRebuildSections(b *testing.B) {
	for _, size := range []int{100, 500, 2000} {
		b.Run(fmt.Sprintf("tasks_%d", size), func(b *testing.B) {
			tasks := makeBenchTasks(size)
			cfg := benchConfig()
			m := NewModel(cfg, tasks, nil, nil)
			m.width = 120
			m.height = 40
			b.ResetTimer()
			for b.Loop() {
				m.rebuildSections()
			}
		})
	}
}

func BenchmarkApplySubstringFilter(b *testing.B) {
	tasks := makeBenchTasks(1000)
	for _, filter := range []string{"task", "important risk", "nonexistent"} {
		b.Run(filter, func(b *testing.B) {
			for b.Loop() {
				applySubstringFilter(tasks, filter)
			}
		})
	}
}

func BenchmarkApplyDSLFilter(b *testing.B) {
	tasks := makeBenchTasks(1000)
	now := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)

	queries := []struct {
		name  string
		query string
	}{
		{"simple", "open"},
		{"tag", "@today"},
		{"date", "open and @due < today"},
		{"complex", "open and (@due < today or @today) and not @weekly"},
	}

	for _, tt := range queries {
		b.Run(tt.name, func(b *testing.B) {
			for b.Loop() {
				_, _ = applyDSLFilter(tasks, tt.query, now)
			}
		})
	}
}

func BenchmarkScrollWindow(b *testing.B) {
	for b.Loop() {
		scrollWindow(500, 2000, 40)
	}
}

func BenchmarkTruncateView(b *testing.B) {
	// Build a realistic rendered output with one reverse-video line.
	var sb strings.Builder
	for i := range 200 {
		if i == 100 {
			sb.WriteString(reverseVideoEsc + "  ○ selected task line\033[0m\n")
		} else {
			fmt.Fprintf(&sb, "  ○ task line %d @today\n", i)
		}
	}
	output := sb.String()

	m := Model{height: 40}
	b.ResetTimer()
	for b.Loop() {
		m.truncateView(output)
	}
}

func BenchmarkFlatTasks(b *testing.B) {
	tasks := makeBenchTasks(1000)
	cfg := benchConfig()
	m := NewModel(cfg, tasks, nil, nil)
	m.width = 120
	m.height = 40
	b.ResetTimer()
	for b.Loop() {
		flatTasks(m.displaySections())
	}
}
