package query

import (
	"testing"
	"time"
)

func TestParseSimpleAtoms(t *testing.T) {
	tests := []struct {
		input    string
		wantType string
	}{
		{"open", "open"},
		{"completed", "completed"},
		{"task", "task"},
		{"bullet", "bullet"},
		{"@today", "tag"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			node, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.input, err)
			}
			if node.nodeType() != tt.wantType {
				t.Errorf("nodeType() = %q, want %q", node.nodeType(), tt.wantType)
			}
		})
	}

	// Check tag name for @today
	node, err := Parse("@today")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	tn, ok := node.(*tagNode)
	if !ok {
		t.Fatalf("expected *tagNode, got %T", node)
	}
	if tn.Name != "today" {
		t.Errorf("tag name = %q, want %q", tn.Name, "today")
	}
}

func TestParseAndPrecedence(t *testing.T) {
	node, err := Parse("open and @due")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	and, ok := node.(*andNode)
	if !ok {
		t.Fatalf("expected *andNode, got %T", node)
	}
	if _, ok := and.Left.(*openNode); !ok {
		t.Errorf("left = %T, want *openNode", and.Left)
	}
	if tag, ok := and.Right.(*tagNode); !ok {
		t.Errorf("right = %T, want *tagNode", and.Right)
	} else if tag.Name != "due" {
		t.Errorf("right tag name = %q, want %q", tag.Name, "due")
	}
}

func TestParseOrPrecedence(t *testing.T) {
	node, err := Parse("open or @due")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	or, ok := node.(*orNode)
	if !ok {
		t.Fatalf("expected *orNode, got %T", node)
	}
	if _, ok := or.Left.(*openNode); !ok {
		t.Errorf("left = %T, want *openNode", or.Left)
	}
	if tag, ok := or.Right.(*tagNode); !ok {
		t.Errorf("right = %T, want *tagNode", or.Right)
	} else if tag.Name != "due" {
		t.Errorf("right tag name = %q, want %q", tag.Name, "due")
	}
}

func TestParseAndOrPrecedence(t *testing.T) {
	// "a or b and c" should parse as "a or (b and c)" since and binds tighter
	node, err := Parse("open or @due and completed")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	or, ok := node.(*orNode)
	if !ok {
		t.Fatalf("expected *orNode at root, got %T", node)
	}
	if _, ok := or.Left.(*openNode); !ok {
		t.Errorf("left = %T, want *openNode", or.Left)
	}
	and, ok := or.Right.(*andNode)
	if !ok {
		t.Fatalf("right = %T, want *andNode", or.Right)
	}
	if _, ok := and.Left.(*tagNode); !ok {
		t.Errorf("and.left = %T, want *tagNode", and.Left)
	}
	if _, ok := and.Right.(*completedNode); !ok {
		t.Errorf("and.right = %T, want *completedNode", and.Right)
	}
}

func TestParseParens(t *testing.T) {
	node, err := Parse("(open or completed) and @today")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	and, ok := node.(*andNode)
	if !ok {
		t.Fatalf("expected *andNode, got %T", node)
	}
	or, ok := and.Left.(*orNode)
	if !ok {
		t.Fatalf("left = %T, want *orNode", and.Left)
	}
	if _, ok := or.Left.(*openNode); !ok {
		t.Errorf("or.left = %T, want *openNode", or.Left)
	}
	if _, ok := or.Right.(*completedNode); !ok {
		t.Errorf("or.right = %T, want *completedNode", or.Right)
	}
	if tag, ok := and.Right.(*tagNode); !ok {
		t.Errorf("right = %T, want *tagNode", and.Right)
	} else if tag.Name != "today" {
		t.Errorf("tag name = %q, want %q", tag.Name, "today")
	}
}

func TestParseNot(t *testing.T) {
	node, err := Parse("not @horizon")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	notN, ok := node.(*notNode)
	if !ok {
		t.Fatalf("expected *notNode, got %T", node)
	}
	tag, ok := notN.Expr.(*tagNode)
	if !ok {
		t.Fatalf("expr = %T, want *tagNode", notN.Expr)
	}
	if tag.Name != "horizon" {
		t.Errorf("tag name = %q, want %q", tag.Name, "horizon")
	}
}

