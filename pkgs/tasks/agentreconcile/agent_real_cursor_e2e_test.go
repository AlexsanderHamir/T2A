//go:build cursor_real

// agent_real_cursor_e2e_test.go is the operator-run, full-stack smoke
// for the V1 agent worker against the real cursor-agent binary. It is
// excluded from default builds by the cursor_real build tag and
// additionally gated by T2A_TEST_REAL_CURSOR=1 so even with the tag
// set the test no-ops unless the operator opted in. See
// docs/AGENT-WORKER.md "Smoke run" for the operator runbook and
// pkgs/agents/agentsmoke/doc.go for the prompt + assertion rationale.
//
// Run it locally as:
//
//	$env:T2A_TEST_REAL_CURSOR='1'
//	$env:T2A_AGENT_WORKER_CURSOR_BIN='C:\path\to\cursor-agent.cmd' # optional override
//	go test -tags=cursor_real -run TestAgentE2E_RealCursor -race ./pkgs/tasks/agentreconcile/... -count=1
//
// Prerequisites: cursor-agent on PATH (or the env override) and
// Cursor logged in. The test owns a fresh tempdir and a fresh
// in-memory SQLite store; teardown is automatic.

package agentreconcile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/taskapi"
	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/agentsmoke"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"net/http/httptest"
)

// realCursorRunGateEnv is the on/off switch for the real-binary
// e2e. Even with the cursor_real build tag set, the test skips
// unless this env var is exactly "1" so a stray go test invocation
// against the package never triggers a paid Cursor run.
const realCursorRunGateEnv = "T2A_TEST_REAL_CURSOR"

// realCursorBinaryEnv lets operators point the e2e at a specific
// cursor-agent binary (for example the .cmd shim on Windows). When
// unset the adapter's default ("cursor-agent" resolved against PATH)
// is used.
const realCursorBinaryEnv = "T2A_AGENT_WORKER_CURSOR_BIN"

// e2eRealCursorPollTimeout bounds the wait for task.status == done.
// Generous because Cursor cold caches + first-tool-call latency can
// easily exceed a minute in practice; still well below the worker's
// DefaultRunTimeout so this test never masks a genuine worker hang.
const e2eRealCursorPollTimeout = 120 * time.Second

// e2eRealCursorPollInterval is the spacing between store reads while
// waiting for the worker to drive the task to a terminal state. Short
// enough to keep p50 latency on the test low; long enough that the
// store does not see thousands of redundant reads per minute.
const e2eRealCursorPollInterval = 100 * time.Millisecond

// e2eRealCursorReconcileTick mirrors the production reconcile cadence
// closely (taskapiconfig default is 1s). 250ms keeps the test snappy
// without thrashing the queue.
const e2eRealCursorReconcileTick = 250 * time.Millisecond

// e2eRealCursorRunTimeout is the per-attempt budget passed to
// worker.Options.RunTimeout. Sized to the same 90s ceiling as the
// Stage 2 runner-layer smoke; the e2e is otherwise blind to the
// difference between "cursor is slow" and "the worker hung".
const e2eRealCursorRunTimeout = 90 * time.Second

// e2eRealCursorPostDoneSettle is the short window the test sleeps
// after observing task.status == done so the worker's post-
// TerminateCycle writes (transitionTask + AckAfterRecv + the final
// CycleChange notify) finish landing before the test snapshots cycles
// / phases / SSE / metrics. Mirrors the e2eIdleSettleWindow constant
// from the existing fake-runner Stage 6 e2e.
const e2eRealCursorPostDoneSettle = 250 * time.Millisecond

// hubCycleNotifier adapts handler.SSEHub to worker.CycleChangeNotifier
// without dragging in the cmd/taskapi cycleChangeSSEAdapter (which
// lives in package main and cannot be imported). Mirrors that
// adapter's behaviour exactly: nil-safe, blank-id-safe, single
// hub.Publish per call. The production wiring is the source of truth;
// this test-local adapter exists only because Go forbids importing
// from main packages.
type hubCycleNotifier struct {
	hub *handler.SSEHub
}

func (n *hubCycleNotifier) PublishCycleChange(taskID, cycleID string) {
	if n == nil || n.hub == nil || taskID == "" {
		return
	}
	n.hub.Publish(handler.TaskChangeEvent{
		Type:    handler.TaskCycleChanged,
		ID:      taskID,
		CycleID: cycleID,
	})
}

