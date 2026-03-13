# Code Review Refactor Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unify duplicated tag coloring logic into a single `internal/style` module, break up the 981-line `tui/model.go`, and build a golden file test suite with ANSI color validation.

**Architecture:** Extract all text styling (tag coloring, link prettification, ANSI helpers) into `internal/style` with a `StyleFunc` callback abstraction. Both stdout (`render`) and TUI (`tui/sections`) call the same algorithm. Split `tui/model.go` into four focused files. Add golden files at `testdata/` covering parsing, styling, and query filtering.

**Tech Stack:** Go, charmbracelet/bubbletea, charmbracelet/lipgloss, muesli/termenv (already transitive dep)

**Spec:** `docs/superpowers/specs/2026-03-13-code-review-refactor-design.md`

---

## Chunk 1: `internal/style` module (tag coloring + ANSI helpers)

### Task 1: Create `internal/style` with `TagToken`, `StripANSI`, and `ANSIStyleFunc`

**Files:**
- Create: `internal/style/style.go`
- Create: `internal/style/style_test.go`

- [ ] **Step 1: Write failing tests for TagToken**

In `internal/style/style_test.go`:

```go
package style

import (
	"testing"

	"pike/internal/model"
)

func TestTagToken(t *testing.T) {
	tests := []struct {
		name string
		tag  model.Tag
		want string
	}{
		{"bare tag", model.Tag{Name: "today"}, "@today"},
		{"valued tag", model.Tag{Name: "due", Value: "2026-03-15"}, "@due(2026-03-15)"},
		{"empty value treated as bare", model.Tag{Name: "risk", Value: ""}, "@risk"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TagToken(tt.tag); got != tt.want {
				t.Errorf("TagToken() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/zach/code/pike && go test ./internal/style/...`
Expected: FAIL — package doesn't exist yet.

- [ ] **Step 3: Write minimal `style.go` with TagToken, StripANSI, ANSIStyleFunc**

In `internal/style/style.go`:

```go
package style

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"pike/internal/model"
)

// StyleFunc applies a color to a string. Callers provide the implementation
// (raw ANSI for stdout, lipgloss for TUI).
type StyleFunc func(text string, color string) string

// namedColors maps color names to ANSI escape sequences.
var namedColors = map[string]string{
	"red":     "\033[31m",
	"green":   "\033[32m",
	"yellow":  "\033[33m",
	"blue":    "\033[34m",
	"magenta": "\033[35m",
	"cyan":    "\033[36m",
	"white":   "\033[37m",
}

const ansiReset = "\033[0m"

// ansiStripRe matches ANSI escape sequences.
var ansiStripRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// TagToken returns the string representation of a tag as it appears in task text.
// Bare tags: "@name", parameterized tags: "@name(value)".
func TagToken(tag model.Tag) string {
	if tag.Value != "" {
		return "@" + tag.Name + "(" + tag.Value + ")"
	}
	return "@" + tag.Name
}

// StripANSI removes all ANSI escape sequences from a string.
func StripANSI(s string) string {
	return ansiStripRe.ReplaceAllString(s, "")
}

// colorCode returns the ANSI escape sequence for a color name or hex value.
func colorCode(color string) string {
	if code, ok := namedColors[color]; ok {
		return code
	}
	if strings.HasPrefix(color, "#") && len(color) == 7 {
		r, errR := strconv.ParseInt(color[1:3], 16, 64)
		g, errG := strconv.ParseInt(color[3:5], 16, 64)
		b, errB := strconv.ParseInt(color[5:7], 16, 64)
		if errR == nil && errG == nil && errB == nil {
			return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
		}
	}
	return ""
}

// ANSIStyleFunc returns a StyleFunc that wraps text in raw ANSI color codes.
func ANSIStyleFunc() StyleFunc {
	return func(text string, color string) string {
		code := colorCode(color)
		if code == "" {
			return text
		}
		return code + text + ansiReset
	}
}
```

- [ ] **Step 4: Write tests for StripANSI and ANSIStyleFunc, then run all tests**

Add to `internal/style/style_test.go`:

```go
func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no ANSI", "hello world", "hello world"},
		{"simple color", "\033[31mred\033[0m", "red"},
		{"24-bit color", "\033[38;2;255;87;51mhex\033[0m", "hex"},
		{"mixed", "before \033[32mgreen\033[0m after", "before green after"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripANSI(tt.input); got != tt.want {
				t.Errorf("StripANSI() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestANSIStyleFunc(t *testing.T) {
	sf := ANSIStyleFunc()

	tests := []struct {
		name  string
		text  string
		color string
		want  string
	}{
		{"named color", "@today", "green", "\033[32m@today\033[0m"},
		{"hex color", "@special", "#FF5733", "\033[38;2;255;87;51m@special\033[0m"},
		{"unknown color returns text unchanged", "@tag", "nope", "@tag"},
		{"empty color returns text unchanged", "@tag", "", "@tag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sf(tt.text, tt.color); got != tt.want {
				t.Errorf("ANSIStyleFunc() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

Run: `cd /home/zach/code/pike && go test ./internal/style/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/style/style.go internal/style/style_test.go
git commit -m "feat: add internal/style with TagToken, StripANSI, ANSIStyleFunc"
```

### Task 2: Add `ColorizeTags` to `internal/style`

**Files:**
- Modify: `internal/style/style.go`
- Modify: `internal/style/style_test.go`

- [ ] **Step 1: Write failing tests for ColorizeTags**

Add to `internal/style/style_test.go`:

```go
func TestColorizeTags(t *testing.T) {
	sf := ANSIStyleFunc()
	green := "\033[32m"
	red := "\033[31m"
	cyan := "\033[36m"
	reset := "\033[0m"

	tagColors := map[string]string{
		"today":    "green",
		"risk":     "red",
		"due":      "red",
		"_default": "cyan",
	}

	tests := []struct {
		name      string
		text      string
		tags      []model.Tag
		tagColors map[string]string
		want      string
	}{
		{
			name:      "bare tag",
			text:      "Buy groceries @today",
			tags:      []model.Tag{{Name: "today"}},
			tagColors: tagColors,
			want:      "Buy groceries " + green + "@today" + reset,
		},
		{
			name:      "valued tag uses three-part render",
			text:      "Submit report @due(2026-03-15)",
			tags:      []model.Tag{{Name: "due", Value: "2026-03-15"}},
			tagColors: tagColors,
			want:      "Submit report " + red + "@due(" + reset + red + "2026-03-15" + reset + red + ")" + reset,
		},
		{
			name:      "multiple tags",
			text:      "Deploy service @risk @today",
			tags:      []model.Tag{{Name: "risk"}, {Name: "today"}},
			tagColors: tagColors,
			want:      "Deploy service " + red + "@risk" + reset + " " + green + "@today" + reset,
		},
		{
			name:      "unknown tag uses _default",
			text:      "Research @someothertag",
			tags:      []model.Tag{{Name: "someothertag"}},
			tagColors: tagColors,
			want:      "Research " + cyan + "@someothertag" + reset,
		},
		{
			name:      "no matching color skips tag",
			text:      "Task @unknown",
			tags:      []model.Tag{{Name: "unknown"}},
			tagColors: map[string]string{},
			want:      "Task @unknown",
		},
		{
			name:      "duplicate tag tokens deduplicated",
			text:      "Task @today @today",
			tags:      []model.Tag{{Name: "today"}, {Name: "today"}},
			tagColors: tagColors,
			want:      "Task " + green + "@today" + reset + " @today",
		},
		{
			name:      "longer token replaced before shorter",
			text:      "Task @due(2026-03-15) and also @due sometime",
			tags:      []model.Tag{{Name: "due", Value: "2026-03-15"}, {Name: "due"}},
			tagColors: tagColors,
			want:      "Task " + red + "@due(" + reset + red + "2026-03-15" + reset + red + ")" + reset + " and also " + red + "@due" + reset + " sometime",
		},
		{
			name:      "nil tagColors returns text unchanged",
			text:      "Task @today",
			tags:      []model.Tag{{Name: "today"}},
			tagColors: nil,
			want:      "Task @today",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ColorizeTags(tt.text, tt.tags, tt.tagColors, sf)
			if got != tt.want {
				t.Errorf("ColorizeTags() =\n  %q\nwant:\n  %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/zach/code/pike && go test ./internal/style/... -run TestColorizeTags`
Expected: FAIL — `ColorizeTags` undefined.

- [ ] **Step 3: Implement ColorizeTags**

Add to `internal/style/style.go`:

```go
// ColorizeTags replaces tag tokens in text with colored versions.
func ColorizeTags(text string, tags []model.Tag, tagColors map[string]string, sf StyleFunc) string {
	if tagColors == nil {
		return text
	}

	type tagReplacement struct {
		token  string
		styled string
	}
	seen := make(map[string]bool)
	var replacements []tagReplacement
	for _, tag := range tags {
		token := TagToken(tag)
		if seen[token] {
			continue
		}
		seen[token] = true
		color, ok := tagColors[tag.Name]
		if !ok {
			color = tagColors["_default"]
		}
		if color == "" {
			continue
		}
		var styled string
		if tag.Value != "" {
			styled = sf("@"+tag.Name+"(", color) + sf(tag.Value, color) + sf(")", color)
		} else {
			styled = sf(token, color)
		}
		replacements = append(replacements, tagReplacement{
			token:  token,
			styled: styled,
		})
	}
	// Sort by token length descending so longer tokens are replaced first.
	sort.Slice(replacements, func(i, j int) bool {
		return len(replacements[i].token) > len(replacements[j].token)
	})
	for _, r := range replacements {
		text = strings.Replace(text, r.token, r.styled, 1)
	}
	return text
}
```

Add `"sort"` to the imports in `style.go`.

- [ ] **Step 4: Run tests**

Run: `cd /home/zach/code/pike && go test ./internal/style/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/style/style.go internal/style/style_test.go
git commit -m "feat: add ColorizeTags with three-part valued tag rendering"
```

### Task 3: Move link prettification to `internal/style`

**Files:**
- Modify: `internal/style/style.go`
- Modify: `internal/style/style_test.go`
- To be deleted later: `internal/tui/prettify.go`, `internal/tui/prettify_test.go`

- [ ] **Step 1: Write tests for PrettifyText and PrettifyLinks in style package**

Add to `internal/style/style_test.go` — these are the same tests from `tui/prettify_test.go` adapted to the exported API:

```go
func TestPrettifyText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"wiki link with display name", "talk to [[zach-thieme|Zach Thieme]] about it", "talk to Zach Thieme about it"},
		{"wiki link without display name", "talk to [[Zach Thieme]] about it", "talk to Zach Thieme about it"},
		{"wiki link slug gets prettified", "see [[jeff-roache]] for details", "see Jeff Roache for details"},
		{"markdown link shows text only", "check [the docs](https://example.com/docs/guide) first", "check the docs first"},
		{"bare URL extracts document name", "see https://example.com/docs/migration-plan for details", "see migration-plan for details"},
		{"bare URL with just host", "visit https://example.com/", "visit example.com"},
		{"bare URL with numeric path includes parent", "fix https://github.com/org/repo/pull/123", "fix pull/123"},
		{"bare URL strips .html extension", "read https://docs.example.com/guide/setup.html", "read setup"},
		{"multiple wiki links", "[[alice-bob|Alice Bob]] and [[charlie-delta|Charlie Delta]]", "Alice Bob and Charlie Delta"},
		{"mixed wiki link and bare URL", "ask [[zach-thieme|Zach Thieme]] about https://example.com/docs/auth-flow", "ask Zach Thieme about auth-flow"},
		{"no links unchanged", "just a plain task @today", "just a plain task @today"},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrettifyText(tt.input); got != tt.want {
				t.Errorf("PrettifyText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPrettifyLinks(t *testing.T) {
	// Use a simple wrapper that uppercases to verify the renderLink function is called.
	render := func(s string) string { return "[" + s + "]" }

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"wiki link styled", "see [[zach-thieme]] here", "see [Zach Thieme] here"},
		{"markdown link styled", "check [docs](https://example.com) now", "check [docs] now"},
		{"bare URL styled", "visit https://example.com/path/page", "visit [page]"},
		{"no links unchanged", "plain text", "plain text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrettifyLinks(tt.input, render); got != tt.want {
				t.Errorf("PrettifyLinks() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShortenURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com/", "example.com"},
		{"https://github.com/org/repo/pull/123", "pull/123"},
		{"https://docs.example.com/guide/setup.html", "setup"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ShortenURL(tt.input); got != tt.want {
				t.Errorf("ShortenURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/zach/code/pike && go test ./internal/style/... -run "TestPrettify|TestShortenURL"`
Expected: FAIL — functions undefined.

- [ ] **Step 3: Add PrettifyText, PrettifyLinks, ShortenURL and helpers to style.go**

Add to `internal/style/style.go`:

```go
var (
	wikiLinkRe = regexp.MustCompile(`\[\[([^\]|]+?)(?:\|([^\]]+?))?\]\]`)
	mdLinkRe   = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	bareURLRe  = regexp.MustCompile(`https?://\S+`)
)

// PrettifyText cleans up markdown link syntax for plain display.
func PrettifyText(s string) string {
	s = wikiLinkRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := wikiLinkRe.FindStringSubmatch(match)
		if sub[2] != "" {
			return sub[2]
		}
		return prettifySlug(sub[1])
	})
	s = mdLinkRe.ReplaceAllString(s, "$1")
	s = bareURLRe.ReplaceAllStringFunc(s, ShortenURL)
	return s
}

