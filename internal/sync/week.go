// This file carries the Week half of a real Sync: it brings each Linked
// Task↔Todo pair's schedule into agreement when one side was rescheduled since
// the last Sync, respecting that HEY only knows Weeks (Sunday-to-Saturday) while
// a Task carries a day-level @due. A HEY-side reschedule — the Todo's Week moved
// and no longer contains the Task's @due day — sets @due to the new Week's
// Saturday. A notes-side reschedule — @due moved into a different Week — Re-creates
// the open Todo with the new date, sharing a single Re-create with any title
// change so the two never fire twice. A Task with no @due is never compared, and
// a completed Todo is never rescheduled.
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

// weekAction is how a Link's Week reconciliation resolves.
type weekAction int

const (
	weekNone     weekAction = iota // both sides already agree, or nothing may be done
	weekSetDue                     // set the Task's @due to the Todo's new Week (Saturday)
	weekRecreate                   // replace the Todo with a fresh one from the notes' new date
)

// resolveWeek decides how to bring a Task's @due day and its Todo's Week into
// agreement. due is the Task's @due day (nil when the Task has none); todoStart
// and todoEnd bound the Todo's current Week (Sunday..Saturday); baseStart is the
// Todo's Week Sunday recorded at the last Sync and hasBase reports whether the
// snapshot knew this Link. todoCompleted is whether the Todo is done: a completed
// Todo is never rescheduled or Re-created.
//
// A Task with no @due never has its Week compared. When @due already falls inside
// the Todo's current Week the two agree, so a Wednesday deadline is never
// flattened to Saturday. Otherwise, with a base, the side that moved @due out of
// the recorded Week is the one that changed: a notes-side move (including a move
// on both sides, where the notes win) Re-creates the Todo with the new date; a
// HEY-only move sets @due to the Todo's new Week's Saturday. Without a base
// neither side's change can be identified, so nothing is done.
func resolveWeek(due *time.Time, todoStart, todoEnd, baseStart time.Time, hasBase, todoCompleted bool) weekAction {
	if due == nil {
		return weekNone
	}
	if todoCompleted {
		return weekNone
	}
	if inWeek(*due, todoStart, todoEnd) {
		return weekNone
	}
	if !hasBase || baseStart.IsZero() {
		// Without a recorded Week — no snapshot, or a Link that never captured one
		// — neither side's move can be identified, so nothing is done.
		return weekNone
	}
	// The two disagree. If @due no longer falls inside the recorded Week, the
	// notes moved it — the notes win, even when HEY also moved its Week.
	if !inWeek(*due, baseStart, baseStart.AddDate(0, 0, 6)) {
		return weekRecreate
	}
	// @due still sits in the recorded Week, so HEY moved the Todo's Week.
	return weekSetDue
}

// inWeek reports whether day falls within the inclusive [start, end] Week span.
func inWeek(day, start, end time.Time) bool {
	return !day.Before(start) && !day.After(end)
}

// reconcileWeeks brings every Link with both a live linked Task and a matching
// HEY Todo into Week agreement. A HEY-side reschedule sets the Task's @due to the
// Todo's new Saturday; a notes-side reschedule Re-creates the Todo with the new
// date. When a Title Re-create will already replace the Todo this Sync, the
// reschedule rides along on that one Re-create — its date comes from the Task's
// @due — so this pass leaves it alone. It refreshes the Link in state and counts
// each outcome on rep. On a dry run it only counts, touching neither side nor the
// state. It reports whether state changed and any warnings from failed mutations,
// which do not stop the run. A Re-created Link is dropped from linkedTasks so the
// completion pass does not also act on the old Todo this Sync.
//
// origLinks is the state snapshot as it stood before any reconciliation ran this
// Sync. Decisions are made against it, not the live state, because an earlier
// pass (a Retitle) may have already advanced a Link's recorded Title and Week —
// resolving a reschedule against those mutated values would misread which side
// moved.
func reconcileWeeks(ctx context.Context, opts Options, linkedTasks map[string]*model.Task, todos []hey.Todo, origLinks map[string]Link, state *State, rep *Report) (bool, []model.Warning) {
	byID := make(map[string]hey.Todo, len(todos))
	for _, td := range todos {
		byID[td.ID] = td
	}
	var warnings []model.Warning
	dirty := false
	for id, t := range linkedTasks {
		td, ok := byID[id]
		if !ok {
			continue // the Todo is gone from HEY; the Orphan pass owns this Link
		}
		base, hasBase := origLinks[id]

		// A notes-side Title change already Re-creates this Todo with the Task's
		// @due, carrying any reschedule with it. Skip so the two share one Re-create.
		notesTitle := normalizeTitle(titleOf(t.Text))
		if resolveTitle(notesTitle, normalizeTitle(td.Title), normalizeTitle(base.Title), hasBase, td.Completed != nil) == titleRecreate {
			continue
		}

		act := resolveWeek(t.Due, td.WeekStart, td.WeekEnd, base.WeekStart, hasBase, td.Completed != nil)
		switch act {
		case weekNone:
			continue
		case weekSetDue:
			if opts.DryRun {
				rep.WouldReschedule++
				continue
			}
			if w := applySetDue(ctx, opts, id, t, td, state); w != nil {
				warnings = append(warnings, *w)
				continue
			}
			rep.Rescheduled++
			dirty = true
		case weekRecreate:
			if opts.DryRun {
				rep.WouldReschedule++
				continue
			}
			w, changed := applyRecreate(ctx, opts, id, t, notesTitle, state, linkedTasks)
			if w != nil {
				warnings = append(warnings, *w)
			}
			if changed {
				rep.Rescheduled++
				dirty = true
			}
		}
	}
	return dirty, warnings
}

// applySetDue rewrites the Task's @due to the Todo's new Week's Saturday and
// refreshes the Link snapshot's Week. It returns a non-nil Warning when the write
// fails; the rest of the line is left untouched.
func applySetDue(ctx context.Context, opts Options, id string, t *model.Task, td hey.Todo, state *State) *model.Warning {
	path := filepath.Join(opts.NotesDir, t.File)
	if err := toggle.SetDue(ctx, path, t.Line, td.WeekEnd); err != nil {
		msg := fmt.Sprintf("rescheduling task %s:%d: %v", t.File, t.Line, err)
		if errors.Is(err, toggle.ErrStaleData) {
			msg = fmt.Sprintf("rescheduling task %s:%d: line changed since scan; skipped", t.File, t.Line)
		}
		return &model.Warning{File: t.File, Line: t.Line, Message: msg}
	}
	link := state.Links[id]
	link.WeekStart = td.WeekStart
	link.Updated = td.Updated
	link.File = t.File
	link.Line = t.Line
	state.Links[id] = link
	return nil
}
