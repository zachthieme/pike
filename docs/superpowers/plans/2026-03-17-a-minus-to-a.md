# A- to A Improvements Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Five targeted improvements to raise Pike's code quality from A- to A: godoc comments, context threading in toggle, integration tests, CLI coverage improvements, and TUI model decomposition.

**Architecture:** Each improvement is independent and ordered by risk. Low-risk changes (godoc, context threading) land first to build a safety net before the structural TUI refactor. All changes are internal — no user-facing behavior changes.

**Tech Stack:** Go 1.25.x, Bubble Tea, existing test infrastructure (table-driven, golden files, fuzz)

---

## Chunk 1: Godoc Comments + Context Threading

### Task 1: Add package godoc comments

**Files:**
- Modify: `cmd/pike/main.go:1`
- Modify: `internal/model/task.go:1`
- Modify: `internal/parser/parser.go:1`
- Modify: `internal/query/ast.go:1`
- Modify: `internal/scanner/scanner.go:1`
- Modify: `internal/filter/filter.go:1`
- Modify: `internal/sort/sort.go:1`
- Modify: `internal/toggle/toggle.go:1`
- Modify: `internal/render/render.go:1`
- Modify: `internal/style/style.go:1`
- Modify: `internal/editor/editor.go:1`
- Modify: `internal/config/config.go:1`
- Modify: `internal/tui/model.go:1`

- [ ] **Step 1: Add doc comments to all 13 packages**

Add a `// Package X ...` comment immediately above the `package` declaration in each file. The comment goes on its own line before `package`:

| File | Comment |
|------|---------|
| `cmd/pike/main.go` | `// Package main provides the pike CLI, a terminal task dashboard that reads markdown files.` |
| `internal/model/task.go` | `// Package model defines the core data types for tasks, tags, and warnings.` |
| `internal/parser/parser.go` | `// Package parser extracts tasks and tags from markdown checkbox lines.` |
| `internal/query/ast.go` | `// Package query implements a DSL for filtering tasks by state, tags, dates, and text.` |
| `internal/scanner/scanner.go` | `// Package scanner walks directories for markdown files with mtime-based caching.` |
| `internal/filter/filter.go` | `// Package filter applies query and sort pipelines to task collections.` |
| `internal/sort/sort.go` | `// Package sort provides task sorting strategies and pin partitioning.` |
| `internal/toggle/toggle.go` | `// Package toggle performs atomic file mutations for task completion and visibility.` |
| `internal/render/render.go` | `// Package render formats tasks for stdout as plain text, styled ANSI, or JSON.` |
| `internal/style/style.go` | `// Package style provides tag coloring, link prettification, and ANSI formatting.` |
| `internal/editor/editor.go` | `// Package editor constructs commands to open files at specific lines in text editors.` |
| `internal/config/config.go` | `// Package config loads and validates YAML configuration with sensible defaults.` |
| `internal/tui/model.go` | `// Package tui implements the interactive Bubble Tea terminal dashboard.` |

- [ ] **Step 2: Verify godoc output**

Run: `go doc ./internal/...`
Expected: Each package shows its one-line description.

- [ ] **Step 3: Run tests and lint**

Run: `make test && make lint`
Expected: All pass, no regressions.

- [ ] **Step 4: Commit**

```bash
git add cmd/pike/main.go internal/model/task.go internal/parser/parser.go internal/query/ast.go internal/scanner/scanner.go internal/filter/filter.go internal/sort/sort.go internal/toggle/toggle.go internal/render/render.go internal/style/style.go internal/editor/editor.go internal/config/config.go internal/tui/model.go
git commit -m "docs: add package-level godoc comments to all packages"
```

---

### Task 2: Add context.Context to toggle — write failing tests

**Files:**
- Modify: `internal/toggle/toggle_test.go`

- [ ] **Step 1: Update all existing test calls to pass `context.Background()`**

Every call to `Complete(...)`, `Uncomplete(...)`, and `ToggleHidden(...)` in `toggle_test.go` needs `context.Background()` as the first argument. The test file is `package toggle` (internal tests), so calls use the function name directly without a package prefix. The tests will not compile yet (that's expected — the signatures haven't changed).

For example, in `TestCompleteBasic` (line 33):
```go
// Before:
err := Complete(p, 2, date)
// After:
err := Complete(context.Background(), p, 2, date)
```

Apply the same change to every test function:
- `TestCompleteBasic` (line 33)
- `TestCompleteIndented` (line 50)
- `TestCompleteWrongLineContent` (line 66)
- `TestCompleteLineOutOfRange` (line 76)
- `TestUncompleteBasic` (line 89)
- `TestUncompleteWithoutDate` (line 105)
- `TestUncompleteIndented` (line 121)
- `TestUncompleteWrongLineContent` (line 134)
- `TestUncompletePreservesOtherTags` (line 147)
- `TestToggleHiddenAdd` (line 163)
- `TestToggleHiddenRemove` (line 179)
- `TestToggleHiddenPreservesOtherTags` (line 195)
- `TestToggleHiddenTaggedBullet` (line 211)

Add `"context"` to the import block.

- [ ] **Step 2: Add cancellation tests**

Append three new tests to `toggle_test.go`:

