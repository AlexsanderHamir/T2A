package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// postCreate centralizes the documented POST /tasks round-trip so the
// table-driven 400 string subtests stay focused on the assertion side.
// (`mustCreateTask` from handler_http_patch_contract_test.go asserts a
// specific success path; this helper keeps the response intact so error-path
// subtests can read it.) Injects checklist_items unless already present.
func postCreate(t *testing.T, baseURL, jsonBody string) (*http.Response, []byte) {
	t.Helper()
	return postCreateRaw(t, baseURL, withCreateChecklist(jsonBody))
}

func postCreateRaw(t *testing.T, baseURL, jsonBody string) (*http.Response, []byte) {
	t.Helper()
	res, err := http.Post(baseURL+"/tasks", "application/json", strings.NewReader(jsonBody))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, raw
}

// TestHTTP_createTask_400ErrorStrings pins every documented POST /tasks 400
// string from docs/api.md against the live handler. Each subtest drives a
// distinct rejection path so a future refactor that changes the store/handler
// wording breaks loudly here, in lockstep with the doc.
func TestHTTP_createTask_400ErrorStrings(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	cases := []struct {
		name string
		body string
		want string
	}{
		{"titleMissing", `{"priority":"medium"}`, "title required"},
		{"titleWhitespace", withCreateChecklist(`{"title":"   ","priority":"medium"}`), "title required"},
		{"priorityMissing", `{"title":"ok","checklist_items":[{"text":"criterion"}]}`, "priority required"},
		{"priorityEmpty", withCreateChecklist(`{"title":"ok","priority":""}`), "priority required"},
		{"priorityInvalid", withCreateChecklist(`{"title":"ok","priority":"nope"}`), "priority"},
		{"statusInvalid", withCreateChecklist(`{"title":"ok","priority":"medium","status":"nope"}`), "status"},
		{"checklistMissing", `{"title":"ok","priority":"medium"}`, "at least one done criterion required"},
		{"checklistEmpty", `{"title":"ok","priority":"medium","checklist_items":[]}`, "at least one done criterion required"},
		{"checklistWhitespaceOnly", `{"title":"ok","priority":"medium","checklist_items":[{"text":"   "}]}`, "at least one done criterion required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := postCreateRaw(t, srv.URL, tc.body)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatalf("decode: %v body=%s", err, raw)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q (docs/api.md)", errBody.Error, tc.want)
			}
		})
	}
}

// TestHTTP_createTask_409DuplicateID pins the documented 409 mapping for a
// caller-supplied `id` that collides with an existing row. There is already a
// 409 test in handler_http_drafts_eval_test.go (TestHTTP_create_duplicate_
// client_id_returns_409) — this one re-pins the bare phrase from the contract
// file's perspective so a future split of the drafts-eval suite cannot lose
// the assertion.
func TestHTTP_createTask_409DuplicateID(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	const id = "60000000-0000-4000-8000-000000000099"
	res1, raw1 := postCreate(t, srv.URL, withCreateChecklist(`{"id":"`+id+`","title":"first","priority":"medium"}`))
	if res1.StatusCode != http.StatusCreated {
		t.Fatalf("first create status %d body=%s", res1.StatusCode, raw1)
	}

	res2, raw2 := postCreate(t, srv.URL, withCreateChecklist(`{"id":"`+id+`","title":"second","priority":"medium"}`))
	if res2.StatusCode != http.StatusConflict {
		t.Fatalf("second create status %d (want 409) body=%s", res2.StatusCode, raw2)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw2, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw2)
	}
	if errBody.Error != "task id already exists" {
		t.Fatalf("error=%q want %q (docs/api.md)", errBody.Error, "task id already exists")
	}
}

// TestHTTP_createTask_defaults pins the documented default: omitted/empty
// `status` falls back to `ready`.
func TestHTTP_createTask_defaults(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	t.Run("statusOmittedDefaultsToReady", func(t *testing.T) {
		res, raw := postCreate(t, srv.URL, withCreateChecklist(`{"title":"d1","priority":"medium"}`))
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("status %d body=%s", res.StatusCode, raw)
		}
		var got domain.Task
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got.Status != domain.StatusReady {
			t.Fatalf("status=%q want %q (docs/api.md default)", got.Status, domain.StatusReady)
		}
	})
}

// TestHTTP_createTask_doneStatusWithCriteriaRejected pins that creating a task
// as done while checklist items are not verified complete returns 400.
func TestHTTP_createTask_doneStatusWithCriteriaRejected(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, raw := postCreate(t, srv.URL, withCreateChecklist(`{"title":"born-done","priority":"medium","status":"done"}`))
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d (want 400; criteria exist but are not verified) body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if !strings.Contains(errBody.Error, "all checklist items must be done") {
		t.Fatalf("error=%q want checklist completion guard", errBody.Error)
	}
}

// TestHTTP_createTask_201ResponseShape pins the flat domain.Task 201 envelope.
func TestHTTP_createTask_201ResponseShape(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, raw := postCreate(t, srv.URL, withCreateChecklist(`{"title":"leaf","priority":"medium"}`))
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode top: %v body=%s", err, raw)
	}
	if _, ok := top["children"]; ok {
		t.Fatalf("flat task row should not serialize \"children\": %s", raw)
	}
	for _, k := range []string{"id", "title", "status", "priority"} {
		if _, ok := top[k]; !ok {
			t.Fatalf("missing top-level %q: %s", k, raw)
		}
	}
}

