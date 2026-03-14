package tui

import (
	"fmt"
	"strings"

	"pike/internal/model"
	"pike/internal/style"

	"github.com/charmbracelet/lipgloss"
)

// lipglossStyleFunc applies foreground color via lipgloss.
func lipglossStyleFunc(text string, color string) string {
	return lipgloss.NewStyle().Foreground(resolveColor(color)).Render(text)
}

// RenderSection renders a single view section with a colored header and task list.
func RenderSection(title string, tasks []model.Task, color string, cursor int, sectionStart int, tagColors map[string]string, width int, linkColor string, hiddenCount int) string {
	if len(tasks) == 0 {
		return ""
	}

	headerStyle := SectionHeaderStyle(color)

	borderColor := resolveColor(color)
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	if width > 0 {
		innerWidth := width - 4
		if innerWidth < 10 {
			innerWidth = 10
		}
		borderStyle = borderStyle.Width(innerWidth)
	}

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
func formatTaskLine(task model.Task, tagColors map[string]string, linkColor string, selected bool) string {
	text := task.Text

	if tagColors != nil {
		text = style.ColorizeTags(text, task.Tags, tagColors, lipglossStyleFunc)
	}

	if linkColor != "" {
		text = style.PrettifyLinks(text, func(s string) string {
			return LinkStyle(linkColor).Render(s)
		})
	} else {
		text = style.PrettifyText(text)
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
