package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"
)

func getTask(t *testing.T, baseURL, id string) (*http.Response, []byte) {
	t.Helper()
	res, err := http.Get(baseURL + "/tasks/" + id)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, raw
}

func TestHTTP_getTask_flatTaskEnvelope(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	id := mustCreateTask(t, srv.URL, `{"title":"root-leaf","priority":"medium"}`)

	res, raw := getTask(t, srv.URL, id)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d (want 200) body=%s", res.StatusCode, raw)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode envelope: %v body=%s", err, raw)
	}

	wantKeys := []string{"created_at", "cursor_model", "id", "initial_prompt", "pickup_not_before", "priority", "runner", "runner_config", "status", "title"}
	gotKeys := make([]string, 0, len(top))
	for k := range top {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	sort.Strings(wantKeys)
	if !equalStringSlices(gotKeys, wantKeys) {
		t.Fatalf("flat task envelope keys=%v want %v", gotKeys, wantKeys)
	}
	if strings.Contains(string(raw), `"children"`) {
		t.Fatalf("body=%s contains \"children\" key (flat task response)", raw)
	}
}

func TestHTTP_getTask_pathSegmentGuard(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	cases := []struct {
		name string
		path string
		want string
	}{
		{"whitespaceOnlyID", "%20%20%20", "id"},
		{"overlongID", strings.Repeat("a", maxTaskPathIDBytes+1), "id too long"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := getTask(t, srv.URL, tc.path)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatalf("decode: %v body=%s", err, raw)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q", errBody.Error, tc.want)
			}
		})
	}
}

func TestHTTP_getTask_unknownIDIs404(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, raw := getTask(t, srv.URL, "11111111-1111-4111-8111-111111111111")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d (want 404) body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if errBody.Error != "not found" {
		t.Fatalf("error=%q want %q", errBody.Error, "not found")
	}
}

func TestHTTP_getTask_trailingSlashIsMux404(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/tasks/")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d (want 404 from mux) body=%s", res.StatusCode, raw)
	}
	if strings.Contains(string(raw), `"tasks":`) {
		t.Fatalf("body=%s looks like the GET /tasks list envelope", raw)
	}
	if strings.Contains(string(raw), `"error"`) {
		t.Fatalf("body=%s contains JSON error envelope; mux 404 must produce text body only", raw)
	}
}

func TestHTTP_getTask_neverPublishesOnSSE(t *testing.T) {
	srv, _, hub := newSSETriggerServer(t)
	defer srv.Close()

	id := mustCreateTask(t, srv.URL, `{"title":"p","priority":"medium"}`)

	ch, unsub := hub.Subscribe()
	defer unsub()

	if res, raw := getTask(t, srv.URL, id); res.StatusCode != http.StatusOK {
		t.Fatalf("get task status %d body=%s", res.StatusCode, raw)
	}
	res404, raw404 := getTask(t, srv.URL, "11111111-1111-4111-8111-111111111111")
	if res404.StatusCode != http.StatusNotFound {
		t.Fatalf("get unknown status %d body=%s", res404.StatusCode, raw404)
	}

	got := summarize(drainSSE(t, ch, 0, 200*time.Millisecond))
	if len(got) != 0 {
		t.Fatalf("drained SSE events %v after GET /tasks/{id}; want zero", got)
	}
}
