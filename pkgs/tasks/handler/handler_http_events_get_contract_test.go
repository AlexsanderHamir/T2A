package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// getEvent is a focused GET /tasks/{id}/events/{seq} helper. Mirrors
// patchEventUserResponse in the sibling PATCH contract suite: returns the
// raw response and body so each subtest asserts only what it cares about.
func getEvent(t *testing.T, baseURL, taskID, seq string) (*http.Response, []byte) {
	t.Helper()
	res, err := http.Get(baseURL + "/tasks/" + taskID + "/events/" + seq)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, raw
}

// TestHTTP_getEvent_envelopeMandatoryKeys pins the documented 200 envelope on
// a freshly created task whose only event is `task_created` at seq=1: the six
// mandatory keys are present (`task_id`, `seq`, `at`, `type`, `by`, `data`)
// and the three "optional when set" keys (`user_response`, `user_response_at`,
// `response_thread`) are absent because no PATCH has populated them. `data`
// must default to `{}` even though the underlying event was inserted with a
// nil payload (the kernel + handler both normalize). The raw-bytes guard keeps
// `data` typed as the JSON object literal `{}` instead of being decoded into
// a Go map (which would erase the surface-level distinction between `{}` and
// `null`). Mirrors `TestHTTP_patchEvent_successEnvelope` for the PATCH side
// so the two routes share one shape verifier.
func TestHTTP_getEvent_envelopeMandatoryKeys(t *testing.T) {
	srv, _, _ := newSSETriggerServer(t)
	defer srv.Close()
	task := postTaskJSON(t, srv, `{"title":"e","priority":"medium"}`, http.StatusCreated)

	res, raw := getEvent(t, srv.URL, task.ID, "1")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d (want 200) body=%s", res.StatusCode, raw)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}

	wantKeys := []string{"task_id", "seq", "at", "type", "by", "data"}
	gotKeys := make([]string, 0, len(top))
	for k := range top {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	sort.Strings(wantKeys)
	if !equalStringSlices(gotKeys, wantKeys) {
		t.Fatalf("envelope keys=%v want %v (docs/API-HTTP.md GET /tasks/{id}/events/{seq}: mandatory keys only when no user_response/response_thread is set)", gotKeys, wantKeys)
	}

	if string(top["data"]) != "{}" {
		t.Fatalf(`data must be the literal "{}" on a fresh task_created event, got %q (docs/API-HTTP.md "defaulted to {} when empty")`, top["data"])
	}
	var taskID string
	if err := json.Unmarshal(top["task_id"], &taskID); err != nil || taskID != task.ID {
		t.Fatalf("task_id=%q want %q", taskID, task.ID)
	}
	var seq int64
	if err := json.Unmarshal(top["seq"], &seq); err != nil || seq != 1 {
		t.Fatalf("seq=%d want 1", seq)
	}
	var typ string
	if err := json.Unmarshal(top["type"], &typ); err != nil || typ != "task_created" {
		t.Fatalf("type=%q want %q", typ, "task_created")
	}
}

// TestHTTP_getEvent_envelopeOptionalFieldsWhenSet pins the documented
// "optional fields when set" rule: after a PATCH populates user_response,
// user_response_at, and response_thread on an approval_requested row, a
// follow-up GET must surface ALL nine documented keys (the six mandatory +
// the three optional). Without this pin the route could silently drop a
// just-PATCHed user_response field on the next read and no test would
// catch it.
func TestHTTP_getEvent_envelopeOptionalFieldsWhenSet(t *testing.T) {
	srv, st, _ := newSSETriggerServer(t)
	defer srv.Close()
	id := seedApprovalRequested(t, srv, st)

	patchRes, _ := patchEventUserResponse(t, srv.URL, id, "2", `{"user_response":"approved"}`, "")
	if patchRes.StatusCode != http.StatusOK {
		t.Fatalf("seed PATCH status %d (want 200)", patchRes.StatusCode)
	}

	res, raw := getEvent(t, srv.URL, id, "2")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET status %d (want 200) body=%s", res.StatusCode, raw)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}

	wantKeys := []string{
		"task_id", "seq", "at", "type", "by", "data",
		"user_response", "user_response_at", "response_thread",
	}
	gotKeys := make([]string, 0, len(top))
	for k := range top {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	sort.Strings(wantKeys)
	if !equalStringSlices(gotKeys, wantKeys) {
		t.Fatalf("envelope keys=%v want %v (docs/API-HTTP.md GET /tasks/{id}/events/{seq}: optional fields surface after PATCH populates them)", gotKeys, wantKeys)
	}

	var ur string
	if err := json.Unmarshal(top["user_response"], &ur); err != nil || ur != "approved" {
		t.Fatalf("user_response=%q want %q", ur, "approved")
	}
	if string(top["user_response_at"]) == "" || string(top["user_response_at"]) == "null" {
		t.Errorf(`user_response_at must be populated after PATCH, got %q`, top["user_response_at"])
	}
	var thread []map[string]json.RawMessage
	if err := json.Unmarshal(top["response_thread"], &thread); err != nil {
		t.Fatalf("decode response_thread: %v", err)
	}
	if len(thread) != 1 {
		t.Fatalf("len(response_thread)=%d want 1", len(thread))
	}
}

// TestHTTP_getEvent_dataNeverNullOnWire pins the cross-route invariant
// `docs/API-HTTP.md` documents for every event row: `data` is always a JSON
// object literal, never the bare `null` token. The unit-level
// `TestTaskEventDetailFromDomain_normalizes_non_object_data` table covers
// the response builder's `normalizeJSONObjectForResponse` defense in
// isolation; this test pins the same invariant through the live HTTP
// boundary so a future refactor that bypassed the response helper (eg.
// switched to direct gorm-row marshalling) would fail here. Asserts via
// raw bytes so the JSON-object literal `{}` is distinguished from the
// JSON null literal `null` (a `map[string]any` decode would erase that).
func TestHTTP_getEvent_dataNeverNullOnWire(t *testing.T) {
	srv, _, _ := newSSETriggerServer(t)
	defer srv.Close()
	task := postTaskJSON(t, srv, `{"title":"d","priority":"medium"}`, http.StatusCreated)

	res, raw := getEvent(t, srv.URL, task.ID, strconv.Itoa(1))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	gotData := strings.TrimSpace(string(top["data"]))
	if gotData == "" || gotData == "null" {
		t.Fatalf("data=%q must be a JSON object literal (docs/API-HTTP.md GET /tasks/{id}/events/{seq}: data defaulted to {} when empty)", gotData)
	}
	if !strings.HasPrefix(gotData, "{") || !strings.HasSuffix(gotData, "}") {
		t.Fatalf("data=%q must be a JSON object literal {...}", gotData)
	}
}

// TestHTTP_getEvent_neverPublishesSSE pins the negative-side invariant for
// this read-only route: a successful GET must never publish on the SSE hub.
// Mirrors the negative-side patterns Sessions 14–19 added next to each write
// route's contract suite (read-only routes own the same invariant for the
// opposite reason — silently fanning out a frame here would teach clients
// to treat reads as state changes and trigger refetch cascades). Without
// this pin a future refactor that hoisted `notifyChange` into a shared
// "audit lookup completed" hook would be undetectable.
func TestHTTP_getEvent_neverPublishesSSE(t *testing.T) {
	srv, _, hub := newSSETriggerServer(t)
	defer srv.Close()
	task := postTaskJSON(t, srv, `{"title":"q","priority":"medium"}`, http.StatusCreated)

	ch, unsub := hub.Subscribe()
	defer unsub()

	res, raw := getEvent(t, srv.URL, task.ID, "1")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}

	got := summarize(drainSSE(t, ch, 0, 200*time.Millisecond))
	mustEqualEvents(t, "GET /tasks/{id}/events/{seq}", got, []string{})
}

// TestHTTP_getEvent_errorPathsNeverPublish pins the negative-side SSE
// invariant for every error branch GET can take: 400 (path-segment guard)
// and 404 (unknown task or unknown seq) must also stay silent. Mirrors
// `TestHTTP_patchEvent_errorPathsNeverPublish` so the read and write event
// routes share one negative-side fixture surface. One subscription drained
// once at the end so the test fails when ANY of the four cases leaks.
func TestHTTP_getEvent_errorPathsNeverPublish(t *testing.T) {
	srv, _, hub := newSSETriggerServer(t)
	defer srv.Close()
	task := postTaskJSON(t, srv, `{"title":"x","priority":"medium"}`, http.StatusCreated)

	ch, unsub := hub.Subscribe()
	defer unsub()

	overlongSeq := strings.Repeat("9", maxTaskEventSeqParamBytes+1)
	missingID := "00000000-0000-4000-8000-000000000000"
	cases := []struct {
		name, taskID, seq string
		wantStatus        int
		wantError         string
	}{
		{"overlongSeq", task.ID, overlongSeq, http.StatusBadRequest, "seq too long"},
		{"seqZero", task.ID, "0", http.StatusBadRequest, "seq must be a positive integer"},
		{"unknownTask", missingID, "1", http.StatusNotFound, "not found"},
		{"unknownSeq", task.ID, "999", http.StatusNotFound, "not found"},
	}
	for _, tc := range cases {
		res, raw := getEvent(t, srv.URL, tc.taskID, tc.seq)
		if res.StatusCode != tc.wantStatus {
			t.Fatalf("%s: status %d (want %d) body=%s", tc.name, res.StatusCode, tc.wantStatus, raw)
		}
		var errBody jsonErrorBody
		if err := json.Unmarshal(raw, &errBody); err != nil {
			t.Fatalf("%s: decode error body: %v raw=%s", tc.name, err, raw)
		}
		if errBody.Error != tc.wantError {
			t.Fatalf("%s: error=%q want %q (docs/API-HTTP.md GET /tasks/{id}/events/{seq}) body=%s", tc.name, errBody.Error, tc.wantError, raw)
		}
	}

	got := summarize(drainSSE(t, ch, 0, 200*time.Millisecond))
	mustEqualEvents(t, "GET /tasks/{id}/events/{seq} (error paths)", got, []string{})
}
