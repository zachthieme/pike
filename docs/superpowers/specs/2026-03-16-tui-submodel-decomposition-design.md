# TUI Sub-Model Decomposition

**Date:** 2026-03-16
**Status:** Approved
**Motivation:** Maintainability > Clarity > Testability

## Problem

The `internal/tui/` package has a single `Model` struct with ~30 fields that owns all TUI state: view mode, filter bar, tag search, cursor, summary, hidden visibility, and terminal dimensions. The `handleKey` function is a large dispatch tree with nested conditionals for filter-active vs. filter-inactive vs. tag-search mode. As features are added, this grows linearly more tangled.

## Decision

Extract the filter bar and tag search into Bubble Tea sub-models within the same `internal/tui/` package. Each sub-model owns its own state, key handling, and rendering. Communication with the main Model uses idiomatic Bubble Tea message passing. Filtering logic (substring, DSL) stays in the main Model.

The summary overlay is NOT extracted — it's 2 fields and a pure render function, not worth the ceremony.

## Sub-Model: FilterBar (`internal/tui/filterbar.go`)

### State

| Field | Type | Purpose |
|---|---|---|
| `input` | `textinput.Model` | Bubble Tea text input widget |
| `active` | `bool` | Whether filter bar is visible |
| `mode` | `filterMode` | Substring vs DSL |
| `text` | `string` | Current input value |
| `queryErr` | `error` | DSL parse error for display |
| `inputFocused` | `bool` | Whether input or results have focus |

### Messages Emitted

| Message | When | Main Model reaction |
|---|---|---|
| `FilterChangedMsg{Text, Mode}` | Every keystroke | Rebuild sections with new filter text |
| `FilterSubmittedMsg{}` | Enter pressed | No-op (focus already moved to results) |
| `FilterClearedMsg{}` | Escape with empty input | Clear filter, maybe exit mode, rebuild sections |
| `FilterModeChangedMsg{Mode}` | Switched between `/` and `?` | Rebuild sections (mode affects filter function) |

### Messages Consumed

| Message | Source |
|---|---|
| `tea.KeyMsg` | Main Model delegates when filter is active |
| `FilterActivateMsg{Mode, InitialValue, Placeholder}` | Main Model activates the filter |
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
| `TagSelectedMsg{Name}` | Enter on a tag | Enter all-tasks mode with `@tag` filter |
| `TagSearchExitMsg{}` | Escape | Return to dashboard |
| `TagSearchFilterChangedMsg{Text}` | Keystroke in filter | Re-render (tag search handles its own filtering) |

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
KeyMsg arrives
  if mode == modeTagSearch  -> tagSearch.Update(msg)
  if filterBar.Active()     -> filterBar.Update(msg)
  otherwise                 -> dashboard/navigation keys (unchanged)

Sub-model messages:
  FilterChangedMsg          -> rebuild sections with new filter text/mode
  FilterSubmittedMsg        -> no-op (focus managed inside sub-model)
  FilterClearedMsg          -> clear filter, maybe exit mode, rebuild sections
  FilterModeChangedMsg      -> rebuild sections
  TagSelectedMsg            -> enter all-tasks mode with @tag initial filter
  TagSearchExitMsg          -> return to dashboard
```

### Code Movement

| Current location | Destination |
|---|---|
| `filterState` struct definition | `FilterBar` fields |
| `setFilterMode()`, `setupFilter()`, `clearFilter()` | `FilterBar` methods |
| Filter key handling in `handleKey` (keys.go:21-131) | `FilterBar.Update()` |
| `handleTagSearchKey()` (keys.go:312-358) | `TagSearch.Update()` |
| `viewTagSearch()`, `flowWrap()` (views.go) | `TagSearch.View()` + shared helper |
| `buildTagList()`, `filteredTags()` (tasks.go) | `TagSearch` internal methods |
| `applySubstringFilter()`, `applyDSLFilter()` | **Stay in tasks.go** |
| `rebuildSections()` | **Stays in tasks.go**, reads filter text/mode from `m.filterBar` |

### New Files

- `internal/tui/filterbar.go` — FilterBar sub-model
- `internal/tui/tagsearch.go` — TagSearch sub-model
- `internal/tui/messages.go` — all message types consolidated

### Files That Shrink

- `keys.go` — loses ~130 lines of filter key handling and `handleTagSearchKey`
- `views.go` — loses `viewTagSearch()` and `flowWrap()`
- `tasks.go` — loses `buildTagList()`, `filteredTags()`, `filterState`, filter setup functions
- `model.go` — loses `filterState` type, simpler `Update`

## Testing Strategy

Sub-model tests are self-contained:
1. Construct with `New*()`
2. Send `tea.KeyMsg` or activation messages
3. Assert on emitted message types/values from `tea.Cmd`
4. Check `View()` output for expected rendering

Main Model tests simplify — instead of simulating full keystroke sequences through the filter, tests send `FilterChangedMsg` or `TagSelectedMsg` directly and assert on section state.

Existing tests in `model_test.go` that exercise filter and tag search behavior become acceptance criteria — they must pass against the new sub-models, restructured into `filterbar_test.go` and `tagsearch_test.go`.

## Migration Plan

Incremental, one sub-model at a time, tests green at each step:

1. **Extract message types** into `messages.go` — pure move, no behavior change
2. **Extract FilterBar** — move state and key handling, wire up message passing, migrate tests
3. **Extract TagSearch** — move state, key handling, and view rendering, wire up messages, migrate tests
4. **Clean up** — remove dead code from `tasks.go`, `keys.go`, `views.go`

Each step is a separate commit with all tests passing.
