package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"
)

// saveDraft is a focused POST /task-drafts helper. Mirrors the patchTask /
// deleteTask helpers in the surrounding contract suites: returns the response
// + raw body so each subtest asserts only what it cares about.
func saveDraft(t *testing.T, baseURL, body string) (*http.Response, []byte) {
	t.Helper()
	res, err := http.Post(baseURL+"/task-drafts", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, raw
}

func getDraft(t *testing.T, baseURL, id string) (*http.Response, []byte) {
	t.Helper()
	res, err := http.Get(baseURL + "/task-drafts/" + id)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, raw
}

func deleteDraft(t *testing.T, baseURL, id string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, baseURL+"/task-drafts/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, raw
}

func listDrafts(t *testing.T, baseURL, query string) (*http.Response, []byte) {
	t.Helper()
	url := baseURL + "/task-drafts"
	if query != "" {
		url += "?" + query
	}
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, raw
}

// TestHTTP_saveDraft_201Envelope pins the documented POST /task-drafts 201
// envelope shape: exactly the four keys {id, name, created_at, updated_at}
// and no `payload` echo. A future change that started returning the payload
// (or any extra field) would silently break the doc claim and double the
// list-page weight.
func TestHTTP_saveDraft_201Envelope(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, raw := saveDraft(t, srv.URL, `{"name":"my draft","payload":{"title":"hi","priority":"medium"}}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d (want 201) body=%s", res.StatusCode, raw)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode envelope: %v body=%s", err, raw)
	}
	wantKeys := []string{"id", "name", "created_at", "updated_at"}
	gotKeys := make([]string, 0, len(top))
	for k := range top {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	sort.Strings(wantKeys)
	if !equalStringSlices(gotKeys, wantKeys) {
		t.Fatalf("envelope keys=%v want %v (docs/API-HTTP.md POST /task-drafts row pins {id,name,created_at,updated_at} with no payload echo)", gotKeys, wantKeys)
	}

	var sum struct {
		ID, Name             string
		CreatedAt, UpdatedAt time.Time `json:"-"`
	}
	if err := json.Unmarshal(raw, &sum); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if sum.ID == "" || sum.Name != "my draft" {
		t.Fatalf("summary={%q,%q} want non-empty id + literal name", sum.ID, sum.Name)
	}
}

// TestHTTP_saveDraft_serverAssignsID pins the documented "id is optional"
// behavior: omitted, empty, or whitespace-only `id` produces a server-assigned
// UUID. Three sub-cases keep the assertions explicit.
func TestHTTP_saveDraft_serverAssignsID(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	cases := []struct {
		name string
		body string
	}{
		{"omittedID", `{"name":"a","payload":{}}`},
		{"emptyID", `{"id":"","name":"b","payload":{}}`},
		{"whitespaceID", `{"id":"   ","name":"c","payload":{}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := saveDraft(t, srv.URL, tc.body)
			if res.StatusCode != http.StatusCreated {
				t.Fatalf("status %d (want 201) body=%s", res.StatusCode, raw)
			}
			var sum struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &sum); err != nil {
				t.Fatalf("decode: %v body=%s", err, raw)
			}
			if strings.TrimSpace(sum.ID) == "" {
				t.Fatalf("server-assigned id=%q want non-empty (docs/API-HTTP.md POST /task-drafts: omitted/empty/whitespace id → server UUID)", sum.ID)
			}
		})
	}
}

