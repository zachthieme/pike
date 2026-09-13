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
}

// State is pike's record of a prior Sync, persisted between runs. Links is keyed
// by HEY todo id.
type State struct {
	Version int             `json:"version"`
	Links   map[string]Link `json:"links"`
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
