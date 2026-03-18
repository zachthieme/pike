// Package main provides the pike CLI, a terminal task dashboard that reads markdown files.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"

	tea "github.com/charmbracelet/bubbletea"

	"pike/internal/config"
	"pike/internal/filter"
	"pike/internal/model"
	"pike/internal/render"
	"pike/internal/scanner"
	"pike/internal/scope"
	"pike/internal/tui"
)

var version = "dev"

const usageText = `pike — a terminal task dashboard for markdown notes

Usage:
  pike [flags]

Flags:
  --dir, -d <path>       Notes directory (overrides config/env)
  --scope, -s <path>     Scope to tasks referencing this file
  --config, -c <path>    Config file path
  --view, -w <name>      Start focused on a specific section
  --summary              Print summary counts to stdout and exit
  --query, -q <query>    Run a one-shot query, print results to stdout and exit
  --sort <order>         Sort order for --query/--scope mode (default: "file")
  --count                Print result count only (use with --query or --scope)
  --json                 Output results as JSON (use with --query or --scope)
  --color                Force color output
  --no-color             Disable color output
  --help, -h             Show help
  --version, -v          Show version
`

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "pike:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	// Expand short flags to long forms before parsing.
	expanded := expandShortFlags(args)

	fs := flag.NewFlagSet("pike", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		dirFlag     string
		configFlag  string
		viewFlag    string
		summaryFlag bool
		queryFlag   string
		sortFlag    string
		scopeFlag   string
		countFlag   bool
		jsonFlag    bool
		colorFlag   bool
		noColorFlag bool
		helpFlag    bool
		versionFlag bool
	)

	fs.StringVar(&dirFlag, "dir", "", "Notes directory")
	fs.StringVar(&configFlag, "config", "", "Config file path")
	fs.StringVar(&viewFlag, "view", "", "Start focused on a specific section")
	fs.BoolVar(&summaryFlag, "summary", false, "Print summary counts")
	fs.StringVar(&queryFlag, "query", "", "Run a one-shot query")
	fs.StringVar(&sortFlag, "sort", "file", "Sort order for --query/--scope mode")
	fs.StringVar(&scopeFlag, "scope", "", "Scope to tasks referencing this file")
	fs.BoolVar(&countFlag, "count", false, "Print result count only")
	fs.BoolVar(&jsonFlag, "json", false, "Output results as JSON")
	fs.BoolVar(&colorFlag, "color", false, "Force color output")
	fs.BoolVar(&noColorFlag, "no-color", false, "Disable color output")
	fs.BoolVar(&helpFlag, "help", false, "Show help")
	fs.BoolVar(&versionFlag, "version", false, "Show version")

	if err := fs.Parse(expanded); err != nil {
		return err
	}

	// Handle --help and --version first (no config/scan needed).
	if helpFlag {
		_, _ = fmt.Fprint(stdout, usageText)
		return nil
	}
	if versionFlag {
		_, _ = fmt.Fprintln(stdout, "pike "+version)
		return nil
	}

	// Warn if both --color and --no-color are specified.
	if colorFlag && noColorFlag {
		_, _ = fmt.Fprintf(stderr, "warning: both --color and --no-color specified; using --no-color\n")
	}

	// Warn if query-only flags are provided without --query.
	if sortFlag != "file" && queryFlag == "" && scopeFlag == "" {
		_, _ = fmt.Fprintf(stderr, "warning: --sort is only used with --query or --scope\n")
	}
	if countFlag && queryFlag == "" && scopeFlag == "" {
		_, _ = fmt.Fprintf(stderr, "warning: --count is only used with --query or --scope\n")
	}
	if jsonFlag && queryFlag == "" && scopeFlag == "" {
		_, _ = fmt.Fprintf(stderr, "warning: --json is only used with --query or --scope\n")
	}

	// Validate --scope flag.
	if scopeFlag != "" && summaryFlag {
		return fmt.Errorf("--scope and --summary cannot be combined")
	}
	if scopeFlag != "" && viewFlag != "" && queryFlag != "" {
		return fmt.Errorf("--view and --query cannot both be used with --scope")
	}

	// Determine color mode.
	noColor := resolveColorMode(colorFlag, noColorFlag, stdout)

	// Load config.
	cfg, err := config.Load(configFlag)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Resolve notes directory: --dir flag > $NOTES env > config.
	notesDir := resolveNotesDir(dirFlag, cfg.NotesDir)
	if notesDir == "" {
		return fmt.Errorf("notes directory not set (use --dir, $NOTES, or notes_dir in config)")
	}
	cfg.NotesDir = notesDir

	// Scan files.
	ctx := context.Background()
	sc, err := scanner.New(cfg.NotesDir, cfg.Include, cfg.Exclude)
	if err != nil {
		return fmt.Errorf("invalid glob patterns: %w", err)
	}
	tasks, err := sc.Scan(ctx)
	if err != nil {
		return fmt.Errorf("scanning: %w", err)
	}
	for _, w := range sc.Warnings {
		_, _ = fmt.Fprintf(stderr, "warning: %s:%d: %s\n", w.File, w.Line, w.Message)
	}

	// Apply scope filter if --scope is set.
	if scopeFlag != "" {
		info, err := os.Stat(scopeFlag)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("scope: file not found: %s", scopeFlag)
			}
			return fmt.Errorf("scope: %w", err)
		}
		if info.IsDir() {
			return fmt.Errorf("scope: expected a file, got directory: %s", scopeFlag)
		}

		identities := scope.Identity(scopeFlag)
		excludePath, err := scope.RelPath(scopeFlag, cfg.NotesDir)
		if err != nil {
			return fmt.Errorf("scope: %w", err)
		}
		tasks = scope.Filter(tasks, identities, excludePath)
	}

	now := time.Now()

	// Branch on mode.
	switch {
	case summaryFlag:
		return runSummary(stdout, tasks, now, noColor)
	case scopeFlag != "":
		// Resolve scope query: --view extracts the view's query+sort,
		// --query uses the given query, otherwise default to "open".
		scopeQuery := "open"
		scopeSort := sortFlag
		if viewFlag != "" {
			v, err := findView(cfg.Views, viewFlag)
			if err != nil {
				return err
			}
			scopeQuery = v.Query
			if v.Sort != "" {
				scopeSort = v.Sort
			}
		} else if queryFlag != "" {
			scopeQuery = queryFlag
		}
		return runQuery(stdout, tasks, scopeQuery, queryOpts{
			sortOrder:  scopeSort,
			tagColors:  cfg.TagColors,
			now:        now,
			noColor:    noColor,
			count:      countFlag,
			jsonOutput: jsonFlag,
		})
	case queryFlag != "":
		return runQuery(stdout, tasks, queryFlag, queryOpts{
			sortOrder:  sortFlag,
			tagColors:  cfg.TagColors,
			now:        now,
			noColor:    noColor,
			count:      countFlag,
			jsonOutput: jsonFlag,
		})
	default:
		return runTUI(stdout, cfg, tasks, sc, viewFlag, configFlag)
	}
}

