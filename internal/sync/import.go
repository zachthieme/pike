// This file carries the import half of a real Sync: it appends an Inbox line
// for every open Todo in HEY that no Task is Linked to, turning a to-do added on
// the phone into a Linked checkbox Task the next Sync leaves alone.
package sync

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/zachthieme/pike/internal/hey"
	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/toggle"
)

// defaultInboxFile is the Inbox file used when Options.InboxFile is unset. It
// matches the TUI's capture default.
const defaultInboxFile = "inbox.md"

// importTodos appends an Inbox line for every unlinked open Todo, in HEY's list
// order, and records each new Link in state. Completed Todos and Todos already
// Linked from the notes (their id in linkedIDs) are skipped. On a dry run it
// counts would-imports and writes nothing. Per-Todo failures are collected as
// Warnings and do not stop the run.
func importTodos(ctx context.Context, opts Options, todos []hey.Todo, linkedIDs map[string]bool, state *State, rep *Report) []model.Warning {
	var warnings []model.Warning
	for _, td := range todos {
		// A Todo pike already holds a Link for is not an unlinked Todo: skip it
		// even when its Task line is gone from the notes (an Orphan), so a Sync
		// leaves the orphaned Todo alone rather than re-importing it.
		if _, known := state.Links[td.ID]; td.Completed != nil || linkedIDs[td.ID] || known {
			continue
		}
		if opts.DryRun {
			rep.WouldImport++
			continue
		}
		if w := importTodo(ctx, opts, td, state, rep); w != nil {
			warnings = append(warnings, *w)
		}
	}
	return warnings
}

// importTodo appends one Todo's Inbox line through the shared append path and
// records its Link, counting the outcome on rep. It returns a non-nil Warning
// when the append fails; a failing import does not stop the run.
func importTodo(ctx context.Context, opts Options, td hey.Todo, state *State, rep *Report) *model.Warning {
	inbox := inboxFile(opts)
	path := filepath.Join(opts.NotesDir, inbox)
	if err := toggle.AppendTask(ctx, path, importText(td)); err != nil {
		rep.Failed++
		return &model.Warning{File: inbox, Message: fmt.Sprintf("importing %q: %v", td.Title, err)}
	}
	state.Links[td.ID] = Link{
		Title:     td.Title,
		WeekStart: td.WeekStart,
		Completed: false,
		Updated:   td.Updated,
		File:      inbox,
	}
	rep.Imported++
	return nil
}

// importText is the Inbox line body for a Todo, without the "- [ ] " checkbox
// prefix the append path adds: the Title, an @due tag on the Week's Saturday,
// and an @hey Link tag. The date is HEY's WeekEnd taken verbatim, no zone
// conversion.
func importText(td hey.Todo) string {
	return fmt.Sprintf("%s @due(%s) @hey(%s)", td.Title, td.WeekEnd.Format("2006-01-02"), td.ID)
}

// inboxFile resolves the Inbox file, defaulting to inbox.md when unconfigured.
func inboxFile(opts Options) string {
	if opts.InboxFile == "" {
		return defaultInboxFile
	}
	return opts.InboxFile
}
