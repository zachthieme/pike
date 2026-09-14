package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zachthieme/pike/internal/hey"
	"github.com/zachthieme/pike/internal/model"
)

// verbCall records one mutating HEY call: the verb and the todo id.
type verbCall struct {
	verb string
	id   string
}

// completionClient records every mutating verb and hands back a fixed todo
// list, so completion-sync tests can assert what pike sent to HEY. completeErr,
// when set, makes the Complete verb fail so a write failure can be exercised.
type completionClient struct {
	todos       []hey.Todo
	calls       []verbCall
	completeErr error
}

func (c *completionClient) List(context.Context) ([]hey.Todo, error) { return c.todos, nil }
func (c *completionClient) Add(_ context.Context, title string, _ *time.Time) (hey.Todo, error) {
	return hey.Todo{ID: "h_add", Title: title}, nil
}
func (c *completionClient) Complete(_ context.Context, id string) error {
	c.calls = append(c.calls, verbCall{"complete", id})
	return c.completeErr
}
func (c *completionClient) Uncomplete(_ context.Context, id string) error {
	c.calls = append(c.calls, verbCall{"uncomplete", id})
	return nil
}
func (c *completionClient) Delete(context.Context, string) error { return nil }

func (c *completionClient) called(verb, id string) bool {
	for _, k := range c.calls {
		if k.verb == verb && k.id == id {
			return true
		}
	}
	return false
}

// linkFixture writes a one-line notes file for a Task Linked to id, seeds an
// optional prior state, and returns the Task, notes dir, and state path.
func linkFixture(t *testing.T, id, rawLine string, task model.Task, prior *State) (model.Task, string, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte(rawLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "hey-state.json")
	if prior != nil {
		if err := SaveState(statePath, prior); err != nil {
			t.Fatal(err)
		}
	}
	task.File = "notes.md"
	task.Line = 1
	task.Raw = rawLine
	return model.TaskWith(task), dir, statePath
}

func TestSyncCompletion_TaskCompletedTodoOpenStateOpen_CompletesTodo(t *testing.T) {
	now := time.Now()
	completed := now
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [x] Ship it @hey(h1) @completed(2026-09-12)",
		model.Task{Text: "Ship it @hey(h1) @completed(2026-09-12)", State: model.Completed, HasCheckbox: true,
			Completed: &completed, Tags: []model.Tag{{Name: "hey", Value: "h1"}, {Name: "completed", Value: "2026-09-12"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", Completed: false, File: "notes.md", Line: 1}}},
	)
	client := &completionClient{todos: []hey.Todo{{ID: "h1", Title: "Ship it", WeekStart: now, WeekEnd: now}}}

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
	if !client.called("complete", "h1") {
		t.Errorf("expected complete h1 to be called; calls=%v", client.calls)
	}
	if rep.Completed != 1 {
		t.Errorf("Completed=%d, want 1", rep.Completed)
	}
	// State now records the Todo as completed.
	st, err := LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Links["h1"].Completed {
		t.Errorf("state should record h1 completed; state=%+v", st.Links["h1"])
	}
}

func TestSyncCompletion_TodoCompletedTaskOpenStateOpen_ChecksLineInLocalDate(t *testing.T) {
	// A clock nine hours ahead of UTC: a completed_at late on the 12th UTC lands
	// on the 13th locally, so the stamped @completed date must be 2026-09-13.
	loc := time.FixedZone("test+9", 9*3600)
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, loc)
	completedAt := time.Date(2026, 9, 12, 23, 30, 0, 0, time.UTC)
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [ ] Read book @hey(h1)",
		model.Task{Text: "Read book @hey(h1)", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}}},
		&State{Links: map[string]Link{"h1": {Title: "Read book", Completed: false, File: "notes.md", Line: 1}}},
	)
	client := &completionClient{todos: []hey.Todo{{ID: "h1", Title: "Read book", WeekStart: now, WeekEnd: now, Completed: &completedAt}}}

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
	if rep.Completed != 1 {
		t.Errorf("Completed=%d, want 1", rep.Completed)
	}
	if len(client.calls) != 0 {
		t.Errorf("no HEY mutation expected; calls=%v", client.calls)
	}
	got, err := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "- [x] Read book @hey(h1) @completed(2026-09-13)\n"
	if string(got) != want {
		t.Errorf("notes line\n got: %q\nwant: %q", string(got), want)
	}
}

