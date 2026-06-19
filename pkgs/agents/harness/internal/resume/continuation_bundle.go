package resume

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// LoadContinuationBundle rehydrates cross-cycle resume context from a parent attempt.
func (s *Service) LoadContinuationBundle(ctx context.Context, parentCycleID string) (ContinuationBundle, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.resume.LoadContinuationBundle",
		"parent_cycle_id", parentCycleID)
	var bundle ContinuationBundle
	bundle.PreviouslyPassed = map[string]CriterionVerdict{}
	parentCycleID = strings.TrimSpace(parentCycleID)
	if parentCycleID == "" {
		return bundle, fmt.Errorf("continuation: empty parent cycle id")
	}
	cycle, err := s.store.GetCycle(ctx, parentCycleID)
	if err != nil {
		return bundle, err
	}
	if !domain.TerminalCycleStatus(cycle.Status) {
		return bundle, fmt.Errorf("continuation: parent cycle %q is not terminal", cycle.Status)
	}
	bundle.ParentCycleID = parentCycleID
	bundle.LineageAttempt = cycle.AttemptSeq

	phases, err := s.store.ListPhasesForCycle(ctx, parentCycleID)
	if err != nil {
		return bundle, err
	}
	bundle.FailureReason = parentFailureReason(phases, cycle)
	if len(phases) == 0 {
		bundle.Warnings = append(bundle.Warnings, "parent cycle has no phases")
	} else {
		lastPhase := phases[len(phases)-1]
		bundle.FailurePhase = lastPhase.Phase
		bundle.FailureClass = classifyParentFailure(phases, cycle, lastPhase)
		lastExecute := lastExecutePhase(phases)
		if lastExecute != nil {
			bundle.ScopeFiles = git.ScopeFilesFromPhaseDetails(ctx, s.gitRepo(), s.opts.WorkingDir, lastExecute.DetailsJSON)
			bundle.RunnerFeedback = runnerFeedbackFromPhase(lastExecute)
			if lastExecute.Status == domain.PhaseStatusFailed {
				summary := phaseSummary(*lastExecute)
				if git.IsExecuteGateReason(summary) {
					bundle.ExecuteFeedback = git.ReasonRemediation(summary)
				}
			}
		}
		if bundle.ExecuteFeedback == "" && bundle.FailureClass == FailureClassExecuteGate {
			bundle.ExecuteFeedback = git.ReasonRemediation(bundle.FailureReason)
		}
		if bundle.FailureClass == FailureClassExecuteGate && strings.Contains(bundle.FailureReason, git.ExecuteUncommittedWorkReason) {
			if diag, derr := git.StatusPorcelain(ctx, s.gitRepo(), s.opts.WorkingDir); derr == nil && diag != "" {
				bundle.GitDiagnostics = diag
			}
		}
		eligible, err := s.store.ListEligibleCommitsForCycle(ctx, parentCycleID)
		if err != nil {
			return bundle, err
		}
		bundle.Entry = routeResumeEntry(phases, lastExecute, lastPhase, cycle, len(eligible) > 0)
	}

	previouslyPassed, _, verifyFeedback, err := s.loadVerifyCheckpointData(ctx, parentCycleID)
	if err != nil {
		return bundle, err
	}
	bundle.PreviouslyPassed = previouslyPassed
	bundle.VerifyFeedback = verifyFeedback

	commits, err := s.loadKnownCommitsForTask(ctx, cycle.TaskID)
	if err != nil {
		return bundle, err
	}
	bundle.Commits = commits

	criteriaRows, err := s.store.ListCriteriaReportsForCycle(ctx, parentCycleID)
	if err != nil {
		return bundle, err
	}
	for i := range criteriaRows {
		if criteriaRows[i].AttemptSeq == domain.ExecuteCriteriaReportAttemptSeq {
			bundle.CriteriaEvidence = append(bundle.CriteriaEvidence, criteriaRows[i])
		}
	}

	if len(phases) == 0 {
		bundle.Entry = EntryExecute
	} else {
		lastPhase := phases[len(phases)-1]
		lastExecute := lastExecutePhase(phases)
		eligible, err := s.store.ListEligibleCommitsForCycle(ctx, parentCycleID)
		if err != nil {
			return bundle, err
		}
		bundle.Entry = routeResumeEntry(phases, lastExecute, lastPhase, cycle, len(eligible) > 0)
	}

	bundle.Sufficient = continuationSufficient(bundle, cycle)
	if !bundle.Sufficient {
		bundle.Warnings = append(bundle.Warnings, "insufficient continuation data for parent attempt")
	}
	return bundle, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func classifyParentFailure(phases []domain.TaskCyclePhase, cycle *domain.TaskCycle, lastPhase domain.TaskCyclePhase) FailureClass {
	reason := parentFailureReason(phases, cycle)
	if reason == "" {
		reason = phaseSummary(lastPhase)
	}
	if reason == cancelledByOperatorReason {
		return FailureClassOperator
	}
	if strings.HasPrefix(reason, verificationFailedReason) || lastPhase.Phase == domain.PhaseVerify {
		return FailureClassVerify
	}
	if git.IsExecuteGateReason(reason) || (lastPhase.Phase == domain.PhaseExecute && lastPhase.Status == domain.PhaseStatusFailed) {
		if git.IsExecuteGateReason(phaseSummary(lastPhase)) || git.IsExecuteGateReason(reason) {
			return FailureClassExecuteGate
		}
	}
	if strings.HasPrefix(reason, "runner_") || strings.Contains(reason, "runner_") {
		return FailureClassRunner
	}
	if lastPhase.Phase == domain.PhaseExecute && lastPhase.Status == domain.PhaseStatusFailed {
		return FailureClassRunner
	}
	if reason == "shutdown" || reason == "panic" || strings.HasSuffix(reason, "_failed") {
		return FailureClassInfrastructure
	}
	return FailureClassInfrastructure
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func routeResumeEntry(phases []domain.TaskCyclePhase, lastExecute *domain.TaskCyclePhase, lastPhase domain.TaskCyclePhase, cycle *domain.TaskCycle, hasEligible bool) Entry {
	reason := parentFailureReason(phases, cycle)
	if reason == "" {
		reason = phaseSummary(lastPhase)
	}
	if lastExecute != nil &&
		lastExecute.Status == domain.PhaseStatusSucceeded &&
		cycle.Status == domain.CycleStatusFailed &&
		(lastPhase.Phase == domain.PhaseVerify || strings.HasPrefix(reason, verificationFailedReason)) &&
		hasEligible {
		return EntryVerifyOnly
	}
	return EntryExecute
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func parentFailureReason(phases []domain.TaskCyclePhase, cycle *domain.TaskCycle) string {
	if len(phases) > 0 {
		last := phases[len(phases)-1]
		if last.Status == domain.PhaseStatusFailed {
			if s := phaseSummary(last); s != "" {
				return s
			}
		}
		for i := len(phases) - 1; i >= 0; i-- {
			if phases[i].Status == domain.PhaseStatusFailed {
				if s := phaseSummary(phases[i]); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func continuationSufficient(bundle ContinuationBundle, cycle *domain.TaskCycle) bool {
	if cycle == nil || cycle.ID == "" {
		return false
	}
	if len(bundle.PreviouslyPassed) > 0 || len(bundle.Commits) > 0 || len(bundle.CriteriaEvidence) > 0 {
		return true
	}
	if bundle.FailureReason != "" || bundle.FailurePhase != "" {
		return true
	}
	return domain.TerminalCycleStatus(cycle.Status)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func lastExecutePhase(phases []domain.TaskCyclePhase) *domain.TaskCyclePhase {
	var last *domain.TaskCyclePhase
	for i := range phases {
		p := &phases[i]
		if p.Phase != domain.PhaseExecute {
			continue
		}
		if last == nil || p.PhaseSeq > last.PhaseSeq {
			last = p
		}
	}
	return last
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func phaseSummary(p domain.TaskCyclePhase) string {
	if p.Summary == nil {
		return ""
	}
	return strings.TrimSpace(*p.Summary)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func runnerFeedbackFromPhase(p *domain.TaskCyclePhase) string {
	if p == nil {
		return ""
	}
	summary := phaseSummary(*p)
	if summary == "" {
		return ""
	}
	if len(summary) > 512 {
		summary = summary[:512] + "…"
	}
	return summary
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func runnerDetailsExcerpt(details []byte) string {
	if len(details) == 0 {
		return ""
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(details, &root); err != nil {
		return ""
	}
	if raw, ok := root["summary"]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil && s != "" {
			if len(s) > 256 {
				s = s[:256] + "…"
			}
			return s
		}
	}
	return ""
}
