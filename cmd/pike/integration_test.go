package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zachthieme/pike/internal/filter"
	"github.com/zachthieme/pike/internal/render"
	"github.com/zachthieme/pike/internal/scanner"
)

// runPipeline executes the scan→filter→render pipeline and returns the output.
func runPipeline(t *testing.T, dir string, queryStr string, sortOrder string, now time.Time) string {
	t.Helper()
	sc, err := scanner.New(dir, []string{"**/*.md"}, nil)
	if err != nil {
		t.Fatalf("scanner.New: %v", err)
	}
	tasks, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatalf("scanner.Scan: %v", err)
	}
	filtered, err := filter.Apply(tasks, queryStr, sortOrder, now)
	if err != nil {
		t.Fatalf("filter.Apply: %v", err)
	}
	var lines []string
	for _, task := range filtered {
		lines = append(lines, render.FormatTask(task, nil, true))
	}
	return strings.Join(lines, "\n") + "\n"
}

// setupFiles creates markdown files in dir from a map of name→content.
func setupFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIntegration_BasicOpen(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] Buy groceries\n- [x] Walk dog @completed(2026-01-01)\n- [ ] Fix bug\n",
	})
	got := runPipeline(t, dir, "open", "file", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(got, "Buy groceries") {
		t.Errorf("expected 'Buy groceries' in output:\n%s", got)
	}
	if !strings.Contains(got, "Fix bug") {
		t.Errorf("expected 'Fix bug' in output:\n%s", got)
	}
	if strings.Contains(got, "Walk dog") {
		t.Errorf("should not contain completed task 'Walk dog':\n%s", got)
	}
}

func TestIntegration_Completed(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] Open task\n- [x] Done task @completed(2026-03-10)\n",
	})
	got := runPipeline(t, dir, "completed", "file", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
	if strings.Contains(got, "Open task") {
		t.Errorf("should not contain open task:\n%s", got)
	}
	if !strings.Contains(got, "Done task") {
		t.Errorf("expected 'Done task' in output:\n%s", got)
	}
}

func TestIntegration_DateQuery(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] Overdue @due(2026-03-10)\n- [ ] Future @due(2026-04-01)\n- [ ] No date\n",
	})
	now := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	got := runPipeline(t, dir, "open and @due < today", "file", now)
	if !strings.Contains(got, "Overdue") {
		t.Errorf("expected 'Overdue' in output:\n%s", got)
	}
	if strings.Contains(got, "Future") {
		t.Errorf("should not contain 'Future':\n%s", got)
	}
}

func TestIntegration_TagQuery(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] Work item @work\n- [ ] Personal item @personal\n- [ ] Both @work @personal\n",
	})
	got := runPipeline(t, dir, "@work", "file", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(got, "Work item") {
		t.Errorf("expected 'Work item':\n%s", got)
	}
	if !strings.Contains(got, "Both") {
		t.Errorf("expected 'Both':\n%s", got)
	}
	if strings.Contains(got, "Personal item") {
		t.Errorf("should not contain 'Personal item' (without @work):\n%s", got)
	}
}

func TestIntegration_RegexQuery(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] Deploy to staging\n- [ ] Deploy to production\n- [ ] Write tests\n",
	})
	got := runPipeline(t, dir, "/[Dd]eploy/", "file", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(got, "Deploy to staging") {
		t.Errorf("expected 'Deploy to staging':\n%s", got)
	}
	if !strings.Contains(got, "Deploy to production") {
		t.Errorf("expected 'Deploy to production':\n%s", got)
	}
	if strings.Contains(got, "Write tests") {
		t.Errorf("should not contain 'Write tests':\n%s", got)
	}
}

func TestIntegration_MultiFile(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"alpha.md": "- [ ] Alpha task\n",
		"beta.md":  "- [ ] Beta task\n",
		"gamma.md": "- [ ] Gamma task\n",
	})
	got := runPipeline(t, dir, "open", "file", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
	alphaIdx := strings.Index(got, "Alpha task")
	betaIdx := strings.Index(got, "Beta task")
	gammaIdx := strings.Index(got, "Gamma task")
	if alphaIdx < 0 || betaIdx < 0 || gammaIdx < 0 {
		t.Fatalf("expected all three tasks in output:\n%s", got)
	}
	if alphaIdx > betaIdx || betaIdx > gammaIdx {
		t.Errorf("expected file-sorted order (alpha < beta < gamma):\n%s", got)
	}
}

