package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCreateBarActivate(t *testing.T) {
	cb := NewCreateBar()
	if cb.Active() {
		t.Error("expected inactive initially")
	}
	cb, _ = cb.Update(CreateActivateMsg{})
	if !cb.Active() {
		t.Error("expected active after activate")
	}
	if !cb.InputFocused() {
		t.Error("expected focused after activate")
	}
}

func TestCreateBarSubmitEmitsText(t *testing.T) {
	cb := NewCreateBar()
	cb, _ = cb.Update(CreateActivateMsg{})
	for _, r := range "buy milk @today" {
		cb, _ = cb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		cb.Output() // drain intermediate changed messages
	}
	cb, _ = cb.Update(tea.KeyMsg{Type: tea.KeyEnter})
	output := cb.Output()
	submitted, ok := output.(CreateSubmittedMsg)
	if !ok {
		t.Fatalf("expected CreateSubmittedMsg, got %T", output)
	}
	if submitted.Text != "buy milk @today" {
		t.Errorf("expected 'buy milk @today', got %q", submitted.Text)
	}
}

func TestCreateBarEscapeEmitsCleared(t *testing.T) {
	cb := NewCreateBar()
	cb, _ = cb.Update(CreateActivateMsg{})
	cb, _ = cb.Update(tea.KeyMsg{Type: tea.KeyEscape})
	output := cb.Output()
	if _, ok := output.(CreateClearedMsg); !ok {
		t.Fatalf("expected CreateClearedMsg, got %T", output)
	}
}

func TestCreateBarEscapeWithTextClearsThenDismisses(t *testing.T) {
	cb := NewCreateBar()
	cb, _ = cb.Update(CreateActivateMsg{})
	cb, _ = cb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	cb.Output() // drain
	// First escape clears text
	cb, _ = cb.Update(tea.KeyMsg{Type: tea.KeyEscape})
	cb.Output() // drain changed msg
	if cb.Text() != "" {
		t.Errorf("expected empty text, got %q", cb.Text())
	}
	// Second escape dismisses
	cb, _ = cb.Update(tea.KeyMsg{Type: tea.KeyEscape})
	output := cb.Output()
	if _, ok := output.(CreateClearedMsg); !ok {
		t.Fatalf("expected CreateClearedMsg on second escape, got %T", output)
	}
}

func TestCreateBarEmptySubmitNoOp(t *testing.T) {
	cb := NewCreateBar()
	cb, _ = cb.Update(CreateActivateMsg{})
	cb, _ = cb.Update(tea.KeyMsg{Type: tea.KeyEnter})
	output := cb.Output()
	if _, ok := output.(CreateSubmittedMsg); ok {
		t.Error("expected no CreateSubmittedMsg for empty input")
	}
}
