package worker

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// injectCriteria appends the Done-criteria block to the operator's
// initial prompt. alreadyVerified is the set of criterion IDs proven
// passed in earlier retry attempts (carried across the retry loop in
// processState.previouslyPassed); when non-empty, those items render
// under a separate "Already verified" header and are omitted from the
// active checklist + the report schema's expected-IDs set so the
// agent doesn't waste tokens re-doing settled work.
//
// reportPath is the absolute path the worker has chosen for this
// cycle's criteria-report.json (under Options.ReportDir, not under
// the operator's RepoRoot). The prompt renders that absolute path
// verbatim so the agent CLI writes outside the working tree and never
// dirties the operator's repo. cycleID is retained for the trace log
// only; it is no longer baked into a relative path.
func injectCriteria(prompt string, items []store.ChecklistVerifyItem, cycleID, reportPath string, alreadyVerified map[string]criterionVerdict) string {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.injectCriteria",
		"cycle_id", cycleID, "items", len(items), "already_verified", len(alreadyVerified))
	if len(items) == 0 {
		return prompt
	}
	active := make([]store.ChecklistVerifyItem, 0, len(items))
	locked := make([]store.ChecklistVerifyItem, 0, len(alreadyVerified))
	for _, it := range items {
		if _, ok := alreadyVerified[it.ID]; ok {
			locked = append(locked, it)
			continue
		}
		active = append(active, it)
	}

	var b strings.Builder
	b.WriteString(prompt)

	if len(locked) > 0 {
		b.WriteString("\n\n## Already verified (do not re-do)\n\n")
		b.WriteString("These criteria were proven passed in an earlier attempt. Do not undo or modify the work that satisfied them; do not include them in your report.\n\n")
		for _, it := range locked {
			b.WriteString(fmt.Sprintf("- [%s] %s\n", it.ID, it.Text))
		}
	}

	if len(active) == 0 {
		// All criteria already passed in earlier attempts; the agent
		// has nothing to do but the worker's loop still expects an
		// execute pass. Render an empty active-section header so the
		// prompt structure stays predictable for downstream tooling.
		b.WriteString("\n\n## Done criteria (required)\n\nAll criteria are already verified. Re-run is a no-op; the worker will exit successfully.\n")
		return b.String()
	}

	b.WriteString("\n\n## Done criteria (required)\n\n")
	b.WriteString("You must satisfy every criterion below. When finished, write a JSON report at:\n")
	b.WriteString(fmt.Sprintf("`%s`\n\n", reportPath))
	b.WriteString("Schema:\n```json\n{\"criteria\":[{\"id\":\"<id>\",\"claimed_done\":true,\"evidence\":\"...\"}]}\n```\n")
	if len(locked) > 0 {
		b.WriteString("(Report only the criteria below; do NOT include already-verified IDs.)\n")
	}
	b.WriteString("\n")
	for _, it := range active {
		b.WriteString(fmt.Sprintf("- [%s] %s\n", it.ID, it.Text))
		if strings.TrimSpace(it.Check) != "" {
			b.WriteString(fmt.Sprintf("  (deterministic check will run: %s)\n", it.Check))
		}
	}
	return b.String()
}

func appendVerifyFeedback(prompt string, feedback string) string {
	feedback = strings.TrimSpace(feedback)
	if feedback == "" {
		return prompt
	}
	return prompt + "\n\n## Previous verification feedback\n\n" + feedback + "\n"
}
