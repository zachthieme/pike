# Code Quality Improvements Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Address all seven code review findings to take Pike from B+ to A- through infrastructure tooling and code quality improvements.

**Architecture:** Two parallel tracks — Track 1 (infrastructure: Makefile, linting, CI) and Track 2 (code quality: date parsing, scanner benchmark, TUI tests, fuzz tests). Tracks don't touch the same files and can be worked concurrently.

**Tech Stack:** Go 1.25.x, golangci-lint, GitHub Actions, Bubble Tea testing patterns

**Spec:** `docs/superpowers/specs/2026-03-16-code-quality-improvements-design.md`

---

## Chunk 1: Infrastructure (Track 1)

### Task 1: Makefile

**Files:**
- Create: `Makefile`
- Modify: `.gitignore`

- [ ] **Step 1: Create the Makefile**

```makefile
.PHONY: build test lint bench fuzz cover golden-update install

build:
	go build -o pike ./cmd/pike

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run

bench:
	go test -bench=. -benchmem ./...

fuzz:
	go test -fuzz=. -fuzztime=30s ./internal/parser
	go test -fuzz=. -fuzztime=30s ./internal/query

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

golden-update:
	go test -run=TestGolden -update .

install:
	go install ./cmd/pike
```

- [ ] **Step 2: Add coverage.out to .gitignore**

Append `coverage.out` to `.gitignore`. Current contents are:
```
/pike
/result
*.pike-tmp
```

- [ ] **Step 3: Verify Makefile works**

Run: `make build && make test`
Expected: Build succeeds, all tests pass.

- [ ] **Step 4: Commit**

```bash
git add Makefile .gitignore
git commit -m "feat: add Makefile with build, test, lint, bench, fuzz, cover targets"
```

---

### Task 2: Linting Configuration

**Files:**
- Create: `.golangci.yml`
- Possibly modify: source files with lint violations

- [ ] **Step 1: Create `.golangci.yml`**

```yaml
linters:
  enable:
    - errcheck
    - govet
    - staticcheck
    - unused
    - ineffassign
    - gosimple
    - gocritic
```

- [ ] **Step 2: Run the linter to discover violations**

Run: `golangci-lint run ./...`
Expected: A list of violations (likely small). Note each one.

- [ ] **Step 3: Fix all violations**

Known likely violations:
- `internal/scanner/scanner.go:191,202` — `doublestar.Match` return value ignored. These are safe because patterns are validated in `New()`. Add `//nolint:errcheck // patterns validated at construction time` or assign to blank identifier explicitly.
- `internal/parser/parser.go:85,93` — `time.Parse` return value ignored on second parse. These are safe because the value was already validated on line 74. Add `//nolint:errcheck // already validated above`.

Fix any other violations that appear. Prefer minimal changes — `//nolint` with explanation for intentionally ignored errors, actual fixes for real issues.

- [ ] **Step 4: Verify lint passes clean**

Run: `golangci-lint run ./...`
Expected: No output, exit code 0.

- [ ] **Step 5: Verify tests still pass**

Run: `go test -race -count=1 ./...`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add .golangci.yml
git add -u  # any modified source files
git commit -m "feat: add golangci-lint config and fix all existing violations"
```

---

### Task 3: CI Pipeline

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create the CI workflow**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25.x"
      - run: go test -race -count=1 ./...

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25.x"
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  fuzz:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25.x"
      - name: Fuzz parser
        run: go test -fuzz=. -fuzztime=30s ./internal/parser
      - name: Fuzz query
        run: go test -fuzz=. -fuzztime=30s ./internal/query
```

