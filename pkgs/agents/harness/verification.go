package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const verificationFailedReason = "verification_failed"

// verifyTamperedError is returned by runVerificationPipeline when the
// post-verify integrity check detects unauthorized changes outside the
// verifier's documented output file. The caller in process.go uses
// errors.As to unwrap and reads (status, reason) so the cycle is
// terminated as failed with a stable, audit-friendly reason. This is
// terminal: a verifier that mutates source cannot be retried — the
// trust property is gone for that cycle.
type verifyTamperedError struct {
	reason string
}

func (e *verifyTamperedError) Error() string {
	if e == nil {
		return ""
	}
	return "verify_tampered: " + e.reason
}

type verificationSnapshot struct {
	enabled      bool
	maxRetries   int
	criteria     []store.ChecklistVerifyItem
	verifyRunner runner.Runner
	verifyModel  string
}

type criterionVerdict struct {
	id        string
	passed    bool
	evidence  string
	verifier  domain.VerifierKind
	reasoning string
}

func (h *Harness) loadVerificationSnapshot(ctx context.Context, taskID string) (verificationSnapshot, error) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.loadVerificationSnapshot",
		"task_id", taskID)
	settings, err := h.store.GetSettings(ctx)
	if err != nil {
		return verificationSnapshot{}, err
	}
	items, err := h.store.ListChecklistForVerify(ctx, taskID)
	if err != nil {
		return verificationSnapshot{}, err
	}
	maxRetries := settings.VerifyMaxRetries
	// The supervisor (cmd/taskapi/run_agentworker.go::applySettings) is
	// the source of truth for which runner verify uses: if the operator
	// configured app_settings.VerifyRunnerName, the supervisor probed
	// and built it and passed it as Options.VerifyRunner. A nil
	// VerifyRunner means either (a) the operator did not configure one
	// (V1 default) or (b) the supervisor's build/probe failed and
	// demoted verify back to the execute runner with a warn — either
	// way, fall back to h.runner here.
	verifyRunner := h.runner
	if h.opts.VerifyRunner != nil {
		verifyRunner = h.opts.VerifyRunner
	}
	snap := verificationSnapshot{
		// Verify runs only when the task has at least one criterion.
		// Zero criteria: skip verify; execute success alone completes the task.
		enabled:      len(items) > 0,
		maxRetries:   maxRetries,
		criteria:     items,
		verifyRunner: verifyRunner,
		verifyModel:  strings.TrimSpace(settings.VerifyRunnerModel),
	}
	return snap, nil
}

func (h *Harness) completeChecklistLegacy(ctx context.Context, taskID string) error {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.completeChecklistLegacy", "task_id", taskID)
	items, err := h.store.ListChecklistForSubject(ctx, taskID)
	if err != nil {
		return err
	}
	for _, it := range items {
		if it.Done {
			continue
		}
		if err := h.store.SetChecklistItemDone(ctx, taskID, it.ID, true, domain.ActorAgent); err != nil {
			return err
		}
	}
	return nil
}

func (h *Harness) applyVerifiedCompletions(ctx context.Context, taskID, cycleID string, verdicts []criterionVerdict) error {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.applyVerifiedCompletions",
		"task_id", taskID, "cycle_id", cycleID, "verdict_count", len(verdicts))
	for _, v := range verdicts {
		if !v.passed {
			continue
		}
		err := h.store.SetChecklistItemDoneWithEvidence(ctx, taskID, v.id, v.evidence, v.verifier, v.reasoning, cycleID, domain.ActorAgent)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
	}
	return nil
}

