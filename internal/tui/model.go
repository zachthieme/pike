package tui

import (
	"time"

	"pike/internal/config"
	"pike/internal/filter"
	"pike/internal/model"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RefreshMsg triggers a re-scan of task files.
type RefreshMsg struct{}

// EditorFinishedMsg is sent after the editor process exits.
type EditorFinishedMsg struct{ Err error }

// viewMode tracks the current display mode.
type viewMode int

const (
	modeDashboard viewMode = iota
	modeAllTasks
	modeTagSearch
	modeRecentlyCompleted
)

// filterMode tracks whether the filter bar uses substring or DSL matching.
type filterMode int

const (
	filterSubstring filterMode = iota
	filterQuery
)

// filterPrompt maps each filter mode to its prompt string.
var filterPrompt = map[filterMode]string{
	filterSubstring: "/ ",
	filterQuery:     "? ",
}

// Model is the main Bubbletea model for the tasks TUI.
type Model struct {
	config      *config.Config
	allTasks    []model.Task
	sections     []filter.ViewResult
	hiddenCounts []int // per-section count of @hidden tasks that were removed
	cursor       int   // index into flat task list across all sections
	focusedView string // "" = dashboard, otherwise title of focused section
	viewLocked  bool   // when true, block mode-switching keys and prevent unfocusing (set via --view flag)
	showSummary bool
	filterInput textinput.Model
	filtering   bool
	filterText  string
	filterMode  filterMode
	mode        viewMode
	tagList     []string // unique tags for tag search mode
	tagCursor   int      // cursor in tag list
	showHidden  bool     // whether to show @hidden tasks
	showAll     bool     // when true, all-tasks includes completed (e.g. from tag search)
	width       int
	height      int
	err         error
	queryErr    error  // DSL parse error shown when filterMode is filterQuery
	scanFunc    func() ([]model.Task, error)             // injected for refresh
	configFunc  func() (*config.Config, error)            // injected for config reload
	editorCmd   string
	tagColors   map[string]string
	keys        KeyMap
	version     string
	now         func() time.Time // injectable for testing
}

// NewModel creates a new TUI model with the given configuration and initial tasks.
func NewModel(cfg *config.Config, tasks []model.Task, scanFunc func() ([]model.Task, error), configFunc ...func() (*config.Config, error)) Model {
	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 256
	ti.Prompt = "/ "
	ti.PromptStyle = lipgloss.NewStyle().Bold(true)
	ti.PlaceholderStyle = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("7"))

	m := Model{
		config:      cfg,
		allTasks:    tasks,
		focusedView: "",
		filterInput: ti,
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
		// Hot-reload config (tag colors, views, etc.)
		if m.configFunc != nil {
			if cfg, err := m.configFunc(); err == nil {
				m.config = cfg
				m.tagColors = cfg.TagColors
				m.editorCmd = cfg.Editor
			}
		}
		if m.scanFunc != nil {
			tasks, err := m.scanFunc()
			if err != nil {
				m.err = err
				return m, nextTick
			}
			m.allTasks = tasks
			if m.mode == modeTagSearch {
				m.buildTagList()
				if tags := m.filteredTags(); len(tags) > 0 {
					m.tagCursor = min(m.tagCursor, len(tags)-1)
				} else {
					m.tagCursor = 0
				}
			}
			m.rebuildSections()
			m.clampCursor()
		}
		return m, nextTick

	case EditorFinishedMsg:
		if msg.Err != nil {
			m.err = msg.Err
		}
		return m, func() tea.Msg { return RefreshMsg{} }

	case tea.KeyMsg:
		m.err = nil // clear error on any key press
		return m.handleKey(msg)
	}

	return m, nil
}