func TestParseDateComparisons(t *testing.T) {
	tests := []struct {
		input     string
		field     string
		op        string
		days      int
		isLiteral bool
		litYear   int
		litMonth  time.Month
		litDay    int
	}{
		{"@due < today", "due", "<", 0, false, 0, 0, 0},
		{"@due >= today+3d", "due", ">=", 3, false, 0, 0, 0},
		{"@completed >= today-14d", "completed", ">=", -14, false, 0, 0, 0},
		{"@due < 2026-03-15", "due", "<", 0, true, 2026, time.March, 15},
		{"@due = today", "due", "=", 0, false, 0, 0, 0},
		{"@due == 2026-03-15", "due", "=", 0, true, 2026, time.March, 15},
		{"@due < tomorrow", "due", "<", 1, false, 0, 0, 0},
		{"@due >= yesterday", "due", ">=", -1, false, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			node, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.input, err)
			}
			dc, ok := node.(*dateCmpNode)
			if !ok {
				t.Fatalf("expected *dateCmpNode, got %T", node)
			}
			if dc.Field != tt.field {
				t.Errorf("field = %q, want %q", dc.Field, tt.field)
			}
			if dc.Op != tt.op {
				t.Errorf("op = %q, want %q", dc.Op, tt.op)
			}
			if tt.isLiteral {
				if dc.Literal == nil {
					t.Fatal("expected non-nil Literal")
				}
				want := time.Date(tt.litYear, tt.litMonth, tt.litDay, 0, 0, 0, 0, time.UTC)
				if !dc.Literal.Equal(want) {
					t.Errorf("literal = %v, want %v", dc.Literal, want)
				}
			} else {
				if dc.Literal != nil {
					t.Errorf("expected nil Literal, got %v", dc.Literal)
				}
				if dc.Days != tt.days {
					t.Errorf("days = %d, want %d", dc.Days, tt.days)
				}
			}
		})
	}
}

func TestParseRegex(t *testing.T) {
	node, err := Parse("/meeting/")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	rx, ok := node.(*regexNode)
	if !ok {
		t.Fatalf("expected *regexNode, got %T", node)
	}
	if rx.Pattern != "meeting" {
		t.Errorf("pattern = %q, want %q", rx.Pattern, "meeting")
	}
}

func TestParseComplexSpecExamples(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"overdue", "open and @due < today"},
		{"daily view", "open and (@today or @weekly)"},
		{"completed last 2 weeks", "completed and @completed >= today-14d"},
		{"meeting search", "open and /meeting/"},
		{"exclude horizon", "open and not @horizon"},
		{"open tasks without due", "open task and not @due"},
		{"bullets with reporting", "bullet and @reporting"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.input, err)
			}
		})
	}
}

func TestParseImplicitAnd(t *testing.T) {
	// "open task" should parse as andNode{openNode, taskNode}
	node, err := Parse("open task")
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", "open task", err)
	}
	and, ok := node.(*andNode)
	if !ok {
		t.Fatalf("expected *andNode, got %T", node)
	}
	if _, ok := and.Left.(*openNode); !ok {
		t.Errorf("left = %T, want *openNode", and.Left)
	}
	if _, ok := and.Right.(*taskNode); !ok {
		t.Errorf("right = %T, want *taskNode", and.Right)
	}

	// "open task and not @due" — mixed implicit and explicit
	node2, err := Parse("open task and not @due")
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", "open task and not @due", err)
	}
	// Should be: and(and(open, task), not(@due))
	topAnd, ok := node2.(*andNode)
	if !ok {
		t.Fatalf("expected *andNode at root, got %T", node2)
	}
	innerAnd, ok := topAnd.Left.(*andNode)
	if !ok {
		t.Fatalf("left = %T, want *andNode (implicit)", topAnd.Left)
	}
	if _, ok := innerAnd.Left.(*openNode); !ok {
		t.Errorf("inner left = %T, want *openNode", innerAnd.Left)
	}
	if _, ok := innerAnd.Right.(*taskNode); !ok {
		t.Errorf("inner right = %T, want *taskNode", innerAnd.Right)
	}
	notN, ok := topAnd.Right.(*notNode)
	if !ok {
		t.Fatalf("right = %T, want *notNode", topAnd.Right)
	}
	if tag, ok := notN.Expr.(*tagNode); !ok || tag.Name != "due" {
		t.Errorf("not expr = %T, want *tagNode{due}", notN.Expr)
	}

	// "@risk @today" — two tags implicitly ANDed
	node3, err := Parse("@risk @today")
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", "@risk @today", err)
	}
	and3, ok := node3.(*andNode)
	if !ok {
		t.Fatalf("expected *andNode, got %T", node3)
	}
	if tag, ok := and3.Left.(*tagNode); !ok || tag.Name != "risk" {
		t.Errorf("left = %T, want *tagNode{risk}", and3.Left)
	}
	if tag, ok := and3.Right.(*tagNode); !ok || tag.Name != "today" {
		t.Errorf("right = %T, want *tagNode{today}", and3.Right)
	}
}

