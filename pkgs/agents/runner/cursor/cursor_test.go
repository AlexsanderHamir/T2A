package cursor_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// captured records every (env, stdin, dir, name, args) tuple a fake ExecFn
// receives, so tests can assert on what the adapter actually invoked.
type captured struct {
	dir   string
	env   []string
	stdin []byte
	name  string
	args  []string
}

// fakeExec returns an ExecFn that records its inputs into *captured and
// returns the configured outputs. cancelOnInvoke=true delays return until
// ctx is cancelled (simulating a long-running child).
func fakeExec(c *captured, stdout, stderr []byte, exitCode int, runErr error, cancelOnInvoke bool) cursor.ExecFn {
	return func(ctx context.Context, dir string, env []string, stdin []byte, name string, args ...string) ([]byte, []byte, int, error) {
		c.dir = dir
		c.env = append([]string(nil), env...)
		c.stdin = append([]byte(nil), stdin...)
		c.name = name
		c.args = append([]string(nil), args...)
		if cancelOnInvoke {
			<-ctx.Done()
			return stdout, stderr, 0, ctx.Err()
		}
		return stdout, stderr, exitCode, runErr
	}
}

func fakeStreamExec(c *captured, stdout, stderr []byte, exitCode int, runErr error) cursor.StreamExecFn {
	return func(ctx context.Context, dir string, env []string, stdin []byte, name string, onStdoutLine func([]byte), args ...string) ([]byte, []byte, int, error) {
		c.dir = dir
		c.env = append([]string(nil), env...)
		c.stdin = append([]byte(nil), stdin...)
		c.name = name
		c.args = append([]string(nil), args...)
		for _, line := range bytes.Split(bytes.TrimSpace(stdout), []byte("\n")) {
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			onStdoutLine(line)
		}
		return stdout, stderr, exitCode, runErr
	}
}

func newAdapter(execFn cursor.ExecFn, extraOpts ...func(*cursor.Options)) *cursor.Adapter {
	opts := cursor.Options{
		BinaryPath:           "fake-cursor-agent",
		ExecFn:               execFn,
		Name:                 "cursor-cli",
		Version:              "test-1.0",
		HomePathReplacements: []string{"/home/runner", `C:\Users\runner`},
	}
	for _, f := range extraOpts {
		f(&opts)
	}
	return cursor.New(opts)
}

func defaultRequest() runner.Request {
	return runner.Request{
		TaskID:     "11111111-1111-4111-8111-111111111111",
		AttemptSeq: 1,
		Phase:      domain.PhaseExecute,
		Prompt:     "do the thing",
		WorkingDir: "/repo/work",
		Timeout:    2 * time.Second,
	}
}

