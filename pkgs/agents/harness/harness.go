package harness

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const harnessLogCmd = "taskapi"

// CancelledByOperatorReason is the cycle/phase termination reason
// recorded when an operator hits "Cancel current run" from the
// settings page (POST /settings/cancel-current-run).
const CancelledByOperatorReason = "cancelled_by_operator"

// DefaultShutdownAbortTimeout bounds the post-cancel best-effort writes
// (CompletePhase + TerminateCycle + Update task) that run on a
// non-cancelled background context after the parent ctx fires.
const DefaultShutdownAbortTimeout = 5 * time.Second

// PanicReason is the cycle/phase termination reason recorded when the
// recover path fires after a runner or store panic.
const PanicReason = "panic"

// DefaultReportDirSubdir is the leaf directory the harness manages
// under os.TempDir() for agent↔worker side-channel report files.
const DefaultReportDirSubdir = "t2a-worker"

// ShutdownReason is the termination reason written when the parent
// context cancels mid-run.
const ShutdownReason = "shutdown"

// completePhaseFailedReason is the cycle termination reason written when
// the harness successfully ran the runner but failed to persist the
// terminal status onto the execute phase row.
const completePhaseFailedReason = "complete_phase_failed"

// checklistCompletionFailedReason is the cycle termination reason
// written when the runner reported success but checklist bookkeeping failed.
const checklistCompletionFailedReason = "checklist_completion_failed"

// CycleChangeNotifier is the optional SSE seam. cmd/taskapi wires an
// adapter that calls hub.Publish(handler.TaskCycleChanged{...}); tests
// pass nil and every PublishCycleChange call becomes a no-op.
//
// Implementations MUST NOT block: the harness invokes PublishCycleChange
// synchronously after each cycle/phase write.
type CycleChangeNotifier interface {
	PublishCycleChange(taskID, cycleID string)
}

// ProgressNotifier is the optional live-progress SSE seam.
//
// Implementations MUST NOT block: the harness invokes PublishRunProgress from
// the runner callback while the child process is still executing.
type ProgressNotifier interface {
	PublishRunProgress(taskID, cycleID string, phaseSeq int64, ev runner.ProgressEvent)
}

// Options bundles the per-Harness tunables. Zero values pick documented
// defaults so cmd/taskapi can construct a Harness without filling in
// every field.
type Options struct {
	RunTimeout           time.Duration
	StreamIdleStuck      time.Duration
	ShutdownAbortTimeout time.Duration
	WorkingDir           string
	ReportDir            string
	Notifier             CycleChangeNotifier
	ProgressNotifier     ProgressNotifier
	VerifyRunner         runner.Runner
	Metrics              RunMetrics
	Clock                func() time.Time
}

// Harness drives one task end-to-end through the execute/verify substrate.
// Construct with New; call Run from the worker after admission checks pass.
type Harness struct {
	store  *store.Store
	runner runner.Runner
	opts   Options
	git    *git.Service
	resume *resume.Service
	verify *verify.Service

	mu               sync.Mutex
	currentRunCancel context.CancelFunc
	cancelByOperator atomic.Bool
}

// New constructs a Harness with sensible defaults applied to opts.
func New(st *store.Store, r runner.Runner, opts Options) *Harness {
	if opts.ShutdownAbortTimeout <= 0 {
		opts.ShutdownAbortTimeout = DefaultShutdownAbortTimeout
	}
	if opts.Clock == nil {
		opts.Clock = func() time.Time {
			return time.Now().UTC()
		}
	}
	if opts.ReportDir == "" {
		opts.ReportDir = filepath.Join(os.TempDir(), DefaultReportDirSubdir)
	}
	return &Harness{
		store:  st,
		runner: r,
		opts:   opts,
		git:    git.NewService(st, git.NewExecRepo(), opts.ReportDir),
	}
}

// CancelCurrentRun cancels the in-flight runner.Run, if any.
func (h *Harness) CancelCurrentRun() bool {
	if h == nil {
		return false
	}
	h.mu.Lock()
	cancel := h.currentRunCancel
	h.mu.Unlock()
	if cancel == nil {
		return false
	}
	h.cancelByOperator.Store(true)
	cancel()
	slog.Info("agent harness run cancelled by operator", "cmd", harnessLogCmd,
		"operation", "agent.harness.Harness.CancelCurrentRun.fired")
	return true
}

func (h *Harness) setCurrentRunCancel(cancel context.CancelFunc) {
	h.mu.Lock()
	h.currentRunCancel = cancel
	h.mu.Unlock()
}

func (h *Harness) consumeOperatorCancel() bool {
	return h.cancelByOperator.Swap(false)
}

func (h *Harness) publish(taskID, cycleID string) {
	if h.opts.Notifier == nil {
		return
	}
	h.opts.Notifier.PublishCycleChange(taskID, cycleID)
}

func (h *Harness) publishProgress(taskID, cycleID string, phaseSeq int64, ev runner.ProgressEvent) {
	if h.opts.ProgressNotifier == nil || ev.Kind == "" {
		return
	}
	h.opts.ProgressNotifier.PublishRunProgress(taskID, cycleID, phaseSeq, ev)
}
