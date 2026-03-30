# Due Dates View Query Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow the due-dates JSON export to be driven by a configurable view query instead of dumping all tasks with a `@due` tag, and allow views to be hidden from the TUI dashboard.

**Architecture:** Add `DueDates` and `Hidden` boolean fields to `ViewConfig`. `writeDueDates` gains a query parameter and uses `filter.Apply` to select tasks. `ApplyViews` skips hidden views so they don't appear in the dashboard. A `dueDatesQuery` helper resolves the query from views or falls back to `"open and @due"`.

**Tech Stack:** Go, Bubble Tea, existing query/filter infrastructure.

---

### Task 1: Add `DueDates` and `Hidden` fields to `ViewConfig`

**Files:**
- Modify: `internal/config/config.go:38-45` (ViewConfig struct)
- Modify: `internal/config/config.go:296-312` (applyDefaults validation)
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for new fields**

Add to `internal/config/config_test.go`:

```go
func TestLoadBytes_ViewDueDatesAndHidden(t *testing.T) {
	yaml := `
views:
  - title: "Open"
    query: "open"
    sort: file
    order: 1
  - title: "Due Export"
    query: "open and @due"
    sort: due_asc
    due_dates: true
    hidden: true
    order: 2
`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Views) != 2 {
		t.Fatalf("Views len = %d, want 2", len(cfg.Views))
	}
	if cfg.Views[0].DueDates {
		t.Error("Views[0].DueDates should be false")
	}
	if cfg.Views[0].Hidden {
		t.Error("Views[0].Hidden should be false")
	}
	if !cfg.Views[1].DueDates {
		t.Error("Views[1].DueDates should be true")
	}
	if !cfg.Views[1].Hidden {
		t.Error("Views[1].Hidden should be true")
	}
}

func TestLoadBytes_MultipleDueDatesViewsError(t *testing.T) {
	yaml := `
views:
  - title: "A"
    query: "open"
    sort: file
    due_dates: true
    order: 1
  - title: "B"
    query: "open and @due"
    sort: file
    due_dates: true
    order: 2
`
	_, err := LoadBytes([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for multiple due_dates views")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/zach/code/pike && go test ./internal/config/ -run 'TestLoadBytes_ViewDueDatesAndHidden|TestLoadBytes_MultipleDueDatesViewsError' -v`

Expected: `TestLoadBytes_ViewDueDatesAndHidden` passes (YAML unmarshalling picks up unknown fields silently), `TestLoadBytes_MultipleDueDatesViewsError` fails (no validation yet).

- [ ] **Step 3: Add fields to ViewConfig and validation to applyDefaults**

In `internal/config/config.go`, update `ViewConfig`:

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

In `applyDefaults`, after the view sort block (after line ~312), add validation:

```go
dueDateViewCount := 0
for _, v := range cfg.Views {
	if v.DueDates {
		dueDateViewCount++
	}
}
if dueDateViewCount > 1 {
	return nil, fmt.Errorf("at most one view may have due_dates: true")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/zach/code/pike && go test ./internal/config/ -run 'TestLoadBytes_ViewDueDatesAndHidden|TestLoadBytes_MultipleDueDatesViewsError' -v`

Expected: Both PASS.

- [ ] **Step 5: Run full test suite and lint**

Run: `cd /home/zach/code/pike && make test && make lint`

Expected: All pass.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add due_dates and hidden fields to ViewConfig

