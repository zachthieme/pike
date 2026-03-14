# Pike Code Review Refactor: Style Unification, Testability, and Golden Files

## Problem

Tag coloring logic is duplicated between `internal/render/render.go` (stdout, raw ANSI) and `internal/tui/sections.go` (TUI, lipgloss). These implementations have drifted apart, producing a string of bug fixes (commits 955d31f, 6cfd271, 531141c) for edge cases around valued tags like `@delegated(name)`. The `tagToken` helper is also duplicated verbatim. The TUI's `model.go` is 981 lines mixing state management, key handling, rendering, and filtering. There are no golden files to catch regressions across the full pipeline.

## Goals

1. **Single coloring code path** — one implementation for tag coloring and link prettification, used by both stdout and TUI rendering
2. **Break up `tui/model.go`** — split into focused files by responsibility
3. **Golden file test suite** — sample markdown inputs with expected outputs at every pipeline stage, including ANSI color validation
4. **Centralize valued-tag handling** — move the three-part render for `@tag(value)` into `ColorizeTags` so it exists in one place

## Non-goals

- Changes to the query DSL, parser, scanner, config, sort, or editor modules
- New features or behavior changes
- New external dependencies

---

## Design

### 1. New `internal/style` module

A single module responsible for text styling: tag coloring, link prettification, and ANSI helpers. This module has **no dependency on lipgloss** — it only knows about raw ANSI codes and the `StyleFunc` abstraction. Lipgloss-based styling is constructed by callers in the `tui` package.

#### API

```go
package style

import "pike/internal/model"

// StyleFunc applies a color to a string. Callers provide the implementation
// (raw ANSI for stdout, lipgloss for TUI).
type StyleFunc func(text string, color string) string

// ColorizeTags replaces tag tokens in text with colored versions.
// Handles both @tag and @tag(value). For valued tags, the prefix (@name(),
// the value, and the closing paren are styled separately so that ANSI resets
// from link styling inside the value don't prevent the closing paren from
// being colored (see "Valued tag handling" below).
// Tags are replaced longest-first to prevent @foo matching before @foo(bar).
// Only the first occurrence of each token is replaced (strings.Replace count=1).
// Deduplication uses the full token string as the key, so @due(2026-03-15)
// and @due(2026-04-01) are treated as distinct tokens and both get colored.
func ColorizeTags(text string, tags []model.Tag, tagColors map[string]string, sf StyleFunc) string

// TagToken returns the string representation of a tag as it appears in task text.
// Bare tags: "@name", parameterized tags: "@name(value)".
func TagToken(tag model.Tag) string

// PrettifyText cleans up markdown link syntax for plain display (no styling).
func PrettifyText(s string) string

// PrettifyLinks cleans up markdown link syntax and applies renderLink to
// the display text of each link. The caller controls how links look
// (bold, colored, etc.) by providing the renderLink function.
func PrettifyLinks(s string, renderLink func(string) string) string

// StripANSI removes all ANSI escape sequences from a string.
// Primarily used by golden file tests to produce human-readable output.
func StripANSI(s string) string

// ANSIStyleFunc returns a StyleFunc that wraps text in raw ANSI color codes.
// Supports named colors (red, green, etc.) and hex (#RRGGBB).
// Returns text unchanged for unrecognized color values.
func ANSIStyleFunc() StyleFunc

// ShortenURL extracts a readable name from a URL.
func ShortenURL(raw string) string
```

#### What moves here

| From | What | Notes |
|------|------|-------|
| `render.go` | `colorCode()`, `namedColors`, `ansiReset`, `tagToken()` | Becomes `ANSIStyleFunc()` internals and exported `TagToken()` |
| `tui/sections.go` | Tag coloring logic (lines 70-113), `tagToken()` | Deleted, callers use `ColorizeTags()` |
| `tui/prettify.go` | `prettifyText()`, `prettifyAndStyleLinks()`, `prettifySlug()`, `shortenURL()`, `isNumeric()`, `truncate()`, regexes | Moves to `style`. `prettifyAndStyleLinks` becomes `PrettifyLinks` with `func(string) string` parameter instead of `lipgloss.Style` |

#### What stays in `tui`

