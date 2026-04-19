package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// mustStartCycle is the contract suite's POST /tasks/{id}/cycles helper. It
// returns the assigned cycle id so individual subtests can target the URL
// segment under test.
func mustStartCycle(t *testing.T, baseURL, taskID string) string {
	t.Helper()
	res, raw := doCyclesRequest(t, http.MethodPost, baseURL+"/tasks/"+taskID+"/cycles", `{}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("seed cycle: status %d body=%s", res.StatusCode, raw)
	}
	var c taskCycleResponse
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode seed cycle: %v body=%s", err, raw)
	}
	return c.ID
}

// TestHTTP_postTaskCycle_400ErrorStrings pins every documented 400 string
// for POST /tasks/{id}/cycles. Drift in any of these phrases breaks the test
// in lockstep with docs/API-HTTP.md (Stage 6 commits the doc rows).
func TestHTTP_postTaskCycle_400ErrorStrings(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)

	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unknownField",
			body: `{"nope":1}`,
			want: `json: unknown field "nope"`,
		},
		{
			name: "trailingData",
			body: `{}{}`,
			want: "request body must contain a single JSON value",
		},
		{
			name: "emptyParentString",
			body: `{"parent_cycle_id":""}`,
			want: "parent_cycle_id",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := doCyclesRequest(t, http.MethodPost, srv.URL+"/tasks/"+taskID+"/cycles", tc.body)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatalf("decode: %v body=%s", err, raw)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q", errBody.Error, tc.want)
			}
		})
	}
}

// TestHTTP_postTaskCycle_rejectsConcurrentRunning ensures the "at most one
// running cycle per task" invariant from the store surfaces as a 400 with
// the documented bare phrase, not a 500.
func TestHTTP_postTaskCycle_rejectsConcurrentRunning(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)
	_ = mustStartCycle(t, srv.URL, taskID)

	res, raw := doCyclesRequest(t, http.MethodPost, srv.URL+"/tasks/"+taskID+"/cycles", `{}`)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Error != "task already has a running cycle" {
		t.Fatalf("error=%q", errBody.Error)
	}
}

// TestHTTP_postTaskCycle_taskNotFound_returns_404 exercises the FK-style
// 404 path: nonexistent task → ErrNotFound mapped to 404 with the standard
// "not found" body.
func TestHTTP_postTaskCycle_taskNotFound_returns_404(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, raw := doCyclesRequest(t, http.MethodPost,
		srv.URL+"/tasks/00000000-0000-0000-0000-000000000099/cycles", `{}`)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Error != "not found" {
		t.Fatalf("error=%q", errBody.Error)
	}
}

// TestHTTP_postTaskCycle_response_shape pins the JSON envelope for the
// 201 success path. Adding or removing a top-level key without updating
// the docs (and the web client) breaks here.
func TestHTTP_postTaskCycle_response_shape(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)

	res, raw := doCyclesRequest(t, http.MethodPost, srv.URL+"/tasks/"+taskID+"/cycles", `{}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	if got := res.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content-type=%q", got)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	requireKeys := []string{"id", "task_id", "attempt_seq", "status", "started_at", "triggered_by", "meta"}
	for _, k := range requireKeys {
		if _, ok := doc[k]; !ok {
			t.Fatalf("missing key %q (full=%v)", k, doc)
		}
	}
	for _, k := range []string{"ended_at", "parent_cycle_id"} {
		if _, ok := doc[k]; ok {
			t.Fatalf("optional key %q must be omitted on a fresh running cycle (full=%v)", k, doc)
		}
	}
}

// TestHTTP_getTaskCycles_400ErrorStrings pins limit-validation messages.
func TestHTTP_getTaskCycles_400ErrorStrings(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)

	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"limitOver200", "?limit=999", "limit must be integer 0..200"},
		{"limitNegative", "?limit=-1", "limit must be integer 0..200"},
		{"limitNonNumeric", "?limit=nope", "limit must be integer 0..200"},
		{"limitTooLong", "?limit=" + strings.Repeat("9", 33), "limit too long"},
		{"beforeZero", "?before_attempt_seq=0", "before_attempt_seq must be a positive integer"},
		{"beforeNegative", "?before_attempt_seq=-1", "before_attempt_seq must be a positive integer"},
		{"beforeNonNumeric", "?before_attempt_seq=nope", "before_attempt_seq must be a positive integer"},
		{"beforeTooLong", "?before_attempt_seq=" + strings.Repeat("9", 33), "before_attempt_seq too long"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := doCyclesRequest(t, http.MethodGet, srv.URL+"/tasks/"+taskID+"/cycles"+tc.query, "")
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatal(err)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q", errBody.Error, tc.want)
			}
		})
	}
}

