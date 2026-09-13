// Package hey talks to the HEY calendar's todo CLI, exposing Todos and a
// Client for listing, creating, completing, and deleting them.
package hey

import (
	"context"
	"errors"
	"time"
)

// ErrUnauthenticated is returned (wrapped) when HEY reports that the request
// was not authenticated — a wrong account or an expired session. Callers use
// errors.Is to decide whether a Sync should abort with a non-zero status.
var ErrUnauthenticated = errors.New("hey: not authenticated")

// Todo is a single to-do item in HEY's calendar. It has no tags, no hierarchy,
// and no day-level date: HEY snaps whatever date it is given to the Week
// (Sunday-to-Saturday) that contains it.
type Todo struct {
	ID        string     // HEY's stable identifier
	Title     string     // the to-do text
	WeekStart time.Time  // Sunday the Week begins
	WeekEnd   time.Time  // Saturday the Week ends
	Completed *time.Time // instant it was completed, nil if still open
	Updated   time.Time  // instant it was last changed
}

// Client is the set of operations pike performs against HEY. The one real
// implementation is [ExecClient]; tests use fakes.
type Client interface {
	// List returns every Todo across all Weeks.
	List(ctx context.Context) ([]Todo, error)
	// Add creates a new Todo with the given title, optionally filed under the
	// Week containing date (nil files it under the current Week).
	Add(ctx context.Context, title string, date *time.Time) (Todo, error)
	// Complete marks the Todo with the given id complete.
	Complete(ctx context.Context, id string) error
	// Uncomplete reopens the Todo with the given id.
	Uncomplete(ctx context.Context, id string) error
	// Delete removes the Todo with the given id.
	Delete(ctx context.Context, id string) error
}