// TestAgentE2E_RealCursor_taskFromHTTPReachesDoneWithFileWritten is
// the full-stack real-cursor smoke (see docs/AGENT-WORKER.md "Smoke
// run"): a real Cursor binary, driven through the same wiring
// cmd/taskapi builds (handler + SSE hub + notifier + reconcile +
// worker + cursor adapter + Prometheus adapter), turns a single
// POST /tasks into a real file on disk and a fully-audited cycle.
// Default-skip in CI; opt-in via T2A_TEST_REAL_CURSOR=1.
func TestAgentE2E_RealCursor_taskFromHTTPReachesDoneWithFileWritten(t *testing.T) {
	if os.Getenv(realCursorRunGateEnv) != "1" {
		t.Skipf("skipping: %s != 1; this test invokes a paid Cursor run", realCursorRunGateEnv)
	}

	binaryPath := os.Getenv(realCursorBinaryEnv)
	if binaryPath == "" {
		binaryPath = "cursor-agent"
	}

	probeCtx, probeCancel := context.WithTimeout(context.Background(), cursor.DefaultProbeTimeout)
	defer probeCancel()
	cursorVersion, probeErr := cursor.Probe(probeCtx, binaryPath, cursor.DefaultProbeTimeout, nil)
	if probeErr != nil {
		t.Fatalf("cursor probe %q failed: %v\nHint: install cursor-agent and set %s to its path",
			binaryPath, probeErr, realCursorBinaryEnv)
	}
	t.Logf("cursor-agent version: %s (binary=%s)", cursorVersion, binaryPath)

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	st := store.NewStore(tasktestdb.OpenSQLite(t))
	hub := handler.NewSSEHub()
	q := agents.NewMemoryQueue(4)
	st.SetReadyTaskNotifier(q)

	sseCh, sseCancel := hub.Subscribe()
	defer sseCancel()

	srv := httptest.NewServer(handler.NewHandler(st, hub, nil))
	defer srv.Close()

	fixture := agentsmoke.NewFixture(t)

	adapter := cursor.New(cursor.Options{
		BinaryPath: binaryPath,
		Version:    cursorVersion,
	})

	reg := prometheus.NewPedanticRegistry()
	metrics, err := taskapi.RegisterAgentWorkerMetricsOn(reg)
	if err != nil {
		t.Fatalf("RegisterAgentWorkerMetricsOn: %v", err)
	}

	w := worker.NewWorker(st, q, adapter, worker.Options{
		RunTimeout: e2eRealCursorRunTimeout,
		WorkingDir: fixture.WorkingDir(),
		Notifier:   &hubCycleNotifier{hub: hub},
		Metrics:    metrics,
	})

	reconcileCtx, reconcileCancel := context.WithCancel(rootCtx)
	defer reconcileCancel()
	reconcileDone := make(chan struct{})
	go func() {
		defer close(reconcileDone)
		agents.RunReconcileLoop(reconcileCtx, st, q, e2eRealCursorReconcileTick)
	}()

	workerCtx, workerCancel := context.WithCancel(rootCtx)
	defer workerCancel()
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- w.Run(workerCtx)
	}()

	createBody, err := json.Marshal(map[string]any{
		"title":          "real cursor smoke",
		"initial_prompt": fixture.Prompt(),
		"status":         "ready",
		"priority":       "medium",
	})
	if err != nil {
		t.Fatalf("marshal create body: %v", err)
	}

	taskID := postTaskAndReturnID(t, srv.URL, string(createBody))
	t.Logf("created task %s with ready status; waiting up to %s for worker to drive it to done",
		taskID, e2eRealCursorPollTimeout)

	finalStatus := waitTaskTerminalE2E(t, rootCtx, st, taskID, e2eRealCursorPollTimeout, e2eRealCursorPollInterval)
	if finalStatus != domain.StatusDone {
		dumpFailedTaskContext(t, rootCtx, st, taskID)
		t.Fatalf("task %s final status = %q, want %q", taskID, finalStatus, domain.StatusDone)
	}

	time.Sleep(e2eRealCursorPostDoneSettle)

	cycles, err := st.ListCyclesForTask(rootCtx, taskID, 10)
	if err != nil {
		t.Fatalf("list cycles for %s: %v", taskID, err)
	}
	if len(cycles) != 1 {
		t.Fatalf("cycle count = %d, want 1 (cycles=%+v)", len(cycles), cycles)
	}
	cyc := cycles[0]
	if cyc.Status != domain.CycleStatusSucceeded {
		t.Fatalf("cycle status = %q, want %q (meta=%s)", cyc.Status, domain.CycleStatusSucceeded, cyc.MetaJSON)
	}
	var meta map[string]string
	if err := json.Unmarshal(cyc.MetaJSON, &meta); err != nil {
		t.Fatalf("unmarshal cycle MetaJSON: %v (raw=%s)", err, cyc.MetaJSON)
	}
	if meta["runner"] != "cursor-cli" {
		t.Fatalf("cycle meta runner = %q, want %q (raw=%s)", meta["runner"], "cursor-cli", cyc.MetaJSON)
	}
	if strings.TrimSpace(meta["runner_version"]) == "" {
		t.Fatalf("cycle meta runner_version is empty; want non-empty cursor version (raw=%s)", cyc.MetaJSON)
	}

	phases, err := st.ListPhasesForCycle(rootCtx, cyc.ID)
	if err != nil {
		t.Fatalf("list phases for cycle %s: %v", cyc.ID, err)
	}
	if len(phases) != 2 {
		t.Fatalf("phase count = %d, want 2 (diagnose+execute); phases=%+v", len(phases), phases)
	}
	if phases[0].Phase != domain.PhaseDiagnose || phases[0].Status != domain.PhaseStatusSkipped {
		t.Fatalf("phase[0] = %q/%q, want diagnose/skipped", phases[0].Phase, phases[0].Status)
	}
	if phases[1].Phase != domain.PhaseExecute || phases[1].Status != domain.PhaseStatusSucceeded {
		t.Fatalf("phase[1] = %q/%q, want execute/succeeded (summary=%q)",
			phases[1].Phase, phases[1].Status, derefString(phases[1].Summary))
	}

	fixture.AssertSucceeded(t)
	if extras := fixture.ExtraFiles(); len(extras) > 0 {
		t.Logf("note: %d extra files inside the agent workdir (likely OS noise on Windows): %v",
			len(extras), extras)
	}

	if !drainSSEUntilCycleChanged(t, sseCh, taskID, cyc.ID, 2*time.Second) {
		t.Fatalf("did not observe a task_cycle_changed SSE event for task=%s cycle=%s within 2s",
			taskID, cyc.ID)
	}

	assertOneSucceededRunRecorded(t, reg)

	workerCancel()
	if err := <-workerDone; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}
	reconcileCancel()
	<-reconcileDone
}

