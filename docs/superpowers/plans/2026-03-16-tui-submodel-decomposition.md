# TUI Sub-Model Decomposition Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the filter bar and tag search into Bubble Tea sub-models to improve maintainability and clarity.

**Architecture:** The monolithic `Model` struct in `internal/tui/` is decomposed into three components: a main `Model` that orchestrates mode transitions and section rendering, a `FilterBar` sub-model that owns the text input widget and filter UI state, and a `TagSearch` sub-model that owns tag list management and the tag picker UI. Sub-models emit output messages as `tea.Cmd` values. The main Model's `handleKey` processes these inline (extracting filter messages from returned cmds and handling them synchronously) to maintain single-Update state settlement that existing tests rely on.

**Tech Stack:** Go, Bubble Tea (github.com/charmbracelet/bubbletea), Bubbles textinput

**Spec:** `docs/superpowers/specs/2026-03-16-tui-submodel-decomposition-design.md`

---

## File Structure

| File | Action | Purpose |
|---|---|---|
| `internal/tui/messages.go` | Create | All message types, `viewMode`, `filterMode`, `filterPrompt` |
| `internal/tui/filterbar.go` | Create | FilterBar sub-model: state, Update, View, accessors |
| `internal/tui/filterbar_test.go` | Create | FilterBar unit tests |
| `internal/tui/tagsearch.go` | Create | TagSearch sub-model: state, Update, View, tag filtering |
| `internal/tui/tagsearch_test.go` | Create | TagSearch unit tests |
| `internal/tui/model.go` | Modify | Remove `filterState`, `filterMode`, msg types; add sub-model fields; rewire Update |
| `internal/tui/keys.go` | Modify | Rewrite handleKey delegation; remove handleTagSearchKey |
| `internal/tui/tasks.go` | Modify | Remove filter/tag functions; read from sub-model accessors |
| `internal/tui/views.go` | Modify | Remove viewTagSearch; use sub-model View(); update filter refs |
| `internal/tui/sections.go` | Modify | Update cursor-suppress check to use FilterBar accessors |
| `internal/tui/model_test.go` | Modify | Update filter/tag field references to use sub-model accessors |

---

## Chunk 1: Extract Message Types and FilterBar

### Task 1: Extract message types into messages.go

**Files:**
- Create: `internal/tui/messages.go`
- Modify: `internal/tui/model.go`

- [ ] **Step 1: Create messages.go with all type definitions moved from model.go**

```go
package tui

import (
	"pike/internal/config"
	"pike/internal/model"
)

// RefreshMsg triggers a re-scan of task files.
type RefreshMsg struct{}

// EditorFinishedMsg is sent after the editor process exits.
type EditorFinishedMsg struct{ Err error }

// toggleResultMsg is sent after a toggle operation completes.
type toggleResultMsg struct{ Err error }

// scanResultMsg is sent after a background scan completes.
type scanResultMsg struct {
	Tasks  []model.Task
	Config *config.Config
	Err    error
}

// viewMode tracks the current display mode.
type viewMode int

const (
	modeDashboard viewMode = iota
	modeAllTasks
	modeTagSearch
	modeRecentlyCompleted
)

// filterMode tracks whether the filter bar uses substring or DSL matching.
type filterMode int

const (
	filterSubstring filterMode = iota
	filterQuery
)

// filterPrompt maps each filter mode to its prompt string.
var filterPrompt = map[filterMode]string{
	filterSubstring: "/ ",
	filterQuery:     "? ",
}

// --- FilterBar messages ---

// FilterActivateMsg tells the FilterBar to activate with the given settings.
type FilterActivateMsg struct {
	Mode        filterMode
	InitialValue string
	Placeholder  string
}

// FilterDeactivateMsg tells the FilterBar to deactivate and reset.
type FilterDeactivateMsg struct{}

// FilterSetErrorMsg passes a DSL parse error to the FilterBar for display.
type FilterSetErrorMsg struct{ Err error }

// FilterChangedMsg is emitted by FilterBar on every keystroke.
type FilterChangedMsg struct {
	Text string
	Mode filterMode
}

// FilterSubmittedMsg is emitted when the user presses Enter in the filter input.
type FilterSubmittedMsg struct{}

// FilterClearedMsg is emitted when the user escapes from an empty filter.
type FilterClearedMsg struct{}

// FilterModeChangedMsg is emitted when the filter mode switches between / and ?.
type FilterModeChangedMsg struct {
	Mode filterMode
}

// --- TagSearch messages ---

// TagSearchActivateMsg tells TagSearch to activate with the given tag list.
type TagSearchActivateMsg struct {
	Tags []string
}

// TagSelectedMsg is emitted when the user selects a tag.
type TagSelectedMsg struct {
	Name string
}

// TagSearchExitMsg is emitted when the user escapes from tag search.
type TagSearchExitMsg struct{}
```

- [ ] **Step 2: Remove moved definitions from model.go**

Remove lines 15-53 from `model.go` (everything from `type RefreshMsg` through `filterPrompt`). Keep the `filterState` struct (lines 55-62) — it will be removed in Task 3 when FilterBar replaces it. Keep the `Model` struct and its methods. Update the import block to remove `"pike/internal/config"` and `"pike/internal/model"` if they're no longer needed in model.go (they still are — `config` for Config and `model` for Task).

- [ ] **Step 3: Run tests**

