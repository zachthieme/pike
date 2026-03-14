package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the key bindings for the TUI.
type KeyMap struct {
	Up, Down, Top, Bottom    key.Binding
	NextSection, PrevSection key.Binding
	FocusSection             [9]key.Binding // 1-9
	Enter, Quit, Summary           key.Binding
	Filter, Escape, Refresh        key.Binding
	AllTasks, TagSearch, ToggleHidden key.Binding
	Toggle key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	km := KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/up", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
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
		NextSection: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next section"),
		),
		PrevSection: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev section"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open in editor"),
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
	}

	for i := 0; i < 9; i++ {
		km.FocusSection[i] = key.NewBinding(
			key.WithKeys(string(rune('1' + i))),
			key.WithHelp(string(rune('1'+i)), "focus section"),
		)
	}

	return km
}
