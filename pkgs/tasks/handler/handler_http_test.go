package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/internal/testdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/google/uuid"
)

func newTaskTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	db := testdb.OpenSQLite(t)
	h := NewHandler(store.NewStore(db), NewSSEHub(), nil)
	return httptest.NewServer(h)
}

func newTaskTestServerWithStore(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	db := testdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil)
	return httptest.NewServer(h), st
}

func newTaskTestServerWithRepo(t *testing.T, repoDir string) *httptest.Server {
	t.Helper()
	db := testdb.OpenSQLite(t)
	r, err := repo.OpenRoot(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(store.NewStore(db), NewSSEHub(), r)
	return httptest.NewServer(h)
}

func TestHTTP_get_task_events(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"hello","priority":"medium"}`))
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

	res2, err := http.Get(srv.URL + "/tasks/" + created.ID + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("events status %d", res2.StatusCode)
	}
	var payload struct {
		TaskID string `json:"task_id"`
		Events []struct {
			Seq  int64  `json:"seq"`
			Type string `json:"type"`
		} `json:"events"`
	}
	if err := json.NewDecoder(res2.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.TaskID != created.ID || len(payload.Events) < 1 || payload.Events[0].Type != string(domain.EventTaskCreated) {
		t.Fatalf("payload %#v", payload)
	}
}

func TestHTTP_patch_task_event_user_response(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	ctx := context.Background()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"hello","priority":"medium"}`))
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
	if err := st.AppendTaskEvent(ctx, created.ID, domain.EventApprovalRequested, domain.ActorAgent, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	reqBadType, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+created.ID+"/events/1", strings.NewReader(`{"user_response":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	reqBadType.Header.Set("Content-Type", "application/json")
	resBadType, err := http.DefaultClient.Do(reqBadType)
	if err != nil {
		t.Fatal(err)
	}
	if cerr := resBadType.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if resBadType.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong event type want 400, got %d", resBadType.StatusCode)
	}

	reqAgent, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+created.ID+"/events/2", strings.NewReader(`{"user_response":"Please confirm"}`))
	if err != nil {
		t.Fatal(err)
	}
	reqAgent.Header.Set("Content-Type", "application/json")
	reqAgent.Header.Set("X-Actor", "agent")
	resAgent, err := http.DefaultClient.Do(reqAgent)
	if err != nil {
		t.Fatal(err)
	}
	agentBody, err := io.ReadAll(resAgent.Body)
	if cerr := resAgent.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if resAgent.StatusCode != http.StatusOK {
		t.Fatalf("agent patch want 200, got %d %s", resAgent.StatusCode, agentBody)
	}
	var agentOut struct {
		ResponseThread []struct {
			By   string `json:"by"`
			Body string `json:"body"`
		} `json:"response_thread"`
	}
	if err := json.Unmarshal(agentBody, &agentOut); err != nil {
		t.Fatal(err)
	}
	if len(agentOut.ResponseThread) != 1 || agentOut.ResponseThread[0].By != "agent" || agentOut.ResponseThread[0].Body != "Please confirm" {
		t.Fatalf("agent thread %#v", agentOut.ResponseThread)
	}

	reqOK, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+created.ID+"/events/2", strings.NewReader(`{"user_response":"LGTM"}`))
	if err != nil {
		t.Fatal(err)
	}
	reqOK.Header.Set("Content-Type", "application/json")
	resOK, err := http.DefaultClient.Do(reqOK)
	if err != nil {
		t.Fatal(err)
	}
	defer resOK.Body.Close()
	if resOK.StatusCode != http.StatusOK {
		tBody, _ := io.ReadAll(resOK.Body)
		t.Fatalf("patch %d %s", resOK.StatusCode, tBody)
	}
	var one struct {
		Seq            int64      `json:"seq"`
		UserResponse   *string    `json:"user_response"`
		UserResponseAt *time.Time `json:"user_response_at"`
		ResponseThread []struct {
			By   string `json:"by"`
			Body string `json:"body"`
		} `json:"response_thread"`
	}
	if err := json.NewDecoder(resOK.Body).Decode(&one); err != nil {
		t.Fatal(err)
	}
	if one.Seq != 2 || one.UserResponse == nil || *one.UserResponse != "LGTM" {
		t.Fatalf("payload %#v", one)
	}
	if one.UserResponseAt == nil {
		t.Fatal("expected user_response_at on PATCH response")
	}
	if len(one.ResponseThread) != 2 {
		t.Fatalf("want 2 thread entries, got %#v", one.ResponseThread)
	}

	resList, err := http.Get(srv.URL + "/tasks/" + created.ID + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resList.Body.Close()
	var listPayload struct {
		Events []struct {
			Seq            int64   `json:"seq"`
			UserResponse   *string `json:"user_response"`
			ResponseThread []struct {
				By   string `json:"by"`
				Body string `json:"body"`
			} `json:"response_thread"`
		} `json:"events"`
	}
	if err := json.NewDecoder(resList.Body).Decode(&listPayload); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range listPayload.Events {
		if e.Seq == 2 && e.UserResponse != nil && *e.UserResponse == "LGTM" && len(e.ResponseThread) == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("list missing user_response or thread: %#v", listPayload.Events)
	}
}

func TestHTTP_get_task_event(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"hello","priority":"medium"}`))
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

	resOK, err := http.Get(srv.URL + "/tasks/" + created.ID + "/events/1")
	if err != nil {
		t.Fatal(err)
	}
	defer resOK.Body.Close()
	if resOK.StatusCode != http.StatusOK {
		tBody, _ := io.ReadAll(resOK.Body)
		t.Fatalf("event status %d %s", resOK.StatusCode, tBody)
	}
	var one struct {
		TaskID string `json:"task_id"`
		Seq    int64  `json:"seq"`
		Type   string `json:"type"`
	}
	if err := json.NewDecoder(resOK.Body).Decode(&one); err != nil {
		t.Fatal(err)
	}
	if one.TaskID != created.ID || one.Seq != 1 || one.Type != string(domain.EventTaskCreated) {
		t.Fatalf("payload %#v", one)
	}

	res404, err := http.Get(srv.URL + "/tasks/" + created.ID + "/events/99")
	if err != nil {
		t.Fatal(err)
	}
	defer res404.Body.Close()
	if res404.StatusCode != http.StatusNotFound {
		t.Fatalf("missing seq want 404, got %d", res404.StatusCode)
	}

	resBad, err := http.Get(srv.URL + "/tasks/" + created.ID + "/events/0")
	if err != nil {
		t.Fatal(err)
	}
	defer resBad.Body.Close()
	if resBad.StatusCode != http.StatusBadRequest {
		t.Fatalf("seq 0 want 400, got %d", resBad.StatusCode)
	}
}

