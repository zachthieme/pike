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
	"github.com/zachthieme/pike/internal/parser"
)

func TestImport_SecondSyncImportsNothing(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	statePath := filepath.Join(notesDir, "hey-state.json")
	weekEnd := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)

	client := &recordingClient{week: now, todos: []hey.Todo{weekTodo("h1", "Buy stamps", weekEnd)}}
	opts := Options{Client: client, Query: "@due or @today", StatePath: statePath, NotesDir: notesDir, Now: now}

	if _, _, err := Push(context.Background(), opts); err != nil {
		t.Fatalf("first Push: %v", err)
	}

	// Re-scan: the imported line now parses as a Task carrying its @hey Link.
	line := "- [ ] Buy stamps @due(2026-09-19) @hey(h1)"
	task, _ := parser.ParseLine(line, "inbox.md", 1)
	if task == nil {
		t.Fatalf("imported line did not parse: %q", line)
	}
	inboxBefore, _ := os.ReadFile(filepath.Join(notesDir, "inbox.md"))

	opts.Tasks = []model.Task{*task}
	rep, warnings, err := Push(context.Background(), opts)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("second Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Imported != 0 {
		t.Errorf("Imported = %d on second sync, want 0", rep.Imported)
	}
	if rep.ExistingLinks != 1 {
		t.Errorf("ExistingLinks = %d, want 1", rep.ExistingLinks)
	}
	inboxAfter, _ := os.ReadFile(filepath.Join(notesDir, "inbox.md"))
	if string(inboxAfter) != string(inboxBefore) {
		t.Errorf("inbox rewritten on second sync\n got: %q\nwant: %q", string(inboxAfter), string(inboxBefore))
	}
}

func TestImport_UsesConfiguredInboxFile(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	weekEnd := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	client := &recordingClient{week: now, todos: []hey.Todo{weekTodo("h1", "Buy stamps", weekEnd)}}

	if _, _, err := Push(context.Background(), Options{
		Client: client, Query: "@due or @today",
		StatePath: filepath.Join(notesDir, "hey-state.json"),
		NotesDir:  notesDir, InboxFile: "capture.md", Now: now,
	}); err != nil {
		t.Fatalf("Push: %v", err)
	}

	if _, err := os.Stat(filepath.Join(notesDir, "inbox.md")); !os.IsNotExist(err) {
		t.Errorf("default inbox.md should not be written when inbox_file is set (err: %v)", err)
	}
	got, err := os.ReadFile(filepath.Join(notesDir, "capture.md"))
	if err != nil {
		t.Fatalf("reading capture.md: %v", err)
	}
	if want := "- [ ] Buy stamps @due(2026-09-19) @hey(h1)\n"; string(got) != want {
		t.Errorf("capture.md\n got: %q\nwant: %q", string(got), want)
	}
}

