package harness

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/orchestration"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

type cycleLoopOpts struct {
	resumeNotice     bool
	interruptedPhase domain.Phase
	skipFirstExecute bool
	knownCommits     []domain.TaskCycleCommit
	continuation     *ContinuationBundle
}

func (h *Harness) composeExecutePrompt(ctx context.Context, task *domain.Task, cycle *domain.TaskCycle, state *processState, opts cycleLoopOpts) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.composeExecutePrompt",
		"task_id", task.ID, "cycle_id", cycle.ID, "resume_notice", opts.resumeNotice)
	promptText := task.InitialPrompt
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
			promptText = prompt.InjectAutomations(promptText, resolved)
		}
	}
	promptText = prompt.InjectCriteria(
		promptText,
		checklistItemsForPrompt(state.verifySnap.Criteria),
		reports.CriteriaReportPath(h.opts.ReportDir, cycle.ID),
		verifiedCriterionIDs(state.previouslyPassed),
	)
	promptText = prompt.AppendVerifyFeedback(promptText, state.verifyFeedback)
	retryMode := retryModeFromCycleMeta(cycle)
	if bundle := opts.continuation; bundle != nil {
		promptText = prompt.ComposeContinuation(promptText, continuationInputFromBundle(cycle, bundle))
		if bundle.ExecuteFeedback != "" {
			promptText = prompt.AppendExecuteHarnessFeedback(promptText, bundle.ExecuteFeedback)
		}
	} else if opts.resumeNotice {
		if retryMode == domain.RetryResume {
			promptText = prompt.AppendOperatorRetryResumeNotice(promptText, cycle, opts.knownCommits)
		} else {
			promptText = prompt.AppendResumeNotice(promptText, cycle, opts.interruptedPhase, opts.knownCommits)
		}
	}
	if !state.gitSnap.Skipped {
		promptText = prompt.AppendGitCommitPolicy(promptText, retryMode == domain.RetryResume)
	}
	return promptText
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func recordPassedCriterionVerdicts(state *processState, verdicts []criterionVerdict) {
	for _, v := range verdicts {
		if !v.Passed {
			continue
		}
		if _, exists := state.previouslyPassed[v.ID]; !exists {
			state.previouslyPassed[v.ID] = v
		}
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
	snap, err := captureExecuteGitSnapshot(parentCtx, h.gitSvc().Repo(), h.repoRootForGit(parentCtx), h.opts.WorkingDir, priorBase)
	if err != nil {
		slog.Warn("agent harness git snapshot failed", "cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.runCycleLoop.git_snapshot",
			"cycle_id", cycle.ID, "err", err)
		h.bestEffortTerminate(parentCtx, state, task.ID, domain.CycleStatusFailed, "execute_git_snapshot_failed")
		return false
	}
	state.gitSnap = snap

	decision, err := h.planExecuteRun(parentCtx, task, cycle, state, opts)
	if err != nil {
		h.bestEffortTerminate(parentCtx, state, task.ID, domain.CycleStatusFailed, "cursor_resume_plan_failed")
		return false
	}
	if decision.Mode == CursorResumeFresh || decision.Mode == CursorResumeFallback {
		_ = reports.ScrubCycleArtifacts(h.opts.ReportDir, cycle.ID)
	}
	_ = reports.EnsureReportCycleDir(h.opts.ReportDir, cycle.ID)

	result, runErr := h.invokeRunnerWithTask(parentCtx, task, cycle, execPhase, decision)
	if errors.Is(runErr, runner.ErrResumeSession) {
		fallback := h.planExecuteResumeFallback(parentCtx, task, cycle, state, opts)
		result, runErr = h.invokeRunnerWithTask(parentCtx, task, cycle, execPhase, fallback)
	}
	operatorCancelled := h.consumeOperatorCancel()

	if parentCtx.Err() != nil {
		effects := orchestration.DecideExecutePostRun(orchestration.ExecutePostRunInput{
			ContextCancelled: true,
		})
		return h.applyExecuteEffects(parentCtx, task, cycle, state, execPhase, result, effects, 0, snap, operatorCancelled, false)
	}

	var ingestOutcome executeCommitIngestOutcome
	var ingestErr error
	ingestAttempted := false
	staleRecovery := errors.Is(runErr, runner.ErrStale)
	if (runErr == nil || staleRecovery) && !operatorCancelled && !snap.Skipped {
		ingestAttempted = true
		ingestOutcome, ingestErr = h.ingestExecuteCommits(
			parentCtx, task.ID, cycle, execPhase.PhaseSeq, snap,
			opts.knownCommits, retryModeFromCycleMeta(cycle),
		)
		if ingestErr != nil {
			slog.Warn("agent harness commit ingest error", "cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.runCycleLoop.commit_ingest_err",
				"cycle_id", cycle.ID, "err", ingestErr)
		}
	}

	commitCount := 0
	if ingestAttempted && ingestErr == nil && ingestOutcome.FailReason == "" {
		commitCount = ingestOutcome.CommitCount
	}

	postRunIn := buildExecutePostRunInput(parentCtx, runErr, operatorCancelled, snap, ingestAttempted, ingestOutcome, ingestErr)
	effects := orchestration.DecideExecutePostRun(postRunIn)
	if staleRecovery && effects.ContinueToVerify {
		recovered := streamIdleRecoveredEvent()
		h.persistProgress(parentCtx, task.ID, cycle.ID, execPhase.PhaseSeq, recovered)
		h.publishProgress(task.ID, cycle.ID, execPhase.PhaseSeq, state.runCorrelationID, recovered)
	}
	cont := h.applyExecuteEffects(parentCtx, task, cycle, state, execPhase, result, effects, commitCount, snap, operatorCancelled, staleRecovery)
	if cont {
		h.anchorPostExecuteState(parentCtx, state, execPhase.PhaseSeq, snap, ingestAttempted, ingestOutcome, ingestErr)
	}
	return cont
}

// runCycleLoopVerify runs verification for one loop iteration. retryLoop is
// true when the outer loop should continue for another execute↔verify attempt.
// skipNextExecute is true when the next iteration should skip execute (ADR-0028).
// terminalFailure is true when verification failed terminally (caller should return).
func (h *Harness) runCycleLoopVerify(
	parentCtx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	state *processState,
) (retryLoop bool, terminalFailure bool, skipNextExecute bool) {
	if orchestration.VerifyDisabled(state.verifySnap.Enabled) {
		checklistErr := h.completeChecklistLegacy(parentCtx, task.ID)
		if checklistErr != nil {
			slog.Warn("agent harness checklist completion failed",
				"cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.runCycleLoop.checklist_err",
				"task_id", task.ID, "err", checklistErr)
		}
		effects := orchestration.DecideVerifyDisabledLegacy(checklistErr)
		retry, term := h.applyVerifyEffects(parentCtx, task, cycle, state, effects, checklistCompletionFailedReason)
		return retry, term, false
	}

	verdicts, feedback, verifyErr := h.runVerificationPipeline(parentCtx, task, cycle, state, state.verifySnap, state.verifyFeedback)
	if verifyErr != nil && feedback != "" {
		state.verifyFeedback = feedback
	}
	recordPassedCriterionVerdicts(state, verdicts)
	if verifyErr != nil {
		state.lastFailedVerdicts = append([]criterionVerdict(nil), verdicts...)
		var tampered *verify.TamperedError
		if errors.As(verifyErr, &tampered) {
			state.reportTampered = true
		}
	}
	if verifyErr == nil {
		return false, false, false
	}

	var result orchestration.VerifyResult
	var tampered *verify.TamperedError
	if errors.As(verifyErr, &tampered) {
		result = orchestration.VerifyResultFailTampered
	} else {
		result = orchestration.VerifyResultFailRetryable
	}

	classifyIn := h.gatherRetryClassifyInput(parentCtx, cycle, state, verdicts, verifyErr)
	retryMode, reasonCode := orchestration.ClassifyVerifyRetryMode(classifyIn)
	executeStillValid := retryMode == orchestration.RetryModeVerifyOnly
	effects := orchestration.DecideVerifyRetryWithValidity(state.verifyAttempt, state.verifySnap.MaxRetries, result, executeStillValid)
	if effects.RetryLoop {
		slog.Info("agent harness verify retry classified", "cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.runCycleLoopVerify.retry_mode",
			"task_id", task.ID, "cycle_id", cycle.ID,
			"retry_mode", string(retryMode), "reason_code", string(reasonCode),
			"skip_next_execute", effects.SkipNextExecute)
	}
	terminalReason := formatVerificationFailedReason(verdicts, state.previouslyPassed)
	retry, term := h.applyVerifyEffects(parentCtx, task, cycle, state, effects, terminalReason)
	return retry, term, effects.SkipNextExecute
}

func (h *Harness) runCycleLoopFinalizeSuccess(
	parentCtx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	state *processState,
) {
	unionVerdicts := unionPreviouslyPassedVerdicts(state)
	completionErr := h.applyVerifiedCompletions(parentCtx, task.ID, cycle.ID, unionVerdicts)
	effects := orchestration.DecideFinalizeSuccess(completionErr)
	if completionErr != nil {
		slog.Warn("agent harness checklist completion failed",
			"cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.runCycleLoop.finalize_err",
			"task_id", task.ID, "err", completionErr)
	}
	_ = h.applyFinalizeEffects(parentCtx, task, cycle, state, effects)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) runCycleLoop(parentCtx context.Context, task *domain.Task, cycle *domain.TaskCycle, state *processState, opts cycleLoopOpts) {
	state.continuation = opts.continuation
	state.resumeNotice = opts.resumeNotice
	state.interruptedPhase = opts.interruptedPhase
	skipExecute := opts.skipFirstExecute
	for {
		if !skipExecute {
			if !h.runCycleLoopExecute(parentCtx, task, cycle, state, opts) {
				return
			}
		} else {
			skipExecute = false
		}

		retryLoop, terminalFailure, skipNextExecute := h.runCycleLoopVerify(parentCtx, task, cycle, state)
		if retryLoop {
			// ADR-0028: skipNextExecute ⇒ must not call runCycleLoopExecute (no scrub, no runner).
			skipExecute = skipNextExecute
			continue
		}
		if terminalFailure {
			return
		}

		h.runCycleLoopFinalizeSuccess(parentCtx, task, cycle, state)
		return
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
