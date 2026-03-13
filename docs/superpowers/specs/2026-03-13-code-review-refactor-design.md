# Pike Code Review Refactor: Style Unification, Testability, and Golden Files

## Problem

Tag coloring logic is duplicated between `internal/render/render.go` (stdout, raw ANSI) and `internal/tui/sections.go` (TUI, lipgloss). These implementations have drifted apart, producing a string of bug fixes (commits 955d31f, 6cfd271, 531141c) for edge cases around valued tags like `@delegated(name)`. The `tagToken` helper is also duplicated verbatim. The TUI's `model.go` is 981 lines mixing state management, key handling, rendering, and filtering. There are no golden files to catch regressions across the full pipeline.

## Goals

1. **Single coloring code path** — one implementation for tag coloring and link prettification, used by both stdout and TUI rendering
2. **Break up `tui/model.go`** — split into focused files by responsibility
3. **Golden file test suite** — sample markdown inputs with expected outputs at every pipeline stage, including ANSI color validation
4. **Simplify valued-tag handling** — eliminate the three-part render workaround for `@tag(value)`

## Non-goals

- Changes to the query DSL, parser, scanner, config, sort, or editor modules
- New features or behavior changes
- New external dependencies

---

## Design

### 1. New `internal/style` module

A single module responsible for all text styling: tag coloring, link prettification, and ANSI output.

#### API

```go
package style

// StyleFunc applies a color to a string. Callers provide the implementation
// (raw ANSI for stdout, lipgloss for TUI).
type StyleFunc func(text string, color string) string

// ColorizeTags replaces tag tokens in text with colored versions.
// Handles both @tag and @tag(value) — the color wraps the full token.
// Tags are replaced longest-first to prevent @foo matching before @foo(bar).
func ColorizeTags(text string, tags []model.Tag, tagColors map[string]string, sf StyleFunc) string

// TagToken returns the string representation of a tag as it appears in task text.
// Bare tags: "@name", parameterized tags: "@name(value)".
func TagToken(tag model.Tag) string

// PrettifyText cleans up markdown link syntax for plain display.
func PrettifyText(s string) string

// PrettifyLinks cleans up markdown link syntax and applies a style to link text.
func PrettifyLinks(s string, sf StyleFunc, linkColor string) string

// StripANSI removes all ANSI escape sequences from a string.
func StripANSI(s string) string

// ANSIStyleFunc returns a StyleFunc that wraps text in raw ANSI color codes.
// Supports named colors (red, green, etc.) and hex (#RRGGBB).
func ANSIStyleFunc() StyleFunc

// ShortenURL extracts a readable name from a URL (exported for testing).
func ShortenURL(raw string) string
```

#### What moves here

| From | What | Notes |
|------|------|-------|
| `render.go` | `colorCode()`, `namedColors`, `ansiReset`, `tagToken()` | Becomes `ANSIStyleFunc()` and `TagToken()` |
| `tui/sections.go` | Tag coloring logic (lines 70-113), `tagToken()` | Deleted, callers use `ColorizeTags()` |
| `tui/prettify.go` | `prettifyText()`, `prettifyAndStyleLinks()`, `prettifySlug()`, `shortenURL()`, `isNumeric()`, `truncate()`, regexes | Moves entirely to `style` |

#### Key design decision: `StyleFunc`

The stdout path and TUI path produce different escape sequences (raw ANSI vs lipgloss). Rather than pick one, `ColorizeTags` takes a `StyleFunc` — a function `(text, color) -> styled text`. This keeps the algorithm in one place while letting each caller control output format.

The TUI provides a lipgloss-based `StyleFunc`:
```go
func lipglossStyleFunc(text string, color string) string {
    return lipgloss.NewStyle().Foreground(resolveColor(color)).Render(text)
}
```

The stdout path provides a raw ANSI `StyleFunc` via `style.ANSIStyleFunc()`.

#### Valued tag simplification

