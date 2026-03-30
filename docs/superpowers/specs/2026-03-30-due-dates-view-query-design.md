# Due Dates View Query Design

## Problem

`writeDueDates` currently dumps every task with a `@due` tag to `due.json`, regardless of task state. There's no way to filter what gets exported (e.g., only open tasks), and the behavior is hardcoded — users can't customize the query.

## Solution

Add two new optional boolean fields to `ViewConfig`:

- `due_dates: true` — marks a view as the source for `due.json`. Its query determines which tasks' due dates are exported.
- `hidden: true` — excludes a view from the TUI dashboard while keeping it available for due-dates export, keybinding targets, and `--view` CLI usage.

When `due_dates_path` is set and no view has `due_dates: true`, Pike defaults to the query `"open and @due"`.

## Config

```yaml
due_dates_path: ~/.local/share/pike/due.json

views:
  - title: "Due Dates Export"
    query: "open and @due"
    sort: due_asc
    due_dates: true
    hidden: true
```

### Validation

- At most one view may have `due_dates: true`. Multiple views with the flag is a config error returned from `applyDefaults`.

## Changes

### `internal/config/config.go`

**`ViewConfig` struct** — add two fields:

```go
type ViewConfig struct {
    Title    string `yaml:"title"`
    Query    string `yaml:"query"`
    Sort     string `yaml:"sort"`
    Color    string `yaml:"color"`
    Order    int    `yaml:"order"`
    DueDates bool   `yaml:"due_dates"`
    Hidden   bool   `yaml:"hidden"`
}
```

**`applyDefaults`** — after sorting views, validate that at most one view has `DueDates: true`:

```go
dueDateCount := 0
for _, v := range cfg.Views {
    if v.DueDates {
        dueDateCount++
    }
}
if dueDateCount > 1 {
    return nil, fmt.Errorf("at most one view may have due_dates: true")
}
```

**`defaultConfigYAML`** — add commented examples for the new fields.

### `cmd/pike/main.go`

**New helper `dueDatesQuery`** — scans `cfg.Views` for the `DueDates` view and returns its query, or `"open and @due"` as the default:

```go
func dueDatesQuery(views []config.ViewConfig) string {
    for _, v := range views {
        if v.DueDates {
            return v.Query
        }
    }
    return "open and @due"
}
```

**`writeDueDates` signature change**:

```go
func writeDueDates(path string, tasks []model.Task, query string, now time.Time)
```

The function runs `filter.Apply(tasks, query, "", now)` to get the filtered task set, then extracts due dates from the results. This replaces the current unfiltered iteration.

**Callers updated** — `runTUI` and `scanRefresh` pass the resolved query and `time.Now()`.

### `internal/tui/model.go`

When building TUI sections from `cfg.Views`, skip views where `Hidden` is true. Hidden views remain in `cfg.Views` so they're still accessible for:

- Due-dates export
- Custom keybinding `view:` targets
- `--view` CLI flag

## Testing

- **Config tests**: `due_dates` and `hidden` parse correctly. Multiple `due_dates: true` views produce an error.
- **`dueDatesQuery` unit tests**: returns the tagged view's query when present, falls back to `"open and @due"` when absent.
- **`writeDueDates` tests**: update existing tests to pass a query and `now`. Verify that completed tasks with due dates are excluded when using `"open and @due"`.
- **TUI tests**: hidden views don't appear in the dashboard section list.
- **Integration test**: existing `TestDueDatesWrittenOnStartup` updated for the new signature.

## Non-goals

- Multiple views feeding `due.json` — out of scope, one view is sufficient.
- Changing the `due.json` output format — remains a flat array of `"yyyy-mm-dd"` strings.