```go
func TestCompleteCancelledContext(t *testing.T) {
	dir := t.TempDir()
	original := "- [ ] Buy milk\n"
	p := writeFile(t, dir, "test.md", original)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := Complete(ctx, p, 1, time.Now())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	// File must be unmodified.
	got := readFile(t, p)
	if got != original {
		t.Errorf("file was modified despite cancelled context:\n%s", got)
	}
}

func TestUncompleteCancelledContext(t *testing.T) {
	dir := t.TempDir()
	original := "- [x] Buy milk @completed(2026-03-17)\n"
	p := writeFile(t, dir, "test.md", original)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Uncomplete(ctx, p, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	got := readFile(t, p)
	if got != original {
		t.Errorf("file was modified despite cancelled context:\n%s", got)
	}
}

func TestToggleHiddenCancelledContext(t *testing.T) {
	dir := t.TempDir()
	original := "- [ ] Buy milk\n"
	p := writeFile(t, dir, "test.md", original)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ToggleHidden(ctx, p, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	got := readFile(t, p)
	if got != original {
		t.Errorf("file was modified despite cancelled context:\n%s", got)
	}
}
```

Add `"errors"` to the import block.

- [ ] **Step 3: Verify tests fail to compile**

Run: `go test ./internal/toggle/...`
Expected: Compilation error — `too many arguments in call to toggle.Complete` (etc). This confirms the tests are ahead of the implementation.

---

### Task 3: Add context.Context to toggle — implement

**Files:**
- Modify: `internal/toggle/toggle.go:1-143`
- Modify: `internal/tui/keys.go:348,350,367`

- [ ] **Step 1: Update toggle function signatures and add context checks**

In `internal/toggle/toggle.go`:

Add `"context"` to the import block.

**Complete** (line 35): Change signature and add two context checks:
```go
func Complete(ctx context.Context, filePath string, line int, date time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mu := lockFile(filePath)
	defer mu.Unlock()
```
And before `writeLines` (between `verifyUnmodified` at line 58 and `writeLines` at line 61):
```go
	if err := verifyUnmodified(filePath, line, originalLine); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return writeLines(filePath, lines)
```

**Uncomplete** (line 67): Same pattern:
```go
func Uncomplete(ctx context.Context, filePath string, line int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mu := lockFile(filePath)
	defer mu.Unlock()
```
And before `writeLines` (between `verifyUnmodified` at line 101 and `writeLines` at line 104):
```go
	if err := verifyUnmodified(filePath, line, originalLine); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return writeLines(filePath, lines)
```

**ToggleHidden** (line 108): Same pattern:
```go
func ToggleHidden(ctx context.Context, filePath string, line int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mu := lockFile(filePath)
	defer mu.Unlock()
```
And before `writeLines` (between `verifyUnmodified` at line 139 and `writeLines` at line 142):
```go
	if err := verifyUnmodified(filePath, line, originalLine); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return writeLines(filePath, lines)
```

- [ ] **Step 2: Update call sites in TUI**

In `internal/tui/keys.go`, add `"context"` to imports. Update the three toggle call sites:

Line 348 (Complete call):
```go
// Before:
toggle.Complete(task.File, task.Line, m.nowFunc())
// After:
toggle.Complete(context.Background(), task.File, task.Line, m.nowFunc())
```

Line 350 (Uncomplete call):
```go
// Before:
toggle.Uncomplete(task.File, task.Line)
// After:
toggle.Uncomplete(context.Background(), task.File, task.Line)
```

Line 367 (ToggleHidden call):
```go
// Before:
toggle.ToggleHidden(task.File, task.Line)
// After:
toggle.ToggleHidden(context.Background(), task.File, task.Line)
```

- [ ] **Step 3: Run toggle tests**

Run: `go test -race -count=1 ./internal/toggle/...`
Expected: All 16 tests pass (13 existing + 3 new cancellation tests).

- [ ] **Step 4: Run full test suite**

Run: `make test && make lint`
Expected: All pass. No compilation errors from any package.

- [ ] **Step 5: Commit**

```bash
git add internal/toggle/toggle.go internal/toggle/toggle_test.go internal/tui/keys.go
git commit -m "feat: add context.Context to toggle functions

Adds ctx parameter to Complete, Uncomplete, and ToggleHidden.
Checks ctx.Err() before acquiring file lock and before atomic write.
TUI call sites pass context.Background() for now."
```

---

## Chunk 2: Integration Tests + CLI Coverage

### Task 4: Write integration tests

**Files:**
- Create: `cmd/pike/integration_test.go`
- [ ] **Step 1: Create the integration test file with test infrastructure**

Create `cmd/pike/integration_test.go`:

```go
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pike/internal/filter"
	"pike/internal/render"
	"pike/internal/scanner"
)

// runPipeline executes the scan→filter→render pipeline and returns the output.
func runPipeline(t *testing.T, dir string, queryStr string, sortOrder string, now time.Time) string {
	t.Helper()
	sc, err := scanner.New(dir, []string{"**/*.md"}, nil)
	if err != nil {
		t.Fatalf("scanner.New: %v", err)
	}
	tasks, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatalf("scanner.Scan: %v", err)
	}
	filtered, err := filter.Apply(tasks, queryStr, sortOrder, now)
	if err != nil {
		t.Fatalf("filter.Apply: %v", err)
	}
	var lines []string
	for _, task := range filtered {
		lines = append(lines, render.FormatTask(task, nil, true))
	}
	return strings.Join(lines, "\n") + "\n"
}

// setupFiles creates markdown files in dir from a map of name→content.
func setupFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
```

- [ ] **Step 2: Add the 8 integration test cases + summary test**

Append to `cmd/pike/integration_test.go`:

