package query

import (
	"testing"
)

func TestLexKeywords(t *testing.T) {
	tests := []struct {
		input    string
		wantType TokenType
	}{
		{"open", TokOpen},
		{"completed", TokCompleted},
		{"and", TokAnd},
		{"or", TokOr},
		{"not", TokNot},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := Lex(tt.input)
			if err != nil {
				t.Fatalf("Lex(%q) error: %v", tt.input, err)
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
			if tokens[1].Type != TokEOF {
				t.Errorf("last token type = %v, want TokEOF", tokens[1].Type)
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
			tokens, err := Lex(tt.input)
			if err != nil {
				t.Fatalf("Lex(%q) error: %v", tt.input, err)
			}
			if len(tokens) != 2 {
				t.Fatalf("expected 2 tokens, got %d", len(tokens))
			}
			if tokens[0].Type != TokTag {
				t.Errorf("token type = %v, want TokTag", tokens[0].Type)
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
		wantType TokenType
	}{
		{"<", TokLT},
		{">", TokGT},
		{"<=", TokLTE},
		{">=", TokGTE},
		{"=", TokEQ},
		{"==", TokEQ},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := Lex(tt.input)
			if err != nil {
				t.Fatalf("Lex(%q) error: %v", tt.input, err)
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
	tokens, err := Lex("<=")
	if err != nil {
		t.Fatalf("Lex(\"<=\") error: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens for '<=', got %d", len(tokens))
	}
	if tokens[0].Type != TokLTE {
		t.Errorf("'<=' should lex as TokLTE, got %v", tokens[0].Type)
	}

	tokens, err = Lex(">=")
	if err != nil {
		t.Fatalf("Lex(\">=\") error: %v", err)
	}
	if tokens[0].Type != TokGTE {
		t.Errorf("'>=' should lex as TokGTE, got %v", tokens[0].Type)
	}
}

func TestLexDates(t *testing.T) {
	tokens, err := Lex("2026-03-15")
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].Type != TokDate {
		t.Errorf("token type = %v, want TokDate", tokens[0].Type)
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
			tokens, err := Lex(tt.input)
			if err != nil {
				t.Fatalf("Lex(%q) error: %v", tt.input, err)
			}
			if len(tokens) != 2 {
				t.Fatalf("expected 2 tokens, got %d", len(tokens))
			}
			if tokens[0].Type != TokOffset {
				t.Errorf("token type = %v, want TokOffset", tokens[0].Type)
			}
			if tokens[0].Offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", tokens[0].Offset, tt.wantOffset)
			}
		})
	}
}

func TestLexRegex(t *testing.T) {
	tokens, err := Lex("/meeting/")
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].Type != TokRegex {
		t.Errorf("token type = %v, want TokRegex", tokens[0].Type)
	}
	if tokens[0].TagName != "meeting" {
		t.Errorf("regex pattern = %q, want %q", tokens[0].TagName, "meeting")
	}
}

func TestLexParens(t *testing.T) {
	tokens, err := Lex("(open)")
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}
	if len(tokens) != 4 { // ( open ) EOF
		t.Fatalf("expected 4 tokens, got %d", len(tokens))
	}
	if tokens[0].Type != TokLParen {
		t.Errorf("token[0] type = %v, want TokLParen", tokens[0].Type)
	}
	if tokens[1].Type != TokOpen {
		t.Errorf("token[1] type = %v, want TokOpen", tokens[1].Type)
	}
	if tokens[2].Type != TokRParen {
		t.Errorf("token[2] type = %v, want TokRParen", tokens[2].Type)
	}
}

func TestLexComplex(t *testing.T) {
	tokens, err := Lex("open and @due < today")
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}
	// open, and, @due, <, today, EOF = 6
	if len(tokens) != 6 {
		t.Fatalf("expected 6 tokens, got %d: %v", len(tokens), tokens)
	}
	expected := []TokenType{TokOpen, TokAnd, TokTag, TokLT, TokToday, TokEOF}
	for i, want := range expected {
		if tokens[i].Type != want {
			t.Errorf("token[%d] type = %v, want %v", i, tokens[i].Type, want)
		}
	}
}

func TestLexErrorUnterminatedRegex(t *testing.T) {
	_, err := Lex("/unterminated")
	if err == nil {
		t.Fatal("expected error for unterminated regex, got nil")
	}
}

func TestLexToday(t *testing.T) {
	tokens, err := Lex("today")
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].Type != TokToday {
		t.Errorf("token type = %v, want TokToday", tokens[0].Type)
	}
}
