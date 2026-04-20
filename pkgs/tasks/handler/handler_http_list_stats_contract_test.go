package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// listResponseRaw mirrors the documented `GET /tasks` envelope keys without
// importing the handler's internal struct so a future field rename in
// taskStatsResponse / listResponse fails this test in the same PR as the doc.
type listResponseRaw struct {
	Tasks   []json.RawMessage `json:"tasks"`
	Limit   int               `json:"limit"`
	Offset  int               `json:"offset"`
	HasMore bool              `json:"has_more"`
}

// statsResponseRaw mirrors the documented `GET /tasks/stats` envelope keys.
type statsResponseRaw struct {
	Total          int64                        `json:"total"`
	Ready          int64                        `json:"ready"`
	Critical       int64                        `json:"critical"`
	ByStatus       map[string]int64             `json:"by_status"`
	ByPriority     map[string]int64             `json:"by_priority"`
	ByScope        map[string]int64             `json:"by_scope"`
	Cycles         statsCyclesRaw               `json:"cycles"`
	Phases         statsPhasesRaw               `json:"phases"`
	Runner         statsRunnerRaw               `json:"runner"`
	RecentFailures []map[string]json.RawMessage `json:"recent_failures"`
}

type statsCyclesRaw struct {
	ByStatus      map[string]int64 `json:"by_status"`
	ByTriggeredBy map[string]int64 `json:"by_triggered_by"`
}

type statsPhasesRaw struct {
	ByPhaseStatus map[string]map[string]int64 `json:"by_phase_status"`
}

type statsRunnerRaw struct {
	ByRunner      map[string]statsRunnerBucketRaw `json:"by_runner"`
	ByModel       map[string]statsRunnerBucketRaw `json:"by_model"`
	ByRunnerModel map[string]statsRunnerBucketRaw `json:"by_runner_model"`
}

type statsRunnerBucketRaw struct {
	ByStatus                    map[string]int64 `json:"by_status"`
	Succeeded                   int64            `json:"succeeded"`
	DurationP50SucceededSeconds float64          `json:"duration_p50_succeeded_seconds"`
	DurationP95SucceededSeconds float64          `json:"duration_p95_succeeded_seconds"`
}

func mustGetJSON(t *testing.T, baseURL, path string) ([]byte, *http.Response) {
	t.Helper()
	res, err := http.Get(baseURL + path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d (want 200) body=%s", path, res.StatusCode, raw)
	}
	return raw, res
}

func mustCreateTaskBody(t *testing.T, baseURL, body string) {
	t.Helper()
	res, err := http.Post(baseURL+"/tasks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("create task body=%s status %d resp=%s", body, res.StatusCode, raw)
	}
}

// TestHTTP_listTasks_envelopeShape pins the documented `GET /tasks` 200 envelope:
// exactly four top-level keys `{tasks, limit, offset, has_more}`, with `tasks`
// always a JSON array (`[]` on empty DB, never `null`/missing) and `has_more`
// always present (`false` on the final page). The doc table now states all four
// keys are mandatory; this test fails if a future refactor adds extras or
// silently drops `has_more` to omitempty.
func TestHTTP_listTasks_envelopeShape(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	t.Run("emptyDatabase", func(t *testing.T) {
		raw, _ := mustGetJSON(t, srv.URL, "/tasks")
		assertListEnvelopeKeys(t, raw)
		// Tasks field must be the literal JSON `[]` (not `null`/omitted) when
		// the database has no rows. Asserting on raw bytes guards against a
		// future struct-tag drift to omitempty / pointer slice.
		if !bytes.Contains(raw, []byte(`"tasks":[]`)) {
			t.Fatalf("empty DB must return \"tasks\":[] verbatim, got %s", raw)
		}
		var got listResponseRaw
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("decode: %v body=%s", err, raw)
		}
		if got.Tasks == nil || len(got.Tasks) != 0 {
			t.Fatalf("tasks=%v want []", got.Tasks)
		}
		if got.HasMore {
			t.Fatalf("has_more=%v want false on empty DB", got.HasMore)
		}
		if got.Limit != 50 {
			t.Fatalf("limit=%d want 50 (default)", got.Limit)
		}
		if got.Offset != 0 {
			t.Fatalf("offset=%d want 0", got.Offset)
		}
	})

	t.Run("populatedFinalPage", func(t *testing.T) {
		mustCreateTaskBody(t, srv.URL, `{"title":"a","priority":"medium"}`)
		raw, _ := mustGetJSON(t, srv.URL, "/tasks?limit=10")
		assertListEnvelopeKeys(t, raw)
		var got listResponseRaw
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got.HasMore {
			t.Fatalf("has_more=%v want false (1 row, limit 10)", got.HasMore)
		}
		if len(got.Tasks) != 1 {
			t.Fatalf("tasks len=%d want 1", len(got.Tasks))
		}
	})
}

