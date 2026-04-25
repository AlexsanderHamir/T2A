// Package runnerfake provides a deterministic in-memory implementation of
// runner.Runner used by every V1 worker test (contract:
// docs/AGENT-WORKER.md). The fake is keyed on (TaskID, Phase) so tests
// can script the outcome of each phase without depending on a real
// CLI.
//
// The fake is exported (capital R Runner) so test files in other packages
// can construct it directly: runnerfake.New() returns a *Runner that
// satisfies runner.Runner.
package runnerfake

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

const runnerfakeLogCmd = "taskapi"

// Runner is a deterministic fake implementation of runner.Runner.
//
// Tests register expected outcomes with Script (success) or Fail (error
// path), then assert against Calls() afterwards. Run lookups that have no
// matching script return runner.ErrInvalidOutput so missing scripts surface
// as test failures rather than silently passing.
type Runner struct {
	name         string
	version      string
	defaultModel string

	mu      sync.Mutex
	scripts map[scriptKey]scripted
	calls   []runner.Request
}

type scriptKey struct {
	taskID string
	phase  domain.Phase
}

type scripted struct {
	result runner.Result
	err    error
}

// New returns a fake runner with default name "fake" and version "v0".
// Override either via WithName / WithVersion before scripting if a test
// needs to assert on Runner.Name / Runner.Version (the worker records
// these in TaskCyclePhase.MetaJSON).
func New() *Runner {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.New")
	return &Runner{
		name:    "fake",
		version: "v0",
		scripts: make(map[scriptKey]scripted),
	}
}

// WithName overrides the value returned by Name().
func (r *Runner) WithName(name string) *Runner {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.WithName", "name", name)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.name = name
	return r
}

// WithVersion overrides the value returned by Version().
func (r *Runner) WithVersion(version string) *Runner {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.WithVersion", "version", version)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.version = version
	return r
}

// WithDefaultModel sets the model that EffectiveModel returns when
// req.CursorModel is empty. Mirrors the cursor adapter's
// DefaultCursorModel option so worker tests can pin the
// cursor_model_effective audit value end-to-end.
func (r *Runner) WithDefaultModel(model string) *Runner {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.WithDefaultModel", "model", model)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultModel = model
	return r
}

// Script registers result as the value Run will return for (taskID, phase).
// Last write wins. Result is stored as-is; tests should typically build it
// via runner.NewResult so caps are applied.
func (r *Runner) Script(taskID string, phase domain.Phase, result runner.Result) {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.Script",
		"task_id", taskID, "phase", string(phase))
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scripts[scriptKey{taskID: taskID, phase: phase}] = scripted{result: result}
}

// Fail registers err as the error Run will return for (taskID, phase). The
// accompanying result is the zero Result (mirroring the contract of
// runner.ErrInvalidOutput).
func (r *Runner) Fail(taskID string, phase domain.Phase, err error) {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.Fail",
		"task_id", taskID, "phase", string(phase), "err", err)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scripts[scriptKey{taskID: taskID, phase: phase}] = scripted{err: err}
}

// FailWithResult registers (result, err) as the pair Run will return. Used
// when the adapter contract requires both a partial Result and a typed
// error (e.g. ErrNonZeroExit with the captured RawOutput).
func (r *Runner) FailWithResult(taskID string, phase domain.Phase, result runner.Result, err error) {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.FailWithResult",
		"task_id", taskID, "phase", string(phase), "err", err)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scripts[scriptKey{taskID: taskID, phase: phase}] = scripted{result: result, err: err}
}

// Run looks up the scripted outcome for (req.TaskID, req.Phase). When no
// script is registered it returns runner.ErrInvalidOutput so missing
// expectations fail tests loudly. Run honours ctx cancellation: a cancelled
// context returns ctx.Err() wrapped with runner.ErrTimeout so callers can
// errors.Is against the typed-error contract.
func (r *Runner) Run(ctx context.Context, req runner.Request) (runner.Result, error) {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.Run",
		"task_id", req.TaskID, "phase", string(req.Phase), "attempt_seq", req.AttemptSeq)

	if err := ctx.Err(); err != nil {
		return runner.Result{}, fmt.Errorf("runnerfake: %w: %v", runner.ErrTimeout, err)
	}

	r.mu.Lock()
	r.calls = append(r.calls, req)
	entry, ok := r.scripts[scriptKey{taskID: req.TaskID, phase: req.Phase}]
	r.mu.Unlock()

	if !ok {
		return runner.Result{}, fmt.Errorf("runnerfake: %w: no script for (task_id=%s, phase=%s)",
			runner.ErrInvalidOutput, req.TaskID, req.Phase)
	}
	return entry.result, entry.err
}

// Name returns the configured runner name (default "fake").
func (r *Runner) Name() string {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.Name")
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.name
}

// Version returns the configured runner version (default "v0").
func (r *Runner) Version() string {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.Version")
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.version
}

// EffectiveModel implements runner.Runner. Mirrors the cursor adapter
// fallback: trim req.CursorModel and use it when non-empty; otherwise
// fall back to the value set via WithDefaultModel (default "").
func (r *Runner) EffectiveModel(req runner.Request) string {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.EffectiveModel",
		"task_id", req.TaskID)
	m := strings.TrimSpace(req.CursorModel)
	if m != "" {
		return m
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.defaultModel
}

// Calls returns a copy of every Request seen by Run, in invocation order.
// Tests use this to assert on what the worker sent to the runner.
func (r *Runner) Calls() []runner.Request {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.Calls")
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]runner.Request, len(r.calls))
	copy(out, r.calls)
	return out
}

// Reset clears recorded calls and registered scripts. Useful in
// table-driven tests that share one *Runner across subtests.
func (r *Runner) Reset() {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.Reset")
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scripts = make(map[scriptKey]scripted)
	r.calls = nil
}

// Compile-time assertion that *Runner implements runner.Runner.
var _ runner.Runner = (*Runner)(nil)
