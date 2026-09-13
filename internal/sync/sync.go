// Package sync reconciles Tasks in the notes with Todos in HEY. This file
// carries the planning pass behind `pike --sync --dry-run`: it reports what a
// Sync would do without changing anything.
package sync

import (
	"context"
	"fmt"
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
	if _, err := LoadState(opts.StatePath); err != nil {
		warnings = append(warnings, model.Warning{Message: fmt.Sprintf("reading hey state: %v", err)})
	}

	node, err := query.Parse(opts.Query)
	if err != nil {
		return nil, warnings, fmt.Errorf("sync query: %w", err)
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
		if node == nil || query.Eval(node, t, opts.Now) {
			rep.WouldPush++
		}
	}

	// A failed first HEY call is the one fatal case for --sync.
	todos, err := opts.Client.List(ctx)
	if err != nil {
		return nil, warnings, fmt.Errorf("listing HEY todos: %w", err)
	}
	for _, td := range todos {
		if td.Completed == nil && !linkedIDs[td.ID] {
			rep.WouldImport++
		}
	}

	return rep, warnings, nil
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

// linkID returns the HEY id a Task is Linked to, if any. A Link is an @hey(id)
// tag on the Task line.
func linkID(t *model.Task) (string, bool) {
	for _, tag := range t.Tags {
		if tag.Name == "hey" && tag.Value != "" {
			return tag.Value, true
		}
	}
	return "", false
}