// runVerificationPipeline opens a verify phase, runs LLM-driven checks
// within it, then closes the phase. The
// caller must have already terminated the execute phase — verification
// is its own phase row, not a step inside execute. See process.go for
// the loop that depends on this contract (verify → execute is the only
// legal retry transition allowed by domain.ValidPhaseTransition).
func (h *Harness) runVerificationPipeline(
	parentCtx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	state *processState,
	snap verificationSnapshot,
	feedback string,
) ([]criterionVerdict, string, error) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.runVerificationPipeline",
		"task_id", task.ID, "cycle_id", cycle.ID, "enabled", snap.enabled)
	if !snap.enabled {
		return nil, "", nil
	}
	if err := ensureReportCycleDir(h.opts.ReportDir, cycle.ID); err != nil {
		// Best-effort: the worker can still proceed if the dir
		// already exists. Hard errors (e.g. ENOSPC, EACCES on the
		// worker tempdir) surface later when parseVerifyReport
		// can't find the file the verifier was told to write.
		slog.Warn("agent harness ensureReportCycleDir failed",
			"cmd", harnessLogCmd, "operation", "agent.harness.Harness.runVerificationPipeline.ensure_err",
			"cycle_id", cycle.ID, "report_dir", h.opts.ReportDir, "err", err)
	}

	verifyStarted := h.opts.Clock()
	defer func() {
		h.observeVerifyDuration(h.opts.Clock().Sub(verifyStarted))
	}()

	phase, err := h.store.StartPhase(parentCtx, cycle.ID, domain.PhaseVerify, domain.ActorAgent)
	if err != nil {
		slog.Warn("agent harness StartPhase(verify) failed",
			"cmd", harnessLogCmd, "operation", "agent.harness.Harness.runVerificationPipeline.start_err",
			"cycle_id", cycle.ID, "err", err)
		return nil, "", fmt.Errorf("start verify phase: %w", err)
	}
	state.runningPhase = domain.PhaseVerify
	state.runningPhaseSeq = phase.PhaseSeq
	h.publish(cycle.TaskID, cycle.ID)

	// Pre-snapshot the working dir so we can detect any modifications
	// the verifier makes to source. The snapshot helper is fail-safe:
	// snapshot errors return an error here and we treat the cycle as
	// tampered (a critical safety property cannot be defeated by the
	// check throwing). Non-git working dirs degrade to a no-op.
	pre, preErr := captureIntegritySnapshot(parentCtx, h.opts.WorkingDir)
	if preErr != nil {
		slog.Warn("agent harness pre-verify integrity snapshot failed",
			"cmd", harnessLogCmd, "operation", "agent.harness.Harness.runVerificationPipeline.pre_snapshot_err",
			"cycle_id", cycle.ID, "err", preErr)
	}

	// attemptSeq is the per-cycle retry counter (1-indexed) used as
	// the idempotency key for verdict upserts. state.verifyAttempt
	// starts at 0 for the first attempt; incrementing here keeps the
	// DB rows aligned with how the SPA renders attempts ("Attempt 1"
	// on first try, "Attempt 2" after the first retry, …).
	attemptSeq := int64(state.verifyAttempt) + 1
	verdicts, feedbackOut, verifyErr := h.runVerifyChecks(parentCtx, task, cycle, phase.PhaseSeq, attemptSeq, snap, state.previouslyPassed, feedback)

	tampered, tamperReason := h.checkVerifyIntegrity(parentCtx, cycle.ID, pre, preErr)

	phaseStatus := domain.PhaseStatusSucceeded
	summary := "verify complete"
	if tampered {
		phaseStatus = domain.PhaseStatusFailed
		summary = tamperReason
		// Replace any in-flight verifyErr with a tampered error so the
		// caller routes this as terminal. A misbehaving verifier
		// invalidates the verdicts regardless of what it claimed.
		verifyErr = &verifyTamperedError{reason: tamperReason}
	} else if verifyErr != nil {
		phaseStatus = domain.PhaseStatusFailed
		summary = verifyErr.Error()
	}
	if _, err := h.store.CompletePhase(parentCtx, store.CompletePhaseInput{
		CycleID:  cycle.ID,
		PhaseSeq: phase.PhaseSeq,
		Status:   phaseStatus,
		Summary:  &summary,
		By:       domain.ActorAgent,
	}); err != nil {
		slog.Warn("agent harness CompletePhase(verify) failed",
			"cmd", harnessLogCmd, "operation", "agent.harness.Harness.runVerificationPipeline.complete_err",
			"cycle_id", cycle.ID, "phase_seq", phase.PhaseSeq, "err", err)
	}
	state.runningPhase = ""
	state.runningPhaseSeq = 0
	h.publish(cycle.TaskID, cycle.ID)
	return verdicts, feedbackOut, verifyErr
}