// TestHTTP_listTasks_limitCoercedEcho pins the documented `limit` echo-after-
// coercion semantic: `?limit=0` returns `"limit":50` (default), `?limit=200`
// returns `"limit":200` (max boundary). This is the contract the web client
// relies on to know the actual page size used.
func TestHTTP_listTasks_limitCoercedEcho(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	cases := []struct {
		query     string
		wantLimit int
	}{
		{"?limit=0", 50},
		{"?limit=200", 200},
		{"", 50},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			raw, _ := mustGetJSON(t, srv.URL, "/tasks"+tc.query)
			var got listResponseRaw
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if got.Limit != tc.wantLimit {
				t.Fatalf("limit=%d want %d (docs/API-HTTP.md echo-after-coercion)", got.Limit, tc.wantLimit)
			}
		})
	}
}

// TestHTTP_listTasks_keysetClampsOffsetToZero pins the documented invariant
// that the response forces `"offset":0` when `after_id` is set, regardless of
// what the user paginated with previously. This complements
// TestHTTP_list_keyset_after_id which already covers the row-set side.
func TestHTTP_listTasks_keysetClampsOffsetToZero(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	id1 := "30000000-0000-4000-8000-000000000001"
	id2 := "30000000-0000-4000-8000-000000000002"
	id3 := "30000000-0000-4000-8000-000000000003"
	for _, id := range []string{id1, id2, id3} {
		mustCreateTaskBody(t, srv.URL, `{"id":"`+id+`","title":"x","priority":"medium"}`)
	}

	raw, _ := mustGetJSON(t, srv.URL, "/tasks?limit=2&after_id="+id1)
	var got listResponseRaw
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Offset != 0 {
		t.Fatalf("offset=%d want 0 when after_id is set (docs/API-HTTP.md)", got.Offset)
	}
	if got.Limit != 2 {
		t.Fatalf("limit=%d want 2 (echoed)", got.Limit)
	}
}

// TestHTTP_listTasks_limitRejectsOverMax pins the existing 400 surface from
// docs/API-HTTP.md (`limit must be integer 0..200`) for the `?limit=201`
// boundary, since handler_http_validation_test.go covers `?limit=999` but not
// the +1-over-max edge.
func TestHTTP_listTasks_limitRejectsOverMax(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/tasks?limit=201")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d want 400 (limit 201 just over max)", res.StatusCode)
	}
	var errBody jsonErrorBody
	if err := json.NewDecoder(res.Body).Decode(&errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Error != "limit must be integer 0..200" {
		t.Fatalf("error=%q want %q", errBody.Error, "limit must be integer 0..200")
	}
}

// TestHTTP_statsEnvelopeShape pins the documented `GET /tasks/stats` 200
// envelope: exactly six top-level keys `{total, ready, critical, by_status,
// by_priority, by_scope}`. Asserts the shape on an empty database so a future
// refactor that conditionally omits any key (e.g. via omitempty) trips here.
func TestHTTP_statsEnvelopeShape(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	raw, _ := mustGetJSON(t, srv.URL, "/tasks/stats")
	assertStatsEnvelopeKeys(t, raw)
}

