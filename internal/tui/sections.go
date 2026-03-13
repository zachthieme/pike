package tui

import (
	"fmt"
	"sort"
	"strings"

	"pike/internal/model"

	"github.com/charmbracelet/lipgloss"
)

// RenderSection renders a single view section with a colored header and task list.
// cursor is the global flat cursor index, sectionStart is the flat index of the
// first task in this section. hiddenCount is the number of @hidden tasks stripped
// from this section; when > 0 a lock icon is appended to the header.
// Returns empty string if tasks is empty.
func RenderSection(title string, tasks []model.Task, color string, cursor int, sectionStart int, tagColors map[string]string, width int, linkColor string, hiddenCount int) string {
	if len(tasks) == 0 {
		return ""
	}

	headerStyle := SectionHeaderStyle(color)

	// Build the border box around the section.
	borderColor := resolveColor(color)
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	if width > 0 {
		// Account for border characters (2 on each side).
		innerWidth := width - 4
		if innerWidth < 10 {
			innerWidth = 10
		}
		borderStyle = borderStyle.Width(innerWidth)
	}

	// Build section content.
	var lines []string

	for i, task := range tasks {
		flatIdx := sectionStart + i
		selected := flatIdx == cursor

		line := formatTaskLine(task, tagColors, linkColor, selected)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	headerLabel := fmt.Sprintf(" %s ", title)
	if hiddenCount > 0 {
		headerLabel = fmt.Sprintf(" %s 🔒", title)
	}
	headerText := headerStyle.Render(headerLabel)
	box := borderStyle.Render(content)

	return headerText + "\n" + box
}

// formatTaskLine formats a single task line with colorized tags and styled links.
// When selected is true, the leading marker is highlighted with reverse video.
func formatTaskLine(task model.Task, tagColors map[string]string, linkColor string, selected bool) string {
	text := task.Text

	// Colorize tags first (before prettify, which may transform wiki-links
	// inside tag values like @delegated([[slug|Name]])).
	if tagColors != nil {
		type tagReplacement struct {
			token  string
			styled string
		}
		seen := make(map[string]bool)
		var replacements []tagReplacement
		for _, tag := range task.Tags {
			token := tagToken(tag)
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
			style := TagStyle(color)
			var styled string
			if tag.Value != "" {
				// Render prefix, value, and closing paren separately so that
				// ANSI resets from link styling inside the value don't prevent
				// the closing paren from being colored.
				styled = style.Render("@"+tag.Name+"(") + style.Render(tag.Value) + style.Render(")")
			} else {
				styled = style.Render(token)
			}
			replacements = append(replacements, tagReplacement{
				token:  token,
				styled: styled,
			})
		}
		// Sort by token length descending so longer tokens (e.g. @delegated(John))
		// are replaced before shorter prefixes (e.g. @delegated).
		sort.Slice(replacements, func(i, j int) bool {
			return len(replacements[i].token) > len(replacements[j].token)
		})
		for _, r := range replacements {
			text = strings.Replace(text, r.token, r.styled, 1)
		}
	}

	// Prettify links after tag coloring so wiki-links inside tag values
	// are transformed without breaking the tag token match.
	if linkColor != "" {
		text = prettifyAndStyleLinks(text, LinkStyle(linkColor))
	} else {
		text = prettifyText(text)
	}

	var marker string
	if task.HasCheckbox {
		marker = "○"
		if task.State == model.Completed {
			marker = "●"
		}
	} else {
		marker = "▸"
	}

	if selected {
		marker = TaskStyle(true).Render(marker)
	}

	return fmt.Sprintf("%s %s", marker, text)
}

// tagToken returns the string representation of a tag as it appears in task text.
func tagToken(tag model.Tag) string {
	if tag.Value != "" {
		return "@" + tag.Name + "(" + tag.Value + ")"
	}
	return "@" + tag.Name
}
