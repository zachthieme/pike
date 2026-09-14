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
// live Task that Links to it; ambiguousIDs holds every id on a multi-@hey line
// (skipped by every pass, but whose id still has a Task line in the notes);
// todos is HEY's current list. It strips the @hey tag and drops the state entry
// for every Linked Task whose Todo is gone from HEY (an unlink), and reports a
// Warning for every state entry whose Task can no longer be found (an Orphan),
// never deleting the surviving side. An id that sits on an ambiguous line is
// never an Orphan and its state entry is never marked, rewritten, or dropped
// while the line stays ambiguous. On a dry run it only counts. It returns
// whether state changed and any Warnings; a failed tag strip is a Warning that
// does not stop the run.
func reconcileOrphans(ctx context.Context, opts Options, linkedTasks map[string]*model.Task, ambiguousIDs map[string]bool, todos []hey.Todo, state *State, rep *Report) (bool, []model.Warning) {
	byID := make(map[string]hey.Todo, len(todos))
	for _, td := range todos {
		byID[td.ID] = td
	}
	var warnings []model.Warning
	dirty := false

	// Unlink: a Linked Task whose Todo has vanished from HEY.
	for id, t := range linkedTasks {
		if _, live := byID[id]; live {
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

	// State entries with no live linked Task fall into two kinds: a PendingDelete
	// pike is still clearing, and an Orphan whose Task line is gone but whose Todo
	// survives in HEY.
	for id, link := range state.Links {
		if ambiguousIDs[id] {
			// The id sits on a multi-@hey line, skipped by every pass. Its Task line
			// exists, so it is not an Orphan; leave the entry exactly as it stands —
			// never mark, rewrite, or drop it while the line stays ambiguous, even
			// when its Todo is completed or gone from HEY.
			continue
		}
		if _, found := linkedTasks[id]; found {
			// The Link is live again (its Task line was restored). Clear a stale
			// Orphaned mark so a later disappearance warns afresh rather than being
			// silenced by a warning from a previous orphan episode.
			if link.Orphaned && !opts.DryRun {
				link.Orphaned = false
				state.Links[id] = link
				dirty = true
			}
			continue
		}
		td, inHey := byID[id]

		if link.PendingDelete {
			if retryPendingDelete(ctx, opts, id, td, inHey, state) {
				dirty = true
			}
			continue
		}

		if orphanDropped(opts, id, td, inHey, state) {
			dirty = true
			continue
		}
		if opts.DryRun {
			rep.WouldOrphan++
			if !link.Orphaned {
				warnings = append(warnings, orphanWarning(id, link))
			}
			continue
		}
		rep.Orphans++
		if !link.Orphaned {
			warnings = append(warnings, orphanWarning(id, link))
			link.Orphaned = true
			state.Links[id] = link
			dirty = true
		}
	}
	return dirty, warnings
}

// retryPendingDelete re-attempts the delete of a Todo pike created or replaced
// but could not remove — a rolled-back push add or a superseded Re-create. It is
// silent (a repeating Warning is the very thing #2 forbids) and never imports
// the Todo. The entry is dropped, reporting dirty, once the Todo is gone from
// HEY or completed; while the Todo is still listed the delete is retried but the
// entry is kept, because this Sync's Todo list — captured before the delete —
// still contains the Todo, and dropping the entry now would let the import pass
// re-add it. A dry run touches nothing. It reports whether it dropped the entry
// (state changed).
func retryPendingDelete(ctx context.Context, opts Options, id string, td hey.Todo, inHey bool, state *State) bool {
	if opts.DryRun {
		return false
	}
	if !inHey || td.Completed != nil {
		delete(state.Links, id)
		return true
	}
	// Best-effort: a failure just means the next Sync retries. Either way the
	// entry stays until a later List confirms the Todo is gone.
	_ = opts.Client.Delete(ctx, id) //nolint:errcheck // retried next Sync; kept until the Todo leaves HEY's list
	return false
}

// orphanDropped drops an Orphan's state entry once its surviving Todo is gone
// from HEY or completed, so the Orphan Warning stops. It reports whether it
// dropped the entry. A dry run never mutates state, so it drops nothing.
func orphanDropped(opts Options, id string, td hey.Todo, inHey bool, state *State) bool {
	if inHey && td.Completed == nil {
		return false
	}
	if opts.DryRun {
		return false
	}
	delete(state.Links, id)
	return true
}

// orphanWarning is the Warning reported for an Orphan: a Link whose Task line is
// gone from the notes while its Todo survives in HEY.
func orphanWarning(id string, link Link) model.Warning {
	return model.Warning{
		File:    link.File,
		Line:    link.Line,
		Message: fmt.Sprintf("orphaned link: HEY todo %s %q has no task in the notes; left in place", id, link.Title),
	}
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
