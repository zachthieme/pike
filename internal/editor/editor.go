package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ResolveEditor returns the editor to use. It checks the explicit editor
// parameter first, then the $EDITOR environment variable, and falls back
// to "hx" if both are empty.
func ResolveEditor(editor string) string {
	if editor != "" {
		return editor
	}
	if env := os.Getenv("EDITOR"); env != "" {
		return env
	}
	return "hx"
}

// Command builds an *exec.Cmd that opens the given file at the specified
// line in the named editor. The editor's base name determines the
// command-line syntax:
//
//   - hx, helix:       editor file:line
//   - nvim, vim, vi:   editor +line file
//   - code:            editor --goto file:line
//   - anything else:   editor file
func Command(editor, file string, line int) *exec.Cmd {
	base := filepath.Base(editor)

	var args []string
	switch base {
	case "hx", "helix":
		args = []string{fmt.Sprintf("%s:%d", file, line)}
	case "nvim", "vim", "vi":
		args = []string{fmt.Sprintf("+%d", line), file}
	case "code":
		args = []string{"--goto", fmt.Sprintf("%s:%d", file, line)}
	default:
		args = []string{file}
	}

	cmd := exec.Command(editor, args...)
	return cmd
}