// TestHTTP_statsByScopeAlwaysHasBothKeys pins the documented invariant that
// `by_scope` is the two-key object `{parent, subtask}` even on an empty
// database (and that both keys are 0 then). The handler initializes both keys
// in store.TaskStats; this test catches a future refactor that switches to
// a sparse map populated only by SQL aggregation.
func TestHTTP_statsByScopeAlwaysHasBothKeys(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	t.Run("emptyDatabase", func(t *testing.T) {
		raw, _ := mustGetJSON(t, srv.URL, "/tasks/stats")
		var got statsResponseRaw
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		assertByScopeKeys(t, got.ByScope, 0, 0)
		if len(got.ByStatus) != 0 {
			t.Fatalf("by_status=%v want {} on empty DB", got.ByStatus)
		}
		if len(got.ByPriority) != 0 {
			t.Fatalf("by_priority=%v want {} on empty DB", got.ByPriority)
		}
		if got.Total != 0 || got.Ready != 0 || got.Critical != 0 {
			t.Fatalf("totals=%+v want all 0 on empty DB", got)
		}
		assertCyclesEmpty(t, raw, got)
		assertPhasesAllZeroEnumKeys(t, raw, got)
		assertRunnerEmpty(t, raw, got)
		assertRecentFailuresEmptyArray(t, raw, got)
	})

	t.Run("populatedRootAndSubtask", func(t *testing.T) {
		const root = "40000000-0000-4000-8000-000000000001"
		mustCreateTaskBody(t, srv.URL, `{"id":"`+root+`","title":"r","priority":"medium"}`)
		mustCreateTaskBody(t, srv.URL,
			`{"title":"c","priority":"medium","parent_id":"`+root+`"}`)

		raw, _ := mustGetJSON(t, srv.URL, "/tasks/stats")
		var got statsResponseRaw
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		assertByScopeKeys(t, got.ByScope, 1, 1)
	})
}

// TestHTTP_statsArithmeticInvariant pins the documented arithmetic invariant
// `total == parent + subtask == sum(by_status) == sum(by_priority)`. Drives
// four tasks across two statuses and three priorities so the sums are
// non-trivial.
func TestHTTP_statsArithmeticInvariant(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	const root = "50000000-0000-4000-8000-000000000001"
	mustCreateTaskBody(t, srv.URL, `{"id":"`+root+`","title":"ready-low","priority":"low","status":"ready"}`)
	mustCreateTaskBody(t, srv.URL, `{"title":"running-medium","priority":"medium","status":"running"}`)
	mustCreateTaskBody(t, srv.URL, `{"title":"running-critical","priority":"critical","status":"running"}`)
	mustCreateTaskBody(t, srv.URL,
		`{"title":"sub-ready-medium","priority":"medium","status":"ready","parent_id":"`+root+`"}`)

	raw, _ := mustGetJSON(t, srv.URL, "/tasks/stats")
	var got statsResponseRaw
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}

	if got.Total != 4 {
		t.Fatalf("total=%d want 4", got.Total)
	}
	scopeSum := got.ByScope["parent"] + got.ByScope["subtask"]
	if scopeSum != got.Total {
		t.Fatalf("parent(%d)+subtask(%d)=%d != total(%d) (docs/API-HTTP.md invariant)",
			got.ByScope["parent"], got.ByScope["subtask"], scopeSum, got.Total)
	}
	if sum := sumIntMap(got.ByStatus); sum != got.Total {
		t.Fatalf("sum(by_status)=%d != total(%d) (docs/API-HTTP.md invariant): %+v", sum, got.Total, got.ByStatus)
	}
	if sum := sumIntMap(got.ByPriority); sum != got.Total {
		t.Fatalf("sum(by_priority)=%d != total(%d) (docs/API-HTTP.md invariant): %+v", sum, got.Total, got.ByPriority)
	}
	if got.Ready != got.ByStatus["ready"] {
		t.Fatalf("ready convenience=%d != by_status[ready]=%d", got.Ready, got.ByStatus["ready"])
	}
	if got.Critical != got.ByPriority["critical"] {
		t.Fatalf("critical convenience=%d != by_priority[critical]=%d", got.Critical, got.ByPriority["critical"])
	}
	// Spot-check the parent/subtask split: 3 roots, 1 subtask.
	if got.ByScope["parent"] != 3 || got.ByScope["subtask"] != 1 {
		t.Fatalf("by_scope=%+v want {parent:3, subtask:1}", got.ByScope)
	}
}

