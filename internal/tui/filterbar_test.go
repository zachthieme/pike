package tui

import (
	"errors"
	"fmt"
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

	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	output := fb.Output()
	changed, ok := output.(FilterChangedMsg)
	if !ok {
		t.Fatalf("expected FilterChangedMsg output, got %T", output)
	}
	if changed.Text != "a" {
		t.Errorf("expected Text == \"a\", got %q", changed.Text)
	}
	if changed.Mode != filterSubstring {
		t.Errorf("expected Mode == filterSubstring, got %v", changed.Mode)
	}
	if fb.Text() != "a" {
		t.Errorf("expected fb.Text() == \"a\", got %q", fb.Text())
	}
}

func TestFilterBarEnterEmitsSubmitted(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterSubstring})

	if !fb.InputFocused() {
		t.Fatal("expected input to be focused after activate")
	}

	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyEnter})

	output := fb.Output()
	if _, ok := output.(FilterSubmittedMsg); !ok {
		t.Fatalf("expected FilterSubmittedMsg output, got %T", output)
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

	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyEscape})

	if fb.Text() != "" {
		t.Errorf("expected Text() == \"\" after Escape with content, got %q", fb.Text())
	}

	output := fb.Output()
	// Should emit FilterChangedMsg, NOT FilterClearedMsg.
	if _, ok := output.(FilterClearedMsg); ok {
		t.Error("expected no FilterClearedMsg when escaping with content")
	}
	changed, ok := output.(FilterChangedMsg)
	if !ok {
		t.Fatalf("expected FilterChangedMsg output, got %T", output)
	}
	if changed.Text != "" {
		t.Errorf("expected FilterChangedMsg.Text == \"\", got %q", changed.Text)
	}
}

func TestFilterBarEscapeEmptyEmitsCleared(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterSubstring})

	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyEscape})

	output := fb.Output()
	if _, ok := output.(FilterClearedMsg); !ok {
		t.Fatalf("expected FilterClearedMsg output, got %T", output)
	}
}

func TestFilterBarTabTogglesFocus(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterSubstring})

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
	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})

	if fb.Mode() != filterQuery {
		t.Errorf("expected Mode() == filterQuery after '?', got %v", fb.Mode())
	}

	output := fb.Output()
	modeMsg, ok := output.(FilterModeChangedMsg)
	if !ok {
		t.Fatalf("expected FilterModeChangedMsg output, got %T", output)
	}
	if modeMsg.Mode != filterQuery {
		t.Errorf("expected Mode == filterQuery, got %v", modeMsg.Mode)
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

func TestFilterBarOutputClearsAfterRead(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterSubstring})

	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyEscape})

	// First call returns the message.
	output := fb.Output()
	if output == nil {
		t.Fatal("expected non-nil output")
	}

	// Second call returns nil (cleared).
	output2 := fb.Output()
	if output2 != nil {
		t.Errorf("expected nil after second Output() call, got %T", output2)
	}
}

// Task 13: Sub-model gap tests

func TestFilterBarModeSwitchPreservesText(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterSubstring, InitialValue: "hello"})
	if fb.Text() != "hello" {
		t.Errorf("text = %q, want 'hello'", fb.Text())
	}
	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	output := fb.Output()
	if modeMsg, ok := output.(FilterModeChangedMsg); ok {
		if modeMsg.Mode != filterQuery {
			t.Errorf("mode = %v, want filterQuery", modeMsg.Mode)
		}
	}
	if fb.Text() != "hello" {
		t.Errorf("text = %q after mode switch, want 'hello'", fb.Text())
	}
}

func TestFilterBarInvalidDSLSetsError(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterQuery, InitialValue: ""})
	fb, _ = fb.Update(FilterSetErrorMsg{Err: fmt.Errorf("parse error")})
	if fb.QueryErr() == nil {
		t.Error("QueryErr should be set")
	}
	if fb.QueryErr().Error() != "parse error" {
		t.Errorf("QueryErr = %v, want 'parse error'", fb.QueryErr())
	}
}

func TestFilterBarEscapeClearsAndExits(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterSubstring, InitialValue: "text"})
	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyEscape})
	output := fb.Output()
	if _, ok := output.(FilterChangedMsg); !ok {
		t.Errorf("first escape should emit FilterChangedMsg, got %T", output)
	}
	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyEscape})
	output = fb.Output()
	if _, ok := output.(FilterClearedMsg); !ok {
		t.Errorf("second escape should emit FilterClearedMsg, got %T", output)
	}
}
