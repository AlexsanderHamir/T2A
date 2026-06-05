package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
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
	_ = ensureT2ADir(w.options.WorkingDir)

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

	verdicts, feedbackOut, verifyErr := w.runVerifyChecks(parentCtx, task, cycle, phase.PhaseSeq, snap, feedback)

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
	snap verificationSnapshot,
	feedback string,
) ([]criterionVerdict, string, error) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.runVerifyChecks",
		"task_id", task.ID, "cycle_id", cycle.ID, "criteria_count", len(snap.criteria))
	expected := make(map[string]struct{}, len(snap.criteria))
	for _, it := range snap.criteria {
		expected[it.ID] = struct{}{}
	}

	selfReport, err := parseCriteriaReport(w.options.WorkingDir, cycle.ID, expected)
	if err != nil {
		return nil, "", err
	}

	verdicts := make([]criterionVerdict, 0, len(snap.criteria))
	needLLMVerify := false

	for _, it := range snap.criteria {
		entry := selfReport[it.ID]
		v := criterionVerdict{
			id:       it.ID,
			evidence: entry.Evidence,
		}
		if !entry.ClaimedDone {
			v.passed = false
			v.reasoning = "execute agent did not claim criterion done"
			verdicts = append(verdicts, v)
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
			continue
		}
		needLLMVerify = true
		verdicts = append(verdicts, v)
	}

	if needLLMVerify {
		if err := w.runLLMVerifyAgent(parentCtx, task, cycle, phaseSeq, snap, selfReport, feedback); err != nil {
			return nil, "", err
		}
		vrep, err := parseVerifyReport(w.options.WorkingDir, cycle.ID, expected)
		if err != nil {
			return nil, "", err
		}
		next := make([]criterionVerdict, 0, len(verdicts))
		for _, v := range verdicts {
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
		}
		verdicts = next
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
	selfReport map[string]criteriaReportEntry,
	feedback string,
) error {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.runLLMVerifyAgent",
		"task_id", task.ID, "cycle_id", cycle.ID)
	diff, err := gitDiff(w.options.WorkingDir, "HEAD")
	if err != nil {
		diff = "(diff unavailable: " + err.Error() + ")"
	}
	var b strings.Builder
	b.WriteString("You are the verification agent. Do not modify source files.\n")
	b.WriteString(fmt.Sprintf("Write `.t2a/%s/verify-report.json` only.\n\n", cycle.ID))
	b.WriteString("Schema: {\"criteria\":[{\"id\":\"...\",\"verified\":true|false,\"reasoning\":\"...\"}]}\n\n")
	for _, it := range snap.criteria {
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
