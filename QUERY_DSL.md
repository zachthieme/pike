# Pike Query DSL Reference

A query language for filtering tasks by state, tags, dates, and text patterns. Used in view configs (`query` field) and the `--query` CLI flag.

## Grammar

```
expr     = or_expr
or_expr  = and_expr ("or" and_expr)*
and_expr = not_expr ("and" not_expr)*
not_expr = "not" not_expr | atom
atom     = "open" | "completed" | @tag | @tag <op> <date> | /regex/ | ( expr )
```

## Atoms

| Atom | Matches |
|------|---------|
| `open` | Tasks with open state (`- [ ]` or tagged bullets) |
| `completed` | Tasks with completed state (`- [x]` / `- [X]` or `@completed` tag) |
| `@tag` | Tasks containing the given tag |
| `@due < DATE` | Tasks with `@due(...)` before DATE |
| `@due > DATE` | Tasks with `@due(...)` after DATE |
| `@due <= DATE` | Tasks with `@due(...)` on or before DATE |
| `@due >= DATE` | Tasks with `@due(...)` on or after DATE |
| `@completed >= DATE` | Tasks with `@completed(...)` on or after DATE |
| `/regex/` | Task text matches regex |

## Operators

| Operator | Description |
|----------|-------------|
| `and` | Both sides must match |
| `or` | Either side must match |
| `not` | Negates the following expression |
| `<`, `>`, `<=`, `>=` | Date comparisons |

## Date Expressions

| Expression | Meaning |
|------------|---------|
| `today` | Current date (midnight) |
| `today+Nd` | N days from today |
| `today-Nd` | N days ago |
| `YYYY-MM-DD` | Literal date |

## Sort Orders

Used in view configs (`sort` field) and the `--sort` CLI flag.

| Value | Behavior |
|-------|----------|
| `due_asc` | By `@due()` date ascending, tasks without due dates last |
| `due_desc` | By `@due()` date descending, tasks without due dates last |
| `completed_asc` | By `@completed()` date ascending |
| `completed_desc` | By `@completed()` date descending |
| `file` | By file path, then line number (default) |
| `alpha` | Alphabetical by task text |

## Tag Format

Tags follow the format `@name` or `@name(value)`.

| Tag | Effect |
|-----|--------|
| `@due(YYYY-MM-DD)` | Sets the task's due date for date comparisons |
| `@completed` | Marks the task as completed (with or without a date) |
| `@completed(YYYY-MM-DD)` | Marks completed and records the completion date |
| `@hidden` | Hides the task from all views by default (toggle with `h`) |
| `@<word>` | Categorical tag for filtering (e.g., `@today`, `@risk`, `@talk`) |

## Examples

```
open                                    # all open tasks
open and @due < today                   # overdue
open and @due >= today and @due <= today+3d   # due within 3 days
completed and @completed >= today-7d    # completed in last week
open and (@weekly or @today)            # tagged weekly or today
open and not @risk                      # open, excluding risk
/deploy/                                # text matches "deploy"
@talk and open                          # open tasks tagged @talk
```

## View Config Example

```yaml
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
```
