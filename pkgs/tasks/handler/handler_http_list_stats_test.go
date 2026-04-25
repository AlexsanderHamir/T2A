package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
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

// TestHTTP_tasks_stats_scheduled_count pins the Stage 6 invariant for
// the new top-level `scheduled` counter on `GET /tasks/stats`. The
// counter must reflect tasks where `status='ready' AND
// pickup_not_before > now` — i.e. the same predicate
// `ready.ListQueueCandidates` uses to *exclude* a row from the SQL
// queue. This lets stats consumers distinguish "0 ready, 12 scheduled"
// (intentionally deferred) from "0 ready, 0 scheduled"
// (truly idle), the central UX goal of the scheduling feature.
//
// Drives four tasks:
//   - ready, no pickup_not_before                  → counted as ready, NOT scheduled
//   - ready, pickup_not_before in the past         → counted as ready, NOT scheduled
//   - ready, pickup_not_before far in the future   → counted as ready AND scheduled
//   - running, pickup_not_before far in the future → NOT counted (status filter)
//
// The +1h horizon is large enough to dwarf any clock skew between the
// HTTP server and the test process; the past time is fixed at "1 hour
// ago" for the same reason.
func TestHTTP_tasks_stats_scheduled_count(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()

	// Disable the global agent_pickup_delay_seconds default
	// (5s) so a bare `POST /tasks` body without
	// `pickup_not_before` lands with a *nil* schedule rather
	// than auto-deferred ~5s into the future. Otherwise every
	// "ready, no schedule" row spuriously satisfies the
	// `pickup_not_before > now` predicate this test pins.
	zero := 0
	if _, err := st.UpdateSettings(context.Background(), store.SettingsPatch{AgentPickupDelaySeconds: &zero}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	now := time.Now().UTC()
	pastISO := now.Add(-1 * time.Hour).Format(time.RFC3339)
	futureISO := now.Add(1 * time.Hour).Format(time.RFC3339)

	for _, body := range []string{
		`{"title":"ready no schedule","priority":"medium","status":"ready"}`,
		`{"title":"ready past schedule","priority":"medium","status":"ready","pickup_not_before":"` + pastISO + `"}`,
		`{"title":"ready future schedule","priority":"medium","status":"ready","pickup_not_before":"` + futureISO + `"}`,
		`{"title":"running future schedule","priority":"medium","status":"running","pickup_not_before":"` + futureISO + `"}`,
	} {
		res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("create task: %v body=%s", err, body)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("create status %d body=%s", res.StatusCode, body)
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
		Ready     int64 `json:"ready"`
		Scheduled int64 `json:"scheduled"`
	}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Ready != 3 {
		t.Fatalf("ready=%d want 3 (3 ready rows regardless of pickup_not_before)", got.Ready)
	}
	if got.Scheduled != 1 {
		t.Fatalf("scheduled=%d want 1 (only the ready+future row); past schedule and running+future must not count", got.Scheduled)
	}
}
