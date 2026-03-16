# TUI Sub-Model Decomposition

**Date:** 2026-03-16
**Status:** Approved
**Motivation:** Maintainability > Clarity > Testability

## Problem

The `internal/tui/` package has a single `Model` struct with ~20 fields (plus 6 in the `filterState` sub-struct) that owns all TUI state: view mode, filter bar, tag search, cursor, summary, hidden visibility, and terminal dimensions. The `handleKey` function is a large dispatch tree with nested conditionals for filter-active vs. filter-inactive vs. tag-search mode. As features are added, this grows linearly more tangled.

## Decision

Extract the filter bar and tag search into Bubble Tea sub-models within the same `internal/tui/` package. Each sub-model owns its own state, key handling, and rendering. Communication with the main Model uses idiomatic Bubble Tea message passing. Filtering logic (substring, DSL) stays in the main Model.

The summary overlay is NOT extracted — it's 2 fields and a pure render function, not worth the ceremony.

## Shared Types (`internal/tui/messages.go`)

The `filterMode` type, its constants (`filterSubstring`, `filterQuery`), and the `filterPrompt` map move from `model.go` to `messages.go` alongside all message type definitions. These are used by both FilterBar and the main Model (for `rebuildSections`), so they must be package-level.

## Sub-Model: FilterBar (`internal/tui/filterbar.go`)

### State

| Field | Type | Purpose |
|---|---|---|
| `input` | `textinput.Model` | Bubble Tea text input widget |
| `active` | `bool` | Whether filter bar is visible |
| `mode` | `filterMode` | Substring vs DSL |
| `text` | `string` | Current input value |
| `queryErr` | `error` | DSL parse error for display |

### Key Delegation Boundary

The main Model delegates `tea.KeyMsg` to FilterBar **only when the input is focused**, plus always delegates Escape and Tab (for clearing/toggling focus). Navigation and action keys (j/k, x, H, Enter, etc.) when results are focused remain in the main Model — the FilterBar does not know about editor launching, toggling, or cursor movement.

Specifically, the main Model dispatches to FilterBar when:
- `filterBar.Active() && filterBar.InputFocused()` — all keystrokes go to FilterBar
- `filterBar.Active() && !filterBar.InputFocused()` — only Escape, Tab, `/`, `?` go to FilterBar; everything else is handled by the main Model's existing navigation/action handlers

### Messages Emitted

| Message | When | Main Model reaction |
|---|---|---|
| `FilterChangedMsg{Text, Mode}` | Every keystroke while input focused | Rebuild sections with new filter text |
| `FilterSubmittedMsg{}` | Enter pressed (blurs input internally) | No-op (focus transition handled inside FilterBar) |
| `FilterClearedMsg{}` | Escape with empty input | See Cross-Model Transitions |
| `FilterModeChangedMsg{Mode}` | Switched between `/` and `?` via key | Rebuild sections (mode affects filter function) |

### Messages Consumed

| Message | Source |
|---|---|
| `tea.KeyMsg` | Main Model delegates per rules above |
| `FilterActivateMsg{Mode, InitialValue, Placeholder}` | Main Model activates the filter |
| `FilterDeactivateMsg{}` | Main Model deactivates (e.g., on mode exit) |
| `FilterSetErrorMsg{error}` | Main Model passes back DSL parse errors after rebuild |

### Public Interface

```go
type FilterBar struct { ... }

func NewFilterBar() FilterBar
func (f FilterBar) Init() tea.Cmd
func (f FilterBar) Update(msg tea.Msg) (FilterBar, tea.Cmd)
func (f FilterBar) View() string
func (f FilterBar) Active() bool
func (f FilterBar) InputFocused() bool
func (f FilterBar) Text() string
func (f FilterBar) Mode() filterMode
```

`NewFilterBar()` replicates the textinput setup currently in `NewModel()`: `CharLimit: 256`, `Prompt: "/ "`, `PromptStyle: BoldStyle()`, `PlaceholderStyle: FaintStyle().Foreground(lipgloss.Color("7"))`.

## Sub-Model: TagSearch (`internal/tui/tagsearch.go`)

### State

| Field | Type | Purpose |
|---|---|---|
| `tagList` | `[]string` | All unique tags, sorted alphabetically |
| `tagCursor` | `int` | Currently selected tag index |
| `filter` | `textinput.Model` | Tag filter input widget |
| `filterText` | `string` | Current filter text |

### Messages Emitted

