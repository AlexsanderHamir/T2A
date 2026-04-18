package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// patchEventUserResponse is a focused PATCH /tasks/{id}/events/{seq} helper.
// Mirrors getTask/deleteTask in the surrounding contract suites: returns the
// raw response and body so each subtest asserts only what it cares about.
func patchEventUserResponse(t *testing.T, baseURL, taskID, seq, body, actor string) (*http.Response, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(http.MethodPatch, baseURL+"/tasks/"+taskID+"/events/"+seq, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		req.Header.Set("X-Actor", actor)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, raw
}

// seedApprovalRequested creates a task and appends an `approval_requested`
// event at seq=2 (seq=1 is the task_created mirror). Returns the task id; the
// patchable seq is always 2.
func seedApprovalRequested(t *testing.T, srv *httptest.Server, st *store.Store) string {
	t.Helper()
	task := postTaskJSON(t, srv, `{"title":"e","priority":"medium"}`, http.StatusCreated)
	if err := st.AppendTaskEvent(context.Background(), task.ID, domain.EventApprovalRequested, domain.ActorAgent, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	return task.ID
}

// TestHTTP_patchEvent_successEnvelope pins the documented 200 envelope. After
// a successful single-user PATCH against a fresh approval_requested row, the
// nine documented keys are all present (the three "optional when set" fields
// — user_response, user_response_at, response_thread — are now populated, so
// they appear in the body). data is defaulted to {} for the empty payload.
func TestHTTP_patchEvent_successEnvelope(t *testing.T) {
	srv, st, _ := newSSETriggerServer(t)
	defer srv.Close()
	id := seedApprovalRequested(t, srv, st)

	res, raw := patchEventUserResponse(t, srv.URL, id, "2", `{"user_response":"approved"}`, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d (want 200) body=%s", res.StatusCode, raw)
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
		t.Fatalf("envelope keys=%v want %v (docs/API-HTTP.md PATCH /tasks/{id}/events/{seq}: all three of user_response/user_response_at/response_thread must be populated after a successful PATCH)", gotKeys, wantKeys)
	}

	if string(top["data"]) == "" || string(top["data"]) == "null" {
		t.Errorf("data must default to {} on PATCH response, got %q", top["data"])
	}
	var typ string
	if err := json.Unmarshal(top["type"], &typ); err != nil || typ != string(domain.EventApprovalRequested) {
		t.Errorf("type=%q want %q", typ, domain.EventApprovalRequested)
	}
	var ur string
	if err := json.Unmarshal(top["user_response"], &ur); err != nil || ur != "approved" {
		t.Errorf("user_response=%q want %q", ur, "approved")
	}
	var thread []map[string]json.RawMessage
	if err := json.Unmarshal(top["response_thread"], &thread); err != nil {
		t.Fatalf("decode response_thread: %v", err)
	}
	if len(thread) != 1 {
		t.Fatalf("len(response_thread)=%d want 1", len(thread))
	}
}

// TestHTTP_patchEvent_pathSegmentGuard pins the bare 400 wire phrases for the
// `{id}` and `{seq}` path segments on PATCH. Mirrors the get/delete contract
// suites for `{id}` and the existing seq-path validation for `{seq}` so the
// PATCH route owns the same wording in one place. seq=2 is real (seeded) so
// the test exercises the path-segment branch and not store-not-found.
func TestHTTP_patchEvent_pathSegmentGuard(t *testing.T) {
	srv, st, _ := newSSETriggerServer(t)
	defer srv.Close()
	id := seedApprovalRequested(t, srv, st)

	cases := []struct {
		name   string
		taskID string
		seq    string
		want   string
	}{
		{"whitespaceID", "%20%20%20", "2", "id"},
		{"overlongID", strings.Repeat("a", maxTaskPathIDBytes+1), "2", "id too long"},
		{"overlongSeq", id, strings.Repeat("9", maxTaskEventSeqParamBytes+1), "seq too long"},
		{"seqZero", id, "0", "seq must be a positive integer"},
		{"seqNegative", id, "-1", "seq must be a positive integer"},
		{"seqNonNumeric", id, "nope", "seq must be a positive integer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := patchEventUserResponse(t, srv.URL, tc.taskID, tc.seq, `{"user_response":"x"}`, "")
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatalf("decode: %v body=%s", err, raw)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q (docs/API-HTTP.md PATCH /tasks/{id}/events/{seq} 400 strings)", errBody.Error, tc.want)
			}
		})
	}
}

