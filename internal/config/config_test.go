package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadBytes_FullConfig(t *testing.T) {
	yaml := `
notes_dir: ~/CloudDocs/Notes
include:
  - "**/*.md"
  - "**/*.txt"
exclude:
  - "templates/**"
  - "archive/**"
refresh_interval: 10s
editor: nvim
tag_colors:
  risk: red
  due: red
  today: green
  completed: green
  weekly: blue
  horizon: yellow
  talk: magenta
  _default: cyan
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
  - title: "Completed"
    query: "completed"
    sort: completed_desc
    color: blue
    order: 3
`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()
	wantDir := filepath.Join(home, "CloudDocs", "Notes")
	if cfg.NotesDir != wantDir {
		t.Errorf("NotesDir = %q, want %q", cfg.NotesDir, wantDir)
	}

	if len(cfg.Include) != 2 {
		t.Fatalf("Include len = %d, want 2", len(cfg.Include))
	}
	if cfg.Include[0] != "**/*.md" || cfg.Include[1] != "**/*.txt" {
		t.Errorf("Include = %v, want [**/*.md **/*.txt]", cfg.Include)
	}

	if len(cfg.Exclude) != 2 {
		t.Fatalf("Exclude len = %d, want 2", len(cfg.Exclude))
	}
	if cfg.Exclude[0] != "templates/**" || cfg.Exclude[1] != "archive/**" {
		t.Errorf("Exclude = %v", cfg.Exclude)
	}

	if cfg.RefreshInterval != 10*time.Second {
		t.Errorf("RefreshInterval = %v, want 10s", cfg.RefreshInterval)
	}

	if cfg.Editor != "nvim" {
		t.Errorf("Editor = %q, want %q", cfg.Editor, "nvim")
	}

	if len(cfg.TagColors) != 8 {
		t.Errorf("TagColors len = %d, want 8", len(cfg.TagColors))
	}
	if cfg.TagColors["risk"] != "red" {
		t.Errorf("TagColors[risk] = %q, want %q", cfg.TagColors["risk"], "red")
	}
	if cfg.TagColors["_default"] != "cyan" {
		t.Errorf("TagColors[_default] = %q, want %q", cfg.TagColors["_default"], "cyan")
	}

	if len(cfg.Views) != 3 {
		t.Fatalf("Views len = %d, want 3", len(cfg.Views))
	}
	if cfg.Views[0].Title != "Overdue" {
		t.Errorf("Views[0].Title = %q, want %q", cfg.Views[0].Title, "Overdue")
	}
	if cfg.Views[0].Query != "open and @due < today" {
		t.Errorf("Views[0].Query = %q", cfg.Views[0].Query)
	}
	if cfg.Views[0].Sort != "due_asc" {
		t.Errorf("Views[0].Sort = %q, want %q", cfg.Views[0].Sort, "due_asc")
	}
	if cfg.Views[0].Color != "red" {
		t.Errorf("Views[0].Color = %q, want %q", cfg.Views[0].Color, "red")
	}
	if cfg.Views[0].Order != 1 {
		t.Errorf("Views[0].Order = %d, want 1", cfg.Views[0].Order)
	}
}

func TestLoadBytes_MinimalConfig(t *testing.T) {
	yaml := `notes_dir: ~/Notes`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()
	wantDir := filepath.Join(home, "Notes")
	if cfg.NotesDir != wantDir {
		t.Errorf("NotesDir = %q, want %q", cfg.NotesDir, wantDir)
	}

	// Defaults should be applied
	if len(cfg.Include) != 1 || cfg.Include[0] != "**/*.md" {
		t.Errorf("Include = %v, want [**/*.md]", cfg.Include)
	}

	if cfg.RefreshInterval != 5*time.Second {
		t.Errorf("RefreshInterval = %v, want 5s", cfg.RefreshInterval)
	}

	if len(cfg.Views) != 1 {
		t.Fatalf("Views len = %d, want 1", len(cfg.Views))
	}
	if cfg.Views[0].Title != "All Open" {
		t.Errorf("Views[0].Title = %q, want %q", cfg.Views[0].Title, "All Open")
	}
	if cfg.Views[0].Query != "open" {
		t.Errorf("Views[0].Query = %q, want %q", cfg.Views[0].Query, "open")
	}
	if cfg.Views[0].Sort != "file" {
		t.Errorf("Views[0].Sort = %q, want %q", cfg.Views[0].Sort, "file")
	}
	if cfg.Views[0].Order != 1 {
		t.Errorf("Views[0].Order = %d, want 1", cfg.Views[0].Order)
	}
}