// postTaskAndReturnID issues POST /tasks against baseURL and returns
// the created task's id. Fails the test loudly with the response body
// on any non-201 so a contract drift in handler is surfaced before
// the e2e wastes a Cursor run.
func postTaskAndReturnID(t *testing.T, baseURL, body string) string {
	t.Helper()
	res, err := http.Post(baseURL+"/tasks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /tasks: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("POST /tasks status=%d body=%s", res.StatusCode, raw)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("decode POST /tasks response: %v body=%s", err, raw)
	}
	if created.ID == "" {
		t.Fatalf("POST /tasks returned empty id; body=%s", raw)
	}
	return created.ID
}

// waitTaskTerminalE2E polls st.Get(taskID) until the row reaches a
// terminal status (done / failed / cancelled) or the deadline trips.
// Returns the last observed status (which may be a non-terminal value
// on timeout — the caller should treat anything != Done as a failure
// and dump context).
func waitTaskTerminalE2E(t *testing.T, ctx context.Context, st *store.Store, taskID string, timeout, interval time.Duration) domain.Status {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last domain.Status
	for time.Now().Before(deadline) {
		got, err := st.Get(ctx, taskID)
		if err == nil && got != nil {
			last = got.Status
			if isTerminalTaskStatus(last) {
				return last
			}
		}
		time.Sleep(interval)
	}
	return last
}

// isTerminalTaskStatus reports whether s is a terminal task state for
// the purposes of the e2e poll loop. The agent worker only ever
// transitions to done or failed; either is sufficient to stop polling
// because the test then asserts on the specific terminal value.
func isTerminalTaskStatus(s domain.Status) bool {
	switch s {
	case domain.StatusDone, domain.StatusFailed:
		return true
	default:
		return false
	}
}

// dumpFailedTaskContext is the operator-readable failure dump for the
// "task did not reach done" path. It pulls the latest cycle + phases +
// task event tail and prints them so the operator does not have to
// crack open the SQLite file to debug. RawOutput on the execute phase
// is already redacted by the cursor adapter; we truncate the tail to
// keep the failure message readable.
func dumpFailedTaskContext(t *testing.T, ctx context.Context, st *store.Store, taskID string) {
	t.Helper()
	tsk, err := st.Get(ctx, taskID)
	if err != nil {
		t.Logf("dump: st.Get(%s): %v", taskID, err)
	} else if tsk != nil {
		t.Logf("dump: task status=%q priority=%q", tsk.Status, tsk.Priority)
	}
	cycles, err := st.ListCyclesForTask(ctx, taskID, 10)
	if err != nil {
		t.Logf("dump: list cycles: %v", err)
		return
	}
	for _, c := range cycles {
		t.Logf("dump: cycle %s status=%q meta=%s", c.ID, c.Status, c.MetaJSON)
		phases, err := st.ListPhasesForCycle(ctx, c.ID)
		if err != nil {
			t.Logf("dump:  list phases for %s: %v", c.ID, err)
			continue
		}
		for _, p := range phases {
			t.Logf("dump:  phase %s status=%q summary=%q details_tail=%s",
				p.Phase, p.Status, derefString(p.Summary), tailString(string(p.DetailsJSON), 600))
		}
	}
}