| Message | When | Main Model reaction |
|---|---|---|
| `TagSelectedMsg{Name}` | Enter on a tag | See Cross-Model Transitions |
| `TagSearchExitMsg{}` | Escape | Return to dashboard |

Note: `TagSearchFilterChangedMsg` is not needed. Tag search handles its own filtering internally, and Bubble Tea always calls `View()` after `Update()`, so the re-render happens automatically.

### Messages Consumed

| Message | Source |
|---|---|
| `tea.KeyMsg` | Main Model delegates when in tag search mode |
| `TagSearchActivateMsg{Tags []string}` | Main Model passes tag list when entering mode |

### Public Interface

```go
type TagSearch struct { ... }

func NewTagSearch() TagSearch
func (t TagSearch) Init() tea.Cmd
func (t TagSearch) Update(msg tea.Msg) (TagSearch, tea.Cmd)
func (t TagSearch) View(tagColors map[string]string, width int) string
func (t TagSearch) FilterText() string
```

Tag search builds and filters its own tag list internally. The main Model passes raw tag names via `TagSearchActivateMsg`. The `View` method takes `tagColors` and `width` as parameters rather than owning them — these belong to the main Model and change on config reload.

### Quit Handling

TagSearch handles `Quit` (q) internally before its text input default branch, matching current behavior where `q` quits from tag search mode (users cannot type `q` in the tag filter). This is the existing behavior in `handleTagSearchKey` (keys.go:319-320).

### Independence from FilterBar

TagSearch owns its own `textinput.Model` and is fully independent of FilterBar. Entering tag search mode does NOT activate the FilterBar — the main Model activates TagSearch directly via `TagSearchActivateMsg`.

## Cross-Model Transitions

Three flows involve coordination between sub-models. All are orchestrated by the main Model synchronously within a single `Update` call.

### Tag search -> filter bar (tag selected)

TagSearch emits `TagSelectedMsg{Name: "today"}`. The main Model:
1. Sets `mode = modeAllTasks`, `showAll = true`
2. Calls `m.filterBar.Update(FilterActivateMsg{Mode: filterSubstring, InitialValue: "@today", Placeholder: "search tasks..."})` inline
3. Rebuilds sections

### Filter bar -> tag search (backspace to empty with showAll)

FilterBar emits `FilterChangedMsg{Text: ""}`. The main Model checks `showAll == true && msg.Text == ""`:
1. Calls `m.filterBar.Update(FilterDeactivateMsg{})` inline
2. Sets `mode = modeTagSearch`
3. Calls `m.tagSearch.Update(TagSearchActivateMsg{Tags: ...})` inline

### Filter bar -> tag search (Escape with content and showAll)

FilterBar emits `FilterClearedMsg{}`. The main Model checks `showAll == true`:
1. Calls `m.filterBar.Update(FilterDeactivateMsg{})` inline
2. Sets `mode = modeTagSearch`
3. Calls `m.tagSearch.Update(TagSearchActivateMsg{Tags: ...})` inline

### Recently-completed Escape special case

When `mode == modeRecentlyCompleted`, the main Model intercepts Escape **before** delegating to FilterBar and calls `exitToDashboard()` directly. The FilterBar never sees the Escape key in this mode.

## Main Model Changes

### Fields Removed

- `filter filterState` — replaced by `filterBar FilterBar`
- `tagList []string` — replaced by `tagSearch TagSearch`
- `tagCursor int` — replaced by `tagSearch TagSearch`

### Fields Retained

- `mode viewMode` — main Model still owns mode transitions
- `showAll bool` — controls whether all-tasks includes completed
- Everything else (tasks, sections, cursor, config, etc.)

### Update Dispatch

```
KeyMsg arrives:
  1. Mode-specific delegation (text inputs must receive all keys):

     if mode == modeRecentlyCompleted && key == Escape:
       -> exitToDashboard() directly (bypass FilterBar)

     if mode == modeTagSearch:
       -> tagSearch.Update(msg)
       (TagSearch handles Quit internally before its text input,
        preserving current behavior where `q` quits from tag search)

     if filterBar.Active() && filterBar.InputFocused():
       -> filterBar.Update(msg)  (ALL keys including `q` — no global Quit intercept)

     if filterBar.Active() && !filterBar.InputFocused():
       -> Quit -> tea.Quit
       -> filterBar.Update(msg)  ONLY for Escape, Tab, /, ?
       -> main Model handles j/k, g/G, x, H, Enter, etc.

     otherwise (dashboard/navigation, no text input focused):
       -> Quit -> tea.Quit
       -> handle navigation/action keys (unchanged)

Sub-model messages:
  FilterChangedMsg       -> if showAll && text == "": transition to tag search
                            else: rebuild sections with new filter text/mode
  FilterSubmittedMsg     -> no-op
  FilterClearedMsg       -> if showAll: transition to tag search
                            elif mode == modeAllTasks: deactivate filter, set mode = modeDashboard, rebuild
                            else: deactivate filter, stay in current mode, rebuild sections
  FilterModeChangedMsg   -> rebuild sections
  TagSelectedMsg         -> transition to filter bar with @tag
  TagSearchExitMsg       -> exitToDashboard()
```