func TestLoad_ExplicitPathMissingFileReturnsError(t *testing.T) {
	// An explicit path (flag or env var) to a missing file should error.
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing explicit config path, got nil")
	}
}

func TestLoad_ImplicitMissingFileReturnsDefaults(t *testing.T) {
	// When no explicit path is given and no config file is discovered,
	// Load should return defaults without error.
	t.Setenv("PIKE_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // empty dir, no config file
	t.Setenv("HOME", t.TempDir())            // prevent ~/.config/pike/config.yaml fallback

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Views) != 1 {
		t.Fatalf("Views len = %d, want 1", len(cfg.Views))
	}
	if cfg.Views[0].Title != "All Open" {
		t.Errorf("Views[0].Title = %q, want %q", cfg.Views[0].Title, "All Open")
	}

	if len(cfg.Include) != 1 || cfg.Include[0] != "**/*.md" {
		t.Errorf("Include = %v, want [**/*.md]", cfg.Include)
	}

	if cfg.RefreshInterval != 5*time.Second {
		t.Errorf("RefreshInterval = %v, want 5s", cfg.RefreshInterval)
	}
}

func TestLoad_ExplicitEnvMissingFileReturnsError(t *testing.T) {
	// When PIKE_CONFIG points to a missing file, Load should error.
	t.Setenv("PIKE_CONFIG", "/nonexistent/pike/config.yaml")

	_, err := Load("")
	if err == nil {
		t.Fatal("expected error when PIKE_CONFIG points to missing file, got nil")
	}
}

