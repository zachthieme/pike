package style

import (
	"testing"

	"github.com/zachthieme/pike/internal/model"
)

func noopStyleFunc(text, color string) string {
	return "\033[32m" + text + "\033[0m"
}

var benchTagColors = map[string]string{
	"due":       "#f38ba8",
	"risk":      "#f38ba8",
	"today":     "#a6e3a1",
	"completed": "#a6e3a1",
	"weekly":    "#89b4fa",
	"_default":  "#94e2d5",
}

func BenchmarkColorizeTags(b *testing.B) {
	tags := []model.Tag{
		{Name: "due", Value: "2026-03-20"},
		{Name: "risk"},
		{Name: "today"},
	}
	text := "deploy API to staging @due(2026-03-20) @risk @today"

	for b.Loop() {
		ColorizeTags(text, tags, benchTagColors, noopStyleFunc)
	}
}

func BenchmarkPrettifyLinks(b *testing.B) {
	cases := []struct {
		name string
		text string
	}{
		{"wiki_link", "see [[project-alpha|Project Alpha]] for details"},
		{"md_link", "check [the docs](https://example.com/docs/guide)"},
		{"bare_url", "deployed to https://github.com/org/repo/pull/123"},
		{"mixed", "see [[notes]] and [link](https://x.com/foo) and https://example.com/bar/baz"},
		{"no_links", "just a plain task with @tags"},
	}

	for _, tt := range cases {
		b.Run(tt.name, func(b *testing.B) {
			for b.Loop() {
				PrettifyText(tt.text)
			}
		})
	}
}

func BenchmarkShortenURL(b *testing.B) {
	urls := []struct {
		name string
		url  string
	}{
		{"github_pr", "https://github.com/org/repo/pull/123"},
		{"docs", "https://example.com/docs/guide/getting-started.html"},
		{"short", "https://example.com"},
		{"long_path", "https://example.com/very/long/nested/path/to/some/resource"},
	}

	for _, tt := range urls {
		b.Run(tt.name, func(b *testing.B) {
			for b.Loop() {
				ShortenURL(tt.url)
			}
		})
	}
}
