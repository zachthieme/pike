package tui

import (
	"pike/internal/config"
	"slices"
	"testing"
)

func TestBuildKeyMap_NoOverrides(t *testing.T) {
	km := BuildKeyMap(nil, nil)
	if !slices.Contains(km.Down.Keys(), "j") {
		t.Error("Down should still match 'j' with no overrides")
	}
	if !km.FocusSection[0].Enabled() {
		t.Error("FocusSection[0] should be enabled with no custom bindings")
	}
}

func TestBuildKeyMap_Override(t *testing.T) {
	overrides := map[string][]string{
		"toggle": {"space", "x"},
	}
	km := BuildKeyMap(overrides, nil)
	if !slices.Contains(km.Toggle.Keys(), "space") {
		t.Error("Toggle should match 'space' after override")
	}
	help := km.Toggle.Help()
	if help.Key != "space/x" {
		t.Errorf("Toggle help key = %q, want 'space/x'", help.Key)
	}
}

func TestBuildKeyMap_CustomDisablesFocusSection(t *testing.T) {
	custom := []config.CustomBinding{
		{Key: "o", View: "Overdue"},
	}
	km := BuildKeyMap(nil, custom)
	for i := range 9 {
		if km.FocusSection[i].Enabled() {
			t.Errorf("FocusSection[%d] should be disabled when custom bindings exist", i)
		}
	}
}

func TestBuildKeyMap_DisableAction(t *testing.T) {
	overrides := map[string][]string{
		"quit": {},
	}
	km := BuildKeyMap(overrides, nil)
	if km.Quit.Enabled() {
		t.Error("Quit should be disabled with empty keys")
	}
}
