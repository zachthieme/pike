package style

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"pike/internal/model"
)

// StyleFunc applies a color to a string. Callers provide the implementation
// (raw ANSI for stdout, lipgloss for TUI).
type StyleFunc func(text string, color string) string

// namedColors maps color names to ANSI escape sequences.
var namedColors = map[string]string{
	"red":     "\033[31m",
	"green":   "\033[32m",
	"yellow":  "\033[33m",
	"blue":    "\033[34m",
	"magenta": "\033[35m",
	"cyan":    "\033[36m",
	"white":   "\033[37m",
}

const ansiReset = "\033[0m"

// ansiStripRe matches ANSI escape sequences.
var ansiStripRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// TagToken returns the string representation of a tag as it appears in task text.
func TagToken(tag model.Tag) string {
	if tag.Value != "" {
		return "@" + tag.Name + "(" + tag.Value + ")"
	}
	return "@" + tag.Name
}

// StripANSI removes all ANSI escape sequences from a string.
func StripANSI(s string) string {
	return ansiStripRe.ReplaceAllString(s, "")
}

// colorCode returns the ANSI escape sequence for a color name or hex value.
func colorCode(color string) string {
	if code, ok := namedColors[color]; ok {
		return code
	}
	if strings.HasPrefix(color, "#") && len(color) == 7 {
		r, errR := strconv.ParseInt(color[1:3], 16, 64)
		g, errG := strconv.ParseInt(color[3:5], 16, 64)
		b, errB := strconv.ParseInt(color[5:7], 16, 64)
		if errR == nil && errG == nil && errB == nil {
			return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
		}
	}
	return ""
}

// ColorizeTags replaces tag tokens in text with colored versions.
func ColorizeTags(text string, tags []model.Tag, tagColors map[string]string, sf StyleFunc) string {
	if tagColors == nil {
		return text
	}

	type tagReplacement struct {
		token  string
		styled string
	}
	seen := make(map[string]bool)
	var replacements []tagReplacement
	for _, tag := range tags {
		token := TagToken(tag)
		if seen[token] {
			continue
		}
		seen[token] = true
		color, ok := tagColors[tag.Name]
		if !ok {
			color = tagColors["_default"]
		}
		if color == "" {
			continue
		}
		var styled string
		if tag.Value != "" {
			styled = sf("@"+tag.Name+"(", color) + sf(tag.Value, color) + sf(")", color)
		} else {
			styled = sf(token, color)
		}
		replacements = append(replacements, tagReplacement{
			token:  token,
			styled: styled,
		})
	}
	sort.Slice(replacements, func(i, j int) bool {
		return len(replacements[i].token) > len(replacements[j].token)
	})
	// Two-pass replacement: first substitute tokens with unique placeholders,
	// then replace placeholders with styled text. This prevents shorter tokens
	// (e.g., @due) from matching inside already-styled longer tokens
	// (e.g., the styled output of @due(2026-03-15) which contains "@due" literally).
	placeholders := make([]string, len(replacements))
	for i, r := range replacements {
		ph := fmt.Sprintf("\x00PH%d\x00", i)
		placeholders[i] = ph
		text = strings.Replace(text, r.token, ph, 1)
	}
	for i, r := range replacements {
		text = strings.Replace(text, placeholders[i], r.styled, 1)
	}
	return text
}

// ANSIStyleFunc returns a StyleFunc that wraps text in raw ANSI color codes.
func ANSIStyleFunc() StyleFunc {
	return func(text string, color string) string {
		code := colorCode(color)
		if code == "" {
			return text
		}
		return code + text + ansiReset
	}
}
