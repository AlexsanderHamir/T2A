package harness

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// appendOperatorRetryResumeNotice is for cross-cycle "Resume from failure" attempts.
// Unlike appendResumeNotice (ADR-0006 in-process restart), this cycle is new while
// git work and indexed commits may carry over from the parent attempt.
func appendOperatorRetryResumeNotice(prompt string, cycle *domain.TaskCycle, parentCommits []domain.TaskCycleCommit) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.appendOperatorRetryResumeNotice",
		"cycle_id", cycleIDOrEmpty(cycle), "parent_commits", len(parentCommits))
	if cycle == nil {
		return prompt
	}
	var b strings.Builder
	b.WriteString("## Operator retry — resume from failure\n\n")
	b.WriteString("This is a **new execution attempt** continuing work from a failed prior attempt ")
	b.WriteString(fmt.Sprintf("(new cycle_id=%s).\n\n", cycle.ID))
	b.WriteString("Before changing anything:\n")
	b.WriteString("1. Inspect the working tree you were given (`git status`, read relevant files).\n")
	b.WriteString("2. Continue from that state; do not revert work that satisfies locked criteria below.\n")
	if block := formatKnownCommitsForResume(parentCommits); block != "" {
		b.WriteString("3. ")
		b.WriteString(strings.TrimSpace(block))
		b.WriteString("Those commits are already indexed — do **not** list them in `criteria-report.json` unless you create **new** commits in this attempt.\n")
		b.WriteString("4. A clean tree does **not** mean the task succeeded — complete remaining criteria and write the criteria report.\n")
	} else {
		b.WriteString("3. A clean tree does **not** mean the task succeeded — complete remaining criteria and write the criteria report.\n")
	}
	b.WriteString("\n")
	return b.String() + prompt
}

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

func appendGitCommitPolicy(prompt string, operatorResume bool) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.appendGitCommitPolicy",
		"operator_resume", operatorResume)
	var b strings.Builder
	b.WriteString("## Git commits (required)\n\n")
	b.WriteString("Before you finish this execute phase, commit all work that satisfies criteria you are claiming. ")
	if operatorResume {
		b.WriteString("In `criteria-report.json`, list **only** commits created in this attempt (`cycle_base_sha..HEAD`). ")
		b.WriteString("Omit commits from prior attempts — the worker already indexed those. ")
		b.WriteString("If you made no new commits but all criteria are satisfied, omit `commits` or use an empty array.\n\n")
	} else {
		b.WriteString("List every commit SHA and branch in `criteria-report.json` under `commits`.\n\n")
	}
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
