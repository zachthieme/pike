package query

import (
	"fmt"
	"strconv"
	"unicode"
)

type TokenType int

const (
	TokOpen      TokenType = iota // "open"
	TokCompleted                  // "completed"
	TokAnd                        // "and"
	TokOr                         // "or"
	TokNot                        // "not"
	TokTag                        // @word
	TokLT                         // <
	TokGT                         // >
	TokLTE                        // <=
	TokGTE                        // >=
	TokDate                       // YYYY-MM-DD
	TokToday                      // "today" (standalone, not part of offset)
	TokOffset                     // today+3d or today-7d
	TokRegex                      // /pattern/
	TokLParen                     // (
	TokRParen                     // )
	TokEOF
)

func (t TokenType) String() string {
	switch t {
	case TokOpen:
		return "TokOpen"
	case TokCompleted:
		return "TokCompleted"
	case TokAnd:
		return "TokAnd"
	case TokOr:
		return "TokOr"
	case TokNot:
		return "TokNot"
	case TokTag:
		return "TokTag"
	case TokLT:
		return "TokLT"
	case TokGT:
		return "TokGT"
	case TokLTE:
		return "TokLTE"
	case TokGTE:
		return "TokGTE"
	case TokDate:
		return "TokDate"
	case TokToday:
		return "TokToday"
	case TokOffset:
		return "TokOffset"
	case TokRegex:
		return "TokRegex"
	case TokLParen:
		return "TokLParen"
	case TokRParen:
		return "TokRParen"
	case TokEOF:
		return "TokEOF"
	default:
		return fmt.Sprintf("TokUnknown(%d)", t)
	}
}

type Token struct {
	Type    TokenType
	Value   string // raw text of the token
	TagName string // for TokTag: the tag name without @
	Offset  int    // for TokOffset: days offset (positive or negative)
}

// Lex tokenizes the input query string into a slice of tokens.
func Lex(input string) ([]Token, error) {
	var tokens []Token
	runes := []rune(input)
	i := 0

	for i < len(runes) {
		ch := runes[i]

		// Skip whitespace
		if unicode.IsSpace(ch) {
			i++
			continue
		}

		// Parentheses
		if ch == '(' {
			tokens = append(tokens, Token{Type: TokLParen, Value: "("})
			i++
			continue
		}
		if ch == ')' {
			tokens = append(tokens, Token{Type: TokRParen, Value: ")"})
			i++
			continue
		}

		// Comparison operators: <=, >=, <, >
		if ch == '<' {
			if i+1 < len(runes) && runes[i+1] == '=' {
				tokens = append(tokens, Token{Type: TokLTE, Value: "<="})
				i += 2
			} else {
				tokens = append(tokens, Token{Type: TokLT, Value: "<"})
				i++
			}
			continue
		}
		if ch == '>' {
			if i+1 < len(runes) && runes[i+1] == '=' {
				tokens = append(tokens, Token{Type: TokGTE, Value: ">="})
				i += 2
			} else {
				tokens = append(tokens, Token{Type: TokGT, Value: ">"})
				i++
			}
			continue
		}

		// Tag: @word
		if ch == '@' {
			i++ // skip @
			start := i
			for i < len(runes) && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			if i == start {
				return nil, fmt.Errorf("expected tag name after '@' at position %d", start)
			}
			name := string(runes[start:i])
			tokens = append(tokens, Token{Type: TokTag, Value: "@" + name, TagName: name})
			continue
		}

		// Regex: /pattern/ (supports \/ to escape a literal /)
		if ch == '/' {
			i++ // skip opening /
			start := i
			var pattern []rune
			for i < len(runes) && runes[i] != '/' {
				if runes[i] == '\\' && i+1 < len(runes) && runes[i+1] == '/' {
					pattern = append(pattern, '/')
					i += 2
				} else {
					pattern = append(pattern, runes[i])
					i++
				}
			}
			if i >= len(runes) {
				return nil, fmt.Errorf("unterminated regex starting at position %d", start-1)
			}
			patternStr := string(pattern)
			raw := string(runes[start:i])
			i++ // skip closing /
			tokens = append(tokens, Token{Type: TokRegex, Value: "/" + raw + "/", TagName: patternStr})
			continue
		}

		// Words (keywords) or date literals
		if unicode.IsLetter(ch) {
			start := i
			for i < len(runes) && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			word := string(runes[start:i])

			switch word {
			case "open":
				tokens = append(tokens, Token{Type: TokOpen, Value: word})
			case "completed":
				tokens = append(tokens, Token{Type: TokCompleted, Value: word})
			case "and":
				tokens = append(tokens, Token{Type: TokAnd, Value: word})
			case "or":
				tokens = append(tokens, Token{Type: TokOr, Value: word})
			case "not":
				tokens = append(tokens, Token{Type: TokNot, Value: word})
			case "today":
				// Check for today+Nd or today-Nd
				if i < len(runes) && (runes[i] == '+' || runes[i] == '-') {
					sign := runes[i]
					i++ // skip + or -
					numStart := i
					for i < len(runes) && unicode.IsDigit(runes[i]) {
						i++
					}
					if i == numStart {
						return nil, fmt.Errorf("expected number after 'today%c' at position %d", sign, numStart)
					}
					if i >= len(runes) || runes[i] != 'd' {
						return nil, fmt.Errorf("expected 'd' suffix in offset at position %d", i)
					}
					numStr := string(runes[numStart:i])
					i++ // skip 'd'
					days, err := strconv.Atoi(numStr)
					if err != nil {
						return nil, fmt.Errorf("invalid number in offset: %s", numStr)
					}
					if sign == '-' {
						days = -days
					}
					fullValue := string(runes[start:i])
					tokens = append(tokens, Token{Type: TokOffset, Value: fullValue, Offset: days})
				} else {
					tokens = append(tokens, Token{Type: TokToday, Value: word})
				}
			default:
				return nil, fmt.Errorf("unexpected word %q at position %d", word, start)
			}
			continue
		}

		// Date literal: YYYY-MM-DD (starts with digit)
		if unicode.IsDigit(ch) {
			start := i
			// Read YYYY-MM-DD: exactly 4 digits, dash, 2 digits, dash, 2 digits
			for i < len(runes) && (unicode.IsDigit(runes[i]) || runes[i] == '-') {
				i++
			}
			dateStr := string(runes[start:i])
			// Validate format
			if len(dateStr) != 10 || dateStr[4] != '-' || dateStr[7] != '-' {
				return nil, fmt.Errorf("invalid date literal %q at position %d", dateStr, start)
			}
			tokens = append(tokens, Token{Type: TokDate, Value: dateStr})
			continue
		}

		return nil, fmt.Errorf("unexpected character %q at position %d", ch, i)
	}

	tokens = append(tokens, Token{Type: TokEOF, Value: ""})
	return tokens, nil
}
