package verify

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func (s *Service) runVerifyChecks(
	parentCtx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	phaseSeq int64,
	attemptSeq int64,
	snap Snapshot,
	previouslyPassed map[string]Verdict,
	feedback string,
) ([]Verdict, string, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.runVerifyChecks",
		"task_id", task.ID, "cycle_id", cycle.ID,
		"criteria_count", len(snap.Criteria), "previously_passed", len(previouslyPassed))
	expected := make(map[string]struct{}, len(snap.Criteria))
	for _, it := range snap.Criteria {
		if _, locked := previouslyPassed[it.ID]; locked {
			continue
		}
		expected[it.ID] = struct{}{}
	}

	selfReport, err := s.loadCriteriaSelfReport(parentCtx, cycle.ID, attemptSeq, expected)
	if err != nil {
		return nil, "", err
	}

	if uerr := s.PersistCriteriaReports(parentCtx, cycle.ID, attemptSeq, snap.Criteria, previouslyPassed, selfReport); uerr != nil {
		slog.Warn("agent harness UpsertCriteriaReports failed",
			"cmd", logCmd, "operation", "agent.harness.verify.runVerifyChecks.upsert_criteria_err",
			"cycle_id", cycle.ID, "attempt_seq", attemptSeq, "err", uerr)
	}

	verdicts := make([]Verdict, 0, len(snap.Criteria))
	needLLMVerify := false

	for _, it := range snap.Criteria {
		if locked, ok := previouslyPassed[it.ID]; ok {
			verdicts = append(verdicts, locked)
			continue
		}
		entry := selfReport[it.ID]
		v := Verdict{
			ID:       it.ID,
			Evidence: entry.Evidence,
		}
		if !entry.ClaimedDone {
			v.Passed = false
			v.Verifier = domain.VerifierAgentSelf
			v.Reasoning = "execute agent did not claim criterion done"
			verdicts = append(verdicts, v)
			s.recordVerdict(domain.VerifierAgentSelf, false)
			continue
		}
		needLLMVerify = true
		verdicts = append(verdicts, v)
	}

	if needLLMVerify {
		cmdEvidence, cmdErr := s.RunCriterionCommands(parentCtx, cycle.ID, attemptSeq, snap, selfReport, nil)
		if cmdErr != nil {
			return nil, "", cmdErr
		}
		if err := s.runLLMVerifyAgent(parentCtx, task, cycle, phaseSeq, snap, previouslyPassed, selfReport, feedback, cmdEvidence); err != nil {
			return nil, "", err
		}
		vrep, err := reports.ParseVerifyReport(s.reportDir, cycle.ID, expected)
		if err != nil {
			return nil, "", err
		}
		next := make([]Verdict, 0, len(verdicts))
		for _, v := range verdicts {
			if _, locked := previouslyPassed[v.ID]; locked {
				next = append(next, v)
				continue
			}
			if v.Verifier == domain.VerifierAgentSelf {
				next = append(next, v)
				continue
			}
			entry := selfReport[v.ID]
			vr := vrep[v.ID]
			nv := Verdict{ID: v.ID, Evidence: entry.Evidence}
			if vr.Verified {
				nv.Passed = true
				nv.Verifier = domain.VerifierVerifyAgent
				nv.Reasoning = vr.Reasoning
			} else {
				nv.Passed = false
				nv.Verifier = domain.VerifierVerifyAgent
				nv.Reasoning = vr.Reasoning
			}
			next = append(next, nv)
			s.recordVerdict(domain.VerifierVerifyAgent, nv.Passed)
		}
		verdicts = next
	}

	if uerr := s.persistVerifyReports(parentCtx, cycle.ID, attemptSeq, verdicts, previouslyPassed); uerr != nil {
		slog.Warn("agent harness UpsertVerifyReports failed",
			"cmd", logCmd, "operation", "agent.harness.verify.runVerifyChecks.upsert_verify_err",
			"cycle_id", cycle.ID, "attempt_seq", attemptSeq, "err", uerr)
	}

	var failures []string
	for _, v := range verdicts {
		if !v.Passed {
			failures = append(failures, fmt.Sprintf("%s: %s", v.ID, v.Reasoning))
		}
	}
	if len(failures) > 0 {
		return verdicts, strings.Join(failures, "; "), fmt.Errorf("verification failed")
	}
	return verdicts, "", nil
}
