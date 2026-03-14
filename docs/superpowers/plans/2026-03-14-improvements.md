# Pike Improvements Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add task counts in headers, pinned tasks, task toggling, a query bar (replacing the filter bar), and a recently-completed view.

**Architecture:** Five independent features built in dependency order. Task counts and pinned tasks touch rendering/sorting. Task toggling adds a new `internal/toggle` package for file writes. The query bar replaces `parseFilterTokens`/`matchesFilter` with DSL parsing + fallback. Recently-completed adds a new view mode that depends on both the query bar and toggle.

**Tech Stack:** Go, bubbletea, lipgloss, existing query DSL parser/evaluator

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/tui/sections.go` | Modify `renderSection()` — add task count to header |
| `internal/sort/sort.go` | Add `StablePartitionPinned()` |
| `internal/sort/sort_test.go` | Test `StablePartitionPinned()` |
| `internal/filter/filter.go` | Call `StablePartitionPinned()` after sorting in `Apply` |
| `internal/toggle/toggle.go` | New: `Complete`/`Uncomplete` file operations |
| `internal/toggle/toggle_test.go` | New: unit tests for toggle |
| `internal/query/eval.go` | Add `EvalOptions`/`EvalWithOptions` for partial tag matching |
| `internal/query/eval_test.go` | Add tests for partial tag matching |
| `internal/config/config.go` | Add `RecentlyCompletedDays` field |
| `internal/config/config_test.go` | Test `RecentlyCompletedDays` default |
| `internal/tui/model.go` | Add `modeRecentlyCompleted`, `queryErr` field |
| `internal/tui/keymap.go` | Add `Toggle`, `RecentlyCompleted` key bindings |
| `internal/tui/keys.go` | Handle `x`, `c` keys; `modeRecentlyCompleted` routing |
| `internal/tui/tasks.go` | Replace filter with DSL parsing, add mode entry methods, apply pinning |
| `internal/tui/views.go` | Route `modeRecentlyCompleted`, render `queryErr` |
| `internal/tui/model_test.go` | Tests for all new TUI behavior |

---

### Task 1: Task counts in section headers

**Files:**
- Modify: `internal/tui/sections.go:57-60`
- Test: `internal/tui/model_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/model_test.go`:

```go
func TestSectionHeaderShowsTaskCount(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.width = 80

	view := m.View()
	// The "Open" section has 3 open tasks (overdue, today, future)
	if !strings.Contains(view, "Open (3)") {
		t.Errorf("expected section header to show count, got:\n%s", view)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestSectionHeaderShowsTaskCount -v`
Expected: FAIL — header shows `Open` without count.

- [ ] **Step 3: Implement task count in header**

In `internal/tui/sections.go`, replace lines 57-60:

```go
	headerLabel := fmt.Sprintf(" %s ", title)
	if hiddenCount > 0 {
		headerLabel = fmt.Sprintf(" %s 🔒", title)
	}
```

With:

```go
	headerLabel := fmt.Sprintf(" %s (%d) ", title, len(tasks))
	if hiddenCount > 0 {
		headerLabel = fmt.Sprintf(" %s (%d) 🔒", title, len(tasks))
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/ -run TestSectionHeaderShowsTaskCount -v`
Expected: PASS

- [ ] **Step 5: Run all existing tests to check for regressions**

Run: `go test ./internal/tui/ -v`
Expected: Some tests may fail because they check exact view output. Fix any that check for header text without count by updating expectations.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/sections.go internal/tui/model_test.go
git commit -m "feat: show task count in section headers"
```

---

### Task 2: Pinned tasks (`@pin`)

**Files:**
- Modify: `internal/sort/sort.go`
- Test: `internal/sort/sort_test.go`
- Modify: `internal/filter/filter.go`
- Modify: `internal/tui/tasks.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/sort/sort_test.go`:

```go
func TestStablePartitionPinned(t *testing.T) {
	tasks := []model.Task{
		{Text: "unpinned-a"},
		{Text: "pinned-b", Tags: []model.Tag{{Name: "pin"}}},
		{Text: "unpinned-c"},
		{Text: "pinned-d", Tags: []model.Tag{{Name: "pin"}}},
		{Text: "unpinned-e"},
	}

	result := StablePartitionPinned(tasks)

	wantTexts := []string{"pinned-b", "pinned-d", "unpinned-a", "unpinned-c", "unpinned-e"}
	if len(result) != len(wantTexts) {
		t.Fatalf("got %d tasks, want %d", len(result), len(wantTexts))
	}
	for i, want := range wantTexts {
		if result[i].Text != want {
			t.Errorf("result[%d].Text = %q, want %q", i, result[i].Text, want)
		}
	}
}

func TestStablePartitionPinnedNoPins(t *testing.T) {
	tasks := []model.Task{
		{Text: "a"},
		{Text: "b"},
	}

	result := StablePartitionPinned(tasks)
	if result[0].Text != "a" || result[1].Text != "b" {
		t.Error("expected order unchanged when no pins")
	}
}

func TestStablePartitionPinnedEmpty(t *testing.T) {
	result := StablePartitionPinned(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sort/ -run TestStablePartitionPinned -v`
Expected: Compilation error — `StablePartitionPinned` undefined.

- [ ] **Step 3: Implement StablePartitionPinned**

Add to `internal/sort/sort.go`:

```go
// StablePartitionPinned returns a new slice with @pin tasks first,
// preserving relative order within each group.
func StablePartitionPinned(tasks []model.Task) []model.Task {
	if len(tasks) == 0 {
		return tasks
	}
	result := make([]model.Task, 0, len(tasks))
	var unpinned []model.Task
	for _, t := range tasks {
		if t.HasTag("pin") {
			result = append(result, t)
		} else {
			unpinned = append(unpinned, t)
		}
	}
	result = append(result, unpinned...)
	return result
}
```

- [ ] **Step 4: Run sort tests**

Run: `go test ./internal/sort/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Add pinning to filter.Apply**

In `internal/filter/filter.go`, add import `tasksort` is already imported as `tasksort "pike/internal/sort"`. After the sort block (after line 43), add:

```go
	matched = tasksort.StablePartitionPinned(matched)
```

So the function body after sorting becomes:

```go
	// Sort results if a sort order is specified.
	if sortOrder != "" {
		if err := tasksort.Sort(matched, sortOrder); err != nil {
			return nil, err
		}
	}

	matched = tasksort.StablePartitionPinned(matched)

	return matched, nil
```

- [ ] **Step 6: Add pinning to modeAllTasks in rebuildSections**

In `internal/tui/tasks.go`, in the `modeAllTasks` branch of `rebuildSections()`, after the filter block and before creating the section (around line 39-49), add:

```go
		tasks = tasksort.StablePartitionPinned(tasks)
```

Add import `tasksort "pike/internal/sort"` to the import block if not already present.

- [ ] **Step 7: Run all tests**

Run: `go test ./... 2>&1 | grep -E "FAIL|ok"`
Expected: All pass (except the pre-existing `TestLoad_ImplicitMissingFileReturnsDefaults`).

- [ ] **Step 8: Commit**

```bash
git add internal/sort/sort.go internal/sort/sort_test.go internal/filter/filter.go internal/tui/tasks.go
git commit -m "feat: add @pin tag to float tasks to top of sections"
```

---

### Task 3: Task toggling (`x` key)

**Files:**
- Create: `internal/toggle/toggle.go`
- Create: `internal/toggle/toggle_test.go`
- Modify: `internal/tui/keymap.go`
- Modify: `internal/tui/keys.go`
- Test: `internal/tui/model_test.go`

- [ ] **Step 1: Write the toggle test file**

Create `internal/toggle/toggle_test.go`:

```go
package toggle

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(data)
}

func TestCompleteBasic(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "# Notes\n- [ ] Buy groceries\n- [ ] Clean house\n")
	date := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)

	err := Complete(p, 2, date)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got := readFile(t, p)
	want := "# Notes\n- [x] Buy groceries @completed(2026-03-14)\n- [ ] Clean house\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompleteIndented(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "  - [ ] Indented task\n")
	date := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)

	err := Complete(p, 1, date)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got := readFile(t, p)
	want := "  - [x] Indented task @completed(2026-03-14)\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompleteWrongLineContent(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "# Just a heading\n")

	err := Complete(p, 1, time.Now())
	if err == nil {
		t.Fatal("expected error for non-checkbox line")
	}
}

