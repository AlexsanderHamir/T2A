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

// deleteTask centralizes the documented DELETE /tasks/{id} round-trip so the
// table-driven 400/404 subtests stay focused on the assertion side. Mirrors
// the patchTask helper in handler_http_patch_contract_test.go.
func deleteTask(t *testing.T, baseURL, id string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, baseURL+"/tasks/"+id, nil)
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

// TestHTTP_deleteTask_204EmptyBody pins the documented success contract:
// 204 No Content with a literally empty response body (no JSON envelope, no
// trailing newline). A future change that starts emitting `{}` or any other
// payload here would silently break clients that rely on the 204 / empty-body
// pair (the SPA uses fetch with .text() and does not attempt a JSON parse).
func TestHTTP_deleteTask_204EmptyBody(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	id := mustCreateTask(t, srv.URL, `{"title":"to-delete","priority":"medium"}`)

	res, raw := deleteTask(t, srv.URL, id)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status %d (want 204) body=%q", res.StatusCode, raw)
	}
	if len(raw) != 0 {
		t.Fatalf("body=%q want empty (DELETE /tasks/{id} is documented as 204 + empty body)", raw)
	}

	res2, raw2 := deleteTask(t, srv.URL, id)
	if res2.StatusCode != http.StatusNotFound {
		t.Fatalf("redelete status %d (want 404) body=%s", res2.StatusCode, raw2)
	}
}

// TestHTTP_deleteTask_pathSegmentGuard pins the path-segment 400 strings
// produced by parseTaskPathID before the request reaches the store. The
// "missing segment" case (a bare `DELETE /tasks/`) is covered separately
// because it is a mux 404 with no JSON body, not a handler 400.
func TestHTTP_deleteTask_pathSegmentGuard(t *testing.T) {
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
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := deleteTask(t, srv.URL, tc.path)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatalf("decode: %v body=%s", err, raw)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q (docs/API-HTTP.md DELETE /tasks/{id} 400 strings)", errBody.Error, tc.want)
			}
		})
	}
}

// TestHTTP_deleteTask_missingPathSegmentIs404 pins the doc claim that the
// `DELETE /tasks/` (trailing slash, no id) request is rejected by the standard
// library mux as a 404 with no JSON body — distinct from the parseTaskPathID
// 400 covered above. The doc explicitly calls this out so a client that
// distinguishes "no such route" from "bad path segment" stays correct.
func TestHTTP_deleteTask_missingPathSegmentIs404(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/tasks/", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d (want 404) body=%q", res.StatusCode, raw)
	}
	// Mux 404 deliberately does not emit the handler's JSON `{"error":"..."}`
	// envelope; the doc's "no JSON `error` body" wording would regress if a
	// future middleware started wrapping this response.
	if strings.Contains(string(raw), `"error"`) {
		t.Fatalf("body=%q must not include a JSON error envelope (mux 404 path)", raw)
	}
}

// TestHTTP_deleteTask_unknownIDIs404 pins the docs/API-HTTP.md store-error
// mapping row: `domain.ErrNotFound` → 404 with the bare wire phrase
// `not found`. The supplied id is a syntactically valid UUID that has never
// been created, so parseTaskPathID accepts it and the store returns
// ErrNotFound from the initial First() lookup.
func TestHTTP_deleteTask_unknownIDIs404(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, raw := deleteTask(t, srv.URL, "00000000-0000-0000-0000-0000000000ff")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d (want 404) body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if errBody.Error != "not found" {
		t.Fatalf("error=%q want %q (docs/API-HTTP.md store-error mapping)", errBody.Error, "not found")
	}
}

// TestHTTP_deleteTask_cascadesToSubtasks pins the documented cascade
// contract: a single DELETE /tasks/{id} on a task with descendants
// removes the entire subtree (root + every child + every grandchild)
// in one call, returning 204. After the call every id in the subtree
// must respond 404 to a follow-up GET.
//
// This replaces the prior `delete subtasks first` 400 rejection: the
// store-side guard was removed in favour of a recursive cascade so
// users no longer have to descend the tree by hand from the SPA. See
// docs/API-HTTP.md "DELETE /tasks/{id}" for the wire-level note.
func TestHTTP_deleteTask_cascadesToSubtasks(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	parent := mustCreateTask(t, srv.URL, `{"title":"p","priority":"medium"}`)
	child := mustCreateTask(t, srv.URL,
		`{"title":"c","priority":"medium","parent_id":"`+parent+`"}`)
	grandchild := mustCreateTask(t, srv.URL,
		`{"title":"gc","priority":"medium","parent_id":"`+child+`"}`)

	res, raw := deleteTask(t, srv.URL, parent)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status %d (want 204 — cascade) body=%s", res.StatusCode, raw)
	}
	if len(raw) != 0 {
		t.Fatalf("body=%q want empty (DELETE cascade still returns 204 + empty body)", raw)
	}

	for _, id := range []string{parent, child, grandchild} {
		resGet, _ := http.Get(srv.URL + "/tasks/" + id)
		_ = resGet.Body.Close()
		if resGet.StatusCode != http.StatusNotFound {
			t.Fatalf("GET %s after cascade status %d (want 404 — every descendant must be gone)",
				id, resGet.StatusCode)
		}
	}
}

