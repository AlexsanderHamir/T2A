package harness

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/verify"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// CursorResumeMode is logged on every runner.Run for ADR-0031 observability.
type CursorResumeMode string

const (
	CursorResumeFresh    CursorResumeMode = "fresh"
	CursorResumeContinue CursorResumeMode = "resume"
	CursorResumeFallback CursorResumeMode = "resume_fallback"
)

// CursorResumeDecision is the harness policy output for one runner.Run.
type CursorResumeDecision struct {
	Mode            CursorResumeMode
	ResumeSessionID string
	Prompt          string
	RecoveryKind    prompt.RecoveryKind
	DenyReason      string
}

func (h *Harness) planExecuteRun(
	ctx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	state *processState,
	opts cycleLoopOpts,
) (CursorResumeDecision, error) {
	decision, err := h.resolveCursorResume(ctx, domain.PhaseExecute, task, cycle, state, opts, false)
	if err != nil {
		slog.Warn("agent harness cursor resume policy failed; using fresh prompt", "cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.planExecuteRun.fallback",
			"cycle_id", cycle.ID, "err", err)
		return h.freshExecuteDecision(ctx, task, cycle, state, opts, "policy_error"), nil
	}
	if decision.Mode == CursorResumeFresh {
		decision.Prompt = h.composeExecutePrompt(ctx, task, cycle, state, opts)
	}
	return decision, nil
}

//funclogmeasure:skip category=hot-path reason="Delegates to freshExecuteDecision; resume_fallback logged at invoke site."
func (h *Harness) planExecuteResumeFallback(
	ctx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	state *processState,
	opts cycleLoopOpts,
) CursorResumeDecision {
	dec := h.freshExecuteDecision(ctx, task, cycle, state, opts, "resume_failed")
	dec.Mode = CursorResumeFallback
	return dec
}

//funclogmeasure:skip category=hot-path reason="Pure decision struct; composeExecutePrompt logs at invoke site."
func (h *Harness) freshExecuteDecision(
	ctx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	state *processState,
	opts cycleLoopOpts,
	denyReason string,
) CursorResumeDecision {
	return CursorResumeDecision{
		Mode:       CursorResumeFresh,
		Prompt:     h.composeExecutePrompt(ctx, task, cycle, state, opts),
		DenyReason: denyReason,
	}
}

func (h *Harness) planVerifyRun(
	ctx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	state *processState,
	snap verificationSnapshot,
	verifyAttempt int,
	feedback string,
	cmdEvidence []verify.CommandEvidence,
	selfReport map[string]reports.CriteriaEntry,
) (verify.VerifyRunPlan, error) {
	opts := cycleLoopOpts{
		resumeNotice:     state.resumeNotice,
		interruptedPhase: state.interruptedPhase,
		continuation:     state.continuation,
	}
	decision, err := h.resolveCursorResume(ctx, domain.PhaseVerify, task, cycle, state, opts, false)
	if err != nil {
		slog.Warn("agent harness verify cursor resume policy failed; using fresh prompt", "cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.planVerifyRun.fallback",
			"cycle_id", cycle.ID, "err", err)
		decision = CursorResumeDecision{
			Mode:       CursorResumeFresh,
			Prompt:     h.verifySvc().BuildVerifyPrompt(ctx, task.ID, snap, cycle.ID, state.previouslyPassed, selfReport, feedback, cmdEvidence),
			DenyReason: "policy_error",
		}
	} else if decision.Mode == CursorResumeFresh || decision.Mode == CursorResumeFallback {
		decision.Prompt = h.verifySvc().BuildVerifyPrompt(ctx, task.ID, snap, cycle.ID, state.previouslyPassed, selfReport, feedback, cmdEvidence)
	} else {
		rc := h.buildRecoveryContext(domain.PhaseVerify, task, cycle, state, opts, retryModeFromCycleMeta(cycle))
		rc.CommandEvidenceDelta = commandEvidenceLines(cmdEvidence)
		decision.Prompt = prompt.ComposeRecoveryDelta(rc)
	}
	state.lastCursorResumeMode = decision.Mode
	logRecoveryCompose(decision)
	return verify.VerifyRunPlan{
		Prompt:           decision.Prompt,
		ResumeSessionID:  decision.ResumeSessionID,
		CursorResumeMode: string(decision.Mode),
		RecoveryKind:     string(decision.RecoveryKind),
	}, nil
}

//funclogmeasure:skip category=hot-path reason="Pure mapping from verify evidence to prompt DTO."
func commandEvidenceLines(evidence []verify.CommandEvidence) []prompt.CommandEvidenceLine {
	out := make([]prompt.CommandEvidenceLine, 0, len(evidence))
	for _, ev := range evidence {
		out = append(out, prompt.CommandEvidenceLine{
			CriterionID: ev.CriterionID,
			Command:     ev.Command,
			ExitCode:    ev.ExitCode,
			Preview:     ev.StdoutPreview,
		})
	}
	return out
}