func TestLoadBytes_InvalidYAML(t *testing.T) {
	yaml := `notes_dir: [invalid`
	_, err := LoadBytes([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadBytes_DurationParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantDur  time.Duration
		wantZero bool
	}{
		{
			name:    "5 seconds",
			input:   `refresh_interval: 5s`,
			wantDur: 5 * time.Second,
		},
		{
			name:    "1 minute",
			input:   `refresh_interval: 1m`,
			wantDur: 1 * time.Minute,
		},
		{
			name:     "zero disables",
			input:    `refresh_interval: "0"`,
			wantDur:  0,
			wantZero: true,
		},
		{
			name:    "default when not specified",
			input:   `notes_dir: ~/Notes`,
			wantDur: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadBytes([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.RefreshInterval != tt.wantDur {
				t.Errorf("RefreshInterval = %v, want %v", cfg.RefreshInterval, tt.wantDur)
			}
		})
	}
}

func TestLoadBytes_ViewOrdering(t *testing.T) {
	yaml := `
views:
  - title: "Third"
    query: "open"
    sort: file
    order: 3
  - title: "First"
    query: "open"
    sort: file
    order: 1
  - title: "Second"
    query: "open"
    sort: file
    order: 2
`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Views) != 3 {
		t.Fatalf("Views len = %d, want 3", len(cfg.Views))
	}

	// Views should be sorted by order
	if cfg.Views[0].Title != "First" {
		t.Errorf("Views[0].Title = %q, want %q", cfg.Views[0].Title, "First")
	}
	if cfg.Views[1].Title != "Second" {
		t.Errorf("Views[1].Title = %q, want %q", cfg.Views[1].Title, "Second")
	}
	if cfg.Views[2].Title != "Third" {
		t.Errorf("Views[2].Title = %q, want %q", cfg.Views[2].Title, "Third")
	}
}

func TestLoadBytes_TildeExpansion(t *testing.T) {
	yaml := `notes_dir: ~/some/path`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "some", "path")
	if cfg.NotesDir != want {
		t.Errorf("NotesDir = %q, want %q", cfg.NotesDir, want)
	}

	// Bare tilde should expand to home directory
	yamlTilde := `notes_dir: "~"`
	cfgTilde, err := LoadBytes([]byte(yamlTilde))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfgTilde.NotesDir != home {
		t.Errorf("NotesDir for ~ = %q, want %q", cfgTilde.NotesDir, home)
	}

	// Should not expand tilde in the middle of a path
	yaml2 := `notes_dir: /home/user/~notes`
	cfg2, err := LoadBytes([]byte(yaml2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg2.NotesDir != "/home/user/~notes" {
		t.Errorf("NotesDir = %q, want %q", cfg2.NotesDir, "/home/user/~notes")
	}
}

func TestLoadBytes_DefaultTagColors(t *testing.T) {
	yaml := `notes_dir: ~/Notes`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When tag_colors is not specified, we should get an empty map (or nil)
	// but not panic on access
	if cfg.TagColors == nil {
		t.Error("TagColors should not be nil, want empty map")
	}
}

func TestLoadBytes_DefaultIncludePatterns(t *testing.T) {
	// When include is not specified, default to ["**/*.md"]
	yaml := `notes_dir: ~/Notes`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Include) != 1 {
		t.Fatalf("Include len = %d, want 1", len(cfg.Include))
	}
	if cfg.Include[0] != "**/*.md" {
		t.Errorf("Include[0] = %q, want %q", cfg.Include[0], "**/*.md")
	}

	// When include is explicitly set, don't override
	yaml2 := `
notes_dir: ~/Notes
include:
  - "**/*.txt"
`
	cfg2, err := LoadBytes([]byte(yaml2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg2.Include) != 1 || cfg2.Include[0] != "**/*.txt" {
		t.Errorf("Include = %v, want [**/*.txt]", cfg2.Include)
	}
}

func TestLoad_EnvVarFallback(t *testing.T) {
	// Create a temp config file
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`notes_dir: ~/EnvNotes`), 0o644)
	if err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	// Set PIKE_CONFIG env var
	t.Setenv("PIKE_CONFIG", cfgPath)

	// Load with empty path — should use env var
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "EnvNotes")
	if cfg.NotesDir != want {
		t.Errorf("NotesDir = %q, want %q", cfg.NotesDir, want)
	}
}

func TestLoad_EditorDefault(t *testing.T) {
	// When editor is not set, should use $EDITOR or fallback to "hx"
	yaml := `notes_dir: ~/Notes`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	editorEnv := os.Getenv("EDITOR")
	if editorEnv != "" {
		if cfg.Editor != editorEnv {
			t.Errorf("Editor = %q, want %q (from $EDITOR)", cfg.Editor, editorEnv)
		}
	} else {
		if cfg.Editor != "hx" {
			t.Errorf("Editor = %q, want %q", cfg.Editor, "hx")
		}
	}
}

func TestLoadBytes_ViewStableOrder(t *testing.T) {
	// Views with the same order value should preserve their list position
	yaml := `
views:
  - title: "Alpha"
    query: "open"
    sort: file
    order: 1
  - title: "Beta"
    query: "open"
    sort: file
    order: 1
  - title: "Gamma"
    query: "open"
    sort: file
    order: 1
`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Views[0].Title != "Alpha" {
		t.Errorf("Views[0].Title = %q, want %q", cfg.Views[0].Title, "Alpha")
	}
	if cfg.Views[1].Title != "Beta" {
		t.Errorf("Views[1].Title = %q, want %q", cfg.Views[1].Title, "Beta")
	}
	if cfg.Views[2].Title != "Gamma" {
		t.Errorf("Views[2].Title = %q, want %q", cfg.Views[2].Title, "Gamma")
	}
}

