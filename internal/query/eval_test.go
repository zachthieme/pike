package query

import (
	"pike/internal/model"
	"regexp"
	"testing"
	"time"
)

func date(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

var now = time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC)

func TestEvalStateMatching(t *testing.T) {
	openTask := &model.Task{State: model.Open, Text: "Do stuff"}
	completedTask := &model.Task{State: model.Completed, Text: "Done stuff"}

	if !Eval(&OpenNode{}, openTask, now) {
		t.Error("OpenNode should match open task")
	}
	if Eval(&OpenNode{}, completedTask, now) {
		t.Error("OpenNode should not match completed task")
	}
	if Eval(&CompletedNode{}, openTask, now) {
		t.Error("CompletedNode should not match open task")
	}
	if !Eval(&CompletedNode{}, completedTask, now) {
		t.Error("CompletedNode should match completed task")
	}
}

func TestEvalTagMatching(t *testing.T) {
	task := &model.Task{
		Text: "Fix bug @today @risk",
		Tags: []model.Tag{
			{Name: "today"},
			{Name: "risk"},
		},
	}

	if !Eval(&TagNode{Name: "today"}, task, now) {
		t.Error("TagNode should match existing tag")
	}
	if !Eval(&TagNode{Name: "risk"}, task, now) {
		t.Error("TagNode should match existing tag")
	}
	if Eval(&TagNode{Name: "horizon"}, task, now) {
		t.Error("TagNode should not match missing tag")
	}
}

func TestEvalAndOrNot(t *testing.T) {
	task := &model.Task{
		State: model.Open,
		Text:  "Task @today",
		Tags:  []model.Tag{{Name: "today"}},
	}

	// And: both true
	if !Eval(&AndNode{Left: &OpenNode{}, Right: &TagNode{Name: "today"}}, task, now) {
		t.Error("and(open, @today) should be true")
	}
	// And: one false
	if Eval(&AndNode{Left: &CompletedNode{}, Right: &TagNode{Name: "today"}}, task, now) {
		t.Error("and(completed, @today) should be false")
	}
	// Or: one true
	if !Eval(&OrNode{Left: &CompletedNode{}, Right: &TagNode{Name: "today"}}, task, now) {
		t.Error("or(completed, @today) should be true")
	}
	// Or: both false
	if Eval(&OrNode{Left: &CompletedNode{}, Right: &TagNode{Name: "horizon"}}, task, now) {
		t.Error("or(completed, @horizon) should be false")
	}
	// Not
	if Eval(&NotNode{Expr: &OpenNode{}}, task, now) {
		t.Error("not(open) should be false for open task")
	}
	if !Eval(&NotNode{Expr: &CompletedNode{}}, task, now) {
		t.Error("not(completed) should be true for open task")
	}
}

func TestEvalDateComparisons(t *testing.T) {
	// Task due 2026-03-10 (3 days before now=2026-03-13)
	overdueTask := &model.Task{
		State: model.Open,
		Text:  "Overdue task",
		Due:   date(2026, time.March, 10),
	}

	// Task due 2026-03-15 (2 days after now)
	futureTask := &model.Task{
		State: model.Open,
		Text:  "Future task",
		Due:   date(2026, time.March, 15),
	}

	// @due < today: overdue should match, future should not
	dueLtToday := &DateCmpNode{Field: "due", Op: "<", Days: 0}
	if !Eval(dueLtToday, overdueTask, now) {
		t.Error("overdue task should match @due < today")
	}
	if Eval(dueLtToday, futureTask, now) {
		t.Error("future task should not match @due < today")
	}

	// @due > today: future should match, overdue should not
	dueGtToday := &DateCmpNode{Field: "due", Op: ">", Days: 0}
	if !Eval(dueGtToday, futureTask, now) {
		t.Error("future task should match @due > today")
	}
	if Eval(dueGtToday, overdueTask, now) {
		t.Error("overdue task should not match @due > today")
	}

	// @due >= today+3d: only future (2026-03-15) matches today+3d (2026-03-16)? No, 15 < 16
	dueGteToday3 := &DateCmpNode{Field: "due", Op: ">=", Days: 3}
	if Eval(dueGteToday3, futureTask, now) {
		t.Error("task due 2026-03-15 should not match @due >= today+3d (2026-03-16)")
	}

	// @due <= today+3d: future (2026-03-15) <= 2026-03-16 is true
	dueLteToday3 := &DateCmpNode{Field: "due", Op: "<=", Days: 3}
	if !Eval(dueLteToday3, futureTask, now) {
		t.Error("task due 2026-03-15 should match @due <= today+3d (2026-03-16)")
	}
}

