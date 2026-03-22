// Package tui implements the interactive Bubble Tea terminal dashboard.
package tui

import (
	"time"

	"github.com/zachthieme/pike/internal/config"
	"github.com/zachthieme/pike/internal/filter"
	"github.com/zachthieme/pike/internal/model"

	tea "github.com/charmbracelet/bubbletea"
)

// Model is the main Bubbletea model for the tasks TUI.
//
// State machine — the primary display state is Model.mode (viewMode):
//
//	modeDashboard ←────── Escape ─────── modeAllTasks
//	    │                                     ↑
//	    ├─ 1-9/custom → modeFocused           │
//	    │                   │          'a' or tag select
//	    ├─ 'a' ─────→ modeAllTasks            │
//	    ├─ 't' ─────→ modeTagSearch ── select ┘
//	    ├─ 'c' ─────→ modeRecentlyCompleted
//	    └─ Escape ──→ modeDashboard (from modeFocused, if !viewLocked)
//
// Orthogonal modifiers (independent of mode):
//   - showSummary: overlay toggled with 's', dismissed with Escape
//   - showHidden:  whether @hidden tasks are visible, toggled with 'h'
//   - filterBar:   sub-model text input (active/inactive, focused/blurred)
//
// focusedView holds the section title only when mode == modeFocused.
// showAll is true only when modeAllTasks was entered from tag search.
type Model struct {
	// Data — task source and config.
	config   *config.Config
	allTasks []model.Task

	// Section cache — rebuilt together in rebuildSections/rebuildDashboard.
	sections           []filter.ViewResult // current filtered/sorted sections
	unfilteredSections []filter.ViewResult // pre-filter cache for visibleSections()
	hiddenCounts       []int              // per-section count of @hidden tasks removed
	openCount          int                // cached open checkbox count, updated on rebuild
	completedThisWeek  int                // cached completed-this-week count

	// Navigation and view state.
	nav          Navigator // cursor + section navigation
	focusedView  string    // section title when mode == modeFocused; empty otherwise
	viewLocked   bool      // when true, block mode-switching keys (set via --view flag)
	mode         viewMode
	sortOverride string // per-query sort order from custom bindings; "" uses default
	showSummary  bool
	showHidden   bool // whether to show @hidden tasks
	showAll      bool // when true, all-tasks includes completed (e.g. from tag search)

	// Sub-models.
	filterBar FilterBar
	tagSearch TagSearch

	// Viewport.
	width  int
	height int
	err    error

	// Key bindings.
	keys           KeyMap
	customBindings []config.CustomBinding
	customKeyIndex map[string]int // key string → index in customBindings for O(1) lookup

	// Injected dependencies.
	scanFunc   func() ([]model.Task, error)  // refresh callback
	configFunc func() (*config.Config, error) // config reload callback
	editorCmd  string
	tagColors  map[string]string
	version    string
	now        func() time.Time          // injectable for testing
	warnings   []model.Warning           // parse warnings from last scan
	warningsFunc func() []model.Warning  // returns latest parse warnings
}

// NewModel creates a new TUI model with the given configuration and initial tasks.
func NewModel(cfg *config.Config, tasks []model.Task, scanFunc func() ([]model.Task, error), configFunc func() (*config.Config, error)) Model {
	m := Model{
		config:         cfg,
		allTasks:       tasks,
		focusedView:    "",
		filterBar:      NewFilterBar(),
		tagSearch:      NewTagSearch(),
		scanFunc:       scanFunc,
		editorCmd:      cfg.Editor,
		tagColors:      cfg.TagColors,
		keys:           BuildKeyMap(cfg.Keybindings, cfg.CustomBindings),
		customBindings: cfg.CustomBindings,
		customKeyIndex: buildCustomKeyIndex(cfg.CustomBindings),
		now:            time.Now,
	}
	m.configFunc = configFunc

	m.rebuildSections()
	m.nav.ClampCursor(m.displaySections())

	return m
}

// buildCustomKeyIndex creates a map from key string to custom binding index
// for O(1) lookup in handleKeyCustomBinding.
func buildCustomKeyIndex(bindings []config.CustomBinding) map[string]int {
	idx := make(map[string]int, len(bindings))
	for i, cb := range bindings {
		idx[cb.Key] = i
	}
	return idx
}

