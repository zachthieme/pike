package sync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zachthieme/pike/internal/hey"
	"github.com/zachthieme/pike/internal/model"
)

// These tests cover the shared guarantee of #13: every note write a Sync makes
// verifies that the line still holds exactly the Task that was scanned. When the
// line changed between the scan and the write, the mutation writes nothing, the
// Task's Link is left untouched, one Warning naming the HEY id is emitted, and
// one failure is counted in the Report.

// overwriteLine rewrites the single-line notes file to content the scan never
// saw, simulating an edit (or another Sync's write) landing between scan and
// write. It returns the new byte-for-byte file content for an unchanged-file
// assertion.
func overwriteLine(t *testing.T, notesDir, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return content
}

func TestSyncCompletion_LineChangedSinceScan_SkippedWarnedCounted(t *testing.T) {
	now := time.Now()
	completedAt := now
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [ ] Ship it @hey(h1)",
		model.Task{Text: "Ship it @hey(h1)", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", Completed: false, File: "notes.md", Line: 1}}},
	)
	// HEY completed the Todo; base says open, so the Task would be completed. But
	// the Task line was edited since the scan.
	after := overwriteLine(t, notesDir, "- [ ] Ship it, actually tomorrow @hey(h1)\n")
	client := &completionClient{todos: []hey.Todo{
		{ID: "h1", Title: "Ship it", Completed: &completedAt, WeekStart: now, WeekEnd: now},
	}}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	assertSkippedWarnedCounted(t, rep.Completed, rep.Failed, warnings, notesDir, after, "h1")
}

func TestSyncUnlink_LineChangedSinceScan_SkippedWarnedCounted(t *testing.T) {
	now := time.Now()
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [ ] Ship it @hey(h1)",
		model.Task{Text: "Ship it @hey(h1)", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", File: "notes.md", Line: 1}}},
	)
	// The Todo has vanished from HEY, so the @hey tag would be stripped. But the
	// Task line was edited since the scan.
	after := overwriteLine(t, notesDir, "- [ ] Ship it @hey(h1) @today\n")
	client := &completionClient{todos: nil} // h1 gone from HEY

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	assertSkippedWarnedCounted(t, rep.Unlinked, rep.Failed, warnings, notesDir, after, "h1")
	// The Link is left in place since nothing was written.
	if st, _ := LoadState(statePath); st.Links["h1"].File == "" {
		t.Error("Link should survive a skipped unlink")
	}
}

func TestSyncRetitle_LineChangedSinceScan_SkippedWarnedCounted(t *testing.T) {
	now := time.Now()
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [ ] Ship it @hey(h1)",
		model.Task{Text: "Ship it @hey(h1)", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", File: "notes.md", Line: 1}}},
	)
	// HEY renamed the Todo, so the Task text would be rewritten. But the Task line
	// was edited since the scan.
	after := overwriteLine(t, notesDir, "- [ ] Ship it now @hey(h1)\n")
	client := &titleClient{todos: []hey.Todo{{ID: "h1", Title: "Deploy it now", WeekStart: now}}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	assertSkippedWarnedCounted(t, rep.Retitled, rep.Failed, warnings, notesDir, after, "h1")
	if len(client.calls) != 0 {
		t.Errorf("no HEY mutation expected for a skipped retitle; calls=%v", client.calls)
	}
}

func TestSyncReschedule_LineChangedSinceScan_SkippedWarnedCounted(t *testing.T) {
	now := time.Now()
	oldDue := day(2026, 9, 16) // Wed in W0
	task, notesDir, statePath := linkFixture(t,
		"h1", weekLine("h1", oldDue), weekLinkTask("h1", oldDue),
		&State{Links: map[string]Link{"h1": {Title: "Ship it", WeekStart: day(2026, 9, 13), File: "notes.md", Line: 1}}},
	)
	// HEY moved the Todo to a new Week, so @due would be set to the new Saturday.
	// But the Task line was edited since the scan.
	after := overwriteLine(t, notesDir, "- [ ] Ship it @hey(h1) @due(2026-09-16) @today\n")
	client := &titleClient{todos: []hey.Todo{
		{ID: "h1", Title: "Ship it", WeekStart: day(2026, 9, 20), WeekEnd: day(2026, 9, 26)},
	}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	assertSkippedWarnedCounted(t, rep.Rescheduled, rep.Failed, warnings, notesDir, after, "h1")
}

func TestSyncRecreateTagRewrite_LineChangedSinceScan_SkippedWarnedCounted(t *testing.T) {
	now := time.Now()
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [ ] Renamed here @hey(h1)",
		model.Task{Text: "Renamed here @hey(h1)", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", File: "notes.md", Line: 1}}},
	)
	// The notes renamed the Task, so its Todo would be Re-created and the @hey tag
	// rewritten to the new id. But the Task line was edited since the scan, so the
	// tag rewrite finds a line it never scanned.
	after := overwriteLine(t, notesDir, "- [ ] Renamed elsewhere @hey(h1)\n")
	client := &titleClient{todos: []hey.Todo{{ID: "h1", Title: "Ship it", WeekStart: now}}, week: now}

	opts := Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	}
	rep, warnings, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	assertSkippedWarnedCounted(t, rep.Recreated, rep.Failed, warnings, notesDir, after, "h1")
	// The freshly-added Todo is rolled back: pike added it, then deleted the same
	// new id — never the old id, whose Link the refused rewrite left in place.
	if want := []string{"add:Renamed here", "delete:h_new1"}; !equalStrings(client.calls, want) {
		t.Errorf("client calls = %v, want %v (add then delete of the new id, no delete of the old)", client.calls, want)
	}
	// The Link still points at the original Todo: the rewrite never landed, and no
	// pending-delete entry was left behind since the rollback delete succeeded.
	st, _ := LoadState(statePath)
	if st.Links["h1"].File == "" {
		t.Error("original Link should survive a skipped tag rewrite")
	}
	if len(st.Links) != 1 {
		t.Errorf("only the original Link should remain, got %+v", st.Links)
	}

	// A second Sync leaves no duplicate behind: the rolled-back Todo is gone from
	// HEY, so nothing is imported and the original Link is untouched.
	client.calls = nil
	rep2, _, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("second Push: %v", err)
	}
	if rep2.Imported != 0 {
		t.Errorf("second Sync Imported=%d, want 0 — a rolled-back Todo must never be imported", rep2.Imported)
	}
	if st2, _ := LoadState(statePath); st2.Links["h1"].File == "" {
		t.Errorf("original Link should still be present after a second Sync, state=%+v", st2.Links)
	}
}

