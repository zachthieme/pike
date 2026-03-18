// Package scope resolves file identities and matches tasks by reference.
package scope

import (
	"fmt"
	"path/filepath"
	"strings"

	"pike/internal/model"
)

// Identity returns match targets derived from a markdown filename.
// The input may be a bare filename or a relative/absolute path;
// only the base name (minus .md extension) is used.
func Identity(filename string) []string {
	// Strip directory, get base name.
	base := filepath.Base(filename)

	// Strip .md extension only.
	base = strings.TrimSuffix(base, ".md")

	seen := make(map[string]bool)
	var result []string
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	// Original form (as-is from filename).
	add(base)

	// If base contains spaces, generate slug (spaces → hyphens, lowercased) and lowercase.
	if strings.Contains(base, " ") {
		add(strings.ToLower(strings.ReplaceAll(base, " ", "-")))
		add(strings.ToLower(base))
	}

	// If base contains hyphens, generate title case (hyphens → spaces, capitalized) and lowercase.
	if strings.Contains(base, "-") {
		words := strings.Split(base, "-")
		for i, w := range words {
			if len(w) > 0 {
				words[i] = strings.ToUpper(w[:1]) + w[1:]
			}
		}
		add(strings.Join(words, " "))
		add(strings.ToLower(strings.ReplaceAll(base, "-", " ")))
	}

	// Lowercase variant.
	add(strings.ToLower(base))

	// Title case for single words (capitalize first letter).
	if !strings.Contains(base, " ") && !strings.Contains(base, "-") {
		if len(base) > 0 {
			add(strings.ToUpper(base[:1]) + base[1:])
		}
	}

	return result
}

// RelPath resolves a scope file path to a forward-slash-normalized path
// relative to notesDir, matching the format used by scanner for task.File.
// If scopePath is already relative, it is returned as-is (normalized to forward slashes).
// If scopePath is absolute, it is made relative to notesDir.
func RelPath(scopePath, notesDir string) (string, error) {
	if !filepath.IsAbs(scopePath) {
		return filepath.ToSlash(scopePath), nil
	}
	rel, err := filepath.Rel(notesDir, scopePath)
	if err != nil {
		return "", fmt.Errorf("cannot resolve %q relative to %q: %w", scopePath, notesDir, err)
	}
	return filepath.ToSlash(rel), nil
}

// Filter returns tasks that reference any of the given identities,
// excluding tasks from the scoped file itself.
// excludeRelPath is the forward-slash-normalized relative path of the scoped file.
func Filter(tasks []model.Task, identities []string, excludeRelPath string) []model.Task {
	// Pre-lower identities once to avoid per-task allocations in match.
	lowered := make([]string, len(identities))
	for i, id := range identities {
		lowered[i] = strings.ToLower(id)
	}

	var result []model.Task
	for i := range tasks {
		if tasks[i].File == excludeRelPath {
			continue
		}
		if match(&tasks[i], lowered) {
			result = append(result, tasks[i])
		}
	}
	return result
}

// Match returns true if any identity variant appears in task.Text
// (case-insensitive substring match).
func Match(task *model.Task, identities []string) bool {
	lowered := make([]string, len(identities))
	for i, id := range identities {
		lowered[i] = strings.ToLower(id)
	}
	return match(task, lowered)
}

// match is the inner matching function that expects pre-lowered identities.
func match(task *model.Task, loweredIdentities []string) bool {
	lower := strings.ToLower(task.Text)
	for _, id := range loweredIdentities {
		if strings.Contains(lower, id) {
			return true
		}
	}
	return false
}
