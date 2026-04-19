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

// getTask is a focused GET /tasks/{id} helper. Mirrors the deleteTask /
// patchTask helpers in the surrounding contract suites: returns response +
// raw body so each subtest asserts only what it cares about.
func getTask(t *testing.T, baseURL, id string) (*http.Response, []byte) {
	t.Helper()
	res, err := http.Get(baseURL + "/tasks/" + id)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, raw
}

// TestHTTP_getTask_leafRootEnvelope pins the documented two omitempty rules
// for a brand-new leaf root row:
//   - `parent_id` is **omitted entirely** (not "parent_id":null) because the
//     row has no parent.
//   - `children` is **omitted entirely** (not "children":[] and not "children":null)
//     because the row has no descendants.
//
// The remaining keys (including pickup_not_before when the default agent
// pickup delay applies) must always be present for this envelope. A future change that
// emitted "parent_id":null or "children":[] for leaves would silently break
// the doc claim and double the wire size of large list/get responses.
func TestHTTP_getTask_leafRootEnvelope(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	id := mustCreateTask(t, srv.URL, `{"title":"root-leaf","priority":"medium"}`)

	res, raw := getTask(t, srv.URL, id)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d (want 200) body=%s", res.StatusCode, raw)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode envelope: %v body=%s", err, raw)
	}

	wantKeys := []string{"checklist_inherit", "cursor_model", "id", "initial_prompt", "pickup_not_before", "priority", "runner", "status", "task_type", "title"}
	gotKeys := make([]string, 0, len(top))
	for k := range top {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	sort.Strings(wantKeys)
	if !equalStringSlices(gotKeys, wantKeys) {
		t.Fatalf("leaf-root envelope keys=%v want %v (docs/API-HTTP.md GET /tasks/{id}: parent_id and children must be omitempty for a leaf root row, never serialized as null/[])", gotKeys, wantKeys)
	}

	// Defensive raw-bytes guard: a future drift to "parent_id":null or
	// "children":[] would silently break the doc claim above.
	if strings.Contains(string(raw), `"parent_id"`) {
		t.Fatalf("body=%s contains \"parent_id\" key for a root row (must be omitted)", raw)
	}
	if strings.Contains(string(raw), `"children"`) {
		t.Fatalf("body=%s contains \"children\" key for a leaf row (must be omitted)", raw)
	}
}

