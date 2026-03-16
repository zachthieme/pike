# CLAUDE.md — Pike Development Guide

## What This Project Is

Pike is a terminal task dashboard that reads markdown files for checkbox/tagged items and displays them in an interactive TUI. Written in Go 1.25.x using Bubble Tea.

## Project Structure

```
cmd/pike/          CLI entry point, flag parsing, mode routing
internal/
  model/           Task, Tag, Warning types
  parser/          Regex-based markdown line parser
  query/           Query DSL (lexer → parser → AST → evaluator)
  scanner/         File walker with mtime-based caching
  filter/          Query + sort pipeline
  sort/            Sort strategies and pin partitioning
  toggle/          Atomic file writes for task completion
  render/          stdout and JSON formatting
  style/           Tag coloring, link prettification
  editor/          Editor command construction
  config/          YAML config loading
  tui/             Bubble Tea TUI (model, views, sub-models)
```

## How to Build, Test, Lint

```
make build          # go build -o pike ./cmd/pike
make test           # go test -race -count=1 ./...
make lint           # golangci-lint run
make bench          # go test -bench=. -benchmem ./...
make fuzz           # 30s fuzz runs on parser + query
make cover          # coverage report (opens in browser)
make golden-update  # update golden test fixtures
make install        # go install ./cmd/pike
```

Always run `make test` and `make lint` before committing. CI enforces both.

## Branching and Merge Strategy

**Small, self-contained changes** (bug fixes, config tweaks, docs, simple features touching 1-3 files): commit directly to `main` and push. CI runs on push.

**Larger features or risky changes** (new subsystems, refactors touching many files, changes to the query DSL or parser, anything that could break existing behavior): create a feature branch, open a PR against `main`. CI runs on the PR. Merge after tests pass.

**Rule of thumb:** if you'd want someone to review it before it ships, use a PR. If it's obviously correct, push to main.

## How to Release

Releases are fully automated via GoReleaser + GitHub Actions.

1. Update `CHANGELOG.md` with the new version's changes
2. Tag the commit: `git tag v1.X.0`
3. Push the tag: `git push origin v1.X.0`
4. GitHub Actions builds binaries for linux/darwin (amd64/arm64), creates a GitHub Release, and auto-updates `flake.nix` with new SRI hashes

Do not manually edit `flake.nix` version or hashes — the release workflow handles this.

## Code Conventions

- **Error handling:** wrap with context using `fmt.Errorf("operation: %w", err)`. Use sentinel errors (like `toggle.ErrStaleData`) for programmatic handling.
- **Testing:** table-driven tests, golden file tests for output regression, benchmarks on hot paths. Use `t.TempDir()` for filesystem isolation. Inject `time.Now` for deterministic tests.
- **Dependencies:** keep them minimal. The Charm ecosystem (bubbletea/bubbles/lipgloss) for TUI, doublestar for globs, yaml.v3 for config. Don't add libraries for things the stdlib handles.
- **Linting:** `.golangci.yml` enforces errcheck, govet, staticcheck, unused, ineffassign, gocritic. Use `//nolint:linter // reason` when suppressing — always include the reason.
- **TUI patterns:** sub-models (FilterBar, TagSearch) communicate via messages. `Update()` dispatches to `handleKey()`. `processFilterOutput()` bridges FilterBar actions to model state.

## What Not to Do

- Don't edit `flake.nix` manually for releases — the CI workflow handles version and hash updates.
- Don't skip `make lint` — CI will catch it anyway and the push will be red.
- Don't add dependencies without justification. Check if the stdlib or existing deps can do it.
- Don't change `ParseLine` or query DSL grammar without updating the fuzz tests and golden files.
