package tui

import "testing"

func TestPrettifyText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "wiki link with display name",
			input: "talk to [[zach-thieme|Zach Thieme]] about it",
			want:  "talk to Zach Thieme about it",
		},
		{
			name:  "wiki link without display name",
			input: "talk to [[Zach Thieme]] about it",
			want:  "talk to Zach Thieme about it",
		},
		{
			name:  "wiki link slug gets prettified",
			input: "see [[jeff-roache]] for details",
			want:  "see Jeff Roache for details",
		},
		{
			name:  "markdown link shows text only",
			input: "check [the docs](https://example.com/docs/guide) first",
			want:  "check the docs first",
		},
		{
			name:  "bare URL extracts document name",
			input: "see https://example.com/docs/migration-plan for details",
			want:  "see migration-plan for details",
		},
		{
			name:  "bare URL with just host",
			input: "visit https://example.com/",
			want:  "visit example.com",
		},
		{
			name:  "bare URL with numeric path includes parent",
			input: "fix https://github.com/org/repo/pull/123",
			want:  "fix pull/123",
		},
		{
			name:  "bare URL strips .html extension",
			input: "read https://docs.example.com/guide/setup.html",
			want:  "read setup",
		},
		{
			name:  "multiple wiki links",
			input: "[[alice-bob|Alice Bob]] and [[charlie-delta|Charlie Delta]]",
			want:  "Alice Bob and Charlie Delta",
		},
		{
			name:  "mixed wiki link and bare URL",
			input: "ask [[zach-thieme|Zach Thieme]] about https://example.com/docs/auth-flow",
			want:  "ask Zach Thieme about auth-flow",
		},
		{
			name:  "no links unchanged",
			input: "just a plain task @today",
			want:  "just a plain task @today",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prettifyText(tt.input)
			if got != tt.want {
				t.Errorf("prettifyText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