func TestCompleteLineOutOfRange(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Only line\n")

	err := Complete(p, 5, time.Now())
	if err == nil {
		t.Fatal("expected error for out-of-range line")
	}
}

func TestUncompleteBasic(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [x] Done task @completed(2026-03-14)\n")

	err := Uncomplete(p, 1)
	if err != nil {
		t.Fatalf("Uncomplete: %v", err)
	}

	got := readFile(t, p)
	want := "- [ ] Done task\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUncompleteWithoutDate(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [x] Done task @completed\n")

	err := Uncomplete(p, 1)
	if err != nil {
		t.Fatalf("Uncomplete: %v", err)
	}

	got := readFile(t, p)
	want := "- [ ] Done task\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUncompleteIndented(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "  - [x] Indented @completed(2026-03-14)\n")

	err := Uncomplete(p, 1)
	if err != nil {
		t.Fatalf("Uncomplete: %v", err)
	}

	got := readFile(t, p)
	want := "  - [ ] Indented\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUncompleteWrongLineContent(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Still open\n")

	err := Uncomplete(p, 1)
	if err == nil {
		t.Fatal("expected error for non-completed line")
	}
}

func TestUncompletePreservesOtherTags(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [x] Task @today @completed(2026-03-14) @risk\n")

	err := Uncomplete(p, 1)
	if err != nil {
		t.Fatalf("Uncomplete: %v", err)
	}

	got := readFile(t, p)
	want := "- [ ] Task @today @risk\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/toggle/ -v`
Expected: Compilation error — package does not exist.

- [ ] **Step 3: Implement the toggle package**

Create `internal/toggle/toggle.go`:

```go
package toggle

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var completedTagRe = regexp.MustCompile(`\s*@completed(\([^)]*\))?(?:\s|$)`)

// Complete marks an open checkbox task as completed by modifying the source file.
// Replaces - [ ] with - [x] and appends @completed(YYYY-MM-DD).
// Returns an error if the line doesn't contain - [ ] (stale data).
func Complete(filePath string, line int, date time.Time) error {
	lines, err := readLines(filePath)
	if err != nil {
		return err
	}
	if line < 1 || line > len(lines) {
		return fmt.Errorf("line %d out of range (file has %d lines)", line, len(lines))
	}

	idx := line - 1
	l := lines[idx]
	if !strings.Contains(l, "- [ ]") {
		return fmt.Errorf("line %d does not contain '- [ ]': %q", line, l)
	}

	l = strings.Replace(l, "- [ ]", "- [x]", 1)
	l += fmt.Sprintf(" @completed(%s)", date.Format("2006-01-02"))
	lines[idx] = l

	return writeLines(filePath, lines)
}

// Uncomplete marks a completed checkbox task as open by modifying the source file.
// Replaces - [x] with - [ ] and removes @completed(...) tag.
// Returns an error if the line doesn't contain - [x] (stale data).
func Uncomplete(filePath string, line int) error {
	lines, err := readLines(filePath)
	if err != nil {
		return err
	}
	if line < 1 || line > len(lines) {
		return fmt.Errorf("line %d out of range (file has %d lines)", line, len(lines))
	}

	idx := line - 1
	l := lines[idx]
	if !strings.Contains(l, "- [x]") {
		return fmt.Errorf("line %d does not contain '- [x]': %q", line, l)
	}

	l = strings.Replace(l, "- [x]", "- [ ]", 1)

	// Remove @completed(...) tag. The regex may match mid-line or at end.
	// If the match ends with whitespace, replace with a single space to
	// avoid joining adjacent content. If at end of string, remove entirely.
	l = completedTagRe.ReplaceAllStringFunc(l, func(match string) string {
		if strings.HasSuffix(match, " ") || strings.HasSuffix(match, "\t") {
			return " "
		}
		return ""
	})
	l = strings.TrimRight(l, " \t")

	lines[idx] = l

	return writeLines(filePath, lines)
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := string(data)
	// Preserve trailing newline handling
	if strings.HasSuffix(s, "\n") {
		s = s[:len(s)-1]
	}
	return strings.Split(s, "\n"), nil
}

func writeLines(path string, lines []string) error {
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}
```