// TestHTTP_createTask_acceptsPickupNotBefore_overrideGlobalDelay pins the
// Stage 2 contract from .cursor/plans/task_scheduling_e74b47fe.plan.md: an
// explicit `pickup_not_before` on POST takes precedence over the global
// `agent_pickup_delay_seconds` setting. The default delay is 5s in
// DefaultAppSettings; a PATCH-and-POST round-trip with an explicit time at
// now+1h must surface the explicit time on the wire (NOT now+5s) so operator
// intent always wins over the system-wide deferral.
func TestHTTP_createTask_acceptsPickupNotBefore_overrideGlobalDelay(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	want := time.Now().UTC().Add(1 * time.Hour).Truncate(time.Second)
	body := withCreateChecklist(`{"title":"explicit","priority":"medium","pickup_not_before":"` + want.Format(time.RFC3339) + `"}`)
	res, raw := postCreate(t, srv.URL, body)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d (want 201) body=%s", res.StatusCode, raw)
	}
	var got domain.Task
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.PickupNotBefore == nil {
		t.Fatalf("pickup_not_before nil; explicit value should win over global delay")
	}
	if !got.PickupNotBefore.Equal(want) {
		t.Fatalf("pickup_not_before=%s want %s (explicit wins over global delay)",
			got.PickupNotBefore.UTC().Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

// TestHTTP_createTask_rejectsBadPickupNotBefore pins the per-stage 400 strings
// for malformed scheduling input on POST. Each subtest drives a distinct
// rejection path so a future refactor breaks loudly here, in lockstep with
// docs/data-model.md.
func TestHTTP_createTask_rejectsBadPickupNotBefore(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	cases := []struct {
		name       string
		body       string
		wantSubstr string
	}{
		{
			name:       "malformed",
			body:       withCreateChecklist(`{"title":"x","priority":"medium","pickup_not_before":"yesterday"}`),
			wantSubstr: "pickup_not_before must be RFC3339",
		},
		{
			name:       "pre2000Sentinel",
			body:       withCreateChecklist(`{"title":"x","priority":"medium","pickup_not_before":"1999-12-31T23:59:59Z"}`),
			wantSubstr: "pickup_not_before must be on or after 2000-01-01",
		},
		{
			name:       "emptyStringOnCreate",
			body:       withCreateChecklist(`{"title":"x","priority":"medium","pickup_not_before":""}`),
			wantSubstr: "pickup_not_before must not be empty on create",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := postCreate(t, srv.URL, tc.body)
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

// TestHTTP_createTask_pickupNotBefore_pastIsAllowed pins the locked decision
// that a past `pickup_not_before` is NOT a validation error. Operators
// recovering from a typo (or pasting back an already-elapsed schedule) get a
// no-op deferral that the worker treats as immediately eligible — see the
// Stage 0 `shouldNotifyReadyNow` gate in pkgs/tasks/store/facade_tasks.go.
func TestHTTP_createTask_pickupNotBefore_pastIsAllowed(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	past := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	body := withCreateChecklist(`{"title":"past","priority":"medium","pickup_not_before":"` + past.Format(time.RFC3339) + `"}`)
	res, raw := postCreate(t, srv.URL, body)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d (want 201; past pickup is no-op deferral) body=%s", res.StatusCode, raw)
	}
	var got domain.Task
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.PickupNotBefore == nil || !got.PickupNotBefore.Equal(past) {
		t.Fatalf("pickup_not_before=%v want %s (verbatim past time)", got.PickupNotBefore, past.Format(time.RFC3339))
	}
}

// TestHTTP_createTask_publishesTaskCreated pins the documented SSE side effect.
func TestHTTP_createTask_publishesTaskCreated(t *testing.T) {
	srv, _, hub := newSSETriggerServer(t)
	defer srv.Close()

	ch, unsub := hub.Subscribe()
	defer unsub()

	res, raw := postCreate(t, srv.URL, withCreateChecklist(`{"title":"child","priority":"medium"}`))
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var task domain.Task
	if err := json.Unmarshal(raw, &task); err != nil {
		t.Fatal(err)
	}
	got := summarize(drainSSE(t, ch, 1, 2*time.Second))
	mustEqualEvents(t, "POST /tasks", got, []string{"task_created:" + task.ID})
}

// TestHTTP_createTask_checklistItemsPersisted pins atomic checklist insert on create.
func TestHTTP_createTask_checklistItemsPersisted(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, raw := postCreateRaw(t, srv.URL, `{"title":"with-criteria","priority":"medium","checklist_items":[{"text":"Ship feature"},{"text":"Add tests"}]}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var task domain.Task
	if err := json.Unmarshal(raw, &task); err != nil {
		t.Fatal(err)
	}
	clRes, err := http.Get(srv.URL + "/tasks/" + task.ID + "/checklist")
	if err != nil {
		t.Fatal(err)
	}
	clRaw, _ := io.ReadAll(clRes.Body)
	_ = clRes.Body.Close()
	if clRes.StatusCode != http.StatusOK {
		t.Fatalf("checklist GET status %d body=%s", clRes.StatusCode, clRaw)
	}
	var clBody struct {
		Items []struct {
			Text string `json:"text"`
		} `json:"items"`
	}
	if err := json.Unmarshal(clRaw, &clBody); err != nil {
		t.Fatalf("decode checklist: %v body=%s", err, clRaw)
	}
	if len(clBody.Items) != 2 {
		t.Fatalf("items=%d want 2: %s", len(clBody.Items), clRaw)
	}
	if clBody.Items[0].Text != "Ship feature" || clBody.Items[1].Text != "Add tests" {
		t.Fatalf("items=%v", clBody.Items)
	}
}
