package tui

import (
	"fmt"
	"strings"

	"pike/internal/model"
	"pike/internal/style"

	"github.com/charmbracelet/lipgloss"
)

// minSectionWidth is the minimum inner width of a section border box.
const minSectionWidth = 10

// lipglossStyleFunc applies foreground color via lipgloss using cached styles.
func lipglossStyleFunc(text string, color string) string {
	return TagStyle(color).Render(text)
}

// renderSection renders a single view section with a colored header and task list.
// cursor is the global flat cursor index, sectionStart is the flat index of the
// first task in this section. hiddenCount is the number of @hidden tasks stripped;
// when > 0 a ○ icon is shown. When showHidden is true and the section contains
// @hidden tasks, a ◉ icon is shown instead. If totalCount is provided and > 0,
// it overrides the header count (used when tasks is a windowed slice).
func (m Model) renderSection(title string, tasks []model.Task, color string, sectionStart int, hiddenCount int, totalCount ...int) string {
	if len(tasks) == 0 {
		return ""
	}

	headerStyle := SectionHeaderStyle(color)

	borderColor := resolveColor(color)
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	if m.width > 0 {
		innerWidth := m.width - 4 // 4 = left/right border + padding
		if innerWidth < minSectionWidth {
			innerWidth = minSectionWidth
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
		selected := flatIdx == m.nav.Cursor() && (!m.filterBar.Active() || !m.filterBar.InputFocused())
		line := formatTaskLine(task, m.tagColors, linkColor, selected)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	headerCount := len(tasks)
	if len(totalCount) > 0 && totalCount[0] > 0 {
		headerCount = totalCount[0]
	}
	headerLabel := fmt.Sprintf(" %s (%d) ", title, headerCount)
	hiddenIcon := ""
	if hiddenCount > 0 && m.config != nil {
		if m.showHidden {
			hiddenIcon = lipglossStyleFunc("◉", m.config.VisibleColor)
		} else {
			hiddenIcon = lipglossStyleFunc("○", m.config.HiddenColor)
		}
	}
	headerText := headerStyle.Render(headerLabel)
	if hiddenIcon != "" {
		headerText += " " + hiddenIcon
	}
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

	marker := style.TaskMarker(task, true)

	if selected {
		marker = TaskStyle(true).Render(marker)
	}

	return fmt.Sprintf("%s %s", marker, text)
}