// TestHTTP_saveDraft_isUpsert pins the documented upsert semantic: a second
// POST with the same `id` replaces `name` and `payload`, refreshes
// `updated_at`, and **preserves** `created_at` from the first save. This is
// the contract clients rely on for autosave + manual save coexistence.
func TestHTTP_saveDraft_isUpsert(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	const id = "draft-upsert-001"
	res1, raw1 := saveDraft(t, srv.URL,
		`{"id":"`+id+`","name":"first","payload":{"title":"v1"}}`)
	if res1.StatusCode != http.StatusCreated {
		t.Fatalf("first save status %d body=%s", res1.StatusCode, raw1)
	}
	var first struct {
		ID, Name  string
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	if err := json.Unmarshal(raw1, &first); err != nil {
		t.Fatalf("decode first: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	res2, raw2 := saveDraft(t, srv.URL,
		`{"id":"`+id+`","name":"second","payload":{"title":"v2"}}`)
	if res2.StatusCode != http.StatusCreated {
		t.Fatalf("re-save status %d body=%s", res2.StatusCode, raw2)
	}
	var second struct {
		ID, Name  string
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	if err := json.Unmarshal(raw2, &second); err != nil {
		t.Fatalf("decode second: %v", err)
	}
	if second.ID != id {
		t.Fatalf("upsert id=%q want %q (id must be preserved)", second.ID, id)
	}
	if second.Name != "second" {
		t.Fatalf("upsert name=%q want %q (name must be replaced)", second.Name, "second")
	}
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("upsert created_at=%v want %v (created_at must be preserved across upsert)", second.CreatedAt, first.CreatedAt)
	}
	if !second.UpdatedAt.After(first.UpdatedAt) {
		t.Fatalf("upsert updated_at=%v want > %v (updated_at must move forward)", second.UpdatedAt, first.UpdatedAt)
	}

	resGet, rawGet := getDraft(t, srv.URL, id)
	if resGet.StatusCode != http.StatusOK {
		t.Fatalf("GET after upsert status %d body=%s", resGet.StatusCode, rawGet)
	}
	var detail struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(rawGet, &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if !strings.Contains(string(detail.Payload), `"v2"`) {
		t.Fatalf("payload after upsert=%s want to contain v2 (payload must be replaced)", detail.Payload)
	}
}

// TestHTTP_saveDraft_400ErrorStrings pins every documented POST /task-drafts
// 400 wire phrase against the live handler, mirroring the table-driven
// pattern from Sessions 9 and 11. Each subtest drives one rejection path so a
// future refactor that changes the store/handler/encoding-json wording breaks
// loudly here.
func TestHTTP_saveDraft_400ErrorStrings(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	cases := []struct {
		name string
		body string
		want string
	}{
		{"missingName", `{"payload":{}}`, "draft name required"},
		{"emptyName", `{"name":"","payload":{}}`, "draft name required"},
		{"whitespaceName", `{"name":"   ","payload":{}}`, "draft name required"},
		{"unknownField", `{"name":"x","payload":{},"extra":1}`, `json: unknown field "extra"`},
		{"trailingData", `{"name":"x","payload":{}}{"name":"y"}`, "request body must contain a single JSON value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := saveDraft(t, srv.URL, tc.body)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatalf("decode: %v body=%s", err, raw)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q (docs/API-HTTP.md /task-drafts/* 400 strings)", errBody.Error, tc.want)
			}
		})
	}
}

