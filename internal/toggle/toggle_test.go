package toggle

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

	err := Complete(p, 2, date)
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

	err := Complete(p, 1, date)
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

	err := Complete(p, 1, time.Now())
	if err == nil {
		t.Fatal("expected error for non-checkbox line")
	}
}

func TestCompleteLineOutOfRange(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [ ] Only line\n")

	err := Complete(p, 5, time.Now())
	if err == nil {
		t.Fatal("expected error for out-of-range line")
	}
}

func TestUncompleteBasic(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [x] Done task @completed(2026-03-14)\n")

	err := Uncomplete(p, 1)
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

	err := Uncomplete(p, 1)
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

	err := Uncomplete(p, 1)
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

	err := Uncomplete(p, 1)
	if err == nil {
		t.Fatal("expected error for non-completed line")
	}
}

func TestUncompletePreservesOtherTags(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.md", "- [x] Task @today @completed(2026-03-14) @risk\n")

	err := Uncomplete(p, 1)
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

	err := ToggleHidden(p, 1)
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

	err := ToggleHidden(p, 1)
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

	err := ToggleHidden(p, 1)
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

	err := ToggleHidden(p, 1)
	if err != nil {
		t.Fatalf("ToggleHidden: %v", err)
	}

	got := readFile(t, p)
	want := "- Review design @talk @hidden\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
