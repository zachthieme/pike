package parser

import (
	"testing"
)

func FuzzParseLine(f *testing.F) {
	seeds := []string{
		"- [ ] task @due(2026-03-16)",
		"- [x] done @completed(2026-03-10)",
		"- bullet @today @risk",
		"- [ ] @due(bad-date) text",
		"- [ ] @due(2026/3/16) normalizable",
		"- [ ] @due(2026.03.16) dots",
		"just text no task",
		"",
		"- [ ] ",
		"- [x] @due(9999-99-99)",
		"   - [ ] indented @tag(value)",
		"  - [ ] indented task @tag",
		"    - [x] deep indented @due(2026-01-01)",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		task, warnings := ParseLine(input, "fuzz.md", 1)

		if task != nil {
			if task.Indent < 0 {
				t.Errorf("Indent = %d, want >= 0", task.Indent)
			}
			if task.Due != nil {
				if task.Due.Format("2006-01-02") == "" {
					t.Error("task.Due formatted to empty string")
				}
			}
			if task.Completed != nil {
				if task.Completed.Format("2006-01-02") == "" {
					t.Error("task.Completed formatted to empty string")
				}
			}
		}

		for _, w := range warnings {
			if w.File == "" {
				t.Error("warning has empty File")
			}
			if w.Line <= 0 {
				t.Errorf("warning has non-positive Line: %d", w.Line)
			}
			if w.Message == "" {
				t.Error("warning has empty Message")
			}
		}
	})
}
