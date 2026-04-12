package tui

import (
	"github.com/zachthieme/pike/internal/config"
	"github.com/zachthieme/pike/internal/model"
)

// RefreshMsg triggers a re-scan of task files.
type RefreshMsg struct{}

// EditorFinishedMsg is sent after the editor process exits.
type EditorFinishedMsg struct{ Err error }

// toggleResultMsg is sent after a toggle operation completes.
type toggleResultMsg struct{ Err error }

// scanResultMsg is sent after a background scan completes.
type scanResultMsg struct {
	Tasks  []model.Task
	Config *config.Config
	Err    error
}

// viewMode tracks the current display mode.
type viewMode int

const (
	modeDashboard viewMode = iota
	modeFocused   // single section focus; Model.focusedView holds title
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
var filterPrompt = [...]string{
	filterSubstring: "/ ",
	filterQuery:     "? ",
}

func (v viewMode) String() string {
	switch v {
	case modeDashboard:
		return "dashboard"
	case modeFocused:
		return "focused"
	case modeAllTasks:
		return "all-tasks"
	case modeTagSearch:
		return "tag-search"
	case modeRecentlyCompleted:
		return "recently-completed"
	default:
		return "unknown"
	}
}

func (f filterMode) String() string {
	switch f {
	case filterSubstring:
		return "substring"
	case filterQuery:
		return "query"
	default:
		return "unknown"
	}
}

// --- FilterBar messages ---

type FilterActivateMsg struct {
	Mode         filterMode
	InitialValue string
	Placeholder  string
}

type FilterDeactivateMsg struct{}

type FilterSetErrorMsg struct{ Err error }

type FilterChangedMsg struct {
	Text string
	Mode filterMode
}

type FilterSubmittedMsg struct{}

type FilterClearedMsg struct{}

type FilterModeChangedMsg struct {
	Mode filterMode
}

// --- TagSearch messages ---

// TagSearchActivateMsg tells TagSearch to activate with the given tag list.
// Resets cursor and filter text (used when entering tag search mode).
type TagSearchActivateMsg struct {
	Tags []string
}

// TagSearchRefreshMsg updates the tag list without resetting cursor or filter.
// Used during background scans to avoid disrupting the user's position.
type TagSearchRefreshMsg struct {
	Tags []string
}

type TagSelectedMsg struct {
	Name string
}

type TagSearchExitMsg struct{}

// --- InputBar messages ---

// InputActivateMsg tells InputBar to activate with the given prompt and placeholder.
type InputActivateMsg struct {
	Prompt       string
	Placeholder  string
	InitialValue string
}

// InputDeactivateMsg tells InputBar to deactivate and clear.
type InputDeactivateMsg struct{}

// InputChangedMsg is emitted when the input text changes.
type InputChangedMsg struct {
	Text string
}

// InputSubmittedMsg is emitted when the user presses Enter.
type InputSubmittedMsg struct {
	Text string
}

// InputClearedMsg is emitted when the user dismisses the input (Escape on empty).
type InputClearedMsg struct{}

// --- CreateBar messages ---

// CreateActivateMsg tells CreateBar to activate for task creation.
type CreateActivateMsg struct{}

// CreateSubmittedMsg is emitted when the user submits a new task.
type CreateSubmittedMsg struct {
	Text string
}

// CreateClearedMsg is emitted when the user cancels task creation.
type CreateClearedMsg struct{}
