package query

import (
	"regexp"
	"time"
)

// Node is the interface for all AST nodes in the query DSL.
type Node interface {
	nodeType() string
}

// openNode matches tasks with State == Open.
type openNode struct{}

func (n *openNode) nodeType() string { return "open" }

// completedNode matches tasks with State == Completed.
type completedNode struct{}

func (n *completedNode) nodeType() string { return "completed" }

// tagNode matches tasks that have a specific tag.
type tagNode struct {
	Name string // tag name without @
}

func (n *tagNode) nodeType() string { return "tag" }

// dateCmpNode compares a task's date field against a target date.
// Field is the tag name ("due" or "completed").
// Op is one of "<", ">", "<=", ">=", "=".
// Days is the offset from "now": 0 = today, +3 = today+3d, -7 = today-7d.
// If Literal is non-nil, it's a literal YYYY-MM-DD date and Days is ignored.
type dateCmpNode struct {
	Field   string     // "due" or "completed"
	Op      string     // "<", ">", "<=", ">=", "=" (== is normalized to = at parse time)
	Days    int        // relative to now
	Literal *time.Time // if non-nil, use this instead of Days
}

func (n *dateCmpNode) nodeType() string { return "datecmp" }

// regexNode matches tasks whose text matches a regex pattern.
type regexNode struct {
	Pattern    string
	CompiledRe *regexp.Regexp
}

func (n *regexNode) nodeType() string { return "regex" }

// andNode is a logical AND of two sub-expressions.
type andNode struct {
	Left  Node
	Right Node
}

func (n *andNode) nodeType() string { return "and" }

// orNode is a logical OR of two sub-expressions.
type orNode struct {
	Left  Node
	Right Node
}

func (n *orNode) nodeType() string { return "or" }

// notNode is a logical NOT of a sub-expression.
type notNode struct {
	Expr Node
}

func (n *notNode) nodeType() string { return "not" }

// textNode matches tasks whose text contains a substring (case-insensitive).
type textNode struct {
	Pattern      string // the text to search for
	LowerPattern string // pre-lowercased for efficient per-task matching
}

func (n *textNode) nodeType() string { return "text" }
