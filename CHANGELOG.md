# Changelog

## v1.9.0 — April 13, 2026: Task & Bullet Keywords, Implicit AND

**Query DSL keywords:**
- Added `task` keyword — matches checkbox items (`- [ ]` / `- [x]`)
- Added `bullet` keyword — matches plain tagged items (`- item @tag`)
- These let you distinguish todos from tagged notes in queries: `open task not @due`

**Implicit AND:**
- Adjacent atoms are now implicitly ANDed: `open task @risk` is equivalent to `open and task and @risk`
- Explicit `and` still works and is equivalent

## v1.8.1 — March 31, 2026: CI Cleanup

- CI workflow now only runs on version tag pushes (no longer triggers on every push to main or PRs)

## v1.8.0 — March 30, 2026: Configurable Due Dates Export

**Due dates query:**
- `writeDueDates` now filters tasks through `filter.Apply` with a configurable query instead of dumping all tasks with a `@due` tag
- Default query is `"open and @due"` when no view is tagged — only open tasks with due dates are exported

**View config:**
- Added `due_dates: true` field to `ViewConfig` — marks a view as the source query for the `due.json` export (at most one view)
- Added `hidden: true` field to `ViewConfig` — excludes a view from the TUI dashboard while keeping it available for due-dates export, keybinding targets, and `--view` CLI usage
- Config validation rejects multiple views with `due_dates: true`

## v1.7.4 — March 22, 2026: Code Quality & API Surface

**Doc comments:**
- Added Go doc comments to exported types `TaskState`, `Tag`, and `Task` in the model package

**Parser decomposition:**
- Broke `ParseLine` into `matchLine` (regex matching, checkbox detection) and `extractTags` (tag parsing, date normalization, warning generation) — `ParseLine` is now a slim orchestrator

**Interface abstraction:**
- Added `TaskSource` interface to the scanner package (`Scan` + `Refresh`) — enables test doubles and alternative backends without coupling to filesystem scanning

**TUI key handling:**
- Extracted `handleFocusSectionKey` from `handleKeyDashboard` for the 1-9 section focus keys, following the same `(Model, Cmd, bool)` pattern as sibling handlers

**TUI integration tests:**
- Added 11 `View()` integration tests covering dashboard rendering (section headers, task text, markers, footer counts), focused mode, all-tasks mode, summary overlay, recently-completed mode, empty task sets, viewport sizing, and error display

## v1.7.3 — March 22, 2026: Explicit TUI State Machine

**TUI state machine:**
- Added `modeFocused` to `viewMode` enum — section focus is now an explicit mode instead of an implicit `focusedView != ""` check alongside `modeDashboard`
- `View()` dispatch is now a single exhaustive `switch m.mode` with no fallthrough conditionals
- Escape priority chain uses explicit mode matching instead of checking multiple boolean/string fields
- `exitToDashboard()` now resets all state including `focusedView`, making it a complete state reset
- Documented the full state transition diagram in the `Model` doc comment

**Toggle type safety:**
- Replaced `sync.Map` with type assertion (`v.(*sync.Mutex)`) with a typed `fileMutexMap` struct backed by a regular `map[string]*sync.Mutex` — no type assertions, proper Go types throughout

**View rendering clarity:**
- Extracted `taskViewport` struct and `computeTaskViewport()` method to name the windowing parameters (`available`, `maxTasks`) instead of computing them inline

## v1.7.2 — March 22, 2026: Code Quality & Refactoring

**Model invariants:**
- `NewTask()` constructor and `AddTag()` method enforce `LowerText` and `tagSet` invariants at construction time
- Unexported `tagSet` field — callers can no longer mutate the tag index directly, preventing `Tags`/`tagSet` drift
- Added `TaskWith()` constructor for struct-literal style construction with automatic `tagSet` initialization
- Parser uses `NewTask`/`AddTag` instead of manual struct assembly

