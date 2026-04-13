package query

import (
	"github.com/zachthieme/pike/internal/model"
	"testing"
	"time"
)

var fuzzTasks = func() []*model.Task {
	t1 := model.TaskWith(model.Task{Text: "open task @due(2026-03-16) @today", State: model.Open, Tags: []model.Tag{{Name: "due", Value: "2026-03-16"}, {Name: "today"}}, Due: timePtr(time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC))})
	t2 := model.TaskWith(model.Task{Text: "completed task @completed(2026-03-10)", State: model.Completed, Tags: []model.Tag{{Name: "completed", Value: "2026-03-10"}}, Completed: timePtr(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC))})
	t3 := model.TaskWith(model.Task{Text: "plain task @risk", State: model.Open, Tags: []model.Tag{{Name: "risk"}}})
	t4 := model.TaskWith(model.Task{Text: "checkbox task @today", State: model.Open, HasCheckbox: true, Tags: []model.Tag{{Name: "today"}}})
	return []*model.Task{&t1, &t2, &t3, &t4}
}()

func timePtr(t time.Time) *time.Time { return &t }

func FuzzParse(f *testing.F) {
	seeds := []string{
		"open",
		"completed",
		"open and @due < today",
		"open or completed",
		"not @risk",
		"@due >= today+3d",
		"@due = 2026-03-16",
		"\"partial text\"",
		"/regex.*pattern/",
		"(open or completed) and @today",
		"",
		"@",
		"(((",
		"and and and",
		"open and @due < today+9999d",
		"task",
		"bullet",
		"open task and not @due",
		"bullet and @risk",
		"open task",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	now := time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

	f.Fuzz(func(t *testing.T, input string) {
		node, err := Parse(input)
		if err != nil {
			return
		}
		if node == nil {
			return
		}
		for _, task := range fuzzTasks {
			Eval(node, task, now)
		}
	})
}
