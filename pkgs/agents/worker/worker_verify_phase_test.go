package worker_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// hookRunner wraps a runnerfake so tests can run a side effect (write
// criteria-report.json, mutate working dir) when Run lands on a given
// phase. Used by the verify-phase tests to script verify-report.json
// authorship without rebuilding the runner.Request plumbing.
type hookRunner struct {
	*runnerfake.Runner
	preRun func(req runner.Request)
}

func (h *hookRunner) Run(ctx context.Context, req runner.Request) (runner.Result, error) {
	if h.preRun != nil {
		h.preRun(req)
	}
	if req.OnProgress != nil {
		req.OnProgress(runner.ProgressEvent{Kind: "stream", Subtype: "tool_use", Message: "verify probe"})
	}
	return h.Runner.Run(ctx, req)
}

func writeCriteriaReport(t *testing.T, dir, cycleID string, ids []string) {
	t.Helper()
	cdir := filepath.Join(dir, ".t2a", cycleID)
	if err := os.MkdirAll(cdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	type entry struct {
		ID          string `json:"id"`
		ClaimedDone bool   `json:"claimed_done"`
		Evidence    string `json:"evidence"`
	}
	rep := struct {
		Criteria []entry `json:"criteria"`
	}{}
	for _, id := range ids {
		rep.Criteria = append(rep.Criteria, entry{ID: id, ClaimedDone: true, Evidence: "execute did the thing"})
	}
	b, _ := json.Marshal(rep)
	if err := os.WriteFile(filepath.Join(cdir, "criteria-report.json"), b, 0o644); err != nil {
		t.Fatalf("write criteria: %v", err)
	}
}

func writeVerifyReport(t *testing.T, dir, cycleID string, ids []string) {
	t.Helper()
	cdir := filepath.Join(dir, ".t2a", cycleID)
	if err := os.MkdirAll(cdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	type entry struct {
		ID        string `json:"id"`
		Verified  bool   `json:"verified"`
		Reasoning string `json:"reasoning"`
	}
	rep := struct {
		Criteria []entry `json:"criteria"`
	}{}
	for _, id := range ids {
		rep.Criteria = append(rep.Criteria, entry{
			ID:        id,
			Verified:  true,
			Reasoning: "verifier confirmed via diff inspection and file content review of the change set under test",
		})
	}
	b, _ := json.Marshal(rep)
	if err := os.WriteFile(filepath.Join(cdir, "verify-report.json"), b, 0o644); err != nil {
		t.Fatalf("write verify: %v", err)
	}
}

func gitInitTestRepo(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed; skipping git-backed integrity test")
	}
	for _, args := range [][]string{
		{"init"},
		{"-c", "user.email=t@e.local", "-c", "user.name=t", "commit", "--allow-empty", "-m", "init"},
	} {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// TestWorker_VerifyPhase_opensWhileExecuteIsTerminal pins the fix for
// the bug where the worker called StartPhase(verify) while the execute
// phase was still in `running`, tripping the cycle's "no running phase"
// invariant inside the transaction. The verify phase must always open
// AFTER execute is terminal so the cycle's phase ledger reflects the
// real sequence and the verify→execute retry transition is legal.
//
// Symptom this test guards against: every cycle with verification
// enabled would terminate with `execute_phase_start_failed` on the
// retry attempt because the state machine forbids execute→execute.
func TestWorker_VerifyPhase_opensWhileExecuteIsTerminal(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "verify-phase")

	// One retry only, so the loop runs at most twice. The runner never
	// writes criteria-report.json so verification fails on every attempt
	// — the point of the test is the phase ledger, not the verdict.
	maxRetries := 1
	if _, err := h.store.UpdateSettings(ctx, store.SettingsPatch{VerifyMaxRetries: &maxRetries}); err != nil {
		t.Fatalf("set verify max retries: %v", err)
	}

	if _, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion one", "", domain.ActorUser); err != nil {
		t.Fatalf("add checklist item: %v", err)
	}

	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "ran cleanly",
		json.RawMessage(`{"ok":true}`), "",
	))

	// Use a temp WorkingDir so the worker's .t2a/<cycle>/ paths land
	// somewhere isolated and parseCriteriaReport hits ErrCriteriaReportMissing
	// deterministically (no stray files from earlier test runs).
	_, done := h.startWorker(ctx, r, worker.Options{WorkingDir: t.TempDir()})
	final := h.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}

	if final.Status != domain.StatusFailed {
		t.Fatalf("task status = %q, want failed", final.Status)
	}

	bg := context.Background()
	cycle := assertCycleStatus(t, h.store, tsk.ID, 1, domain.CycleStatusFailed)

	phases, err := h.store.ListPhasesForCycle(bg, cycle.ID)
	if err != nil {
		t.Fatalf("list phases: %v", err)
	}

	// Expected ledger: diagnose(skipped) → execute(succeeded) →
	// verify(failed) → execute(succeeded) → verify(failed).
	wantSeq := []struct {
		phase  domain.Phase
		status domain.PhaseStatus
	}{
		{domain.PhaseDiagnose, domain.PhaseStatusSkipped},
		{domain.PhaseExecute, domain.PhaseStatusSucceeded},
		{domain.PhaseVerify, domain.PhaseStatusFailed},
		{domain.PhaseExecute, domain.PhaseStatusSucceeded},
		{domain.PhaseVerify, domain.PhaseStatusFailed},
	}
	if len(phases) != len(wantSeq) {
		t.Fatalf("phase count = %d, want %d (got %+v)", len(phases), len(wantSeq), phases)
	}
	for i, want := range wantSeq {
		if phases[i].Phase != want.phase || phases[i].Status != want.status {
			t.Errorf("phase[%d] = %q/%q, want %q/%q",
				i, phases[i].Phase, phases[i].Status, want.phase, want.status)
		}
	}

	// Execute must NEVER fail with the synthetic reason that fired before
	// the fix. Walk cycle_failed events; the worker stamps the terminal
	// reason in the event's Data JSON.
	events, err := h.store.ListTaskEvents(bg, tsk.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	sawVerificationFailed := false
	for _, e := range events {
		if e.Type != domain.EventCycleFailed {
			continue
		}
		body := string(e.Data)
		if strings.Contains(body, "execute_phase_start_failed") {
			t.Errorf("cycle_failed carries execute_phase_start_failed (regression of the verify-phase bug): %s", body)
		}
		if strings.Contains(body, "verification_failed") {
			sawVerificationFailed = true
		}
	}
	if !sawVerificationFailed {
		t.Errorf("expected at least one cycle_failed event with reason=verification_failed; got events=%+v", events)
	}

	// Runner must have been invoked twice for execute (initial + 1
	// retry). If the state machine rejected the retry, only one call
	// would have landed.
	executeCalls := 0
	for _, c := range r.Calls() {
		if c.Phase == domain.PhaseExecute {
			executeCalls++
		}
	}
	if executeCalls != 2 {
		t.Fatalf("execute runner calls = %d, want 2 (initial + retry)", executeCalls)
	}
}

