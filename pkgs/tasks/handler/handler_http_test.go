package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestHTTP_create_and_list(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"hello"}`))
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

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"x","nope":1}`))
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
}

func TestHTTP_patch_and_delete(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"t"}`))
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

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"   "}`))
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

	body := `{"title":"ok","status":"not_a_real_status"}`
	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(body))
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

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"t"}`))
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
		t.Fatalf("default priority: %s", created.Priority)
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

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"x"}`))
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
		strings.NewReader(`{"title":"t","initial_prompt":"@nope.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res2.Body)
		t.Fatalf("create status %d body %s", res2.StatusCode, b)
	}

	res3, err := http.Post(srv.URL+"/tasks", "application/json",
		strings.NewReader(`{"title":"t2","initial_prompt":"@note.txt(1-2)"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res3.Body.Close()
	if res3.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res3.Body)
		t.Fatalf("create valid mention status %d body %s", res3.StatusCode, b)
	}
}
