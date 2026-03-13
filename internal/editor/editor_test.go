package editor

import (
	"fmt"
	"testing"
)

func TestCommand(t *testing.T) {
	tests := []struct {
		name     string
		editor   string
		file     string
		line     int
		wantPath string
		wantArgs []string
	}{
		{
			name:     "hx uses file:line format",
			editor:   "hx",
			file:     "todo.md",
			line:     42,
			wantPath: "hx",
			wantArgs: []string{"hx", "todo.md:42"},
		},
		{
			name:     "helix uses file:line format",
			editor:   "helix",
			file:     "todo.md",
			line:     10,
			wantPath: "helix",
			wantArgs: []string{"helix", "todo.md:10"},
		},
		{
			name:     "nvim uses +line file format",
			editor:   "nvim",
			file:     "tasks.md",
			line:     7,
			wantPath: "nvim",
			wantArgs: []string{"nvim", "+7", "tasks.md"},
		},
		{
			name:     "vim uses +line file format",
			editor:   "vim",
			file:     "tasks.md",
			line:     3,
			wantPath: "vim",
			wantArgs: []string{"vim", "+3", "tasks.md"},
		},
		{
			name:     "vi uses +line file format",
			editor:   "vi",
			file:     "tasks.md",
			line:     1,
			wantPath: "vi",
			wantArgs: []string{"vi", "+1", "tasks.md"},
		},
		{
			name:     "code uses --goto file:line format",
			editor:   "code",
			file:     "notes.md",
			line:     99,
			wantPath: "code",
			wantArgs: []string{"code", "--goto", "notes.md:99"},
		},
		{
			name:     "unknown editor falls back to file only",
			editor:   "nano",
			file:     "readme.md",
			line:     5,
			wantPath: "nano",
			wantArgs: []string{"nano", "readme.md"},
		},
		{
			name:     "full path editor resolved by base name",
			editor:   "/usr/bin/nvim",
			file:     "todo.md",
			line:     20,
			wantPath: "/usr/bin/nvim",
			wantArgs: []string{"/usr/bin/nvim", "+20", "todo.md"},
		},
		{
			name:     "full path to hx resolved by base name",
			editor:   "/usr/local/bin/hx",
			file:     "todo.md",
			line:     15,
			wantPath: "/usr/local/bin/hx",
			wantArgs: []string{"/usr/local/bin/hx", "todo.md:15"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := Command(tt.editor, tt.file, tt.line)

			// cmd.Args[0] is the editor command as given (not resolved via LookPath)
			if len(cmd.Args) != len(tt.wantArgs) {
				t.Fatalf("Args = %v (len %d), want %v (len %d)", cmd.Args, len(cmd.Args), tt.wantArgs, len(tt.wantArgs))
			}

			for i := range tt.wantArgs {
				if cmd.Args[i] != tt.wantArgs[i] {
					t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], tt.wantArgs[i])
				}
			}
		})
	}
}

func TestResolveEditor(t *testing.T) {
	tests := []struct {
		name    string
		editor  string
		envVar  string
		want    string
	}{
		{
			name:   "explicit editor takes precedence",
			editor: "nvim",
			envVar: "code",
			want:   "nvim",
		},
		{
			name:   "falls back to EDITOR env var",
			editor: "",
			envVar: "vim",
			want:   "vim",
		},
		{
			name:   "falls back to hx when both empty",
			editor: "",
			envVar: "",
			want:   "hx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("EDITOR", tt.envVar)

			got := ResolveEditor(tt.editor)
			if got != tt.want {
				t.Errorf("ResolveEditor(%q) = %q, want %q (EDITOR=%q)", tt.editor, got, tt.want, tt.envVar)
			}
		})
	}
}

func TestCommandIntegrationWithResolveEditor(t *testing.T) {
	t.Setenv("EDITOR", "")

	editor := ResolveEditor("")
	cmd := Command(editor, "tasks.md", 5)

	wantArgs := []string{"hx", fmt.Sprintf("tasks.md:%d", 5)}

	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("Args = %v, want %v", cmd.Args, wantArgs)
	}

	for i := range wantArgs {
		if cmd.Args[i] != wantArgs[i] {
			t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], wantArgs[i])
		}
	}
}
