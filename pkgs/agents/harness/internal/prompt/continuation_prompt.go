package prompt

import (
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
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

// ComposeContinuation prepends continuation context before the base execute prompt.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
	if block := FormatKnownCommitsForResume(in.Commits); block != "" {
		b.WriteString("### Known commits (indexed for this task)\n\n")
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
