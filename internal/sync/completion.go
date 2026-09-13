// This file carries the completion half of a real Sync: it brings each Linked
// Task↔Todo pair into completion agreement. Completing or uncompleting on
// either side reaches the other on the next Sync, resolved against the state
// snapshot: a one-sided change since the last Sync propagates to the other side
// (in either direction, including uncomplete); a Link with no recorded snapshot
// — a missing or unreadable state file, or a brand-new Link — falls back to the
// conflict rule that completed wins.
package sync

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/zachthieme/pike/internal/hey"
	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/toggle"
)

// action is how a Link's completion reconciliation resolves.
type action int

const (
	actNone           action = iota // both sides already agree
	actCompleteTask                 // mark the Task done from HEY
	actUncompleteTask               // reopen the Task from HEY
	actCompleteTodo                 // mark the Todo done from the notes
	actUncompleteTodo               // reopen the Todo from the notes
)

// resolveCompletion decides how to bring a Task and its Todo into completion
// agreement. taskDone and todoDone are each side's current completion; base is
// the completion recorded at the last Sync and hasBase reports whether the
// state snapshot knew this Link.
//
// With a base, exactly one side can have changed when the two disagree, and
// that change propagates to the other side — a HEY completion becomes a Task
// completion, a Task reopen becomes a HEY uncomplete, and so on. Without a base
// (missing or unreadable state, or a Link the snapshot never recorded) the
// conflict rule applies: completed wins, so the open side is brought up to done.
func resolveCompletion(taskDone, todoDone, base, hasBase bool) action {
	if taskDone == todoDone {
		return actNone
	}
	if hasBase {
		// The two disagree, so exactly one side differs from the base: that is
		// the side that changed. Propagate its state to the other.
		if taskDone != base {
			if taskDone {
				return actCompleteTodo
			}
			return actUncompleteTodo
		}
		if todoDone {
			return actCompleteTask
		}
		return actUncompleteTask
	}
	// No base: completed wins.
	if taskDone {
		return actCompleteTodo
	}
	return actCompleteTask
}

// reconcileCompletions brings every Link with both a live linked Task and a
// matching HEY Todo into completion agreement. It applies each resolved action
// through the existing complete/uncomplete mutations, refreshes the Link in
// state, and counts the outcome on rep. On a dry run it only counts, touching
// neither side nor the state. It reports whether state changed and any warnings
// from failed mutations, which do not stop the run.
func reconcileCompletions(ctx context.Context, opts Options, linkedTasks map[string]*model.Task, todos []hey.Todo, state *State, rep *Report) (bool, []model.Warning) {
	byID := make(map[string]hey.Todo, len(todos))
	for _, td := range todos {
		byID[td.ID] = td
	}
	var warnings []model.Warning
	dirty := false
	for id, t := range linkedTasks {
		td, ok := byID[id]
		if !ok {
			continue // the Todo is gone from HEY; leave the Link untouched
		}
		changed, w := reconcileLink(ctx, opts, id, t, td, state, rep)
		if changed {
			dirty = true
		}
		if w != nil {
			warnings = append(warnings, *w)
		}
	}
	return dirty, warnings
}

// reconcileLink resolves one Task↔Todo pair. It returns whether the state entry
// changed and a non-nil Warning when a mutation fails.
func reconcileLink(ctx context.Context, opts Options, id string, t *model.Task, td hey.Todo, state *State, rep *Report) (bool, *model.Warning) {
	taskDone := t.State == model.Completed
	todoDone := td.Completed != nil
	base, hasBase := state.Links[id]
	act := resolveCompletion(taskDone, todoDone, base.Completed, hasBase)

	if opts.DryRun {
		switch act {
		case actCompleteTask, actCompleteTodo:
			rep.WouldComplete++
		case actUncompleteTask, actUncompleteTodo:
			rep.WouldUncomplete++
		}
		return false, nil
	}

	if w := applyAction(ctx, opts, act, id, t, td); w != nil {
		return false, w
	}
	switch act {
	case actCompleteTask, actCompleteTodo:
		rep.Completed++
	case actUncompleteTask, actUncompleteTodo:
		rep.Uncompleted++
	}

	// Record the agreed completion so the next Sync has a fresh base. done is
	// where both sides land once the action is applied.
	var done bool
	switch act {
	case actNone:
		done = taskDone
	case actCompleteTask, actCompleteTodo:
		done = true
	case actUncompleteTask, actUncompleteTodo:
		done = false
	}
	newLink := Link{
		Title:     base.Title,
		WeekStart: td.WeekStart,
		Completed: done,
		Updated:   td.Updated,
		File:      t.File,
		Line:      t.Line,
	}
	if newLink.Title == "" {
		newLink.Title = td.Title
	}
	if !hasBase || !linkEqual(base, newLink) {
		state.Links[id] = newLink
		return true, nil
	}
	return false, nil
}

// applyAction performs one resolved completion action, returning a Warning when
// the write fails. actNone writes nothing.
func applyAction(ctx context.Context, opts Options, act action, id string, t *model.Task, td hey.Todo) *model.Warning {
	switch act {
	case actNone:
		return nil
	case actCompleteTodo:
		if err := opts.Client.Complete(ctx, id); err != nil {
			return &model.Warning{File: t.File, Line: t.Line, Message: fmt.Sprintf("completing HEY todo %s: %v", id, err)}
		}
	case actUncompleteTodo:
		if err := opts.Client.Uncomplete(ctx, id); err != nil {
			return &model.Warning{File: t.File, Line: t.Line, Message: fmt.Sprintf("reopening HEY todo %s: %v", id, err)}
		}
	case actCompleteTask:
		path := filepath.Join(opts.NotesDir, t.File)
		if err := toggle.Complete(ctx, path, t.Line, completedDate(td, opts.Now)); err != nil {
			return completionWarning(t, "completing", err)
		}
	case actUncompleteTask:
		path := filepath.Join(opts.NotesDir, t.File)
		if err := toggle.Uncomplete(ctx, path, t.Line); err != nil {
			return completionWarning(t, "reopening", err)
		}
	}
	return nil
}

// completedDate maps a Todo's completed_at instant to the local calendar date
// pike stamps on the Task's @completed tag. now supplies the local zone, so a
// completed_at late in the day UTC lands on the correct local date.
func completedDate(td hey.Todo, now time.Time) time.Time {
	if td.Completed == nil {
		return now
	}
	return td.Completed.In(now.Location())
}

// completionWarning wraps a Task-side mutation failure, downgrading a stale-line
// error to a clearer message.
func completionWarning(t *model.Task, verb string, err error) *model.Warning {
	msg := fmt.Sprintf("%s task %s:%d: %v", verb, t.File, t.Line, err)
	if errors.Is(err, toggle.ErrStaleData) {
		msg = fmt.Sprintf("%s task %s:%d: line changed since scan", verb, t.File, t.Line)
	}
	return &model.Warning{File: t.File, Line: t.Line, Message: msg}
}

// linkEqual reports whether two Links carry the same recorded fields, comparing
// the time fields by instant rather than wall-clock representation.
func linkEqual(a, b Link) bool {
	return a.Title == b.Title &&
		a.Completed == b.Completed &&
		a.File == b.File &&
		a.Line == b.Line &&
		a.WeekStart.Equal(b.WeekStart) &&
		a.Updated.Equal(b.Updated)
}
