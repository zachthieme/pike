package hey

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixtureRunner returns a runner that yields the named testdata fixture on
// stdout, recording the args it was invoked with. No real hey binary is ever
// run.
func fixtureRunner(t *testing.T, fixture string) (func(ctx context.Context, args []string) ([]byte, error), *[]string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	var gotArgs []string
	run := func(_ context.Context, args []string) ([]byte, error) {
		gotArgs = args
		return data, nil
	}
	return run, &gotArgs
}

// errorRunner returns a runner that fails like the real hey CLI: nothing on
// stdout, the named fixture's bytes as stderr, and the given exit status.
func errorRunner(t *testing.T, fixture string, exitCode int) func(ctx context.Context, args []string) ([]byte, error) {
	t.Helper()
	stderr, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return func(_ context.Context, _ []string) ([]byte, error) {
		return nil, &execError{exitCode: exitCode, stderr: stderr}
	}
}

func TestExecList_OpenAndCompletedTodos(t *testing.T) {
	run, gotArgs := fixtureRunner(t, "list.json")
	c := &ExecClient{command: "hey", run: run}

	todos, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(todos) != 2 {
		t.Fatalf("got %d todos, want 2", len(todos))
	}

	open := todos[0]
	// The numeric id is kept as its decimal string form.
	if open.ID != "133760954" || open.Title != "Buy groceries" {
		t.Errorf("open todo = %+v", open)
	}
	if open.Completed != nil {
		t.Errorf("open todo should have nil Completed, got %v", open.Completed)
	}
	// Week is starts_at (Sunday) to ends_at (Saturday), calendar date verbatim.
	wantStart := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	if !open.WeekStart.Equal(wantStart) {
		t.Errorf("WeekStart = %v, want %v", open.WeekStart, wantStart)
	}
	if !open.WeekEnd.Equal(wantEnd) {
		t.Errorf("WeekEnd = %v, want %v", open.WeekEnd, wantEnd)
	}
	// updated_at is RFC3339 with fractional seconds.
	wantUpdated := time.Date(2026, 9, 13, 8, 0, 0, 123456000, time.UTC)
	if !open.Updated.Equal(wantUpdated) {
		t.Errorf("Updated = %v, want %v", open.Updated, wantUpdated)
	}

	done := todos[1]
	if done.ID != "133760955" {
		t.Errorf("completed todo id = %q, want %q", done.ID, "133760955")
	}
	if done.Completed == nil {
		t.Fatal("completed todo should have a non-nil Completed instant")
	}
	wantDone := time.Date(2026, 9, 10, 14, 30, 0, 0, time.UTC)
	if !done.Completed.Equal(wantDone) {
		t.Errorf("Completed = %v, want %v", done.Completed, wantDone)
	}

	// list uses `todo list --all --json --quiet`.
	assertArgsContain(t, *gotArgs, "todo", "list", "--all", "--json", "--quiet")
}

func TestExecList_ErrorEnvelope(t *testing.T) {
	// A not_found error: written to stderr after a warning line, exit status 2.
	c := &ExecClient{command: "hey", run: errorRunner(t, "error_generic.json", 2)}

	_, err := c.List(context.Background())
	if err == nil {
		t.Fatal("expected an error from an ok:false envelope")
	}
	if errors.Is(err, ErrUnauthenticated) {
		t.Errorf("generic error should not be an auth error: %v", err)
	}
	if !strings.Contains(err.Error(), "todo not found") {
		t.Errorf("error should carry HEY's message, got: %v", err)
	}
}

func TestExecList_AuthErrorEnvelope(t *testing.T) {
	// An auth error: written to stderr after a warning line, exit status 3.
	c := &ExecClient{command: "hey", run: errorRunner(t, "error_auth.json", 3)}

	_, err := c.List(context.Background())
	if err == nil {
		t.Fatal("expected an error from an ok:false auth envelope")
	}
	if !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("expected ErrUnauthenticated, got %v", err)
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("error should carry HEY's message, got: %v", err)
	}
}

func TestExecList_AuthExitNoEnvelope(t *testing.T) {
	// Exit status 3 with no parseable envelope on stderr still means unauth.
	run := func(_ context.Context, _ []string) ([]byte, error) {
		return nil, &execError{exitCode: 3, stderr: []byte("keyring: no backend\n")}
	}
	c := &ExecClient{command: "hey", run: run}

	_, err := c.List(context.Background())
	if !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("exit 3 without an envelope should be unauth, got %v", err)
	}
}

func TestExecAdd_ParsesCreatedTodo(t *testing.T) {
	run, gotArgs := fixtureRunner(t, "add.json")
	c := &ExecClient{command: "hey", account: "work", run: run}

	date := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	todo, err := c.Add(context.Background(), "Ship release", &date)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	// The created todo's numeric id comes back as a string.
	if todo.ID != "133760999" || todo.Title != "Ship release" {
		t.Errorf("todo = %+v", todo)
	}
	assertArgsContain(t, *gotArgs, "todo", "add", "Ship release", "--json", "--quiet")
	assertArgsContain(t, *gotArgs, "--account", "work")
	assertArgsContain(t, *gotArgs, "--date", "2026-09-15")
}

func TestExecDelete_NullAndEmptyStdout(t *testing.T) {
	for name, out := range map[string][]byte{
		"null":  []byte("null\n"),
		"empty": {},
	} {
		t.Run(name, func(t *testing.T) {
			run := func(_ context.Context, _ []string) ([]byte, error) { return out, nil }
			c := &ExecClient{command: "hey", run: run}
			if err := c.Delete(context.Background(), "133760954"); err != nil {
				t.Errorf("Delete with %s stdout should succeed, got %v", name, err)
			}
		})
	}
}

func assertArgsContain(t *testing.T, args []string, want ...string) {
	t.Helper()
	for _, w := range want {
		found := false
		for _, a := range args {
			if a == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("args %v missing %q", args, w)
		}
	}
}
