# Filter Mode Split: `/` Substring vs `?` Query

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the filter bar into two explicit modes — `/` for substring matching and `?` for DSL queries — with the prompt character reflecting the active mode.

**Architecture:** Add a `filterMode` enum to `Model` (`filterSubstring` / `filterQuery`). Each entry point sets the mode and prompt accordingly. The filter application logic branches on mode instead of using the current try-DSL-then-fallback approach.

**Tech Stack:** Go, Bubbletea, bubbles/textinput

---

## Chunk 1: Implementation

### Task 1: Add filterMode type and model field

**Files:**
- Modify: `internal/tui/model.go`

- [ ] **Step 1: Add the filterMode type and field**

Add after the `viewMode` constants (around line 29):

```go
// filterMode tracks whether the filter bar uses substring or DSL matching.
type filterMode int

const (
	filterSubstring filterMode = iota
	filterQuery
)
```

Add `filterMode filterMode` field to the `Model` struct, after the `filterText` field (line 42):

```go
	filterText  string
	filterMode  filterMode
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/tui/`

---

### Task 2: Add `?` keybinding

**Files:**
- Modify: `internal/tui/keymap.go`

- [ ] **Step 1: Add Query key binding to KeyMap struct**

Add `Query` to the struct (line 11):

```go
	Filter, Query, Escape, Refresh        key.Binding
```

- [ ] **Step 2: Add Query binding in DefaultKeyMap**

After the Filter binding (line 58):

```go
		Query: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "query"),
		),
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/tui/`

---

### Task 3: Update key handlers to set filterMode

**Files:**
- Modify: `internal/tui/keys.go`

- [ ] **Step 1: Handle `?` key in the main key handler**

In the non-filtering switch block in `handleKey` (after the `Filter` case around line 201-204), add:

```go
	case key.Matches(msg, m.keys.Query):
		m.filtering = true
		m.filterMode = filterQuery
		m.filterInput.Prompt = "? "
		cmd := m.filterInput.Focus()
		return m, cmd
```

- [ ] **Step 2: Set filterMode to filterSubstring when `/` is pressed**

Update the existing `Filter` case (line 201-204) to explicitly set the mode:

```go
	case key.Matches(msg, m.keys.Filter):
		m.filtering = true
		m.filterMode = filterSubstring
		m.filterInput.Prompt = "/ "
		cmd := m.filterInput.Focus()
		return m, cmd
```

- [ ] **Step 3: When results are focused and `/` is pressed to re-focus query bar, also switch to substring mode**

Update the `Filter` case inside the `!m.filterInput.Focused()` block (line 111-114):

```go
			case key.Matches(msg, m.keys.Filter):
				// / returns focus to query bar and switches to substring mode.
				m.filterMode = filterSubstring
				m.filterInput.Prompt = "/ "
				cmd := m.filterInput.Focus()
				return m, cmd
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./internal/tui/`

---

### Task 4: Update entry point functions to set mode and prompt

**Files:**
- Modify: `internal/tui/tasks.go`

- [ ] **Step 1: Update enterAllTasksMode to use substring mode**

In `enterAllTasksMode` (line 376-389), replace `m.filterInput.Prompt = "/ "` with:

```go
	m.filterMode = filterSubstring
	m.filterInput.Prompt = "/ "
```

- [ ] **Step 2: Update enterTagSearchMode to use substring mode**

In `enterTagSearchMode` (line 392-403), replace `m.filterInput.Prompt = "/ "` with:

```go
	m.filterMode = filterSubstring
	m.filterInput.Prompt = "/ "
```

- [ ] **Step 3: Update enterRecentlyCompletedMode to use query mode**

In `enterRecentlyCompletedMode` (line 406-419), replace `m.filterInput.Prompt = "/ "` with:

```go
	m.filterMode = filterQuery
	m.filterInput.Prompt = "? "
```

- [ ] **Step 4: Update clearFilter to reset mode**

In `clearFilter` (line 364-372), add mode reset:

```go
func (m *Model) clearFilter() {
	m.filtering = false
	m.filterText = ""
	m.filterMode = filterSubstring
	m.showAll = false
	m.filterInput.SetValue("")
	m.filterInput.Prompt = "/ "
	m.filterInput.Placeholder = "type to filter..."
	m.filterInput.Blur()
}
```

- [ ] **Step 5: Verify it compiles**

Run: `go build ./internal/tui/`

---

### Task 5: Split filter application by mode

**Files:**
- Modify: `internal/tui/tasks.go`
- Modify: `internal/tui/model.go`

- [ ] **Step 1: Add queryErr back to Model**

In `model.go`, add after the `err` field:

```go
	queryErr    error  // DSL parse error for display in footer
```

- [ ] **Step 2: Split applyQueryFilter into two functions**

Replace the existing `applyQueryFilter` function with:

```go
// applySubstringFilter filters tasks using space-separated substring matching (ANDed).
func applySubstringFilter(tasks []model.Task, filterText string) []model.Task {
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(filterText)))
	if len(tokens) == 0 {
		return tasks
	}
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
	return filtered
}

// applyDSLFilter filters tasks using the query DSL.
func applyDSLFilter(tasks []model.Task, filterText string, now time.Time) ([]model.Task, error) {
	filterText = strings.TrimSpace(filterText)
	node, err := query.Parse(filterText)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return tasks, nil
	}
	opts := query.EvalOptions{PartialTags: true}
	var filtered []model.Task
	for _, t := range tasks {
		if query.EvalWithOptions(node, &t, now, opts) {
			filtered = append(filtered, t)
		}
	}
	return filtered, nil
}
```

