package handler

// POST /task-drafts contract pins. Split out of
// handler_http_drafts_contract_test.go in Session 35 (P6 — file size). Cross-route
// helpers (assertBareError, equalStringSlices) and the SSE no-publish invariant
// remain in the central drafts contract file.

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