func TestImport_DryRunReportsButWritesNothing(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	statePath := filepath.Join(notesDir, "hey-state.json")
	weekEnd := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	// dryFakeClient fails the test if any mutating HEY verb runs.
	client := &dryFakeClient{t: t, todos: []hey.Todo{weekTodo("h1", "Buy stamps", weekEnd)}}

	rep, warnings, err := Push(context.Background(), Options{
		Client: client, Query: "@due or @today", StatePath: statePath,
		NotesDir: notesDir, Now: now, DryRun: true,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.WouldImport != 1 || rep.Imported != 0 {
		t.Errorf("WouldImport=%d Imported=%d, want 1/0", rep.WouldImport, rep.Imported)
	}
	if _, err := os.Stat(filepath.Join(notesDir, "inbox.md")); !os.IsNotExist(err) {
		t.Errorf("inbox written during dry run (stat err: %v)", err)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Errorf("state written during dry run (stat err: %v)", err)
	}
}

// weekTodo returns an open Todo whose Week ends on the given Saturday.
func weekTodo(id, title string, weekEnd time.Time) hey.Todo {
	return hey.Todo{
		ID:        id,
		Title:     title,
		WeekStart: weekEnd.AddDate(0, 0, -6),
		WeekEnd:   weekEnd,
		Updated:   weekEnd,
	}
}

func TestImport_TitleWithAtSigns_RoundTripsWithoutRecreate(t *testing.T) {
	// A HEY Title carrying @ tokens must be imported so the new line parses with
	// no tags other than the @due and @hey pike appends, and the Title pike then
	// computes from that line equals the HEY Title — so the next Sync sees no
	// Title change and never Re-creates the Todo.
	tests := []struct {
		name  string
		title string
	}{
		{"address and tag-like word", "Email bob@example.com re @rent"},
		{"embedded due token", "Pay rent @due(2026-01-01)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			notesDir := t.TempDir()
			statePath := filepath.Join(notesDir, "hey-state.json")
			weekEnd := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
			client := &recordingClient{week: now, todos: []hey.Todo{weekTodo("h1", tt.title, weekEnd)}}
			opts := Options{Client: client, Query: "@due or @today", StatePath: statePath, NotesDir: notesDir, Now: now}

			if _, _, err := Push(context.Background(), opts); err != nil {
				t.Fatalf("first Push: %v", err)
			}

			raw, err := os.ReadFile(filepath.Join(notesDir, "inbox.md"))
			if err != nil {
				t.Fatalf("reading inbox: %v", err)
			}
			line := strings.TrimSuffix(string(raw), "\n")
			task, _ := parser.ParseLine(line, "inbox.md", 1)
			if task == nil {
				t.Fatalf("imported line did not parse: %q", line)
			}

			// The only tags on the imported line are the ones pike appended.
			var names []string
			for _, tag := range task.Tags {
				names = append(names, tag.Name)
			}
			if len(names) != 2 || names[0] != "due" || names[1] != "hey" {
				t.Errorf("imported line has tags %v, want only [due hey]; line=%q", names, line)
			}

			// The Title pike computes from the line equals the HEY Title.
			if got := titleOf(task.Text); got != tt.title {
				t.Errorf("titleOf(imported line) = %q, want HEY title %q", got, tt.title)
			}

			// A second Sync sees no Title change: no add and no delete.
			addsBefore := len(client.added)
			mutationsBefore := len(client.mutations)
			opts.Tasks = []model.Task{*task}
			rep, warnings, err := Push(context.Background(), opts)
			if err != nil || len(warnings) != 0 {
				t.Fatalf("second Push: err=%v warnings=%v", err, warnings)
			}
			if rep.Recreated != 0 || rep.Retitled != 0 {
				t.Errorf("second Sync Recreated=%d Retitled=%d, want 0/0", rep.Recreated, rep.Retitled)
			}
			if len(client.added) != addsBefore {
				t.Errorf("second Sync made %d Add call(s), want 0", len(client.added)-addsBefore)
			}
			if got := client.mutations[mutationsBefore:]; len(got) != 0 {
				t.Errorf("second Sync made HEY mutations, want none: %v", got)
			}
		})
	}
}

func TestImport_AppendsUnlinkedOpenTodosInListOrder(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	statePath := filepath.Join(notesDir, "hey-state.json")
	// A Week running Sunday 13th to Saturday 19th.
	weekEnd := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	completed := now

	client := &recordingClient{
		week: now,
		todos: []hey.Todo{
			weekTodo("h2", "Walk the dog", weekEnd),
			weekTodo("h1", "Buy stamps", weekEnd),
			{ID: "h3", Title: "Already done", WeekStart: weekEnd, WeekEnd: weekEnd, Completed: &completed}, // completed → skip
		},
	}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks:     nil,
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
	if rep.Imported != 2 {
		t.Errorf("Imported = %d, want 2", rep.Imported)
	}

	// Lines land in HEY's list order, one per line, in the default inbox.
	got, err := os.ReadFile(filepath.Join(notesDir, "inbox.md"))
	if err != nil {
		t.Fatalf("reading inbox: %v", err)
	}
	want := "- [ ] Walk the dog @due(2026-09-19) @hey(h2)\n" +
		"- [ ] Buy stamps @due(2026-09-19) @hey(h1)\n"
	if string(got) != want {
		t.Errorf("inbox\n got: %q\nwant: %q", string(got), want)
	}

	// Each imported Todo gets a state entry keyed by its HEY id.
	st, err := LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Links["h2"]; !ok {
		t.Errorf("no state link for h2; state = %+v", st)
	}
	if _, ok := st.Links["h1"]; !ok {
		t.Errorf("no state link for h1; state = %+v", st)
	}
	if _, ok := st.Links["h3"]; ok {
		t.Errorf("completed todo h3 should not be linked")
	}

	// The imported lines parse back as Linked checkbox Tasks.
	for _, line := range []string{
		"- [ ] Walk the dog @due(2026-09-19) @hey(h2)",
		"- [ ] Buy stamps @due(2026-09-19) @hey(h1)",
	} {
		task, _ := parser.ParseLine(line, "inbox.md", 1)
		if task == nil {
			t.Fatalf("imported line did not parse as a task: %q", line)
		}
		if !task.HasCheckbox || task.State != model.Open {
			t.Errorf("imported task not an open checkbox: %+v", task)
		}
		if id, linked := linkID(task); !linked || (id != "h2" && id != "h1") {
			t.Errorf("imported task not Linked to a hey id: %+v", task)
		}
	}
}
