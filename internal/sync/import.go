// This file carries the import half of a real Sync: it appends an Inbox line
// for every open Todo in HEY that no Task is Linked to, turning a to-do added on
// the phone into a Linked checkbox Task the next Sync leaves alone.
package sync

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/zachthieme/pike/internal/hey"
	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/toggle"
)

// defaultInboxFile is the Inbox file used when Options.InboxFile is unset. It
// matches the TUI's capture default.
const defaultInboxFile = "inbox.md"

// importTodos appends an Inbox line for every unlinked open Todo, in HEY's list
// order, and records each new Link in state. Completed Todos and Todos already
// Linked from the notes (their id in linkedIDs) are skipped. On a dry run it
// counts would-imports and writes nothing. Per-Todo failures are collected as
// Warnings and do not stop the run.
func importTodos(ctx context.Context, opts Options, todos []hey.Todo, linkedIDs map[string]bool, state *State, rep *Report) []model.Warning {
	var warnings []model.Warning
	for _, td := range todos {
		// A Todo pike already holds a Link for is not an unlinked Todo: skip it
		// even when its Task line is gone from the notes (an Orphan), so a Sync
		// leaves the orphaned Todo alone rather than re-importing it.
		if _, known := state.Links[td.ID]; td.Completed != nil || linkedIDs[td.ID] || known {
			continue
		}
		if opts.DryRun {
			rep.WouldImport++
			continue
		}
		if w := importTodo(ctx, opts, td, state, rep); w != nil {
			warnings = append(warnings, *w)
		}
	}
	return warnings
}

// importTodo appends one Todo's Inbox line through the shared append path and
// records its Link, counting the outcome on rep. It returns a non-nil Warning
// when the append fails; a failing import does not stop the run.
func importTodo(ctx context.Context, opts Options, td hey.Todo, state *State, rep *Report) *model.Warning {
	inbox := inboxFile(opts)
	path := filepath.Join(opts.NotesDir, inbox)
	if err := toggle.AppendTask(ctx, path, importText(td)); err != nil {
		rep.Failed++
		return &model.Warning{File: inbox, Message: fmt.Sprintf("importing %q: %v", td.Title, err)}
	}
	state.Links[td.ID] = Link{
		Title:     td.Title,
		WeekStart: td.WeekStart,
		Completed: false,
		Updated:   td.Updated,
		File:      inbox,
	}
	rep.Imported++
	return nil
}

// importText is the Inbox line body for a Todo, without the "- [ ] " checkbox
// prefix the append path adds: the encoded Title, an @due tag on the Week's
// Saturday, and an @hey Link tag. The date is HEY's WeekEnd taken verbatim, no
// zone conversion. The Title is normalised to a single line first — the same
// normalising the retitle path applies — so a newline, carriage return or tab in
// the HEY Title can never split the import into a second, unlinked checkbox line
// or inject a heading or a second @hey tag. It is then run through encodeTitle so
// a HEY Title containing "@" text (an email address, a "@rent", an "@due(...)")
// does not plant a stray tag on the line and trigger a spurious Re-create next
// Sync.
func importText(td hey.Todo) string {
	return fmt.Sprintf("%s @due(%s) @hey(%s)", encodeTitle(normalizeTitle(td.Title)), td.WeekEnd.Format("2006-01-02"), td.ID)
}

// heyTagBreak is a zero-width space (U+200B) pike inserts between an "@" and the
// word character that follows it when importing a HEY Title, so the parser reads
// no tag there. ParseLine's tag grammar is an "@" immediately followed by a word
// character; a zero-width space breaks that adjacency without changing how the
// Title reads, and is not itself whitespace to strings.Fields. titleOf strips it
// back out, so the Title pike computes from the imported line equals the HEY
// Title verbatim and the next Sync sees no Title change. This encoding is chosen
// so ParseLine's grammar is left untouched.
const heyTagBreak = "\u200b"

// encodeAtRe matches an "@" that encodeTitle must break: one followed either by
// a run of pre-existing zero-width breaks (the user's own U+200B pasted right
// after an "@") or by a single word character. encodeTitle prepends one break to
// each match, which both stops the parser reading a tag from "@word" and escapes
// a pre-existing "@​" by doubling it — decodeTitle then removes exactly one
// break after every "@", restoring the original either way.
var encodeAtRe = regexp.MustCompile(`@(` + heyTagBreak + `+|\w)`)

// decodeAtBreakRe matches an "@" immediately followed by one zero-width break.
// Because ReplaceAllString consumes non-overlapping matches left to right, it
// strips exactly one break per "@" — undoing encodeTitle's single prepended
// break and leaving any further breaks (the user's own) in place.
var decodeAtBreakRe = regexp.MustCompile(`@` + heyTagBreak)

// encodeTitle renders a HEY Title safe to write on a Task line and is losslessly
// reversible by decodeTitle. It prepends one zero-width break to every "@" that
// is followed by a word character or by the user's own break(s): the first stops
// the parser reading a tag out of the Title, the second escapes a pre-existing
// break so decodeTitle can tell it apart from one pike inserted. Every other "@"
// (a trailing "@", "bob@example.com" mid-word is a word char so it is broken) is
// left untouched.
func encodeTitle(title string) string {
	return encodeAtRe.ReplaceAllString(title, "@"+heyTagBreak+"$1")
}

// decodeTitle is the exact inverse of encodeTitle: decodeTitle(encodeTitle(s)) ==
// s for every input. It removes exactly one zero-width break immediately after
// each "@" — the single break encodeTitle prepends — so an "@word" tag pike broke
// is restored and a pre-existing "@​" that encodeTitle doubled is returned
// to one break. A U+200B the user's Title carries anywhere else, or a lone "@"
// pike never touched, is left as is, so every human-written Task decodes to
// itself and no round trip triggers a spurious Re-create or rename.
func decodeTitle(s string) string {
	if !strings.Contains(s, heyTagBreak) {
		return s
	}
	return decodeAtBreakRe.ReplaceAllString(s, "@")
}

// inboxFile resolves the Inbox file, defaulting to inbox.md when unconfigured.
func inboxFile(opts Options) string {
	if opts.InboxFile == "" {
		return defaultInboxFile
	}
	return opts.InboxFile
}