- [ ] **Step 4: Run toggle tests**

Run: `go test ./internal/toggle/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Add Toggle keybinding**

In `internal/tui/keymap.go`, add to `KeyMap` struct (after `ToggleHidden`):

```go
	Toggle key.Binding
```

In `DefaultKeyMap()`, add after the `ToggleHidden` binding:

```go
		Toggle: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "toggle complete"),
		),
```

- [ ] **Step 6: Add toggle key handler**

In `internal/tui/keys.go`, add import `"pike/internal/toggle"` and `"time"` to the import block.

In the normal mode switch block (after the `Refresh` case, around line 153), add:

```go
	case key.Matches(msg, m.keys.Toggle):
		return m.toggleTask()
```

In the filtering block, add a case before the `default` (around line 53):

```go
	case key.Matches(msg, m.keys.Toggle):
		return m.toggleTask()
```

Add the `toggleTask` method to `keys.go` (after `openEditor`):

```go
// toggleTask completes or uncompletes the task at the cursor.
func (m Model) toggleTask() (tea.Model, tea.Cmd) {
	flatTasks := m.flatTasks()
	if len(flatTasks) == 0 || m.cursor >= len(flatTasks) {
		return m, nil
	}
	task := flatTasks[m.cursor]
	if !task.HasCheckbox {
		return m, nil
	}

	filePath := task.File
	if m.config != nil && m.config.NotesDir != "" {
		filePath = filepath.Join(m.config.NotesDir, task.File)
	}

	var err error
	if task.State == model.Open {
		now := m.nowFunc()
		err = toggle.Complete(filePath, task.Line, now)
	} else {
		err = toggle.Uncomplete(filePath, task.Line)
	}
	if err != nil {
		m.err = err
		return m, func() tea.Msg { return RefreshMsg{} }
	}

	return m, func() tea.Msg { return RefreshMsg{} }
}
```

Add imports for `"pike/internal/model"` and `"pike/internal/toggle"` if not already present.

- [ ] **Step 7: Run all TUI tests**

Run: `go test ./internal/tui/ -v`
Expected: All tests PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/toggle/toggle.go internal/toggle/toggle_test.go internal/tui/keymap.go internal/tui/keys.go
git commit -m "feat: add x key to toggle task completion in source files"
```

