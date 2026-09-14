package sync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/zachthieme/pike/internal/hey"
	"github.com/zachthieme/pike/internal/model"
)

// errDeleteFailed stands in for a HEY delete that fails during a Re-create.
var errDeleteFailed = errors.New("hey delete failed")

// titleClient records Add and Delete calls in order so title-sync tests can
// assert what pike sent to HEY and in what sequence. A just-added Todo shows up
// in the todo list, and Delete can be made to fail via deleteErr.
type titleClient struct {
	todos       []hey.Todo
	calls       []string
	week        time.Time
	nextID      int
	deleteErr   error
	lastAddDate time.Time // the date passed to the most recent Add, for reschedule assertions
}

func (c *titleClient) List(context.Context) ([]hey.Todo, error) { return c.todos, nil }

func (c *titleClient) Add(_ context.Context, title string, date *time.Time) (hey.Todo, error) {
	c.calls = append(c.calls, "add:"+title)
	if date != nil {
		c.lastAddDate = *date
	}
	c.nextID++
	td := hey.Todo{ID: "h_new" + strconv.Itoa(c.nextID), Title: title, WeekStart: c.week, Updated: c.week}
	c.todos = append(c.todos, td)
	return td, nil
}

func (c *titleClient) Complete(_ context.Context, id string) error {
	c.calls = append(c.calls, "complete:"+id)
	return nil
}

func (c *titleClient) Uncomplete(_ context.Context, id string) error {
	c.calls = append(c.calls, "uncomplete:"+id)
	return nil
}

func (c *titleClient) Delete(_ context.Context, id string) error {
	c.calls = append(c.calls, "delete:"+id)
	return c.deleteErr
}

func TestResolveTitle_DecisionMatrix(t *testing.T) {
	tests := []struct {
		name                            string
		notesTitle, heyTitle, baseTitle string
		hasBase, todoCompleted          bool
		want                            titleAction
	}{
		{"all agree", "Ship it", "Ship it", "Ship it", true, false, titleNone},
		{"hey renamed only", "Ship it", "Deploy it", "Ship it", true, false, titleRetitle},
		{"notes renamed only", "Deploy it", "Ship it", "Ship it", true, false, titleRecreate},
		{"both renamed differently, notes win", "Deploy it", "Launch it", "Ship it", true, false, titleRecreate},
		{"both renamed the same way", "Ship v2", "Ship v2", "Ship it", true, false, titleNone},
		// A completed Todo is never retitled or re-created, whichever side moved.
		{"hey renamed but todo completed", "Ship it", "Deploy it", "Ship it", true, true, titleNone},
		{"notes renamed but todo completed", "Deploy it", "Ship it", "Ship it", true, true, titleNone},
		// Without a base snapshot the notes win on text, so the Todo is Re-created.
		{"no base, sides differ", "Deploy it", "Ship it", "", false, false, titleRecreate},
		// A completed Todo is never touched, base or not.
		{"no base, sides differ but todo completed", "Deploy it", "Ship it", "", false, true, titleNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveTitle(tt.notesTitle, tt.heyTitle, tt.baseTitle, tt.hasBase, tt.todoCompleted)
			if got != tt.want {
				t.Errorf("resolveTitle(%q,%q,%q,%v,%v) = %v, want %v",
					tt.notesTitle, tt.heyTitle, tt.baseTitle, tt.hasBase, tt.todoCompleted, got, tt.want)
			}
		})
	}
}

