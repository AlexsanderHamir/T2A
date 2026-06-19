package harness

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

type resumeEntry int

const (
	resumeEntryExecute resumeEntry = iota
	resumeEntryVerifyOnly
	resumeEntryAfterExecuteSuccess
)

type resumeCheckpoint struct {
	entry            resumeEntry
	previouslyPassed map[string]criterionVerdict
	verifyAttempt    int
	verifyFeedback   string
	knownCommits     []domain.TaskCycleCommit
}

func (h *Harness) reconstructCheckpoint(ctx context.Context, cycle *domain.TaskCycle) (resumeCheckpoint, error) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.reconstructCheckpoint",
		"cycle_id", cycle.ID)
	var cp resumeCheckpoint
	cp.previouslyPassed = map[string]criterionVerdict{}
	if cycle == nil {
		return cp, errors.New("resume: nil cycle")
	}
	if cycle.Status != domain.CycleStatusRunning {
		return cp, fmt.Errorf("resume: cycle status %q is not running", cycle.Status)
	}

	phases, err := h.store.ListPhasesForCycle(ctx, cycle.ID)
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
			cp.entry = resumeEntryExecute
		case domain.PhaseVerify:
			cp.entry = resumeEntryVerifyOnly
		default:
			return cp, fmt.Errorf("resume: unexpected interrupted phase %q", last.Phase)
		}
	case last.Phase == domain.PhaseExecute && last.Status == domain.PhaseStatusSucceeded:
		cp.entry = resumeEntryAfterExecuteSuccess
	default:
		return cp, fmt.Errorf("resume: cannot continue from phase %q status %q", last.Phase, last.Status)
	}

	previouslyPassed, maxAttempt, verifyFeedback, err := h.loadVerifyCheckpointData(ctx, cycle.ID)
	if err != nil {
		return cp, err
	}
	cp.previouslyPassed = previouslyPassed
	if maxAttempt > 0 {
		cp.verifyAttempt = int(maxAttempt)
		cp.verifyFeedback = verifyFeedback
	}

	commits, err := h.store.ListCommitsForCycle(ctx, cycle.ID)
	if err != nil {
		return cp, err
	}
	cp.knownCommits = commits

	return cp, nil
}

func (h *Harness) loadCheckpointFromParent(ctx context.Context, parentCycleID string) (resumeCheckpoint, error) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.loadCheckpointFromParent",
		"parent_cycle_id", parentCycleID)
	var cp resumeCheckpoint
	cp.previouslyPassed = map[string]criterionVerdict{}
	parentCycleID = strings.TrimSpace(parentCycleID)
	if parentCycleID == "" {
		return cp, fmt.Errorf("resume parent: empty cycle id")
	}
	cycle, err := h.store.GetCycle(ctx, parentCycleID)
	if err != nil {
		return cp, err
	}
	if !domain.TerminalCycleStatus(cycle.Status) {
		return cp, fmt.Errorf("resume parent: cycle status %q is not terminal", cycle.Status)
	}
	previouslyPassed, maxAttempt, verifyFeedback, err := h.loadVerifyCheckpointData(ctx, parentCycleID)
	if err != nil {
		return cp, err
	}
	cp.previouslyPassed = previouslyPassed
	cp.verifyFeedback = verifyFeedback
	_ = maxAttempt // cross-cycle resume always starts verifyAttempt at 0 in runResumeRetry
	commits, err := h.store.ListCommitsForCycle(ctx, parentCycleID)
	if err != nil {
		return cp, err
	}
	cp.knownCommits = commits
	cp.entry = resumeEntryExecute
	return cp, nil
}

func (h *Harness) loadVerifyCheckpointData(ctx context.Context, cycleID string) (map[string]criterionVerdict, int64, string, error) {
	previouslyPassed := map[string]criterionVerdict{}
	verifyRows, err := h.store.ListVerifyReportsForCycle(ctx, cycleID)
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
		previouslyPassed[row.CriterionID] = criterionVerdict{
			id:        row.CriterionID,
			passed:    true,
			evidence:  "",
			verifier:  row.VerifierKind,
			reasoning: row.Reasoning,
		}
	}
	feedback := ""
	if maxAttempt > 0 {
		feedback = buildVerifyFeedbackFromRows(verifyRows, maxAttempt)
	}
	return previouslyPassed, maxAttempt, feedback, nil
}

func isInterruptPhase(p domain.TaskCyclePhase) bool {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.isInterruptPhase",
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
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.buildVerifyFeedbackFromRows",
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
