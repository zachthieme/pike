package tui

import (
	"github.com/zachthieme/pike/internal/config"
	"strings"
	"testing"
)

func TestRenderSummary_DefaultKeys(t *testing.T) {
	km := DefaultKeyMap()
	output := RenderSummary("v1.4.0", 80, km, nil)
	if !strings.Contains(output, "pike") {
		t.Error("should contain 'pike'")
	}
	// Check a default key appears
	if !strings.Contains(output, "move down") {
		t.Error("should contain 'move down' description")
	}
	if !strings.Contains(output, "1-9") {
		t.Error("should contain '1-9' for focus section")
	}
}

func TestRenderSummary_OverriddenKeys(t *testing.T) {
	km := BuildKeyMap(map[string][]string{"toggle": {"space"}}, nil)
	output := RenderSummary("v1.4.0", 80, km, nil)
	if !strings.Contains(output, "space") {
		t.Error("should contain overridden 'space' key")
	}
}

func TestRenderSummary_CustomBindings(t *testing.T) {
	custom := []config.CustomBinding{{Key: "o", View: "Overdue"}}
	km := BuildKeyMap(nil, custom)
	output := RenderSummary("v1.4.0", 80, km, custom)
	if !strings.Contains(output, "Shortcuts") {
		t.Error("should contain 'Shortcuts' section")
	}
	if !strings.Contains(output, "Overdue") {
		t.Error("should contain 'Overdue' custom binding")
	}
	if strings.Contains(output, "1-9") {
		t.Error("should not contain '1-9' when custom bindings exist")
	}
}

func TestRenderSummary_DisabledBindingOmitted(t *testing.T) {
	km := BuildKeyMap(map[string][]string{"quit": {}}, nil)
	output := RenderSummary("v1.4.0", 80, km, nil)
	// The word "quit" appears in the description of other actions too,
	// so check that the quit key binding row is gone by checking its help desc
	if strings.Contains(output, "quit") {
		t.Error("disabled 'quit' binding should be omitted from summary")
	}
}

func TestRenderSummary_ListsSync(t *testing.T) {
	km := DefaultKeyMap()
	output := RenderSummary("v1.4.0", 80, km, nil)
	if !strings.Contains(output, "sync with HEY") {
		t.Error("help should list the sync action")
	}
}

func TestRenderSummary_ListsCreateAndCollapse(t *testing.T) {
	km := DefaultKeyMap()
	output := RenderSummary("v1.4.0", 80, km, nil)
	if !strings.Contains(output, "new task") {
		t.Error("help should list the new-task action")
	}
	if !strings.Contains(output, "toggle collapse") {
		t.Error("help should list the collapse action")
	}
}

func TestRenderSummary_ListsCreateAndCollapseAfterRebind(t *testing.T) {
	km := BuildKeyMap(map[string][]string{
		"create_task":     {"N"},
		"toggle_collapse": {"Z"},
	}, nil)
	output := RenderSummary("v1.4.0", 80, km, nil)
	// Both rebound keys must appear beside their (unchanged) descriptions.
	for _, want := range []string{"N", "new task", "Z", "toggle collapse"} {
		if !strings.Contains(output, want) {
			t.Errorf("help should show rebound binding %q", want)
		}
	}
}
