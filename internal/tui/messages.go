package tui

import (
	"pike/internal/config"
	"pike/internal/model"
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

type TagSearchActivateMsg struct {
	Tags []string
}

type TagSelectedMsg struct {
	Name string
}

type TagSearchExitMsg struct{}