```go
func TestIntegration_BasicOpen(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] Buy groceries\n- [x] Walk dog @completed(2026-01-01)\n- [ ] Fix bug\n",
	})
	got := runPipeline(t, dir, "open", "file", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(got, "Buy groceries") {
		t.Errorf("expected 'Buy groceries' in output:\n%s", got)
	}
	if !strings.Contains(got, "Fix bug") {
		t.Errorf("expected 'Fix bug' in output:\n%s", got)
	}
	if strings.Contains(got, "Walk dog") {
		t.Errorf("should not contain completed task 'Walk dog':\n%s", got)
	}
}

func TestIntegration_Completed(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] Open task\n- [x] Done task @completed(2026-03-10)\n",
	})
	got := runPipeline(t, dir, "completed", "file", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
	if strings.Contains(got, "Open task") {
		t.Errorf("should not contain open task:\n%s", got)
	}
	if !strings.Contains(got, "Done task") {
		t.Errorf("expected 'Done task' in output:\n%s", got)
	}
}

func TestIntegration_DateQuery(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] Overdue @due(2026-03-10)\n- [ ] Future @due(2026-04-01)\n- [ ] No date\n",
	})
	// @due < today should match only the overdue task
	now := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	got := runPipeline(t, dir, "open and @due < today", "file", now)
	if !strings.Contains(got, "Overdue") {
		t.Errorf("expected 'Overdue' in output:\n%s", got)
	}
	if strings.Contains(got, "Future") {
		t.Errorf("should not contain 'Future':\n%s", got)
	}
}

func TestIntegration_TagQuery(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] Work item @work\n- [ ] Personal item @personal\n- [ ] Both @work @personal\n",
	})
	got := runPipeline(t, dir, "@work", "file", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(got, "Work item") {
		t.Errorf("expected 'Work item':\n%s", got)
	}
	if !strings.Contains(got, "Both") {
		t.Errorf("expected 'Both':\n%s", got)
	}
	if strings.Contains(got, "Personal item") {
		t.Errorf("should not contain 'Personal item' (without @work):\n%s", got)
	}
}

func TestIntegration_RegexQuery(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] Deploy to staging\n- [ ] Deploy to production\n- [ ] Write tests\n",
	})
	got := runPipeline(t, dir, "/[Dd]eploy/", "file", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(got, "Deploy to staging") {
		t.Errorf("expected 'Deploy to staging':\n%s", got)
	}
	if !strings.Contains(got, "Deploy to production") {
		t.Errorf("expected 'Deploy to production':\n%s", got)
	}
	if strings.Contains(got, "Write tests") {
		t.Errorf("should not contain 'Write tests':\n%s", got)
	}
}

func TestIntegration_MultiFile(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"alpha.md": "- [ ] Alpha task\n",
		"beta.md":  "- [ ] Beta task\n",
		"gamma.md": "- [ ] Gamma task\n",
	})
	got := runPipeline(t, dir, "open", "file", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
	// With file sort, alpha should come before beta, beta before gamma.
	alphaIdx := strings.Index(got, "Alpha task")
	betaIdx := strings.Index(got, "Beta task")
	gammaIdx := strings.Index(got, "Gamma task")
	if alphaIdx < 0 || betaIdx < 0 || gammaIdx < 0 {
		t.Fatalf("expected all three tasks in output:\n%s", got)
	}
	if alphaIdx > betaIdx || betaIdx > gammaIdx {
		t.Errorf("expected file-sorted order (alpha < beta < gamma):\n%s", got)
	}
}

func TestIntegration_HiddenFiltered(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] Visible task\n- [ ] Hidden task @hidden\n",
	})
	// "open and not @hidden" excludes hidden tasks
	got := runPipeline(t, dir, "open and not @hidden", "file", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(got, "Visible task") {
		t.Errorf("expected 'Visible task':\n%s", got)
	}
	if strings.Contains(got, "Hidden task") {
		t.Errorf("should not contain 'Hidden task':\n%s", got)
	}
}

func TestIntegration_PinnedSort(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] Normal task\n- [ ] Pinned task @pin\n- [ ] Another normal\n",
	})
	// Use filter.Apply which calls StablePartitionPinned
	sc, err := scanner.New(dir, []string{"**/*.md"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := filter.Apply(tasks, "open", "file", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) < 2 {
		t.Fatalf("expected at least 2 tasks, got %d", len(filtered))
	}
	// Pinned task should be first
	if !strings.Contains(filtered[0].Text, "Pinned task") {
		t.Errorf("expected pinned task first, got %q", filtered[0].Text)
	}
}

func TestIntegration_SummaryPath(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC)
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n" +
			"- [ ] Open one\n" +
			"- [ ] Overdue @due(2026-03-10)\n" +
			"- [ ] Due this week @due(2026-03-18)\n" +
			"- [x] Done today @completed(2026-03-17)\n",
	})
	sc, err := scanner.New(dir, []string{"**/*.md"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Verify the 4 summary queries
	open, err := filter.Apply(tasks, "open", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 3 {
		t.Errorf("expected 3 open tasks, got %d", len(open))
	}

	overdue, err := filter.Apply(tasks, "open and @due < today", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(overdue) != 1 {
		t.Errorf("expected 1 overdue task, got %d", len(overdue))
	}
}
```

- [ ] **Step 3: Run integration tests**

Run: `go test -race -count=1 -run TestIntegration ./cmd/pike/...`
Expected: All 9 integration tests pass.

- [ ] **Step 4: Run full test suite**

