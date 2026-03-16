# Changelog

### v1.3.0 — March 16, 2026: TUI Sub-Model Decomposition

**Architecture:**
- Extracted `FilterBar` and `TagSearch` into Bubble Tea sub-models with message-passing architecture
- Filter bar owns text input widget, mode switching, and activation/deactivation lifecycle
- Tag search owns tag list, cursor, filtering, and flow-wrapped rendering with its own text input
- Main Model delegates via `processFilterOutput` for inline state settlement (no async round-trips)
- New `messages.go` consolidates all message types and shared type definitions

**Quality & hardening:**
- Magic numbers extracted to named constants across scanner, style, TUI, and render packages
- Benchmarks added for parser, query eval, style colorization, and TUI hot paths
- Error handling: explicitly discarded intentionally-ignored errors in config and toggle
- TOCTOU hardening in toggle: re-reads file and verifies line unchanged before writing
- Cancellable context for scan goroutines — in-flight scans stop when TUI exits
- `DefaultKeyMap()` cached in sub-models instead of allocating per-keystroke
- Redundant `text` field removed from FilterBar (derived from `input.Value()`)

**Behavior changes:**
- `q` returns to dashboard from non-dashboard views (tag search, all tasks, recently completed) instead of quitting the program. `q` on the dashboard still quits.
- `j`/`k` type into the filter input when it's focused (previously they moved the cursor). Arrow keys still navigate.
- Background scan in tag search mode preserves cursor position and filter text (was resetting both on every refresh)

### v1.2.0 — March 15, 2026: Catppuccin Mocha & First-Run Config

- Catppuccin Mocha color scheme as default
- Auto-create config file with sensible defaults on first run

### v1.1.1 — March 15, 2026: CLI, DSL & Bubbletea Best Practices

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

### v1.1.0 — March 15, 2026: Architecture & Release Automation

**Architecture improvements:**
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

### v1.0.1 — March 15, 2026: Bug Fixes

- **`--view` lock** — Starting pike with `-w <view>` now locks the TUI to that view. Mode-switching keys (`a`/`t`/`s`/`c`/`1`-`9`) are disabled and Escape cannot unfocus the view.
- **Recently completed escape fix** — Pressing Escape in recently completed mode now returns directly to the dashboard instead of leaving a stale unfiltered view showing all tasks.
- **Escape cleanup** — Escape from any non-dashboard mode now fully resets filter state (previously only cleared `showAll`).

### v1.0.0 — March 13-14, 2026: Initial Release

The entire core built over two days: markdown parser, task model, query DSL (lexer, recursive-descent parser, evaluator), configurable dashboard views, tag coloring, link prettification, and the Bubble Tea TUI.

**Features at launch:**
- Checkbox (`- [ ]`/`- [x]`) and tagged bullet (`- text @tag`) extraction from markdown files
- Query DSL with boolean operators, date comparisons, regex, text matching, partial tag matching
- Configurable dashboard sections via YAML (`views:` with `query`, `sort`, `color`)
- Tag search mode with flow-wrapped tag picker
- All-tasks and recently-completed views
- Task toggling (`x`) — complete/uncomplete directly in source files
- `@pin` tag for floating tasks to section tops
- `@hidden` tag with `h`/`H` toggle visibility
- Editor integration with line-number support (hx, nvim, vim, code)
- File scanning with mtime-based incremental refresh
- Golden file test suite

---

For feature reference, see [README](README.md). For query DSL details, see [docs/query-dsl.md](docs/query-dsl.md).