// TestHTTP_getTaskCycles_response_shape pins the envelope for the empty
// case (no cycles yet). Cycles must be `[]`, never null or omitted.
func TestHTTP_getTaskCycles_response_shape(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)

	res, raw := doCyclesRequest(t, http.MethodGet, srv.URL+"/tasks/"+taskID+"/cycles", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"task_id", "cycles", "limit", "has_more"} {
		if _, ok := doc[k]; !ok {
			t.Fatalf("missing key %q (full=%v)", k, doc)
		}
	}
	if string(doc["cycles"]) != "[]" {
		t.Fatalf("empty cycles must serialise as [], got %s", doc["cycles"])
	}
	if string(doc["has_more"]) != "false" {
		t.Fatalf("has_more must be false on empty list, got %s", doc["has_more"])
	}
	if _, ok := doc["next_before_attempt_seq"]; ok {
		t.Fatalf("next_before_attempt_seq must be omitted when has_more=false (omitempty contract); got %s", doc["next_before_attempt_seq"])
	}
}

// TestHTTP_getTaskCycles_keysetCursor pins the keyset pagination wire
// contract added in Session 30: ?before_attempt_seq= MUST return cycles
// strictly older than the cursor (never the cursor row itself, which
// would duplicate across pages), and the response envelope MUST include
// next_before_attempt_seq when has_more=true so clients can paginate
// without re-implementing the cursor calculation themselves. Mirrors
// the store-side keyset test (TestStore_ListCyclesForTaskBefore_keysetCursor)
// at the HTTP layer so a future handler refactor that drops the cursor
// query param parse, the limit+1 fan-out, or the cursor write-back
// would fail loudly.
func TestHTTP_getTaskCycles_keysetCursor(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)
	listURL := srv.URL + "/tasks/" + taskID + "/cycles"
	// At-most-one-running means we have to terminate each cycle before
	// starting the next; seed three completed cycles so the cursor test has
	// real attempt_seq values to walk.
	for i := 0; i < 3; i++ {
		cid := mustStartCycle(t, srv.URL, taskID)
		res, raw := doCyclesRequest(t, http.MethodPatch,
			srv.URL+"/tasks/"+taskID+"/cycles/"+cid,
			`{"status":"succeeded"}`)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("seed terminate cycle #%d: %d body=%s", i+1, res.StatusCode, raw)
		}
	}

	t.Run("firstPageHasNextCursor", func(t *testing.T) {
		res, raw := doCyclesRequest(t, http.MethodGet, listURL+"?limit=1", "")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status %d body=%s", res.StatusCode, raw)
		}
		var env taskCyclesListResponse
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatal(err)
		}
		if len(env.Cycles) != 1 {
			t.Fatalf("first page len = %d, want 1", len(env.Cycles))
		}
		if !env.HasMore {
			t.Fatalf("has_more must be true when more pages exist")
		}
		if env.NextBeforeAttemptSeq == nil || *env.NextBeforeAttemptSeq != env.Cycles[0].AttemptSeq {
			t.Fatalf("next_before_attempt_seq = %v, want pointer to %d", env.NextBeforeAttemptSeq, env.Cycles[0].AttemptSeq)
		}
	})

	t.Run("strictlyLessThanCursor", func(t *testing.T) {
		res, raw := doCyclesRequest(t, http.MethodGet, listURL+"?limit=1", "")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("seed: status %d", res.StatusCode)
		}
		var first taskCyclesListResponse
		if err := json.Unmarshal(raw, &first); err != nil {
			t.Fatal(err)
		}
		if first.NextBeforeAttemptSeq == nil {
			t.Fatalf("seed expected next_before_attempt_seq")
		}
		nextURL := listURL + "?limit=200&before_attempt_seq=" + strconv.FormatInt(*first.NextBeforeAttemptSeq, 10)
		res, raw = doCyclesRequest(t, http.MethodGet, nextURL, "")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("next page: status %d body=%s", res.StatusCode, raw)
		}
		var next taskCyclesListResponse
		if err := json.Unmarshal(raw, &next); err != nil {
			t.Fatal(err)
		}
		for _, c := range next.Cycles {
			if c.AttemptSeq >= *first.NextBeforeAttemptSeq {
				t.Fatalf("cursor row leaked: attempt_seq=%d should be < %d (strict <)", c.AttemptSeq, *first.NextBeforeAttemptSeq)
			}
		}
		if next.HasMore {
			t.Fatalf("has_more must be false on last page (only 2 cycles past cursor in a 3-cycle seed); got envelope=%+v", next)
		}
		if next.NextBeforeAttemptSeq != nil {
			t.Fatalf("next_before_attempt_seq must be omitted on last page; got %d", *next.NextBeforeAttemptSeq)
		}
	})
}

