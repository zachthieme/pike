// Package config loads and validates YAML configuration with sensible defaults.
package config

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	tasksort "github.com/zachthieme/pike/internal/sort"
)

// Config holds all application configuration.
type Config struct {
	NotesDir              string              `yaml:"-"`
	Include               []string            `yaml:"-"`
	Exclude               []string            `yaml:"-"`
	RefreshInterval       time.Duration       `yaml:"-"`
	Editor                string              `yaml:"-"`
	TagColors             map[string]string   `yaml:"-"`
	LinkColor             string              `yaml:"-"`
	HiddenColor           string              `yaml:"-"` // color for ◌ icon (hidden tasks concealed)
	VisibleColor          string              `yaml:"-"` // color for ◉ icon (hidden tasks revealed)
	WeekStartDay          int                 `yaml:"-"` // 0=Sunday, 1=Monday, ..., 6=Saturday
	RecentlyCompletedDays int                 `yaml:"-"`
	DueDatesPath          string              `yaml:"-"` // path to write due dates JSON for wen integration
	InboxFile             string              `yaml:"-"` // file to append new tasks to (relative to NotesDir)
	Views                 []ViewConfig        `yaml:"-"`
	Keybindings           map[string][]string `yaml:"-"`
	CustomBindings        []CustomBinding     `yaml:"-"`
	Hey                   *HeyConfig          `yaml:"-"` // HEY sync settings; nil when no hey: block is configured
}

// HeyConfig holds settings for syncing tasks with HEY. Its presence (a non-nil
// pointer) is what enables --sync; an absent hey: block leaves it nil.
type HeyConfig struct {
	Command   string // hey CLI command to run (default "hey")
	Query     string // Sync Query selecting Eligible Tasks to push (default "@due or @today")
	Account   string // --account passed to hey when non-empty
	StatePath string // path to the sync state file
}

// ViewConfig defines a single dashboard section.
type ViewConfig struct {
	Title    string `yaml:"title"`
	Query    string `yaml:"query"`
	Sort     string `yaml:"sort"`
	Color    string `yaml:"color"`
	Order    int    `yaml:"order"`
	DueDates bool   `yaml:"due_dates"`
	Hidden   bool   `yaml:"hidden"`
}

// CustomBinding defines a user-configured key shortcut.
type CustomBinding struct {
	Key   string
	View  string // non-empty = focus this view by title
	Query string // non-empty = run this query in all-tasks mode
	Sort  string // sort order for query mode (default "file")
}

type rawKeybindings struct {
	Actions map[string]interface{} `yaml:",inline"`
	Custom  []rawCustomBinding     `yaml:"custom"`
}

type rawCustomBinding struct {
	Key   string `yaml:"key"`
	View  string `yaml:"view"`
	Query string `yaml:"query"`
	Sort  string `yaml:"sort"`
}

// rawConfig mirrors the YAML structure for unmarshalling.
type rawConfig struct {
	NotesDir              string            `yaml:"notes_dir"`
	Include               []string          `yaml:"include"`
	Exclude               []string          `yaml:"exclude"`
	RefreshInterval       string            `yaml:"refresh_interval"`
	Editor                string            `yaml:"editor"`
	TagColors             map[string]string `yaml:"tag_colors"`
	LinkColor             string            `yaml:"link_color"`
	HiddenColor           string            `yaml:"hidden_color"`
	VisibleColor          string            `yaml:"visible_color"`
	WeekStartDay          *int              `yaml:"week_start_day"`
	RecentlyCompletedDays *int              `yaml:"recently_completed_days"`
	DueDatesPath          string            `yaml:"due_dates_path"`
	InboxFile             string            `yaml:"inbox_file"`
	Views                 []ViewConfig      `yaml:"views"`
	Keybindings           *rawKeybindings   `yaml:"keybindings"`
	Hey                   yaml.Node         `yaml:"hey"`
}

// rawHey mirrors the hey: YAML block for unmarshalling.
type rawHey struct {
	Command   string `yaml:"command"`
	Query     string `yaml:"query"`
	Account   string `yaml:"account"`
	StatePath string `yaml:"state_path"`
}

// Load reads configuration from the given path. If path is empty, it checks
// $PIKE_CONFIG, then $XDG_CONFIG_HOME/pike/config.yaml, then
// ~/.config/pike/config.yaml. If no config file is found, a default config
// is written to the XDG config path and defaults are returned.
func Load(path string) (*Config, error) {
	path, explicit := resolveConfigPath(path)

	if path == "" {
		writeDefaultConfig()
		return applyDefaults(&rawConfig{})
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !explicit {
			return applyDefaults(&rawConfig{})
		}
		return nil, err
	}

	return LoadBytes(data)
}

