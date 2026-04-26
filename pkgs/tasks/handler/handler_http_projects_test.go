package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestHTTP_projectsCRUDAndContext(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/projects", "application/json", strings.NewReader(`{"name":"Moat","description":"Long work","context_summary":"Shared memory"}`))
	if err != nil {
		t.Fatal(err)
	}
	projectBytes, err := io.ReadAll(res.Body)
	if cerr := res.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create project status %d body %s", res.StatusCode, projectBytes)
	}
	var project domain.Project
	if err := json.Unmarshal(projectBytes, &project); err != nil {
		t.Fatal(err)
	}
	if project.ID == "" || project.Status != domain.ProjectStatusActive {
		t.Fatalf("project = %#v", project)
	}

	itemRes, err := http.Post(srv.URL+"/projects/"+project.ID+"/context", "application/json", strings.NewReader(`{"kind":"decision","title":"Use relational context","body":"No vector store in v1","pinned":true}`))
	if err != nil {
		t.Fatal(err)
	}
	itemBytes, err := io.ReadAll(itemRes.Body)
	if cerr := itemRes.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if itemRes.StatusCode != http.StatusCreated {
		t.Fatalf("create context status %d body %s", itemRes.StatusCode, itemBytes)
	}
	var item domain.ProjectContextItem
	if err := json.Unmarshal(itemBytes, &item); err != nil {
		t.Fatal(err)
	}
	if item.ProjectID != project.ID || item.Kind != domain.ProjectContextKindDecision || !item.Pinned {
		t.Fatalf("context item = %#v", item)
	}

	listRes, err := http.Get(srv.URL + "/projects/" + project.ID + "/context")
	if err != nil {
		t.Fatal(err)
	}
	defer listRes.Body.Close()
	if listRes.StatusCode != http.StatusOK {
		t.Fatalf("context list status %d", listRes.StatusCode)
	}
	var list struct {
		Items []domain.ProjectContextItem `json:"items"`
	}
	if err := json.NewDecoder(listRes.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != item.ID {
		t.Fatalf("items = %#v", list.Items)
	}

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/projects/"+project.ID, strings.NewReader(`{"status":"archived"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	patchRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer patchRes.Body.Close()
	if patchRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(patchRes.Body)
		t.Fatalf("patch project status %d body %s", patchRes.StatusCode, b)
	}
	var updated domain.Project
	if err := json.NewDecoder(patchRes.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status != domain.ProjectStatusArchived {
		t.Fatalf("updated status = %q", updated.Status)
	}
}

func TestHTTP_taskProjectIDCreatePatchAndClear(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	project := postProjectJSON(t, srv, `{"name":"Project owner"}`, http.StatusCreated)
	task := postTaskJSON(t, srv, `{"title":"linked","priority":"medium","project_id":"`+project.ID+`"}`, http.StatusCreated)
	if task.ProjectID == nil || *task.ProjectID != project.ID {
		t.Fatalf("created task project_id = %#v, want %s", task.ProjectID, project.ID)
	}

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+task.ID, strings.NewReader(`{"project_id":null}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("patch task project status %d body %s", res.StatusCode, b)
	}
	var cleared domain.Task
	if err := json.NewDecoder(res.Body).Decode(&cleared); err != nil {
		t.Fatal(err)
	}
	if cleared.ProjectID != nil {
		t.Fatalf("cleared task project_id = %#v, want nil", cleared.ProjectID)
	}
}

func postProjectJSON(t *testing.T, srv *httptest.Server, body string, want int) domain.Project {
	t.Helper()
	res, err := http.Post(srv.URL+"/projects", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != want {
		t.Fatalf("POST /projects status %d body %s", res.StatusCode, b)
	}
	var project domain.Project
	if err := json.Unmarshal(b, &project); err != nil {
		t.Fatal(err)
	}
	return project
}