// TestHTTP_getTaskCycle_404_when_taskMismatch protects against cross-task
// id smuggling: a valid cycle id under a different task must surface as
// 404, not the foreign cycle's contents.
func TestHTTP_getTaskCycle_404_when_taskMismatch(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskA := mustCreateTaskForCycles(t, srv.URL)
	taskB := mustCreateTaskForCycles(t, srv.URL)
	cycleID := mustStartCycle(t, srv.URL, taskA)

	res, raw := doCyclesRequest(t, http.MethodGet, srv.URL+"/tasks/"+taskB+"/cycles/"+cycleID, "")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-task GET cycle: status %d body=%s", res.StatusCode, raw)
	}
}

// TestHTTP_patchTaskCycle_404_when_taskMismatch covers the same protection
// for the terminate path: a foreign cycle id must not be mutated.
func TestHTTP_patchTaskCycle_404_when_taskMismatch(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskA := mustCreateTaskForCycles(t, srv.URL)
	taskB := mustCreateTaskForCycles(t, srv.URL)
	cycleID := mustStartCycle(t, srv.URL, taskA)

	res, raw := doCyclesRequest(t, http.MethodPatch,
		srv.URL+"/tasks/"+taskB+"/cycles/"+cycleID, `{"status":"succeeded"}`)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-task PATCH cycle: status %d body=%s", res.StatusCode, raw)
	}
}

// TestHTTP_patchTaskCycle_400ErrorStrings pins terminate-status validation.
func TestHTTP_patchTaskCycle_400ErrorStrings(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)
	cycleID := mustStartCycle(t, srv.URL, taskID)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"unknownField", `{"status":"succeeded","oops":1}`, `json: unknown field "oops"`},
		{"emptyBody", `{}`, "status must be a terminal cycle status"},
		{"runningStatus", `{"status":"running"}`, "status must be a terminal cycle status"},
		{"invalidEnum", `{"status":"nope"}`, "status must be a terminal cycle status"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := doCyclesRequest(t, http.MethodPatch, srv.URL+"/tasks/"+taskID+"/cycles/"+cycleID, tc.body)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatal(err)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q", errBody.Error, tc.want)
			}
		})
	}
}

