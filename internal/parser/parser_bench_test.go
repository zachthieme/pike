package parser

import "testing"

var benchLines = []struct {
	name string
	line string
}{
	{"checkbox_simple", "- [ ] simple task"},
	{"checkbox_tags", "- [ ] deploy API @due(2026-03-20) @risk @today"},
	{"completed", "- [x] done thing @completed(2026-03-15)"},
	{"plain_bullet_tag", "- standup notes @weekly"},
	{"no_match", "This is just a paragraph of text."},
	{"nested_indent", "      - [ ] deeply nested task @due(2026-04-01)"},
}

func BenchmarkParseLine(b *testing.B) {
	for _, tt := range benchLines {
		b.Run(tt.name, func(b *testing.B) {
			for b.Loop() {
				ParseLine(tt.line, "bench.md", 1)
			}
		})
	}
}
