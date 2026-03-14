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

// renderSection renders a single view section with a colored header and task list.
// cursor is the global flat cursor index, sectionStart is the flat index of the
// first task in this section. hiddenCount is the number of @hidden tasks stripped;
// when > 0 a lock icon is appended to the header.
func (m Model) renderSection(title string, tasks []model.Task, color string, sectionStart int, hiddenCount int) string {
	if len(tasks) == 0 {
		return ""
	}

	headerStyle := SectionHeaderStyle(color)

	borderColor := resolveColor(color)
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	if m.width > 0 {
		innerWidth := m.width - 4
		if innerWidth < 10 {
			innerWidth = 10
		}
		borderStyle = borderStyle.Width(innerWidth)
	}

	linkColor := ""
	if m.config != nil {
		linkColor = m.config.LinkColor
	}

	var lines []string
	for i, task := range tasks {
		flatIdx := sectionStart + i
		selected := flatIdx == m.cursor
		line := formatTaskLine(task, m.tagColors, linkColor, selected)
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