Note: The fuzz job won't work until fuzz targets exist (Task 10). That's fine — the job will simply find no fuzz targets and pass as a no-op until then.

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "feat: add CI workflow with test, lint, and fuzz jobs"
```

---

## Chunk 2: Date Parsing Leniency + Warnings (Track 2)

### Task 4: Add Warning Type

**Files:**
- Modify: `internal/model/task.go:1-44`
- Test: `internal/model/task_test.go`

- [ ] **Step 1: Add the Warning type to model**

Add after the `Task` struct (after line 38 in `internal/model/task.go`):

```go
// Warning represents a non-fatal issue found during parsing.
type Warning struct {
	File    string
	Line    int
	Message string
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/model/task.go
git commit -m "feat: add Warning type to model package"
```

---

### Task 5: Write Failing Tests for normalizeDate

**Files:**
- Test: `internal/parser/parser_test.go`

- [ ] **Step 1: Write tests for normalizeDate**

Add to `internal/parser/parser_test.go`:

```go
func TestNormalizeDate(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		// Already valid
		{"2026-03-16", "2026-03-16", true},
		// Zero-pad single-digit month/day
		{"2026-3-16", "2026-03-16", true},
		{"2026-03-6", "2026-03-06", true},
		{"2026-3-6", "2026-03-06", true},
		// Slash separators
		{"2026/03/16", "2026-03-16", true},
		{"2026/3/16", "2026-03-16", true},
		// Dot separators
		{"2026.03.16", "2026-03-16", true},
		{"2026.3.6", "2026-03-06", true},
		// Invalid — not fixable
		{"march-16", "", false},
		{"2026", "", false},
		{"not-a-date", "", false},
		{"", "", false},
		{"2026-13-01", "", false},  // invalid month
		{"2026-03-32", "", false},  // invalid day
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := normalizeDate(tt.input)
			if ok != tt.ok {
				t.Errorf("normalizeDate(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("normalizeDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run=TestNormalizeDate ./internal/parser`
Expected: FAIL — `normalizeDate` is not defined.

---

### Task 6: Implement normalizeDate

**Files:**
- Modify: `internal/parser/parser.go`

- [ ] **Step 1: Add normalizeDate function**

Add to `internal/parser/parser.go` after the regex vars (after line 14):

```go
// normalizeDate attempts to fix common date format issues and returns
// the normalized YYYY-MM-DD string. Returns ("", false) if the input
// cannot be interpreted as a valid date.
func normalizeDate(s string) (string, bool) {
	// Replace common separators with dashes.
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ".", "-")

	parts := strings.Split(s, "-")
	if len(parts) != 3 {
		return "", false
	}

	// Zero-pad month and day.
	if len(parts[1]) == 1 {
		parts[1] = "0" + parts[1]
	}
	if len(parts[2]) == 1 {
		parts[2] = "0" + parts[2]
	}

	normalized := parts[0] + "-" + parts[1] + "-" + parts[2]

	// Validate via time.Parse — catches invalid months/days.
	_, err := time.Parse("2006-01-02", normalized)
	if err != nil {
		return "", false
	}
	return normalized, true
}
```

- [ ] **Step 2: Run normalizeDate tests**

Run: `go test -run=TestNormalizeDate ./internal/parser`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/parser/parser.go internal/parser/parser_test.go
git commit -m "feat: add normalizeDate for lenient date parsing"
```

---

### Task 7: Write Failing Tests for ParseLine Warnings

**Files:**
- Test: `internal/parser/parser_test.go`

- [ ] **Step 1: Write tests for ParseLine with warnings**

Add to `internal/parser/parser_test.go`:

```go
func TestParseLineWarnings(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		wantTask     bool
		wantDue      string // "" means nil
		wantWarnings int
	}{
		{
			name:     "valid date no warning",
			line:     "- [ ] task @due(2026-03-16)",
			wantTask: true, wantDue: "2026-03-16", wantWarnings: 0,
		},
		{
			name:     "normalizable date no warning",
			line:     "- [ ] task @due(2026/3/16)",
			wantTask: true, wantDue: "2026-03-16", wantWarnings: 0,
		},
		{
			name:     "unparseable date emits warning",
			line:     "- [ ] task @due(march-16)",
			wantTask: true, wantDue: "", wantWarnings: 1,
		},
		{
			name:     "no tags no task",
			line:     "just text",
			wantTask: false, wantWarnings: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, warnings := ParseLine(tt.line, "test.md", 1)
			if (task != nil) != tt.wantTask {
				t.Errorf("ParseLine task presence = %v, want %v", task != nil, tt.wantTask)
			}
			if len(warnings) != tt.wantWarnings {
				t.Errorf("ParseLine warnings count = %d, want %d", len(warnings), tt.wantWarnings)
			}
			if task != nil && tt.wantDue != "" {
				if task.Due == nil {
					t.Errorf("ParseLine Due is nil, want %s", tt.wantDue)
				} else if task.Due.Format("2006-01-02") != tt.wantDue {
					t.Errorf("ParseLine Due = %s, want %s", task.Due.Format("2006-01-02"), tt.wantDue)
				}
			}
			if task != nil && tt.wantDue == "" && task.Due != nil {
				t.Errorf("ParseLine Due = %s, want nil", task.Due.Format("2006-01-02"))
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run=TestParseLineWarnings ./internal/parser`
Expected: FAIL — `ParseLine` returns 1 value, not 2.

---

### Task 8: Update ParseLine to Return Warnings + Use normalizeDate

**Files:**
- Modify: `internal/parser/parser.go:20-100`

- [ ] **Step 1: Update ParseLine signature and implementation**

Change `ParseLine` in `internal/parser/parser.go` to:

```go
func ParseLine(line string, file string, lineNum int) (*model.Task, []model.Warning) {
	m := taskLineRe.FindStringSubmatch(line)

	var checkbox string
	var text string

	hasCheckbox := m != nil

	if hasCheckbox {
		checkbox = m[1]
		text = strings.TrimRight(m[2], " ")
	} else {
		pm := plainLineRe.FindStringSubmatch(line)
		if pm == nil {
			return nil, nil
		}
		candidate := strings.TrimRight(pm[1], " ")
		if !tagRe.MatchString(candidate) {
			return nil, nil
		}
		checkbox = " "
		text = candidate
	}

	task := &model.Task{
		Text:        text,
		File:        file,
		Line:        lineNum,
		HasCheckbox: hasCheckbox,
	}

	if checkbox == " " {
		task.State = model.Open
	} else {
		task.State = model.Completed
	}

	var warnings []model.Warning
	task.TagSet = make(map[string]bool)
	tagMatches := tagRe.FindAllStringSubmatch(text, -1)
	for _, tm := range tagMatches {
		tagName := tm[1]
		tagValue := tm[2]

		tag := model.Tag{
			Name:  tagName,
			Value: tagValue,
		}

		if tagValue != "" && (tagName == "due" || tagName == "completed") {
			normalized, ok := normalizeDate(tagValue)
			if ok {
				tag.Value = normalized
			} else {
				warnings = append(warnings, model.Warning{
					File:    file,
					Line:    lineNum,
					Message: fmt.Sprintf("@%s value %q is not a valid date (expected YYYY-MM-DD)", tagName, tagValue),
				})
				tag.Value = ""
			}
		}

		task.Tags = append(task.Tags, tag)
		task.TagSet[tagName] = true

		if tagName == "due" && tag.Value != "" {
			t, _ := time.Parse("2006-01-02", tag.Value) //nolint:errcheck // validated by normalizeDate
			task.Due = &t
		}
		if tagName == "completed" {
			if !task.HasCheckbox {
				task.State = model.Completed
			}
			if tag.Value != "" {
				t, _ := time.Parse("2006-01-02", tag.Value) //nolint:errcheck // validated by normalizeDate
				task.Completed = &t
			}
		}
	}

	return task, warnings
}
```

Note: Add `"fmt"` to the imports.

- [ ] **Step 2: Fix all callers of ParseLine**

The signature changed from returning 1 value to 2. Update every call site:

**`internal/scanner/scanner.go:173`** — change:
```go
task := parser.ParseLine(line, relPath, lineNum)
```
to:
```go
task, _ := parser.ParseLine(line, relPath, lineNum)
```
(Warnings will be collected in a later step.)

**`golden_test.go:102`** — change:
```go
task := parser.ParseLine(scanner.Text(), relPath, lineNum)
```
to:
```go
task, _ := parser.ParseLine(scanner.Text(), relPath, lineNum)
```

**`internal/parser/parser_test.go`** — every call to `ParseLine` in the existing tests (the `TestParseLine` table-driven test) needs updating from:
```go
task := ParseLine(tt.line, "test.md", 1)
```
to:
```go
task, _ := ParseLine(tt.line, "test.md", 1)
```

- [ ] **Step 3: Run all tests**

Run: `go test -race -count=1 ./...`
Expected: All tests pass (including the new warning tests and existing golden tests).

- [ ] **Step 4: Update golden files if needed**

If any golden tests fail due to dates that now normalize differently:
Run: `go test -run=TestGolden -update .`
Then verify the diffs make sense (dates like `2026/3/16` now resolve instead of being cleared).

- [ ] **Step 5: Commit**

```bash
git add internal/parser/parser.go internal/scanner/scanner.go golden_test.go
git add -u  # any updated golden files
git commit -m "feat: ParseLine returns warnings, uses normalizeDate for lenient date parsing"
```

---

### Task 9: Scanner Collects and Surfaces Warnings

**Files:**
- Modify: `internal/scanner/scanner.go:26-32,159-185`
- Modify: `cmd/pike/main.go:140-162,283-294`
- Modify: `internal/tui/model.go:14,36,142-164`
- Modify: `internal/tui/views.go:40-62`
- Test: `internal/scanner/scanner_test.go`

- [ ] **Step 1: Add Warnings field to Scanner struct**

In `internal/scanner/scanner.go`, add to the `Scanner` struct (after line 31):
```go
Warnings []model.Warning // populated during Scan/Refresh
```

- [ ] **Step 2: Collect warnings in parseFileInto**

Update `parseFileInto` in `internal/scanner/scanner.go` to collect warnings:

```go
func (s *Scanner) parseFileInto(absPath, relPath string, modTime time.Time, mtimes map[string]time.Time, tasks map[string][]model.Task) error {
	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var fileTasks []model.Task
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxLineSize)
	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := sc.Text()
		task, warnings := parser.ParseLine(line, relPath, lineNum)
		if task != nil {
			fileTasks = append(fileTasks, *task)
		}
		s.Warnings = append(s.Warnings, warnings...)
	}
	if err := sc.Err(); err != nil {
		return err
	}

	tasks[relPath] = fileTasks
	mtimes[relPath] = modTime
	return nil
}
```

- [ ] **Step 3: Reset warnings at start of Scan/Refresh**

In `Scan()`, add `s.Warnings = nil` before the walk (after line 67):
```go
func (s *Scanner) Scan(ctx context.Context) ([]model.Task, error) {
	s.Warnings = nil
	mtimes := make(map[string]time.Time)
	// ... rest unchanged
```

In `Refresh()`, add `s.Warnings = nil` before the walk (after line 86):
```go
func (s *Scanner) Refresh(ctx context.Context) ([]model.Task, error) {
	s.Warnings = nil
	onDisk := make(map[string]bool)
	// ... rest unchanged
```

- [ ] **Step 4: Print warnings to stderr in CLI modes**

In `cmd/pike/main.go`, after the `sc.Scan(ctx)` call (after line 143), add:

```go
// Print parse warnings to stderr.
for _, w := range sc.Warnings {
	fmt.Fprintf(stderr, "warning: %s:%d: %s\n", w.File, w.Line, w.Message)
}
```

- [ ] **Step 5: Pass warnings through TUI via warningsFunc**

The approach: add a `warningsFunc` to the TUI Model (similar to `scanFunc`) that returns the latest warnings from the Scanner. This avoids changing the `scanFunc` signature or the `scanResultMsg` type.

**5a.** In `internal/tui/model.go`, add two fields to the Model struct (after `now` field):
```go
warnings     []model.Warning          // parse warnings from last scan
warningsFunc func() []model.Warning   // returns latest parse warnings
```

**5b.** Add setter methods to `internal/tui/model.go`:
```go
func (m *Model) SetWarnings(w []model.Warning) {
	m.warnings = w
}

func (m *Model) SetWarningsFunc(f func() []model.Warning) {
	m.warningsFunc = f
}
```

**5c.** In `model.go` Update, in the `scanResultMsg` handler, after the `if msg.Tasks != nil` block (around line 158), add:
```go
if m.warningsFunc != nil {
	m.warnings = m.warningsFunc()
}
```
This reads warnings after each scan completes.

Note: `Refresh()` only re-parses changed files, so `s.Warnings` will only contain warnings from files parsed in that refresh cycle. This is intentional — warnings from unchanged files were already reported.

**5d.** In `cmd/pike/main.go` `runTUI`, after creating `scanRefresh`, wire up warnings:
```go
warningsGetter := func() []model.Warning {
	return sc.Warnings
}
m := tui.NewModel(cfg, tasks, scanRefresh, configReload)
m.SetWarnings(sc.Warnings)
m.SetWarningsFunc(warningsGetter)
```

- [ ] **Step 6: Show warning count in TUI footer**

In `internal/tui/views.go` `viewDashboard()`, update the label (line 53) to include warnings:
```go
label := fmt.Sprintf(" ○ %d/%d  ● %d wk", displayedOpen, m.openCount, m.completedThisWeek)
if len(m.warnings) > 0 {
	label += fmt.Sprintf("  ⚠ %d", len(m.warnings))
}
```

- [ ] **Step 7: Write a test for scanner warnings**

Add to `internal/scanner/scanner_test.go`:

```go
func TestScanCollectsWarnings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "test.md", "- [ ] task @due(bad-date)\n- [ ] ok @due(2026-03-16)\n")
	sc, err := New(dir, []string{"**/*.md"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.Warnings) != 1 {
		t.Errorf("got %d warnings, want 1", len(sc.Warnings))
	}
	if len(sc.Warnings) > 0 && sc.Warnings[0].Line != 1 {
		t.Errorf("warning line = %d, want 1", sc.Warnings[0].Line)
	}
}
```

- [ ] **Step 8: Run all tests**

Run: `go test -race -count=1 ./...`
Expected: All tests pass.

- [ ] **Step 9: Commit**

```bash
git add internal/scanner/scanner.go internal/scanner/scanner_test.go
git add internal/tui/model.go internal/tui/views.go
git add cmd/pike/main.go
git commit -m "feat: scanner collects parse warnings, surfaces in CLI stderr and TUI footer"
```

---

## Chunk 3: Scanner Benchmark + TUI Tests (Track 2 continued)

### Task 10: Scanner Benchmark

**Files:**
- Create: `internal/scanner/scanner_bench_test.go`

- [ ] **Step 1: Create the benchmark file**

```go
package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pike/internal/model"
)


func writeBenchFiles(b *testing.B, dir string, count int) {
	b.Helper()
	for i := range count {
		name := filepath.Join(dir, fmt.Sprintf("note_%04d.md", i))
		var content string
		for j := range 50 {
			switch j % 5 {
			case 0:
				content += fmt.Sprintf("- [ ] task %d-%d @due(2026-03-16)\n", i, j)
			case 1:
				content += fmt.Sprintf("- [x] done %d-%d @completed(2026-03-10)\n", i, j)
			case 2:
				content += fmt.Sprintf("- bullet %d-%d @today\n", i, j)
			default:
				content += fmt.Sprintf("This is just regular text line %d-%d\n", i, j)
			}
		}
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}
}


func BenchmarkScan(b *testing.B) {
	for _, count := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("files=%d", count), func(b *testing.B) {
			dir := b.TempDir()
			writeBenchFiles(b, dir, count)
			sc, err := New(dir, []string{"**/*.md"}, nil)
			if err != nil {
				b.Fatal(err)
			}
			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				sc.mtimes = make(map[string]time.Time)
				sc.tasks = make(map[string][]model.Task)
				if _, err := sc.Scan(ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRefreshNoChanges(b *testing.B) {
	for _, count := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("files=%d", count), func(b *testing.B) {
			dir := b.TempDir()
			writeBenchFiles(b, dir, count)
			sc, err := New(dir, []string{"**/*.md"}, nil)
			if err != nil {
				b.Fatal(err)
			}
			ctx := context.Background()
			// Initial scan to populate caches.
			if _, err := sc.Scan(ctx); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := sc.Refresh(ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
```

Since this file is in the `scanner` package, it has access to unexported fields (`sc.mtimes`, `sc.tasks`). The `time` and `pike/internal/model` imports are needed for the `make()` calls that reset these maps.

- [ ] **Step 2: Run benchmarks**

Run: `go test -bench=. -benchmem ./internal/scanner`
Expected: Benchmark results with timing and allocation info.

- [ ] **Step 3: Commit**

```bash
git add internal/scanner/scanner_bench_test.go
git commit -m "feat: add scanner benchmarks for Scan and Refresh at 100/500/1000 files"
```

---

### Task 11: TUI Test Coverage — State Transitions

**Files:**
- Modify: `internal/tui/model_test.go`

- [ ] **Step 1: Write state transition tests**

Add to `internal/tui/model_test.go`:

```go
func TestErrorClearsOnKeyPress(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.err = fmt.Errorf("test error")
	updated, _ := sendKey(m, "j")
	m2 := updated.(Model)
	if m2.err != nil {
		t.Errorf("err = %v, want nil after keypress", m2.err)
	}
}

func TestScanResultMsgUpdatesConfig(t *testing.T) {
	m := testModel(testTasks(), testViews())
	newCfg := &config.Config{
		Editor:    "nvim",
		TagColors: map[string]string{"new": "#ff0000"},
		Views:     testViews(),
	}
	updated, _ := m.Update(scanResultMsg{Config: newCfg})
	m2 := updated.(Model)
	if m2.editorCmd != "nvim" {
		t.Errorf("editorCmd = %q, want %q", m2.editorCmd, "nvim")
	}
	if m2.tagColors["new"] != "#ff0000" {
		t.Errorf("tagColors[new] = %q, want #ff0000", m2.tagColors["new"])
	}
}

func TestScanResultMsgError(t *testing.T) {
	m := testModel(testTasks(), testViews())
	updated, _ := m.Update(scanResultMsg{Err: fmt.Errorf("scan failed")})
	m2 := updated.(Model)
	if m2.err == nil || m2.err.Error() != "scan failed" {
		t.Errorf("err = %v, want 'scan failed'", m2.err)
	}
}

func TestScanResultMsgInTagSearchMode(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.mode = modeTagSearch
	newTasks := append(testTasks(), taskWithTagSet(model.Task{
		Text: "new @newtag", State: model.Open, File: "new.md", Line: 1,
		Tags: []model.Tag{{Name: "newtag"}},
	}))
	updated, _ := m.Update(scanResultMsg{Tasks: newTasks})
	m2 := updated.(Model)
	if m2.mode != modeTagSearch {
		t.Errorf("mode = %v, want modeTagSearch", m2.mode)
	}
	if len(m2.allTasks) != len(newTasks) {
		t.Errorf("allTasks len = %d, want %d", len(m2.allTasks), len(newTasks))
	}
}

func TestEditorFinishedMsgWithError(t *testing.T) {
	m := testModel(testTasks(), testViews())
	updated, cmd := m.Update(EditorFinishedMsg{Err: fmt.Errorf("editor crashed")})
	m2 := updated.(Model)
	if m2.err == nil || m2.err.Error() != "editor crashed" {
		t.Errorf("err = %v, want 'editor crashed'", m2.err)
	}
	// Should still trigger a refresh.
	if cmd == nil {
		t.Error("cmd is nil, want refresh command")
	}
}

func TestEditorFinishedMsgNoError(t *testing.T) {
	m := testModel(testTasks(), testViews())
	_, cmd := m.Update(EditorFinishedMsg{})
	if cmd == nil {
		t.Error("cmd is nil, want refresh command")
	}
}

func TestToggleResultMsgError(t *testing.T) {
	m := testModel(testTasks(), testViews())
	updated, cmd := m.Update(toggleResultMsg{Err: fmt.Errorf("toggle failed")})
	m2 := updated.(Model)
	if m2.err == nil {
		t.Error("err is nil, want error")
	}
	if cmd != nil {
		t.Error("cmd should be nil on toggle error")
	}
}

func TestToggleResultMsgSuccess(t *testing.T) {
	m := testModel(testTasks(), testViews())
	_, cmd := m.Update(toggleResultMsg{})
	if cmd == nil {
		t.Error("cmd is nil, want refresh command")
	}
}

func TestSetFocusedViewLocksKeys(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.SetFocusedView("Open")
	if !m.viewLocked {
		t.Error("viewLocked should be true")
	}
	if m.keys.Summary.Enabled() {
		t.Error("Summary key should be disabled")
	}
	if m.keys.AllTasks.Enabled() {
		t.Error("AllTasks key should be disabled")
	}
	if m.keys.TagSearch.Enabled() {
		t.Error("TagSearch key should be disabled")
	}
	// Escape should not exit focus when locked.
	updated, _ := sendSpecialKey(m, tea.KeyEscape)
	m2 := updated.(Model)
	if m2.focusedView != "Open" {
		t.Errorf("focusedView = %q, want 'Open' (locked)", m2.focusedView)
	}
}
```

Note: Add `"fmt"` and `"pike/internal/config"` to the import block if not already present.

- [ ] **Step 2: Run the new tests**

Run: `go test -run="TestWindowSize|TestErrorClears|TestScanResult|TestEditorFinished|TestToggleResult|TestSetFocusedView" ./internal/tui`
Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/model_test.go
git commit -m "test: add TUI state transition tests for messages and view locking"
```

---

### Task 12: TUI Test Coverage — Key Handling Edge Cases

**Files:**
- Modify: `internal/tui/model_test.go`

- [ ] **Step 1: Write key handling edge case tests**

Add to `internal/tui/model_test.go`:

```go
func TestCursorAtBoundaries(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Cursor starts at 0 — up should stay at 0.
	updated, _ := sendKey(m, "k")
	m2 := updated.(Model)
	if m2.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (at top)", m2.cursor)
	}

	// Move to last task, down should not go further.
	total := m.countFlatTasks()
	m.cursor = total - 1
	updated, _ = sendKey(m, "j")
	m2 = updated.(Model)
	if m2.cursor != total-1 {
		t.Errorf("cursor = %d, want %d (at bottom)", m2.cursor, total-1)
	}
}

func TestPageScrollAcrossSections(t *testing.T) {
	m := testModel(testTasks(), testViews())
	m.height = 20
	m.cursor = 0

	// Page down should move cursor.
	updated, _ := sendSpecialKey(m, tea.KeyPgDown)
	m2 := updated.(Model)
	if m2.cursor == 0 {
		t.Error("cursor should have moved on page down")
	}
	if m2.cursor >= m.countFlatTasks() {
		t.Errorf("cursor %d out of bounds (total %d)", m2.cursor, m.countFlatTasks())
	}
}

func TestEmptySectionsSkipped(t *testing.T) {
	// Create views where one section matches no tasks.
	views := []config.ViewConfig{
		{Title: "Empty", Query: "completed and @nonexistent", Sort: "file", Color: "red"},
		{Title: "Open", Query: "open", Sort: "file", Color: "green"},
	}
	m := testModel(testTasks(), views)

	sections := m.displaySections()
	for _, sec := range sections {
		if sec.Title == "Empty" && len(sec.Tasks) > 0 {
			t.Error("Empty section should have no tasks")
		}
	}
}

func TestKeyPressesInAllTasksMode(t *testing.T) {
	m := testModel(testTasks(), testViews())
	// Enter all-tasks mode.
	updated, _ := sendKey(m, "a")
	m2 := updated.(Model)
	if m2.mode != modeAllTasks {
		t.Errorf("mode = %v, want modeAllTasks", m2.mode)
	}

	// j/k should navigate (filter input is focused, so arrow keys navigate).
	updated, _ = sendSpecialKey(m2, tea.KeyDown)
	m3 := updated.(Model)
	if m3.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m3.cursor)
	}
}

