package sync

import (
	"encoding/json"
	"fmt"
	"io"
)

// Report is the outcome of a planning pass: how many Eligible Tasks would be
// pushed to HEY, how many unlinked open Todos would be imported, and how many
// Links already exist.
type Report struct {
	DryRun        bool `json:"dry_run"`
	WouldPush     int  `json:"would_push"`
	WouldImport   int  `json:"would_import"`
	ExistingLinks int  `json:"existing_links"`
	// Pushed and Failed are populated by a real (non-dry-run) Sync: the number
	// of Eligible Tasks turned into Todos, and the number whose push failed.
	Pushed int `json:"pushed"`
	Failed int `json:"failed"`
	// Imported is populated by a real (non-dry-run) Sync: the number of unlinked
	// open Todos appended to the Inbox as new Tasks.
	Imported int `json:"imported"`
	// Completed and Uncompleted are populated by a real (non-dry-run) Sync: the
	// number of Links whose completion was brought into agreement by marking the
	// lagging side done, or by reopening it. Each counts one side of one Link.
	Completed   int `json:"completed"`
	Uncompleted int `json:"uncompleted"`
	// WouldComplete and WouldUncomplete are the dry-run equivalents.
	WouldComplete   int `json:"would_complete"`
	WouldUncomplete int `json:"would_uncomplete"`
}

// WriteText renders the Report as human-readable lines. A dry run reports what
// a Sync would do; a real run reports what it did.
func (r *Report) WriteText(w io.Writer) error {
	if r.DryRun {
		_, err := fmt.Fprintf(w,
			"Sync (dry run — nothing written)\n  %d task(s) would push to HEY\n  %d todo(s) would import to the inbox\n  %d item(s) would complete\n  %d item(s) would uncomplete\n  %d link(s) already exist\n",
			r.WouldPush, r.WouldImport, r.WouldComplete, r.WouldUncomplete, r.ExistingLinks)
		return err
	}
	_, err := fmt.Fprintf(w,
		"Sync\n  %d task(s) pushed to HEY\n  %d task(s) failed to push\n  %d todo(s) imported to the inbox\n  %d item(s) completed\n  %d item(s) uncompleted\n  %d link(s) already exist\n",
		r.Pushed, r.Failed, r.Imported, r.Completed, r.Uncompleted, r.ExistingLinks)
	return err
}

// WriteJSON renders the Report as indented JSON.
func (r *Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
