package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/zachthieme/pike/internal/config"
)

// KeyMap defines the key bindings for the TUI.
type KeyMap struct {
	Up, Down, Top, Bottom        key.Binding
	PageDown, PageUp             key.Binding
	NextSection, PrevSection     key.Binding
	FocusSection                 [9]key.Binding // 1-9
	Enter, Quit, Summary         key.Binding
	Filter, Query, Escape, Refresh key.Binding
	AllTasks, TagSearch, ToggleHidden key.Binding
	Toggle, ToggleHiddenTag, RecentlyCompleted key.Binding
	ToggleCollapse key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	km := KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up", "ctrl+p"),
			key.WithHelp("k/up", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down", "ctrl+n"),
			key.WithHelp("j/down", "move down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "go to top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "go to bottom"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "page down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "page up"),
		),
		NextSection: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", "next section"),
		),
		PrevSection: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("Shift+Tab", "prev section"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "open in editor"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		Summary: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "toggle summary"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Query: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "query"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "escape"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		AllTasks: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "all open tasks"),
		),
		TagSearch: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "search tags"),
		),
		ToggleHidden: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "toggle hidden"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "toggle complete"),
		),
		ToggleHiddenTag: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "toggle @hidden tag"),
		),
		RecentlyCompleted: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "recently completed"),
		),
		ToggleCollapse: key.NewBinding(
			key.WithKeys("z"),
			key.WithHelp("z", "toggle collapse"),
		),
	}

	for i := 0; i < 9; i++ {
		km.FocusSection[i] = key.NewBinding(
			key.WithKeys(string(rune('1' + i))),
			key.WithHelp(string(rune('1'+i)), "focus section"),
		)
	}

	return km
}

// BuildKeyMap creates a KeyMap starting from defaults, applying user overrides,
// and disabling FocusSection keys when custom bindings are present.
func BuildKeyMap(overrides map[string][]string, custom []config.CustomBinding) KeyMap {
	km := DefaultKeyMap()

	bindings := map[string]*key.Binding{
		"up": &km.Up, "down": &km.Down, "top": &km.Top, "bottom": &km.Bottom,
		"page_down": &km.PageDown, "page_up": &km.PageUp,
		"next_section": &km.NextSection, "prev_section": &km.PrevSection,
		"enter": &km.Enter, "quit": &km.Quit, "summary": &km.Summary,
		"filter": &km.Filter, "query": &km.Query, "escape": &km.Escape,
		"refresh": &km.Refresh, "all_tasks": &km.AllTasks,
		"tag_search": &km.TagSearch, "toggle_hidden": &km.ToggleHidden,
		"toggle": &km.Toggle, "toggle_hidden_tag": &km.ToggleHiddenTag,
		"recently_completed": &km.RecentlyCompleted,
		"toggle_collapse": &km.ToggleCollapse,
	}

	for name, keys := range overrides {
		b, ok := bindings[name]
		if !ok {
			continue
		}
		if len(keys) == 0 {
			b.SetEnabled(false)
			continue
		}
		helpKey := strings.Join(keys, "/")
		helpDesc := b.Help().Desc
		*b = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(helpKey, helpDesc),
		)
	}

	if len(custom) > 0 {
		for i := range km.FocusSection {
			km.FocusSection[i].SetEnabled(false)
		}
	}

	return km
}