// PrettifyLinks cleans up markdown link syntax and applies renderLink to
// the display text of each link.
func PrettifyLinks(s string, renderLink func(string) string) string {
	s = wikiLinkRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := wikiLinkRe.FindStringSubmatch(match)
		var display string
		if sub[2] != "" {
			display = sub[2]
		} else {
			display = prettifySlug(sub[1])
		}
		return renderLink(display)
	})
	s = mdLinkRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := mdLinkRe.FindStringSubmatch(match)
		return renderLink(sub[1])
	})
	s = bareURLRe.ReplaceAllStringFunc(s, func(match string) string {
		return renderLink(ShortenURL(match))
	})
	return s
}

// ShortenURL extracts a readable name from a URL.
func ShortenURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return truncate(raw, 25)
	}
	p := strings.TrimRight(u.Path, "/")
	if p == "" {
		return u.Host
	}
	name := path.Base(p)
	if isNumeric(name) {
		parent := path.Base(path.Dir(p))
		if parent != "." && parent != "/" {
			name = parent + "/" + name
		}
	}
	for _, ext := range []string{".html", ".htm", ".md", ".pdf"} {
		name = strings.TrimSuffix(name, ext)
	}
	if len(name) > 40 {
		name = name[:37] + "..."
	}
	return name
}

