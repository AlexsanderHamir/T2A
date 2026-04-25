package cursor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

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

const diagnosticTailBytes = 4 * 1024

// stderrSummaryHintRunes caps the first-line stderr excerpt appended to the
// phase summary on non-zero exit so operators see why the CLI failed
// without opening raw logs, while staying within runner.MaxSummaryRunes.
const stderrSummaryHintRunes = 280

// ExecFn is the seam unit tests use to avoid shelling out. It receives
// everything the adapter would pass to os/exec and returns the captured
// stdout, stderr, exit code, and error. A nil error with a non-zero
// exitCode means the process ran to completion but exited unsuccessfully.
// A non-nil error means the process did not complete (start failure,
// killed by ctx, etc).
type ExecFn func(ctx context.Context, dir string, env []string, stdin []byte, name string, args ...string) (stdout []byte, stderr []byte, exitCode int, err error)

// StreamExecFn is the production execution path for live cursor-agent
// progress. It invokes onStdoutLine once per complete stdout line while
// the child is still running, then returns the full captured streams so
// Run can build the durable terminal Result exactly as before.
type StreamExecFn func(ctx context.Context, dir string, env []string, stdin []byte, name string, onStdoutLine func([]byte), args ...string) (stdout []byte, stderr []byte, exitCode int, err error)

// Options configures an Adapter at construction time.
type Options struct {
	// BinaryPath is the cursor-agent executable. Defaults to "cursor-agent"
	// (resolved against PATH).
	BinaryPath string
	// Args is an optional fixed argv tail (tests). When nil, argv is built
	// per Run from DefaultCursorModel and runner.Request.CursorModel.
	// When non-nil, Args is passed verbatim and DefaultCursorModel /
	// Request.CursorModel are ignored.
	Args []string
	// DefaultCursorModel is used when Request.CursorModel is empty (typical
	// app-settings default at worker construction).
	DefaultCursorModel string
	// Name is the runner.Name() value (recorded in TaskCyclePhase MetaJSON
	// by the worker). Defaults to "cursor-cli".
	Name string
	// Version is the runner.Version() value. Defaults to
	// "0.0.0-unknown"; binaries should override with the real value.
	Version string
	// ExecFn replaces os/exec for tests. nil means use the real exec path.
	ExecFn ExecFn
	// StreamExecFn replaces the live os/exec path for tests that need
	// incremental stdout delivery. When both ExecFn and StreamExecFn are
	// nil, the adapter uses the real streaming exec path. ExecFn takes
	// precedence to keep existing batch tests stable.
	StreamExecFn StreamExecFn
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
	binaryPath         string
	args               []string
	defaultCursorModel string
	name               string
	version            string
	exec               ExecFn
	streamExec         StreamExecFn
	extraKeys          []string
	homePaths          []string
}

// New returns a configured Adapter. Zero-value Options yields the V1
// defaults documented on Options.
func New(opts Options) *Adapter {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.New")
	a := &Adapter{
		binaryPath:         opts.BinaryPath,
		defaultCursorModel: strings.TrimSpace(opts.DefaultCursorModel),
		name:               opts.Name,
		version:            opts.Version,
		exec:               opts.ExecFn,
		streamExec:         opts.StreamExecFn,
		extraKeys:          append([]string(nil), opts.ExtraAllowedEnvKeys...),
		homePaths:          append([]string(nil), opts.HomePathReplacements...),
	}
	if len(opts.Args) > 0 {
		a.args = append([]string(nil), opts.Args...)
	}
	if a.binaryPath == "" {
		a.binaryPath = defaultBinaryPath
	}
	if a.name == "" {
		a.name = defaultName
	}
	if a.version == "" {
		a.version = defaultVersion
	}
	if a.exec == nil && a.streamExec == nil {
		a.streamExec = defaultStreamExecFn
	}
	if len(a.homePaths) == 0 {
		a.homePaths = liveHomePaths()
	}
	return a
}