Run: `go build ./... && go test ./internal/tui/ -v -count=1`
Expected: All tests pass, no compile errors.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/messages.go internal/tui/model.go
git commit -m "refactor: extract message types into messages.go"
```

---

### Task 2: Create FilterBar sub-model

**Files:**
- Create: `internal/tui/filterbar.go`
- Create: `internal/tui/filterbar_test.go`

- [ ] **Step 1: Create filterbar.go**

```go
package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FilterBar is a Bubble Tea sub-model that manages the filter input widget.
// It handles keystroke capture when the input is focused, and emits messages
// for the parent Model to react to (rebuild sections, change modes, etc.).
type FilterBar struct {
	input    textinput.Model
	active   bool
	mode     filterMode
	text     string
	queryErr error
}

// NewFilterBar creates a FilterBar with the default textinput configuration.
func NewFilterBar() FilterBar {
	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 256
	ti.Prompt = "/ "
	ti.PromptStyle = BoldStyle()
	ti.PlaceholderStyle = FaintStyle().Foreground(lipgloss.Color("7"))
	return FilterBar{input: ti}
}

// Init implements the Bubble Tea sub-model interface.
func (f FilterBar) Init() tea.Cmd { return nil }

// Update handles messages for the FilterBar.
//
// When processing tea.KeyMsg, the parent Model is responsible for only
// forwarding keys according to the delegation rules in the spec:
//   - All keys when InputFocused() is true
//   - Only Escape, Tab, /, ? when InputFocused() is false
func (f FilterBar) Update(msg tea.Msg) (FilterBar, tea.Cmd) {
	switch msg := msg.(type) {
	case FilterActivateMsg:
		f.active = true
		f.mode = msg.Mode
		f.queryErr = nil
		f.input.SetValue(msg.InitialValue)
		if msg.InitialValue != "" {
			f.input.CursorEnd()
		}
		f.text = msg.InitialValue
		f.input.Prompt = filterPrompt[msg.Mode]
		f.input.Placeholder = msg.Placeholder
		return f, f.input.Focus()

	case FilterDeactivateMsg:
		f.active = false
		f.text = ""
		f.mode = filterSubstring
		f.queryErr = nil
		f.input.SetValue("")
		f.input.Prompt = filterPrompt[filterSubstring]
		f.input.Placeholder = "type to filter..."
		f.input.Blur()
		return f, nil

	case FilterSetErrorMsg:
		f.queryErr = msg.Err
		return f, nil

	case tea.KeyMsg:
		return f.handleKey(msg)
	}
	return f, nil
}

// handleKey processes key events for the FilterBar.
func (f FilterBar) handleKey(msg tea.KeyMsg) (FilterBar, tea.Cmd) {
	km := DefaultKeyMap()

	switch {
	case key.Matches(msg, km.Escape):
		if f.input.Value() != "" {
			// Clear the input text, re-focus if blurred.
			f.input.SetValue("")
			f.text = ""
			if !f.input.Focused() {
				cmd := f.input.Focus()
				return f, tea.Batch(cmd, func() tea.Msg {
					return FilterChangedMsg{Text: "", Mode: f.mode}
				})
			}
			return f, func() tea.Msg {
				return FilterChangedMsg{Text: "", Mode: f.mode}
			}
		}
		// Empty input — signal the parent to exit filter mode.
		return f, func() tea.Msg { return FilterClearedMsg{} }

	case key.Matches(msg, km.NextSection):
		// Tab: toggle focus between input and results.
		if f.input.Focused() {
			f.input.Blur()
		} else {
			cmd := f.input.Focus()
			return f, cmd
		}
		return f, nil

	case key.Matches(msg, km.Filter):
		// Switch to substring mode.
		if f.mode != filterSubstring {
			f.mode = filterSubstring
			f.input.Prompt = filterPrompt[filterSubstring]
			cmd := f.input.Focus()
			return f, tea.Batch(cmd, func() tea.Msg {
				return FilterModeChangedMsg{Mode: filterSubstring}
			})
		}
		// Already in substring mode — just re-focus.
		return f, f.input.Focus()

	case key.Matches(msg, km.Query):
		// Switch to DSL query mode.
		if f.mode != filterQuery {
			f.mode = filterQuery
			f.input.Prompt = filterPrompt[filterQuery]
			cmd := f.input.Focus()
			return f, tea.Batch(cmd, func() tea.Msg {
				return FilterModeChangedMsg{Mode: filterQuery}
			})
		}
		return f, f.input.Focus()

	case key.Matches(msg, km.Enter):
		if f.input.Focused() {
			// Submit: blur input (focus moves to results in parent).
			f.input.Blur()
			return f, func() tea.Msg { return FilterSubmittedMsg{} }
		}
		// Not focused — parent handles Enter (open editor).
		return f, nil
	}

	// If input is focused, route remaining keys to the text input widget.
	if f.input.Focused() {
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		f.text = f.input.Value()
		return f, tea.Batch(cmd, func() tea.Msg {
			return FilterChangedMsg{Text: f.text, Mode: f.mode}
		})
	}

	return f, nil
}

// View renders the filter input widget.
func (f FilterBar) View() string {
	return f.input.View()
}

// Active returns whether the filter bar is currently active.
func (f FilterBar) Active() bool { return f.active }

// InputFocused returns whether the text input widget has focus.
func (f FilterBar) InputFocused() bool { return f.input.Focused() }

// Text returns the current filter text.
func (f FilterBar) Text() string { return f.text }

// Mode returns the current filter mode (substring or DSL query).
func (f FilterBar) Mode() filterMode { return f.mode }

