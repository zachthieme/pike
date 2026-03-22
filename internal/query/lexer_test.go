package query

import (
	"strings"
	"testing"
)

func TestLexKeywords(t *testing.T) {
	tests := []struct {
		input    string
		wantType tokenType
	}{
		{"open", tokOpen},
		{"completed", tokCompleted},
		{"and", tokAnd},
		{"or", tokOr},
		{"not", tokNot},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := lex(tt.input)
			if err != nil {
				t.Fatalf("lex(%q) error: %v", tt.input, err)
			}
			if len(tokens) != 2 { // keyword + EOF
				t.Fatalf("expected 2 tokens, got %d", len(tokens))
			}
			if tokens[0].Type != tt.wantType {
				t.Errorf("token type = %v, want %v", tokens[0].Type, tt.wantType)
			}
			if tokens[0].Value != tt.input {
				t.Errorf("token value = %q, want %q", tokens[0].Value, tt.input)
			}
			if tokens[1].Type != tokEOF {
				t.Errorf("last token type = %v, want tokEOF", tokens[1].Type)
			}
		})
	}
}

func TestLexTags(t *testing.T) {
	tests := []struct {
		input   string
		wantTag string
	}{
		{"@due", "due"},
		{"@today", "today"},
		{"@risk", "risk"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := lex(tt.input)
			if err != nil {
				t.Fatalf("lex(%q) error: %v", tt.input, err)
			}
			if len(tokens) != 2 {
				t.Fatalf("expected 2 tokens, got %d", len(tokens))
			}
			if tokens[0].Type != tokTag {
				t.Errorf("token type = %v, want tokTag", tokens[0].Type)
			}
			if tokens[0].TagName != tt.wantTag {
				t.Errorf("tag name = %q, want %q", tokens[0].TagName, tt.wantTag)
			}
		})
	}
}

func TestLexComparisons(t *testing.T) {
	tests := []struct {
		input    string
		wantType tokenType
	}{
		{"<", tokLT},
		{">", tokGT},
		{"<=", tokLTE},
		{">=", tokGTE},
		{"=", tokEQ},
		{"==", tokEQ},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := lex(tt.input)
			if err != nil {
				t.Fatalf("lex(%q) error: %v", tt.input, err)
			}
			if len(tokens) != 2 {
				t.Fatalf("expected 2 tokens, got %d", len(tokens))
			}
			if tokens[0].Type != tt.wantType {
				t.Errorf("token type = %v, want %v", tokens[0].Type, tt.wantType)
			}
		})
	}

	// Ensure <= is lexed as one token, not < then =
	tokens, err := lex("<=")
	if err != nil {
		t.Fatalf("lex(\"<=\") error: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens for '<=', got %d", len(tokens))
	}
	if tokens[0].Type != tokLTE {
		t.Errorf("'<=' should lex as tokLTE, got %v", tokens[0].Type)
	}

	tokens, err = lex(">=")
	if err != nil {
		t.Fatalf("lex(\">=\") error: %v", err)
	}
	if tokens[0].Type != tokGTE {
		t.Errorf("'>=' should lex as tokGTE, got %v", tokens[0].Type)
	}
}

func TestLexDates(t *testing.T) {
	tokens, err := lex("2026-03-15")
	if err != nil {
		t.Fatalf("lex error: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].Type != tokDate {
		t.Errorf("token type = %v, want tokDate", tokens[0].Type)
	}
	if tokens[0].Value != "2026-03-15" {
		t.Errorf("token value = %q, want %q", tokens[0].Value, "2026-03-15")
	}
}

func TestLexOffsets(t *testing.T) {
	tests := []struct {
		input      string
		wantOffset int
	}{
		{"today+3d", 3},
		{"today-14d", -14},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := lex(tt.input)
			if err != nil {
				t.Fatalf("lex(%q) error: %v", tt.input, err)
			}
			if len(tokens) != 2 {
				t.Fatalf("expected 2 tokens, got %d", len(tokens))
			}
			if tokens[0].Type != tokOffset {
				t.Errorf("token type = %v, want tokOffset", tokens[0].Type)
			}
			if tokens[0].Offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", tokens[0].Offset, tt.wantOffset)
			}
		})
	}
}

func TestLexRegex(t *testing.T) {
	tokens, err := lex("/meeting/")
	if err != nil {
		t.Fatalf("lex error: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].Type != tokRegex {
		t.Errorf("token type = %v, want tokRegex", tokens[0].Type)
	}
	if tokens[0].TagName != "meeting" {
		t.Errorf("regex pattern = %q, want %q", tokens[0].TagName, "meeting")
	}
}

func TestLexParens(t *testing.T) {
	tokens, err := lex("(open)")
	if err != nil {
		t.Fatalf("lex error: %v", err)
	}
	if len(tokens) != 4 { // ( open ) EOF
		t.Fatalf("expected 4 tokens, got %d", len(tokens))
	}
	if tokens[0].Type != tokLParen {
		t.Errorf("token[0] type = %v, want tokLParen", tokens[0].Type)
	}
	if tokens[1].Type != tokOpen {
		t.Errorf("token[1] type = %v, want tokOpen", tokens[1].Type)
	}
	if tokens[2].Type != tokRParen {
		t.Errorf("token[2] type = %v, want tokRParen", tokens[2].Type)
	}
}

func TestLexComplex(t *testing.T) {
	tokens, err := lex("open and @due < today")
	if err != nil {
		t.Fatalf("lex error: %v", err)
	}
	// open, and, @due, <, today, EOF = 6
	if len(tokens) != 6 {
		t.Fatalf("expected 6 tokens, got %d: %v", len(tokens), tokens)
	}
	expected := []tokenType{tokOpen, tokAnd, tokTag, tokLT, tokToday, tokEOF}
	for i, want := range expected {
		if tokens[i].Type != want {
			t.Errorf("token[%d] type = %v, want %v", i, tokens[i].Type, want)
		}
	}
}

func TestLexErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantContain string
	}{
		{"unterminated string", `"hello`, `missing closing '"'`},
		{"unterminated regex", "/unterminated", "missing closing '/'"},
		{"offset missing d suffix", "today+3x", `expected 'd' suffix`},
		{"offset missing number", "today+d", "expected number after"},
		{"invalid date", "2026-1-1", "expected YYYY-MM-DD"},
		{"unexpected character", "~", "unexpected character"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := lex(tt.input)
			if err == nil {
				t.Fatalf("lex(%q): expected error, got nil", tt.input)
			}
			if !strings.Contains(err.Error(), tt.wantContain) {
				t.Errorf("lex(%q) error = %q, want substring %q", tt.input, err.Error(), tt.wantContain)
			}
		})
	}
}

func TestLexTomorrowYesterday(t *testing.T) {
	tests := []struct {
		input      string
		wantType   tokenType
		wantOffset int
	}{
		{"tomorrow", tokOffset, 1},
		{"yesterday", tokOffset, -1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := lex(tt.input)
			if err != nil {
				t.Fatalf("lex(%q) error: %v", tt.input, err)
			}
			if len(tokens) != 2 {
				t.Fatalf("expected 2 tokens, got %d", len(tokens))
			}
			if tokens[0].Type != tt.wantType {
				t.Errorf("token type = %v, want %v", tokens[0].Type, tt.wantType)
			}
			if tokens[0].Offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", tokens[0].Offset, tt.wantOffset)
			}
		})
	}
}

func TestLexToday(t *testing.T) {
	tokens, err := lex("today")
	if err != nil {
		t.Fatalf("lex error: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].Type != tokToday {
		t.Errorf("token type = %v, want tokToday", tokens[0].Type)
	}
}
