# `tasks` — A Bubble Tea TUI for Markdown Task Management

## Overview

A terminal UI application written in Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea) that replaces the tasks pane currently implemented as a combination of fish shell functions (`ft`, `td`, `_colorize_tags`) and zellij layout panes. It scans a directory of markdown files for task items, filters and groups them into configurable views, and allows the user to open tasks in their editor.

## Background

The current implementation uses:
- **`ft`** (fish function) — ripgrep + awk + perl to find, filter, and colorize tasks, piped into fzf
- **`_colorize_tags`** — perl one-liner applying ANSI colors to `@tag` tokens
- **`td`** — summary dashboard printing counts (open, overdue, due this week, completed this week)
- **Zellij layout** — 4 hardcoded panes in the "tasks" tab: `ft -t 'talk'`, `ft -o`, `ft -u 3`, `ft -t 'risk|horizon'`

Problems with the current approach:
- Each pane is a separate shell process running ripgrep independently (redundant I/O)
- Adding/removing/reordering views requires editing Nix source and rebuilding
- No live refresh — panes are static after launch
- fzf provides selection but no persistent multi-view dashboard

## Task Format

Tasks are markdown checkbox list items found in `*.md` files:

```markdown
- [ ] Open task description @tag1 @tag2 @due(2026-03-15)
- [x] Completed task @due(2026-03-10) @completed(2026-03-10)
```

### Metadata Tokens

| Token | Format | Meaning |
|-------|--------|---------|
| `@due(YYYY-MM-DD)` | Date in parens | Due date |
| `@completed(YYYY-MM-DD)` | Date in parens | Completion date |
| `@<word>` | Bare tag | Categorical tag (e.g., `@today`, `@talk`, `@risk`, `@horizon`, `@weekly`) |

A task line is **open** if it matches `- [ ]` and **completed** if it matches `- [x]` or `- [X]`.

## Configuration

### File Location

The config file is loaded from (in order of precedence):
1. `--config <path>` CLI flag
2. `$TASKS_CONFIG` environment variable
3. `$XDG_CONFIG_HOME/tasks/config.yaml` (default: `~/.config/tasks/config.yaml`)

If no config file is found, the program runs with sensible defaults (a single "All Open" section).

### Schema

```yaml
# Directory to scan for markdown files (required, or pass via --dir / $NOTES)
notes_dir: ~/CloudDocs/Notes

# File glob patterns to include (default: ["**/*.md"])
include:
  - "**/*.md"

# File glob patterns to exclude (default: none)
exclude:
  - "templates/**"
  - "archive/**"

# How often to re-scan files for changes (default: 5s, 0 disables)
refresh_interval: 5s

# Editor command for opening tasks (default: $EDITOR, fallback: "hx")
editor: hx

# Tag color definitions — map tag names/patterns to colors
# Colors: "red", "green", "blue", "yellow", "magenta", "cyan", "white", or hex "#FF5733"
tag_colors:
  risk: red
  due: red
  today: green
  completed: green
  weekly: blue
  monthly: blue
  quarterly: blue
  horizon: yellow
  talk: magenta
  _default: cyan          # fallback for unrecognized tags

# View definitions — each becomes a section in the dashboard.
# The "order" field controls top-to-bottom display order (ascending).
# Views with the same order value are sorted by their position in the list.
views:
  - title: "Overdue"
    query: "open and @due < today"
    sort: due_asc
    color: red
    order: 1

  - title: "Today"
    query: "open and (@today or @weekly)"
    sort: due_asc
    color: green
    order: 2

  - title: "Next 3 Days"
    query: "open and @due >= today and @due <= today+3d"
    sort: due_asc
    color: yellow
    order: 3

  - title: "Talk"
    query: "@talk and open"
    sort: file
    color: magenta
    order: 4

  - title: "Horizon"
    query: "open and (@risk or @horizon)"
    sort: file
    color: yellow
    order: 5

  - title: "Completed"
    query: "completed and @completed >= today-7d"
    sort: completed_desc
    color: green
    order: 6
```

### Query Language

