package harness

import (
	"context"
	"errors"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func (h *Harness) bestEffortMirrorExecuteCriteria(
	ctx context.Context,
	cycleID string,
	state *processState,
) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.bestEffortMirrorExecuteCriteria",
		"cycle_id", cycleID)
	if !state.verifySnap.Enabled || len(state.verifySnap.Criteria) == 0 {
		return
	}
	selfReport, err := reports.ParseCriteriaReportPartial(h.opts.ReportDir, cycleID)
	if err != nil {
		if !errors.Is(err, ErrCriteriaReportMissing) {
			slog.Warn("agent harness execute criteria mirror parse failed",
				"cmd", harnessLogCmd, "operation", "agent.harness.bestEffortMirrorExecuteCriteria.parse_err",
				"cycle_id", cycleID, "err", err)
		}
		return
	}
	if uerr := h.persistCriteriaReports(ctx, cycleID, domain.ExecuteCriteriaReportAttemptSeq,
		state.verifySnap.Criteria, state.previouslyPassed, selfReport); uerr != nil {
		slog.Warn("agent harness execute criteria mirror upsert failed",
			"cmd", harnessLogCmd, "operation", "agent.harness.bestEffortMirrorExecuteCriteria.upsert_err",
			"cycle_id", cycleID, "err", uerr)
	}
}
