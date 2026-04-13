package query

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Parse tokenizes the input query and parses it into an AST using recursive descent.
func Parse(input string) (Node, error) {
	tokens, err := lex(input)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens, pos: 0}
	// Empty input (just EOF) means "no filter" — return nil, nil.
	if p.current().Type == tokEOF {
		return nil, nil
	}
	node, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.current().Type != tokEOF {
		return nil, fmt.Errorf("unexpected token %v at end of input", p.current().Type)
	}
	return node, nil
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) current() token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return token{Type: tokEOF}
}

func (p *parser) advance() token {
	tok := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *parser) expect(tt tokenType) (token, error) {
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
	for p.current().Type == tokOr {
		p.advance() // consume "or"
		right, err := p.parseAndExpr()
		if err != nil {
			return nil, err
		}
		left = &orNode{Left: left, Right: right}
	}
	return left, nil
}

// canStartAtom reports whether a token type can begin a new atom or not_expr.
// Used by parseAndExpr to support implicit AND (juxtaposition).
func canStartAtom(t tokenType) bool {
	switch t {
	case tokOpen, tokCompleted, tokTask, tokBullet, tokTag, tokRegex, tokWord, tokString, tokLParen, tokNot:
		return true
	}
	return false
}

// and_expr = not_expr (("and" | implicit) not_expr)*
func (p *parser) parseAndExpr() (Node, error) {
	left, err := p.parseNotExpr()
	if err != nil {
		return nil, err
	}
	for {
		if p.current().Type == tokAnd {
			p.advance() // consume explicit "and"
		} else if !canStartAtom(p.current().Type) {
			break
		}
		right, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}
		left = &andNode{Left: left, Right: right}
	}
	return left, nil
}

// not_expr = "not" not_expr | atom
func (p *parser) parseNotExpr() (Node, error) {
	if p.current().Type == tokNot {
		p.advance() // consume "not"
		expr, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}
		return &notNode{Expr: expr}, nil
	}
	return p.parseAtom()
}

// atom = "open" | "completed" | "task" | "bullet" | tag_or_datecmp | regex | "(" expr ")"
func (p *parser) parseAtom() (Node, error) {
	tok := p.current()

	switch tok.Type {
	case tokOpen:
		p.advance()
		return &openNode{}, nil

	case tokCompleted:
		p.advance()
		return &completedNode{}, nil

	case tokTask:
		p.advance()
		return &taskNode{}, nil

	case tokBullet:
		p.advance()
		return &bulletNode{}, nil

	case tokTag:
		return p.parseTagOrDateCmp()

	case tokRegex:
		p.advance()
		re, err := regexp.Compile(tok.TagName)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", tok.TagName, err)
		}
		return &regexNode{Pattern: tok.TagName, CompiledRe: re}, nil

	case tokWord, tokString:
		p.advance()
		return &textNode{Pattern: tok.Value, LowerPattern: strings.ToLower(tok.Value)}, nil

	case tokLParen:
		p.advance() // consume "("
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokRParen); err != nil {
			return nil, fmt.Errorf("unclosed parenthesis: %w", err)
		}
		return expr, nil

	default:
		return nil, fmt.Errorf("unexpected token %v (%q)", tok.Type, tok.Value)
	}
}

// parseTagOrDateCmp handles @tag and @tag <op> <date> patterns.
// If the tag is followed by a comparison operator, it becomes a dateCmpNode.
func (p *parser) parseTagOrDateCmp() (Node, error) {
	tagTok := p.advance() // consume the tag token
	tagName := tagTok.TagName

	// Check if next token is a comparison operator
	next := p.current()
	if next.Type == tokLT || next.Type == tokGT || next.Type == tokLTE || next.Type == tokGTE || next.Type == tokEQ {
		// Only @due and @completed support date comparisons
		if tagName != "due" && tagName != "completed" {
			return nil, fmt.Errorf("unsupported date field @%s; only @due and @completed support date comparisons", tagName)
		}
		opTok := p.advance() // consume the operator
		op := opTok.Value
		if op == "==" {
			op = "=" // normalize equality operator
		}

		// Parse the date value: today, today+Nd, today-Nd, or YYYY-MM-DD
		dateTok := p.current()
		switch dateTok.Type {
		case tokToday:
			p.advance()
			return &dateCmpNode{Field: tagName, Op: op, Days: 0}, nil
		case tokOffset:
			p.advance()
			return &dateCmpNode{Field: tagName, Op: op, Days: dateTok.Offset}, nil
		case tokDate:
			p.advance()
			t, err := time.Parse("2006-01-02", dateTok.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid date %q: %w", dateTok.Value, err)
			}
			return &dateCmpNode{Field: tagName, Op: op, Literal: &t}, nil
		default:
			return nil, fmt.Errorf("expected date value after %q, got %v (%q)", op, dateTok.Type, dateTok.Value)
		}
	}

	// Plain tag match
	return &tagNode{Name: tagName}, nil
}
