package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// execCmd executes a single tea.Cmd and returns the resulting tea.Msg.
func execCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

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

func TestFilterBarActivate(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{
		Mode:         filterQuery,
		InitialValue: "open",
	})

	if !fb.Active() {
		t.Error("expected Active() == true after activate")
	}
	if fb.Mode() != filterQuery {
		t.Errorf("expected Mode() == filterQuery, got %v", fb.Mode())
	}
	if fb.Text() != "open" {
		t.Errorf("expected Text() == \"open\", got %q", fb.Text())
	}
	if !fb.InputFocused() {
		t.Error("expected InputFocused() == true after activate")
	}
}

func TestFilterBarDeactivate(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{
		Mode:         filterQuery,
		InitialValue: "something",
	})
	fb, _ = fb.Update(FilterDeactivateMsg{})

	if fb.Active() {
		t.Error("expected Active() == false after deactivate")
	}
	if fb.Text() != "" {
		t.Errorf("expected Text() == \"\", got %q", fb.Text())
	}
	if fb.InputFocused() {
		t.Error("expected InputFocused() == false after deactivate")
	}
}

func TestFilterBarKeystrokeEmitsChanged(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterSubstring})

	fb, cmd := fb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	msgs := execBatchCmds(cmd)
	changed, ok := findMsg[FilterChangedMsg](msgs)
	if !ok {
		t.Fatal("expected FilterChangedMsg to be emitted")
	}
	if changed.Text != "a" {
		t.Errorf("expected FilterChangedMsg.Text == \"a\", got %q", changed.Text)
	}
	if changed.Mode != filterSubstring {
		t.Errorf("expected FilterChangedMsg.Mode == filterSubstring, got %v", changed.Mode)
	}
	if fb.Text() != "a" {
		t.Errorf("expected fb.Text() == \"a\", got %q", fb.Text())
	}
}

func TestFilterBarEnterEmitsSubmitted(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterSubstring})

	// Input should be focused after activate.
	if !fb.InputFocused() {
		t.Fatal("expected input to be focused after activate")
	}

	fb, cmd := fb.Update(tea.KeyMsg{Type: tea.KeyEnter})

	msgs := execBatchCmds(cmd)
	_, ok := findMsg[FilterSubmittedMsg](msgs)
	if !ok {
		t.Fatal("expected FilterSubmittedMsg to be emitted on Enter")
	}
	if fb.InputFocused() {
		t.Error("expected InputFocused() == false after Enter")
	}
}

func TestFilterBarEscapeWithContentClears(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{
		Mode:         filterSubstring,
		InitialValue: "test",
	})

	fb, cmd := fb.Update(tea.KeyMsg{Type: tea.KeyEscape})

	if fb.Text() != "" {
		t.Errorf("expected Text() == \"\" after Escape with content, got %q", fb.Text())
	}

	msgs := execBatchCmds(cmd)

	// Should emit FilterChangedMsg, NOT FilterClearedMsg.
	_, clearedOk := findMsg[FilterClearedMsg](msgs)
	if clearedOk {
		t.Error("expected no FilterClearedMsg when escaping with content")
	}

	changed, changedOk := findMsg[FilterChangedMsg](msgs)
	if !changedOk {
		t.Fatal("expected FilterChangedMsg to be emitted on Escape with content")
	}
	if changed.Text != "" {
		t.Errorf("expected FilterChangedMsg.Text == \"\", got %q", changed.Text)
	}
}

func TestFilterBarEscapeEmptyEmitsCleared(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterSubstring})

	// No initial value, so input is empty.
	fb, cmd := fb.Update(tea.KeyMsg{Type: tea.KeyEscape})

	msgs := execBatchCmds(cmd)
	_, ok := findMsg[FilterClearedMsg](msgs)
	if !ok {
		t.Fatal("expected FilterClearedMsg when Escape on empty input")
	}
}

func TestFilterBarTabTogglesFocus(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterSubstring})

	// Should be focused after activate.
	if !fb.InputFocused() {
		t.Fatal("expected input focused after activate")
	}

	// Tab → blur.
	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyTab})
	if fb.InputFocused() {
		t.Error("expected input blurred after first Tab")
	}

	// Tab again → focus.
	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !fb.InputFocused() {
		t.Error("expected input focused after second Tab")
	}
}

func TestFilterBarModeSwitch(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterSubstring})

	// Send '?' to switch to query mode.
	fb, cmd := fb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})

	if fb.Mode() != filterQuery {
		t.Errorf("expected Mode() == filterQuery after '?', got %v", fb.Mode())
	}

	msgs := execBatchCmds(cmd)
	modeMsg, ok := findMsg[FilterModeChangedMsg](msgs)
	if !ok {
		t.Fatal("expected FilterModeChangedMsg to be emitted")
	}
	if modeMsg.Mode != filterQuery {
		t.Errorf("expected FilterModeChangedMsg.Mode == filterQuery, got %v", modeMsg.Mode)
	}
}

func TestFilterBarSetError(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterQuery})

	testErr := errors.New("bad query")
	fb, _ = fb.Update(FilterSetErrorMsg{Err: testErr})

	if fb.QueryErr() == nil {
		t.Fatal("expected QueryErr() to return an error")
	}
	if fb.QueryErr().Error() != "bad query" {
		t.Errorf("expected error message \"bad query\", got %q", fb.QueryErr().Error())
	}
}