**CLI refactoring:**
- Extracted `cliFlags` struct with `parseFlags()` and `validate()` methods — `run()` drops from ~170 to ~60 lines
- Extracted `applyScope()` and `runScopedQuery()` helpers for clearer separation of concerns

**Toggle improvements:**
- Encapsulated per-file locks in a `Toggler` struct — `NewToggler()` creates isolated instances for parallel test cases; package-level functions delegate to a default instance
- `writeLines` now uses `os.CreateTemp` for atomic writes instead of a fixed temp filename, preventing clobber between concurrent processes

**TUI improvements:**
- Extracted task actions (`openEditor`, `toggleTask`, `toggleHiddenTag`) from `keys.go` into `actions.go` — `keys.go` is now purely key dispatch
- Deduplicated navigation and task-action keys into shared `handleCursorMovement()` and `handleTaskAction()` helpers
- Extracted `filterTasks()` helper to deduplicate the filter application pattern between single-section and dashboard rebuilds
- `applySubstringFilter` now uses pre-computed `task.LowerText` instead of recomputing `strings.ToLower` on every keypress
- Custom key bindings now use O(1) map lookup instead of O(n) loop per keypress
- Custom binding `sort` field now applies the requested sort order
- Model struct fields organized into documented logical groups

**Query DSL:**
- Improved lexer error messages: unterminated strings/regexes mention the missing delimiter, date errors include expected format, offset errors show examples

**Documentation:**
- Tag names are case-sensitive — documented in README and query DSL reference

## v1.7.1 — March 19, 2026: Cleaner CLI Output

- Removed `file:line` prefix from `--query` and `--scope` output — tasks now render as plain checkbox lines (e.g. `- [ ] Buy groceries @today`)

## v1.7.0 — March 18, 2026: File-Scoped Task Queries

- `--scope <file>` flag: filter open tasks to those referencing the given file (by wiki-link, filename, or display name). Composes with `--query`, `--view`, `--json`, `--count`, and `--sort`.

### Collected changes — September 14, 2026

