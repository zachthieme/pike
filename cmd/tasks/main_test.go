package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVersionFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"--version"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "tasks "+version) {
		t.Errorf("expected version output containing %q, got %q", "tasks "+version, out)
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
	t.Setenv("TASKS_CONFIG", "")

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
	input := []string{"-d", "/tmp", "-q", "open", "-h"}
	got := expandShortFlags(input)
	expected := []string{"--dir", "/tmp", "--query", "open", "--help"}
	if len(got) != len(expected) {
		t.Fatalf("expected %d args, got %d", len(expected), len(got))
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("arg %d: expected %q, got %q", i, expected[i], got[i])
		}
	}
}
