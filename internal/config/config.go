package config

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	NotesDir        string            `yaml:"-"`
	Include         []string          `yaml:"-"`
	Exclude         []string          `yaml:"-"`
	RefreshInterval time.Duration     `yaml:"-"`
	Editor          string            `yaml:"-"`
	TagColors       map[string]string `yaml:"-"`
	LinkColor       string            `yaml:"-"`
	Views           []ViewConfig      `yaml:"-"`
}

// ViewConfig defines a single dashboard section.
type ViewConfig struct {
	Title string `yaml:"title"`
	Query string `yaml:"query"`
	Sort  string `yaml:"sort"`
	Color string `yaml:"color"`
	Order int    `yaml:"order"`
}

// rawConfig mirrors the YAML structure for unmarshalling.
type rawConfig struct {
	NotesDir        string            `yaml:"notes_dir"`
	Include         []string          `yaml:"include"`
	Exclude         []string          `yaml:"exclude"`
	RefreshInterval string            `yaml:"refresh_interval"`
	Editor          string            `yaml:"editor"`
	TagColors       map[string]string `yaml:"tag_colors"`
	LinkColor       string            `yaml:"link_color"`
	Views           []ViewConfig      `yaml:"views"`
}

// Load reads configuration from the given path. If path is empty, it checks
// $PIKE_CONFIG, then $XDG_CONFIG_HOME/pike/config.yaml, then
// ~/.config/pike/config.yaml. If no file is found at any path, defaults are
// returned.
func Load(path string) (*Config, error) {
	path, explicit := resolveConfigPath(path)

	if path == "" {
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

	// LinkColor: default to blue
	if raw.LinkColor != "" {
		cfg.LinkColor = raw.LinkColor
	} else {
		cfg.LinkColor = "blue"
	}

	// Views: default to a single "All Open" view
	if len(raw.Views) > 0 {
		cfg.Views = raw.Views
		// Sort views by Order (stable to preserve list position for equal orders)
		sort.SliceStable(cfg.Views, func(i, j int) bool {
			return cfg.Views[i].Order < cfg.Views[j].Order
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

	return cfg, nil
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
