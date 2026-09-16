package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/zachthieme/pike/internal/model"
)

var ctx = context.Background()

func TestSingleFileMultipleTasks(t *testing.T) {
	dir := t.TempDir()
	content := `# Project Notes

- [ ] Buy groceries @today
Some random text
- [x] Fix the bug @due(2026-03-10) @completed(2026-03-10)
- [ ] Write tests @risk
`
	writeFile(t, dir, "notes.md", content)

	s := newScanner(t, dir, []string{"**/*.md"}, nil)
	tasks, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// Verify first task
	assertTask(t, tasks[0], "notes.md", 3, "Buy groceries @today", model.Open)
	// Verify second task
	assertTask(t, tasks[1], "notes.md", 5, "Fix the bug @due(2026-03-10) @completed(2026-03-10)", model.Completed)
	// Verify third task
	assertTask(t, tasks[2], "notes.md", 6, "Write tests @risk", model.Open)
}

func TestMultipleFilesNestedDirectories(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "root.md", "- [ ] Root task\n")
	writeFile(t, dir, "sub/project.md", "- [ ] Sub task\n")
	writeFile(t, dir, "sub/deep/notes.md", "- [x] Deep task\n")

	s := newScanner(t, dir, []string{"**/*.md"}, nil)
	tasks, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// Tasks should be sorted by file path
	files := make([]string, len(tasks))
	for i, task := range tasks {
		files[i] = task.File
	}
	if !sort.StringsAreSorted(files) {
		t.Errorf("tasks not sorted by file path: %v", files)
	}

	// Verify each file is represented
	fileSet := map[string]bool{}
	for _, task := range tasks {
		fileSet[task.File] = true
	}
	for _, expected := range []string{"root.md", "sub/project.md", "sub/deep/notes.md"} {
		if !fileSet[expected] {
			t.Errorf("missing tasks from file %q", expected)
		}
	}
}

func TestIncludeGlobs(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "notes.md", "- [ ] Markdown task\n")
	writeFile(t, dir, "notes.txt", "- [ ] Text task\n")
	writeFile(t, dir, "data.csv", "- [ ] CSV task\n")

	s := newScanner(t, dir, []string{"**/*.md"}, nil)
	tasks, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task (only .md), got %d", len(tasks))
	}
	if tasks[0].Text != "Markdown task" {
		t.Errorf("expected 'Markdown task', got %q", tasks[0].Text)
	}
}

func TestExcludeGlobs(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "active.md", "- [ ] Active task\n")
	writeFile(t, dir, "archive/old.md", "- [ ] Archived task\n")
	writeFile(t, dir, "archive/deep/ancient.md", "- [ ] Ancient task\n")

	s := newScanner(t, dir, []string{"**/*.md"}, []string{"archive/**"})
	tasks, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task (archive excluded), got %d", len(tasks))
	}
	if tasks[0].Text != "Active task" {
		t.Errorf("expected 'Active task', got %q", tasks[0].Text)
	}
}

func TestRelativePaths(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "sub/dir/file.md", "- [ ] Some task\n")

	s := newScanner(t, dir, []string{"**/*.md"}, nil)
	tasks, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	expected := "sub/dir/file.md"
	if tasks[0].File != expected {
		t.Errorf("expected relative path %q, got %q", expected, tasks[0].File)
	}

	// Ensure it's not an absolute path
	if filepath.IsAbs(tasks[0].File) {
		t.Errorf("task.File should be relative, got absolute path %q", tasks[0].File)
	}
}

