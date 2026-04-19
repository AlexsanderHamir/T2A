package cursor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

const cursorLogCmd = "taskapi"

// Default identity values; override via Options when wiring into a binary.
const (
	defaultName       = "cursor-cli"
	defaultVersion    = "0.0.0-unknown"
	defaultBinaryPath = "cursor-agent"
)

// stderrTailBytes caps the slice of stderr embedded in Result.Details on
// non-zero exit. Kept well below runner.MaxResultDetailsBytes so the
// final Details payload comfortably fits even after JSON wrapping.
const stderrTailBytes = 8 * 1024

// ExecFn is the seam unit tests use to avoid shelling out. It receives
// everything the adapter would pass to os/exec and returns the captured
// stdout, stderr, exit code, and error. A nil error with a non-zero
// exitCode means the process ran to completion but exited unsuccessfully.
// A non-nil error means the process did not complete (start failure,
// killed by ctx, etc).
type ExecFn func(ctx context.Context, dir string, env []string, stdin []byte, name string, args ...string) (stdout []byte, stderr []byte, exitCode int, err error)

// Options configures an Adapter at construction time.
type Options struct {
	// BinaryPath is the cursor-agent executable. Defaults to "cursor-agent"
	// (resolved against PATH).
	BinaryPath string
	// Args is the fixed argv tail appended after BinaryPath. Defaults to
	// []string{"--print", "--output-format", "json", "--force"}. The
	// "--force" flag instructs cursor-agent to auto-approve filesystem
	// and shell tool calls instead of blocking on an interactive prompt
	// the worker has no way to answer; without it the child would
	// reliably wedge until Request.Timeout. Override only if a runner
	// variant should not auto-approve (e.g. a future "plan-only" mode).
	Args []string
	// Name is the runner.Name() value (recorded in TaskCyclePhase MetaJSON
	// by the worker). Defaults to "cursor-cli".
	Name string
	// Version is the runner.Version() value. Defaults to
	// "0.0.0-unknown"; binaries should override with the real value.
	Version string
	// ExecFn replaces os/exec for tests. nil means use the real exec path.
	ExecFn ExecFn
	// ExtraAllowedEnvKeys widens the parent-env passthrough beyond the
	// default {PATH, HOME, USERPROFILE}. Entries are still subject to the
	// hardcoded deny-list (DATABASE_URL, T2A_*).
	ExtraAllowedEnvKeys []string
	// HomePathReplacements lets tests inject the values used to scrub
	// absolute home paths from RawOutput. Empty slice means use the live
	// $HOME and $USERPROFILE.
	HomePathReplacements []string
}

// Adapter is the cursor.Runner implementation. Construct via New.
type Adapter struct {
	binaryPath string
	args       []string
	name       string
	version    string
	exec       ExecFn
	extraKeys  []string
	homePaths  []string
}

// New returns a configured Adapter. Zero-value Options yields the V1
// defaults documented on Options.
func New(opts Options) *Adapter {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.New")
	a := &Adapter{
		binaryPath: opts.BinaryPath,
		args:       opts.Args,
		name:       opts.Name,
		version:    opts.Version,
		exec:       opts.ExecFn,
		extraKeys:  append([]string(nil), opts.ExtraAllowedEnvKeys...),
		homePaths:  append([]string(nil), opts.HomePathReplacements...),
	}
	if a.binaryPath == "" {
		a.binaryPath = defaultBinaryPath
	}
	if a.args == nil {
		a.args = []string{"--print", "--output-format", "json", "--force"}
	}
	if a.name == "" {
		a.name = defaultName
	}
	if a.version == "" {
		a.version = defaultVersion
	}
	if a.exec == nil {
		a.exec = defaultExecFn
	}
	if len(a.homePaths) == 0 {
		a.homePaths = liveHomePaths()
	}
	return a
}

// Name implements runner.Runner.
func (a *Adapter) Name() string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.Name")
	return a.name
}

// Version implements runner.Runner.
func (a *Adapter) Version() string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.Version")
	return a.version
}

