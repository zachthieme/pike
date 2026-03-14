# Pike Improvements Design

Five features: task counts in headers, pinned tasks, task toggling, recently completed view, and query bar.

## Feature 1: Task Counts in Section Headers

Section headers change from `Today` to `Today (3)`. The count reflects visible tasks after all filtering (query DSL, hidden filter, text filter). Zero-task sections are already hidden, so count is always >= 1. When hidden tasks exist, the format is `Today (3) 🔒` (count before lock icon).

### Implementation

In `renderSection()` (`internal/tui/sections.go`), append ` (N)` to the header label where N is `len(tasks)`. The count is already available — it's the length of the tasks slice passed to the function.

### Files to modify
- `internal/tui/sections.go` — modify `renderSection()` header label

## Feature 2: Pinned Tasks (`@pin`)

Tasks tagged `@pin` float to the top of their section. Within the pinned group, the section's configured sort order is preserved. Unpinned tasks follow in their normal sort order. `@pin` is a plain tag with no special behavior beyond sort priority.

Pinning applies everywhere tasks are displayed: dashboard sections, all-tasks mode, and recently-completed mode.

### Implementation

Add a `StablePartitionPinned(tasks []model.Task) []model.Task` function in `internal/sort/sort.go` that partitions without disturbing the existing sort order within each group.

Call `StablePartitionPinned()` in the `Apply` function in `internal/filter/filter.go` (after sorting), so both `ApplyViews` and any direct caller benefit. Also apply it in the `modeAllTasks` branch of `rebuildSections()` in `tasks.go`.

### Implementation detail

`StablePartitionPinned` uses two-pass collection (pinned first, then unpinned) to preserve order by construction, rather than re-sorting.

### Files to modify
- `internal/sort/sort.go` — add `StablePartitionPinned()`
- `internal/filter/filter.go` — call `StablePartitionPinned()` after sorting in `Apply`
- `internal/tui/tasks.go` — call `StablePartitionPinned()` after assembling tasks in both `modeAllTasks` and `modeRecentlyCompleted` branches of `rebuildSections()`

## Feature 3: Task Toggling (`x` key)

### Completing a task