// TestHTTP_saveDraft_emptyPayloadCoercedToObject pins the documented "missing
// or null payload silently coerced to {}" semantic. Two sub-cases (null and
// omitted) both round-trip through GET and assert the wire-level "{}".
func TestHTTP_saveDraft_emptyPayloadCoercedToObject(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	cases := []struct{ name, body string }{
		{"omittedPayload", `{"name":"omit"}`},
		{"nullPayload", `{"name":"null","payload":null}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := saveDraft(t, srv.URL, tc.body)
			if res.StatusCode != http.StatusCreated {
				t.Fatalf("save status %d body=%s", res.StatusCode, raw)
			}
			var sum struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &sum); err != nil {
				t.Fatalf("decode: %v", err)
			}
			resGet, rawGet := getDraft(t, srv.URL, sum.ID)
			if resGet.StatusCode != http.StatusOK {
				t.Fatalf("GET status %d body=%s", resGet.StatusCode, rawGet)
			}
			var detail struct {
				Payload json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawGet, &detail); err != nil {
				t.Fatalf("decode detail: %v", err)
			}
			if string(detail.Payload) != "{}" {
				t.Fatalf("payload=%s want %q (docs claim missing/null payload coerces to JSON object)", detail.Payload, "{}")
			}
		})
	}
}

// TestHTTP_listDrafts_envelope pins the GET /task-drafts envelope contract:
// `{"drafts":[...]}` always present, drafts is always a JSON array (`[]`
// when empty, never null/omitted), each row exact key set without payload,
// ordering is updated_at DESC.
func TestHTTP_listDrafts_envelope(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	t.Run("emptyDB_returnsEmptyArray", func(t *testing.T) {
		res, raw := listDrafts(t, srv.URL, "")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status %d (want 200) body=%s", res.StatusCode, raw)
		}
		// Raw-bytes guard: a future drift to `null` or omitting the key would
		// silently break the SPA (`drafts.map(...)` on null throws).
		if !strings.Contains(string(raw), `"drafts":[]`) {
			t.Fatalf("body=%s want literal `\"drafts\":[]` substring (docs claim drafts is always a JSON array, [] when empty)", raw)
		}
	})

	t.Run("populated_orderingAndKeys", func(t *testing.T) {
		// Three drafts saved in order alpha → beta → gamma; the GET response
		// must return them gamma → beta → alpha (updated_at DESC). A 10ms
		// sleep between saves keeps the timestamps strictly monotonic across
		// SQLite's millisecond resolution.
		ids := make([]string, 3)
		for i, n := range []string{"alpha", "beta", "gamma"} {
			res, raw := saveDraft(t, srv.URL, `{"name":"`+n+`","payload":{}}`)
			if res.StatusCode != http.StatusCreated {
				t.Fatalf("seed %s status %d body=%s", n, res.StatusCode, raw)
			}
			var sum struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &sum); err != nil {
				t.Fatalf("decode seed: %v", err)
			}
			ids[i] = sum.ID
			time.Sleep(10 * time.Millisecond)
		}

		res, raw := listDrafts(t, srv.URL, "")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("list status %d body=%s", res.StatusCode, raw)
		}
		var env struct {
			Drafts []map[string]json.RawMessage `json:"drafts"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("decode envelope: %v body=%s", err, raw)
		}
		if len(env.Drafts) != 3 {
			t.Fatalf("len(drafts)=%d want 3 body=%s", len(env.Drafts), raw)
		}
		wantKeys := []string{"id", "name", "created_at", "updated_at"}
		sort.Strings(wantKeys)
		for i, row := range env.Drafts {
			gotKeys := make([]string, 0, len(row))
			for k := range row {
				gotKeys = append(gotKeys, k)
			}
			sort.Strings(gotKeys)
			if !equalStringSlices(gotKeys, wantKeys) {
				t.Fatalf("drafts[%d] keys=%v want %v (no payload on summary view)", i, gotKeys, wantKeys)
			}
		}
		// Order: gamma (newest) → beta → alpha (oldest).
		var first, second, third struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(raw, &struct {
			Drafts []*struct {
				Name string `json:"name"`
			} `json:"drafts"`
		}{Drafts: []*struct {
			Name string `json:"name"`
		}{&first, &second, &third}})
		if first.Name != "gamma" || second.Name != "beta" || third.Name != "alpha" {
			t.Fatalf("ordering=%q,%q,%q want gamma,beta,alpha (updated_at DESC)",
				first.Name, second.Name, third.Name)
		}
	})
}

// TestHTTP_listDrafts_400Limit pins the documented bare 400 wire phrases for
// the limit query parameter. The handler emits its own messages here (not
// the store's invalidInputDetail path), so the wording is asserted verbatim.
func TestHTTP_listDrafts_400Limit(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"overlongValue", "limit=" + strings.Repeat("1", maxListIntQueryParamBytes+1), "limit value too long"},
		{"nonNumeric", "limit=nope", "limit must be integer 0..100"},
		{"negative", "limit=-1", "limit must be integer 0..100"},
		{"overMax", "limit=101", "limit must be integer 0..100"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := listDrafts(t, srv.URL, tc.query)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatalf("decode: %v body=%s", err, raw)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q (docs/API-HTTP.md /task-drafts/* 400 strings)", errBody.Error, tc.want)
			}
		})
	}
}

// TestHTTP_getDraft_envelope pins the GET /task-drafts/{id} 200 envelope
// shape (exact five keys) and the always-present payload invariant.
func TestHTTP_getDraft_envelope(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	resSave, rawSave := saveDraft(t, srv.URL, `{"name":"detail","payload":{"k":"v"}}`)
	if resSave.StatusCode != http.StatusCreated {
		t.Fatalf("seed save status %d body=%s", resSave.StatusCode, rawSave)
	}
	var sum struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rawSave, &sum)

	res, raw := getDraft(t, srv.URL, sum.ID)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d (want 200) body=%s", res.StatusCode, raw)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	wantKeys := []string{"id", "name", "payload", "created_at", "updated_at"}
	gotKeys := make([]string, 0, len(top))
	for k := range top {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	sort.Strings(wantKeys)
	if !equalStringSlices(gotKeys, wantKeys) {
		t.Fatalf("envelope keys=%v want %v (docs/API-HTTP.md GET /task-drafts/{id} pins {id,name,payload,created_at,updated_at})", gotKeys, wantKeys)
	}
	if string(top["payload"]) == "" || string(top["payload"]) == "null" {
		t.Fatalf("payload=%q want non-empty JSON value (docs claim payload is always present)", top["payload"])
	}
}

// TestHTTP_getDraft_404 pins the documented 404 mapping for an unknown id.
func TestHTTP_getDraft_404(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, raw := getDraft(t, srv.URL, "no-such-draft-9999")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d (want 404) body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if errBody.Error != "not found" {
		t.Fatalf("error=%q want %q (store-error mapping)", errBody.Error, "not found")
	}
}

// TestHTTP_draftsPathSegmentGuard pins the path-segment 400 phrases for the
// GET-detail and DELETE routes (the same `parseTaskPathID` guard covered by
// Session 14's DELETE /tasks/{id} contract). Two sub-tests cover both verbs.
func TestHTTP_draftsPathSegmentGuard(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	cases := []struct {
		name string
		path string
		want string
	}{
		{"whitespaceOnlyID", "%20%20%20", "id"},
		{"overlongID", strings.Repeat("a", 129), "id too long"},
	}

	t.Run("GET", func(t *testing.T) {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				res, raw := getDraft(t, srv.URL, tc.path)
				assertBareError(t, res, raw, http.StatusBadRequest, tc.want)
			})
		}
	})
	t.Run("DELETE", func(t *testing.T) {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				res, raw := deleteDraft(t, srv.URL, tc.path)
				assertBareError(t, res, raw, http.StatusBadRequest, tc.want)
			})
		}
	})
}

// TestHTTP_deleteDraft_204AndIdempotent404 pins the 204 + empty-body success
// contract and the documented "re-DELETE returns 404" behavior.
func TestHTTP_deleteDraft_204AndIdempotent404(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	resSave, rawSave := saveDraft(t, srv.URL, `{"name":"to-delete","payload":{}}`)
	if resSave.StatusCode != http.StatusCreated {
		t.Fatalf("seed save status %d body=%s", resSave.StatusCode, rawSave)
	}
	var sum struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rawSave, &sum)

	res1, raw1 := deleteDraft(t, srv.URL, sum.ID)
	if res1.StatusCode != http.StatusNoContent {
		t.Fatalf("first delete status %d (want 204) body=%q", res1.StatusCode, raw1)
	}
	if len(raw1) != 0 {
		t.Fatalf("first delete body=%q want empty (docs claim 204 + empty body)", raw1)
	}

	res2, raw2 := deleteDraft(t, srv.URL, sum.ID)
	if res2.StatusCode != http.StatusNotFound {
		t.Fatalf("re-delete status %d (want 404) body=%s", res2.StatusCode, raw2)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw2, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw2)
	}
	if errBody.Error != "not found" {
		t.Fatalf("re-delete error=%q want %q", errBody.Error, "not found")
	}
}

// TestHTTP_drafts_neverPublishOnSSE pins the documented invariant that none
// of the four /task-drafts/* routes (POST, GET list, GET detail, DELETE)
// publish anything on the SSE hub. docs/API-SSE.md states this generally for
// the wildcard `/task-drafts/*`; this test pins it per-route so a future
// regression that adds a `notifyChange` call to e.g. saveTaskDraft breaks
// loudly here.
func TestHTTP_drafts_neverPublishOnSSE(t *testing.T) {
	srv, _, hub := newSSETriggerServer(t)
	defer srv.Close()

	ch, unsub := hub.Subscribe()
	defer unsub()

	res, raw := saveDraft(t, srv.URL, `{"id":"sse-drafts-001","name":"sse-test","payload":{"k":"v"}}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("save status %d body=%s", res.StatusCode, raw)
	}

	if res, raw := listDrafts(t, srv.URL, ""); res.StatusCode != http.StatusOK {
		t.Fatalf("list status %d body=%s", res.StatusCode, raw)
	}
	if res, raw := getDraft(t, srv.URL, "sse-drafts-001"); res.StatusCode != http.StatusOK {
		t.Fatalf("get status %d body=%s", res.StatusCode, raw)
	}
	if res, raw := deleteDraft(t, srv.URL, "sse-drafts-001"); res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status %d body=%s", res.StatusCode, raw)
	}

	// Drain with want=0 returns whatever showed up inside the timeout — for
	// the no-publish invariant we just want "nothing arrived".
	got := summarize(drainSSE(t, ch, 0, 200*time.Millisecond))
	if len(got) != 0 {
		t.Fatalf("drained SSE events %v after /task-drafts/* round-trip; want zero (docs/API-SSE.md: /task-drafts/* is not part of the SSE surface)", got)
	}
}

// equalStringSlices is a tiny test-only helper. The contract suites grew
// independently so each file rolls its own; the shared signal is "did the
// keys come back as the documented set?"
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// assertBareError centralizes the "status 4xx + JSON body whose `error`
// matches a wire-stable bare phrase" assertion used by both verbs in the
// path-segment-guard suite.
func assertBareError(t *testing.T, res *http.Response, raw []byte, wantStatus int, wantError string) {
	t.Helper()
	if res.StatusCode != wantStatus {
		t.Fatalf("status %d (want %d) body=%s", res.StatusCode, wantStatus, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if errBody.Error != wantError {
		t.Fatalf("error=%q want %q (docs/API-HTTP.md /task-drafts/* 400 strings)", errBody.Error, wantError)
	}
}
