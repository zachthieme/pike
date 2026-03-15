package query

import (
	"pike/internal/model"
	"strings"
	"time"
)

// Eval evaluates an AST node against a task, returning true if the task matches.
// The now parameter is used to resolve relative date expressions (today, today+Nd).
func Eval(node Node, task *model.Task, now time.Time) bool {
	return EvalWithOptions(node, task, now, EvalOptions{})
}

// EvalOptions configures evaluation behavior.
type EvalOptions struct {
	PartialTags bool // When true, @tag matches any tag containing the name as substring
}

// EvalWithOptions evaluates an AST node against a task with configurable options.
func EvalWithOptions(node Node, task *model.Task, now time.Time, opts EvalOptions) bool {
	switch n := node.(type) {
	case *OpenNode:
		return task.State == model.Open
	case *CompletedNode:
		return task.State == model.Completed
	case *TagNode:
		if opts.PartialTags {
			return hasTagPartial(task, strings.ToLower(n.Name))
		}
		return task.HasTag(n.Name)
	case *DateCmpNode:
		return evalDateCmp(n, task, now)
	case *TextNode:
		return strings.Contains(strings.ToLower(task.Text), n.LowerPattern)
	case *RegexNode:
		return n.CompiledRe.MatchString(task.Text)
	case *AndNode:
		return EvalWithOptions(n.Left, task, now, opts) && EvalWithOptions(n.Right, task, now, opts)
	case *OrNode:
		return EvalWithOptions(n.Left, task, now, opts) || EvalWithOptions(n.Right, task, now, opts)
	case *NotNode:
		return !EvalWithOptions(n.Expr, task, now, opts)
	default:
		return false
	}
}

// hasTagPartial returns true if any tag name contains the query as a substring.
// The name parameter must already be lowercased.
func hasTagPartial(task *model.Task, lowerName string) bool {
	for _, tag := range task.Tags {
		if strings.Contains(strings.ToLower(tag.Name), lowerName) {
			return true
		}
	}
	return false
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
	case "=", "==":
		return td.Equal(target)
	default:
		return false
	}
}
