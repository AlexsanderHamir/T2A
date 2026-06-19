package harness

import (
	"context"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

type startCycleOpts struct {
	parentCycleID *string
	retryMode     domain.RetryMode
}

// RunWithRetry starts a new cycle. intent==nil is the existing first-run path.
func (h *Harness) RunWithRetry(parentCtx context.Context, task *domain.Task, intent *domain.PendingRetry) {
	if intent == nil {
		h.runFreshCycle(parentCtx, task, startCycleOpts{})
		return
	}
	if err := intent.Validate(); err != nil {
		slog.Warn("agent harness retry intent invalid", "cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.RunWithRetry.invalid_intent",
			"task_id", task.ID, "err", err)
		h.resumeSvc().FailTaskAfterRetryPrep(parentCtx, task.ID, "retry_invalid_intent")
		return
	}
	switch intent.Mode {
	case domain.RetryFresh:
		h.runFreshRetry(parentCtx, task, intent)
	case domain.RetryResume:
		h.runResumeRetry(parentCtx, task, intent)
	default:
		h.resumeSvc().FailTaskAfterRetryPrep(parentCtx, task.ID, "retry_invalid_intent")
	}
}

func (h *Harness) runFreshRetry(parentCtx context.Context, task *domain.Task, intent *domain.PendingRetry) {
	if _, err := h.gitResetForFreshRetry(parentCtx, intent.ParentCycleID); err != nil {
		reason := retryGitResetFailed
		if strings.Contains(err.Error(), retryResetAnchorMissing) {
			reason = retryResetAnchorMissing
		}
		slog.Warn("agent harness fresh retry git reset failed", "cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.runFreshRetry.reset_err",
			"task_id", task.ID, "parent_cycle_id", intent.ParentCycleID, "err", err)
		h.resumeSvc().FailTaskAfterRetryPrep(parentCtx, task.ID, reason)
		return
	}
	parentID := intent.ParentCycleID
	h.runFreshCycle(parentCtx, task, startCycleOpts{
		parentCycleID: &parentID,
		retryMode:     domain.RetryFresh,
	})
}

func (h *Harness) runResumeRetry(parentCtx context.Context, task *domain.Task, intent *domain.PendingRetry) {
	cp, err := h.loadCheckpointFromParent(parentCtx, intent.ParentCycleID)
	if err != nil {
		slog.Warn("agent harness resume retry checkpoint failed", "cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.runResumeRetry.checkpoint_err",
			"task_id", task.ID, "parent_cycle_id", intent.ParentCycleID, "err", err)
		h.resumeSvc().FailTaskAfterRetryPrep(parentCtx, task.ID, "retry_checkpoint_failed")
		return
	}
	startedAt := h.opts.Clock()
	state := processState{
		startedAt:        startedAt,
		previouslyPassed: harnessVerdictsFromResume(cp.PreviouslyPassed),
		verifyAttempt:    0,
		verifyFeedback:   cp.VerifyFeedback,
	}
	defer h.recoverFromPanic(&state, *task)

	parentID := intent.ParentCycleID
	cycle, ok := h.startCycle(parentCtx, task, &state, startCycleOpts{
		parentCycleID: &parentID,
		retryMode:     domain.RetryResume,
	})
	if !ok {
		h.bestEffortFailTask(parentCtx, task.ID)
		return
	}
	if cp.Entry == resumeEntryVerifyOnly {
		if err := h.resumeSvc().SeedCrossCycleExecuteFromParent(parentCtx, cycle, intent.ParentCycleID); err != nil {
			slog.Warn("agent harness verify-only resume seed execute failed", "cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.runResumeRetry.seed_execute_err",
				"task_id", task.ID, "parent_cycle_id", intent.ParentCycleID, "err", err)
			h.resumeSvc().FailTaskAfterRetryPrep(parentCtx, task.ID, "retry_verify_only_seed_failed")
			return
		}
		if err := h.resumeSvc().MirrorParentCriteriaForVerifyOnly(parentCtx, cycle.ID, intent.ParentCycleID); err != nil {
			slog.Warn("agent harness verify-only resume mirror criteria failed", "cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.runResumeRetry.mirror_criteria_err",
				"task_id", task.ID, "parent_cycle_id", intent.ParentCycleID, "err", err)
			h.resumeSvc().FailTaskAfterRetryPrep(parentCtx, task.ID, "retry_verify_only_mirror_failed")
			return
		}
	}
	state.verifySnap, _ = h.loadVerificationSnapshot(parentCtx, task.ID)
	h.runCycleLoop(parentCtx, task, cycle, &state, cycleLoopOpts{
		resumeNotice:     true,
		interruptedPhase: domain.PhaseExecute,
		skipFirstExecute: cp.Entry == resumeEntryVerifyOnly,
		knownCommits:     cp.KnownCommits,
		continuation:     cp.Continuation,
	})
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) runFreshCycle(parentCtx context.Context, task *domain.Task, opts startCycleOpts) {
	startedAt := h.opts.Clock()
	state := processState{startedAt: startedAt, previouslyPassed: map[string]criterionVerdict{}}
	defer h.recoverFromPanic(&state, *task)

	cycle, ok := h.startCycle(parentCtx, task, &state, opts)
	if !ok {
		h.bestEffortFailTask(parentCtx, task.ID)
		return
	}
	state.verifySnap, _ = h.loadVerificationSnapshot(parentCtx, task.ID)
	h.runCycleLoop(parentCtx, task, cycle, &state, cycleLoopOpts{})
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) reconstructCheckpoint(ctx context.Context, cycle *domain.TaskCycle) (resumeCheckpoint, error) {
	return h.resumeSvc().ReconstructCheckpoint(ctx, cycle)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) loadCheckpointFromParent(ctx context.Context, parentCycleID string) (resumeCheckpoint, error) {
	return h.resumeSvc().LoadCheckpointFromParent(ctx, parentCycleID)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) loadContinuationBundle(ctx context.Context, parentCycleID string) (ContinuationBundle, error) {
	return h.resumeSvc().LoadContinuationBundle(ctx, parentCycleID)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) seedCrossCycleExecuteFromParent(ctx context.Context, cycle *domain.TaskCycle, parentCycleID string) error {
	return h.resumeSvc().SeedCrossCycleExecuteFromParent(ctx, cycle, parentCycleID)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) mirrorParentCriteriaForVerifyOnly(ctx context.Context, childCycleID, parentCycleID string) error {
	return h.resumeSvc().MirrorParentCriteriaForVerifyOnly(ctx, childCycleID, parentCycleID)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) failTaskAfterRetryPrep(ctx context.Context, taskID, reason string) {
	h.resumeSvc().FailTaskAfterRetryPrep(ctx, taskID, reason)
}