---

### Task 4: Query bar — EvalWithOptions for partial tag matching

**Files:**
- Modify: `internal/query/eval.go`
- Test: `internal/query/eval_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/query/eval_test.go`:

```go
func TestEvalWithOptionsPartialTagMatch(t *testing.T) {
	task := &model.Task{
		Text: "Task @due(2026-03-15) @duration(2h)",
		Tags: []model.Tag{
			{Name: "due", Value: "2026-03-15"},
			{Name: "duration", Value: "2h"},
		},
	}
	opts := EvalOptions{PartialTags: true}

	// "du" should match both "due" and "duration"
	if !EvalWithOptions(&TagNode{Name: "du"}, task, now, opts) {
		t.Error("partial tag @du should match task with @due")
	}

	// Exact match still works
	if !EvalWithOptions(&TagNode{Name: "due"}, task, now, opts) {
		t.Error("exact tag @due should match")
	}

	// No match
	if EvalWithOptions(&TagNode{Name: "risk"}, task, now, opts) {
		t.Error("@risk should not match")
	}
}

func TestEvalWithOptionsExactByDefault(t *testing.T) {
	task := &model.Task{
		Text: "Task @due(2026-03-15)",
		Tags: []model.Tag{{Name: "due", Value: "2026-03-15"}},
	}

	// Without PartialTags, "du" should NOT match "due"
	opts := EvalOptions{PartialTags: false}
	if EvalWithOptions(&TagNode{Name: "du"}, task, now, opts) {
		t.Error("without PartialTags, @du should not match @due")
	}

	// Original Eval should still be exact
	if Eval(&TagNode{Name: "du"}, task, now) {
		t.Error("Eval should use exact matching")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/query/ -run TestEvalWithOptions -v`
Expected: Compilation error — `EvalOptions` and `EvalWithOptions` undefined.

- [ ] **Step 3: Implement EvalWithOptions**

In `internal/query/eval.go`, add after the imports:

```go
// EvalOptions configures evaluation behavior.
type EvalOptions struct {
	PartialTags bool // When true, @tag matches any tag containing the name as substring
}
```

Add `EvalWithOptions` function after `Eval`:

```go
// EvalWithOptions evaluates an AST node against a task with configurable options.
func EvalWithOptions(node Node, task *model.Task, now time.Time, opts EvalOptions) bool {
	switch n := node.(type) {
	case *OpenNode:
		return task.State == model.Open
	case *CompletedNode:
		return task.State == model.Completed
	case *TagNode:
		if opts.PartialTags {
			return hasTagPartial(task, n.Name)
		}
		return task.HasTag(n.Name)
	case *DateCmpNode:
		return evalDateCmp(n, task, now)
	case *RegexNode:
		return n.CompiledRe.MatchString(task.Text)
	case *AndNode:
		return EvalWithOptions(n.Left, task, now, opts) && EvalWithOptions(n.Right, task, now, opts)
	case *OrNode:
		return EvalWithOptions(n.Left, task, now, opts) || EvalWithOptions(n.Right, task, now, opts)
	case *NotNode:
		return !EvalWithOptions(n.Expr, task, now, opts)
	default:
		return false
	}
}

// hasTagPartial returns true if any tag name contains the query as a substring (case-insensitive).
func hasTagPartial(task *model.Task, name string) bool {
	lower := strings.ToLower(name)
	for _, tag := range task.Tags {
		if strings.Contains(strings.ToLower(tag.Name), lower) {
			return true
		}
	}
	return false
}
```

Add `"strings"` to the import block.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/query/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/query/eval.go internal/query/eval_test.go
git commit -m "feat: add EvalWithOptions with partial tag matching"
```

---

### Task 5: Query bar — replace filter bar with DSL parsing

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/tasks.go`
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/keys.go`
- Test: `internal/tui/model_test.go`

- [ ] **Step 1: Add queryErr field to Model**

In `internal/tui/model.go`, add to the Model struct (after `err error`):

```go
	queryErr    error  // DSL parse error for display in footer
```

- [ ] **Step 2: Replace filter logic in tasks.go**

In `internal/tui/tasks.go`:

1. Add imports: `"pike/internal/query"` and `"strings"` (if not present).

2. Delete `filterToken` type, `parseFilterTokens()`, and `matchesFilter()` functions (lines 114-169).

3. Add the new filtering functions:

```go
// hasDSLTokens checks if input contains DSL-specific tokens that distinguish
// it from a plain text search. Uses word-boundary matching for keywords.
func hasDSLTokens(input string) bool {
	if strings.ContainsAny(input, "@<>/") {
		return true
	}
	for _, word := range strings.Fields(input) {
		lower := strings.ToLower(word)
		if lower == "and" || lower == "or" || lower == "not" || lower == "open" || lower == "completed" {
			return true
		}
	}
	return false
}

// applyQueryFilter filters tasks using DSL parsing with fallback to substring matching.
// Returns (filtered tasks, parse error or nil).
// When DSL parsing fails and input has DSL tokens, returns (nil, error) to signal
// the caller should preserve existing sections.
func applyQueryFilter(tasks []model.Task, filterText string, now time.Time) ([]model.Task, error) {
	// Try DSL parsing first
	node, err := query.Parse(filterText)
	if err == nil {
		opts := query.EvalOptions{PartialTags: true}
		var filtered []model.Task
		for _, t := range tasks {
			if query.EvalWithOptions(node, &t, now, opts) {
				filtered = append(filtered, t)
			}
		}
		return filtered, nil
	}

	// DSL parse failed — check if input looks like DSL
	if hasDSLTokens(filterText) {
		return nil, err // signal to preserve existing sections
	}

	// Fallback: simple substring matching (space-separated tokens, ANDed)
	tokens := strings.Fields(strings.ToLower(filterText))
	var filtered []model.Task
	for _, t := range tasks {
		lower := strings.ToLower(t.Text)
		match := true
		for _, tok := range tokens {
			if !strings.Contains(lower, tok) {
				match = false
				break
			}
		}
		if match {
			filtered = append(filtered, t)
		}
	}
	return filtered, nil
}
```

4. Update `rebuildSections()` to use the new filter. Replace the filter application blocks.

In the `modeAllTasks` branch (around lines 30-39), replace the filter block:

```go
		if m.filterText != "" {
			tokens := parseFilterTokens(strings.Fields(m.filterText))
			var filtered []model.Task
			for _, t := range tasks {
				if matchesFilter(t, tokens) {
					filtered = append(filtered, t)
				}
			}
			tasks = filtered
		}
```

With:

```go
		if m.filterText != "" {
			now := m.nowFunc()
			filtered, err := applyQueryFilter(tasks, m.filterText, now)
			if err != nil {
				m.queryErr = err
				return // preserve existing sections
			}
			m.queryErr = nil
			tasks = filtered
		} else {
			m.queryErr = nil
		}
