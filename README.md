# tasks

A terminal dashboard for managing tasks scattered across markdown files. Scans your notes directory for checkbox items (`- [ ]`/`- [x]`) and tagged bullets (`- text @tag`), groups them into configurable views, and displays them in an interactive TUI.

## Installation

```bash
go install ./cmd/tasks
```

Or build locally:

```bash
go build -o tasks ./cmd/tasks
```

## Quick Start

```bash
# Point at your notes and launch the dashboard
tasks --dir ~/notes

# Or set it in config and just run
tasks
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

Any other `@word` tag (e.g. `@today`, `@risk`, `@weekly`, `@talk`) is a plain tag used for filtering and categorization.

## Usage

```
tasks [flags]
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
tasks

# Show only overdue tasks
tasks -q "open and @due < today"

# List everything tagged @risk, sorted alphabetically
tasks -q "@risk" --sort alpha

# Print a summary of open/overdue/due counts
tasks --summary

# Start focused on the "Today" section
tasks -v Today
```

## Configuration

Config is loaded from (in order of precedence):

1. `--config` flag
2. `$TASKS_CONFIG` environment variable
3. `$XDG_CONFIG_HOME/tasks/config.yaml`
4. `~/.config/tasks/config.yaml`
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
atom     = "open" | "completed" | @tag | @tag <op> <date> | /regex/ | ( expr )
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
/deploy/                                # text matches "deploy"
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
| `/` | Activate filter bar |
| `s` | Toggle summary overlay |
| `r` | Refresh (re-scan files) |
| `q` | Quit |

### Filter Mode

When the filter bar is active (`/`):

| Key | Action |
|-----|--------|
| Type | Filter tasks across all sections |
| `Up` / `Ctrl+P` | Move cursor up |
| `Down` / `Ctrl+N` | Move cursor down |
| `Tab` | Jump to next section |
| `Enter` | Open selected task in editor |
| `Esc` | Clear filter and exit filter mode |

## Display

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
cmd/tasks/main.go              CLI entrypoint and flag parsing
internal/
  model/task.go                Task, Tag, and TaskState types
  parser/parser.go             Markdown line parser
  config/config.go             YAML config loading with defaults
  query/
    lexer.go                   Query DSL tokenizer
    ast.go                     AST node types
    parser.go                  Recursive-descent parser
    eval.go                    AST evaluator
  sort/sort.go                 Task sorting (6 orders)
  scanner/scanner.go           File walker with mtime-based caching
  filter/filter.go             Query + sort pipeline, view engine
  editor/editor.go             Editor command construction
  render/render.go             Non-interactive stdout formatting
  tui/
    model.go                   Bubbletea Model (Init/Update/View)
    sections.go                Section rendering with borders
    styles.go                  Lipgloss style helpers
    keymap.go                  Key bindings
    summary.go                 Summary overlay
    prettify.go                Link/URL prettification
```