func TestSyncCompletion_TodoUncompletedTaskCompletedStateCompleted_UnchecksLine(t *testing.T) {
	now := time.Now()
	completed := now
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [x] Ship it @hey(h1) @completed(2026-09-12)",
		model.Task{Text: "Ship it @hey(h1) @completed(2026-09-12)", State: model.Completed, HasCheckbox: true,
			Completed: &completed, Tags: []model.Tag{{Name: "hey", Value: "h1"}, {Name: "completed", Value: "2026-09-12"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", Completed: true, File: "notes.md", Line: 1}}},
	)
	// The Todo has reopened in HEY since the last Sync.
	client := &completionClient{todos: []hey.Todo{{ID: "h1", Title: "Ship it", WeekStart: now, WeekEnd: now}}}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Uncompleted != 1 {
		t.Errorf("Uncompleted=%d, want 1", rep.Uncompleted)
	}
	got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	want := "- [ ] Ship it @hey(h1)\n"
	if string(got) != want {
		t.Errorf("notes line\n got: %q\nwant: %q", string(got), want)
	}
	st, _ := LoadState(statePath)
	if st.Links["h1"].Completed {
		t.Errorf("state should record h1 reopened; state=%+v", st.Links["h1"])
	}
}

func TestSyncCompletion_TaskUncompletedTodoCompletedStateCompleted_UncompletesTodo(t *testing.T) {
	now := time.Now()
	completedAt := now
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [ ] Ship it @hey(h1)",
		model.Task{Text: "Ship it @hey(h1)", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", Completed: true, File: "notes.md", Line: 1}}},
	)
	// The Todo is still completed in HEY; the Task was reopened locally.
	client := &completionClient{todos: []hey.Todo{{ID: "h1", Title: "Ship it", WeekStart: now, WeekEnd: now, Completed: &completedAt}}}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if !client.called("uncomplete", "h1") {
		t.Errorf("expected uncomplete h1; calls=%v", client.calls)
	}
	if rep.Uncompleted != 1 {
		t.Errorf("Uncompleted=%d, want 1", rep.Uncompleted)
	}
	// The notes line is untouched — the change flows to HEY.
	got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if string(got) != "- [ ] Ship it @hey(h1)\n" {
		t.Errorf("notes line should be unchanged, got: %q", string(got))
	}
}

func TestResolveCompletion_DecisionMatrix(t *testing.T) {
	tests := []struct {
		name                     string
		taskDone, todoDone, base bool
		hasBase                  bool
		want                     action
	}{
		{"agree open, base open", false, false, false, true, actNone},
		{"agree done, base done", true, true, true, true, actNone},
		{"task completed since sync", true, false, false, true, actCompleteTodo},
		{"todo completed since sync", false, true, false, true, actCompleteTask},
		{"task reopened since sync", false, true, true, true, actUncompleteTodo},
		{"todo reopened since sync", true, false, true, true, actUncompleteTask},
		// No base: completed always wins, in whichever direction.
		{"no base, task done wins", true, false, false, false, actCompleteTodo},
		{"no base, todo done wins", false, true, false, false, actCompleteTask},
		{"no base, both open", false, false, false, false, actNone},
		{"no base, both done", true, true, false, false, actNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveCompletion(tt.taskDone, tt.todoDone, tt.base, tt.hasBase)
			if got != tt.want {
				t.Errorf("resolveCompletion(%v,%v,%v,%v) = %v, want %v",
					tt.taskDone, tt.todoDone, tt.base, tt.hasBase, got, tt.want)
			}
		})
	}
}

func TestSyncCompletion_NoStateFile_CompletedWinsAndStateRebuilt(t *testing.T) {
	now := time.Now()
	completed := now
	// No prior state file: deleting it is a safe reset, so the Link resolves by
	// conflict rules alone. The Task is completed, the Todo open → completed wins.
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [x] Ship it @hey(h1) @completed(2026-09-12)",
		model.Task{Text: "Ship it @hey(h1) @completed(2026-09-12)", State: model.Completed, HasCheckbox: true,
			Completed: &completed, Tags: []model.Tag{{Name: "hey", Value: "h1"}, {Name: "completed", Value: "2026-09-12"}}},
		nil,
	)
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("state file should be absent at start (err: %v)", err)
	}
	client := &completionClient{todos: []hey.Todo{{ID: "h1", Title: "Ship it", WeekStart: now, WeekEnd: now}}}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if !client.called("complete", "h1") || rep.Completed != 1 {
		t.Errorf("expected completed side to win; calls=%v Completed=%d", client.calls, rep.Completed)
	}
	// The notes line is preserved — no data lost.
	got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if string(got) != "- [x] Ship it @hey(h1) @completed(2026-09-12)\n" {
		t.Errorf("notes should be preserved, got: %q", string(got))
	}
	// A fresh state file is written recording the agreed completion.
	st, err := LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Links["h1"].Completed {
		t.Errorf("state should be rebuilt with h1 completed; state=%+v", st.Links)
	}
}