// QueryErr returns the current DSL parse error, if any.
func (f FilterBar) QueryErr() error { return f.queryErr }
```

- [ ] **Step 2: Create filterbar_test.go**

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// execCmd runs a tea.Cmd and returns the resulting message, or nil.
func execCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// execBatchCmd runs a tea.Cmd that may be a Batch, collecting all messages.
func execBatchCmds(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batchMsg, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batchMsg {
			if c != nil {
				msgs = append(msgs, c())
			}
		}
		return msgs
	}
	return []tea.Msg{msg}
}

func findMsg[T any](msgs []tea.Msg) (T, bool) {
	var zero T
	for _, m := range msgs {
		if typed, ok := m.(T); ok {
			return typed, true
		}
	}
	return zero, false
}

func TestFilterBarActivate(t *testing.T) {
	f := NewFilterBar()
	if f.Active() {
		t.Fatal("expected inactive initially")
	}

	f, _ = f.Update(FilterActivateMsg{
		Mode:         filterQuery,
		InitialValue: "open",
		Placeholder:  "search...",
	})

	if !f.Active() {
		t.Error("expected active after activate msg")
	}
	if f.Mode() != filterQuery {
		t.Error("expected filterQuery mode")
	}
	if f.Text() != "open" {
		t.Errorf("expected text 'open', got %q", f.Text())
	}
	if !f.InputFocused() {
		t.Error("expected input to be focused")
	}
}

func TestFilterBarDeactivate(t *testing.T) {
	f := NewFilterBar()
	f, _ = f.Update(FilterActivateMsg{Mode: filterSubstring, Placeholder: "filter..."})
	f, _ = f.Update(FilterDeactivateMsg{})

	if f.Active() {
		t.Error("expected inactive after deactivate")
	}
	if f.Text() != "" {
		t.Errorf("expected empty text, got %q", f.Text())
	}
	if f.InputFocused() {
		t.Error("expected input blurred after deactivate")
	}
}

func TestFilterBarKeystrokeEmitsChanged(t *testing.T) {
	f := NewFilterBar()
	f, _ = f.Update(FilterActivateMsg{Mode: filterSubstring, Placeholder: "filter..."})

	f, cmd := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	msgs := execBatchCmds(cmd)
	changed, ok := findMsg[FilterChangedMsg](msgs)
	if !ok {
		t.Fatal("expected FilterChangedMsg")
	}
	if changed.Text != "a" {
		t.Errorf("expected text 'a', got %q", changed.Text)
	}
	if changed.Mode != filterSubstring {
		t.Error("expected filterSubstring mode")
	}
}

func TestFilterBarEnterEmitsSubmitted(t *testing.T) {
	f := NewFilterBar()
	f, _ = f.Update(FilterActivateMsg{Mode: filterSubstring, Placeholder: "filter..."})

	f, cmd := f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := execCmd(cmd)
	if _, ok := msg.(FilterSubmittedMsg); !ok {
		t.Errorf("expected FilterSubmittedMsg, got %T", msg)
	}
	if f.InputFocused() {
		t.Error("expected input blurred after Enter")
	}
}

func TestFilterBarEscapeWithContentClears(t *testing.T) {
	f := NewFilterBar()
	f, _ = f.Update(FilterActivateMsg{Mode: filterSubstring, InitialValue: "test", Placeholder: "filter..."})

	f, cmd := f.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if f.Text() != "" {
		t.Errorf("expected empty text after Escape, got %q", f.Text())
	}
	// Should emit FilterChangedMsg with empty text, not FilterClearedMsg.
	msgs := execBatchCmds(cmd)
	if _, ok := findMsg[FilterClearedMsg](msgs); ok {
		t.Error("should not emit FilterClearedMsg when input had content")
	}
	changed, ok := findMsg[FilterChangedMsg](msgs)
	if !ok {
		t.Fatal("expected FilterChangedMsg with empty text")
	}
	if changed.Text != "" {
		t.Errorf("expected empty text in changed msg, got %q", changed.Text)
	}
}

func TestFilterBarEscapeEmptyEmitsCleared(t *testing.T) {
	f := NewFilterBar()
	f, _ = f.Update(FilterActivateMsg{Mode: filterSubstring, Placeholder: "filter..."})

	_, cmd := f.Update(tea.KeyMsg{Type: tea.KeyEscape})
	msg := execCmd(cmd)
	if _, ok := msg.(FilterClearedMsg); !ok {
		t.Errorf("expected FilterClearedMsg, got %T", msg)
	}
}

func TestFilterBarTabTogglesFocus(t *testing.T) {
	f := NewFilterBar()
	f, _ = f.Update(FilterActivateMsg{Mode: filterSubstring, Placeholder: "filter..."})
	if !f.InputFocused() {
		t.Fatal("expected focused after activate")
	}

	// Tab blurs
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyTab})
	if f.InputFocused() {
		t.Error("expected blurred after Tab")
	}

	// Tab re-focuses
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !f.InputFocused() {
		t.Error("expected focused after second Tab")
	}
}

func TestFilterBarModeSwitch(t *testing.T) {
	f := NewFilterBar()
	f, _ = f.Update(FilterActivateMsg{Mode: filterSubstring, Placeholder: "filter..."})

	// Switch to query mode with ?
	f, cmd := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	msgs := execBatchCmds(cmd)
	modeChanged, ok := findMsg[FilterModeChangedMsg](msgs)
	if !ok {
		t.Fatal("expected FilterModeChangedMsg")
	}
	if modeChanged.Mode != filterQuery {
		t.Error("expected filterQuery mode")
	}
	if f.Mode() != filterQuery {
		t.Error("expected mode to be filterQuery")
	}
}

func TestFilterBarSetError(t *testing.T) {
	f := NewFilterBar()
	f, _ = f.Update(FilterActivateMsg{Mode: filterQuery, Placeholder: "query..."})

	testErr := fmt.Errorf("parse error")
	f, _ = f.Update(FilterSetErrorMsg{Err: testErr})
	if f.QueryErr() != testErr {
		t.Errorf("expected query error to be set")
	}
}
```