### Code Movement

| Current location | Destination |
|---|---|
| `filterState` struct definition | Removed (replaced by `FilterBar` fields) |
| `filterMode`, `filterPrompt` (model.go:42-53) | `messages.go` |
| `setFilterMode()`, `setupFilter()`, `clearFilter()` | `FilterBar` internal methods |
| Filter key handling in `handleKey` (keys.go:21-131) | `FilterBar.Update()` (input-focused keys) + main Model (result-focused keys) |
| `handleTagSearchKey()` (keys.go:312-358) | `TagSearch.Update()` |
| `viewTagSearch()` (views.go) | `TagSearch.View()` |
| `flowWrap()` (views.go) | **Stays in views.go** as a package-level helper, called by `TagSearch.View()` |
| `buildTagList()`, `filteredTags()` (tasks.go) | `TagSearch` internal methods |
| `applySubstringFilter()`, `applyDSLFilter()` | **Stay in tasks.go** |
| `rebuildSections()` | **Stays in tasks.go**, reads filter text/mode via `m.filterBar.Text()` and `m.filterBar.Mode()` |

### Rendering Dependency

`sections.go:55` currently uses `m.filter.Active && m.filter.Input.Focused()` to suppress cursor highlighting when the filter input has focus. This changes to `m.filterBar.Active() && m.filterBar.InputFocused()`.

### New Files

- `internal/tui/filterbar.go` — FilterBar sub-model
- `internal/tui/tagsearch.go` — TagSearch sub-model
- `internal/tui/messages.go` — all message types and shared type definitions

### Files That Shrink

- `keys.go` — loses ~150 lines of filter key handling and `handleTagSearchKey`
- `views.go` — loses `viewTagSearch()` (keeps `flowWrap`)
- `tasks.go` — loses `buildTagList()`, `filteredTags()`, `filterState`, filter setup functions
- `model.go` — loses `filterState` type and `filterMode`/`filterPrompt` definitions, simpler `Update`

## Testing Strategy

Sub-model tests are self-contained:
1. Construct with `New*()`
2. Send `tea.KeyMsg` or activation messages
3. Assert on emitted message types/values from `tea.Cmd`
4. Check `View()` output for expected rendering

Main Model tests simplify — instead of simulating full keystroke sequences through the filter, tests send `FilterChangedMsg` or `TagSelectedMsg` directly and assert on section state.

Existing tests in `model_test.go` that exercise filter and tag search behavior become acceptance criteria — they must pass against the new sub-models, restructured into `filterbar_test.go` and `tagsearch_test.go`.

`bench_test.go` references `applySubstringFilter` and `applyDSLFilter` which stay in `tasks.go` — no benchmark changes needed.

## Migration Plan

Incremental, one sub-model at a time, tests green at each step:

1. **Extract message types and shared definitions** into `messages.go` — move `filterMode`, `filterPrompt`, all `Msg` types. Pure move, no behavior change.
2. **Extract FilterBar** — create `filterbar.go`, move filter state and input-focused key handling, wire up message passing in main Model, update `sections.go` rendering reference, migrate filter-specific tests to `filterbar_test.go`. **Migration note:** Tag search currently piggybacks on `m.filter.Input` via `setupFilter()`. During this step, tag search temporarily accesses the FilterBar's input via `m.filterBar` until step 3 gives it its own `textinput.Model`. Alternatively, steps 2 and 3 can be done as a single atomic commit if the shim feels too awkward.
3. **Extract TagSearch** — create `tagsearch.go`, move tag state, key handling, and `viewTagSearch()`, wire up messages and cross-model transitions, give TagSearch its own `textinput.Model` (replacing temporary FilterBar access from step 2), migrate tag search tests to `tagsearch_test.go`.
4. **Clean up** — remove dead code from `tasks.go`, `keys.go`, `views.go`, `model.go`. Verify no unused imports or functions.

Each step is a separate commit with all tests passing.
