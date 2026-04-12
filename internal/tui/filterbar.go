package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// FilterBar is a Bubble Tea sub-model managing the filter text input.
// After each Update call, check Output() for a filter action message
// that the parent should process inline (e.g., FilterChangedMsg).
type FilterBar struct {
	bar      InputBar
	mode     filterMode
	queryErr error
	output   tea.Msg
	keys     KeyMap
}

// NewFilterBar creates a new FilterBar with default settings.
func NewFilterBar() FilterBar {
	return FilterBar{
		bar:  NewInputBar(),
		keys: DefaultKeyMap(),
	}
}

// Output returns the pending output message from the last Update call,
// then clears it.
func (f *FilterBar) Output() tea.Msg {
	msg := f.output
	f.output = nil
	return msg
}

// Update handles incoming messages and returns an updated FilterBar and optional command.
func (f FilterBar) Update(msg tea.Msg) (FilterBar, tea.Cmd) {
	switch m := msg.(type) {
	case FilterActivateMsg:
		f.mode = m.Mode
		f.queryErr = nil
		prompt := filterPrompt[m.Mode]
		placeholder := "type to filter..."
		if m.Placeholder != "" {
			placeholder = m.Placeholder
		}
		var cmd tea.Cmd
		f.bar, cmd = f.bar.Update(InputActivateMsg{
			Prompt:       prompt,
			Placeholder:  placeholder,
			InitialValue: m.InitialValue,
		})
		return f, cmd

	case FilterDeactivateMsg:
		f.mode = filterSubstring
		f.queryErr = nil
		f.bar, _ = f.bar.Update(InputDeactivateMsg{})
		return f, nil

	case FilterSetErrorMsg:
		f.queryErr = m.Err
		return f, nil

	case tea.KeyMsg:
		return f.handleKey(m)
	}

	return f, nil
}

// handleKey processes key messages, handling filter-specific keys before
// delegating to InputBar.
func (f FilterBar) handleKey(msg tea.KeyMsg) (FilterBar, tea.Cmd) {
	// Filter-specific mode switching: / and ?
	if key.Matches(msg, f.keys.Filter) {
		if f.mode != filterSubstring {
			f.mode = filterSubstring
			f.bar.input.Prompt = filterPrompt[filterSubstring]
			f.output = FilterModeChangedMsg{Mode: filterSubstring}
		}
		return f, f.bar.input.Focus()
	}
	if key.Matches(msg, f.keys.Query) {
		if f.mode != filterQuery {
			f.mode = filterQuery
			f.bar.input.Prompt = filterPrompt[filterQuery]
			f.output = FilterModeChangedMsg{Mode: filterQuery}
		}
		return f, f.bar.input.Focus()
	}

	// Delegate to InputBar, then translate output messages.
	var cmd tea.Cmd
	f.bar, cmd = f.bar.Update(msg)

	barOutput := f.bar.Output()
	if barOutput != nil {
		switch m := barOutput.(type) {
		case InputChangedMsg:
			f.output = FilterChangedMsg{Text: m.Text, Mode: f.mode}
		case InputSubmittedMsg:
			f.output = FilterSubmittedMsg{}
		case InputClearedMsg:
			f.output = FilterClearedMsg{}
		}
	}

	return f, cmd
}

// Active returns whether the filter bar is currently active.
func (f FilterBar) Active() bool { return f.bar.Active() }

// InputFocused returns whether the text input currently has focus.
func (f FilterBar) InputFocused() bool { return f.bar.InputFocused() }

// Text returns the current filter text.
func (f FilterBar) Text() string { return f.bar.Text() }

// Mode returns the current filter mode.
func (f FilterBar) Mode() filterMode { return f.mode }

// QueryErr returns the current query parse error, if any.
func (f FilterBar) QueryErr() error { return f.queryErr }

// View renders the filter bar.
func (f FilterBar) View() string { return f.bar.View() }
