package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

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