A minimal query DSL for filtering tasks. Queries compose with `and`, `or`, `not`, and parentheses.

#### Atoms

| Atom | Matches |
|------|---------|
| `open` | Tasks with `- [ ]` |
| `completed` | Tasks with `- [x]` / `- [X]` |
| `@tag` | Tasks containing the literal `@tag` token |
| `@due < DATE` | Tasks with `@due(...)` before DATE |
| `@due > DATE` | Tasks with `@due(...)` after DATE |
| `@due <= DATE` | Tasks with `@due(...)` on or before DATE |
| `@due >= DATE` | Tasks with `@due(...)` on or after DATE |
| `@completed >= DATE` | Tasks with `@completed(...)` on or after DATE |
| `/regex/` | Task text matches regex |

#### Date Expressions

| Expression | Meaning |
|------------|---------|
| `today` | Current date |
| `today+Nd` | N days from today |
| `today-Nd` | N days ago |
| `YYYY-MM-DD` | Literal date |

#### Examples

```
open and @due < today                         # overdue
open and (@today or @weekly)                  # daily view
completed and @completed >= today-14d         # completed last 2 weeks
open and /meeting/                            # open tasks mentioning "meeting"
open and not @horizon                         # open, excluding horizon items
```

### Sort Orders

| Value | Behavior |
|-------|----------|
| `due_asc` | By `@due()` date ascending, tasks without due dates last |
| `due_desc` | By `@due()` date descending, tasks without due dates last |
| `completed_desc` | By `@completed()` date descending |
| `completed_asc` | By `@completed()` date ascending |
| `file` | By file path, then line number (default) |
| `alpha` | Alphabetical by task text |

## User Interface

### Default View: Unified Dashboard

The default view renders **all configured views as stacked sections** in a single scrollable pane, ordered by each view's `order` field. Each section has a colored header and its filtered/sorted task list beneath it. Empty sections are hidden.

```
╭─ Overdue ─────────────────────────────────────────────────────╮
│  - [ ] Ship auth fix @risk @due(2026-03-10)                   │
│  - [ ] File expense report @due(2026-03-11)                   │
╰───────────────────────────────────────────────────────────────╯
╭─ Today ───────────────────────────────────────────────────────╮
│  - [ ] Review quarterly OKRs @weekly @due(2026-03-15)         │
│  - [ ] Call dentist @today                                    │
╰───────────────────────────────────────────────────────────────╯
╭─ Next 3 Days ─────────────────────────────────────────────────╮
│  - [ ] Prepare slide deck @talk @due(2026-03-14)              │
╰───────────────────────────────────────────────────────────────╯
╭─ Talk ────────────────────────────────────────────────────────╮
│  - [ ] Prepare slide deck @talk @due(2026-03-14)              │
│  - [ ] Discuss arch proposal with team @talk                  │
╰───────────────────────────────────────────────────────────────╯
╭─ Horizon ─────────────────────────────────────────────────────╮
│  - [ ] Write migration plan @horizon                          │
│  - [ ] Evaluate vendor risk @risk                             │
╰───────────────────────────────────────────────────────────────╯
──────────────────────────────────────────────────────── 23 open
```

- Section headers use the view's configured `color`
- The cursor moves through tasks across all sections as a single flat list
- Sections with zero matching tasks are collapsed (hidden entirely)
- A footer shows aggregate task count

### Focused View

Pressing the number key for a section (or `Enter` on a section header) switches to a **focused view** showing only that section's tasks full-screen. Press `Esc` to return to the dashboard.

### Summary Overlay

Pressing `s` toggles a summary overlay (replacing the `td` function):

```
═══ Task Summary ═══

  Open tasks              42
  Overdue                  3
  Due this week            7
  Completed this week     12
```

Overdue count is highlighted in red when > 0.

### Keybindings

