# Pike: A- to A Improvements

**Date:** 2026-03-17
**Status:** Approved
**Scope:** Five targeted improvements to raise project quality from A- to A

## Overview

Five improvements identified from a principal-engineer-level code review, chosen for their effort-to-value ratio. Ordered by dependency: godoc (zero risk) → context threading (API change) → integration tests (validates pipeline) → CLI coverage (validates entry point) → TUI decomposition (structural refactor).

## 1. TUI Model Decomposition

### Problem
`keys.go` is 370 lines total, with `handleKey()` spanning 234 lines across 6 nested dispatch contexts. `tasks.go` is 446 lines mixing navigation, filtering, and mode transitions. The TUI package is the weakest-tested area (70.2% coverage).

### Design

**Navigator** (`internal/tui/navigator.go`):
- Stateful helper (not a tea.Model sub-model — no message passing overhead)
- Owns: `cursor int`, `height int`
- Methods that need section data accept `[]filter.ViewResult` as a parameter (not a stored callback) to avoid stale closure bugs with Bubble Tea's value semantics
- Navigator methods: `CursorUp()`, `CursorDown(sections)`, `JumpToNextSection(sections)`, `JumpToPrevSection(sections)`, `JumpToTop()`, `JumpToBottom(sections)`, `ClampCursor(sections)`, `FocusSection(sections, index)`, `Cursor() int`, `PageScroll(direction, sections)`
- Package-level functions: `flatTasks(sections) []model.Task`, `countFlatTasks(sections) int`
- Extracted from: `tasks.go` (10 cursor/navigation functions including `flatTasks`, `countFlatTasks`, `pageScroll`)

**Modes file** (`internal/tui/modes.go`):
- Methods on Model, moved from `tasks.go`
- Contains: `enterAllTasksMode()`, `enterQueryMode()`, `enterTagSearchMode()`, `enterRecentlyCompletedMode()`, `exitToDashboard()`
- Also contains: `rebuildSections()`, `rebuildDashboard()`, `rebuildSingleSection()`, filter-apply functions

**handleKey() simplification:**
- Cursor logic replaced by `m.nav.CursorUp()` etc.
- `toggleTask()` and `openEditor()` resolve the selected task via `flatTasks(m.displaySections())`
- Nested dispatch structure preserved (correctly models priority chain)
- Estimated: `handleKey()` from ~234 → ~180 lines
- `processFilterOutput`, `openEditor`, `resolveFilePath`, `toggleTask`, `toggleHiddenTag` remain in `keys.go`

**Files:**
- New: `navigator.go`, `navigator_test.go`, `modes.go`
- Modified: `model.go` (add `nav Navigator` field), `keys.go` (replace inline cursor logic), `tasks.go` (split into modes.go)

## 2. Integration Tests

### Problem
Unit tests are excellent but nothing exercises the full scan → parse → filter → render pipeline end-to-end with real markdown files.

### Design

**File:** `cmd/pike/integration_test.go`

Table-driven tests. Each case defines markdown files (as strings), a query, sort order, and expected golden output. Uses `t.TempDir()` for isolation. Calls `scanner.New()` → `scanner.Scan()` → `filter.Apply()` → `render.FormatTask()` directly.

**Test cases (8):**

| Case | Validates |
|------|-----------|
| Basic open tasks | scan → parse → filter `open` → render |
| Completed tasks | filter `completed` → correct state |
| Date query (`@due < today`) | date parsing → date comparison |
| Tag query (`@work`) | tag extraction → tag filtering |
| Regex query (`/deploy/`) | text matching across files |
| Multi-file scan | scanner picks up tasks from multiple .md files |
| Hidden tasks filtered | `@hidden` excluded by default |
| Pinned sort | `@pin` tasks partition to top |

Plus one summary-path case exercising the 4 hardcoded summary queries.

**Golden files:** `cmd/pike/testdata/integration/*.golden`, updatable via `-update` flag.

## 3. cmd/pike Coverage Improvements

### Problem
`cmd/pike/main.go` is at 64.6% coverage. Untested: color mode edge cases, config/scanner errors, `--view` flag, warning output, invalid queries.

### Design

**New tests in `cmd/pike/main_test.go`:**

| Test | Covers |
|------|--------|
| `TestResolveColorMode_ForceColor` | `--color` forces color on |
| `TestResolveColorMode_ForceNoColor` | `--no-color` disables color |
| `TestConfigLoadError` | Malformed YAML → error to stderr |
| `TestScannerError` | Invalid glob → scanner creation error |
| `TestViewFlag` | `--view "Section"` focuses a view |
| `TestSummaryWithNoTasks` | `--summary` on empty dir → zero counts |
| `TestInvalidQueryError` | `--query "((("` → parse error |
| `TestWarningOutput` | Malformed date → warning to stderr |

**Skipped:** `runTUI()` (requires terminal), `main()` (trivial wrapper).

**Target:** 64.6% → ~82-85%.

## 4. context.Context in toggle

### Problem
`scanner.Scan()` accepts `context.Context` but `toggle.Complete/Uncomplete/ToggleHidden` do not. In-flight toggles can't be cancelled.

### Design

**New signatures:**
```go
func Complete(ctx context.Context, filePath string, line int, date time.Time) error
func Uncomplete(ctx context.Context, filePath string, line int) error
func ToggleHidden(ctx context.Context, filePath string, line int) error
```

**Context check points:**
1. Before acquiring per-file mutex (early exit if cancelled)
2. Before atomic write (don't write if cancelled between read and write)

**Call site updates:**
- `internal/tui/keys.go`: pass `context.Background()` at toggle call sites (TUI cancellation context is a separate future concern)

**Test updates:**
- Update all existing calls to pass `context.Background()`
- Add 3 tests: one per function verifying cancelled context returns `context.Canceled` without modifying the file

## 5. Package godoc Comments

### Problem
All 13 packages lack `// Package X ...` doc comments. `go doc ./internal/...` returns bare output.

### Design

One-line comments on each package's primary `.go` file:

| Package | Comment |
|---------|---------|
| `cmd/pike` | `// Package main provides the pike CLI, a terminal task dashboard that reads markdown files.` |
| `model` | `// Package model defines the core data types for tasks, tags, and warnings.` |
| `parser` | `// Package parser extracts tasks and tags from markdown checkbox lines.` |
| `query` | `// Package query implements a DSL for filtering tasks by state, tags, dates, and text.` |
| `scanner` | `// Package scanner walks directories for markdown files with mtime-based caching.` |
| `filter` | `// Package filter applies query and sort pipelines to task collections.` |
| `sort` | `// Package sort provides task sorting strategies and pin partitioning.` |
| `toggle` | `// Package toggle performs atomic file mutations for task completion and visibility.` |
| `render` | `// Package render formats tasks for stdout as plain text, styled ANSI, or JSON.` |
| `style` | `// Package style provides tag coloring, link prettification, and ANSI formatting.` |
| `editor` | `// Package editor constructs commands to open files at specific lines in text editors.` |
| `config` | `// Package config loads and validates YAML configuration with sensible defaults.` |
| `tui` | `// Package tui implements the interactive Bubble Tea terminal dashboard.` |

## Implementation Order

1. **Godoc comments** (zero risk, no deps)
2. **context.Context in toggle** (API change, must happen before integration tests that might use it)
3. **Integration tests** (validates pipeline, independent of TUI)
4. **cmd/pike coverage** (validates entry point, can reference integration test patterns)
5. **TUI decomposition** (highest risk, benefits from all other tests being in place first)
