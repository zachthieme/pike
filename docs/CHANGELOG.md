# Pike Changelog & Development History

## Timeline

Pike was built over 3 days — March 13-15, 2026 — across 90+ commits.

### Day 1 — March 13: Foundation

The entire core was built in a single day: markdown parser, task model, query DSL (lexer, parser, evaluator), configurable views, tag coloring, link prettification, and the Bubbletea TUI with dashboard, all-tasks, and tag-search modes. By end of day the app was functional with `@hidden` support, scroll windowing, config hot-reload, and a design spec for the next round of work.

### Day 2 — March 14: Polish & Power Features

The biggest day. Started with a major refactor — extracted a shared `style` package, split the monolithic `model.go` into focused files, and added a golden file test suite. Then rapid-fire feature work:

- **Tag search redesign** — flow-wrapped inline display with colored/highlighted tags
- **Task toggling** (`x`) — complete/uncomplete tasks directly in source files
- **`@pin` tag** — float important tasks to section tops
- **Query DSL upgrade** — full boolean expressions with partial tag matching (`@du` matches `@due`), text literals, regex
- **Recently completed** (`c`) — pre-filled DSL query for last N days
- **Tab focus model** — toggle between query bar and results
- **`H` key** — toggle `@hidden` tag on any task
- **Ctrl-D/U** — half-page scrolling
- Dozens of layout/spacing fixes for the all-tasks and tag-search views

### Day 3 — March 15: UX Refinement

