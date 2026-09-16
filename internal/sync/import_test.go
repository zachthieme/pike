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

func TestEncodeDecodeTitle_RoundTrip(t *testing.T) {
	// decodeTitle is the exact inverse of encodeTitle: decode(encode(title)) == title,
	// including a Title holding both a literal U+200B (which encode never touches and
	// decode must leave alone) and an "@" that encode breaks.
	titles := []string{
		"plain title",
		"Email bob@example.com re @rent",
		"Renew it @due(2026-01-01)",
		"has a \u200b literal zero-width space",
		"paste\u200bmemo about @rent due @due(2026-01-01)",
		"@leading tag-like start",
		// A user's own U+200B sitting directly after an "@" is the case a naive
		// encoder leaves alone but a naive decoder still strips: the pair must
		// escape it so the round trip stays lossless.
		"Pay @\u200brent",
		"@\u200bhey(h9)",
		"@\u200b\u200bx",
		"Pay @rent",
		"bob@example.com",
		"trailing @",
		"",
		"no at sign here",
	}
	for _, title := range titles {
		t.Run(title, func(t *testing.T) {
			if got := decodeTitle(encodeTitle(title)); got != title {
				t.Errorf("decode(encode(%q)) = %q, want %q", title, got, title)
			}
		})
	}
}

