package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestHTTP_createTask_tagsMilestoneDependsOn(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	dep := mustCreateTask(t, srv.URL, `{"title":"dep","priority":"medium","status":"ready"}`)
	res, raw := postCreate(t, srv.URL, `{
		"title":"blocked",
		"priority":"medium",
		"status":"ready",
		"tags":["infra","api"],
		"milestone":"launch-v1",
		"depends_on":["`+dep+`"],
		"gate":{"kind":"manual_approval","status":"active","hold":false}
	}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var node struct {
		domain.Task
		Children []any `json:"children,omitempty"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatal(err)
	}
	if len(node.Tags) != 2 || node.Milestone == nil || *node.Milestone != "launch-v1" {
		t.Fatalf("tags/milestone: tags=%v milestone=%v", node.Tags, node.Milestone)
	}
	if len(node.DependsOn) != 1 || node.DependsOn[0] != dep {
		t.Fatalf("depends_on=%v", node.DependsOn)
	}
	if node.Gate == nil || node.Gate.Status != domain.ProjectStepGateActive {
		t.Fatalf("gate=%+v", node.Gate)
	}
}

func TestHTTP_patchTask_depCycleRejected(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	a := mustCreateTask(t, srv.URL, `{"title":"a","priority":"medium"}`)
	b := mustCreateTask(t, srv.URL, `{"title":"b","priority":"medium"}`)
	res, raw := patchTask(t, srv.URL, a, `{"depends_on":["`+b+`"]}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("patch a dep b: %d %s", res.StatusCode, raw)
	}
	res, raw = patchTask(t, srv.URL, b, `{"depends_on":["`+a+`"]}`)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("cycle status %d want 400 body=%s", res.StatusCode, raw)
	}
	if !strings.Contains(string(raw), "cycle") {
		t.Fatalf("body=%s want cycle mention", raw)
	}
}

func TestHTTP_taskDependencies_endpoints(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	a := mustCreateTask(t, srv.URL, `{"title":"a","priority":"medium"}`)
	b := mustCreateTask(t, srv.URL, `{"title":"b","priority":"medium"}`)

	res, raw := httpPostJSON(t, srv.URL+"/tasks/"+a+"/dependencies", `{"depends_on_task_id":"`+b+`"}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("add dep: %d %s", res.StatusCode, raw)
	}
	res, raw = httpGet(t, srv.URL+"/tasks/"+a+"/dependencies")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list dep: %d %s", res.StatusCode, raw)
	}
	var listed taskDependenciesListResponse
	if err := json.Unmarshal(raw, &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.DependsOn) != 1 || listed.DependsOn[0] != b {
		t.Fatalf("listed=%v", listed.DependsOn)
	}
	res, _ = httpDelete(t, srv.URL+"/tasks/"+a+"/dependencies/"+b)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete dep status %d", res.StatusCode)
	}
}

func TestHTTP_patchTaskGate_release(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	id := mustCreateTask(t, srv.URL, `{
		"title":"gated",
		"priority":"medium",
		"gate":{"kind":"manual_approval","status":"active","hold":false}
	}`)
	res, raw := patchTask(t, srv.URL, id, `{"gate":null}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("clear gate via patch: %d %s", res.StatusCode, raw)
	}
	res, raw = httpPatchJSON(t, srv.URL+"/tasks/"+id+"/gate", `{"action":"release"}`)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("release without gate: %d %s", res.StatusCode, raw)
	}
	id2 := mustCreateTask(t, srv.URL, `{
		"title":"gated2",
		"priority":"medium",
		"gate":{"kind":"manual_approval","status":"pending_release","hold":false}
	}`)
	res, raw = httpPatchJSON(t, srv.URL+"/tasks/"+id2+"/gate", `{"action":"release"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("release: %d %s", res.StatusCode, raw)
	}
	var node struct {
		domain.Task
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatal(err)
	}
	if node.Gate == nil || node.Gate.Status != domain.ProjectStepGateReleased {
		t.Fatalf("gate=%+v", node.Gate)
	}
}

func httpPostJSON(t *testing.T, url, body string) (*http.Response, []byte) {
	t.Helper()
	res, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := ioReadAll(res)
	return res, raw
}

func httpPatchJSON(t *testing.T, url, body string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := ioReadAll(res)
	return res, raw
}

func httpGet(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := ioReadAll(res)
	return res, raw
}

func httpDelete(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := ioReadAll(res)
	return res, raw
}

func ioReadAll(res *http.Response) ([]byte, error) {
	defer res.Body.Close()
	return io.ReadAll(res.Body)
}