Note: add `"fmt"` to the import block for the last test.

- [ ] **Step 3: Run tests to verify FilterBar works in isolation**

Run: `go build ./... && go test ./internal/tui/ -run TestFilterBar -v -count=1`
Expected: All FilterBar tests pass.

- [ ] **Step 4: Commit FilterBar sub-model (implementation + tests)**

```bash
git add internal/tui/filterbar.go internal/tui/filterbar_test.go
git commit -m "feat: add FilterBar sub-model with tests"
```

---

### Task 3: Wire FilterBar into main Model

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/keys.go`
- Modify: `internal/tui/tasks.go`
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/sections.go`
- Modify: `internal/tui/model_test.go`

- [ ] **Step 1: Update Model struct in model.go**

Replace the `filter filterState` field with `filterBar FilterBar`. Remove the `filterState` struct definition (if still present after Task 1). Update `NewModel`:

```go
m := Model{
	config:      cfg,
	allTasks:    tasks,
	focusedView: "",
	filterBar:   NewFilterBar(),
	scanFunc:    scanFunc,
	editorCmd:   cfg.Editor,
	tagColors:   cfg.TagColors,
	keys:        DefaultKeyMap(),
	now:         time.Now,
}
```

Remove the `textinput` and `lipgloss` imports from `model.go` if no longer needed there (they move to `filterbar.go`).

Filter output messages are now handled inline by `processFilterCmd` / `handleFilterOutputMsg` (see Step 2 above), so the main `Update()` does NOT need `FilterChangedMsg`, `FilterClearedMsg`, etc. cases. It only needs `FilterSetErrorMsg`:

```go
case FilterSetErrorMsg:
	m.filterBar, _ = m.filterBar.Update(msg)
	return m, nil
```

The actual filter message handling lives in `handleFilterOutputMsg` in `keys.go`.

- [ ] **Step 2: Rewrite handleKey in keys.go**

Replace the filter-active block (lines 21-131) with sub-model delegation:

```go
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Tag search mode: delegate all keys.
	if m.mode == modeTagSearch {
		return m.handleTagSearchKey(msg)
	}

	// Recently-completed: intercept Escape before FilterBar.
	if m.mode == modeRecentlyCompleted && m.filterBar.Active() && key.Matches(msg, m.keys.Escape) {
		m.exitToDashboard()
		return m, nil
	}

	// FilterBar active + input focused: all keys go to FilterBar.
	// Output messages (FilterChangedMsg, etc.) are processed inline via
	// processFilterCmd to maintain single-Update state settlement.
	if m.filterBar.Active() && m.filterBar.InputFocused() {
		var cmd tea.Cmd
		m.filterBar, cmd = m.filterBar.Update(msg)
		return m.processFilterCmd(cmd)
	}

	// FilterBar active + results focused: delegate only Escape/Tab/mode-switch.
	if m.filterBar.Active() {
		switch {
		case key.Matches(msg, m.keys.Escape), key.Matches(msg, m.keys.NextSection),
			key.Matches(msg, m.keys.Filter), key.Matches(msg, m.keys.Query):
			var cmd tea.Cmd
			m.filterBar, cmd = m.filterBar.Update(msg)
			return m.processFilterCmd(cmd)
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Toggle):
			return m.toggleTask()
		case key.Matches(msg, m.keys.ToggleHiddenTag):
			return m.toggleHiddenTag()
		case key.Matches(msg, m.keys.ToggleHidden):
			m.showHidden = !m.showHidden
			m.rebuildSections()
			m.clampCursor()
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.cursorDown()
			return m, nil
		case key.Matches(msg, m.keys.Up):
			m.cursorUp()
			return m, nil
		case key.Matches(msg, m.keys.Top):
			m.cursor = 0
			return m, nil
		case key.Matches(msg, m.keys.Bottom):
			m.cursor = max(0, m.countFlatTasks()-1)
			return m, nil
		case key.Matches(msg, m.keys.PageDown):
			m.pageScroll(1)
			return m, tea.ClearScreen
		case key.Matches(msg, m.keys.PageUp):
			m.pageScroll(-1)
			return m, tea.ClearScreen
		case key.Matches(msg, m.keys.PrevSection):
			m.jumpToPrevSection()
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			return m.openEditor()
		}
		return m, nil
	}

	// Dashboard/navigation keys (unchanged from current lines 133-241).
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	// ... (rest of the existing dashboard switch, unchanged)
	}
	// ... (existing 1-9 focus section keys, unchanged)
}
```

Add `processFilterCmd` to `keys.go` — this extracts FilterBar output messages from the returned `tea.Cmd` and handles them synchronously, while preserving non-filter cmds (e.g., textinput blink/focus):

