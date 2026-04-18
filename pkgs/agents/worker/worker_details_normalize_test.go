package worker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// Regression for: when runner.Result.Details holds a well-formed JSON
// value that is NOT an object (literal null, string, array, number, bool)
// or malformed JSON, the worker forwarded it verbatim into
// store.CompletePhase. The store's kernel.NormalizeJSONObject chokepoint
// (sessions 1+2) rejects non-object payloads with domain.ErrInvalidInput,
// so CompletePhase returns an error, the worker bails, and the cycle /
// phase / task rows are left in `running` (state.cycleStarted is cleared
// so the deferred recovery does NOT terminate the cycle). Only the
// startup orphan sweep eventually cleans them — until a restart, the SPA
// shows a permanently in-flight task.
//
// Original symptom (2026-04-18): cursor adapter returns
// `parsed.Details json.RawMessage` straight from `cursor --output-format
// json`. Any cursor invocation that emits `"details": null` (or any non-
// object value) silently orphans the workflow.
//
// The fix: detailsBytes() must coerce non-object / malformed JSON into a
// JSON object so the store invariant always holds; the original bytes
// are preserved in the envelope so the audit trail still shows what the
// runner produced.
func TestRegression_Worker_normalizes_non_object_runner_details(t *testing.T) {
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
			h := newHarness(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tsk := h.createReadyTask(ctx, "details:"+tc.name)

			r := runnerfake.New()
			r.Script(tsk.ID, domain.PhaseExecute, runner.Result{
				Status:  domain.PhaseStatusSucceeded,
				Summary: "ok",
				Details: tc.details,
			})

			_, done := h.startWorker(ctx, r, worker.Options{})
			final := h.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
			cancel()
			if err := <-done; err != nil {
				t.Fatalf("worker exit err: %v", err)
			}

			if final.Status != domain.StatusDone {
				t.Fatalf("task status = %q, want done", final.Status)
			}

			cycle := assertCycleStatus(t, h.store, tsk.ID, 1, domain.CycleStatusSucceeded)

			bg := context.Background()
			phases, err := h.store.ListPhasesForCycle(bg, cycle.ID)
			if err != nil {
				t.Fatalf("list phases: %v", err)
			}
			if len(phases) != 2 {
				t.Fatalf("phase count = %d, want 2", len(phases))
			}
			exec := phases[1]
			if exec.Phase != domain.PhaseExecute || exec.Status != domain.PhaseStatusSucceeded {
				t.Fatalf("execute phase = %q/%q, want execute/succeeded", exec.Phase, exec.Status)
			}

			trimmed := bytes.TrimSpace(exec.DetailsJSON)
			if len(trimmed) == 0 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
				t.Fatalf("phase details_json = %q, want a JSON object payload", string(exec.DetailsJSON))
			}
			var asObject map[string]json.RawMessage
			if err := json.Unmarshal(exec.DetailsJSON, &asObject); err != nil {
				t.Fatalf("phase details_json is not a JSON object: %v (raw=%q)", err, string(exec.DetailsJSON))
			}
		})
	}
}

// TestRegression_Worker_object_details_pass_through_unchanged pins the
// happy-path side of the normalization rule: a runner that already
// returns a JSON object in Details must see the bytes land verbatim on
// the phase row (no envelope wrap, no re-marshal). This guards against a
// future "always wrap" regression in detailsBytes.
func TestRegression_Worker_object_details_pass_through_unchanged(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "object-details")

	original := json.RawMessage(`{"ok":true,"count":3,"nested":{"k":"v"}}`)
	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.Result{
		Status:  domain.PhaseStatusSucceeded,
		Summary: "ok",
		Details: original,
	})

	_, done := h.startWorker(ctx, r, worker.Options{})
	final := h.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	cancel()
	<-done

	if final.Status != domain.StatusDone {
		t.Fatalf("task status = %q, want done", final.Status)
	}
	cycle := assertCycleStatus(t, h.store, tsk.ID, 1, domain.CycleStatusSucceeded)
	bg := context.Background()
	phases, err := h.store.ListPhasesForCycle(bg, cycle.ID)
	if err != nil {
		t.Fatalf("list phases: %v", err)
	}
	exec := phases[1]
	if !bytes.Equal(exec.DetailsJSON, []byte(original)) {
		t.Fatalf("phase details_json = %q, want %q", string(exec.DetailsJSON), string(original))
	}
}
