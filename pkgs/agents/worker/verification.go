package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strings"
	"time"

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
	checkTimeout time.Duration
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

func (w *Worker) loadVerificationSnapshot(ctx context.Context, taskID string) (verificationSnapshot, error) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.loadVerificationSnapshot",
		"task_id", taskID)
	settings, err := w.store.GetSettings(ctx)
	if err != nil {
		return verificationSnapshot{}, err
	}
	items, err := w.store.ListChecklistForVerify(ctx, taskID)
	if err != nil {
		return verificationSnapshot{}, err
	}
	maxRetries := settings.VerifyMaxRetries
	if maxRetries > domain.MaxVerifyMaxRetries {
		maxRetries = domain.MaxVerifyMaxRetries
	}
	// The supervisor (cmd/taskapi/run_agentworker.go::applySettings) is
	// the source of truth for which runner verify uses: if the operator
	// configured app_settings.VerifyRunnerName, the supervisor probed
	// and built it and passed it as Options.VerifyRunner. A nil
	// VerifyRunner means either (a) the operator did not configure one
	// (V1 default) or (b) the supervisor's build/probe failed and
	// demoted verify back to the execute runner with a warn — either
	// way, fall back to w.runner here.
	verifyRunner := w.runner
	if w.options.VerifyRunner != nil {
		verifyRunner = w.options.VerifyRunner
	}
	snap := verificationSnapshot{
		enabled:      settings.VerifyEnabled && len(items) > 0,
		maxRetries:   maxRetries,
		checkTimeout: time.Duration(settings.CheckCommandTimeoutSeconds) * time.Second,
		criteria:     items,
		verifyRunner: verifyRunner,
		verifyModel:  strings.TrimSpace(settings.VerifyRunnerModel),
	}
	return snap, nil
}

func (w *Worker) completeChecklistLegacy(ctx context.Context, taskID string) error {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.completeChecklistLegacy", "task_id", taskID)
	items, err := w.store.ListChecklistForSubject(ctx, taskID)
	if err != nil {
		return err
	}
	for _, it := range items {
		if it.Done {
			continue
		}
		if err := w.store.SetChecklistItemDone(ctx, taskID, it.ID, true, domain.ActorAgent); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) applyVerifiedCompletions(ctx context.Context, taskID, cycleID string, verdicts []criterionVerdict) error {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.applyVerifiedCompletions",
		"task_id", taskID, "cycle_id", cycleID, "verdict_count", len(verdicts))
	for _, v := range verdicts {
		if !v.passed {
			continue
		}
		err := w.store.SetChecklistItemDoneWithEvidence(ctx, taskID, v.id, v.evidence, v.verifier, v.reasoning, cycleID, domain.ActorAgent)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
	}
	return nil
}