// TestHTTP_patchEvent_bodyValidation400Strings pins the bare 400 wire phrases
// for body validation after the path-segment guards pass. Each case asserts
// the exact `error` string so a future copy-edit in the store or handler
// fails this test in the same PR (not a downstream client).
func TestHTTP_patchEvent_bodyValidation400Strings(t *testing.T) {
	srv, st, _ := newSSETriggerServer(t)
	defer srv.Close()
	id := seedApprovalRequested(t, srv, st)
	tooLong := `{"user_response":"` + strings.Repeat("a", 10_001) + `"}`

	cases := []struct {
		name string
		seq  string
		body string
		want string
	}{
		{"emptyBodyObject", "2", `{}`, "message cannot be empty"},
		{"emptyUserResponse", "2", `{"user_response":""}`, "message cannot be empty"},
		{"whitespaceUserResponse", "2", `{"user_response":"   "}`, "message cannot be empty"},
		{"tooLong", "2", tooLong, "message too long (max 10000 bytes)"},
		{"unknownField", "2", `{"text":"hi"}`, `json: unknown field "text"`},
		{"trailingData", "2", `{"user_response":"x"}{}`, "request body must contain a single JSON value"},
		{"nonAcceptingType", "1", `{"user_response":"x"}`, "this event type does not accept thread messages"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := patchEventUserResponse(t, srv.URL, id, tc.seq, tc.body, "")
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatalf("decode: %v body=%s", err, raw)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q (docs/API-HTTP.md PATCH body-validation 400 subsection)", errBody.Error, tc.want)
			}
		})
	}
}

// TestHTTP_patchEvent_unknownTaskIs404 pins the documented 404 for a
// well-formed UUID that does not match any task. The store loads
// (task_id, seq) in one WHERE so this collapses to the same gorm
// `ErrRecordNotFound` → `domain.ErrNotFound` → handler "not found" path as
// the missing-seq case below.
func TestHTTP_patchEvent_unknownTaskIs404(t *testing.T) {
	srv, _, _ := newSSETriggerServer(t)
	defer srv.Close()

	res, raw := patchEventUserResponse(t, srv.URL,
		"11111111-1111-4111-8111-111111111111", "2",
		`{"user_response":"hello"}`, "")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d (want 404) body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if errBody.Error != "not found" {
		t.Fatalf("error=%q want %q", errBody.Error, "not found")
	}
}

// TestHTTP_patchEvent_unknownSeqIs404 pins the documented 404 for an
// existing task with a missing seq. Pairs with the unknown-task test above:
// both 404 paths share the same wire phrase so a client cannot distinguish
// "wrong task" from "wrong seq" — by design (the (task_id, seq) tuple is
// validated as one unit).
func TestHTTP_patchEvent_unknownSeqIs404(t *testing.T) {
	srv, st, _ := newSSETriggerServer(t)
	defer srv.Close()
	id := seedApprovalRequested(t, srv, st)

	res, raw := patchEventUserResponse(t, srv.URL, id, "999", `{"user_response":"hello"}`, "")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d (want 404) body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if errBody.Error != "not found" {
		t.Fatalf("error=%q want %q", errBody.Error, "not found")
	}
}