// TestRun_successPath covers the happy path: 0 exit + valid JSON stdout
// produces a Result with PhaseStatusSucceeded and the parsed Summary /
// Details intact.
func TestRun_successPath(t *testing.T) {
	t.Parallel()

	stdout := []byte(`{"type":"result","subtype":"success","is_error":false,"duration_ms":1200,"duration_api_ms":1100,"result":"all good","session_id":"sess-abc","request_id":"req-xyz","usage":{"inputTokens":10,"outputTokens":3}}`)
	var c captured
	a := newAdapter(fakeExec(&c, stdout, nil, 0, nil, false))

	res, err := a.Run(context.Background(), defaultRequest())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != domain.PhaseStatusSucceeded {
		t.Errorf("Status: got %q want %q", res.Status, domain.PhaseStatusSucceeded)
	}
	if res.Summary != "all good" {
		t.Errorf("Summary: got %q", res.Summary)
	}
	var details struct {
		Type       string          `json:"type"`
		Subtype    string          `json:"subtype"`
		IsError    bool            `json:"is_error"`
		DurationMs int64           `json:"duration_ms"`
		SessionID  string          `json:"session_id"`
		RequestID  string          `json:"request_id"`
		Usage      json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(res.Details, &details); err != nil {
		t.Fatalf("Details unmarshal: %v (raw=%s)", err, res.Details)
	}
	if details.Type != "result" || details.Subtype != "success" {
		t.Errorf("Details type/subtype: got %q/%q want result/success", details.Type, details.Subtype)
	}
	if details.IsError {
		t.Errorf("Details.is_error must be false on happy path")
	}
	if details.SessionID != "sess-abc" || details.RequestID != "req-xyz" {
		t.Errorf("Details ids: got session=%q request=%q", details.SessionID, details.RequestID)
	}
	if details.DurationMs != 1200 {
		t.Errorf("Details.duration_ms: got %d", details.DurationMs)
	}
	if len(details.Usage) == 0 {
		t.Errorf("Details.usage missing; got %s", res.Details)
	}
	if c.name != "fake-cursor-agent" {
		t.Errorf("invoked name: got %q", c.name)
	}
	wantArgs := []string{"--print", "--output-format", "stream-json", "--force"}
	if !equalStrSlice(c.args, wantArgs) {
		t.Errorf("args: got %v want %v", c.args, wantArgs)
	}
	if res.ResolvedModel != "" {
		t.Errorf("ResolvedModel on legacy single-object stdout should be empty; got %q", res.ResolvedModel)
	}
	if string(c.stdin) != "do the thing" {
		t.Errorf("stdin: got %q", c.stdin)
	}
	if c.dir != "/repo/work" {
		t.Errorf("dir: got %q", c.dir)
	}
}

// TestRun_streamJSONCapturesResolvedModel pins the new plumbing: when
// cursor-agent emits its --output-format stream-json NDJSON, the
// adapter must
//   - extract the resolved model from the first `system.init` event
//     (the ONLY surface where cursor-agent reveals what model `auto`
//     routed to — the terminal `result` event has no model field; see
//     https://cursor.com/docs/cli/reference/output-format),
//   - still recover the Summary / session_id / timing from the
//     terminal `result` event (wire-identical to the old json format),
//   - surface the captured model both on Result.ResolvedModel (so the
//     worker can record it in cycle MetaJSON as
//     cursor_model_resolved) AND inside Result.Details.resolved_model
//     (so the per-phase details_json audit row carries it too).
func TestRun_streamJSONCapturesResolvedModel(t *testing.T) {
	t.Parallel()

	stdout := []byte(
		`{"type":"system","subtype":"init","apiKeySource":"login","cwd":"/tmp","session_id":"sess-abc","model":"Claude 4 Sonnet","permissionMode":"default"}` + "\n" +
			`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"do the thing"}]},"session_id":"sess-abc"}` + "\n" +
			`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"working"}]},"session_id":"sess-abc"}` + "\n" +
			`{"type":"result","subtype":"success","is_error":false,"duration_ms":1200,"duration_api_ms":1100,"result":"all good","session_id":"sess-abc","request_id":"req-xyz"}` + "\n",
	)
	a := newAdapter(fakeExec(&captured{}, stdout, nil, 0, nil, false))

	res, err := a.Run(context.Background(), defaultRequest())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != domain.PhaseStatusSucceeded {
		t.Errorf("Status: got %q want %q", res.Status, domain.PhaseStatusSucceeded)
	}
	if res.Summary != "all good" {
		t.Errorf("Summary: got %q want %q", res.Summary, "all good")
	}
	if res.ResolvedModel != "Claude 4 Sonnet" {
		t.Errorf("ResolvedModel: got %q want %q", res.ResolvedModel, "Claude 4 Sonnet")
	}
	var details struct {
		ResolvedModel string `json:"resolved_model"`
		SessionID     string `json:"session_id"`
	}
	if err := json.Unmarshal(res.Details, &details); err != nil {
		t.Fatalf("Details unmarshal: %v (raw=%s)", err, res.Details)
	}
	if details.ResolvedModel != "Claude 4 Sonnet" {
		t.Errorf("Details.resolved_model: got %q want %q", details.ResolvedModel, "Claude 4 Sonnet")
	}
	if details.SessionID != "sess-abc" {
		t.Errorf("Details.session_id should come from the terminal result event: got %q", details.SessionID)
	}
}

func TestRun_streamJSONEmitsLiveProgress(t *testing.T) {
	t.Parallel()

	stdout := []byte(
		`{"type":"system","subtype":"init","model":"Claude 4 Sonnet"}` + "\n" +
			`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Reading the task files."}]}}` + "\n" +
			`{"type":"tool_call","subtype":"started","name":"ReadFile"}` + "\n" +
			`{"type":"result","subtype":"success","is_error":false,"result":"done","session_id":"sess-live"}` + "\n",
	)
	var c captured
	a := newAdapter(nil, func(opts *cursor.Options) {
		opts.StreamExecFn = fakeStreamExec(&c, stdout, nil, 0, nil)
	})
	var progress []runner.ProgressEvent
	req := defaultRequest()
	req.OnProgress = func(ev runner.ProgressEvent) {
		progress = append(progress, ev)
	}

	res, err := a.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Summary != "done" {
		t.Fatalf("Summary: got %q want done", res.Summary)
	}
	if len(progress) != 3 {
		t.Fatalf("progress count: got %d want 3 (%+v)", len(progress), progress)
	}
	if progress[0].Kind != "system" || progress[0].Message != "Using Claude 4 Sonnet" {
		t.Errorf("system progress: %+v", progress[0])
	}
	if progress[1].Kind != "assistant" || progress[1].Message != "Reading the task files." {
		t.Errorf("assistant progress: %+v", progress[1])
	}
	if progress[2].Kind != "tool_call" || progress[2].Subtype != "started" || progress[2].Tool != "ReadFile" {
		t.Errorf("tool progress: %+v", progress[2])
	}
}

// TestRun_streamJSONMissingSystemEventLeavesResolvedModelEmpty covers
// the forward-compat gap: an older cursor-agent build might emit a
// stream without a `system.init` event (e.g. if a future version
// reshapes the init shape or drops it entirely). The adapter must
// still succeed by finding the terminal result event, and record
// ResolvedModel="" — truthful "unknown" rather than a fabricated
// fallback.
func TestRun_streamJSONMissingSystemEventLeavesResolvedModelEmpty(t *testing.T) {
	t.Parallel()

	stdout := []byte(
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"working"}]},"session_id":"sess-no-init"}` + "\n" +
			`{"type":"result","subtype":"success","is_error":false,"result":"done","session_id":"sess-no-init"}` + "\n",
	)
	a := newAdapter(fakeExec(&captured{}, stdout, nil, 0, nil, false))

	res, err := a.Run(context.Background(), defaultRequest())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != domain.PhaseStatusSucceeded {
		t.Errorf("Status: got %q", res.Status)
	}
	if res.ResolvedModel != "" {
		t.Errorf("ResolvedModel must stay empty when the stream lacks a system.init event; got %q", res.ResolvedModel)
	}
	if res.Summary != "done" {
		t.Errorf("Summary: got %q", res.Summary)
	}
}

// TestRun_isErrorTrueMapsToFailure pins the contract that an exit-0
// run with cursor-agent reporting "is_error": true is treated as a
// recoverable runner failure (ErrNonZeroExit + PhaseStatusFailed) so
// the worker writes a failed cycle instead of silently treating the
// run as success. The agent's own "result" text becomes the Summary.
func TestRun_isErrorTrueMapsToFailure(t *testing.T) {
	t.Parallel()

	stdout := []byte(`{"type":"result","subtype":"error","is_error":true,"result":"could not authenticate","session_id":"sess-err","request_id":"req-err"}`)
	a := newAdapter(fakeExec(&captured{}, stdout, nil, 0, nil, false))

	res, err := a.Run(context.Background(), defaultRequest())
	if !errors.Is(err, runner.ErrNonZeroExit) {
		t.Fatalf("err: got %v want errors.Is(_, ErrNonZeroExit)", err)
	}
	if res.Status != domain.PhaseStatusFailed {
		t.Errorf("Status: got %q want %q", res.Status, domain.PhaseStatusFailed)
	}
	if res.Summary != "could not authenticate" {
		t.Errorf("Summary: got %q want the agent's result text", res.Summary)
	}
	var details struct {
		IsError   bool   `json:"is_error"`
		Subtype   string `json:"subtype"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(res.Details, &details); err != nil {
		t.Fatalf("Details unmarshal: %v (raw=%s)", err, res.Details)
	}
	if !details.IsError || details.Subtype != "error" {
		t.Errorf("Details mismatch: got is_error=%v subtype=%q", details.IsError, details.Subtype)
	}
	if details.SessionID != "sess-err" {
		t.Errorf("Details.session_id: got %q", details.SessionID)
	}
}

