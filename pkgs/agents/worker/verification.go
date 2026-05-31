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
	snap := verificationSnapshot{
		enabled:      settings.VerifyEnabled && len(items) > 0,
		maxRetries:   maxRetries,
		checkTimeout: time.Duration(settings.CheckCommandTimeoutSeconds) * time.Second,
		criteria:     items,
		verifyRunner: w.runner,
		verifyModel:  strings.TrimSpace(settings.VerifyRunnerModel),
	}
	if name := strings.TrimSpace(settings.VerifyRunnerName); name != "" && name != w.runner.Name() {
		slog.Warn("verify_runner_name ignored; using execute runner", "cmd", workerLogCmd,
			"wanted", name, "have", w.runner.Name())
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

	verdicts, feedbackOut, verifyErr := w.runVerifyChecks(parentCtx, task, cycle, snap, feedback)

	phaseStatus := domain.PhaseStatusSucceeded
	summary := "verify complete"
	if verifyErr != nil {
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

// runVerifyChecks performs the deterministic and (when needed) LLM
// verification work. It does NOT manage the verify phase row — the
// caller wraps it with StartPhase / CompletePhase.
func (w *Worker) runVerifyChecks(
	parentCtx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
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
		if err := w.runLLMVerifyAgent(parentCtx, task, cycle, snap, selfReport, feedback); err != nil {
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
