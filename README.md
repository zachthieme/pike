# pike

> **pike** /pa&#618;k/ *n.* — a long pointed tool, used to pick through things quickly and with precision.
>
> Your tasks are scattered across dozens of markdown files, buried between meeting notes and half-finished paragraphs. Pike reaches in and pulls them out.

---

Your notes are already in markdown. Your tasks are already in your notes. You don't need another app, another inbox, another tab. You need something that reads what you've already written and shows you what matters — in your terminal, where you already are.

Pike scans your notes directory for checkbox items (`- [ ]`/`- [x]`) and tagged bullets (`- text @tag`), groups them into configurable views via a query DSL, and renders them in an interactive TUI dashboard. No syncing, no database, no account. Just your files.

## Installation

```bash
go install ./cmd/pike
```

Or build locally:

```bash
go build -o pike ./cmd/pike
```

## Quick Start

```bash
# Point at your notes and launch the dashboard
pike --dir ~/notes

# Or set it in config and just run
pike
```

## Task Format

Tasks are extracted from markdown files. Two formats are recognized:

**Checkbox tasks** have explicit state:

```markdown
- [ ] Open task @today
- [x] Completed task @completed(2026-03-10)
```

**Tagged bullets** are plain list items that contain at least one `@tag`:

```markdown
- Review the auth design @talk
- Ship metrics endpoint @risk @due(2026-04-01)
```

Tags follow the format `@name` or `@name(value)`.

### Special Tags

| Tag | Effect |
|-----|--------|
| `@due(YYYY-MM-DD)` | Sets the task's due date for date comparisons |
| `@completed` | Marks the task as completed (with or without a date) |
| `@completed(YYYY-MM-DD)` | Marks completed and records the completion date |
| `@hidden` | Hides the task from all views by default (toggle with `h`) |
| `@pin` | Floats the task to the top of its section |

Any other `@word` tag (e.g. `@today`, `@risk`, `@weekly`, `@talk`) is a plain tag used for filtering and categorization.

## Usage

```
pike [flags]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--dir <path>` | `-d` | Notes directory (overrides config and `$NOTES` env) |
| `--config <path>` | `-c` | Config file path |
| `--view <name>` | `-v` | Start focused on a named section |
| `--query <query>` | `-q` | Run a query, print results to stdout, and exit |
| `--sort <order>` | | Sort order for `--query` mode (default: `file`) |
| `--summary` | | Print task summary counts and exit |
| `--color` | | Force color output |
| `--no-color` | | Disable color output |
| `--version` | | Print version |
| `--help` | `-h` | Print help |

### Examples

```bash
# Launch the TUI dashboard
pike

# Show only overdue tasks
pike -q "open and @due < today"

# List everything tagged @risk, sorted alphabetically
pike -q "@risk" --sort alpha

# Print a summary of open/overdue/due counts
pike --summary

# Start focused on the "Today" section
pike -v Today
```

## Configuration

Config is loaded from (in order of precedence):

1. `--config` flag
2. `$PIKE_CONFIG` environment variable
3. `$XDG_CONFIG_HOME/pike/config.yaml`
4. `~/.config/pike/config.yaml`
5. Built-in defaults

### Config File

```yaml
# Directory containing your markdown notes (supports ~ expansion)
notes_dir: ~/notes

# File patterns to include (default: all .md files)
include:
  - "**/*.md"

# File patterns to exclude
exclude:
  - "templates/**"
  - "archive/**"

# How often to re-scan files for changes (default: 5s)
refresh_interval: 5s

# Editor command for opening tasks (default: $EDITOR, then hx)
editor: hx

# Color for rendering prettified links (default: blue)
link_color: blue

# Days to show in recently-completed view (default: 7)
recently_completed_days: 7

# Map tag names to display colors
# Supports named colors (red, green, yellow, blue, magenta, cyan, white)
# and hex colors (#FF5733)
tag_colors:
  risk: red
  due: red
  today: green
  completed: green
  weekly: blue
  horizon: yellow
  talk: magenta
  _default: cyan       # fallback for unspecified tags

# Dashboard sections — each view is a filtered, sorted slice of your tasks
views:
  - title: "Today"
    query: "open and @today"
    sort: due_asc
    color: green
    order: 1

  - title: "Overdue"
    query: "open and @due < today"
    sort: due_asc
    color: red
    order: 2

  - title: "Next 3 Days"
    query: "open and @due >= today and @due <= today+3d"
    sort: due_asc
    color: yellow
    order: 3

  - title: "Talk"
    query: "open and @talk"
    sort: file
    color: magenta
    order: 4

  - title: "Horizon"
    query: "@risk or @horizon"
    sort: file
    color: yellow
    order: 5
```

### View Config Fields

| Field | Description |
|-------|-------------|
| `title` | Section header text |
| `query` | Query DSL expression to filter tasks |
| `sort` | Sort order for results |
| `color` | Section color (named or hex) |
| `order` | Display position (ascending) |