func TestLoadBytes_DefaultRecentlyCompletedDays(t *testing.T) {
	yaml := `notes_dir: ~/Notes`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RecentlyCompletedDays != 7 {
		t.Errorf("RecentlyCompletedDays = %d, want 7", cfg.RecentlyCompletedDays)
	}
}

func TestLoadBytes_DefaultHiddenColor(t *testing.T) {
	yaml := `notes_dir: ~/Notes`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HiddenColor != "#6c7086" {
		t.Errorf("HiddenColor = %q, want %q", cfg.HiddenColor, "#6c7086")
	}
}

func TestLoadBytes_ExplicitHiddenColor(t *testing.T) {
	yaml := `
notes_dir: ~/Notes
hidden_color: "red"
`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HiddenColor != "red" {
		t.Errorf("HiddenColor = %q, want %q", cfg.HiddenColor, "red")
	}
}

func TestLoadBytes_DefaultVisibleColor(t *testing.T) {
	yaml := `notes_dir: ~/Notes`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.VisibleColor != "#f5c2e7" {
		t.Errorf("VisibleColor = %q, want %q", cfg.VisibleColor, "#f5c2e7")
	}
}

func TestLoadBytes_ExplicitVisibleColor(t *testing.T) {
	yaml := `
notes_dir: ~/Notes
visible_color: "#FF5733"
`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.VisibleColor != "#FF5733" {
		t.Errorf("VisibleColor = %q, want %q", cfg.VisibleColor, "#FF5733")
	}
}

func TestLoadBytes_ExplicitRecentlyCompletedDays(t *testing.T) {
	yaml := `
notes_dir: ~/Notes
recently_completed_days: 14
`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RecentlyCompletedDays != 14 {
		t.Errorf("RecentlyCompletedDays = %d, want 14", cfg.RecentlyCompletedDays)
	}
}

func TestLoadBytes_KeybindingsOverride(t *testing.T) {
	input := `
keybindings:
  toggle: ["space", "x"]
  quit: ["q", "ctrl+c"]
views:
  - title: "Open"
    query: "open"
    sort: file
    order: 1
`
	cfg, err := LoadBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Keybindings) == 0 {
		t.Fatal("Keybindings should not be empty")
	}
	toggle := cfg.Keybindings["toggle"]
	if len(toggle) != 2 || toggle[0] != "space" || toggle[1] != "x" {
		t.Errorf("toggle = %v, want [space x]", toggle)
	}
	quit := cfg.Keybindings["quit"]
	if len(quit) != 2 || quit[0] != "q" || quit[1] != "ctrl+c" {
		t.Errorf("quit = %v, want [q ctrl+c]", quit)
	}
}

func TestLoadBytes_CustomBindings(t *testing.T) {
	input := `
keybindings:
  custom:
    - key: "o"
      view: "Overdue"
    - key: "d"
      query: "open and @due < today+3d"
      sort: "due_asc"
views:
  - title: "Overdue"
    query: "open and @due < today"
    sort: due_asc
    order: 1
`
	cfg, err := LoadBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.CustomBindings) != 2 {
		t.Fatalf("CustomBindings len = %d, want 2", len(cfg.CustomBindings))
	}
	if cfg.CustomBindings[0].Key != "o" || cfg.CustomBindings[0].View != "Overdue" {
		t.Errorf("binding 0 = %+v, want key=o view=Overdue", cfg.CustomBindings[0])
	}
	if cfg.CustomBindings[1].Key != "d" || cfg.CustomBindings[1].Query != "open and @due < today+3d" {
		t.Errorf("binding 1 = %+v, want key=d query=...", cfg.CustomBindings[1])
	}
	if cfg.CustomBindings[1].Sort != "due_asc" {
		t.Errorf("binding 1 sort = %q, want due_asc", cfg.CustomBindings[1].Sort)
	}
}

