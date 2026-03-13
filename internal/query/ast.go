package query

import "time"

// Node is the interface for all AST nodes in the query DSL.
type Node interface {
	nodeType() string
}

// OpenNode matches tasks with State == Open.
type OpenNode struct{}

func (n *OpenNode) nodeType() string { return "open" }

// CompletedNode matches tasks with State == Completed.
type CompletedNode struct{}

func (n *CompletedNode) nodeType() string { return "completed" }

// TagNode matches tasks that have a specific tag.
type TagNode struct {
	Name string // tag name without @
}

func (n *TagNode) nodeType() string { return "tag" }

// DateCmpNode compares a task's date field against a target date.
// Field is the tag name ("due" or "completed").
// Op is one of "<", ">", "<=", ">=".
// Days is the offset from "now": 0 = today, +3 = today+3d, -7 = today-7d.
// If Literal is non-nil, it's a literal YYYY-MM-DD date and Days is ignored.
type DateCmpNode struct {
	Field   string     // "due" or "completed"
	Op      string     // "<", ">", "<=", ">="
	Days    int        // relative to now
	Literal *time.Time // if non-nil, use this instead of Days
}

func (n *DateCmpNode) nodeType() string { return "datecmp" }

// RegexNode matches tasks whose text matches a regex pattern.
type RegexNode struct {
	Pattern string
}

func (n *RegexNode) nodeType() string { return "regex" }

// AndNode is a logical AND of two sub-expressions.
type AndNode struct {
	Left  Node
	Right Node
}

func (n *AndNode) nodeType() string { return "and" }

// OrNode is a logical OR of two sub-expressions.
type OrNode struct {
	Left  Node
	Right Node
}

func (n *OrNode) nodeType() string { return "or" }

// NotNode is a logical NOT of a sub-expression.
type NotNode struct {
	Expr Node
}

func (n *NotNode) nodeType() string { return "not" }