func (a *Adapter) argvFor(req runner.Request) []string {
	if len(a.args) > 0 {
		return a.args
	}
	m := strings.TrimSpace(req.CursorModel)
	if m == "" {
		m = a.defaultCursorModel
	}
	// stream-json is a strict superset of json: it emits the same
	// terminal {"type":"result",...} event at the end, plus a
	// {"type":"system","subtype":"init","model":"<resolved>"} event
	// at the start that carries the concrete model cursor-agent
	// actually routed to (the only signal available when the
	// operator picked `auto` or the global default). Per
	// https://cursor.com/docs/cli/reference/output-format, the json
	// format explicitly does NOT include a model field. Switching to
	// stream-json is additive: the parser below still extracts the
	// same Summary / Details from the terminal result event, and
	// also captures resolved_model from the initial system event.
	out := []string{"--print", "--output-format", "stream-json"}
	if m != "" {
		out = append(out, "--model", m)
	}
	out = append(out, "--force")
	return out
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

// EffectiveModel implements runner.Runner. Mirrors the fallback applied
// inside argvFor: if req.CursorModel is set (after trimming) use it,
// otherwise fall back to the adapter's DefaultCursorModel from
// app_settings. Returns "" when neither is configured — the worker
// records that empty string verbatim into TaskCycle.MetaJSON so the
// audit trail can distinguish "operator picked the global default which
// happened to be unset" from "operator explicitly picked the global
// default which is opus".
func (a *Adapter) EffectiveModel(req runner.Request) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.EffectiveModel",
		"task_id", req.TaskID)
	m := strings.TrimSpace(req.CursorModel)
	if m != "" {
		return m
	}
	return a.defaultCursorModel
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
	argv := a.argvFor(req)
	var stdout, stderr []byte
	var exitCode int
	var execErr error
	if a.streamExec != nil {
		stdout, stderr, exitCode, execErr = a.streamExec(
			runCtx,
			req.WorkingDir,
			env,
			[]byte(req.Prompt),
			a.binaryPath,
			func(line []byte) {
				emitProgressFromLine(req.OnProgress, line, a.homePaths)
			},
			argv...,
		)
	} else {
		stdout, stderr, exitCode, execErr = a.exec(
			runCtx,
			req.WorkingDir,
			env,
			[]byte(req.Prompt),
			a.binaryPath,
			argv...,
		)
	}

	rawOutput := redact(combineStreams(stdout, stderr), a.homePaths)

	if execErr != nil && !isCtxErr(runCtx) && isClosedPipeReadError(execErr) && len(bytes.TrimSpace(stdout)) > 0 {
		slog.Debug("ignoring closed stdout pipe after cursor output",
			"cmd", cursorLogCmd, "operation", "cursor.Adapter.Run.closed_pipe_with_stdout",
			"stdout_bytes", len(stdout), "stderr_bytes", len(stderr), "err", execErr)
		execErr = nil
	}

	if execErr != nil {
		if isCtxErr(runCtx) {
			return runner.NewResult(domain.PhaseStatusFailed, timeoutSummary(req.Timeout),
					failureDetails("timeout", execErr, stdout, stderr, a.homePaths, map[string]any{
						"timeout_ns":         int64(req.Timeout),
						"timeout_configured": req.Timeout > 0,
					}), rawOutput),
				fmt.Errorf("cursor: %w: %v", runner.ErrTimeout, execErr)
		}
		return runner.NewResult(domain.PhaseStatusFailed, execFailedSummary(execErr, a.homePaths),
				failureDetails("exec", execErr, stdout, stderr, a.homePaths, map[string]any{
					"binary":      redact(a.binaryPath, a.homePaths),
					"argv":        argv,
					"working_dir": redact(req.WorkingDir, a.homePaths),
				}), rawOutput),
			fmt.Errorf("cursor: %w: %v", runner.ErrInvalidOutput, execErr)
	}

	if exitCode != 0 {
		details := stderrTailDetails(stderr, a.homePaths)
		combined := string(stderr) + "\n" + string(stdout)
		kind, stdMsg := classifyCursorFailure(combined)
		if kind != "" {
			details = mergeDetailsJSON(details, map[string]any{
				"failure_kind":         kind,
				"standardized_message": stdMsg,
			})
		}
		summary := fmt.Sprintf("cursor: exit %d", exitCode)
		switch kind {
		case FailureKindCursorUsageLimit:
			summary = titleForFailureKind(kind)
		default:
			if hint := stderrFirstLineHint(stderr, a.homePaths); hint != "" {
				summary = summary + ": " + hint
			}
		}
		return runner.NewResult(domain.PhaseStatusFailed, summary, details, rawOutput),
			fmt.Errorf("cursor: %w: exit %d", runner.ErrNonZeroExit, exitCode)
	}

	parsed, parseErr := parseStdout(stdout)
	if parseErr != nil {
		return runner.NewResult(domain.PhaseStatusFailed, invalidOutputSummary(parseErr, a.homePaths),
				failureDetails("parse_stdout", parseErr, stdout, stderr, a.homePaths, nil), rawOutput),
			fmt.Errorf("cursor: %w: %v", runner.ErrInvalidOutput, parseErr)
	}

	summary := redact(parsed.Result, a.homePaths)
	details := buildDetails(parsed)

	if parsed.IsError {
		if summary == "" {
			summary = "cursor: agent reported is_error=true"
		}
		res := runner.NewResult(domain.PhaseStatusFailed, summary, details, rawOutput)
		res.ResolvedModel = parsed.ResolvedModel
		return res, fmt.Errorf("cursor: %w: agent reported is_error=true", runner.ErrNonZeroExit)
	}

	res := runner.NewResult(domain.PhaseStatusSucceeded, summary, details, rawOutput)
	res.ResolvedModel = parsed.ResolvedModel
	return res, nil
}

