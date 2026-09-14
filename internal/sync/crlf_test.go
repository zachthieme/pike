package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zachthieme/pike/internal/hey"
	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/scanner"
)

// scanNotes runs the real scanner over dir and returns the Tasks it finds. The
// scanner reads lines with bufio.Scanner, so each Task's Raw has its trailing
// "\r" dropped — the exact input a Sync sees for a CRLF notes file.
func scanNotes(t *testing.T, dir string) []model.Task {
	t.Helper()
	sc, err := scanner.New(dir, []string{"**/*.md"}, nil)
	if err != nil {
		t.Fatalf("scanner.New: %v", err)
	}
	tasks, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return tasks
}

// TestSync_CRLFNotesPushLinkAndNoOp drives a full Push over CRLF notes read
// through the real scanner. The first Sync pushes the eligible Task and links it
// with one Add and no Delete, no Warnings and no failures; the notes keep their
// CRLF endings; a second Sync over the now-linked notes touches HEY only to list.
func TestSync_CRLFNotesPushLinkAndNoOp(t *testing.T) {
	now := time.Now()
	dir := t.TempDir()
	notes := filepath.Join(dir, "notes.md")
	original := "# Notes\r\n- [ ] Buy milk @today\r\n- [ ] Someday reading\r\n"
	if err := os.WriteFile(notes, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "hey-state.json")
	client := &recordingClient{week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: scanNotes(t, dir), Client: client, Query: "@due or @today",
		StatePath: statePath, NotesDir: dir, Now: now,
	})
	if err != nil {
		t.Fatalf("first Push: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if rep.Pushed != 1 || rep.Failed != 0 {
		t.Errorf("Pushed=%d Failed=%d, want 1/0", rep.Pushed, rep.Failed)
	}
	if len(client.added) != 1 {
		t.Errorf("Add called %d times, want 1", len(client.added))
	}
	for _, m := range client.mutations {
		if m == "delete:h_new1" {
			t.Errorf("no Delete expected on a clean push; mutations=%v", client.mutations)
		}
	}

	// The linked line kept its CRLF ending, and so did every other line.
	want := "# Notes\r\n- [ ] Buy milk @today @hey(h_new1)\r\n- [ ] Someday reading\r\n"
	if got, _ := os.ReadFile(notes); string(got) != want {
		t.Errorf("notes after push\n got: %q\nwant: %q", string(got), want)
	}

	// Second Sync over the now-linked notes: nothing to push, HEY touched only to
	// list, and the file endings are still CRLF.
	addsBefore := len(client.added)
	mutationsBefore := len(client.mutations)
	listsBefore := client.listed

	rep2, warnings2, err := Push(context.Background(), Options{
		Tasks: scanNotes(t, dir), Client: client, Query: "@due or @today",
		StatePath: statePath, NotesDir: dir, Now: now,
	})
	if err != nil {
		t.Fatalf("second Push: %v", err)
	}
	if len(warnings2) != 0 {
		t.Errorf("unexpected warnings on second sync: %v", warnings2)
	}
	if rep2.Pushed != 0 || rep2.ExistingLinks != 1 {
		t.Errorf("Pushed=%d ExistingLinks=%d, want 0/1", rep2.Pushed, rep2.ExistingLinks)
	}
	if len(client.added) != addsBefore {
		t.Errorf("second sync made %d extra Add call(s), want 0", len(client.added)-addsBefore)
	}
	if got := client.mutations[mutationsBefore:]; len(got) != 0 {
		t.Errorf("second sync made mutating HEY calls beyond list: %v", got)
	}
	if client.listed != listsBefore+1 {
		t.Errorf("second sync made %d List calls, want exactly 1", client.listed-listsBefore)
	}
	if got, _ := os.ReadFile(notes); string(got) != want {
		t.Errorf("notes changed on no-op sync\n got: %q\nwant: %q", string(got), want)
	}
}

// TestSync_CRLFHeySideCompletionKeepsCRLF drives a HEY-side completion over CRLF
// notes read through the real scanner: the Todo completed in HEY since the last
// Sync, so pike checks the Task's line, and the "@completed" stamp lands before
// the line's "\r\n" ending, which is left intact.
func TestSync_CRLFHeySideCompletionKeepsCRLF(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	completedAt := time.Date(2026, 9, 13, 8, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	notes := filepath.Join(dir, "notes.md")
	original := "# Notes\r\n- [ ] Read book @hey(h1)\r\n"
	if err := os.WriteFile(notes, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "hey-state.json")
	if err := SaveState(statePath, &State{Links: map[string]Link{"h1": {Title: "Read book", Completed: false, File: "notes.md", Line: 2}}}); err != nil {
		t.Fatal(err)
	}
	client := &completionClient{todos: []hey.Todo{{ID: "h1", Title: "Read book", WeekStart: now, WeekEnd: now, Completed: &completedAt}}}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: scanNotes(t, dir), Client: client, Query: "@today",
		StatePath: statePath, NotesDir: dir, Now: now,
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
	want := "# Notes\r\n- [x] Read book @hey(h1) @completed(2026-09-13)\r\n"
	if got, _ := os.ReadFile(notes); string(got) != want {
		t.Errorf("notes after HEY-side completion\n got: %q\nwant: %q", string(got), want)
	}
}