func TestIncrementalRefreshMtime(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "a.md", "- [ ] Task A original\n")
	writeFile(t, dir, "b.md", "- [ ] Task B unchanged\n")

	s := newScanner(t, dir, []string{"**/*.md"}, nil)
	tasks, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks after initial scan, got %d", len(tasks))
	}

	// Wait a moment so mtime will differ
	time.Sleep(50 * time.Millisecond)

	// Modify file a.md
	writeFile(t, dir, "a.md", "- [ ] Task A updated\n- [ ] Task A second\n")
	// Touch to ensure mtime is newer
	now := time.Now().Add(time.Second)
	if err := os.Chtimes(filepath.Join(dir, "a.md"), now, now); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	tasks, err = s.Refresh(ctx)
	if err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks after refresh, got %d", len(tasks))
	}

	// Verify a.md tasks are updated
	aTasks := filterByFile(tasks, "a.md")
	if len(aTasks) != 2 {
		t.Fatalf("expected 2 tasks from a.md, got %d", len(aTasks))
	}
	if aTasks[0].Text != "Task A updated" {
		t.Errorf("expected 'Task A updated', got %q", aTasks[0].Text)
	}
	if aTasks[1].Text != "Task A second" {
		t.Errorf("expected 'Task A second', got %q", aTasks[1].Text)
	}

	// Verify b.md task is unchanged
	bTasks := filterByFile(tasks, "b.md")
	if len(bTasks) != 1 {
		t.Fatalf("expected 1 task from b.md, got %d", len(bTasks))
	}
	if bTasks[0].Text != "Task B unchanged" {
		t.Errorf("expected 'Task B unchanged', got %q", bTasks[0].Text)
	}
}

func TestDeletedFileHandling(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "keep.md", "- [ ] Keep this\n")
	writeFile(t, dir, "delete.md", "- [ ] Delete this\n")

	s := newScanner(t, dir, []string{"**/*.md"}, nil)
	tasks, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	// Delete one file
	err = os.Remove(filepath.Join(dir, "delete.md"))
	if err != nil {
		t.Fatalf("failed to remove file: %v", err)
	}

	tasks, err = s.Refresh(ctx)
	if err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after deletion, got %d", len(tasks))
	}
	if tasks[0].Text != "Keep this" {
		t.Errorf("expected 'Keep this', got %q", tasks[0].Text)
	}
}

func TestNonTaskLinesSkipped(t *testing.T) {
	dir := t.TempDir()
	content := `# Heading

Regular paragraph text.

- Normal list item (not a task)
- Another list item

Some more text.

- [ ] The only real task
`
	writeFile(t, dir, "mixed.md", content)

	s := newScanner(t, dir, []string{"**/*.md"}, nil)
	tasks, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Text != "The only real task" {
		t.Errorf("expected 'The only real task', got %q", tasks[0].Text)
	}
}

func TestEmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	s := newScanner(t, dir, []string{"**/*.md"}, nil)
	tasks, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestScanCollectsWarnings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "test.md", "- [ ] task @due(bad-date)\n- [ ] ok @due(2026-03-16)\n")
	sc, err := New(dir, []string{"**/*.md"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	warns := sc.Warns()
	if len(warns) != 1 {
		t.Errorf("got %d warnings, want 1", len(warns))
	}
	if len(warns) > 0 && warns[0].Line != 1 {
		t.Errorf("warning line = %d, want 1", warns[0].Line)
	}
}

// TestConcurrentWarnsDuringRefresh reads warnings through Warns() while another
// goroutine calls Refresh. It must not data-race (run under -race): the raw
// field is unexported, so Warns() is the only reader and every access is guarded
// by the same lock the publish write takes.
func TestConcurrentWarnsDuringRefresh(t *testing.T) {
	dir := t.TempDir()
	// A file with a bad date so every scan collects a warning to read.
	writeFile(t, dir, "notes/bad.md", "- [ ] task @due(bad-date)\n")

	s := newScanner(t, dir, []string{"**/*.md"}, nil)
	if _, err := s.Scan(ctx); err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	const goroutines = 32
	var wg sync.WaitGroup

	// Writers: refresh repeatedly, republishing warnings under the lock.
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Refresh(ctx); err != nil {
				t.Errorf("Refresh() error: %v", err)
			}
		}()
	}

	// Readers: read warnings through the accessor while refreshes publish.
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				_ = s.Warns()
			}
		}()
	}

	wg.Wait()
}