```go
// processFilterCmd executes the cmd returned by FilterBar.Update, handles
// any filter output messages (FilterChangedMsg, etc.) inline, and returns
// remaining cmds to the Bubble Tea runtime. This ensures section rebuilds
// and mode transitions happen within a single Update cycle.
func (m Model) processFilterCmd(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	if cmd == nil {
		return m, nil
	}
	msg := cmd()
	if msg == nil {
		return m, nil
	}

	// Handle tea.BatchMsg: extract filter messages, keep the rest.
	if batch, ok := msg.(tea.BatchMsg); ok {
		var remaining []tea.Cmd
		for _, c := range batch {
			if c == nil {
				continue
			}
			batchMsg := c()
			if !m.handleFilterOutputMsg(batchMsg) {
				// Not a filter message — preserve it for the runtime.
				capturedMsg := batchMsg
				remaining = append(remaining, func() tea.Msg { return capturedMsg })
			}
		}
		switch len(remaining) {
		case 0:
			return m, nil
		case 1:
			return m, remaining[0]
		default:
			return m, tea.Batch(remaining...)
		}
	}

	// Single message: handle if it's a filter message, otherwise return as cmd.
	if m.handleFilterOutputMsg(msg) {
		return m, nil
	}
	capturedMsg := msg
	return m, func() tea.Msg { return capturedMsg }
}

// handleFilterOutputMsg processes a single filter output message inline.
// Returns true if the message was a filter message and was handled.
func (m *Model) handleFilterOutputMsg(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case FilterChangedMsg:
		if m.showAll && msg.Text == "" && m.mode != modeRecentlyCompleted {
			m.filterBar, _ = m.filterBar.Update(FilterDeactivateMsg{})
			m.enterTagSearchMode()
			return true
		}
		m.rebuildSections()
		m.clampCursor()
		return true
	case FilterSubmittedMsg:
		return true
	case FilterClearedMsg:
		if m.showAll {
			m.filterBar, _ = m.filterBar.Update(FilterDeactivateMsg{})
			m.enterTagSearchMode()
			return true
		}
		m.filterBar, _ = m.filterBar.Update(FilterDeactivateMsg{})
		m.showAll = false
		if m.mode == modeAllTasks {
			m.mode = modeDashboard
		}
		m.rebuildSections()
		m.clampCursor()
		return true
	case FilterModeChangedMsg:
		m.rebuildSections()
		m.clampCursor()
		return true
	}
	return false
}
```

**Note on behavior change:** In the current code, pressing `q` when filter results are focused is silently ignored. With the new dispatch, `q` now quits the app (matching the spec). Pressing `j`/`k` when the filter input is focused now types those characters into the input instead of moving the cursor (improvement: users can now filter for text containing `j` or `k`).

- [ ] **Step 3: Update tasks.go**

Replace all `m.filter.*` references with `m.filterBar.*` accessor calls:

In `rebuildSections()`:
- `m.filter.QueryErr = nil` → remove (FilterBar owns queryErr)
- After rebuild, if DSL filter produced an error, send it to FilterBar

In `rebuildSingleSection()`:
- `m.filter.Text` → `m.filterBar.Text()`
- `m.filter.Mode == filterQuery` → `m.filterBar.Mode() == filterQuery`
- `m.filter.QueryErr = err` → `m.filterBar, _ = m.filterBar.Update(FilterSetErrorMsg{Err: err})`

In `rebuildDashboard()`:
- Same replacements as above

Remove these functions (now in FilterBar or obsolete):
- `setFilterMode()` (line 386-388)
- `clearFilter()` (lines 413-422)

**Keep `setupFilter()` for now** — `enterTagSearchMode()` still calls it until Task 5 gives TagSearch its own textinput. Remove it in Task 5.

Update `enterAllTasksMode()`:
```go
func (m *Model) enterAllTasksMode(showAll bool, initialFilter string) tea.Cmd {
	m.mode = modeAllTasks
	m.showAll = showAll
	m.cursor = 0
	var cmd tea.Cmd
	m.filterBar, cmd = m.filterBar.Update(FilterActivateMsg{
		Mode:         filterSubstring,
		InitialValue: initialFilter,
		Placeholder:  "search tasks...",
	})
	m.rebuildSections()
	m.clampCursor()
	return cmd
}
```

Update `enterRecentlyCompletedMode()`:
```go
func (m *Model) enterRecentlyCompletedMode() tea.Cmd {
	queryStr := fmt.Sprintf("completed and @completed >= today-%dd", m.config.RecentlyCompletedDays)
	m.mode = modeRecentlyCompleted
	m.cursor = 0
	var cmd tea.Cmd
	m.filterBar, cmd = m.filterBar.Update(FilterActivateMsg{
		Mode:         filterQuery,
		InitialValue: queryStr,
		Placeholder:  "type to filter...",
	})
	m.rebuildSections()
	m.clampCursor()
	return cmd
}
```

Update `exitToDashboard()`:
```go
func (m *Model) exitToDashboard() {
	m.mode = modeDashboard
	m.filterBar, _ = m.filterBar.Update(FilterDeactivateMsg{})
	m.showAll = false
	m.rebuildSections()
	m.clampCursor()
}
```

- [ ] **Step 4: Update views.go**

Replace `m.filter.*` references:
- `m.filter.Input.View()` → `m.filterBar.View()`
- `m.filter.QueryErr` → `m.filterBar.QueryErr()`
- `m.filter.Active` → `m.filterBar.Active()`

In `renderSections()` (line 294-296):
```go
if m.filterBar.Active() {
	parts = append(parts, m.filterBar.View())
	parts = append(parts, "")
}
```

- [ ] **Step 5: Update sections.go line 55**

```go
selected := flatIdx == m.cursor && !(m.filterBar.Active() && m.filterBar.InputFocused())
```

- [ ] **Step 6: Update model_test.go**

Replace all `m.filter.Active` → `m.filterBar.Active()`, `m.filter.Mode` → `m.filterBar.Mode()`, `m.filter.Text` → `m.filterBar.Text()`, etc.

