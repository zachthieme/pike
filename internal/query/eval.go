package query

import (
	"pike/internal/model"
	"time"
)

// Eval evaluates an AST node against a task, returning true if the task matches.
// The now parameter is used to resolve relative date expressions (today, today+Nd).
func Eval(node Node, task *model.Task, now time.Time) bool {
	switch n := node.(type) {
	case *OpenNode:
		return task.State == model.Open
	case *CompletedNode:
		return task.State == model.Completed
	case *TagNode:
		return task.HasTag(n.Name)
	case *DateCmpNode:
		return evalDateCmp(n, task, now)
	case *RegexNode:
		return n.CompiledRe.MatchString(task.Text)
	case *AndNode:
		return Eval(n.Left, task, now) && Eval(n.Right, task, now)
	case *OrNode:
		return Eval(n.Left, task, now) || Eval(n.Right, task, now)
	case *NotNode:
		return !Eval(n.Expr, task, now)
	default:
		return false
	}
}

// evalDateCmp evaluates a date comparison node against a task.
func evalDateCmp(n *DateCmpNode, task *model.Task, now time.Time) bool {
	// Resolve the task's date field
	var taskDate *time.Time
	switch n.Field {
	case "due":
		taskDate = task.Due
	case "completed":
		taskDate = task.Completed
	default:
		return false
	}

	// If the task's date field is nil, it doesn't match
	if taskDate == nil {
		return false
	}

	// Resolve the target date
	var target time.Time
	if n.Literal != nil {
		target = *n.Literal
	} else {
		// Normalize now to midnight
		target = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		target = target.AddDate(0, 0, n.Days)
	}

	// Normalize task date to midnight for comparison
	td := time.Date(taskDate.Year(), taskDate.Month(), taskDate.Day(), 0, 0, 0, 0, taskDate.Location())

	switch n.Op {
	case "<":
		return td.Before(target)
	case ">":
		return td.After(target)
	case "<=":
		return !td.After(target)
	case ">=":
		return !td.Before(target)
	default:
		return false
	}
}