// TestConcurrentRefreshStableFiles runs many Refresh calls at once against an
// unchanging file set. It must not data-race (run under -race) and every caller
// must see the same complete list a single sequential Refresh returns.
func TestConcurrentRefreshStableFiles(t *testing.T) {
	dir := t.TempDir()
	for i := range 20 {
		name := fileName(i)
		writeFile(t, dir, filepath.Join("notes", name),
			fmt.Sprintf("- [ ] task %s\n- [x] done %s\n", name, name))
	}

	s := newScanner(t, dir, []string{"**/*.md"}, nil)
	if _, err := s.Scan(ctx); err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	// Independent sequential reference over the same files.
	want := sequentialTasks(t, dir)

	const goroutines = 32
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := s.Refresh(ctx)
			if err != nil {
				errs <- err
				return
			}
			if !tasksEqual(got, want) {
				errs <- fmt.Errorf("concurrent Refresh returned %d tasks, want %d (torn or inconsistent)", len(got), len(want))
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// TestConcurrentRefreshWithChurn runs many Refresh calls while files are added,
// modified and removed. It must not data-race or panic, and once the churn
// settles a final Refresh must match a fresh sequential scan of the final files.
func TestConcurrentRefreshWithChurn(t *testing.T) {
	dir := t.TempDir()
	for i := range 30 {
		writeFile(t, dir, fileName(i), "- [ ] initial "+fileName(i)+"\n")
	}

	s := newScanner(t, dir, []string{"**/*.md"}, nil)
	if _, err := s.Scan(ctx); err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 64)

	// Churn: modify, remove and add files concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 15 {
			// modify an existing file
			writeFile(t, dir, fileName(i), "- [ ] changed "+fileName(i)+"\n- [ ] extra\n")
			// remove another
			_ = os.Remove(filepath.Join(dir, fileName(i+15)))
			// add a new one
			writeFile(t, dir, fileName(100+i), "- [ ] added "+fileName(100+i)+"\n")
		}
	}()

	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := s.Refresh(ctx)
			if err != nil {
				errs <- err
				return
			}
			// Each caller must see an internally consistent, well-formed list:
			// sorted by file, no torn per-file state.
			if !isSortedByFile(got) {
				errs <- fmt.Errorf("concurrent Refresh returned tasks not sorted by file")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	// After the churn, the cache must be consistent with the final files.
	got, err := s.Refresh(ctx)
	if err != nil {
		t.Fatalf("final Refresh() error: %v", err)
	}
	want := sequentialTasks(t, dir)
	if !tasksEqual(got, want) {
		t.Errorf("final Refresh returned %d tasks, want %d (cache diverged from disk)", len(got), len(want))
	}
}

// --- helpers ---

func fileName(i int) string {
	return fmt.Sprintf("note_%02d.md", i)
}

// sequentialTasks scans dir with a brand-new Scanner, giving an independent
// reference result unaffected by any shared cache.
func sequentialTasks(t *testing.T, dir string) []model.Task {
	t.Helper()
	ref, err := New(dir, []string{"**/*.md"}, nil)
	if err != nil {
		t.Fatalf("New() reference scanner: %v", err)
	}
	tasks, err := ref.Scan(ctx)
	if err != nil {
		t.Fatalf("reference Scan(): %v", err)
	}
	return tasks
}

func tasksEqual(a, b []model.Task) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].File != b[i].File || a[i].Line != b[i].Line ||
			a[i].Text != b[i].Text || a[i].State != b[i].State {
			return false
		}
	}
	return true
}

func isSortedByFile(tasks []model.Task) bool {
	files := make([]string, len(tasks))
	for i, task := range tasks {
		files[i] = task.File
	}
	return sort.StringsAreSorted(files)
}

func newScanner(t *testing.T, root string, include, exclude []string) *Scanner {
	t.Helper()
	s, err := New(root, include, exclude)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	return s
}

func writeFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	err := os.MkdirAll(filepath.Dir(full), 0o755)
	if err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	err = os.WriteFile(full, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func assertTask(t *testing.T, task model.Task, file string, line int, text string, state model.TaskState) {
	t.Helper()
	if task.File != file {
		t.Errorf("expected file %q, got %q", file, task.File)
	}
	if task.Line != line {
		t.Errorf("expected line %d, got %d", line, task.Line)
	}
	if task.Text != text {
		t.Errorf("expected text %q, got %q", text, task.Text)
	}
	if task.State != state {
		t.Errorf("expected state %v, got %v", state, task.State)
	}
}

func filterByFile(tasks []model.Task, file string) []model.Task {
	var result []model.Task
	for _, task := range tasks {
		if task.File == file {
			result = append(result, task)
		}
	}
	return result
}
