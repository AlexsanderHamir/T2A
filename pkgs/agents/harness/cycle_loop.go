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

	_ = reports.ScrubCycleArtifacts(h.opts.ReportDir, cycle.ID)
	_ = reports.EnsureReportCycleDir(h.opts.ReportDir, cycle.ID)
	promptText := h.composeExecutePrompt(parentCtx, task, cycle, state, opts)
	execTask := *task
	execTask.InitialPrompt = promptText

	result, runErr := h.invokeRunnerWithTask(parentCtx, &execTask, cycle, execPhase)
	operatorCancelled := h.consumeOperatorCancel()

	if parentCtx.Err() != nil {
		effects := orchestration.DecideExecutePostRun(orchestration.ExecutePostRunInput{
			ContextCancelled: true,
		})
		return h.applyExecuteEffects(parentCtx, task, cycle, state, execPhase, result, effects, 0, snap, operatorCancelled)
	}

	var ingestOutcome executeCommitIngestOutcome
	var ingestErr error
	ingestAttempted := false
	if runErr == nil && !operatorCancelled && !snap.Skipped {
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
	return h.applyExecuteEffects(parentCtx, task, cycle, state, execPhase, result, effects, commitCount, snap, operatorCancelled)
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
	if orchestration.VerifyDisabled(state.verifySnap.Enabled) {
		checklistErr := h.completeChecklistLegacy(parentCtx, task.ID)
		if checklistErr != nil {
			slog.Warn("agent harness checklist completion failed",
				"cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.runCycleLoop.checklist_err",
				"task_id", task.ID, "err", checklistErr)
		}
		effects := orchestration.DecideVerifyDisabledLegacy(checklistErr)
		return h.applyVerifyDisabledLegacyEffects(parentCtx, task, cycle, state, effects)
	}

	verdicts, feedback, verifyErr := h.runVerificationPipeline(parentCtx, task, cycle, state, state.verifySnap, state.verifyFeedback)
	if verifyErr != nil && feedback != "" {
		state.verifyFeedback = feedback
	}
	recordPassedCriterionVerdicts(state, verdicts)
	if verifyErr == nil {
		return false, false
	}

	var result orchestration.VerifyResult
	var tampered *verify.TamperedError
	if errors.As(verifyErr, &tampered) {
		result = orchestration.VerifyResultFailTampered
	} else {
		result = orchestration.VerifyResultFailRetryable
	}

	effects := orchestration.DecideVerifyRetry(state.verifyAttempt, state.verifySnap.MaxRetries, result)
	terminalReason := formatVerificationFailedReason(verdicts, state.previouslyPassed)
	return h.applyVerifyEffects(parentCtx, task, cycle, state, effects, terminalReason)
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

func (h *Harness) runCycleLoop(parentCtx context.Context, task *domain.Task, cycle *domain.TaskCycle, state *processState, opts cycleLoopOpts) {
	skipExecute := opts.skipFirstExecute
	for {
		if !skipExecute {
			if !h.runCycleLoopExecute(parentCtx, task, cycle, state, opts) {
				return
			}
		} else {
			skipExecute = false
		}

		retryLoop, terminalFailure := h.runCycleLoopVerify(parentCtx, task, cycle, state)
		if retryLoop {
			continue
		}
		if terminalFailure {
			return
		}

		h.runCycleLoopFinalizeSuccess(parentCtx, task, cycle, state)
		return
	}
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
