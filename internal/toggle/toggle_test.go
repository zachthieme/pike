package toggle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUncompleteUppercaseX(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [X] Finished task @completed(2026-03-15)\n")
	ctx := context.Background()

	err := Uncomplete(ctx, p, 1)
	if err != nil {
		t.Fatalf("Uncomplete: %v", err)
	}
	got := readFile(t, p)
	if got != "- [ ] Finished task\n" {
		t.Errorf("unexpected result:\n%s", got)
	}
}

func TestVerifyUnmodifiedDetectsExternalChange(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Original task\n")

	// Simulate: read the file, then externally modify it, then verify.
	originalLine := "- [ ] Original task"
	if err := os.WriteFile(p, []byte("- [ ] Changed by someone else\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := verifyUnmodified(p, 1, originalLine)
	if !errors.Is(err, ErrStaleData) {
		t.Fatalf("expected ErrStaleData, got: %v", err)
	}
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(data)
}

func TestCompleteBasic(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "# Notes\n- [ ] Buy groceries\n- [ ] Clean house\n")
	date := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)

	err := Complete(context.Background(), p, 2, date)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got := readFile(t, p)
	want := "# Notes\n- [x] Buy groceries @completed(2026-03-14)\n- [ ] Clean house\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompleteIndented(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "  - [ ] Indented task\n")
	date := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)

	err := Complete(context.Background(), p, 1, date)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got := readFile(t, p)
	want := "  - [x] Indented task @completed(2026-03-14)\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompleteWrongLineContent(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "# Just a heading\n")

	err := Complete(context.Background(), p, 1, time.Now())
	if err == nil {
		t.Fatal("expected error for non-checkbox line")
	}
}

func TestCompleteLineOutOfRange(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Only line\n")

	err := Complete(context.Background(), p, 5, time.Now())
	if err == nil {
		t.Fatal("expected error for out-of-range line")
	}
}

func TestUncompleteBasic(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [x] Done task @completed(2026-03-14)\n")

	err := Uncomplete(context.Background(), p, 1)
	if err != nil {
		t.Fatalf("Uncomplete: %v", err)
	}

	got := readFile(t, p)
	want := "- [ ] Done task\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUncompleteWithoutDate(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [x] Done task @completed\n")

	err := Uncomplete(context.Background(), p, 1)
	if err != nil {
		t.Fatalf("Uncomplete: %v", err)
	}

	got := readFile(t, p)
	want := "- [ ] Done task\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUncompleteIndented(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "  - [x] Indented @completed(2026-03-14)\n")

	err := Uncomplete(context.Background(), p, 1)
	if err != nil {
		t.Fatalf("Uncomplete: %v", err)
	}

	got := readFile(t, p)
	want := "  - [ ] Indented\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUncompleteWrongLineContent(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Still open\n")

	err := Uncomplete(context.Background(), p, 1)
	if err == nil {
		t.Fatal("expected error for non-completed line")
	}
}

func TestUncompletePreservesOtherTags(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [x] Task @today @completed(2026-03-14) @risk\n")

	err := Uncomplete(context.Background(), p, 1)
	if err != nil {
		t.Fatalf("Uncomplete: %v", err)
	}

	got := readFile(t, p)
	want := "- [ ] Task @today @risk\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestToggleHiddenAdd(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Buy groceries @today\n")

	err := ToggleHidden(context.Background(), p, 1)
	if err != nil {
		t.Fatalf("ToggleHidden: %v", err)
	}

	got := readFile(t, p)
	want := "- [ ] Buy groceries @today @hidden\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestToggleHiddenRemove(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Buy groceries @today @hidden\n")

	err := ToggleHidden(context.Background(), p, 1)
	if err != nil {
		t.Fatalf("ToggleHidden: %v", err)
	}

	got := readFile(t, p)
	want := "- [ ] Buy groceries @today\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestToggleHiddenPreservesOtherTags(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Task @today @hidden @risk\n")

	err := ToggleHidden(context.Background(), p, 1)
	if err != nil {
		t.Fatalf("ToggleHidden: %v", err)
	}

	got := readFile(t, p)
	want := "- [ ] Task @today @risk\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestToggleHiddenTaggedBullet(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- Review design @talk\n")

	err := ToggleHidden(context.Background(), p, 1)
	if err != nil {
		t.Fatalf("ToggleHidden: %v", err)
	}

	got := readFile(t, p)
	want := "- Review design @talk @hidden\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompleteCancelledContext(t *testing.T) {
	dir := t.TempDir()
	original := "- [ ] Buy milk\n"
	p := writeFile(t, dir, "test.md", original)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Complete(ctx, p, 1, time.Now())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	got := readFile(t, p)
	if got != original {
		t.Errorf("file was modified despite cancelled context:\n%s", got)
	}
}

func TestUncompleteCancelledContext(t *testing.T) {
	dir := t.TempDir()
	original := "- [x] Buy milk @completed(2026-03-17)\n"
	p := writeFile(t, dir, "test.md", original)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Uncomplete(ctx, p, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	got := readFile(t, p)
	if got != original {
		t.Errorf("file was modified despite cancelled context:\n%s", got)
	}
}

func TestToggleHiddenCancelledContext(t *testing.T) {
	dir := t.TempDir()
	original := "- [ ] Buy milk\n"
	p := writeFile(t, dir, "test.md", original)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ToggleHidden(ctx, p, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	got := readFile(t, p)
	if got != original {
		t.Errorf("file was modified despite cancelled context:\n%s", got)
	}
}

func TestAppendTask(t *testing.T) {
	t.Run("appends to existing file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "tasks.md")
		os.WriteFile(path, []byte("- [ ] existing task\n"), 0o644) //nolint:errcheck // test setup

		err := AppendTask(context.Background(), path, "buy milk @today")
		if err != nil {
			t.Fatalf("AppendTask error: %v", err)
		}

		data, _ := os.ReadFile(path)
		lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 lines, got %d: %q", len(lines), string(data))
		}
		if lines[1] != "- [ ] buy milk @today" {
			t.Errorf("line 2 = %q, want '- [ ] buy milk @today'", lines[1])
		}
	})

	t.Run("creates file if not exists", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "new.md")

		err := AppendTask(context.Background(), path, "first task")
		if err != nil {
			t.Fatalf("AppendTask error: %v", err)
		}

		data, _ := os.ReadFile(path)
		want := "- [ ] first task\n"
		if string(data) != want {
			t.Errorf("file content = %q, want %q", string(data), want)
		}
	})

	t.Run("empty text returns error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "tasks.md")

		err := AppendTask(context.Background(), path, "")
		if err == nil {
			t.Error("expected error for empty text")
		}
	})
}
