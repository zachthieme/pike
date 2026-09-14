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

// fakeClient is a Client that returns canned Todos and fails the test if any
// mutating verb is called, so tests can assert a dry run writes nothing to HEY.
type fakeClient struct {
	t     *testing.T
	todos []hey.Todo
	err   error
}

func (f *fakeClient) List(context.Context) ([]hey.Todo, error) { return f.todos, f.err }

func (f *fakeClient) Add(context.Context, string, *time.Time) (hey.Todo, error) {
	f.t.Fatal("Add called during a dry run")
	return hey.Todo{}, nil
}
func (f *fakeClient) Complete(context.Context, string) error {
	f.t.Fatal("Complete called during a dry run")
	return nil
}
func (f *fakeClient) Uncomplete(context.Context, string) error {
	f.t.Fatal("Uncomplete called during a dry run")
	return nil
}
func (f *fakeClient) Delete(context.Context, string) error {
	f.t.Fatal("Delete called during a dry run")
	return nil
}

func openTask(text string, tags ...model.Tag) model.Task {
	return model.TaskWith(model.Task{Text: text, State: model.Open, HasCheckbox: true, Tags: tags, File: "notes.md", Line: 1})
}

func openTodo(id, title string) hey.Todo {
	return hey.Todo{ID: id, Title: title, WeekStart: time.Now(), WeekEnd: time.Now()}
}

func TestPlan_WouldPushHonoursEligibilityAndQuery(t *testing.T) {
	now := time.Now()
	completed := now
	tasks := []model.Task{
		openTask("Buy milk @today", model.Tag{Name: "today"}),                                                                                                                            // eligible + matches → push
		openTask("Linked task @today @hey(h1)", model.Tag{Name: "today"}, model.Tag{Name: "hey", Value: "h1"}),                                                                           // linked → existing link, not pushed
		openTask("Hidden task @today @hidden", model.Tag{Name: "today"}, model.Tag{Name: "hidden"}),                                                                                      // hidden → not pushed
		model.TaskWith(model.Task{Text: "Bullet @today", State: model.Open, HasCheckbox: false, Tags: []model.Tag{{Name: "today"}}, File: "notes.md", Line: 4}),                          // no checkbox → not pushed
		model.TaskWith(model.Task{Text: "Done @today", State: model.Completed, HasCheckbox: true, Tags: []model.Tag{{Name: "today"}}, Completed: &completed, File: "notes.md", Line: 5}), // completed → not pushed
		openTask("Someday reading"), // no @today/@due → does not match query
	}

	client := &fakeClient{
		t: t,
		todos: []hey.Todo{
			openTodo("h2", "Unlinked open todo"), // import candidate
			openTodo("h1", "Linked open todo"),   // already linked → skip
			{ID: "h3", Title: "Completed todo", WeekStart: now, WeekEnd: now, Completed: &completed}, // completed → skip
		},
	}

	rep, warnings, err := Plan(context.Background(), Options{
		Tasks:  tasks,
		Client: client,
		Query:  "@due or @today",
		Now:    now,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if rep.WouldPush != 1 {
		t.Errorf("WouldPush = %d, want 1", rep.WouldPush)
	}
	if rep.ExistingLinks != 1 {
		t.Errorf("ExistingLinks = %d, want 1", rep.ExistingLinks)
	}
	if rep.WouldImport != 1 {
		t.Errorf("WouldImport = %d, want 1", rep.WouldImport)
	}
}

func TestPlan_ReadsStateFileIfPresent(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "hey-state.json")
	if err := os.WriteFile(statePath, []byte(`{"version":1,"links":{"h1":{"title":"Old","file":"notes.md","line":2}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// The state entry h1 has both a live Task in the notes and a Todo in HEY, so
	// it is a normal existing Link — not an Orphan.
	client := &fakeClient{t: t, todos: []hey.Todo{openTodo("h1", "Old")}}
	_, warnings, err := Plan(context.Background(), Options{
		Tasks: []model.Task{
			openTask("Buy milk @today", model.Tag{Name: "today"}),
			openTask("Old @hey(h1)", model.Tag{Name: "hey", Value: "h1"}),
		},
		Client:    client,
		Query:     "@due or @today",
		StatePath: statePath,
		Now:       time.Now(),
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("valid state file should not warn: %v", warnings)
	}
}

func TestPlan_CorruptStateFileWarns(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "hey-state.json")
	if err := os.WriteFile(statePath, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	client := &fakeClient{t: t}
	_, warnings, err := Plan(context.Background(), Options{
		Tasks:     nil,
		Client:    client,
		Query:     "@due or @today",
		StatePath: statePath,
		Now:       time.Now(),
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("Plan should not fail on corrupt state: %v", err)
	}
	if len(warnings) == 0 {
		t.Error("expected a warning about the corrupt state file")
	}
}

func TestPlan_ListErrorPropagates(t *testing.T) {
	client := &fakeClient{t: t, err: hey.ErrUnauthenticated}
	_, _, err := Plan(context.Background(), Options{
		Tasks:  nil,
		Client: client,
		Query:  "@due or @today",
		Now:    time.Now(),
		DryRun: true,
	})
	if err == nil {
		t.Fatal("expected Plan to propagate the List error")
	}
}

func TestSaveState_CreatesMissingParentDir(t *testing.T) {
	// The default state path lives under ~/.local/share/pike, which may not
	// exist on a fresh machine. Saving must create the parent directory.
	path := filepath.Join(t.TempDir(), "share", "pike", "hey-state.json")
	st := &State{Links: map[string]Link{"133760954": {Title: "Buy milk"}}}
	if err := SaveState(path, st); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("state file should be written: %v", err)
	}
	if len(data) == 0 {
		t.Error("state file is empty")
	}
}

func TestLoadState_AbsentIsEmpty(t *testing.T) {
	st, err := LoadState(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("absent state should not error: %v", err)
	}
	if st == nil {
		t.Fatal("expected non-nil state")
	}
	if len(st.Links) != 0 {
		t.Errorf("expected empty Links, got %v", st.Links)
	}
}