Key test references to update:
- `TestFilterActivation` (line 211): `m.filter.Active` → `m.filterBar.Active()`, `m.filter.Mode` → `m.filterBar.Mode()`
- `TestQueryModeActivation` (line 230): same
- `TestRecentlyCompletedUsesQueryMode` (line 243): `m.filter.Mode` → `m.filterBar.Mode()`
- `TestEscapeDismissesFilter` (line 351): `m.filter.Active` → `m.filterBar.Active()`, `m.filter.Text` → `m.filterBar.Text()`
- `TestRecentlyCompletedMode` (line 860): `m.filter.Active` → `m.filterBar.Active()`, `m.filter.Text` → `m.filterBar.Text()`
- `TestBackspaceToEmptyReturnsToTagSearch` (line 698): `m.filter.Text` → `m.filterBar.Text()`

- [ ] **Step 7: Run all tests**

Run: `go build ./... && go test ./internal/tui/ -v -count=1`
Expected: ALL tests pass (existing + new FilterBar tests).

- [ ] **Step 8: Run full test suite**

Run: `go test ./... -count=1`
Expected: All packages pass.

- [ ] **Step 9: Commit**

```bash
git add internal/tui/model.go internal/tui/keys.go internal/tui/tasks.go \
       internal/tui/views.go internal/tui/sections.go internal/tui/model_test.go
git commit -m "refactor: wire FilterBar sub-model into main Model"
```

---

## Chunk 2: Extract TagSearch and Clean Up

### Task 4: Create TagSearch sub-model

**Files:**
- Create: `internal/tui/tagsearch.go`
- Create: `internal/tui/tagsearch_test.go`

- [ ] **Step 1: Create tagsearch.go**

```go
package tui

import (
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TagSearch is a Bubble Tea sub-model for the tag picker UI.
// It owns the tag list, cursor, and its own text input for filtering tags.
type TagSearch struct {
	tagList    []string
	tagCursor  int
	filter     textinput.Model
	filterText string
}

// NewTagSearch creates a TagSearch with the default textinput configuration.
func NewTagSearch() TagSearch {
	ti := textinput.New()
	ti.Placeholder = "search tags..."
	ti.CharLimit = 256
	ti.Prompt = "/ "
	ti.PromptStyle = BoldStyle()
	ti.PlaceholderStyle = FaintStyle().Foreground(lipgloss.Color("7"))
	return TagSearch{filter: ti}
}

// Init implements the Bubble Tea sub-model interface.
func (t TagSearch) Init() tea.Cmd { return nil }

// Update handles messages for the TagSearch.
func (t TagSearch) Update(msg tea.Msg) (TagSearch, tea.Cmd) {
	switch msg := msg.(type) {
	case TagSearchActivateMsg:
		t.tagList = msg.Tags
		slices.Sort(t.tagList)
		t.tagCursor = 0
		t.filterText = ""
		t.filter.SetValue("")
		t.filter.Placeholder = "search tags..."
		return t, t.filter.Focus()

	case tea.KeyMsg:
		return t.handleKey(msg)
	}
	return t, nil
}

// handleKey processes key events for the TagSearch.
func (t TagSearch) handleKey(msg tea.KeyMsg) (TagSearch, tea.Cmd) {
	km := DefaultKeyMap()

	switch {
	case key.Matches(msg, km.Escape):
		return t, func() tea.Msg { return TagSearchExitMsg{} }

	case key.Matches(msg, km.Quit):
		return t, tea.Quit

	case key.Matches(msg, km.NextSection) || key.Matches(msg, km.Down):
		tags := t.filteredTags()
		if len(tags) > 0 {
			t.tagCursor = (t.tagCursor + 1) % len(tags)
		}
		return t, nil

	case key.Matches(msg, km.PrevSection) || key.Matches(msg, km.Up):
		tags := t.filteredTags()
		if len(tags) > 0 {
			t.tagCursor = (t.tagCursor - 1 + len(tags)) % len(tags)
		}
		return t, nil

	case key.Matches(msg, km.Enter):
		tags := t.filteredTags()
		if t.tagCursor < len(tags) {
			selectedTag := tags[t.tagCursor]
			return t, func() tea.Msg { return TagSelectedMsg{Name: selectedTag} }
		}
		return t, nil

	default:
		var cmd tea.Cmd
		t.filter, cmd = t.filter.Update(msg)
		t.filterText = t.filter.Value()
		t.tagCursor = 0
		return t, cmd
	}
}

// View renders the tag search UI with flow-wrapped, colorized tags.
func (t TagSearch) View(tagColors map[string]string, width int) string {
	var parts []string

	parts = append(parts, t.filter.View())

	filtered := t.filteredTags()
	if len(t.tagList) == 0 {
		parts = append(parts, "  No tags found")
		return strings.Join(parts, "\n")
	}

	matchedSet := make(map[string]bool, len(filtered))
	for _, tag := range filtered {
		matchedSet[tag] = true
	}

	selectedTag := ""
	if len(filtered) > 0 && t.tagCursor < len(filtered) {
		selectedTag = filtered[t.tagCursor]
	}

	fs := FaintStyle()
	delim := fs.Render("\u2009·\u2009")
	var tagParts []string
	for _, tag := range t.tagList {
		if tag == selectedTag {
			tagParts = append(tagParts, TaskStyle(true).Render(tag))
		} else if matchedSet[tag] {
			color := resolveTagColor(tagColors, tag)
			if color != "" {
				tagParts = append(tagParts, TagStyle(color).Render(tag))
			} else {
				tagParts = append(tagParts, tag)
			}
		} else {
			tagParts = append(tagParts, fs.Render(tag))
		}
	}

	if width > 0 {
		parts = append(parts, flowWrap(tagParts, delim, width-2))
	} else {
		parts = append(parts, "  "+strings.Join(tagParts, delim))
	}

	if len(filtered) == 0 && t.filterText != "" {
		parts = append(parts, "")
		parts = append(parts, "  No results")
	}

	return strings.Join(parts, "\n")
}

// FilterText returns the current filter text.
func (t TagSearch) FilterText() string { return t.filterText }

// filteredTags returns tags matching the current filter text.
func (t TagSearch) filteredTags() []string {
	if t.filterText == "" {
		return t.tagList
	}
	lower := strings.ToLower(strings.TrimPrefix(t.filterText, "@"))
	var result []string
	for _, tag := range t.tagList {
		if strings.Contains(strings.ToLower(tag), lower) {
			result = append(result, tag)
		}
	}
	return result
}

// resolveTagColor returns the configured color for a tag name, falling back to "_default".
func resolveTagColor(tagColors map[string]string, tagName string) string {
	if color, ok := tagColors[tagName]; ok {
		return color
	}
	if color, ok := tagColors["_default"]; ok {
		return color
	}
	return ""
}
```