- `styles.go` — `resolveColor()`, `namedColorMap`, `TagStyle()`, `LinkStyle()`, `SectionHeaderStyle()`, etc. These are lipgloss-specific and used by TUI rendering beyond just tag coloring.
- The `lipglossStyleFunc` closure is defined in `tui/sections.go` (the caller), not in `style`. This avoids a circular dependency (`tui` imports `style`, not the reverse).
- `LinkStyle()` stays in `tui/styles.go`. The TUI caller wraps it: `style.PrettifyLinks(text, func(s string) string { return LinkStyle(linkColor).Render(s) })`. This preserves the bold+color behavior without `style` needing to know about lipgloss or boldness.

#### `StyleFunc` design decision

The stdout path and TUI path produce different escape sequences (raw ANSI vs lipgloss). Rather than pick one, `ColorizeTags` takes a `StyleFunc` — a function `(text, color) -> styled text`. This keeps the algorithm in one place while letting each caller control output format.

The TUI provides a lipgloss-based `StyleFunc` defined in `tui/sections.go`:
```go
// lipglossStyleFunc applies foreground color via lipgloss.
// Defined in tui package to avoid style -> tui circular dependency.
func lipglossStyleFunc(text string, color string) string {
    return lipgloss.NewStyle().Foreground(resolveColor(color)).Render(text)
}
```

The stdout path provides a raw ANSI `StyleFunc` via `style.ANSIStyleFunc()`.

#### Valued tag handling

The three-part render for `@tag(value)` is **not eliminated** — it is **centralized** inside `ColorizeTags`. When a tag has a non-empty value, `ColorizeTags` styles the prefix `@name(`, the value, and the closing `)` as three separate calls to `StyleFunc`. This is necessary because link prettification (which runs after tag coloring) can inject ANSI resets inside the value (e.g., `@delegated([[zach-thieme]])` → the wiki-link gets bold+color with a reset at the end). Without the three-part split, that reset would kill the tag color before the closing paren.

```go
// Inside ColorizeTags:
if tag.Value != "" {
    styled = sf("@"+tag.Name+"(", color) + sf(tag.Value, color) + sf(")", color)
} else {
    styled = sf(token, color)
}
```

The win is not eliminating this logic but having it in **one place** instead of two divergent implementations.

### 2. Breaking up `tui/model.go`

Current: 981 lines in one file.

| New file | Content |
|----------|---------|
| `model.go` | `Model` struct, `NewModel()`, `Init()`, `Update()`, `SetFocusedView()` |
| `keys.go` | `handleKey()`, `handleTagSearchKey()`, `openEditor()` |
| `views.go` | `View()`, `viewDashboard()`, `viewFocused()`, `viewSummary()`, `viewAllTasks()`, `viewTagSearch()`, `renderSections()`, `truncateView()`, `truncateViewPinTop()`, `scrollWindow()` |
| `tasks.go` | `rebuildSections()`, `applyHiddenFilter()`, `matchesFilter()`, `flatTasks()`, `clampCursor()`, `jumpToNextSection()`, `jumpToPrevSection()`, `displaySections()`, `visibleSections()`, `buildTagList()`, `filteredTags()`, `hiddenCountFor()`, counting functions, `clearFilter()`, `nowFunc()` |

All files remain in `package tui`. No API changes — this is purely a file-level split. Existing `model_test.go` continues to work unchanged since it tests exported and unexported functions within the same package.

### 3. Updating callers

#### `internal/render/render.go`

```go
// Before: ~50 lines of inline coloring logic
// After:
func FormatTask(task model.Task, tagColors map[string]string, noColor bool) string {
    text := task.Text
    if !noColor && tagColors != nil {
        text = style.ColorizeTags(text, task.Tags, tagColors, style.ANSIStyleFunc())
    }
    // ... format string unchanged
}
```

`colorCode()`, `namedColors`, `ansiReset`, `tagToken()`, and the inline replacement loop are all deleted from `render.go`.

`FormatSummary()` keeps its inline `"\033[31m"` for the overdue highlight — it's a single hardcoded color, not tag coloring, so the `style` abstraction adds no value there.

#### `internal/tui/sections.go`

```go
func formatTaskLine(task model.Task, tagColors map[string]string, linkColor string, selected bool) string {
    text := task.Text
    if tagColors != nil {
        text = style.ColorizeTags(text, task.Tags, tagColors, lipglossStyleFunc)
    }
    if linkColor != "" {
        text = style.PrettifyLinks(text, func(s string) string {
            return LinkStyle(linkColor).Render(s)
        })
    } else {
        text = style.PrettifyText(text)
    }
    // ... marker logic unchanged
}
```

The `tagToken()` function and all tag replacement logic are deleted from this file.

#### `internal/tui/prettify.go`