// TestHTTP_patchTaskCycle_alreadyTerminal_400 — the second terminate is the
// documented 400 path, confirming the store's terminal guard reaches the
// HTTP boundary intact.
func TestHTTP_patchTaskCycle_alreadyTerminal_400(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)
	cycleID := mustStartCycle(t, srv.URL, taskID)
	if res, raw := doCyclesRequest(t, http.MethodPatch, srv.URL+"/tasks/"+taskID+"/cycles/"+cycleID, `{"status":"succeeded"}`); res.StatusCode != http.StatusOK {
		t.Fatalf("first terminate: %d %s", res.StatusCode, raw)
	}

	res, raw := doCyclesRequest(t, http.MethodPatch, srv.URL+"/tasks/"+taskID+"/cycles/"+cycleID, `{"status":"failed"}`)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("second terminate status %d body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Error != "cycle already terminal" {
		t.Fatalf("error=%q", errBody.Error)
	}
}

// TestHTTP_postTaskCyclePhase_400ErrorStrings pins phase-start validation.
func TestHTTP_postTaskCyclePhase_400ErrorStrings(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)
	cycleID := mustStartCycle(t, srv.URL, taskID)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"unknownField", `{"phase":"diagnose","oops":1}`, `json: unknown field "oops"`},
		{"emptyPhase", `{}`, "phase"},
		{"invalidPhase", `{"phase":"nope"}`, "phase"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := doCyclesRequest(t, http.MethodPost,
				srv.URL+"/tasks/"+taskID+"/cycles/"+cycleID+"/phases", tc.body)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatal(err)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q", errBody.Error, tc.want)
			}
		})
	}
}

// TestHTTP_postTaskCyclePhase_invalid_transition_400 — phase state machine
// rejection from the store reaches HTTP unchanged.
func TestHTTP_postTaskCyclePhase_invalid_transition_400(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)
	cycleID := mustStartCycle(t, srv.URL, taskID)

	res, raw := doCyclesRequest(t, http.MethodPost,
		srv.URL+"/tasks/"+taskID+"/cycles/"+cycleID+"/phases", `{"phase":"persist"}`)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(errBody.Error, "phase transition") {
		t.Fatalf("error=%q must start with 'phase transition'", errBody.Error)
	}
}

// TestHTTP_postTaskCyclePhase_404_when_taskMismatch — same cross-task guard.
func TestHTTP_postTaskCyclePhase_404_when_taskMismatch(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskA := mustCreateTaskForCycles(t, srv.URL)
	taskB := mustCreateTaskForCycles(t, srv.URL)
	cycleID := mustStartCycle(t, srv.URL, taskA)

	res, raw := doCyclesRequest(t, http.MethodPost,
		srv.URL+"/tasks/"+taskB+"/cycles/"+cycleID+"/phases", `{"phase":"diagnose"}`)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
}

// TestHTTP_patchTaskCyclePhase_400_path_validation pins {phaseSeq} parsing.
func TestHTTP_patchTaskCyclePhase_400_path_validation(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)
	cycleID := mustStartCycle(t, srv.URL, taskID)
	base := srv.URL + "/tasks/" + taskID + "/cycles/" + cycleID + "/phases/"

	cases := []struct {
		name string
		seg  string
		want string
	}{
		{"zero", "0", "phase_seq must be a positive integer"},
		{"negative", "-1", "phase_seq must be a positive integer"},
		{"nonNumeric", "abc", "phase_seq must be a positive integer"},
		{"tooLong", strings.Repeat("9", 33), "phase_seq too long"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := doCyclesRequest(t, http.MethodPatch, base+tc.seg, `{"status":"succeeded"}`)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatal(err)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q", errBody.Error, tc.want)
			}
		})
	}
}

