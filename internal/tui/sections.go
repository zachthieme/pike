package tui

import (
	"fmt"
	"strings"

	"tasks/internal/model"

	"github.com/charmbracelet/lipgloss"
)

// RenderSection renders a single view section with a colored header and task list.
// cursor is the global flat cursor index, sectionStart is the flat index of the
// first task in this section. Returns empty string if tasks is empty.
func RenderSection(title string, tasks []model.Task, color string, cursor int, sectionStart int, tagColors map[string]string, width int, linkColor string) string {
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

	headerText := headerStyle.Render(fmt.Sprintf(" %s ", title))
	box := borderStyle.Render(content)

	return headerText + "\n" + box
}

// formatTaskLine formats a single task line with colorized tags and styled links.
// When selected is true, the leading marker is highlighted with reverse video.
func formatTaskLine(task model.Task, tagColors map[string]string, linkColor string, selected bool) string {
	var text string
	if linkColor != "" {
		text = prettifyAndStyleLinks(task.Text, LinkStyle(linkColor))
	} else {
		text = prettifyText(task.Text)
	}

	// Colorize tags in the text.
	if tagColors != nil {
		for _, tag := range task.Tags {
			token := tagToken(tag)
			color, ok := tagColors[tag.Name]
			if !ok {
				color = tagColors["_default"]
			}
			if color == "" {
				continue
			}
			styled := TagStyle(color).Render(token)
			text = strings.ReplaceAll(text, token, styled)
		}
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
