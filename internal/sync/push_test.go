package sync

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zachthieme/pike/internal/hey"
	"github.com/zachthieme/pike/internal/model"
)

// addCall records one invocation of the HEY client's Add verb.
type addCall struct {
	title string
	date  *time.Time
}

// recordingClient is a Client that records Add calls and hands back freshly
// created Todos. It lets push tests assert what pike sent to HEY. A title listed
// in addErr fails that one push without stopping the run.
type recordingClient struct {
	todos  []hey.Todo
	listed int
	added  []addCall
	addErr map[string]error
	week   time.Time
	nextID int
}

func (c *recordingClient) List(context.Context) ([]hey.Todo, error) {
	c.listed++
	return c.todos, nil
}

func (c *recordingClient) Add(_ context.Context, title string, date *time.Time) (hey.Todo, error) {
	c.added = append(c.added, addCall{title: title, date: date})
	if err := c.addErr[title]; err != nil {
		return hey.Todo{}, err
	}
	c.nextID++
	return hey.Todo{
		ID:        "h_new" + strconv.Itoa(c.nextID),
		Title:     title,
		WeekStart: c.week,
		Updated:   c.week,
	}, nil
}

func (c *recordingClient) Complete(context.Context, string) error   { return nil }
func (c *recordingClient) Uncomplete(context.Context, string) error { return nil }
func (c *recordingClient) Delete(context.Context, string) error     { return nil }

// pushFixture writes a notes file and returns a Task pointing at its first line,
// along with the notes dir and state path for a push.
func pushFixture(t *testing.T, line string, tags ...model.Tag) (task model.Task, notesDir, statePath string) {
	t.Helper()
	notesDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte("- [ ] "+line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath = filepath.Join(notesDir, "hey-state.json")
	task = model.TaskWith(model.Task{Text: line, State: model.Open, HasCheckbox: true, Tags: tags, File: "notes.md", Line: 1})
	return task, notesDir, statePath
}

func TestPush_CreatesTodoAppendsTagAndRecordsLink(t *testing.T) {
	now := time.Now()
	task, notesDir, statePath := pushFixture(t, "Buy milk @today", model.Tag{Name: "today"})
	client := &recordingClient{week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks:     []model.Task{task},
		Client:    client,
		Query:     "@due or @today",
		StatePath: statePath,
		NotesDir:  notesDir,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if rep.Pushed != 1 || rep.Failed != 0 {
		t.Errorf("Pushed=%d Failed=%d, want 1/0", rep.Pushed, rep.Failed)
	}

	// The Title is the Task text with every Tag removed.
	if len(client.added) != 1 {
		t.Fatalf("Add called %d times, want 1", len(client.added))
	}
	if client.added[0].title != "Buy milk" {
		t.Errorf("add title = %q, want %q", client.added[0].title, "Buy milk")
	}
	if client.added[0].date != nil {
		t.Errorf("add date = %v, want nil (no @due)", client.added[0].date)
	}

	// Exactly one @hey(id) appended; the rest of the line is unchanged.
	got, err := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "- [ ] Buy milk @today @hey(h_new1)\n"; string(got) != want {
		t.Errorf("notes line\n got: %q\nwant: %q", string(got), want)
	}

	// The Link is recorded in the state file.
	st, err := LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	link, ok := st.Links["h_new1"]
	if !ok {
		t.Fatalf("no link recorded for h_new1; state = %+v", st)
	}
	if link.Title != "Buy milk" || link.File != "notes.md" || link.Line != 1 {
		t.Errorf("link = %+v, want Title=Buy milk File=notes.md Line=1", link)
	}
}

func TestPush_StripsMidTextTagsAndPassesDueDate(t *testing.T) {
	now := time.Now()
	due := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	task, notesDir, statePath := pushFixture(t, "Call @urgent mom about @due(2026-09-16) lunch",
		model.Tag{Name: "urgent"}, model.Tag{Name: "due", Value: "2026-09-16"})
	task.Due = &due
	client := &recordingClient{week: now}

	_, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@due or @today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if len(client.added) != 1 {
		t.Fatalf("Add called %d times, want 1", len(client.added))
	}
	if client.added[0].title != "Call mom about lunch" {
		t.Errorf("title = %q, want %q", client.added[0].title, "Call mom about lunch")
	}
	if client.added[0].date == nil || !client.added[0].date.Equal(due) {
		t.Errorf("add date = %v, want %v", client.added[0].date, due)
	}
}

func TestPush_SecondRunPushesNothingAndWritesNothing(t *testing.T) {
	now := time.Now()
	task, notesDir, statePath := pushFixture(t, "Buy milk @today", model.Tag{Name: "today"})
	client := &recordingClient{week: now}

	// First sync links the task.
	if _, _, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@due or @today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	}); err != nil {
		t.Fatalf("first Push: %v", err)
	}

	// Re-scan: the task line now carries its @hey Link.
	linked := model.TaskWith(model.Task{
		Text:  "Buy milk @today @hey(h_new1)",
		State: model.Open, HasCheckbox: true,
		Tags: []model.Tag{{Name: "today"}, {Name: "hey", Value: "h_new1"}},
		File: "notes.md", Line: 1,
	})
	stateBefore, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	notesBefore, err := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	addsBefore := len(client.added)
	listsBefore := client.listed

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{linked}, Client: client, Query: "@due or @today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("second Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Pushed != 0 || rep.ExistingLinks != 1 {
		t.Errorf("Pushed=%d ExistingLinks=%d, want 0/1", rep.Pushed, rep.ExistingLinks)
	}
	if len(client.added) != addsBefore {
		t.Errorf("second run made %d Add call(s); want none beyond list", len(client.added)-addsBefore)
	}
	if client.listed != listsBefore+1 {
		t.Errorf("second run made %d List calls, want exactly 1", client.listed-listsBefore)
	}
	stateAfter, _ := os.ReadFile(statePath)
	if string(stateAfter) != string(stateBefore) {
		t.Error("state file was rewritten on a no-op sync")
	}
	notesAfter, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if string(notesAfter) != string(notesBefore) {
		t.Error("notes file was rewritten on a no-op sync")
	}
}