func TestSyncTitle_HeyRenamed_RetitlesTaskKeepingTags(t *testing.T) {
	now := time.Now()
	due := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [ ] Ship it @hey(h1) @due(2026-09-20)",
		model.Task{Text: "Ship it @hey(h1) @due(2026-09-20)", State: model.Open, HasCheckbox: true, Due: &due,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}, {Name: "due", Value: "2026-09-20"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", File: "notes.md", Line: 1}}},
	)
	// HEY renamed the Todo since the last Sync; the notes still say "Ship it".
	client := &titleClient{todos: []hey.Todo{{ID: "h1", Title: "Deploy it now", WeekStart: now}}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Retitled != 1 {
		t.Errorf("Retitled=%d, want 1", rep.Retitled)
	}
	if len(client.calls) != 0 {
		t.Errorf("no HEY mutation expected for a retitle; calls=%v", client.calls)
	}
	got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	want := "- [ ] Deploy it now @hey(h1) @due(2026-09-20)\n"
	if string(got) != want {
		t.Errorf("notes line\n got: %q\nwant: %q", string(got), want)
	}
	st, _ := LoadState(statePath)
	if st.Links["h1"].Title != "Deploy it now" {
		t.Errorf("state Title=%q, want %q", st.Links["h1"].Title, "Deploy it now")
	}
}

func TestSyncTitle_NotesRenamed_RecreatesTodoAddBeforeDelete(t *testing.T) {
	now := time.Now()
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [ ] Deploy it now @hey(h1)",
		model.Task{Text: "Deploy it now @hey(h1)", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", File: "notes.md", Line: 1}}},
	)
	// The notes were renamed; HEY still holds the old Title.
	client := &titleClient{todos: []hey.Todo{{ID: "h1", Title: "Ship it", WeekStart: now}}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Recreated != 1 {
		t.Errorf("Recreated=%d, want 1", rep.Recreated)
	}
	want := []string{"add:Deploy it now", "delete:h1"}
	if len(client.calls) != 2 || client.calls[0] != want[0] || client.calls[1] != want[1] {
		t.Errorf("calls=%v, want %v (add before delete)", client.calls, want)
	}
	got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if string(got) != "- [ ] Deploy it now @hey(h_new1)\n" {
		t.Errorf("notes @hey not rewritten to new id, got: %q", string(got))
	}
	st, _ := LoadState(statePath)
	if _, stillOld := st.Links["h1"]; stillOld {
		t.Errorf("old link h1 should be gone; state=%+v", st.Links)
	}
	if st.Links["h_new1"].Title != "Deploy it now" {
		t.Errorf("new link missing or wrong title; state=%+v", st.Links)
	}
}

func TestSyncTitle_BothRenamed_NotesWin(t *testing.T) {
	now := time.Now()
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [ ] Notes title @hey(h1)",
		model.Task{Text: "Notes title @hey(h1)", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}}},
		&State{Links: map[string]Link{"h1": {Title: "Original", File: "notes.md", Line: 1}}},
	)
	// Both sides diverged from the snapshot; the notes must win.
	client := &titleClient{todos: []hey.Todo{{ID: "h1", Title: "Hey title", WeekStart: now}}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Recreated != 1 || rep.Retitled != 0 {
		t.Errorf("Recreated=%d Retitled=%d, want 1/0", rep.Recreated, rep.Retitled)
	}
	if len(client.calls) == 0 || client.calls[0] != "add:Notes title" {
		t.Errorf("expected Todo re-created with the notes title; calls=%v", client.calls)
	}
}

func TestSyncTitle_NoStateFile_NotesTitleWinsRecreates(t *testing.T) {
	now := time.Now()
	// No prior state file: with differing Titles and an open Todo, the notes win
	// on text, so the Todo is Re-created with the notes' Title.
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [ ] Deploy it now @hey(h1)",
		model.Task{Text: "Deploy it now @hey(h1)", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}}},
		nil,
	)
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("state file should be absent at start (err: %v)", err)
	}
	client := &titleClient{todos: []hey.Todo{{ID: "h1", Title: "Ship it", WeekStart: now}}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Recreated != 1 || rep.Retitled != 0 {
		t.Errorf("Recreated=%d Retitled=%d, want 1/0", rep.Recreated, rep.Retitled)
	}
	want := []string{"add:Deploy it now", "delete:h1"}
	if len(client.calls) != 2 || client.calls[0] != want[0] || client.calls[1] != want[1] {
		t.Errorf("calls=%v, want %v", client.calls, want)
	}
	got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if string(got) != "- [ ] Deploy it now @hey(h_new1)\n" {
		t.Errorf("notes @hey not rewritten to new id, got: %q", string(got))
	}
	st, _ := LoadState(statePath)
	if st.Links["h_new1"].Title != "Deploy it now" {
		t.Errorf("new link missing or wrong title; state=%+v", st.Links)
	}
}