// TestWorker_VerifyPhase_usesSeparateRunnerWhenConfigured pins the
// adversarial-separation contract: when Options.VerifyRunner is non-nil
// the verify pass MUST land on it, not on the execute runner. Without
// this the docs' verifier_kind=verify_agent claim of adversarial review
// is structurally false.
func TestWorker_VerifyPhase_usesSeparateRunnerWhenConfigured(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "verify-multi-runner")
	item, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion one", "", domain.ActorUser)
	if err != nil {
		t.Fatalf("add checklist item: %v", err)
	}

	workDir := t.TempDir()

	execRunner := runnerfake.New().WithName("exec-runner")
	execHook := &hookRunner{Runner: execRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseExecute {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeCriteriaReport(t, workDir, cycles[0].ID, []string{item.ID})
		}
	}}
	execRunner.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "exec ok", nil, ""))

	verifyRunner := runnerfake.New().WithName("verify-runner")
	verifyHook := &hookRunner{Runner: verifyRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseVerify {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeVerifyReport(t, workDir, cycles[0].ID, []string{item.ID})
		}
	}}
	verifyRunner.Script(tsk.ID, domain.PhaseVerify, runner.NewResult(
		domain.PhaseStatusSucceeded, "verify ok", nil, ""))

	_, done := h.startWorker(ctx, execHook, worker.Options{
		WorkingDir:   workDir,
		VerifyRunner: verifyHook,
	})
	h.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}

	execCalls := execRunner.Calls()
	for _, c := range execCalls {
		if c.Phase == domain.PhaseVerify {
			t.Fatalf("execute runner saw a verify request: %+v", c)
		}
	}
	verifyCalls := verifyRunner.Calls()
	if len(verifyCalls) != 1 || verifyCalls[0].Phase != domain.PhaseVerify {
		t.Fatalf("verify runner calls = %+v, want exactly 1 verify request", verifyCalls)
	}
}

