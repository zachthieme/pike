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

func TestSetTextKeepsTags(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantText string
		newText  string
		want     string
	}{
		{
			name:     "tags at end preserved in order",
			line:     "- [ ] Old title @hey(h1) @due(2026-01-01)",
			wantText: "Old title @hey(h1) @due(2026-01-01)",
			newText:  "New title",
			want:     "- [ ] New title @hey(h1) @due(2026-01-01)\n",
		},
		{
			name:     "interspersed tags moved to end in original order",
			line:     "- [ ] Buy @urgent milk @hey(h1)",
			wantText: "Buy @urgent milk @hey(h1)",
			newText:  "Buy groceries",
			want:     "- [ ] Buy groceries @urgent @hey(h1)\n",
		},
		{
			name:     "completed checkbox untouched",
			line:     "- [x] Old @hey(h1) @completed(2026-01-01)",
			wantText: "Old @hey(h1) @completed(2026-01-01)",
			newText:  "New",
			want:     "- [x] New @hey(h1) @completed(2026-01-01)\n",
		},
		{
			name:     "indentation preserved",
			line:     "  - [ ] Old @hey(h1)",
			wantText: "Old @hey(h1)",
			newText:  "New name",
			want:     "  - [ ] New name @hey(h1)\n",
		},
		{
			name:     "no tags",
			line:     "- [ ] Old title",
			wantText: "Old title",
			newText:  "New title",
			want:     "- [ ] New title\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			p := writeFile(t, dir, "test.md", tt.line+"\n")
			if err := SetText(context.Background(), p, 1, tt.wantText, tt.newText); err != nil {
				t.Fatalf("SetText: %v", err)
			}
			if got := readFile(t, p); got != tt.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestSetTextStaleWhenTextChanged(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Current text @hey(h1)\n")
	err := SetText(context.Background(), p, 1, "Different text @hey(h1)", "New")
	if !errors.Is(err, ErrStaleData) {
		t.Fatalf("expected ErrStaleData, got: %v", err)
	}
	if got := readFile(t, p); got != "- [ ] Current text @hey(h1)\n" {
		t.Errorf("file should be unchanged, got: %q", got)
	}
}

func TestSetTagValueRewritesValue(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Task @hey(h1) @due(2026-01-01)\n")
	if err := SetTagValue(context.Background(), p, 1, "hey", "h2"); err != nil {
		t.Fatalf("SetTagValue: %v", err)
	}
	want := "- [ ] Task @hey(h2) @due(2026-01-01)\n"
	if got := readFile(t, p); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestSetTagValueStaleWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Task with no link\n")
	err := SetTagValue(context.Background(), p, 1, "hey", "h2")
	if !errors.Is(err, ErrStaleData) {
		t.Fatalf("expected ErrStaleData, got: %v", err)
	}
}

func TestSetTagValueAmbiguous(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Task @hey(h1) @hey(h9)\n")
	err := SetTagValue(context.Background(), p, 1, "hey", "h2")
	if !errors.Is(err, ErrAmbiguousTag) {
		t.Fatalf("expected ErrAmbiguousTag, got: %v", err)
	}
	if got := readFile(t, p); got != "- [ ] Task @hey(h1) @hey(h9)\n" {
		t.Errorf("file should be unchanged, got: %q", got)
	}
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

func TestAppendTagAppendsLeavingRestUnchanged(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Buy milk @today\nother line\n")

	if err := AppendTag(context.Background(), p, 1, "Buy milk @today", "@hey(h1)"); err != nil {
		t.Fatalf("AppendTag: %v", err)
	}

	got := readFile(t, p)
	want := "- [ ] Buy milk @today @hey(h1)\nother line\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAppendTagStaleLineIsSkipped(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Reworded since scan\n")

	// The scan saw "Buy milk @today"; the line has since been edited.
	err := AppendTag(context.Background(), p, 1, "Buy milk @today", "@hey(h1)")
	if !errors.Is(err, ErrStaleData) {
		t.Fatalf("expected ErrStaleData, got: %v", err)
	}
	if got := readFile(t, p); got != "- [ ] Reworded since scan\n" {
		t.Errorf("stale line must not be corrupted, got:\n%s", got)
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

func TestRemoveTagStripsTagLeavingRestUnchanged(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Buy milk @hey(h1) @due(2026-01-01)\nother line\n")

	if err := RemoveTag(context.Background(), p, 1, "hey"); err != nil {
		t.Fatalf("RemoveTag: %v", err)
	}

	got := readFile(t, p)
	want := "- [ ] Buy milk @due(2026-01-01)\nother line\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveTagAtEndOfLine(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Buy milk @today @hey(h1)\n")

	if err := RemoveTag(context.Background(), p, 1, "hey"); err != nil {
		t.Fatalf("RemoveTag: %v", err)
	}

	got := readFile(t, p)
	want := "- [ ] Buy milk @today\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveTagMissingTagIsStale(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Buy milk @today\n")

	err := RemoveTag(context.Background(), p, 1, "hey")
	if !errors.Is(err, ErrStaleData) {
		t.Fatalf("expected ErrStaleData when the tag is absent, got: %v", err)
	}
	if got := readFile(t, p); got != "- [ ] Buy milk @today\n" {
		t.Errorf("line must be left unchanged, got:\n%s", got)
	}
}

func TestRemoveTagTwoTagsIsAmbiguousAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Buy milk @hey(h1) @hey(h2)\n")

	err := RemoveTag(context.Background(), p, 1, "hey")
	if !errors.Is(err, ErrAmbiguousTag) {
		t.Fatalf("expected ErrAmbiguousTag for two @hey tags, got: %v", err)
	}
	if got := readFile(t, p); got != "- [ ] Buy milk @hey(h1) @hey(h2)\n" {
		t.Errorf("ambiguous line must not be written, got:\n%s", got)
	}
}
