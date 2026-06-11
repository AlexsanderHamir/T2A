package harness

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

const cycleCommitMarkerPrefix = "t2a:cycle="

func cycleCommitMarker(cycleID string) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.cycleCommitMarker", "cycle_id", cycleID)
	return cycleCommitMarkerPrefix + cycleID
}

func appendResumeNotice(prompt string, cycle *domain.TaskCycle, interruptedPhase domain.Phase, commitPolicyOn bool) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.appendResumeNotice",
		"cycle_id", cycleIDOrEmpty(cycle), "phase", string(interruptedPhase))
	if cycle == nil {
		return prompt
	}
	var b strings.Builder
	b.WriteString("## Worker resume notice\n\n")
	b.WriteString("This is a **resume** of an in-flight cycle, not a new task. ")
	b.WriteString("The server restarted while this cycle was running ")
	b.WriteString(fmt.Sprintf("(cycle_id=%s, interrupted during %s).\n\n", cycle.ID, interruptedPhase))
	b.WriteString("Before changing anything:\n")
	b.WriteString("1. Inspect the working tree you were given (`git status`, read relevant files).\n")
	b.WriteString("2. Continue from that state; do not revert work that satisfies locked criteria below.\n")
	if commitPolicyOn {
		b.WriteString(fmt.Sprintf("3. If the working tree is clean, inspect commits tagged with `%s` ", cycleCommitMarker(cycle.ID)))
		b.WriteString(fmt.Sprintf("(e.g. `git log --grep='%s' --oneline`).\n", cycleCommitMarker(cycle.ID)))
		b.WriteString("4. A clean tree with no matching commits does **not** mean the task succeeded — complete remaining criteria and write the criteria report.\n")
	} else {
		b.WriteString("3. If the working tree is clean, inspect recent commit history since ")
		b.WriteString(cycle.StartedAt.UTC().Format("2006-01-02T15:04:05Z"))
		b.WriteString(" on the current branch.\n")
		b.WriteString("4. A clean tree does **not** mean the task succeeded — complete remaining criteria and write the criteria report.\n")
	}
	b.WriteString("\n")
	return b.String() + prompt
}

func appendGitCommitPolicy(prompt string, cycleID string, enabled bool) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.appendGitCommitPolicy", "cycle_id", cycleID, "enabled", enabled)
	if !enabled || strings.TrimSpace(cycleID) == "" {
		return prompt
	}
	marker := cycleCommitMarker(cycleID)
	var b strings.Builder
	b.WriteString("## Git commits (required)\n\n")
	b.WriteString("Before you finish this execute phase, commit all work that satisfies criteria you are claiming. ")
	b.WriteString("Every commit message MUST include the marker:\n")
	b.WriteString(fmt.Sprintf("  %s\n\n", marker))
	b.WriteString("You may commit incrementally during the run using the same marker.\n")
	b.WriteString("Do not push. Do not amend unrelated history.\n\n")
	return b.String() + prompt
}

func appendGitNoCommitPolicy(prompt string) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.appendGitNoCommitPolicy")
	return "## Git commits (forbidden)\n\nDo not create git commits during this execute phase. Leave changes uncommitted.\n\n" + prompt
}

func verifyDiffSection(workingDir, cycleID string, commitPolicyOn bool) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.verifyDiffSection",
		"cycle_id", cycleID, "commit_policy_on", commitPolicyOn)
	diff, err := gitDiff(workingDir, "HEAD")
	if err != nil {
		return "(diff unavailable: " + err.Error() + ")"
	}
	if len(strings.TrimSpace(diff)) > 0 {
		return diff
	}
	if commitPolicyOn && strings.TrimSpace(cycleID) != "" {
		return fmt.Sprintf("Working tree is clean; inspect HEAD and/or git log --grep='%s' for this cycle's commits.", cycleCommitMarker(cycleID))
	}
	return diff
}

func cycleIDOrEmpty(cycle *domain.TaskCycle) string {
	if cycle == nil {
		slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.cycleIDOrEmpty", "cycle_id", "")
		return ""
	}
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.cycleIDOrEmpty", "cycle_id", cycle.ID)
	return cycle.ID
}
