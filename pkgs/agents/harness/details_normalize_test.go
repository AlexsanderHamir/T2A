package harness_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/harnesstest"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func TestHarness_normalizes_non_object_runner_details(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		details json.RawMessage
	}{
		{"json_null", json.RawMessage(`null`)},
		{"json_padded_null", json.RawMessage("  null  ")},
		{"json_string", json.RawMessage(`"some string"`)},
		{"json_array", json.RawMessage(`[1,2,3]`)},
		{"json_number", json.RawMessage(`42`)},
		{"json_true", json.RawMessage(`true`)},
		{"json_false", json.RawMessage(`false`)},
		{"json_malformed", json.RawMessage(`{not json`)},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := harnesstest.NewEnv(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tsk := env.TransitionRunning(ctx, env.CreateReadyTask(ctx, "details:"+tc.name))

			r := runnerfake.New()
			r.Script(tsk.ID, domain.PhaseExecute, runner.Result{
				Status:  domain.PhaseStatusSucceeded,
				Summary: "ok",
				Details: tc.details,
			})

			done := env.RunHarness(ctx, env.NewHarness(r, harness.Options{}), tsk)
			<-done
			final := env.WaitTaskStatus(ctx, tsk.ID, domain.StatusDone)
			if final.Status != domain.StatusDone {
				t.Fatalf("task status = %q, want done", final.Status)
			}

			cycle := harnesstest.AssertCycleStatus(t, env.Store, tsk.ID, 1, domain.CycleStatusSucceeded)
			phases, err := env.Store.ListPhasesForCycle(ctx, cycle.ID)
			if err != nil {
				t.Fatalf("list phases: %v", err)
			}
			if len(phases) != 1 {
				t.Fatalf("phase count = %d, want 1", len(phases))
			}
			exec := phases[0]
			trimmed := bytes.TrimSpace(exec.DetailsJSON)
			if len(trimmed) == 0 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
				t.Fatalf("phase details_json = %q, want a JSON object payload", string(exec.DetailsJSON))
			}
		})
	}
}

func TestHarness_object_details_pass_through_unchanged(t *testing.T) {
	t.Parallel()
	env := harnesstest.NewEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := env.TransitionRunning(ctx, env.CreateReadyTask(ctx, "object-details"))

	original := json.RawMessage(`{"ok":true,"count":3,"nested":{"k":"v"}}`)
	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.Result{
		Status:  domain.PhaseStatusSucceeded,
		Summary: "ok",
		Details: original,
	})

	done := env.RunHarness(ctx, env.NewHarness(r, harness.Options{}), tsk)
	<-done
	env.WaitTaskStatus(ctx, tsk.ID, domain.StatusDone)

	cycle := harnesstest.AssertCycleStatus(t, env.Store, tsk.ID, 1, domain.CycleStatusSucceeded)
	phases, err := env.Store.ListPhasesForCycle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("list phases: %v", err)
	}
	exec := phases[0]
	var got map[string]any
	if err := json.Unmarshal(exec.DetailsJSON, &got); err != nil {
		t.Fatalf("unmarshal details: %v", err)
	}
	var want map[string]any
	if err := json.Unmarshal(original, &want); err != nil {
		t.Fatal(err)
	}
	for k, v := range want {
		if fmt.Sprint(got[k]) != fmt.Sprint(v) {
			t.Fatalf("details[%q] = %v, want %v", k, got[k], v)
		}
	}
}