Pressing `x` on an open checkbox task:
1. Reads the source file at `task.File`
2. Validates that the line at `task.Line` contains `- [ ]` (guards against stale line numbers from external edits)
3. On the line, replaces `- [ ]` with `- [x]` (matches the pattern anywhere on the line to support indented tasks)
4. Appends ` @completed(YYYY-MM-DD)` (today's date) to the end of the line
5. Writes the file back
6. Triggers a `RefreshMsg` to update the dashboard

### Un-completing a task

Pressing `x` on a completed checkbox task:
1. Reads the source file at `task.File`
2. Validates that the line at `task.Line` contains `- [x]` (guards against stale line numbers)
3. On the line, replaces `- [x]` with `- [ ]` (matches anywhere on the line for indented tasks)
4. Removes the `@completed(...)` tag using regex `\s*@completed(\([^)]*\))?(?:\s|$)` — uses word boundary at end to avoid matching `@completedAt(...)` etc. When the match ends at `\s`, preserves one space so adjacent content isn't joined.
5. Writes the file back
6. Triggers a `RefreshMsg`

### Constraints

- Non-checkbox tasks (tagged bullets without `[ ]`/`[x]`) ignore `x`
- The toggle uses `task.File` (relative path) joined with `config.NotesDir` to get the absolute path
- The `x` key works in normal mode, filter mode, all-tasks mode, and recently-completed mode — anywhere a task cursor is active
- If `NotesDir` is empty or the file can't be written, show an error via `m.err`
- If the line content doesn't match the expected pattern (stale data), show an error and trigger a refresh

### New keybinding

Add `Toggle key.Binding` to `KeyMap` with key `"x"`.

### New package

`internal/toggle/toggle.go` — contains `Complete(filePath string, line int, date time.Time) error` and `Uncomplete(filePath string, line int) error`. Pure file operations, no TUI dependency. Reads the file, modifies the specific line, writes it back. Both functions validate the line content before modifying.

### Files to create
- `internal/toggle/toggle.go` — Complete/Uncomplete file operations
- `internal/toggle/toggle_test.go` — unit tests

### Files to modify
- `internal/tui/keymap.go` — add `Toggle` key binding (`"x"`)
- `internal/tui/keys.go` — handle `x` key in normal mode and filtering mode, call toggle then refresh

## Feature 4: Recently Completed View (`c` key)

Pressing `c` enters a view showing recently completed tasks.

### Behavior

- Uses a new `modeRecentlyCompleted` viewMode constant (not `modeAllTasks`) to avoid overloading `showAll` semantics
- Pre-populates the query bar with: `completed and @completed >= today-Nd` where N comes from `config.RecentlyCompletedDays`
- Section title: "Recently Completed" — set explicitly by `enterRecentlyCompletedMode()`
- The query bar is editable — users can refine the query further
- Pressing `x` on a task here un-completes it (same toggle behavior)
- `Esc` returns to dashboard
- Clearing the query bar (backspace to empty) stays in recently-completed mode (unlike tag search which returns to tag picker)

### Mode handling

`modeRecentlyCompleted` shares rendering logic with `modeAllTasks` (both use `viewAllTasks()`). In `rebuildSections()`, the `modeRecentlyCompleted` branch collects all tasks (checkbox + tagged, any state) and applies the DSL query filter using `EvalWithOptions` with `PartialTags: true` (same as the query bar). The section title is "Recently Completed". Pressing `c` while already in `modeRecentlyCompleted` is a no-op.

The `modeRecentlyCompleted` guard in `handleKey` can share the filtering key handling with `modeAllTasks` since the interaction is identical.

### Config addition

```yaml
recently_completed_days: 7
```

Added as `RecentlyCompletedDays int` field on `Config` struct with default 7. Uses `*int` in `rawConfig` to distinguish "not set" from "set to 0". Setting to 0 means "completed today only" which is valid behavior.

### New keybinding

Add `RecentlyCompleted key.Binding` to `KeyMap` with key `"c"`.

### Files to modify
- `internal/config/config.go` — add `RecentlyCompletedDays` field with default 7
- `internal/tui/model.go` — add `modeRecentlyCompleted` to viewMode const
- `internal/tui/keymap.go` — add `RecentlyCompleted` key binding (`"c"`)
- `internal/tui/keys.go` — handle `c` key, route `modeRecentlyCompleted` to shared filtering handler
- `internal/tui/tasks.go` — add `enterRecentlyCompletedMode()`, handle `modeRecentlyCompleted` in `rebuildSections()`
- `internal/tui/views.go` — route `modeRecentlyCompleted` to `viewAllTasks()`

## Feature 5: Query Bar (Replaces Filter Bar)

The `/` key opens a query bar that accepts full DSL syntax instead of the simple token filter. Live evaluation as the user types.

### Query DSL changes

**Partial tag matching in interactive mode only:** The `@tag` atom gains partial matching behavior, but only when used through the interactive query bar. View queries defined in `config.yaml` retain exact matching to avoid breaking existing views (e.g., `@today` should not match `@today_standup`).

Implementation: Add an `EvalOptions` struct with a `PartialTags bool` field to `internal/query/eval.go`. The existing `Eval(task, node, now)` function keeps exact matching. Add `EvalWithOptions(task, node, now, opts)` that the query bar code path uses with `PartialTags: true`. When `PartialTags` is true, tag matching uses `strings.Contains(strings.ToLower(tag.Name), strings.ToLower(queryTag))` instead of exact equality.

### Filter bar replacement

Currently `rebuildSections()` in `tasks.go` uses `parseFilterTokens()` + `matchesFilter()` for text filtering. This is replaced with:

1. Parse the filter text as a DSL query via `query.Parse()`
2. If parse succeeds, evaluate each task against the AST via `query.EvalWithOptions()` with `PartialTags: true`
3. If parse fails, apply fallback logic (see below)

### Error state

New Model field: `queryErr error` — set when DSL parsing fails and fallback doesn't apply, cleared on successful parse or successful fallback. Rendered in the footer as a faint error message (e.g., `parse error: unexpected "and"`).

**Caching on error:** When DSL parsing fails and the input contains DSL tokens (error case), `rebuildSections()` is not called — the existing `m.sections` are preserved as-is. This avoids flickering without needing to cache ASTs.

### Prompt change

The filter input prompt changes from `"/ "` and `"> "` to `"query: "` in dashboard filter, all-tasks, and recently-completed modes. Tag search mode keeps its current `"> "` prompt and `"search tags..."` placeholder since users are searching tag names, not writing DSL queries. After a tag is selected and results are shown (which enters all-tasks mode), the prompt becomes `"query: "`.

### Fallback logic

```
1. Try parsing input as DSL query
2. If parse succeeds → use DSL evaluation with PartialTags: true
3. If parse fails:
   a. If input contains no DSL tokens → treat as simple substring filter
      (case-insensitive, space-separated tokens ANDed)
   b. Otherwise → show parse error, keep previous sections as-is
```

**DSL token detection** uses word-boundary matching: split input on whitespace, check if any token exactly equals `and`, `or`, `not` (case-insensitive), or if input contains `@`, `<`, `>`, `<=`, `>=`, or `/regex/` patterns. This avoids false positives from natural words like "notify" containing "not".

This means:
- `deploy` → simple substring match (works like before)
- `deploy server` → both substrings must match (works like before)
- `@due` → DSL: partial tag match
- `open and @due < today` → DSL: full query
- `open and and` → parse error shown in footer

**Negation (`!`) in fallback mode:** The old filter supported `!term` and `!@tag`. In fallback mode (no DSL tokens detected), `!` prefix is NOT supported since it's ambiguous. Users who want negation use the DSL: `not @tag` or `not /pattern/`. The `!` prefix detection from the old filter is removed entirely.

### Files to modify
- `internal/query/eval.go` — add `EvalOptions` struct and `EvalWithOptions()`, keep `Eval()` with exact matching
- `internal/query/eval_test.go` — add tests for partial tag matching via EvalWithOptions
- `internal/tui/model.go` — add `queryErr error` field
- `internal/tui/tasks.go` — replace `parseFilterTokens`/`matchesFilter` with DSL parsing + fallback logic, update prompts
- `internal/tui/views.go` — render `queryErr` in footer when set
- `internal/tui/keys.go` — skip rebuild on parse error (preserve sections)

### Files to remove/deprecate
- `parseFilterTokens()` and `matchesFilter()` in `internal/tui/tasks.go` — replaced by DSL parsing + fallback. The `filterToken` type is also removed.

## Implementation Order

1. Task counts in headers (no dependencies)
2. Pinned tasks (no dependencies)
3. Task toggling (independent, no dependencies)
4. Query bar (replaces filter system)
5. Recently completed view (depends on query bar for pre-filled DSL query, depends on toggle for `x` key)
