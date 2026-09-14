// This file carries the push half of a real Sync: it creates a Todo in HEY for
// every Eligible Task matching the Sync Query, appends an @hey(id) Link tag to
// the Task's line, and records the Link in the state file.
package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/query"
	"github.com/zachthieme/pike/internal/toggle"
)

// tagRe matches an @name or @name(value) token anywhere in a line. It mirrors
// the parser's tag grammar so titles strip exactly the tokens pike recognises.
var tagRe = regexp.MustCompile(`@\w+(?:\([^)]*\))?`)

// Push performs a real Sync's push: it creates a Todo in HEY for every Eligible
// Task matching the Sync Query, appends an @hey(id) Link tag to the Task's line,
// and records the Link in the state file. Per-Task failures are collected as
// Warnings and do not stop the run. It returns an error only when HEY cannot be
// reached or the Sync Query is invalid. When opts.DryRun is true it writes
// nothing on either side.
func Push(ctx context.Context, opts Options) (*Report, []model.Warning, error) {
	var warnings []model.Warning

	state, err := LoadState(opts.StatePath)
	if err != nil {
		warnings = append(warnings, model.Warning{Message: fmt.Sprintf("reading hey state: %v", err)})
		state = &State{Links: make(map[string]Link)}
	}

	node, err := query.Parse(opts.Query)
	if err != nil {
		return nil, warnings, fmt.Errorf("sync query: %w", err)
	}

	// A failed HEY list is fatal, as it is for the dry-run plan. A re-run with
	// nothing to push makes no further HEY calls beyond this one.
	todos, err := opts.Client.List(ctx)
	if err != nil {
		return nil, warnings, fmt.Errorf("listing HEY todos: %w", err)
	}

	rep := &Report{DryRun: opts.DryRun}
	linkedIDs := make(map[string]bool)
	linkedTasks := make(map[string]*model.Task)
	var toPush []*model.Task
	for i := range opts.Tasks {
		t := &opts.Tasks[i]
		if id, ok := linkID(t); ok {
			rep.ExistingLinks++
			linkedIDs[id] = true
			linkedTasks[id] = t
			continue
		}
		if !eligible(t) {
			continue
		}
		if node != nil && !query.Eval(node, t, opts.Now) {
			continue
		}
		toPush = append(toPush, t)
	}

	// Orphan reconciliation runs against the state as loaded, before any push
	// mutates it, so a Task pushed this run is never mistaken for an Orphan.
	orphanDirty, orphanWarnings := reconcileOrphans(ctx, opts, linkedTasks, todos, state, rep)
	warnings = append(warnings, orphanWarnings...)

	for _, t := range toPush {
		if w := pushTask(ctx, opts, t, state, rep); w != nil {
			warnings = append(warnings, *w)
		}
	}

	warnings = append(warnings, importTodos(ctx, opts, todos, linkedIDs, state, rep)...)

	// Title reconciliation runs before completion so a Re-created Link (which
	// moves to a fresh Todo id and is dropped from linkedTasks) is not also
	// touched by the completion pass this Sync.
	// Snapshot the Links before any reconciliation mutates them, so the Week pass
	// can read each Link's recorded Title and Week as they stood at the last Sync.
	origLinks := snapshotLinks(state)

	titleDirty, titleWarnings := reconcileTitles(ctx, opts, linkedTasks, todos, state, rep)
	warnings = append(warnings, titleWarnings...)

	// Week reconciliation runs after titles (so a shared Re-create fires once) and
	// before completion (so a Re-created Link, dropped from linkedTasks, is not
	// also touched by the completion pass this Sync).
	weekDirty, weekWarnings := reconcileWeeks(ctx, opts, linkedTasks, todos, origLinks, state, rep)
	warnings = append(warnings, weekWarnings...)

	completionDirty, completionWarnings := reconcileCompletions(ctx, opts, linkedTasks, todos, state, rep)
	warnings = append(warnings, completionWarnings...)

	if !opts.DryRun && (rep.Pushed > 0 || rep.Imported > 0 || titleDirty || weekDirty || completionDirty || orphanDirty) {
		if err := SaveState(opts.StatePath, state); err != nil {
			warnings = append(warnings, model.Warning{Message: fmt.Sprintf("writing hey state: %v", err)})
		}
	}

	return rep, warnings, nil
}

// pushTask creates one Todo, appends its Link tag, and records the Link,
// counting the outcome on rep. It returns a non-nil Warning when the push fails;
// a failing push does not stop the run. On a dry run it counts the Task as a
// would-push and writes nothing.
func pushTask(ctx context.Context, opts Options, t *model.Task, state *State, rep *Report) *model.Warning {
	title := titleOf(t.Text)
	if opts.DryRun {
		rep.WouldPush++
		return nil
	}

	todo, err := opts.Client.Add(ctx, title, t.Due)
	if err != nil {
		rep.Failed++
		return &model.Warning{File: t.File, Line: t.Line, Message: fmt.Sprintf("pushing %q to HEY: %v", title, err)}
	}

	path := filepath.Join(opts.NotesDir, t.File)
	if err := toggle.AppendTag(ctx, path, t.Line, t.Raw, "@hey("+todo.ID+")"); err != nil {
		rep.Failed++
		return &model.Warning{File: t.File, Line: t.Line, Message: fmt.Sprintf("linking %q: %v", title, err)}
	}

	state.Links[todo.ID] = Link{
		Title:     title,
		WeekStart: todo.WeekStart,
		Completed: todo.Completed != nil,
		Updated:   todo.Updated,
		File:      t.File,
		Line:      t.Line,
	}
	rep.Pushed++
	return nil
}

// refreshScannedLine re-reads the Task's line from disk into t.Raw after a
// successful write, so a later mutation in the same Sync verifies against the
// line as this write left it rather than the line as first scanned. Two HEY-side
// changes to one line — a Retitle then a reschedule — both land this way. A read
// error or a line now out of range leaves t.Raw unchanged; the next mutation's
// stale guard then skips, which is the safe outcome.
func refreshScannedLine(t *model.Task, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if t.Line >= 1 && t.Line <= len(lines) {
		t.Raw = lines[t.Line-1]
	}
}

// titleOf derives a Todo Title from a Task's text: every @tag removed, any
// zero-width tag breaks an import inserted into the Title decoded away, and
// surrounding whitespace collapsed to single spaces. Decoding makes the Title
// pike computes from an imported line equal the HEY Title verbatim; a Task never
// touched by import carries no breaks, so decoding is the identity there.
func titleOf(text string) string {
	stripped := tagRe.ReplaceAllString(text, "")
	return strings.Join(strings.Fields(decodeTitle(stripped)), " ")
}
