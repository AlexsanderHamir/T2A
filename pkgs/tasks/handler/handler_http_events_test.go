package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

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