func prettifySlug(slug string) string {
	words := strings.Split(slug, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}
```

Add `"net/url"` and `"path"` to imports.

- [ ] **Step 4: Run all style tests**

Run: `cd /home/zach/code/pike && go test ./internal/style/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/style/style.go internal/style/style_test.go
git commit -m "feat: add PrettifyText, PrettifyLinks, ShortenURL to style module"
```

## Chunk 2: Wire callers to use `internal/style`, delete duplicates

### Task 4: Update `internal/render/render.go` to use `style`

**Files:**
- Modify: `internal/render/render.go`
- Modify: `internal/render/render_test.go`

- [ ] **Step 1: Replace inline coloring logic with `style.ColorizeTags`**

Replace the entire `FormatTask` function body and delete `colorCode`, `namedColors`, `ansiReset`, and the render-local `tagToken`:

`internal/render/render.go` should become:

```go
package render

import (
	"fmt"
	"strings"

	"pike/internal/model"
	"pike/internal/style"
)

// FormatTask formats a single task for non-interactive output.
func FormatTask(task model.Task, tagColors map[string]string, noColor bool) string {
	state := " "
	if task.State == model.Completed {
		state = "x"
	}

	text := task.Text
	if !noColor && tagColors != nil {
		text = style.ColorizeTags(text, task.Tags, tagColors, style.ANSIStyleFunc())
	}

	if task.HasCheckbox {
		return fmt.Sprintf("%s:%d  - [%s] %s", task.File, task.Line, state, text)
	}
	return fmt.Sprintf("%s:%d  - %s", task.File, task.Line, text)
}

// FormatSummary formats the task summary counts for non-interactive output.
func FormatSummary(open, overdue, dueThisWeek, completedThisWeek int, noColor bool) string {
	const width = 30
	const ansiReset = "\033[0m"

	header := "\u2550\u2550\u2550 Task Summary \u2550\u2550\u2550"

	formatLine := func(label string, count int) string {
		countStr := fmt.Sprintf("%d", count)
		padding := width - len(label) - len(countStr)
		if padding < 1 {
			padding = 1
		}
		return fmt.Sprintf("  %s%s%s", label, strings.Repeat(" ", padding), countStr)
	}

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")
	b.WriteString(formatLine("Open tasks", open))
	b.WriteString("\n")

	overdueLine := formatLine("Overdue", overdue)
	if !noColor && overdue > 0 {
		overdueLine = "\033[31m" + overdueLine + ansiReset
	}
	b.WriteString(overdueLine)
	b.WriteString("\n")

	b.WriteString(formatLine("Due this week", dueThisWeek))
	b.WriteString("\n")
	b.WriteString(formatLine("Completed this week", completedThisWeek))

	return b.String()
}
```

- [ ] **Step 2: Update render_test.go for three-part valued tag output**

The tests for valued tags (`@due(2026-03-15)`, `@completed(2026-03-10)`) now produce three ANSI spans instead of one. Update the `want` strings:

Update each valued-tag test case's `want` string. The three-part render wraps `@name(`, `value`, and `)` each in their own color span:

Test `"colorized @due(...) tag in red"`:
```go
want: fmt.Sprintf("work/tasks.md:3  - [ ] Submit report %s@due(%s%s2026-03-15%s%s)%s", red, reset, red, reset, red, reset),
```

Test `"completed task with color"`:
```go
want: fmt.Sprintf("notes/done.md:8  - [x] Write tests %s@completed(%s%s2026-03-10%s%s)%s", green, reset, green, reset, green, reset),
```

Test `"no-color mode skips ANSI codes"` — unchanged (no ANSI in output).

Test `"multiple tags colorized independently"` — unchanged (bare tags only, no values).

- [ ] **Step 3: Run render tests**

Run: `cd /home/zach/code/pike && go test ./internal/render/... -v`
Expected: PASS

- [ ] **Step 4: Run full test suite to check nothing else broke**

Run: `cd /home/zach/code/pike && go test ./...`
Expected: PASS (or only pre-existing failures unrelated to this change)

- [ ] **Step 5: Commit**

```bash
git add internal/render/render.go internal/render/render_test.go
git commit -m "refactor: render.FormatTask delegates to style.ColorizeTags"
```

### Task 5: Update `internal/tui/sections.go` and delete `prettify.go`

**Files:**
- Modify: `internal/tui/sections.go`
- Modify: `internal/tui/styles.go`
- Delete: `internal/tui/prettify.go`
- Delete: `internal/tui/prettify_test.go`

- [ ] **Step 1: Delete `prettify.go` and `prettify_test.go` first**

Delete these files BEFORE adding `LinkStyle` to `styles.go` to avoid a duplicate symbol compilation error (`LinkStyle` is currently defined in `prettify.go`):

```bash
git rm internal/tui/prettify.go internal/tui/prettify_test.go
```

- [ ] **Step 2: Move `LinkStyle` to `styles.go`**

Add to the end of `internal/tui/styles.go`:

```go
// LinkStyle returns a bold style with the given color for rendering links.
func LinkStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(resolveColor(color))
}
```

- [ ] **Step 3: Add `lipglossStyleFunc` and rewrite `formatTaskLine` in `sections.go`**

Replace the entire `internal/tui/sections.go` content with:

```go
package tui

import (
	"fmt"
	"strings"

	"pike/internal/model"
	"pike/internal/style"

	"github.com/charmbracelet/lipgloss"
)

// lipglossStyleFunc applies foreground color via lipgloss.
func lipglossStyleFunc(text string, color string) string {
	return lipgloss.NewStyle().Foreground(resolveColor(color)).Render(text)
}

