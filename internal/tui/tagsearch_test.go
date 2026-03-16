package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestTagSearchActivate verifies that activating TagSearch stores and sorts tags.
func TestTagSearchActivate(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due", "today", "risk"}})

	if len(ts.tagList) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(ts.tagList))
	}
	// Tags should be sorted: due, risk, today
	want := []string{"due", "risk", "today"}
	for i, w := range want {
		if ts.tagList[i] != w {
			t.Errorf("tagList[%d]: want %q, got %q", i, w, ts.tagList[i])
		}
	}
}

// TestTagSearchCursorNavigation verifies Tab cycles cursor forward and wraps.
func TestTagSearchCursorNavigation(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"a", "b", "c"}})

	if ts.tagCursor != 0 {
		t.Fatalf("expected initial cursor 0, got %d", ts.tagCursor)
	}

	// Tab → cursor 1
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyTab})
	if ts.tagCursor != 1 {
		t.Errorf("after 1st Tab: want cursor 1, got %d", ts.tagCursor)
	}

	// Tab → cursor 2
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyTab})
	if ts.tagCursor != 2 {
		t.Errorf("after 2nd Tab: want cursor 2, got %d", ts.tagCursor)
	}

	// Tab → wraps to 0
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyTab})
	if ts.tagCursor != 0 {
		t.Errorf("after 3rd Tab (wrap): want cursor 0, got %d", ts.tagCursor)
	}
}

// TestTagSearchSelectEmitsMsg verifies that Enter emits TagSelectedMsg for the current tag.
func TestTagSearchSelectEmitsMsg(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due", "risk", "today"}})

	// Tags are sorted: due, risk, today. Tab to index 1 → "risk".
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyTab})

	_, cmd := ts.Update(tea.KeyMsg{Type: tea.KeyEnter})

	msgs := execBatchCmds(cmd)
	selected, ok := findMsg[TagSelectedMsg](msgs)
	if !ok {
		t.Fatal("expected TagSelectedMsg to be emitted on Enter")
	}
	if selected.Name != "risk" {
		t.Errorf("expected TagSelectedMsg.Name == \"risk\", got %q", selected.Name)
	}
}

// TestTagSearchEscapeEmitsExit verifies that Escape emits TagSearchExitMsg.
func TestTagSearchEscapeEmitsExit(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due"}})

	_, cmd := ts.Update(tea.KeyMsg{Type: tea.KeyEscape})

	msgs := execBatchCmds(cmd)
	_, ok := findMsg[TagSearchExitMsg](msgs)
	if !ok {
		t.Fatal("expected TagSearchExitMsg to be emitted on Escape")
	}
}

// TestTagSearchQuitEmitsQuit verifies that pressing 'q' returns tea.QuitMsg.
func TestTagSearchQuitEmitsQuit(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due"}})

	_, cmd := ts.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	msg := execCmd(cmd)
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

// TestTagSearchFiltering verifies that typing a character filters the tag list.
func TestTagSearchFiltering(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due", "risk", "today", "done"}})

	// Type 'd' to filter; substring match hits "done", "due", and "today" (all contain 'd').
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	filtered := ts.filteredTags()
	if len(filtered) != 3 {
		t.Fatalf("expected 3 filtered tags after 'd', got %d: %v", len(filtered), filtered)
	}

	// Both "done" and "due" should appear (sorted order: done, due).
	has := func(name string) bool {
		for _, f := range filtered {
			if f == name {
				return true
			}
		}
		return false
	}
	if !has("done") {
		t.Error("expected \"done\" in filtered tags")
	}
	if !has("due") {
		t.Error("expected \"due\" in filtered tags")
	}
}

// TestTagSearchAtPrefixStripped verifies that typing "@du" matches "due".
func TestTagSearchAtPrefixStripped(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due", "risk", "today"}})

	// Type '@', 'd', 'u' one at a time.
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'@'}})
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})

	filtered := ts.filteredTags()
	found := false
	for _, f := range filtered {
		if f == "due" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected \"due\" in filteredTags after typing \"@du\", got %v", filtered)
	}
}

// TestTagSearchViewRendersWithoutPanic verifies that View produces non-empty output.
func TestTagSearchViewRendersWithoutPanic(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due", "risk", "today"}})

	tagColors := map[string]string{
		"due":  "red",
		"risk": "yellow",
	}

	output := ts.View(tagColors, 80)
	if output == "" {
		t.Error("expected non-empty output from View")
	}
}
