package resume

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

const (
	cancelledByOperatorReason = "cancelled_by_operator"
	verificationFailedReason  = "verification_failed"
)

// ReconstructCheckpoint rebuilds resume state for an in-flight running cycle.
func (s *Service) ReconstructCheckpoint(ctx context.Context, cycle *domain.TaskCycle) (Checkpoint, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.resume.ReconstructCheckpoint",
		"cycle_id", cycle.ID)
	var cp Checkpoint
	cp.PreviouslyPassed = map[string]CriterionVerdict{}
	if cycle == nil {
		return cp, errors.New("resume: nil cycle")
	}
	if cycle.Status != domain.CycleStatusRunning {
		return cp, fmt.Errorf("resume: cycle status %q is not running", cycle.Status)
	}

	phases, err := s.store.ListPhasesForCycle(ctx, cycle.ID)
	if err != nil {
		return cp, err
	}
	if len(phases) == 0 {
		return cp, errors.New("resume: cycle has no phases")
	}
	last := phases[len(phases)-1]
	for _, p := range phases {
		if p.Status == domain.PhaseStatusRunning {
			return cp, fmt.Errorf("resume: phase_seq=%d still running after finalize", p.PhaseSeq)
		}
	}

	switch {
	case isInterruptPhase(last):
		switch last.Phase {
		case domain.PhaseExecute:
			cp.Entry = EntryExecute
		case domain.PhaseVerify:
			cp.Entry = EntryVerifyOnly
		default:
			return cp, fmt.Errorf("resume: unexpected interrupted phase %q", last.Phase)
		}
	case last.Phase == domain.PhaseExecute && last.Status == domain.PhaseStatusSucceeded:
		cp.Entry = EntryAfterExecuteSuccess
	default:
		return cp, fmt.Errorf("resume: cannot continue from phase %q status %q", last.Phase, last.Status)
	}

	previouslyPassed, maxAttempt, verifyFeedback, err := s.loadVerifyCheckpointData(ctx, cycle.ID)
	if err != nil {
		return cp, err
	}
	cp.PreviouslyPassed = previouslyPassed
	if maxAttempt > 0 {
		cp.VerifyAttempt = int(maxAttempt)
		cp.VerifyFeedback = verifyFeedback
	}

	commits, err := s.loadKnownCommitsForTask(ctx, cycle.TaskID)
	if err != nil {
		return cp, err
	}
	cp.KnownCommits = commits

	return cp, nil
}

// LoadCheckpointFromParent builds a checkpoint from a terminal parent cycle continuation bundle.
func (s *Service) LoadCheckpointFromParent(ctx context.Context, parentCycleID string) (Checkpoint, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.resume.LoadCheckpointFromParent",
		"parent_cycle_id", parentCycleID)
	bundle, err := s.LoadContinuationBundle(ctx, parentCycleID)
	if err != nil {
		return Checkpoint{PreviouslyPassed: map[string]CriterionVerdict{}}, err
	}
	if !bundle.Sufficient {
		return Checkpoint{PreviouslyPassed: map[string]CriterionVerdict{}},
			fmt.Errorf("continuation: insufficient data for parent %s", parentCycleID)
	}
	return bundleToCheckpoint(bundle), nil
}

func (s *Service) loadKnownCommitsForTask(ctx context.Context, taskID string) ([]domain.TaskCycleCommit, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.resume.loadKnownCommitsForTask",
		"task_id", taskID)
	return s.store.ListCommitsForTask(ctx, taskID)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) loadVerifyCheckpointData(ctx context.Context, cycleID string) (map[string]CriterionVerdict, int64, string, error) {
	previouslyPassed := map[string]CriterionVerdict{}
	verifyRows, err := s.store.ListVerifyReportsForCycle(ctx, cycleID)
	if err != nil {
		return nil, 0, "", err
	}
	var maxAttempt int64
	for _, row := range verifyRows {
		if row.AttemptSeq > maxAttempt {
			maxAttempt = row.AttemptSeq
		}
		if !row.Verified {
			continue
		}
		if _, ok := previouslyPassed[row.CriterionID]; ok {
			continue
		}
		previouslyPassed[row.CriterionID] = CriterionVerdict{
			ID:        row.CriterionID,
			Passed:    true,
			Evidence:  "",
			Verifier:  row.VerifierKind,
			Reasoning: row.Reasoning,
		}
	}
	feedback := ""
	if maxAttempt > 0 {
		feedback = buildVerifyFeedbackFromRows(verifyRows, maxAttempt)
	}
	return previouslyPassed, maxAttempt, feedback, nil
}

func isInterruptPhase(p domain.TaskCyclePhase) bool {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.resume.isInterruptPhase",
		"phase_seq", p.PhaseSeq, "phase", string(p.Phase), "status", string(p.Status))
	if p.Status != domain.PhaseStatusFailed {
		return false
	}
	if p.Summary == nil {
		return false
	}
	return *p.Summary == domain.PhaseInterruptReason
}

func buildVerifyFeedbackFromRows(rows []domain.TaskCycleVerifyReport, attemptSeq int64) string {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.resume.buildVerifyFeedbackFromRows",
		"attempt_seq", attemptSeq, "rows", len(rows))
	var failures []string
	for _, row := range rows {
		if row.AttemptSeq != attemptSeq || row.Verified {
			continue
		}
		failures = append(failures, fmt.Sprintf("%s: %s", row.CriterionID, row.Reasoning))
	}
	return strings.Join(failures, "; ")
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func bundleToCheckpoint(bundle ContinuationBundle) Checkpoint {
	return Checkpoint{
		Entry:            bundle.Entry,
		PreviouslyPassed: bundle.PreviouslyPassed,
		VerifyAttempt:    0,
		VerifyFeedback:   bundle.VerifyFeedback,
		KnownCommits:     bundle.Commits,
		Continuation:     &bundle,
	}
}
