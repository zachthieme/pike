// This file carries the TUI's Push: sending a single Task completion change to
// its Linked Todo immediately when a Task is toggled, outside a full Sync. It
// also holds the helpers that gate HEY integration and locate a Task's Link.
package tui

import (
	"context"
	"fmt"

	"github.com/zachthieme/pike/internal/model"
	heysync "github.com/zachthieme/pike/internal/sync"

	tea "github.com/charmbracelet/bubbletea"
)

// heyEnabled reports whether HEY integration is configured: a client was
// injected and a hey: block is present. When false, Push on toggle and the sync
// action are no-ops.
func (m Model) heyEnabled() bool {
	return m.heyClient != nil && m.config != nil && m.config.Hey != nil
}

// heyLinkID returns the HEY todo id a Task is Linked to, if any — the value of
// its @hey(id) tag.
func heyLinkID(t model.Task) (string, bool) {
	for _, tag := range t.Tags {
		if tag.Name == "hey" && tag.Value != "" {
			return tag.Value, true
		}
	}
	return "", false
}

// runSync runs a full HEY Sync as a Bubble Tea command: it pushes Eligible
// Tasks, imports unlinked open Todos, and reconciles completion in both
// directions, then reports a one-line summary. It is a no-op (nil command) when
// HEY is disabled, so the sync key does nothing without a hey: block.
func (m Model) runSync() tea.Cmd {
	if !m.heyEnabled() {
		return nil
	}
	client := m.heyClient
	cfg := m.config
	tasks := m.allTasks
	now := m.nowFunc()
	return func() tea.Msg {
		rep, warnings, err := heysync.Push(context.Background(), heysync.Options{
			Tasks:     tasks,
			Client:    client,
			Query:     cfg.Hey.Query,
			StatePath: cfg.Hey.StatePath,
			NotesDir:  cfg.NotesDir,
			InboxFile: cfg.InboxFile,
			Now:       now,
		})
		if err != nil {
			return syncResultMsg{Err: err}
		}
		return syncResultMsg{Summary: syncSummaryLine(rep, len(warnings))}
	}
}

// syncSummaryLine renders a Sync Report as a single status-line string.
func syncSummaryLine(rep *heysync.Report, warnings int) string {
	s := fmt.Sprintf("sync: %d pushed, %d imported, %d completed, %d uncompleted",
		rep.Pushed, rep.Imported, rep.Completed, rep.Uncompleted)
	if rep.Failed > 0 {
		s += fmt.Sprintf(", %d failed", rep.Failed)
	}
	if warnings > 0 {
		s += fmt.Sprintf(" (%d warning(s))", warnings)
	}
	return s
}

// pushCompletion sends a completion change for each Linked id to HEY and records
// the new completed flag in the state file so the next Sync does not resend it.
// done is the state both the toggled Task and any auto-completed parent land in.
// It is a no-op when HEY is disabled or there are no Linked ids. A failed HEY
// call returns an error without saving state; the notes change has already been
// written and stays.
func (m Model) pushCompletion(ctx context.Context, ids []string, done bool) error {
	if !m.heyEnabled() || len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		var err error
		verb := "completing"
		if done {
			err = m.heyClient.Complete(ctx, id)
		} else {
			verb = "reopening"
			err = m.heyClient.Uncomplete(ctx, id)
		}
		if err != nil {
			return fmt.Errorf("%s HEY todo %s: %w", verb, id, err)
		}
	}
	return m.recordCompletion(ids, done)
}

// recordCompletion updates the completed flag of each named Link in the state
// file. Links absent from the state are left alone: a full Sync reconciles them
// from scratch. An empty state path is a no-op.
func (m Model) recordCompletion(ids []string, done bool) error {
	path := m.config.Hey.StatePath
	if path == "" {
		return nil
	}
	state, err := heysync.LoadState(path)
	if err != nil {
		return fmt.Errorf("reading hey state: %w", err)
	}
	changed := false
	for _, id := range ids {
		link, ok := state.Links[id]
		if !ok || link.Completed == done {
			continue
		}
		link.Completed = done
		state.Links[id] = link
		changed = true
	}
	if !changed {
		return nil
	}
	if err := heysync.SaveState(path, state); err != nil {
		return fmt.Errorf("writing hey state: %w", err)
	}
	return nil
}