Currently the TUI splits `@tag(value)` into three separately-styled parts to work around ANSI resets from link styling inside values. With the new approach:

1. Tags are colorized first (full token wrapped in color)
2. Links are prettified second

Since `ColorizeTags` wraps the **entire** `@tag(value)` in a single color span, there's no split, no paren coloring issue, and the existing order-of-operations (colorize tags, then prettify links) continues to work.

### 2. Breaking up `tui/model.go`

Current: 981 lines in one file.

| New file | Content | Source lines (approx) |
|----------|---------|----------------------|
| `model.go` | `Model` struct, `NewModel()`, `Init()`, `Update()`, `SetFocusedView()` | ~100 |
| `keys.go` | `handleKey()`, `handleTagSearchKey()`, `openEditor()` | ~250 |
| `views.go` | `View()`, `viewDashboard()`, `viewFocused()`, `viewSummary()`, `viewAllTasks()`, `viewTagSearch()`, `renderSections()`, `truncateView()`, `truncateViewPinTop()`, `scrollWindow()` | ~300 |
| `tasks.go` | `rebuildSections()`, `applyHiddenFilter()`, `matchesFilter()`, `flatTasks()`, `clampCursor()`, `jumpToNextSection()`, `jumpToPrevSection()`, `displaySections()`, `visibleSections()`, `buildTagList()`, `filteredTags()`, `hiddenCountFor()`, counting functions, `clearFilter()`, `nowFunc()` | ~330 |

All files remain in `package tui`. No API changes — this is purely a file-level split. Existing tests continue to work unchanged.

### 3. Updating callers

#### `internal/render/render.go`

```go
// Before:
text = strings.Replace(text, r.token, r.colored, 1)  // inline coloring logic

// After:
text = style.ColorizeTags(text, task.Tags, tagColors, style.ANSIStyleFunc())
```

The `FormatTask` function shrinks from ~50 lines to ~15. `colorCode()`, `namedColors`, `tagToken()`, and the inline replacement loop are all deleted.

#### `internal/tui/sections.go`

```go
// Before:
// 45 lines of inline tag coloring + link prettification

// After:
func formatTaskLine(task model.Task, tagColors map[string]string, linkColor string, selected bool) string {
    text := task.Text
    if tagColors != nil {
        text = style.ColorizeTags(text, task.Tags, tagColors, lipglossStyleFunc)
    }
    if linkColor != "" {
        text = style.PrettifyLinks(text, lipglossStyleFunc, linkColor)
    } else {
        text = style.PrettifyText(text)
    }
    // ... marker logic unchanged
}
```

The `tagToken()` function and all tag replacement logic are deleted from this file.

#### `internal/tui/prettify.go`

Deleted entirely. All content moves to `internal/style/`.

### 4. Golden file test suite

#### Directory structure

```
testdata/
  config.yaml                    # Fixed config with known tag colors, views
  notes/
    basic.md                     # Simple checkbox tasks
    tags.md                      # Various tag formats: @tag, @tag(value), @tag([[link]])
    links.md                     # Wiki-links, markdown links, bare URLs
    mixed.md                     # All features combined
    edge-cases.md                # Empty tags, nested parens, unicode, malformed input
  golden/
    parsed/
      basic.json                 # Expected []Task after parsing
      tags.json
      links.json
      mixed.json
      edge-cases.json
    styled/
      ansi/
        basic.txt                # Expected stdout output with ANSI codes
        tags.txt
        mixed.txt
      plain/
        basic.txt                # Expected stdout output, ANSI stripped
        tags.txt
        mixed.txt
      lipgloss/
        basic.txt                # Expected TUI-style output
        tags.txt
        mixed.txt
    query/
      open-and-today.json        # Expected filtered task lists
      due-before-today.json
      overdue.json
```

#### Test runner