func TestSyncCompletion_HeySideWriteFails_CountedAndWarned(t *testing.T) {
	now := time.Now()
	completed := now
	// The Task was completed since the last Sync, so the Todo would be completed
	// in HEY — but the HEY write fails. The failure is counted, not applied.
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [x] Ship it @hey(h1) @completed(2026-09-12)",
		model.Task{Text: "Ship it @hey(h1) @completed(2026-09-12)", State: model.Completed, HasCheckbox: true,
			Completed: &completed, Tags: []model.Tag{{Name: "hey", Value: "h1"}, {Name: "completed", Value: "2026-09-12"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", Completed: false, File: "notes.md", Line: 1}}},
	)
	client := &completionClient{
		todos:       []hey.Todo{{ID: "h1", Title: "Ship it", WeekStart: now, WeekEnd: now}},
		completeErr: context.DeadlineExceeded,
	}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if rep.Failed != 1 {
		t.Errorf("Failed=%d, want 1", rep.Failed)
	}
	if rep.Completed != 0 {
		t.Errorf("Completed=%d, want 0 — the write failed", rep.Completed)
	}
	if len(warnings) != 1 {
		t.Fatalf("want 1 Warning for the failed write, got %d: %v", len(warnings), warnings)
	}
}

func TestPlanCompletion_DryRunCountsAndWritesNothing(t *testing.T) {
	now := time.Now()
	completed := now
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [x] Ship it @hey(h1) @completed(2026-09-12)",
		model.Task{Text: "Ship it @hey(h1) @completed(2026-09-12)", State: model.Completed, HasCheckbox: true,
			Completed: &completed, Tags: []model.Tag{{Name: "hey", Value: "h1"}, {Name: "completed", Value: "2026-09-12"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", Completed: false, File: "notes.md", Line: 1}}},
	)
	notesBefore, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	stateBefore, _ := os.ReadFile(statePath)
	// A client whose mutating verbs fail the test: a dry run must not call them.
	client := &dryFakeClient{t: t, todos: []hey.Todo{{ID: "h1", Title: "Ship it", WeekStart: now, WeekEnd: now}}}

	rep, warnings, err := Plan(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now, DryRun: true,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Plan: err=%v warnings=%v", err, warnings)
	}
	if rep.WouldComplete != 1 || rep.Completed != 0 {
		t.Errorf("WouldComplete=%d Completed=%d, want 1/0", rep.WouldComplete, rep.Completed)
	}
	notesAfter, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	stateAfter, _ := os.ReadFile(statePath)
	if string(notesAfter) != string(notesBefore) {
		t.Error("notes modified during a dry run")
	}
	if string(stateAfter) != string(stateBefore) {
		t.Error("state file modified during a dry run")
	}
}
