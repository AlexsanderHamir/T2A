package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// TestHTTP_createTask_onHoldStatusAllowed pins the contract that
// callers can submit POST /tasks with status="on_hold" so the create
// modal's "Autonomous execution" toggle in the SPA round-trips through
// the wire untouched. The agent worker only dequeues tasks with
// status="ready" (ReadyForAgentPickup,
// pkgs/tasks/store/internal/tasks/readiness.go), so creating a row in
// on_hold is the supported way to keep a brand-new task out of the
// pickup queue without coupling to a deferred pickup_not_before.
func TestHTTP_createTask_onHoldStatusAllowed(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, raw := postCreate(t, srv.URL, withCreateChecklist(`{"title":"hold","priority":"medium","status":"on_hold"}`))
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d (want 201; on_hold is a valid create-time status) body=%s", res.StatusCode, raw)
	}
	var got domain.Task
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.StatusOnHold {
		t.Fatalf("status=%q want %q (server must echo the on_hold the client asked for)", got.Status, domain.StatusOnHold)
	}
}

// TestHTTP_patchTask_onHoldRoundTrip pins the bidirectional toggle the
// SPA detail page exposes: ready → on_hold (operator parks a task) and
// on_hold → ready (operator hands control back to the agent). Both
// transitions go through PATCH /tasks/{id} with the standard status
// patch shape; the AutonomyConfirmDialog is purely a client-side
// safety net.
func TestHTTP_patchTask_onHoldRoundTrip(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	id := mustCreateTask(t, srv.URL, `{"title":"toggle","priority":"medium"}`)

	// ready → on_hold
	res, raw := patchTask(t, srv.URL, id, `{"status":"on_hold"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("ready→on_hold: status %d body=%s", res.StatusCode, raw)
	}
	var afterHold domain.Task
	if err := json.Unmarshal(raw, &afterHold); err != nil {
		t.Fatal(err)
	}
	if afterHold.Status != domain.StatusOnHold {
		t.Fatalf("status=%q want on_hold", afterHold.Status)
	}

	// on_hold → ready
	res, raw = patchTask(t, srv.URL, id, `{"status":"ready"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("on_hold→ready: status %d body=%s", res.StatusCode, raw)
	}
	var afterResume domain.Task
	if err := json.Unmarshal(raw, &afterResume); err != nil {
		t.Fatal(err)
	}
	if afterResume.Status != domain.StatusReady {
		t.Fatalf("status=%q want ready", afterResume.Status)
	}
}