// runVerificationPipeline opens a verify phase, runs deterministic and
// optional LLM-driven checks within it, then closes the phase. The
// caller must have already terminated the execute phase — verification
// is its own phase row, not a step inside execute. See process.go for
// the loop that depends on this contract (verify → execute is the only
// legal retry transition allowed by domain.ValidPhaseTransition).
func (w *Worker) runVerificationPipeline(
	parentCtx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	state *processState,
	snap verificationSnapshot,
	feedback string,
) ([]criterionVerdict, string, error) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.runVerificationPipeline",
		"task_id", task.ID, "cycle_id", cycle.ID, "enabled", snap.enabled)
	if !snap.enabled {
		return nil, "", nil
	}
	if err := ensureReportCycleDir(w.options.ReportDir, cycle.ID); err != nil {
		// Best-effort: the worker can still proceed if the dir
		// already exists. Hard errors (e.g. ENOSPC, EACCES on the
		// worker tempdir) surface later when parseVerifyReport
		// can't find the file the verifier was told to write.
		slog.Warn("agent worker ensureReportCycleDir failed",
			"cmd", workerLogCmd, "operation", "agent.worker.Worker.runVerificationPipeline.ensure_err",
			"cycle_id", cycle.ID, "report_dir", w.options.ReportDir, "err", err)
	}

	verifyStarted := w.options.Clock()
	defer func() {
		w.observeVerifyDuration(w.options.Clock().Sub(verifyStarted))
	}()

	phase, err := w.store.StartPhase(parentCtx, cycle.ID, domain.PhaseVerify, domain.ActorAgent)
	if err != nil {
		slog.Warn("agent worker StartPhase(verify) failed",
			"cmd", workerLogCmd, "operation", "agent.worker.Worker.runVerificationPipeline.start_err",
			"cycle_id", cycle.ID, "err", err)
		return nil, "", fmt.Errorf("start verify phase: %w", err)
	}
	state.runningPhase = domain.PhaseVerify
	state.runningPhaseSeq = phase.PhaseSeq
	w.publish(cycle.TaskID, cycle.ID)

	// Pre-snapshot the working dir so we can detect any modifications
	// the verifier makes to source. The snapshot helper is fail-safe:
	// snapshot errors return an error here and we treat the cycle as
	// tampered (a critical safety property cannot be defeated by the
	// check throwing). Non-git working dirs degrade to a no-op.
	pre, preErr := captureIntegritySnapshot(parentCtx, w.options.WorkingDir)
	if preErr != nil {
		slog.Warn("agent worker pre-verify integrity snapshot failed",
			"cmd", workerLogCmd, "operation", "agent.worker.Worker.runVerificationPipeline.pre_snapshot_err",
			"cycle_id", cycle.ID, "err", preErr)
	}

	// attemptSeq is the per-cycle retry counter (1-indexed) used as
	// the idempotency key for verdict upserts. state.verifyAttempt
	// starts at 0 for the first attempt; incrementing here keeps the
	// DB rows aligned with how the SPA renders attempts ("Attempt 1"
	// on first try, "Attempt 2" after the first retry, …).
	attemptSeq := int64(state.verifyAttempt) + 1
	verdicts, feedbackOut, verifyErr := w.runVerifyChecks(parentCtx, task, cycle, phase.PhaseSeq, attemptSeq, snap, state.previouslyPassed, feedback)

	tampered, tamperReason := w.checkVerifyIntegrity(parentCtx, cycle.ID, pre, preErr)

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
	if _, err := w.store.CompletePhase(parentCtx, store.CompletePhaseInput{
		CycleID:  cycle.ID,
		PhaseSeq: phase.PhaseSeq,
		Status:   phaseStatus,
		Summary:  &summary,
		By:       domain.ActorAgent,
	}); err != nil {
		slog.Warn("agent worker CompletePhase(verify) failed",
			"cmd", workerLogCmd, "operation", "agent.worker.Worker.runVerificationPipeline.complete_err",
			"cycle_id", cycle.ID, "phase_seq", phase.PhaseSeq, "err", err)
	}
	state.runningPhase = ""
	state.runningPhaseSeq = 0
	w.publish(cycle.TaskID, cycle.ID)
	return verdicts, feedbackOut, verifyErr
}

// checkVerifyIntegrity performs the post-verify integrity check. Returns
// (tampered, reason). tampered=true means the cycle should be
// terminated with verifyTamperedReason. Fail-safe under uncertainty:
// if the pre-snapshot itself errored, OR the post-snapshot errors
// here, OR HEAD moved, OR any path outside the allowed verify-report
// changed, the verifier failed integrity and the cycle dies terminal.
func (w *Worker) checkVerifyIntegrity(ctx context.Context, cycleID string, pre integritySnapshot, preErr error) (bool, string) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.checkVerifyIntegrity",
		"cycle_id", cycleID)
	if pre.notGitRepo {
		return false, ""
	}
	if preErr != nil {
		return true, "pre-verify integrity snapshot failed: " + preErr.Error()
	}
	post, err := captureIntegritySnapshot(ctx, w.options.WorkingDir)
	if err != nil {
		slog.Warn("agent worker post-verify integrity snapshot failed",
			"cmd", workerLogCmd, "operation", "agent.worker.Worker.checkVerifyIntegrity.post_snapshot_err",
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
			"cmd", workerLogCmd, "operation", "agent.worker.Worker.checkVerifyIntegrity.tampered",
			"cycle_id", cycleID, "summary", summary)
	}
	return tampered, summary
}

