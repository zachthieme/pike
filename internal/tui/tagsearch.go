package tui

import (
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TagSearch is a Bubble Tea sub-model for the tag picker UI.
// It owns its own textinput.Model, independent of FilterBar.
type TagSearch struct {
	tagList    []string
	tagCursor  int
	filter     textinput.Model
	filterText string
}

// NewTagSearch creates a new TagSearch with default settings.
func NewTagSearch() TagSearch {
	ti := textinput.New()
	ti.Placeholder = "search tags..."
	ti.CharLimit = 256
	ti.Prompt = "/ "
	ti.PromptStyle = BoldStyle()
	ti.PlaceholderStyle = FaintStyle().Foreground(lipgloss.Color("7"))
	return TagSearch{
		filter: ti,
	}
}

// Init implements tea.Model. Returns nil.
func (t TagSearch) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and returns an updated TagSearch and optional command.
func (t TagSearch) Update(msg tea.Msg) (TagSearch, tea.Cmd) {
	switch m := msg.(type) {
	case TagSearchActivateMsg:
		t.tagList = make([]string, len(m.Tags))
		copy(t.tagList, m.Tags)
		slices.Sort(t.tagList)
		t.tagCursor = 0
		t.filterText = ""
		t.filter.SetValue("")
		cmd := t.filter.Focus()
		return t, cmd

	case TagSearchRefreshMsg:
		t.tagList = make([]string, len(m.Tags))
		copy(t.tagList, m.Tags)
		slices.Sort(t.tagList)
		// Clamp cursor to valid range without resetting position or filter text.
		if tags := t.filteredTags(); len(tags) > 0 {
			if t.tagCursor >= len(tags) {
				t.tagCursor = len(tags) - 1
			}
		} else {
			t.tagCursor = 0
		}
		return t, nil

	case tea.KeyMsg:
		return t.handleKey(m)
	}

	return t, nil
}

// handleKey processes key messages for tag search navigation.
func (t TagSearch) handleKey(msg tea.KeyMsg) (TagSearch, tea.Cmd) {
	km := DefaultKeyMap()

	switch {
	case key.Matches(msg, km.Escape):
		return t, func() tea.Msg { return TagSearchExitMsg{} }

	case key.Matches(msg, km.Quit):
		return t, tea.Quit

	case key.Matches(msg, km.NextSection) || key.Matches(msg, km.Down):
		tags := t.filteredTags()
		if len(tags) > 0 {
			t.tagCursor = (t.tagCursor + 1) % len(tags)
		}
		return t, nil

	case key.Matches(msg, km.PrevSection) || key.Matches(msg, km.Up):
		tags := t.filteredTags()
		if len(tags) > 0 {
			t.tagCursor = (t.tagCursor - 1 + len(tags)) % len(tags)
		}
		return t, nil

	case key.Matches(msg, km.Enter):
		tags := t.filteredTags()
		if t.tagCursor < len(tags) {
			name := tags[t.tagCursor]
			return t, func() tea.Msg { return TagSelectedMsg{Name: name} }
		}
		return t, nil

	default:
		var cmd tea.Cmd
		t.filter, cmd = t.filter.Update(msg)
		t.filterText = t.filter.Value()
		t.tagCursor = 0
		return t, cmd
	}
}

// filteredTags returns the subset of tagList matching the current filter text.
// If filterText is empty, all tags are returned. Otherwise tags are matched by
// substring after stripping a leading "@" from the filter and lowercasing both.
func (t TagSearch) filteredTags() []string {
	if t.filterText == "" {
		return t.tagList
	}
	lower := strings.ToLower(strings.TrimPrefix(t.filterText, "@"))
	var result []string
	for _, tag := range t.tagList {
		if strings.Contains(strings.ToLower(tag), lower) {
			result = append(result, tag)
		}
	}
	return result
}

// View renders the tag search UI.
// tagColors maps tag names to color strings; width is the terminal width.
func (t TagSearch) View(tagColors map[string]string, width int) string {
	var parts []string

	parts = append(parts, t.filter.View())

	if len(t.tagList) == 0 {
		parts = append(parts, "  No tags found")
		return strings.Join(parts, "\n")
	}

	filtered := t.filteredTags()

	// Build a set of matched tag names for quick lookup.
	matchedSet := make(map[string]bool, len(filtered))
	for _, tag := range filtered {
		matchedSet[tag] = true
	}

	// Determine which filtered tag is currently selected.
	selectedTag := ""
	if len(filtered) > 0 && t.tagCursor < len(filtered) {
		selectedTag = filtered[t.tagCursor]
	}

	fs := FaintStyle()
	delim := fs.Render("\u2009·\u2009")
	var tagParts []string
	for _, tag := range t.tagList {
		if tag == selectedTag {
			tagParts = append(tagParts, TaskStyle(true).Render(tag))
		} else if matchedSet[tag] {
			if color := resolveTagColor(tagColors, tag); color != "" {
				tagParts = append(tagParts, TagStyle(color).Render(tag))
			} else {
				tagParts = append(tagParts, tag)
			}
		} else {
			tagParts = append(tagParts, fs.Render(tag))
		}
	}

	if width > 0 {
		parts = append(parts, flowWrap(tagParts, delim, width-2))
	} else {
		parts = append(parts, "  "+strings.Join(tagParts, delim))
	}

	if len(filtered) == 0 && t.filterText != "" {
		parts = append(parts, "  No results")
	}

	return strings.Join(parts, "\n")
}

// resolveTagColor looks up tagName in tagColors, falling back to "_default".
// Returns "" if neither is found.
func resolveTagColor(tagColors map[string]string, tagName string) string {
	if color, ok := tagColors[tagName]; ok {
		return color
	}
	if color, ok := tagColors["_default"]; ok {
		return color
	}
	return ""
}

// FilterText returns the current filter text.
func (t TagSearch) FilterText() string {
	return t.filterText
}