func TestHTTP_get_task_events_paged_cursor(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"paged","priority":"medium"}`))
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

	patchBody := `{"title":"paged two"}`
	reqPatch, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+created.ID, strings.NewReader(patchBody))
	if err != nil {
		t.Fatal(err)
	}
	reqPatch.Header.Set("Content-Type", "application/json")
	resPatch, err := http.DefaultClient.Do(reqPatch)
	if err != nil {
		t.Fatal(err)
	}
	if cerr := resPatch.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if resPatch.StatusCode != http.StatusOK {
		t.Fatalf("patch %d", resPatch.StatusCode)
	}

	resOff, err := http.Get(srv.URL + "/tasks/" + created.ID + "/events?limit=5&offset=0")
	if err != nil {
		t.Fatal(err)
	}
	defer resOff.Body.Close()
	if resOff.StatusCode != http.StatusBadRequest {
		t.Fatalf("offset with events want 400, got %d", resOff.StatusCode)
	}

	res2, err := http.Get(srv.URL + "/tasks/" + created.ID + "/events?limit=1")
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("events %d", res2.StatusCode)
	}
	var payload struct {
		TaskID          string `json:"task_id"`
		Limit           int    `json:"limit"`
		Total           int64  `json:"total"`
		HasMoreOlder    bool   `json:"has_more_older"`
		HasMoreNewer    bool   `json:"has_more_newer"`
		RangeStart      int64  `json:"range_start"`
		RangeEnd        int64  `json:"range_end"`
		ApprovalPending bool   `json:"approval_pending"`
		Events          []struct {
			Seq int64 `json:"seq"`
		} `json:"events"`
	}
	if err := json.NewDecoder(res2.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.TaskID != created.ID || payload.Limit != 1 || payload.Total != 2 {
		t.Fatalf("payload %#v", payload)
	}
	if len(payload.Events) != 1 || !payload.HasMoreOlder || payload.HasMoreNewer {
		t.Fatalf("head page of 1: %#v", payload)
	}
	if payload.RangeStart != 1 || payload.RangeEnd != 1 {
		t.Fatalf("range %d-%d", payload.RangeStart, payload.RangeEnd)
	}
	newestSeq := payload.Events[0].Seq

	res3, err := http.Get(srv.URL + "/tasks/" + created.ID + "/events?limit=10&before_seq=" + strconv.FormatInt(newestSeq, 10))
	if err != nil {
		t.Fatal(err)
	}
	defer res3.Body.Close()
	if res3.StatusCode != http.StatusOK {
		t.Fatalf("before page %d", res3.StatusCode)
	}
	var payload2 struct {
		HasMoreOlder bool  `json:"has_more_older"`
		HasMoreNewer bool  `json:"has_more_newer"`
		RangeStart   int64 `json:"range_start"`
		RangeEnd     int64 `json:"range_end"`
		Events       []struct {
			Seq int64 `json:"seq"`
		} `json:"events"`
	}
	if err := json.NewDecoder(res3.Body).Decode(&payload2); err != nil {
		t.Fatal(err)
	}
	if len(payload2.Events) != 1 || payload2.HasMoreOlder || !payload2.HasMoreNewer {
		t.Fatalf("older page: %#v", payload2)
	}
	if payload2.RangeStart != 2 || payload2.RangeEnd != 2 {
		t.Fatalf("range %d-%d", payload2.RangeStart, payload2.RangeEnd)
	}
}

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

func TestHTTP_health(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	for _, path := range []string{"/health", "/health/live"} {
		t.Run(path, func(t *testing.T) {
			res, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusOK {
				t.Fatalf("status %d", res.StatusCode)
			}
			var body struct {
				Status string `json:"status"`
			}
			if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Status != "ok" {
				t.Fatalf("status field %q", body.Status)
			}
		})
	}
}

func TestHTTP_health_ready_ok(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/health/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" || body.Checks["database"] != "ok" {
		t.Fatalf("body %+v", body)
	}
}

func TestHTTP_health_ready_degraded_when_db_closed(t *testing.T) {
	db := testdb.OpenSQLite(t)
	st := store.NewStore(db)
	srv := httptest.NewServer(NewHandler(st, NewSSEHub(), nil))
	defer srv.Close()

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	res, err := http.Get(srv.URL + "/health/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "degraded" || body.Checks["database"] != "fail" {
		t.Fatalf("body %+v", body)
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

func TestHTTP_repo_search_and_create_rejects_bad_file_mention(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(p, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/repo/search?q=note")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("search status %d", res.StatusCode)
	}
	var searchPayload struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(res.Body).Decode(&searchPayload); err != nil {
		t.Fatal(err)
	}
	if len(searchPayload.Paths) != 1 || searchPayload.Paths[0] != "note.txt" {
		t.Fatalf("paths %#v", searchPayload.Paths)
	}

	res2, err := http.Post(srv.URL+"/tasks", "application/json",
		strings.NewReader(`{"title":"t","initial_prompt":"@nope.txt","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res2.Body)
		t.Fatalf("create status %d body %s", res2.StatusCode, b)
	}

	res3, err := http.Post(srv.URL+"/tasks", "application/json",
		strings.NewReader(`{"title":"t2","initial_prompt":"@note.txt(1-2)","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res3.Body.Close()
	if res3.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res3.Body)
		t.Fatalf("create valid mention status %d body %s", res3.StatusCode, b)
	}
}

func TestHTTP_repo_validate_range_ok(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(p, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/repo/validate-range?path=note.txt&start=1&end=2")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var payload struct {
		OK        bool `json:"ok"`
		LineCount int  `json:"line_count"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !payload.OK || payload.LineCount != 3 {
		t.Fatalf("ok=%v line_count=%d", payload.OK, payload.LineCount)
	}
}

func TestHTTP_repo_validate_range_missing_params(t *testing.T) {
	dir := t.TempDir()
	srv := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/repo/validate-range")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_patch_checklist_item_text_updates_and_returns_items(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	ctx := context.Background()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"chk","priority":"medium"}`))
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
	it, err := st.AddChecklistItem(ctx, created.ID, "alpha", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPatch,
		srv.URL+"/tasks/"+created.ID+"/checklist/items/"+it.ID,
		strings.NewReader(`{"text":"beta"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resPatch, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	patchBody, err := io.ReadAll(resPatch.Body)
	if cerr := resPatch.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if resPatch.StatusCode != http.StatusOK {
		t.Fatalf("patch %d %s", resPatch.StatusCode, patchBody)
	}
	var out struct {
		Items []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"items"`
	}
	if err := json.Unmarshal(patchBody, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || out.Items[0].Text != "beta" {
		t.Fatalf("items %#v", out.Items)
	}
}

func TestHTTP_patch_checklist_item_done_rejects_default_user_actor(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	ctx := context.Background()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"chk","priority":"medium"}`))
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
	it, err := st.AddChecklistItem(ctx, created.ID, "c", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPatch,
		srv.URL+"/tasks/"+created.ID+"/checklist/items/"+it.ID,
		strings.NewReader(`{"done":true}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resPatch, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resPatch.Body.Close()
	if resPatch.StatusCode != http.StatusBadRequest {
		t.Fatalf("user done patch want 400, got %d", resPatch.StatusCode)
	}
}

func TestHTTP_patch_checklist_item_rejects_text_and_done_together(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	ctx := context.Background()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"chk","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(res.Body)
	if cerr := res.Body.Close(); cerr != nil {
		t.Fatal(err)
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
	it, err := st.AddChecklistItem(ctx, created.ID, "c", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPatch,
		srv.URL+"/tasks/"+created.ID+"/checklist/items/"+it.ID,
		strings.NewReader(`{"text":"x","done":true}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resPatch, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resPatch.Body.Close()
	if resPatch.StatusCode != http.StatusBadRequest {
		t.Fatalf("both fields want 400, got %d", resPatch.StatusCode)
	}
}

func TestHTTP_repo_validate_range_invalid_start_end(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/repo/validate-range?path=a.txt&start=nope&end=1")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}
