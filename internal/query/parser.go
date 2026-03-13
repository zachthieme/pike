package query

import (
	"fmt"
	"regexp"
	"time"
)

// Parse tokenizes the input query and parses it into an AST using recursive descent.
func Parse(input string) (Node, error) {
	tokens, err := Lex(input)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens, pos: 0}
	// Empty input (just EOF) means "no filter" — return nil, nil.
	if p.current().Type == TokEOF {
		return nil, nil
	}
	node, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.current().Type != TokEOF {
		return nil, fmt.Errorf("unexpected token %v at end of input", p.current().Type)
	}
	return node, nil
}

type parser struct {
	tokens []Token
	pos    int
}

func (p *parser) current() Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return Token{Type: TokEOF}
}

func (p *parser) advance() Token {
	tok := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *parser) expect(tt TokenType) (Token, error) {
	tok := p.current()
	if tok.Type != tt {
		return tok, fmt.Errorf("expected %v, got %v (%q)", tt, tok.Type, tok.Value)
	}
	p.advance()
	return tok, nil
}

// expr = or_expr
func (p *parser) parseExpr() (Node, error) {
	return p.parseOrExpr()
}

// or_expr = and_expr ("or" and_expr)*
func (p *parser) parseOrExpr() (Node, error) {
	left, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}
	for p.current().Type == TokOr {
		p.advance() // consume "or"
		right, err := p.parseAndExpr()
		if err != nil {
			return nil, err
		}
		left = &OrNode{Left: left, Right: right}
	}
	return left, nil
}

// and_expr = not_expr ("and" not_expr)*
func (p *parser) parseAndExpr() (Node, error) {
	left, err := p.parseNotExpr()
	if err != nil {
		return nil, err
	}
	for p.current().Type == TokAnd {
		p.advance() // consume "and"
		right, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}
		left = &AndNode{Left: left, Right: right}
	}
	return left, nil
}

// not_expr = "not" not_expr | atom
func (p *parser) parseNotExpr() (Node, error) {
	if p.current().Type == TokNot {
		p.advance() // consume "not"
		expr, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}
		return &NotNode{Expr: expr}, nil
	}
	return p.parseAtom()
}

// atom = "open" | "completed" | tag_or_datecmp | regex | "(" expr ")"
func (p *parser) parseAtom() (Node, error) {
	tok := p.current()

	switch tok.Type {
	case TokOpen:
		p.advance()
		return &OpenNode{}, nil

	case TokCompleted:
		p.advance()
		return &CompletedNode{}, nil

	case TokTag:
		return p.parseTagOrDateCmp()

	case TokRegex:
		p.advance()
		re, err := regexp.Compile(tok.TagName)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", tok.TagName, err)
		}
		return &RegexNode{Pattern: tok.TagName, CompiledRe: re}, nil

	case TokLParen:
		p.advance() // consume "("
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokRParen); err != nil {
			return nil, fmt.Errorf("unclosed parenthesis: %w", err)
		}
		return expr, nil

	default:
		return nil, fmt.Errorf("unexpected token %v (%q)", tok.Type, tok.Value)
	}
}

// parseTagOrDateCmp handles @tag and @tag <op> <date> patterns.
// If the tag is followed by a comparison operator, it becomes a DateCmpNode.
func (p *parser) parseTagOrDateCmp() (Node, error) {
	tagTok := p.advance() // consume the tag token
	tagName := tagTok.TagName

	// Check if next token is a comparison operator
	next := p.current()
	if next.Type == TokLT || next.Type == TokGT || next.Type == TokLTE || next.Type == TokGTE {
		// Only @due and @completed support date comparisons
		if tagName != "due" && tagName != "completed" {
			return nil, fmt.Errorf("unsupported date field @%s; only @due and @completed support date comparisons", tagName)
		}
		opTok := p.advance() // consume the operator
		op := opTok.Value

		// Parse the date value: today, today+Nd, today-Nd, or YYYY-MM-DD
		dateTok := p.current()
		switch dateTok.Type {
		case TokToday:
			p.advance()
			return &DateCmpNode{Field: tagName, Op: op, Days: 0}, nil
		case TokOffset:
			p.advance()
			return &DateCmpNode{Field: tagName, Op: op, Days: dateTok.Offset}, nil
		case TokDate:
			p.advance()
			t, err := time.Parse("2006-01-02", dateTok.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid date %q: %w", dateTok.Value, err)
			}
			return &DateCmpNode{Field: tagName, Op: op, Literal: &t}, nil
		default:
			return nil, fmt.Errorf("expected date value after %q, got %v (%q)", op, dateTok.Type, dateTok.Value)
		}
	}

	// Plain tag match
	return &TagNode{Name: tagName}, nil
}