// TestHTTP_patchTaskCyclePhase_400ErrorStrings pins phase-complete status
// validation including the body-level enum guards.
func TestHTTP_patchTaskCyclePhase_400ErrorStrings(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)
	cycleID := mustStartCycle(t, srv.URL, taskID)
	_, phRaw := doCyclesRequest(t, http.MethodPost,
		srv.URL+"/tasks/"+taskID+"/cycles/"+cycleID+"/phases", `{"phase":"diagnose"}`)
	var ph taskCyclePhaseResponse
	if err := json.Unmarshal(phRaw, &ph); err != nil {
		t.Fatal(err)
	}
	url := srv.URL + "/tasks/" + taskID + "/cycles/" + cycleID + "/phases/" + strItoa(ph.PhaseSeq)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"unknownField", `{"status":"succeeded","oops":1}`, `json: unknown field "oops"`},
		{"emptyBody", `{}`, "status must be a terminal phase status"},
		{"runningStatus", `{"status":"running"}`, "status must be a terminal phase status"},
		{"invalidEnum", `{"status":"nope"}`, "status must be a terminal phase status"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := doCyclesRequest(t, http.MethodPatch, url, tc.body)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatal(err)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q", errBody.Error, tc.want)
			}
		})
	}
}

// TestHTTP_cycle_path_segment_caps pins the 128-byte abuse guard for {id}
// and {cycleId}. The same cap is documented for every other task route.
func TestHTTP_cycle_path_segment_caps(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	long := strings.Repeat("a", 129)

	t.Run("taskIdTooLong", func(t *testing.T) {
		res, raw := doCyclesRequest(t, http.MethodGet, srv.URL+"/tasks/"+long+"/cycles", "")
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d body=%s", res.StatusCode, raw)
		}
		var errBody jsonErrorBody
		if err := json.Unmarshal(raw, &errBody); err != nil {
			t.Fatal(err)
		}
		if errBody.Error != "id too long" {
			t.Fatalf("error=%q", errBody.Error)
		}
	})

	t.Run("cycleIdTooLong", func(t *testing.T) {
		taskID := mustCreateTaskForCycles(t, srv.URL)
		res, raw := doCyclesRequest(t, http.MethodGet, srv.URL+"/tasks/"+taskID+"/cycles/"+long, "")
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d body=%s", res.StatusCode, raw)
		}
		var errBody jsonErrorBody
		if err := json.Unmarshal(raw, &errBody); err != nil {
			t.Fatal(err)
		}
		if errBody.Error != "cycle id too long" {
			t.Fatalf("error=%q", errBody.Error)
		}
	})
}

// TestHTTP_getTaskCycle_phase_response_shape pins the envelope and per-phase
// keys returned by GET /tasks/{id}/cycles/{cycleId}.
func TestHTTP_getTaskCycle_phase_response_shape(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)
	cycleID := mustStartCycle(t, srv.URL, taskID)
	doCyclesRequest(t, http.MethodPost,
		srv.URL+"/tasks/"+taskID+"/cycles/"+cycleID+"/phases", `{"phase":"diagnose"}`)

	res, raw := doCyclesRequest(t, http.MethodGet, srv.URL+"/tasks/"+taskID+"/cycles/"+cycleID, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"id", "task_id", "attempt_seq", "status", "started_at", "triggered_by", "meta", "phases"} {
		if _, ok := doc[k]; !ok {
			t.Fatalf("missing top-level key %q (full=%v)", k, doc)
		}
	}
	var phases []map[string]json.RawMessage
	if err := json.Unmarshal(doc["phases"], &phases); err != nil {
		t.Fatalf("decode phases: %v raw=%s", err, doc["phases"])
	}
	if len(phases) != 1 {
		t.Fatalf("phases=%d want 1", len(phases))
	}
	for _, k := range []string{"id", "cycle_id", "phase", "phase_seq", "status", "started_at", "details", "event_seq"} {
		if _, ok := phases[0][k]; !ok {
			t.Fatalf("phases[0] missing key %q (full=%v)", k, phases[0])
		}
	}
	for _, k := range []string{"ended_at", "summary"} {
		if _, ok := phases[0][k]; ok {
			t.Fatalf("phases[0] optional key %q must be omitted on running phase (full=%v)", k, phases[0])
		}
	}
}

// SSE publishing for cycle and phase mutations is asserted by the
// `task_cycle_changed` subtests in TestHTTP_SSE_triggerSurface
// (sse_trigger_surface_test.go). The Stage 4 "must emit zero SSE" guardrail
// was retired in Stage 5 once the trigger surface was lit up.