func TestIntegration_HiddenFiltered(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] Visible task\n- [ ] Hidden task @hidden\n",
	})
	got := runPipeline(t, dir, "open and not @hidden", "file", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(got, "Visible task") {
		t.Errorf("expected 'Visible task':\n%s", got)
	}
	if strings.Contains(got, "Hidden task") {
		t.Errorf("should not contain 'Hidden task':\n%s", got)
	}
}

func TestIntegration_PinnedSort(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] Normal task\n- [ ] Pinned task @pin\n- [ ] Another normal\n",
	})
	sc, err := scanner.New(dir, []string{"**/*.md"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := filter.Apply(tasks, "open", "file", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) < 2 {
		t.Fatalf("expected at least 2 tasks, got %d", len(filtered))
	}
	if !strings.Contains(filtered[0].Text, "Pinned task") {
		t.Errorf("expected pinned task first, got %q", filtered[0].Text)
	}
}

func TestIntegration_SummaryPath(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC)
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n" +
			"- [ ] Open one\n" +
			"- [ ] Overdue @due(2026-03-10)\n" +
			"- [ ] Due this week @due(2026-03-18)\n" +
			"- [x] Done today @completed(2026-03-17)\n",
	})
	sc, err := scanner.New(dir, []string{"**/*.md"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	open, err := filter.Apply(tasks, "open", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 3 {
		t.Errorf("expected 3 open tasks, got %d", len(open))
	}

	overdue, err := filter.Apply(tasks, "open and @due < today", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(overdue) != 1 {
		t.Errorf("expected 1 overdue task, got %d", len(overdue))
	}
}

// runCLI is a helper that calls run() with the given args and returns stdout/stderr.
func runCLI(t *testing.T, args []string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err = run(args, &out, &errOut)
	return out.String(), errOut.String(), err
}

func TestIntegration_ScopeBasic(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"people/Bob Smith.md": "# Bob Smith\n- [ ] My own task @today\n",
		"projects/alpha.md":   "# Alpha\n- [ ] Talk to [[Bob Smith]] about deployment @talk\n- [x] Done with [[bob-smith]] @completed(2026-01-01)\n- [ ] Unrelated task @today\n",
		"projects/beta.md":    "# Beta\n- [ ] @delegated([[bob-smith]]) finish report\n",
	})

	stdout, _, err := runCLI(t, []string{"--dir", dir, "--scope", filepath.Join(dir, "people/Bob Smith.md"), "--no-color"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should include open tasks referencing Bob from other files
	if !strings.Contains(stdout, "Talk to [[Bob Smith]]") {
		t.Errorf("expected wikilink task in output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "@delegated([[bob-smith]])") {
		t.Errorf("expected delegated task in output:\n%s", stdout)
	}
	// Should exclude self (Bob Smith.md's own tasks)
	if strings.Contains(stdout, "My own task") {
		t.Errorf("should not contain scoped file's own tasks:\n%s", stdout)
	}
	// Should exclude unrelated tasks
	if strings.Contains(stdout, "Unrelated task") {
		t.Errorf("should not contain unrelated tasks:\n%s", stdout)
	}
	// Should exclude completed tasks (implicit open filter)
	if strings.Contains(stdout, "Done with") {
		t.Errorf("should not contain completed tasks:\n%s", stdout)
	}
}

func TestIntegration_ScopeWithQuery(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"people/Bob Smith.md": "# Bob Smith\n",
		"tasks.md":            "# Tasks\n- [ ] Talk to [[Bob Smith]] @talk\n- [ ] Ask [[bob-smith]] about API @today\n",
	})

	stdout, _, err := runCLI(t, []string{"--dir", dir, "--scope", filepath.Join(dir, "people/Bob Smith.md"), "--query", "@talk", "--no-color"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(stdout, "@talk") {
		t.Errorf("expected @talk task in output:\n%s", stdout)
	}
	// @today task references Bob but doesn't have @talk
	if strings.Contains(stdout, "@today") {
		t.Errorf("should not contain non-@talk tasks:\n%s", stdout)
	}
}

func TestIntegration_ScopeWithCount(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"people/Bob Smith.md": "# Bob Smith\n",
		"tasks.md":            "# Tasks\n- [ ] Talk to [[Bob Smith]] @talk\n- [ ] Ask [[bob-smith]] about API\n",
	})

	stdout, _, err := runCLI(t, []string{"--dir", dir, "--scope", filepath.Join(dir, "people/Bob Smith.md"), "--count"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.TrimSpace(stdout) != "2" {
		t.Errorf("expected count 2, got %q", strings.TrimSpace(stdout))
	}
}

func TestIntegration_ScopeWithJSON(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"people/Bob Smith.md": "# Bob Smith\n",
		"tasks.md":            "# Tasks\n- [ ] Talk to [[Bob Smith]] @talk\n",
	})

	stdout, _, err := runCLI(t, []string{"--dir", dir, "--scope", filepath.Join(dir, "people/Bob Smith.md"), "--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(stdout, `"text"`) {
		t.Errorf("expected JSON output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Bob Smith") {
		t.Errorf("expected Bob Smith in JSON output:\n%s", stdout)
	}
}

func TestIntegration_ScopePlusSummaryError(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"people/Bob Smith.md": "# Bob Smith\n",
	})

	_, _, err := runCLI(t, []string{"--dir", dir, "--scope", filepath.Join(dir, "people/Bob Smith.md"), "--summary"})
	if err == nil {
		t.Fatal("expected error for --scope + --summary")
	}
	if !strings.Contains(err.Error(), "--scope and --summary cannot be combined") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestIntegration_ScopeViewAndQueryError(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"people/Bob Smith.md": "# Bob Smith\n",
	})

	_, _, err := runCLI(t, []string{"--dir", dir, "--scope", filepath.Join(dir, "people/Bob Smith.md"), "--view", "Today", "--query", "@talk"})
	if err == nil {
		t.Fatal("expected error for --scope + --view + --query")
	}
	if !strings.Contains(err.Error(), "--view and --query cannot both be used with --scope") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestIntegration_ScopeFileNotFound(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"tasks.md": "# Tasks\n- [ ] A task\n",
	})

	_, _, err := runCLI(t, []string{"--dir", dir, "--scope", filepath.Join(dir, "nonexistent.md")})
	if err == nil {
		t.Fatal("expected error for non-existent scope file")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestIntegration_ScopeNoMatches(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"people/Bob Smith.md": "# Bob Smith\n",
		"tasks.md":            "# Tasks\n- [ ] Unrelated task @today\n",
	})

	stdout, _, err := runCLI(t, []string{"--dir", dir, "--scope", filepath.Join(dir, "people/Bob Smith.md"), "--no-color"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty output for no matches, got:\n%s", stdout)
	}
}

func TestIntegration_ScopeWithView(t *testing.T) {
	dir := t.TempDir()
	setupFiles(t, dir, map[string]string{
		"people/Bob Smith.md": "# Bob Smith\n",
		"tasks.md":            "# Tasks\n- [ ] Talk to [[Bob Smith]] @today\n- [ ] Ask [[bob-smith]] about API @talk\n",
	})

	// Write a config file with a "Today" view.
	configPath := filepath.Join(dir, "config.yaml")
	configContent := "notes_dir: " + dir + "\nviews:\n  - title: \"Today\"\n    query: \"open and @today\"\n    sort: file\n    order: 1\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Clear env vars that could override notes_dir from config.
	t.Setenv("NOTES", "")
	t.Setenv("PIKE_CONFIG", "")

	stdout, _, err := runCLI(t, []string{"--config", configPath, "--scope", filepath.Join(dir, "people/Bob Smith.md"), "--view", "Today", "--no-color"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(stdout, "@today") {
		t.Errorf("expected @today task in output:\n%s", stdout)
	}
	if strings.Contains(stdout, "@talk") {
		t.Errorf("should not contain non-today tasks:\n%s", stdout)
	}
}
