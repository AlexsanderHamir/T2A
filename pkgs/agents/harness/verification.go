package harness

import (
	"context"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

type criterionVerdict = verify.Verdict
type verificationSnapshot = verify.Snapshot

const verificationFailedReason = verify.FailedReasonPrefix

func (h *Harness) verifySvc() *verify.Service {
	if h.verify == nil {
		h.verify = verify.NewService(verify.Deps{
			Store:        h.store,
			Runner:       h.runner,
			VerifyRunner: h.opts.VerifyRunner,
			ReportDir:    h.opts.ReportDir,
			WorkingDir:   h.opts.WorkingDir,
			Git:          h.gitSvc(),
			Clock:        h.opts.Clock,
			Hooks: verify.Hooks{
				Publish: h.publish,
				PersistProgress: func(ctx context.Context, taskID, cycleID string, phaseSeq int64, ev runner.ProgressEvent) {
					h.persistProgress(ctx, taskID, cycleID, phaseSeq, ev)
					h.publishProgress(taskID, cycleID, phaseSeq, ev)
				},
				RecordVerdict:   h.recordVerifyVerdict,
				ObserveDuration: h.observeVerifyDuration,
			},
		})
	}
	h.verify.SetReportDir(h.opts.ReportDir)
	h.verify.SetWorkingDir(h.opts.WorkingDir)
	h.verify.SetVerifyRunner(h.opts.VerifyRunner)
	return h.verify
}

func (h *Harness) loadVerificationSnapshot(ctx context.Context, taskID string) (verificationSnapshot, error) {
	return h.verifySvc().LoadSnapshot(ctx, taskID)
}

func (h *Harness) completeChecklistLegacy(ctx context.Context, taskID string) error {
	return h.verifySvc().CompleteChecklistLegacy(ctx, taskID)
}

func (h *Harness) applyVerifiedCompletions(ctx context.Context, taskID, cycleID string, verdicts []criterionVerdict) error {
	return h.verifySvc().ApplyVerifiedCompletions(ctx, taskID, cycleID, verdicts)
}

func (h *Harness) runVerificationPipeline(
	parentCtx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	state *processState,
	snap verificationSnapshot,
	feedback string,
) ([]criterionVerdict, string, error) {
	return h.verifySvc().RunPipeline(parentCtx, task, cycle, snap, state.verifyAttempt, state.previouslyPassed, feedback, verify.PhaseCallbacks{
		OnStarted: func(phaseSeq int64) {
			state.runningPhase = domain.PhaseVerify
			state.runningPhaseSeq = phaseSeq
		},
		OnEnded: func() {
			state.runningPhase = ""
			state.runningPhaseSeq = 0
		},
	})
}

func formatVerificationFailedReason(finalVerdicts []criterionVerdict, lockedPasses map[string]criterionVerdict) string {
	return verify.FormatFailedReason(finalVerdicts, lockedPasses)
}

func verifyDiffSection(workingDir string) string {
	return verify.DiffSection(workingDir)
}

func (h *Harness) persistCriteriaReports(
	ctx context.Context,
	cycleID string,
	attemptSeq int64,
	criteria []store.ChecklistVerifyItem,
	previouslyPassed map[string]criterionVerdict,
	selfReport map[string]reports.CriteriaEntry,
) error {
	return h.verifySvc().PersistCriteriaReports(ctx, cycleID, attemptSeq, criteria, previouslyPassed, selfReport)
}
