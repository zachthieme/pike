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
	"sort"

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
// while the line stays ambiguous — unless it carries a PendingDelete, which is
// retried and dropped on its own terms regardless of the line. On a dry run it
// only counts. It returns
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

		// A pending delete is retried and dropped on its own terms — before the
		// ambiguous-line skip below — whether or not its id also sits on an
		// ambiguous line. Otherwise the skip would strand the leaked Todo in HEY
		// forever and leave its state entry unreachable.
		if link.PendingDelete {
			dropped, outstanding, w := retryPendingDelete(ctx, opts, id, td, inHey, link, state)
			if dropped {
				dirty = true
			}
			if w != nil {
				warnings = append(warnings, *w)
				rep.Failed++
			}
			// Name the pending delete only if it is still outstanding once the run
			// ends: a real run's retry that cleared the Todo (or found it already
			// gone) is not named, even though its state entry lingers a run longer.
			if outstanding {
				rep.PendingDeletes = append(rep.PendingDeletes, Item{ID: id, Title: link.Title})
			}
			continue
		}

		if ambiguousIDs[id] {
			// The id sits on a multi-@hey line, skipped by every pass. Its Task line
			// exists, so it is not an Orphan; leave the entry exactly as it stands —
			// never mark, rewrite, or drop it while the line stays ambiguous, even
			// when its Todo is completed or gone from HEY.
			continue
		}

		if orphanCleared(td, inHey) {
			// The surviving Todo is gone from HEY or completed there: a real Sync
			// drops the entry and stops warning, and a dry run reports what that run
			// would leave — nothing outstanding for this id. So it is neither counted,
			// named, nor warned, in a dry run as in a real run.
			if !opts.DryRun {
				delete(state.Links, id)
				dirty = true
			}
			continue
		}
		if opts.DryRun {
			rep.WouldOrphan++
			rep.OrphanItems = append(rep.OrphanItems, Item{ID: id, Title: link.Title})
			if !link.Orphaned {
				warnings = append(warnings, orphanWarning(id, link))
			}
			continue
		}
		rep.Orphans++
		// Name every Orphan counted, independently of the warn-once rule, so a
		// later Sync that no longer warns still lists it.
		rep.OrphanItems = append(rep.OrphanItems, Item{ID: id, Title: link.Title})
		if !link.Orphaned {
			warnings = append(warnings, orphanWarning(id, link))
			link.Orphaned = true
			state.Links[id] = link
			dirty = true
		}
	}
	// Both lists are collected by iterating state.Links, a map, so sort them by id
	// to give the report a stable order — two runs over unchanged state then render
	// byte-identically and diffing reports stays quiet.
	sortItems(rep.OrphanItems)
	sortItems(rep.PendingDeletes)
	return dirty, warnings
}

// sortItems orders reported Items by id in place.
func sortItems(items []Item) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}

// retryPendingDelete re-attempts the delete of a Todo pike created or replaced
// but could not remove — a rolled-back push add or a superseded Re-create. It
// never imports the Todo. The entry is dropped, reporting dirty, once the Todo
// is gone from HEY or completed; while the Todo is still listed the delete is
// retried but the entry is kept, because this Sync's Todo list — captured before
// the delete — still contains the Todo, and dropping the entry now would let the
// import pass re-add it. A dry run touches nothing. A failed delete does not
// stop the run, but unlike the warn-once Orphan case it is not silent: it
// returns a Warning naming the id so the user is not misled into thinking the
// leaked Todo is gone (#32). It reports whether it dropped the entry (state
// changed), whether the delete is still outstanding once the run ends (so the
// report can name only what a real run leaves un-cleared), and, when the retried
// delete failed, a Warning to surface.
func retryPendingDelete(ctx context.Context, opts Options, id string, td hey.Todo, inHey bool, link Link, state *State) (dropped, outstanding bool, w *model.Warning) {
	if opts.DryRun {
		// Nothing is attempted; report what a real run would leave outstanding — a
		// Todo still live in HEY and open, the same case whose entry a real run
		// keeps for a retry.
		return false, inHey && td.Completed == nil, nil
	}
	if !inHey || td.Completed != nil {
		delete(state.Links, id)
		return true, false, nil
	}
	// The entry stays until a later List confirms the Todo is gone. A failure
	// just means the next Sync retries — but it is surfaced, not swallowed, and
	// the leak is still outstanding. A success clears the Todo from HEY now, so it
	// is no longer outstanding even though the entry lingers one more run.
	if err := opts.Client.Delete(ctx, id); err != nil {
		pw := pendingDeleteWarning(id, link, err)
		return false, true, &pw
	}
	return false, false, nil
}

// pendingDeleteWarning wraps a failed retry of a pending delete: pike could not
// remove a Todo it created or replaced, and until it is gone the reset procedure
// would re-import it as a duplicate. It names the id and Title so the user can
// remove it by hand.
func pendingDeleteWarning(id string, link Link, err error) model.Warning {
	return model.Warning{
		File:    link.File,
		Line:    link.Line,
		Message: fmt.Sprintf("pending delete: could not remove HEY todo %s %q; will retry next sync: %v", id, link.Title, err),
	}
}

// orphanCleared reports whether an Orphan's surviving Todo is gone from HEY or
// completed there — the condition under which a real Sync drops the entry and
// stops warning, and a dry run treats the id as no longer outstanding.
func orphanCleared(td hey.Todo, inHey bool) bool {
	return !inHey || td.Completed != nil
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
