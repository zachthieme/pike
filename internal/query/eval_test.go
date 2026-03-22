package query

import (
	"github.com/zachthieme/pike/internal/model"
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

	if !Eval(&openNode{}, openTask, now) {
		t.Error("openNode should match open task")
	}
	if Eval(&openNode{}, completedTask, now) {
		t.Error("openNode should not match completed task")
	}
	if Eval(&completedNode{}, openTask, now) {
		t.Error("completedNode should not match open task")
	}
	if !Eval(&completedNode{}, completedTask, now) {
		t.Error("completedNode should match completed task")
	}
}

func TestEvalTagMatching(t *testing.T) {
	task := &model.Task{
		Text: "Fix bug @today @risk",
		Tags: []model.Tag{
			{Name: "today"},
			{Name: "risk"},
		},
		TagSet: map[string]bool{"today": true, "risk": true},
	}

	if !Eval(&tagNode{Name: "today"}, task, now) {
		t.Error("tagNode should match existing tag")
	}
	if !Eval(&tagNode{Name: "risk"}, task, now) {
		t.Error("tagNode should match existing tag")
	}
	if Eval(&tagNode{Name: "horizon"}, task, now) {
		t.Error("tagNode should not match missing tag")
	}
}

func TestEvalAndOrNot(t *testing.T) {
	task := &model.Task{
		State: model.Open,
		Text:  "Task @today",
		Tags:  []model.Tag{{Name: "today"}},
		TagSet: map[string]bool{"today": true},
	}

	// And: both true
	if !Eval(&andNode{Left: &openNode{}, Right: &tagNode{Name: "today"}}, task, now) {
		t.Error("and(open, @today) should be true")
	}
	// And: one false
	if Eval(&andNode{Left: &completedNode{}, Right: &tagNode{Name: "today"}}, task, now) {
		t.Error("and(completed, @today) should be false")
	}
	// Or: one true
	if !Eval(&orNode{Left: &completedNode{}, Right: &tagNode{Name: "today"}}, task, now) {
		t.Error("or(completed, @today) should be true")
	}
	// Or: both false
	if Eval(&orNode{Left: &completedNode{}, Right: &tagNode{Name: "horizon"}}, task, now) {
		t.Error("or(completed, @horizon) should be false")
	}
	// Not
	if Eval(&notNode{Expr: &openNode{}}, task, now) {
		t.Error("not(open) should be false for open task")
	}
	if !Eval(&notNode{Expr: &completedNode{}}, task, now) {
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
	dueLtToday := &dateCmpNode{Field: "due", Op: "<", Days: 0}
	if !Eval(dueLtToday, overdueTask, now) {
		t.Error("overdue task should match @due < today")
	}
	if Eval(dueLtToday, futureTask, now) {
		t.Error("future task should not match @due < today")
	}

	// @due > today: future should match, overdue should not
	dueGtToday := &dateCmpNode{Field: "due", Op: ">", Days: 0}
	if !Eval(dueGtToday, futureTask, now) {
		t.Error("future task should match @due > today")
	}
	if Eval(dueGtToday, overdueTask, now) {
		t.Error("overdue task should not match @due > today")
	}

	// @due >= today+3d: only future (2026-03-15) matches today+3d (2026-03-16)? No, 15 < 16
	dueGteToday3 := &dateCmpNode{Field: "due", Op: ">=", Days: 3}
	if Eval(dueGteToday3, futureTask, now) {
		t.Error("task due 2026-03-15 should not match @due >= today+3d (2026-03-16)")
	}

	// @due <= today+3d: future (2026-03-15) <= 2026-03-16 is true
	dueLteToday3 := &dateCmpNode{Field: "due", Op: "<=", Days: 3}
	if !Eval(dueLteToday3, futureTask, now) {
		t.Error("task due 2026-03-15 should match @due <= today+3d (2026-03-16)")
	}
}

func TestEvalDateEqualityToday(t *testing.T) {
	todayTask := &model.Task{
		State:  model.Open,
		Text:   "Due today",
		Due:    date(2026, time.March, 13),
		Tags:   []model.Tag{{Name: "due", Value: "2026-03-13"}},
		TagSet: map[string]bool{"due": true},
	}
	tomorrowTask := &model.Task{
		State:  model.Open,
		Text:   "Due tomorrow",
		Due:    date(2026, time.March, 14),
		Tags:   []model.Tag{{Name: "due", Value: "2026-03-14"}},
		TagSet: map[string]bool{"due": true},
	}

	dueEqToday := &dateCmpNode{Field: "due", Op: "=", Days: 0}
	if !Eval(dueEqToday, todayTask, now) {
		t.Error("task due today should match @due = today")
	}
	if Eval(dueEqToday, tomorrowTask, now) {
		t.Error("task due tomorrow should not match @due = today")
	}
}

func TestEvalDateComparisonNilDate(t *testing.T) {
	task := &model.Task{
		State: model.Open,
		Text:  "No due date",
	}
	node := &dateCmpNode{Field: "due", Op: "<", Days: 0}
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
	node := &dateCmpNode{Field: "due", Op: "<", Literal: &lit}
	if !Eval(node, task, now) {
		t.Error("task due 2026-03-10 should match @due < 2026-03-15")
	}
}

func TestEvalRegex(t *testing.T) {
	task := &model.Task{
		Text: "Review meeting notes @today",
	}
	if !Eval(&regexNode{Pattern: "meeting", CompiledRe: regexp.MustCompile("meeting")}, task, now) {
		t.Error("regex 'meeting' should match")
	}
	if Eval(&regexNode{Pattern: "budget", CompiledRe: regexp.MustCompile("budget")}, task, now) {
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
		State:  model.Open,
		Text:   "Overdue @due(2026-03-10)",
		Tags:   []model.Tag{{Name: "due", Value: "2026-03-10"}},
		TagSet: map[string]bool{"due": true},
		Due:    date(2026, time.March, 10),
	}
	if !Eval(node, overdueOpen, now) {
		t.Error("open overdue task should match 'open and @due < today'")
	}

	// Completed overdue task should NOT match
	overdueCompleted := &model.Task{
		State:  model.Completed,
		Text:   "Overdue completed @due(2026-03-10)",
		Tags:   []model.Tag{{Name: "due", Value: "2026-03-10"}},
		TagSet: map[string]bool{"due": true},
		Due:    date(2026, time.March, 10),
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
		State:  model.Open,
		Text:   "Call dentist @today",
		Tags:   []model.Tag{{Name: "today"}},
		TagSet: map[string]bool{"today": true},
	}
	if !Eval(node, todayTask, now) {
		t.Error("open task with @today should match 'open and (@today or @weekly)'")
	}

	weeklyTask := &model.Task{
		State:  model.Open,
		Text:   "Review OKRs @weekly",
		Tags:   []model.Tag{{Name: "weekly"}},
		TagSet: map[string]bool{"weekly": true},
	}
	if !Eval(node, weeklyTask, now) {
		t.Error("open task with @weekly should match 'open and (@today or @weekly)'")
	}

	riskTask := &model.Task{
		State:  model.Open,
		Text:   "Something @risk",
		Tags:   []model.Tag{{Name: "risk"}},
		TagSet: map[string]bool{"risk": true},
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
		TagSet:    map[string]bool{"completed": true},
		Completed: date(2026, time.March, 10),
	}
	if !Eval(node, recentlyCompleted, now) {
		t.Error("recently completed task should match 'completed and @completed >= today-7d'")
	}

	oldCompleted := &model.Task{
		State:     model.Completed,
		Text:      "Done long ago @completed(2026-02-01)",
		Tags:      []model.Tag{{Name: "completed", Value: "2026-02-01"}},
		TagSet:    map[string]bool{"completed": true},
		Completed: date(2026, time.February, 1),
	}
	if Eval(node, oldCompleted, now) {
		t.Error("old completed task should not match 'completed and @completed >= today-7d'")
	}
}

func TestEvalWithOptionsPartialTagMatch(t *testing.T) {
	task := &model.Task{
		Text: "Task @due(2026-03-15) @duration(2h)",
		Tags: []model.Tag{
			{Name: "due", Value: "2026-03-15"},
			{Name: "duration", Value: "2h"},
		},
		TagSet: map[string]bool{"due": true, "duration": true},
	}
	opts := EvalOptions{PartialTags: true}

	// "du" should match both "due" and "duration"
	if !EvalWithOptions(&tagNode{Name: "du"}, task, now, opts) {
		t.Error("partial tag @du should match task with @due")
	}

	// Exact match still works
	if !EvalWithOptions(&tagNode{Name: "due"}, task, now, opts) {
		t.Error("exact tag @due should match")
	}

	// No match
	if EvalWithOptions(&tagNode{Name: "risk"}, task, now, opts) {
		t.Error("@risk should not match")
	}
}

func TestEvalWithOptionsExactByDefault(t *testing.T) {
	task := &model.Task{
		Text:   "Task @due(2026-03-15)",
		Tags:   []model.Tag{{Name: "due", Value: "2026-03-15"}},
		TagSet: map[string]bool{"due": true},
	}

	// Without PartialTags, "du" should NOT match "due"
	opts := EvalOptions{PartialTags: false}
	if EvalWithOptions(&tagNode{Name: "du"}, task, now, opts) {
		t.Error("without PartialTags, @du should not match @due")
	}

	// Original Eval should still be exact
	if Eval(&tagNode{Name: "du"}, task, now) {
		t.Error("Eval should use exact matching")
	}
}

func TestEvalTextNode(t *testing.T) {
	task := &model.Task{Text: "Deploy the service to production", LowerText: "deploy the service to production"}

	if !Eval(&textNode{Pattern: "deploy", LowerPattern: "deploy"}, task, now) {
		t.Error("textNode should match case-insensitively")
	}
	if !Eval(&textNode{Pattern: "service to", LowerPattern: "service to"}, task, now) {
		t.Error("textNode should match multi-word substrings")
	}
	if Eval(&textNode{Pattern: "staging", LowerPattern: "staging"}, task, now) {
		t.Error("textNode should not match missing text")
	}
}

func TestEvalIntegrationTextSearch(t *testing.T) {
	// "open and deploy" should parse and match
	node, err := Parse("open and deploy")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	task := &model.Task{State: model.Open, Text: "Deploy the service", LowerText: "deploy the service"}
	if !Eval(node, task, now) {
		t.Error("'open and deploy' should match open task containing 'deploy'")
	}

	closedTask := &model.Task{State: model.Completed, Text: "Deploy the service", LowerText: "deploy the service"}
	if Eval(node, closedTask, now) {
		t.Error("'open and deploy' should not match completed task")
	}
}

func TestEvalIntegrationQuotedText(t *testing.T) {
	node, err := Parse(`open and "meeting notes"`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	task := &model.Task{State: model.Open, Text: "Review meeting notes from Monday", LowerText: "review meeting notes from monday"}
	if !Eval(node, task, now) {
		t.Error(`'open and "meeting notes"' should match`)
	}

	task2 := &model.Task{State: model.Open, Text: "Review meeting agenda", LowerText: "review meeting agenda"}
	if Eval(node, task2, now) {
		t.Error(`'open and "meeting notes"' should not match "meeting agenda"`)
	}
}