// TestRun_isErrorTrueWithEmptyResultGetsFallbackSummary covers the
// edge case where cursor-agent sets is_error=true but does not emit a
// "result" string. The Summary must still be non-empty so the audit
// row is honest about the failure.
func TestRun_isErrorTrueWithEmptyResultGetsFallbackSummary(t *testing.T) {
	t.Parallel()

	stdout := []byte(`{"type":"result","is_error":true}`)
	a := newAdapter(fakeExec(&captured{}, stdout, nil, 0, nil, false))

	res, err := a.Run(context.Background(), defaultRequest())
	if !errors.Is(err, runner.ErrNonZeroExit) {
		t.Fatalf("err: got %v want errors.Is(_, ErrNonZeroExit)", err)
	}
	if res.Summary == "" {
		t.Errorf("Summary must not be empty on is_error fallback")
	}
}

// TestRun_nonZeroExit asserts the documented error mapping plus the
// stderr_tail-in-Details contract.
func TestRun_nonZeroExit(t *testing.T) {
	t.Parallel()

	stderr := []byte("compile failed\nerror: missing semicolon\n")
	var c captured
	a := newAdapter(fakeExec(&c, []byte(""), stderr, 7, nil, false))

	res, err := a.Run(context.Background(), defaultRequest())
	if !errors.Is(err, runner.ErrNonZeroExit) {
		t.Fatalf("err: got %v want errors.Is(_, ErrNonZeroExit)", err)
	}
	if res.Status != domain.PhaseStatusFailed {
		t.Errorf("Status: got %q want %q", res.Status, domain.PhaseStatusFailed)
	}
	if !strings.Contains(res.Summary, "exit 7") {
		t.Errorf("Summary should mention exit code: got %q", res.Summary)
	}
	if !strings.Contains(res.Summary, "compile failed") {
		t.Errorf("Summary should include first stderr line hint: got %q", res.Summary)
	}
	var details struct {
		StderrTail string `json:"stderr_tail"`
	}
	if err := json.Unmarshal(res.Details, &details); err != nil {
		t.Fatalf("Details unmarshal: %v (raw=%s)", err, res.Details)
	}
	if !strings.Contains(details.StderrTail, "missing semicolon") {
		t.Errorf("stderr_tail missing expected content: %q", details.StderrTail)
	}
	if !strings.Contains(res.RawOutput, "missing semicolon") {
		t.Errorf("RawOutput should include redacted stderr: %q", res.RawOutput)
	}
}