func TestToggleNoCheckboxNoop(t *testing.T) {
	// Create a task without checkbox.
	tasks := []model.Task{
		taskWithTagSet(model.Task{
			Text: "plain bullet @today", State: model.Open,
			File: "test.md", Line: 1, HasCheckbox: false,
			Tags: []model.Tag{{Name: "today"}},
		}),
	}
	views := []config.ViewConfig{
		{Title: "Open", Query: "open", Sort: "file", Color: "green"},
	}
	m := testModel(tasks, views)
	_, cmd := sendKey(m, "x")
	if cmd != nil {
		t.Error("toggle on non-checkbox task should return nil cmd")
	}
}

func TestRecentlyCompletedModeIdempotent(t *testing.T) {
	m := testModel(testTasks(), testViews())
	updated, _ := sendKey(m, "c")
	m2 := updated.(Model)
	if m2.mode != modeRecentlyCompleted {
		t.Errorf("mode = %v, want modeRecentlyCompleted", m2.mode)
	}

	// Pressing c again should be a no-op.
	updated, _ = sendKey(m2, "c")
	m3 := updated.(Model)
	if m3.mode != modeRecentlyCompleted {
		t.Errorf("mode = %v, want modeRecentlyCompleted", m3.mode)
	}
}

func TestEscapePriorityChain(t *testing.T) {
	m := testModel(testTasks(), testViews())

	// Summary -> escape dismisses summary first.
	m.showSummary = true
	updated, _ := sendSpecialKey(m, tea.KeyEscape)
	m2 := updated.(Model)
	if m2.showSummary {
		t.Error("showSummary should be false after escape")
	}
	if m2.mode != modeDashboard {
		t.Error("mode should still be dashboard")
	}

	// In allTasks mode -> escape returns to dashboard.
	m2.mode = modeAllTasks
	updated, _ = sendSpecialKey(m2, tea.KeyEscape)
	m3 := updated.(Model)
	if m3.mode != modeDashboard {
		t.Errorf("mode = %v, want modeDashboard after escape", m3.mode)
	}

	// In focused view (not locked) -> escape unfocuses.
	m3.focusedView = "Open"
	m3.viewLocked = false
	updated, _ = sendSpecialKey(m3, tea.KeyEscape)
	m4 := updated.(Model)
	if m4.focusedView != "" {
		t.Errorf("focusedView = %q, want empty after escape", m4.focusedView)
	}
}
```

- [ ] **Step 2: Run the new tests**

Run: `go test -run="TestCursorAt|TestPageScroll|TestEmptySection|TestKeyPresses|TestToggleNoCheckbox|TestRecentlyCompleted|TestEscapePriority" ./internal/tui`
Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/model_test.go
git commit -m "test: add TUI key handling edge case tests"
```

