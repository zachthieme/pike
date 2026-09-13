// This file carries the push half of a real Sync: it creates a Todo in HEY for
// every Eligible Task matching the Sync Query, appends an @hey(id) Link tag to
// the Task's line, and records the Link in the state file.
package sync

import (
	"context"
	"fmt"
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
	for i := range opts.Tasks {
		t := &opts.Tasks[i]
		if id, ok := linkID(t); ok {
			rep.ExistingLinks++
			linkedIDs[id] = true
			continue
		}
		if !eligible(t) {
			continue
		}
		if node != nil && !query.Eval(node, t, opts.Now) {
			continue
		}
		if w := pushTask(ctx, opts, t, state, rep); w != nil {
			warnings = append(warnings, *w)
		}
	}

	for _, td := range todos {
		if td.Completed == nil && !linkedIDs[td.ID] {
			rep.WouldImport++
		}
	}

	if !opts.DryRun && rep.Pushed > 0 {
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
	if err := toggle.AppendTag(ctx, path, t.Line, t.Text, "@hey("+todo.ID+")"); err != nil {
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

// titleOf derives a Todo Title from a Task's text: every @tag removed and
// surrounding whitespace collapsed to single spaces.
func titleOf(text string) string {
	stripped := tagRe.ReplaceAllString(text, "")
	return strings.Join(strings.Fields(stripped), " ")
}
