package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// postCreate centralizes the documented POST /tasks round-trip so the
// table-driven 400 string subtests stay focused on the assertion side.
// (`mustCreateTask` from handler_http_patch_contract_test.go asserts a
// specific success path; this helper keeps the response intact so error-path
// subtests can read it.)
func postCreate(t *testing.T, baseURL, jsonBody string) (*http.Response, []byte) {
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
// string from docs/API-HTTP.md against the live handler. Each subtest drives a
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
		{"titleWhitespace", `{"title":"   ","priority":"medium"}`, "title required"},
		{"priorityMissing", `{"title":"ok"}`, "priority required"},
		{"priorityEmpty", `{"title":"ok","priority":""}`, "priority required"},
		{"priorityInvalid", `{"title":"ok","priority":"nope"}`, "priority"},
		{"statusInvalid", `{"title":"ok","priority":"medium","status":"nope"}`, "status"},
		{"taskTypeInvalid", `{"title":"ok","priority":"medium","task_type":"nope"}`, "task_type"},
		{"checklistInheritWithoutParent", `{"title":"ok","priority":"medium","checklist_inherit":true}`, "checklist_inherit requires parent_id"},
		{"checklistInheritWithEmptyParent", `{"title":"ok","priority":"medium","checklist_inherit":true,"parent_id":""}`, "checklist_inherit requires parent_id"},
		{"parentNotFound", `{"title":"ok","priority":"medium","parent_id":"00000000-0000-0000-0000-000000000099"}`, "parent not found"},
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
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q (docs/API-HTTP.md)", errBody.Error, tc.want)
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
	res1, raw1 := postCreate(t, srv.URL, `{"id":"`+id+`","title":"first","priority":"medium"}`)
	if res1.StatusCode != http.StatusCreated {
		t.Fatalf("first create status %d body=%s", res1.StatusCode, raw1)
	}

	res2, raw2 := postCreate(t, srv.URL, `{"id":"`+id+`","title":"second","priority":"medium"}`)
	if res2.StatusCode != http.StatusConflict {
		t.Fatalf("second create status %d (want 409) body=%s", res2.StatusCode, raw2)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw2, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw2)
	}
	if errBody.Error != "task id already exists" {
		t.Fatalf("error=%q want %q (docs/API-HTTP.md)", errBody.Error, "task id already exists")
	}
}

// TestHTTP_createTask_defaults pins the documented defaults: omitted/empty
// `status` falls back to `ready`; omitted/empty `task_type` falls back to
// `general`. Both are covered indirectly by happy-path tests but neither is
// pinned to the bare enum value in a contract-style test today.
func TestHTTP_createTask_defaults(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	t.Run("statusOmittedDefaultsToReady", func(t *testing.T) {
		res, raw := postCreate(t, srv.URL, `{"title":"d1","priority":"medium"}`)
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("status %d body=%s", res.StatusCode, raw)
		}
		var got domain.Task
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got.Status != domain.StatusReady {
			t.Fatalf("status=%q want %q (docs/API-HTTP.md default)", got.Status, domain.StatusReady)
		}
	})

	t.Run("taskTypeOmittedDefaultsToGeneral", func(t *testing.T) {
		res, raw := postCreate(t, srv.URL, `{"title":"d2","priority":"medium"}`)
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("status %d body=%s", res.StatusCode, raw)
		}
		var got domain.Task
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got.TaskType != domain.TaskTypeGeneral {
			t.Fatalf("task_type=%q want %q (docs/API-HTTP.md default)", got.TaskType, domain.TaskTypeGeneral)
		}
	})

	t.Run("taskTypeEmptyStringDefaultsToGeneral", func(t *testing.T) {
		res, raw := postCreate(t, srv.URL, `{"title":"d3","priority":"medium","task_type":""}`)
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("status %d body=%s", res.StatusCode, raw)
		}
		var got domain.Task
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got.TaskType != domain.TaskTypeGeneral {
			t.Fatalf("task_type=%q want %q (empty string falls through to default)", got.TaskType, domain.TaskTypeGeneral)
		}
	})
}

