package harness

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

type cycleLoopOpts struct {
	resumeNotice     bool
	interruptedPhase domain.Phase
	skipFirstExecute bool
	knownCommits     []domain.TaskCycleCommit
}

func (h *Harness) composeExecutePrompt(ctx context.Context, task *domain.Task, cycle *domain.TaskCycle, state *processState, opts cycleLoopOpts) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.composeExecutePrompt",
		"task_id", task.ID, "cycle_id", cycle.ID, "resume_notice", opts.resumeNotice)
	prompt := task.InitialPrompt
	if len(task.AutomationSelections) > 0 {
		resolved, err := h.store.ResolveAutomationsForTask(ctx, task.AutomationSelections)
		if err != nil {
			slog.Warn("agent harness resolve automations failed", "cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.composeExecutePrompt.resolveAutomations",
				"task_id", task.ID, "cycle_id", cycle.ID, "err", err)
		} else {
			if len(resolved) < len(task.AutomationSelections) {
				slog.Warn("agent harness skipped missing or archived automations", "cmd", harnessLogCmd,
					"operation", "agent.harness.Harness.composeExecutePrompt.resolveAutomations",
					"task_id", task.ID, "cycle_id", cycle.ID,
					"requested", len(task.AutomationSelections), "resolved", len(resolved))
			}
			prompt = injectAutomations(prompt, resolved)
		}
	}
	prompt = injectCriteria(
		prompt,
		state.verifySnap.criteria,
		cycle.ID,
		criteriaReportPath(h.opts.ReportDir, cycle.ID),
		state.previouslyPassed,
	)
	prompt = appendVerifyFeedback(prompt, state.verifyFeedback)
	if opts.resumeNotice {
		prompt = appendResumeNotice(prompt, cycle, opts.interruptedPhase, opts.knownCommits)
	}
	if !state.gitSnap.Skipped {
		prompt = appendGitCommitPolicy(prompt)
	}
	return prompt
}

func (h *Harness) repoRootForGit(ctx context.Context) string {
	settings, err := h.store.GetSettings(ctx)
	if err != nil {
		return strings.TrimSpace(h.opts.WorkingDir)
	}
	if v := strings.TrimSpace(settings.RepoRoot); v != "" {
		return v
	}
	return strings.TrimSpace(h.opts.WorkingDir)
}

func applyOperatorCancelToRunResult(result runner.Result, operatorCancelled bool, reason string) (runner.Result, string) {
	if !operatorCancelled {
		return result, reason
	}
	reason = CancelledByOperatorReason
	if result.Summary == "" || strings.HasPrefix(result.Summary, "cursor: timeout") {
		result.Summary = "cancelled by operator"
	}
	return result, reason
}

func applyExecuteCommitIngestOutcome(
	runErr error,
	operatorCancelled bool,
	snap gitPhaseSnapshot,
	cycleID string,
	ingestErr error,
	outcome executeCommitIngestOutcome,
	phaseStatus domain.PhaseStatus,
	cycleStatus domain.CycleStatus,
	taskStatus domain.Status,
	reason string,
	result runner.Result,
) (domain.PhaseStatus, domain.CycleStatus, domain.Status, string, runner.Result, int) {
	if runErr != nil || operatorCancelled || snap.Skipped {
		return phaseStatus, cycleStatus, taskStatus, reason, result, 0
	}
	if ingestErr != nil {
		slog.Warn("agent harness commit ingest error", "cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.runCycleLoop.commit_ingest_err",
			"cycle_id", cycleID, "err", ingestErr)
		result.Summary = executeInvalidCommitReason
		return domain.PhaseStatusFailed, domain.CycleStatusFailed, domain.StatusFailed,
			executeInvalidCommitReason, result, 0
	}
	if outcome.FailReason != "" {
		result.Summary = outcome.FailReason
		return domain.PhaseStatusFailed, domain.CycleStatusFailed, domain.StatusFailed,
			outcome.FailReason, result, 0
	}
	return phaseStatus, cycleStatus, taskStatus, reason, result, outcome.CommitCount
}

func recordPassedCriterionVerdicts(state *processState, verdicts []criterionVerdict) {
	for _, v := range verdicts {
		if !v.passed {
			continue
		}
		if _, exists := state.previouslyPassed[v.id]; !exists {
			state.previouslyPassed[v.id] = v
		}
	}
}