func TestImport_TitleWithNewline_ImportsAsSingleNormalisedLine(t *testing.T) {
	// A HEY Title carrying a newline must be normalised to a single line before it
	// is written, so it lands as exactly one checkbox Task whose @hey Link sits on
	// the same line — never a second, unlinked open checkbox that the next Sync
	// pushes back to HEY and multiplies on every run.
	now := time.Now()
	notesDir := t.TempDir()
	statePath := filepath.Join(notesDir, "hey-state.json")
	weekEnd := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	client := &recordingClient{week: now, todos: []hey.Todo{weekTodo("h1", "Call bob\nabout the lease", weekEnd)}}
	opts := Options{Client: client, Query: "@due or @today", StatePath: statePath, NotesDir: notesDir, Now: now}

	if _, _, err := Push(context.Background(), opts); err != nil {
		t.Fatalf("first Push: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(notesDir, "inbox.md"))
	if err != nil {
		t.Fatalf("reading inbox: %v", err)
	}
	body := strings.TrimSuffix(string(raw), "\n")
	if strings.Contains(body, "\n") {
		t.Fatalf("imported title split across lines:\n%q", body)
	}
	task, _ := parser.ParseLine(body, "inbox.md", 1)
	if task == nil {
		t.Fatalf("imported line did not parse: %q", body)
	}
	if got, want := titleOf(task.Text), "Call bob about the lease"; got != want {
		t.Errorf("titleOf(imported line) = %q, want %q", got, want)
	}

	// A second Sync makes no add and imports nothing.
	addsBefore := len(client.added)
	opts.Tasks = []model.Task{*task}
	rep, warnings, err := Push(context.Background(), opts)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("second Push: err=%v warnings=%v", err, warnings)
	}
	if rep.Imported != 0 {
		t.Errorf("second Sync Imported=%d, want 0", rep.Imported)
	}
	if len(client.added) != addsBefore {
		t.Errorf("second Sync made %d Add call(s), want 0", len(client.added)-addsBefore)
	}
}

func TestImport_TitleInjectingCheckboxAndHey_YieldsOneLineOneHeyTag(t *testing.T) {
	// A crafted HEY Title carrying its own checkbox and @hey token must not smuggle
	// a second checkbox Task or a second @hey Link into the notes: normalising folds
	// it onto one line, and encodeTitle breaks the injected @hey so it plants no tag.
	now := time.Now()
	notesDir := t.TempDir()
	statePath := filepath.Join(notesDir, "hey-state.json")
	weekEnd := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	client := &recordingClient{week: now, todos: []hey.Todo{weekTodo("h1", "x\n- [ ] sneaky @hey(h9)", weekEnd)}}
	opts := Options{Client: client, Query: "@due or @today", StatePath: statePath, NotesDir: notesDir, Now: now}

	if _, _, err := Push(context.Background(), opts); err != nil {
		t.Fatalf("first Push: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(notesDir, "inbox.md"))
	if err != nil {
		t.Fatalf("reading inbox: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("import produced %d lines, want 1:\n%q", len(lines), string(raw))
	}
	task, _ := parser.ParseLine(lines[0], "inbox.md", 1)
	if task == nil {
		t.Fatalf("imported line did not parse: %q", lines[0])
	}
	heyTags := 0
	for _, tag := range task.Tags {
		if tag.Name == "hey" {
			heyTags++
			if tag.Value != "h1" {
				t.Errorf("@hey tag value = %q, want h1 (the real Link, not the injected h9)", tag.Value)
			}
		}
	}
	if heyTags != 1 {
		t.Errorf("imported line has %d @hey tags, want exactly 1; tags=%+v", heyTags, task.Tags)
	}
}

func TestImport_TitleWithTabsAndSpaceRuns_CollapsesLikeRetitle(t *testing.T) {
	// Tabs and runs of spaces in a HEY Title collapse the same way the retitle path
	// collapses them (normalizeTitle), so the imported line's Title is identical to
	// what a HEY-side rename to the same Title would have written.
	now := time.Now()
	notesDir := t.TempDir()
	statePath := filepath.Join(notesDir, "hey-state.json")
	weekEnd := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	raw := "Pay\trent   now  "
	client := &recordingClient{week: now, todos: []hey.Todo{weekTodo("h1", raw, weekEnd)}}
	opts := Options{Client: client, Query: "@due or @today", StatePath: statePath, NotesDir: notesDir, Now: now}

	if _, _, err := Push(context.Background(), opts); err != nil {
		t.Fatalf("first Push: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(notesDir, "inbox.md"))
	if err != nil {
		t.Fatalf("reading inbox: %v", err)
	}
	line := strings.TrimSuffix(string(body), "\n")
	task, _ := parser.ParseLine(line, "inbox.md", 1)
	if task == nil {
		t.Fatalf("imported line did not parse: %q", line)
	}
	if got := titleOf(task.Text); got != normalizeTitle(raw) {
		t.Errorf("titleOf(imported line) = %q, want retitle-collapsed %q", got, normalizeTitle(raw))
	}
}

func TestImport_TitleWithLiteralZeroWidthSpace_NoSpuriousRecreate(t *testing.T) {
	// A HEY Title that already carries a literal zero-width space (pasted from a
	// web page) must round-trip through import unchanged: decodeTitle removes only
	// the breaks encodeTitle inserts after an "@", not the user's own U+200B. So the
	// notes Title still matches the snapshot and no Sync Re-creates the Todo or
	// silently renames it in HEY.
	now := time.Now()
	notesDir := t.TempDir()
	statePath := filepath.Join(notesDir, "hey-state.json")
	weekEnd := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	title := "Read the\u200bmemo"
	client := &recordingClient{week: now, todos: []hey.Todo{weekTodo("h1", title, weekEnd)}}
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
	if got := titleOf(task.Text); got != title {
		t.Errorf("titleOf(imported line) = %q, want HEY title %q (literal U+200B preserved)", got, title)
	}

	// A second and third Sync make no HEY calls of any kind.
	opts.Tasks = []model.Task{*task}
	for _, pass := range []string{"second", "third"} {
		addsBefore := len(client.added)
		mutationsBefore := len(client.mutations)
		rep, warnings, err := Push(context.Background(), opts)
		if err != nil || len(warnings) != 0 {
			t.Fatalf("%s Push: err=%v warnings=%v", pass, err, warnings)
		}
		if rep.Recreated != 0 || rep.Retitled != 0 || rep.Imported != 0 {
			t.Errorf("%s Sync Recreated=%d Retitled=%d Imported=%d, want 0/0/0", pass, rep.Recreated, rep.Retitled, rep.Imported)
		}
		if len(client.added) != addsBefore {
			t.Errorf("%s Sync made %d Add call(s), want 0", pass, len(client.added)-addsBefore)
		}
		if got := client.mutations[mutationsBefore:]; len(got) != 0 {
			t.Errorf("%s Sync made HEY mutations, want none: %v", pass, got)
		}
	}
}

func TestImport_TitleWithZeroWidthSpaceAfterAt_NoSpuriousRecreate(t *testing.T) {
	// A HEY Title carrying its own U+200B directly after an "@" is the case a naive
	// decoder mangled: encodeTitle escapes the pre-existing break so decodeTitle
	// restores it exactly. The imported line parses with only pike's own tags, its
	// Title matches the HEY Title, and a second and third Sync make no HEY calls.
	now := time.Now()
	notesDir := t.TempDir()
	statePath := filepath.Join(notesDir, "hey-state.json")
	weekEnd := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	title := "Pay @\u200brent"
	client := &recordingClient{week: now, todos: []hey.Todo{weekTodo("h1", title, weekEnd)}}
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

	if got := titleOf(task.Text); got != title {
		t.Errorf("titleOf(imported line) = %q, want HEY title %q (U+200B after @ preserved)", got, title)
	}

	// A second and third Sync make no HEY calls of any kind.
	opts.Tasks = []model.Task{*task}
	for _, pass := range []string{"second", "third"} {
		addsBefore := len(client.added)
		mutationsBefore := len(client.mutations)
		rep, warnings, err := Push(context.Background(), opts)
		if err != nil || len(warnings) != 0 {
			t.Fatalf("%s Push: err=%v warnings=%v", pass, err, warnings)
		}
		if rep.Recreated != 0 || rep.Retitled != 0 || rep.Imported != 0 {
			t.Errorf("%s Sync Recreated=%d Retitled=%d Imported=%d, want 0/0/0", pass, rep.Recreated, rep.Retitled, rep.Imported)
		}
		if len(client.added) != addsBefore {
			t.Errorf("%s Sync made %d Add call(s), want 0", pass, len(client.added)-addsBefore)
		}
		if got := client.mutations[mutationsBefore:]; len(got) != 0 {
			t.Errorf("%s Sync made HEY mutations, want none: %v", pass, got)
		}
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