```

In the dashboard filter block (around lines 67-81), replace:

```go
	if m.filterText != "" {
		tokens := parseFilterTokens(strings.Fields(m.filterText))
		for i := range results {
			var filtered []model.Task
			for _, t := range results[i].Tasks {
				if matchesFilter(t, tokens) {
					filtered = append(filtered, t)
				}
			}
			if filtered == nil {
				filtered = []model.Task{}
			}
			results[i].Tasks = filtered
		}
	}
```

With:

```go
	if m.filterText != "" {
		now := m.nowFunc()
		node, err := query.Parse(m.filterText)
		if err == nil {
			m.queryErr = nil
			opts := query.EvalOptions{PartialTags: true}
			for i := range results {
				var filtered []model.Task
				for _, t := range results[i].Tasks {
					if query.EvalWithOptions(node, &t, now, opts) {
						filtered = append(filtered, t)
					}
				}
				if filtered == nil {
					filtered = []model.Task{}
				}
				results[i].Tasks = filtered
			}
		} else if hasDSLTokens(m.filterText) {
			m.queryErr = err
			return // preserve existing sections
		} else {
			m.queryErr = nil
			// Fallback: simple substring
			tokens := strings.Fields(strings.ToLower(m.filterText))
			for i := range results {
				var filtered []model.Task
				for _, t := range results[i].Tasks {
					lower := strings.ToLower(t.Text)
					match := true
					for _, tok := range tokens {
						if !strings.Contains(lower, tok) {
							match = false
							break
						}
					}
					if match {
						filtered = append(filtered, t)
					}
				}
				if filtered == nil {
					filtered = []model.Task{}
				}
				results[i].Tasks = filtered
			}
		}
	} else {
		m.queryErr = nil
	}
```

5. Update all prompt strings. In `clearFilter()` (around line 400):

Change `m.filterInput.Prompt = "/ "` — this is already `"/ "`, keep it.

In `enterAllTasksMode()` (around line 413):

Change `m.filterInput.Prompt = "> "` to `m.filterInput.Prompt = "/ "`.

In `enterTagSearchMode()` (around line 429):

Change `m.filterInput.Prompt = "> "` to `m.filterInput.Prompt = "/ "`.

- [ ] **Step 3: Render queryErr in footer**

In `internal/tui/views.go`, in `viewDashboard()` (around line 43), after the footer line, add error display:

```go
	if m.queryErr != nil {
		footer += "\n" + FooterStyle().Render("  "+m.queryErr.Error())
	}
```

In `viewAllTasks()`, after the footer line (around line 128), add:

```go
	if m.queryErr != nil {
		parts = append(parts, FooterStyle().Render("  "+m.queryErr.Error()))
	}
```

- [ ] **Step 4: Write tests for query bar**

Add to `internal/tui/model_test.go`:

```go
func TestQueryBarDSLFilter(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Activate filter and type a DSL query
	updated, _ := sendKey(m, "/")
	m = updated.(Model)
	for _, ch := range "open and @today" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}

	flat := m.flatTasks()
	for _, task := range flat {
		if task.State != model.Open || !task.HasTag("today") {
			t.Errorf("expected only open @today tasks, got %q", task.Text)
		}
	}
}

func TestQueryBarFallbackSubstring(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Type a simple substring (no DSL tokens)
	updated, _ := sendKey(m, "/")
	m = updated.(Model)
	for _, ch := range "overdue" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}

	flat := m.flatTasks()
	for _, task := range flat {
		if !strings.Contains(strings.ToLower(task.Text), "overdue") {
			t.Errorf("expected substring match, got %q", task.Text)
		}
	}
}

func TestQueryBarPartialTag(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Type partial tag @du — should match @due via DSL
	updated, _ := sendKey(m, "/")
	m = updated.(Model)
	for _, ch := range "@du" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}

	flat := m.flatTasks()
	if len(flat) == 0 {
		t.Fatal("expected partial tag @du to match tasks with @due")
	}
	for _, task := range flat {
		if !task.HasTag("due") {
			t.Errorf("expected @due tag, got %q", task.Text)
		}
	}
}
```

- [ ] **Step 5: Run all tests**

Run: `go test ./internal/tui/ -v`
Expected: All tests PASS. Some old filter tests may need updating if they relied on `!` negation or `!@tag` syntax (which is now removed). Update those tests to use DSL syntax instead (e.g., `not @tag`).

- [ ] **Step 6: Fix any broken tests**

The following existing tests may break and need updating:
- `TestFilterPartialTagMatch` — should still work since `@du` is now DSL with partial matching
- `TestFilterFullTagMatch` — should still work since `@today` is valid DSL
- `TestNegationWithPartialTag` — uses `!@du` which is no longer supported. Update to use DSL: type `not @du` instead of `!@du`

For `TestNegationWithPartialTag`, change the input from `"!@du"` to `"not @du"` and update expectations accordingly.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/model.go internal/tui/tasks.go internal/tui/views.go internal/tui/keys.go internal/tui/model_test.go
git commit -m "feat: replace filter bar with query bar supporting full DSL"
```