// checkVerifyIntegrity performs the post-verify integrity check. Returns
// (tampered, reason). tampered=true means the cycle should be
// terminated with verifyTamperedReason. Fail-safe under uncertainty:
// if the pre-snapshot itself errored, OR the post-snapshot errors
// here, OR HEAD moved, OR any path outside the allowed verify-report
// changed, the verifier failed integrity and the cycle dies terminal.
func (h *Harness) checkVerifyIntegrity(ctx context.Context, cycleID string, pre integritySnapshot, preErr error) (bool, string) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.checkVerifyIntegrity",
		"cycle_id", cycleID)
	if pre.notGitRepo {
		return false, ""
	}
	if preErr != nil {
		return true, "pre-verify integrity snapshot failed: " + preErr.Error()
	}
	post, err := captureIntegritySnapshot(ctx, h.opts.WorkingDir)
	if err != nil {
		slog.Warn("agent harness post-verify integrity snapshot failed",
			"cmd", harnessLogCmd, "operation", "agent.harness.Harness.checkVerifyIntegrity.post_snapshot_err",
			"cycle_id", cycleID, "err", err)
		return true, "post-verify integrity snapshot failed: " + err.Error()
	}
	if post.notGitRepo {
		// Pre saw a git repo, post sees no git repo: verifier nuked .git.
		return true, ".git directory disappeared during verify pass"
	}
	diff := diffIntegritySnapshots(pre, post)
	tampered, summary := classifyIntegrityDiff(diff, cycleID)
	if tampered {
		slog.Warn("verify pass tampered with working dir",
			"cmd", harnessLogCmd, "operation", "agent.harness.Harness.checkVerifyIntegrity.tampered",
			"cycle_id", cycleID, "summary", summary)
	}
	return tampered, summary
}

