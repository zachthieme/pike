package editor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ResolveEditor returns the editor to use. The config layer is expected to
// have already resolved $EDITOR and the "hx" default, so this simply
// returns editorCmd when non-empty, falling back to "hx" only as a
// last-resort safety net.
func ResolveEditor(editorCmd string) string {
	if editorCmd != "" {
		return editorCmd
	}
	return "hx"
}

// Command builds an *exec.Cmd that opens the given file at the specified
// line in the named editor. The editor string is split on whitespace so
// that values like "code --wait" work correctly (the first token is the
// executable, the rest are prefix arguments). The executable's base name
// determines the command-line syntax for file/line arguments:
//
//   - hx, helix:       editor file:line
//   - nvim, vim, vi:   editor +line file
//   - code:            editor --goto file:line
//   - anything else:   editor file
func Command(editor, file string, line int) *exec.Cmd {
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		parts = []string{editor}
	}
	executable := parts[0]
	prefixArgs := parts[1:]

	base := filepath.Base(executable)

	var fileArgs []string
	switch base {
	case "hx", "helix":
		fileArgs = []string{fmt.Sprintf("%s:%d", file, line)}
	case "nvim", "vim", "vi":
		fileArgs = []string{fmt.Sprintf("+%d", line), file}
	case "code":
		fileArgs = []string{"--goto", fmt.Sprintf("%s:%d", file, line)}
	default:
		fileArgs = []string{file}
	}

	allArgs := append(prefixArgs, fileArgs...)
	cmd := exec.Command(executable, allArgs...)
	return cmd
}