// TestWorker_VerifyPhase_failsCycleWhenVerifyTampers pins the
// integrity-check contract. A verify runner that mutates source files
// MUST cause the cycle to terminate as verify_tampered with no
// retries, regardless of verify_max_retries. Tampering is verifier
// misbehaviour; retrying execute cannot fix it.
func TestWorker_VerifyPhase_failsCycleWhenVerifyTampers(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "verify-tampers")
	item, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion one", "", domain.ActorUser)
	if err != nil {
		t.Fatalf("add checklist item: %v", err)
	}

	maxRetries := 3
	if _, err := h.store.UpdateSettings(ctx, store.SettingsPatch{VerifyMaxRetries: &maxRetries}); err != nil {
		t.Fatalf("set verify max retries: %v", err)
	}

	workDir := t.TempDir()
	gitInitTestRepo(t, workDir)

	execRunner := runnerfake.New()
	execHook := &hookRunner{Runner: execRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseExecute {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeCriteriaReport(t, workDir, cycles[0].ID, []string{item.ID})
		}
	}}
	execRunner.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "exec ok", nil, ""))

	verifyRunner := runnerfake.New().WithName("naughty-verify")
	verifyHook := &hookRunner{Runner: verifyRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseVerify {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeVerifyReport(t, workDir, cycles[0].ID, []string{item.ID})
		}
		// Tamper: drop a stray file in the working dir root, outside
		// the allowed .t2a/<cycleID>/verify-report.json path.
		if err := os.WriteFile(filepath.Join(workDir, "MUTATED.txt"), []byte("hi"), 0o644); err != nil {
			t.Logf("tamper write: %v", err)
		}
	}}
	verifyRunner.Script(tsk.ID, domain.PhaseVerify, runner.NewResult(
		domain.PhaseStatusSucceeded, "verify ok", nil, ""))

	_, done := h.startWorker(ctx, execHook, worker.Options{
		WorkingDir:   workDir,
		VerifyRunner: verifyHook,
	})
	final := h.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}

	if final.Status != domain.StatusFailed {
		t.Fatalf("task status = %q, want failed", final.Status)
	}

	bg := context.Background()
	cycles, _ := h.store.ListCyclesForTask(bg, tsk.ID, 5)
	if len(cycles) != 1 {
		t.Fatalf("cycle count = %d, want 1 (no retries on tamper)", len(cycles))
	}
	if cycles[0].Status != domain.CycleStatusFailed {
		t.Fatalf("cycle status = %q, want failed", cycles[0].Status)
	}

	events, _ := h.store.ListTaskEvents(bg, tsk.ID)
	sawTampered := false
	for _, e := range events {
		if e.Type != domain.EventCycleFailed {
			continue
		}
		if strings.Contains(string(e.Data), "verify_tampered") {
			sawTampered = true
		}
	}
	if !sawTampered {
		t.Fatalf("expected cycle_failed event with reason=verify_tampered; events=%+v", events)
	}

	// Verify must have been invoked exactly once: tampering is
	// terminal, retries do not run.
	verifyCallCount := 0
	for _, c := range verifyRunner.Calls() {
		if c.Phase == domain.PhaseVerify {
			verifyCallCount++
		}
	}
	if verifyCallCount != 1 {
		t.Fatalf("verify runner verify calls = %d, want 1 (terminal-not-retryable)", verifyCallCount)
	}
}

// TestWorker_VerifyPhase_persistsAndPublishesProgressEventsUnderVerifyPhaseSeq
// pins the SPA Activity-panel P3 visibility property: progress events
// emitted by the verify runner MUST be persisted under the verify
// phase row's seq so the per-phase filter renders them. Today's V1 had
// zero P3 stream events because the verify runner.Request had no
// OnProgress callback.
func TestWorker_VerifyPhase_persistsAndPublishesProgressEventsUnderVerifyPhaseSeq(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "verify-progress")
	item, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion one", "", domain.ActorUser)
	if err != nil {
		t.Fatalf("add checklist item: %v", err)
	}

	workDir := t.TempDir()

	execRunner := runnerfake.New()
	execHook := &hookRunner{Runner: execRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseExecute {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeCriteriaReport(t, workDir, cycles[0].ID, []string{item.ID})
		}
	}}
	execRunner.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "exec ok", nil, ""))

	verifyRunner := runnerfake.New()
	var verifyProgressFired atomic.Bool
	verifyHook := &hookRunner{Runner: verifyRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseVerify {
			return
		}
		if req.OnProgress != nil {
			verifyProgressFired.Store(true)
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeVerifyReport(t, workDir, cycles[0].ID, []string{item.ID})
		}
	}}
	verifyRunner.Script(tsk.ID, domain.PhaseVerify, runner.NewResult(
		domain.PhaseStatusSucceeded, "verify ok", nil, ""))

	_, done := h.startWorker(ctx, execHook, worker.Options{
		WorkingDir:   workDir,
		VerifyRunner: verifyHook,
	})
	h.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}

	if !verifyProgressFired.Load() {
		t.Fatal("verify runner.Request.OnProgress was nil; progress wiring missing")
	}

	bg := context.Background()
	cycles, _ := h.store.ListCyclesForTask(bg, tsk.ID, 1)
	if len(cycles) != 1 {
		t.Fatalf("cycle count = %d, want 1", len(cycles))
	}
	phases, _ := h.store.ListPhasesForCycle(bg, cycles[0].ID)
	var verifyPhaseSeq int64
	for _, p := range phases {
		if p.Phase == domain.PhaseVerify {
			verifyPhaseSeq = p.PhaseSeq
		}
	}
	if verifyPhaseSeq == 0 {
		t.Fatalf("no verify phase row found; phases=%+v", phases)
	}

	deadline := time.Now().Add(2 * time.Second)
	var verifyEvents int
	for time.Now().Before(deadline) {
		events, err := h.store.ListCycleStreamEvents(bg, cycles[0].ID, 0, 50)
		if err != nil {
			t.Fatalf("list cycle stream events: %v", err)
		}
		verifyEvents = 0
		for _, ev := range events {
			if ev.PhaseSeq == verifyPhaseSeq {
				verifyEvents++
			}
		}
		if verifyEvents > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if verifyEvents == 0 {
		t.Fatalf("no stream events under verify phase_seq=%d (P3 panel would be empty)", verifyPhaseSeq)
	}
}
