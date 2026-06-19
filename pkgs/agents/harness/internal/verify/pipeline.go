package verify

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// PhaseCallbacks notify harness when verify phase rows open and close.
type PhaseCallbacks struct {
	OnStarted func(phaseSeq int64)
	OnEnded   func()
}

// RunPipeline opens a verify phase, runs checks, closes the phase, and returns verdicts.
func (s *Service) RunPipeline(
	parentCtx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	snap Snapshot,
	verifyAttempt int,
	previouslyPassed map[string]Verdict,
	feedback string,
	phaseCB PhaseCallbacks,
) ([]Verdict, string, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.RunPipeline",
		"task_id", task.ID, "cycle_id", cycle.ID, "enabled", snap.Enabled)
	if !snap.Enabled {
		return nil, "", nil
	}
	if err := reports.EnsureReportCycleDir(s.reportDir, cycle.ID); err != nil {
		slog.Warn("agent harness ensureReportCycleDir failed",
			"cmd", logCmd, "operation", "agent.harness.verify.RunPipeline.ensure_err",
			"cycle_id", cycle.ID, "report_dir", s.reportDir, "err", err)
	}

	verifyStarted := s.clock()
	defer func() {
		s.observeDuration(s.clock().Sub(verifyStarted))
	}()

	phase, err := s.store.StartPhase(parentCtx, cycle.ID, domain.PhaseVerify, domain.ActorAgent)
	if err != nil {
		slog.Warn("agent harness StartPhase(verify) failed",
			"cmd", logCmd, "operation", "agent.harness.verify.RunPipeline.start_err",
			"cycle_id", cycle.ID, "err", err)
		return nil, "", fmt.Errorf("start verify phase: %w", err)
	}
	if phaseCB.OnStarted != nil {
		phaseCB.OnStarted(phase.PhaseSeq)
	}
	s.publish(cycle.TaskID, cycle.ID)

	pre, preErr := s.captureIntegritySnapshot(parentCtx)
	if preErr != nil {
		slog.Warn("agent harness pre-verify integrity snapshot failed",
			"cmd", logCmd, "operation", "agent.harness.verify.RunPipeline.pre_snapshot_err",
			"cycle_id", cycle.ID, "err", preErr)
	}

	attemptSeq := int64(verifyAttempt) + 1
	verdicts, feedbackOut, verifyErr := s.runVerifyChecks(parentCtx, task, cycle, phase.PhaseSeq, attemptSeq, snap, previouslyPassed, feedback)

	tampered, tamperReason := s.checkIntegrity(parentCtx, cycle.ID, pre, preErr)

	phaseStatus := domain.PhaseStatusSucceeded
	summary := FormatPhaseSummary(snap.Criteria, verdicts, true)
	var details []byte
	if tampered {
		phaseStatus = domain.PhaseStatusFailed
		summary = tamperReason
		verifyErr = &TamperedError{Reason: tamperReason}
	} else if verifyErr != nil {
		phaseStatus = domain.PhaseStatusFailed
		summary = FormatPhaseSummary(snap.Criteria, verdicts, false)
		details = EncodePhaseDetails(attemptSeq, snap.Criteria, verdicts)
	} else {
		details = EncodePhaseDetails(attemptSeq, snap.Criteria, verdicts)
	}
	if _, err := s.store.CompletePhase(parentCtx, store.CompletePhaseInput{
		CycleID:  cycle.ID,
		PhaseSeq: phase.PhaseSeq,
		Status:   phaseStatus,
		Summary:  &summary,
		Details:  details,
		By:       domain.ActorAgent,
	}); err != nil {
		slog.Warn("agent harness CompletePhase(verify) failed",
			"cmd", logCmd, "operation", "agent.harness.verify.RunPipeline.complete_err",
			"cycle_id", cycle.ID, "phase_seq", phase.PhaseSeq, "err", err)
	}
	if phaseCB.OnEnded != nil {
		phaseCB.OnEnded()
	}
	s.publish(cycle.TaskID, cycle.ID)
	return verdicts, feedbackOut, verifyErr
}
