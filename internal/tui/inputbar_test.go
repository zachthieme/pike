package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInputBarActivateDeactivate(t *testing.T) {
	bar := NewInputBar()
	if bar.Active() {
		t.Error("expected inactive initially")
	}

	bar, _ = bar.Update(InputActivateMsg{
		Prompt:      "+ ",
		Placeholder: "new task...",
	})
	if !bar.Active() {
		t.Error("expected active after activate")
	}
	if !bar.InputFocused() {
		t.Error("expected focused after activate")
	}

	bar, _ = bar.Update(InputDeactivateMsg{})
	if bar.Active() {
		t.Error("expected inactive after deactivate")
	}
	if bar.Text() != "" {
		t.Errorf("expected empty text after deactivate, got %q", bar.Text())
	}
}

func TestInputBarTypingEmitsChanged(t *testing.T) {
	bar := NewInputBar()
	bar, _ = bar.Update(InputActivateMsg{Prompt: "> "})

	bar, _ = bar.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	output := bar.Output()
	changed, ok := output.(InputChangedMsg)
	if !ok {
		t.Fatalf("expected InputChangedMsg, got %T", output)
	}
	if changed.Text != "h" {
		t.Errorf("expected text 'h', got %q", changed.Text)
	}
}

func TestInputBarEnterEmitsSubmitted(t *testing.T) {
	bar := NewInputBar()
	bar, _ = bar.Update(InputActivateMsg{Prompt: "> ", InitialValue: "hello"})

	bar, _ = bar.Update(tea.KeyMsg{Type: tea.KeyEnter})
	output := bar.Output()
	submitted, ok := output.(InputSubmittedMsg)
	if !ok {
		t.Fatalf("expected InputSubmittedMsg, got %T", output)
	}
	if submitted.Text != "hello" {
		t.Errorf("expected text 'hello', got %q", submitted.Text)
	}
}

func TestInputBarEscapeWithTextClears(t *testing.T) {
	bar := NewInputBar()
	bar, _ = bar.Update(InputActivateMsg{Prompt: "> ", InitialValue: "text"})

	bar, _ = bar.Update(tea.KeyMsg{Type: tea.KeyEscape})
	output := bar.Output()
	changed, ok := output.(InputChangedMsg)
	if !ok {
		t.Fatalf("expected InputChangedMsg on first escape, got %T", output)
	}
	if changed.Text != "" {
		t.Errorf("expected empty text, got %q", changed.Text)
	}
}

func TestInputBarEscapeEmptyEmitsCleared(t *testing.T) {
	bar := NewInputBar()
	bar, _ = bar.Update(InputActivateMsg{Prompt: "> "})

	bar, _ = bar.Update(tea.KeyMsg{Type: tea.KeyEscape})
	output := bar.Output()
	if _, ok := output.(InputClearedMsg); !ok {
		t.Fatalf("expected InputClearedMsg, got %T", output)
	}
}

func TestInputBarTabTogglesFocus(t *testing.T) {
	bar := NewInputBar()
	bar, _ = bar.Update(InputActivateMsg{Prompt: "> "})
	if !bar.InputFocused() {
		t.Fatal("expected focused after activate")
	}

	bar, _ = bar.Update(tea.KeyMsg{Type: tea.KeyTab})
	if bar.InputFocused() {
		t.Error("expected blurred after tab")
	}

	bar, _ = bar.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !bar.InputFocused() {
		t.Error("expected focused after second tab")
	}
}

func TestInputBarOutputClearsAfterRead(t *testing.T) {
	bar := NewInputBar()
	bar, _ = bar.Update(InputActivateMsg{Prompt: "> "})
	bar, _ = bar.Update(tea.KeyMsg{Type: tea.KeyEscape})

	out1 := bar.Output()
	if out1 == nil {
		t.Fatal("expected non-nil output")
	}
	out2 := bar.Output()
	if out2 != nil {
		t.Errorf("expected nil on second read, got %T", out2)
	}
}
