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

### v1.1.1 — March 15: CLI, DSL & Bubbletea Best Practices

**New features:**
- `tomorrow` and `yesterday` DSL date keywords (`@due < tomorrow`)
- `--count` flag: print result count only (`pike -q "open" --count`)
- `--json` flag: output results as JSON array (`pike -q "open" --json`)
- `-v` reassigned to `--version` (Unix convention), `-w` for `--view`
- Context-aware footer bars: dashboard shows `○ 12/42  ● 5 wk`, other views show result count
- `week_start_day` config option (0=Sunday through 6=Saturday)

**Bubbletea best practices:**
- Async I/O: file toggles and scan/config reload wrapped in `tea.Cmd` (non-blocking Update)
- Cached lipgloss styles at package level with `sync.Map` for parameterized styles
- All key bindings routed through KeyMap (`PageDown`/`PageUp`, `Ctrl+N`/`Ctrl+P` added)
- No more raw `msg.Type` checks in key handlers

**Fixes:**
- Dashboard footer counts only open tasks in displayed sections (was mixing open + completed)
- `completedThisWeek` gated on `t.State == model.Completed`
- `viewFocused` shows QueryErr even when result count is 0
- Config-only scan result now triggers section rebuild (prevents stale display)
- Hidden icon changed to `○`/`◉` toggle pair for better visibility (default color 245)

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
- `○`/`◉` icons replace `🔒` on section headers for hidden task visibility
- `○` (configurable color) shown when hidden tasks are concealed
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

For feature reference, see [README.md](../README.md). For query DSL details, see [QUERY_DSL.md](../QUERY_DSL.md).
