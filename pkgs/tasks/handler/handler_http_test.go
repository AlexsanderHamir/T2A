package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/google/uuid"
)

func TestHTTP_create_and_list(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"hello","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		b, errRead := io.ReadAll(res.Body)
		if errRead != nil {
			t.Fatal(errRead)
		}
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	var created domain.Task
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Title != "hello" {
		t.Fatalf("title %q", created.Title)
	}
	if _, err := uuid.Parse(created.ID); err != nil {
		t.Fatalf("id not a UUID: %q", created.ID)
	}

	res2, err := http.Get(srv.URL + "/tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("list status %d", res2.StatusCode)
	}
	var payload struct {
		Tasks []domain.Task `json:"tasks"`
	}
	if err := json.NewDecoder(res2.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Tasks) != 1 {
		t.Fatalf("len tasks %d", len(payload.Tasks))
	}
}

func TestHTTP_list_keyset_after_id(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	id1 := "20000000-0000-4000-8000-000000000001"
	id2 := "20000000-0000-4000-8000-000000000002"
	id3 := "20000000-0000-4000-8000-000000000003"
	for _, id := range []string{id1, id2, id3} {
		res, err := http.Post(srv.URL+"/tasks", "application/json",
			strings.NewReader(`{"id":"`+id+`","title":"x","priority":"medium"}`))
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("create %s: %d", id, res.StatusCode)
		}
	}
	type idRow struct {
		ID string `json:"id"`
	}
	var page1 struct {
		Tasks   []idRow `json:"tasks"`
		HasMore bool    `json:"has_more"`
	}
	res, err := http.Get(srv.URL + "/tasks?limit=2")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&page1); err != nil {
		t.Fatal(err)
	}
	if !page1.HasMore || len(page1.Tasks) != 2 || page1.Tasks[0].ID != id1 || page1.Tasks[1].ID != id2 {
		t.Fatalf("page1 %+v", page1)
	}
	res2, err := http.Get(srv.URL + "/tasks?limit=2&after_id=" + id2)
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	var page2 struct {
		Tasks   []idRow `json:"tasks"`
		HasMore bool    `json:"has_more"`
	}
	if err := json.NewDecoder(res2.Body).Decode(&page2); err != nil {
		t.Fatal(err)
	}
	if page2.HasMore || len(page2.Tasks) != 1 || page2.Tasks[0].ID != id3 {
		t.Fatalf("page2 %+v", page2)
	}
}

func TestHTTP_tasks_stats_global_counts(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	for _, body := range []string{
		`{"id":"20000000-0000-4000-8000-000000000001","title":"ready one","priority":"medium","status":"ready"}`,
		`{"title":"critical one","priority":"critical","status":"running"}`,
		`{"title":"critical ready","priority":"critical","status":"ready"}`,
		`{"title":"subtask","priority":"low","status":"ready","parent_id":"20000000-0000-4000-8000-000000000001"}`,
	} {
		res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("create status %d", res.StatusCode)
		}
	}

	res, err := http.Get(srv.URL + "/tasks/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("stats %d", res.StatusCode)
	}
	var got struct {
		Total      int64            `json:"total"`
		Ready      int64            `json:"ready"`
		Critical   int64            `json:"critical"`
		ByStatus   map[string]int64 `json:"by_status"`
		ByPriority map[string]int64 `json:"by_priority"`
		ByScope    map[string]int64 `json:"by_scope"`
	}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 4 || got.Ready != 3 || got.Critical != 2 {
		t.Fatalf("stats %+v", got)
	}
	if got.ByStatus["ready"] != 3 || got.ByStatus["running"] != 1 {
		t.Fatalf("stats by_status %+v", got.ByStatus)
	}
	if got.ByPriority["critical"] != 2 || got.ByPriority["medium"] != 1 {
		t.Fatalf("stats by_priority %+v", got.ByPriority)
	}
	if got.ByScope["parent"] != 3 || got.ByScope["subtask"] != 1 {
		t.Fatalf("stats by_scope %+v", got.ByScope)
	}
}

func TestHTTP_create_rejects_unknown_field(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"x","nope":1,"priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_get_not_found(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/tasks/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", res.StatusCode)
	}
	var errBody struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Error != "not found" {
		t.Fatalf("error body %q", errBody.Error)
	}
}

func TestHTTP_patch_and_delete(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"t","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(res.Body)
	if cerr := res.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create %d %s", res.StatusCode, b)
	}
	var created domain.Task
	if err := json.Unmarshal(b, &created); err != nil {
		t.Fatal(err)
	}

	patchBody := `{"status":"running"}`
	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+created.ID, strings.NewReader(patchBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	patchBytes, err := io.ReadAll(res2.Body)
	if cerr := res2.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("patch %d %s", res2.StatusCode, patchBytes)
	}
	var updated domain.Task
	if err := json.Unmarshal(patchBytes, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status != domain.StatusRunning {
		t.Fatalf("status %s", updated.Status)
	}

	reqDel, err := http.NewRequest(http.MethodDelete, srv.URL+"/tasks/"+created.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	res3, err := http.DefaultClient.Do(reqDel)
	if err != nil {
		t.Fatal(err)
	}
	defer res3.Body.Close()
	if res3.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status %d", res3.StatusCode)
	}
}

