package hey

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
)

// weekDateLayout is HEY's date format for the --date flag.
const weekDateLayout = "2006-01-02"

// ExecClient is the real [Client]: it shells out to the configured hey command
// with `todo <verb> ... --json --quiet`, reads the bare data value from stdout,
// and maps a non-zero exit (with an {"ok":false,...} envelope on stderr) to an
// error.
type ExecClient struct {
	command string
	account string
	// run executes the hey command with the given args and returns its stdout.
	// On a non-zero exit it returns an [*execError] carrying the exit status and
	// stderr. It is a field so tests can inject recorded outputs without a real
	// binary.
	run func(ctx context.Context, args []string) ([]byte, error)
}

// NewExecClient returns an ExecClient that runs the given command, passing
// --account when account is non-empty.
func NewExecClient(command, account string) *ExecClient {
	c := &ExecClient{command: command, account: account}
	c.run = c.execRun
	return c
}

// execError is the failure of a hey invocation: a non-zero exit status and the
// stderr it wrote, which carries HEY's {"ok":false,...} error envelope.
type execError struct {
	exitCode int
	stderr   []byte
}

func (e *execError) Error() string {
	return fmt.Sprintf("hey exited with status %d", e.exitCode)
}

// execRun runs the configured command with args. On success it returns stdout;
// on a non-zero exit it returns an *execError with the exit status and stderr.
func (c *ExecClient) execRun(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, &execError{exitCode: ee.ExitCode(), stderr: stderr.Bytes()}
		}
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
		return nil, mapRunError(err)
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
		return Todo{}, mapRunError(err)
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

// simpleVerb runs a verb whose result payload pike does not need. Success is a
// zero exit status; stdout (a result object, null, or empty) is ignored.
func (c *ExecClient) simpleVerb(ctx context.Context, verb, id string) error {
	if _, err := c.run(ctx, c.args(verb, id)); err != nil {
		return mapRunError(err)
	}
	return nil
}

// parseTodo parses a single created/returned Todo object.
func parseTodo(out []byte) (Todo, error) {
	var r rawTodo
	if err := json.Unmarshal(out, &r); err != nil {
		return Todo{}, fmt.Errorf("parsing hey todo output: %w", err)
	}
	return r.toTodo()
}

// errorEnvelope is HEY's {"ok":false,...} failure shape, written to stderr with
// a non-zero exit status.
type errorEnvelope struct {
	OK    *bool  `json:"ok"`
	Error string `json:"error"`
	Code  string `json:"code"`
	Hint  string `json:"hint"`
}

// message renders HEY's error text, appending the hint when present.
func (e errorEnvelope) message() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s (%s)", e.Error, e.Hint)
	}
	return e.Error
}

// mapRunError turns a runner failure into a caller-facing error. A non-zero
// exit carrying an {"ok":false,...} envelope surfaces HEY's own message; an
// "auth" code (or a bare exit status 3) wraps [ErrUnauthenticated]. Failures
// that are not an *execError (e.g. the command not being found) pass through.
func mapRunError(err error) error {
	var ee *execError
	if !errors.As(err, &ee) {
		return err
	}
	if env, ok := parseErrorEnvelope(ee.stderr); ok {
		if env.Code == "auth" {
			return fmt.Errorf("%w: %s", ErrUnauthenticated, env.message())
		}
		return fmt.Errorf("hey: %s", env.message())
	}
	// No envelope: surface the exit status and stderr's text. Exit status 3
	// still means unauthenticated, but its diagnostic (a keyring failure, a
	// backtrace) must not be dropped. Fold it to a single bounded line so a
	// multi-line backtrace cannot inflate the TUI's fixed-height status footer.
	detail := ee.Error()
	if trimmed := oneLine(ee.stderr); trimmed != "" {
		detail = fmt.Sprintf("%s: %s", ee.Error(), trimmed)
	}
	// An envelope can sit past the scan cap and be dropped. Say so, so the
	// failure does not read as a clean no-envelope error when it is really a
	// truncated scan — the real diagnostic may be beyond what we looked at.
	if len(ee.stderr) > maxEnvelopeScan {
		detail += " (stderr too large to scan for an error envelope)"
	}
	if ee.exitCode == 3 {
		return fmt.Errorf("%w: %s", ErrUnauthenticated, detail)
	}
	return fmt.Errorf("hey: %s", detail)
}

// maxEnvelopeScan bounds how much of stderr parseErrorEnvelope inspects. A real
// hey 1.4.1 envelope (with any warning noise around it) is a few hundred bytes;
// capping the scan keeps a pathological stderr — megabytes of '{' or deeply
// nested JSON — from turning the per-brace decode into quadratic work.
const maxEnvelopeScan = 64 << 10