func unionPreviouslyPassedVerdicts(state *processState) []criterionVerdict {
	unionVerdicts := make([]criterionVerdict, 0, len(state.previouslyPassed))
	for _, v := range state.previouslyPassed {
		unionVerdicts = append(unionVerdicts, v)
	}
	return unionVerdicts
}

// runCycleLoopExecute runs one execute phase iteration. Returns false when
// runCycleLoop should return immediately.
func (h *Harness) runCycleLoopExecute(
	parentCtx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	state *processState,
	opts cycleLoopOpts,
) bool {
	execPhase, ok := h.startExecutePhase(parentCtx, cycle, state)
	if !ok {
		h.bestEffortTerminate(parentCtx, state, task.ID, domain.CycleStatusFailed, "execute_phase_start_failed")
		return false
	}
	priorBase, err := h.priorCycleBaseSHA(parentCtx, cycle.ID, execPhase.PhaseSeq)
	if err != nil {
		slog.Warn("agent harness prior cycle base lookup failed", "cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.runCycleLoop.prior_cycle_base",
			"cycle_id", cycle.ID, "err", err)
	}
	snap, err := captureExecuteGitSnapshot(parentCtx, h.repoRootForGit(parentCtx), h.opts.WorkingDir, priorBase)
	if err != nil {
		slog.Warn("agent harness git snapshot failed", "cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.runCycleLoop.git_snapshot",
			"cycle_id", cycle.ID, "err", err)
		h.bestEffortTerminate(parentCtx, state, task.ID, domain.CycleStatusFailed, "execute_git_snapshot_failed")
		return false
	}
	state.gitSnap = snap

	_ = scrubCycleArtifacts(h.opts.ReportDir, cycle.ID)
	_ = ensureReportCycleDir(h.opts.ReportDir, cycle.ID)
	prompt := h.composeExecutePrompt(parentCtx, task, cycle, state, opts)
	execTask := *task
	execTask.InitialPrompt = prompt

	result, runErr := h.invokeRunnerWithTask(parentCtx, &execTask, cycle, execPhase)
	operatorCancelled := h.consumeOperatorCancel()

	if parentCtx.Err() != nil {
		h.handleShutdownAfterRun(state, task.ID)
		return false
	}

	phaseStatus, cycleStatus, taskStatus, reason := classifyRunOutcome(runErr)
	result, reason = applyOperatorCancelToRunResult(result, operatorCancelled, reason)

	var ingestOutcome executeCommitIngestOutcome
	var ingestErr error
	if runErr == nil && !operatorCancelled && !snap.Skipped {
		ingestOutcome, ingestErr = h.ingestExecuteCommits(parentCtx, task.ID, cycle, execPhase.PhaseSeq, snap)
	}
	phaseStatus, cycleStatus, taskStatus, reason, result, commitCount := applyExecuteCommitIngestOutcome(
		runErr, operatorCancelled, snap, cycle.ID, ingestErr, ingestOutcome,
		phaseStatus, cycleStatus, taskStatus, reason, result,
	)
	phaseDetails := mergeRunnerDetailsWithGit(detailsBytes(result), snap, commitCount)

	if !h.completeExecutePhase(parentCtx, state, cycle, execPhase, phaseStatus, result, phaseDetails) {
		h.bestEffortTerminate(parentCtx, state, task.ID, domain.CycleStatusFailed, completePhaseFailedReason)
		return false
	}

	if runErr != nil || operatorCancelled || phaseStatus == domain.PhaseStatusFailed {
		if !h.terminateCycle(parentCtx, state, cycle.TaskID, cycleStatus, reason) {
			return false
		}
		if taskStatus == domain.StatusFailed {
			_ = h.transitionTask(parentCtx, task.ID, taskStatus, "final_task_transition")
		}
		return false
	}
	return true
}

