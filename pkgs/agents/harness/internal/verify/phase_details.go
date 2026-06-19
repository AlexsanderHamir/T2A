package verify

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
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.criterionTextIndex", "items", len(items))
	out := make(map[string]string, len(items))
	for _, it := range items {
		out[it.ID] = it.Text
	}
	return out
}

func countVerdictOutcome(verdicts []Verdict) (passed, failed int) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.countVerdictOutcome", "verdicts", len(verdicts))
	for _, v := range verdicts {
		if v.Passed {
			passed++
		} else {
			failed++
		}
	}
	return passed, failed
}

// FormatPhaseSummary builds human-readable verify phase.summary for audit mirrors.
func FormatPhaseSummary(
	criteria []store.ChecklistVerifyItem,
	verdicts []Verdict,
	succeeded bool,
) string {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.FormatPhaseSummary",
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
		if v.Passed {
			continue
		}
		text := textByID[v.ID]
		if text == "" {
			text = v.ID
		}
		b.WriteString("\n\n- ")
		b.WriteString(text)
		if v.Reasoning != "" {
			b.WriteString(" — ")
			b.WriteString(v.Reasoning)
		}
	}
	return b.String()
}

// EncodePhaseDetails returns structured verify phase details JSON for phase rows.
func EncodePhaseDetails(
	attemptSeq int64,
	criteria []store.ChecklistVerifyItem,
	verdicts []Verdict,
) []byte {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.EncodePhaseDetails",
		"attempt_seq", attemptSeq, "criteria", len(criteria), "verdicts", len(verdicts))
	textByID := criterionTextIndex(criteria)
	passed, failed := countVerdictOutcome(verdicts)
	rows := make([]verifyCriterionPayload, 0, len(verdicts))
	for _, v := range verdicts {
		row := verifyCriterionPayload{
			CriterionID: v.ID,
			Text:        textByID[v.ID],
			Verified:    v.Passed,
		}
		if v.Verifier != "" {
			row.VerifierKind = string(v.Verifier)
		}
		if v.Reasoning != "" {
			row.Reasoning = v.Reasoning
		}
		if v.Evidence != "" {
			row.Evidence = v.Evidence
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