// cursorOutput is the cursor-agent terminal `result` event. When the
// adapter invokes cursor-agent with `--output-format stream-json`
// (see argvFor) the stream ends with a line of this shape, identical
// to what `--output-format json` would have emitted as the sole
// output:
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
// ResolvedModel is NOT present in this event — cursor-agent only
// surfaces the concrete routed model through the earlier
// `{"type":"system","subtype":"init","model":"<display name>"}`
// event of the same NDJSON stream. parseStdout lifts it out of that
// system event and stores it on cursorOutput.ResolvedModel so the
// rest of the adapter can treat it as "one more field on the terminal
// result" without having to know about event ordering.
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
	// ResolvedModel is the display name cursor-agent reported having
	// routed to for this run (e.g. "Claude 4 Sonnet" when the
	// operator picked `auto`). Extracted from the stream-json
	// `system.init.model` field by parseStdout; empty when the
	// upstream stream did not surface one (older cursor-agent
	// version, or the adapter is being fed legacy single-object
	// output via test fixtures that predate stream-json).
	ResolvedModel string `json:"-"`
	// MissingTerminalResult is true when cursor-agent exited cleanly
	// but omitted the documented terminal result event. The adapter
	// degrades to the last complete assistant message only when the
	// stream has no unmatched tool calls.
	MissingTerminalResult bool `json:"-"`
}