// runVerifyChecks performs the deterministic and (when needed) LLM
// verification work. It does NOT manage the verify phase row — the
// caller wraps it with StartPhase / CompletePhase. phaseSeq is the
// verify phase row's seq, threaded through so progress events from the
// verify runner land on the verify phase (not execute) for the SPA
// activity panel's per-phase filter.
func (w *Worker) runVerifyChecks(
	parentCtx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	phaseSeq int64,
	attemptSeq int64,
	snap verificationSnapshot,
	previouslyPassed map[string]criterionVerdict,
	feedback string,
) ([]criterionVerdict, string, error) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.runVerifyChecks",
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

	selfReport, err := parseCriteriaReport(w.options.ReportDir, cycle.ID, expected)
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
	if uerr := w.persistCriteriaReports(parentCtx, cycle.ID, attemptSeq, snap.criteria, previouslyPassed, selfReport); uerr != nil {
		slog.Warn("agent worker UpsertCriteriaReports failed",
			"cmd", workerLogCmd, "operation", "agent.worker.Worker.runVerifyChecks.upsert_criteria_err",
			"cycle_id", cycle.ID, "attempt_seq", attemptSeq, "err", uerr)
	}

	verdicts := make([]criterionVerdict, 0, len(snap.criteria))
	needLLMVerify := false

	for _, it := range snap.criteria {
		// Short-circuit locked passes: the verifier has already
		// approved this criterion in an earlier attempt. Re-running
		// deterministic checks or the LLM verify on a settled item
		// is wasted budget and risks a flaky check failing what
		// we've already decided.
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
			w.recordVerifyVerdict(domain.VerifierAgentSelf, false)
			continue
		}
		if strings.TrimSpace(it.Check) != "" {
			out := runDeterministicCheck(parentCtx, w.options.WorkingDir, it.Check, snap.checkTimeout)
			if out.passed {
				v.passed = true
				v.verifier = domain.VerifierDeterministicCheck
				v.reasoning = "deterministic check passed"
			} else {
				v.passed = false
				v.verifier = domain.VerifierDeterministicCheck
				v.reasoning = fmt.Sprintf("deterministic check failed: %s %s", out.stdout, out.stderr)
			}
			verdicts = append(verdicts, v)
			w.recordVerifyVerdict(domain.VerifierDeterministicCheck, v.passed)
			continue
		}
		needLLMVerify = true
		verdicts = append(verdicts, v)
	}

	if needLLMVerify {
		if err := w.runLLMVerifyAgent(parentCtx, task, cycle, phaseSeq, snap, previouslyPassed, selfReport, feedback); err != nil {
			return nil, "", err
		}
		vrep, err := parseVerifyReport(w.options.ReportDir, cycle.ID, expected)
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
			it := findVerifyItem(snap.criteria, v.id)
			if it != nil && strings.TrimSpace(it.Check) != "" {
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
			w.recordVerifyVerdict(domain.VerifierVerifyAgent, nv.passed)
		}
		verdicts = next
	}

	// Mirror this attempt's final verdicts (deterministic_check +
	// agent_self + verify_agent) into the DB at the verify-phase
	// boundary. Carrying-over locked passes from earlier attempts is
	// intentionally NOT replayed here — those rows already exist
	// under their original attempt_seq, and re-writing them would
	// violate the "rows reflect what each attempt actually decided"
	// invariant the SPA timeline depends on. Idempotent against
	// (cycle, attempt, criterion) so a partial-failure rewrite is
	// safe; observability-only, errors are logged and dropped (same
	// rationale as persistCriteriaReports).
	if uerr := w.persistVerifyReports(parentCtx, cycle.ID, attemptSeq, verdicts, previouslyPassed); uerr != nil {
		slog.Warn("agent worker UpsertVerifyReports failed",
			"cmd", workerLogCmd, "operation", "agent.worker.Worker.runVerifyChecks.upsert_verify_err",
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

func findVerifyItem(items []store.ChecklistVerifyItem, id string) *store.ChecklistVerifyItem {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

// runLLMVerifyAgent invokes the verify runner against the criteria
// self-report. It performs no phase bookkeeping; the caller (the verify
// phase wrapper in runVerificationPipeline) owns StartPhase / CompletePhase.
func (w *Worker) runLLMVerifyAgent(
	ctx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	phaseSeq int64,
	snap verificationSnapshot,
	previouslyPassed map[string]criterionVerdict,
	selfReport map[string]criteriaReportEntry,
	feedback string,
) error {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.runLLMVerifyAgent",
		"task_id", task.ID, "cycle_id", cycle.ID, "locked_passes", len(previouslyPassed))
	diff, err := gitDiff(w.options.WorkingDir, "HEAD")
	if err != nil {
		diff = "(diff unavailable: " + err.Error() + ")"
	}
	var b strings.Builder
	b.WriteString("You are the verification agent. Do not modify source files.\n")
	// Render the absolute, worker-managed verify-report path so the
	// agent CLI writes outside the operator's RepoRoot. Any source
	// mutation in RepoRoot during the verify pass is now treated as
	// tampering with no allowlist (see verify_integrity.go).
	b.WriteString(fmt.Sprintf("Write `%s` only.\n\n", verifyReportPath(w.options.ReportDir, cycle.ID)))
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
		if strings.TrimSpace(it.Check) != "" {
			continue
		}
		e := selfReport[it.ID]
		b.WriteString(fmt.Sprintf("- [%s] %s\n  execute evidence: %s\n", it.ID, it.Text, e.Evidence))
	}
	b.WriteString("\nDiff:\n")
	b.WriteString(diff)
	prompt := b.String()
	if feedback != "" {
		prompt = appendVerifyFeedback(prompt, feedback)
	}
	_, err = snap.verifyRunner.Run(ctx, runner.Request{
		TaskID:      task.ID,
		AttemptSeq:  cycle.AttemptSeq,
		Phase:       domain.PhaseVerify,
		Prompt:      prompt,
		WorkingDir:  w.options.WorkingDir,
		CursorModel: snap.verifyModel,
		// Stream verify-phase progress events under the verify phase
		// row's seq (not execute's). The SPA Activity panel filters
		// stream events by phase_seq, so without this verify shows up
		// as an empty P3 entry. See process.go::invokeRunner for the
		// matching execute-phase wiring.
		OnProgress: func(ev runner.ProgressEvent) {
			w.persistProgress(ctx, task.ID, cycle.ID, phaseSeq, ev)
			w.publishProgress(task.ID, cycle.ID, phaseSeq, ev)
		},
	})
	return err
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
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.formatVerificationFailedReason",
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
func (w *Worker) persistCriteriaReports(
	ctx context.Context,
	cycleID string,
	attemptSeq int64,
	criteria []store.ChecklistVerifyItem,
	previouslyPassed map[string]criterionVerdict,
	selfReport map[string]criteriaReportEntry,
) error {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.persistCriteriaReports",
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
	return w.store.UpsertCriteriaReports(ctx, cycleID, attemptSeq, entries)
}

// persistVerifyReports mirrors this attempt's final verdicts into
// task_cycle_verify_reports. Stores every verdict produced in the
// verify phase (deterministic_check, agent_self for "did not claim
// done", verify_agent for LLM verdicts) so the SPA can render a
// uniform per-criterion timeline regardless of how the decision was
// reached. Rows for criteria that were locked-passed in earlier
// attempts are skipped — those already exist at their original
// attempt_seq and re-writing them under the current attempt_seq
// would lie about which attempt evaluated them.
func (w *Worker) persistVerifyReports(
	ctx context.Context,
	cycleID string,
	attemptSeq int64,
	verdicts []criterionVerdict,
	previouslyPassed map[string]criterionVerdict,
) error {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.persistVerifyReports",
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
	return w.store.UpsertVerifyReports(ctx, cycleID, attemptSeq, entries)
}

func encodeCriteriaSnapshot(items []store.ChecklistVerifyItem) []byte {
	type row struct {
		ID           string `json:"id"`
		Text         string `json:"text"`
		Check        string `json:"check"`
		SourceTaskID string `json:"source_task_id"`
	}
	rows := make([]row, len(items))
	for i, it := range items {
		rows[i] = row{ID: it.ID, Text: it.Text, Check: it.Check, SourceTaskID: it.SourceTaskID}
	}
	b, _ := json.Marshal(rows)
	return b
}
