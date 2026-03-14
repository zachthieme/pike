package pike_test

import (
	"bufio"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"pike/internal/config"
	"pike/internal/filter"
	"pike/internal/model"
	"pike/internal/parser"
	"pike/internal/render"
	"pike/internal/style"
)

var update = flag.Bool("update", false, "update golden files")

var goldenNow = time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

// parsedTask is a JSON-friendly version of model.Task for golden files.
type parsedTask struct {
	Text        string   `json:"text"`
	State       string   `json:"state"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Tags        []string `json:"tags"`
	Due         string   `json:"due,omitempty"`
	Completed   string   `json:"completed,omitempty"`
	HasCheckbox bool     `json:"has_checkbox"`
}

func toParsedTask(t model.Task) parsedTask {
	pt := parsedTask{
		Text:        t.Text,
		State:       t.State.String(),
		File:        t.File,
		Line:        t.Line,
		HasCheckbox: t.HasCheckbox,
	}
	for _, tag := range t.Tags {
		pt.Tags = append(pt.Tags, style.TagToken(tag))
	}
	if t.Due != nil {
		pt.Due = t.Due.Format("2006-01-02")
	}
	if t.Completed != nil {
		pt.Completed = t.Completed.Format("2006-01-02")
	}
	return pt
}

func loadGoldenConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load("testdata/config.yaml")
	if err != nil {
		t.Fatalf("load test config: %v", err)
	}
	return cfg
}

func scanTestNotes(t *testing.T) map[string][]model.Task {
	t.Helper()
	result := make(map[string][]model.Task)

	entries, err := os.ReadDir("testdata/notes")
	if err != nil {
		t.Fatalf("read testdata/notes: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		fpath := filepath.Join("testdata", "notes", entry.Name())
		tasks := parseNoteFile(t, fpath, entry.Name())
		if len(tasks) > 0 {
			result[entry.Name()] = tasks
		}
	}
	return result
}

func parseNoteFile(t *testing.T, fpath, relPath string) []model.Task {
	t.Helper()
	f, err := os.Open(fpath)
	if err != nil {
		t.Fatalf("open %s: %v", fpath, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	var tasks []model.Task
	for scanner.Scan() {
		lineNum++
		task := parser.ParseLine(scanner.Text(), relPath, lineNum)
		if task != nil {
			tasks = append(tasks, *task)
		}
	}
	return tasks
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func allGoldenTasks(byFile map[string][]model.Task) []model.Task {
	var all []model.Task
	for _, tasks := range byFile {
		all = append(all, tasks...)
	}
	return all
}

func goldenCompare(t *testing.T, goldenPath string, actual []byte) {
	t.Helper()
	if *update {
		dir := filepath.Dir(goldenPath)
		os.MkdirAll(dir, 0755)
		if err := os.WriteFile(goldenPath, actual, 0644); err != nil {
			t.Fatalf("update golden %s: %v", goldenPath, err)
		}
		return
	}
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", goldenPath, err)
	}
	if string(actual) != string(expected) {
		t.Errorf("golden mismatch for %s:\n--- expected ---\n%s\n--- actual ---\n%s", goldenPath, string(expected), string(actual))
	}
}

func TestGoldenParsed(t *testing.T) {
	byFile := scanTestNotes(t)

	for _, filename := range sortedKeys(byFile) {
		tasks := byFile[filename]
		baseName := strings.TrimSuffix(filename, ".md")
		t.Run(baseName, func(t *testing.T) {
			var pts []parsedTask
			for _, task := range tasks {
				pts = append(pts, toParsedTask(task))
			}
			data, err := json.MarshalIndent(pts, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			data = append(data, '\n')
			goldenCompare(t, filepath.Join("testdata", "golden", "parsed", baseName+".json"), data)
		})
	}
}

func TestGoldenStyledANSI(t *testing.T) {
	cfg := loadGoldenConfig(t)
	byFile := scanTestNotes(t)
	sf := style.ANSIStyleFunc()

	for _, filename := range sortedKeys(byFile) {
		tasks := byFile[filename]
		baseName := strings.TrimSuffix(filename, ".md")
		t.Run(baseName, func(t *testing.T) {
			var lines []string
			for _, task := range tasks {
				text := style.ColorizeTags(task.Text, task.Tags, cfg.TagColors, sf)
				lines = append(lines, text)
			}
			actual := []byte(strings.Join(lines, "\n") + "\n")
			goldenCompare(t, filepath.Join("testdata", "golden", "styled", "ansi", baseName+".txt"), actual)
		})
	}
}

func TestGoldenStyledPlain(t *testing.T) {
	cfg := loadGoldenConfig(t)
	byFile := scanTestNotes(t)
	sf := style.ANSIStyleFunc()

	for _, filename := range sortedKeys(byFile) {
		tasks := byFile[filename]
		baseName := strings.TrimSuffix(filename, ".md")
		t.Run(baseName, func(t *testing.T) {
			var lines []string
			for _, task := range tasks {
				text := style.ColorizeTags(task.Text, task.Tags, cfg.TagColors, sf)
				text = style.StripANSI(text)
				lines = append(lines, text)
			}
			actual := []byte(strings.Join(lines, "\n") + "\n")
			goldenCompare(t, filepath.Join("testdata", "golden", "styled", "plain", baseName+".txt"), actual)
		})
	}
}

func TestGoldenStyledRender(t *testing.T) {
	cfg := loadGoldenConfig(t)
	byFile := scanTestNotes(t)

	for _, filename := range sortedKeys(byFile) {
		tasks := byFile[filename]
		baseName := strings.TrimSuffix(filename, ".md")
		t.Run(baseName, func(t *testing.T) {
			var lines []string
			for _, task := range tasks {
				line := render.FormatTask(task, cfg.TagColors, false)
				lines = append(lines, line)
			}
			actual := []byte(strings.Join(lines, "\n") + "\n")
			goldenCompare(t, filepath.Join("testdata", "golden", "styled", "render", baseName+".txt"), actual)
		})
	}
}

func TestGoldenQuery(t *testing.T) {
	byFile := scanTestNotes(t)
	all := allGoldenTasks(byFile)

	queries := map[string]string{
		"open-and-today": "open and @today",
		"overdue":        "open and @due < today",
		"completed":      "completed",
	}

	for _, name := range sortedKeys(queries) {
		queryStr := queries[name]
		t.Run(name, func(t *testing.T) {
			results, err := filter.Apply(all, queryStr, "file", goldenNow)
			if err != nil {
				t.Fatalf("filter: %v", err)
			}
			var pts []parsedTask
			for _, task := range results {
				pts = append(pts, toParsedTask(task))
			}
			data, err := json.MarshalIndent(pts, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			data = append(data, '\n')
			goldenCompare(t, filepath.Join("testdata", "golden", "query", name+".json"), data)
		})
	}
}
