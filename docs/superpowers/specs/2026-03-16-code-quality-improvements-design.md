# Pike Code Quality Improvements — Design Spec

## Context

A principal-level code review of Pike identified seven gaps keeping the project at B+ instead of A-. This spec addresses all seven through two parallel implementation tracks.

## Track 1 — Infrastructure

### 1. Makefile

Eight self-contained targets, no dependency chains between them:

| Target | Command |
|--------|---------|
| `build` | `go build -o pike ./cmd/pike` |
| `test` | `go test -race -count=1 ./...` |
| `lint` | `golangci-lint run` |
| `bench` | `go test -bench=. -benchmem ./...` |
| `fuzz` | Runs `go test -fuzz=. -fuzztime=30s` sequentially for `./internal/parser` and `./internal/query` (Go does not support `-fuzz` across multiple packages in one invocation) |
| `cover` | `go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out` |
| `golden-update` | `go test -run=TestGolden -update .` (root package only — only the root package defines the `-update` flag) |
| `install` | `go install ./cmd/pike` |

Add `coverage.out` to `.gitignore`.

### 2. Linting Configuration (`.golangci.yml`)

**Enabled linters (beyond defaults):**
- `errcheck` — catch ignored errors
- `govet` — catch suspicious constructs
- `staticcheck` — the gold standard for Go static analysis
- `unused` — dead code detection
- `ineffassign` — assignments to variables that are never read
- `gosimple` — simplification suggestions
- `gocritic` — opinionated but catches real bugs

**Intentionally not enabled:**
- `gofumpt`/`goimports` — style-only, defer to `gofmt`
- `exhaustive` — too noisy for this codebase's switch patterns
- `wrapcheck` — would flag existing `err` returns that don't wrap, too much churn

**Existing violations:** Fix all upfront. Expected violations are minimal — likely `errcheck` on `doublestar.Match` calls in `scanner.go` (patterns validated at construction time) and `time.Parse` calls in `parser.go` (dates validated before second parse). Either add `//nolint` comments with explanations or refactor slightly.

### 3. CI Pipeline (`.github/workflows/ci.yml`)

New workflow, separate from `release.yml`. Triggers on push to `main` and all PRs targeting `main`.

**Three parallel jobs:**

1. **test** — `go test -race -count=1 ./...` on ubuntu-latest with Go 1.25.x
2. **lint** — `golangci-lint` via the official GitHub Action
3. **fuzz** — separate steps per package (`./internal/parser` and `./internal/query`), each running `go test -fuzz=. -fuzztime=30s` (Go does not support `-fuzz` across multiple packages)

No coverage upload, no badges.

## Track 2 — Code Quality

### 4. Date Parsing Leniency + Warnings

**Parser changes (`internal/parser/parser.go`):**

Add a `normalizeDate` function that attempts to fix common date format issues before rejecting:
- `2026-3-16` → `2026-03-16` (zero-pad single-digit month/day)
- `2026/03/16` → `2026-03-16` (slash to dash)
- `2026.03.16` → `2026-03-16` (dot to dash)

If normalization succeeds, use the corrected date silently. If it fails, the date is genuinely unparseable.

**Behavioral change note:** This is a deliberate semantic change. Tasks with dates like `@due(2026/03/16)` that were previously treated as having no due date (value silently cleared) will now resolve to valid due dates. Users with existing files containing slash or dot-separated dates may see tasks appear in due-date views/queries that didn't before. This is the intended improvement.

**Warning collection:**

New `Warning` type in `internal/model/`:

```go
type Warning struct {
    File    string
    Line    int
    Message string
}
```

Change `ParseLine` signature:

```go
func ParseLine(line string, file string, lineNum int) (*model.Task, []model.Warning)
```

A malformed date that can't be normalized emits a warning like: `@due value "march-16" is not a valid date (expected YYYY-MM-DD)`. A successfully normalized date emits nothing.

**Surfacing warnings:**