// runVerifyChecks performs LLM verification work. It does NOT manage
// the verify phase row — the caller wraps it with StartPhase /
// CompletePhase. phaseSeq is the verify phase row's seq, threaded
// through so progress events from the verify runner land on the verify
// phase (not execute) for the SPA activity panel's per-phase filter.
func (h *Harness) runVerifyChecks(
	parentCtx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	phaseSeq int64,
	attemptSeq int64,
	snap verificationSnapshot,
	previouslyPassed map[string]criterionVerdict,
	feedback string,
) ([]criterionVerdict, string, error) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.runVerifyChecks",
		"task_id", task.ID, "cycle_id", cycle.ID,
		"criteria_count", len(snap.criteria), "previously_passed", len(previouslyPassed))
	// expected is the set of criterion IDs the execute agent MUST
	// include in criteria-report.json. Items already proven passed in
	// earlier attempts are excluded so a retry prompt that legitimately
	// omits them does not parse-fail; per docs/data-model.md the
	// atomic-decision contract is preserved by carrying the verdicts
	// in memory until terminal-success.
	expected := make(map[string]struct{}, len(snap.criteria))
	for _, it := range snap.criteria {
		if _, locked := previouslyPassed[it.ID]; locked {
			continue
		}
		expected[it.ID] = struct{}{}
	}

	selfReport, err := h.loadCriteriaSelfReport(parentCtx, cycle.ID, attemptSeq, expected)
	if err != nil {
		return nil, "", err
	}

	// Mirror the criteria-report file into the DB at the same boundary
	// the worker parses it. Rows are keyed by (cycle, attempt,
	// criterion) so a parse-then-store retry (e.g. transient SQL
	// failure on a flaky network) is idempotent. Upsert errors are
	// logged and dropped — the verify pass continues against the
	// in-memory report — because durable mirroring is observability,
	// not gating logic. If we ever need it to be gating, swap the log
	// for a return.
	if uerr := h.persistCriteriaReports(parentCtx, cycle.ID, attemptSeq, snap.criteria, previouslyPassed, selfReport); uerr != nil {
		slog.Warn("agent harness UpsertCriteriaReports failed",
			"cmd", harnessLogCmd, "operation", "agent.harness.Harness.runVerifyChecks.upsert_criteria_err",
			"cycle_id", cycle.ID, "attempt_seq", attemptSeq, "err", uerr)
	}

	verdicts := make([]criterionVerdict, 0, len(snap.criteria))
	needLLMVerify := false

	for _, it := range snap.criteria {
		// Short-circuit locked passes: the verifier has already
		// approved this criterion in an earlier attempt. Re-running
		// verify on a settled item is wasted budget and risks a
		// flaky failure on what we've already decided.
		if locked, ok := previouslyPassed[it.ID]; ok {
			verdicts = append(verdicts, locked)
			continue
		}
		entry := selfReport[it.ID]
		v := criterionVerdict{
			id:       it.ID,
			evidence: entry.Evidence,
		}
		if !entry.ClaimedDone {
			v.passed = false
			v.verifier = domain.VerifierAgentSelf
			v.reasoning = "execute agent did not claim criterion done"
			verdicts = append(verdicts, v)
			h.recordVerifyVerdict(domain.VerifierAgentSelf, false)
			continue
		}
		needLLMVerify = true
		verdicts = append(verdicts, v)
	}

	if needLLMVerify {
		if err := h.runLLMVerifyAgent(parentCtx, task, cycle, phaseSeq, snap, previouslyPassed, selfReport, feedback); err != nil {
			return nil, "", err
		}
		vrep, err := parseVerifyReport(h.opts.ReportDir, cycle.ID, expected)
		if err != nil {
			return nil, "", err
		}
		next := make([]criterionVerdict, 0, len(verdicts))
		for _, v := range verdicts {
			// Locked passes carry their original verifier kind +
			// reasoning forward unchanged. They were not in the
			// expected-IDs set passed to the verifier, so vrep has no
			// entry for them and re-evaluating would either fail or
			// flake.
			if _, locked := previouslyPassed[v.id]; locked {
				next = append(next, v)
				continue
			}
			if v.verifier == domain.VerifierAgentSelf {
				next = append(next, v)
				continue
			}
			entry := selfReport[v.id]
			vr := vrep[v.id]
			nv := criterionVerdict{id: v.id, evidence: entry.Evidence}
			if vr.Verified {
				nv.passed = true
				nv.verifier = domain.VerifierVerifyAgent
				nv.reasoning = vr.Reasoning
			} else {
				nv.passed = false
				nv.verifier = domain.VerifierVerifyAgent
				nv.reasoning = vr.Reasoning
			}
			next = append(next, nv)
			h.recordVerifyVerdict(domain.VerifierVerifyAgent, nv.passed)
		}
		verdicts = next
	}

	// Mirror this attempt's final verdicts (agent_self for "did not claim
	// done", verify_agent for LLM verdicts) into the DB at the verify-phase
	// boundary. Carrying-over locked passes from earlier attempts is
	// intentionally NOT replayed here — those rows already exist
	// under their original attempt_seq, and re-writing them would
	// violate the "rows reflect what each attempt actually decided"
	// invariant the SPA timeline depends on. Idempotent against
	// (cycle, attempt, criterion) so a partial-failure rewrite is
	// safe; observability-only, errors are logged and dropped (same
	// rationale as persistCriteriaReports).
	if uerr := h.persistVerifyReports(parentCtx, cycle.ID, attemptSeq, verdicts, previouslyPassed); uerr != nil {
		slog.Warn("agent harness UpsertVerifyReports failed",
			"cmd", harnessLogCmd, "operation", "agent.harness.Harness.runVerifyChecks.upsert_verify_err",
			"cycle_id", cycle.ID, "attempt_seq", attemptSeq, "err", uerr)
	}

	var failures []string
	for _, v := range verdicts {
		if !v.passed {
			failures = append(failures, fmt.Sprintf("%s: %s", v.id, v.reasoning))
		}
	}
	if len(failures) > 0 {
		return verdicts, strings.Join(failures, "; "), fmt.Errorf("verification failed")
	}
	return verdicts, "", nil
}