---

### Task 6: Config — add RecentlyCompletedDays

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestLoadBytes_DefaultRecentlyCompletedDays(t *testing.T) {
	yaml := `notes_dir: ~/Notes`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RecentlyCompletedDays != 7 {
		t.Errorf("RecentlyCompletedDays = %d, want 7", cfg.RecentlyCompletedDays)
	}
}

func TestLoadBytes_ExplicitRecentlyCompletedDays(t *testing.T) {
	yaml := `
notes_dir: ~/Notes
recently_completed_days: 14
`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RecentlyCompletedDays != 14 {
		t.Errorf("RecentlyCompletedDays = %d, want 14", cfg.RecentlyCompletedDays)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoadBytes_DefaultRecentlyCompleted -v`
Expected: Compilation error.

- [ ] **Step 3: Implement**

In `internal/config/config.go`:

Add to `Config` struct (after `LinkColor`): `RecentlyCompletedDays int \`yaml:"-"\``

Add to `rawConfig` struct (after `LinkColor`): `RecentlyCompletedDays *int \`yaml:"recently_completed_days"\``

Add to `applyDefaults` (after LinkColor block):

```go
	// RecentlyCompletedDays: default to 7
	if raw.RecentlyCompletedDays != nil {
		cfg.RecentlyCompletedDays = *raw.RecentlyCompletedDays
	} else {
		cfg.RecentlyCompletedDays = 7
	}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/config/ -run TestLoadBytes -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add RecentlyCompletedDays config field (default 7)"
```

---

### Task 7: Recently completed view (`c` key)

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/keymap.go`
- Modify: `internal/tui/keys.go`
- Modify: `internal/tui/tasks.go`
- Modify: `internal/tui/views.go`
- Test: `internal/tui/model_test.go`

- [ ] **Step 1: Add modeRecentlyCompleted**

In `internal/tui/model.go`, add to the viewMode const block (after `modeTagSearch`):

```go
	modeRecentlyCompleted
```

- [ ] **Step 2: Add RecentlyCompleted keybinding**

In `internal/tui/keymap.go`, add to `KeyMap` struct:

```go
	RecentlyCompleted key.Binding
```

In `DefaultKeyMap()`, add:

```go
		RecentlyCompleted: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "recently completed"),
		),
```

- [ ] **Step 3: Add enterRecentlyCompletedMode to tasks.go**

In `internal/tui/tasks.go`, add after `enterTagSearchMode`:

```go
// enterRecentlyCompletedMode switches to recently-completed view with a pre-filled query.
func (m *Model) enterRecentlyCompletedMode() tea.Cmd {
	days := 7
	if m.config != nil && m.config.RecentlyCompletedDays >= 0 {
		days = m.config.RecentlyCompletedDays
	}
	queryStr := fmt.Sprintf("completed and @completed >= today-%dd", days)

	m.mode = modeRecentlyCompleted
	m.filtering = true
	m.filterInput.SetValue(queryStr)
	m.filterText = queryStr
	m.filterInput.Prompt = "/ "
	m.filterInput.Placeholder = "type to filter..."
	m.cursor = 0
	m.rebuildSections()
	m.clampCursor()
	return m.filterInput.Focus()
}
```

- [ ] **Step 4: Handle modeRecentlyCompleted in rebuildSections**

In `internal/tui/tasks.go`, in `rebuildSections()`, add a new branch before the dashboard/view branch. After the `modeAllTasks` block (around line 53), add:

```go
	if m.mode == modeRecentlyCompleted {
		// Collect all tasks (any state, any type) — query will filter
		tasks := make([]model.Task, len(m.allTasks))
		copy(tasks, m.allTasks)

		if m.filterText != "" {
			now := m.nowFunc()
			filtered, err := applyQueryFilter(tasks, m.filterText, now)
			if err != nil {
				m.queryErr = err
				return
			}
			m.queryErr = nil
			tasks = filtered
		} else {
			m.queryErr = nil
		}

		tasks = tasksort.StablePartitionPinned(tasks)

		sections := []filter.ViewResult{{
			Title: "Recently Completed",
			Color: "blue",
			Tasks: tasks,
		}}
		m.sections = sections
		m.applyHiddenFilter()
		return
	}
