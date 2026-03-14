package style

import (
	"testing"

	"pike/internal/model"
)

func TestTagToken(t *testing.T) {
	tests := []struct {
		name string
		tag  model.Tag
		want string
	}{
		{"bare tag", model.Tag{Name: "today"}, "@today"},
		{"valued tag", model.Tag{Name: "due", Value: "2026-03-15"}, "@due(2026-03-15)"},
		{"empty value treated as bare", model.Tag{Name: "risk", Value: ""}, "@risk"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TagToken(tt.tag); got != tt.want {
				t.Errorf("TagToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no ANSI", "hello world", "hello world"},
		{"simple color", "\033[31mred\033[0m", "red"},
		{"24-bit color", "\033[38;2;255;87;51mhex\033[0m", "hex"},
		{"mixed", "before \033[32mgreen\033[0m after", "before green after"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripANSI(tt.input); got != tt.want {
				t.Errorf("StripANSI() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestANSIStyleFunc(t *testing.T) {
	sf := ANSIStyleFunc()
	tests := []struct {
		name  string
		text  string
		color string
		want  string
	}{
		{"named color", "@today", "green", "\033[32m@today\033[0m"},
		{"hex color", "@special", "#FF5733", "\033[38;2;255;87;51m@special\033[0m"},
		{"unknown color returns text unchanged", "@tag", "nope", "@tag"},
		{"empty color returns text unchanged", "@tag", "", "@tag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sf(tt.text, tt.color); got != tt.want {
				t.Errorf("ANSIStyleFunc() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestColorizeTags(t *testing.T) {
	sf := ANSIStyleFunc()
	green := "\033[32m"
	red := "\033[31m"
	cyan := "\033[36m"
	reset := "\033[0m"

	tagColors := map[string]string{
		"today":    "green",
		"risk":     "red",
		"due":      "red",
		"_default": "cyan",
	}

	tests := []struct {
		name      string
		text      string
		tags      []model.Tag
		tagColors map[string]string
		want      string
	}{
		{
			name:      "bare tag",
			text:      "Buy groceries @today",
			tags:      []model.Tag{{Name: "today"}},
			tagColors: tagColors,
			want:      "Buy groceries " + green + "@today" + reset,
		},
		{
			name:      "valued tag uses three-part render",
			text:      "Submit report @due(2026-03-15)",
			tags:      []model.Tag{{Name: "due", Value: "2026-03-15"}},
			tagColors: tagColors,
			want:      "Submit report " + red + "@due(" + reset + red + "2026-03-15" + reset + red + ")" + reset,
		},
		{
			name:      "multiple tags",
			text:      "Deploy service @risk @today",
			tags:      []model.Tag{{Name: "risk"}, {Name: "today"}},
			tagColors: tagColors,
			want:      "Deploy service " + red + "@risk" + reset + " " + green + "@today" + reset,
		},
		{
			name:      "unknown tag uses _default",
			text:      "Research @someothertag",
			tags:      []model.Tag{{Name: "someothertag"}},
			tagColors: tagColors,
			want:      "Research " + cyan + "@someothertag" + reset,
		},
		{
			name:      "no matching color skips tag",
			text:      "Task @unknown",
			tags:      []model.Tag{{Name: "unknown"}},
			tagColors: map[string]string{},
			want:      "Task @unknown",
		},
		{
			name:      "duplicate tag tokens deduplicated",
			text:      "Task @today @today",
			tags:      []model.Tag{{Name: "today"}, {Name: "today"}},
			tagColors: tagColors,
			want:      "Task " + green + "@today" + reset + " @today",
		},
		{
			name:      "longer token replaced before shorter",
			text:      "Task @due(2026-03-15) and also @due sometime",
			tags:      []model.Tag{{Name: "due", Value: "2026-03-15"}, {Name: "due"}},
			tagColors: tagColors,
			want:      "Task " + red + "@due(" + reset + red + "2026-03-15" + reset + red + ")" + reset + " and also " + red + "@due" + reset + " sometime",
		},
		{
			name:      "nil tagColors returns text unchanged",
			text:      "Task @today",
			tags:      []model.Tag{{Name: "today"}},
			tagColors: nil,
			want:      "Task @today",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ColorizeTags(tt.text, tt.tags, tt.tagColors, sf)
			if got != tt.want {
				t.Errorf("ColorizeTags() =\n  %q\nwant:\n  %q", got, tt.want)
			}
		})
	}
}

func TestPrettifyText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"wiki link with display name", "talk to [[zach-thieme|Zach Thieme]] about it", "talk to Zach Thieme about it"},
		{"wiki link without display name", "talk to [[Zach Thieme]] about it", "talk to Zach Thieme about it"},
		{"wiki link slug gets prettified", "see [[jeff-roache]] for details", "see Jeff Roache for details"},
		{"markdown link shows text only", "check [the docs](https://example.com/docs/guide) first", "check the docs first"},
		{"bare URL extracts document name", "see https://example.com/docs/migration-plan for details", "see migration-plan for details"},
		{"bare URL with just host", "visit https://example.com/", "visit example.com"},
		{"bare URL with numeric path includes parent", "fix https://github.com/org/repo/pull/123", "fix pull/123"},
		{"bare URL strips .html extension", "read https://docs.example.com/guide/setup.html", "read setup"},
		{"multiple wiki links", "[[alice-bob|Alice Bob]] and [[charlie-delta|Charlie Delta]]", "Alice Bob and Charlie Delta"},
		{"mixed wiki link and bare URL", "ask [[zach-thieme|Zach Thieme]] about https://example.com/docs/auth-flow", "ask Zach Thieme about auth-flow"},
		{"no links unchanged", "just a plain task @today", "just a plain task @today"},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrettifyText(tt.input); got != tt.want {
				t.Errorf("PrettifyText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPrettifyLinks(t *testing.T) {
	render := func(s string) string { return "[" + s + "]" }
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"wiki link styled", "see [[zach-thieme]] here", "see [Zach Thieme] here"},
		{"markdown link styled", "check [docs](https://example.com) now", "check [docs] now"},
		{"bare URL styled", "visit https://example.com/path/page", "visit [page]"},
		{"no links unchanged", "plain text", "plain text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrettifyLinks(tt.input, render); got != tt.want {
				t.Errorf("PrettifyLinks() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShortenURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com/", "example.com"},
		{"https://github.com/org/repo/pull/123", "pull/123"},
		{"https://docs.example.com/guide/setup.html", "setup"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ShortenURL(tt.input); got != tt.want {
				t.Errorf("ShortenURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
