package harness

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

type verifyCriterionPayload struct {
	CriterionID  string `json:"criterion_id"`
	Text         string `json:"text,omitempty"`
	Verified     bool   `json:"verified"`
	VerifierKind string `json:"verifier_kind,omitempty"`
	Reasoning    string `json:"reasoning,omitempty"`
	Evidence     string `json:"evidence,omitempty"`
}

type verifySnapshotPayload struct {
	AttemptSeq  int64                    `json:"attempt_seq"`
	PassedCount int                      `json:"passed_count"`
	FailedCount int                      `json:"failed_count"`
	Criteria    []verifyCriterionPayload `json:"criteria"`
}

type verifyPhaseDetailsPayload struct {
	Verification verifySnapshotPayload `json:"verification"`
}

func criterionTextIndex(items []store.ChecklistVerifyItem) map[string]string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.criterionTextIndex", "items", len(items))
	out := make(map[string]string, len(items))
	for _, it := range items {
		out[it.ID] = it.Text
	}
	return out
}

func countVerdictOutcome(verdicts []criterionVerdict) (passed, failed int) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.countVerdictOutcome", "verdicts", len(verdicts))
	for _, v := range verdicts {
		if v.passed {
			passed++
		} else {
			failed++
		}
	}
	return passed, failed
}

// formatVerifyPhaseSummary builds the human-readable phase.summary written
// into phase_failed / phase_completed audit mirrors. Unlike verifyErr.Error()
// ("verification failed"), this includes counts and per-criterion reasoning
// so the SPA audit timeline and event detail page can explain the outcome
// without a second round-trip to the verdicts API.
func formatVerifyPhaseSummary(
	criteria []store.ChecklistVerifyItem,
	verdicts []criterionVerdict,
	succeeded bool,
) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.formatVerifyPhaseSummary",
		"criteria", len(criteria), "verdicts", len(verdicts), "succeeded", succeeded)
	textByID := criterionTextIndex(criteria)
	n := len(verdicts)
	if n == 0 {
		if succeeded {
			return "verify complete"
		}
		return "verification failed"
	}
	passed, failed := countVerdictOutcome(verdicts)
	if succeeded {
		return fmt.Sprintf("All %d criteria verified", passed)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d of %d criteria failed", failed, n)
	for _, v := range verdicts {
		if v.passed {
			continue
		}
		text := textByID[v.id]
		if text == "" {
			text = v.id
		}
		b.WriteString("\n\n- ")
		b.WriteString(text)
		if v.reasoning != "" {
			b.WriteString(" — ")
			b.WriteString(v.reasoning)
		}
	}
	return b.String()
}

func encodeVerifyPhaseDetails(
	attemptSeq int64,
	criteria []store.ChecklistVerifyItem,
	verdicts []criterionVerdict,
) []byte {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.encodeVerifyPhaseDetails",
		"attempt_seq", attemptSeq, "criteria", len(criteria), "verdicts", len(verdicts))
	textByID := criterionTextIndex(criteria)
	passed, failed := countVerdictOutcome(verdicts)
	rows := make([]verifyCriterionPayload, 0, len(verdicts))
	for _, v := range verdicts {
		row := verifyCriterionPayload{
			CriterionID: v.id,
			Text:        textByID[v.id],
			Verified:    v.passed,
		}
		if v.verifier != "" {
			row.VerifierKind = string(v.verifier)
		}
		if v.reasoning != "" {
			row.Reasoning = v.reasoning
		}
		if v.evidence != "" {
			row.Evidence = v.evidence
		}
		rows = append(rows, row)
	}
	payload := verifyPhaseDetailsPayload{
		Verification: verifySnapshotPayload{
			AttemptSeq:  attemptSeq,
			PassedCount: passed,
			FailedCount: failed,
			Criteria:    rows,
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return []byte("{}")
	}
	return b
}