- [ ] **Step 2: Create tagsearch_test.go**

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTagSearchActivate(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due", "today", "risk"}})

	tags := ts.filteredTags()
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(tags))
	}
	// Tags should be sorted
	if tags[0] != "due" || tags[1] != "risk" || tags[2] != "today" {
		t.Errorf("expected sorted tags, got %v", tags)
	}
}

func TestTagSearchCursorNavigation(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"a", "b", "c"}})

	// Down
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyTab})
	if ts.tagCursor != 1 {
		t.Errorf("expected cursor 1 after Tab, got %d", ts.tagCursor)
	}

	// Down again
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyTab})
	if ts.tagCursor != 2 {
		t.Errorf("expected cursor 2 after 2nd Tab, got %d", ts.tagCursor)
	}

	// Wraps around
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyTab})
	if ts.tagCursor != 0 {
		t.Errorf("expected cursor 0 after wrap, got %d", ts.tagCursor)
	}
}

func TestTagSearchSelectEmitsMsg(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due", "risk", "today"}})

	// Move to "risk" (index 1 after sort: due, risk, today)
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyTab})
	ts, cmd := ts.Update(tea.KeyMsg{Type: tea.KeyEnter})

	msg := execCmd(cmd)
	selected, ok := msg.(TagSelectedMsg)
	if !ok {
		t.Fatalf("expected TagSelectedMsg, got %T", msg)
	}
	if selected.Name != "risk" {
		t.Errorf("expected 'risk', got %q", selected.Name)
	}
}

func TestTagSearchEscapeEmitsExit(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due"}})

	_, cmd := ts.Update(tea.KeyMsg{Type: tea.KeyEscape})
	msg := execCmd(cmd)
	if _, ok := msg.(TagSearchExitMsg); !ok {
		t.Errorf("expected TagSearchExitMsg, got %T", msg)
	}
}

