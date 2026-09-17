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
)

func TestReport_RepeatedDryRun_ByteIdenticalWithOrphansAndPendingDeletes(t *testing.T) {
	now := time.Now()
	notesDir := t.TempDir()
	// No task lines, so every Linked state entry whose Todo survives in HEY is an
	// Orphan; the notes file exists but holds nothing pike links to.
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte("# notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(notesDir, "hey-state.json")
	state := &State{Links: map[string]Link{
		"h_orphan_b": {Title: "Orphan B", File: "notes.md", Line: 1},
		"h_orphan_a": {Title: "Orphan A", File: "notes.md", Line: 1},
		"h_pd_c":     {PendingDelete: true, Title: "Pending C", File: "notes.md", Line: 1},
		"h_pd_a":     {PendingDelete: true, Title: "Pending A", File: "notes.md", Line: 1},
		"h_pd_b":     {PendingDelete: true, Title: "Pending B", File: "notes.md", Line: 1},
	}}
	if err := SaveState(statePath, state); err != nil {
		t.Fatal(err)
	}
	stateBefore, _ := os.ReadFile(statePath)

	// Every state Todo is still live and open in HEY, so the orphans stay orphaned
	// and the pending deletes stay outstanding across runs.
	todos := []hey.Todo{
		openTodo("h_orphan_a", "Orphan A"), openTodo("h_orphan_b", "Orphan B"),
		openTodo("h_pd_a", "Pending A"), openTodo("h_pd_b", "Pending B"), openTodo("h_pd_c", "Pending C"),
	}

	render := func() (string, string) {
		t.Helper()
		client := &recordingClient{week: now, todos: append([]hey.Todo(nil), todos...)}
		rep, _, err := Plan(context.Background(), Options{
			Client: client, Query: "@today", StatePath: statePath, NotesDir: notesDir, Now: now, DryRun: true,
		})
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		var text, jsonBuf bytes.Buffer
		if err := rep.WriteText(&text); err != nil {
			t.Fatalf("WriteText: %v", err)
		}
		if err := rep.WriteJSON(&jsonBuf); err != nil {
			t.Fatalf("WriteJSON: %v", err)
		}
		return text.String(), jsonBuf.String()
	}

	text1, json1 := render()
	text2, json2 := render()
	if text1 != text2 {
		t.Errorf("dry-run text not byte-identical across runs:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", text1, text2)
	}
	if json1 != json2 {
		t.Errorf("dry-run JSON not byte-identical across runs:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", json1, json2)
	}
	// Every orphan and pending delete must be named, in sorted order.
	for _, want := range []string{"h_orphan_a", "h_orphan_b", "h_pd_a", "h_pd_b", "h_pd_c"} {
		if !strings.Contains(text1, want) {
			t.Errorf("text report missing %q:\n%s", want, text1)
		}
	}
	if i, j := strings.Index(text1, "h_pd_a"), strings.Index(text1, "h_pd_c"); i < 0 || j < 0 || i > j {
		t.Errorf("pending deletes not sorted by id in text:\n%s", text1)
	}
	// A dry run writes nothing to the state file.
	stateAfter, _ := os.ReadFile(statePath)
	if !bytes.Equal(stateBefore, stateAfter) {
		t.Error("dry run modified the state file")
	}
}

func TestReportWriteText_ShowsCountsAndDryRun(t *testing.T) {
	rep := &Report{DryRun: true, WouldPush: 2, WouldImport: 3, ExistingLinks: 4}
	var buf bytes.Buffer
	if err := rep.WriteText(&buf); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"2", "3", "4", "push", "import"} {
		if !strings.Contains(strings.ToLower(out), strings.ToLower(want)) {
			t.Errorf("text report missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(strings.ToLower(out), "dry") {
		t.Errorf("dry-run report should say so:\n%s", out)
	}
}

func TestReportWriteText_RealRunShowsPushedAndFailures(t *testing.T) {
	rep := &Report{Pushed: 2, Failed: 1, ExistingLinks: 4, Imported: 5}
	var buf bytes.Buffer
	if err := rep.WriteText(&buf); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := strings.ToLower(buf.String())
	for _, want := range []string{"2", "push", "1", "fail", "4", "link", "5", "import"} {
		if !strings.Contains(out, want) {
			t.Errorf("text report missing %q:\n%s", want, buf.String())
		}
	}
	if strings.Contains(out, "dry") {
		t.Errorf("real run should not say dry:\n%s", buf.String())
	}
}

func TestReportWriteText_FailedLabelIsGeneric(t *testing.T) {
	// A failure count covers completion, unlink and reschedule write failures, not
	// only pushes, so the label must be a generic "failed", never "failed to push".
	rep := &Report{Failed: 3}
	var buf bytes.Buffer
	if err := rep.WriteText(&buf); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "3 item(s) failed") {
		t.Errorf("want a generic \"3 item(s) failed\" line:\n%s", out)
	}
	if strings.Contains(out, "failed to push") {
		t.Errorf("failure label should not be push-specific:\n%s", out)
	}
}

func TestReportWriteText_NamesOrphansAndPendingDeletes(t *testing.T) {
	rep := &Report{
		Orphans:     1,
		OrphanItems: []Item{{ID: "h_lost", Title: "Gone task"}},
		PendingDeletes: []Item{
			{ID: "h_leak1", Title: "Leaked one"},
			{ID: "h_leak2", Title: "Leaked two"},
		},
	}
	var buf bytes.Buffer
	if err := rep.WriteText(&buf); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"h_lost", "Gone task", "h_leak1", "Leaked one", "h_leak2", "Leaked two"} {
		if !strings.Contains(out, want) {
			t.Errorf("text report missing %q:\n%s", want, out)
		}
	}
}

func TestReportWriteText_OmitsEmptyItemSections(t *testing.T) {
	// With no orphans or pending deletes, the extra sections must not appear.
	rep := &Report{Pushed: 1}
	var buf bytes.Buffer
	if err := rep.WriteText(&buf); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := strings.ToLower(buf.String())
	if strings.Contains(out, "pending delete") {
		t.Errorf("empty pending-delete section should not render:\n%s", buf.String())
	}
}

func TestReportWriteJSON_CarriesItemArrays(t *testing.T) {
	rep := &Report{
		OrphanItems:    []Item{{ID: "h_lost", Title: "Gone task"}},
		PendingDeletes: []Item{{ID: "h_leak", Title: "Leaked"}},
	}
	var buf bytes.Buffer
	if err := rep.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var got Report
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if !reflect.DeepEqual(got.OrphanItems, rep.OrphanItems) {
		t.Errorf("OrphanItems round-trip = %+v, want %+v", got.OrphanItems, rep.OrphanItems)
	}
	if !reflect.DeepEqual(got.PendingDeletes, rep.PendingDeletes) {
		t.Errorf("PendingDeletes round-trip = %+v, want %+v", got.PendingDeletes, rep.PendingDeletes)
	}
}

func TestReportWriteJSON_RoundTrips(t *testing.T) {
	rep := &Report{DryRun: true, WouldPush: 2, WouldImport: 3, ExistingLinks: 4}
	var buf bytes.Buffer
	if err := rep.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var got Report
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if !reflect.DeepEqual(got, *rep) {
		t.Errorf("round-trip = %+v, want %+v", got, *rep)
	}
}