func TestSyncTitle_RecreateDeleteFails_OldIdKeptSupersededRetriedNextSync(t *testing.T) {
	now := time.Now()
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [ ] Deploy it now @hey(h1)",
		model.Task{Text: "Deploy it now @hey(h1)", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", File: "notes.md", Line: 1}}},
	)
	client := &titleClient{
		todos:     []hey.Todo{{ID: "h1", Title: "Ship it", WeekStart: now}},
		week:      now,
		deleteErr: errDeleteFailed,
	}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	// The Re-create still counts: the Link already points at the new Todo.
	if rep.Recreated != 1 {
		t.Errorf("Recreated=%d, want 1", rep.Recreated)
	}
	if len(warnings) != 1 {
		t.Fatalf("want one warning for the failed delete, got %v", warnings)
	}
	got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if string(got) != "- [ ] Deploy it now @hey(h_new1)\n" {
		t.Errorf("link should point at the new Todo, got: %q", string(got))
	}
	st, _ := LoadState(statePath)
	// The old id is kept, marked superseded (a pending delete), so it is never
	// imported as a second copy of the Task.
	old, stillOld := st.Links["h1"]
	if !stillOld || !old.PendingDelete {
		t.Errorf("old link h1 should be kept as a pending delete; state=%+v", st.Links)
	}
	if st.Links["h_new1"].Title != "Deploy it now" {
		t.Errorf("new link missing; state=%+v", st.Links)
	}

	// The next Sync re-scans the line (now linked to h_new1), retries the delete
	// of the old Todo, and imports nothing.
	relinked := model.TaskWith(model.Task{
		Text: "Deploy it now @hey(h_new1)", Raw: "- [ ] Deploy it now @hey(h_new1)",
		State: model.Open, HasCheckbox: true,
		Tags: []model.Tag{{Name: "hey", Value: "h_new1"}}, File: "notes.md", Line: 1,
	})
	callsBefore := len(client.calls)
	rep2, _, err := Push(context.Background(), Options{
		Tasks: []model.Task{relinked}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil {
		t.Fatalf("second Push: %v", err)
	}
	if rep2.Imported != 0 {
		t.Errorf("second Sync Imported=%d, want 0 (superseded Todo is never imported)", rep2.Imported)
	}
	retried := false
	for _, c := range client.calls[callsBefore:] {
		if c == "delete:h1" {
			retried = true
		}
	}
	if !retried {
		t.Errorf("second Sync should retry delete:h1; calls=%v", client.calls[callsBefore:])
	}
}

func TestSyncTitle_CompletedTodoRenamed_NothingWritten(t *testing.T) {
	now := time.Now()
	completedAt := now
	completed := now
	// The Todo is completed on both sides; the notes text was renamed. A
	// completed Todo is never Retitled or Re-created, and completion agrees, so
	// nothing is written anywhere.
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [x] New name @hey(h1) @completed(2026-09-10)",
		model.Task{Text: "New name @hey(h1) @completed(2026-09-10)", State: model.Completed, HasCheckbox: true,
			Completed: &completed, Tags: []model.Tag{{Name: "hey", Value: "h1"}, {Name: "completed", Value: "2026-09-10"}}},
		&State{Links: map[string]Link{"h1": {Title: "Old name", Completed: true, File: "notes.md", Line: 1}}},
	)
	notesBefore, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	client := &titleClient{todos: []hey.Todo{{ID: "h1", Title: "Old name", WeekStart: now, Completed: &completedAt}}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Retitled != 0 || rep.Recreated != 0 {
		t.Errorf("Retitled=%d Recreated=%d, want 0/0", rep.Retitled, rep.Recreated)
	}
	if len(client.calls) != 0 {
		t.Errorf("no HEY mutation expected on a completed Todo; calls=%v", client.calls)
	}
	notesAfter, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if string(notesAfter) != string(notesBefore) {
		t.Errorf("notes changed:\n got: %q\nwant: %q", string(notesAfter), string(notesBefore))
	}
}

func TestPlanTitle_DryRunCountsAndWritesNothing(t *testing.T) {
	now := time.Now()
	// One HEY-only rename (would retitle) and one notes-only rename (would
	// re-create). A dry run must count both and touch nothing.
	retitleTask := model.TaskWith(model.Task{Text: "Ship it @hey(h1)", State: model.Open, HasCheckbox: true,
		Tags: []model.Tag{{Name: "hey", Value: "h1"}}, File: "notes.md", Line: 1})
	recreateTask := model.TaskWith(model.Task{Text: "Renamed here @hey(h2)", State: model.Open, HasCheckbox: true,
		Tags: []model.Tag{{Name: "hey", Value: "h2"}}, File: "notes.md", Line: 2})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"),
		[]byte("- [ ] Ship it @hey(h1)\n- [ ] Renamed here @hey(h2)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "hey-state.json")
	if err := SaveState(statePath, &State{Links: map[string]Link{
		"h1": {Title: "Ship it", File: "notes.md", Line: 1},
		"h2": {Title: "Old name", File: "notes.md", Line: 2},
	}}); err != nil {
		t.Fatal(err)
	}
	notesBefore, _ := os.ReadFile(filepath.Join(dir, "notes.md"))
	stateBefore, _ := os.ReadFile(statePath)
	client := &dryFakeClient{t: t, todos: []hey.Todo{
		{ID: "h1", Title: "Deploy it", WeekStart: now}, // HEY renamed → would retitle
		{ID: "h2", Title: "Old name", WeekStart: now},  // notes renamed → would re-create
	}}

	rep, warnings, err := Plan(context.Background(), Options{
		Tasks: []model.Task{retitleTask, recreateTask}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: dir, Now: now, DryRun: true,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Plan: err=%v warnings=%v", err, warnings)
	}
	if rep.WouldRetitle != 1 || rep.WouldRecreate != 1 {
		t.Errorf("WouldRetitle=%d WouldRecreate=%d, want 1/1", rep.WouldRetitle, rep.WouldRecreate)
	}
	if rep.Retitled != 0 || rep.Recreated != 0 {
		t.Errorf("real counts should be zero on a dry run: Retitled=%d Recreated=%d", rep.Retitled, rep.Recreated)
	}
	notesAfter, _ := os.ReadFile(filepath.Join(dir, "notes.md"))
	stateAfter, _ := os.ReadFile(statePath)
	if string(notesAfter) != string(notesBefore) {
		t.Error("notes modified during a dry run")
	}
	if string(stateAfter) != string(stateBefore) {
		t.Error("state file modified during a dry run")
	}
}
