package main

import (
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
	"pike/internal/tui"
)

var version = "dev"

const usageText = `pike — a terminal task dashboard for markdown notes

Usage:
  pike [flags]

Flags:
  --dir, -d <path>       Notes directory (overrides config/env)
  --config, -c <path>    Config file path
  --view, -v <name>      Start focused on a specific section
  --summary              Print summary counts to stdout and exit
  --query, -q <query>    Run a one-shot query, print results to stdout and exit
  --sort <order>         Sort order for --query mode (default: "file")
  --color                Force color output
  --no-color             Disable color output
  --help, -h             Show help
  --version              Show version
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
	fs.StringVar(&sortFlag, "sort", "file", "Sort order for --query mode")
	fs.BoolVar(&colorFlag, "color", false, "Force color output")
	fs.BoolVar(&noColorFlag, "no-color", false, "Disable color output")
	fs.BoolVar(&helpFlag, "help", false, "Show help")
	fs.BoolVar(&versionFlag, "version", false, "Show version")

	if err := fs.Parse(expanded); err != nil {
		return err
	}

	// Handle --help and --version first (no config/scan needed).
	if helpFlag {
		fmt.Fprint(stdout, usageText)
		return nil
	}
	if versionFlag {
		fmt.Fprintln(stdout, "pike "+version)
		return nil
	}

	// Warn if both --color and --no-color are specified.
	if colorFlag && noColorFlag {
		fmt.Fprintf(stderr, "warning: both --color and --no-color specified; using --no-color\n")
	}

	// Warn if --sort is provided without --query.
	if sortFlag != "file" && queryFlag == "" {
		fmt.Fprintf(stderr, "warning: --sort is only used with --query\n")
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
	sc, err := scanner.New(cfg.NotesDir, cfg.Include, cfg.Exclude)
	if err != nil {
		return fmt.Errorf("invalid glob patterns: %w", err)
	}
	tasks, err := sc.Scan()
	if err != nil {
		return fmt.Errorf("scanning: %w", err)
	}

	now := time.Now()

	// Branch on mode.
	switch {
	case summaryFlag:
		return runSummary(stdout, tasks, now, noColor)
	case queryFlag != "":
		return runQuery(stdout, tasks, queryFlag, sortFlag, cfg.TagColors, now, noColor)
	default:
		return runTUI(stdout, cfg, tasks, sc, viewFlag)
	}
}

// expandShortFlags replaces short flag forms with their long equivalents
// so the standard flag package can parse them.
func expandShortFlags(args []string) []string {
	shortToLong := map[string]string{
		"-d": "--dir",
		"-c": "--config",
		"-v": "--view",
		"-q": "--query",
		"-h": "--help",
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
	fmt.Fprintln(w, out)
	return nil
}

func runQuery(w io.Writer, tasks []model.Task, queryStr, sortOrder string, tagColors map[string]string, now time.Time, noColor bool) error {
	results, err := filter.Apply(tasks, queryStr, sortOrder, now)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}

	var lines []string
	for _, task := range results {
		lines = append(lines, render.FormatTask(task, tagColors, noColor))
	}
	if len(lines) > 0 {
		fmt.Fprintln(w, strings.Join(lines, "\n"))
	}
	return nil
}

func runTUI(_ io.Writer, cfg *config.Config, tasks []model.Task, sc *scanner.Scanner, viewFlag string) error {
	m := tui.NewModel(cfg, tasks, sc.Refresh)

	// If --view flag is set, find and focus that section.
	if viewFlag != "" {
		found := false
		for _, v := range cfg.Views {
			if strings.EqualFold(v.Title, viewFlag) {
				m.SetFocusedView(v.Title)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown view %q", viewFlag)
		}
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