// runCycleLoopVerify runs verification for one loop iteration. retryLoop is
// true when the outer loop should continue for another execute↔verify attempt.
// terminalFailure is true when verification failed terminally (caller should return).
func (h *Harness) runCycleLoopVerify(
	parentCtx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	state *processState,
) (retryLoop bool, terminalFailure bool) {
	if !state.verifySnap.enabled {
		if err := h.completeChecklistLegacy(parentCtx, task.ID); err != nil {
			slog.Warn("agent harness checklist completion failed",
				"cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.runCycleLoop.checklist_err",
				"task_id", task.ID, "err", err)
			cycleStatus := domain.CycleStatusFailed
			taskStatus := domain.StatusFailed
			reason := checklistCompletionFailedReason
			if !h.terminateCycle(parentCtx, state, cycle.TaskID, cycleStatus, reason) {
				return false, true
			}
			_ = h.transitionTask(parentCtx, task.ID, taskStatus, "final_task_transition")
			return false, true
		}
		return false, false
	}

	verdicts, feedback, verifyErr := h.runVerificationPipeline(parentCtx, task, cycle, state, state.verifySnap, state.verifyFeedback)
	if verifyErr != nil && feedback != "" {
		state.verifyFeedback = feedback
	}
	recordPassedCriterionVerdicts(state, verdicts)
	if verifyErr == nil {
		return false, false
	}

	var tampered *verifyTamperedError
	if errors.As(verifyErr, &tampered) {
		if !h.terminateCycle(parentCtx, state, cycle.TaskID, domain.CycleStatusFailed, verifyTamperedReason) {
			return false, true
		}
		_ = h.transitionTask(parentCtx, task.ID, domain.StatusFailed, "final_task_transition")
		return false, true
	}
	if state.verifyAttempt < state.verifySnap.maxRetries {
		state.verifyAttempt++
		return true, false
	}

	cycleStatus := domain.CycleStatusFailed
	taskStatus := domain.StatusFailed
	reason := formatVerificationFailedReason(verdicts, state.previouslyPassed)
	if !h.terminateCycle(parentCtx, state, cycle.TaskID, cycleStatus, reason) {
		return false, true
	}
	_ = h.transitionTask(parentCtx, task.ID, taskStatus, "final_task_transition")
	return false, true
}

func (h *Harness) runCycleLoopFinalizeSuccess(
	parentCtx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	state *processState,
	cycleStatus domain.CycleStatus,
	taskStatus domain.Status,
	reason string,
) {
	unionVerdicts := unionPreviouslyPassedVerdicts(state)
	if err := h.applyVerifiedCompletions(parentCtx, task.ID, cycle.ID, unionVerdicts); err != nil {
		cycleStatus = domain.CycleStatusFailed
		taskStatus = domain.StatusFailed
		reason = checklistCompletionFailedReason
	}
	if !h.terminateCycle(parentCtx, state, cycle.TaskID, cycleStatus, reason) {
		return
	}
	if !h.transitionTask(parentCtx, task.ID, taskStatus, "final_task_transition") {
		return
	}
	h.publish(task.ID, cycle.ID)
	slog.Info("agent harness run complete", "cmd", harnessLogCmd,
		"operation", "agent.harness.Harness.runCycleLoop.summary",
		"task_id", task.ID, "cycle_id", cycle.ID, "attempt_seq", cycle.AttemptSeq,
		"terminal_cycle_status", string(cycleStatus), "task_status", string(taskStatus),
		"runner", h.runner.Name(), "runner_version", h.runner.Version(),
		"duration_ms", h.opts.Clock().Sub(state.startedAt).Milliseconds())
}

func (h *Harness) runCycleLoop(parentCtx context.Context, task *domain.Task, cycle *domain.TaskCycle, state *processState, opts cycleLoopOpts) {
	skipExecute := opts.skipFirstExecute
	for {
		var cycleStatus domain.CycleStatus
		var taskStatus domain.Status
		var reason string

		if !skipExecute {
			if !h.runCycleLoopExecute(parentCtx, task, cycle, state, opts) {
				return
			}
			cycleStatus = domain.CycleStatusSucceeded
			taskStatus = domain.StatusDone
		} else {
			skipExecute = false
			cycleStatus = domain.CycleStatusSucceeded
			taskStatus = domain.StatusDone
		}

		retryLoop, terminalFailure := h.runCycleLoopVerify(parentCtx, task, cycle, state)
		if retryLoop {
			continue
		}
		if terminalFailure {
			return
		}

		h.runCycleLoopFinalizeSuccess(parentCtx, task, cycle, state, cycleStatus, taskStatus, reason)
		return
	}
}