```go
// internal/style/golden_test.go

var update = flag.Bool("update", false, "update golden files")

func TestGoldenStyled(t *testing.T) {
    cfg := loadTestConfig(t)
    notes := loadTestNotes(t)
    tasks := parseAllTasks(t, notes)

    for _, tc := range styledTestCases {
        t.Run(tc.name, func(t *testing.T) {
            var actual string
            for _, task := range tasks {
                line := style.ColorizeTags(task.Text, task.Tags, cfg.TagColors, tc.styleFunc)
                actual += line + "\n"
            }
            goldenPath := filepath.Join("testdata", "golden", "styled", tc.dir, tc.file)
            if *update {
                os.WriteFile(goldenPath, []byte(actual), 0644)
                return
            }
            expected, _ := os.ReadFile(goldenPath)
            if actual != string(expected) {
                t.Errorf("mismatch for %s:\n%s", tc.name, diff(string(expected), actual))
            }
        })
    }
}
```

A parallel test runner in `internal/golden_test.go` (or similar top-level) tests the full pipeline: scan notes → parse → filter → render → compare.

#### Color validation approach

- **`styled/ansi/`**: Raw ANSI escape sequences from `ANSIStyleFunc`. Byte-for-byte comparison catches any color regression.
- **`styled/plain/`**: Same output run through `StripANSI()`. Human-readable, validates text transformation correctness independent of coloring.
- **`styled/lipgloss/`**: Lipgloss-rendered output for TUI path. Also byte-for-byte.
- **`-update` flag**: `go test ./... -update` regenerates all golden files. Review the git diff to validate changes.

#### What the golden files cover

| Feature | Golden file |
|---------|-------------|
| Checkbox parsing (`- [ ]`, `- [x]`) | `parsed/basic.json` |
| Tag extraction (`@tag`, `@tag(value)`) | `parsed/tags.json` |
| Date parsing (`@due(2026-03-15)`) | `parsed/tags.json` |
| Plain bullet parsing (`- text @tag`) | `parsed/basic.json` |
| Tag coloring (bare tags) | `styled/*/tags.txt` |
| Tag coloring (valued tags) | `styled/*/tags.txt` |
| Tag coloring (tags with links in values) | `styled/*/links.txt` |
| Link prettification (wiki, markdown, bare) | `styled/*/links.txt` |
| Query filtering | `query/*.json` |
| Mixed features | `styled/*/mixed.txt` |
| Edge cases | `parsed/edge-cases.json`, `styled/*/edge-cases.txt` |

### 5. Files changed summary

| File | Action |
|------|--------|
| `internal/style/style.go` | **New** — tag coloring, link prettification, ANSI helpers |
| `internal/style/style_test.go` | **New** — unit tests for style functions |
| `internal/style/golden_test.go` | **New** — golden file test runner |
| `internal/render/render.go` | **Simplified** — delegates to `style`, drops coloring logic |
| `internal/render/render_test.go` | **Updated** — tests still pass, may simplify |
| `internal/tui/model.go` | **Split** into `model.go`, `keys.go`, `views.go`, `tasks.go` |
| `internal/tui/sections.go` | **Simplified** — delegates to `style`, drops tag coloring |
| `internal/tui/prettify.go` | **Deleted** — moved to `style` |
| `internal/tui/prettify_test.go` | **Moved** — to `style` package |
| `internal/tui/styles.go` | **Unchanged** — lipgloss style builders stay in TUI |
| `testdata/` | **New** — golden file suite |

### 6. Risk assessment

| Risk | Mitigation |
|------|------------|
| Lipgloss output varies across terminal types | Golden tests run with a fixed color profile (e.g., `TERM=xterm-256color`) |
| Lipgloss version changes output format | Pin version in go.mod (already done) |
| Golden file churn on unrelated changes | Golden files only cover style/rendering, not layout. Structured by pipeline stage so changes are localized. |
| `StyleFunc` adds indirection | It's a single function type, not an interface. Concrete implementations are one-liners. |