// SetVersion sets the version string for display in the summary overlay.
func (m *Model) SetVersion(v string) {
	m.version = v
}

// SetWarnings sets the current parse warnings slice.
func (m *Model) SetWarnings(w []model.Warning) {
	m.warnings = w
}

// SetWarningsFunc sets a function that returns the latest parse warnings.
func (m *Model) SetWarningsFunc(f func() []model.Warning) {
	m.warningsFunc = f
}

// SetFocusedView sets the focused view by section title, locks the view so
// mode-switching keys (a/t/s/c/1-9) are disabled, and rebuilds sections.
func (m *Model) SetFocusedView(title string) {
	m.mode = modeFocused
	m.focusedView = title
	m.viewLocked = true
	m.keys.Summary.SetEnabled(false)
	m.keys.AllTasks.SetEnabled(false)
	m.keys.TagSearch.SetEnabled(false)
	m.keys.RecentlyCompleted.SetEnabled(false)
	for i := range m.keys.FocusSection {
		m.keys.FocusSection[i].SetEnabled(false)
	}
	m.rebuildSections()
	m.nav.ClampCursor(m.displaySections())
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.config != nil && m.config.RefreshInterval > 0 {
		return tea.Tick(m.config.RefreshInterval, func(time.Time) tea.Msg {
			return RefreshMsg{}
		})
	}
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.nav.SetHeight(msg.Height)
		return m, nil

	case RefreshMsg:
		var nextTick tea.Cmd
		if m.config != nil && m.config.RefreshInterval > 0 {
			nextTick = tea.Tick(m.config.RefreshInterval, func(time.Time) tea.Msg {
				return RefreshMsg{}
			})
		}
		// Launch async scan + config reload.
		scanFn := m.scanFunc
		configFn := m.configFunc
		scanCmd := func() tea.Msg {
			var cfg *config.Config
			if configFn != nil {
				c, err := configFn()
				if err != nil {
					return scanResultMsg{Err: err}
				}
				cfg = c
			}
			if scanFn != nil {
				tasks, err := scanFn()
				if err != nil {
					return scanResultMsg{Err: err}
				}
				return scanResultMsg{Tasks: tasks, Config: cfg}
			}
			return scanResultMsg{Config: cfg}
		}
		return m, tea.Batch(nextTick, scanCmd)

	case FilterSetErrorMsg:
		m.filterBar, _ = m.filterBar.Update(msg)
		return m, nil

	case scanResultMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		if msg.Config != nil {
			m.config = msg.Config
			m.tagColors = msg.Config.TagColors
			m.editorCmd = msg.Config.Editor
			m.keys = BuildKeyMap(msg.Config.Keybindings, msg.Config.CustomBindings)
			m.customBindings = msg.Config.CustomBindings
			m.customKeyIndex = buildCustomKeyIndex(msg.Config.CustomBindings)
		}
		if msg.Tasks != nil {
			m.allTasks = msg.Tasks
			if m.mode == modeTagSearch {
				tags := extractTagNames(m.allTasks)
				m.tagSearch, _ = m.tagSearch.Update(TagSearchRefreshMsg{Tags: tags})
			}
		}
		// Rebuild sections if tasks or config changed (config affects views, tag colors).
		if msg.Tasks != nil || msg.Config != nil {
			m.rebuildSections()
			m.nav.ClampCursor(m.displaySections())
		}
		if m.warningsFunc != nil {
			m.warnings = m.warningsFunc()
		}
		return m, nil

	case toggleResultMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		return m, func() tea.Msg { return RefreshMsg{} }

	case EditorFinishedMsg:
		if msg.Err != nil {
			m.err = msg.Err
		}
		return m, func() tea.Msg { return RefreshMsg{} }

	case TagSelectedMsg:
		if msg.Name == "hidden" {
			m.showHidden = true
		}
		cmd := m.enterAllTasksMode(true, "@"+msg.Name)
		return m, cmd

	case TagSearchExitMsg:
		m.exitToDashboard()
		return m, nil

	case tea.KeyMsg:
		m.err = nil // clear error on any key press
		return m.handleKey(msg)
	}

	return m, nil
}