// RenderSection renders a single view section with a colored header and task list.
func RenderSection(title string, tasks []model.Task, color string, cursor int, sectionStart int, tagColors map[string]string, width int, linkColor string, hiddenCount int) string {
	if len(tasks) == 0 {
		return ""
	}

	headerStyle := SectionHeaderStyle(color)

	borderColor := resolveColor(color)
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	if width > 0 {
		innerWidth := width - 4
		if innerWidth < 10 {
			innerWidth = 10
		}
		borderStyle = borderStyle.Width(innerWidth)
	}

	var lines []string
	for i, task := range tasks {
		flatIdx := sectionStart + i
		selected := flatIdx == cursor
		line := formatTaskLine(task, tagColors, linkColor, selected)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	headerLabel := fmt.Sprintf(" %s ", title)
	if hiddenCount > 0 {
		headerLabel = fmt.Sprintf(" %s 🔒", title)
	}
	headerText := headerStyle.Render(headerLabel)
	box := borderStyle.Render(content)

	return headerText + "\n" + box
}

// formatTaskLine formats a single task line with colorized tags and styled links.
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

	var marker string
	if task.HasCheckbox {
		marker = "○"
		if task.State == model.Completed {
			marker = "●"
		}
	} else {
		marker = "▸"
	}

	if selected {
		marker = TaskStyle(true).Render(marker)
	}

	return fmt.Sprintf("%s %s", marker, text)
}
```

- [ ] **Step 4: Run all tests**

Run: `cd /home/zach/code/pike && go test ./...`
Expected: PASS. The `prettify_test.go` tests are now covered by `style_test.go`.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/sections.go internal/tui/styles.go
git commit -m "refactor: tui delegates to style, delete prettify.go"
```

## Chunk 3: Split `tui/model.go`

### Task 6: Split `model.go` into `model.go`, `keys.go`, `views.go`, `tasks.go`

This is a pure file-level split. No logic changes. All files stay in `package tui`.

**Approach:** Read `internal/tui/model.go`, extract the named functions listed below into their target files verbatim (no modifications to function bodies), then delete those functions from `model.go`. Each extracted file needs its own `package tui` declaration and appropriate imports. The function lists below are exhaustive — every function not listed under a target file stays in `model.go`.

**Files:**
- Modify: `internal/tui/model.go` (shrink to struct + Init/Update/NewModel/SetFocusedView)
- Create: `internal/tui/keys.go` (handleKey, handleTagSearchKey, openEditor)
- Create: `internal/tui/views.go` (View, viewDashboard, viewFocused, viewSummary, viewAllTasks, viewTagSearch, renderSections, truncateView, truncateViewPinTop, scrollWindow)
- Create: `internal/tui/tasks.go` (rebuildSections, applyHiddenFilter, matchesFilter, flatTasks, clampCursor, jumpToNextSection, jumpToPrevSection, displaySections, visibleSections, buildTagList, filteredTags, hiddenCountFor, countOpen, countOverdue, countDueThisWeek, countCompletedThisWeek, clearFilter, nowFunc)

- [ ] **Step 1: Create `internal/tui/keys.go`**

Read `internal/tui/model.go` and extract these exact functions into `internal/tui/keys.go`: `handleKey`, `openEditor`, `handleTagSearchKey`.

```go
package tui

import (
	"path/filepath"

	"pike/internal/editor"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ... exact code from model.go lines 162-320
}

func (m Model) openEditor() (tea.Model, tea.Cmd) {
	// ... exact code from model.go lines 322-341
}

func (m Model) handleTagSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ... exact code from model.go lines 900-951
}
```

- [ ] **Step 2: Create `internal/tui/views.go`**

Read `internal/tui/model.go` and extract these exact functions into `internal/tui/views.go`: `View`, `viewDashboard`, `viewFocused`, `viewSummary`, `displaySections`, `viewAllTasks`, `viewTagSearch`, `renderSections`, `truncateView`, `truncateViewPinTop`, `scrollWindow`.

```go
package tui

import (
	"fmt"
	"strings"

	"pike/internal/filter"
)

func (m Model) View() string {
	// ... exact code from model.go lines 344-366
}

func (m Model) viewDashboard() string {
	// ... exact code from model.go lines 368-376
}

func (m Model) viewFocused() string {
	// ... exact code from model.go lines 378-384
}

func (m Model) viewSummary() string {
	// ... exact code from model.go lines 386-393
}

func (m Model) displaySections() []filter.ViewResult {
	// ... exact code from model.go lines 396-413
}

func (m Model) viewAllTasks() string {
	// ... exact code from model.go lines 833-874
}

func (m Model) viewTagSearch() string {
	// ... exact code from model.go lines 877-897
}

func (m Model) renderSections() (string, int) {
	// ... exact code from model.go lines 729-752
}

func (m Model) truncateView(s string) string {
	return m.truncateViewPinTop(s, 0)
}

func (m Model) truncateViewPinTop(s string, pinnedTop int) string {
	// ... exact code from model.go lines 774-804
}

func scrollWindow(cursor, total, maxVisible int) (start, end int) {
	// ... exact code from model.go lines 808-822
}
```

- [ ] **Step 3: Create `internal/tui/tasks.go`**

Read `internal/tui/model.go` and extract these exact functions into `internal/tui/tasks.go`: `rebuildSections`, `applyHiddenFilter`, `matchesFilter`, `flatTasks`, `clampCursor`, `jumpToNextSection`, `jumpToPrevSection`, `visibleSections`, `buildTagList`, `filteredTags`, `hiddenCountFor`, `countOpen`, `countOverdue`, `countDueThisWeek`, `countCompletedThisWeek`, `clearFilter`, `nowFunc`.