// LoadBytes parses YAML configuration from raw bytes. Useful for testing
// without touching the filesystem.
func LoadBytes(data []byte) (*Config, error) {
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return applyDefaults(&raw)
}

func resolveConfigPath(path string) (string, bool) {
	if path != "" {
		return path, true
	}

	if env := os.Getenv("PIKE_CONFIG"); env != "" {
		return env, true
	}

	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg != "" {
		p := filepath.Join(xdg, "pike", "config.yaml")
		if fileExists(p) {
			return p, false
		}
	}

	home, err := os.UserHomeDir()
	if err == nil {
		p := filepath.Join(home, ".config", "pike", "config.yaml")
		if fileExists(p) {
			return p, false
		}
	}

	return "", false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var knownActions = map[string]bool{
	"up": true, "down": true, "top": true, "bottom": true,
	"page_down": true, "page_up": true, "next_section": true, "prev_section": true,
	"enter": true, "quit": true, "summary": true, "filter": true,
	"query": true, "escape": true, "refresh": true, "all_tasks": true,
	"tag_search": true, "toggle_hidden": true, "toggle": true,
	"toggle_hidden_tag": true, "recently_completed": true,
	"create_task": true,
}

func parseKeybindings(raw *rawKeybindings) (map[string][]string, []CustomBinding, error) {
	if raw == nil {
		return nil, nil, nil
	}
	actions := make(map[string][]string)
	for name, val := range raw.Actions {
		if name == "custom" {
			continue
		}
		if !knownActions[name] {
			return nil, nil, fmt.Errorf("unknown keybinding action: %q", name)
		}
		slice, ok := val.([]interface{})
		if !ok {
			return nil, nil, fmt.Errorf("keybinding %q: expected list of strings", name)
		}
		keys := make([]string, 0, len(slice))
		for _, elem := range slice {
			s, ok := elem.(string)
			if !ok {
				return nil, nil, fmt.Errorf("keybinding %q: expected string, got %T", name, elem)
			}
			keys = append(keys, s)
		}
		if name == "escape" && len(keys) == 0 {
			return nil, nil, fmt.Errorf("escape keybinding cannot be disabled")
		}
		actions[name] = keys
	}
	var custom []CustomBinding
	for _, rc := range raw.Custom {
		if rc.View == "" && rc.Query == "" {
			return nil, nil, fmt.Errorf("custom binding for key %q must specify view or query", rc.Key)
		}
		if rc.View != "" && rc.Query != "" {
			return nil, nil, fmt.Errorf("custom binding for key %q cannot specify both view and query", rc.Key)
		}
		sort := rc.Sort
		if sort == "" {
			sort = "file"
		}
		if !tasksort.ValidOrders[sort] {
			return nil, nil, fmt.Errorf("custom binding for key %q: unknown sort order %q", rc.Key, sort)
		}
		custom = append(custom, CustomBinding{Key: rc.Key, View: rc.View, Query: rc.Query, Sort: sort})
	}
	return actions, custom, nil
}

func applyDefaults(raw *rawConfig) (*Config, error) {
	cfg := &Config{}

	// NotesDir: expand ~
	cfg.NotesDir = expandTilde(raw.NotesDir)

	// Include: default to ["**/*.md"]
	if len(raw.Include) > 0 {
		cfg.Include = raw.Include
	} else {
		cfg.Include = []string{"**/*.md"}
	}

	// Exclude: pass through
	cfg.Exclude = raw.Exclude

	// RefreshInterval: parse duration string, default 5s
	if raw.RefreshInterval != "" {
		d, err := time.ParseDuration(raw.RefreshInterval)
		if err != nil {
			return nil, err
		}
		cfg.RefreshInterval = d
	} else {
		cfg.RefreshInterval = 5 * time.Second
	}

	// Editor: use configured value, then $EDITOR, then "hx"
	switch {
	case raw.Editor != "":
		cfg.Editor = raw.Editor
	case os.Getenv("EDITOR") != "":
		cfg.Editor = os.Getenv("EDITOR")
	default:
		cfg.Editor = "hx"
	}

	// TagColors: default to empty map
	if raw.TagColors != nil {
		cfg.TagColors = raw.TagColors
	} else {
		cfg.TagColors = make(map[string]string)
	}

	// LinkColor: default to Catppuccin Mocha blue
	if raw.LinkColor != "" {
		cfg.LinkColor = raw.LinkColor
	} else {
		cfg.LinkColor = "#89b4fa"
	}

	// HiddenColor: default to Catppuccin Mocha overlay0
	if raw.HiddenColor != "" {
		cfg.HiddenColor = raw.HiddenColor
	} else {
		cfg.HiddenColor = "#6c7086"
	}

	// VisibleColor: default to Catppuccin Mocha pink
	if raw.VisibleColor != "" {
		cfg.VisibleColor = raw.VisibleColor
	} else {
		cfg.VisibleColor = "#f5c2e7"
	}

	// WeekStartDay: default to 0 (Sunday)
	if raw.WeekStartDay != nil {
		if *raw.WeekStartDay < 0 || *raw.WeekStartDay > 6 {
			return nil, fmt.Errorf("week_start_day must be 0-6 (Sunday-Saturday), got %d", *raw.WeekStartDay)
		}
		cfg.WeekStartDay = *raw.WeekStartDay
	} else {
		cfg.WeekStartDay = 0
	}

	// RecentlyCompletedDays: default to 7
	if raw.RecentlyCompletedDays != nil {
		cfg.RecentlyCompletedDays = *raw.RecentlyCompletedDays
	} else {
		cfg.RecentlyCompletedDays = 7
	}

	// DueDatesPath: write due dates JSON for wen integration (opt-in)
	if raw.DueDatesPath != "" {
		cfg.DueDatesPath = expandTilde(raw.DueDatesPath)
	}

	// InboxFile: default to "inbox.md"
	if raw.InboxFile != "" {
		cfg.InboxFile = raw.InboxFile
	} else {
		cfg.InboxFile = "inbox.md"
	}

	// Views: default to a single "All Open" view
	if len(raw.Views) > 0 {
		cfg.Views = raw.Views
		// Sort views by Order (stable to preserve list position for equal orders)
		slices.SortStableFunc(cfg.Views, func(a, b ViewConfig) int {
			return cmp.Compare(a.Order, b.Order)
		})
	} else {
		cfg.Views = []ViewConfig{
			{
				Title: "All Open",
				Query: "open",
				Sort:  "file",
				Order: 1,
			},
		}
	}

	dueDateViewCount := 0
	for _, v := range cfg.Views {
		if v.DueDates {
			dueDateViewCount++
		}
	}
	if dueDateViewCount > 1 {
		return nil, fmt.Errorf("at most one view may have due_dates: true")
	}

	// Hey: only populated when a hey: block is present, so its absence leaves
	// all existing behaviour untouched. A present-but-null value (every child
	// commented out) enables the feature with defaults, the same as `hey: {}`.
	// Any other value (boolean, string, number, sequence) is a mistake — most
	// dangerously `hey: false`, which a user would reasonably expect to switch
	// sync off — so reject it rather than silently enabling defaults.
	switch {
	case raw.Hey.Kind == 0:
		// absent: sync stays disabled.
	case raw.Hey.Kind == yaml.MappingNode:
		var rh rawHey
		if err := raw.Hey.Decode(&rh); err != nil {
			return nil, err
		}
		cfg.Hey = applyHeyDefaults(&rh)
	case raw.Hey.Kind == yaml.ScalarNode && raw.Hey.Tag == "!!null":
		cfg.Hey = applyHeyDefaults(&rawHey{})
	default:
		return nil, fmt.Errorf("hey: must be a mapping (or removed to disable sync), got %s", yamlKindName(raw.Hey.Kind))
	}

	keybindings, customBindings, err := parseKeybindings(raw.Keybindings)
	if err != nil {
		return nil, err
	}
	cfg.Keybindings = keybindings
	cfg.CustomBindings = customBindings

	return cfg, nil
}

// yamlKindName names a YAML node kind for diagnostics. Non-scalar nodes have no
// scalar Value, so naming the kind avoids rendering an empty %q for `hey: [1]`.
func yamlKindName(k yaml.Kind) string {
	switch k {
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}

// applyHeyDefaults fills in the documented defaults for an explicit hey: block.
func applyHeyDefaults(raw *rawHey) *HeyConfig {
	hey := &HeyConfig{
		Command: raw.Command,
		Query:   raw.Query,
		Account: raw.Account,
	}
	if hey.Command == "" {
		hey.Command = "hey"
	}
	if hey.Query == "" {
		hey.Query = "@due or @today"
	}
	if raw.StatePath != "" {
		hey.StatePath = expandTilde(raw.StatePath)
	} else {
		hey.StatePath = defaultHeyStatePath()
	}
	return hey
}

// defaultHeyStatePath returns $XDG_DATA_HOME/pike/hey-state.json, falling back
// to ~/.local/share/pike/hey-state.json.
func defaultHeyStatePath() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "pike", "hey-state.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "pike", "hey-state.json")
	}
	return filepath.Join(home, ".local", "share", "pike", "hey-state.json")
}

