package harness_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/harnesstest"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

type correlationCapturingRunner struct {
	mu  sync.Mutex
	req runner.Request
}

func (r *correlationCapturingRunner) Name() string    { return "capture" }
func (r *correlationCapturingRunner) Version() string { return "test" }

func (r *correlationCapturingRunner) EffectiveModel(_ runner.Request) string { return "" }

func (r *correlationCapturingRunner) Run(_ context.Context, req runner.Request) (runner.Result, error) {
	r.mu.Lock()
	r.req = req
	r.mu.Unlock()
	return runner.NewResult(domain.PhaseStatusSucceeded, "ok", json.RawMessage(`{"ok":true}`), ""), nil
}

func (r *correlationCapturingRunner) lastRequest() runner.Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.req
}

func TestHarness_execute_propagates_run_correlation_id_to_runner(t *testing.T) {
	t.Parallel()
	env := harnesstest.NewEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := env.TransitionRunning(ctx, env.CreateReadyTask(ctx, "run-correlation-id"))
	r := &correlationCapturingRunner{}

	done := env.RunHarness(ctx, env.NewHarness(r, harness.Options{}), tsk)
	<-done
	env.WaitTaskStatus(ctx, tsk.ID, domain.StatusDone)

	req := r.lastRequest()
	if req.RunCorrelationID == "" {
		t.Fatal("expected RunCorrelationID on runner.Request")
	}

	bg := context.Background()
	cycles, err := env.Store.ListCyclesForTask(bg, tsk.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) != 1 {
		t.Fatalf("cycles: got %d want 1", len(cycles))
	}
	phases, err := env.Store.ListPhasesForCycle(bg, cycles[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(phases) == 0 {
		t.Fatal("no phases")
	}
	got := domain.RunCorrelationIDFromDetailsJSON(phases[0].DetailsJSON)
	if got != req.RunCorrelationID {
		t.Fatalf("phase details id = %q, runner req id = %q", got, req.RunCorrelationID)
	}
}