```go
package tui

import (
	gosort "sort"
	"strings"
	"time"

	"pike/internal/filter"
	"pike/internal/model"
)

func (m *Model) rebuildSections() {
	// ... exact code from model.go lines 432-493
}

func (m *Model) applyHiddenFilter() {
	// ... exact code from model.go lines 497-520
}

func matchesFilter(t model.Task, tokens []string) bool {
	// ... exact code from model.go lines 523-560
}

func (m Model) flatTasks() []model.Task {
	// ... exact code from model.go lines 563-571
}

func (m *Model) clampCursor() {
	// ... exact code from model.go lines 574-586
}

func (m *Model) jumpToNextSection() {
	// ... exact code from model.go lines 589-622
}

func (m *Model) jumpToPrevSection() {
	// ... exact code from model.go lines 625-663
}

func (m Model) visibleSections() []filter.ViewResult {
	// ... exact code from model.go lines 416-429
}

func (m *Model) buildTagList() {
	// ... exact code from model.go lines 954-966
}

func (m Model) filteredTags() []string {
	// ... exact code from model.go lines 969-981
}

func (m Model) hiddenCountFor(title string) int {
	// ... exact code from model.go lines 755-762
}

func (m Model) countOpen() int {
	// ... exact code from model.go lines 666-674
}

func (m Model) countOverdue() int {
	// ... exact code from model.go lines 677-687
}

func (m Model) countDueThisWeek() int {
	// ... exact code from model.go lines 690-701
}

func (m Model) countCompletedThisWeek() int {
	// ... exact code from model.go lines 704-715
}

func (m *Model) clearFilter() {
	// ... exact code from model.go lines 718-725
}

func (m Model) nowFunc() time.Time {
	// ... exact code from model.go lines 824-829
}
```

- [ ] **Step 4: Trim `model.go` to only struct, types, NewModel, Init, Update, SetFocusedView**

`internal/tui/model.go` should contain only:
- Package declaration + imports
- `RefreshMsg`, `EditorFinishedMsg` types
- `viewMode` type and constants
- `Model` struct
- `NewModel()` (lines 64-91)
- `SetFocusedView()` (lines 94-98)
- `Init()` (lines 101-108)
- `Update()` (lines 111-160)

Remove all other functions from model.go — they now live in keys.go, views.go, tasks.go.

- [ ] **Step 5: Run all tests to confirm nothing broke**

Run: `cd /home/zach/code/pike && go test ./...`
Expected: PASS — same package, all tests still see all functions.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/model.go internal/tui/keys.go internal/tui/views.go internal/tui/tasks.go
git commit -m "refactor: split tui/model.go into model, keys, views, tasks"
```

## Chunk 4: Golden file test suite

### Task 7: Create test fixture markdown files

**Files:**
- Create: `testdata/config.yaml`
- Create: `testdata/notes/basic.md`
- Create: `testdata/notes/tags.md`
- Create: `testdata/notes/links.md`
- Create: `testdata/notes/mixed.md`
- Create: `testdata/notes/edge-cases.md`

- [ ] **Step 1: Create `testdata/config.yaml`**

```yaml
notes_dir: "testdata/notes"
include:
  - "**/*.md"
refresh_interval: "5s"
editor: "vi"
link_color: "#6CB4EE"
tag_colors:
  today: "green"
  risk: "red"
  due: "red"
  completed: "green"
  delegated: "yellow"
  blocked: "magenta"
  _default: "cyan"
views:
  - title: "Overdue"
    query: "open and @due < today"
    sort: "due_asc"
    color: "red"
    order: 1
  - title: "Today"
    query: "open and @today"
    sort: "file"
    color: "green"
    order: 2
  - title: "All Open"
    query: "open"
    sort: "file"
    color: "blue"
    order: 3
```

- [ ] **Step 2: Create `testdata/notes/basic.md`**

```markdown
# Basic Tasks

- [ ] Buy groceries
- [x] Clean the kitchen
- [ ] Walk the dog
- Plain text paragraph, not a task
- [X] Ship the package
```

- [ ] **Step 3: Create `testdata/notes/tags.md`**

```markdown
# Tagged Tasks

- [ ] Review PR @today
- [ ] Submit report @due(2026-03-15)
- [x] Write docs @completed(2026-03-10)
- [ ] Deploy service @risk @today
- [ ] Escalate issue @delegated(zach)
- [ ] Fix authentication @blocked
- Research spike @risk
- [ ] Hidden task @hidden
- [ ] Multiple dates @due(2026-03-20) with @risk
```

- [ ] **Step 4: Create `testdata/notes/links.md`**

```markdown
# Tasks With Links

