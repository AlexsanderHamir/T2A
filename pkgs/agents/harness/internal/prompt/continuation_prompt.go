package prompt

import (
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// ContinuationInput carries cross-cycle resume context for prompt assembly.
type ContinuationInput struct {
	LineageAttempt  int64
	Cycle           *domain.TaskCycle
	FailureClass    string
	FailureReason   string
	FailurePhase    domain.Phase
	ScopeFiles      []string
	Commits         []domain.TaskCycleCommit
	ExecuteFeedback string
	RunnerFeedback  string
	GitDiagnostics  string
	Warnings        []string
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ComposeContinuation prepends continuation context before the base execute prompt.
func ComposeContinuation(base string, in ContinuationInput) string {
	if in.Cycle == nil {
		return base
	}
	var b strings.Builder
	b.WriteString("## Continuation — resume from failure\n\n")
	b.WriteString(fmt.Sprintf("You are continuing work from attempt #%d into attempt #%d (new cycle_id=%s).\n",
		in.LineageAttempt, in.Cycle.AttemptSeq, in.Cycle.ID))
	b.WriteString("Do **not** restart discovery or revert eligible work from prior attempts.\n\n")

	if in.FailureReason != "" {
		b.WriteString("### Prior failure\n\n")
		b.WriteString(fmt.Sprintf("- Class: %s\n", in.FailureClass))
		b.WriteString(fmt.Sprintf("- Reason: %s\n", in.FailureReason))
		if in.FailurePhase != "" {
			b.WriteString(fmt.Sprintf("- Last phase: %s\n", in.FailurePhase))
		}
		b.WriteString("\n")
	}
	if len(in.ScopeFiles) > 0 {
		b.WriteString("### Scope lock (files already touched)\n\n")
		b.WriteString("Continue work on these paths — do not pick a different target:\n")
		for _, f := range in.ScopeFiles {
			b.WriteString("- ")
			b.WriteString(f)
			b.WriteByte('\n')
		}
		b.WriteString("\n")
	}
	if block := FormatCommitsByStatusForResume(in.Commits); block != "" {
		b.WriteString(block)
	}
	if in.ExecuteFeedback != "" {
		b.WriteString("### Execute harness feedback\n\n")
		b.WriteString(in.ExecuteFeedback)
		b.WriteString("\n\n")
	}
	if in.RunnerFeedback != "" {
		b.WriteString("### Prior runner outcome\n\n")
		b.WriteString(in.RunnerFeedback)
		b.WriteString("\n\n")
	}
	if in.GitDiagnostics != "" {
		b.WriteString("### Git working tree (porcelain)\n\n```\n")
		b.WriteString(in.GitDiagnostics)
		b.WriteString("\n```\n\n")
	}
	for _, w := range in.Warnings {
		b.WriteString("> ")
		b.WriteString(w)
		b.WriteByte('\n')
	}
	if len(in.Warnings) > 0 {
		b.WriteByte('\n')
	}
	return b.String() + base
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// FormatCommitsByStatusForResume groups known commits by eligibility status.
func FormatCommitsByStatusForResume(commits []domain.TaskCycleCommit) string {
	if len(commits) == 0 {
		return ""
	}
	var eligible, observed, inherited, other []domain.TaskCycleCommit
	for _, c := range commits {
		switch c.Status {
		case domain.CommitEligible:
			eligible = append(eligible, c)
		case domain.CommitObserved:
			observed = append(observed, c)
		case domain.CommitInherited:
			inherited = append(inherited, c)
		default:
			other = append(other, c)
		}
	}
	var b strings.Builder
	b.WriteString("### Known commits (by status)\n\n")
	writeCommitGroup := func(title string, rows []domain.TaskCycleCommit) {
		if len(rows) == 0 {
			return
		}
		b.WriteString("**")
		b.WriteString(title)
		b.WriteString(":**\n")
		for _, c := range rows {
			short := c.SHA
			if len(short) > 12 {
				short = short[:12]
			}
			b.WriteString("- ")
			b.WriteString(short)
			b.WriteString(" — ")
			b.WriteString(c.Message)
			if c.GateReason != "" {
				b.WriteString(" (")
				b.WriteString(c.GateReason)
				b.WriteString(")")
			}
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	writeCommitGroup("Eligible (verify-ready)", eligible)
	writeCommitGroup("Observed (blocked by gates)", observed)
	writeCommitGroup("Inherited", inherited)
	writeCommitGroup("Other", other)
	b.WriteString("Do **not** re-discover targets when scope files or eligible commits exist above.\n\n")
	return b.String()
}
