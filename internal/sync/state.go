package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// stateVersion is the schema version written to new state files.
const stateVersion = 1

// Link is pike's persisted record of one Task↔Todo association, captured at the
// last Sync. It remembers enough about the Todo to detect later changes (Title
// and Week) and about the Task to find it again (file and line).
type Link struct {
	Title     string    `json:"title"`      // the Todo's Title at sync time (Tags stripped)
	WeekStart time.Time `json:"week_start"` // Sunday the Todo's Week begins
	Completed bool      `json:"completed"`  // whether the Todo was completed
	Updated   time.Time `json:"updated_at"` // HEY's updated_at for the Todo
	File      string    `json:"file"`       // notes-relative file of the linked Task
	Line      int       `json:"line"`       // 1-based line hint for the Task
	// PendingDelete marks a Todo pike created or replaced this run but could not
	// delete: a push whose @hey tag write was refused after the Add (unlinked by
	// pike), or a Re-create whose delete failed (the old Todo, superseded by the
	// new one). Such an entry is never imported and is skipped by every
	// reconciliation pass; each later Sync retries the delete and drops the entry
	// once the Todo is gone from HEY or completed.
	PendingDelete bool `json:"pending_delete,omitempty"`
	// Orphaned marks a Link whose Task line can no longer be found while its Todo
	// survives in HEY. It is set the first time a Sync reports the Orphan Warning,
	// so later Syncs still count the Orphan but do not repeat the Warning. The
	// entry is dropped once the Todo is gone from HEY or completed.
	Orphaned bool `json:"orphaned,omitempty"`
}

// State is pike's record of a prior Sync, persisted between runs. Links is keyed
// by HEY todo id.
type State struct {
	Version int             `json:"version"`
	Links   map[string]Link `json:"links"`
}

// snapshotLinks returns a shallow copy of the state's Links map, capturing each
// Link as it stands before reconciliation mutates the live state. Links are
// value types, so copying the map is enough to freeze the recorded fields.
func snapshotLinks(st *State) map[string]Link {
	out := make(map[string]Link, len(st.Links))
	for id, link := range st.Links {
		out[id] = link
	}
	return out
}

// LoadState reads the sync state file. An absent file (or an empty path)
// yields an empty State and no error; a present but unreadable or malformed
// file is an error the caller reports as a Warning.
func LoadState(path string) (*State, error) {
	if path == "" {
		return &State{Links: make(map[string]Link)}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{Links: make(map[string]Link)}, nil
		}
		return nil, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if st.Links == nil {
		st.Links = make(map[string]Link)
	}
	return &st, nil
}

// SaveState writes the state file atomically (write-to-temp + rename). An empty
// path is a no-op.
func SaveState(path string, st *State) error {
	if path == "" {
		return nil
	}
	if st.Version == 0 {
		st.Version = stateVersion
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding hey state: %w", err)
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	// Create the parent directory when missing, so the default state path under
	// ~/.local/share/pike works on a fresh machine.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating state directory %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".pike-hey-state-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()        //nolint:errcheck // cleaning up on error
		os.Remove(tmpPath) //nolint:errcheck // cleaning up on error
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath) //nolint:errcheck // cleaning up on error
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) //nolint:errcheck // cleaning up on error; rename error takes precedence
		return err
	}
	return nil
}
