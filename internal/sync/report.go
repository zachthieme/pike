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
	// Pushed is populated by a real (non-dry-run) Sync: the number of Eligible
	// Tasks turned into Todos. Failed is the number of per-item write failures
	// across every reconciliation pass — a failed push, completion, reschedule,
	// retitle, re-create, or unlink — each counted once.
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
	// Unlinked is populated by a real (non-dry-run) Sync: the number of Linked
	// Tasks whose Todo has vanished from HEY and whose @hey tag was stripped.
	// Orphans is the number of Links whose Task line could not be found in the
	// notes; the surviving Todo is left alone and reported as a Warning.
	Unlinked int `json:"unlinked"`
	Orphans  int `json:"orphans"`
	// WouldUnlink and WouldOrphan are the dry-run equivalents.
	WouldUnlink int `json:"would_unlink"`
	WouldOrphan int `json:"would_orphan"`
	// Retitled and Recreated are populated by a real (non-dry-run) Sync: the
	// number of Links whose Task text was rewritten from HEY's changed Title, and
	// the number whose Todo was Re-created to carry the notes' changed Title.
	Retitled  int `json:"retitled"`
	Recreated int `json:"recreated"`
	// WouldRetitle and WouldRecreate are the dry-run equivalents.
	WouldRetitle  int `json:"would_retitle"`
	WouldRecreate int `json:"would_recreate"`
	// Rescheduled is populated by a real (non-dry-run) Sync: the number of Links
	// whose schedule was brought into agreement — either a Task's @due set to a
	// Todo's changed Week's Saturday, or a Todo Re-created for a @due moved to a
	// new Week in the notes.
	Rescheduled int `json:"rescheduled"`
	// WouldReschedule is the dry-run equivalent.
	WouldReschedule int `json:"would_reschedule"`
}

// WriteText renders the Report as human-readable lines. A dry run reports what
// a Sync would do; a real run reports what it did.
func (r *Report) WriteText(w io.Writer) error {
	if r.DryRun {
		_, err := fmt.Fprintf(w,
			"Sync (dry run — nothing written)\n  %d task(s) would push to HEY\n  %d todo(s) would import to the inbox\n  %d item(s) would complete\n  %d item(s) would uncomplete\n  %d task(s) would retitle\n  %d todo(s) would re-create\n  %d item(s) would reschedule\n  %d link(s) would unlink\n  %d orphan(s) would be reported\n  %d link(s) already exist\n",
			r.WouldPush, r.WouldImport, r.WouldComplete, r.WouldUncomplete, r.WouldRetitle, r.WouldRecreate, r.WouldReschedule, r.WouldUnlink, r.WouldOrphan, r.ExistingLinks)
		return err
	}
	_, err := fmt.Fprintf(w,
		"Sync\n  %d task(s) pushed to HEY\n  %d item(s) failed\n  %d todo(s) imported to the inbox\n  %d item(s) completed\n  %d item(s) uncompleted\n  %d task(s) retitled\n  %d todo(s) re-created\n  %d item(s) rescheduled\n  %d link(s) unlinked\n  %d orphan(s) reported\n  %d link(s) already exist\n",
		r.Pushed, r.Failed, r.Imported, r.Completed, r.Uncompleted, r.Retitled, r.Recreated, r.Rescheduled, r.Unlinked, r.Orphans, r.ExistingLinks)
	return err
}

// WriteJSON renders the Report as indented JSON.
func (r *Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
