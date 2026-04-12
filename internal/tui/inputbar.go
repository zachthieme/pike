package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InputBar is a generic text input sub-model providing activate/deactivate,
// text entry, escape (clear then dismiss), enter (submit), and tab (toggle focus).
// After each Update call, check Output() for a pending message.
type InputBar struct {
	input  textinput.Model
	active bool
	output tea.Msg
	keys   KeyMap
}

// NewInputBar creates a new InputBar with default settings.
func NewInputBar() InputBar {
	ti := textinput.New()
	ti.CharLimit = 256
	ti.PromptStyle = BoldStyle()
	ti.PlaceholderStyle = FaintStyle().Foreground(lipgloss.Color("7"))
	return InputBar{
		input: ti,
		keys:  DefaultKeyMap(),
	}
}

// Output returns and clears the pending output message from the last Update.
func (b *InputBar) Output() tea.Msg {
	msg := b.output
	b.output = nil
	return msg
}

// Update handles incoming messages.
func (b InputBar) Update(msg tea.Msg) (InputBar, tea.Cmd) {
	switch m := msg.(type) {
	case InputActivateMsg:
		b.active = true
		b.input.Prompt = m.Prompt
		if m.Placeholder != "" {
			b.input.Placeholder = m.Placeholder
		}
		b.input.SetValue(m.InitialValue)
		b.input.CursorEnd()
		return b, b.input.Focus()

	case InputDeactivateMsg:
		b.active = false
		b.input.SetValue("")
		b.input.Blur()
		return b, nil

	case tea.KeyMsg:
		if !b.active {
			return b, nil
		}
		return b.handleKey(m)
	}

	return b, nil
}

func (b InputBar) handleKey(msg tea.KeyMsg) (InputBar, tea.Cmd) {
	switch {
	case key.Matches(msg, b.keys.Escape):
		if b.input.Value() != "" {
			b.input.SetValue("")
			if !b.input.Focused() {
				b.input.Focus()
			}
			b.output = InputChangedMsg{Text: ""}
			return b, nil
		}
		b.output = InputClearedMsg{}
		return b, nil

	case key.Matches(msg, b.keys.NextSection): // Tab
		if b.input.Focused() {
			b.input.Blur()
		} else {
			b.input.Focus()
		}
		return b, nil

	case key.Matches(msg, b.keys.Enter):
		if b.input.Focused() {
			b.input.Blur()
			b.output = InputSubmittedMsg{Text: b.input.Value()}
			return b, nil
		}
		return b, nil

	default:
		if b.input.Focused() {
			var cmd tea.Cmd
			b.input, cmd = b.input.Update(msg)
			b.output = InputChangedMsg{Text: b.input.Value()}
			return b, cmd
		}
		return b, nil
	}
}

// Active returns whether the input bar is currently active.
func (b InputBar) Active() bool { return b.active }

// InputFocused returns whether the text input currently has focus.
func (b InputBar) InputFocused() bool { return b.input.Focused() }

// Text returns the current input text.
func (b InputBar) Text() string { return b.input.Value() }

// View renders the input bar.
func (b InputBar) View() string {
	if !b.active {
		return ""
	}
	return b.input.View()
}