func TestLoadBytes_KeybindingsUnknownAction(t *testing.T) {
	input := `
keybindings:
  bogus: ["x"]
views:
  - title: "Open"
    query: "open"
    sort: file
    order: 1
`
	_, err := LoadBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestLoadBytes_CustomBindingBothViewAndQuery(t *testing.T) {
	input := `
keybindings:
  custom:
    - key: "o"
      view: "Overdue"
      query: "open"
views:
  - title: "Open"
    query: "open"
    sort: file
    order: 1
`
	_, err := LoadBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for binding with both view and query")
	}
}

func TestLoadBytes_CustomBindingNeitherViewNorQuery(t *testing.T) {
	input := `
keybindings:
  custom:
    - key: "o"
views:
  - title: "Open"
    query: "open"
    sort: file
    order: 1
`
	_, err := LoadBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for binding with neither view nor query")
	}
}

func TestLoadBytes_EscapeCannotBeDisabled(t *testing.T) {
	input := `
keybindings:
  escape: []
views:
  - title: "Open"
    query: "open"
    sort: file
    order: 1
`
	_, err := LoadBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for disabled escape")
	}
}

func TestLoadBytes_DueDatesPathNotSetByDefault(t *testing.T) {
	yaml := `notes_dir: ~/Notes`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DueDatesPath != "" {
		t.Errorf("DueDatesPath = %q, want empty (disabled by default)", cfg.DueDatesPath)
	}
}

func TestLoadBytes_DueDatesPathExplicit(t *testing.T) {
	yaml := `
notes_dir: ~/Notes
due_dates_path: ~/custom/due.json
`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "custom", "due.json")
	if cfg.DueDatesPath != want {
		t.Errorf("DueDatesPath = %q, want %q", cfg.DueDatesPath, want)
	}
}

func TestLoadBytes_DueDatesPathAbsolute(t *testing.T) {
	yaml := `
notes_dir: ~/Notes
due_dates_path: /tmp/pike/due.json
`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DueDatesPath != "/tmp/pike/due.json" {
		t.Errorf("DueDatesPath = %q, want %q", cfg.DueDatesPath, "/tmp/pike/due.json")
	}
}

func TestLoadBytes_NoKeybindingsBackwardCompatible(t *testing.T) {
	input := `
views:
  - title: "Open"
    query: "open"
    sort: file
    order: 1
`
	cfg, err := LoadBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Keybindings != nil {
		t.Errorf("Keybindings should be nil when not specified, got %v", cfg.Keybindings)
	}
	if cfg.CustomBindings != nil {
		t.Errorf("CustomBindings should be nil when not specified, got %v", cfg.CustomBindings)
	}
}

func TestLoadBytes_ViewDueDatesAndHidden(t *testing.T) {
	yaml := `
views:
  - title: "Open"
    query: "open"
    sort: file
    order: 1
  - title: "Due Export"
    query: "open and @due"
    sort: due_asc
    due_dates: true
    hidden: true
    order: 2
`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Views) != 2 {
		t.Fatalf("Views len = %d, want 2", len(cfg.Views))
	}
	if cfg.Views[0].DueDates {
		t.Error("Views[0].DueDates should be false")
	}
	if cfg.Views[0].Hidden {
		t.Error("Views[0].Hidden should be false")
	}
	if !cfg.Views[1].DueDates {
		t.Error("Views[1].DueDates should be true")
	}
	if !cfg.Views[1].Hidden {
		t.Error("Views[1].Hidden should be true")
	}
}