func (h *Harness) resolveCursorResume(
	ctx context.Context,
	phase domain.Phase,
	task *domain.Task,
	cycle *domain.TaskCycle,
	state *processState,
	opts cycleLoopOpts,
	forceFresh bool,
) (CursorResumeDecision, error) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.resolveCursorResume",
		"cycle_id", cycle.ID, "phase", string(phase), "force_fresh", forceFresh)
	if forceFresh {
		return CursorResumeDecision{Mode: CursorResumeFresh, DenyReason: "resume_failed"}, nil
	}
	settings, err := h.store.GetSettings(ctx)
	if err != nil {
		return CursorResumeDecision{}, err
	}
	if !settings.CursorSessionResumeEnabled {
		return CursorResumeDecision{Mode: CursorResumeFresh, DenyReason: "settings_disabled"}, nil
	}
	retryMode := retryModeFromCycleMeta(cycle)
	if retryMode == domain.RetryFresh {
		return CursorResumeDecision{Mode: CursorResumeFresh, DenyReason: "retry_fresh"}, nil
	}
	if opts.resumeNotice && retryMode != domain.RetryResume && phase == domain.PhaseExecute {
		if state.reportTampered {
			return CursorResumeDecision{Mode: CursorResumeFresh, DenyReason: "tamper"}, nil
		}
	}
	if phase == domain.PhaseVerify && h.firstVerifyAfterNewExecute(state) {
		return CursorResumeDecision{Mode: CursorResumeFresh, DenyReason: "verify_fresh_after_execute"}, nil
	}
	if !state.gitSnap.Skipped && state.postExecuteHeadSHA != "" {
		headMatches := true
		current, ok, herr := h.resolveCurrentHeadSHA(ctx, state.gitSnap)
		if herr != nil {
			headMatches = false
		} else if ok {
			headMatches = strings.EqualFold(strings.TrimSpace(current), strings.TrimSpace(state.postExecuteHeadSHA))
		}
		if !headMatches {
			return CursorResumeDecision{Mode: CursorResumeFresh, DenyReason: "head_drift"}, nil
		}
	}
	if state.reportTampered {
		return CursorResumeDecision{Mode: CursorResumeFresh, DenyReason: "tamper"}, nil
	}
	lookupCycleID := h.sessionLookupCycleID(ctx, cycle, phase, retryMode, opts)
	sessionID, err := h.store.LastSessionID(ctx, lookupCycleID, phase)
	if err != nil {
		return CursorResumeDecision{}, err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return CursorResumeDecision{Mode: CursorResumeFresh, DenyReason: "no_session_id"}, nil
	}
	workdir := strings.TrimSpace(h.opts.WorkingDir)
	if repoRoot := strings.TrimSpace(h.repoRootForGit(ctx)); repoRoot != "" {
		workdir = repoRoot
	}
	if workdir == "" {
		return CursorResumeDecision{Mode: CursorResumeFresh, DenyReason: "workspace_mismatch"}, nil
	}
	recoveryCtx := h.buildRecoveryContext(phase, task, cycle, state, opts, retryMode)
	delta := prompt.ComposeRecoveryDelta(recoveryCtx)
	decision := CursorResumeDecision{
		Mode:            CursorResumeContinue,
		ResumeSessionID: sessionID,
		Prompt:          delta,
		RecoveryKind:    recoveryCtx.Kind,
	}
	state.lastCursorResumeMode = decision.Mode
	logRecoveryCompose(decision)
	return decision, nil
}

//funclogmeasure:skip category=hot-path reason="Pure cycle id routing; resolveCursorResume logs policy outcome."
func (h *Harness) sessionLookupCycleID(
	ctx context.Context,
	cycle *domain.TaskCycle,
	phase domain.Phase,
	retryMode domain.RetryMode,
	opts cycleLoopOpts,
) string {
	if retryMode == domain.RetryResume && cycle.ParentCycleID != nil {
		parentID := strings.TrimSpace(*cycle.ParentCycleID)
		if parentID != "" {
			childID, err := h.store.LastSessionID(ctx, cycle.ID, phase)
			if err == nil && strings.TrimSpace(childID) == "" {
				switch {
				case phase == domain.PhaseExecute:
					return parentID
				case phase == domain.PhaseVerify && opts.continuation != nil && opts.continuation.Entry == resumeEntryVerifyOnly:
					return parentID
				}
			}
		}
	}
	return cycle.ID
}

//funclogmeasure:skip category=hot-path reason="Pure state comparison for verify fresh-after-execute deny."
func (h *Harness) firstVerifyAfterNewExecute(state *processState) bool {
	return state.lastVerifyAfterExecuteSeq < state.lastCompletedExecutePhaseSeq
}