// TestHTTP_getTask_subtaskHasParentID pins that a child row carries
// `parent_id` as a string UUID (not null). Pairs with the leaf-root test:
// together they prove the omitempty rule is wired correctly in both
// directions.
func TestHTTP_getTask_subtaskHasParentID(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	parent := mustCreateTask(t, srv.URL, `{"title":"p","priority":"medium"}`)
	child := mustCreateTask(t, srv.URL, `{"title":"c","priority":"medium","parent_id":"`+parent+`"}`)

	res, raw := getTask(t, srv.URL, child)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var got struct {
		ParentID *string `json:"parent_id"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if got.ParentID == nil || *got.ParentID != parent {
		t.Fatalf("parent_id=%v want %q (subtask must carry the parent UUID as a string, not null)", got.ParentID, parent)
	}
}

// TestHTTP_getTask_treeRecursive pins the recursive `children` invariant:
// a parent row with one child returns the child nested under `children`,
// and the **child element** itself follows the same omitempty rules (its
// own `children` key is omitted because the child is a leaf). This guards
// the doc claim that `children[]` carries the full subtree recursively, not
// a flattened list.
func TestHTTP_getTask_treeRecursive(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	parent := mustCreateTask(t, srv.URL, `{"title":"p","priority":"medium"}`)
	child := mustCreateTask(t, srv.URL, `{"title":"c","priority":"medium","parent_id":"`+parent+`"}`)

	res, raw := getTask(t, srv.URL, parent)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var node struct {
		ID       string                       `json:"id"`
		Children []map[string]json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if node.ID != parent {
		t.Fatalf("root id=%q want %q", node.ID, parent)
	}
	if len(node.Children) != 1 {
		t.Fatalf("len(children)=%d want 1 body=%s", len(node.Children), raw)
	}
	// Child element: parent_id present (string), children omitted (leaf).
	childRow := node.Children[0]
	if _, ok := childRow["parent_id"]; !ok {
		t.Fatalf("child row missing parent_id key (subtask must carry parent_id) body=%s", raw)
	}
	if _, ok := childRow["children"]; ok {
		t.Fatalf("child row has \"children\" key but the child is a leaf (omitempty must apply recursively) body=%s", raw)
	}
	var childIDStr string
	if err := json.Unmarshal(childRow["id"], &childIDStr); err != nil || childIDStr != child {
		t.Fatalf("child id=%q (err=%v) want %q", childIDStr, err, child)
	}
}

// TestHTTP_getTask_pathSegmentGuard pins the documented bare 400 wire phrases
// for the path-segment guard. Mirrors Sessions 14 and 15's tables for
// DELETE /tasks/{id} and /task-drafts/{id} so the same guard wording is
// pinned across all three routes.
func TestHTTP_getTask_pathSegmentGuard(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	cases := []struct {
		name string
		path string
		want string
	}{
		{"whitespaceOnlyID", "%20%20%20", "id"},
		{"overlongID", strings.Repeat("a", maxTaskPathIDBytes+1), "id too long"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := getTask(t, srv.URL, tc.path)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatalf("decode: %v body=%s", err, raw)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q (docs/API-HTTP.md GET /tasks/{id} 400 strings)", errBody.Error, tc.want)
			}
		})
	}
}

// TestHTTP_getTask_unknownIDIs404 pins the documented 404 mapping for a
// well-formed UUID that does not match any task row. Returns the bare
// "not found" wire phrase from `storeErrorClientMessage`.
func TestHTTP_getTask_unknownIDIs404(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, raw := getTask(t, srv.URL, "11111111-1111-4111-8111-111111111111")
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

// TestHTTP_getTask_trailingSlashIsMux404 pins the documented mux behavior
// for `GET /tasks/` (with the trailing slash but no id segment). The
// standard library `http.ServeMux` returns the literal text body
// `404 page not found\n` (no JSON envelope) — and crucially does **not**
// fall through to `GET /tasks` (the list route). A future framework swap
// or middleware that started rewriting trailing slashes would silently
// change this behavior, so the body shape is asserted explicitly.
func TestHTTP_getTask_trailingSlashIsMux404(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/tasks/")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d (want 404 from mux) body=%s", res.StatusCode, raw)
	}
	// Critical: must NOT have fallen through to the list route.
	if strings.Contains(string(raw), `"tasks":`) {
		t.Fatalf("body=%s looks like the GET /tasks list envelope; the trailing-slash request must NOT fall through to the list route", raw)
	}
	// Mux 404 has no JSON `error` envelope — handler-generated 400s do.
	if strings.Contains(string(raw), `"error"`) {
		t.Fatalf("body=%s contains JSON error envelope; mux 404 must produce text body only", raw)
	}
}

// TestHTTP_getTask_neverPublishesOnSSE pins the read-only-no-publish
// invariant from docs/API-SSE.md ("Read-only `GET` routes never publish")
// for this specific route. Mirrors Session 15's per-route SSE pin for
// /task-drafts/* — colocates the invariant with the GET row's other
// contracts so a reader does not need to cross-reference API-SSE.md to
// learn that GET /tasks/{id} cannot publish.
func TestHTTP_getTask_neverPublishesOnSSE(t *testing.T) {
	srv, _, hub := newSSETriggerServer(t)
	defer srv.Close()

	parent := mustCreateTask(t, srv.URL, `{"title":"p","priority":"medium"}`)
	_ = mustCreateTask(t, srv.URL, `{"title":"c","priority":"medium","parent_id":"`+parent+`"}`)

	// Subscribe AFTER the seed creates so we don't drain task_created events.
	ch, unsub := hub.Subscribe()
	defer unsub()

	if res, raw := getTask(t, srv.URL, parent); res.StatusCode != http.StatusOK {
		t.Fatalf("get parent status %d body=%s", res.StatusCode, raw)
	}
	if res, raw := getTask(t, srv.URL, "11111111-1111-4111-8111-111111111111"); res.StatusCode != http.StatusNotFound {
		t.Fatalf("get unknown status %d body=%s", res.StatusCode, raw)
	}
	if res, raw := getTask(t, srv.URL, "%20"); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("get bad-id status %d body=%s", res.StatusCode, raw)
	}

	got := summarize(drainSSE(t, ch, 0, 200*time.Millisecond))
	if len(got) != 0 {
		t.Fatalf("drained SSE events %v after GET /tasks/{id} round-trip; want zero (read-only routes never publish)", got)
	}
}
