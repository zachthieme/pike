package tui

import (
	"strings"
	"testing"

	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/style"
)

// TestFormatTaskLineHidesHeyTag verifies the @hey(id) Link tag is not rendered
// in the TUI, while other tags on the same line survive unaffected.
func TestFormatTaskLineHidesHeyTag(t *testing.T) {
	task := model.TaskWith(model.Task{
		Text:        "Ship release @hey(123456789) @risk",
		State:       model.Open,
		File:        "work/tasks.md",
		Line:        1,
		Tags:        []model.Tag{{Name: "hey", Value: "123456789"}, {Name: "risk"}},
		HasCheckbox: true,
	})
	tagColors := map[string]string{"risk": "red"}

	got := style.StripANSI(formatTaskLine(task, tagColors, "", false))

	if strings.Contains(got, "hey") || strings.Contains(got, "123456789") {
		t.Errorf("formatTaskLine() should hide @hey tag, got %q", got)
	}
	if !strings.Contains(got, "@risk") {
		t.Errorf("formatTaskLine() should keep @risk tag, got %q", got)
	}
	if strings.Contains(got, "  ") {
		t.Errorf("formatTaskLine() should not leave doubled spaces, got %q", got)
	}
}
