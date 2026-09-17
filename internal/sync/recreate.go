// This file carries the Re-create step of a Sync's Title reconciliation. HEY
// offers no way to edit a Todo's Title in place, so a notes-side rename is
// propagated by replacing the Todo: add a fresh Todo with the new Title and the
// Task's date, rewrite the Task's @hey tag to the new id, then delete the old
// Todo — in that order, so a failure part-way never loses the Todo. Once the
// tag points at the new Todo the Re-create has succeeded; a failed delete only
// leaves the old Todo behind for the next Sync to unlink or the user to remove.
package sync

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/toggle"
)

// applyRecreate replaces the Todo id is Linked to with a fresh one carrying
// newTitle, moving the Link. It returns a Warning when a step fails, whether the
// Re-create succeeded (the Link now points at the new Todo), whether the state
// changed (so it must be saved), and whether the delete of the replaced Todo
// failed after an otherwise-successful Re-create (a per-item failure the caller
// counts, distinct from the Re-create itself succeeding). A successful Re-create
// drops the old id from linkedTasks so the completion pass does not also act on
// the Todo being replaced this Sync.
func applyRecreate(ctx context.Context, opts Options, id string, t *model.Task, newTitle string, state *State, linkedTasks map[string]*model.Task) (w *model.Warning, recreated, changedState, deleteFailed bool) {
	newTodo, err := opts.Client.Add(ctx, newTitle, t.Due)
	if err != nil {
		return &model.Warning{File: t.File, Line: t.Line, Message: fmt.Sprintf("re-creating %q in HEY: %v", newTitle, err)}, false, false, false
	}

	// Move the Link before deleting the old Todo, so a failure here leaves the
	// old Todo reachable rather than orphaning the freshly-added one silently.
	path := filepath.Join(opts.NotesDir, t.File)
	if err := toggle.SetTagValue(ctx, path, t.Line, t.Raw, "hey", newTodo.ID); err != nil {
		msg := fmt.Sprintf("re-linking task %s:%d (hey %s) to new HEY todo %s: %v", t.File, t.Line, id, newTodo.ID, err)
		if errors.Is(err, toggle.ErrStaleData) {
			msg = fmt.Sprintf("re-linking task %s:%d (hey %s) to new HEY todo %s: line changed since scan; skipped", t.File, t.Line, id, newTodo.ID)
		}
		// Nothing in the notes records the new Todo, so roll it back exactly as a
		// refused Push add is: delete the Todo just added, recording a PendingDelete
		// if that delete also fails so the next Sync retries it and it is never
		// imported. The old id's Link and state entry are left untouched.
		dirty := rollBackAdd(ctx, opts, newTodo.ID, newTitle, t, state)
		return &model.Warning{File: t.File, Line: t.Line, Message: msg}, false, dirty, false
	}

	// The Link now points at the new Todo: record it and forget the old one.
	delete(state.Links, id)
	state.Links[newTodo.ID] = Link{
		Title:     newTitle,
		WeekStart: newTodo.WeekStart,
		Completed: newTodo.Completed != nil,
		Updated:   newTodo.Updated,
		File:      t.File,
		Line:      t.Line,
	}
	delete(linkedTasks, id)

	// Deleting the old Todo is best-effort cleanup; the Re-create has already
	// succeeded, so a failure is only a Warning. But the old id must not be
	// forgotten: were it dropped, the still-open old Todo — unknown to pike —
	// would be imported next Sync as a second copy of the Task. Keep it in state
	// marked superseded (a pending delete) so it is never imported and the next
	// Sync retries the delete.
	if err := opts.Client.Delete(ctx, id); err != nil {
		state.Links[id] = Link{PendingDelete: true, Title: newTitle, File: t.File, Line: t.Line}
		return &model.Warning{File: t.File, Line: t.Line, Message: fmt.Sprintf("deleting replaced HEY todo %s: %v; left in place", id, err)}, true, true, true
	}
	return nil, true, true, false
}
