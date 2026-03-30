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
