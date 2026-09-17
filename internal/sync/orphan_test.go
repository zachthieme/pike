package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/zachthieme/pike/internal/hey"
	"github.com/zachthieme/pike/internal/model"
)

// noDeleteClient hands back a fixed todo list and fails the test if the Delete
// verb is ever called — Sync must never delete in an Orphan case.
type noDeleteClient struct {
	t     *testing.T
	todos []hey.Todo
}

func (c *noDeleteClient) List(context.Context) ([]hey.Todo, error) { return c.todos, nil }
func (c *noDeleteClient) Add(_ context.Context, title string, _ *time.Time) (hey.Todo, error) {
	return hey.Todo{ID: "h_add", Title: title}, nil
}
func (c *noDeleteClient) Complete(context.Context, string) error   { return nil }
func (c *noDeleteClient) Uncomplete(context.Context, string) error { return nil }
func (c *noDeleteClient) Delete(_ context.Context, id string) error {
	c.t.Fatalf("Delete called on %s: Sync must never delete in an Orphan case", id)
	return nil
}

// TestOrphan_UnlinksTaskWhoseTodoIsGoneFromHey covers the first Orphan case: a
// Linked Task whose id no longer appears in HEY's list has its @hey tag
// stripped, its state entry dropped, and is counted as an unlink. The Todo side
// is already gone, so nothing is deleted.
func TestOrphan_UnlinksTaskWhoseTodoIsGoneFromHey(t *testing.T) {
	now := time.Now()
	task, notesDir, statePath := linkFixture(t,
		"h_gone", "- [ ] Buy milk @today @hey(h_gone)",
		model.Task{Text: "Buy milk @today @hey(h_gone)", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "today"}, {Name: "hey", Value: "h_gone"}}},
		&State{Links: map[string]Link{"h_gone": {Title: "Buy milk", File: "notes.md", Line: 1}}},
	)
	// HEY's list no longer contains h_gone.
	client := &noDeleteClient{t: t, todos: []hey.Todo{}}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if rep.Unlinked != 1 {
		t.Errorf("Unlinked = %d, want 1", rep.Unlinked)
	}

	// The @hey tag is gone; the rest of the line is unchanged.
	got, err := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "- [ ] Buy milk @today\n"; string(got) != want {
		t.Errorf("notes after unlink:\n got: %q\nwant: %q", string(got), want)
	}

	// The state entry is dropped.
	state, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if _, ok := state.Links["h_gone"]; ok {
		t.Errorf("state entry for h_gone should be dropped, state: %+v", state.Links)
	}
}

// TestOrphan_ReportsLinkWhoseTaskLineIsGone covers the second Orphan case: a
// state entry whose Task line can no longer be found in the notes. The surviving
// Todo is left in HEY (never deleted, never re-imported), a Warning naming the
// id and Title is returned, and the run counts one Orphan.
func TestOrphan_ReportsLinkWhoseTaskLineIsGone(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	// The notes no longer carry a Task Linked to h_lost.
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte("- [ ] Something else\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(notesDir, "hey-state.json")
	if err := SaveState(statePath, &State{Links: map[string]Link{
		"h_lost": {Title: "Gone task", File: "notes.md", Line: 9},
	}}); err != nil {
		t.Fatal(err)
	}
	// The Todo still exists in HEY.
	client := &noDeleteClient{t: t, todos: []hey.Todo{{ID: "h_lost", Title: "Gone task", WeekStart: now, WeekEnd: now}}}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: nil, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if rep.Orphans != 1 {
		t.Errorf("Orphans = %d, want 1", rep.Orphans)
	}
	// The orphaned Todo is not re-imported.
	if rep.Imported != 0 {
		t.Errorf("Imported = %d, want 0 — an orphaned Todo must be left alone", rep.Imported)
	}
	// A Warning names both the id and the Title.
	if len(warnings) != 1 {
		t.Fatalf("want exactly 1 Warning, got %d: %v", len(warnings), warnings)
	}
	if msg := warnings[0].Message; !strings.Contains(msg, "h_lost") || !strings.Contains(msg, "Gone task") {
		t.Errorf("Warning should name the id and Title, got: %q", msg)
	}
	// The Todo side survives: nothing was deleted, and the state entry is kept.
	state, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if _, ok := state.Links["h_lost"]; !ok {
		t.Errorf("orphan state entry must be kept so the Warning persists, state: %+v", state.Links)
	}
}

// TestOrphan_TwoHeyTagsAreSkippedWithWarning covers a Task line carrying two
// @hey tags whose Todos have both vanished from HEY: pike cannot tell which id
// the line means, so the line is skipped by every pass with a single Warning and
// nothing is written — no unlink is attempted, so no failure is counted.
func TestOrphan_TwoHeyTagsAreSkippedWithWarning(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	line := "- [ ] Buy milk @today @hey(h1) @hey(h2)\n"
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(notesDir, "hey-state.json")
	task := model.TaskWith(model.Task{
		Text: "Buy milk @today @hey(h1) @hey(h2)", State: model.Open, HasCheckbox: true,
		Tags: []model.Tag{{Name: "today"}, {Name: "hey", Value: "h1"}, {Name: "hey", Value: "h2"}},
		File: "notes.md", Line: 1,
	})
	// Neither id is in HEY's list; the ambiguous line must still be skipped, not
	// unlinked, and never touch HEY (noDeleteClient fails the test on Delete).
	client := &noDeleteClient{t: t, todos: []hey.Todo{}}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if rep.Unlinked != 0 {
		t.Errorf("Unlinked = %d, want 0 — an ambiguous line must be skipped", rep.Unlinked)
	}
	// The line is skipped, not written to: no unlink is attempted, so no failure.
	if rep.Failed != 0 {
		t.Errorf("Failed = %d, want 0 — an ambiguous line is skipped, not failed", rep.Failed)
	}
	if len(warnings) != 1 {
		t.Fatalf("want exactly 1 Warning for the ambiguous line, got %d: %v", len(warnings), warnings)
	}
	if got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md")); string(got) != line {
		t.Errorf("ambiguous line must not be written:\n got: %q\nwant: %q", string(got), line)
	}
	// Nothing was written, so no state file was created.
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Errorf("state should not be written when the only change is skipped (stat err: %v)", err)
	}
}