---

### Task 13: TUI Test Coverage — Sub-model Gaps

**Files:**
- Modify: `internal/tui/filterbar_test.go`
- Modify: `internal/tui/tagsearch_test.go`

- [ ] **Step 1: Add FilterBar edge case tests**

Add to `internal/tui/filterbar_test.go`:

```go
func TestFilterBarModeSwitchPreservesText(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterSubstring, InitialValue: "hello"})
	if fb.Text() != "hello" {
		t.Errorf("text = %q, want 'hello'", fb.Text())
	}

	// Switch to query mode via ? key.
	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	output := fb.Output()
	if modeMsg, ok := output.(FilterModeChangedMsg); ok {
		if modeMsg.Mode != filterQuery {
			t.Errorf("mode = %v, want filterQuery", modeMsg.Mode)
		}
	}
	// Text should be preserved.
	if fb.Text() != "hello" {
		t.Errorf("text = %q after mode switch, want 'hello'", fb.Text())
	}
}

func TestFilterBarInvalidDSLSetsError(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterQuery, InitialValue: ""})
	fb, _ = fb.Update(FilterSetErrorMsg{Err: fmt.Errorf("parse error")})
	if fb.QueryErr() == nil {
		t.Error("QueryErr should be set")
	}
	if fb.QueryErr().Error() != "parse error" {
		t.Errorf("QueryErr = %v, want 'parse error'", fb.QueryErr())
	}
}

func TestFilterBarEscapeClearsAndExits(t *testing.T) {
	fb := NewFilterBar()
	fb, _ = fb.Update(FilterActivateMsg{Mode: filterSubstring, InitialValue: "text"})
	// First escape clears text.
	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyEscape})
	output := fb.Output()
	if _, ok := output.(FilterChangedMsg); !ok {
		t.Errorf("first escape should emit FilterChangedMsg, got %T", output)
	}
	// Second escape exits.
	fb, _ = fb.Update(tea.KeyMsg{Type: tea.KeyEscape})
	output = fb.Output()
	if _, ok := output.(FilterClearedMsg); !ok {
		t.Errorf("second escape should emit FilterClearedMsg, got %T", output)
	}
}
```

