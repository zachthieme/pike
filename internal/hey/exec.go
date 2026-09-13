package hey

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// weekDateLayout is HEY's date format for Week boundaries and the --date flag.
const weekDateLayout = "2006-01-02"

// ExecClient is the real [Client]: it shells out to the configured hey command
// with `todo <verb> ... --json --quiet`, reads stdout, and maps
// {"ok":false,...} envelopes to errors.
type ExecClient struct {
	command string
	account string
	// run executes the hey command with the given args and returns its stdout.
	// It is a field so tests can inject recorded outputs without a real binary.
	run func(ctx context.Context, args []string) ([]byte, error)
}

// NewExecClient returns an ExecClient that runs the given command, passing
// --account when account is non-empty.
func NewExecClient(command, account string) *ExecClient {
	c := &ExecClient{command: command, account: account}
	c.run = c.execRun
	return c
}

// execRun runs the configured command with args, returning combined stdout.
// stderr is not consulted: HEY reports everything pike needs on stdout.
func (c *ExecClient) execRun(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.command, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running %s: %w", c.command, err)
	}
	return stdout.Bytes(), nil
}

// args builds the argument list for a verb, appending --json --quiet and
// --account when configured.
func (c *ExecClient) args(verb string, rest ...string) []string {
	args := append([]string{"todo", verb}, rest...)
	args = append(args, "--json", "--quiet")
	if c.account != "" {
		args = append(args, "--account", c.account)
	}
	return args
}

// List returns every Todo across all Weeks.
func (c *ExecClient) List(ctx context.Context) ([]Todo, error) {
	out, err := c.run(ctx, c.args("list", "--all"))
	if err != nil {
		return nil, err
	}
	if err := checkEnvelope(out); err != nil {
		return nil, err
	}
	var raws []rawTodo
	if err := json.Unmarshal(out, &raws); err != nil {
		return nil, fmt.Errorf("parsing hey list output: %w", err)
	}
	todos := make([]Todo, 0, len(raws))
	for _, r := range raws {
		t, err := r.toTodo()
		if err != nil {
			return nil, err
		}
		todos = append(todos, t)
	}
	return todos, nil
}

// Add creates a Todo with the given title, filed under the Week containing date.
func (c *ExecClient) Add(ctx context.Context, title string, date *time.Time) (Todo, error) {
	rest := []string{title}
	if date != nil {
		rest = append(rest, "--date", date.Format(weekDateLayout))
	}
	out, err := c.run(ctx, c.args("add", rest...))
	if err != nil {
		return Todo{}, err
	}
	return parseTodo(out)
}

// Complete marks the Todo complete.
func (c *ExecClient) Complete(ctx context.Context, id string) error {
	return c.simpleVerb(ctx, "complete", id)
}

// Uncomplete reopens the Todo.
func (c *ExecClient) Uncomplete(ctx context.Context, id string) error {
	return c.simpleVerb(ctx, "uncomplete", id)
}

// Delete removes the Todo.
func (c *ExecClient) Delete(ctx context.Context, id string) error {
	return c.simpleVerb(ctx, "delete", id)
}

func (c *ExecClient) simpleVerb(ctx context.Context, verb, id string) error {
	out, err := c.run(ctx, c.args(verb, id))
	if err != nil {
		return err
	}
	return checkEnvelope(out)
}

// parseTodo parses a single created/returned Todo object.
func parseTodo(out []byte) (Todo, error) {
	if err := checkEnvelope(out); err != nil {
		return Todo{}, err
	}
	var r rawTodo
	if err := json.Unmarshal(out, &r); err != nil {
		return Todo{}, fmt.Errorf("parsing hey todo output: %w", err)
	}
	return r.toTodo()
}

// errorEnvelope is HEY's {"ok":false,...} failure shape. A successful call
// omits "ok" (or sets it true) and carries its payload directly.
type errorEnvelope struct {
	OK    *bool  `json:"ok"`
	Error string `json:"error"`
	Kind  string `json:"kind"`
}

// checkEnvelope inspects an object payload for an {"ok":false,...} error. Array
// payloads (list output) and success objects pass through untouched.
func checkEnvelope(out []byte) error {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil
	}
	var env errorEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		return nil // not an envelope we recognise; let the payload parser report it
	}
	if env.OK == nil || *env.OK {
		return nil
	}
	if env.Kind == "auth" {
		return fmt.Errorf("%w: %s", ErrUnauthenticated, env.Error)
	}
	return fmt.Errorf("hey: %s", env.Error)
}

// rawTodo mirrors HEY's JSON todo object.
type rawTodo struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	WeekStart   string  `json:"week_start"`
	WeekEnd     string  `json:"week_end"`
	CompletedAt *string `json:"completed_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func (r rawTodo) toTodo() (Todo, error) {
	weekStart, err := time.Parse(weekDateLayout, r.WeekStart)
	if err != nil {
		return Todo{}, fmt.Errorf("parsing week_start %q: %w", r.WeekStart, err)
	}
	weekEnd, err := time.Parse(weekDateLayout, r.WeekEnd)
	if err != nil {
		return Todo{}, fmt.Errorf("parsing week_end %q: %w", r.WeekEnd, err)
	}
	t := Todo{
		ID:        r.ID,
		Title:     r.Title,
		WeekStart: weekStart,
		WeekEnd:   weekEnd,
	}
	if r.UpdatedAt != "" {
		updated, err := time.Parse(time.RFC3339, r.UpdatedAt)
		if err != nil {
			return Todo{}, fmt.Errorf("parsing updated_at %q: %w", r.UpdatedAt, err)
		}
		t.Updated = updated
	}
	if r.CompletedAt != nil && *r.CompletedAt != "" {
		completed, err := time.Parse(time.RFC3339, *r.CompletedAt)
		if err != nil {
			return Todo{}, fmt.Errorf("parsing completed_at %q: %w", *r.CompletedAt, err)
		}
		t.Completed = &completed
	}
	return t, nil
}