// TestOrphan_ManualUnlink_WarnsOnceThenDropsWhenTodoGone covers a manual
// un-link: after the user deletes the @hey tag by hand, the state entry lingers
// while the Todo stays in HEY. The Orphan Warning must appear on the first Sync
// that finds it and not repeat on later Syncs, the Orphan is still counted every
// Sync, and the entry is dropped once the Todo is gone from HEY.
func TestOrphan_ManualUnlink_WarnsOnceThenDropsWhenTodoGone(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	// The user deleted the @hey tag; the task line stays but is no longer Linked,
	// and does not match the Sync Query, so it is not re-pushed here.
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte("- [ ] Buy milk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(notesDir, "hey-state.json")
	if err := SaveState(statePath, &State{Links: map[string]Link{
		"h1": {Title: "Buy milk", File: "notes.md", Line: 1},
	}}); err != nil {
		t.Fatal(err)
	}
	task := model.TaskWith(model.Task{Text: "Buy milk", Raw: "- [ ] Buy milk",
		State: model.Open, HasCheckbox: true, File: "notes.md", Line: 1})
	// The Todo survives in HEY; nothing should ever delete it (noDeleteClient).
	client := &noDeleteClient{t: t, todos: []hey.Todo{{ID: "h1", Title: "Buy milk", WeekStart: now, WeekEnd: now}}}
	opts := Options{Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now}

	// First Sync: the Orphan is counted and warned.
	rep1, warn1, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("first Push: %v", err)
	}
	if rep1.Orphans != 1 {
		t.Errorf("first Sync Orphans=%d, want 1", rep1.Orphans)
	}
	if len(warn1) != 1 || !strings.Contains(warn1[0].Message, "h1") {
		t.Fatalf("first Sync should warn once naming h1, got: %v", warn1)
	}

	// Second Sync: still counted, but the Warning does not repeat.
	rep2, warn2, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("second Push: %v", err)
	}
	if rep2.Orphans != 1 {
		t.Errorf("second Sync Orphans=%d, want 1 (still an orphan)", rep2.Orphans)
	}
	if len(warn2) != 0 {
		t.Errorf("second Sync should not repeat the orphan Warning, got: %v", warn2)
	}

	// The Todo is removed from HEY; the next Sync drops the entry.
	client.todos = nil
	rep3, _, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("third Push: %v", err)
	}
	if rep3.Orphans != 0 {
		t.Errorf("third Sync Orphans=%d, want 0 (entry dropped)", rep3.Orphans)
	}
	st, _ := LoadState(statePath)
	if _, ok := st.Links["h1"]; ok {
		t.Errorf("orphan entry should be dropped once the Todo is gone; state=%+v", st.Links)
	}
}

// TestOrphan_LinkRestoredThenReorphaned_WarnsAgain checks that a Link which was
// orphaned (and warned), then restored, and then orphaned again warns afresh the
// second time — the Orphaned mark is cleared while the Link is live, so a later
// disappearance is a new orphan episode rather than a silenced repeat.
func TestOrphan_LinkRestoredThenReorphaned_WarnsAgain(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	statePath := filepath.Join(notesDir, "hey-state.json")
	// The Link was orphaned and warned in a prior episode, but its Task line is
	// back in the notes now.
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte("- [ ] Buy milk @hey(h1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(statePath, &State{Links: map[string]Link{
		"h1": {Title: "Buy milk", File: "notes.md", Line: 1, Orphaned: true},
	}}); err != nil {
		t.Fatal(err)
	}
	live := model.TaskWith(model.Task{Text: "Buy milk @hey(h1)", Raw: "- [ ] Buy milk @hey(h1)",
		State: model.Open, HasCheckbox: true, Tags: []model.Tag{{Name: "hey", Value: "h1"}}, File: "notes.md", Line: 1})
	client := &noDeleteClient{t: t, todos: []hey.Todo{{ID: "h1", Title: "Buy milk", WeekStart: now, WeekEnd: now}}}
	opts := Options{Tasks: []model.Task{live}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now}

	// First Sync: the Link is live, so no orphan Warning and the stale mark clears.
	if _, warn, err := Push(context.Background(), opts); err != nil || len(warn) != 0 {
		t.Fatalf("first Push: err=%v warnings=%v", err, warn)
	}
	if st, _ := LoadState(statePath); st.Links["h1"].Orphaned {
		t.Errorf("Orphaned mark should clear while the Link is live; state=%+v", st.Links["h1"])
	}

	// The Task line disappears again: the orphan warns afresh.
	opts.Tasks = nil
	rep, warn, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("second Push: %v", err)
	}
	if rep.Orphans != 1 || len(warn) != 1 {
		t.Errorf("re-orphan should warn afresh: Orphans=%d warnings=%d, want 1/1", rep.Orphans, len(warn))
	}
}