// Run implements runner.Runner. See package documentation for the full
// invocation contract, env policy, redaction guarantees, and error
// mapping.
func (a *Adapter) Run(ctx context.Context, req runner.Request) (runner.Result, error) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.Run",
		"task_id", req.TaskID, "phase", string(req.Phase),
		"attempt_seq", req.AttemptSeq, "working_dir", req.WorkingDir,
		"timeout_ns", int64(req.Timeout))

	if err := ctx.Err(); err != nil {
		return runner.Result{}, fmt.Errorf("cursor: %w: %v", runner.ErrTimeout, err)
	}

	runCtx, cancel := withOptionalTimeout(ctx, req.Timeout)
	defer cancel()

	env := buildEnv(req.Env, a.extraKeys)
	stdout, stderr, exitCode, execErr := a.exec(
		runCtx,
		req.WorkingDir,
		env,
		[]byte(req.Prompt),
		a.binaryPath,
		a.args...,
	)

	rawOutput := redact(combineStreams(stdout, stderr), a.homePaths)

	if execErr != nil {
		if isCtxErr(runCtx) {
			return runner.NewResult(domain.PhaseStatusFailed, "cursor: timeout", nil, rawOutput),
				fmt.Errorf("cursor: %w: %v", runner.ErrTimeout, execErr)
		}
		return runner.NewResult(domain.PhaseStatusFailed, "cursor: exec failed", nil, rawOutput),
			fmt.Errorf("cursor: %w: %v", runner.ErrInvalidOutput, execErr)
	}

	if exitCode != 0 {
		details := stderrTailDetails(stderr, a.homePaths)
		return runner.NewResult(domain.PhaseStatusFailed,
				fmt.Sprintf("cursor: exit %d", exitCode), details, rawOutput),
			fmt.Errorf("cursor: %w: exit %d", runner.ErrNonZeroExit, exitCode)
	}

	parsed, parseErr := parseStdout(stdout)
	if parseErr != nil {
		return runner.NewResult(domain.PhaseStatusFailed,
				"cursor: invalid JSON output", nil, rawOutput),
			fmt.Errorf("cursor: %w: %v", runner.ErrInvalidOutput, parseErr)
	}

	summary := redact(parsed.Result, a.homePaths)
	details := buildDetails(parsed)

	if parsed.IsError {
		if summary == "" {
			summary = "cursor: agent reported is_error=true"
		}
		return runner.NewResult(domain.PhaseStatusFailed, summary, details, rawOutput),
			fmt.Errorf("cursor: %w: agent reported is_error=true", runner.ErrNonZeroExit)
	}

	return runner.NewResult(domain.PhaseStatusSucceeded, summary, details, rawOutput), nil
}

// cursorOutput is the cursor-agent --output-format json envelope.
//
// Schema (as observed against cursor-agent 2026.04.x; missing fields
// are zero-valued so the parser stays forward-compatible with new
// cursor-agent metadata):
//
//	{
//	  "type": "result",
//	  "subtype": "success",
//	  "is_error": false,
//	  "duration_ms": 17590,
//	  "duration_api_ms": 17590,
//	  "result": "<human-readable summary the agent emitted>",
//	  "session_id": "...",
//	  "request_id": "...",
//	  "usage": { "inputTokens": ..., "outputTokens": ..., ... }
//	}
//
// On is_error=true cursor-agent still exits 0; the adapter maps that
// to runner.ErrNonZeroExit with PhaseStatusFailed so the worker
// records a failed cycle instead of silently treating it as success.
type cursorOutput struct {
	Type          string          `json:"type,omitempty"`
	Subtype       string          `json:"subtype,omitempty"`
	IsError       bool            `json:"is_error,omitempty"`
	Result        string          `json:"result,omitempty"`
	DurationMs    int64           `json:"duration_ms,omitempty"`
	DurationAPIMs int64           `json:"duration_api_ms,omitempty"`
	SessionID     string          `json:"session_id,omitempty"`
	RequestID     string          `json:"request_id,omitempty"`
	Usage         json.RawMessage `json:"usage,omitempty"`
}