**Bug fixes:**
- [user] `--sync` now verifies every note line it writes still holds exactly the task that was scanned, and writes nothing otherwise. A line completed, retitled, rescheduled, replaced by another task, or shifted by an edit between the scan and the write is skipped with a warning naming the file, line, and HEY id, and counted as a failure — so Sync no longer completes the wrong task or overwrites another task's `@hey` link when the notes are open in an editor or another Sync (#13).
- [user] Sync's `@due` and `@hey` tag rewrites now respect tag boundaries: `@duedate` is never mistaken for `@due`, and `@heybot` alongside `@hey(1)` reads as a single link (#13).
- [user] The dashboard's sync key (`S`) now scans the notes before reconciling, so a link written by a cron `pike --sync` since the last refresh is seen and the task is not pushed to HEY again (#13).
- [user] HEY Sync: a Todo renamed in HEY no longer loses that rename when the notes also move its week in the same sync — the re-created Todo now carries HEY's title, so HEY, the notes, and state agree and the next sync makes no calls (#15).
- [user] HEY Sync: with no state file and a title mismatch on an open Todo, the notes now win and the Todo is re-created, instead of the two sides staying permanently out of step (#15).
- [user] HEY Sync: the run report labels its failure count "item(s) failed" instead of "task(s) failed to push", since it counts completion, reschedule, retitle, re-create, and unlink failures too — not only pushes (#15).
- [user] HEY Sync: a Todo imported from HEY whose title contains "@" text — an email address like `bob@example.com`, a `@rent`, or an `@due(2026-01-01)` — no longer plants a stray tag on the inbox line that the next Sync misreads as a title change and re-creates with a mangled title. The title is written so the line parses with only pike's own `@due` and `@hey` tags, and round-trips unchanged (#16).
- [user] HEY Sync: when Sync adds a Todo but the `@hey` tag write is refused (a line edited since the scan), it now deletes the Todo it just added instead of leaving it behind for the next Sync to import as a second copy of the task. If that rollback delete also fails, the Todo is tracked so it is never imported and the next Sync retries the delete (#16).
- [user] HEY Sync: a Re-create whose delete of the replaced Todo fails now keeps the old Todo tracked as superseded, so it is never imported as a second copy of the task; the next Sync retries the delete and drops the record once the Todo is gone from HEY (#16).
- [user] HEY Sync: a task line carrying more than one `@hey` tag is now skipped by every pass with a single warning per sync — before, only the first id was used (silently completing or re-creating its Todo) while the rest were imported as duplicate tasks. Every id on the line now counts as linked, so none is imported, and a dry run reports the same (#16).
- [user] HEY Sync: the "orphaned link" warning (a linked task's line gone from the notes while its Todo survives in HEY — including after you delete an `@hey` tag by hand) now appears only on the first sync that finds it, instead of repeating on every sync. The orphan is still counted each sync, and the tracking entry is dropped once its Todo is removed from HEY or completed (#16).
- [user] TUI: auto-completing a Linked parent no longer pushes it to HEY or marks it completed in state when writing the parent line failed (e.g. a stale line); only the parent whose write succeeded is pushed (#17).
- [user] TUI: the Sync status line now lists every non-zero outcome — including retitles, reschedules, re-creations, unlinks, and orphans — reads "sync: up to date" when nothing changed, and surfaces the first warning's text with a count of any others, instead of only showing pushed/imported/completed/uncompleted (#17).
- [user] TUI: adding, removing, or editing the `hey:` config block while the dashboard is running now takes effect on config reload — the HEY client and sync settings are rebuilt, so `S` and Push follow the new command and account instead of the values from startup (#17).
- [user] Hiding a bare `@hey` Link tag no longer mangles a longer tag on the same line: `Ping @heyday then @hey` now renders as `Ping @heyday then` with `@heyday` intact and no doubled spaces (#17).

**Documentation:**
- [user] HEY Sync docs now match what ships after the sync fixes: the README explains that deleting the state file (or starting with none) is a safe reset because the default state directory is recreated automatically, that un-linking a task by hand leaves the old todo as an orphan reported once until you remove it from HEY, that an imported title containing `@` text is written so it plants no stray tag, and that a failed `@hey` tag write rolls back the todo pike just added; `CONTEXT.md` redefines **Orphan** as a surviving todo whose task line is gone and adds **Un-link** for a todo already gone from HEY (#18).

**Fixed:**
- [user] `pike --sync` now works against the real `hey` CLI (hey-cli 1.4.1): the adapter parses hey's actual todo shape (numeric `id`, `starts_at`/`ends_at` week boundaries, `completed_at`), where before every sync failed immediately on the numeric id. Authentication failures (`code: auth` or exit status 3) now surface HEY's own message and hint and exit non-zero, and the default state file's parent directory (`~/.local/share/pike`) is created on a fresh machine (#14).
- [user] A `hey:` config key with all its children commented out (loading as YAML null) now enables HEY sync with defaults, the same as `hey: {}` (#14).

### Collected changes — September 13, 2026

**Features:**
- [user] `@hey(id)` Link tags are now hidden in the TUI and `--query` text output so nine-digit HEY ids never clutter the dashboard; the tag is still present in `--json` output for scripting (#3).

**Added:**
- [user] `--sync --dry-run` previews a HEY sync: with a `hey:` config block it reports how many eligible tasks would push, how many unlinked open todos would import, and how many links already exist — writing nothing (#4).
- [user] `hey:` config block (`command`, `query`, `account`, `state_path`) enabling HEY sync; absent, all existing behaviour is unchanged (#4).
- [user] `--sync` (without `--dry-run`) now performs a real HEY sync: it creates a Todo for every Eligible Task matching the Sync Query, appends an `@hey(id)` Link tag to the task's line, and records the link in the state file. Tasks with `@due` push on that date; the rest land in HEY's current week. The report counts tasks pushed and failures, per-task failures don't stop the run, and a second sync with no changes pushes nothing (#5).
- [user] `--sync` (without `--dry-run`) now imports HEY-born todos into the inbox: every open Todo in HEY that no task is linked to becomes a new inbox line `- [ ] <title> @due(<saturday>) @hey(<id>)`, where the due date is the Saturday ending the Todo's week (taken verbatim from HEY, no zone conversion). Completed unlinked todos are never imported, the inbox file (default `inbox.md`) is created if absent, and because imported lines carry an `@hey` link a second sync imports nothing. The report counts todos imported, and `--dry-run` reports the count without writing (#6).
- [user] `--sync` now reconciles completion in both directions: completing or uncompleting a task in the notes or its linked HEY todo reaches the other side on the next sync, using the state snapshot so a one-sided change propagates (including uncomplete) and a genuine conflict resolves in favour of completed. The report counts items completed and uncompleted, and `--dry-run` previews these counts without writing (#7).
- [user] In the TUI, completing or uncompleting a Linked Task now Pushes the change to HEY immediately — the toggled Task's Todo and any Linked parent the cascade auto-completed are completed/uncompleted, and the sync state is updated so the next Sync does not resend it (#11).
- [user] New `sync` keybinding (default `S`, rebindable, listed in the help view) runs a full HEY Sync from the TUI, refreshes the task list, and shows a one-line result or error in the status line; with no `hey:` block it does nothing (#11).
- [user] `--sync` now never deletes on either side (per ADR 0002). When a linked task's HEY todo has vanished from the list, its `@hey` tag is stripped and the link is dropped (an unlink); deleting the `@hey` tag by hand stays the supported way to un-link. When a link's task line can no longer be found in the notes, the surviving todo is left in place and reported as an orphan with a warning naming its id and title. A line carrying two `@hey` tags is skipped with a warning rather than stripped. The report counts links unlinked and orphans reported, and `--dry-run` previews both without writing (#8).
- [user] `--sync` now reconciles title changes in both directions. When a linked todo was renamed in HEY since the last sync, its task's text is rewritten to the new title with its `@tag`s preserved and re-appended in their original order, leaving the checkbox untouched. When the task's text was renamed in the notes (or on both sides, where the notes win), the open todo is re-created — a fresh todo is added, the task's `@hey` tag is rewritten to the new id, and the old todo is deleted, in that order so a failure part-way never loses the todo. Completed linked todos are never retitled or re-created. The report counts tasks retitled and todos re-created, and `--dry-run` previews both without writing (#9).
- [user] `--sync` now reconciles week (schedule) changes in both directions, respecting that HEY only knows weeks. When a linked todo's week moved in HEY and no longer contains the task's `@due` day, `@due` is set to that week's Saturday; a `@due` that still falls inside the todo's week is left untouched, so a mid-week deadline is never flattened to Saturday. When the notes moved `@due` into a different week, the open todo is re-created with the new date, sharing a single re-create with any title change (so a task renamed and rescheduled at once is one add and one delete). Tasks with no `@due` are never compared, and completed todos are never rescheduled. The report counts items rescheduled, and `--dry-run` previews them without writing (#10).
- [user] HEY sync: pike can now mirror tasks to and from HEY's to-do calendar. Add a `hey:` block to your config (`command`, `query`, `account`, `state_path`) and run `pike --sync` (or press `S` in the TUI) to reconcile. Eligible tasks matching the Sync Query are pushed to HEY as new todos and linked with an `@hey(id)` tag; unlinked open todos are imported into the inbox; and completion, title, and schedule changes on either side are carried to the other. Sync never deletes on either side — a vanished todo strips the task's `@hey` tag, a vanished task line is reported as an orphan. `--dry-run` previews a sync without writing, `--json` emits a machine-readable report, and the state file at `state_path` can be safely deleted to reset (links live in the `@hey` tags, so nothing is lost). Sync stays disabled until a `hey:` block is present (#12).

**Changed:**
- [user] `--sync` conflicts with `--summary`, `--query`, `--scope`, and `--view`; `--dry-run` without `--sync` warns; `--sync --json` emits the report as JSON (#4).

### v1.6.1 — March 17, 2026: Idiomatic Go & Bubble Tea Cleanup

**Toggle file permissions:**
- `writeLines` now preserves original file permissions instead of hardcoding `0644`
- All toggle operations (`Complete`, `Uncomplete`, `ToggleHidden`) stat the file before writing

**TUI cleanup:**
- Replaced `sync.Map` style caches with plain maps (Bubble Tea is single-threaded)
- Changed `filterPrompt` from map to array (indexed by iota enum)
- Made `NewModel` `configFunc` parameter explicit instead of variadic
- Removed unused `Init()` methods from `FilterBar` and `TagSearch` sub-models
- Added `String()` methods to `viewMode` and `filterMode` enums

### v1.6.0 — March 17, 2026: Code Quality Improvements

**TUI decomposition:**
- Extracted `Navigator` type for cursor/section navigation, removing 9 functions from the monolithic `tasks.go`
- Split `tasks.go` (446 lines) into `modes.go` (mode transitions, section rebuilding) and `tasks.go` (utilities)
- Navigator accepts sections as parameters to avoid stale closure bugs with Bubble Tea's value semantics

**context.Context threading:**
- `toggle.Complete`, `toggle.Uncomplete`, and `toggle.ToggleHidden` now accept `context.Context` as their first parameter
- Context checked before acquiring file lock and before atomic write, enabling cancellation of in-flight toggles

**Testing:**
- 9 integration tests exercising the full scan → parse → filter → render pipeline with real files
- 8 new CLI tests covering color mode flags, config/scanner errors, query validation, and warning output
- 12 Navigator unit tests covering cursor movement, section jumping, page scrolling, and edge cases
- 3 toggle cancellation tests verifying cancelled contexts don't modify files

**Documentation:**
- Package-level godoc comments added to all 13 packages

### v1.5.0 — March 16, 2026: Custom Keybindings

**Configurable keybindings:**
- Remap any built-in TUI action via `keybindings:` section in `config.yaml`
- Replacement model: listed keys are the complete set (e.g., `toggle: ["space", "x"]`)
- Empty keys list disables an action (except `escape`, which cannot be disabled)

**Custom shortcuts:**
- Bind any key to focus a dashboard view by title (e.g., `o` → focus "Overdue")
- Bind any key to run a query in all-tasks mode (e.g., `d` → `open and @due < today+3d`)
- Custom shortcuts replace the default 1-9 positional focus keys when defined
- Custom shortcuts take priority over built-in keys on conflict

**Summary overlay:**
- The `s` help overlay now renders from actual configured keybindings instead of hardcoded defaults
- Custom shortcuts shown in a dedicated "Shortcuts" section
- Disabled bindings are omitted

### v1.4.0 — March 16, 2026: Code Quality & Tooling

**Lenient date parsing:**
- `@due(2026/3/16)`, `@due(2026.03.16)`, `@due(2026-3-6)` now normalize to `YYYY-MM-DD` instead of being silently dropped
- Parse warnings for genuinely invalid dates (e.g., `@due(march-16)`) surfaced via stderr in CLI modes and a warning count in the TUI footer

**Developer tooling:**
- `Makefile` with targets: build, test, lint, bench, fuzz, cover, golden-update, install
- `golangci-lint` config (errcheck, govet, staticcheck, gocritic, unused, ineffassign) — all existing violations fixed
- CI pipeline: test (with `-race`), lint, and fuzz jobs run on every push to main and PR
- Scanner benchmarks for Scan/Refresh at 100/500/1000 files

**Testing:**
- Fuzz targets for query DSL parser/evaluator and markdown line parser
- 24 new TUI tests covering state transitions, key handling edge cases, and sub-model gaps
- TUI test coverage: 65.7% → 70.7%

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