// TestSync_TwoHeyTags_OneWarningSkippedNoImportSameInDryRun covers a line
// carrying more than one @hey tag: it is a Warning and is skipped by every pass,
// every id on it counts as Linked (so neither Todo is imported), and a dry run
// reports the same as a real Sync.
func TestSync_TwoHeyTags_OneWarningSkippedNoImportSameInDryRun(t *testing.T) {
	now := time.Now()
	line := "- [ ] Buy milk @today @hey(h1) @hey(h2)\n"
	task := model.TaskWith(model.Task{
		Text: "Buy milk @today @hey(h1) @hey(h2)", State: model.Open, HasCheckbox: true,
		Tags: []model.Tag{{Name: "today"}, {Name: "hey", Value: "h1"}, {Name: "hey", Value: "h2"}},
		File: "notes.md", Line: 1,
	})
	// Both ids are open in HEY; a stray unlinked id would otherwise import.
	todos := []hey.Todo{openTodo("h1", "one"), openTodo("h2", "two")}

	run := func(t *testing.T, dry bool) (*Report, []model.Warning) {
		t.Helper()
		notesDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		statePath := filepath.Join(notesDir, "hey-state.json")
		client := &recordingClient{week: now, todos: append([]hey.Todo(nil), todos...)}
		opts := Options{Tasks: []model.Task{task}, Client: client, Query: "@today",
			StatePath: statePath, NotesDir: notesDir, Now: now, DryRun: dry}

		var rep *Report
		var warnings []model.Warning
		var err error
		if dry {
			rep, warnings, err = Plan(context.Background(), opts)
		} else {
			rep, warnings, err = Push(context.Background(), opts)
		}
		if err != nil {
			t.Fatalf("run (dry=%v): %v", dry, err)
		}
		// Exactly one Warning about the ambiguous line.
		if len(warnings) != 1 {
			t.Fatalf("want exactly 1 Warning (dry=%v), got %d: %v", dry, len(warnings), warnings)
		}
		if msg := warnings[0].Message; !strings.Contains(msg, "hey") {
			t.Errorf("Warning should describe the @hey ambiguity, got: %q", msg)
		}
		// Neither Todo is imported.
		if rep.Imported != 0 || rep.WouldImport != 0 {
			t.Errorf("Imported=%d WouldImport=%d (dry=%v), want 0/0", rep.Imported, rep.WouldImport, dry)
		}
		// No HEY mutations and no add.
		if len(client.added) != 0 || len(client.mutations) != 0 {
			t.Errorf("HEY was mutated (dry=%v): added=%v mutations=%v", dry, client.added, client.mutations)
		}
		// The notes line is untouched, and nothing wrote a state file.
		if got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md")); string(got) != line {
			t.Errorf("notes line changed (dry=%v):\n got: %q\nwant: %q", dry, string(got), line)
		}
		if _, err := os.Stat(statePath); !os.IsNotExist(err) {
			t.Errorf("state file written (dry=%v) though nothing changed (stat err: %v)", dry, err)
		}
		return rep, warnings
	}

	real, realWarn := run(t, false)
	dry, dryWarn := run(t, true)
	if len(realWarn) != len(dryWarn) || realWarn[0].Message != dryWarn[0].Message {
		t.Errorf("dry-run Warning differs from real:\n real: %q\n dry:  %q", realWarn[0].Message, dryWarn[0].Message)
	}
	if real.ExistingLinks != dry.ExistingLinks {
		t.Errorf("ExistingLinks differs real=%d dry=%d", real.ExistingLinks, dry.ExistingLinks)
	}
}

// TestOrphan_TwoHeyTags_IdInState_NotOrphaned covers an ambiguous line whose
// first id already has a state entry with both Todos open in HEY: the line is
// still only its single ambiguous-line Warning, never an Orphan, and the state
// file is left byte-for-byte unchanged — the same in a real Sync and a dry run.
// Without the fix the ambiguous-line skip leaves the Task out of linkedTasks, so
// Orphan detection treats h1's entry as having no Task line and falsely reports
// it as an Orphan, marking the entry orphaned in state.
func TestOrphan_TwoHeyTags_IdInState_NotOrphaned(t *testing.T) {
	now := time.Now()
	line := "- [ ] Buy milk @today @hey(h1) @hey(h2)\n"
	task := model.TaskWith(model.Task{
		Text: "Buy milk @today @hey(h1) @hey(h2)", State: model.Open, HasCheckbox: true,
		Tags: []model.Tag{{Name: "today"}, {Name: "hey", Value: "h1"}, {Name: "hey", Value: "h2"}},
		File: "notes.md", Line: 1,
	})

	run := func(t *testing.T, dry bool, todos []hey.Todo) {
		t.Helper()
		notesDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		statePath := filepath.Join(notesDir, "hey-state.json")
		// h1 already has a state entry from a Sync before the second @hey tag was
		// added by hand.
		if err := SaveState(statePath, &State{Links: map[string]Link{
			"h1": {Title: "Buy milk", File: "notes.md", Line: 1},
		}}); err != nil {
			t.Fatal(err)
		}
		stateBefore, _ := os.ReadFile(statePath)
		client := &noDeleteClient{t: t, todos: todos}
		opts := Options{Tasks: []model.Task{task}, Client: client, Query: "@today",
			StatePath: statePath, NotesDir: notesDir, Now: now, DryRun: dry}

		var rep *Report
		var warnings []model.Warning
		var err error
		if dry {
			rep, warnings, err = Plan(context.Background(), opts)
		} else {
			rep, warnings, err = Push(context.Background(), opts)
		}
		if err != nil {
			t.Fatalf("run (dry=%v): %v", dry, err)
		}
		if len(warnings) != 1 {
			t.Fatalf("want exactly 1 Warning (the ambiguous line, dry=%v), got %d: %v", dry, len(warnings), warnings)
		}
		if !strings.Contains(warnings[0].Message, "@hey") {
			t.Errorf("Warning should describe the @hey ambiguity, got: %q", warnings[0].Message)
		}
		if rep.Orphans != 0 || rep.WouldOrphan != 0 {
			t.Errorf("Orphans=%d WouldOrphan=%d (dry=%v), want 0/0 — an ambiguous id is never an Orphan", rep.Orphans, rep.WouldOrphan, dry)
		}
		// h1's entry survives untouched: never marked, rewritten, or dropped.
		if got, _ := os.ReadFile(statePath); string(got) != string(stateBefore) {
			t.Errorf("state changed (dry=%v):\n got: %s\nwant: %s", dry, string(got), string(stateBefore))
		}
	}

	bothOpen := []hey.Todo{openTodo("h1", "Buy milk"), openTodo("h2", "two")}
	t.Run("both todos open", func(t *testing.T) {
		run(t, false, append([]hey.Todo(nil), bothOpen...))
		run(t, true, append([]hey.Todo(nil), bothOpen...))
	})

	// Even after h1 is completed in HEY (which for a live Link would drop the
	// entry), an ambiguous line's entry is left in place.
	completedAt := now
	h1done := []hey.Todo{
		{ID: "h1", Title: "Buy milk", WeekStart: now, WeekEnd: now, Completed: &completedAt},
		openTodo("h2", "two"),
	}
	t.Run("h1 completed in hey", func(t *testing.T) {
		run(t, false, append([]hey.Todo(nil), h1done...))
		run(t, true, append([]hey.Todo(nil), h1done...))
	})
}