- [ ] Talk to [[zach-thieme|Zach Thieme]] about deployment @today
- [ ] Read [migration guide](https://example.com/docs/migration) @risk
- [ ] Check https://github.com/org/repo/pull/123 @today
- [ ] Delegate to [[jeff-roache]] @delegated([[jeff-roache]])
- [ ] Review https://docs.example.com/guide/setup.html
```

- [ ] **Step 5: Create `testdata/notes/mixed.md`**

```markdown
# Mixed Content

Some paragraph text that should be ignored.

- [ ] High priority bug @risk @due(2026-03-14) see https://github.com/org/repo/issues/456
- [x] Completed review @completed(2026-03-12) @today
- [ ] Ask [[alice-bob|Alice Bob]] about [the plan](https://example.com/plan) @delegated(alice)
- Not a task, just a bullet
- Regular bullet with @tag makes it a task
```

- [ ] **Step 6: Create `testdata/notes/edge-cases.md`**

```markdown
# Edge Cases

- [ ] No tags at all
- [ ] Tag at start @today of the line
- [ ] Multiple same tag @risk and @risk again
- [ ] Empty parens @note()
- [ ] Unicode text 日本語タスク @today
- [ ] Very long tag value @delegated(some very long person name here)
- [ ] Nested parens in text (not a tag) @due(2026-04-01)
```

- [ ] **Step 7: Commit**

```bash
git add testdata/
git commit -m "feat: add golden file test fixtures (notes + config)"
```

### Task 8: Create golden file test runner and generate initial golden files

**Files:**
- Create: `golden_test.go`

- [ ] **Step 1: Write the golden file test runner**

Create `golden_test.go` at the project root:

```go
package pike_test

import (
	"bufio"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"pike/internal/config"
	"pike/internal/filter"
	"pike/internal/model"
	"pike/internal/parser"
	"pike/internal/render"
	"pike/internal/style"
)

func init() {
	// Force a deterministic color profile for lipgloss golden files.
	lipgloss.SetColorProfile(termenv.TrueColor)
}

var update = flag.Bool("update", false, "update golden files")

var testNow = time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

// parsedTask is a JSON-friendly version of model.Task for golden files.
type parsedTask struct {
	Text        string   `json:"text"`
	State       string   `json:"state"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Tags        []string `json:"tags"`
	Due         string   `json:"due,omitempty"`
	Completed   string   `json:"completed,omitempty"`
	HasCheckbox bool     `json:"has_checkbox"`
}

func toParsedTask(t model.Task) parsedTask {
	pt := parsedTask{
		Text:        t.Text,
		State:       t.State.String(),
		File:        t.File,
		Line:        t.Line,
		HasCheckbox: t.HasCheckbox,
	}
	for _, tag := range t.Tags {
		pt.Tags = append(pt.Tags, style.TagToken(tag))
	}
	if t.Due != nil {
		pt.Due = t.Due.Format("2006-01-02")
	}
	if t.Completed != nil {
		pt.Completed = t.Completed.Format("2006-01-02")
	}
	return pt
}

func loadTestConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load("testdata/config.yaml")
	if err != nil {
		t.Fatalf("load test config: %v", err)
	}
	return cfg
}

func scanTestNotes(t *testing.T) map[string][]model.Task {
	t.Helper()
	result := make(map[string][]model.Task)

	entries, err := os.ReadDir("testdata/notes")
	if err != nil {
		t.Fatalf("read testdata/notes: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		fpath := filepath.Join("testdata", "notes", entry.Name())
		f, err := os.Open(fpath)
		if err != nil {
			t.Fatalf("open %s: %v", fpath, err)
		}
		defer f.Close()

		relPath := entry.Name()
		scanner := bufio.NewScanner(f)
		lineNum := 0
		var tasks []model.Task
		for scanner.Scan() {
			lineNum++
			task := parser.ParseLine(scanner.Text(), relPath, lineNum)
			if task != nil {
				tasks = append(tasks, *task)
			}
		}
		if len(tasks) > 0 {
			result[relPath] = tasks
		}
	}
	return result
}

func allTasks(byFile map[string][]model.Task) []model.Task {
	var all []model.Task
	for _, tasks := range byFile {
		all = append(all, tasks...)
	}
	return all
}

func goldenCompare(t *testing.T, goldenPath string, actual []byte) {
	t.Helper()
	if *update {
		dir := filepath.Dir(goldenPath)
		os.MkdirAll(dir, 0755)
		if err := os.WriteFile(goldenPath, actual, 0644); err != nil {
			t.Fatalf("update golden %s: %v", goldenPath, err)
		}
		return
	}
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", goldenPath, err)
	}
	if string(actual) != string(expected) {
		t.Errorf("golden mismatch for %s:\n--- expected ---\n%s\n--- actual ---\n%s", goldenPath, string(expected), string(actual))
	}
}

func TestGoldenParsed(t *testing.T) {
	byFile := scanTestNotes(t)

	for filename, tasks := range byFile {
		baseName := strings.TrimSuffix(filename, ".md")
		t.Run(baseName, func(t *testing.T) {
			var pts []parsedTask
			for _, task := range tasks {
				pts = append(pts, toParsedTask(task))
			}
			data, err := json.MarshalIndent(pts, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			data = append(data, '\n')
			goldenCompare(t, filepath.Join("testdata", "golden", "parsed", baseName+".json"), data)
		})
	}
}

func TestGoldenStyledANSI(t *testing.T) {
	cfg := loadTestConfig(t)
	byFile := scanTestNotes(t)
	sf := style.ANSIStyleFunc()

	for filename, tasks := range byFile {
		baseName := strings.TrimSuffix(filename, ".md")
		t.Run(baseName, func(t *testing.T) {
			var lines []string
			for _, task := range tasks {
				text := style.ColorizeTags(task.Text, task.Tags, cfg.TagColors, sf)
				lines = append(lines, text)
			}
			actual := []byte(strings.Join(lines, "\n") + "\n")
			goldenCompare(t, filepath.Join("testdata", "golden", "styled", "ansi", baseName+".txt"), actual)
		})
	}
}

func TestGoldenStyledPlain(t *testing.T) {
	cfg := loadTestConfig(t)
	byFile := scanTestNotes(t)
	sf := style.ANSIStyleFunc()

	for filename, tasks := range byFile {
		baseName := strings.TrimSuffix(filename, ".md")
		t.Run(baseName, func(t *testing.T) {
			var lines []string
			for _, task := range tasks {
				text := style.ColorizeTags(task.Text, task.Tags, cfg.TagColors, sf)
				text = style.StripANSI(text)
				lines = append(lines, text)
			}
			actual := []byte(strings.Join(lines, "\n") + "\n")
			goldenCompare(t, filepath.Join("testdata", "golden", "styled", "plain", baseName+".txt"), actual)
		})
	}
}

func TestGoldenStyledRender(t *testing.T) {
	cfg := loadTestConfig(t)
	byFile := scanTestNotes(t)

	for filename, tasks := range byFile {
		baseName := strings.TrimSuffix(filename, ".md")
		t.Run(baseName, func(t *testing.T) {
			var lines []string
			for _, task := range tasks {
				line := render.FormatTask(task, cfg.TagColors, false)
				lines = append(lines, line)
			}
			actual := []byte(strings.Join(lines, "\n") + "\n")
			goldenCompare(t, filepath.Join("testdata", "golden", "styled", "render", baseName+".txt"), actual)
		})
	}
}

func TestGoldenStyledLipgloss(t *testing.T) {
	cfg := loadTestConfig(t)
	byFile := scanTestNotes(t)

	// lipglossStyleFunc mirrors the TUI's styling approach.
	sf := func(text string, color string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(text)
	}

	for filename, tasks := range byFile {
		baseName := strings.TrimSuffix(filename, ".md")
		t.Run(baseName, func(t *testing.T) {
			var lines []string
			for _, task := range tasks {
				text := style.ColorizeTags(task.Text, task.Tags, cfg.TagColors, sf)
				lines = append(lines, text)
			}
			actual := []byte(strings.Join(lines, "\n") + "\n")
			goldenCompare(t, filepath.Join("testdata", "golden", "styled", "lipgloss", baseName+".txt"), actual)
		})
	}
}

func TestGoldenQuery(t *testing.T) {
	cfg := loadTestConfig(t)
	byFile := scanTestNotes(t)
	all := allTasks(byFile)

	queries := map[string]string{
		"open-and-today":    "open and @today",
		"overdue":           "open and @due < today",
		"completed":         "completed",
	}

	for name, queryStr := range queries {
		t.Run(name, func(t *testing.T) {
			results, err := filter.Apply(all, queryStr, "file", testNow)
			if err != nil {
				t.Fatalf("filter: %v", err)
			}
			var pts []parsedTask
			for _, task := range results {
				pts = append(pts, toParsedTask(task))
			}
			data, err := json.MarshalIndent(pts, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			data = append(data, '\n')
			goldenCompare(t, filepath.Join("testdata", "golden", "query", name+".json"), data)
		})
	}
}
```

- [ ] **Step 2: Generate initial golden files**

Run: `cd /home/zach/code/pike && go test -run TestGolden -update`
This creates all golden files with current output.

- [ ] **Step 3: Verify golden files were created**

Run: `ls -R testdata/golden/`
Expected: Files in `parsed/`, `styled/ansi/`, `styled/plain/`, `styled/render/`, `styled/lipgloss/`, `query/`.

**Note on `lipgloss.SetColorProfile`:** If this API does not exist in lipgloss v1.1.0, use the renderer approach instead: `lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(os.Stderr, termenv.WithProfile(termenv.TrueColor)))`. Check the actual API available in `go doc github.com/charmbracelet/lipgloss` and adapt.

- [ ] **Step 4: Run golden tests without -update to confirm they pass**

Run: `cd /home/zach/code/pike && go test -run TestGolden -v`
Expected: PASS

- [ ] **Step 5: Inspect golden files for correctness**

Review the generated files to confirm:
- Parsed JSON has correct tags, states, dates
- Plain styled output has tag text preserved without ANSI
- ANSI styled output has escape codes around tags
- Query results return the right tasks

- [ ] **Step 6: Commit**

```bash
git add golden_test.go testdata/
git commit -m "feat: add golden file test suite for parsing, styling, and queries"
```

### Task 9: Run full test suite and verify build

**Files:** None (verification only)

- [ ] **Step 1: Run complete test suite**

Run: `cd /home/zach/code/pike && go test ./... -v`
Expected: All tests PASS.

- [ ] **Step 2: Build the binary**

Run: `cd /home/zach/code/pike && go build ./cmd/pike`
Expected: Clean build, no errors.

- [ ] **Step 3: Verify the binary runs**

Run: `cd /home/zach/code/pike && ./pike --help`
Expected: Help output, no crashes.

- [ ] **Step 4: Run `go vet`**

Run: `cd /home/zach/code/pike && go vet ./...`
Expected: No issues.
