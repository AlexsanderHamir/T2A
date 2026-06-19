package verify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func (s *Service) loadCriteriaSelfReport(ctx context.Context, cycleID string, attemptSeq int64, expected map[string]struct{}) (map[string]reports.CriteriaEntry, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.loadCriteriaSelfReport",
		"cycle_id", cycleID, "attempt_seq", attemptSeq, "expected", len(expected))
	selfReport, err := reports.ParseCriteriaReport(s.reportDir, cycleID, expected)
	if err == nil {
		return selfReport, nil
	}
	if !errors.Is(err, reports.ErrCriteriaReportMissing) {
		return nil, err
	}
	rows, err := s.store.ListCriteriaReportsForCycle(ctx, cycleID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]reports.CriteriaEntry, len(expected))
	for _, row := range rows {
		if row.AttemptSeq != attemptSeq {
			continue
		}
		if _, want := expected[row.CriterionID]; !want {
			continue
		}
		out[row.CriterionID] = reports.CriteriaEntry{
			ID:          row.CriterionID,
			ClaimedDone: row.ClaimedDone,
			Evidence:    row.Evidence,
		}
	}
	for id := range expected {
		if _, ok := out[id]; !ok {
			return nil, fmt.Errorf("%w: criterion %q missing from DB fallback", reports.ErrCriteriaReportMissing, id)
		}
	}
	return out, nil
}

// PersistCriteriaReports mirrors parsed criteria-report rows for one attempt.
func (s *Service) PersistCriteriaReports(
	ctx context.Context,
	cycleID string,
	attemptSeq int64,
	criteria []store.ChecklistVerifyItem,
	previouslyPassed map[string]Verdict,
	selfReport map[string]reports.CriteriaEntry,
) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.persistCriteriaReports",
		"cycle_id", cycleID, "attempt_seq", attemptSeq)
	entries := make([]store.CriteriaReportEntry, 0, len(criteria))
	for _, it := range criteria {
		if _, locked := previouslyPassed[it.ID]; locked {
			continue
		}
		e, ok := selfReport[it.ID]
		if !ok {
			continue
		}
		entries = append(entries, store.CriteriaReportEntry{
			CriterionID: it.ID,
			ClaimedDone: e.ClaimedDone,
			Evidence:    e.Evidence,
		})
	}
	return s.store.UpsertCriteriaReports(ctx, cycleID, attemptSeq, entries)
}

func (s *Service) persistVerifyReports(
	ctx context.Context,
	cycleID string,
	attemptSeq int64,
	verdicts []Verdict,
	previouslyPassed map[string]Verdict,
) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.persistVerifyReports",
		"cycle_id", cycleID, "attempt_seq", attemptSeq, "verdict_count", len(verdicts))
	entries := make([]store.VerifyReportEntry, 0, len(verdicts))
	for _, v := range verdicts {
		if _, locked := previouslyPassed[v.ID]; locked {
			continue
		}
		entries = append(entries, store.VerifyReportEntry{
			CriterionID:  v.ID,
			Verified:     v.Passed,
			VerifierKind: v.Verifier,
			Reasoning:    v.Reasoning,
		})
	}
	return s.store.UpsertVerifyReports(ctx, cycleID, attemptSeq, entries)
}

func (s *Service) loadEligibleCommits(ctx context.Context, cycleID string) []domain.TaskCycleCommit {
	commits, _ := s.store.ListEligibleCommitsForCycle(ctx, cycleID)
	return commits
}