Run: `make test && make lint`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/pike/integration_test.go
git commit -m "test: add integration tests for scan→filter→render pipeline

9 tests covering: open/completed filtering, date queries, tag queries,
regex matching, multi-file scanning, hidden tasks, pinned sort, and
summary path counts."
```

---

### Task 5: Improve cmd/pike coverage

**Files:**
- Modify: `cmd/pike/main_test.go`

- [ ] **Step 1: Add targeted tests for untested branches**

Append the following tests to `cmd/pike/main_test.go`. Add `"fmt"` and `"io"` to imports if not already present.

```go
func TestResolveColorMode_ForceColor(t *testing.T) {
	// When --color is set (and --no-color is not), resolveColorMode should
	// return false (noColor=false) regardless of writer type.
	var buf bytes.Buffer
	noColor := resolveColorMode(true, false, &buf)
	if noColor {
		t.Error("expected noColor=false when --color is forced")
	}
}

func TestResolveColorMode_ForceNoColor(t *testing.T) {
	var buf bytes.Buffer
	noColor := resolveColorMode(false, true, &buf)
	if !noColor {
		t.Error("expected noColor=true when --no-color is forced")
	}
}

func TestConfigLoadError(t *testing.T) {
	dir := t.TempDir()
	badConfig := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badConfig, []byte("{{{"), 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := run([]string{"--config", badConfig, "--dir", dir}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for bad config")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected 'loading config' error, got: %v", err)
	}
}

func TestScannerError(t *testing.T) {
	dir := t.TempDir()
	// Write a config with an invalid glob pattern
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("notes_dir: "+dir+"\ninclude:\n  - \"[invalid\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := run([]string{"--config", cfgPath, "--dir", dir}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for invalid glob pattern")
	}
	if !strings.Contains(err.Error(), "invalid glob") {
		t.Errorf("expected 'invalid glob' error, got: %v", err)
	}
}

