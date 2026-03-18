package scope

import (
	"testing"

	"pike/internal/model"
)

func TestIdentity(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     []string
	}{
		{
			name:     "spaces in name",
			filename: "Bob Smith.md",
			want:     []string{"Bob Smith", "bob-smith", "bob smith"},
		},
		{
			name:     "hyphenated slug",
			filename: "project-alpha.md",
			want:     []string{"project-alpha", "Project Alpha", "project alpha"},
		},
		{
			name:     "directory stripped",
			filename: "people/Bob Smith.md",
			want:     []string{"Bob Smith", "bob-smith", "bob smith"},
		},
		{
			name:     "special characters preserved",
			filename: "O'Brien.md",
			want:     []string{"O'Brien", "o'brien"},
		},
		{
			name:     "multi-word slug",
			filename: "meeting-notes.md",
			want:     []string{"meeting-notes", "Meeting Notes", "meeting notes"},
		},
		{
			name:     "absolute path stripped",
			filename: "/home/user/notes/people/Bob Smith.md",
			want:     []string{"Bob Smith", "bob-smith", "bob smith"},
		},
		{
			name:     "no extension",
			filename: "readme",
			want:     []string{"readme", "Readme"},
		},
		{
			name:     "non-md extension not stripped",
			filename: "notes.txt",
			want:     []string{"notes.txt", "Notes.txt"},
		},
		{
			name:     "single word",
			filename: "pike.md",
			want:     []string{"pike", "Pike"},
		},
		{
			name:     "deduplication",
			filename: "alpha.md",
			want:     []string{"alpha", "Alpha"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Identity(tt.filename)
			if len(got) != len(tt.want) {
				t.Fatalf("Identity(%q) = %v (len %d), want %v (len %d)", tt.filename, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Identity(%q)[%d] = %q, want %q", tt.filename, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMatch(t *testing.T) {
	bobIDs := Identity("Bob Smith.md")

	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "wikilink in text",
			text: "@talk ask [[Bob Smith]] about X",
			want: true,
		},
		{
			name: "wikilink slug in tag value",
			text: "@delegated([[bob-smith]]) finish report",
			want: true,
		},
		{
			name: "plain text mention",
			text: "ask Bob Smith about the API @today",
			want: true,
		},
		{
			name: "wikilink with display name",
			text: "review [[bob-smith|Bob]] notes",
			want: true,
		},
		{
			name: "case insensitive match",
			text: "talk to BOB SMITH tomorrow",
			want: true,
		},
		{
			name: "no match",
			text: "talk to Alice about the project",
			want: false,
		},
		{
			name: "short name false positive is expected",
			text: "check the algorithm for errors",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &model.Task{Text: tt.text}
			got := Match(task, bobIDs)
			if got != tt.want {
				t.Errorf("Match(%q, bobIDs) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// TestMatchShortNameFalsePositive documents the known trade-off that
// short filenames will substring-match longer words.
func TestMatchShortNameFalsePositive(t *testing.T) {
	alIDs := Identity("Al.md")
	task := &model.Task{Text: "talk to Albert about the project"}
	if !Match(task, alIDs) {
		t.Error("expected short name 'Al' to match 'Albert' (known substring trade-off)")
	}
}

func TestRelPath(t *testing.T) {
	tests := []struct {
		name      string
		scopePath string
		notesDir  string
		want      string
		wantErr   bool
	}{
		{
			name:      "relative path within notes dir",
			scopePath: "people/Bob Smith.md",
			notesDir:  "/home/user/notes",
			want:      "people/Bob Smith.md",
		},
		{
			name:      "absolute path within notes dir",
			scopePath: "/home/user/notes/people/Bob Smith.md",
			notesDir:  "/home/user/notes",
			want:      "people/Bob Smith.md",
		},
		{
			name:      "file at root of notes dir",
			scopePath: "/home/user/notes/todo.md",
			notesDir:  "/home/user/notes",
			want:      "todo.md",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RelPath(tt.scopePath, tt.notesDir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RelPath(%q, %q) error = %v, wantErr %v", tt.scopePath, tt.notesDir, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("RelPath(%q, %q) = %q, want %q", tt.scopePath, tt.notesDir, got, tt.want)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	tasks := []model.Task{
		{Text: "talk to [[Bob Smith]] @talk", File: "projects/alpha.md", State: model.Open},
		{Text: "unrelated task @today", File: "projects/beta.md", State: model.Open},
		{Text: "@delegated([[bob-smith]]) do thing", File: "projects/gamma.md", State: model.Open},
		{Text: "Bob Smith's own task", File: "people/Bob Smith.md", State: model.Open},
	}

	ids := Identity("Bob Smith.md")
	got := Filter(tasks, ids, "people/Bob Smith.md")

	// Should include alpha (wikilink) and gamma (tag value), exclude beta (no ref) and self
	if len(got) != 2 {
		t.Fatalf("Filter() returned %d tasks, want 2: %v", len(got), got)
	}
	if got[0].File != "projects/alpha.md" {
		t.Errorf("got[0].File = %q, want %q", got[0].File, "projects/alpha.md")
	}
	if got[1].File != "projects/gamma.md" {
		t.Errorf("got[1].File = %q, want %q", got[1].File, "projects/gamma.md")
	}
}

func TestFilterEmpty(t *testing.T) {
	tasks := []model.Task{
		{Text: "unrelated task", File: "foo.md", State: model.Open},
	}
	ids := Identity("Bob Smith.md")
	got := Filter(tasks, ids, "people/Bob Smith.md")
	if len(got) != 0 {
		t.Fatalf("Filter() returned %d tasks, want 0", len(got))
	}
}