func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~/") && path != "~" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}

// defaultConfigYAML is the config written on first run.
const defaultConfigYAML = `# Pike configuration
# See https://github.com/zachthieme/pike for full documentation.

# Directory containing your markdown notes (supports ~ expansion)
# notes_dir: ~/notes

# File patterns to include (default: all .md files)
# include:
#   - "**/*.md"

# File patterns to exclude
# exclude:
#   - "templates/**"
#   - "archive/**"

# How often to re-scan files for changes (default: 5s)
# refresh_interval: 5s

# Editor command for opening tasks (default: $EDITOR, then hx)
# editor: hx

# Days to show in recently-completed view (default: 7)
# recently_completed_days: 7

# Day the week starts on: 0=Sunday, 1=Monday, ..., 6=Saturday (default: 0)
# week_start_day: 0

# Write due dates JSON for wen calendar integration (disabled by default)
# due_dates_path: ~/.local/share/pike/due.json

# File (relative to notes_dir) that new tasks are appended to, including todos
# imported by HEY sync (default: inbox.md)
# inbox_file: inbox.md

# HEY sync (disabled until this block is present). Uncomment to reconcile tasks
# with HEY via 'pike --sync'. Eligible tasks matching 'query' are pushed as new
# todos and linked with an @hey(id) tag; unlinked open todos are imported into
# the inbox. Sync never deletes on either side. See the README for details.
# hey:
#   command: hey                         # the hey CLI to run (default: hey)
#   query: "@due or @today"              # which eligible tasks push to HEY
#   account: ""                          # --account passed to hey (optional)
#   state_path: ~/.local/share/pike/hey-state.json  # sync state file

# Color theme (Catppuccin Mocha)
link_color: "#89b4fa"
hidden_color: "#6c7086"
visible_color: "#f5c2e7"

tag_colors:
  risk: "#f38ba8"
  due: "#f38ba8"
  today: "#a6e3a1"
  completed: "#a6e3a1"
  weekly: "#89b4fa"
  horizon: "#f9e2af"
  talk: "#cba6f7"
  _default: "#94e2d5"

# Keybindings — remap actions or add custom shortcuts
# Action names: up, down, top, bottom, page_down, page_up, next_section,
#   prev_section, enter, quit, summary, filter, query, escape, refresh,
#   all_tasks, tag_search, toggle_hidden, toggle, toggle_hidden_tag,
#   recently_completed, create_task, toggle_collapse
# keybindings:
#   toggle: ["space", "x"]       # override: list ALL keys you want
#   quit: ["q", "ctrl+c"]
#
#   # Custom shortcuts (replaces 1-9 focus keys when defined)
#   custom:
#     - key: "o"
#       view: "Overdue"          # focus a dashboard view by title
#     - key: "d"
#       query: "open and @due < today+3d"  # run a query

# Dashboard sections — each view is a filtered, sorted slice of your tasks
# Optional fields:
#   due_dates: true    — use this view's query for the due.json export (at most one)
#   hidden: true       — exclude from dashboard (still usable for due_dates, keybindings, --view)
views:
  - title: "Today"
    query: "open and @today"
    sort: due_asc
    color: "#a6e3a1"
    order: 1

  - title: "Overdue"
    query: "open and @due < today"
    sort: due_asc
    color: "#f38ba8"
    order: 2

  - title: "This Week"
    query: "open and @due >= today and @due <= today+7d"
    sort: due_asc
    color: "#f9e2af"
    order: 3

  # Uncomment to customize which tasks feed the due.json export:
  # - title: "Due Dates Export"
  #   query: "open and @due"
  #   sort: due_asc
  #   due_dates: true
  #   hidden: true
`

// writeDefaultConfig writes a default config file to the XDG config directory
// if no config file exists. Errors are silently ignored (best-effort).
func writeDefaultConfig() {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		dir = filepath.Join(home, ".config")
	}
	configDir := filepath.Join(dir, "pike")
	configPath := filepath.Join(configDir, "config.yaml")

	// Don't overwrite an existing file.
	if fileExists(configPath) {
		return
	}

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(configPath, []byte(defaultConfigYAML), 0o644) // best-effort; errors are intentionally ignored
}