// TestHTTP_deleteTask_publishesTaskDeleted pins the row-level SSE cross
// reference for the DELETE row in docs/API-HTTP.md. Session 4's trigger
// surface (sse_trigger_surface_test.go) covers this in the table-driven
// pass; this sibling subtest restates the contract next to the 400/404
// strings so a reader of the DELETE contract suite finds the SSE invariant
// in one place.
func TestHTTP_deleteTask_publishesTaskDeleted(t *testing.T) {
	srv, _, hub := newSSETriggerServer(t)
	defer srv.Close()

	t.Run("noParent_emitsTaskDeleted", func(t *testing.T) {
		id := mustCreateTask(t, srv.URL, `{"title":"orphan","priority":"medium"}`)
		ch, unsub := hub.Subscribe()
		defer unsub()

		if res, raw := deleteTask(t, srv.URL, id); res.StatusCode != http.StatusNoContent {
			t.Fatalf("delete status %d body=%s", res.StatusCode, raw)
		}
		got := summarize(drainSSE(t, ch, 1, 2*time.Second))
		mustEqualEvents(t, "DELETE /tasks/{id} (no parent)", got,
			[]string{string(TaskDeleted) + ":" + id})
	})

	t.Run("withParent_emitsTaskDeletedPlusParentTaskUpdated", func(t *testing.T) {
		parent := mustCreateTask(t, srv.URL, `{"title":"p","priority":"medium"}`)
		child := mustCreateTask(t, srv.URL,
			`{"title":"c","priority":"medium","parent_id":"`+parent+`"}`)
		ch, unsub := hub.Subscribe()
		defer unsub()

		if res, raw := deleteTask(t, srv.URL, child); res.StatusCode != http.StatusNoContent {
			t.Fatalf("delete child status %d body=%s", res.StatusCode, raw)
		}
		got := summarize(drainSSE(t, ch, 2, 2*time.Second))
		// summarize() sorts alphabetically: "task_deleted" < "task_updated".
		mustEqualEvents(t, "DELETE /tasks/{id} (with parent)", got, []string{
			string(TaskDeleted) + ":" + child,
			string(TaskUpdated) + ":" + parent,
		})
	})

	// Cascade pin: deleting a parent that has its own descendants must
	// emit one task_deleted per removed row (root first, then BFS
	// children) plus exactly one task_updated for the surviving
	// grandparent. Mirrors the row-level metric contract: counter
	// increments by N, not by 1.
	t.Run("cascade_emitsOneTaskDeletedPerRowPlusGrandparentUpdated", func(t *testing.T) {
		grandparent := mustCreateTask(t, srv.URL, `{"title":"gp","priority":"medium"}`)
		parent := mustCreateTask(t, srv.URL,
			`{"title":"p","priority":"medium","parent_id":"`+grandparent+`"}`)
		child := mustCreateTask(t, srv.URL,
			`{"title":"c","priority":"medium","parent_id":"`+parent+`"}`)
		ch, unsub := hub.Subscribe()
		defer unsub()

		if res, raw := deleteTask(t, srv.URL, parent); res.StatusCode != http.StatusNoContent {
			t.Fatalf("delete parent (cascade) status %d body=%s", res.StatusCode, raw)
		}
		// 2 task_deleted (parent + child) and 1 task_updated (grandparent).
		// summarize() sorts alphabetically; UUIDs are random, so we sort
		// `want` the same way instead of betting on UUID-collation luck.
		got := summarize(drainSSE(t, ch, 3, 2*time.Second))
		want := []string{
			string(TaskDeleted) + ":" + child,
			string(TaskDeleted) + ":" + parent,
			string(TaskUpdated) + ":" + grandparent,
		}
		sort.Strings(want)
		mustEqualEvents(t, "DELETE /tasks/{id} (cascade)", got, want)
	})
}