func TestEvalDateComparisonNilDate(t *testing.T) {
	task := &model.Task{
		State: model.Open,
		Text:  "No due date",
	}
	node := &DateCmpNode{Field: "due", Op: "<", Days: 0}
	if Eval(node, task, now) {
		t.Error("task without due date should not match date comparison")
	}
}

func TestEvalDateComparisonLiteral(t *testing.T) {
	task := &model.Task{
		State: model.Open,
		Text:  "Task with due",
		Due:   date(2026, time.March, 10),
	}
	lit := time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC)
	node := &DateCmpNode{Field: "due", Op: "<", Literal: &lit}
	if !Eval(node, task, now) {
		t.Error("task due 2026-03-10 should match @due < 2026-03-15")
	}
}

func TestEvalRegex(t *testing.T) {
	task := &model.Task{
		Text: "Review meeting notes @today",
	}
	if !Eval(&RegexNode{Pattern: "meeting", CompiledRe: regexp.MustCompile("meeting")}, task, now) {
		t.Error("regex 'meeting' should match")
	}
	if Eval(&RegexNode{Pattern: "budget", CompiledRe: regexp.MustCompile("budget")}, task, now) {
		t.Error("regex 'budget' should not match")
	}
}

func TestEvalIntegrationOverdue(t *testing.T) {
	// "open and @due < today" matches overdue open task
	node, err := Parse("open and @due < today")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	overdueOpen := &model.Task{
		State: model.Open,
		Text:  "Overdue @due(2026-03-10)",
		Tags:  []model.Tag{{Name: "due", Value: "2026-03-10"}},
		Due:   date(2026, time.March, 10),
	}
	if !Eval(node, overdueOpen, now) {
		t.Error("open overdue task should match 'open and @due < today'")
	}

	// Completed overdue task should NOT match
	overdueCompleted := &model.Task{
		State: model.Completed,
		Text:  "Overdue completed @due(2026-03-10)",
		Tags:  []model.Tag{{Name: "due", Value: "2026-03-10"}},
		Due:   date(2026, time.March, 10),
	}
	if Eval(node, overdueCompleted, now) {
		t.Error("completed overdue task should not match 'open and @due < today'")
	}
}

func TestEvalIntegrationDailyView(t *testing.T) {
	// "open and (@today or @weekly)" matches task with @today tag
	node, err := Parse("open and (@today or @weekly)")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	todayTask := &model.Task{
		State: model.Open,
		Text:  "Call dentist @today",
		Tags:  []model.Tag{{Name: "today"}},
	}
	if !Eval(node, todayTask, now) {
		t.Error("open task with @today should match 'open and (@today or @weekly)'")
	}

	weeklyTask := &model.Task{
		State: model.Open,
		Text:  "Review OKRs @weekly",
		Tags:  []model.Tag{{Name: "weekly"}},
	}
	if !Eval(node, weeklyTask, now) {
		t.Error("open task with @weekly should match 'open and (@today or @weekly)'")
	}

	riskTask := &model.Task{
		State: model.Open,
		Text:  "Something @risk",
		Tags:  []model.Tag{{Name: "risk"}},
	}
	if Eval(node, riskTask, now) {
		t.Error("open task with @risk should not match 'open and (@today or @weekly)'")
	}
}

func TestEvalIntegrationRecentlyCompleted(t *testing.T) {
	// "completed and @completed >= today-7d" matches recently completed task
	node, err := Parse("completed and @completed >= today-7d")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	recentlyCompleted := &model.Task{
		State:     model.Completed,
		Text:      "Done recently @completed(2026-03-10)",
		Tags:      []model.Tag{{Name: "completed", Value: "2026-03-10"}},
		Completed: date(2026, time.March, 10),
	}
	if !Eval(node, recentlyCompleted, now) {
		t.Error("recently completed task should match 'completed and @completed >= today-7d'")
	}

	oldCompleted := &model.Task{
		State:     model.Completed,
		Text:      "Done long ago @completed(2026-02-01)",
		Tags:      []model.Tag{{Name: "completed", Value: "2026-02-01"}},
		Completed: date(2026, time.February, 1),
	}
	if Eval(node, oldCompleted, now) {
		t.Error("old completed task should not match 'completed and @completed >= today-7d'")
	}
}
