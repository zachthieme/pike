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

// day is a UTC calendar date at midnight, matching how the parser and the hey
// client materialise @due days and Week boundaries.
func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestResolveWeek_DecisionMatrix(t *testing.T) {
	// Two adjacent Weeks: W0 runs Sun Sep 13 .. Sat Sep 19; W1 runs Sun Sep 20
	// .. Sat Sep 26. W2 begins Sun Sep 27.
	w0start, w0end := day(2026, 9, 13), day(2026, 9, 19)
	w1start, w1end := day(2026, 9, 20), day(2026, 9, 26)
	wed := day(2026, 9, 16) // a Wednesday inside W0
	inW1 := day(2026, 9, 23)
	inW2 := day(2026, 9, 30)

	ptr := func(d time.Time) *time.Time { return &d }

	tests := []struct {
		name                   string
		due                    *time.Time
		todoStart, todoEnd     time.Time
		baseStart              time.Time
		hasBase, todoCompleted bool
		want                   weekAction
	}{
		{"no due, week ignored", nil, w0start, w0end, w0start, true, false, weekNone},
		{"due inside todo week", ptr(wed), w0start, w0end, w0start, true, false, weekNone},
		{"due on week's saturday", ptr(w0end), w0start, w0end, w0start, true, false, weekNone},
		{"due on week's sunday", ptr(w0start), w0start, w0end, w0start, true, false, weekNone},
		// HEY moved the Todo's Week forward; the notes @due (still in W0) is stale.
		{"hey moved week, due outside", ptr(wed), w1start, w1end, w0start, true, false, weekSetDue},
		// The notes moved @due into another Week; HEY still holds W0.
		{"notes moved due to another week", ptr(inW1), w0start, w0end, w0start, true, false, weekRecreate},
		// Both sides moved and disagree; the notes win.
		{"both moved differently, notes win", ptr(inW2), w1start, w1end, w0start, true, false, weekRecreate},
		// Without a base we cannot tell which side moved, so do nothing.
		{"no base, sides disagree", ptr(inW1), w0start, w0end, time.Time{}, false, false, weekNone},
		// A base that never recorded a Week is treated the same as having none.
		{"base without recorded week", ptr(inW1), w0start, w0end, time.Time{}, true, false, weekNone},
		// A completed Todo is never rescheduled or re-created.
		{"completed todo", ptr(inW1), w0start, w0end, w0start, true, true, weekNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveWeek(tt.due, tt.todoStart, tt.todoEnd, tt.baseStart, tt.hasBase, tt.todoCompleted)
			if got != tt.want {
				t.Errorf("resolveWeek() = %v, want %v", got, tt.want)
			}
		})
	}
}

// weekLinkTask builds a linked, open, checkbox Task carrying an @hey tag and an
// @due day, matching the raw line a test writes to disk.
func weekLinkTask(id string, due time.Time) model.Task {
	dueStr := due.Format("2006-01-02")
	text := "Ship it @hey(" + id + ") @due(" + dueStr + ")"
	return model.Task{Text: text, State: model.Open, HasCheckbox: true, Due: &due,
		Tags: []model.Tag{{Name: "hey", Value: id}, {Name: "due", Value: dueStr}}}
}

func weekLine(id string, due time.Time) string {
	return "- [ ] Ship it @hey(" + id + ") @due(" + due.Format("2006-01-02") + ")"
}

