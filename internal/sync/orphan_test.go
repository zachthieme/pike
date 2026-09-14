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
