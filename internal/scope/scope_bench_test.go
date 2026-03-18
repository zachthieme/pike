package scope

import (
	"fmt"
	"testing"

	"pike/internal/model"
)

func BenchmarkFilter(b *testing.B) {
	// Build a realistic task set: 1000 tasks, ~10% reference "Bob Smith".
	tasks := make([]model.Task, 1000)
	for i := range tasks {
		if i%10 == 0 {
			tasks[i] = model.Task{Text: fmt.Sprintf("task %d ask [[Bob Smith]] about X @talk", i), File: fmt.Sprintf("file%d.md", i)}
		} else {
			tasks[i] = model.Task{Text: fmt.Sprintf("task %d unrelated work @today", i), File: fmt.Sprintf("file%d.md", i)}
		}
	}
	ids := Identity("Bob Smith.md")

	b.ResetTimer()
	for range b.N {
		Filter(tasks, ids, "people/Bob Smith.md")
	}
}

func BenchmarkMatch(b *testing.B) {
	task := &model.Task{Text: "@delegated([[bob-smith]]) finish the report @today"}
	ids := Identity("Bob Smith.md")

	b.ResetTimer()
	for range b.N {
		Match(task, ids)
	}
}