// runLLMVerifyAgent invokes the verify runner against the criteria
// self-report. It performs no phase bookkeeping; the caller (the verify
// phase wrapper in runVerificationPipeline) owns StartPhase / CompletePhase.
func (h *Harness) runLLMVerifyAgent(
	ctx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	phaseSeq int64,
	snap verificationSnapshot,
	previouslyPassed map[string]criterionVerdict,
	selfReport map[string]criteriaReportEntry,
	feedback string,
) error {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.runLLMVerifyAgent",
		"task_id", task.ID, "cycle_id", cycle.ID, "locked_passes", len(previouslyPassed))
	commitOn := h.agentCommitExecuteWork(ctx)
	diff := verifyDiffSection(h.opts.WorkingDir, cycle.ID, commitOn)
	var b strings.Builder
	b.WriteString("You are the verification agent. Do not modify source files.\n")
	// Render the absolute, worker-managed verify-report path so the
	// agent CLI writes outside the operator's RepoRoot. Any source
	// mutation in RepoRoot during the verify pass is now treated as
	// tampering with no allowlist (see verify_integrity.go).
	b.WriteString(fmt.Sprintf("Write `%s` only.\n\n", verifyReportPath(h.opts.ReportDir, cycle.ID)))
	b.WriteString("Schema: {\"criteria\":[{\"id\":\"...\",\"verified\":true|false,\"reasoning\":\"...\"}]}\n\n")
	if len(previouslyPassed) > 0 {
		b.WriteString("## Locked passes (do not re-evaluate)\n\n")
		b.WriteString("These criteria were verified in earlier attempts. Do NOT include them in your report.\n\n")
		for id := range previouslyPassed {
			b.WriteString(fmt.Sprintf("- [%s]\n", id))
		}
		b.WriteString("\n")
	}
	for _, it := range snap.criteria {
		if _, locked := previouslyPassed[it.ID]; locked {
			continue
		}
		e, ok := selfReport[it.ID]
		if !ok || !e.ClaimedDone {
			continue
		}
		b.WriteString(fmt.Sprintf("- [%s] %s\n  execute claimed_done: true (assertion only)\n  execute evidence: %s\n", it.ID, it.Text, e.Evidence))
	}
	b.WriteString("\nDiff:\n")
	b.WriteString(diff)
	prompt := b.String()
	if feedback != "" {
		prompt = appendVerifyFeedback(prompt, feedback)
	}
	_, err := snap.verifyRunner.Run(ctx, runner.Request{
		TaskID:      task.ID,
		AttemptSeq:  cycle.AttemptSeq,
		Phase:       domain.PhaseVerify,
		Prompt:      prompt,
		WorkingDir:  h.opts.WorkingDir,
		CursorModel: snap.verifyModel,
		// Stream verify-phase progress events under the verify phase
		// row's seq (not execute's). The SPA Activity panel filters
		// stream events by phase_seq, so without this verify shows up
		// as an empty P3 entry. See process.go::invokeRunner for the
		// matching execute-phase wiring.
		OnProgress: func(ev runner.ProgressEvent) {
			h.persistProgress(ctx, task.ID, cycle.ID, phaseSeq, ev)
			h.publishProgress(task.ID, cycle.ID, phaseSeq, ev)
		},
	})
	return err
}

func (h *Harness) loadCriteriaSelfReport(ctx context.Context, cycleID string, attemptSeq int64, expected map[string]struct{}) (map[string]criteriaReportEntry, error) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.loadCriteriaSelfReport",
		"cycle_id", cycleID, "attempt_seq", attemptSeq, "expected", len(expected))
	selfReport, err := parseCriteriaReport(h.opts.ReportDir, cycleID, expected)
	if err == nil {
		return selfReport, nil
	}
	if !errors.Is(err, ErrCriteriaReportMissing) {
		return nil, err
	}
	rows, err := h.store.ListCriteriaReportsForCycle(ctx, cycleID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]criteriaReportEntry, len(expected))
	for _, row := range rows {
		if row.AttemptSeq != attemptSeq {
			continue
		}
		if _, want := expected[row.CriterionID]; !want {
			continue
		}
		out[row.CriterionID] = criteriaReportEntry{
			ID:          row.CriterionID,
			ClaimedDone: row.ClaimedDone,
			Evidence:    row.Evidence,
		}
	}
	for id := range expected {
		if _, ok := out[id]; !ok {
			return nil, fmt.Errorf("%w: criterion %q missing from DB fallback", ErrCriteriaReportMissing, id)
		}
	}
	return out, nil
}

func gitDiff(dir, rev string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "diff", rev)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	if len(out) > 200*1024 {
		return string(out[:200*1024]) + "\n…(truncated)", nil
	}
	return string(out), nil
}

