package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/handler/readpolicy"
)

func TestHTTP_bootstrap_returns_aggregate_envelope(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/v1/bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d, want 200, body=%s", res.StatusCode, body)
	}
	if got := res.Header.Get("Content-Type"); got == "" || got[:16] != "application/json" {
		t.Errorf("Content-Type = %q, want application/json...", got)
	}
	if res.Header.Get("ETag") == "" {
		t.Error("bootstrap response missing ETag header")
	}

	var body map[string]json.RawMessage
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, field := range []string{"settings", "tasks", "stats", "projects", "drafts"} {
		if _, ok := body[field]; !ok {
			t.Errorf("bootstrap envelope missing field %q (have %v)", field, bootstrapBodyKeys(body))
		}
	}

	// tasks payload must be the same wire shape as GET /tasks.
	var tasksPayload struct {
		Tasks   json.RawMessage `json:"tasks"`
		Limit   int             `json:"limit"`
		Offset  int             `json:"offset"`
		HasMore bool            `json:"has_more"`
	}
	if err := json.Unmarshal(body["tasks"], &tasksPayload); err != nil {
		t.Fatalf("decode tasks payload: %v", err)
	}
	if tasksPayload.Limit != readpolicy.BootstrapListLimit {
		t.Errorf("tasks.limit = %d, want %d", tasksPayload.Limit, readpolicy.BootstrapListLimit)
	}

	var projectsPayload struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(body["projects"], &projectsPayload); err != nil {
		t.Fatalf("decode projects payload: %v", err)
	}
	if projectsPayload.Limit != readpolicy.BootstrapProjectsLimit {
		t.Errorf("projects.limit = %d, want %d", projectsPayload.Limit, readpolicy.BootstrapProjectsLimit)
	}
}

func TestHTTP_bootstrap_returns_304_on_if_none_match(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	first, err := http.Get(srv.URL + "/v1/bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(first.Body)
	_ = first.Body.Close()
	etag := first.Header.Get("ETag")
	if etag == "" {
		t.Fatal("first response missing ETag")
	}

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/bootstrap", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("If-None-Match", etag)
	second, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Body.Close()

	if second.StatusCode != http.StatusNotModified {
		body, _ := io.ReadAll(second.Body)
		t.Fatalf("status = %d, want 304, body=%s", second.StatusCode, body)
	}
	if got := second.Header.Get("ETag"); got != etag {
		t.Errorf("ETag on 304 = %q, want %q", got, etag)
	}
}

func bootstrapBodyKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
