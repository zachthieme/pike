package config

import (
	"os"
	"path/filepath"
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