// formatVerificationFailedReason builds the terminate-reason string for
// a cycle that exhausted retries with at least one criterion still
// failing. Format: "verification_failed:<id1>,<id2>,...". The
// "verification_failed" prefix is contract-stable — clients consuming
// terminate_reason MUST use prefix matching (`startsWith`) per
// docs/api.md and docs/data-model.md. Bare "verification_failed" is
// no longer emitted by the worker but remains a valid value for older
// cycle rows.
//
// IDs are sorted for deterministic output (test pinning + grep-friendly
// audit trail) and de-duplicated across attempts. The terminate_reason
// column is varchar(256); if the comma-separated list exceeds that, we
// truncate the suffix with a trailing "…" to keep the prefix intact.
//
// finalVerdicts are this attempt's verdicts; lockedPasses is the union
// of criteria proved passed in earlier attempts. A criterion in
// lockedPasses is by construction NOT failing.
func formatVerificationFailedReason(finalVerdicts []criterionVerdict, lockedPasses map[string]criterionVerdict) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.formatVerificationFailedReason",
		"verdict_count", len(finalVerdicts), "locked_count", len(lockedPasses))
	failing := make([]string, 0, len(finalVerdicts))
	seen := map[string]struct{}{}
	for _, v := range finalVerdicts {
		if v.passed {
			continue
		}
		if _, locked := lockedPasses[v.id]; locked {
			continue
		}
		if _, dup := seen[v.id]; dup {
			continue
		}
		seen[v.id] = struct{}{}
		failing = append(failing, v.id)
	}
	if len(failing) == 0 {
		return verificationFailedReason
	}
	sort.Strings(failing)
	const maxLen = 256
	const prefix = verificationFailedReason + ":"
	body := strings.Join(failing, ",")
	full := prefix + body
	if len(full) <= maxLen {
		return full
	}
	const ellipsis = "…"
	budget := maxLen - len(prefix) - len(ellipsis)
	if budget < 0 {
		budget = 0
	}
	if budget > len(body) {
		budget = len(body)
	}
	trimmed := body[:budget]
	if i := strings.LastIndex(trimmed, ","); i > 0 {
		trimmed = trimmed[:i]
	}
	return prefix + trimmed + ellipsis
}

// persistCriteriaReports mirrors the parsed criteria-report.json
// payload for one (cycle, attempt) into task_cycle_criteria_reports.
// Filters out criteria that were already locked-passed in earlier
// attempts because the agent does not re-include them in the file
// (they are not in the expected-IDs set passed to parseCriteriaReport
// at line 285), so a row would be missing fields and would only
// confuse the SPA timeline.
func (h *Harness) persistCriteriaReports(
	ctx context.Context,
	cycleID string,
	attemptSeq int64,
	criteria []store.ChecklistVerifyItem,
	previouslyPassed map[string]criterionVerdict,
	selfReport map[string]criteriaReportEntry,
) error {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.persistCriteriaReports",
		"cycle_id", cycleID, "attempt_seq", attemptSeq)
	entries := make([]store.CriteriaReportEntry, 0, len(criteria))
	for _, it := range criteria {
		if _, locked := previouslyPassed[it.ID]; locked {
			continue
		}
		e, ok := selfReport[it.ID]
		if !ok {
			continue
		}
		entries = append(entries, store.CriteriaReportEntry{
			CriterionID: it.ID,
			ClaimedDone: e.ClaimedDone,
			Evidence:    e.Evidence,
		})
	}
	return h.store.UpsertCriteriaReports(ctx, cycleID, attemptSeq, entries)
}

// persistVerifyReports mirrors this attempt's final verdicts into
// task_cycle_verify_reports. Stores every verdict produced in the
// verify phase (agent_self for "did not claim done", verify_agent for
// LLM verdicts) so the SPA can render a uniform per-criterion timeline regardless of how the decision was
// reached. Rows for criteria that were locked-passed in earlier
// attempts are skipped — those already exist at their original
// attempt_seq and re-writing them under the current attempt_seq
// would lie about which attempt evaluated them.
func (h *Harness) persistVerifyReports(
	ctx context.Context,
	cycleID string,
	attemptSeq int64,
	verdicts []criterionVerdict,
	previouslyPassed map[string]criterionVerdict,
) error {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.persistVerifyReports",
		"cycle_id", cycleID, "attempt_seq", attemptSeq, "verdict_count", len(verdicts))
	entries := make([]store.VerifyReportEntry, 0, len(verdicts))
	for _, v := range verdicts {
		if _, locked := previouslyPassed[v.id]; locked {
			continue
		}
		entries = append(entries, store.VerifyReportEntry{
			CriterionID:  v.id,
			Verified:     v.passed,
			VerifierKind: v.verifier,
			Reasoning:    v.reasoning,
		})
	}
	return h.store.UpsertVerifyReports(ctx, cycleID, attemptSeq, entries)
}

func encodeCriteriaSnapshot(items []store.ChecklistVerifyItem) []byte {
	type row struct {
		ID           string `json:"id"`
		Text         string `json:"text"`
		SourceTaskID string `json:"source_task_id"`
	}
	rows := make([]row, len(items))
	for i, it := range items {
		rows[i] = row{ID: it.ID, Text: it.Text, SourceTaskID: it.SourceTaskID}
	}
	b, _ := json.Marshal(rows)
	return b
}