// TestRun_invalidJSON exercises the parse-failure branch.
func TestRun_invalidJSON(t *testing.T) {
	t.Parallel()

	a := newAdapter(fakeExec(&captured{}, []byte("not json at all"), nil, 0, nil, false))
	res, err := a.Run(context.Background(), defaultRequest())
	if !errors.Is(err, runner.ErrInvalidOutput) {
		t.Fatalf("err: got %v want errors.Is(_, ErrInvalidOutput)", err)
	}
	if res.Status != domain.PhaseStatusFailed {
		t.Errorf("Status: got %q", res.Status)
	}
}

// TestRun_emptyStdoutInvalid catches an edge case: 0 exit but no stdout
// must be ErrInvalidOutput, not silent success.
func TestRun_emptyStdoutInvalid(t *testing.T) {
	t.Parallel()

	a := newAdapter(fakeExec(&captured{}, []byte("   "), nil, 0, nil, false))
	_, err := a.Run(context.Background(), defaultRequest())
	if !errors.Is(err, runner.ErrInvalidOutput) {
		t.Errorf("got %v want errors.Is(_, ErrInvalidOutput)", err)
	}
}

// TestRun_timeout drives the per-call timeout path: the ExecFn blocks
// until ctx is cancelled, the adapter must return ErrTimeout with status
// Failed.
func TestRun_timeout(t *testing.T) {
	t.Parallel()

	a := newAdapter(fakeExec(&captured{}, nil, nil, 0, nil, true))
	req := defaultRequest()
	req.Timeout = 25 * time.Millisecond

	res, err := a.Run(context.Background(), req)
	if !errors.Is(err, runner.ErrTimeout) {
		t.Fatalf("err: got %v want errors.Is(_, ErrTimeout)", err)
	}
	if res.Status != domain.PhaseStatusFailed {
		t.Errorf("Status on timeout: got %q want %q", res.Status, domain.PhaseStatusFailed)
	}
}

// TestRun_alreadyCancelledContext short-circuits without invoking exec.
func TestRun_alreadyCancelledContext(t *testing.T) {
	t.Parallel()

	called := false
	exec := func(ctx context.Context, dir string, env []string, stdin []byte, name string, args ...string) ([]byte, []byte, int, error) {
		called = true
		return nil, nil, 0, nil
	}
	a := newAdapter(exec)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := a.Run(ctx, defaultRequest())
	if !errors.Is(err, runner.ErrTimeout) {
		t.Errorf("got %v want errors.Is(_, ErrTimeout)", err)
	}
	if called {
		t.Errorf("exec must not be invoked when ctx is already cancelled")
	}
}

// TestRun_redactionAuthorizationHeader proves Authorization values are
// scrubbed from RawOutput.
func TestRun_redactionAuthorizationHeader(t *testing.T) {
	t.Parallel()

	stderr := []byte("Authorization: Bearer sk-live-supersecrettoken\n")
	a := newAdapter(fakeExec(&captured{}, []byte(""), stderr, 1, nil, false))

	res, _ := a.Run(context.Background(), defaultRequest())
	if strings.Contains(res.RawOutput, "sk-live-supersecrettoken") {
		t.Errorf("RawOutput leaks bearer token: %q", res.RawOutput)
	}
	if !strings.Contains(res.RawOutput, "[REDACTED]") {
		t.Errorf("RawOutput missing redaction marker: %q", res.RawOutput)
	}
}

