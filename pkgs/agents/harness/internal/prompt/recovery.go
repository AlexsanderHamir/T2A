package prompt

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

const recoveryLogCmd = "taskapi"

// RecoveryKind selects the structured delta template for a resumed Cursor session.
type RecoveryKind string

const (
	RecoveryVerifyImplementation  RecoveryKind = "verify_implementation_fail"
	RecoveryCriteriaReportInvalid RecoveryKind = "criteria_report_invalid"
	RecoveryCriteriaReportMissing RecoveryKind = "criteria_report_missing"
	RecoveryProcessRestart        RecoveryKind = "process_restart"
	RecoveryOperatorRetryResume   RecoveryKind = "operator_retry_resume"
	RecoveryVerifyInfra           RecoveryKind = "verify_infra_retry"
	RecoveryVerifyFeedback        RecoveryKind = "verify_feedback_carry"
)

// CriterionFailure is one failed criterion for structured verify recovery text.
type CriterionFailure struct {
	ID        string
	Reasoning string
	Verifier  string
}

// CommandEvidenceLine is a compact command-run summary for verify recovery deltas.
type CommandEvidenceLine struct {
	CriterionID string
	Command     string
	ExitCode    int
	Preview     string
}

// RecoveryContext carries harness state into delta-only stdin prompts (ADR-0031).
type RecoveryContext struct {
	Kind          RecoveryKind
	Phase         domain.Phase
	CycleID       string
	AttemptSeq    int64
	VerifyAttempt int
	ReportPath    string

	FailedCriteria       []CriterionFailure
	LockedCriteria       []string
	ReportParseErr       string
	ExpectedIDs          []string
	CommandEvidenceDelta []CommandEvidenceLine
	ScopeFiles           []string
	FailureClass         string
	FailureReason        string
	InterruptedPhase     domain.Phase
	GitPorcelain         string
	PriorVerifyFeedback  string
}

const (
	recoveryMaxTotalBytes       = 8 * 1024
	recoveryMaxReasoningRunes   = 2048
	recoverySectionContinuation = "## Continuation (Hamix attempt %d)"
)

