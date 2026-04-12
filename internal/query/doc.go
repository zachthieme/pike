// Package query implements a DSL for filtering tasks by state, tags, dates,
// and text patterns.
//
// # Grammar
//
// The query language uses recursive descent parsing with the following grammar:
//
//	expr       = or_expr
//	or_expr    = and_expr ("or" and_expr)*
//	and_expr   = not_expr ("and" not_expr)*
//	not_expr   = "not" not_expr | atom
//	atom       = "open" | "completed"
//	           | "@" tag
//	           | date_cmp
//	           | text_match
//	           | "(" expr ")"
//	date_cmp   = ("@due" | "@completed") op date_val
//	op         = "<" | ">" | "<=" | ">=" | "="
//	date_val   = literal_date | "today" | "today" ("+" | "-") digits "d"
//	           | "tomorrow" | "yesterday"
//	text_match = quoted_string | "/" regex "/" | bare_word
//
// For the full reference including all operators, date expressions, sort
// orders, and usage examples, see docs/query-dsl.md.
//
// # Evaluation
//
// [Parse] tokenizes the input and builds an AST of [Node] values.
// [Eval] walks the AST against a [model.Task], returning true if the task
// matches. Logical operators (and, or) short-circuit. Date comparisons
// normalize both sides to midnight UTC.
//
// [EvalWithOptions] supports partial tag matching for live filter-as-you-type,
// where "@pri" matches "@priority".
package query