## Query DSL

The query language filters tasks by state, tags, dates, and text patterns. Queries are used in view configs and the `--query` flag.

### Grammar

```
expr     = or_expr
or_expr  = and_expr ("or" and_expr)*
and_expr = not_expr ("and" not_expr)*
not_expr = "not" not_expr | atom
atom     = "open" | "completed" | @tag | @tag <op> <date> | /regex/ | "text" | word | ( expr )
```

### Atoms

| Atom | Matches |
|------|---------|
| `open` | Tasks with open state |
| `completed` | Tasks with completed state |
| `@tag` | Tasks that have the given tag |
| `@due < today` | Date comparison on the due field |
| `@completed >= today-7d` | Date comparison on the completed field |
| `/pattern/` | Regex match against task text |
| `word` | Case-insensitive substring match against task text |
| `"multi word"` | Quoted substring match against task text |

### Operators

| Operator | Description |
|----------|-------------|
| `and` | Both sides must match |
| `or` | Either side must match |
| `not` | Negates the following expression |
| `<`, `>`, `<=`, `>=` | Date comparisons |

### Date Values

| Value | Description |
|-------|-------------|
| `today` | Current date (midnight) |
| `today+Nd` | N days from today |
| `today-Nd` | N days before today |
| `YYYY-MM-DD` | Absolute date literal |

### Example Queries

```
open                                    # all open tasks
open and @due < today                   # overdue
open and @due >= today and @due <= today+3d   # due within 3 days
completed and @completed >= today-7d    # completed in last week
open and (@weekly or @today)            # tagged weekly or today
open and not @risk                      # open, excluding risk
/deploy/                                # regex matches "deploy"
open and deploy                         # open tasks containing "deploy"
open and "meeting notes"                # open tasks containing "meeting notes"
```

## Sort Orders

| Order | Description |
|-------|-------------|
| `due_asc` | By due date, earliest first (nil last) |
| `due_desc` | By due date, latest first (nil last) |
| `completed_asc` | By completion date, earliest first (nil last) |
| `completed_desc` | By completion date, latest first (nil last) |
| `file` | By file path, then line number |
| `alpha` | Alphabetically by task text |

## TUI Keybindings

### Navigation

| Key | Action |
|-----|--------|
| `j` / `Down` | Move cursor down |
| `k` / `Up` | Move cursor up |
| `Ctrl+D` | Scroll down half page |
| `Ctrl+U` | Scroll up half page |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `Tab` | Jump to next section |
| `Shift+Tab` | Jump to previous section |
| `1`-`9` | Focus on section N |
| `Esc` | Exit focus / dismiss summary |

### Actions

| Key | Action |
|-----|--------|
| `Enter` | Open task in editor at the correct line |
| `/` | Activate query bar |
| `a` | All tasks — show every task with search |
| `t` | Tag search — browse and pick a tag |
| `x` | Toggle task complete/incomplete |
| `H` | Toggle `@hidden` tag on selected task |
| `c` | Recently completed tasks |
| `h` | Toggle hidden tasks visibility (show/hide `@hidden` tasks) |
| `s` | Toggle summary overlay |
| `r` | Refresh (re-scan files) |
| `q` | Quit |

### Query Mode

When the query bar is active (`/` or `a`), you can type full query DSL expressions or plain text:

| Input | Meaning |
|-------|---------|
| `foo` | Text must contain "foo" (simple substring) |
| `foo bar` | Text must contain both "foo" and "bar" |
| `@tag` | Task must have `@tag` (partial match: `@du` matches `@due`) |
| `open and @due < today` | Full DSL query |
| `not @risk` | DSL negation |

Plain text is matched as case-insensitive substrings (space-separated, ANDed). If the input contains DSL keywords (`and`, `or`, `not`, `@tag`, operators), it is parsed as a full query DSL expression with partial tag matching. Invalid DSL shows a parse error in the footer without clearing results.

Press `Tab` to switch focus between the query bar and the results list. When results are focused, `j`/`k`, `g`/`G`, and `x` (toggle complete) work directly on tasks. Press `Tab` or `/` to return to editing the query. `Esc` from results returns to the query bar; `Esc` from the query bar exits filter mode.

| Key | Action |
|-----|--------|
| Type | Filter tasks across all sections |
| `Tab` | Toggle focus between query bar and results |
| `Up` / `Down` / `Ctrl+P` / `Ctrl+N` | Move cursor up/down |
| `Ctrl+D` | Scroll down half page |
| `Ctrl+U` | Scroll up half page |
| `x` | Toggle task complete (when results focused) |
| `H` | Toggle `@hidden` tag (when results focused) |
| `j` / `k` / `g` / `G` | Navigate results (when results focused) |
| `/` | Return focus to query bar |
| `Enter` | Open selected task in editor |
| `Esc` | Return to query bar, or exit filter mode |