func TestTagSearchQuitEmitsQuit(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due"}})

	_, cmd := ts.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	msg := execCmd(cmd)
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestTagSearchFiltering(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due", "risk", "today", "done"}})

	// Type "d" — should match "due" and "done"
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	tags := ts.filteredTags()
	if len(tags) != 2 {
		t.Fatalf("expected 2 filtered tags, got %d: %v", len(tags), tags)
	}
}

func TestTagSearchAtPrefixStripped(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due", "risk", "today"}})

	// Type "@du" — should match "due"
	for _, ch := range "@du" {
		ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	tags := ts.filteredTags()
	found := false
	for _, tag := range tags {
		if tag == "due" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'due' in filtered tags, got %v", tags)
	}
}

func TestTagSearchViewRendersWithoutPanic(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due", "risk", "today"}})

	tagColors := map[string]string{"due": "red", "_default": "cyan"}
	view := ts.View(tagColors, 80)
	if view == "" {
		t.Error("expected non-empty view")
	}
}
```

- [ ] **Step 3: Run TagSearch tests**

Run: `go build ./... && go test ./internal/tui/ -run TestTagSearch -v -count=1`
Expected: All TagSearch tests pass.

- [ ] **Step 4: Commit TagSearch sub-model**

```bash
git add internal/tui/tagsearch.go internal/tui/tagsearch_test.go
git commit -m "feat: add TagSearch sub-model with tests"
```

---

### Task 5: Wire TagSearch into main Model

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/keys.go`
- Modify: `internal/tui/tasks.go`
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/model_test.go`

- [ ] **Step 1: Update Model struct**

Replace `tagList []string` and `tagCursor int` with `tagSearch TagSearch`. Add to `NewModel`:

```go
m := Model{
	// ...existing fields...
	filterBar:  NewFilterBar(),
	tagSearch:  NewTagSearch(),
	// ...
}
```

- [ ] **Step 2: Update Update() for TagSearch messages**

Add to the `switch msg.(type)` in `Update()`:

```go
case TagSelectedMsg:
	m.mode = modeAllTasks
	m.showAll = true
	if msg.Name == "hidden" {
		m.showHidden = true
	}
	var cmd tea.Cmd
	m.filterBar, cmd = m.filterBar.Update(FilterActivateMsg{
		Mode:         filterSubstring,
		InitialValue: "@" + msg.Name,
		Placeholder:  "search tasks...",
	})
	m.rebuildSections()
	m.clampCursor()
	return m, cmd

case TagSearchExitMsg:
	m.exitToDashboard()
	return m, nil
```

Update the `scanResultMsg` handler — replace `m.buildTagList()` / `m.filteredTags()` / `m.tagCursor` with TagSearch updates:

```go
if msg.Tasks != nil {
	m.allTasks = msg.Tasks
	if m.mode == modeTagSearch {
		tags := extractTagNames(m.allTasks)
		m.tagSearch, _ = m.tagSearch.Update(TagSearchActivateMsg{Tags: tags})
	}
}
```

Add helper (in tasks.go):
```go
func extractTagNames(tasks []model.Task) []string {
	seen := make(map[string]bool)
	for _, t := range tasks {
		for _, tag := range t.Tags {
			seen[tag.Name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names
}
```

- [ ] **Step 3: Update handleKey in keys.go**

Replace the tag search delegation:

```go
if m.mode == modeTagSearch {
	var cmd tea.Cmd
	m.tagSearch, cmd = m.tagSearch.Update(msg)
	return m, cmd
}
```

Remove the `handleTagSearchKey` method entirely (lines 312-358).

- [ ] **Step 4: Update views.go**

Replace `viewTagSearch()` with TagSearch delegation:

```go
case modeTagSearch:
	return errLine + m.tagSearch.View(m.tagColors, m.width)
```

Remove the `viewTagSearch()` method (lines 179-236).

- [ ] **Step 5: Update tasks.go**

Remove:
- `buildTagList()` (lines 345-357)
- `filteredTags()` (lines 360-373)
- `setupFilter()` (lines 391-402) — no longer needed now that TagSearch has its own textinput
- `resolveTagColor()` method on Model (lines 457-466) — now `resolveTagColor` is a package-level function in tagsearch.go

Update `enterTagSearchMode()`:
```go
func (m *Model) enterTagSearchMode() tea.Cmd {
	m.mode = modeTagSearch
	m.showAll = false
	tags := extractTagNames(m.allTasks)
	var cmd tea.Cmd
	m.tagSearch, cmd = m.tagSearch.Update(TagSearchActivateMsg{Tags: tags})
	return cmd
}
```

Update the `FilterChangedMsg` and `FilterClearedMsg` handlers in `Update()` to use `enterTagSearchMode()` instead of inline tag search activation:

```go
case FilterChangedMsg:
	if m.showAll && msg.Text == "" && m.mode != modeRecentlyCompleted {
		m.filterBar, _ = m.filterBar.Update(FilterDeactivateMsg{})
		cmd := m.enterTagSearchMode()
		return m, cmd
	}
	// ...
```

- [ ] **Step 6: Update model_test.go**

Replace `m.filteredTags()` calls with `m.tagSearch.filteredTags()` — but `filteredTags` is unexported on TagSearch. Since it's in the same package, this works. Alternatively, test via the `View()` output or the TagSearch's behavior.

Key tests to update:
- `TestTagSearchWithAtPrefix` (line 552): use `m.tagSearch.filteredTags()` instead of `m.filteredTags()`
- `TestTagSearchWithoutAtPrefix` (line 580): same
- `TestTagSelectShowsCompletedTasks` (line 605): `m.filteredTags()` → `m.tagSearch.filteredTags()`
- `TestBackspaceToEmptyReturnsToTagSearch` (line 698): `m.filter.Text` → `m.filterBar.Text()`

- [ ] **Step 7: Run all tests**

Run: `go build ./... && go test ./internal/tui/ -v -count=1`
Expected: ALL tests pass.

- [ ] **Step 8: Run full test suite**

Run: `go test ./... -count=1`
Expected: All packages pass.

- [ ] **Step 9: Commit**

```bash
git add internal/tui/model.go internal/tui/keys.go internal/tui/tasks.go \
       internal/tui/views.go internal/tui/model_test.go
git commit -m "refactor: wire TagSearch sub-model into main Model"
```

---

### Task 6: Clean up dead code

**Files:**
- Modify: all `internal/tui/*.go` files

- [ ] **Step 1: Remove unused imports and dead code**

Run `go build ./...` — the compiler will flag any unused imports. Fix them.

Check for any remaining references to old field names:
- `m.filter.` → should not appear anywhere
- `m.tagList` → should not appear
- `m.tagCursor` → should not appear
- `filterState` struct → should be deleted

- [ ] **Step 2: Verify no unused functions**

Run: `go vet ./internal/tui/`

Check that these functions are removed:
- `handleTagSearchKey` (was in keys.go)
- `viewTagSearch` (was in views.go)
- `buildTagList` (was in tasks.go)
- `filteredTags` on Model (was in tasks.go)
- `setFilterMode` (was in tasks.go)
- `setupFilter` (was in tasks.go)
- `clearFilter` (was in tasks.go)
- `resolveTagColor` method on Model (was in tasks.go — now package-level in tagsearch.go)
- `setupFilter` (was in tasks.go — no longer needed after TagSearch extraction)

- [ ] **Step 3: Run all tests one final time (including bench_test.go)**

Run: `go build ./... && go test ./... -count=1`
Expected: All packages pass, zero compile warnings.

- [ ] **Step 4: Run benchmarks to verify no regressions**

Run: `go test -bench=. -benchmem -count=1 ./internal/tui/ -benchtime=1s`
Expected: Benchmarks run without error. Numbers should be comparable to pre-refactor.

- [ ] **Step 5: Commit**

```bash
git add -A internal/tui/
git commit -m "refactor: clean up dead code after sub-model extraction"
```