// streamEventHead is the narrow view parseStdout uses to classify each
// NDJSON line without committing to one decoder per event type. The
// `model` field is only meaningful on the `system.init` event; other
// event types leave it zero-valued.
type streamEventHead struct {
	Type      string          `json:"type,omitempty"`
	Subtype   string          `json:"subtype,omitempty"`
	Model     string          `json:"model,omitempty"`
	CallID    string          `json:"call_id,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Message   progressMessage `json:"message,omitempty"`
}

// parseStdout decodes the cursor-agent print-mode output. It accepts
// two shapes:
//
//  1. stream-json (the format the adapter requests today): newline-
//     delimited JSON events. We walk every line and collect
//     - the `system.init.model` value as ResolvedModel, and
//     - the terminal `{"type":"result",...}` event as the result
//     envelope that populates Summary/Details. Non-`result`,
//     non-`system` events are ignored.
//
//  2. Single JSON object (legacy `--output-format json` output, and
//     what every unit-test fixture in this package emits). We detect
//     this by attempting one json.Unmarshal over the full buffer and
//     falling through on success. ResolvedModel stays "" in that case
//     — the legacy format never carried a model field anyway.
//
// Empty / whitespace-only stdout is rejected as invalid output so the
// caller maps it to runner.ErrInvalidOutput rather than silently
// treating it as a success with an empty summary.
func parseStdout(stdout []byte) (cursorOutput, error) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.parseStdout", "bytes", len(stdout))
	stdout = bytes.TrimSpace(stdout)
	if len(stdout) == 0 {
		return cursorOutput{}, errors.New("empty stdout")
	}

	// Fast-path for legacy single-object output (used by every
	// unit-test fixture and by older cursor-agent binaries that
	// only support --output-format json). When the whole buffer
	// parses cleanly as one object AND that object is a terminal
	// result event, skip the NDJSON walk.
	var single cursorOutput
	if err := json.Unmarshal(stdout, &single); err == nil && single.Type == "result" {
		return single, nil
	}

	var (
		out                cursorOutput
		gotResult          bool
		lastDecErr         error
		lastAssistantText  string
		lastSessionID      string
		openToolCalls      = map[string]struct{}{}
		openAnonymousTools int
	)
	for _, raw := range splitNDJSON(stdout) {
		if len(raw) == 0 {
			continue
		}
		var head streamEventHead
		if err := json.Unmarshal(raw, &head); err != nil {
			lastDecErr = err
			continue
		}
		switch head.Type {
		case "system":
			if head.Subtype == "init" && out.ResolvedModel == "" {
				out.ResolvedModel = strings.TrimSpace(head.Model)
			}
			if lastSessionID == "" {
				lastSessionID = strings.TrimSpace(head.SessionID)
			}
		case "assistant":
			if msg := strings.TrimSpace(textContent(head.Message.Content)); msg != "" {
				lastAssistantText = msg
			}
			if lastSessionID == "" {
				lastSessionID = strings.TrimSpace(head.SessionID)
			}
		case "tool_call":
			updateOpenToolCalls(openToolCalls, &openAnonymousTools, head)
			if lastSessionID == "" {
				lastSessionID = strings.TrimSpace(head.SessionID)
			}
		case "result":
			var evt cursorOutput
			if err := json.Unmarshal(raw, &evt); err != nil {
				lastDecErr = err
				continue
			}
			// Preserve any ResolvedModel captured from an
			// earlier system event — the terminal result
			// event does not carry that field and its own
			// ResolvedModel decode is always "".
			resolved := out.ResolvedModel
			out = evt
			out.ResolvedModel = resolved
			gotResult = true
		}
	}

	if !gotResult {
		if lastDecErr != nil {
			return cursorOutput{}, fmt.Errorf("decode stdout: %w", lastDecErr)
		}
		if open := openToolCallCount(openToolCalls, openAnonymousTools); open > 0 {
			return cursorOutput{}, fmt.Errorf("stream-json: no terminal result event; %d open tool call(s)", open)
		}
		if lastAssistantText != "" {
			return cursorOutput{
				Type:                  "result",
				Subtype:               "success",
				Result:                lastAssistantText,
				SessionID:             lastSessionID,
				ResolvedModel:         out.ResolvedModel,
				MissingTerminalResult: true,
			}, nil
		}
		return cursorOutput{}, errors.New("stream-json: no terminal result event")
	}
	return out, nil
}

func updateOpenToolCalls(open map[string]struct{}, openAnonymous *int, head streamEventHead) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.updateOpenToolCalls",
		"subtype", head.Subtype, "call_id", head.CallID)
	callID := strings.TrimSpace(head.CallID)
	switch head.Subtype {
	case "started", "start":
		if callID == "" {
			*openAnonymous = *openAnonymous + 1
			return
		}
		open[callID] = struct{}{}
	case "completed", "success", "done", "failed", "error":
		if callID == "" {
			if *openAnonymous > 0 {
				*openAnonymous = *openAnonymous - 1
			}
			return
		}
		delete(open, callID)
	}
}

func openToolCallCount(open map[string]struct{}, anonymous int) int {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.openToolCallCount",
		"open", len(open), "anonymous", anonymous)
	return len(open) + anonymous
}

// splitNDJSON splits NDJSON on literal newlines, tolerating CRLF,
// stripping blank lines, and returning the raw byte slices unchanged
// so each element can be fed straight to json.Unmarshal. No allocation
// of intermediate strings so the hot path on large streams stays
// GC-friendly.
func splitNDJSON(b []byte) [][]byte {
	if len(b) == 0 {
		return nil
	}
	out := make([][]byte, 0, 8)
	start := 0
	for i := 0; i < len(b); i++ {
		if b[i] != '\n' {
			continue
		}
		line := b[start:i]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		line = bytes.TrimSpace(line)
		if len(line) > 0 {
			out = append(out, line)
		}
		start = i + 1
	}
	if start < len(b) {
		tail := bytes.TrimSpace(b[start:])
		if len(tail) > 0 {
			out = append(out, tail)
		}
	}
	return out
}

// buildDetails serialises the cursor-agent metadata fields (everything
// other than "result") into the runner.Result.Details payload so the
// task_cycle_phases audit trail keeps the session/request IDs, timing
// breakdown, and token usage. The "result" text becomes Summary and
// is therefore intentionally elided here to avoid duplication.
func buildDetails(p cursorOutput) json.RawMessage {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.buildDetails",
		"type", p.Type, "subtype", p.Subtype, "is_error", p.IsError,
		"session_id", p.SessionID, "request_id", p.RequestID,
		"resolved_model", p.ResolvedModel)
	d := struct {
		Type          string          `json:"type,omitempty"`
		Subtype       string          `json:"subtype,omitempty"`
		IsError       bool            `json:"is_error,omitempty"`
		DurationMs    int64           `json:"duration_ms,omitempty"`
		DurationAPIMs int64           `json:"duration_api_ms,omitempty"`
		SessionID     string          `json:"session_id,omitempty"`
		RequestID     string          `json:"request_id,omitempty"`
		Usage         json.RawMessage `json:"usage,omitempty"`
		ResolvedModel string          `json:"resolved_model,omitempty"`
		MissingResult bool            `json:"missing_terminal_result,omitempty"`
	}{
		Type:          p.Type,
		Subtype:       p.Subtype,
		IsError:       p.IsError,
		DurationMs:    p.DurationMs,
		DurationAPIMs: p.DurationAPIMs,
		SessionID:     p.SessionID,
		RequestID:     p.RequestID,
		Usage:         p.Usage,
		ResolvedModel: p.ResolvedModel,
		MissingResult: p.MissingTerminalResult,
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

type progressMessage struct {
	Role    string            `json:"role,omitempty"`
	Content []progressContent `json:"content,omitempty"`
}

type progressContent struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

type progressEventLine struct {
	Type    string          `json:"type,omitempty"`
	Subtype string          `json:"subtype,omitempty"`
	Model   string          `json:"model,omitempty"`
	Name    string          `json:"name,omitempty"`
	Tool    string          `json:"tool,omitempty"`
	Message progressMessage `json:"message,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
}

