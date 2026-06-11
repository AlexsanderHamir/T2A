package harness

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// Resume continues an in-flight cycle after process interruption. The task
// must already be StatusRunning and cycle must be StatusRunning. The worker
// calls this after FinalizeInterruptedPhases and queue admission.
func (h *Harness) Resume(parentCtx context.Context, task *domain.Task, cycle *domain.TaskCycle) {
	slog.Info("agent harness resume", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.Resume",
		"task_id", task.ID, "cycle_id", cycle.ID, "attempt_seq", cycle.AttemptSeq)
	startedAt := h.opts.Clock()
	cp, err := h.reconstructCheckpoint(parentCtx, cycle)
	if err != nil {
		slog.Warn("agent harness resume checkpoint failed", "cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.Resume.checkpoint_err",
			"task_id", task.ID, "cycle_id", cycle.ID, "err", err)
		h.bestEffortFailTask(parentCtx, task.ID)
		return
	}

	state := processState{
		cycleID:          cycle.ID,
		cycleStarted:     true,
		startedAt:        startedAt,
		previouslyPassed: cp.previouslyPassed,
		verifyAttempt:    cp.verifyAttempt,
		verifyFeedback:   cp.verifyFeedback,
		effectiveModel:   effectiveModelFromCycleMeta(h.runner, task, cycle),
	}
	state.verifySnap, _ = h.loadVerificationSnapshot(parentCtx, task.ID)

	defer h.recoverFromPanic(&state, *task)

	opts := cycleLoopOpts{}
	switch cp.entry {
	case resumeEntryExecute:
		opts.resumeNotice = true
		opts.interruptedPhase = domain.PhaseExecute
	case resumeEntryVerifyOnly:
		opts.resumeNotice = false
		opts.skipFirstExecute = true
		opts.interruptedPhase = domain.PhaseVerify
	case resumeEntryAfterExecuteSuccess:
		opts.skipFirstExecute = true
	}

	slog.Info("agent harness resume branch", "cmd", harnessLogCmd,
		"operation", "agent.harness.Harness.Resume.branch",
		"task_id", task.ID, "cycle_id", cycle.ID,
		"entry", cp.entry, "locked_count", len(cp.previouslyPassed),
		"verify_attempt", cp.verifyAttempt)

	h.runCycleLoop(parentCtx, task, cycle, &state, opts)
}
