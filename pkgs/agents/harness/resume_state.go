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
	commitPolicyOn   bool
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

	settings, err := h.store.GetSettings(ctx)
	if err != nil {
		return cp, err
	}
	cp.commitPolicyOn = settings.AgentCommitExecuteWork

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

	verifyRows, err := h.store.ListVerifyReportsForCycle(ctx, cycle.ID)
	if err != nil {
		return cp, err
	}
	var maxAttempt int64
	for _, row := range verifyRows {
		if row.AttemptSeq > maxAttempt {
			maxAttempt = row.AttemptSeq
		}
		if !row.Verified {
			continue
		}
		if _, ok := cp.previouslyPassed[row.CriterionID]; ok {
			continue
		}
		cp.previouslyPassed[row.CriterionID] = criterionVerdict{
			id:        row.CriterionID,
			passed:    true,
			evidence:  "",
			verifier:  row.VerifierKind,
			reasoning: row.Reasoning,
		}
	}
	if maxAttempt > 0 {
		cp.verifyAttempt = int(maxAttempt)
	}

	if maxAttempt > 0 {
		cp.verifyFeedback = buildVerifyFeedbackFromRows(verifyRows, maxAttempt)
	}

	return cp, nil
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

func (h *Harness) agentCommitExecuteWork(ctx context.Context) bool {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.agentCommitExecuteWork")
	settings, err := h.store.GetSettings(ctx)
	if err != nil {
		return true
	}
	return settings.AgentCommitExecuteWork
}