// maxEnvelopeDepth bounds the JSON nesting parseErrorEnvelope will decode at any
// one brace. A genuine hey envelope is a flat object of scalar fields (depth 1)
// and warning noise is shallow, so this is generous; its purpose is to keep a
// pathologically nested stderr from turning the per-brace retry into work
// quadratic in nesting depth, which would stall a Sync.
const maxEnvelopeDepth = 32

// maxStderrDetail bounds, in display columns, the stderr text folded into a
// fallback error. Measuring by display width (East Asian wide characters count
// as two columns) rather than rune count means the bound holds a real one-line
// hey diagnostic yet keeps a backtrace or a runaway stderr from overflowing the
// TUI's fixed-height status line for any script, ASCII or CJK.
const maxStderrDetail = 200

// oneLine collapses stderr into a single bounded line: leading/trailing space is
// dropped and every internal whitespace run (including newlines) becomes one
// space, then the result is truncated with an ellipsis past maxStderrDetail
// display columns. The empty string means stderr held nothing printable.
func oneLine(stderr []byte) string {
	s := strings.Join(strings.Fields(string(stderr)), " ")
	return runewidth.Truncate(s, maxStderrDetail, "…")
}

// parseErrorEnvelope finds HEY's JSON error envelope anywhere in stderr, which
// may be single-line or pretty-printed across several lines (as hey-cli 1.4.1
// does) and surrounded by unrelated warning noise — plain text, braces embedded
// in prose (`warning: token {abc} rejected`), or whole JSON warning objects. It
// scans each '{' in turn and decodes the JSON value there, accepting the first
// that is a genuine error envelope: an object carrying ok:false (or ok with a
// non-empty error). Braces that do not begin a decodable object, objects
// without `ok`, and success objects (ok:true with no error) are skipped, so a
// success log line does not shadow the real failure.
func parseErrorEnvelope(stderr []byte) (errorEnvelope, bool) {
	if len(stderr) > maxEnvelopeScan {
		stderr = stderr[:maxEnvelopeScan]
	}
	for i := 0; i < len(stderr); i++ {
		if stderr[i] != '{' {
			continue
		}
		// Skip a value that nests deeper than a real envelope ever would before
		// handing it to Decode: the guard is O(maxEnvelopeDepth) per brace, so
		// the whole scan stays linear even against deeply nested JSON.
		if exceedsDepth(stderr[i:], maxEnvelopeDepth) {
			continue
		}
		var env errorEnvelope
		if err := json.NewDecoder(bytes.NewReader(stderr[i:])).Decode(&env); err != nil {
			continue
		}
		if env.OK != nil && (!*env.OK || env.Error != "") {
			return env, true
		}
	}
	return errorEnvelope{}, false
}

// exceedsDepth reports whether the JSON value beginning at data opens more than
// limit nested objects/arrays before closing. It scans only far enough to reach
// that point (or the first value's close), ignoring braces inside strings, so it
// is a cheap guard rather than a full parse.
func exceedsDepth(data []byte, limit int) bool {
	depth := 0
	inStr := false
	esc := false
	for _, c := range data {
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{', '[':
			depth++
			if depth > limit {
				return true
			}
		case '}', ']':
			depth--
			if depth <= 0 {
				return false // first value closed within the limit
			}
		}
	}
	return false
}

// rawTodo mirrors HEY's JSON todo object. Only the fields pike needs are kept.
type rawTodo struct {
	ID          json.Number `json:"id"`
	Title       string      `json:"title"`
	StartsAt    string      `json:"starts_at"`
	EndsAt      string      `json:"ends_at"`
	CompletedAt *string     `json:"completed_at"`
	UpdatedAt   string      `json:"updated_at"`
}

func (r rawTodo) toTodo() (Todo, error) {
	weekStart, err := parseWeekDate(r.StartsAt)
	if err != nil {
		return Todo{}, fmt.Errorf("parsing starts_at %q: %w", r.StartsAt, err)
	}
	weekEnd, err := parseWeekDate(r.EndsAt)
	if err != nil {
		return Todo{}, fmt.Errorf("parsing ends_at %q: %w", r.EndsAt, err)
	}
	t := Todo{
		ID:        r.ID.String(),
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

// parseWeekDate reads an RFC3339 Week boundary (UTC midnight) and keeps its
// calendar date verbatim, with no zone conversion.
func parseWeekDate(s string) (time.Time, error) {
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, err
	}
	ts = ts.UTC()
	return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC), nil
}
