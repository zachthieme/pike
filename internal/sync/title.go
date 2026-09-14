// This file carries the Title half of a real Sync: it brings each Linked
// Task↔Todo pair's Title into agreement when one side was renamed since the
// last Sync. A HEY-side rename is a Retitle — the Task's text is rewritten to
// the new Title with its Tags re-appended in their original order. A notes-side
// rename (or a rename on both sides, where the notes win) is a Re-create — HEY
// offers no way to edit a Todo in place, so a fresh Todo is added and the Link
// moved to it. Completed Todos are never Retitled or Re-created.
package sync

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/zachthieme/pike/internal/hey"
	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/toggle"
)

// titleAction is how a Link's Title reconciliation resolves.
type titleAction int

const (
	titleNone     titleAction = iota // both sides already agree, or nothing may be done
	titleRetitle                     // rewrite the Task's text from HEY's Title
	titleRecreate                    // replace the Todo with a fresh one from the notes
)

// resolveTitle decides how to bring a Task and its Todo into Title agreement.
// notesTitle is the Task's text with Tags stripped, heyTitle is the Todo's
// current Title, and baseTitle is the Title recorded at the last Sync; hasBase
// reports whether the snapshot knew this Link. todoCompleted is whether the
// Todo is done: a completed Todo is never Retitled or Re-created.
//
// When the two sides disagree and a base is known, the side that differs from
// the base is the one that changed. A notes-side change wins — including when
// both sides changed — and is propagated to HEY by Re-creating the Todo; a
// HEY-only change is propagated to the notes by Retitling the Task. Without a
// base (a missing or unreadable state file, or a Link the snapshot never
// recorded) neither side's change can be identified, so the notes win on text
// and the Todo is Re-created.
func resolveTitle(notesTitle, heyTitle, baseTitle string, hasBase, todoCompleted bool) titleAction {
	if todoCompleted {
		return titleNone
	}
	if notesTitle == heyTitle {
		return titleNone
	}
	if !hasBase {
		// No snapshot (missing or unreadable state, or a Link never recorded):
		// neither side's change can be identified, so the notes win on text and
		// the Todo is Re-created to carry the notes' Title.
		return titleRecreate
	}
	if notesTitle != baseTitle {
		return titleRecreate
	}
	if heyTitle != baseTitle {
		return titleRetitle
	}
	return titleNone
}

// normalizeTitle collapses runs of whitespace to single spaces and trims the
// ends, matching how titleOf derives a Todo Title from a Task's text so a
// Title unchanged since the last Sync compares equal on both sides.
func normalizeTitle(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// reconcileTitles brings every Link with both a live linked Task and a matching
// HEY Todo into Title agreement. It applies each resolved action — a Retitle
// rewrites the Task's text, a Re-create replaces the Todo — refreshes the Link
// in state, and counts the outcome on rep. On a dry run it only counts, touching
// neither side nor the state. It reports whether state changed and any warnings
// from failed mutations, which do not stop the run. A Re-created Link is dropped
// from linkedTasks so the completion pass does not also act on the old Todo this
// Sync.
func reconcileTitles(ctx context.Context, opts Options, linkedTasks map[string]*model.Task, todos []hey.Todo, state *State, rep *Report) (bool, []model.Warning) {
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
		base, hasBase := state.Links[id]
		notesTitle := normalizeTitle(titleOf(t.Text))
		heyTitle := normalizeTitle(td.Title)
		act := resolveTitle(notesTitle, heyTitle, normalizeTitle(base.Title), hasBase, td.Completed != nil)

		switch act {
		case titleNone:
			continue
		case titleRetitle:
			if opts.DryRun {
				rep.WouldRetitle++
				continue
			}
			if w := applyRetitle(ctx, opts, id, t, heyTitle, td, state); w != nil {
				warnings = append(warnings, *w)
				rep.Failed++
				continue
			}
			rep.Retitled++
			dirty = true
		case titleRecreate:
			if opts.DryRun {
				rep.WouldRecreate++
				continue
			}
			w, changed, changedState := applyRecreate(ctx, opts, id, t, notesTitle, state, linkedTasks)
			if w != nil {
				warnings = append(warnings, *w)
			}
			if changed {
				rep.Recreated++
			} else {
				rep.Failed++
			}
			if changedState {
				dirty = true
			}
		}
	}
	return dirty, warnings
}

// applyRetitle rewrites the Task's text to HEY's Title, keeping its Tags, and
// refreshes the Link snapshot. HEY's Title is written through encodeTitle — the
// same encoding the import path uses — so a Title carrying "@" text (an email
// address, a "@rent", an "@due(...)") plants no stray tag on the line and does
// not give it a second @due; the snapshot still records the decoded Title as HEY
// holds it. It returns a non-nil Warning when the write fails; the checkbox state
// is left untouched.
func applyRetitle(ctx context.Context, opts Options, id string, t *model.Task, newTitle string, td hey.Todo, state *State) *model.Warning {
	path := filepath.Join(opts.NotesDir, t.File)
	if err := toggle.SetText(ctx, path, t.Line, t.Raw, encodeTitle(newTitle)); err != nil {
		msg := fmt.Sprintf("retitling task %s:%d (hey %s): %v", t.File, t.Line, id, err)
		if errors.Is(err, toggle.ErrStaleData) {
			msg = fmt.Sprintf("retitling task %s:%d (hey %s): line changed since scan; skipped", t.File, t.Line, id)
		}
		return &model.Warning{File: t.File, Line: t.Line, Message: msg}
	}
	// Keep t.Raw abreast of the write so a reschedule or completion later this
	// Sync verifies against the retitled line, not the line as first scanned.
	refreshScannedLine(t, path)
	link := state.Links[id]
	link.Title = newTitle
	link.WeekStart = td.WeekStart
	link.Updated = td.Updated
	link.File = t.File
	link.Line = t.Line
	state.Links[id] = link
	return nil
}