func parseStdout(stdout []byte) (cursorOutput, error) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.parseStdout", "bytes", len(stdout))
	stdout = bytes.TrimSpace(stdout)
	if len(stdout) == 0 {
		return cursorOutput{}, errors.New("empty stdout")
	}
	var out cursorOutput
	if err := json.Unmarshal(stdout, &out); err != nil {
		return cursorOutput{}, fmt.Errorf("decode stdout: %w", err)
	}
	return out, nil
}

// buildDetails serialises the cursor-agent metadata fields (everything
// other than "result") into the runner.Result.Details payload so the
// task_cycle_phases audit trail keeps the session/request IDs, timing
// breakdown, and token usage. The "result" text becomes Summary and
// is therefore intentionally elided here to avoid duplication.
func buildDetails(p cursorOutput) json.RawMessage {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.buildDetails",
		"type", p.Type, "subtype", p.Subtype, "is_error", p.IsError,
		"session_id", p.SessionID, "request_id", p.RequestID)
	d := struct {
		Type          string          `json:"type,omitempty"`
		Subtype       string          `json:"subtype,omitempty"`
		IsError       bool            `json:"is_error,omitempty"`
		DurationMs    int64           `json:"duration_ms,omitempty"`
		DurationAPIMs int64           `json:"duration_api_ms,omitempty"`
		SessionID     string          `json:"session_id,omitempty"`
		RequestID     string          `json:"request_id,omitempty"`
		Usage         json.RawMessage `json:"usage,omitempty"`
	}{
		Type:          p.Type,
		Subtype:       p.Subtype,
		IsError:       p.IsError,
		DurationMs:    p.DurationMs,
		DurationAPIMs: p.DurationAPIMs,
		SessionID:     p.SessionID,
		RequestID:     p.RequestID,
		Usage:         p.Usage,
	}
	b, err := json.Marshal(d)
	if err != nil {
		// Marshalling a struct of strings/ints/RawMessage cannot fail
		// in practice; fall back to nil so NewResult emits no details
		// rather than a malformed payload.
		return nil
	}
	return b
}

func combineStreams(stdout, stderr []byte) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.combineStreams",
		"stdout_bytes", len(stdout), "stderr_bytes", len(stderr))
	var b strings.Builder
	b.Grow(len(stdout) + len(stderr) + 16)
	if len(stdout) > 0 {
		b.WriteString("[stdout]\n")
		b.Write(stdout)
		if !bytes.HasSuffix(stdout, []byte{'\n'}) {
			b.WriteByte('\n')
		}
	}
	if len(stderr) > 0 {
		b.WriteString("[stderr]\n")
		b.Write(stderr)
	}
	return b.String()
}

func stderrTailDetails(stderr []byte, homePaths []string) json.RawMessage {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.stderrTailDetails",
		"stderr_bytes", len(stderr))
	tail := stderr
	if len(tail) > stderrTailBytes {
		tail = tail[len(tail)-stderrTailBytes:]
	}
	redacted := redact(string(tail), homePaths)
	payload, err := json.Marshal(struct {
		StderrTail string `json:"stderr_tail"`
	}{StderrTail: redacted})
	if err != nil {
		// json.Marshal of a struct of strings cannot fail in practice;
		// fall back to a static sentinel rather than ever returning nil.
		return json.RawMessage(`{"stderr_tail":"[redaction failure]"}`)
	}
	return payload
}

// envDeniedKeys lists keys that must NEVER reach the child process, even
// when the caller explicitly placed them in Request.Env. Defense in depth
// against accidental credential leaks.
var envDeniedKeys = map[string]struct{}{
	"DATABASE_URL": {},
}

// buildEnv assembles the env slice in os/exec format ("KEY=VALUE"). The
// passthrough set is {PATH, HOME, USERPROFILE} ∪ extraKeys ∪ keys(reqEnv),
// minus the deny-list. Later sources override earlier ones; reqEnv wins
// over parent env so callers can shadow PATH if they need to.
func buildEnv(reqEnv map[string]string, extraKeys []string) []string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.buildEnv",
		"req_env_count", len(reqEnv), "extra_keys", len(extraKeys))
	allowed := map[string]struct{}{
		"PATH":        {},
		"HOME":        {},
		"USERPROFILE": {},
	}
	for _, k := range extraKeys {
		if k == "" || isDeniedEnvKey(k) {
			continue
		}
		allowed[k] = struct{}{}
	}
	merged := map[string]string{}
	for k := range allowed {
		if v := os.Getenv(k); v != "" {
			merged[k] = v
		}
	}
	for k, v := range reqEnv {
		if k == "" || isDeniedEnvKey(k) {
			continue
		}
		merged[k] = v
	}
	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	return out
}

