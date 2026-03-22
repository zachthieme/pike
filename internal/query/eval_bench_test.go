package query

import (
	"testing"
	"time"

	"github.com/zachthieme/pike/internal/model"
)

var benchNow = time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)

func makeTask(text string, state model.TaskState, tags []model.Tag, due *time.Time) model.Task {
	t := model.TaskWith(model.Task{
		Text:        text,
		State:       state,
		Tags:        tags,
		Due:         due,
		HasCheckbox: true,
		File:        "bench.md",
		Line:        1,
	})
	return t
}

var benchQueries = []struct {
	name  string
	query string
}{
	{"simple_open", "open"},
	{"tag_match", "@today"},
	{"date_comparison", "@due < today"},
	{"compound_and", "open and @today"},
	{"compound_or", "@risk or @urgent"},
	{"complex", "open and (@due < today or @today) and not @deferred"},
	{"regex", "/deploy/"},
	{"text_match", `"deploy API"`},
}

func BenchmarkParseAndEval(b *testing.B) {
	dueDate := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	task := makeTask(
		"deploy API to staging @due(2026-03-14) @risk @today",
		model.Open,
		[]model.Tag{
			{Name: "due", Value: "2026-03-14"},
			{Name: "risk"},
			{Name: "today"},
		},
		&dueDate,
	)

	for _, tt := range benchQueries {
		b.Run(tt.name, func(b *testing.B) {
			node, err := Parse(tt.query)
			if err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for b.Loop() {
				Eval(node, &task, benchNow)
			}
		})
	}
}

func BenchmarkParse(b *testing.B) {
	for _, tt := range benchQueries {
		b.Run(tt.name, func(b *testing.B) {
			for b.Loop() {
				_, _ = Parse(tt.query)
			}
		})
	}
}
