// This file carries the Orphan half of a real Sync. Per ADR 0002, Sync never
// deletes on either side. When a Linked Task's id has vanished from HEY, the
// @hey tag is stripped, the state entry dropped, and the outcome counted as an
// unlink — deleting the tag by hand is the supported way to un-link. When a
// Link's Task line can no longer be found in the notes, the surviving Todo is
// left alone and an Orphan Warning naming the id and Title goes to the caller.
package sync

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/zachthieme/pike/internal/hey"
	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/toggle"
)

// reconcileOrphans handles both Orphan cases. linkedTasks maps a HEY id to the
// live Task that Links to it; todos is HEY's current list. It strips the @hey
// tag and drops the state entry for every Linked Task whose Todo is gone from
// HEY (an unlink), and reports a Warning for every state entry whose Task can no
// longer be found (an Orphan), never deleting the surviving side. On a dry run
// it only counts. It returns whether state changed and any Warnings; a failed
// tag strip is a Warning that does not stop the run.
func reconcileOrphans(ctx context.Context, opts Options, linkedTasks map[string]*model.Task, todos []hey.Todo, state *State, rep *Report) (bool, []model.Warning) {
	live := make(map[string]bool, len(todos))
	for _, td := range todos {
		live[td.ID] = true
	}
	var warnings []model.Warning
	dirty := false

	// Unlink: a Linked Task whose Todo has vanished from HEY.
	for id, t := range linkedTasks {
		if live[id] {
			continue
		}
		if opts.DryRun {
			rep.WouldUnlink++
			continue
		}
		path := filepath.Join(opts.NotesDir, t.File)
		if err := toggle.RemoveTag(ctx, path, t.Line, t.Raw, "hey"); err != nil {
			warnings = append(warnings, unlinkWarning(t, id, err))
			rep.Failed++
			continue
		}
		delete(state.Links, id)
		dirty = true
		rep.Unlinked++
	}

	// Orphan: a state entry whose Task line can no longer be found in the notes.
	for id, link := range state.Links {
		if _, found := linkedTasks[id]; found {
			continue
		}
		if opts.DryRun {
			rep.WouldOrphan++
		} else {
			rep.Orphans++
		}
		warnings = append(warnings, model.Warning{
			File:    link.File,
			Line:    link.Line,
			Message: fmt.Sprintf("orphaned link: HEY todo %s %q has no task in the notes; left in place", id, link.Title),
		})
	}
	return dirty, warnings
}

// unlinkWarning wraps a failed @hey tag strip. An ambiguous line (two @hey tags)
// or a line that changed since the scan is skipped with nothing written.
func unlinkWarning(t *model.Task, id string, err error) model.Warning {
	msg := fmt.Sprintf("unlinking task %s:%d (hey %s): %v", t.File, t.Line, id, err)
	switch {
	case errors.Is(err, toggle.ErrAmbiguousTag):
		msg = fmt.Sprintf("unlinking task %s:%d (hey %s): line has more than one @hey tag; skipped", t.File, t.Line, id)
	case errors.Is(err, toggle.ErrStaleData):
		msg = fmt.Sprintf("unlinking task %s:%d (hey %s): line changed since scan; skipped", t.File, t.Line, id)
	}
	return model.Warning{File: t.File, Line: t.Line, Message: msg}
}