// TestHTTP_patchEvent_threadFullIs400 pins the `thread is full` invariant.
// Seeds the cap directly through the store (200 message rows) so the test
// stays sub-second; then exercises the documented HTTP 400 wire phrase on
// the next append.
func TestHTTP_patchEvent_threadFullIs400(t *testing.T) {
	srv, st, _ := newSSETriggerServer(t)
	defer srv.Close()
	id := seedApprovalRequested(t, srv, st)

	ctx := context.Background()
	// 200 = maxResponseThreadEntries (private const in store package).
	for i := 0; i < 200; i++ {
		actor := domain.ActorUser
		if i%2 == 1 {
			actor = domain.ActorAgent
		}
		if err := st.AppendTaskEventResponseMessage(ctx, id, 2, "msg", actor); err != nil {
			t.Fatalf("seed thread entry %d: %v", i, err)
		}
	}

	res, raw := patchEventUserResponse(t, srv.URL, id, "2", `{"user_response":"one too many"}`, "")
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if errBody.Error != "thread is full (max 200 messages)" {
		t.Fatalf("error=%q want %q", errBody.Error, "thread is full (max 200 messages)")
	}
}

// TestHTTP_patchEvent_publishesTaskUpdated pins the SSE invariant: a
// successful PATCH publishes exactly `task_updated:{id}` (and nothing else).
// Subscribing AFTER seed avoids draining the task_created event from the
// fixture. Mirrors Sessions 14–16 colocation pattern: the SSE assertion
// lives next to the rest of this route's contract instead of only in
// sse_trigger_surface_test.go.
func TestHTTP_patchEvent_publishesTaskUpdated(t *testing.T) {
	srv, st, hub := newSSETriggerServer(t)
	defer srv.Close()
	id := seedApprovalRequested(t, srv, st)

	ch, unsub := hub.Subscribe()
	defer unsub()

	res, raw := patchEventUserResponse(t, srv.URL, id, "2", `{"user_response":"go"}`, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}

	got := summarize(drainSSE(t, ch, 1, 2*time.Second))
	mustEqualEvents(t, "PATCH /tasks/{id}/events/{seq}", got, []string{"task_updated:" + id})
}

// TestHTTP_patchEvent_errorPathsNeverPublish pins the negative-side SSE
// invariant for this route: 400 (path-segment, body-validation, non-accepting
// type) and 404 (unknown task, missing seq) must never publish. Without this
// pin, a future refactor that moved `notifyChange` above the error-write
// branch would leak ghost SSE events to clients.
func TestHTTP_patchEvent_errorPathsNeverPublish(t *testing.T) {
	srv, st, hub := newSSETriggerServer(t)
	defer srv.Close()
	id := seedApprovalRequested(t, srv, st)

	ch, unsub := hub.Subscribe()
	defer unsub()

	// 400 path-segment (whitespace id).
	if res, raw := patchEventUserResponse(t, srv.URL, "%20", "2", `{"user_response":"x"}`, ""); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("whitespace id status %d body=%s", res.StatusCode, raw)
	}
	// 400 body validation (empty user_response).
	if res, raw := patchEventUserResponse(t, srv.URL, id, "2", `{"user_response":"   "}`, ""); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("whitespace user_response status %d body=%s", res.StatusCode, raw)
	}
	// 400 non-accepting type (seq=1 is task_created).
	if res, raw := patchEventUserResponse(t, srv.URL, id, "1", `{"user_response":"x"}`, ""); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-accepting type status %d body=%s", res.StatusCode, raw)
	}
	// 404 unknown task.
	if res, raw := patchEventUserResponse(t, srv.URL,
		"22222222-2222-4222-8222-222222222222", "2",
		`{"user_response":"x"}`, ""); res.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown task status %d body=%s", res.StatusCode, raw)
	}
	// 404 missing seq.
	if res, raw := patchEventUserResponse(t, srv.URL, id, "999", `{"user_response":"x"}`, ""); res.StatusCode != http.StatusNotFound {
		t.Fatalf("missing seq status %d body=%s", res.StatusCode, raw)
	}

	got := summarize(drainSSE(t, ch, 0, 200*time.Millisecond))
	if len(got) != 0 {
		t.Fatalf("drained SSE events %v after PATCH error round-trips; want zero (400/404 paths must never publish)", got)
	}
}
