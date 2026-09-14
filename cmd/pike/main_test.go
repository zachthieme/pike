package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zachthieme/pike/internal/config"
	"github.com/zachthieme/pike/internal/model"
)

func TestVersionFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"--version"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "pike "+version) {
		t.Errorf("expected version output containing %q, got %q", "pike "+version, out)
	}
}

func TestHelpFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"--help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Usage:") {
		t.Errorf("expected help output containing 'Usage:', got %q", out)
	}
	if !strings.Contains(out, "--dir") {
		t.Errorf("expected help output containing '--dir', got %q", out)
	}
	if !strings.Contains(out, "--query") {
		t.Errorf("expected help output containing '--query', got %q", out)
	}
}

func TestHelpShortFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"-h"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Usage:") {
		t.Errorf("expected help output containing 'Usage:', got %q", out)
	}
}

func TestMissingNotesDir(t *testing.T) {
	t.Setenv("NOTES", "")
	t.Setenv("PIKE_CONFIG", "")

	var stdout, stderr bytes.Buffer
	err := run([]string{"--config", "/dev/null"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing notes dir, got nil")
	}
	if !strings.Contains(err.Error(), "notes directory not set") {
		t.Errorf("expected 'notes directory not set' error, got: %v", err)
	}
}

func TestSummaryMode(t *testing.T) {
	dir := t.TempDir()
	today := time.Now().Format("2006-01-02")
	content := "# Test\n" +
		"- [ ] Open task @due(2020-01-01)\n" +
		"- [ ] Another open task\n" +
		"- [x] Done task @completed(" + today + ")\n"
	if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := run([]string{"--dir", dir, "--summary", "--no-color"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Open tasks") {
		t.Errorf("expected summary to contain 'Open tasks', got %q", out)
	}
	if !strings.Contains(out, "Overdue") {
		t.Errorf("expected summary to contain 'Overdue', got %q", out)
	}
}

func TestQueryMode(t *testing.T) {
	dir := t.TempDir()
	content := "# Test\n" +
		"- [ ] Buy groceries @today\n" +
		"- [ ] Fix bug @risk\n" +
		"- [x] Ship feature @completed(2026-01-01)\n"
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := run([]string{"--dir", dir, "-q", "open", "--no-color"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Buy groceries") {
		t.Errorf("expected query output to contain 'Buy groceries', got %q", out)
	}
	if !strings.Contains(out, "Fix bug") {
		t.Errorf("expected query output to contain 'Fix bug', got %q", out)
	}
	if strings.Contains(out, "Ship feature") {
		t.Errorf("expected query output to NOT contain 'Ship feature', got %q", out)
	}
}

func TestQueryModeWithSort(t *testing.T) {
	dir := t.TempDir()
	content := "# Test\n" +
		"- [ ] Zebra task\n" +
		"- [ ] Alpha task\n"
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := run([]string{"--dir", dir, "--query", "open", "--sort", "alpha", "--no-color"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	alphaIdx := strings.Index(out, "Alpha task")
	zebraIdx := strings.Index(out, "Zebra task")
	if alphaIdx < 0 || zebraIdx < 0 {
		t.Fatalf("expected both tasks in output, got %q", out)
	}
	if alphaIdx > zebraIdx {
		t.Errorf("expected Alpha before Zebra with alpha sort, got %q", out)
	}
}

func TestDirShortFlag(t *testing.T) {
	// Verify -d is expanded to --dir (tested via query mode to avoid TUI)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte("- [ ] hello\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := run([]string{"-d", dir, "-q", "open", "--no-color"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "hello") {
		t.Errorf("expected output to contain 'hello', got %q", stdout.String())
	}
}

func TestExpandShortFlags(t *testing.T) {
	input := []string{"-d", "/tmp", "-q", "open", "-h", "-v", "-w", "Today"}
	got := expandShortFlags(input)
	expected := []string{"--dir", "/tmp", "--query", "open", "--help", "--version", "--view", "Today"}
	if len(got) != len(expected) {
		t.Fatalf("expected %d args, got %d", len(expected), len(got))
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("arg %d: expected %q, got %q", i, expected[i], got[i])
		}
	}
}

func TestVersionShortFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"-v"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "pike "+version) {
		t.Errorf("expected version output, got %q", stdout.String())
	}
}

func TestCountFlag(t *testing.T) {
	dir := t.TempDir()
	content := "# Test\n- [ ] Task one\n- [ ] Task two\n- [x] Done\n"
	if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := run([]string{"--dir", dir, "-q", "open", "--count"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(stdout.String()) != "2" {
		t.Errorf("expected count '2', got %q", stdout.String())
	}
}

func TestJSONFlag(t *testing.T) {
	dir := t.TempDir()
	content := "# Test\n- [ ] Buy milk @today\n"
	if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := run([]string{"--dir", dir, "-q", "open", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"text"`) {
		t.Errorf("expected JSON output with 'text' field, got %q", out)
	}
	if !strings.Contains(out, "Buy milk") {
		t.Errorf("expected JSON to contain 'Buy milk', got %q", out)
	}
}

func TestResolveColorMode_ForceColor(t *testing.T) {
	var buf bytes.Buffer
	noColor := resolveColorMode(true, false, &buf)
	if noColor {
		t.Error("expected noColor=false when --color is forced")
	}
}

func TestResolveColorMode_ForceNoColor(t *testing.T) {
	var buf bytes.Buffer
	noColor := resolveColorMode(false, true, &buf)
	if !noColor {
		t.Error("expected noColor=true when --no-color is forced")
	}
}

func TestConfigLoadError(t *testing.T) {
	dir := t.TempDir()
	badConfig := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badConfig, []byte("{{{"), 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := run([]string{"--config", badConfig, "--dir", dir}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for bad config")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected 'loading config' error, got: %v", err)
	}
}

func TestScannerError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("notes_dir: "+dir+"\ninclude:\n  - \"[invalid\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := run([]string{"--config", cfgPath, "--dir", dir}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for invalid glob pattern")
	}
	if !strings.Contains(err.Error(), "invalid glob") {
		t.Errorf("expected 'invalid glob' error, got: %v", err)
	}
}

func TestViewFlagIgnoredInQueryMode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte("- [ ] A task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := run([]string{"--dir", dir, "--query", "open", "--view", "SomeView", "--no-color"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "A task") {
		t.Errorf("expected query results despite --view flag, got: %q", stdout.String())
	}
}

func TestSummaryWithNoTasks(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := run([]string{"--dir", dir, "--summary", "--no-color"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Open tasks") {
		t.Errorf("expected summary output even with no tasks, got: %q", out)
	}
}

func TestInvalidQueryError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte("- [ ] task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := run([]string{"--dir", dir, "--query", "((("}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for invalid query")
	}
}

func TestWriteDueDates_WritesCorrectJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "due.json")

	d1 := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC) // duplicate of d1
	tasks := []model.Task{
		{Due: &d1},
		{Due: &d2},
		{Due: &d3},
		{Due: nil}, // no due date
	}

	writeDueDates(path, tasks, "", time.Now())

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read due.json: %v", err)
	}

	var dates []string
	if err := json.Unmarshal(data, &dates); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(dates) != 2 {
		t.Fatalf("expected 2 unique dates, got %d: %v", len(dates), dates)
	}
	// Should be sorted
	if dates[0] != "2026-03-15" || dates[1] != "2026-03-20" {
		t.Errorf("dates = %v, want [2026-03-15 2026-03-20]", dates)
	}
}

func TestWriteDueDates_EmptyPathIsNoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "should-not-exist.json")

	d := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tasks := []model.Task{{Due: &d}}

	writeDueDates("", tasks, "", time.Now())

	if _, err := os.Stat(path); err == nil {
		t.Error("expected no file when path is empty")
	}
}

func TestWriteDueDates_NoTasksWritesEmptyArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "due.json")

	writeDueDates(path, nil, "", time.Now())

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read due.json: %v", err)
	}
	if string(data) != "[]" {
		t.Errorf("expected empty JSON array, got %q", string(data))
	}
}

func TestWriteDueDates_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "due.json")

	d := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	writeDueDates(path, []model.Task{{Due: &d}}, "", time.Now())

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read due.json: %v", err)
	}

	var dates []string
	if err := json.Unmarshal(data, &dates); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(dates) != 1 || dates[0] != "2026-05-01" {
		t.Errorf("dates = %v, want [2026-05-01]", dates)
	}
}

func TestWriteDueDates_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "due.json")

	// Write initial content
	d1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	writeDueDates(path, []model.Task{{Due: &d1}}, "", time.Now())

	// Overwrite with new content
	d2 := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	writeDueDates(path, []model.Task{{Due: &d2}}, "", time.Now())

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read due.json: %v", err)
	}

	var dates []string
	if err := json.Unmarshal(data, &dates); err != nil {
		t.Fatalf("invalid JSON after overwrite: %v", err)
	}
	if len(dates) != 1 || dates[0] != "2026-06-15" {
		t.Errorf("dates = %v, want [2026-06-15]", dates)
	}

	// No temp files should remain
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".due-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestQueryMode_DoesNotWriteDueDates(t *testing.T) {
	notesDir := t.TempDir()
	content := "- [ ] Task @due(2026-03-20)\n"
	if err := os.WriteFile(filepath.Join(notesDir, "test.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	dueDir := t.TempDir()
	duePath := filepath.Join(dueDir, "due.json")

	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	cfgContent := "notes_dir: " + notesDir + "\ndue_dates_path: " + duePath + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := run([]string{"--config", cfgPath, "--query", "open", "--no-color"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(duePath); err == nil {
		t.Error("due.json should not be written in --query mode")
	}
}

func TestDueDatesQuery_NoTaggedView(t *testing.T) {
	views := []config.ViewConfig{
		{Title: "Open", Query: "open", Sort: "file"},
	}
	got := dueDatesQuery(views)
	if got != "open and @due" {
		t.Errorf("dueDatesQuery() = %q, want %q", got, "open and @due")
	}
}

func TestDueDatesQuery_TaggedView(t *testing.T) {
	views := []config.ViewConfig{
		{Title: "Open", Query: "open", Sort: "file"},
		{Title: "Export", Query: "open and @due < today+30d", Sort: "due_asc", DueDates: true},
	}
	got := dueDatesQuery(views)
	if got != "open and @due < today+30d" {
		t.Errorf("dueDatesQuery() = %q, want %q", got, "open and @due < today+30d")
	}
}

func TestWriteDueDates_FiltersWithQuery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "due.json")

	openDue := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	completedDue := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		model.TaskWith(model.Task{Text: "open task", State: model.Open, HasCheckbox: true,
			Tags: []model.Tag{{Name: "due", Value: "2026-03-20"}},
			Due:  &openDue}),
		model.TaskWith(model.Task{Text: "done task", State: model.Completed, HasCheckbox: true,
			Tags: []model.Tag{{Name: "due", Value: "2026-03-15"}},
			Due:  &completedDue}),
	}

	now := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	writeDueDates(path, tasks, "open and @due", now)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read due.json: %v", err)
	}

	var dates []string
	if err := json.Unmarshal(data, &dates); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Only the open task's due date should appear
	if len(dates) != 1 {
		t.Fatalf("expected 1 date, got %d: %v", len(dates), dates)
	}
	if dates[0] != "2026-03-20" {
		t.Errorf("dates[0] = %q, want %q", dates[0], "2026-03-20")
	}
}

func TestNotesEnvFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte("- [ ] hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NOTES", dir)

	var stdout, stderr bytes.Buffer
	err := run([]string{"-q", "open", "--no-color"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "hello") {
		t.Errorf("expected output to contain 'hello', got %q", stdout.String())
	}
}

func TestEditorDefaultFallback(t *testing.T) {
	t.Setenv("EDITOR", "")
	cfg, err := config.LoadBytes([]byte("notes_dir: ~/Notes"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Editor != "hx" {
		t.Errorf("Editor = %q, want %q when $EDITOR is empty", cfg.Editor, "hx")
	}
}

func TestWarningOutput(t *testing.T) {
	dir := t.TempDir()
	content := "- [ ] Task with bad date @due(not-a-date)\n"
	if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := run([]string{"--dir", dir, "--query", "open", "--no-color"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "warning:") || !strings.Contains(stderr.String(), "test.md") {
		t.Errorf("expected warning mentioning file on stderr, got: %q", stderr.String())
	}
}

// writeSyncConfig writes a config file with a hey: block pointing at the test
// stub, and a notes directory with a few tasks. It returns the config path,
// notes dir, and the state path (which should never be created by a dry run).
func writeSyncConfig(t *testing.T, extraHey string) (cfgPath, notesDir, statePath string) {
	t.Helper()
	dir := t.TempDir()
	notesDir = filepath.Join(dir, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	notes := "- [ ] Buy milk @today\n" +
		"- [ ] Old linked @today @hey(500001)\n" +
		"- [ ] Secret @today @hidden\n"
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte(notes), 0o644); err != nil {
		t.Fatal(err)
	}
	stub, err := filepath.Abs(filepath.Join("testdata", "hey-stub.sh"))
	if err != nil {
		t.Fatal(err)
	}
	statePath = filepath.Join(dir, "hey-state.json")
	cfgPath = filepath.Join(dir, "config.yaml")
	cfg := "hey:\n  command: " + stub + "\n  state_path: " + statePath + "\n" + extraHey
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath, notesDir, statePath
}

func TestSyncWithoutHeyBlockErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("- [ ] task\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := run([]string{"--config", "/dev/null", "--dir", dir, "--sync"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error when --sync is used without a hey: block")
	}
	if !strings.Contains(err.Error(), "hey") {
		t.Errorf("error should name the hey block, got: %v", err)
	}
}

func TestSyncConflictingFlags(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("- [ ] task\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, conflict := range [][]string{
		{"--summary"},
		{"--query", "open"},
		{"--scope", filepath.Join(dir, "notes.md")},
		{"--view", "Today"},
	} {
		args := append([]string{"--dir", dir, "--sync"}, conflict...)
		var stdout, stderr bytes.Buffer
		err := run(args, &stdout, &stderr)
		if err == nil {
			t.Errorf("expected --sync %v to conflict", conflict)
		}
	}
}

func TestDryRunWithoutSyncWarns(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("- [ ] task\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := run([]string{"--dir", dir, "--query", "open", "--dry-run", "--no-color"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "dry-run") {
		t.Errorf("expected a warning about --dry-run without --sync, got: %q", stderr.String())
	}
}

func TestSyncDryRunEndToEnd(t *testing.T) {
	cfgPath, notesDir, statePath := writeSyncConfig(t, "")
	notesFile := filepath.Join(notesDir, "notes.md")
	before, err := os.ReadFile(notesFile)
	if err != nil {
		t.Fatal(err)
	}
	mutationLog := filepath.Join(t.TempDir(), "mutations.log")
	t.Setenv("PIKE_STUB_MUTATION_LOG", mutationLog)

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--config", cfgPath, "--dir", notesDir, "--sync", "--dry-run"}, &stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}

	out := stdout.String()
	// 1 eligible task (Buy milk) would push; 1 unlinked open todo (500002)
	// would import; 1 link (500001) already exists.
	for _, want := range []string{"1 task", "1 todo", "1 link"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}

	// Nothing written to notes, HEY, or state.
	after, err := os.ReadFile(notesFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("notes file was modified during a dry run")
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Errorf("state file should not be written during a dry run (stat err: %v)", err)
	}
	if _, err := os.Stat(mutationLog); !os.IsNotExist(err) {
		t.Error("a mutating HEY verb was called during a dry run")
	}
}

func TestSyncRealPushEndToEnd(t *testing.T) {
	cfgPath, notesDir, statePath := writeSyncConfig(t, "")
	notesFile := filepath.Join(notesDir, "notes.md")
	mutationLog := filepath.Join(t.TempDir(), "mutations.log")
	t.Setenv("PIKE_STUB_MUTATION_LOG", mutationLog)

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--config", cfgPath, "--dir", notesDir, "--sync"}, &stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}

	// Only the eligible "Buy milk" task is pushed; the linked and hidden tasks
	// are left alone.
	if !strings.Contains(stdout.String(), "1 task(s) pushed") {
		t.Errorf("report should count one push:\n%s", stdout.String())
	}

	after, err := os.ReadFile(notesFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "- [ ] Buy milk @today @hey(500009)\n" +
		"- [ ] Old linked @today @hey(500001)\n" +
		"- [ ] Secret @today @hidden\n"
	if string(after) != want {
		t.Errorf("notes after push:\n got: %q\nwant: %q", string(after), want)
	}

	// The add call carried the stripped title and no --date (no @due).
	log, err := os.ReadFile(mutationLog)
	if err != nil {
		t.Fatalf("expected a recorded add call: %v", err)
	}
	if !strings.Contains(string(log), "add Buy milk") {
		t.Errorf("add call should use the stripped title:\n%s", string(log))
	}
	if strings.Contains(string(log), "--date") {
		t.Errorf("task without @due should push no date:\n%s", string(log))
	}

	// The Link is recorded in the state file.
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("state file should be written: %v", err)
	}
	if !strings.Contains(string(stateData), "500009") || !strings.Contains(string(stateData), "Buy milk") {
		t.Errorf("state file missing the new link:\n%s", string(stateData))
	}
}

func TestSyncRealImportEndToEnd(t *testing.T) {
	cfgPath, notesDir, statePath := writeSyncConfig(t, "")
	inboxFile := filepath.Join(notesDir, "inbox.md")
	if _, err := os.Stat(inboxFile); !os.IsNotExist(err) {
		t.Fatalf("inbox.md should be absent before sync (stat err: %v)", err)
	}

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--config", cfgPath, "--dir", notesDir, "--sync"}, &stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}

	// The report counts the one unlinked open todo (500002) imported.
	if !strings.Contains(stdout.String(), "1 todo(s) imported") {
		t.Errorf("report should count one import:\n%s", stdout.String())
	}

	// The inbox is created and carries the imported line: @due on the Week's
	// Saturday and an @hey Link, so a later scan treats it as Linked.
	got, err := os.ReadFile(inboxFile)
	if err != nil {
		t.Fatalf("inbox should be created: %v", err)
	}
	want := "- [ ] Unlinked todo @due(2026-09-19) @hey(500002)\n"
	if string(got) != want {
		t.Errorf("inbox after import:\n got: %q\nwant: %q", string(got), want)
	}

	// The completed todo (500003) is never imported.
	if strings.Contains(string(got), "Old todo") {
		t.Errorf("completed todo should not be imported:\n%s", string(got))
	}

	// The import Link is recorded in the state file.
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("state file should be written: %v", err)
	}
	if !strings.Contains(string(stateData), "500002") {
		t.Errorf("state file missing the import link:\n%s", string(stateData))
	}
}

func TestSyncDryRunJSON(t *testing.T) {
	cfgPath, notesDir, _ := writeSyncConfig(t, "")
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--config", cfgPath, "--dir", notesDir, "--sync", "--dry-run", "--json"}, &stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}
	var rep struct {
		DryRun        bool `json:"dry_run"`
		WouldPush     int  `json:"would_push"`
		WouldImport   int  `json:"would_import"`
		ExistingLinks int  `json:"existing_links"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, stdout.String())
	}
	if !rep.DryRun || rep.WouldPush != 1 || rep.WouldImport != 1 || rep.ExistingLinks != 1 {
		t.Errorf("unexpected report: %+v", rep)
	}
}

func TestSyncCreatesDefaultStateDir(t *testing.T) {
	// With no state_path configured, the state file defaults under
	// $XDG_DATA_HOME/pike, which does not exist on a fresh machine. Sync must
	// create the directory and write the file.
	dir := t.TempDir()
	notesDir := filepath.Join(dir, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte("- [ ] Buy milk @today\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stub, err := filepath.Abs(filepath.Join("testdata", "hey-stub.sh"))
	if err != nil {
		t.Fatal(err)
	}
	xdgData := filepath.Join(dir, "missing-xdg-data")
	t.Setenv("XDG_DATA_HOME", xdgData)
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("hey:\n  command: "+stub+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--config", cfgPath, "--dir", notesDir, "--sync"}, &stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}

	statePath := filepath.Join(xdgData, "pike", "hey-state.json")
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("default state file should be written under a missing XDG dir: %v", err)
	}
}

func TestSyncUnauthenticatedExitsNonZero(t *testing.T) {
	cfgPath, notesDir, _ := writeSyncConfig(t, "")
	t.Setenv("PIKE_STUB_AUTH", "1")
	var stdout, stderr bytes.Buffer
	err := run([]string{"--config", cfgPath, "--dir", notesDir, "--sync", "--dry-run"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected a non-zero exit when HEY reports an auth error")
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("error should carry HEY's message, got: %v", err)
	}
}

func TestSyncCommandMissingExitsNonZero(t *testing.T) {
	_, notesDir, statePath := writeSyncConfig(t, "")
	// Point the command at a path that does not exist.
	missingCfg := filepath.Join(t.TempDir(), "config.yaml")
	cfg := "hey:\n  command: /nonexistent/hey-binary\n  state_path: " + statePath + "\n"
	if err := os.WriteFile(missingCfg, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := run([]string{"--config", missingCfg, "--dir", notesDir, "--sync", "--dry-run"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected a non-zero exit when the hey command cannot run")
	}
}

func TestSyncCompletionEndToEnd(t *testing.T) {
	dir := t.TempDir()
	notesDir := filepath.Join(dir, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 500001 is open in HEY (stub) but completed in the notes → completed wins,
	// so the Todo is completed in HEY. 500003 is completed in HEY but open in the
	// notes → completed wins, so the notes line is checked with @completed.
	notes := "- [x] Finish report @hey(500001) @completed(2026-09-12)\n" +
		"- [ ] Read book @hey(500003)\n"
	notesFile := filepath.Join(notesDir, "notes.md")
	if err := os.WriteFile(notesFile, []byte(notes), 0o644); err != nil {
		t.Fatal(err)
	}
	stub, err := filepath.Abs(filepath.Join("testdata", "hey-stub.sh"))
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "hey-state.json")
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := "hey:\n  command: " + stub + "\n  state_path: " + statePath + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	mutationLog := filepath.Join(t.TempDir(), "mutations.log")
	t.Setenv("PIKE_STUB_MUTATION_LOG", mutationLog)

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--config", cfgPath, "--dir", notesDir, "--sync"}, &stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}

	// The report counts two completions (one on each side).
	if !strings.Contains(stdout.String(), "2 item(s) completed") {
		t.Errorf("report should count two completions:\n%s", stdout.String())
	}

	// The open Todo was completed in HEY.
	log, err := os.ReadFile(mutationLog)
	if err != nil {
		t.Fatalf("expected a recorded complete call: %v", err)
	}
	if !strings.Contains(string(log), "complete 500001") {
		t.Errorf("expected complete 500001 in mutation log:\n%s", string(log))
	}

	// The open notes Task was checked and stamped from HEY's completed_at.
	after, err := os.ReadFile(notesFile)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(after), "\n")
	if !strings.HasPrefix(lines[1], "- [x] Read book @hey(500003) @completed(") {
		t.Errorf("second line should be completed from HEY:\n%s", string(after))
	}
	// The completed Task in the notes was left as-is.
	if lines[0] != "- [x] Finish report @hey(500001) @completed(2026-09-12)" {
		t.Errorf("first line should be preserved:\n%s", string(after))
	}

	// The state file records both agreed completions.
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("state file should be written: %v", err)
	}
	if !strings.Contains(string(stateData), "500001") || !strings.Contains(string(stateData), "500003") {
		t.Errorf("state should record both links:\n%s", string(stateData))
	}
}

func TestHelpDocumentsSync(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "--sync") {
		t.Errorf("help should document --sync:\n%s", out)
	}
	if !strings.Contains(out, "--dry-run") {
		t.Errorf("help should document --dry-run:\n%s", out)
	}
}