func isDeniedEnvKey(k string) bool {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.isDeniedEnvKey", "key", k)
	if _, denied := envDeniedKeys[k]; denied {
		return true
	}
	return strings.HasPrefix(k, "T2A_")
}

// liveHomePaths returns the absolute home paths to scrub from RawOutput,
// based on the parent process's env.
func liveHomePaths() []string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.liveHomePaths")
	out := make([]string, 0, 2)
	for _, k := range []string{"HOME", "USERPROFILE"} {
		if v := os.Getenv(k); v != "" {
			out = append(out, v)
		}
	}
	return out
}

var (
	// authHeaderRe matches "Authorization: <rest of line>" case-insensitively
	// and consumes the remainder of the line so the entire credential
	// (which may include dots, slashes, +, =, spaces in scheme variants)
	// is replaced — never just the first word.
	authHeaderRe = regexp.MustCompile(`(?i)(authorization:[ \t]*)([^\r\n]+)`)
	// cookieHeaderRe matches "Cookie:" and "Set-Cookie:" headers
	// case-insensitively and consumes the rest of the line. Session
	// cookies and Set-Cookie tokens are credential-bearing and must
	// be treated symmetrically with Authorization. The leading word
	// boundary `\b` keeps us from accidentally matching arbitrary
	// "...cookie:..." substrings inside JSON or paths.
	cookieHeaderRe = regexp.MustCompile(`(?i)\b((?:set-)?cookie:[ \t]*)([^\r\n]+)`)
	// t2aEnvRe matches "T2A_FOO=value" assignments. The value half is
	// taken as a single shell token (\S+), which is the conventional
	// shape T2A_* env vars take in logs. If a T2A value contains spaces
	// it will only have its first token redacted; that is acceptable for
	// the V1 floor (the env allowlist is the primary defense).
	t2aEnvRe = regexp.MustCompile(`(T2A_[A-Z0-9_]+)\s*=\s*\S+`)
)

// Redact replaces secret-shaped substrings in s with [REDACTED] markers
// and rewrites absolute home paths to "~". It is exported so callers
// (worker logs, future adapters) can apply the same floor.
func Redact(s string) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Redact", "bytes", len(s))
	return redact(s, liveHomePaths())
}

func redact(s string, homePaths []string) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.redact",
		"bytes", len(s), "home_paths", len(homePaths))
	if s == "" {
		return s
	}
	out := authHeaderRe.ReplaceAllString(s, "${1}[REDACTED]")
	out = cookieHeaderRe.ReplaceAllString(out, "${1}[REDACTED]")
	out = t2aEnvRe.ReplaceAllString(out, "$1=[REDACTED]")
	for _, hp := range homePaths {
		if hp == "" {
			continue
		}
		out = strings.ReplaceAll(out, hp, "~")
	}
	return out
}

func withOptionalTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.withOptionalTimeout",
		"timeout_ns", int64(d))
	if d <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}

func isCtxErr(ctx context.Context) bool {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.isCtxErr")
	err := ctx.Err()
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// defaultExecFn is the production ExecFn that actually shells out. Tests
// inject their own ExecFn via Options.ExecFn so go test never spawns a
// real Cursor binary.
func defaultExecFn(ctx context.Context, dir string, env []string, stdin []byte, name string, args ...string) ([]byte, []byte, int, error) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.defaultExecFn",
		"dir", dir, "env_count", len(env), "stdin_bytes", len(stdin), "name", name, "argc", len(args))
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
			err = nil
		}
	}
	return stdoutBuf.Bytes(), stderrBuf.Bytes(), exitCode, err
}

// Compile-time assertion that *Adapter implements runner.Runner.
var _ runner.Runner = (*Adapter)(nil)