// expandShortFlags replaces short flag forms with their long equivalents
// so the standard flag package can parse them.
func expandShortFlags(args []string) []string {
	shortToLong := map[string]string{
		"-d": "--dir",
		"-c": "--config",
		"-w": "--view",
		"-q": "--query",
		"-s": "--scope",
		"-h": "--help",
		"-v": "--version",
	}

	out := make([]string, 0, len(args))
	for _, arg := range args {
		if long, ok := shortToLong[arg]; ok {
			out = append(out, long)
		} else {
			out = append(out, arg)
		}
	}
	return out
}

// resolveColorMode determines whether color output should be disabled.
// Returns true if color should be disabled (noColor).
func resolveColorMode(forceColor, forceNoColor bool, w io.Writer) bool {
	if forceNoColor {
		return true
	}
	if forceColor {
		return false
	}
	// Auto-detect: enable color if stdout is a TTY.
	if f, ok := w.(*os.File); ok {
		return !isatty.IsTerminal(f.Fd())
	}
	// Not a real file (e.g., bytes.Buffer in tests) — no color.
	return true
}

// resolveNotesDir picks the notes directory from flag, env, or config.
func resolveNotesDir(flagDir, configDir string) string {
	if flagDir != "" {
		return flagDir
	}
	if env := os.Getenv("NOTES"); env != "" {
		return env
	}
	return configDir
}