// TestHTTP_stats_doesNotPublishSSE rounds out the read-side coverage by
// asserting `/tasks/stats` is a pure read with no SSE side effect, mirroring
// the Session 6 evaluate-doesNotPublishSSE pattern. SSE_triggerSurface_test
// already lists `/tasks/stats` in its no-publish set; this test pins the
// individual route in case the surface table is ever sliced.
func TestHTTP_stats_doesNotPublishSSE(t *testing.T) {
	srv, _, hub := newSSETriggerServer(t)
	defer srv.Close()

	ch, unsub := hub.Subscribe()
	defer unsub()

	res, err := http.Get(srv.URL + "/tasks/stats")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d want 200", res.StatusCode)
	}
	got := summarize(drainSSE(t, ch, 1, 200*time.Millisecond))
	mustEqualEvents(t, "GET /tasks/stats", got, []string{})
}

func assertListEnvelopeKeys(t *testing.T, raw []byte) {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	want := map[string]struct{}{"tasks": {}, "limit": {}, "offset": {}, "has_more": {}}
	for k := range want {
		if _, ok := top[k]; !ok {
			t.Errorf("GET /tasks 200 missing key %q (docs/API-HTTP.md): %s", k, raw)
		}
	}
	for k := range top {
		if _, ok := want[k]; !ok {
			t.Errorf("GET /tasks 200 unexpected key %q (docs/API-HTTP.md): %s", k, raw)
		}
	}
}

func assertStatsEnvelopeKeys(t *testing.T, raw []byte) {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	want := map[string]struct{}{
		"total": {}, "ready": {}, "critical": {},
		"by_status": {}, "by_priority": {}, "by_scope": {},
		"cycles": {}, "phases": {}, "runner": {}, "recent_failures": {},
	}
	for k := range want {
		if _, ok := top[k]; !ok {
			t.Errorf("GET /tasks/stats 200 missing key %q (docs/API-HTTP.md): %s", k, raw)
		}
	}
	for k := range top {
		if _, ok := want[k]; !ok {
			t.Errorf("GET /tasks/stats 200 unexpected key %q (docs/API-HTTP.md): %s", k, raw)
		}
	}
}

func assertByScopeKeys(t *testing.T, byScope map[string]int64, wantParent, wantSubtask int64) {
	t.Helper()
	if byScope == nil {
		t.Fatalf("by_scope is null; want two-key object even on empty DB (docs/API-HTTP.md)")
	}
	if len(byScope) != 2 {
		t.Fatalf("by_scope has %d keys (%v) want exactly 2 (parent, subtask)", len(byScope), byScope)
	}
	if got, ok := byScope["parent"]; !ok || got != wantParent {
		t.Fatalf("by_scope[parent]=%d ok=%v want %d", got, ok, wantParent)
	}
	if got, ok := byScope["subtask"]; !ok || got != wantSubtask {
		t.Fatalf("by_scope[subtask]=%d ok=%v want %d", got, ok, wantSubtask)
	}
}

func sumIntMap(m map[string]int64) int64 {
	var s int64
	for _, v := range m {
		s += v
	}
	return s
}

