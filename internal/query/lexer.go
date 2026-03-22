package query

import (
	"fmt"
	"strconv"
	"unicode"
)

type tokenType int

const (
	tokOpen      tokenType = iota // "open"
	tokCompleted                  // "completed"
	tokAnd                        // "and"
	tokOr                         // "or"
	tokNot                        // "not"
	tokTag                        // @word
	tokLT                         // <
	tokGT                         // >
	tokLTE                        // <=
	tokGTE                        // >=
	tokEQ                         // = or ==
	tokDate                       // YYYY-MM-DD
	tokToday                      // "today" (standalone, not part of offset)
	tokOffset                     // today+3d or today-7d
	tokRegex                      // /pattern/
	tokString                     // "quoted text"
	tokWord                       // bare word (not a keyword)
	tokLParen                     // (
	tokRParen                     // )
	tokEOF
)

func (t tokenType) String() string {
	switch t {
	case tokOpen:
		return "tokOpen"
	case tokCompleted:
		return "tokCompleted"
	case tokAnd:
		return "tokAnd"
	case tokOr:
		return "tokOr"
	case tokNot:
		return "tokNot"
	case tokTag:
		return "tokTag"
	case tokLT:
		return "tokLT"
	case tokGT:
		return "tokGT"
	case tokLTE:
		return "tokLTE"
	case tokGTE:
		return "tokGTE"
	case tokEQ:
		return "tokEQ"
	case tokDate:
		return "tokDate"
	case tokToday:
		return "tokToday"
	case tokOffset:
		return "tokOffset"
	case tokRegex:
		return "tokRegex"
	case tokString:
		return "tokString"
	case tokWord:
		return "tokWord"
	case tokLParen:
		return "tokLParen"
	case tokRParen:
		return "tokRParen"
	case tokEOF:
		return "tokEOF"
	default:
		return fmt.Sprintf("TokUnknown(%d)", t)
	}
}

type token struct {
	Type    tokenType
	Value   string // raw text of the token
	TagName string // for tokTag: the tag name without @
	Offset  int    // for tokOffset: days offset (positive or negative)
}

// lex tokenizes the input query string into a slice of tokens.
func lex(input string) ([]token, error) {
	var tokens []token
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
			tokens = append(tokens, token{Type: tokLParen, Value: "("})
			i++
			continue
		}
		if ch == ')' {
			tokens = append(tokens, token{Type: tokRParen, Value: ")"})
			i++
			continue
		}

		// Comparison operators: <=, >=, <, >, =, ==
		if ch == '<' {
			if i+1 < len(runes) && runes[i+1] == '=' {
				tokens = append(tokens, token{Type: tokLTE, Value: "<="})
				i += 2
			} else {
				tokens = append(tokens, token{Type: tokLT, Value: "<"})
				i++
			}
			continue
		}
		if ch == '>' {
			if i+1 < len(runes) && runes[i+1] == '=' {
				tokens = append(tokens, token{Type: tokGTE, Value: ">="})
				i += 2
			} else {
				tokens = append(tokens, token{Type: tokGT, Value: ">"})
				i++
			}
			continue
		}
		if ch == '=' {
			if i+1 < len(runes) && runes[i+1] == '=' {
				tokens = append(tokens, token{Type: tokEQ, Value: "=="})
				i += 2
			} else {
				tokens = append(tokens, token{Type: tokEQ, Value: "="})
				i++
			}
			continue
		}

		// Quoted string: "text"
		if ch == '"' {
			i++ // skip opening quote
			start := i
			for i < len(runes) && runes[i] != '"' {
				i++
			}
			if i >= len(runes) {
				return nil, fmt.Errorf("unterminated string at position %d: missing closing '\"'", start-1)
			}
			text := string(runes[start:i])
			i++ // skip closing quote
			tokens = append(tokens, token{Type: tokString, Value: text})
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
			tokens = append(tokens, token{Type: tokTag, Value: "@" + name, TagName: name})
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
				return nil, fmt.Errorf("unterminated regex at position %d: missing closing '/'", start-1)
			}
			patternStr := string(pattern)
			raw := string(runes[start:i])
			i++ // skip closing /
			tokens = append(tokens, token{Type: tokRegex, Value: "/" + raw + "/", TagName: patternStr})
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
				tokens = append(tokens, token{Type: tokOpen, Value: word})
			case "completed":
				tokens = append(tokens, token{Type: tokCompleted, Value: word})
			case "and":
				tokens = append(tokens, token{Type: tokAnd, Value: word})
			case "or":
				tokens = append(tokens, token{Type: tokOr, Value: word})
			case "not":
				tokens = append(tokens, token{Type: tokNot, Value: word})
			case "tomorrow":
				tokens = append(tokens, token{Type: tokOffset, Value: "tomorrow", Offset: 1})
			case "yesterday":
				tokens = append(tokens, token{Type: tokOffset, Value: "yesterday", Offset: -1})
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
					if i >= len(runes) {
						return nil, fmt.Errorf("expected 'd' suffix in offset at position %d (e.g. today+3d)", i)
					}
					if runes[i] != 'd' {
						return nil, fmt.Errorf("expected 'd' suffix in offset at position %d, got %q (e.g. today+3d)", i, string(runes[i]))
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
					tokens = append(tokens, token{Type: tokOffset, Value: fullValue, Offset: days})
				} else {
					tokens = append(tokens, token{Type: tokToday, Value: word})
				}
			default:
				tokens = append(tokens, token{Type: tokWord, Value: word})
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
				return nil, fmt.Errorf("invalid date literal %q at position %d (expected YYYY-MM-DD)", dateStr, start)
			}
			tokens = append(tokens, token{Type: tokDate, Value: dateStr})
			continue
		}

		return nil, fmt.Errorf("unexpected character %q at position %d", ch, i)
	}

	tokens = append(tokens, token{Type: tokEOF, Value: ""})
	return tokens, nil
}