func TestHTTP_list_bad_limit(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/tasks?limit=999")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var out jsonErrorBody
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != "limit must be integer 0..200" {
		t.Fatalf("error %q", out.Error)
	}
}

func TestHTTP_get_task_rejects_overlong_path_id(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	long := strings.Repeat("a", maxTaskPathIDBytes+1)
	res, err := http.Get(srv.URL + "/tasks/" + long)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("overlong id: status %d want %d", res.StatusCode, http.StatusBadRequest)
	}
}

func TestHTTP_list_query_validation_error_messages(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	getErr := func(rawURL string) string {
		t.Helper()
		res, err := http.Get(rawURL)
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(res.Body)
		if cerr := res.Body.Close(); cerr != nil {
			t.Fatal(cerr)
		}
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status %d body %s", rawURL, res.StatusCode, b)
		}
		var out jsonErrorBody
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return out.Error
	}

	base := srv.URL + "/tasks"
	if got := getErr(base + "?limit=999"); got != "limit must be integer 0..200" {
		t.Fatalf("limit 999: %q", got)
	}
	if got := getErr(base + "?limit=nope"); got != "limit must be integer 0..200" {
		t.Fatalf("limit nope: %q", got)
	}
	if got := getErr(base + "?offset=-1"); got != "offset must be non-negative integer" {
		t.Fatalf("offset -1: %q", got)
	}
	id := "11111111-1111-4111-8111-111111111111"
	if got := getErr(base + "?after_id=" + id + "&offset=0"); got != "offset cannot be used with after_id" {
		t.Fatalf("after_id+offset: %q", got)
	}
	if got := getErr(base + "?after_id=not-a-uuid"); got != "after_id must be a UUID" {
		t.Fatalf("bad uuid: %q", got)
	}
	long := strings.Repeat("1", maxListIntQueryParamBytes+1)
	if got := getErr(base + "?limit=" + long); got != "limit value too long" {
		t.Fatalf("long limit: %q", got)
	}
	if got := getErr(base + "?offset=" + long); got != "offset value too long" {
		t.Fatalf("long offset: %q", got)
	}
	longAfter := strings.Repeat("a", maxListAfterIDParamBytes+1)
	if got := getErr(base + "?after_id=" + longAfter); got != "after_id too long" {
		t.Fatalf("long after_id: %q", got)
	}
}

func TestHTTP_create_rejects_empty_title(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"   ","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_create_rejects_invalid_status(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	body := `{"title":"ok","status":"not_a_real_status","priority":"medium"}`
	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_create_rejects_invalid_task_type(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	body := `{"title":"ok","priority":"medium","task_type":"not_a_real_type"}`
	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_evaluate_rejects_invalid_task_type(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	body := `{"id":"draft-x","title":"ok","priority":"medium","task_type":"not_a_real_type"}`
	res, err := http.Post(srv.URL+"/tasks/evaluate", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_patch_rejects_invalid_task_type(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	createRes, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"task","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	createBody, err := io.ReadAll(createRes.Body)
	if cerr := createRes.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if createRes.StatusCode != http.StatusCreated {
		t.Fatalf("create %d %s", createRes.StatusCode, createBody)
	}
	var created domain.Task
	if err := json.Unmarshal(createBody, &created); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+created.ID, strings.NewReader(`{"task_type":"not_a_real_type"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_create_rejects_missing_priority(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"ok"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_patch_json_null_leaves_field_unchanged(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"t","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(res.Body)
	if cerr := res.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create %d %s", res.StatusCode, b)
	}
	var created domain.Task
	if err := json.Unmarshal(b, &created); err != nil {
		t.Fatal(err)
	}
	if created.Priority != domain.PriorityMedium {
		t.Fatalf("priority: %s", created.Priority)
	}

	// JSON null must behave like omitted: do not clear status; still apply priority.
	patchBody := `{"status":null,"priority":"high"}`
	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+created.ID, strings.NewReader(patchBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	patchBytes, err := io.ReadAll(res2.Body)
	if cerr := res2.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("patch %d %s", res2.StatusCode, patchBytes)
	}
	var updated domain.Task
	if err := json.Unmarshal(patchBytes, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status != domain.StatusReady {
		t.Fatalf("status should stay ready, got %s", updated.Status)
	}
	if updated.Priority != domain.PriorityHigh {
		t.Fatalf("priority: %s", updated.Priority)
	}
}

func TestHTTP_patch_rejects_empty_patch(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"x","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(res.Body)
	if cerr := res.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create %d %s", res.StatusCode, b)
	}
	var created domain.Task
	if err := json.Unmarshal(b, &created); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+created.ID, strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusBadRequest {
		t.Fatalf("patch status %d", res2.StatusCode)
	}
}

func TestHTTP_patch_not_found(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/00000000-0000-0000-0000-000000000001",
		strings.NewReader(`{"status":"running"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_delete_not_found(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/tasks/00000000-0000-0000-0000-000000000002", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_list_limit_200_ok(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/tasks?limit=200&offset=0")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_list_limit_zero_reports_coerced_default(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/tasks?limit=0&offset=0")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		Limit int `json:"limit"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Limit != 50 {
		t.Fatalf("limit %d want 50", body.Limit)
	}
}

func TestHTTP_method_not_allowed_routes_only_registered_verbs(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPut, srv.URL+"/tasks", bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("PUT /tasks: status %d want %d", res.StatusCode, http.StatusMethodNotAllowed)
	}
}