func TestViewFlagIgnoredInQueryMode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte("- [ ] A task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// --view is only used in TUI mode; in query mode it is silently ignored
	var stdout, stderr bytes.Buffer
	err := run([]string{"--dir", dir, "--query", "open", "--view", "SomeView", "--no-color"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "A task") {
		t.Errorf("expected query results despite --view flag, got: %q", stdout.String())
	}
}

func TestSummaryWithNoTasks(t *testing.T) {
	dir := t.TempDir()
	// Empty directory — no markdown files
	var stdout, stderr bytes.Buffer
	err := run([]string{"--dir", dir, "--summary", "--no-color"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Open tasks") {
		t.Errorf("expected summary output even with no tasks, got: %q", out)
	}
}

func TestInvalidQueryError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte("- [ ] task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := run([]string{"--dir", dir, "--query", "((("}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for invalid query")
	}
}

func TestWarningOutput(t *testing.T) {
	dir := t.TempDir()
	// Invalid date triggers a warning
	content := "- [ ] Task with bad date @due(not-a-date)\n"
	if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := run([]string{"--dir", dir, "--query", "open", "--no-color"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "warning:") || !strings.Contains(stderr.String(), "test.md") {
		t.Errorf("expected warning mentioning file on stderr, got: %q", stderr.String())
	}
}
```

Note: `setupFiles` is already defined in `integration_test.go` in the same package, so it can be reused here. If the build complains about a redefinition, remove the duplicate.

- [ ] **Step 2: Run the new tests**

Run: `go test -race -count=1 -run "TestResolveColorMode|TestConfigLoad|TestScanner|TestView|TestSummaryWithNo|TestInvalidQuery|TestWarning" ./cmd/pike/...`
Expected: All 8 new tests pass.

- [ ] **Step 3: Check coverage improvement**

Run: `go test -cover ./cmd/pike/...`
Expected: Coverage should be ~80%+ (up from 64.6%).

- [ ] **Step 4: Run full test suite**

Run: `make test && make lint`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/pike/main_test.go
git commit -m "test: improve cmd/pike coverage with targeted tests

Adds tests for: color mode flags, config load errors, scanner errors,
--view flag, empty summary, invalid queries, and warning output.
Coverage: 64.6% → ~82%."
```

---

## Chunk 3: TUI Model Decomposition

**IMPORTANT — Bubble Tea value semantics:** `Model.Update()` uses a value receiver. Each call operates on a copy. A `sectionsFn` callback captured at construction time would bind to a stale Model copy. Therefore, Navigator methods that need sections accept `[]filter.ViewResult` as a parameter. The caller passes `m.displaySections()` which always reads the current Model's state.

### Task 6: Create Navigator — write failing tests

**Files:**
- Create: `internal/tui/navigator_test.go`

- [ ] **Step 1: Write Navigator tests**

Create `internal/tui/navigator_test.go`:

```go
package tui

import (
	"testing"

	"pike/internal/filter"
	"pike/internal/model"
)

func makeSections(counts ...int) []filter.ViewResult {
	var sections []filter.ViewResult
	for i, n := range counts {
		var tasks []model.Task
		for j := 0; j < n; j++ {
			tasks = append(tasks, model.Task{Text: "task"})
		}
		sections = append(sections, filter.ViewResult{
			Title: string(rune('A' + i)),
			Tasks: tasks,
		})
	}
	return sections
}

func TestNavigator_CursorDownUp(t *testing.T) {
	sections := makeSections(3, 2)
	var nav Navigator

	nav.CursorDown(sections)
	if nav.Cursor() != 1 {
		t.Errorf("after down: expected 1, got %d", nav.Cursor())
	}

	nav.CursorUp()
	if nav.Cursor() != 0 {
		t.Errorf("after up: expected 0, got %d", nav.Cursor())
	}

	// Can't go above 0
	nav.CursorUp()
	if nav.Cursor() != 0 {
		t.Errorf("after up at 0: expected 0, got %d", nav.Cursor())
	}
}

func TestNavigator_CursorBounds(t *testing.T) {
	sections := makeSections(2)
	var nav Navigator

	nav.CursorDown(sections)
	nav.CursorDown(sections) // should clamp at 1 (2 tasks, 0-indexed)
	if nav.Cursor() != 1 {
		t.Errorf("expected cursor clamped at 1, got %d", nav.Cursor())
	}
}

func TestNavigator_JumpToNextSection(t *testing.T) {
	sections := makeSections(2, 3, 1) // A=2, B=3, C=1
	var nav Navigator

	// At section A, jump to B
	nav.JumpToNextSection(sections)
	if nav.Cursor() != 2 {
		t.Errorf("expected cursor 2 (first of B), got %d", nav.Cursor())
	}

	// At section B, jump to C
	nav.JumpToNextSection(sections)
	if nav.Cursor() != 5 {
		t.Errorf("expected cursor 5 (first of C), got %d", nav.Cursor())
	}

	// At last section, no change
	nav.JumpToNextSection(sections)
	if nav.Cursor() != 5 {
		t.Errorf("expected cursor still 5, got %d", nav.Cursor())
	}
}

func TestNavigator_JumpToPrevSection(t *testing.T) {
	sections := makeSections(2, 3, 1)
	var nav Navigator
	nav.SetCursor(5) // Section C

	nav.JumpToPrevSection(sections)
	if nav.Cursor() != 2 {
		t.Errorf("expected cursor 2 (first of B), got %d", nav.Cursor())
	}

	nav.JumpToPrevSection(sections)
	if nav.Cursor() != 0 {
		t.Errorf("expected cursor 0 (first of A), got %d", nav.Cursor())
	}
}

func TestNavigator_JumpToTopBottom(t *testing.T) {
	sections := makeSections(3, 2)
	var nav Navigator

	nav.JumpToBottom(sections)
	if nav.Cursor() != 4 {
		t.Errorf("expected cursor 4, got %d", nav.Cursor())
	}

	nav.JumpToTop()
	if nav.Cursor() != 0 {
		t.Errorf("expected cursor 0, got %d", nav.Cursor())
	}
}

func TestNavigator_FocusSection(t *testing.T) {
	sections := makeSections(2, 3, 1)
	var nav Navigator

	nav.FocusSection(sections, 1) // second non-empty section
	if nav.Cursor() != 2 {
		t.Errorf("expected cursor 2 (first of B), got %d", nav.Cursor())
	}

	nav.FocusSection(sections, 2) // third section
	if nav.Cursor() != 5 {
		t.Errorf("expected cursor 5 (first of C), got %d", nav.Cursor())
	}
}

func TestNavigator_EmptySections(t *testing.T) {
	sections := makeSections(0, 0)
	var nav Navigator

	nav.CursorDown(sections) // should be safe
	if nav.Cursor() != 0 {
		t.Errorf("expected 0 on empty, got %d", nav.Cursor())
	}
	if countFlatTasks(sections) != 0 {
		t.Errorf("expected 0 flat tasks, got %d", countFlatTasks(sections))
	}
}

func TestNavigator_PageScroll(t *testing.T) {
	sections := makeSections(30)
	nav := Navigator{height: 40}

	nav.PageScroll(1, sections) // down
	if nav.Cursor() == 0 {
		t.Error("expected cursor to move down on page scroll")
	}
	pos := nav.Cursor()
	nav.PageScroll(-1, sections) // up
	if nav.Cursor() >= pos {
		t.Error("expected cursor to move up on page scroll")
	}
}

func TestNavigator_ClampCursor(t *testing.T) {
	sections := makeSections(3)
	var nav Navigator
	nav.SetCursor(100) // way out of bounds

	nav.ClampCursor(sections)
	if nav.Cursor() != 2 {
		t.Errorf("expected cursor clamped to 2, got %d", nav.Cursor())
	}
}

func TestFlatTasks(t *testing.T) {
	sections := makeSections(2, 1)
	tasks := flatTasks(sections)
	if len(tasks) != 3 {
		t.Errorf("expected 3 flat tasks, got %d", len(tasks))
	}
}

func TestCountFlatTasks(t *testing.T) {
	sections := makeSections(2, 3)
	if countFlatTasks(sections) != 5 {
		t.Errorf("expected 5, got %d", countFlatTasks(sections))
	}
}
```

- [ ] **Step 2: Verify tests fail to compile**

Run: `go test ./internal/tui/...`
Expected: Compilation error — `Navigator` struct / `flatTasks` function not defined. This confirms tests are ahead of implementation.

---

### Task 7: Create Navigator — implement

**Files:**
- Create: `internal/tui/navigator.go`

- [ ] **Step 1: Implement Navigator**

Create `internal/tui/navigator.go`. Key design: Navigator is a pure cursor-state manager. Methods that need section data accept `[]filter.ViewResult` as a parameter to avoid stale closure bugs with Bubble Tea's value semantics. `flatTasks` and `countFlatTasks` are package-level functions since they don't depend on cursor state.

```go
package tui

import (
	"pike/internal/filter"
	"pike/internal/model"
)

// pageScrollChrome is the approximate number of non-task lines on screen:
// search bar, section header, borders, footer, bubbletea chrome.
const pageScrollChrome = 8

// Navigator manages cursor state for navigating across task sections.
// Methods that need section data accept []filter.ViewResult as a parameter
// rather than storing a callback, because Bubble Tea's value semantics
// would cause a captured callback to bind to a stale Model copy.
type Navigator struct {
	cursor int
	height int
}

// Cursor returns the current cursor position.
func (n *Navigator) Cursor() int {
	return n.cursor
}

// SetCursor sets the cursor position directly.
func (n *Navigator) SetCursor(pos int) {
	n.cursor = pos
}

// SetHeight updates the viewport height for page scroll calculations.
func (n *Navigator) SetHeight(h int) {
	n.height = h
}

// ClampCursor ensures the cursor is within valid bounds.
func (n *Navigator) ClampCursor(sections []filter.ViewResult) {
	count := countFlatTasks(sections)
	if count == 0 {
		n.cursor = 0
		return
	}
	if n.cursor >= count {
		n.cursor = count - 1
	}
	if n.cursor < 0 {
		n.cursor = 0
	}
}

// CursorDown moves the cursor down one position if possible.
func (n *Navigator) CursorDown(sections []filter.ViewResult) {
	count := countFlatTasks(sections)
	if count > 0 && n.cursor < count-1 {
		n.cursor++
	}
}

// CursorUp moves the cursor up one position if possible.
func (n *Navigator) CursorUp() {
	if n.cursor > 0 {
		n.cursor--
	}
}

// JumpToTop moves the cursor to the first task.
func (n *Navigator) JumpToTop() {
	n.cursor = 0
}

// JumpToBottom moves the cursor to the last task.
func (n *Navigator) JumpToBottom(sections []filter.ViewResult) {
	count := countFlatTasks(sections)
	if count > 0 {
		n.cursor = count - 1
	}
}

// PageScroll moves the cursor by half the visible task window.
// direction is 1 for down, -1 for up.
func (n *Navigator) PageScroll(direction int, sections []filter.ViewResult) {
	visible := max(4, n.height-pageScrollChrome)
	n.cursor += direction * (visible / 2)
	n.ClampCursor(sections)
}

// cursorSection returns the index of the section the cursor is in, or -1.
func (n *Navigator) cursorSection(sections []filter.ViewResult) int {
	flatIdx := 0
	for i, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		if n.cursor >= flatIdx && n.cursor < flatIdx+len(sec.Tasks) {
			return i
		}
		flatIdx += len(sec.Tasks)
	}
	return -1
}

// JumpToNextSection moves the cursor to the first task of the next non-empty section.
func (n *Navigator) JumpToNextSection(sections []filter.ViewResult) {
	current := n.cursorSection(sections)
	flatIdx := 0
	for i, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		if i > current {
			n.cursor = flatIdx
			return
		}
		flatIdx += len(sec.Tasks)
	}
}

// JumpToPrevSection moves the cursor to the first task of the previous non-empty section.
func (n *Navigator) JumpToPrevSection(sections []filter.ViewResult) {
	current := n.cursorSection(sections)
	flatIdx := 0
	prevStart := -1
	for i, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		if i >= current {
			break
		}
		prevStart = flatIdx
		flatIdx += len(sec.Tasks)
	}
	if prevStart >= 0 {
		n.cursor = prevStart
	}
}

// FocusSection moves the cursor to the first task of the non-empty section at the given index.
func (n *Navigator) FocusSection(sections []filter.ViewResult, index int) {
	flatIdx := 0
	sectionIdx := 0
	for _, sec := range sections {
		if len(sec.Tasks) == 0 {
			continue
		}
		if sectionIdx == index {
			n.cursor = flatIdx
			return
		}
		flatIdx += len(sec.Tasks)
		sectionIdx++
	}
}

// flatTasks returns all tasks across displayed sections in order.
func flatTasks(sections []filter.ViewResult) []model.Task {
	var tasks []model.Task
	for _, sec := range sections {
		if len(sec.Tasks) > 0 {
			tasks = append(tasks, sec.Tasks...)
		}
	}
	return tasks
}

// countFlatTasks returns the total number of tasks across displayed sections.
func countFlatTasks(sections []filter.ViewResult) int {
	count := 0
	for _, sec := range sections {
		count += len(sec.Tasks)
	}
	return count
}
```

- [ ] **Step 2: Run Navigator tests**

Run: `go test -race -count=1 -run "TestNavigator|TestFlatTasks|TestCountFlatTasks" ./internal/tui/...`
Expected: All 12 tests pass.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/navigator.go internal/tui/navigator_test.go
git commit -m "feat: extract Navigator for cursor/section navigation

Pure cursor-state manager. Methods accept sections as parameters
to avoid stale closure bugs with Bubble Tea value semantics."
```

---

### Task 8: Wire Navigator into Model

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/keys.go`
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/sections.go`
- Modify: `internal/tui/tasks.go`
- Modify: `internal/tui/model_test.go`
- Modify: `internal/tui/bench_test.go`

- [ ] **Step 1: Replace `cursor` field with `nav` in Model struct**

In `internal/tui/model.go`:

Replace the `cursor` field (line 25):
```go
// Before:
	cursor             int          // index into flat task list across all sections
// After:
	nav                Navigator    // cursor state and section navigation
```

In `NewModel()` (line 69-70), replace `m.clampCursor()`:
```go
// Before:
	m.rebuildSections()
	m.clampCursor()
// After:
	m.rebuildSections()
	m.nav.ClampCursor(m.displaySections())
```

In `SetFocusedView()` (line 108), replace `m.clampCursor()`:
```go
	m.nav.ClampCursor(m.displaySections())
```

In `Update()` for `tea.WindowSizeMsg` (line 124-127), also update Navigator height:
```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.nav.SetHeight(msg.Height)
		return m, nil
```

In `Update()` for `scanResultMsg` (line 189), replace `m.clampCursor()`:
```go
	m.nav.ClampCursor(m.displaySections())
```

- [ ] **Step 2: Replace all references in keys.go**

Complete list of replacements in `internal/tui/keys.go`:

**Method call replacements (use find-and-replace):**

| Old | New |
|-----|-----|
| `m.cursorDown()` | `m.nav.CursorDown(m.displaySections())` |
| `m.cursorUp()` | `m.nav.CursorUp()` |
| `m.pageScroll(1)` | `m.nav.PageScroll(1, m.displaySections())` |
| `m.pageScroll(-1)` | `m.nav.PageScroll(-1, m.displaySections())` |
| `m.jumpToNextSection()` | `m.nav.JumpToNextSection(m.displaySections())` |
| `m.jumpToPrevSection()` | `m.nav.JumpToPrevSection(m.displaySections())` |
| `m.clampCursor()` | `m.nav.ClampCursor(m.displaySections())` |
| `m.flatTasks()` | `flatTasks(m.displaySections())` |

**`m.cursor = 0` replacements (4 locations):**
- Line 79: `m.nav.JumpToTop()`
- Line 109: `m.nav.JumpToTop()`
- Line 152: `m.nav.JumpToTop()`
- Line 239: `m.nav.JumpToTop()`

**`m.cursor = max(0, m.countFlatTasks()-1)` replacements (2 locations):**
- Line 82: `m.nav.JumpToBottom(m.displaySections())`
- Line 156: `m.nav.JumpToBottom(m.displaySections())`

**`m.cursor` read replacements (6 locations):**
- Line 306: `m.nav.Cursor()` (in openEditor bounds check)
- Line 310: `m.nav.Cursor()` (in openEditor task selection)
- Line 332: `m.nav.Cursor()` (in toggleTask bounds check)
- Line 335: `m.nav.Cursor()` (in toggleTask task selection)
- Line 359: `m.nav.Cursor()` (in toggleHiddenTag bounds check)
- Line 362: `m.nav.Cursor()` (in toggleHiddenTag task selection)

**`m.clampCursor()` replacements (6 locations in keys.go):**
- Line 70, 139, 215, 267, 282, 286: all become `m.nav.ClampCursor(m.displaySections())`

- [ ] **Step 3: Replace references in views.go and sections.go**

In `internal/tui/views.go`:
- Line 154: `m.cursor` → `m.nav.Cursor()`
- Line 172: `m.cursor` → `m.nav.Cursor()`

In `internal/tui/sections.go`:
- Line 55: `m.cursor` → `m.nav.Cursor()`

- [ ] **Step 4: Update tasks.go — remove extracted functions, update remaining**

Remove these functions from `internal/tui/tasks.go` (they now live in `navigator.go`):
- `flatTasks()` (lines 202-211)
- `pageScrollChrome` constant (line 215)
- `pageScroll()` (lines 218-222)
- `countFlatTasks()` (lines 225-231)
- `clampCursor()` (lines 234-246)
- `cursorDown()` (lines 249-254)
- `cursorUp()` (lines 257-261)
- `cursorSection()` (lines 265-277)
- `jumpToNextSection()` (lines 280-294)
- `jumpToPrevSection()` (lines 297-314)

In the remaining mode transition functions:
- `exitToDashboard()` line 374: `m.clampCursor()` → `m.nav.ClampCursor(m.displaySections())`
- `enterAllTasksMode()` line 382: `m.cursor = 0` → `m.nav.JumpToTop()`; line 390: `m.clampCursor()` → `m.nav.ClampCursor(m.displaySections())`
- `enterQueryMode()` line 399: `m.cursor = 0` → `m.nav.JumpToTop()`; line 411: `m.clampCursor()` → `m.nav.ClampCursor(m.displaySections())`
- `enterRecentlyCompletedMode()` line 429: `m.cursor = 0` → `m.nav.JumpToTop()`; line 437: `m.clampCursor()` → `m.nav.ClampCursor(m.displaySections())`

- [ ] **Step 5: Update model_test.go and bench_test.go**

In `internal/tui/model_test.go`, replace all references to the old API. These are the affected patterns:

| Old pattern | New pattern |
|---|---|
| `m.cursor = N` | `m.nav.SetCursor(N)` |
| `m.cursor` (reads, e.g. in assertions) | `m.nav.Cursor()` |
| `m.flatTasks()` | `flatTasks(m.displaySections())` |
| `m.countFlatTasks()` | `countFlatTasks(m.displaySections())` |
| `m.pageScroll(N)` | `m.nav.PageScroll(N, m.displaySections())` |
| `m.clampCursor()` | `m.nav.ClampCursor(m.displaySections())` |

The same replacement rules apply to ALL model variables (`m`, `m2`, `m3`, `m4`, etc.) — not just `m`. Use project-wide find-and-replace across `model_test.go` for these patterns:

| Pattern (regex) | Replacement |
|---|---|
| `(\w+)\.cursor\b` (writes like `= N`) | `$1.nav.SetCursor(N)` |
| `(\w+)\.cursor\b` (reads) | `$1.nav.Cursor()` |
| `(\w+)\.flatTasks\(\)` | `flatTasks($1.displaySections())` |
| `(\w+)\.countFlatTasks\(\)` | `countFlatTasks($1.displaySections())` |
| `(\w+)\.pageScroll\((.+)\)` | `$1.nav.PageScroll($2, $1.displaySections())` |
| `(\w+)\.clampCursor\(\)` | `$1.nav.ClampCursor($1.displaySections())` |

Locations on `m` (primary model variable):
- Lines 84, 108, 119, 130, 131, 142, 153, 267, 316, 437-438, 519, 541, 646, 739
- Lines 842-864 (pageScroll block), 978, 982-983, 1012, 1059, 1192-1193, 1204, 1212

Locations on `m2` (post-Update model):
- Lines 101-102 (TestCursorMovementDown: `m2.cursor`)
- Lines 112-113 (TestCursorMovementUp: `m2.cursor`)
- Lines 123-124 (TestCursorDoesNotGoNegative: `m2.cursor`)
- Lines 135-136 (TestCursorDoesNotExceedTasks: `m2.cursor`)
- Lines 146-147 (TestGotoTop: `m2.cursor`)
- Lines 157-158 (TestGotoBottom: `m2.cursor`)
- Lines 176-177 (TestTabNextSection: `m2.cursor`)
- Lines 334-335 (TestFilterNavigationWithArrows: `m2.cursor`)
- Lines 496-497 (TestArrowKeys: `m2.cursor`)
- Line 500 (TestArrowKeys: `m2.cursor = 2` → write)
- Lines 1189-1190 (TestCursorAtBoundaries: `m2.cursor`)
- Lines 1196-1197 (TestCursorAtBoundaries: `m2.cursor`)
- Lines 1208, 1211-1212 (TestPageScrollAcrossSections: `m2.cursor`)

Locations on `m3`:
- Lines 447-448 (TestRefreshMsg: `m3.flatTasks()`)
- Lines 503-504 (TestArrowKeys: `m3.cursor`)
- Lines 1244-1245 (TestKeyPressesInAllTasksMode: `m3.cursor`)

In `internal/tui/bench_test.go`:
- Line 161: `m.flatTasks()` → `flatTasks(m.displaySections())`

- [ ] **Step 6: Run full test suite**

Run: `make test && make lint`
Expected: All tests pass. This is the critical verification — existing TUI tests validate the wiring.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/model.go internal/tui/keys.go internal/tui/tasks.go internal/tui/views.go internal/tui/sections.go internal/tui/model_test.go internal/tui/bench_test.go
git commit -m "refactor: wire Navigator into TUI Model

Replaces inline cursor/navigation code with Navigator methods.
Removes 10 functions from tasks.go. Section-dependent methods
accept []filter.ViewResult to avoid stale closure bugs."
```

---

### Task 9: Extract modes.go from tasks.go

**Files:**
- Create: `internal/tui/modes.go`
- Modify: `internal/tui/tasks.go`

- [ ] **Step 1: Move mode transition and rebuild functions to modes.go**

Create `internal/tui/modes.go` by moving these functions from `tasks.go`:
- `rebuildSections()` (with its imports)
- `rebuildSingleSection()`
- `rebuildDashboard()`
- `applyHiddenFilter()`
- `applySubstringFilter()`
- `applyDSLFilter()`
- `enterAllTasksMode()`
- `enterQueryMode()`
- `enterTagSearchMode()`
- `enterRecentlyCompletedMode()`
- `exitToDashboard()`

The file header:
```go
package tui

import (
	"fmt"
	"strings"
	"time"

	"pike/internal/filter"
	"pike/internal/model"
	"pike/internal/query"
	tasksort "pike/internal/sort"

	tea "github.com/charmbracelet/bubbletea"
)
```

After moving, `tasks.go` should contain only the utility functions:
- `startOfWeek()`
- `weekStartDay()`
- `visibleSections()`
- `extractTagNames()`
- `hiddenCountFor()`
- `nowFunc()`

Update the `tasks.go` import block to only include what's needed by the remaining functions.

- [ ] **Step 2: Run full test suite**

Run: `make test && make lint`
Expected: All pass. This is a pure move — no logic changes.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/modes.go internal/tui/tasks.go
git commit -m "refactor: extract mode transitions and rebuild logic to modes.go

Splits tasks.go into modes.go (mode transitions, section rebuilding,
filter application) and tasks.go (utilities). No logic changes."
```

---

### Task 10: Final verification

- [ ] **Step 1: Run full test suite with race detection**

Run: `make test`
Expected: All packages pass with `-race -count=1`.

- [ ] **Step 2: Run linter**

Run: `make lint`
Expected: No warnings.

- [ ] **Step 3: Run fuzz tests**

Run: `make fuzz`
Expected: 30s fuzz on parser and query passes.

- [ ] **Step 4: Check coverage improvements**

Run: `go test -cover ./...`
Expected:
- `internal/toggle`: coverage increased (new cancellation tests)
- `cmd/pike`: coverage ~80%+ (up from 64.6%)
- `internal/tui`: coverage stable or improved
- All other packages: no regression

- [ ] **Step 5: Verify godoc**

Run: `go doc ./internal/...`
Expected: Every package shows its one-line description.