Adds DueDates and Hidden boolean fields to ViewConfig. Validates
that at most one view may have due_dates: true."
```

---

### Task 2: Filter hidden views in `ApplyViews`

**Files:**
- Modify: `internal/filter/filter.go:52-70` (ApplyViews function)
- Test: `internal/filter/filter_test.go`

- [ ] **Step 1: Write failing test for hidden view filtering**

Add to `internal/filter/filter_test.go`:

```go
func TestApplyViews_HiddenViewsExcluded(t *testing.T) {
	tasks := []model.Task{
		{Text: "task @due(2026-03-15)", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "due", Value: "2026-03-15"}},
			Due:  timePtr(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC))},
	}
	views := []config.ViewConfig{
		{Title: "Open", Query: "open", Sort: "file"},
		{Title: "Due Export", Query: "open and @due", Sort: "due_asc", Hidden: true},
	}
	now := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)

	results, err := ApplyViews(tasks, views, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (hidden excluded), got %d", len(results))
	}
	if results[0].Title != "Open" {
		t.Errorf("results[0].Title = %q, want %q", results[0].Title, "Open")
	}
}
```

Check what `timePtr` helper exists in the test file. If not present, add:

```go
func timePtr(t time.Time) *time.Time { return &t }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/zach/code/pike && go test ./internal/filter/ -run TestApplyViews_HiddenViewsExcluded -v`

Expected: FAIL — returns 2 results instead of 1.

- [ ] **Step 3: Skip hidden views in ApplyViews**

In `internal/filter/filter.go`, update `ApplyViews`:

```go
func ApplyViews(tasks []model.Task, views []config.ViewConfig, now time.Time) ([]ViewResult, error) {
	results := make([]ViewResult, 0, len(views))

	for _, view := range views {
		if view.Hidden {
			continue
		}
		filtered, err := Apply(tasks, view.Query, view.Sort, now)
		if err != nil {
			return nil, err
		}

		results = append(results, ViewResult{
			Title: view.Title,
			Color: view.Color,
			Tasks: filtered,
		})
	}

	return results, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/zach/code/pike && go test ./internal/filter/ -run TestApplyViews_HiddenViewsExcluded -v`

Expected: PASS.

- [ ] **Step 5: Run full test suite and lint**

Run: `cd /home/zach/code/pike && make test && make lint`

Expected: All pass.

- [ ] **Step 6: Commit**

```bash
git add internal/filter/filter.go internal/filter/filter_test.go
git commit -m "feat(filter): skip hidden views in ApplyViews

Views with Hidden: true are excluded from the dashboard results
returned by ApplyViews."
```

---

### Task 3: Add `dueDatesQuery` helper and update `writeDueDates`

**Files:**
- Modify: `cmd/pike/main.go:383-431` (writeDueDates function)
- Test: `cmd/pike/main_test.go`

- [ ] **Step 1: Write failing tests**

Add to `cmd/pike/main_test.go`:

```go
func TestDueDatesQuery_NoTaggedView(t *testing.T) {
	views := []config.ViewConfig{
		{Title: "Open", Query: "open", Sort: "file"},
	}
	got := dueDatesQuery(views)
	if got != "open and @due" {
		t.Errorf("dueDatesQuery() = %q, want %q", got, "open and @due")
	}
}

func TestDueDatesQuery_TaggedView(t *testing.T) {
	views := []config.ViewConfig{
		{Title: "Open", Query: "open", Sort: "file"},
		{Title: "Export", Query: "open and @due < today+30d", Sort: "due_asc", DueDates: true},
	}
	got := dueDatesQuery(views)
	if got != "open and @due < today+30d" {
		t.Errorf("dueDatesQuery() = %q, want %q", got, "open and @due < today+30d")
	}
}

func TestWriteDueDates_FiltersWithQuery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "due.json")

	openDue := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	completedDue := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		{Text: "open task", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "due", Value: "2026-03-20"}},
			Due:  &openDue},
		{Text: "done task", State: model.Completed, HasCheckbox: true,
			Tags: []model.Tag{{Name: "due", Value: "2026-03-15"}},
			Due:  &completedDue},
	}

	now := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	writeDueDates(path, tasks, "open and @due", now)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read due.json: %v", err)
	}

	var dates []string
	if err := json.Unmarshal(data, &dates); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Only the open task's due date should appear
	if len(dates) != 1 {
		t.Fatalf("expected 1 date, got %d: %v", len(dates), dates)
	}
	if dates[0] != "2026-03-20" {
		t.Errorf("dates[0] = %q, want %q", dates[0], "2026-03-20")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/zach/code/pike && go test ./cmd/pike/ -run 'TestDueDatesQuery|TestWriteDueDates_FiltersWithQuery' -v`

Expected: FAIL — `dueDatesQuery` undefined, `writeDueDates` has wrong signature.

- [ ] **Step 3: Implement dueDatesQuery and update writeDueDates**

In `cmd/pike/main.go`, add the `dueDatesQuery` helper (before `writeDueDates`):

```go
// dueDatesQuery returns the query string for the due-dates export. If a view
// is tagged with DueDates: true, its query is used; otherwise the default
// "open and @due" is returned.
func dueDatesQuery(views []config.ViewConfig) string {
	for _, v := range views {
		if v.DueDates {
			return v.Query
		}
	}
	return "open and @due"
}
```

Update `writeDueDates` to accept a query and now, and use `filter.Apply`:

```go
func writeDueDates(path string, tasks []model.Task, query string, now time.Time) {
	if path == "" {
		return
	}

	filtered, err := filter.Apply(tasks, query, "", now)
	if err != nil {
		return
	}

	seen := make(map[string]bool)
	for _, t := range filtered {
		if t.Due != nil {
			seen[t.Due.Format("2006-01-02")] = true
		}
	}

	dates := make([]string, 0, len(seen))
	for d := range seen {
		dates = append(dates, d)
	}
	slices.Sort(dates)

	data, err := json.Marshal(dates)
	if err != nil {
		return
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	// Atomic write: temp file + rename so readers never see partial content.
	tmp, err := os.CreateTemp(dir, ".due-*.json")
	if err != nil {
		return
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()           //nolint:errcheck // cleaning up on error
		os.Remove(tmpPath)    //nolint:errcheck // cleaning up on error
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath) //nolint:errcheck // cleaning up on error
		return
	}
	_ = os.Rename(tmpPath, path) //nolint:errcheck // best-effort; errors are intentionally ignored
}
```

- [ ] **Step 4: Update existing writeDueDates tests for new signature**

Update all existing `writeDueDates` call sites in `cmd/pike/main_test.go` to pass the query and now parameters. Use `""` as query (empty query matches all tasks — preserves existing behavior) and `time.Now()` as now:

- `TestWriteDueDates_WritesCorrectJSON`: `writeDueDates(path, tasks, "", time.Now())`
- `TestWriteDueDates_EmptyPathIsNoop`: `writeDueDates("", tasks, "", time.Now())`
- `TestWriteDueDates_NoTasksWritesEmptyArray`: `writeDueDates(path, nil, "", time.Now())`
- `TestWriteDueDates_CreatesParentDirs`: `writeDueDates(path, []model.Task{{Due: &d}}, "", time.Now())`
- `TestWriteDueDates_AtomicWrite`: both calls updated similarly

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/zach/code/pike && go test ./cmd/pike/ -run 'TestDueDatesQuery|TestWriteDueDates' -v`

Expected: All PASS.

- [ ] **Step 6: Run full test suite and lint**

Run: `cd /home/zach/code/pike && make test && make lint`

Expected: All pass.

- [ ] **Step 7: Commit**

```bash
git add cmd/pike/main.go cmd/pike/main_test.go
git commit -m "feat: filter due dates export through configurable query

writeDueDates now accepts a query string and runs it through
filter.Apply. dueDatesQuery resolves the query from the tagged
view or falls back to 'open and @due'."
```

---

### Task 4: Wire up callers in `runTUI`

**Files:**
- Modify: `cmd/pike/main.go:433-461` (runTUI function)

- [ ] **Step 1: Update runTUI to pass query and now to writeDueDates**

In `cmd/pike/main.go`, update `runTUI`. The initial call and the `scanRefresh` closure both need the resolved query and `time.Now()`:

```go
func runTUI(_ io.Writer, cfg *config.Config, tasks []model.Task, sc *scanner.Scanner, viewFlag string, configPath string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Resolve the due-dates query from config views.
	dueQuery := dueDatesQuery(cfg.Views)

	// Write due dates on initial launch.
	writeDueDates(cfg.DueDatesPath, tasks, dueQuery, time.Now())

	var dueMu sync.Mutex
	dueDatesPath := cfg.DueDatesPath
	currentDueQuery := dueQuery
	configReload := func() (*config.Config, error) {
		c, err := config.Load(configPath)
		if err == nil {
			dueMu.Lock()
			dueDatesPath = c.DueDatesPath
			currentDueQuery = dueDatesQuery(c.Views)
			dueMu.Unlock()
		}
		return c, err
	}
	scanRefresh := func() ([]model.Task, error) {
		tasks, err := sc.Refresh(ctx)
		if err == nil {
			dueMu.Lock()
			path := dueDatesPath
			q := currentDueQuery
			dueMu.Unlock()
			writeDueDates(path, tasks, q, time.Now())
		}
		return tasks, err
	}
```

The rest of `runTUI` stays the same.

- [ ] **Step 2: Run full test suite and lint**

Run: `cd /home/zach/code/pike && make test && make lint`

Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add cmd/pike/main.go
git commit -m "feat: wire due dates query through runTUI callers

runTUI resolves the due-dates query from config views and passes
it to writeDueDates on startup and on each scan refresh."
```

---

### Task 5: Update default config YAML

**Files:**
- Modify: `internal/config/config.go:338-419` (defaultConfigYAML)

- [ ] **Step 1: Add commented examples for new fields**

In `internal/config/config.go`, update `defaultConfigYAML`. Add comments for `due_dates` and `hidden` in the views section. Add a commented-out example view:

```yaml
# Dashboard sections — each view is a filtered, sorted slice of your tasks
# Optional fields:
#   due_dates: true    — use this view's query for the due.json export (at most one)
#   hidden: true       — exclude from dashboard (still usable for due_dates, keybindings, --view)
views:
```

Also add a commented-out example after the existing views:

```yaml
  # Uncomment to customize which tasks feed the due.json export:
  # - title: "Due Dates Export"
  #   query: "open and @due"
  #   sort: due_asc
  #   due_dates: true
  #   hidden: true
```

- [ ] **Step 2: Run full test suite and lint**

Run: `cd /home/zach/code/pike && make test && make lint`

Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "docs: add due_dates and hidden field examples to default config"
```
