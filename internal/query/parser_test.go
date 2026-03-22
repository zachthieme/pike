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