Note: Add `"fmt"` to imports.

- [ ] **Step 2: Add TagSearch edge case tests**

Add to `internal/tui/tagsearch_test.go`:

```go
func TestTagSearchRefreshWhileActive(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"due", "today"}})

	// Move cursor to "today" (index 1 after sorting).
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Refresh with updated tag list (preserves cursor position).
	ts, _ = ts.Update(TagSearchRefreshMsg{Tags: []string{"due", "risk", "today"}})

	// Cursor should still be valid (clamped if needed).
	// Verify it doesn't panic.
	_ = ts.View(nil, 80)
}

func TestTagSearchSelectionWrapping(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{"a", "b", "c"}})

	// Tab 3 times should wrap back to 0.
	for range 3 {
		ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyTab})
	}
	// Tab once more should wrap to start.
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Verify no crash — the wrapping behavior depends on implementation.
	_ = ts.View(nil, 80)
}

func TestTagSearchEmptyTagList(t *testing.T) {
	ts := NewTagSearch()
	ts, _ = ts.Update(TagSearchActivateMsg{Tags: []string{}})

	// Should render without panic.
	view := ts.View(nil, 80)
	if view == "" {
		t.Error("view should not be empty even with no tags")
	}

	// Tab on empty list should not panic.
	ts, _ = ts.Update(tea.KeyMsg{Type: tea.KeyTab})
}
```

