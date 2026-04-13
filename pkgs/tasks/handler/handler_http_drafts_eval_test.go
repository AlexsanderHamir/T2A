package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestHTTP_evaluate_task_draft_creates_persisted_evaluation(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()

	body := `{
		"id":"draft-123",
		"title":"Improve mention parser reliability",
		"initial_prompt":"Handle nested mentions and malformed ranges with clear errors.",
		"priority":"high",
		"status":"ready",
		"checklist_inherit":false,
		"checklist_items":[{"text":"Add parser tests"},{"text":"Document edge cases"}]
	}`
	res, err := http.Post(srv.URL+"/tasks/evaluate", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	var out struct {
		EvaluationID string `json:"evaluation_id"`
		OverallScore int    `json:"overall_score"`
		Sections     []struct {
			Key         string   `json:"key"`
			Score       int      `json:"score"`
			Suggestions []string `json:"suggestions"`
		} `json:"sections"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.EvaluationID == "" {
		t.Fatal("expected evaluation_id")
	}
	if out.OverallScore < 0 || out.OverallScore > 100 {
		t.Fatalf("overall score %d out of range", out.OverallScore)
	}
	if len(out.Sections) < 4 {
		t.Fatalf("expected section scores, got %#v", out.Sections)
	}
	for _, s := range out.Sections {
		if s.Key == "" {
			t.Fatal("section key missing")
		}
		if s.Score < 0 || s.Score > 100 {
			t.Fatalf("section score %d out of range", s.Score)
		}
		if len(s.Suggestions) == 0 {
			t.Fatalf("expected suggestions for section %q", s.Key)
		}
	}

	records, err := st.ListDraftEvaluations(context.Background(), "draft-123", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 evaluation row, got %d", len(records))
	}
}

func TestHTTP_evaluate_task_draft_rejects_unknown_field(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks/evaluate", "application/json", strings.NewReader(`{"title":"x","priority":"medium","nope":1}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_create_task_attaches_previous_draft_evaluations(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()

	for i := 0; i < 2; i++ {
		resEval, err := http.Post(srv.URL+"/tasks/evaluate", "application/json", strings.NewReader(`{
			"id":"draft-link-http",
			"title":"Evaluate then create",
			"initial_prompt":"Attach previous evaluations on create",
			"priority":"high"
		}`))
		if err != nil {
			t.Fatal(err)
		}
		_ = resEval.Body.Close()
		if resEval.StatusCode != http.StatusCreated {
			t.Fatalf("evaluate status %d", resEval.StatusCode)
		}
	}

	resCreate, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{
		"title":"Finalized task",
		"initial_prompt":"from evaluated draft",
		"priority":"high",
		"draft_id":"draft-link-http"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resCreate.Body)
	if cerr := resCreate.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if resCreate.StatusCode != http.StatusCreated {
		t.Fatalf("create status %d body %s", resCreate.StatusCode, body)
	}
	var created domain.Task
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatal(err)
	}

	rows, err := st.ListDraftEvaluations(context.Background(), "draft-link-http", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	for _, row := range rows {
		if row.TaskID == nil || *row.TaskID != created.ID {
			t.Fatalf("expected task_id %q, got %#v", created.ID, row.TaskID)
		}
	}
}

func TestHTTP_task_drafts_crud(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	saveRes, err := http.Post(srv.URL+"/task-drafts", "application/json", strings.NewReader(`{
		"name":"Draft one",
		"payload":{"title":"hello","priority":"medium"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(saveRes.Body)
	_ = saveRes.Body.Close()
	if saveRes.StatusCode != http.StatusCreated {
		t.Fatalf("save status %d body %s", saveRes.StatusCode, body)
	}
	var saved struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" {
		t.Fatal("missing draft id")
	}

	listRes, err := http.Get(srv.URL + "/task-drafts")
	if err != nil {
		t.Fatal(err)
	}
	defer listRes.Body.Close()
	if listRes.StatusCode != http.StatusOK {
		t.Fatalf("list status %d", listRes.StatusCode)
	}

	getRes, err := http.Get(srv.URL + "/task-drafts/" + saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	getBody, _ := io.ReadAll(getRes.Body)
	_ = getRes.Body.Close()
	if getRes.StatusCode != http.StatusOK {
		t.Fatalf("get status %d body %s", getRes.StatusCode, getBody)
	}

	delReq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/task-drafts/"+saved.ID, nil)
	delRes, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = delRes.Body.Close()
	if delRes.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status %d", delRes.StatusCode)
	}
}

func TestHTTP_task_drafts_list_limit_zero_coerces_to_default(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	ctx := context.Background()
	for i := 0; i < 55; i++ {
		name := "draft-" + strconv.Itoa(i)
		if _, err := st.SaveDraft(ctx, "", name, []byte(`{}`)); err != nil {
			t.Fatalf("seed draft %d: %v", i, err)
		}
	}
	res, err := http.Get(srv.URL + "/task-drafts?limit=0")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		Drafts []json.RawMessage `json:"drafts"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Drafts) > 50 {
		t.Fatalf("limit=0: got %d drafts want <=50", len(body.Drafts))
	}
}

func TestHTTP_task_drafts_list_overlong_limit(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	long := strings.Repeat("1", maxListIntQueryParamBytes+1)
	res, err := http.Get(srv.URL + "/task-drafts?limit=" + long)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("overlong limit: status %d want %d", res.StatusCode, http.StatusBadRequest)
	}
}

func TestHTTP_create_duplicate_client_id_returns_409(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	id := "30000000-0000-4000-8000-000000000099"
	res1, err := http.Post(srv.URL+"/tasks", "application/json",
		strings.NewReader(`{"id":"`+id+`","title":"first","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = res1.Body.Close()
	if res1.StatusCode != http.StatusCreated {
		t.Fatalf("first create status %d", res1.StatusCode)
	}
	res2, err := http.Post(srv.URL+"/tasks", "application/json",
		strings.NewReader(`{"id":"`+id+`","title":"second","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(res2.Body)
		t.Fatalf("status %d body %s", res2.StatusCode, b)
	}
	var errBody jsonErrorBody
	if err := json.NewDecoder(res2.Body).Decode(&errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Error != "task id already exists" {
		t.Fatalf("error message %q", errBody.Error)
	}
}