// TestHTTP_createTask_emptyParentIDIsSilentlyOrphan pins the (asymmetric)
// documented behavior that POST treats `"parent_id":""` (and whitespace) as
// **no parent**, while PATCH rejects the same payload with `parent_id must not
// be empty`. Catches a regression that would mistakenly add a "parent_id must
// not be empty" check on the create path and break clients that re-use a
// single payload shape across create/patch.
func TestHTTP_createTask_emptyParentIDIsSilentlyOrphan(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	cases := []struct {
		name string
		body string
	}{
		{"emptyString", `{"title":"orphan-empty","priority":"medium","parent_id":""}`},
		{"whitespace", `{"title":"orphan-ws","priority":"medium","parent_id":"   "}`},
		{"jsonNull", `{"title":"orphan-null","priority":"medium","parent_id":null}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := postCreate(t, srv.URL, tc.body)
			if res.StatusCode != http.StatusCreated {
				t.Fatalf("status %d (want 201) body=%s", res.StatusCode, raw)
			}
			var got domain.Task
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if got.ParentID != nil && *got.ParentID != "" {
				t.Fatalf("parent_id=%q want nil/empty (silently orphaned)", *got.ParentID)
			}
		})
	}
}

// TestHTTP_createTask_doneStatusAllowedAtCreate pins the documented edge that
// `status:"done"` IS allowed at create time because a brand-new row has no
// descendants and no checklist items, so the precondition `validateCanMarkDone`
// trivially succeeds. This is the create-side dual of the PATCH "all subtasks
// must be done"/"all checklist items must be done" 400 strings, and a future
// refactor that hoists the precondition before the row exists would regress
// here.
func TestHTTP_createTask_doneStatusAllowedAtCreate(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, raw := postCreate(t, srv.URL, `{"title":"born-done","priority":"medium","status":"done"}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d (want 201; brand-new row has no descendants/checklist) body=%s", res.StatusCode, raw)
	}
	var got domain.Task
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.StatusDone {
		t.Fatalf("status=%q want %q", got.Status, domain.StatusDone)
	}
}

// TestHTTP_createTask_201ResponseShape pins the documented 201 envelope: the
// response is a `store.TaskNode` JSON (domain.Task fields plus optional
// `children` array). Per the doc table, `children` is **omitempty** — leaf
// rows omit the key entirely (NOT `"children":[]`); it appears only when the
// row has at least one descendant. This test will catch any future drift in
// the omitempty tag (e.g. dropping it would break the documented "missing key
// == empty subtree" contract that the web client relies on).
func TestHTTP_createTask_201ResponseShape(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	t.Run("leafOmitsChildrenKey", func(t *testing.T) {
		res, raw := postCreate(t, srv.URL, `{"title":"leaf","priority":"medium"}`)
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("status %d body=%s", res.StatusCode, raw)
		}
		var top map[string]json.RawMessage
		if err := json.Unmarshal(raw, &top); err != nil {
			t.Fatalf("decode top: %v body=%s", err, raw)
		}
		if _, ok := top["children"]; ok {
			t.Fatalf("leaf row should omit \"children\" key (omitempty); got %s", raw)
		}
		for _, k := range []string{"id", "title", "status", "priority", "task_type"} {
			if _, ok := top[k]; !ok {
				t.Fatalf("missing top-level %q (domain.Task field): %s", k, raw)
			}
		}
	})

	t.Run("parentSerializesChildren_grandchildOmitsKey", func(t *testing.T) {
		parentRes, parentRaw := postCreate(t, srv.URL, `{"title":"parent","priority":"medium"}`)
		if parentRes.StatusCode != http.StatusCreated {
			t.Fatalf("parent create status %d body=%s", parentRes.StatusCode, parentRaw)
		}
		var parent domain.Task
		if err := json.Unmarshal(parentRaw, &parent); err != nil {
			t.Fatal(err)
		}
		childRes, childRaw := postCreate(t, srv.URL,
			`{"title":"child","priority":"medium","parent_id":"`+parent.ID+`"}`)
		if childRes.StatusCode != http.StatusCreated {
			t.Fatalf("child create status %d body=%s", childRes.StatusCode, childRaw)
		}

		getRes, err := http.Get(srv.URL + "/tasks/" + parent.ID)
		if err != nil {
			t.Fatal(err)
		}
		getRaw, _ := io.ReadAll(getRes.Body)
		_ = getRes.Body.Close()
		if getRes.StatusCode != http.StatusOK {
			t.Fatalf("get parent status %d body=%s", getRes.StatusCode, getRaw)
		}

		// Parent JSON must carry "children" with one entry...
		var parentTop map[string]json.RawMessage
		if err := json.Unmarshal(getRaw, &parentTop); err != nil {
			t.Fatal(err)
		}
		childrenRaw, ok := parentTop["children"]
		if !ok {
			t.Fatalf("parent (with one child) must serialize \"children\" key: %s", getRaw)
		}
		var children []json.RawMessage
		if err := json.Unmarshal(childrenRaw, &children); err != nil {
			t.Fatal(err)
		}
		if len(children) != 1 {
			t.Fatalf("parent.children len=%d want 1", len(children))
		}
		// ...and the nested child (which is itself a leaf) must omit "children".
		var childTop map[string]json.RawMessage
		if err := json.Unmarshal(children[0], &childTop); err != nil {
			t.Fatal(err)
		}
		if _, ok := childTop["children"]; ok {
			t.Fatalf("nested leaf child should omit \"children\" key (omitempty); got %s", children[0])
		}

		// Defensive cross-check: store.TaskNode decode keeps the slice empty
		// when the JSON omits the key (Go zero-value), which is the documented
		// "missing key == empty subtree" contract for the web client.
		var tree store.TaskNode
		if err := json.Unmarshal(getRaw, &tree); err != nil {
			t.Fatal(err)
		}
		if len(tree.Children) != 1 {
			t.Fatalf("decoded parent.children len=%d want 1", len(tree.Children))
		}
		if tree.Children[0].Children != nil {
			t.Fatalf("decoded nested child.children=%v want nil (key omitted in JSON)", tree.Children[0].Children)
		}
	})
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
	body := `{"title":"explicit","priority":"medium","pickup_not_before":"` + want.Format(time.RFC3339) + `"}`
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
// docs/SCHEDULING.md.
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
			body:       `{"title":"x","priority":"medium","pickup_not_before":"yesterday"}`,
			wantSubstr: "pickup_not_before must be RFC3339",
		},
		{
			name:       "pre2000Sentinel",
			body:       `{"title":"x","priority":"medium","pickup_not_before":"1999-12-31T23:59:59Z"}`,
			wantSubstr: "pickup_not_before must be on or after 2000-01-01",
		},
		{
			name:       "emptyStringOnCreate",
			body:       `{"title":"x","priority":"medium","pickup_not_before":""}`,
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
	body := `{"title":"past","priority":"medium","pickup_not_before":"` + past.Format(time.RFC3339) + `"}`
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

// TestHTTP_createTask_publishesTaskCreatedAndParentTaskUpdated pins both
// documented SSE side effects of a successful POST. Mirrors the existing
// happy-path coverage in sse_trigger_surface_test.go but is colocated with the
// create-contract suite so doc readers find it next to the 400/409 strings.
func TestHTTP_createTask_publishesTaskCreatedAndParentTaskUpdated(t *testing.T) {
	srv, _, hub := newSSETriggerServer(t)
	defer srv.Close()

	parentID := mustCreateTask(t, srv.URL, `{"title":"parent","priority":"medium"}`)

	ch, unsub := hub.Subscribe()
	defer unsub()

	res, raw := postCreate(t, srv.URL,
		`{"title":"child","priority":"medium","parent_id":"`+parentID+`"}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var child domain.Task
	if err := json.Unmarshal(raw, &child); err != nil {
		t.Fatal(err)
	}
	got := summarize(drainSSE(t, ch, 2, 2*time.Second))
	mustEqualEvents(t, "POST /tasks (with parent)", got, []string{
		"task_created:" + child.ID,
		"task_updated:" + parentID,
	})
}
