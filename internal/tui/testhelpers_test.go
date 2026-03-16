package tui

import tea "github.com/charmbracelet/bubbletea"

// execBatchCmds executes a tea.Cmd which may be a tea.BatchMsg,
// returning all resulting messages.
func execBatchCmds(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batchMsg, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batchMsg {
			if c != nil {
				msgs = append(msgs, c())
			}
		}
		return msgs
	}
	return []tea.Msg{msg}
}

// findMsg searches a slice of tea.Msg for the first value of type T.
func findMsg[T any](msgs []tea.Msg) (T, bool) {
	var zero T
	for _, m := range msgs {
		if typed, ok := m.(T); ok {
			return typed, true
		}
	}
	return zero, false
}
