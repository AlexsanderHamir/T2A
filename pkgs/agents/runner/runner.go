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
	// EffectiveModel returns the concrete model identifier the adapter
	// would use for req after applying its own defaults (e.g. the cursor
	// adapter falls back from req.CursorModel to its DefaultCursorModel
	// from app_settings). Pure function: MUST NOT touch the network or
	// the filesystem; called from the worker on the hot path before each
	// cycle starts so the value can be recorded in TaskCycle.MetaJSON
	// (cursor_model_effective) and emitted as the Prometheus model
	// label.
	//
	// May return "" when no model is configured anywhere — that is the
	// truthful audit value, NOT a placeholder. Callers MUST NOT
	// substitute their own default; the empty string is what the
	// breakdown panel renders as "default model" for pre-feature
	// cycles.
	EffectiveModel(req Request) string
}

// ProgressEvent is an ephemeral, best-effort update emitted while a runner is
// still executing. It is intended for live UI feedback only; terminal phase
// rows and task_events remain the durable audit trail.
type ProgressEvent struct {
	Kind    string `json:"kind"`
	Subtype string `json:"subtype,omitempty"`
	Message string `json:"message,omitempty"`
	Tool    string `json:"tool,omitempty"`
}

// Request is the per-attempt input passed to Runner.Run. The JSON shape is
// pinned by runner_test.go; see package doc for the wire-format contract.
//
// Env should contain ONLY entries the adapter is willing to forward to the
// underlying tool. Cursor adapter convention (see docs/AGENT-WORKER.md
// "Security model") is to pass through PATH and HOME and nothing else
// by default; everything else must be explicitly allowlisted by the
// caller.
type Request struct {
	TaskID     string            `json:"task_id"`
	AttemptSeq int64             `json:"attempt_seq"`
	Phase      domain.Phase      `json:"phase"`
	Prompt     string            `json:"prompt"`
	WorkingDir string            `json:"working_dir"`
	Timeout    time.Duration     `json:"timeout_ns"`
	Env        map[string]string `json:"env,omitempty"`
	// CursorModel is optional per-run model selection for the Cursor CLI
	// adapter. Empty means use the adapter default (from app settings at
	// worker construction).
	CursorModel string `json:"cursor_model,omitempty"`
	// OnProgress is an optional live-update callback. It is excluded from
	// JSON so the persisted/tested request wire shape stays stable.
	OnProgress func(ProgressEvent) `json:"-"`
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
	// ResolvedModel is the concrete model the underlying tool reported
	// having used for this run, distinct from EffectiveModel which is
	// the intent-level identifier resolved before the run starts. Set
	// only by adapters that can observe the tool's own "which model did
	// I pick" signal — e.g. the cursor adapter reads it from the
	// `system.init.model` event of `--output-format stream-json`, which
	// is the only cursor-agent surface that reveals the actual routed
	// model when the operator picked `auto`. Empty string means the
	// adapter had no way to observe a resolved model for this run (not
	// an error) and callers MUST treat it as "unknown", not substitute
	// a default.
	ResolvedModel string `json:"resolved_model,omitempty"`
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
// MaxResultDetailsBytes AND is well-formed JSON. When the input is
// over budget OR contains invalid JSON it is replaced with a sentinel
// object so consumers (worker dual-write into TaskCyclePhase.MetaJSON,
// SPA, log shipper) never have to handle malformed JSON. nil and the
// empty slice are passed through unchanged so "no details" stays
// distinguishable from "had details but they were lost".
func clipDetails(d json.RawMessage) (json.RawMessage, bool) {
	slog.Debug("trace", "cmd", runnerLogCmd, "operation", "runner.clipDetails", "bytes", len(d))
	if len(d) == 0 {
		return d, false
	}
	if len(d) > MaxResultDetailsBytes {
		sentinel := fmt.Sprintf(`{"truncated":true,"original_bytes":%d}`, len(d))
		return json.RawMessage(sentinel), true
	}
	if !json.Valid(d) {
		// Invalid-but-under-cap: still emit the sentinel so the
		// "well-formed JSON" contract on Result.Details holds. We
		// reuse the same shape as the size-overflow sentinel — the
		// truncated flag carries the "details were lost" signal,
		// and original_bytes preserves the size for forensic
		// analysis. Distinguishing invalid-vs-oversize is not
		// worth a wire-format change today; downstream consumers
		// only need to know "we didn't get usable details".
		sentinel := fmt.Sprintf(`{"truncated":true,"original_bytes":%d}`, len(d))
		return json.RawMessage(sentinel), true
	}
	return d, false
}