Warnings are stored on the `Scanner` struct as a field (`s.Warnings []model.Warning`), populated during `Scan()` and `Refresh()`. This avoids changing the `([]model.Task, error)` return signature, which would cascade through the `scanFunc` closure type in `cmd/pike/main.go` and the TUI `Model.scanFunc` field. Callers access warnings via `s.Warnings` after calling `Scan()`/`Refresh()`.

- CLI query/summary modes: after scanning, check `s.Warnings` and print to stderr before output
- TUI: `main.go` wraps `scanFunc` to also read `s.Warnings` and pass them via a new `scanResultMsg.Warnings` field. TUI shows warning count in status bar (e.g., "2 parse warnings"), viewable via a key binding (`W`)

**Files touched:** `internal/model/task.go` (Warning type), `internal/parser/parser.go` (normalizeDate, ParseLine signature), `internal/scanner/scanner.go` (Warnings field, collection), `cmd/pike/main.go` (stderr output, scanFunc wrapper), `internal/tui/model.go` (scanResultMsg, warning state), `internal/tui/views.go` (status bar), `golden_test.go` (updated ParseLine call sites)

### 5. Scanner Benchmark

New file: `internal/scanner/scanner_bench_test.go`

- Creates temp directories with configurable file counts (100, 500, 1000)
- Each file contains ~50 lines with a mix of tasks and non-task content
- Benchmarks:
  - `Scan()` — cold, full parse
  - `Refresh()` — warm, no changes (measures mtime-skip overhead)
- Uses `b.ReportAllocs()` for allocation tracking

Measurement only. No optimization work.

### 6. TUI Test Coverage (65.7% → 85%+)

**State transition coverage (`internal/tui/model.go` — the `Update` method and `handleKey` live here):**
- Window resize handling (width/height propagation)
- Error display and clearing on next keypress
- `scanResultMsg` with config changes (tag color reload, editor cmd update)
- `scanResultMsg` during tag search mode (`TagSearchRefreshMsg` dispatch)
- `EditorFinishedMsg` with and without error
- `toggleResultMsg` error path
- View lock behavior (`SetFocusedView` disables mode-switching keys)

**Key handling edge cases:**
- Cursor at boundaries (first task, last task, empty sections)
- Page up/down across section boundaries
- Tab navigation skipping empty sections
- Key presses in each mode (dashboard, allTasks, tagSearch, recentlyCompleted)
- Filter activation/deactivation in each mode
- Toggle on a task with no checkbox

**Sub-model gaps:**
- FilterBar: mode switching preserves text, invalid DSL sets error, escape clears and exits
- TagSearch: tag list refresh while active, selection wrapping at boundaries, empty tag list

**Approach:** Table-driven tests using existing `testModel()` helper. Send messages directly to `Update()`, assert on model state. Target specific uncovered branches.

### 7. Fuzz Testing

**`internal/query/fuzz_test.go`:**
- `FuzzParse` — random strings into `query.Parse()`. Must never panic. Valid parses get run through `query.Eval()` against a fixed task set to verify the evaluator also doesn't panic.

**`internal/parser/fuzz_test.go`:**
- `FuzzParseLine` — random strings into `parser.ParseLine()`. Must never panic. Must never return a task with invalid state (e.g., non-nil `Due` with unparseable date). Returned warnings (from the updated `ParseLine` signature) must have non-empty `File` and `Message` fields and positive `Line` numbers.

**Seed corpus:** Representative inputs in `testdata/fuzz/` — valid queries, edge cases from existing unit tests, tricky patterns (empty strings, unicode, deeply nested parens, unclosed quotes).

**CI integration:** The fuzz job in CI runs each target for 30 seconds. Developers can run `make fuzz` locally for longer sessions.

## Ordering

Tracks are independent and can be worked in parallel:

**Track 1 order:** Makefile → `.golangci.yml` + fix violations → CI pipeline
**Track 2 order:** Date parsing + warnings → Scanner benchmark → TUI tests → Fuzz tests

The tracks converge when CI runs the new tests and fuzz targets added in Track 2.

## Out of Scope

- Scanner parallelization (deferred; benchmark added to measure when it matters)
- Coverage badges or upload
- Pre-commit hooks (CI is the enforcement point)
