package prompt

import (
	"fmt"
	"strings"
)

// ChecklistItem is one Done-criteria row for execute prompt injection.
type ChecklistItem struct {
	ID   string
	Text string
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// InjectCriteria prepends the Done-criteria block before the operator's
// initial prompt. alreadyVerified is the set of criterion IDs proven
// passed in earlier retry attempts; when non-empty, those items render
// under a separate "Already verified" header and are omitted from the
// active checklist.
//
// reportPath is the absolute path the worker has chosen for this
// cycle's criteria-report.json (under Options.ReportDir, not under
// the operator's RepoRoot).
func InjectCriteria(prompt string, items []ChecklistItem, reportPath string, alreadyVerified map[string]struct{}) string {
	if len(items) == 0 {
		return prompt
	}
	active := make([]ChecklistItem, 0, len(items))
	locked := make([]ChecklistItem, 0, len(alreadyVerified))
	for _, it := range items {
		if _, ok := alreadyVerified[it.ID]; ok {
			locked = append(locked, it)
			continue
		}
		active = append(active, it)
	}

	var criteria strings.Builder

	if len(locked) > 0 {
		criteria.WriteString("\n\n## Already verified (do not re-do)\n\n")
		criteria.WriteString("These criteria were proven passed in an earlier attempt. Do not undo or modify the work that satisfied them; do not include them in your report.\n\n")
		for _, it := range locked {
			criteria.WriteString(fmt.Sprintf("- [%s] %s\n", it.ID, it.Text))
		}
	}

	if len(active) == 0 {
		criteria.WriteString("\n\n## Done criteria (required)\n\nAll criteria are already verified. Re-run is a no-op; the worker will exit successfully.\n")
		return strings.TrimPrefix(criteria.String(), "\n\n") + "\n\n" + prompt
	}

	criteria.WriteString("\n\n## Done criteria (required)\n\n")
	criteria.WriteString("You must satisfy every criterion below. When finished, write a JSON report at:\n")
	criteria.WriteString(fmt.Sprintf("`%s`\n\n", reportPath))
	criteria.WriteString("Schema:\n```json\n{\"criteria\":[{\"id\":\"<id>\",\"claimed_done\":true,\"evidence\":\"...\"}]}\n```\n")
	criteria.WriteString("claimed_done is your assertion that you completed the work; the verification agent independently decides whether each criterion is satisfied.\n")
	if len(locked) > 0 {
		criteria.WriteString("(Report only the criteria below; do NOT include already-verified IDs.)\n")
	}
	criteria.WriteString("\n")
	for _, it := range active {
		criteria.WriteString(fmt.Sprintf("- [%s] %s\n", it.ID, it.Text))
	}
	return strings.TrimPrefix(criteria.String(), "\n\n") + "\n\n" + prompt
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// AppendVerifyFeedback appends prior verification feedback when non-empty.
func AppendVerifyFeedback(prompt string, feedback string) string {
	feedback = strings.TrimSpace(feedback)
	if feedback == "" {
		return prompt
	}
	return prompt + "\n\n## Previous verification feedback\n\n" + feedback + "\n"
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// AppendExecuteHarnessFeedback appends execute-phase harness feedback when non-empty.
func AppendExecuteHarnessFeedback(prompt string, feedback string) string {
	feedback = strings.TrimSpace(feedback)
	if feedback == "" {
		return prompt
	}
	return prompt + "\n\n## Execute harness feedback\n\n" + feedback + "\n"
}