func TestInboxFileDefault(t *testing.T) {
	cfg, err := LoadBytes([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.InboxFile != "inbox.md" {
		t.Errorf("InboxFile = %q, want 'inbox.md'", cfg.InboxFile)
	}
}

func TestInboxFileConfigured(t *testing.T) {
	cfg, err := LoadBytes([]byte("inbox_file: capture.md"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.InboxFile != "capture.md" {
		t.Errorf("InboxFile = %q, want 'capture.md'", cfg.InboxFile)
	}
}

func TestLoadBytes_MultipleDueDatesViewsError(t *testing.T) {
	yaml := `
views:
  - title: "A"
    query: "open"
    sort: file
    due_dates: true
    order: 1
  - title: "B"
    query: "open and @due"
    sort: file
    due_dates: true
    order: 2
`
	_, err := LoadBytes([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for multiple due_dates views")
	}
}

func TestLoadBytes_HeyDefaults(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	yaml := `
hey: {}
`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Hey == nil {
		t.Fatal("expected Hey block to be present")
	}
	if cfg.Hey.Command != "hey" {
		t.Errorf("Command = %q, want %q", cfg.Hey.Command, "hey")
	}
	if cfg.Hey.Query != "@due or @today" {
		t.Errorf("Query = %q, want %q", cfg.Hey.Query, "@due or @today")
	}
	if cfg.Hey.Account != "" {
		t.Errorf("Account = %q, want empty", cfg.Hey.Account)
	}
	home, _ := os.UserHomeDir()
	wantState := filepath.Join(home, ".local", "share", "pike", "hey-state.json")
	if cfg.Hey.StatePath != wantState {
		t.Errorf("StatePath = %q, want %q", cfg.Hey.StatePath, wantState)
	}
}

func TestLoadBytes_HeyStatePathXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdgdata")
	cfg, err := LoadBytes([]byte("hey: {}\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("/tmp/xdgdata", "pike", "hey-state.json")
	if cfg.Hey.StatePath != want {
		t.Errorf("StatePath = %q, want %q", cfg.Hey.StatePath, want)
	}
}

func TestLoadBytes_HeyOverrides(t *testing.T) {
	yaml := `
hey:
  command: /usr/local/bin/hey
  query: "@today"
  account: work
  state_path: ~/custom/state.json
`
	cfg, err := LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Hey.Command != "/usr/local/bin/hey" {
		t.Errorf("Command = %q", cfg.Hey.Command)
	}
	if cfg.Hey.Query != "@today" {
		t.Errorf("Query = %q", cfg.Hey.Query)
	}
	if cfg.Hey.Account != "work" {
		t.Errorf("Account = %q", cfg.Hey.Account)
	}
	home, _ := os.UserHomeDir()
	wantState := filepath.Join(home, "custom", "state.json")
	if cfg.Hey.StatePath != wantState {
		t.Errorf("StatePath = %q, want %q", cfg.Hey.StatePath, wantState)
	}
}

func TestLoadBytes_NoHeyBlock(t *testing.T) {
	cfg, err := LoadBytes([]byte("notes_dir: ~/notes\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Hey != nil {
		t.Errorf("expected Hey to be nil when block absent, got %+v", cfg.Hey)
	}
}

// TestDefaultConfigHasCommentedHeyKeys asserts the config written on first run
// carries the hey: block, commented out, with every key a user needs to enable
// HEY sync. The keys ship commented so an untouched config leaves HEY disabled.
func TestDefaultConfigHasCommentedHeyKeys(t *testing.T) {
	for _, want := range []string{
		"# hey:",
		"#   command:",
		"#   query:",
		"#   account:",
		"#   state_path:",
	} {
		if !strings.Contains(defaultConfigYAML, want) {
			t.Errorf("default config missing commented hey key %q", want)
		}
	}

	// The commented block must be inert: an untouched default config still
	// leaves Hey nil (sync disabled).
	cfg, err := LoadBytes([]byte(defaultConfigYAML))
	if err != nil {
		t.Fatalf("default config does not parse: %v", err)
	}
	if cfg.Hey != nil {
		t.Errorf("commented hey: block should leave Hey nil, got %+v", cfg.Hey)
	}
}