Deleted entirely. All content moves to `internal/style/`. The `prettify_test.go` tests move to `internal/style/style_test.go` — function references change from unexported `prettifyText` to exported `style.PrettifyText` (or just `PrettifyText` within the `style` package tests).

#### `noColor` handling

The `noColor` flag is handled by callers, not by `style`. When `noColor` is true, the caller simply skips the `ColorizeTags` call. This keeps the `style` API clean — it always styles, and the caller decides whether to invoke it.

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

#### Test placement

- **`internal/style/style_test.go`**: Unit tests for `ColorizeTags`, `PrettifyText`, `PrettifyLinks`, `StripANSI`, `TagToken`, `ShortenURL`. These are pure function tests with no cross-package dependencies.
- **`internal/style/golden_test.go`**: Golden file tests for the style module (tag coloring + link prettification with both ANSI and lipgloss style funcs).
- **`golden_test.go` (top-level package)**: Full-pipeline golden tests that import `config`, `parser`, `scanner`, `filter`, `render`, and `style`. Tests the complete flow: load config → scan notes → parse → filter → render → compare against golden files. This avoids circular dependencies since it's a separate test package.

#### Test runner

```go
// golden_test.go (top-level)
package pike_test

var update = flag.Bool("update", false, "update golden files")

func TestGoldenPipeline(t *testing.T) {
    // Load fixed test config, scan testdata/notes/, parse, filter, render
    // Compare against testdata/golden/ files
    // With -update, overwrite golden files with actual output
}
```

#### Color validation approach

- **`styled/ansi/`**: Raw ANSI escape sequences from `ANSIStyleFunc`. Byte-for-byte comparison catches any color regression.
- **`styled/plain/`**: Same output run through `StripANSI()`. Human-readable, validates text transformation correctness independent of coloring.
- **`styled/lipgloss/`**: Lipgloss-rendered output for TUI path. Also byte-for-byte.
- **`-update` flag**: `go test ./... -update` regenerates all golden files. Review the git diff to validate changes.

#### Lipgloss determinism in tests

Lipgloss uses `muesli/termenv` which queries the terminal at runtime. To ensure golden files are deterministic across environments (local dev, CI, headless), golden tests must force a fixed color profile:

```go
import "github.com/muesli/termenv"

func init() {
    lipgloss.SetColorProfile(termenv.TrueColor)
}
```

This ensures the same ANSI sequences regardless of the actual terminal.

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
| `internal/style/style.go` | **New** — tag coloring, link prettification, ANSI helpers. No lipgloss dependency. |
| `internal/style/style_test.go` | **New** — unit tests + tests migrated from `prettify_test.go` |
| `internal/style/golden_test.go` | **New** — golden file tests for style module |
| `golden_test.go` | **New** — full-pipeline golden tests (top-level test package) |
| `internal/render/render.go` | **Simplified** — delegates tag coloring to `style`, drops `colorCode`, `namedColors`, `tagToken`, replacement loop. `FormatSummary` unchanged. |
| `internal/render/render_test.go` | **Updated** — ANSI assertions change because valued tags now produce three spans instead of one (visually identical, different bytes) |
| `internal/tui/model.go` | **Split** into `model.go`, `keys.go`, `views.go`, `tasks.go` |
| `internal/tui/sections.go` | **Simplified** — delegates to `style`, adds `lipglossStyleFunc` closure, drops tag coloring logic and `tagToken` |
| `internal/tui/prettify.go` | **Deleted** — moved to `style` |
| `internal/tui/prettify_test.go` | **Deleted** — migrated to `style/style_test.go` |
| `internal/tui/styles.go` | **Updated** — `LinkStyle()` moves here from deleted `prettify.go` |
| `testdata/` | **New** — golden file suite (notes, config, expected outputs) |

### 6. Risk assessment

| Risk | Mitigation |
|------|------------|
| Lipgloss output varies across terminal types | Golden tests force `termenv.TrueColor` profile via `lipgloss.SetColorProfile()` |
| Lipgloss version changes output format | Pin version in go.mod (already done); golden file diffs make changes visible |
| Golden file churn on unrelated changes | Golden files only cover style/rendering, not layout. Structured by pipeline stage so changes are localized. |
| `StyleFunc` adds indirection | Single function type, not an interface. Concrete implementations are one-liners. |
| Three-part valued tag render still complex | Centralized in one function instead of two — complexity contained, covered by golden files |
| `style` package accidentally imports `tui` | `style` has no lipgloss dependency by design. Callers construct `StyleFunc` closures in their own packages. |
