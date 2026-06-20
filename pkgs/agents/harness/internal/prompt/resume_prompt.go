package prompt

import (
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// AppendOperatorRetryResumeNotice is for cross-cycle "Resume from failure" attempts.
// Unlike AppendResumeNotice (ADR-0006 in-process restart), this cycle is new while
// git work and indexed commits may carry over from the parent attempt.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func AppendOperatorRetryResumeNotice(prompt string, cycle *domain.TaskCycle, parentCommits []domain.TaskCycleCommit) string {
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
	if block := FormatKnownCommitsForResume(parentCommits); block != "" {
		b.WriteString("3. ")
		b.WriteString(strings.TrimSpace(block))
		b.WriteString("Those commits are already indexed for this task — list only **new** commits you create in `commits[]` on your criteria report.\n")
		b.WriteString("4. A clean tree does **not** mean the task succeeded — complete remaining criteria and write the criteria report.\n")
	} else {
		b.WriteString("3. A clean tree does **not** mean the task succeeded — complete remaining criteria and write the criteria report.\n")
	}
	b.WriteString("\n")
	return b.String() + prompt
}

// AppendResumeNotice prepends an in-process worker resume notice.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func AppendResumeNotice(prompt string, cycle *domain.TaskCycle, interruptedPhase domain.Phase, knownCommits []domain.TaskCycleCommit) string {
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
	if block := FormatKnownCommitsForResume(knownCommits); block != "" {
		b.WriteString("3. ")
		b.WriteString(strings.TrimSpace(block))
		b.WriteString("4. A clean tree does **not** mean the task succeeded — complete remaining criteria and write the criteria report.\n")
	} else {
		b.WriteString("3. A clean tree does **not** mean the task succeeded — complete remaining criteria and write the criteria report.\n")
	}
	b.WriteString("\n")
	return b.String() + prompt
}

// AppendGitCommitPolicy appends execute-phase git commit instructions.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func AppendGitCommitPolicy(prompt string, operatorResume bool) string {
	var b strings.Builder
	b.WriteString("## Git commits (required)\n\n")
	b.WriteString("Before you finish this execute phase, commit work that satisfies criteria you are claiming, and list new commits in `commits[]` on your criteria report.\n\n")
	if operatorResume {
		b.WriteString("Create **new** commits only in this attempt; prior attempt SHAs are already indexed.\n\n")
	}
	b.WriteString("Use normal descriptive commit messages only — do **not** embed task IDs, cycle IDs, or `t2a:` markers.\n")
	b.WriteString("Create **new commits only** — fix mistakes with a follow-up commit; never amend, rebase, squash, or delete history.\n")
	b.WriteString("You may commit incrementally during the run. Uncommitted local changes are allowed if you already committed the work you are claiming.\n")
	b.WriteString("Do not push.\n\n")
	return b.String() + prompt
}

// FormatKnownCommitsForResume lists commits already indexed for the task.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FormatKnownCommitsForResume(commits []domain.TaskCycleCommit) string {
	if len(commits) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Known commits already indexed for this task (all prior attempts):\n")
	for _, c := range commits {
		short := c.SHA
		if len(short) > 12 {
			short = short[:12]
		}
		b.WriteString(fmt.Sprintf("- %s — %s\n", short, c.Message))
	}
	b.WriteByte('\n')
	return b.String()
}

// FormatVerifyDiffSection renders the git diff block for verify prompts.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FormatVerifyDiffSection(diff string, fetchErr error) string {
	if fetchErr != nil {
		return "(diff unavailable: " + fetchErr.Error() + ")"
	}
	return diff
}