// TestRun_redactionCookieHeader proves Cookie and Set-Cookie header
// values are scrubbed from RawOutput. The Authorization header is
// already redacted (TestRun_redactionAuthorizationHeader), but Cookie
// and Set-Cookie headers are equally credential-bearing — a session
// cookie is functionally equivalent to a bearer token. Cursor's CLI
// can emit HTTP-style traces in verbose / error paths (or any embedded
// HTTP client logging) where these headers leak verbatim. Treating
// only Authorization as secret-shaped while leaving Cookie /
// Set-Cookie in the clear is a defense-in-depth gap. The fix matches
// both `Cookie:` and `Set-Cookie:` case-insensitively (the latter
// covers the response-side header variant) and consumes the rest of
// the line, mirroring the Authorization redaction shape exactly.
func TestRun_redactionCookieHeader(t *testing.T) {
	t.Parallel()

	stderr := []byte("Cookie: session=abc.def.ghi; csrf=xyz123\n" +
		"Set-Cookie: auth=tok-1234567890; Path=/; HttpOnly\n")
	a := newAdapter(fakeExec(&captured{}, []byte(""), stderr, 1, nil, false))

	res, _ := a.Run(context.Background(), defaultRequest())
	if strings.Contains(res.RawOutput, "session=abc.def.ghi") {
		t.Errorf("RawOutput leaks Cookie value: %q", res.RawOutput)
	}
	if strings.Contains(res.RawOutput, "csrf=xyz123") {
		t.Errorf("RawOutput leaks Cookie attribute: %q", res.RawOutput)
	}
	if strings.Contains(res.RawOutput, "auth=tok-1234567890") {
		t.Errorf("RawOutput leaks Set-Cookie value: %q", res.RawOutput)
	}
	if !strings.Contains(res.RawOutput, "Cookie: [REDACTED]") {
		t.Errorf("missing Cookie redaction marker: %q", res.RawOutput)
	}
	if !strings.Contains(res.RawOutput, "Set-Cookie: [REDACTED]") {
		t.Errorf("missing Set-Cookie redaction marker: %q", res.RawOutput)
	}
}

// TestRun_redactionT2AEnv proves T2A_* env values are scrubbed from
// RawOutput. Exact mechanism: stderr accidentally echoing an env line
// like "T2A_DATABASE_URL=postgres://...".
func TestRun_redactionT2AEnv(t *testing.T) {
	t.Parallel()

	stderr := []byte("env dump: T2A_DATABASE_URL=postgres://user:pw@host/db PATH=/usr/bin\n")
	a := newAdapter(fakeExec(&captured{}, []byte(""), stderr, 1, nil, false))

	res, _ := a.Run(context.Background(), defaultRequest())
	if strings.Contains(res.RawOutput, "postgres://user:pw@host/db") {
		t.Errorf("RawOutput leaks DATABASE_URL value: %q", res.RawOutput)
	}
	if !strings.Contains(res.RawOutput, "T2A_DATABASE_URL=[REDACTED]") {
		t.Errorf("expected T2A_DATABASE_URL=[REDACTED]: %q", res.RawOutput)
	}
}

