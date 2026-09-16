package toggle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zachthieme/pike/internal/scanner"
)

// scanRaws runs the real scanner over the file at path and returns each Task's
// Raw — the exact source line as the scanner reads it, with any trailing "\r"
// dropped. It lets an AppendTask test assert on what pike will actually see when
// it next reads the file, rather than on bytes alone.
func scanRaws(t *testing.T, path string) []string {
	t.Helper()
	sc, err := scanner.New(filepath.Dir(path), []string{"**/*.md"}, nil)
	if err != nil {
		t.Fatalf("scanner.New: %v", err)
	}
	tasks, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	raws := make([]string, len(tasks))
	for i, tk := range tasks {
		raws[i] = tk.Raw
	}
	return raws
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

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
		name    string
		line    string
		newText string
		want    string
	}{
		{
			name:    "tags at end preserved in order",
			line:    "- [ ] Old title @hey(h1) @due(2026-01-01)",
			newText: "New title",
			want:    "- [ ] New title @hey(h1) @due(2026-01-01)\n",
		},
		{
			name:    "interspersed tags moved to end in original order",
			line:    "- [ ] Buy @urgent milk @hey(h1)",
			newText: "Buy groceries",
			want:    "- [ ] Buy groceries @urgent @hey(h1)\n",
		},
		{
			name:    "completed checkbox untouched",
			line:    "- [x] Old @hey(h1) @completed(2026-01-01)",
			newText: "New",
			want:    "- [x] New @hey(h1) @completed(2026-01-01)\n",
		},
		{
			name:    "indentation preserved",
			line:    "  - [ ] Old @hey(h1)",
			newText: "New name",
			want:    "  - [ ] New name @hey(h1)\n",
		},
		{
			name:    "no tags",
			line:    "- [ ] Old title",
			newText: "New title",
			want:    "- [ ] New title\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			p := writeFile(t, dir, "test.md", tt.line+"\n")
			if err := SetText(context.Background(), p, 1, tt.line, tt.newText); err != nil {
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
	err := SetText(context.Background(), p, 1, "- [ ] Different text @hey(h1)", "New")
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
	if err := SetTagValue(context.Background(), p, 1, "- [ ] Task @hey(h1) @due(2026-01-01)", "hey", "h2"); err != nil {
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
	err := SetTagValue(context.Background(), p, 1, "- [ ] Task with no link", "hey", "h2")
	if !errors.Is(err, ErrStaleData) {
		t.Fatalf("expected ErrStaleData, got: %v", err)
	}
}

func TestSetTagValueAmbiguous(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Task @hey(h1) @hey(h9)\n")
	err := SetTagValue(context.Background(), p, 1, "- [ ] Task @hey(h1) @hey(h9)", "hey", "h2")
	if !errors.Is(err, ErrAmbiguousTag) {
		t.Fatalf("expected ErrAmbiguousTag, got: %v", err)
	}
	if got := readFile(t, p); got != "- [ ] Task @hey(h1) @hey(h9)\n" {
		t.Errorf("file should be unchanged, got: %q", got)
	}
}

func TestSetDueReplacesExistingValue(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Task @hey(h1) @due(2026-01-01)\n")
	date := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	if err := SetDue(context.Background(), p, 1, "- [ ] Task @hey(h1) @due(2026-01-01)", date); err != nil {
		t.Fatalf("SetDue: %v", err)
	}
	want := "- [ ] Task @hey(h1) @due(2026-09-19)\n"
	if got := readFile(t, p); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestSetDueAppendsWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Task @hey(h1)\n")
	date := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	if err := SetDue(context.Background(), p, 1, "- [ ] Task @hey(h1)", date); err != nil {
		t.Fatalf("SetDue: %v", err)
	}
	want := "- [ ] Task @hey(h1) @due(2026-09-19)\n"
	if got := readFile(t, p); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestSetDueAmbiguous(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Task @due(2026-01-01) @due(2026-02-02)\n")
	date := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	err := SetDue(context.Background(), p, 1, "- [ ] Task @due(2026-01-01) @due(2026-02-02)", date)
	if !errors.Is(err, ErrAmbiguousTag) {
		t.Fatalf("expected ErrAmbiguousTag, got: %v", err)
	}
	if got := readFile(t, p); got != "- [ ] Task @due(2026-01-01) @due(2026-02-02)\n" {
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

	if err := AppendTag(context.Background(), p, 1, "- [ ] Buy milk @today", "@hey(h1)"); err != nil {
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

	// The scan saw "- [ ] Buy milk @today"; the line has since been edited.
	err := AppendTag(context.Background(), p, 1, "- [ ] Buy milk @today", "@hey(h1)")
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

	t.Run("appends a CRLF line to a CRLF file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "tasks.md")
		os.WriteFile(path, []byte("# Notes\r\n- [ ] existing task\r\n"), 0o644) //nolint:errcheck // test setup

		if err := AppendTask(context.Background(), path, "buy milk @today"); err != nil {
			t.Fatalf("AppendTask error: %v", err)
		}

		want := "# Notes\r\n- [ ] existing task\r\n- [ ] buy milk @today\r\n"
		if got := readFile(t, path); got != want {
			t.Errorf("file content\n got: %q\nwant: %q", got, want)
		}
	})

	t.Run("appends an LF line to an LF file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "tasks.md")
		os.WriteFile(path, []byte("- [ ] existing task\n"), 0o644) //nolint:errcheck // test setup

		if err := AppendTask(context.Background(), path, "buy milk @today"); err != nil {
			t.Fatalf("AppendTask error: %v", err)
		}

		want := "- [ ] existing task\n- [ ] buy milk @today\n"
		if got := readFile(t, path); got != want {
			t.Errorf("file content\n got: %q\nwant: %q", got, want)
		}
	})

	t.Run("appends to a file whose final line ends in a bare CR", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "tasks.md")
		os.WriteFile(path, []byte("# N\r\n- [ ] existing\r"), 0o644) //nolint:errcheck // test setup

		if err := AppendTask(context.Background(), path, "new task"); err != nil {
			t.Fatalf("AppendTask error: %v", err)
		}

		// No doubled CR: the bare "\r" ending is swapped for the file's own "\r\n",
		// and the appended line uses that same ending.
		want := "# N\r\n- [ ] existing\r\n- [ ] new task\r\n"
		if got := readFile(t, path); got != want {
			t.Errorf("file content\n got: %q\nwant: %q", got, want)
		}
		if strings.Contains(readFile(t, path), "\r\r") {
			t.Errorf("appended file has a doubled CR: %q", readFile(t, path))
		}
		// Re-scanning finds the pre-existing Task unchanged and the appended one.
		if got, want := scanRaws(t, path), []string{"- [ ] existing", "- [ ] new task"}; !equalStrings(got, want) {
			t.Errorf("re-scan after append\n got: %q\nwant: %q", got, want)
		}
	})

	t.Run("appending to a file ending in two CRs leaves the existing line's scanner-visible content unchanged", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "tasks.md")
		// parseLines treats one of the two trailing CRs as the line's ending and
		// leaves the other as content, so the scanner already reads "- [ ] a\r".
		// Appending must not change that line it does not own.
		os.WriteFile(path, []byte("- [ ] a\r\r"), 0o644) //nolint:errcheck // test setup
		before := scanRaws(t, path)
		if len(before) != 1 || before[0] != "- [ ] a\r" {
			t.Fatalf("precondition: scanner sees %q, want [\"- [ ] a\\r\"]", before)
		}

		if err := AppendTask(context.Background(), path, "new task"); err != nil {
			t.Fatalf("AppendTask error: %v", err)
		}

		// The existing line's scanner-visible content survives; the appended one parses.
		if got, want := scanRaws(t, path), []string{"- [ ] a\r", "- [ ] new task"}; !equalStrings(got, want) {
			t.Errorf("re-scan after append\n got: %q\nwant: %q", got, want)
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

// dueDate is a fixed date the line-verifying mutation tests reschedule to.
var dueDate = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

// TestLineVerifyingMutations_ByteForByteGuard exercises every Sync mutation
// against the four ways a line can drift between scan and write. An unchanged
// line is written; a line edited to still contain the scanned text, a line
// replaced by another Task, and a line shifted down by an insertion above it
// each return ErrStaleData and leave the file byte-for-byte unchanged.
func TestLineVerifyingMutations_ByteForByteGuard(t *testing.T) {
	ctx := context.Background()
	muts := []struct {
		name    string
		scanned string                                          // the full line as scanned
		run     func(p string, line int, wantLine string) error // invoke the mutation
		success string                                          // file content after a write on the unchanged line
	}{
		{
			name:    "AppendTag",
			scanned: "- [ ] Buy milk @today",
			run:     func(p string, line int, w string) error { return AppendTag(ctx, p, line, w, "@hey(h1)") },
			success: "- [ ] Buy milk @today @hey(h1)\n",
		},
		{
			name:    "RemoveTag",
			scanned: "- [ ] Buy milk @hey(h1)",
			run:     func(p string, line int, w string) error { return RemoveTag(ctx, p, line, w, "hey") },
			success: "- [ ] Buy milk\n",
		},
		{
			name:    "SetTagValue",
			scanned: "- [ ] Buy milk @hey(h1)",
			run:     func(p string, line int, w string) error { return SetTagValue(ctx, p, line, w, "hey", "h2") },
			success: "- [ ] Buy milk @hey(h2)\n",
		},
		{
			name:    "SetText",
			scanned: "- [ ] Old title @hey(h1)",
			run:     func(p string, line int, w string) error { return SetText(ctx, p, line, w, "New title") },
			success: "- [ ] New title @hey(h1)\n",
		},
		{
			name:    "SetDue",
			scanned: "- [ ] Buy milk @hey(h1)",
			run:     func(p string, line int, w string) error { return SetDue(ctx, p, line, w, dueDate) },
			success: "- [ ] Buy milk @hey(h1) @due(2026-09-01)\n",
		},
		{
			name:    "CompleteLine",
			scanned: "- [ ] Buy milk @hey(h1)",
			run:     func(p string, line int, w string) error { return CompleteLine(ctx, p, line, w, dueDate) },
			success: "- [x] Buy milk @hey(h1) @completed(2026-09-01)\n",
		},
		{
			name:    "UncompleteLine",
			scanned: "- [x] Buy milk @hey(h1) @completed(2026-01-01)",
			run:     func(p string, line int, w string) error { return UncompleteLine(ctx, p, line, w) },
			success: "- [ ] Buy milk @hey(h1)\n",
		},
	}

	for _, mt := range muts {
		t.Run(mt.name+"/unchanged line is written", func(t *testing.T) {
			dir := t.TempDir()
			p := writeFile(t, dir, "test.md", mt.scanned+"\n")
			if err := mt.run(p, 1, mt.scanned); err != nil {
				t.Fatalf("%s on unchanged line: %v", mt.name, err)
			}
			if got := readFile(t, p); got != mt.success {
				t.Errorf("got:\n%q\nwant:\n%q", got, mt.success)
			}
		})

		// Each stale case pairs a file the mutation now finds with the scanned
		// line it still believes it is acting on; every one must be skipped.
		stale := []struct {
			name string
			file string
		}{
			{"edited to still contain old text", mt.scanned + " @urgent\n"},
			{"replaced by another task", "- [ ] Something entirely different @hey(h9)\n"},
			{"shifted down by an insertion", "- [ ] Inserted above\n" + mt.scanned + "\n"},
		}
		for _, sc := range stale {
			t.Run(mt.name+"/"+sc.name, func(t *testing.T) {
				dir := t.TempDir()
				p := writeFile(t, dir, "test.md", sc.file)
				err := mt.run(p, 1, mt.scanned)
				if !errors.Is(err, ErrStaleData) {
					t.Fatalf("%s: expected ErrStaleData, got: %v", mt.name, err)
				}
				if got := readFile(t, p); got != sc.file {
					t.Errorf("stale line must be left byte-for-byte unchanged\n got: %q\nwant: %q", got, sc.file)
				}
			})
		}
	}
}

// endingMutation is one Sync mutation for the ending-preservation matrix: the
// line as the scanner hands it over (no trailing "\r"), how to invoke the
// mutation, and that line's content after a successful write (no ending).
type endingMutation struct {
	name    string
	scanned string
	run     func(p string, line int, wantLine string) error
	success string
}

// endingMutations lists every Sync mutation the ending-preservation matrix
// drives, so each line-ending style below reuses one table rather than copying it.
func endingMutations(ctx context.Context) []endingMutation {
	return []endingMutation{
		{
			name:    "AppendTag",
			scanned: "- [ ] Buy milk @today",
			run:     func(p string, line int, w string) error { return AppendTag(ctx, p, line, w, "@hey(h1)") },
			success: "- [ ] Buy milk @today @hey(h1)",
		},
		{
			name:    "RemoveTag",
			scanned: "- [ ] Buy milk @hey(h1)",
			run:     func(p string, line int, w string) error { return RemoveTag(ctx, p, line, w, "hey") },
			success: "- [ ] Buy milk",
		},
		{
			name:    "SetTagValue",
			scanned: "- [ ] Buy milk @hey(h1)",
			run:     func(p string, line int, w string) error { return SetTagValue(ctx, p, line, w, "hey", "h2") },
			success: "- [ ] Buy milk @hey(h2)",
		},
		{
			name:    "SetText",
			scanned: "- [ ] Old title @hey(h1)",
			run:     func(p string, line int, w string) error { return SetText(ctx, p, line, w, "New title") },
			success: "- [ ] New title @hey(h1)",
		},
		{
			name:    "SetDue",
			scanned: "- [ ] Buy milk @hey(h1)",
			run:     func(p string, line int, w string) error { return SetDue(ctx, p, line, w, dueDate) },
			success: "- [ ] Buy milk @hey(h1) @due(2026-09-01)",
		},
		{
			name:    "CompleteLine",
			scanned: "- [ ] Buy milk @hey(h1)",
			run:     func(p string, line int, w string) error { return CompleteLine(ctx, p, line, w, dueDate) },
			success: "- [x] Buy milk @hey(h1) @completed(2026-09-01)",
		},
		{
			name:    "UncompleteLine",
			scanned: "- [x] Buy milk @hey(h1) @completed(2026-01-01)",
			run:     func(p string, line int, w string) error { return UncompleteLine(ctx, p, line, w) },
			success: "- [ ] Buy milk @hey(h1)",
		},
	}
}

// TestLineVerifyingMutations_EndingsPreserved runs every Sync mutation against a
// file in each line-ending style pike must round-trip: CRLF throughout, a final
// line ended only by a bare "\r" (a truncated write or classic CR-only endings),
// LF and CRLF lines mixed in one file, and a final line with no ending at all.
// The scanned line the caller passes never carries a trailing "\r" (the
// scanner's bufio.Scanner drops it), so each write must match the line by its
// content, change only that content, and leave every other byte — including each
// neighbour's own ending — untouched. A line that really changed is refused with
// ErrStaleData and nothing is written.
func TestLineVerifyingMutations_EndingsPreserved(t *testing.T) {
	ctx := context.Background()
	// staleContent is a genuinely different line the mutation finds at the target
	// position; it must never match the scanned line and so must be refused.
	const staleContent = "- [ ] Something entirely different @hey(h9)"
	styles := []struct {
		name string
		line int                         // 1-based position of the target line
		file func(content string) string // frame the target line's content into a whole file
	}{
		{
			name: "CRLF",
			line: 2,
			// A three-line CRLF file so a neighbour's ending is checked too.
			file: func(c string) string { return "# Notes\r\n" + c + "\r\n- [ ] Another\r\n" },
		},
		{
			name: "bare-CR final line",
			line: 2,
			// A CRLF header then a final line terminated only by a bare "\r".
			file: func(c string) string { return "# Notes\r\n" + c + "\r" },
		},
		{
			name: "mixed LF and CRLF",
			line: 2,
			// A CRLF header, the LF-terminated target line, then a CRLF neighbour:
			// each line must keep its own ending across the write.
			file: func(c string) string { return "# Notes\r\n" + c + "\n- [ ] Another\r\n" },
		},
		{
			name: "no trailing newline",
			line: 2,
			// The target is the final line, ended by nothing at all.
			file: func(c string) string { return "# Notes\n" + c },
		},
	}

	for _, st := range styles {
		for _, mt := range endingMutations(ctx) {
			t.Run(st.name+"/"+mt.name+"/unchanged line is written, endings preserved", func(t *testing.T) {
				dir := t.TempDir()
				p := writeFile(t, dir, "test.md", st.file(mt.scanned))
				if err := mt.run(p, st.line, mt.scanned); err != nil {
					t.Fatalf("%s on unchanged %s line: %v", mt.name, st.name, err)
				}
				want := st.file(mt.success)
				if got := readFile(t, p); got != want {
					t.Errorf("got:\n%q\nwant:\n%q", got, want)
				}
			})

			t.Run(st.name+"/"+mt.name+"/stale line refused when it really changed", func(t *testing.T) {
				dir := t.TempDir()
				file := st.file(staleContent)
				p := writeFile(t, dir, "test.md", file)
				err := mt.run(p, st.line, mt.scanned)
				if !errors.Is(err, ErrStaleData) {
					t.Fatalf("%s: expected ErrStaleData, got: %v", mt.name, err)
				}
				if got := readFile(t, p); got != file {
					t.Errorf("stale %s line must be left byte-for-byte unchanged\n got: %q\nwant: %q", st.name, got, file)
				}
			})
		}
	}
}

// TestSetDue_RespectsTagBoundary confirms @due never matches the "@due" inside
// a longer tag like @duedate: only the real @due(...) token is rewritten.
func TestSetDue_RespectsTagBoundary(t *testing.T) {
	dir := t.TempDir()
	line := "- [ ] x @duedate @due(2026-09-01)"
	p := writeFile(t, dir, "test.md", line+"\n")
	if err := SetDue(context.Background(), p, 1, line, time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SetDue: %v", err)
	}
	want := "- [ ] x @duedate @due(2026-12-25)\n"
	if got := readFile(t, p); got != want {
		t.Errorf("only @due(...) should change\n got: %q\nwant: %q", got, want)
	}
}

// TestSetTagValue_RespectsTagBoundary confirms a line carrying @heybot alongside
// @hey(1) holds exactly one Link: only the real @hey token is rewritten, so the
// mutation is unambiguous rather than seeing two @hey tags.
func TestSetTagValue_RespectsTagBoundary(t *testing.T) {
	dir := t.TempDir()
	line := "- [ ] Task @heybot @hey(1)"
	p := writeFile(t, dir, "test.md", line+"\n")
	if err := SetTagValue(context.Background(), p, 1, line, "hey", "h2"); err != nil {
		t.Fatalf("SetTagValue: %v", err)
	}
	want := "- [ ] Task @heybot @hey(h2)\n"
	if got := readFile(t, p); got != want {
		t.Errorf("only @hey(...) should change\n got: %q\nwant: %q", got, want)
	}
}

func TestRemoveTagStripsTagLeavingRestUnchanged(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Buy milk @hey(h1) @due(2026-01-01)\nother line\n")

	if err := RemoveTag(context.Background(), p, 1, "- [ ] Buy milk @hey(h1) @due(2026-01-01)", "hey"); err != nil {
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

	if err := RemoveTag(context.Background(), p, 1, "- [ ] Buy milk @today @hey(h1)", "hey"); err != nil {
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

	err := RemoveTag(context.Background(), p, 1, "- [ ] Buy milk @today", "hey")
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

	err := RemoveTag(context.Background(), p, 1, "- [ ] Buy milk @hey(h1) @hey(h2)", "hey")
	if !errors.Is(err, ErrAmbiguousTag) {
		t.Fatalf("expected ErrAmbiguousTag for two @hey tags, got: %v", err)
	}
	if got := readFile(t, p); got != "- [ ] Buy milk @hey(h1) @hey(h2)\n" {
		t.Errorf("ambiguous line must not be written, got:\n%s", got)
	}
}