| Key | Action |
|-----|--------|
| `j` / `k` or `↓` / `↑` | Move selection down / up (skips section headers) |
| `g` / `G` | Jump to first / last task |
| `Tab` / `Shift+Tab` | Jump to next / previous section header |
| `1`–`9` | Focus section N (full-screen that section's tasks) |
| `Enter` | Open selected task in editor at its line number |
| `s` | Toggle summary overlay |
| `/` | Filter across all sections (fuzzy search within visible tasks) |
| `Esc` | Clear filter / exit focused view / dismiss summary |
| `r` | Force refresh (re-scan files) |
| `q` | Quit |

### Editor Integration

When the user presses `Enter`, the program:
1. Suspends the TUI (releases the terminal via Bubble Tea's `tea.ExecProcess`)
2. Runs `$editor $file:$line` (or `$editor +$line $file` depending on editor — detect `hx`/`helix`/`nvim`/`vim`/`vi`/`code` and adjust syntax)
3. On editor exit, resumes the TUI and triggers a refresh

## CLI Interface

```
tasks [flags]

Flags:
  --dir, -d <path>       Notes directory (overrides config/env)
  --config, -c <path>    Config file path
  --view, -v <name>      Start focused on a specific section
  --summary              Print summary counts to stdout and exit (non-interactive, replaces `td`)
  --query, -q <query>    Run a one-shot query, print results to stdout and exit (replaces `ft`)
  --color                Force color output in non-interactive mode (default: auto-detect TTY)
  --no-color             Disable color output
  --help, -h             Show help
  --version              Show version
```

### Non-Interactive Mode

`--summary` and `--query` print to stdout without launching the TUI, making the tool composable with pipes and scripts. Output format matches the current `ft` output: one task per line, colorized tags, `file:line` reference.

```bash
# Replace: ft -o
tasks -q 'open and @due < today'

# Replace: ft '@weekly|@today'
tasks -q 'open and (@weekly or @today)'

# Replace: ft -t 'talk'
tasks -q '@talk'

# Replace: ft -c 14
tasks -q 'completed and @completed >= today-14d'

# Replace: td
tasks --summary
```

## File Scanning

- Recursively walk `notes_dir` matching `include` globs, excluding `exclude` globs
- Parse each file line-by-line, identifying lines matching `^\s*- \[([ xX])\] (.*)$`
- Extract: checkbox state, full task text, file path, line number, all `@tag` and `@tag(value)` tokens
- On refresh (timer or manual), re-scan only files with `mtime` newer than last scan (full scan on startup)

## Data Model

```go
type Task struct {
    Text      string            // Full line text after "- [ ] " / "- [x] "
    State     TaskState         // Open or Completed
    File      string            // Relative path from notes_dir
    Line      int               // 1-based line number
    Tags      []Tag             // Parsed @tag tokens
    Due       *time.Time        // Parsed from @due(YYYY-MM-DD), nil if absent
    Completed *time.Time        // Parsed from @completed(YYYY-MM-DD), nil if absent
}

type TaskState int
const (
    Open TaskState = iota
    Completed
)

type Tag struct {
    Name  string  // e.g., "today", "due", "risk"
    Value string  // e.g., "2026-03-15" for @due(2026-03-15), empty for bare tags
}
```

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Styling/colors
- [Bubbles](https://github.com/charmbracelet/bubbles) — List, tabs, text input components
- [doublestar](https://github.com/bmatcuk/doublestar) — Glob matching for include/exclude
- Standard library `gopkg.in/yaml.v3` for config parsing

## Build & Install

```bash
go build -o tasks ./cmd/tasks
```

Intended to be added to this dotfiles repo under a `tools/tasks/` directory and built via Nix (added to the package set in `packages/common.nix`).

## Migration Path

Once the Go program is working:

1. Add it to `packages/common.nix` as a Nix package (use `buildGoModule`)
2. Ship a default config in `config/tasks/config.yaml` symlinked via `home.file`
3. Replace the zellij "tasks" tab's 4 panes with a single `tasks` pane
4. Replace the "daily" tab's `ft '@weekly|@today'` pane with `tasks -v Today`
5. Alias `ft` and `td` to `tasks -q` and `tasks --summary` for backwards compatibility
6. Remove `ft`, `td`, and `_colorize_tags` from `fish.nix`
