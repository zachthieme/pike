package tui

import (
	"time"

	"pike/internal/config"
	"pike/internal/filter"
	"pike/internal/model"

	tea "github.com/charmbracelet/bubbletea"
)

// Model is the main Bubbletea model for the tasks TUI.
type Model struct {
	config      *config.Config
	allTasks    []model.Task
	sections    []filter.ViewResult
	// unfilteredSections caches the full (pre-filter) view results so
	// visibleSections() doesn't have to recompute every query on each keypress.
	unfilteredSections []filter.ViewResult
	hiddenCounts       []int        // per-section count of @hidden tasks that were removed
	openCount          int          // cached count of open checkbox tasks, updated on rebuild
	completedThisWeek  int          // cached count of tasks completed since start of week
	cursor             int          // index into flat task list across all sections
	focusedView        string       // "" = dashboard, otherwise title of focused section
	viewLocked         bool         // when true, block mode-switching keys and prevent unfocusing (set via --view flag)
	showSummary        bool
	filterBar          FilterBar
	tagSearch          TagSearch
	mode               viewMode
	showHidden         bool     // whether to show @hidden tasks
	showAll            bool     // when true, all-tasks includes completed (e.g. from tag search)
	width              int
	height             int
	err                error
	scanFunc           func() ([]model.Task, error)  // injected for refresh
	configFunc         func() (*config.Config, error) // injected for config reload
	editorCmd          string
	tagColors          map[string]string
	keys               KeyMap
	version            string
	now                func() time.Time // injectable for testing
}

// NewModel creates a new TUI model with the given configuration and initial tasks.
func NewModel(cfg *config.Config, tasks []model.Task, scanFunc func() ([]model.Task, error), configFunc ...func() (*config.Config, error)) Model {
	m := Model{
		config:      cfg,
		allTasks:    tasks,
		focusedView: "",
		filterBar:   NewFilterBar(),
		tagSearch:   NewTagSearch(),
		scanFunc:    scanFunc,
		editorCmd:   cfg.Editor,
		tagColors:   cfg.TagColors,
		keys:        DefaultKeyMap(),
		now:         time.Now,
	}
	if len(configFunc) > 0 {
		m.configFunc = configFunc[0]
	}

	m.rebuildSections()
	m.clampCursor()

	return m
}

// SetVersion sets the version string for display in the summary overlay.
func (m *Model) SetVersion(v string) {
	m.version = v
}

// SetFocusedView sets the focused view by section title, locks the view so
// mode-switching keys (a/t/s/c/1-9) are disabled, and rebuilds sections.
func (m *Model) SetFocusedView(title string) {
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
	m.clampCursor()
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
		}
		if msg.Tasks != nil {
			m.allTasks = msg.Tasks
			if m.mode == modeTagSearch {
				tags := extractTagNames(m.allTasks)
				m.tagSearch, _ = m.tagSearch.Update(TagSearchActivateMsg{Tags: tags})
			}
		}
		// Rebuild sections if tasks or config changed (config affects views, tag colors).
		if msg.Tasks != nil || msg.Config != nil {
			m.rebuildSections()
			m.clampCursor()
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
		m.mode = modeAllTasks
		m.showAll = true
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