// TestOrphan_PendingDeleteOnAmbiguousLine_RetriedAndDropped covers a pending
// delete whose id also sits on a multi-@hey (ambiguous) line alongside a normal
// Link. The ambiguous-line skip must not swallow the pending delete: its retry
// runs on its own terms — delete is called for the pending id and the entry is
// dropped once the Todo leaves HEY — while the ambiguous line still gets exactly
// one Warning and nothing else on either side is touched. Without the fix the
// ambiguous-line skip runs first, so the delete is never retried and the entry
// can never be dropped.
func TestOrphan_PendingDeleteOnAmbiguousLine_RetriedAndDropped(t *testing.T) {
	now := time.Now()
	line := "- [ ] Buy milk @today @hey(h1) @hey(h_new1)\n"
	notesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(notesDir, "hey-state.json")
	// h1 is a normal Link; h_new1 is a leaked Todo pike could not delete. Both ids
	// happen to sit on the same ambiguous line (the user pasted the pending id in).
	if err := SaveState(statePath, &State{Links: map[string]Link{
		"h1":     {Title: "Buy milk", File: "notes.md", Line: 1},
		"h_new1": {PendingDelete: true, Title: "Buy milk", File: "notes.md", Line: 1},
	}}); err != nil {
		t.Fatal(err)
	}
	task := model.TaskWith(model.Task{
		Text: "Buy milk @today @hey(h1) @hey(h_new1)", State: model.Open, HasCheckbox: true,
		Tags: []model.Tag{{Name: "today"}, {Name: "hey", Value: "h1"}, {Name: "hey", Value: "h_new1"}},
		File: "notes.md", Line: 1,
	})
	// Both Todos are still listed in HEY: h1 open, and the leaked h_new1 pike wants gone.
	client := &recordingClient{week: now, todos: []hey.Todo{openTodo("h1", "Buy milk"), openTodo("h_new1", "Buy milk")}}
	opts := Options{Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now}

	// First Sync: the pending delete is retried (delete of h_new1), the ambiguous
	// line still gets exactly one Warning, and nothing else is written.
	rep, warnings, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Message, "@hey") {
		t.Fatalf("want exactly 1 ambiguous-line Warning, got %d: %v", len(warnings), warnings)
	}
	if rep.Orphans != 0 || rep.Unlinked != 0 || rep.Imported != 0 {
		t.Errorf("no orphan/unlink/import expected: Orphans=%d Unlinked=%d Imported=%d", rep.Orphans, rep.Unlinked, rep.Imported)
	}
	if want := []string{"delete:h_new1"}; !equalStrings(client.mutations, want) {
		t.Errorf("mutations=%v, want %v (only the pending id deleted, no other write)", client.mutations, want)
	}
	// The ambiguous line is untouched on disk.
	if got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md")); string(got) != line {
		t.Errorf("ambiguous line must not be written:\n got: %q\nwant: %q", string(got), line)
	}
	// The pending-delete entry is kept until a later List confirms the Todo is gone
	// (dropping it now would let the import pass re-add it); the normal Link stands.
	st, _ := LoadState(statePath)
	if nw, ok := st.Links["h_new1"]; !ok || !nw.PendingDelete {
		t.Errorf("pending delete should be kept this Sync; state=%+v", st.Links)
	}
	if _, ok := st.Links["h1"]; !ok {
		t.Errorf("the normal Link h1 must survive; state=%+v", st.Links)
	}

	// The delete removed h_new1 from HEY. The next Sync sees it gone and drops the
	// entry, retrying no delete and still leaving the ambiguous line its one Warning.
	_, warn2, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("second Push: %v", err)
	}
	if len(warn2) != 1 {
		t.Errorf("second Sync should still warn once on the ambiguous line, got %d: %v", len(warn2), warn2)
	}
	if len(client.mutations) != 1 {
		t.Errorf("no further delete expected once the Todo is gone; mutations=%v", client.mutations)
	}
	st2, _ := LoadState(statePath)
	if _, ok := st2.Links["h_new1"]; ok {
		t.Errorf("pending-delete entry should be dropped once the Todo is gone; state=%+v", st2.Links)
	}
	if _, ok := st2.Links["h1"]; !ok {
		t.Errorf("the normal Link h1 must still survive; state=%+v", st2.Links)
	}
}