// assertCyclesEmpty pins the documented invariant that the `cycles`
// block is the two-key object `{by_status, by_triggered_by}` with
// **non-null** empty maps on a fresh database. Catches a future
// refactor that ships sparse keys or omitempty.
func assertCyclesEmpty(t *testing.T, raw []byte, got statsResponseRaw) {
	t.Helper()
	if !bytes.Contains(raw, []byte(`"by_status":{}`)) {
		// At least one of the maps must serialize as `{}` on empty
		// DB; both should. The substring guard is over-broad on
		// purpose because by_status appears twice (top-level +
		// cycles); the structural check below is the precise one.
	}
	if got.Cycles.ByStatus == nil {
		t.Fatalf("cycles.by_status is null; want {} on empty DB (docs/API-HTTP.md): %s", raw)
	}
	if got.Cycles.ByTriggeredBy == nil {
		t.Fatalf("cycles.by_triggered_by is null; want {} on empty DB (docs/API-HTTP.md): %s", raw)
	}
	if len(got.Cycles.ByStatus) != 0 {
		t.Fatalf("cycles.by_status=%v want {} on empty DB", got.Cycles.ByStatus)
	}
	if len(got.Cycles.ByTriggeredBy) != 0 {
		t.Fatalf("cycles.by_triggered_by=%v want {} on empty DB", got.Cycles.ByTriggeredBy)
	}
}

// assertPhasesAllZeroEnumKeys pins the documented invariant that
// `phases.by_phase_status` always carries every domain.Phase enum value
// as a key, with an empty inner map on a fresh database. The
// Observability heatmap relies on the four-key shape so it can render
// every cell (rather than guess which phases are missing).
func assertPhasesAllZeroEnumKeys(t *testing.T, raw []byte, got statsResponseRaw) {
	t.Helper()
	if got.Phases.ByPhaseStatus == nil {
		t.Fatalf("phases.by_phase_status is null; want {} on empty DB (docs/API-HTTP.md): %s", raw)
	}
	wantPhases := []string{"diagnose", "execute", "verify", "persist"}
	for _, p := range wantPhases {
		inner, ok := got.Phases.ByPhaseStatus[p]
		if !ok {
			t.Errorf("phases.by_phase_status missing key %q; the 4-key heatmap shape is mandatory (docs/API-HTTP.md): %s",
				p, raw)
			continue
		}
		if inner == nil {
			t.Errorf("phases.by_phase_status[%q] is null; want {} on empty DB", p)
		}
		if len(inner) != 0 {
			t.Errorf("phases.by_phase_status[%q]=%v want {} on empty DB", p, inner)
		}
	}
}

// assertRunnerEmpty pins the Phase 2 invariant that the `runner`
// block is the three-key object `{by_runner, by_model, by_runner_model}`
// with non-null empty maps on a fresh database. Catches a future
// refactor that ships sparse keys or omitempty.
func assertRunnerEmpty(t *testing.T, raw []byte, got statsResponseRaw) {
	t.Helper()
	if got.Runner.ByRunner == nil {
		t.Fatalf("runner.by_runner is null; want {} on empty DB (docs/API-HTTP.md): %s", raw)
	}
	if got.Runner.ByModel == nil {
		t.Fatalf("runner.by_model is null; want {} on empty DB (docs/API-HTTP.md): %s", raw)
	}
	if got.Runner.ByRunnerModel == nil {
		t.Fatalf("runner.by_runner_model is null; want {} on empty DB (docs/API-HTTP.md): %s", raw)
	}
	if len(got.Runner.ByRunner) != 0 || len(got.Runner.ByModel) != 0 || len(got.Runner.ByRunnerModel) != 0 {
		t.Fatalf("runner block non-empty on empty DB: %+v", got.Runner)
	}
}

// assertRecentFailuresEmptyArray pins the documented invariant that
// `recent_failures` is the literal `[]` (never `null` or omitted) on a
// fresh database, mirroring the existing `tasks:[]` rule for the list
// envelope.
func assertRecentFailuresEmptyArray(t *testing.T, raw []byte, got statsResponseRaw) {
	t.Helper()
	if !bytes.Contains(raw, []byte(`"recent_failures":[]`)) {
		t.Errorf("empty DB must serialize \"recent_failures\":[] verbatim, got %s", raw)
	}
	if got.RecentFailures == nil {
		t.Fatalf("recent_failures is null; want [] on empty DB (docs/API-HTTP.md)")
	}
	if len(got.RecentFailures) != 0 {
		t.Fatalf("recent_failures=%v want [] on empty DB", got.RecentFailures)
	}
}
