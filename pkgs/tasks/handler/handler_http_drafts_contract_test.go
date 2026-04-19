package handler

// /task-drafts cross-route contract pins + shared helpers.
//
// Per-route assertions live in sibling files following the
// handler_http_<surface>_<verb>_contract_test.go convention introduced in the
// checklist/events splits:
//
//   - handler_http_drafts_save_contract_test.go    (POST)
//   - handler_http_drafts_list_contract_test.go    (GET list)
//   - handler_http_drafts_get_contract_test.go     (GET detail)
//   - handler_http_drafts_delete_contract_test.go  (DELETE)
//
// This file keeps two cross-route pins (the path-segment guard that fires for
// both GET and DELETE through `parseTaskPathID`, and the SSE no-publish
// invariant that spans all four verbs) plus the two helpers that are reused
// across contract files in this package: `equalStringSlices` (shared with the
// events / get / events-patch contract suites — same package, same helper) and
// `assertBareError` (local to drafts but kept here so the path-segment guard
// can stay on its own file without dragging the SSE no-publish pin out of
// view).

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

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
