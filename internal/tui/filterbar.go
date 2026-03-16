package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FilterBar is a Bubble Tea sub-model managing the filter text input.
// After each Update call, check Output() for a filter action message
// that the parent should process inline (e.g., FilterChangedMsg).
type FilterBar struct {
	input    textinput.Model
	active   bool
	mode     filterMode
	text     string
	queryErr error
	output   tea.Msg // pending output message for parent; nil if none
}

// Output returns the pending output message from the last Update call,
// then clears it. The parent should call this after Update to handle
// filter actions (FilterChangedMsg, FilterClearedMsg, etc.) inline.
func (f *FilterBar) Output() tea.Msg {
	msg := f.output
	f.output = nil
	return msg
}

// NewFilterBar creates a new FilterBar with default settings.
func NewFilterBar() FilterBar {
	ti := textinput.New()
	ti.CharLimit = 256
	ti.Prompt = "/ "
	ti.PromptStyle = BoldStyle()
	ti.PlaceholderStyle = FaintStyle().Foreground(lipgloss.Color("7"))
	ti.Placeholder = "type to filter..."
	return FilterBar{
		input: ti,
	}
}

// Init implements tea.Model. Returns nil.
func (f FilterBar) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and returns an updated FilterBar and optional command.
func (f FilterBar) Update(msg tea.Msg) (FilterBar, tea.Cmd) {
	switch m := msg.(type) {
	case FilterActivateMsg:
		f.active = true
		f.mode = m.Mode
		f.text = m.InitialValue
		f.queryErr = nil

		// Set prompt based on mode.
		if prompt, ok := filterPrompt[m.Mode]; ok {
			f.input.Prompt = prompt
		}
		// Set placeholder if provided.
		if m.Placeholder != "" {
			f.input.Placeholder = m.Placeholder
		}
		// Set initial value.
		f.input.SetValue(m.InitialValue)
		// Position cursor at end.
		f.input.CursorEnd()
		// Focus the input.
		cmd := f.input.Focus()
		return f, cmd

	case FilterDeactivateMsg:
		f.active = false
		f.mode = filterSubstring
		f.text = ""
		f.queryErr = nil
		f.input.SetValue("")
		f.input.Prompt = "/ "
		f.input.Placeholder = "type to filter..."
		f.input.Blur()
		return f, nil

	case FilterSetErrorMsg:
		f.queryErr = m.Err
		return f, nil

	case tea.KeyMsg:
		return f.handleKey(m)
	}

	return f, nil
}

// handleKey processes key messages.
func (f FilterBar) handleKey(msg tea.KeyMsg) (FilterBar, tea.Cmd) {
	km := DefaultKeyMap()

	switch {
	case key.Matches(msg, km.Escape):
		if f.input.Value() != "" {
			// Clear content, re-focus if blurred, store output for parent.
			f.input.SetValue("")
			f.text = ""
			if !f.input.Focused() {
				f.input.Focus()
			}
			f.output = FilterChangedMsg{Text: "", Mode: f.mode}
			return f, nil
		}
		// Input is empty → signal parent to exit filter mode.
		f.output = FilterClearedMsg{}
		return f, nil

	case key.Matches(msg, km.NextSection): // Tab
		if f.input.Focused() {
			f.input.Blur()
		} else {
			f.input.Focus()
		}
		return f, nil

	case key.Matches(msg, km.Filter): // /
		if f.mode != filterSubstring {
			f.mode = filterSubstring
			f.input.Prompt = filterPrompt[filterSubstring]
			f.output = FilterModeChangedMsg{Mode: filterSubstring}
		}
		return f, f.input.Focus()

	case key.Matches(msg, km.Query): // ?
		if f.mode != filterQuery {
			f.mode = filterQuery
			f.input.Prompt = filterPrompt[filterQuery]
			f.output = FilterModeChangedMsg{Mode: filterQuery}
		}
		return f, f.input.Focus()

	case key.Matches(msg, km.Enter):
		if f.input.Focused() {
			f.input.Blur()
			f.output = FilterSubmittedMsg{}
			return f, nil
		}
		// Not focused — return nil so parent can handle.
		return f, nil

	default:
		if f.input.Focused() {
			var cmd tea.Cmd
			f.input, cmd = f.input.Update(msg)
			f.text = f.input.Value()
			f.output = FilterChangedMsg{Text: f.text, Mode: f.mode}
			return f, cmd
		}
		return f, nil
	}
}

// Active returns whether the filter bar is currently active.
func (f FilterBar) Active() bool { return f.active }

// InputFocused returns whether the text input currently has focus.
func (f FilterBar) InputFocused() bool { return f.input.Focused() }

// Text returns the current filter text.
func (f FilterBar) Text() string { return f.text }

// Mode returns the current filter mode.
func (f FilterBar) Mode() filterMode { return f.mode }

// QueryErr returns the current query parse error, if any.
func (f FilterBar) QueryErr() error { return f.queryErr }

// View renders the filter bar.
func (f FilterBar) View() string {
	if !f.active {
		return ""
	}
	return f.input.View()
}
