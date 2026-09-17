// Package sync reconciles Tasks in the notes with Todos in HEY. This file
// carries the planning pass behind `pike --sync --dry-run`: it reports what a
// Sync would do without changing anything.
package sync

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zachthieme/pike/internal/hey"
	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/query"
)

// Options configures a planning pass.
type Options struct {
	Tasks     []model.Task // all Tasks scanned from the notes
	Client    hey.Client   // HEY client (real or fake)
	Query     string       // the Sync Query selecting Eligible Tasks to push
	StatePath string       // path to the sync state file, may be empty or absent
	NotesDir  string       // notes directory, joined with a Task's File to locate it
	InboxFile string       // notes-relative Inbox file for imports; "" means inbox.md
	Now       time.Time    // reference time for query evaluation
	DryRun    bool         // when true, nothing is written on either side
}

// Plan computes what a Sync would do and returns a Report plus any non-fatal
// Warnings. It returns an error only when HEY cannot be reached (the command
// cannot run or its first call reports an authentication error). It never
// writes to the notes, HEY, or the state file.
func Plan(ctx context.Context, opts Options) (*Report, []model.Warning, error) {
	var warnings []model.Warning

	// Reading the state file is best-effort: an absent file is normal on the
	// first run; a corrupt one is a Warning, not a failure.
	state, err := LoadState(opts.StatePath)
	if err != nil {
		warnings = append(warnings, model.Warning{Message: fmt.Sprintf("reading hey state: %v", err)})
		state = &State{Links: make(map[string]Link)}
	}

	node, err := query.Parse(opts.Query)
	if err != nil {
		return nil, warnings, fmt.Errorf("sync query: %w", err)
	}

	rep := &Report{DryRun: opts.DryRun}
	linkedIDs, linkedTasks, ambiguousIDs, toPush, classifyWarnings := classifyTasks(opts.Tasks, node, opts.Now, rep)
	warnings = append(warnings, classifyWarnings...)
	rep.WouldPush += len(toPush)

	// A failed first HEY call is the one fatal case for --sync.
	todos, err := opts.Client.List(ctx)
	if err != nil {
		return nil, warnings, fmt.Errorf("listing HEY todos: %w", err)
	}
	for _, td := range todos {
		// A Todo pike already holds a Link for is not an unlinked Todo, so it is
		// never a would-import — even when its Task line is gone (an Orphan).
		_, known := state.Links[td.ID]
		if td.Completed == nil && !linkedIDs[td.ID] && !known {
			rep.WouldImport++
		}
	}

	// Plan never writes, so it resolves no pending deletes; cleared stays empty and
	// the end-of-run list is filtered by what a real run would leave outstanding.
	cleared := make(map[string]bool)
	_, orphanWarnings := reconcileOrphans(ctx, opts, linkedTasks, ambiguousIDs, todos, state, rep, cleared)
	warnings = append(warnings, orphanWarnings...)

	origLinks := snapshotLinks(state)

	_, titleWarnings := reconcileTitles(ctx, opts, linkedTasks, todos, state, rep)
	warnings = append(warnings, titleWarnings...)

	_, weekWarnings := reconcileWeeks(ctx, opts, linkedTasks, todos, origLinks, state, rep)
	warnings = append(warnings, weekWarnings...)

	_, completionWarnings := reconcileCompletions(ctx, opts, linkedTasks, todos, state, rep)
	warnings = append(warnings, completionWarnings...)

	// Name what a real run would leave outstanding, read from the (unchanged) state:
	// every pending delete whose Todo is still live and open in HEY.
	rep.PendingDeletes = collectPendingDeletes(state, todos, cleared, opts.DryRun)

	return rep, warnings, nil
}

// classifyTasks partitions the scanned Tasks for a Sync. It records every Linked
// id in linkedIDs (so its Todo is never imported), maps each singly-Linked id to
// its live Task in linkedTasks (so the reconciliation passes can act on it), and
// returns the Tasks eligible to push that match the Sync Query.
//
// A line carrying more than one @hey tag is ambiguous: pike cannot tell which id
// the line means, so it is skipped by every pass (it is left out of linkedTasks)
// while every id on it still counts as Linked (so neither Todo is imported) and
// as having a Task line (recorded in ambiguousIDs, so Orphan detection never
// mistakes such an id's state entry for one whose line is gone). One Warning per
// such line is returned. Ambiguous and singly-Linked Tasks both count as
// ExistingLinks, so a Plan and a Push classify identically.
func classifyTasks(tasks []model.Task, node query.Node, now time.Time, rep *Report) (linkedIDs map[string]bool, linkedTasks map[string]*model.Task, ambiguousIDs map[string]bool, toPush []*model.Task, warnings []model.Warning) {
	linkedIDs = make(map[string]bool)
	linkedTasks = make(map[string]*model.Task)
	ambiguousIDs = make(map[string]bool)
	for i := range tasks {
		t := &tasks[i]
		ids := linkIDs(t)
		switch {
		case len(ids) > 1:
			rep.ExistingLinks++
			for _, id := range ids {
				linkedIDs[id] = true
				ambiguousIDs[id] = true
			}
			warnings = append(warnings, ambiguousLinkWarning(t, ids))
		case len(ids) == 1:
			rep.ExistingLinks++
			linkedIDs[ids[0]] = true
			linkedTasks[ids[0]] = t
		default:
			if !eligible(t) {
				continue
			}
			if node == nil || query.Eval(node, t, now) {
				toPush = append(toPush, t)
			}
		}
	}
	return linkedIDs, linkedTasks, ambiguousIDs, toPush, warnings
}

// ambiguousLinkWarning reports a Task line carrying more than one @hey tag. Such
// a line is skipped by every pass, so the Warning is the only trace a Sync
// leaves of it; it names the file, line, and every id so the user can fix it.
func ambiguousLinkWarning(t *model.Task, ids []string) model.Warning {
	return model.Warning{
		File: t.File, Line: t.Line,
		Message: fmt.Sprintf("task %s:%d has more than one @hey tag (%s); skipped", t.File, t.Line, strings.Join(ids, ", ")),
	}
}

// eligible reports whether a Task is an Eligible Task: unlinked, open, with a
// checkbox, and not hidden. Hidden-ness is checked here, independently of the
// Sync Query.
func eligible(t *model.Task) bool {
	if _, linked := linkID(t); linked {
		return false
	}
	return t.State == model.Open && t.HasCheckbox && !t.HasTag("hidden")
}

// linkID returns the first HEY id a Task is Linked to, if any. A Link is an
// @hey(id) tag on the Task line. A line may carry more than one — see linkIDs.
func linkID(t *model.Task) (string, bool) {
	for _, tag := range t.Tags {
		if tag.Name == "hey" && tag.Value != "" {
			return tag.Value, true
		}
	}
	return "", false
}

// linkIDs returns every HEY id a Task is Linked to. A well-formed Link has
// exactly one @hey(id) tag; a line carrying more than one is ambiguous and
// classifyTasks skips it.
func linkIDs(t *model.Task) []string {
	var ids []string
	for _, tag := range t.Tags {
		if tag.Name == "hey" && tag.Value != "" {
			ids = append(ids, tag.Value)
		}
	}
	return ids
}
