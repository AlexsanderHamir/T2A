package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestPostRUM_acceptsBatchAndRecordsMetrics is the happy-path contract
// for the /v1/rum beacon: a small mixed batch (mutation lifecycle plus
// a web-vital plus an SSE reconnect) returns 204 and bumps the
// rum_events_accepted_total counter by the batch size. Without this
// test the SPA could ship a working beacon while the server silently
// dropped every event — there is no client-visible signal because
// sendBeacon never sees the response body.
func TestPostRUM_acceptsBatchAndRecordsMetrics(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	h := NewHandler(store.NewStore(db), NewSSEHub(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	acceptedBefore := testutil.ToFloat64(middleware.RUMEventsAcceptedCounter())
	droppedBefore := testutil.ToFloat64(middleware.RUMEventsDroppedCounter())

	body, _ := json.Marshal(map[string]any{
		"events": []map[string]any{
			{"type": "mutation_started", "mutation_kind": "task_patch"},
			{"type": "mutation_optimistic_applied", "mutation_kind": "task_patch", "duration_seconds": 0.012},
			{"type": "mutation_settled", "mutation_kind": "task_patch", "duration_seconds": 0.087, "status_code": 200},
			{"type": "sse_reconnected", "duration_seconds": 1.5},
			{"type": "sse_resync_received"},
			{"type": "web_vitals", "name": "LCP", "value": 1234.5},
		},
	})
	res, err := http.Post(srv.URL+"/v1/rum", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d want 204", res.StatusCode)
	}
	if got, want := testutil.ToFloat64(middleware.RUMEventsAcceptedCounter()), acceptedBefore+6; got != want {
		t.Fatalf("accepted counter=%v want %v", got, want)
	}
	if got, want := testutil.ToFloat64(middleware.RUMEventsDroppedCounter()), droppedBefore; got != want {
		t.Fatalf("dropped counter=%v want %v (no events should drop)", got, want)
	}
}

// TestPostRUM_dropsUnknownEventTypes pins the forward-compat policy:
// an event whose `type` is not in validRUMTypes is dropped, counted as
// dropped, but does NOT 400 the rest of the batch. This lets the SPA
// ship a new event type ahead of the server without taking down the
// whole RUM pipeline.
func TestPostRUM_dropsUnknownEventTypes(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	h := NewHandler(store.NewStore(db), NewSSEHub(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	acceptedBefore := testutil.ToFloat64(middleware.RUMEventsAcceptedCounter())
	droppedBefore := testutil.ToFloat64(middleware.RUMEventsDroppedCounter())

	body := []byte(`{"events":[
		{"type":"mutation_started","mutation_kind":"task_patch"},
		{"type":"future_event_we_do_not_know"},
		{"type":"web_vitals","name":"NOT_A_REAL_VITAL","value":1}
	]}`)
	res, err := http.Post(srv.URL+"/v1/rum", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d want 204", res.StatusCode)
	}
	if got, want := testutil.ToFloat64(middleware.RUMEventsAcceptedCounter()), acceptedBefore+1; got != want {
		t.Fatalf("accepted=%v want %v", got, want)
	}
	if got, want := testutil.ToFloat64(middleware.RUMEventsDroppedCounter()), droppedBefore+2; got != want {
		t.Fatalf("dropped=%v want %v", got, want)
	}
}

// TestPostRUM_rejectsEmptyAndOversizedBatches pins the input-validation
// guards. An empty batch is a client bug (the SPA should not flush an
// empty queue), and a >100-event batch suggests the SPA is either
// runaway-looping or trying to DoS the metrics pipeline; both 400.
func TestPostRUM_rejectsEmptyAndOversizedBatches(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	h := NewHandler(store.NewStore(db), NewSSEHub(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	emptyRes, err := http.Post(srv.URL+"/v1/rum", "application/json", strings.NewReader(`{"events":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	emptyRes.Body.Close()
	if emptyRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty batch status=%d want 400", emptyRes.StatusCode)
	}

	events := make([]map[string]any, maxRUMBatchSize+1)
	for i := range events {
		events[i] = map[string]any{"type": "mutation_started", "mutation_kind": "task_patch"}
	}
	body, _ := json.Marshal(map[string]any{"events": events})
	bigRes, err := http.Post(srv.URL+"/v1/rum", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	bigRes.Body.Close()
	if bigRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("oversize batch status=%d want 400", bigRes.StatusCode)
	}
}

// TestFoldRUMEvent_validatesDurationAndStatus is a unit-level pin for
// the per-event validation rules. The HTTP path tests cover the happy
// case; this one exercises the boundary policy directly so a refactor
// of foldRUMEvent that loosens (or tightens) acceptance shows up as a
// failing test rather than a metrics surprise.
func TestFoldRUMEvent_validatesDurationAndStatus(t *testing.T) {
	cases := []struct {
		name string
		ev   rumEvent
		ok   bool
	}{
		{"mutation_started accepted", rumEvent{Type: "mutation_started", MutationKind: "task_patch"}, true},
		{"optimistic with neg duration dropped", rumEvent{Type: "mutation_optimistic_applied", MutationKind: "task_patch", DurationSeconds: -0.1}, false},
		{"settled with huge duration dropped", rumEvent{Type: "mutation_settled", MutationKind: "task_patch", DurationSeconds: 99999, StatusCode: 200}, false},
		{"settled with 5xx accepted (errors are observable)", rumEvent{Type: "mutation_settled", MutationKind: "task_patch", DurationSeconds: 0.1, StatusCode: 503}, true},
		{"reconnect with zero duration accepted", rumEvent{Type: "sse_reconnected", DurationSeconds: 0}, true},
		{"reconnect with negative duration dropped", rumEvent{Type: "sse_reconnected", DurationSeconds: -1}, false},
		{"web_vitals unknown name dropped", rumEvent{Type: "web_vitals", Name: "BOGUS", Value: 1}, false},
		{"web_vitals known name accepted", rumEvent{Type: "web_vitals", Name: "INP", Value: 50}, true},
		{"unknown type dropped", rumEvent{Type: "garbage"}, false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if got := foldRUMEvent(c.ev); got != c.ok {
				t.Fatalf("foldRUMEvent=%v want %v", got, c.ok)
			}
		})
	}
}
