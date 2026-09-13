package hey

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixtureRunner returns a runner that yields the named testdata fixture,
// recording the args it was invoked with. No real hey binary is ever run.
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
	if open.ID != "todo_01HZOPEN" || open.Title != "Buy groceries" {
		t.Errorf("open todo = %+v", open)
	}
	if open.Completed != nil {
		t.Errorf("open todo should have nil Completed, got %v", open.Completed)
	}
	// Week-snapped date shape: Sunday-to-Saturday span.
	wantStart := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	if !open.WeekStart.Equal(wantStart) {
		t.Errorf("WeekStart = %v, want %v", open.WeekStart, wantStart)
	}
	if !open.WeekEnd.Equal(wantEnd) {
		t.Errorf("WeekEnd = %v, want %v", open.WeekEnd, wantEnd)
	}

	done := todos[1]
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
	run, _ := fixtureRunner(t, "error_generic.json")
	c := &ExecClient{command: "hey", run: run}

	_, err := c.List(context.Background())
	if err == nil {
		t.Fatal("expected an error from an ok:false envelope")
	}
	if errors.Is(err, ErrUnauthenticated) {
		t.Errorf("generic error should not be an auth error: %v", err)
	}
}

func TestExecList_AuthErrorEnvelope(t *testing.T) {
	run, _ := fixtureRunner(t, "error_auth.json")
	c := &ExecClient{command: "hey", run: run}

	_, err := c.List(context.Background())
	if err == nil {
		t.Fatal("expected an error from an ok:false auth envelope")
	}
	if !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("expected ErrUnauthenticated, got %v", err)
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
	if todo.ID != "todo_01HZNEW" || todo.Title != "Ship release" {
		t.Errorf("todo = %+v", todo)
	}
	assertArgsContain(t, *gotArgs, "todo", "add", "Ship release", "--json", "--quiet")
	assertArgsContain(t, *gotArgs, "--account", "work")
	assertArgsContain(t, *gotArgs, "--date", "2026-09-15")
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