- [ ] **Step 3: Update rebuildSingleSection to use mode-aware filtering**

```go
func (m *Model) rebuildSingleSection(title, color string, tasks []model.Task, now time.Time) {
	if m.filterText != "" {
		if m.filterMode == filterQuery {
			filtered, err := applyDSLFilter(tasks, m.filterText, now)
			if err != nil {
				m.queryErr = err
				return
			}
			tasks = filtered
		} else {
			tasks = applySubstringFilter(tasks, m.filterText)
		}
	}
	tasks = tasksort.StablePartitionPinned(tasks)
	m.sections = []filter.ViewResult{{Title: title, Color: color, Tasks: tasks}}
	m.applyHiddenFilter()
}
```

- [ ] **Step 4: Update rebuildDashboard to use mode-aware filtering**

```go
func (m *Model) rebuildDashboard(now time.Time) {
	results, err := filter.ApplyViews(m.allTasks, m.config.Views, now)
	if err != nil {
		m.err = err
		return
	}

	if m.filterText != "" {
		for i := range results {
			if m.filterMode == filterQuery {
				filtered, qErr := applyDSLFilter(results[i].Tasks, m.filterText, now)
				if qErr != nil {
					m.queryErr = qErr
					return
				}
				results[i].Tasks = tasksort.StablePartitionPinned(filtered)
			} else {
				filtered := applySubstringFilter(results[i].Tasks, m.filterText)
				if filtered == nil {
					filtered = []model.Task{}
				}
				results[i].Tasks = tasksort.StablePartitionPinned(filtered)
			}
		}
	}

	m.sections = results
	m.applyHiddenFilter()
}
```

- [ ] **Step 5: Clear queryErr at the top of rebuildSections**

Add `m.queryErr = nil` as the first line in `rebuildSections`.

- [ ] **Step 6: Verify it compiles**

Run: `go build ./internal/tui/`

---

### Task 6: Show queryErr in footer for query mode

**Files:**
- Modify: `internal/tui/views.go`

- [ ] **Step 1: Show queryErr in viewAllTasks**

After the `m.filterInput.View()` line in `viewAllTasks` (line 94), add:

```go
	if m.queryErr != nil {
		parts = append(parts, FooterStyle().Render("  "+m.queryErr.Error()))
	}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/tui/`

---

### Task 7: Update tests

**Files:**
- Modify: `internal/tui/model_test.go`

- [ ] **Step 1: Update TestFilterActivation to verify substring mode**

After verifying `m2.filtering` is true, add:

```go
	if m2.filterMode != filterSubstring {
		t.Error("expected filterSubstring mode after pressing '/'")
	}
```

- [ ] **Step 2: Add TestQueryModeActivation**

```go
func TestQueryModeActivation(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendKey(m, "?")
	m2 := updated.(Model)
	if !m2.filtering {
		t.Error("expected filtering to be true after pressing '?'")
	}
	if m2.filterMode != filterQuery {
		t.Error("expected filterQuery mode after pressing '?'")
	}
}
```

- [ ] **Step 3: Add TestRecentlyCompletedUsesQueryMode**

```go
func TestRecentlyCompletedUsesQueryMode(t *testing.T) {
	m := testModel(testTasks(), testViews())

	updated, _ := sendKey(m, "c")
	m2 := updated.(Model)
	if m2.filterMode != filterQuery {
		t.Error("expected filterQuery mode for recently completed")
	}
}
```

- [ ] **Step 4: Add TestSubstringFilterWithTags**

Verify that `/` mode with `@delegated foo` does substring matching (no DSL errors):

```go
func TestSubstringFilterWithTags(t *testing.T) {
	tasks := []model.Task{
		{Text: "Fix bug @delegated to bob", State: model.Open, File: "t.md", Line: 1,
			Tags: []model.Tag{{Name: "delegated"}}, HasCheckbox: true},
		{Text: "Write docs", State: model.Open, File: "t.md", Line: 2, HasCheckbox: true},
	}
	views := []config.ViewConfig{
		{Title: "All", Query: "open", Sort: "file", Color: "green"},
	}
	m := testModel(tasks, views)

	// Activate substring filter
	updated, _ := sendKey(m, "/")
	m = updated.(Model)
	for _, ch := range "@delegated bob" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}

	flat := m.flatTasks()
	if len(flat) != 1 {
		t.Fatalf("expected 1 matching task, got %d", len(flat))
	}
	if !strings.Contains(flat[0].Text, "delegated") {
		t.Errorf("expected task with @delegated, got %q", flat[0].Text)
	}
}
```

- [ ] **Step 5: Run all tests**

Run: `go test ./internal/tui/ -v`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/model.go internal/tui/keymap.go internal/tui/keys.go internal/tui/tasks.go internal/tui/views.go internal/tui/model_test.go
git commit -m "feat: split filter into / (substring) and ? (query DSL) modes"
```