func TestPush_StaleLineSkippedWithWarningNotCorrupted(t *testing.T) {
	now := time.Now()
	task, notesDir, statePath := pushFixture(t, "Buy milk @today", model.Tag{Name: "today"})
	// The scan saw "Buy milk @today"; the file has since been edited.
	reworded := "- [ ] Totally different wording\n"
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte(reworded), 0o644); err != nil {
		t.Fatal(err)
	}
	client := &recordingClient{week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@due or @today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if rep.Failed != 1 || rep.Pushed != 0 {
		t.Errorf("Failed=%d Pushed=%d, want 1/0", rep.Failed, rep.Pushed)
	}
	if len(warnings) != 1 {
		t.Fatalf("want 1 warning, got %d: %v", len(warnings), warnings)
	}
	if got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md")); string(got) != reworded {
		t.Errorf("stale line corrupted:\n%s", string(got))
	}
}

func TestPush_PerTaskFailureDoesNotStopRun(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	notes := "- [ ] First @today\n- [ ] Second @today\n"
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte(notes), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(notesDir, "hey-state.json")
	first := model.TaskWith(model.Task{Text: "First @today", State: model.Open, HasCheckbox: true, Tags: []model.Tag{{Name: "today"}}, File: "notes.md", Line: 1})
	second := model.TaskWith(model.Task{Text: "Second @today", State: model.Open, HasCheckbox: true, Tags: []model.Tag{{Name: "today"}}, File: "notes.md", Line: 2})
	client := &recordingClient{week: now, addErr: map[string]error{"First": context.DeadlineExceeded}}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{first, second}, Client: client, Query: "@due or @today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if rep.Failed != 1 || rep.Pushed != 1 {
		t.Errorf("Failed=%d Pushed=%d, want 1/1", rep.Failed, rep.Pushed)
	}
	if len(warnings) != 1 {
		t.Errorf("want 1 warning for the failed task, got %d: %v", len(warnings), warnings)
	}
	// The second task was still pushed and linked.
	if got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md")); !contains(string(got), "Second @today @hey(") {
		t.Errorf("second task not linked:\n%s", string(got))
	}
}

func TestPush_DryRunWritesNothing(t *testing.T) {
	now := time.Now()
	task, notesDir, statePath := pushFixture(t, "Buy milk @today", model.Tag{Name: "today"})
	notesBefore, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	client := &dryFakeClient{t: t}

	rep, _, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@due or @today",
		StatePath: statePath, NotesDir: notesDir, Now: now, DryRun: true,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if rep.WouldPush != 1 || rep.Pushed != 0 {
		t.Errorf("WouldPush=%d Pushed=%d, want 1/0", rep.WouldPush, rep.Pushed)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Errorf("state file written during dry run (stat err: %v)", err)
	}
	notesAfter, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if string(notesAfter) != string(notesBefore) {
		t.Error("notes file modified during dry run")
	}
}

// dryFakeClient fails the test if any mutating verb runs, asserting a dry run
// touches neither the notes nor HEY.
type dryFakeClient struct {
	t     *testing.T
	todos []hey.Todo
}

func (c *dryFakeClient) List(context.Context) ([]hey.Todo, error) { return c.todos, nil }
func (c *dryFakeClient) Add(context.Context, string, *time.Time) (hey.Todo, error) {
	c.t.Fatal("Add called during a dry run")
	return hey.Todo{}, nil
}
func (c *dryFakeClient) Complete(context.Context, string) error {
	c.t.Fatal("Complete in dry run")
	return nil
}
func (c *dryFakeClient) Uncomplete(context.Context, string) error {
	c.t.Fatal("Uncomplete in dry run")
	return nil
}
func (c *dryFakeClient) Delete(context.Context, string) error {
	c.t.Fatal("Delete in dry run")
	return nil
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
