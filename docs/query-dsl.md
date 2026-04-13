# Pike Query DSL Reference

A query language for filtering tasks by state, tags, dates, and text patterns. Used in view configs (`query` field) and the `--query` CLI flag.

## Grammar

```
expr     = or_expr
or_expr  = and_expr ("or" and_expr)*
and_expr = not_expr (("and")? not_expr)*
not_expr = "not" not_expr | atom
atom     = "open" | "completed" | "task" | "bullet" | @tag | @tag <op> <date> | /regex/ | "text" | word | ( expr )
```

Adjacent atoms are implicitly ANDed: `open task @risk` is equivalent to `open and task and @risk`.

## Atoms

| Atom | Matches |
|------|---------|
| `open` | Items with open state (unchecked checkboxes and tagged bullets) |
| `completed` | Tasks with completed state (`- [x]` / `- [X]` or `@completed` tag) |
| `task` | Items with a checkbox (`- [ ]` or `- [x]`) |
| `bullet` | Plain tagged items without a checkbox (`- item @tag`) |
| `@tag` | Tasks containing the given tag |
| `@due < DATE` | Tasks with `@due(...)` before DATE |
| `@due > DATE` | Tasks with `@due(...)` after DATE |
| `@due <= DATE` | Tasks with `@due(...)` on or before DATE |
| `@due >= DATE` | Tasks with `@due(...)` on or after DATE |
| `@due = DATE` | Tasks with `@due(...)` exactly on DATE (`==` also accepted) |
| `@completed >= DATE` | Tasks with `@completed(...)` on or after DATE |
| `/regex/` | Task text matches regex |
| `word` | Case-insensitive substring match against task text |
| `"multi word"` | Quoted substring match against task text |

## Operators

| Operator | Description |
|----------|-------------|
| `and` | Both sides must match |
| `or` | Either side must match |
| `not` | Negates the following expression |
| `<`, `>`, `<=`, `>=`, `=` | Date comparisons (`==` also accepted) |

## Date Expressions

| Expression | Meaning |
|------------|---------|
| `today` | Current date (midnight) |
| `tomorrow` | Tomorrow (shorthand for `today+1d`) |
| `yesterday` | Yesterday (shorthand for `today-1d`) |
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

Tags follow the format `@name` or `@name(value)`. See [README](../README.md#special-tags) for the full list of special tags (`@due`, `@completed`, `@hidden`, `@pin`).

**Case sensitivity:** The `@tag` atom matches tag names exactly as written — `@Today` and `@today` are different queries. Use lowercase by convention. Bare words and quoted strings are matched case-insensitively against task text. In the TUI filter bar's query mode (`?`), partial tag matching is case-insensitive.

## Examples

```
open                                    # all open tasks
open and @due < today                   # overdue
open and @due = today                   # due exactly today
open and @due < tomorrow                # due today or overdue
open and @due >= today and @due <= today+3d   # due within 3 days
completed and @completed >= today-7d    # completed in last week
open and (@weekly or @today)            # tagged weekly or today
open and not @risk                      # open, excluding risk
/deploy/                                # text matches "deploy"
open and deploy                         # open tasks containing "deploy"
open and "meeting notes"                # open tasks containing "meeting notes"
@talk and open                          # open tasks tagged @talk
open task and not @due                  # open checkbox tasks without due dates (implicit AND between open and task)
bullet and @risk                        # plain tagged items (no checkbox) tagged @risk
```

## Scope

`--scope <file>` is a CLI-only filter that restricts results to tasks referencing the given file. It can be combined with any query or view:

```
pike --scope "Bob Smith.md"                        # all open tasks about Bob
pike --scope "Bob Smith.md" --query "@talk"         # @talk tasks about Bob
pike --scope "Bob Smith.md" --query "open and @due < today"  # overdue tasks about Bob
pike --scope "Bob Smith.md" --view "Today"          # today's view scoped to Bob
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