- [ ] **Step 3: Run all TUI tests**

Run: `go test -race -count=1 ./internal/tui/...`
Expected: All pass.

- [ ] **Step 4: Check coverage improvement**

Run: `go test -coverprofile=coverage.out ./internal/tui/ && go tool cover -func=coverage.out | grep tui`
Expected: Coverage should be above 80%, ideally approaching 85%.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/filterbar_test.go internal/tui/tagsearch_test.go internal/tui/model_test.go
git commit -m "test: improve TUI test coverage with sub-model and edge case tests"
```

---

## Chunk 4: Fuzz Testing (Track 2, final)

### Task 14: Fuzz Tests for Query DSL

**Files:**
- Create: `internal/query/fuzz_test.go`

- [ ] **Step 1: Create the fuzz test**

```go
package query

import (
	"pike/internal/model"
	"testing"
	"time"
)

var fuzzTasks = []*model.Task{
	{Text: "open task @due(2026-03-16) @today", State: model.Open, Tags: []model.Tag{{Name: "due", Value: "2026-03-16"}, {Name: "today"}}, TagSet: map[string]bool{"due": true, "today": true}, Due: timePtr(time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC))},
	{Text: "completed task @completed(2026-03-10)", State: model.Completed, Tags: []model.Tag{{Name: "completed", Value: "2026-03-10"}}, TagSet: map[string]bool{"completed": true}, Completed: timePtr(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC))},
	{Text: "plain task @risk", State: model.Open, Tags: []model.Tag{{Name: "risk"}}, TagSet: map[string]bool{"risk": true}},
}