//funclogmeasure:skip category=hot-path reason="Pure DTO assembly; ComposeRecoveryDelta logs hint metrics."
func (h *Harness) buildRecoveryContext(
	phase domain.Phase,
	task *domain.Task,
	cycle *domain.TaskCycle,
	state *processState,
	opts cycleLoopOpts,
	retryMode domain.RetryMode,
) prompt.RecoveryContext {
	reportPath := reports.CriteriaReportPath(h.opts.ReportDir, cycle.ID)
	locked := lockedCriterionIDs(state.previouslyPassed)
	expected := activeCriterionIDs(state)
	kind := h.selectRecoveryKind(phase, state, opts, retryMode)
	ctx := prompt.RecoveryContext{
		Kind:                kind,
		Phase:               phase,
		CycleID:             cycle.ID,
		AttemptSeq:          cycle.AttemptSeq,
		VerifyAttempt:       state.verifyAttempt,
		ReportPath:          reportPath,
		FailedCriteria:      failedCriteriaFromVerdicts(state.lastFailedVerdicts),
		LockedCriteria:      locked,
		ReportParseErr:      state.reportParseErr,
		ExpectedIDs:         expected,
		InterruptedPhase:    opts.interruptedPhase,
		PriorVerifyFeedback: state.verifyFeedback,
	}
	if bundle := opts.continuation; bundle != nil && kind == prompt.RecoveryOperatorRetryResume {
		ctx.FailureClass = string(bundle.FailureClass)
		ctx.FailureReason = bundle.FailureReason
		ctx.ScopeFiles = append([]string(nil), bundle.ScopeFiles...)
	}
	if kind == prompt.RecoveryCriteriaReportMissing {
		_ = task
	}
	return ctx
}

//funclogmeasure:skip category=hot-path reason="Pure kind selection from in-memory state."
func (h *Harness) selectRecoveryKind(
	phase domain.Phase,
	state *processState,
	opts cycleLoopOpts,
	retryMode domain.RetryMode,
) prompt.RecoveryKind {
	if phase == domain.PhaseVerify {
		if state.verifyAttempt > 0 {
			return prompt.RecoveryVerifyFeedback
		}
		return prompt.RecoveryVerifyInfra
	}
	if state.reportParseErr != "" {
		if strings.Contains(strings.ToLower(state.reportParseErr), "missing") {
			return prompt.RecoveryCriteriaReportMissing
		}
		return prompt.RecoveryCriteriaReportInvalid
	}
	if retryMode == domain.RetryResume && opts.continuation != nil {
		return prompt.RecoveryOperatorRetryResume
	}
	if opts.resumeNotice {
		return prompt.RecoveryProcessRestart
	}
	if len(state.lastFailedVerdicts) > 0 {
		return prompt.RecoveryVerifyImplementation
	}
	return prompt.RecoveryVerifyImplementation
}

//funclogmeasure:skip category=hot-path reason="Pure verdict to DTO mapping."
func failedCriteriaFromVerdicts(verdicts []criterionVerdict) []prompt.CriterionFailure {
	var out []prompt.CriterionFailure
	for _, v := range verdicts {
		if v.Passed {
			continue
		}
		out = append(out, prompt.CriterionFailure{
			ID:        v.ID,
			Reasoning: v.Reasoning,
			Verifier:  string(v.Verifier),
		})
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure id extraction from locked verdict map."
func lockedCriterionIDs(locked map[string]criterionVerdict) []string {
	if len(locked) == 0 {
		return nil
	}
	ids := make([]string, 0, len(locked))
	for id := range locked {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

//funclogmeasure:skip category=hot-path reason="Pure active checklist id list from state."
func activeCriterionIDs(state *processState) []string {
	expected := make([]string, 0)
	for _, it := range state.verifySnap.Criteria {
		if _, ok := state.previouslyPassed[it.ID]; ok {
			continue
		}
		expected = append(expected, it.ID)
	}
	sort.Strings(expected)
	return expected
}

func logRecoveryCompose(decision CursorResumeDecision) {
	attrs := []any{
		"cmd", harnessLogCmd,
		"operation", "agent.harness.Harness.cursorResume",
		"cursor_resume_mode", string(decision.Mode),
		"recovery_hint_bytes", len(decision.Prompt),
	}
	if decision.DenyReason != "" {
		attrs = append(attrs, "deny_reason", decision.DenyReason)
	}
	if decision.RecoveryKind != "" {
		attrs = append(attrs, "recovery_hint_kind", string(decision.RecoveryKind),
			"recovery_failed_criteria_count", 0)
	}
	slog.Debug("trace", attrs...)
}