func emitProgressFromLine(onProgress func(runner.ProgressEvent), raw []byte, homePaths []string) {
	if onProgress == nil {
		return
	}
	ev, ok := progressFromLine(raw, homePaths)
	if !ok {
		return
	}
	defer func() {
		_ = recover()
	}()
	onProgress(ev)
}

func progressFromLine(raw []byte, homePaths []string) (runner.ProgressEvent, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return runner.ProgressEvent{}, false
	}
	var line progressEventLine
	if err := json.Unmarshal(raw, &line); err != nil {
		return runner.ProgressEvent{}, false
	}
	switch line.Type {
	case "system":
		if line.Subtype == "init" && strings.TrimSpace(line.Model) != "" {
			return runner.ProgressEvent{
				Kind:    "system",
				Subtype: "init",
				Message: "Using " + strings.TrimSpace(line.Model),
				Payload: progressPayload(raw, homePaths),
			}, true
		}
	case "assistant":
		msg := clipSummaryRunes(redact(strings.TrimSpace(textContent(line.Message.Content)), homePaths), 240)
		if msg != "" {
			return runner.ProgressEvent{Kind: "assistant", Message: msg, Payload: progressPayload(raw, homePaths)}, true
		}
	case "tool_call":
		tool := firstNonEmpty(line.Name, line.Tool)
		subtype := strings.TrimSpace(line.Subtype)
		msg := toolProgressMessage(tool, subtype)
		return runner.ProgressEvent{
			Kind:    "tool_call",
			Subtype: subtype,
			Tool:    tool,
			Message: msg,
			Payload: progressPayload(raw, homePaths),
		}, true
	}
	return runner.ProgressEvent{}, false
}

func progressPayload(raw []byte, homePaths []string) json.RawMessage {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.progressPayload", "bytes", len(raw))
	redacted := []byte(redact(string(raw), homePaths))
	if !json.Valid(redacted) {
		return nil
	}
	return json.RawMessage(redacted)
}

