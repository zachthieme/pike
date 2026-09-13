package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// Report is the outcome of a planning pass: how many Eligible Tasks would be
// pushed to HEY, how many unlinked open Todos would be imported, and how many
// Links already exist.
type Report struct {
	DryRun        bool `json:"dry_run"`
	WouldPush     int  `json:"would_push"`
	WouldImport   int  `json:"would_import"`
	ExistingLinks int  `json:"existing_links"`
}

// WriteText renders the Report as human-readable lines.
func (r *Report) WriteText(w io.Writer) error {
	mode := "Sync"
	if r.DryRun {
		mode = "Sync (dry run — nothing written)"
	}
	_, err := fmt.Fprintf(w,
		"%s\n  %d task(s) would push to HEY\n  %d todo(s) would import to the inbox\n  %d link(s) already exist\n",
		mode, r.WouldPush, r.WouldImport, r.ExistingLinks)
	return err
}

// WriteJSON renders the Report as indented JSON.
func (r *Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// State is pike's record of a prior Sync, persisted between runs.
type State struct {
	Version int               `json:"version"`
	Links   map[string]string `json:"links"` // HEY todo id -> Task key (file:line)
}

// LoadState reads the sync state file. An absent file (or an empty path)
// yields an empty State and no error; a present but unreadable or malformed
// file is an error the caller reports as a Warning.
func LoadState(path string) (*State, error) {
	if path == "" {
		return &State{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{}, nil
		}
		return nil, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if st.Links == nil {
		st.Links = make(map[string]string)
	}
	return &st, nil
}
