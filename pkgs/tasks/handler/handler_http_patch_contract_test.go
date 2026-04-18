package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// patchTaskHelper centralizes the documented PATCH /tasks/{id} round-trip so
// the table-driven 400 string subtests stay focused on the assertion side.
func patchTask(t *testing.T, baseURL, id, body string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, baseURL+"/tasks/"+id, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, raw
}

// mustCreateTask is a tiny POST /tasks helper for the patch contract suite.
// Other contract files have their own narrower variants (mustCreateChecklistTask,
// mustCreateChildInheriting); this one accepts an arbitrary body fragment so a
// single helper can drive parent/child/done permutations.
func mustCreateTask(t *testing.T, baseURL, jsonBody string) string {
	t.Helper()
	res, err := http.Post(baseURL+"/tasks", "application/json", strings.NewReader(jsonBody))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create task status %d body=%s", res.StatusCode, body)
	}
	var task domain.Task
	if err := json.Unmarshal(body, &task); err != nil {
		t.Fatalf("decode created task: %v body=%s", err, body)
	}
	return task.ID
}

// TestHTTP_patchTask_400ErrorStrings pins every documented PATCH /tasks/{id}
// 400 string from docs/API-HTTP.md against the live handler. Each subtest
// drives a distinct rejection path so a future refactor that changes the
// store/handler wording breaks loudly here, in lockstep with the doc.
func TestHTTP_patchTask_400ErrorStrings(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	id := mustCreateTask(t, srv.URL, `{"title":"base","priority":"medium"}`)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"noFields_emptyObject", `{}`, "no fields to update"},
		{"noFields_allNulls", `{"status":null,"priority":null,"title":null,"task_type":null,"initial_prompt":null,"checklist_inherit":null}`, "no fields to update"},
		{"emptyTitle", `{"title":"   "}`, "title"},
		{"emptyParentString", `{"parent_id":""}`, "parent_id must not be empty"},
		{"parentNotFound", `{"parent_id":"00000000-0000-0000-0000-000000000099"}`, "parent not found"},
		{"selfParent", `{"parent_id":"` + id + `"}`, "task cannot be its own parent"},
		{"invalidPriority", `{"priority":"nope"}`, "priority"},
		{"invalidStatus", `{"status":"nope"}`, "status"},
		{"invalidTaskType", `{"task_type":"nope"}`, "task_type"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := patchTask(t, srv.URL, id, tc.body)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatalf("decode: %v body=%s", err, raw)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q (docs/API-HTTP.md)", errBody.Error, tc.want)
			}
		})
	}
}

