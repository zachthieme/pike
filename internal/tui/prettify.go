package tui

import (
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// [[Display Name]] or [[slug|Display Name]]
	wikiLinkRe = regexp.MustCompile(`\[\[([^\]|]+?)(?:\|([^\]]+?))?\]\]`)
	// [text](url)
	mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	// Bare URLs: http:// or https://
	bareURLRe = regexp.MustCompile(`https?://\S+`)
)

// prettifyText cleans up markdown syntax for display:
//   - [[slug|Display Name]] → Display Name
//   - [[Display Name]] → Display Name
//   - [text](url) → text
//   - Long URLs → extracted document name or truncated host
func prettifyText(s string) string {
	// Wiki-links: use display name if present, otherwise the slug prettified.
	s = wikiLinkRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := wikiLinkRe.FindStringSubmatch(match)
		if sub[2] != "" {
			return sub[2]
		}
		return prettifySlug(sub[1])
	})

	// Markdown links: show just the text.
	s = mdLinkRe.ReplaceAllString(s, "$1")

	// Bare URLs: extract a meaningful name.
	s = bareURLRe.ReplaceAllStringFunc(s, shortenURL)

	return s
}

// prettifyAndStyleLinks works like prettifyText but applies a lipgloss style
// to the link display text so links are visually distinct.
func prettifyAndStyleLinks(s string, linkStyle lipgloss.Style) string {
	// Wiki-links: use display name if present, otherwise the slug prettified.
	s = wikiLinkRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := wikiLinkRe.FindStringSubmatch(match)
		var display string
		if sub[2] != "" {
			display = sub[2]
		} else {
			display = prettifySlug(sub[1])
		}
		return linkStyle.Render(display)
	})

	// Markdown links: show just the text, styled.
	s = mdLinkRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := mdLinkRe.FindStringSubmatch(match)
		return linkStyle.Render(sub[1])
	})

	// Bare URLs: extract a meaningful name, styled.
	s = bareURLRe.ReplaceAllStringFunc(s, func(match string) string {
		return linkStyle.Render(shortenURL(match))
	})

	return s
}

// LinkStyle returns a bold style with the given color for rendering links.
func LinkStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(resolveColor(color))
}

// prettifySlug turns "zach-thieme" into "Zach Thieme".
func prettifySlug(slug string) string {
	words := strings.Split(slug, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// shortenURL extracts a readable name from a URL.
// Tries to find the last meaningful path segment as a document name.
// Falls back to the hostname if the path is empty or just "/".
func shortenURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return truncate(raw, 25)
	}

	// Clean trailing slashes and get the path.
	p := strings.TrimRight(u.Path, "/")

	if p == "" {
		return u.Host
	}

	// Get the last path segment as the document name.
	name := path.Base(p)

	// If it looks like a bare number (e.g., PR #123), include the parent.
	if isNumeric(name) {
		parent := path.Base(path.Dir(p))
		if parent != "." && parent != "/" {
			name = parent + "/" + name
		}
	}

	// Strip common file extensions for readability.
	for _, ext := range []string{".html", ".htm", ".md", ".pdf"} {
		name = strings.TrimSuffix(name, ext)
	}

	if len(name) > 40 {
		name = name[:37] + "..."
	}

	return name
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
