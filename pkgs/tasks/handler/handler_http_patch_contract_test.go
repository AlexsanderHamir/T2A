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
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
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

// TestHTTP_patchTask_setsPickupNotBefore pins the Stage 2 happy path: a PATCH
// with an RFC3339 `pickup_not_before` mutates the column and surfaces the new
// time on the response envelope (UTC, second-precision).
func TestHTTP_patchTask_setsPickupNotBefore(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	id := mustCreateTask(t, srv.URL, `{"title":"sched","priority":"medium"}`)
	want := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	res, raw := patchTask(t, srv.URL, id, `{"pickup_not_before":"`+want.Format(time.RFC3339)+`"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var got domain.Task
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if got.PickupNotBefore == nil || !got.PickupNotBefore.Equal(want) {
		t.Fatalf("pickup_not_before=%v want %s", got.PickupNotBefore, want.Format(time.RFC3339))
	}
}

// TestHTTP_patchTask_clearsPickupNotBefore pins the two documented "clear the
// schedule" wire shapes: explicit JSON null AND explicit empty string. Both
// must result in NULL on the column. The empty-string path is what the
// SchedulePicker emits when the operator hits "Clear" in Stages 3+ — we keep
// it symmetric with the null path so SPA code never needs to special-case
// either.
func TestHTTP_patchTask_clearsPickupNotBefore(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()

	cases := []struct {
		name string
		body string
	}{
		{"jsonNull", `{"pickup_not_before":null}`},
		{"emptyString", `{"pickup_not_before":""}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := mustCreateTask(t, srv.URL, `{"title":"clear","priority":"medium"}`)
			seed := time.Now().UTC().Add(3 * time.Hour).Truncate(time.Second)
			if _, err := st.Update(context.Background(), id,
				store.UpdateTaskInput{PickupNotBefore: &store.PickupNotBeforePatch{At: seed}},
				domain.ActorUser); err != nil {
				t.Fatalf("seed pickup: %v", err)
			}
			res, raw := patchTask(t, srv.URL, id, tc.body)
			if res.StatusCode != http.StatusOK {
				t.Fatalf("status %d body=%s", res.StatusCode, raw)
			}
			var got domain.Task
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("decode: %v body=%s", err, raw)
			}
			if got.PickupNotBefore != nil {
				t.Fatalf("pickup_not_before=%s want nil after clear", got.PickupNotBefore.UTC().Format(time.RFC3339))
			}
		})
	}
}

// TestHTTP_patchTask_rejectsBadPickupNotBefore pins the per-stage 400 strings
// for malformed scheduling input on PATCH. The malformed-string path goes
// through decodeJSON's `json decode:` envelope so the visible message is the
// patchPickupNotBeforeField error verbatim. The pre-2000 sentinel path also
// surfaces the bare error string because the same UnmarshalJSON gate fires
// before the store layer ever sees the value.
func TestHTTP_patchTask_rejectsBadPickupNotBefore(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	id := mustCreateTask(t, srv.URL, `{"title":"x","priority":"medium"}`)
	cases := []struct {
		name       string
		body       string
		wantSubstr string
	}{
		{"malformed", `{"pickup_not_before":"yesterday"}`, "pickup_not_before must be RFC3339"},
		{"pre2000", `{"pickup_not_before":"1999-12-31T23:59:59Z"}`, "pickup_not_before must be on or after 2000-01-01"},
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
			if !strings.Contains(errBody.Error, tc.wantSubstr) {
				t.Fatalf("error=%q want substring %q", errBody.Error, tc.wantSubstr)
			}
		})
	}
}

// TestHTTP_patchTask_pickupNotBeforeAloneCounts pins the "no fields to update"
// guard's awareness of the new field: a PATCH carrying ONLY pickup_not_before
// (no title/status/etc.) must succeed, NOT 400 with "no fields to update". The
// internal/tasks.Update guard was extended in Stage 2 to include
// in.PickupNotBefore; this regression test fails on an upstream that forgets
// to add the new field to that guard.
func TestHTTP_patchTask_pickupNotBeforeAloneCounts(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	id := mustCreateTask(t, srv.URL, `{"title":"alone","priority":"medium"}`)
	want := time.Now().UTC().Add(15 * time.Minute).Truncate(time.Second)
	res, raw := patchTask(t, srv.URL, id, `{"pickup_not_before":"`+want.Format(time.RFC3339)+`"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d (want 200; pickup_not_before alone counts as a field) body=%s", res.StatusCode, raw)
	}
}

// TestHTTP_patchTask_emitsTaskUpdatedSSE_onScheduleChange pins the documented
// SSE side-effect for a schedule-only PATCH. The reuse of the existing
// task_updated frame is decision D6 in the plan ("no new event type") — this
// test is the regression gate for that reuse.
func TestHTTP_patchTask_emitsTaskUpdatedSSE_onScheduleChange(t *testing.T) {
	srv, _, hub := newSSETriggerServer(t)
	defer srv.Close()

	id := mustCreateTask(t, srv.URL, `{"title":"sse","priority":"medium"}`)
	ch, unsub := hub.Subscribe()
	defer unsub()

	want := time.Now().UTC().Add(10 * time.Minute).Truncate(time.Second)
	res, raw := patchTask(t, srv.URL, id, `{"pickup_not_before":"`+want.Format(time.RFC3339)+`"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	got := summarize(drainSSE(t, ch, 1, 2*time.Second))
	mustEqualEvents(t, "PATCH /tasks/{id} pickup_not_before", got, []string{string(TaskUpdated) + ":" + id})
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