// TestHTTP_patchTask_clearParentWithNull pins the JSON-null "clear parent"
// semantic. Reparents a child onto a parent via PATCH, then PATCHes
// `parent_id:null` and asserts the child's ParentID flipped from set to nil
// (the doc claims "JSON `null` for `parent_id` clears the parent").
func TestHTTP_patchTask_clearParentWithNull(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	parent := mustCreateTask(t, srv.URL, `{"title":"p","priority":"medium"}`)
	child := mustCreateTask(t, srv.URL,
		`{"title":"c","priority":"medium","parent_id":"`+parent+`"}`)

	res, raw := patchTask(t, srv.URL, child, `{"parent_id":null}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("PATCH parent_id=null status %d body=%s", res.StatusCode, raw)
	}
	var got domain.Task
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if got.ParentID != nil {
		t.Fatalf("parent_id=%q want nil after clear", *got.ParentID)
	}
}

// TestHTTP_patchTask_setParentToExisting pins the happy-path reparent: a string
// `parent_id` referencing an existing task moves the child into its subtree.
func TestHTTP_patchTask_setParentToExisting(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	a := mustCreateTask(t, srv.URL, `{"title":"a","priority":"medium"}`)
	b := mustCreateTask(t, srv.URL, `{"title":"b","priority":"medium"}`)

	res, raw := patchTask(t, srv.URL, b, `{"parent_id":"`+a+`"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var got domain.Task
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.ParentID == nil || *got.ParentID != a {
		t.Fatalf("parent_id=%v want %s", got.ParentID, a)
	}
}

// TestHTTP_patchTask_parentCycle pins the cycle-detection 400. Builds parent→
// child, then asks the parent to re-parent under its own child; the doc lists
// `parent would create a cycle` as the bare 400 string.
func TestHTTP_patchTask_parentCycle(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	parent := mustCreateTask(t, srv.URL, `{"title":"p","priority":"medium"}`)
	child := mustCreateTask(t, srv.URL,
		`{"title":"c","priority":"medium","parent_id":"`+parent+`"}`)

	res, raw := patchTask(t, srv.URL, parent, `{"parent_id":"`+child+`"}`)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Error != "parent would create a cycle" {
		t.Fatalf("error=%q want %q (docs/API-HTTP.md)", errBody.Error, "parent would create a cycle")
	}
}

// TestHTTP_patchTask_checklistInheritRequiresParent pins the 400 string when a
// task tries to enable `checklist_inherit` without having a parent. The
// validation runs at the end of applyTaskPatches, after every per-field patch
// has been applied, so it also catches the `parent_id:null + checklist_inherit:true`
// composite (clearing parent in the same patch should still fail).
func TestHTTP_patchTask_checklistInheritRequiresParent(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	orphan := mustCreateTask(t, srv.URL, `{"title":"o","priority":"medium"}`)
	res, raw := patchTask(t, srv.URL, orphan, `{"checklist_inherit":true}`)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Error != "checklist_inherit requires parent_id" {
		t.Fatalf("error=%q want %q (docs/API-HTTP.md)", errBody.Error, "checklist_inherit requires parent_id")
	}
}

// TestHTTP_patchTask_doneBlockedByOpenSubtask pins the descendants-must-be-done
// precondition string. Builds parent→child both `ready`, asks the parent to go
// `done`, and asserts the documented bare 400 phrase.
func TestHTTP_patchTask_doneBlockedByOpenSubtask(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	parent := mustCreateTask(t, srv.URL, `{"title":"p","priority":"medium"}`)
	_ = mustCreateTask(t, srv.URL,
		`{"title":"c","priority":"medium","parent_id":"`+parent+`"}`)

	res, raw := patchTask(t, srv.URL, parent, `{"status":"done"}`)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatal(err)
	}
	const want = "all subtasks must be done before marking this task done"
	if errBody.Error != want {
		t.Fatalf("error=%q want %q (docs/API-HTTP.md)", errBody.Error, want)
	}
}

// TestHTTP_patchTask_doneBlockedByIncompleteChecklist pins the checklist-must-
// be-complete precondition string. Adds a checklist item directly via the store
// (so we don't depend on POST /tasks/{id}/checklist/items wiring), leaves it
// uncompleted, and asserts the bare 400 phrase from
// validateChecklistCompleteTx propagates.
func TestHTTP_patchTask_doneBlockedByIncompleteChecklist(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()

	id := mustCreateTask(t, srv.URL, `{"title":"t","priority":"medium"}`)
	if _, err := st.AddChecklistItem(context.Background(), id, "not done yet", domain.ActorUser); err != nil {
		t.Fatal(err)
	}

	res, raw := patchTask(t, srv.URL, id, `{"status":"done"}`)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatal(err)
	}
	const want = "all checklist items must be done before marking this task done"
	if errBody.Error != want {
		t.Fatalf("error=%q want %q (docs/API-HTTP.md)", errBody.Error, want)
	}
}

// TestHTTP_patchTask_publishesTaskUpdated pins the documented SSE side effect:
// every successful PATCH /tasks/{id} fans out exactly one `task_updated` for
// `{id}` (no extra `task_created` / `task_deleted`). Sessions 4 covered this in
// the trigger surface table; this is the row-level cross reference for the
// PATCH contract spec.
func TestHTTP_patchTask_publishesTaskUpdated(t *testing.T) {
	srv, _, hub := newSSETriggerServer(t)
	defer srv.Close()

	id := mustCreateTask(t, srv.URL, `{"title":"t","priority":"medium"}`)
	ch, unsub := hub.Subscribe()
	defer unsub()

	res, raw := patchTask(t, srv.URL, id, `{"status":"running"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("PATCH status %d body=%s", res.StatusCode, raw)
	}
	got := summarize(drainSSE(t, ch, 1, 2*time.Second))
	mustEqualEvents(t, "PATCH /tasks/{id}", got, []string{string(TaskUpdated) + ":" + id})
}