func TestSyncWeek_HeyMovedWeek_SetsDueToNewSaturday(t *testing.T) {
	now := time.Now()
	oldDue := day(2026, 9, 16) // Wed in W0
	task, notesDir, statePath := linkFixture(t,
		"h1", weekLine("h1", oldDue), weekLinkTask("h1", oldDue),
		&State{Links: map[string]Link{"h1": {Title: "Ship it", WeekStart: day(2026, 9, 13), File: "notes.md", Line: 1}}},
	)
	// HEY moved the Todo to W1 (Sun Sep 20 .. Sat Sep 26). The notes @due no
	// longer falls inside its Week, so @due is set to the new Week's Saturday.
	client := &titleClient{todos: []hey.Todo{
		{ID: "h1", Title: "Ship it", WeekStart: day(2026, 9, 20), WeekEnd: day(2026, 9, 26)},
	}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Rescheduled != 1 {
		t.Errorf("Rescheduled=%d, want 1", rep.Rescheduled)
	}
	if len(client.calls) != 0 {
		t.Errorf("no HEY mutation expected for a set-due; calls=%v", client.calls)
	}
	got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	want := "- [ ] Ship it @hey(h1) @due(2026-09-26)\n"
	if string(got) != want {
		t.Errorf("notes line\n got: %q\nwant: %q", string(got), want)
	}
	st, _ := LoadState(statePath)
	if !st.Links["h1"].WeekStart.Equal(day(2026, 9, 20)) {
		t.Errorf("state WeekStart=%v, want %v", st.Links["h1"].WeekStart, day(2026, 9, 20))
	}
}

func TestSyncWeek_DueInsideWeek_LineUntouched(t *testing.T) {
	now := time.Now()
	due := day(2026, 9, 16) // Wed inside W0
	task, notesDir, statePath := linkFixture(t,
		"h1", weekLine("h1", due), weekLinkTask("h1", due),
		&State{Links: map[string]Link{"h1": {Title: "Ship it", WeekStart: day(2026, 9, 13), File: "notes.md", Line: 1}}},
	)
	before, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	// HEY's Week is unchanged and still contains the Wednesday deadline.
	client := &titleClient{todos: []hey.Todo{
		{ID: "h1", Title: "Ship it", WeekStart: day(2026, 9, 13), WeekEnd: day(2026, 9, 19)},
	}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Rescheduled != 0 {
		t.Errorf("Rescheduled=%d, want 0", rep.Rescheduled)
	}
	after, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if string(after) != string(before) {
		t.Errorf("Wednesday deadline flattened:\n got: %q\nwant: %q", string(after), string(before))
	}
}

func TestSyncWeek_NotesMovedDue_RecreatesTodoWithNewDate(t *testing.T) {
	now := time.Now()
	newDue := day(2026, 9, 23) // moved into W1
	task, notesDir, statePath := linkFixture(t,
		"h1", weekLine("h1", newDue), weekLinkTask("h1", newDue),
		&State{Links: map[string]Link{"h1": {Title: "Ship it", WeekStart: day(2026, 9, 13), File: "notes.md", Line: 1}}},
	)
	// HEY still holds the old Week (W0); the notes moved @due into W1.
	client := &titleClient{todos: []hey.Todo{
		{ID: "h1", Title: "Ship it", WeekStart: day(2026, 9, 13), WeekEnd: day(2026, 9, 19)},
	}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Rescheduled != 1 {
		t.Errorf("Rescheduled=%d, want 1", rep.Rescheduled)
	}
	// Exactly one Re-create: add the fresh Todo, then delete the old one.
	want := []string{"add:Ship it", "delete:h1"}
	if len(client.calls) != 2 || client.calls[0] != want[0] || client.calls[1] != want[1] {
		t.Errorf("calls=%v, want %v", client.calls, want)
	}
	// The Add carried the new @due day so HEY files the Todo under the new Week.
	if !client.lastAddDate.Equal(newDue) {
		t.Errorf("Add date=%v, want %v", client.lastAddDate, newDue)
	}
	st, _ := LoadState(statePath)
	if _, stillOld := st.Links["h1"]; stillOld {
		t.Errorf("old link h1 should be gone; state=%+v", st.Links)
	}
}

func TestSyncWeek_NotesTitleAndDueBothChanged_SingleRecreate(t *testing.T) {
	now := time.Now()
	newDue := day(2026, 9, 23) // moved into W1
	dueStr := newDue.Format("2006-01-02")
	line := "- [ ] Renamed here @hey(h1) @due(" + dueStr + ")"
	task, notesDir, statePath := linkFixture(t,
		"h1", line,
		model.Task{Text: "Renamed here @hey(h1) @due(" + dueStr + ")", State: model.Open, HasCheckbox: true, Due: &newDue,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}, {Name: "due", Value: dueStr}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", WeekStart: day(2026, 9, 13), File: "notes.md", Line: 1}}},
	)
	// Both the title and the @due moved in the notes; HEY holds the old Title and
	// the old Week. This must collapse to a single Re-create (one add, one delete).
	client := &titleClient{todos: []hey.Todo{
		{ID: "h1", Title: "Ship it", WeekStart: day(2026, 9, 13), WeekEnd: day(2026, 9, 19)},
	}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	adds, deletes := 0, 0
	for _, c := range client.calls {
		switch {
		case len(c) >= 4 && c[:4] == "add:":
			adds++
		case len(c) >= 7 && c[:7] == "delete:":
			deletes++
		}
	}
	if adds != 1 || deletes != 1 {
		t.Errorf("adds=%d deletes=%d, want 1/1; calls=%v", adds, deletes, client.calls)
	}
	if !client.lastAddDate.Equal(newDue) {
		t.Errorf("Add date=%v, want %v", client.lastAddDate, newDue)
	}
	// The single Re-create carried the new title.
	if client.calls[0] != "add:Renamed here" {
		t.Errorf("expected the re-create to use the new title; calls=%v", client.calls)
	}
	if rep.Recreated != 1 {
		t.Errorf("Recreated=%d, want 1", rep.Recreated)
	}
}

func TestSyncWeek_HeyRenamedAndMovedWeek_RetitlesAndSetsDue(t *testing.T) {
	now := time.Now()
	oldDue := day(2026, 9, 16) // Wed in W0
	task, notesDir, statePath := linkFixture(t,
		"h1", weekLine("h1", oldDue), weekLinkTask("h1", oldDue),
		&State{Links: map[string]Link{"h1": {Title: "Ship it", WeekStart: day(2026, 9, 13), File: "notes.md", Line: 1}}},
	)
	// HEY both renamed the Todo and moved it to W1. The rename retitles the Task's
	// text; the Week move sets @due to the new Saturday. Neither is a notes-side
	// change, so no Re-create fires.
	client := &titleClient{todos: []hey.Todo{
		{ID: "h1", Title: "Deploy it now", WeekStart: day(2026, 9, 20), WeekEnd: day(2026, 9, 26)},
	}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Retitled != 1 || rep.Rescheduled != 1 {
		t.Errorf("Retitled=%d Rescheduled=%d, want 1/1", rep.Retitled, rep.Rescheduled)
	}
	if len(client.calls) != 0 {
		t.Errorf("no Re-create expected for two HEY-side changes; calls=%v", client.calls)
	}
	got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	want := "- [ ] Deploy it now @hey(h1) @due(2026-09-26)\n"
	if string(got) != want {
		t.Errorf("notes line\n got: %q\nwant: %q", string(got), want)
	}
}

func TestSyncWeek_HeyRenamedAndNotesMovedWeek_RecreateCarriesHeyTitle(t *testing.T) {
	now := time.Now()
	newDue := day(2026, 9, 23) // moved into W1 in the notes
	dueStr := newDue.Format("2006-01-02")
	// The notes still carry the old Title "Ship it" but moved @due into W1; HEY
	// renamed the Todo to "Deploy it now" and left it in W0. The Title pass
	// retitles the line to HEY's Title, then the Week pass Re-creates for the
	// moved @due — and that Re-create must carry HEY's Title, not the pre-retitle
	// one, or HEY, notes and state disagree and the next Sync churns.
	task, notesDir, statePath := linkFixture(t,
		"h1", weekLine("h1", newDue), weekLinkTask("h1", newDue),
		&State{Links: map[string]Link{"h1": {Title: "Ship it", WeekStart: day(2026, 9, 13), File: "notes.md", Line: 1}}},
	)
	client := &titleClient{todos: []hey.Todo{
		{ID: "h1", Title: "Deploy it now", WeekStart: day(2026, 9, 13), WeekEnd: day(2026, 9, 19)},
	}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Retitled != 1 || rep.Rescheduled != 1 {
		t.Errorf("Retitled=%d Rescheduled=%d, want 1/1", rep.Retitled, rep.Rescheduled)
	}
	// One Re-create carrying HEY's Title and the new date, then a delete.
	want := []string{"add:Deploy it now", "delete:h1"}
	if len(client.calls) != 2 || client.calls[0] != want[0] || client.calls[1] != want[1] {
		t.Errorf("calls=%v, want %v", client.calls, want)
	}
	if !client.lastAddDate.Equal(newDue) {
		t.Errorf("Add date=%v, want %v", client.lastAddDate, newDue)
	}
	// The notes carry HEY's Title and point at the new Todo.
	got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	wantLine := "- [ ] Deploy it now @hey(h_new1) @due(" + dueStr + ")\n"
	if string(got) != wantLine {
		t.Errorf("notes line\n got: %q\nwant: %q", string(got), wantLine)
	}
	// State holds HEY's Title under the new id; the old link is gone.
	st, _ := LoadState(statePath)
	if _, stillOld := st.Links["h1"]; stillOld {
		t.Errorf("old link h1 should be gone; state=%+v", st.Links)
	}
	if st.Links["h_new1"].Title != "Deploy it now" {
		t.Errorf("state Title=%q, want %q", st.Links["h_new1"].Title, "Deploy it now")
	}

	// A second Sync, re-scanning the retitled+relinked line against a HEY list
	// that now agrees, must make no HEY calls beyond the list.
	relinked := model.TaskWith(model.Task{
		Text: "Deploy it now @hey(h_new1) @due(" + dueStr + ")", Raw: wantLine[:len(wantLine)-1],
		State: model.Open, HasCheckbox: true, Due: &newDue,
		Tags: []model.Tag{{Name: "hey", Value: "h_new1"}, {Name: "due", Value: dueStr}},
		File: "notes.md", Line: 1,
	})
	client2 := &titleClient{todos: []hey.Todo{
		{ID: "h_new1", Title: "Deploy it now", WeekStart: day(2026, 9, 20), WeekEnd: day(2026, 9, 26)},
	}, week: now}
	if _, w2, err := Push(context.Background(), Options{
		Tasks: []model.Task{relinked}, Client: client2, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	}); err != nil || len(w2) != 0 {
		t.Fatalf("second Push: err=%v warnings=%v", err, w2)
	}
	if len(client2.calls) != 0 {
		t.Errorf("second Sync made HEY calls beyond list: %v", client2.calls)
	}
}

func TestSyncWeek_HeyRenamedWithAtSignsAndNotesMovedWeek_RecreateSendsDecodedTitle(t *testing.T) {
	now := time.Now()
	newDue := day(2026, 9, 23) // moved into W1 in the notes
	heyTitle := "Email bob@example.com re @rent"
	// The notes still carry "Ship it" but moved @due into W1; HEY renamed the Todo
	// to a Title carrying @ tokens and left it in W0. The Title pass retitles the
	// line (encoded, so no stray tag), then the Week pass Re-creates for the moved
	// @due — and that Re-create must send HEY's Title decoded, not the encoded form.
	task, notesDir, statePath := linkFixture(t,
		"h1", weekLine("h1", newDue), weekLinkTask("h1", newDue),
		&State{Links: map[string]Link{"h1": {Title: "Ship it", WeekStart: day(2026, 9, 13), File: "notes.md", Line: 1}}},
	)
	client := &titleClient{todos: []hey.Todo{
		{ID: "h1", Title: heyTitle, WeekStart: day(2026, 9, 13), WeekEnd: day(2026, 9, 19)},
	}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Retitled != 1 || rep.Rescheduled != 1 {
		t.Errorf("Retitled=%d Rescheduled=%d, want 1/1", rep.Retitled, rep.Rescheduled)
	}
	// The Re-create sends HEY's decoded Title verbatim, then a delete.
	want := []string{"add:" + heyTitle, "delete:h1"}
	if len(client.calls) != 2 || client.calls[0] != want[0] || client.calls[1] != want[1] {
		t.Errorf("calls=%v, want %v", client.calls, want)
	}
	if !client.lastAddDate.Equal(newDue) {
		t.Errorf("Add date=%v, want %v", client.lastAddDate, newDue)
	}
	// The relinked line parses with only [hey due] and its Title decodes to HEY's.
	got, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	line := strings.TrimSuffix(string(got), "\n")
	relinked, _ := parser.ParseLine(line, "notes.md", 1)
	if relinked == nil {
		t.Fatalf("relinked line did not parse: %q", line)
	}
	var names []string
	for _, tag := range relinked.Tags {
		names = append(names, tag.Name)
	}
	if len(names) != 2 || names[0] != "hey" || names[1] != "due" {
		t.Fatalf("relinked line tags %v, want [hey due]; line=%q", names, line)
	}
	if relinked.Tags[0].Value != "h_new1" {
		t.Errorf("@hey value=%q, want h_new1; line=%q", relinked.Tags[0].Value, line)
	}
	if got := titleOf(relinked.Text); got != heyTitle {
		t.Errorf("titleOf(relinked line) = %q, want %q", got, heyTitle)
	}
	st, _ := LoadState(statePath)
	if st.Links["h_new1"].Title != heyTitle {
		t.Errorf("state Title=%q, want %q", st.Links["h_new1"].Title, heyTitle)
	}

	// A second Sync, re-scanning the relinked line against a HEY list that now
	// agrees, must make no HEY calls beyond the list.
	client2 := &titleClient{todos: []hey.Todo{
		{ID: "h_new1", Title: heyTitle, WeekStart: day(2026, 9, 20), WeekEnd: day(2026, 9, 26)},
	}, week: now}
	if _, w2, err := Push(context.Background(), Options{
		Tasks: []model.Task{*relinked}, Client: client2, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	}); err != nil || len(w2) != 0 {
		t.Fatalf("second Push: err=%v warnings=%v", err, w2)
	}
	if len(client2.calls) != 0 {
		t.Errorf("second Sync made HEY calls beyond list: %v", client2.calls)
	}
}

func TestSyncWeek_TaskWithoutDue_WeekIgnored(t *testing.T) {
	now := time.Now()
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [ ] Ship it @hey(h1)",
		model.Task{Text: "Ship it @hey(h1)", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", WeekStart: day(2026, 9, 13), File: "notes.md", Line: 1}}},
	)
	before, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	// HEY moved the Week, but the Task has no @due so its Week is never compared.
	client := &titleClient{todos: []hey.Todo{
		{ID: "h1", Title: "Ship it", WeekStart: day(2026, 9, 20), WeekEnd: day(2026, 9, 26)},
	}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Rescheduled != 0 {
		t.Errorf("Rescheduled=%d, want 0", rep.Rescheduled)
	}
	if len(client.calls) != 0 {
		t.Errorf("no HEY mutation expected; calls=%v", client.calls)
	}
	after, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if string(after) != string(before) {
		t.Errorf("notes changed for a due-less Task:\n got: %q\nwant: %q", string(after), string(before))
	}
}

func TestSyncWeek_CompletedTodo_NothingWritten(t *testing.T) {
	now := time.Now()
	completedAt := now
	completed := now
	due := day(2026, 9, 16)
	dueStr := due.Format("2006-01-02")
	// The Todo is completed on both sides; HEY moved the Week. A completed Todo is
	// never rescheduled, and completion agrees, so nothing is written anywhere.
	task, notesDir, statePath := linkFixture(t,
		"h1", "- [x] Ship it @hey(h1) @due("+dueStr+") @completed(2026-09-10)",
		model.Task{Text: "Ship it @hey(h1) @due(" + dueStr + ") @completed(2026-09-10)", State: model.Completed, HasCheckbox: true,
			Due: &due, Completed: &completed,
			Tags: []model.Tag{{Name: "hey", Value: "h1"}, {Name: "due", Value: dueStr}, {Name: "completed", Value: "2026-09-10"}}},
		&State{Links: map[string]Link{"h1": {Title: "Ship it", WeekStart: day(2026, 9, 13), Completed: true, File: "notes.md", Line: 1}}},
	)
	before, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	client := &titleClient{todos: []hey.Todo{
		{ID: "h1", Title: "Ship it", WeekStart: day(2026, 9, 20), WeekEnd: day(2026, 9, 26), Completed: &completedAt},
	}, week: now}

	rep, warnings, err := Push(context.Background(), Options{
		Tasks: []model.Task{task}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: notesDir, Now: now,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Rescheduled != 0 {
		t.Errorf("Rescheduled=%d, want 0", rep.Rescheduled)
	}
	if len(client.calls) != 0 {
		t.Errorf("no HEY mutation expected on a completed Todo; calls=%v", client.calls)
	}
	after, _ := os.ReadFile(filepath.Join(notesDir, "notes.md"))
	if string(after) != string(before) {
		t.Errorf("notes changed:\n got: %q\nwant: %q", string(after), string(before))
	}
}

func TestPlanWeek_DryRunCountsAndWritesNothing(t *testing.T) {
	now := time.Now()
	// One HEY-side reschedule (would set @due) and one notes-side reschedule
	// (would re-create). A dry run must count both and touch nothing.
	heyMoved := model.TaskWith(model.Task{Text: "Ship it @hey(h1) @due(2026-09-16)", State: model.Open, HasCheckbox: true,
		Due: ptrDay(2026, 9, 16), Tags: []model.Tag{{Name: "hey", Value: "h1"}, {Name: "due", Value: "2026-09-16"}}, File: "notes.md", Line: 1})
	notesMoved := model.TaskWith(model.Task{Text: "Deploy it @hey(h2) @due(2026-09-23)", State: model.Open, HasCheckbox: true,
		Due: ptrDay(2026, 9, 23), Tags: []model.Tag{{Name: "hey", Value: "h2"}, {Name: "due", Value: "2026-09-23"}}, File: "notes.md", Line: 2})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"),
		[]byte("- [ ] Ship it @hey(h1) @due(2026-09-16)\n- [ ] Deploy it @hey(h2) @due(2026-09-23)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "hey-state.json")
	if err := SaveState(statePath, &State{Links: map[string]Link{
		"h1": {Title: "Ship it", WeekStart: day(2026, 9, 13), File: "notes.md", Line: 1},
		"h2": {Title: "Deploy it", WeekStart: day(2026, 9, 13), File: "notes.md", Line: 2},
	}}); err != nil {
		t.Fatal(err)
	}
	notesBefore, _ := os.ReadFile(filepath.Join(dir, "notes.md"))
	stateBefore, _ := os.ReadFile(statePath)
	client := &dryFakeClient{t: t, todos: []hey.Todo{
		{ID: "h1", Title: "Ship it", WeekStart: day(2026, 9, 20), WeekEnd: day(2026, 9, 26)},   // hey moved → would set @due
		{ID: "h2", Title: "Deploy it", WeekStart: day(2026, 9, 13), WeekEnd: day(2026, 9, 19)}, // notes moved → would re-create
	}}

	rep, warnings, err := Plan(context.Background(), Options{
		Tasks: []model.Task{heyMoved, notesMoved}, Client: client, Query: "@today",
		StatePath: statePath, NotesDir: dir, Now: now, DryRun: true,
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Plan: err=%v warnings=%v", err, warnings)
	}
	if rep.WouldReschedule != 2 {
		t.Errorf("WouldReschedule=%d, want 2", rep.WouldReschedule)
	}
	if rep.Rescheduled != 0 {
		t.Errorf("real count should be zero on a dry run: Rescheduled=%d", rep.Rescheduled)
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

func ptrDay(y int, m time.Month, d int) *time.Time {
	t := day(y, m, d)
	return &t
}
