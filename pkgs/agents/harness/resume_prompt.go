package harness

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func appendResumeNotice(prompt string, cycle *domain.TaskCycle, interruptedPhase domain.Phase, knownCommits []domain.TaskCycleCommit) string {
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
	if block := formatKnownCommitsForResume(knownCommits); block != "" {
		b.WriteString("3. ")
		b.WriteString(strings.TrimSpace(block))
		b.WriteString("4. A clean tree does **not** mean the task succeeded — complete remaining criteria and write the criteria report.\n")
	} else {
		b.WriteString("3. A clean tree does **not** mean the task succeeded — complete remaining criteria and write the criteria report.\n")
	}
	b.WriteString("\n")
	return b.String() + prompt
}

func appendGitCommitPolicy(prompt string) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.appendGitCommitPolicy")
	var b strings.Builder
	b.WriteString("## Git commits (required)\n\n")
	b.WriteString("Before you finish this execute phase, commit all work that satisfies criteria you are claiming. ")
	b.WriteString("List every commit SHA and branch in `criteria-report.json` under `commits`.\n\n")
	b.WriteString("Use normal descriptive commit messages only — do **not** embed task IDs, cycle IDs, or `t2a:` markers.\n")
	b.WriteString("Create **new commits only** — fix mistakes with a follow-up commit; never amend, rebase, or squash work from this cycle.\n")
	b.WriteString("You may commit incrementally during the run.\n")
	b.WriteString("Do not push.\n\n")
	return b.String() + prompt
}

func verifyDiffSection(workingDir string) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.verifyDiffSection")
	diff, err := gitDiff(workingDir, "HEAD")
	if err != nil {
		return "(diff unavailable: " + err.Error() + ")"
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