func TestParseImplicitAndPrecedence(t *testing.T) {
	// "open or completed task" → open OR (completed AND task)
	// Implicit AND binds tighter than OR, same as explicit AND.
	node, err := Parse("open or completed task")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	or, ok := node.(*orNode)
	if !ok {
		t.Fatalf("expected *orNode at root, got %T", node)
	}
	if _, ok := or.Left.(*openNode); !ok {
		t.Errorf("left = %T, want *openNode", or.Left)
	}
	and, ok := or.Right.(*andNode)
	if !ok {
		t.Fatalf("right = %T, want *andNode", or.Right)
	}
	if _, ok := and.Left.(*completedNode); !ok {
		t.Errorf("and.left = %T, want *completedNode", and.Left)
	}
	if _, ok := and.Right.(*taskNode); !ok {
		t.Errorf("and.right = %T, want *taskNode", and.Right)
	}

	// "not not task" → not(not(task))
	node2, err := Parse("not not task")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	outer, ok := node2.(*notNode)
	if !ok {
		t.Fatalf("expected *notNode, got %T", node2)
	}
	inner, ok := outer.Expr.(*notNode)
	if !ok {
		t.Fatalf("inner = %T, want *notNode", outer.Expr)
	}
	if _, ok := inner.Expr.(*taskNode); !ok {
		t.Errorf("innermost = %T, want *taskNode", inner.Expr)
	}

	// "(open) (task)" → implicit AND across parenthesized groups
	node3, err := Parse("(open) (task)")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	and3, ok := node3.(*andNode)
	if !ok {
		t.Fatalf("expected *andNode, got %T", node3)
	}
	if _, ok := and3.Left.(*openNode); !ok {
		t.Errorf("left = %T, want *openNode", and3.Left)
	}
	if _, ok := and3.Right.(*taskNode); !ok {
		t.Errorf("right = %T, want *taskNode", and3.Right)
	}

	// "open task bullet" → and(and(open, task), bullet) — three implicit ANDs
	node4, err := Parse("open task bullet")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	top, ok := node4.(*andNode)
	if !ok {
		t.Fatalf("expected *andNode at root, got %T", node4)
	}
	leftAnd, ok := top.Left.(*andNode)
	if !ok {
		t.Fatalf("left = %T, want *andNode", top.Left)
	}
	if _, ok := leftAnd.Left.(*openNode); !ok {
		t.Errorf("left.left = %T, want *openNode", leftAnd.Left)
	}
	if _, ok := leftAnd.Right.(*taskNode); !ok {
		t.Errorf("left.right = %T, want *taskNode", leftAnd.Right)
	}
	if _, ok := top.Right.(*bulletNode); !ok {
		t.Errorf("right = %T, want *bulletNode", top.Right)
	}
}

// TestCanStartAtomCoversParseAtom verifies that canStartAtom stays in sync
// with parseAtom. Every token type that parseAtom handles must be in
// canStartAtom (plus tokNot for not_expr). If someone adds a new atom type
// and forgets to update canStartAtom, this test fails.
func TestCanStartAtomCoversParseAtom(t *testing.T) {
	// Tokens that should successfully parse as a standalone query.
	// Each entry is a raw input string and the expected canStartAtom result
	// for its first token.
	parseable := []struct {
		input     string
		firstTok  tokenType
		shouldStart bool
	}{
		{"open", tokOpen, true},
		{"completed", tokCompleted, true},
		{"task", tokTask, true},
		{"bullet", tokBullet, true},
		{"@due", tokTag, true},
		{"/pattern/", tokRegex, true},
		{"someword", tokWord, true},
		{`"quoted"`, tokString, true},
		{"(open)", tokLParen, true},
		{"not open", tokNot, true},
	}

	for _, tt := range parseable {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := lex(tt.input)
			if err != nil {
				t.Fatalf("lex(%q) error: %v", tt.input, err)
			}
			got := canStartAtom(tokens[0].Type)
			if got != tt.shouldStart {
				t.Errorf("canStartAtom(%v) = %v, want %v", tokens[0].Type, got, tt.shouldStart)
			}
			// Also verify it actually parses successfully
			_, err = Parse(tt.input)
			if err != nil {
				t.Errorf("Parse(%q) should succeed but got: %v", tt.input, err)
			}
		})
	}

	// Tokens that must NOT start an atom — implicit AND must not trigger on these.
	nonStarting := []struct {
		name string
		tok  tokenType
	}{
		{"and", tokAnd},
		{"or", tokOr},
		{"<", tokLT},
		{">", tokGT},
		{"<=", tokLTE},
		{">=", tokGTE},
		{"=", tokEQ},
		{"date", tokDate},
		{"today", tokToday},
		{"offset", tokOffset},
		{"rparen", tokRParen},
		{"eof", tokEOF},
	}

	for _, tt := range nonStarting {
		t.Run("not_"+tt.name, func(t *testing.T) {
			if canStartAtom(tt.tok) {
				t.Errorf("canStartAtom(%v) = true, want false", tt.tok)
			}
		})
	}
}

func TestParseErrorUnclosedParen(t *testing.T) {
	_, err := Parse("(open and @due")
	if err == nil {
		t.Fatal("expected error for unclosed paren, got nil")
	}
}

func TestParseErrorUnexpectedToken(t *testing.T) {
	_, err := Parse("and")
	if err == nil {
		t.Fatal("expected error for unexpected token, got nil")
	}
}
