package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

const runnerLogCmd = "taskapi"

// MaxResultRawOutputBytes caps Result.RawOutput at 64 KiB after the adapter
// has already redacted secrets. Larger blobs are clipped to the trailing
// MaxResultRawOutputBytes bytes (errors usually surface near the end of CLI
// output) and Result.Truncated is set so the audit trail records the loss.
const MaxResultRawOutputBytes = 64 * 1024

// MaxResultDetailsBytes caps Result.Details at 16 KiB. Oversized details are
// replaced with a sentinel JSON object (see NewResult) so consumers always
// see well-formed JSON.
const MaxResultDetailsBytes = 16 * 1024

// MaxSummaryRunes caps Result.Summary at 512 runes; longer values are
// clipped at construction time. Summary is meant to be a one-screen
// human-readable note, not a log.
const MaxSummaryRunes = 512

// Runner is the contract a single agent invocation must satisfy. Run is the
// per-phase entry point; Name and Version identify the adapter for audit
// purposes (they are recorded in TaskCyclePhase.MetaJSON by the worker).
//
// Implementations MUST be safe for concurrent use: the worker may invoke
// multiple Runs in parallel for different tasks.
type Runner interface {
	Run(ctx context.Context, req Request) (Result, error)
	Name() string
	Version() string
}

// Request is the per-attempt input passed to Runner.Run. The JSON shape is
// pinned by runner_test.go; see package doc for the wire-format contract.
//
// Env should contain ONLY entries the adapter is willing to forward to the
// underlying tool. Cursor adapter convention (Stage 2 of
// docs/AGENT-WORKER-PLAN.md) is to pass through PATH and HOME and nothing
// else by default; everything else must be explicitly allowlisted by the
// caller.
type Request struct {
	TaskID     string            `json:"task_id"`
	AttemptSeq int64             `json:"attempt_seq"`
	Phase      domain.Phase      `json:"phase"`
	Prompt     string            `json:"prompt"`
	WorkingDir string            `json:"working_dir"`
	Timeout    time.Duration     `json:"timeout_ns"`
	Env        map[string]string `json:"env,omitempty"`
}

// Result is what Runner.Run returns. On wrapped error returns
// (ErrTimeout, ErrNonZeroExit, ErrInvalidOutput) the Result is still
// populated so callers can persist the partial outcome on the phase row.
//
// Always construct Results through NewResult so the byte caps and the
// Truncated marker are applied consistently.
type Result struct {
	Status    domain.PhaseStatus `json:"status"`
	Summary   string             `json:"summary,omitempty"`
	Details   json.RawMessage    `json:"details,omitempty"`
	RawOutput string             `json:"raw_output,omitempty"`
	Truncated bool               `json:"truncated,omitempty"`
}

// Typed errors. Adapters wrap these (fmt.Errorf("%w", ErrTimeout)) so
// callers can use errors.Is for recoverable-failure classification.
var (
	// ErrTimeout indicates the underlying tool exceeded Request.Timeout.
	// The Result returned alongside should carry whatever partial output
	// was captured before the kill.
	ErrTimeout = errors.New("runner: timeout")

	// ErrNonZeroExit indicates the underlying tool exited with a non-zero
	// status. The Result.Status is typically PhaseStatusFailed.
	ErrNonZeroExit = errors.New("runner: non-zero exit")

	// ErrInvalidOutput indicates the runner could not produce a usable
	// Result (e.g. malformed JSON from the tool, or no script entry in
	// the fake runner). The Result returned is the zero value.
	ErrInvalidOutput = errors.New("runner: invalid output")
)

// NewResult constructs a Result with byte/rune caps already applied. It is
// the only correct way to build a Result that crosses the runner boundary:
// it guarantees Truncated is set whenever any field was clipped, so the
// dual-write into task_cycle_phases never silently loses bytes.
//
// Clipping policy:
//
//   - Summary: clipped to the first MaxSummaryRunes runes (rune-correct,
//     never mid-codepoint).
//   - RawOutput: clipped to the trailing MaxResultRawOutputBytes bytes;
//     errors usually surface near the end of CLI output. Clipping happens
//     at a UTF-8 boundary so the result is still valid UTF-8.
//   - Details: when the marshalled bytes exceed MaxResultDetailsBytes, the
//     value is replaced with a sentinel JSON object so consumers always
//     see well-formed JSON:
//     {"truncated":true,"original_bytes":N}
//
// Adapters MUST redact secrets BEFORE calling NewResult. The runner package
// only enforces shape and size, not content.
func NewResult(status domain.PhaseStatus, summary string, details json.RawMessage, rawOutput string) Result {
	slog.Debug("trace", "cmd", runnerLogCmd, "operation", "runner.NewResult",
		"status", string(status), "summary_runes", utf8.RuneCountInString(summary),
		"details_bytes", len(details), "raw_output_bytes", len(rawOutput))

	clippedSummary, summaryClipped := clipSummary(summary)
	clippedRaw, rawClipped := clipRawOutput(rawOutput)
	clippedDetails, detailsClipped := clipDetails(details)

	return Result{
		Status:    status,
		Summary:   clippedSummary,
		Details:   clippedDetails,
		RawOutput: clippedRaw,
		Truncated: summaryClipped || rawClipped || detailsClipped,
	}
}

// clipSummary clips s to the first MaxSummaryRunes runes. Returns the
// clipped string and whether clipping occurred.
func clipSummary(s string) (string, bool) {
	slog.Debug("trace", "cmd", runnerLogCmd, "operation", "runner.clipSummary")
	if utf8.RuneCountInString(s) <= MaxSummaryRunes {
		return s, false
	}
	runes := []rune(s)
	return string(runes[:MaxSummaryRunes]), true
}

// clipRawOutput clips s to the trailing MaxResultRawOutputBytes bytes,
// snapping forward to the next UTF-8 boundary so the result is valid UTF-8.
// Returns the clipped string and whether clipping occurred.
func clipRawOutput(s string) (string, bool) {
	slog.Debug("trace", "cmd", runnerLogCmd, "operation", "runner.clipRawOutput")
	if len(s) <= MaxResultRawOutputBytes {
		return s, false
	}
	start := len(s) - MaxResultRawOutputBytes
	for start < len(s) && !utf8.RuneStart(s[start]) {
		start++
	}
	return s[start:], true
}

// clipDetails returns a JSON-safe Details payload that fits inside
// MaxResultDetailsBytes. When the input is over budget it is replaced with
// a sentinel object so consumers never have to handle malformed JSON.
func clipDetails(d json.RawMessage) (json.RawMessage, bool) {
	slog.Debug("trace", "cmd", runnerLogCmd, "operation", "runner.clipDetails", "bytes", len(d))
	if len(d) <= MaxResultDetailsBytes {
		return d, false
	}
	sentinel := fmt.Sprintf(`{"truncated":true,"original_bytes":%d}`, len(d))
	return json.RawMessage(sentinel), true
}