// TestSyncRecreateTagRewrite_RollbackDeleteFails_KeptPendingRetriedNextSync
// covers a Re-create whose @hey rewrite is refused (a stale line) and whose
// rollback delete then also fails: the freshly-added Todo cannot be removed, so
// its id is kept as a pending delete — never imported — and the next Sync
// retries the delete without adding anything or importing the leaked Todo.
func TestSyncRecreateTagRewrite_RollbackDeleteFails_KeptPendingRetriedNextSync(t *testing.T) {
	now := time.Now()
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [ ] Renamed here @hey(h1)",
		model.Task{Text: "Renamed here @hey(h1)", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", File: "notes.md", Line: 1}}},
	)
	// The line changed since the scan, so the tag rewrite is refused; the rollback
	// delete of the just-added Todo then fails too.
	overwriteLine(t, notesDir, "- [ ] Renamed elsewhere @hey(h1)\n")
	client := &titleClient{
		todos:     []hey.Todo{{ID: "h1", Title: "Ship it", WeekStart: now}},
		week:      now,
		deleteErr: errDeleteFailed,
	}
	opts := Options{Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now}

	rep, warnings, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if rep.Recreated != 0 || rep.Failed != 1 {
		t.Errorf("Recreated=%d Failed=%d, want 0/1", rep.Recreated, rep.Failed)
	}
	if len(warnings) != 1 {
		t.Fatalf("want exactly 1 Warning, got %d: %v", len(warnings), warnings)
	}
	if want := []string{"add:Renamed here", "delete:h_new1"}; !equalStrings(client.calls, want) {
		t.Errorf("client calls = %v, want %v", client.calls, want)
	}
	st, _ := LoadState(statePath)
	// The old Link is untouched; the leaked new id is kept as a pending delete.
	if old := st.Links["h1"]; old.Title != "Ship it" || old.PendingDelete {
		t.Errorf("old Link h1 should be unchanged, got %+v", old)
	}
	if nw, ok := st.Links["h_new1"]; !ok || !nw.PendingDelete {
		t.Errorf("new id h_new1 should be kept as a pending delete; state=%+v", st.Links)
	}

	// The next Sync retries the delete, imports nothing, and makes no further add.
	// The line now agrees with HEY (the user reverted the stale edit), so no
	// Re-create fires this time.
	overwriteLine(t, notesDir, "- [ ] Ship it @hey(h1)\n")
	relinked := model.TaskWith(model.Task{
		Text: "Ship it @hey(h1)", Raw: "- [ ] Ship it @hey(h1)",
		State: model.Open, HasCheckbox: true,
		Tags: []model.Tag{{Name: "hey", Value: "h1"}}, File: "notes.md", Line: 1,
	})
	callsBefore := len(client.calls)
	opts.Tasks = []model.Task{relinked}
	rep2, _, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("second Push: %v", err)
	}
	if rep2.Imported != 0 {
		t.Errorf("second Sync Imported=%d, want 0 — a leaked Todo is never imported", rep2.Imported)
	}
	second := client.calls[callsBefore:]
	retried := false
	for _, c := range second {
		if strings.HasPrefix(c, "add:") {
			t.Errorf("second Sync made a further add: %v", second)
		}
		if c == "delete:h_new1" {
			retried = true
		}
	}
	if !retried {
		t.Errorf("second Sync should retry delete:h_new1; calls=%v", second)
	}
}

// equalStrings reports whether two string slices are element-wise equal.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// assertSkippedWarnedCounted checks the shared stale-line outcome: nothing was
// applied (successCount == 0), exactly one failure was counted, exactly one
// Warning naming the HEY id was emitted, and the file is byte-for-byte the
// content the write found rather than what the scan saw.
func assertSkippedWarnedCounted(t *testing.T, successCount, failed int, warnings []model.Warning, notesDir, wantFile, id string) {
	t.Helper()
	if successCount != 0 {
		t.Errorf("nothing should have been applied, got count %d", successCount)
	}
	if failed != 1 {
		t.Errorf("Failed=%d, want 1", failed)
	}
	if len(warnings) != 1 {
		t.Fatalf("want exactly 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0].Message, id) {
		t.Errorf("warning should name HEY id %q; got %q", id, warnings[0].Message)
	}
	if got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md")); string(got) != wantFile {
		t.Errorf("stale line must be left byte-for-byte unchanged\n got: %q\nwant: %q", string(got), wantFile)
	}
}