// TestOrphan_PendingDeleteOnAmbiguousLine_DryRunReportsSameWritesNothing checks
// that the same case in a dry run reports identically to a real Sync (a single
// ambiguous-line Warning, no orphan/unlink/import counted) yet touches nothing:
// no HEY delete, no notes write, no state write.
func TestOrphan_PendingDeleteOnAmbiguousLine_DryRunReportsSameWritesNothing(t *testing.T) {
	now := time.Now()
	line := "- [ ] Buy milk @today @hey(h1) @hey(h_new1)\n"
	task := model.TaskWith(model.Task{
		Text: "Buy milk @today @hey(h1) @hey(h_new1)", State: model.Open, HasCheckbox: true,
		Tags: []model.Tag{{Name: "today"}, {Name: "hey", Value: "h1"}, {Name: "hey", Value: "h_new1"}},
		File: "notes.md", Line: 1,
	})
	todos := []hey.Todo{openTodo("h1", "Buy milk"), openTodo("h_new1", "Buy milk")}

	run := func(t *testing.T, dry bool) (*Report, []model.Warning) {
		t.Helper()
		notesDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		statePath := filepath.Join(notesDir, "hey-state.json")
		if err := SaveState(statePath, &State{Links: map[string]Link{
			"h1":     {Title: "Buy milk", File: "notes.md", Line: 1},
			"h_new1": {PendingDelete: true, Title: "Buy milk", File: "notes.md", Line: 1},
		}}); err != nil {
			t.Fatal(err)
		}
		stateBefore, _ := os.ReadFile(statePath)

		var rep *Report
		var warnings []model.Warning
		var err error
		if dry {
			// dryFakeClient fails the test on any mutating verb, so a dry run that
			// tried to retry the delete would be caught.
			opts := Options{Tasks: []model.Task{task}, Client: &dryFakeClient{t: t, todos: append([]hey.Todo(nil), todos...)},
				Query: "@today", StatePath: statePath, NotesDir: notesDir, Now: now, DryRun: true}
			rep, warnings, err = Plan(context.Background(), opts)
		} else {
			opts := Options{Tasks: []model.Task{task}, Client: &recordingClient{week: now, todos: append([]hey.Todo(nil), todos...)},
				Query: "@today", StatePath: statePath, NotesDir: notesDir, Now: now}
			rep, warnings, err = Push(context.Background(), opts)
		}
		if err != nil {
			t.Fatalf("run (dry=%v): %v", dry, err)
		}
		if len(warnings) != 1 || !strings.Contains(warnings[0].Message, "@hey") {
			t.Fatalf("want exactly 1 ambiguous-line Warning (dry=%v), got %d: %v", dry, len(warnings), warnings)
		}
		if rep.Orphans != 0 || rep.WouldOrphan != 0 {
			t.Errorf("Orphans=%d WouldOrphan=%d (dry=%v), want 0/0", rep.Orphans, rep.WouldOrphan, dry)
		}
		if dry {
			// Nothing written: the ambiguous line and the state are byte-for-byte unchanged.
			if got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md")); string(got) != line {
				t.Errorf("dry run wrote the ambiguous line:\n got: %q\nwant: %q", string(got), line)
			}
			if got, _ := os.ReadFile(statePath); string(got) != string(stateBefore) {
				t.Errorf("dry run modified state:\n got: %s\nwant: %s", string(got), string(stateBefore))
			}
		}
		return rep, warnings
	}

	real, realWarn := run(t, false)
	dry, dryWarn := run(t, true)
	if len(realWarn) != len(dryWarn) || realWarn[0].Message != dryWarn[0].Message {
		t.Errorf("dry-run Warning differs from real:\n real: %q\n dry:  %q", realWarn[0].Message, dryWarn[0].Message)
	}
	if real.Orphans != dry.Orphans {
		t.Errorf("Orphans differ real=%d dry=%d", real.Orphans, dry.Orphans)
	}
}

