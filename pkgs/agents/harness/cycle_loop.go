package harness

import (
	"context"
	"errors"
	"log/slog"
	"strings"

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

func (h *Harness) runCycleLoop(parentCtx context.Context, task *domain.Task, cycle *domain.TaskCycle, state *processState, opts cycleLoopOpts) {
	skipExecute := opts.skipFirstExecute
	for {
		var cycleStatus domain.CycleStatus
		var taskStatus domain.Status
		var reason string

		if !skipExecute {
			execPhase, ok := h.startExecutePhase(parentCtx, cycle, state)
			if !ok {
				h.bestEffortTerminate(parentCtx, state, task.ID, domain.CycleStatusFailed, "execute_phase_start_failed")
				return
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
				return
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
				return
			}

			phaseStatus, cs, ts, r := classifyRunOutcome(runErr)
			cycleStatus = cs
			taskStatus = ts
			reason = r
			if operatorCancelled {
				reason = CancelledByOperatorReason
				if result.Summary == "" || strings.HasPrefix(result.Summary, "cursor: timeout") {
					result.Summary = "cancelled by operator"
				}
			}

			commitCount := 0
			var phaseDetails []byte
			if runErr == nil && !operatorCancelled && !snap.Skipped {
				outcome, ingestErr := h.ingestExecuteCommits(parentCtx, task.ID, cycle, execPhase.PhaseSeq, snap)
				if ingestErr != nil {
					slog.Warn("agent harness commit ingest error", "cmd", harnessLogCmd,
						"operation", "agent.harness.Harness.runCycleLoop.commit_ingest_err",
						"cycle_id", cycle.ID, "err", ingestErr)
					phaseStatus = domain.PhaseStatusFailed
					cycleStatus = domain.CycleStatusFailed
					taskStatus = domain.StatusFailed
					reason = executeInvalidCommitReason
					result.Summary = executeInvalidCommitReason
				} else if outcome.FailReason != "" {
					phaseStatus = domain.PhaseStatusFailed
					cycleStatus = domain.CycleStatusFailed
					taskStatus = domain.StatusFailed
					reason = outcome.FailReason
					result.Summary = outcome.FailReason
				} else {
					commitCount = outcome.CommitCount
				}
			}
			phaseDetails = mergeRunnerDetailsWithGit(detailsBytes(result), snap, commitCount)

			if !h.completeExecutePhase(parentCtx, state, cycle, execPhase, phaseStatus, result, phaseDetails) {
				h.bestEffortTerminate(parentCtx, state, task.ID, domain.CycleStatusFailed, completePhaseFailedReason)
				return
			}

			if runErr != nil || operatorCancelled || phaseStatus == domain.PhaseStatusFailed {
				if !h.terminateCycle(parentCtx, state, cycle.TaskID, cycleStatus, reason) {
					return
				}
				if taskStatus == domain.StatusFailed {
					_ = h.transitionTask(parentCtx, task.ID, taskStatus, "final_task_transition")
				}
				return
			}
		} else {
			skipExecute = false
			cycleStatus = domain.CycleStatusSucceeded
			taskStatus = domain.StatusDone
		}

		var verdicts []criterionVerdict
		if state.verifySnap.enabled {
			var verifyErr error
			var feedback string
			verdicts, feedback, verifyErr = h.runVerificationPipeline(parentCtx, task, cycle, state, state.verifySnap, state.verifyFeedback)
			if verifyErr != nil && feedback != "" {
				state.verifyFeedback = feedback
			}
			for _, v := range verdicts {
				if !v.passed {
					continue
				}
				if _, exists := state.previouslyPassed[v.id]; !exists {
					state.previouslyPassed[v.id] = v
				}
			}
			if verifyErr != nil {
				var tampered *verifyTamperedError
				if errors.As(verifyErr, &tampered) {
					if !h.terminateCycle(parentCtx, state, cycle.TaskID, domain.CycleStatusFailed, verifyTamperedReason) {
						return
					}
					_ = h.transitionTask(parentCtx, task.ID, domain.StatusFailed, "final_task_transition")
					return
				}
				if state.verifyAttempt < state.verifySnap.maxRetries {
					state.verifyAttempt++
					continue
				}
				cycleStatus = domain.CycleStatusFailed
				taskStatus = domain.StatusFailed
				reason = formatVerificationFailedReason(verdicts, state.previouslyPassed)
				if !h.terminateCycle(parentCtx, state, cycle.TaskID, cycleStatus, reason) {
					return
				}
				_ = h.transitionTask(parentCtx, task.ID, taskStatus, "final_task_transition")
				return
			}
		} else if err := h.completeChecklistLegacy(parentCtx, task.ID); err != nil {
			slog.Warn("agent harness checklist completion failed",
				"cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.runCycleLoop.checklist_err",
				"task_id", task.ID, "err", err)
			cycleStatus = domain.CycleStatusFailed
			taskStatus = domain.StatusFailed
			reason = checklistCompletionFailedReason
			if !h.terminateCycle(parentCtx, state, cycle.TaskID, cycleStatus, reason) {
				return
			}
			_ = h.transitionTask(parentCtx, task.ID, taskStatus, "final_task_transition")
			return
		}

		unionVerdicts := make([]criterionVerdict, 0, len(state.previouslyPassed))
		for _, v := range state.previouslyPassed {
			unionVerdicts = append(unionVerdicts, v)
		}
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
		return
	}
}