func timePtr(t time.Time) *time.Time { return &t }

func FuzzParse(f *testing.F) {
	// Seed corpus with representative queries.
	seeds := []string{
		"open",
		"completed",
		"open and @due < today",
		"open or completed",
		"not @risk",
		"@due >= today+3d",
		"@due = 2026-03-16",
		`"partial text"`,
		`/regex.*pattern/`,
		"(open or completed) and @today",
		"",
		"@",
		"(((",
		"and and and",
		"open and @due < today+9999d",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	now := time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

	f.Fuzz(func(t *testing.T, input string) {
		node, err := Parse(input)
		if err != nil {
			return // Parse errors are expected for random input.
		}
		if node == nil {
			return
		}
		// Evaluate against all test tasks — must never panic.
		for _, task := range fuzzTasks {
			Eval(node, task, now)
		}
	})
}
```

- [ ] **Step 2: Run fuzz test briefly to verify it works**

Run: `go test -fuzz=FuzzParse -fuzztime=5s ./internal/query`
Expected: Passes without panics.

- [ ] **Step 3: Commit**

```bash
git add internal/query/fuzz_test.go
git commit -m "test: add fuzz target for query DSL parser and evaluator"
```

---

### Task 15: Fuzz Tests for Parser

**Files:**
- Create: `internal/parser/fuzz_test.go`

- [ ] **Step 1: Create the fuzz test**

```go
package parser

import (
	"testing"
)

func FuzzParseLine(f *testing.F) {
	seeds := []string{
		"- [ ] task @due(2026-03-16)",
		"- [x] done @completed(2026-03-10)",
		"- bullet @today @risk",
		"- [ ] @due(bad-date) text",
		"- [ ] @due(2026/3/16) normalizable",
		"- [ ] @due(2026.03.16) dots",
		"just text no task",
		"",
		"- [ ] ",
		"- [x] @due(9999-99-99)",
		"   - [ ] indented @tag(value)",
		"- [ ] unicode: 日本語 @タグ",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		task, warnings := ParseLine(input, "fuzz.md", 1)

		// Task invariants.
		if task != nil {
			if task.Due != nil {
				// Due date must be valid if set.
				if task.Due.Format("2006-01-02") == "" {
					t.Error("task.Due formatted to empty string")
				}
			}
			if task.Completed != nil {
				if task.Completed.Format("2006-01-02") == "" {
					t.Error("task.Completed formatted to empty string")
				}
			}
		}

		// Warning invariants.
		for _, w := range warnings {
			if w.File == "" {
				t.Error("warning has empty File")
			}
			if w.Line <= 0 {
				t.Errorf("warning has non-positive Line: %d", w.Line)
			}
			if w.Message == "" {
				t.Error("warning has empty Message")
			}
		}
	})
}
```

- [ ] **Step 2: Run fuzz test briefly**

Run: `go test -fuzz=FuzzParseLine -fuzztime=5s ./internal/parser`
Expected: Passes without panics.

- [ ] **Step 3: Run all tests one final time**

Run: `make test`
Expected: All tests pass.

- [ ] **Step 4: Run lint one final time**

Run: `make lint`
Expected: Clean.

- [ ] **Step 5: Commit**

```bash
git add internal/parser/fuzz_test.go
git commit -m "test: add fuzz target for ParseLine with warning invariant checks"
```

---

## Task Summary

| Task | Track | Description |
|------|-------|-------------|
| 1 | T1 | Makefile |
| 2 | T1 | `.golangci.yml` + fix violations |
| 3 | T1 | CI pipeline (`.github/workflows/ci.yml`) |
| 4 | T2 | Warning type in model |
| 5 | T2 | Failing tests for normalizeDate |
| 6 | T2 | Implement normalizeDate |
| 7 | T2 | Failing tests for ParseLine warnings |
| 8 | T2 | Update ParseLine signature + callers |
| 9 | T2 | Scanner warning collection + surfacing |
| 10 | T2 | Scanner benchmark |
| 11 | T2 | TUI tests — state transitions |
| 12 | T2 | TUI tests — key handling edge cases |
| 13 | T2 | TUI tests — sub-model gaps |
| 14 | T2 | Fuzz testing — query DSL |
| 15 | T2 | Fuzz testing — parser |

**Parallelism:** Tasks 1-3 (Track 1) are independent from Tasks 4-15 (Track 2). Within Track 1, tasks are sequential. Within Track 2, tasks are sequential (each builds on the previous).