func runSummary(w io.Writer, tasks []model.Task, now time.Time, noColor bool) error {
	openTasks, err := filter.Apply(tasks, "open", "", now)
	if err != nil {
		return fmt.Errorf("summary open: %w", err)
	}

	overdueTasks, err := filter.Apply(tasks, "open and @due < today", "", now)
	if err != nil {
		return fmt.Errorf("summary overdue: %w", err)
	}

	dueThisWeekTasks, err := filter.Apply(tasks, "open and @due >= today and @due <= today+7d", "", now)
	if err != nil {
		return fmt.Errorf("summary due this week: %w", err)
	}

	completedThisWeekTasks, err := filter.Apply(tasks, "completed and @completed >= today-7d", "", now)
	if err != nil {
		return fmt.Errorf("summary completed this week: %w", err)
	}

	out := render.FormatSummary(
		len(openTasks),
		len(overdueTasks),
		len(dueThisWeekTasks),
		len(completedThisWeekTasks),
		noColor,
	)
	_, err = fmt.Fprintln(w, out)
	return err
}

// queryOpts groups output-mode options for runQuery.
type queryOpts struct {
	sortOrder  string
	tagColors  map[string]string
	now        time.Time
	noColor    bool
	count      bool
	jsonOutput bool
}

func runQuery(w io.Writer, tasks []model.Task, queryStr string, opts queryOpts) error {
	results, err := filter.Apply(tasks, queryStr, opts.sortOrder, opts.now)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}

	if opts.count {
		_, err = fmt.Fprintln(w, len(results))
		return err
	}

	if opts.jsonOutput {
		return render.FormatJSON(w, results)
	}

	var lines []string
	for _, task := range results {
		lines = append(lines, render.FormatTask(task, opts.tagColors, opts.noColor))
	}
	if len(lines) > 0 {
		_, err = fmt.Fprintln(w, strings.Join(lines, "\n"))
		return err
	}
	return nil
}

// findView returns the ViewConfig matching the given title (case-insensitive).
func findView(views []config.ViewConfig, title string) (*config.ViewConfig, error) {
	for i := range views {
		if strings.EqualFold(views[i].Title, title) {
			return &views[i], nil
		}
	}
	return nil, fmt.Errorf("unknown view %q", title)
}

func runTUI(_ io.Writer, cfg *config.Config, tasks []model.Task, sc *scanner.Scanner, viewFlag string, configPath string) error {
	// Create a cancellable context so in-flight scan goroutines stop when the TUI exits.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	configReload := func() (*config.Config, error) {
		return config.Load(configPath)
	}
	scanRefresh := func() ([]model.Task, error) {
		return sc.Refresh(ctx)
	}
	warningsGetter := func() []model.Warning {
		return sc.Warnings
	}
	m := tui.NewModel(cfg, tasks, scanRefresh, configReload)
	m.SetVersion(version)
	m.SetWarnings(sc.Warnings)
	m.SetWarningsFunc(warningsGetter)

	// If --view flag is set, find and focus that section.
	if viewFlag != "" {
		v, err := findView(cfg.Views, viewFlag)
		if err != nil {
			return err
		}
		m.SetFocusedView(v.Title)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