// ComposeRecoveryDelta builds a self-contained follow-up prompt for --resume.
//
//funclogmeasure:skip category=hot-path reason="Pure prompt compose without I/O; caller logs byte metrics."
func ComposeRecoveryDelta(ctx RecoveryContext) string {
	slog.Debug("trace", "cmd", recoveryLogCmd, "operation", "prompt.ComposeRecoveryDelta",
		"kind", string(ctx.Kind), "phase", string(ctx.Phase))
	var b strings.Builder
	if ctx.Phase == domain.PhaseVerify {
		composeVerifyRecoveryDelta(&b, ctx)
	} else {
		composeExecuteRecoveryDelta(&b, ctx)
	}
	out := truncateRecoveryBytes(b.String(), recoveryMaxTotalBytes)
	slog.Debug("trace", "cmd", recoveryLogCmd, "operation", "prompt.ComposeRecoveryDelta.done",
		"recovery_hint_kind", string(ctx.Kind), "recovery_hint_bytes", len(out))
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure string builder; ComposeRecoveryDelta logs byte metrics."
func composeExecuteRecoveryDelta(b *strings.Builder, ctx RecoveryContext) {
	fmt.Fprintf(b, recoverySectionContinuation+"\n\n", ctx.AttemptSeq)
	b.WriteString("You are continuing the same Cursor session. Do not restart discovery or revert locked work.\n\n")
	b.WriteString("### What changed\n\n")
	switch ctx.Kind {
	case RecoveryVerifyImplementation:
		b.WriteString("Verification rejected the implementation. Address the failures below.\n\n")
		b.WriteString(FormatVerifyFailuresStructured(ctx.FailedCriteria))
	case RecoveryCriteriaReportInvalid:
		b.WriteString(ComposeCriteriaReportRecoveryDelta(ctx.ReportPath, ctx.ReportParseErr, ctx.ExpectedIDs, ctx.LockedCriteria))
	case RecoveryCriteriaReportMissing:
		b.WriteString("The criteria self-report file is missing.\n\n")
		if ctx.ReportPath != "" {
			fmt.Fprintf(b, "Write it at: `%s`\n\n", ctx.ReportPath)
		}
		if len(ctx.ExpectedIDs) > 0 {
			b.WriteString("Expected criterion IDs: ")
			b.WriteString(strings.Join(ctx.ExpectedIDs, ", "))
			b.WriteString("\n\n")
		}
	case RecoveryProcessRestart:
		fmt.Fprintf(b, "The worker restarted during %s. Inspect the tree and continue from the last known good state.\n\n", ctx.InterruptedPhase)
		if ctx.FailureReason != "" {
			fmt.Fprintf(b, "Last failure: %s\n\n", ctx.FailureReason)
		}
	case RecoveryOperatorRetryResume:
		b.WriteString("Operator chose Resume from failure on a new task attempt.\n\n")
		if ctx.FailureClass != "" {
			fmt.Fprintf(b, "Failure class: %s\n", ctx.FailureClass)
		}
		if ctx.FailureReason != "" {
			fmt.Fprintf(b, "Failure reason: %s\n\n", ctx.FailureReason)
		}
		if len(ctx.ScopeFiles) > 0 {
			b.WriteString("Scope lock (do not modify outside this set):\n")
			for _, f := range ctx.ScopeFiles {
				fmt.Fprintf(b, "- %s\n", f)
			}
			b.WriteString("\n")
		}
	default:
		b.WriteString("Continue the in-progress execute work.\n\n")
	}
	b.WriteString("### Do this next\n\n")
	b.WriteString("1. Fix the issue described above.\n")
	if ctx.ReportPath != "" && ctx.Kind != RecoveryCriteriaReportInvalid && ctx.Kind != RecoveryCriteriaReportMissing {
		fmt.Fprintf(b, "2. Update `%s` for active criteria only.\n", ctx.ReportPath)
	}
	b.WriteString("\n### Do not\n\n")
	if len(ctx.LockedCriteria) > 0 {
		b.WriteString("- Re-do locked criteria: ")
		b.WriteString(strings.Join(ctx.LockedCriteria, ", "))
		b.WriteString("\n")
	}
	b.WriteString("- Amend, rebase, or squash commits from this cycle\n\n")
	if ctx.ReportPath != "" {
		b.WriteString("### Artifacts\n\n")
		fmt.Fprintf(b, "- criteria-report.json: `%s` (schema v1, claimed_done + evidence per active id)\n", ctx.ReportPath)
	}
}

//funclogmeasure:skip category=hot-path reason="Pure string builder; ComposeRecoveryDelta logs byte metrics."
func composeVerifyRecoveryDelta(b *strings.Builder, ctx RecoveryContext) {
	fmt.Fprintf(b, recoverySectionContinuation+"\n\n", ctx.AttemptSeq)
	b.WriteString("You are continuing the same verify Cursor session. Do not modify source files.\n\n")
	b.WriteString("### What changed\n\n")
	switch ctx.Kind {
	case RecoveryVerifyInfra:
		b.WriteString("Infrastructure or command checks produced new evidence since the last verify attempt.\n\n")
		formatCommandEvidenceDelta(b, ctx.CommandEvidenceDelta)
	case RecoveryVerifyFeedback:
		b.WriteString("Prior verification feedback still applies. Re-run judgment with any new evidence.\n\n")
		if ctx.PriorVerifyFeedback != "" {
			b.WriteString("### Prior verify feedback\n\n")
			b.WriteString(ctx.PriorVerifyFeedback)
			b.WriteString("\n\n")
		}
		formatCommandEvidenceDelta(b, ctx.CommandEvidenceDelta)
	default:
		b.WriteString("Continue verification for this cycle.\n\n")
	}
	if len(ctx.FailedCriteria) > 0 {
		b.WriteString(FormatVerifyFailuresStructured(ctx.FailedCriteria))
	}
	b.WriteString("### Do this next\n\n")
	b.WriteString("1. Re-evaluate active criteria and write the verify report only.\n\n")
	if len(ctx.LockedCriteria) > 0 {
		b.WriteString("### Do not\n\n")
		b.WriteString("- Re-evaluate locked criteria: ")
		b.WriteString(strings.Join(ctx.LockedCriteria, ", "))
		b.WriteString("\n")
	}
}

//funclogmeasure:skip category=hot-path reason="Pure string builder; ComposeRecoveryDelta logs byte metrics."
func formatCommandEvidenceDelta(b *strings.Builder, lines []CommandEvidenceLine) {
	if len(lines) == 0 {
		return
	}
	b.WriteString("### New command evidence\n\n")
	for _, ev := range lines {
		fmt.Fprintf(b, "- [%s] `%s` exit=%d\n", ev.CriterionID, ev.Command, ev.ExitCode)
		if p := strings.TrimSpace(ev.Preview); p != "" {
			fmt.Fprintf(b, "  ```\n%s\n  ```\n", truncateRecoveryRunes(p, 512))
		}
	}
	b.WriteString("\n")
}

// FormatVerifyFailuresStructured renders per-criterion failure blocks for resume deltas.
//
//funclogmeasure:skip category=hot-path reason="Pure string format without I/O."
func FormatVerifyFailuresStructured(failures []CriterionFailure) string {
	if len(failures) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("### Verification failures\n\n")
	for _, f := range failures {
		verifier := f.Verifier
		if verifier == "" {
			verifier = "verify"
		}
		fmt.Fprintf(&b, "- **[%s]** (%s)\n", f.ID, verifier)
		if r := strings.TrimSpace(f.Reasoning); r != "" {
			fmt.Fprintf(&b, "  Reasoning: %s\n", truncateRecoveryRunes(r, recoveryMaxReasoningRunes))
		}
		b.WriteString("  Required: address this before claiming done again.\n")
	}
	b.WriteString("\n")
	return b.String()
}

// ComposeCriteriaReportRecoveryDelta builds the invalid-report recovery section.
//
//funclogmeasure:skip category=hot-path reason="Pure string format without I/O."
func ComposeCriteriaReportRecoveryDelta(reportPath, parseErr string, expected, locked []string) string {
	var b strings.Builder
	b.WriteString("The criteria self-report JSON is invalid or incomplete.\n\n")
	if parseErr != "" {
		fmt.Fprintf(&b, "Parse error: %s\n\n", parseErr)
	}
	if reportPath != "" {
		fmt.Fprintf(&b, "Fix the file at: `%s`\n\n", reportPath)
	}
	if len(expected) > 0 {
		sort.Strings(expected)
		b.WriteString("Expected criterion IDs: ")
		b.WriteString(strings.Join(expected, ", "))
		b.WriteString("\n\n")
	}
	b.WriteString("Schema:\n```json\n{\"criteria\":[{\"id\":\"<id>\",\"claimed_done\":true,\"evidence\":\"...\"}]}\n```\n\n")
	if len(locked) > 0 {
		b.WriteString("Locked criteria are already satisfied; omit them from the report.\n\n")
	}
	return b.String()
}

//funclogmeasure:skip category=hot-path reason="Pure truncation helper without I/O."
func truncateRecoveryRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

//funclogmeasure:skip category=hot-path reason="Pure truncation helper without I/O."
func truncateRecoveryBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