```

- [ ] **Step 5: Route modeRecentlyCompleted in views and keys**

In `internal/tui/views.go`, add to the `View()` switch (after `modeTagSearch`):

```go
	case modeRecentlyCompleted:
		return errLine + m.viewAllTasks()
```

In `internal/tui/keys.go`:

Add `c` key handler in normal mode switch (after `TagSearch` case):

```go
	case key.Matches(msg, m.keys.RecentlyCompleted):
		if m.mode == modeRecentlyCompleted {
			return m, nil // no-op if already in this mode
		}
		focusCmd := m.enterRecentlyCompletedMode()
		return m, tea.Batch(focusCmd, func() tea.Msg { return tea.ClearScreen() })
```

In the filtering block's Escape handler (around line 22-28), add `modeRecentlyCompleted` to the mode exit check:

```go
		if m.mode == modeAllTasks || m.mode == modeRecentlyCompleted {
			m.mode = modeDashboard
		}
```

Also in the filtering block's "backspace to empty returns to tag search" logic (around line 59-61), add a guard so it doesn't apply to recently-completed mode:

```go
		if m.showAll && m.filterText == "" && m.mode != modeRecentlyCompleted {
			m.enterTagSearchMode()
			return m, cmd
		}
```

- [ ] **Step 6: Write tests**

Add to `internal/tui/model_test.go`:

```go
func TestRecentlyCompletedMode(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendKey(m, "c")
	m = updated.(Model)

	if m.mode != modeRecentlyCompleted {
		t.Errorf("expected modeRecentlyCompleted, got %d", m.mode)
	}
	if !m.filtering {
		t.Error("expected filtering to be true")
	}
	if !strings.Contains(m.filterText, "completed and @completed") {
		t.Errorf("expected pre-filled query, got %q", m.filterText)
	}
}

func TestRecentlyCompletedEscapeReturnsToDashboard(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendKey(m, "c")
	m = updated.(Model)

	updated, _ = sendSpecialKey(m, tea.KeyEscape)
	m = updated.(Model)

	if m.mode != modeDashboard {
		t.Errorf("expected modeDashboard after Esc, got %d", m.mode)
	}
}

func TestRecentlyCompletedNoOpWhenAlreadyActive(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendKey(m, "c")
	m = updated.(Model)

	// Press c again — should be no-op
	updated, _ = sendKey(m, "c")
	m2 := updated.(Model)
	if m2.mode != modeRecentlyCompleted {
		t.Errorf("expected to stay in modeRecentlyCompleted, got %d", m2.mode)
	}
}
```

- [ ] **Step 7: Run all tests**

Run: `go test ./... 2>&1 | grep -E "FAIL|ok"`
Expected: All pass.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/model.go internal/tui/keymap.go internal/tui/keys.go internal/tui/tasks.go internal/tui/views.go internal/tui/model_test.go
git commit -m "feat: add recently-completed view with c key"
```

---

### Task 8: Update README and config

**Files:**
- Modify: `README.md`
- Modify: `config.yaml`

- [ ] **Step 1: Update README**

In `README.md`:

1. Add to the **Actions** keybinding table:
```
| `x` | Toggle task complete/incomplete |
| `f` | Search files — full-text search with preview |
| `c` | Recently completed tasks |
```
Wait — `f` (search) was abandoned. Only add `x` and `c`:
```
| `x` | Toggle task complete/incomplete |
| `c` | Recently completed tasks |
```

2. Update the **Filter Mode** section header to **Query Mode** and note that it accepts full DSL syntax or plain text.

3. Add `@pin` to the **Special Tags** table:
```
| `@pin` | Floats the task to the top of its section |
```

4. Add config fields to the config example:
```yaml
# Days to show in recently-completed view (default: 7)
recently_completed_days: 7
```

5. Note task counts in headers in the **Display** section.

- [ ] **Step 2: Update config.yaml**

Add to `config.yaml`:
```yaml
recently_completed_days: 7
```

- [ ] **Step 3: Run all tests one final time**

Run: `go test ./...`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add README.md config.yaml
git commit -m "docs: add toggle, pinned, query bar, recently-completed to README"
```