// TestRun_redactionHomePath proves absolute home paths are rewritten to
// "~" so RawOutput does not depend on the operator's filesystem layout.
func TestRun_redactionHomePath(t *testing.T) {
	t.Parallel()

	stderr := []byte("error in /home/runner/.cache/cursor/config.json\nalso C:\\Users\\runner\\AppData\\Local\\cursor\\log.txt\n")
	a := newAdapter(fakeExec(&captured{}, []byte(""), stderr, 1, nil, false))

	res, _ := a.Run(context.Background(), defaultRequest())
	if strings.Contains(res.RawOutput, "/home/runner/") {
		t.Errorf("Unix home path not redacted: %q", res.RawOutput)
	}
	if strings.Contains(res.RawOutput, `C:\Users\runner\`) {
		t.Errorf("Windows home path not redacted: %q", res.RawOutput)
	}
	if !strings.Contains(res.RawOutput, "~/.cache/cursor/config.json") {
		t.Errorf("expected ~/-prefixed unix path in: %q", res.RawOutput)
	}
}

// TestRedact_publicHelper covers the exported Redact entry point used by
// future callers (worker logs).
func TestRedact_publicHelper(t *testing.T) {
	t.Parallel()

	in := "Authorization: Bearer abc.def.ghi\nT2A_FOO=secretvalue\nCookie: sid=cookie-secret-12345\nSet-Cookie: x=y; HttpOnly\n"
	got := cursor.Redact(in)
	if strings.Contains(got, "abc.def.ghi") || strings.Contains(got, "secretvalue") {
		t.Errorf("Redact leaked secret: %q", got)
	}
	if strings.Contains(got, "cookie-secret-12345") {
		t.Errorf("Redact leaked Cookie value: %q", got)
	}
	if strings.Contains(got, "x=y") {
		t.Errorf("Redact leaked Set-Cookie value: %q", got)
	}
}

// TestRun_envAllowlist asserts that DATABASE_URL and T2A_* keys are
// stripped from the env passed to the child process even when the caller
// places them in Request.Env. This is the defense-in-depth guarantee.
//
// Cannot run in parallel: t.Setenv mutates process-global state.
func TestRun_envAllowlist(t *testing.T) {
	t.Setenv("PATH", "/test/path")
	t.Setenv("HOME", "/home/runner")
	t.Setenv("DATABASE_URL", "postgres://should-not-leak")
	t.Setenv("T2A_SECRET_TOKEN", "should-not-leak")
	t.Setenv("ALLOWED_EXTRA", "yes-please")

	var c captured
	a := newAdapter(
		fakeExec(&c, []byte(`{"type":"result","subtype":"success","result":"ok"}`), nil, 0, nil, false),
		func(o *cursor.Options) {
			o.ExtraAllowedEnvKeys = []string{"ALLOWED_EXTRA"}
		},
	)
	req := defaultRequest()
	req.Env = map[string]string{
		"DATABASE_URL":     "from-request-must-also-be-stripped",
		"T2A_BACKDOOR":     "must-be-stripped",
		"REQUEST_PROVIDED": "request-wins-over-parent",
	}

	if _, err := a.Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}

	envMap := envSliceToMap(c.env)
	if _, present := envMap["DATABASE_URL"]; present {
		t.Errorf("DATABASE_URL must never be passed to child: %v", envMap)
	}
	for k := range envMap {
		if strings.HasPrefix(k, "T2A_") {
			t.Errorf("T2A_* keys must never be passed to child: %s", k)
		}
	}
	if envMap["PATH"] != "/test/path" {
		t.Errorf("PATH not passed through: got %q", envMap["PATH"])
	}
	if envMap["HOME"] != "/home/runner" {
		t.Errorf("HOME not passed through: got %q", envMap["HOME"])
	}
	if envMap["ALLOWED_EXTRA"] != "yes-please" {
		t.Errorf("ExtraAllowedEnvKeys not honoured: got %q", envMap["ALLOWED_EXTRA"])
	}
	if envMap["REQUEST_PROVIDED"] != "request-wins-over-parent" {
		t.Errorf("Request.Env not merged: got %q", envMap["REQUEST_PROVIDED"])
	}
}

// TestRun_envRequestShadowsParent: Request.Env overrides the inherited
// parent value for the same key.
//
// Cannot run in parallel: t.Setenv mutates process-global state.
func TestRun_envRequestShadowsParent(t *testing.T) {
	t.Setenv("PATH", "/parent/path")
	var c captured
	a := newAdapter(fakeExec(&c, []byte(`{"type":"result","subtype":"success","result":"ok"}`), nil, 0, nil, false))
	req := defaultRequest()
	req.Env = map[string]string{"PATH": "/request/path"}

	if _, err := a.Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if envSliceToMap(c.env)["PATH"] != "/request/path" {
		t.Errorf("Request.Env did not shadow parent PATH: %v", c.env)
	}
}

// TestRun_envSystemVarsForwarded asserts the curated default-passthrough
// list (defaultPassthroughEnvKeys) actually reaches the child for the
// Windows system vars that were missing from the original {PATH, HOME,
// USERPROFILE} seed.
//
// 2026-04-19 incident: with SYSTEMDRIVE / SYSTEMROOT / TEMP / etc.
// stripped from the child env block, components in the cursor-agent
// process tree (Software Licensing Service, ETW, .NET CLR config
// loaders, Defender hooks) called ExpandEnvironmentStrings on hardcoded
// paths like "%SystemDrive%\\ProgramData\\Microsoft\\Windows\\Caches\\..."
// against the empty env, got the literal "%SystemDrive%\\..." string
// back, and CreateFile resolved it as a relative path under the child's
// cwd — which is AppSettings.RepoRoot. The child silently wrote a
// literal "%SystemDrive%" directory tree into the operator's worktree,
// surfacing as junk in `git status` and forcing a manual cleanup. This
// test pins the wider passthrough so a future "minimise the env" refactor
// cannot reintroduce the regression without flagging it loudly in CI.
//
// Cannot run in parallel: t.Setenv mutates process-global state.
func TestRun_envSystemVarsForwarded(t *testing.T) {
	cases := []struct {
		key, value string
	}{
		// Windows process model + shell (the canonical "system" set).
		{"SYSTEMDRIVE", "C:"},
		{"SYSTEMROOT", `C:\Windows`},
		{"WINDIR", `C:\Windows`},
		{"COMSPEC", `C:\Windows\System32\cmd.exe`},
		{"PATHEXT", ".COM;.EXE;.BAT;.CMD"},
		// Known folders.
		{"LOCALAPPDATA", `C:\Users\runner\AppData\Local`},
		{"APPDATA", `C:\Users\runner\AppData\Roaming`},
		{"PROGRAMDATA", `C:\ProgramData`},
		{"ALLUSERSPROFILE", `C:\ProgramData`},
		{"PUBLIC", `C:\Users\Public`},
		{"TEMP", `C:\Users\runner\AppData\Local\Temp`},
		{"TMP", `C:\Users\runner\AppData\Local\Temp`},
		// Program / DLL lookup.
		{"PROGRAMFILES", `C:\Program Files`},
		{"PROGRAMFILES(X86)", `C:\Program Files (x86)`},
		{"PROGRAMW6432", `C:\Program Files`},
		{"COMMONPROGRAMFILES", `C:\Program Files\Common Files`},
		{"COMMONPROGRAMFILES(X86)", `C:\Program Files (x86)\Common Files`},
		// Identity.
		{"USERNAME", "runner"},
		{"USERDOMAIN", "BUILDBOX"},
		{"COMPUTERNAME", "BUILDBOX"},
		{"LOGONSERVER", `\\BUILDBOX`},
		{"SESSIONNAME", "Console"},
		// Architecture / CPU.
		{"OS", "Windows_NT"},
		{"PROCESSOR_ARCHITECTURE", "AMD64"},
		{"PROCESSOR_IDENTIFIER", "Intel64 Family 6 Model 142 Stepping 12, GenuineIntel"},
		{"PROCESSOR_LEVEL", "6"},
		{"PROCESSOR_REVISION", "8e0c"},
		{"NUMBER_OF_PROCESSORS", "8"},
	}
	for _, tc := range cases {
		t.Setenv(tc.key, tc.value)
	}

	var c captured
	a := newAdapter(fakeExec(&c, []byte(`{"type":"result","subtype":"success","result":"ok"}`), nil, 0, nil, false))

	if _, err := a.Run(context.Background(), defaultRequest()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	envMap := envSliceToMap(c.env)
	for _, tc := range cases {
		got, ok := envMap[tc.key]
		if !ok {
			t.Errorf("env key %q must be forwarded by default; child saw env=%v", tc.key, envMap)
			continue
		}
		// The buildEnv contract is "forward whatever os.Getenv returns at
		// lookup time", not "preserve every t.Setenv override exactly":
		// a handful of Windows-reserved vars (NUMBER_OF_PROCESSORS is
		// the canonical example, computed by the Session Manager from
		// physical CPU topology) ignore SetEnvironmentVariable for the
		// session, so the value buildEnv sees can differ from what
		// t.Setenv just wrote. Asserting "child env value matches
		// parent env value at the call site" still proves the
		// passthrough policy without coupling the test to those quirks.
		// The t.Setenv calls above are still doing real work — they
		// guarantee the key is non-empty so buildEnv does not skip it.
		want := os.Getenv(tc.key)
		if got != want {
			t.Errorf("env key %q: child got %q, parent has %q (buildEnv must passthrough verbatim)", tc.key, got, want)
		}
	}
}

// TestRun_workingDirPropagated already covered indirectly by the success
// path; this test pins it explicitly with a non-default value.
func TestRun_workingDirPropagated(t *testing.T) {
	t.Parallel()

	var c captured
	a := newAdapter(fakeExec(&c, []byte(`{"type":"result","subtype":"success","result":"ok"}`), nil, 0, nil, false))
	req := defaultRequest()
	req.WorkingDir = "/some/other/repo"

	if _, err := a.Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if c.dir != "/some/other/repo" {
		t.Errorf("dir: got %q want %q", c.dir, "/some/other/repo")
	}
}

// TestNew_defaults pins the documented default values so wire-up code
// can rely on them.
func TestNew_defaults(t *testing.T) {
	t.Parallel()

	a := cursor.New(cursor.Options{})
	if a.Name() != "cursor-cli" {
		t.Errorf("default Name: got %q", a.Name())
	}
	if a.Version() != "0.0.0-unknown" {
		t.Errorf("default Version: got %q", a.Version())
	}
}

// TestNew_overridesNameVersion confirms the worker can override the
// MetaJSON-recorded identity values at construction.
func TestNew_overridesNameVersion(t *testing.T) {
	t.Parallel()

	a := cursor.New(cursor.Options{Name: "cursor-prod", Version: "1.42.0"})
	if a.Name() != "cursor-prod" {
		t.Errorf("Name override lost: %q", a.Name())
	}
	if a.Version() != "1.42.0" {
		t.Errorf("Version override lost: %q", a.Version())
	}
}

// TestRunner_compileTimeConformance keeps the interface surface stable.
func TestRunner_compileTimeConformance(t *testing.T) {
	t.Parallel()
	var _ runner.Runner = cursor.New(cursor.Options{})
}

// TestRun_stderrTailDoesNotSplitMultibyteUTF8 pins the byte-cap tail
// slicing in stderrTailDetails to a UTF-8 rune boundary. Mirrors the
// fix landed in pkgs/repo/read_preview_io.go for file previews
// (TestReadFilePreview_truncatedDoesNotSplitMultibyteUTF8): both call
// sites take the last/first N bytes of a buffer that may contain
// multibyte runes at the boundary, and both must drop the partial
// leading rune before the bytes become a Go string. Without the fix
// json.Marshal silently rewrites the dangling continuation byte to
// U+FFFD, leaking a corrupted diagnostic into the audit
// task_cycle_phases.details_json payload (and into every API response
// that surfaces it).
func TestRun_stderrTailDoesNotSplitMultibyteUTF8(t *testing.T) {
	t.Parallel()

	// Place a 3-byte UTF-8 character ("中" = E4 B8 AD) at offset 0 so
	// the 8 KiB tail truncation lands exactly at index 1 — the second
	// byte of the rune (B8, a continuation byte). Total stderr length
	// = 3 + 8190 = 8193, so tail = stderr[8193-8192:] = stderr[1:].
	const trailing = 8190
	stderrIn := append([]byte("中"), bytes.Repeat([]byte("y"), trailing)...)
	if len(stderrIn) != 8193 {
		t.Fatalf("test setup: stderr len = %d, want 8193", len(stderrIn))
	}

	var c captured
	a := newAdapter(fakeExec(&c, []byte(""), stderrIn, 7, nil, false))

	res, err := a.Run(context.Background(), defaultRequest())
	if !errors.Is(err, runner.ErrNonZeroExit) {
		t.Fatalf("err: got %v want errors.Is(_, ErrNonZeroExit)", err)
	}
	var details struct {
		StderrTail string `json:"stderr_tail"`
	}
	if err := json.Unmarshal(res.Details, &details); err != nil {
		t.Fatalf("Details unmarshal: %v (raw=%s)", err, res.Details)
	}
	if !utf8.ValidString(details.StderrTail) {
		t.Fatalf("stderr_tail must be valid UTF-8 after byte-cap truncation; got %q", details.StderrTail)
	}
	if strings.HasPrefix(details.StderrTail, "\uFFFD") {
		t.Errorf("stderr_tail must not start with the U+FFFD replacement char (truncation split a UTF-8 rune); got prefix %q",
			details.StderrTail[:min(len(details.StderrTail), 6)])
	}
}

func envSliceToMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, kv := range env {
		i := strings.IndexByte(kv, '=')
		if i < 0 {
			continue
		}
		out[kv[:i]] = kv[i+1:]
	}
	return out
}

func equalStrSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestAdapter_EffectiveModel pins the per-adapter resolution rule the
// worker depends on for cycle_meta.cursor_model_effective and the new
// Prometheus model label: trim req.CursorModel and use it; otherwise
// fall back to Options.DefaultCursorModel; otherwise return "" so the
// audit row records the truth ("no model configured anywhere") rather
// than a substituted placeholder.
func TestAdapter_EffectiveModel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		defaultModel string
		reqModel     string
		want         string
	}{
		{"both empty", "", "", ""},
		{"default only", "opus", "", "opus"},
		{"request overrides default", "opus", "sonnet-4.5", "sonnet-4.5"},
		{"request whitespace falls back", "opus", "   ", "opus"},
		{"request trimmed", "opus", "  sonnet-4.5  ", "sonnet-4.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := cursor.New(cursor.Options{
				DefaultCursorModel: tc.defaultModel,
				ExecFn:             fakeExec(&captured{}, []byte("{}"), nil, 0, nil, false),
			})
			got := a.EffectiveModel(runner.Request{CursorModel: tc.reqModel})
			if got != tc.want {
				t.Errorf("EffectiveModel(req.CursorModel=%q) with default %q: got %q want %q",
					tc.reqModel, tc.defaultModel, got, tc.want)
			}
		})
	}
}