func textContent(parts []progressContent) string {
	var b strings.Builder
	for _, part := range parts {
		if part.Type != "text" {
			continue
		}
		text := strings.TrimSpace(part.Text)
		if text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(text)
	}
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func toolProgressMessage(tool, subtype string) string {
	label := strings.TrimSpace(tool)
	if label == "" {
		label = "tool"
	}
	switch subtype {
	case "started", "start":
		return "Started " + label
	case "completed", "success", "done":
		return "Finished " + label
	case "failed", "error":
		return "Failed " + label
	default:
		return label
	}
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

// stderrFirstLineHint returns a short, redacted first non-empty line from
// stderr for human-readable summaries when cursor-agent exits non-zero.
func stderrFirstLineHint(stderr []byte, homePaths []string) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.stderrFirstLineHint",
		"stderr_bytes", len(stderr))
	if len(stderr) == 0 {
		return ""
	}
	normalized := strings.ReplaceAll(string(stderr), "\r\n", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		return clipSummaryRunes(redact(t, homePaths), stderrSummaryHintRunes)
	}
	return ""
}

func timeoutSummary(timeout time.Duration) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.timeoutSummary", "timeout_ns", int64(timeout))
	if timeout > 0 {
		return "cursor: timeout after " + timeout.String()
	}
	return "cursor: cancelled"
}

func execFailedSummary(err error, homePaths []string) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.execFailedSummary")
	if err == nil {
		return "cursor: exec failed"
	}
	msg := clipSummaryRunes(redact(strings.TrimSpace(err.Error()), homePaths), stderrSummaryHintRunes)
	if msg == "" {
		return "cursor: exec failed"
	}
	return "cursor: exec failed: " + msg
}

func invalidOutputSummary(err error, homePaths []string) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.invalidOutputSummary")
	if err == nil {
		return "cursor: invalid output"
	}
	msg := clipSummaryRunes(redact(strings.TrimSpace(err.Error()), homePaths), stderrSummaryHintRunes)
	if msg == "" {
		return "cursor: invalid output"
	}
	return "cursor: invalid output: " + msg
}

func clipSummaryRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= maxRunes {
			b.WriteRune('…')
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

func failureDetails(stage string, err error, stdout, stderr []byte, homePaths []string, extra map[string]any) json.RawMessage {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.failureDetails",
		"stage", stage, "stdout_bytes", len(stdout), "stderr_bytes", len(stderr))
	out := map[string]any{
		"failure_stage": stage,
	}
	if err != nil {
		out["error"] = redact(err.Error(), homePaths)
	}
	if tail := redactedTail(stdout, homePaths, diagnosticTailBytes); tail != "" {
		out["stdout_tail"] = tail
	}
	if tail := redactedTail(stderr, homePaths, diagnosticTailBytes); tail != "" {
		out["stderr_tail"] = tail
	}
	for k, v := range extra {
		out[k] = v
	}
	payload, marshalErr := json.Marshal(out)
	if marshalErr != nil {
		return json.RawMessage(`{"failure_stage":"details_marshal_failed"}`)
	}
	return payload
}

func redactedTail(b []byte, homePaths []string, maxBytes int) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.redactedTail",
		"bytes", len(b), "max_bytes", maxBytes)
	if len(b) == 0 || maxBytes <= 0 {
		return ""
	}
	tail := b
	if len(tail) > maxBytes {
		tail = trimLeadingPartialRune(tail[len(tail)-maxBytes:])
	}
	return redact(string(tail), homePaths)
}

func stderrTailDetails(stderr []byte, homePaths []string) json.RawMessage {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.stderrTailDetails",
		"stderr_bytes", len(stderr))
	tail := stderr
	if len(tail) > stderrTailBytes {
		tail = tail[len(tail)-stderrTailBytes:]
		// The byte-cap cut may land in the middle of a multibyte UTF-8
		// rune. Drop any leading continuation bytes so the subsequent
		// string conversion (and downstream json.Marshal) does not
		// rewrite the dangling bytes to U+FFFD and leak a corrupted
		// diagnostic into Result.Details.stderr_tail. Mirrors the
		// pkgs/repo/read_preview_io.go::trimTrailingPartialRune fix.
		tail = trimLeadingPartialRune(tail)
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

// trimLeadingPartialRune drops up to 3 leading UTF-8 continuation bytes
// (10xxxxxx) so a buffer cut that landed mid-rune does not surface as
// U+FFFD after string conversion. The longest valid UTF-8 sequence is
// 4 bytes, so at most 3 continuation bytes can precede the next valid
// rune-start, which bounds the loop. Returns the input unchanged when
// the first byte is already a rune-start (the common case).
// Compile-time assertion that *Adapter implements runner.Runner.
var _ runner.Runner = (*Adapter)(nil)