// derefString safely dereferences an optional string column for log
// output. Stored as *string by the persistence layer (NULL-able);
// returns "" on nil so format strings stay readable.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// tailString returns the last n runes of s, prefixed with an
// ellipsis when truncated, so the failure log is readable but not
// flooded by a multi-KB redacted RawOutput payload.
func tailString(s string, n int) string {
	if s == "" {
		return "<empty>"
	}
	if len(s) <= n {
		return s
	}
	return fmt.Sprintf("...(%d bytes elided)...%s", len(s)-n, s[len(s)-n:])
}

// drainSSEUntilCycleChanged reads from the hub channel until it sees
// a task_cycle_changed event matching (taskID, cycleID) or timeout.
// Returns true on first match. Mirrors the assertion shape of the
// existing handler/sse_trigger_surface_test.go drain helper but
// matches on (type, ID, CycleID) instead of returning the full slice
// because the worker emits multiple cycle-change events per run
// (StartCycle, StartPhase, CompletePhase, TerminateCycle) and any one
// of them satisfies the Stage 3 sanity check.
func drainSSEUntilCycleChanged(t *testing.T, ch <-chan string, taskID, cycleID string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case s, ok := <-ch:
			if !ok {
				return false
			}
			var ev handler.TaskChangeEvent
			if err := json.Unmarshal([]byte(s), &ev); err != nil {
				continue
			}
			if ev.Type == handler.TaskCycleChanged && ev.ID == taskID && ev.CycleID == cycleID {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// assertOneSucceededRunRecorded gathers the pedantic registry and
// fails unless exactly one observation landed in
// t2a_agent_runs_total{runner="cursor-cli",terminal_status="succeeded"}
// and exactly one observation landed in
// t2a_agent_run_duration_seconds{runner="cursor-cli"}. Other label
// combinations failing the run would be a regression and are surfaced
// via t.Logf so the operator gets a useful pointer.
func assertOneSucceededRunRecorded(t *testing.T, reg *prometheus.Registry) {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("registry.Gather: %v", err)
	}
	var sawCounter, sawHistogram bool
	for _, mf := range mfs {
		switch mf.GetName() {
		case "t2a_agent_runs_total":
			sawCounter = true
			for _, m := range mf.GetMetric() {
				labels := labelMap(m)
				count := m.GetCounter().GetValue()
				t.Logf("metric: t2a_agent_runs_total{runner=%q,terminal_status=%q} = %v",
					labels["runner"], labels["terminal_status"], count)
				if labels["runner"] == "cursor-cli" && labels["terminal_status"] == "succeeded" {
					if count != 1 {
						t.Errorf("t2a_agent_runs_total{runner=cursor-cli,terminal_status=succeeded} = %v, want 1", count)
					}
				} else if count > 0 {
					t.Errorf("unexpected non-zero counter t2a_agent_runs_total{runner=%q,terminal_status=%q} = %v",
						labels["runner"], labels["terminal_status"], count)
				}
			}
		case "t2a_agent_run_duration_seconds":
			sawHistogram = true
			for _, m := range mf.GetMetric() {
				labels := labelMap(m)
				h := m.GetHistogram()
				t.Logf("metric: t2a_agent_run_duration_seconds{runner=%q} count=%d sum=%v",
					labels["runner"], h.GetSampleCount(), h.GetSampleSum())
				if labels["runner"] == "cursor-cli" {
					if h.GetSampleCount() != 1 {
						t.Errorf("t2a_agent_run_duration_seconds{runner=cursor-cli} count = %d, want 1",
							h.GetSampleCount())
					}
				}
			}
		}
	}
	if !sawCounter {
		t.Errorf("metric t2a_agent_runs_total not present in registry; agent worker metrics did not record")
	}
	if !sawHistogram {
		t.Errorf("metric t2a_agent_run_duration_seconds not present in registry; agent worker metrics did not record")
	}
}

// labelMap collapses a metric's labels into a name->value map so the
// per-metric assertions above stay readable.
func labelMap(m *dto.Metric) map[string]string {
	out := make(map[string]string, len(m.GetLabel()))
	for _, lp := range m.GetLabel() {
		out[lp.GetName()] = lp.GetValue()
	}
	return out
}
