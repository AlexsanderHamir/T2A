// Package cursor adapts Cursor's headless CLI ("cursor-agent") to the
// runner.Runner contract from pkgs/agents/runner. One Adapter.Run call
// shells out exactly once, captures stdout/stderr, redacts secrets, and
// returns a runner.Result with byte caps already applied.
//
// # Invocation contract
//
// V1 invokes the binary as:
//
//	cursor-agent --print --output-format json --force
//
// with the prompt sent on stdin. The binary path is configurable via
// Options.BinaryPath; the argv tail is configurable via Options.Args.
// "--force" instructs cursor-agent to auto-approve filesystem and
// shell tool calls instead of blocking on an interactive prompt the
// worker has no way to answer; without it the child reliably wedges
// until Request.Timeout. Pin a comment here when the default flag set
// changes so callers know the wire format.
//
// The CLI emits a single JSON object on stdout with cursor-agent's
// own envelope:
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
// "result" becomes runner.Result.Summary (after redaction). Everything
// else (type, subtype, is_error, durations, session/request IDs,
// usage) is packed into runner.Result.Details so the
// task_cycle_phases audit trail keeps the runner-side metadata.
// Unknown top-level fields are silently ignored for forward
// compatibility with future cursor-agent metadata.
//
// # Environment policy
//
// The child process inherits NOTHING from the parent process by default.
// The adapter passes through only:
//
//   - PATH (so the binary can find tools)
//   - HOME (Unix), USERPROFILE (Windows)
//   - Anything explicitly placed in runner.Request.Env by the caller
//   - Anything explicitly placed in Options.ExtraAllowedEnvKeys
//
// The adapter ALWAYS strips entries whose key is "DATABASE_URL" or whose
// key has a "T2A_" prefix, even when the caller asked for them. Store
// credentials and T2A internals must never reach a runner. This is a
// belt-and-suspenders defense against caller mistakes.
//
// # Redaction
//
// Before runner.NewResult is called, the captured stdout+stderr is run
// through Redact, which replaces:
//
//   - "Authorization: <anything>" header values with "Authorization: [REDACTED]"
//   - any "T2A_FOO=value" assignment with "T2A_FOO=[REDACTED]"
//   - the contents of $HOME / $USERPROFILE in absolute paths with "~"
//
// Callers that need stricter redaction can wrap the adapter and post-
// process Result.RawOutput; the adapter's own redaction is a floor, not
// a ceiling.
//
// # Error mapping
//
//   - ctx.Err() (DeadlineExceeded or Canceled): runner.ErrTimeout
//   - non-zero exit code: runner.ErrNonZeroExit, with the redacted tail
//     of stderr in Result.Details under {"stderr_tail": "..."}
//   - cursor-agent reported {"is_error": true} (process still exits 0):
//     runner.ErrNonZeroExit with PhaseStatusFailed and the agent's own
//     "result" text as Summary
//   - exec start failure or stdout JSON parse failure:
//     runner.ErrInvalidOutput
//
// All errors are wrapped with fmt.Errorf("%w") so callers can use
// errors.Is for classification.
//
// # Test substrate
//
// All exec calls go through Options.ExecFn so cursor_test.go can drive
// every code path (success, non-zero, parse-fail, timeout, redaction)
// without invoking a real Cursor binary. Default ExecFn is
// defaultExecFn, which uses os/exec and is exercised only by integration
// tests gated under the "integration" build tag (none in V1).
package cursor
