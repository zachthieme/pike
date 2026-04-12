package tui

import tea "github.com/charmbracelet/bubbletea"

// CreateBar is a Bubble Tea sub-model for inline task creation.
// Wraps InputBar with create-specific behavior: Enter submits the task text,
// Escape cancels. Empty submissions are suppressed.
// After each Update call, check Output() for CreateSubmittedMsg or CreateClearedMsg.
type CreateBar struct {
	bar    InputBar
	output tea.Msg
}

// NewCreateBar creates a new CreateBar.
func NewCreateBar() CreateBar {
	return CreateBar{bar: NewInputBar()}
}

// Output returns and clears the pending output message.
func (c *CreateBar) Output() tea.Msg {
	msg := c.output
	c.output = nil
	return msg
}

// Update handles incoming messages.
func (c CreateBar) Update(msg tea.Msg) (CreateBar, tea.Cmd) {
	switch msg.(type) {
	case CreateActivateMsg:
		var cmd tea.Cmd
		c.bar, cmd = c.bar.Update(InputActivateMsg{
			Prompt:      "+ ",
			Placeholder: "new task...",
		})
		return c, cmd

	case InputDeactivateMsg:
		c.bar, _ = c.bar.Update(msg)
		return c, nil
	}

	// Delegate to InputBar, then translate output messages.
	var cmd tea.Cmd
	c.bar, cmd = c.bar.Update(msg)

	barOutput := c.bar.Output()
	if barOutput != nil {
		switch m := barOutput.(type) {
		case InputSubmittedMsg:
			if m.Text != "" {
				c.output = CreateSubmittedMsg(m)
			}
		case InputClearedMsg:
			c.output = CreateClearedMsg{}
		}
	}

	return c, cmd
}

// Active returns whether the create bar is currently active.
func (c CreateBar) Active() bool { return c.bar.Active() }

// InputFocused returns whether the text input currently has focus.
func (c CreateBar) InputFocused() bool { return c.bar.InputFocused() }

// Text returns the current input text.
func (c CreateBar) Text() string { return c.bar.Text() }

// View renders the create bar.
func (c CreateBar) View() string { return c.bar.View() }