### Tag Search Mode

Press `t` to browse all tags found in your notes. Tags are displayed in a compact flow-wrapped line. Matched tags are highlighted with their configured color, the selected tag gets reverse video, and non-matching tags are faint. Type to narrow matches (the `@` prefix is optional — both `@due` and `due` work). Selecting a tag shows all tasks with that tag, including completed tasks and tagged bullets.

| Key | Action |
|-----|--------|
| Type | Filter tag list (partial match, `@` optional) |
| `Tab` / `Down` | Cycle forward through matched tags |
| `Shift+Tab` / `Up` | Cycle backward through matched tags |
| `Enter` | Select tag and show all matching tasks |
| `Backspace` to empty | Return to tag search from filtered results |
| `Esc` | Cancel and return to dashboard |

### Hidden Tasks

Tasks tagged `@hidden` are excluded from all views by default. Sections that contain hidden tasks display a 🔒 icon next to their title. Press `h` to toggle visibility — when enabled, hidden tasks appear normally and the lock icons disappear.

This is useful for tasks you want to keep in your notes but don't need to see day-to-day (e.g., deferred items, low-priority backlog, sensitive tasks).

### Task Toggling

Press `x` on a checkbox task to toggle its completion state directly in the source file:

- **Completing:** `- [ ] Task` becomes `- [x] Task @completed(2026-03-14)` (today's date is appended)
- **Un-completing:** `- [x] Task @completed(2026-03-14)` becomes `- [ ] Task` (checkbox is unchecked and `@completed(...)` tag is removed)

Indented tasks and tasks with other tags are handled correctly. Non-checkbox tasks (tagged bullets) are not affected. If the file has been modified externally since the last scan, the toggle validates the line content before writing and shows an error if it doesn't match.

To toggle tasks while the query bar is active, press `Tab` to focus the results list first, then `x` to toggle.

### Recently Completed

Press `c` to see tasks completed in the last N days (configurable via `recently_completed_days` in config, default 7). The view opens with a pre-filled query `completed and @completed >= today-Nd` which you can edit. Press `x` to un-complete a task (undo an accidental completion).

### Pinned Tasks

Tasks tagged `@pin` float to the top of their section, regardless of sort order. Within the pinned group and the unpinned group, the section's configured sort order is preserved.

## Display

Section headers show the task count: `Today (3)`. When hidden tasks exist, a lock icon appears: `Today (3) 🔒`.

### Task Markers

| Marker | Meaning |
|--------|---------|
| `○` | Open checkbox task |
| `●` | Completed checkbox task |
| `▸` | Tagged bullet (non-checkbox) |

### Link Prettification

Markdown syntax is cleaned up for display:

| Source | Display |
|--------|---------|
| `[[slug\|Display Name]]` | **Display Name** |
| `[[slug#Display Name]]` | **Display Name** |
| `[[Display Name]]` | **Display Name** |
| `[[zach-thieme]]` | **Zach Thieme** |
| `[link text](url)` | **link text** |
| `https://example.com/docs/guide` | **guide** |
| `https://github.com/org/repo/pull/123` | **pull/123** |

Links are rendered in bold with a configurable color (default: blue, set via `link_color` in config).

## Editor Integration

When you press `Enter` on a task, the configured editor opens the file at the task's line number. Editor argument syntax is auto-detected:

| Editor | Command |
|--------|---------|
| `hx` | `hx file:line` |
| `nvim` / `vim` | `nvim +line file` |
| `code` | `code --goto file:line` |
| Other | `$EDITOR file` |

## Project Structure

```
cmd/pike/main.go               CLI entrypoint and flag parsing
internal/
  model/task.go                Task, Tag, and TaskState types
  parser/parser.go             Markdown line parser
  config/config.go             YAML config loading with defaults
  query/
    lexer.go                   Query DSL tokenizer
    ast.go                     AST node types
    parser.go                  Recursive-descent parser
    eval.go                    AST evaluator
  sort/sort.go                 Task sorting (6 orders) and pin partitioning
  toggle/toggle.go             Task completion toggling (file writes)
  scanner/scanner.go           File walker with mtime-based caching
  filter/filter.go             Query + sort pipeline, view engine
  editor/editor.go             Editor command construction
  render/render.go             Non-interactive stdout formatting
  style/style.go               Tag coloring, link prettification, ANSI helpers
  tui/
    model.go                   Bubbletea Model struct, Init, Update
    keys.go                    Key handlers and mode transitions
    views.go                   View rendering (dashboard, all-tasks, tag search)
    tasks.go                   Task filtering, sections, cursor, counting
    sections.go                Section rendering with borders
    styles.go                  Lipgloss style helpers
    keymap.go                  Key bindings
    summary.go                 Summary overlay
testdata/                      Golden file test fixtures and expected outputs
golden_test.go                 Golden file test runner
```