// TestPendingDelete_FailedRetryWarnsAndCountsFailure covers a pending-delete
// retry whose delete fails: it must emit one Warning naming the id, count one
// failure, and keep the record so the next Sync retries it — rather than
// discarding the error silently.
func TestPendingDelete_FailedRetryWarnsAndCountsFailure(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte("- [ ] Something else\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(notesDir, "hey-state.json")
	if err := SaveState(statePath, &State{Links: map[string]Link{
		"h_leak": {PendingDelete: true, Title: "Leaked todo", File: "notes.md", Line: 1},
	}}); err != nil {
		t.Fatal(err)
	}
	// The leaked Todo is still in HEY and its delete fails this run.
	client := &recordingClient{week: now, deleteErr: errDeleteFailed, todos: []hey.Todo{openTodo("h_leak", "Leaked todo")}}
	opts := Options{Tasks: nil, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now}

	rep, warnings, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if rep.Failed != 1 {
		t.Errorf("Failed=%d, want 1 — a failed retry must count a failure", rep.Failed)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Message, "h_leak") {
		t.Fatalf("want exactly 1 Warning naming h_leak, got %d: %v", len(warnings), warnings)
	}
	// The delete was attempted.
	if want := []string{"delete:h_leak"}; !equalStrings(client.mutations, want) {
		t.Errorf("mutations=%v, want %v", client.mutations, want)
	}
	// The record is kept and retried next Sync.
	st, _ := LoadState(statePath)
	if link, ok := st.Links["h_leak"]; !ok || !link.PendingDelete {
		t.Errorf("pending-delete entry should be kept for retry; state=%+v", st.Links)
	}
	// The still-leaked Todo is named as outstanding, matching the one failure.
	if want := []Item{{ID: "h_leak", Title: "Leaked todo"}}; !reflect.DeepEqual(rep.PendingDeletes, want) {
		t.Errorf("PendingDeletes=%+v, want %+v — a failed retry is still outstanding", rep.PendingDeletes, want)
	}
}

// TestPendingDelete_SuccessfulRetryIsSilent covers a pending-delete retry whose
// delete succeeds: it emits no Warning and counts no failure, as before.
func TestPendingDelete_SuccessfulRetryIsSilent(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte("- [ ] Something else\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(notesDir, "hey-state.json")
	if err := SaveState(statePath, &State{Links: map[string]Link{
		"h_leak": {PendingDelete: true, Title: "Leaked todo", File: "notes.md", Line: 1},
	}}); err != nil {
		t.Fatal(err)
	}
	// The leaked Todo is still in HEY and its delete succeeds this run.
	client := &recordingClient{week: now, todos: []hey.Todo{openTodo("h_leak", "Leaked todo")}}
	opts := Options{Tasks: nil, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now}

	rep, warnings, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if rep.Failed != 0 {
		t.Errorf("Failed=%d, want 0 — a successful retry counts no failure", rep.Failed)
	}
	if len(warnings) != 0 {
		t.Errorf("a successful retry must stay silent, got warnings: %v", warnings)
	}
	if want := []string{"delete:h_leak"}; !equalStrings(client.mutations, want) {
		t.Errorf("mutations=%v, want %v", client.mutations, want)
	}
	// The retry cleared the Todo from HEY this run, so it is no longer outstanding:
	// the report must not name it (it self-clears the state entry next run).
	if len(rep.PendingDeletes) != 0 {
		t.Errorf("PendingDeletes=%+v, want empty — a cleared pending delete is not outstanding", rep.PendingDeletes)
	}
}

// TestReport_NamesOrphansAndPendingDeletes_AcrossSyncsAndRenders covers acceptance
// B: the report carries the identity (id and title) of every Orphan and every
// outstanding pending delete, in text and JSON, on the first Sync and again on a
// second Sync where the Orphan's warn-once Warning no longer fires.
func TestReport_NamesOrphansAndPendingDeletes_AcrossSyncsAndRenders(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	// No Task line for either id: h_lost is an Orphan; h_leak is a pending delete.
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte("- [ ] Something else\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(notesDir, "hey-state.json")
	if err := SaveState(statePath, &State{Links: map[string]Link{
		"h_lost": {Title: "Gone task", File: "notes.md", Line: 9},
		"h_leak": {PendingDelete: true, Title: "Leaked todo", File: "notes.md", Line: 1},
	}}); err != nil {
		t.Fatal(err)
	}
	// Both Todos survive in HEY; the pending delete keeps failing so it persists
	// across both Syncs and stays outstanding.
	client := &recordingClient{week: now, deleteErr: errDeleteFailed, todos: []hey.Todo{
		openTodo("h_lost", "Gone task"), openTodo("h_leak", "Leaked todo"),
	}}
	opts := Options{Tasks: nil, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now}

	wantOrphans := []Item{{ID: "h_lost", Title: "Gone task"}}
	wantPending := []Item{{ID: "h_leak", Title: "Leaked todo"}}

	assertNamed := func(t *testing.T, pass string, rep *Report) {
		t.Helper()
		if !reflect.DeepEqual(rep.OrphanItems, wantOrphans) {
			t.Errorf("%s: OrphanItems=%+v, want %+v", pass, rep.OrphanItems, wantOrphans)
		}
		if !reflect.DeepEqual(rep.PendingDeletes, wantPending) {
			t.Errorf("%s: PendingDeletes=%+v, want %+v", pass, rep.PendingDeletes, wantPending)
		}
		// Text names both by id and title.
		var text bytes.Buffer
		if err := rep.WriteText(&text); err != nil {
			t.Fatalf("%s WriteText: %v", pass, err)
		}
		for _, want := range []string{"h_lost", "Gone task", "h_leak", "Leaked todo"} {
			if !strings.Contains(text.String(), want) {
				t.Errorf("%s: text output missing %q:\n%s", pass, want, text.String())
			}
		}
		// JSON carries them as arrays.
		var js bytes.Buffer
		if err := rep.WriteJSON(&js); err != nil {
			t.Fatalf("%s WriteJSON: %v", pass, err)
		}
		var got Report
		if err := json.Unmarshal(js.Bytes(), &got); err != nil {
			t.Fatalf("%s: JSON invalid: %v\n%s", pass, err, js.String())
		}
		if !reflect.DeepEqual(got.OrphanItems, wantOrphans) || !reflect.DeepEqual(got.PendingDeletes, wantPending) {
			t.Errorf("%s: JSON arrays = %+v / %+v, want %+v / %+v", pass, got.OrphanItems, got.PendingDeletes, wantOrphans, wantPending)
		}
	}

	rep1, warn1, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("first Push: %v", err)
	}
	// First Sync: the Orphan warns once and the failed pending delete warns.
	if len(warn1) != 2 {
		t.Fatalf("first Sync want 2 Warnings (orphan + pending), got %d: %v", len(warn1), warn1)
	}
	assertNamed(t, "first Sync", rep1)

	rep2, warn2, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("second Push: %v", err)
	}
	// Second Sync: the Orphan's warn-once Warning is suppressed; only the pending
	// delete still warns. Both are still named in the report.
	if len(warn2) != 1 || !strings.Contains(warn2[0].Message, "h_leak") {
		t.Fatalf("second Sync want only the pending-delete Warning, got %d: %v", len(warn2), warn2)
	}
	assertNamed(t, "second Sync", rep2)
}

// TestReport_DeterministicOrderAcrossRuns covers acceptance A: with several
// Orphans and several outstanding pending deletes, two Syncs over unchanged state
// name them in the same id-sorted order, so the rendered text and JSON are
// byte-identical between runs and diffing two reports is quiet.
func TestReport_DeterministicOrderAcrossRuns(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	// No Task line for any id: the h_o* ids are Orphans, the h_p* ids pending
	// deletes. The Orphans are pre-marked so no state changes between runs.
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte("- [ ] Something else\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(notesDir, "hey-state.json")
	links := map[string]Link{}
	var todos []hey.Todo
	// Insert in an order that is not the sorted order.
	for _, id := range []string{"h_o3", "h_o1", "h_o4", "h_o2"} {
		links[id] = Link{Title: "orphan " + id, File: "notes.md", Line: 9, Orphaned: true}
		todos = append(todos, openTodo(id, "orphan "+id))
	}
	for _, id := range []string{"h_p2", "h_p4", "h_p1", "h_p3"} {
		links[id] = Link{PendingDelete: true, Title: "pending " + id, File: "notes.md", Line: 1}
		todos = append(todos, openTodo(id, "pending "+id))
	}
	if err := SaveState(statePath, &State{Links: links}); err != nil {
		t.Fatal(err)
	}
	// The pending deletes keep failing so they stay outstanding across both runs.
	client := &recordingClient{week: now, deleteErr: errDeleteFailed, todos: todos}
	opts := Options{Tasks: nil, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now}

	wantOrphans := []Item{
		{ID: "h_o1", Title: "orphan h_o1"}, {ID: "h_o2", Title: "orphan h_o2"},
		{ID: "h_o3", Title: "orphan h_o3"}, {ID: "h_o4", Title: "orphan h_o4"},
	}
	wantPending := []Item{
		{ID: "h_p1", Title: "pending h_p1"}, {ID: "h_p2", Title: "pending h_p2"},
		{ID: "h_p3", Title: "pending h_p3"}, {ID: "h_p4", Title: "pending h_p4"},
	}

	render := func(t *testing.T, rep *Report) (string, string) {
		t.Helper()
		if !reflect.DeepEqual(rep.OrphanItems, wantOrphans) {
			t.Errorf("OrphanItems=%+v, want id-sorted %+v", rep.OrphanItems, wantOrphans)
		}
		if !reflect.DeepEqual(rep.PendingDeletes, wantPending) {
			t.Errorf("PendingDeletes=%+v, want id-sorted %+v", rep.PendingDeletes, wantPending)
		}
		var text, js bytes.Buffer
		if err := rep.WriteText(&text); err != nil {
			t.Fatalf("WriteText: %v", err)
		}
		if err := rep.WriteJSON(&js); err != nil {
			t.Fatalf("WriteJSON: %v", err)
		}
		return text.String(), js.String()
	}

	rep1, _, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("first Push: %v", err)
	}
	text1, js1 := render(t, rep1)

	rep2, _, err := Push(context.Background(), opts)
	if err != nil {
		t.Fatalf("second Push: %v", err)
	}
	text2, js2 := render(t, rep2)

	if text1 != text2 {
		t.Errorf("text output differs between runs:\n run1:\n%s\n run2:\n%s", text1, text2)
	}
	if js1 != js2 {
		t.Errorf("JSON output differs between runs:\n run1:\n%s\n run2:\n%s", js1, js2)
	}
}

// TestReport_DryRunNamesSameItemsWritesNothing covers acceptance B for a dry run:
// it names the same Orphan and pending delete as a real Sync yet writes nothing
// on either side or to the state file — so `pike --sync --dry-run --json` always
// names what is outstanding.
func TestReport_DryRunNamesSameItemsWritesNothing(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	notes := "- [ ] Something else\n"
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte(notes), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(notesDir, "hey-state.json")
	if err := SaveState(statePath, &State{Links: map[string]Link{
		"h_lost": {Title: "Gone task", File: "notes.md", Line: 9},
		"h_leak": {PendingDelete: true, Title: "Leaked todo", File: "notes.md", Line: 1},
	}}); err != nil {
		t.Fatal(err)
	}
	stateBefore, _ := os.ReadFile(statePath)
	// dryFakeClient fails the test on any mutating verb, so a dry run that tried
	// to retry the delete would be caught.
	client := &dryFakeClient{t: t, todos: []hey.Todo{
		openTodo("h_lost", "Gone task"), openTodo("h_leak", "Leaked todo"),
	}}
	rep, _, err := Plan(context.Background(), Options{
		Tasks: nil, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now, DryRun: true,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if want := []Item{{ID: "h_lost", Title: "Gone task"}}; !reflect.DeepEqual(rep.OrphanItems, want) {
		t.Errorf("OrphanItems=%+v, want %+v", rep.OrphanItems, want)
	}
	if want := []Item{{ID: "h_leak", Title: "Leaked todo"}}; !reflect.DeepEqual(rep.PendingDeletes, want) {
		t.Errorf("PendingDeletes=%+v, want %+v", rep.PendingDeletes, want)
	}
	// Nothing written: notes and state are byte-for-byte unchanged.
	if got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md")); string(got) != notes {
		t.Errorf("dry run modified notes:\n%s", string(got))
	}
	if got, _ := os.ReadFile(statePath); string(got) != string(stateBefore) {
		t.Errorf("dry run modified state:\n%s", string(got))
	}
}

// TestReport_DryRunOmitsOrphansAlreadyGoneFromHey covers acceptance C: a dry run
// must report what a real run would leave outstanding. An Orphan whose Todo has
// been deleted from HEY, or completed there, is one a real Sync would drop, so it
// is neither counted nor named; only the genuinely outstanding Orphan is. The dry
// run still writes nothing to HEY, the notes, or the state file.
func TestReport_DryRunOmitsOrphansAlreadyGoneFromHey(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	notes := "- [ ] Something else\n"
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte(notes), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(notesDir, "hey-state.json")
	// Three state entries with no Task line: h_here is a live Orphan; h_done was
	// completed in HEY; h_gone has left HEY entirely. Only h_here is outstanding.
	if err := SaveState(statePath, &State{Links: map[string]Link{
		"h_here": {Title: "Still here", File: "notes.md", Line: 9},
		"h_done": {Title: "Finished", File: "notes.md", Line: 9},
		"h_gone": {Title: "Vanished", File: "notes.md", Line: 9},
	}}); err != nil {
		t.Fatal(err)
	}
	stateBefore, _ := os.ReadFile(statePath)
	completedAt := now
	client := &dryFakeClient{t: t, todos: []hey.Todo{
		openTodo("h_here", "Still here"),
		{ID: "h_done", Title: "Finished", WeekStart: now, WeekEnd: now, Completed: &completedAt},
	}}
	rep, warnings, err := Plan(context.Background(), Options{
		Tasks: nil, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now, DryRun: true,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	// Only the genuinely outstanding Orphan is counted and named.
	if rep.WouldOrphan != 1 {
		t.Errorf("WouldOrphan=%d, want 1 — gone/completed Orphans are not outstanding", rep.WouldOrphan)
	}
	if want := []Item{{ID: "h_here", Title: "Still here"}}; !reflect.DeepEqual(rep.OrphanItems, want) {
		t.Errorf("OrphanItems=%+v, want %+v", rep.OrphanItems, want)
	}
	// Only h_here draws a Warning; the resolved entries are silent.
	if len(warnings) != 1 || !strings.Contains(warnings[0].Message, "h_here") {
		t.Fatalf("want exactly 1 Warning naming h_here, got %d: %v", len(warnings), warnings)
	}
	// Nothing written: notes and state are byte-for-byte unchanged.
	if got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md")); string(got) != notes {
		t.Errorf("dry run modified notes:\n%s", string(got))
	}
	if got, _ := os.ReadFile(statePath); string(got) != string(stateBefore) {
		t.Errorf("dry run modified state:\n%s", string(got))
	}
}

// TestOrphan_DryRunReportsBothCasesWritesNothing covers the planning pass: a
// dry run counts both an unlink and an Orphan and touches nothing on either side
// or in the state file.
func TestOrphan_DryRunReportsBothCasesWritesNothing(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	// One Linked Task whose Todo is gone (would unlink); the notes carry no Task
	// for h_lost (would orphan).
	notes := "- [ ] Buy milk @today @hey(h_gone)\n"
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte(notes), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(notesDir, "hey-state.json")
	if err := SaveState(statePath, &State{Links: map[string]Link{
		"h_gone": {Title: "Buy milk", File: "notes.md", Line: 1},
		"h_lost": {Title: "Gone task", File: "notes.md", Line: 9},
	}}); err != nil {
		t.Fatal(err)
	}
	stateBefore, _ := os.ReadFile(statePath)
	// HEY still lists h_lost but not h_gone.
	client := &noDeleteClient{t: t, todos: []hey.Todo{{ID: "h_lost", Title: "Gone task", WeekStart: now, WeekEnd: now}}}
	task := model.TaskWith(model.Task{
		Text: "Buy milk @today @hey(h_gone)", State: model.Open, HasCheckbox: true,
		Tags: []model.Tag{{Name: "today"}, {Name: "hey", Value: "h_gone"}}, File: "notes.md", Line: 1,
	})

	rep, warnings, err := Plan(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now, DryRun: true,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if rep.WouldUnlink != 1 {
		t.Errorf("WouldUnlink = %d, want 1", rep.WouldUnlink)
	}
	if rep.WouldOrphan != 1 {
		t.Errorf("WouldOrphan = %d, want 1", rep.WouldOrphan)
	}
	// The orphaned Todo (h_lost) must not be counted as an import.
	if rep.WouldImport != 0 {
		t.Errorf("WouldImport = %d, want 0 — an orphaned Todo is not unlinked", rep.WouldImport)
	}
	// The orphan Warning still reaches the user in a dry run.
	if len(warnings) != 1 {
		t.Fatalf("want 1 Warning (the Orphan), got %d: %v", len(warnings), warnings)
	}
	// Nothing written: notes and state are byte-for-byte unchanged.
	if got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md")); string(got) != notes {
		t.Errorf("notes modified during dry run:\n%s", string(got))
	}
	if got, _ := os.ReadFile(statePath); string(got) != string(stateBefore) {
		t.Errorf("state modified during dry run:\n%s", string(got))
	}
}