- **Nix flake** for cross-platform installation
- **Query box focus behavior** — Enter submits query (doesn't open file), no cursor highlight while typing, two-step Escape (clear text, then exit)
- **Filter mode split** — `/` for substring search, `?` for DSL queries, prompt character shows active mode
- **Code review fixes** — captured dropped focus commands, added `?` handler when results focused, restored `queryErr` display in all views
- **Simplification pass** — extracted `cursorUp`/`cursorDown`/`cursorSection`/`countFlatTasks` helpers, centralized filter mode activation via `setFilterMode` and `filterPrompt` map
- **Summary overlay** — full-screen layout with version, README description, and keybindings organized into sections (Navigation, Actions, Search & Filter, Other)

### v1.1.0 — March 15: Architecture & Release Automation

**Architecture improvements (12 changes):**
- Atomic file writes in toggle (write-to-temp + rename) for crash safety
- Per-file mutex locking to prevent concurrent mutation races
- Typed sentinel errors (`ErrStaleData`, `ErrLineOutOfRange`) for programmatic handling
- O(1) `HasTag` via `TagSet` map populated at parse time
- `=` and `==` operators in query DSL for date equality (`@due = today`)
- Deduplicated scanner walk logic into shared `walkMatching` helper
- `context.Context` threading through scanner for cancellation support
- Extracted `FilterState` struct from TUI Model
- Cached unfiltered sections so `visibleSections()` avoids recomputing queries per keypress
- Cached `openCount` in `rebuildSections` instead of rescanning on every `View()`
- Config reload errors now surface in TUI footer
- Shared `style.TaskMarker` for consistent markers across render paths

**Visibility icons:**
- `◌`/`◉` icons replace `🔒` on section headers for hidden task visibility
- `◌` (configurable color) shown when hidden tasks are concealed
- `◉` (configurable color) shown when hidden tasks are revealed via `h`
- New config options: `hidden_color` and `visible_color`
- `h` key now works when filter results are focused

**Release automation:**
- GitHub Actions workflow triggers on `v*` tag push
- GoReleaser builds binaries for linux/darwin x amd64/arm64
- Nix flake restructured: `pike-bin` (prebuilt, default) and `pike-src` (source build)
- Workflow auto-updates flake.nix with version and binary hashes

### v1.0.1 — March 15: Bug Fixes

- **`--view` lock** — Starting pike with `-v <view>` now locks the TUI to that view. Mode-switching keys (`a`/`t`/`s`/`c`/`1`-`9`) are disabled and Escape cannot unfocus the view.
- **Recently completed escape fix** — Pressing Escape in recently completed mode now returns directly to the dashboard instead of leaving a stale unfiltered view showing all tasks.
- **Escape cleanup** — Escape from any non-dashboard mode now fully resets filter state (previously only cleared `showAll`).

---

## Changelog (by feature)

### Core
- Markdown task scanner with mtime-based caching
- Checkbox tasks (`- [ ]`/`- [x]`) and tagged bullets (`- text @tag`)
- YAML config with views, tag colors, editor, include/exclude patterns
- Config hot-reload on refresh interval

### Query DSL
- Boolean operators: `and`, `or`, `not`, parentheses
- State filters: `open`, `completed`
- Tag filters: `@tag`, with partial matching (`@du` matches `@due`)
- Date comparisons: `@due < today`, `@completed >= today-7d`, `@due = today`
- Relative dates: `today+3d`, `today-7d`
- Regex: `/pattern/`
- Text literals: `"multi word"`, `word`
- CLI mode: `pike -q "open and @due < today"`

### TUI Dashboard
- Configurable multi-section views with colored borders
- Section focus (`1`-`9`), navigation (`j`/`k`/`g`/`G`/Tab/Shift-Tab)
- Half-page scroll (`Ctrl-D`/`Ctrl-U`)
- Summary overlay (`s`) with version, description, and organized keybindings reference
- Scroll windowing for large task lists
- Task count in section headers
- Link prettification (wiki-links, markdown links, bare URLs)

### Filter Modes
- **`/` — Substring mode**: case-insensitive, space-separated tokens (ANDed)
- **`?` — Query mode**: full DSL with parse error display
- Prompt character (`/ ` or `? `) shows active mode
- Enter submits filter and moves focus to results
- Tab toggles focus between filter bar and results
- Two-step Escape: clear text first, then exit
- No cursor highlight in results while typing

### Tag Search (`t`)
- Flow-wrapped tag display with configured colors
- Selected tag in reverse video, non-matching tags faint
- Type to filter (with or without `@` prefix)
- Enter selects tag, shows all matching tasks (including completed)
- Backspace to empty returns to tag picker

### Task Actions
- `x` — Toggle task complete/incomplete in source file (appends `@completed(date)`)
- `H` — Toggle `@hidden` tag on selected task
- `Enter` — Open task in editor at correct line (auto-detects hx/nvim/vim/code)

### Special Tags
- `@due(YYYY-MM-DD)` — due date for comparisons
- `@completed(YYYY-MM-DD)` — completion date
- `@hidden` — hide from views (toggle with `h`)
- `@pin` — float to top of section

### Recently Completed (`c`)
- Shows tasks completed in last N days (configurable)
- Opens in query mode with pre-filled DSL expression
- `x` to un-complete (undo accidental completion)

### Installation
- `go install` / `go build`
- Nix flake: `nix run github:zachthieme/pike` (prebuilt binary) or `nix build .#pike-src` (from source)
- GitHub Releases: prebuilt binaries for linux/darwin x amd64/arm64

---

## Use Cases

**1. Morning triage** — Launch `pike`, scan the dashboard for overdue (red) and today (green) sections. Press `x` to check off anything done. Press `Enter` on a task to jump to its file and add notes.

**2. "What am I forgetting?"** — Press `a` to see all open tasks across every file. Type a keyword to narrow down. Press `Enter` to jump to context.

**3. Tag-based workflows** — Press `t`, type `risk`, Enter to see everything tagged `@risk` including completed items. Or `t` → `delegated` to review delegated work.

**4. Weekly review** — Press `c` to see everything completed in the last 7 days. Press `s` for the summary overlay (open count, overdue count, due this week, completed this week).

**5. Complex queries** — Press `?` and type `open and @due < today and not @hidden` to find overdue non-hidden tasks. Or `open and (@risk or @blocked)` to surface risk items.

**6. Quick text search** — Press `/` and type `deploy api` to find any task mentioning both words, regardless of tags or state.

**7. Hiding noise** — Select a low-priority task, press `H` to tag it `@hidden`. It disappears from all views (lock icon shows hidden count). Press `h` to temporarily reveal hidden tasks.

**8. Pinning priorities** — Add `@pin` to a task in your markdown. It floats to the top of whatever section it appears in.

**9. Undoing mistakes** — Accidentally completed a task? Press `c` to find it in recently completed, navigate to it, press `x` to un-complete. The `@completed(...)` tag is removed and the checkbox is unchecked.

**10. Multi-file note system** — Point pike at `~/notes` containing dozens of markdown files. Pike scans all `**/*.md` files, extracts tasks, and groups them by your configured views. Edit any file in any editor — pike picks up changes on the next refresh (default 5s).
