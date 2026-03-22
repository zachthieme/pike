package scope

import (
	"fmt"
	"strings"
	"testing"

	"github.com/zachthieme/pike/internal/model"
)

func BenchmarkFilter(b *testing.B) {
	// Build a realistic task set: 1000 tasks, ~10% reference "Bob Smith".
	tasks := make([]model.Task, 1000)
	for i := range tasks {
		var text string
		if i%10 == 0 {
			text = fmt.Sprintf("task %d ask [[Bob Smith]] about X @talk", i)
		} else {
			text = fmt.Sprintf("task %d unrelated work @today", i)
		}
		tasks[i] = model.Task{Text: text, LowerText: strings.ToLower(text), File: fmt.Sprintf("file%d.md", i)}
	}
	ids := Identity("Bob Smith.md")

	b.ResetTimer()
	for range b.N {
		Filter(tasks, ids, "people/Bob Smith.md")
	}
}

func BenchmarkMatch(b *testing.B) {
	task := &model.Task{Text: "@delegated([[bob-smith]]) finish the report @today", LowerText: "@delegated([[bob-smith]]) finish the report @today"}
	ids := Identity("Bob Smith.md")

	b.ResetTimer()
	for range b.N {
		matchIdentities(task, ids)
	}
}
